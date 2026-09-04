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
	"sync/atomic"
	"time"

	goerrors "github.com/go-errors/errors"
	log "github.com/sirupsen/logrus"
	"github.com/tebeka/atexit"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
)

// exportDataExitSnapshotCaptured claims the one exit capture a run is allowed.
//
// A signal makes both exit paths run at once: the handler fires on the signal
// goroutine while the export goroutine unwinds through its exit defer. Claiming
// has to be a single atomic compare-and-swap, so exactly one of them captures --
// checking a flag and setting it after the capture lets both pass the check and
// write two exit snapshots.
var exportDataExitSnapshotCaptured atomic.Bool

// captureExportDataExitSnapshot captures the exit snapshot and marks it captured, so
// no later site fires a second one. Source-exporter only. The caller chooses the
// context; captureSourceSchemaSnapshot caps it at schemasnapshot.CaptureTimeout.
func captureExportDataExitSnapshot(ctx context.Context, reason string) {
	if exporterRole != SOURCE_DB_EXPORTER_ROLE {
		return
	}
	// Claim before capturing, not after: see exportDataExitSnapshotCaptured.
	if !exportDataExitSnapshotCaptured.CompareAndSwap(false, true) {
		log.Infof("schema-snapshot exit capture already recorded; skipping the %q capture", reason)
		return
	}
	captureSourceSchemaSnapshotBestEffort(ctx, schemasnapshot.LabelExportDataFromSourceExit, reason, true)
}

// captureExportDataExitSnapshotFresh is captureExportDataExitSnapshot on a fresh
// context, for exit sites where the run's own context may already be cancelled: the
// error `return false` paths (which must capture inline, before exportData's deferred
// Disconnect closes the connection) and the atexit fallback.
func captureExportDataExitSnapshotFresh(reason string) {
	captureExportDataExitSnapshot(context.Background(), reason)
}

// exportDataExitReason classifies an abnormal exit from the shutdown flags:
// SIGINT/SIGTERM is an interrupt, SIGUSR2 (end-migration teardown) a clean
// completion, anything else a genuine error.
//
// The inline error paths must use this too, not assume ReasonError: a signal kills
// the in-flight child, so the export reports failure and reaches `return false`
// first, and the atexit hook then no-ops. Hardcoding ReasonError there recorded
// every Ctrl-C as an error.
//
// Reading the flags races with main.go's signal goroutine, benignly: they are only
// ever set, and the worst case is today's unconditional behaviour.
func exportDataExitReason() string {
	if !ProcessShutdownRequested {
		return schemasnapshot.ReasonError
	}
	if EndMigrationStopRequested {
		return schemasnapshot.ReasonComplete
	}
	return schemasnapshot.ReasonInterrupt
}

// registerExportDataExitSnapshotHook covers the exit paths that never unwind, so
// exportData's exit defer cannot run: signals, and utils.ErrExit (whose os.Exit skips
// defers, leaving the connection open). Whichever path gets there first wins the claim.
func registerExportDataExitSnapshotHook() {
	atexit.Register(func() {
		captureExportDataExitSnapshotFresh(exportDataExitReason())
	})
}

// captureSourceSchemaSnapshotBestEffort is captureSourceSchemaSnapshot with the
// policy the export hooks need: a failed snapshot is logged and swallowed, never
// surfaced. Schema capture is off the data path, so an export must not fail or stall
// because a snapshot could not be taken.
//
// Every export hook goes through here. Anything that genuinely wants to act on a
// failure calls captureSourceSchemaSnapshot directly and handles the error.
func captureSourceSchemaSnapshotBestEffort(ctx context.Context, label, reason string, placeholderOnFailure bool) {
	if err := captureSourceSchemaSnapshot(ctx, label, reason, placeholderOnFailure); err != nil {
		log.Warnf("schema-snapshot capture for label %q failed (continuing, migration unaffected): %v", label, err)
	}
}

