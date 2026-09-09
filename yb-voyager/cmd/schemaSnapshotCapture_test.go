//go:build unit

/*
Copyright (c) YugabyteDB, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

// TestSnapshotStartReasonFor pins the export-data start-snapshot reason classification.
// The actual data-directory state is decided BEFORE the --start-clean flag, so
// clean_restart only applies when prior export-data output actually exists to discard.
// In particular --start-clean on an empty export dir — including passing it on the very
// first run — is "initial", not a mislabeled "clean_restart".
func TestSnapshotStartReasonFor(t *testing.T) {
	tests := []struct {
		name         string
		startClean   bool
		dataDirEmpty bool
		want         string
	}{
		{"first run (empty dir, no start-clean)", false, true, schemasnapshot.ReasonInitial},
		{"start-clean on the first run (empty dir) is initial, not clean_restart", true, true, schemasnapshot.ReasonInitial},
		{"start-clean on a re-run with prior output is clean_restart", true, false, schemasnapshot.ReasonCleanRestart},
		{"prior output without start-clean is resume", false, false, schemasnapshot.ReasonResume},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, snapshotStartReasonFor(tt.startClean, tt.dataDirEmpty))
		})
	}
}

// TestExportDataExitReason pins the abnormal-exit reason classification, which is shared
// by the atexit hook and by exportData's exit defer. Verified end-to-end against
// PostgreSQL — before the two shared this, SIGINT/SIGTERM/SIGUSR2 all recorded
// reason=error.
func TestExportDataExitReason(t *testing.T) {
	origShutdown, origEndMigration := ProcessShutdownRequested.Load(), EndMigrationStopRequested.Load()
	t.Cleanup(func() {
		ProcessShutdownRequested.Store(origShutdown)
		EndMigrationStopRequested.Store(origEndMigration)
	})

	tests := []struct {
		name             string
		shutdownReq      bool
		endMigrationStop bool
		want             string
	}{
		{"no signal: a genuine failure is an error", false, false, schemasnapshot.ReasonError},
		{"SIGINT/SIGTERM is a user interrupt", true, false, schemasnapshot.ReasonInterrupt},
		{"SIGUSR2 (end migration teardown) is a clean completion", true, true, schemasnapshot.ReasonComplete},
		// Unreachable in production: main.go sets ProcessShutdownRequested before
		// EndMigrationStopRequested, so the end-migration flag is never observed on its
		// own. Pinned anyway so the classification is defined for every input.
		{"end-migration flag alone is a completion", false, true, schemasnapshot.ReasonComplete},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ProcessShutdownRequested.Store(tt.shutdownReq)
			EndMigrationStopRequested.Store(tt.endMigrationStop)
			assert.Equal(t, tt.want, exportDataExitReason())
		})
	}
}

// TestCaptureExportDataExitSnapshotRoleGate pins the exporter-role gate: only the source
// exporter records source schema snapshots, so `export data from target` must not.
//
// The gate is asserted through the one-shot claim rather than through a captured
// snapshot, which keeps it a unit test: a gated-off call must not even claim, since
// claiming would consume the single exit capture a run is allowed and silently suppress
// the real one. schemasnapshot.SourceCapture deliberately knows nothing about roles, so
// this belongs here rather than with the capture policy.
//
// No database is involved. DBType is set so source.DB() can construct a handle
// (newSourceDB panics on an empty type) and the capture is disabled so it short-circuits
// on its own gate before reaching the nil metaDB.
func TestCaptureExportDataExitSnapshotRoleGate(t *testing.T) {
	origRole, origSource, origDisabled := exporterRole, source, disableSchemaSnapshotCapture
	t.Cleanup(func() {
		exporterRole, source, disableSchemaSnapshotCapture = origRole, origSource, origDisabled
		exportDataExitSnapshotCaptured.Store(false)
	})

	source = srcdb.Source{DBType: POSTGRESQL}
	disableSchemaSnapshotCapture = utils.BoolStr(true)

	tests := []struct {
		name       string
		role       string
		wantClaim  bool
		wantReason string
	}{
		{"a target-side exporter must not capture", TARGET_DB_EXPORTER_FF_ROLE, false,
			"a non-source exporter must not claim the exit capture"},
		{"a fall-back exporter must not capture", TARGET_DB_EXPORTER_FB_ROLE, false,
			"a non-source exporter must not claim the exit capture"},
		{"the source exporter captures", SOURCE_DB_EXPORTER_ROLE, true,
			"the source exporter must claim the exit capture"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exportDataExitSnapshotCaptured.Store(false)
			exporterRole = tt.role

			captureExportDataExitSnapshot(context.Background(), schemasnapshot.ReasonInterrupt)

			assert.Equal(t, tt.wantClaim, exportDataExitSnapshotCaptured.Load(), tt.wantReason)
		})
	}
}

// TestValidateSchemaSnapshotCaptureInterval pins the flag validation. An interval of 0
// or less would silently disable periodic capture, so it must fail at startup instead.
//
// The last case pins the flag-presence scoping: with the flag absent from the command,
// the guard no-ops whatever the global holds. It sets 0 explicitly to exercise that,
// which a real `export data from target` run would not -- IntVar writes 60 into the
// global at registration, so that scoping is defensive rather than load-bearing.
func TestValidateSchemaSnapshotCaptureInterval(t *testing.T) {
	orig := schemaSnapshotCaptureInterval
	t.Cleanup(func() { schemaSnapshotCaptureInterval = orig })

	tests := []struct {
		name       string
		registered bool // whether the command under test exposes the flag at all
		interval   int
		wantErr    bool
	}{
		{"default interval passes", true, 60, false},
		{"the minimum interval passes", true, 1, false},
		{"zero is rejected", true, 0, true},
		{"negative is rejected", true, -5, true},
		{"a command without the flag is not validated (export data from target)", false, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "export-stub"}
			if tt.registered {
				registerSchemaSnapshotIntervalFlag(cmd)
			}
			// registerSchemaSnapshotIntervalFlag binds the global and resets it to the
			// flag default, so set the value under test after building the command.
			schemaSnapshotCaptureInterval = tt.interval

			err := validateSchemaSnapshotCaptureInterval(cmd)
			if tt.wantErr {
				require.Error(t, err, "an interval of %d must be rejected", tt.interval)
				assert.Contains(t, err.Error(), "must be at least 1",
					"the error must say what a valid interval is")
				return
			}
			require.NoError(t, err)
		})
	}
}
