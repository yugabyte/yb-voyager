//go:build integration_live_migration

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
package testlivemigration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/cmd"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schema/schemadrift"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemadiff"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

const exportDataDriftReport = "drift_analysis_report_export_data.json"

// startStreamingExportForDriftCheck starts a live export and returns once the
// source exporter has recorded that Debezium is streaming changes. Parallel
// tests share the source container, so each passes its own database.
func startStreamingExportForDriftCheck(t *testing.T, databaseName string) *LiveMigrationTest {
	return startStreamingExportForDriftCheckWith(t, databaseName, nil, nil)
}

// startStreamingExportForDriftCheckWith also runs extraSchemaSQL after the base
// schema and passes extraExportArgs to export data.
func startStreamingExportForDriftCheckWith(t *testing.T, databaseName string, extraSchemaSQL []string, extraExportArgs map[string]string) *LiveMigrationTest {
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: databaseName,
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: append([]string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.orders (id SERIAL PRIMARY KEY, amount NUMERIC);`,
		}, extraSchemaSQL...),
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.orders REPLICA IDENTITY FULL;`,
		},
		// export data skips empty tables, so seed one row for streaming to start.
		InitialDataSQL: []string{
			`INSERT INTO test_schema.orders (amount) VALUES (100);`,
		},
		CleanupSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema CASCADE;`,
		},
	})
	t.Cleanup(lm.Cleanup)

	testutils.FatalIfError(t, lm.SetupContainers(context.Background()), "failed to setup containers")
	testutils.FatalIfError(t, lm.SetupSchema(), "failed to setup schema")
	// The check follows capture, so pin it on rather than rely on its default.
	exportArgs := map[string]string{"--disable-schema-snapshot-capture": "false"}
	for k, v := range extraExportArgs {
		exportArgs[k] = v
	}
	testutils.FatalIfError(t, lm.StartExportData(true, exportArgs), "failed to start export data")

	// Set right after the exporter sees the switch to streaming, before it next
	// checks whether Debezium is still running.
	require.Eventually(t, func() bool {
		started := false
		err := lm.WithMetaDB(0, func(m *metadb.MetaDB) error {
			msr, err := m.GetMigrationStatusRecord()
			if err != nil {
				return err
			}
			if msr != nil {
				started = msr.ExportDataFromSourceStarted
			}
			return nil
		})
		if err != nil {
			t.Logf("reading the migration status record: %v", err)
		}
		return started
	}, 4*time.Minute, 2*time.Second, "export data never started streaming changes")
	return lm
}

// assertNoDriftCheck pins that export data neither ran the check nor wrote its report.
func assertNoDriftCheck(t *testing.T, lm *LiveMigrationTest) {
	assert.NotContains(t, lm.GetExportCommandStdout(), "Checking the source schema for drift")
	assert.NoFileExists(t, filepath.Join(lm.GetCurrentExportDir(), "reports", exportDataDriftReport))
}

// TestLiveExportDataChecksSchemaDriftWhenDebeziumDiesWhileStreaming pins the
// drift check on export failure: a column added mid-stream makes Debezium fail on
// its own once a row uses it. The drift summary then follows "Export of data
// failed!", the check writes its own report, and the export still exits 1.
func TestLiveExportDataChecksSchemaDriftWhenDebeziumDiesWhileStreaming(t *testing.T) {
	t.Parallel()
	lm := startStreamingExportForDriftCheck(t, "drift_on_failure")

	// Voyager's exporter fails on a change event carrying a column it holds no
	// schema for, so the row after the ALTER is what stops Debezium.
	testutils.FatalIfError(t, lm.ExecuteOnSource(
		`INSERT INTO test_schema.orders (amount) VALUES (150);`,
		`ALTER TABLE test_schema.orders ADD COLUMN note TEXT;`,
		`INSERT INTO test_schema.orders (amount, note) VALUES (200, 'after the change');`,
	), "failed to change the schema mid-stream")
	require.Error(t, lm.WaitForExportDataExitTimeout(3*time.Minute), "export data must fail once Debezium meets the new column")
	assert.Equal(t, testutils.ExitCode(1), lm.exportCmd.ExitCode(), "the check must not change the export's exit code")

	stdout := lm.GetExportCommandStdout()
	failedAt := strings.Index(stdout, "Export of data failed!")
	require.GreaterOrEqual(t, failedAt, 0, "export output: %s", stdout)
	afterFailure := stdout[failedAt:]
	checkAt := strings.Index(afterFailure, "Checking the source schema for drift...")
	require.GreaterOrEqual(t, checkAt, 0, "export output: %s", stdout)
	summary := afterFailure[checkAt:]
	assert.Contains(t, summary, "Changes detected  : 1")
	assert.Contains(t, summary, "drift_analysis_report_export_data.html")
	assert.Contains(t, summary, exportDataDriftReport)

	reportsDir := filepath.Join(lm.GetCurrentExportDir(), "reports")
	raw, err := os.ReadFile(filepath.Join(reportsDir, exportDataDriftReport))
	require.NoError(t, err)
	var report schemadrift.Report
	require.NoError(t, json.Unmarshal(raw, &report))
	assert.Equal(t, 1, report.Summary.ChangeCount)
	assert.False(t, report.Summary.LiveCompared, "the check takes no live read")
	require.Len(t, report.Drifts, 1)
	assert.Equal(t, schemadiff.ColumnAdded, report.Drifts[0].Type)
	assert.Equal(t, schemasnapshot.ObjectRef{Schema: "test_schema", Name: "orders"}, report.Drifts[0].Object)
	assert.Equal(t, "note", report.Drifts[0].SubObject)
	assert.NoFileExists(t, filepath.Join(reportsDir, cmd.DRIFT_REPORT_FILE_NAME+".json"),
		"the check must leave the schema detect-drift report alone")
}

// TestLiveExportDataSkipsSchemaDriftCheckWhenStopped pins the end-to-end
// property that stopping export data is never reported as drift, even with drift
// on the source. StopExportData sends SIGTERM to Voyager, as end migration does;
// Voyager's own signal handling then ends the run before either interrupt guard
// in the drift path is reached. The guards themselves are pinned by
// TestLiveExportDataSkipsSchemaDriftCheckWhenDebeziumIsTerminated and by
// TestDebeziumFailure.
func TestLiveExportDataSkipsSchemaDriftCheckWhenStopped(t *testing.T) {
	t.Parallel()
	lm := startStreamingExportForDriftCheck(t, "drift_on_stop")

	testutils.FatalIfError(t, lm.ExecuteOnSource(`ALTER TABLE test_schema.orders ADD COLUMN note TEXT;`), "failed to add a column")
	require.NoError(t, lm.StopExportData())

	assertNoDriftCheck(t, lm)
}

// TestLiveExportDataSkipsSchemaDriftCheckWhenDebeziumIsTerminated pins the
// exit-code guard: Ctrl-C reaches Debezium too, and it can exit with 143 (or
// 130) before Voyager's own handler runs. SIGTERM to Debezium alone reproduces
// that ordering, and the export must fail without checking for drift.
func TestLiveExportDataSkipsSchemaDriftCheckWhenDebeziumIsTerminated(t *testing.T) {
	t.Parallel()
	lm := startStreamingExportForDriftCheck(t, "drift_on_dbz_sigterm")

	testutils.FatalIfError(t, lm.ExecuteOnSource(`ALTER TABLE test_schema.orders ADD COLUMN note TEXT;`), "failed to add a column")
	pidStr, err := dbzm.GetPIDOfDebeziumOnExportDir(lm.GetCurrentExportDir(), cmd.SOURCE_DB_EXPORTER_ROLE)
	require.NoError(t, err)
	pid, err := strconv.Atoi(strings.TrimSpace(pidStr))
	require.NoError(t, err)
	require.NoError(t, syscall.Kill(pid, syscall.SIGTERM))

	require.Error(t, lm.WaitForExportDataExitTimeout(3*time.Minute), "export data must fail once Debezium is terminated")
	assert.Equal(t, testutils.ExitCode(1), lm.exportCmd.ExitCode())
	logBytes, err := os.ReadFile(filepath.Join(lm.GetCurrentExportDir(), "logs", "yb-voyager-export-data.log"))
	require.NoError(t, err)
	assert.Contains(t, string(logBytes), "exit status 143", "Debezium must have exited with Java's SIGTERM code")
	assertNoDriftCheck(t, lm)
}

const cutoverCannotConfirmDrift = "Cutover not started: the source schema changed during this migration (1 change, listed above), and this run cannot ask for confirmation."

// runCutoverToTarget runs cutover with --yes and no fall-back, and returns its
// output and whether it succeeded.
func runCutoverToTarget(lm *LiveMigrationTest, extraArgs map[string]string) (string, bool) {
	err := lm.InitiateCutoverToTarget(false, extraArgs)
	return lm.GetCutoverToTargetCommandStdout() + lm.GetCutoverToTargetCommandStderr(), err == nil
}

func cutoverToTargetRequested(t *testing.T, lm *LiveMigrationTest) bool {
	requested := false
	require.NoError(t, lm.WithMetaDB(0, func(m *metadb.MetaDB) error {
		msr, err := m.GetMigrationStatusRecord()
		if err != nil {
			return err
		}
		requested = msr.CutoverToTargetRequested
		return nil
	}))
	return requested
}

// TestLiveCutoverToTargetPreCheckPassesWithoutDrift pins that an unchanged
// source lets cutover proceed after one "no drift" line.
func TestLiveCutoverToTargetPreCheckPassesWithoutDrift(t *testing.T) {
	t.Parallel()
	lm := startStreamingExportForDriftCheck(t, "cutover_no_drift")

	out, ok := runCutoverToTarget(lm, nil)
	require.True(t, ok, "cutover must succeed without drift: %s", out)
	assert.Contains(t, out, "No schema drift found on the source. Report:")
	assert.True(t, cutoverToTargetRequested(t, lm))

	raw, err := os.ReadFile(filepath.Join(lm.GetCurrentExportDir(), "reports", "drift_analysis_report_cutover_to_target.json"))
	require.NoError(t, err)
	var report schemadrift.Report
	require.NoError(t, json.Unmarshal(raw, &report))
	assert.Equal(t, 0, report.Summary.ChangeCount)
	assert.True(t, report.Summary.LiveCompared, "the pre-check must compare against a live read")
}

// TestLiveCutoverToTargetPreCheckIgnoresTablesOutsideTheExport pins the check's
// scope: export data runs with --table-list test_schema.orders, so a column added
// to test_schema.audit_log, in the same schema but never exported, is not drift
// for cutover.
func TestLiveCutoverToTargetPreCheckIgnoresTablesOutsideTheExport(t *testing.T) {
	t.Parallel()
	lm := startStreamingExportForDriftCheckWith(t, "cutover_scope",
		[]string{`CREATE TABLE test_schema.audit_log (id SERIAL PRIMARY KEY, msg TEXT);`},
		map[string]string{"--table-list": "test_schema.orders"})

	testutils.FatalIfError(t, lm.ExecuteOnSource(`ALTER TABLE test_schema.audit_log ADD COLUMN note TEXT;`), "failed to add a column")

	out, ok := runCutoverToTarget(lm, nil)
	require.True(t, ok, "drift outside the exported tables must not stop cutover: %s", out)
	assert.Contains(t, out, "No schema drift found on the source. Report:")
	assert.True(t, cutoverToTargetRequested(t, lm))
}

// TestLiveCutoverToTargetPreCheckBlocksOnDrift pins the pre-check's failure
// paths in the order a user meets them: a check that cannot connect fails
// cutover, drift found under --yes fails it, so does a "y" piped in without
// --yes, --skip-pre-checks lets it through, and a re-run once cutover is
// requested does not check again. The column is added after streaming starts,
// so only the live read sees it.
func TestLiveCutoverToTargetPreCheckBlocksOnDrift(t *testing.T) {
	t.Parallel()
	lm := startStreamingExportForDriftCheck(t, "cutover_drift")

	out, ok := runCutoverToTarget(lm, map[string]string{"--source-db-password": "not-the-password"})
	require.False(t, ok, "a check that cannot connect must fail cutover: %s", out)
	assert.Contains(t, out, "Could not check the source schema for drift:")
	assert.Contains(t, out, "--skip-pre-checks schema_drift")
	assert.False(t, cutoverToTargetRequested(t, lm))

	testutils.FatalIfError(t, lm.ExecuteOnSource(`ALTER TABLE test_schema.orders ADD COLUMN note TEXT;`), "failed to add a column")

	out, ok = runCutoverToTarget(lm, nil)
	require.False(t, ok, "drift under --yes must fail cutover: %s", out)
	assert.Contains(t, out, "Changes detected  : 1")
	assert.Contains(t, out, cutoverCannotConfirmDrift)
	assert.False(t, cutoverToTargetRequested(t, lm))

	require.Error(t, lm.InitiateCutoverToTargetAnswering(false, nil, "y\n"), "a piped yes must not confirm drift")
	out = lm.GetCutoverToTargetCommandStdout() + lm.GetCutoverToTargetCommandStderr()
	assert.Contains(t, out, "Changes detected  : 1")
	assert.Contains(t, out, cutoverCannotConfirmDrift)
	assert.False(t, cutoverToTargetRequested(t, lm))

	raw, err := os.ReadFile(filepath.Join(lm.GetCurrentExportDir(), "reports", "drift_analysis_report_cutover_to_target.json"))
	require.NoError(t, err)
	var report schemadrift.Report
	require.NoError(t, json.Unmarshal(raw, &report))
	assert.True(t, report.Summary.LiveCompared)
	require.Len(t, report.Drifts, 1)
	assert.Equal(t, schemadiff.ColumnAdded, report.Drifts[0].Type)
	assert.Equal(t, "note", report.Drifts[0].SubObject)

	out, ok = runCutoverToTarget(lm, map[string]string{"--skip-pre-checks": "schema_drift"})
	require.True(t, ok, "--skip-pre-checks must let cutover through: %s", out)
	assert.Contains(t, out, "Skipping the schema drift check (--skip-pre-checks includes schema_drift).")
	assert.True(t, cutoverToTargetRequested(t, lm))

	// Once cutover is requested, a re-run must not check again, even with the drift
	// still on the source and no skip flag.
	out, ok = runCutoverToTarget(lm, nil)
	require.True(t, ok, "a re-run after cutover was requested must not re-check drift: %s", out)
	assert.NotContains(t, out, "Checking the source schema for drift")
	assert.Contains(t, out, "cutover to target already initiated, wait for it to complete")
}
