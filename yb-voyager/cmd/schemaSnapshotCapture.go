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
	"sync"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/tebeka/atexit"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
)

// exportDataExitSnapshotCaptured records whether the export-data run already
// captured (or attempted to capture) its LabelExportDataFromSourceExit
// snapshot via the normal complete/cutover code path. The atexit-registered
// error/interrupt handler checks this flag so it doesn't also fire a
// placeholder for a run that already exited cleanly.
//
// It is atomic because the atexit handler can run on the signal goroutine
// (main.go's signal handler calls atexit.Exit) concurrently with the main
// export goroutine still writing it — a plain bool would be a data race.
var exportDataExitSnapshotCaptured atomic.Bool

// schemaSnapshotExitCaptureTimeout bounds the best-effort schema capture attempted on
// an abnormal export-data exit (error/signal). The run's own context is already
// cancelled by then, so this fresh budget lets a healthy catalog read finish (a
// metadata-only read is well under this) while capping how long a wedged or
// unreachable source DB delays the exit. The pgx driver honors the deadline by setting
// a deadline on the underlying connection, so even a silently-hung socket is
// interrupted promptly — no dependence on reaching the dead host — which is why a plain
// context timeout is a sufficient bound here.
const schemaSnapshotExitCaptureTimeout = 10 * time.Second

// registerExportDataExitSnapshotHook registers an atexit handler that captures the
// source schema snapshot on an abnormal export-data exit (error or signal), unless
// the normal complete/cutover path already recorded one (exportDataExitSnapshotCaptured).
// The exit reason is derived from the shutdown flags. The run's own context is already
// cancelled by exit time, so the capture uses a fresh, bounded context. Source-exporter
// only; the caller gates on exporterRole.
func registerExportDataExitSnapshotHook() {
	atexit.Register(func() {
		if exportDataExitSnapshotCaptured.Load() {
			return // normal complete/cutover path already recorded the exit
		}
		// Abnormal exit (error or signal). Attempt a full capture so a drift-caused
		// failure records the drifted end-state schema; captureSourceSchemaSnapshot
		// falls back to a placeholder if the schema can't be read. The run's own ctx
		// is already cancelled by now, so use a fresh, bounded context.
		reason := schemasnapshot.ReasonError
		if ProcessShutdownRequested {
			// SIGINT/SIGTERM is a user interrupt; SIGUSR2 is the end-migration
			// command's controlled teardown, treated as a clean exit.
			reason = schemasnapshot.ReasonInterrupt
			if EndMigrationStopRequested {
				reason = schemasnapshot.ReasonComplete
			}
		}
		exitCtx, cancel := context.WithTimeout(context.Background(), schemaSnapshotExitCaptureTimeout)
		defer cancel()
		captureSourceSchemaSnapshot(exitCtx, schemasnapshot.LabelExportDataFromSourceExit, reason, true)
	})
}

// captureSourceSchemaSnapshot captures the source schema and persists it as a
// snapshot for the given label/reason. It is BEST-EFFORT and off the data path:
// it never returns an error and never blocks the migration — every failure is
// logged and swallowed.
// Honors --suppress-schema-snapshot-capture. PostgreSQL only (no-op with a log for
// other engines). Callers are responsible for exporter-role gating.
func captureSourceSchemaSnapshot(ctx context.Context, label, reason string, placeholderOnFailure bool) {
	if source.DBType != POSTGRESQL {
		log.Infof("schema-snapshot capture skipped for label %q: only PostgreSQL sources are supported", label)
		return
	}
	if bool(suppressSchemaSnapshotCapture) {
		log.Infof("schema-snapshot capture suppressed (--suppress-schema-snapshot-capture); skipping %s", label)
		return
	}
	pg, ok := source.DB().(*srcdb.PostgreSQL)
	if !ok {
		log.Warnf("schema-snapshot capture skipped for label %q: PostgreSQL source lacks a *srcdb.PostgreSQL handle", label)
		return
	}
	db := pg.GetDB()
	if db == nil {
		log.Warnf("schema-snapshot capture skipped for label %q: no active database handle", label)
		if placeholderOnFailure {
			// Still record the timeline marker so a lifecycle moment (e.g. an exit)
			// isn't lost just because the DB handle is gone during teardown.
			saveSourceSchemaSnapshotPlaceholder(label, reason)
		}
		return
	}
	captureParams := schemasnapshot.CaptureParams{
		DatabaseType: source.DBType,
		DBMetadata:   schemasnapshot.DBMetadata{Host: source.Host, Port: source.Port, Database: source.DBName, User: source.User},
		Schemas:      source.GetSchemaList(),
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
		log.Warnf("schema-snapshot capture for label %q failed (continuing, migration unaffected): %v", label, err)
		return
	}
	log.Infof("captured schema snapshot %q", name)
}

// startPeriodicSourceSchemaSnapshotCapture launches a background ticker that
// captures a source schema snapshot every --schema-snapshot-capture-interval
// minutes for the full duration of the export — both the snapshot and streaming
// phases. It returns a stop function to be invoked via defer.
// Best-effort and off the data path: a no-op when suppressed, when the interval is
// <= 0, or when this is not the source exporter; periodic capture failures are
// logged and never affect export.
func startPeriodicSourceSchemaSnapshotCapture(ctx context.Context) func() {
	if exporterRole != SOURCE_DB_EXPORTER_ROLE || source.DBType != POSTGRESQL || bool(suppressSchemaSnapshotCapture) {
		return func() {}
	}
	interval := time.Duration(schemaSnapshotCaptureInterval) * time.Minute
	if interval <= 0 {
		return func() {}
	}
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stop:
				return
			case <-ticker.C:
				captureSourceSchemaSnapshot(ctx, schemasnapshot.LabelExportDataFromSourcePeriodic, "", false)
			}
		}
	}()
	var once sync.Once
	return func() { once.Do(func() { close(stop) }) }
}

// saveSourceSchemaSnapshotPlaceholder records a metadata-only timeline marker
// (no schema read) for a lifecycle moment we can't fully capture, e.g. an
// error/interrupt exit. Best-effort; never fails the caller. Honors suppression.
func saveSourceSchemaSnapshotPlaceholder(label, reason string) {
	if source.DBType != POSTGRESQL {
		return
	}
	if bool(suppressSchemaSnapshotCapture) {
		return
	}
	h := schemasnapshot.SnapshotHeader{
		Label:         label,
		Reason:        reason,
		Side:          schemasnapshot.SideSource,
		CapturedAt:    time.Now().UTC(),
		Schemas:       source.GetSchemaList(),
		IsPlaceholder: true,
	}
	if _, err := schemasnapshot.SavePlaceholder(metaDB, h); err != nil {
		log.Warnf("schema-snapshot placeholder for label %q failed: %v", label, err)
	}
}