// captureSourceSchemaSnapshot captures the source schema and persists it as a snapshot
// for the given label/reason, returning why it could not.
//
// A skip is not an error: a non-PostgreSQL source or --suppress-schema-snapshot-capture
// returns nil, since nothing went wrong. Honors exporter-role gating in the caller.
//
// Callers decide what a failure means. The export hooks want it swallowed, so they use
// captureSourceSchemaSnapshotBestEffort.
func captureSourceSchemaSnapshot(ctx context.Context, label, reason string, placeholderOnFailure bool) error {
	if source.DBType != POSTGRESQL {
		log.Infof("schema-snapshot capture skipped for label %q: only PostgreSQL sources are supported", label)
		return nil
	}
	if bool(suppressSchemaSnapshotCapture) {
		log.Infof("schema-snapshot capture suppressed (--suppress-schema-snapshot-capture); skipping %s", label)
		return nil
	}

	// Bound the catalog read and the metaDB write together, so a wedged source can
	// never block the migration on best-effort work. A caller's tighter deadline
	// still wins. (The placeholder path below deliberately uses a fresh context.)
	ctx, cancel := context.WithTimeout(ctx, schemasnapshot.CaptureTimeout)
	defer cancel()

	pg, ok := source.DB().(*srcdb.PostgreSQL)
	if !ok {
		return goerrors.Errorf("PostgreSQL source lacks a *srcdb.PostgreSQL handle")
	}
	db := pg.GetDB()
	if db == nil {
		if placeholderOnFailure {
			// Still record the timeline marker so a lifecycle moment (e.g. an exit)
			// isn't lost just because the DB handle is gone during teardown.
			saveSourceSchemaSnapshotPlaceholder(label, reason)
		}
		return goerrors.Errorf("no active database handle")
	}
	captureParams := schemasnapshot.CaptureParams{
		DatabaseType: source.DBType,
		DBMetadata:   schemasnapshot.DBMetadata{Host: source.Host, Port: source.Port, Database: source.DBName, User: source.User},
		Schemas:      source.GetSchemaListUnquoted(),
		Label:        label,
		Reason:       reason,
	}

	// Every capture is persisted unconditionally — no dedup. Periodic snapshots record the
	// source schema at each interval so the drift timeline shows exactly when a change
	// appeared, even when consecutive snapshots are identical.
	req := schemasnapshot.CaptureRequest{
		CaptureParams:        captureParams,
		PlaceholderOnFailure: placeholderOnFailure,
	}
	name, err := schemasnapshot.CaptureAndSaveSnapshot(ctx, db, metaDB, req)
	if err != nil {
		return err
	}
	log.Infof("captured schema snapshot %q", name)
	return nil
}

// startPeriodicSourceSchemaSnapshotCapture tickers a capture every `interval` for the
// whole export, snapshot and streaming phases alike. The interval is a parameter, not
// the global, so tests can use a small one. The goroutine stops when ctx is cancelled,
// so no separate stop function is needed.
//
// Best-effort: a no-op when suppressed, when interval <= 0, or off the source exporter.
func startPeriodicSourceSchemaSnapshotCapture(ctx context.Context, interval time.Duration) {
	if exporterRole != SOURCE_DB_EXPORTER_ROLE || source.DBType != POSTGRESQL || bool(suppressSchemaSnapshotCapture) {
		return
	}
	if interval <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				captureSourceSchemaSnapshotBestEffort(ctx, schemasnapshot.LabelExportDataFromSourcePeriodic, "", false)
			}
		}
	}()
}

// saveSourceSchemaSnapshotPlaceholder records a metadata-only timeline marker for a
// moment we can't fully capture. Best-effort; honors suppression.
//
// It uses its OWN fresh, bounded context: the capture context may be exactly what
// died, and reusing it would drop the marker just when it is needed.
func saveSourceSchemaSnapshotPlaceholder(label, reason string) {
	if source.DBType != POSTGRESQL {
		return
	}
	if bool(suppressSchemaSnapshotCapture) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), schemasnapshot.CaptureTimeout)
	defer cancel()
	h := schemasnapshot.SnapshotHeader{
		Label:         label,
		Reason:        reason,
		Side:          schemasnapshot.SideSource,
		CapturedAt:    time.Now().UTC(),
		Schemas:       source.GetSchemaListUnquoted(),
		IsPlaceholder: true,
	}
	if _, err := schemasnapshot.SavePlaceholder(ctx, metaDB, h); err != nil {
		log.Warnf("schema-snapshot placeholder for label %q failed: %v", label, err)
	}
}
