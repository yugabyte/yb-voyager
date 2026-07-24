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

// The best-effort capture budget lives in one place — schemasnapshot.CaptureTimeout —
// and is applied both here (the capture wrap and the source-side placeholder) and in the
// schemasnapshot package (its fallback placeholder). See its doc for the rationale.

// captureExportDataExitSnapshot captures the export-data exit snapshot for the given
// reason and marks it captured, so no later exit-capture site (in particular the atexit
// hook) fires a second, degraded capture. Source-exporter only. The context comes from
// the caller: success/cutover paths pass the run context; the error and atexit paths
// pass a fresh one via captureExportDataExitSnapshotFresh (the run context is dead by
// then). Either way captureSourceSchemaSnapshot caps it at schemasnapshot.CaptureTimeout.
func captureExportDataExitSnapshot(ctx context.Context, reason string) {
	if exporterRole != SOURCE_DB_EXPORTER_ROLE {
		return
	}
	captureSourceSchemaSnapshot(ctx, schemasnapshot.LabelExportDataFromSourceExit, reason, true)
	exportDataExitSnapshotCaptured.Store(true)
}

// captureExportDataExitSnapshotFresh is captureExportDataExitSnapshot on a fresh
// context, for exit sites where the run's own context may already be cancelled: the
// error `return false` paths — which must capture inline, while the source connection is
// still open, because exportData's deferred Disconnect closes it before the atexit hook
// runs — and the atexit fallback itself. The capture's time bound is applied inside
// captureSourceSchemaSnapshot (schemasnapshot.CaptureTimeout), which caps how long a
// wedged or unreachable source DB can delay the exit.
func captureExportDataExitSnapshotFresh(reason string) {
	captureExportDataExitSnapshot(context.Background(), reason)
}

// registerExportDataExitSnapshotHook registers an atexit handler that captures the
// source schema snapshot on exit paths that don't capture inline — SIGINT/SIGTERM
// interrupts and utils.ErrExit exits (whose os.Exit bypasses exportData's deferred
// Disconnect, so the source connection is still open). The normal completion/cutover
// paths and the error `return false` paths capture inline and set the flag, so this
// hook no-ops for them. The exit reason is derived from the shutdown flags.
func registerExportDataExitSnapshotHook() {
	atexit.Register(func() {
		if exportDataExitSnapshotCaptured.Load() {
			return // an inline capture (completion, cutover, or error path) already recorded the exit
		}
		reason := schemasnapshot.ReasonError
		if ProcessShutdownRequested {
			// SIGINT/SIGTERM is a user interrupt; SIGUSR2 is the end-migration
			// command's controlled teardown, treated as a clean exit.
			reason = schemasnapshot.ReasonInterrupt
			if EndMigrationStopRequested {
				reason = schemasnapshot.ReasonComplete
			}
		}
		captureExportDataExitSnapshotFresh(reason)
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

	// Bound the capture (catalog read + real metaDB write) so a slow or wedged source DB
	// can never block the migration on this best-effort work. WithTimeout keeps the
	// earlier deadline, so a caller that passed a tighter context still wins. (The
	// db == nil placeholder path below deliberately uses its own fresh context, not this
	// one — see saveSourceSchemaSnapshotPlaceholder.)
	ctx, cancel := context.WithTimeout(ctx, schemasnapshot.CaptureTimeout)
	defer cancel()

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
// phases. The ticker goroutine runs until ctx is cancelled, so the caller stops
// it simply by cancelling the context it already owns (its defer cancel()); no
// separate stop function is needed.
// Best-effort and off the data path: a no-op when suppressed, when the interval is
// <= 0, or when this is not the source exporter; periodic capture failures are
// logged and never affect export.
func startPeriodicSourceSchemaSnapshotCapture(ctx context.Context) {
	if exporterRole != SOURCE_DB_EXPORTER_ROLE || source.DBType != POSTGRESQL || bool(suppressSchemaSnapshotCapture) {
		return
	}
	interval := time.Duration(schemaSnapshotCaptureInterval) * time.Minute
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
				captureSourceSchemaSnapshot(ctx, schemasnapshot.LabelExportDataFromSourcePeriodic, "", false)
			}
		}
	}()
}

// saveSourceSchemaSnapshotPlaceholder records a metadata-only timeline marker
// (no schema read) for a lifecycle moment we can't fully capture, e.g. an
// error/interrupt exit. Best-effort; never fails the caller. Honors suppression.
//
// It uses its OWN fresh, bounded context rather than the capture context: this is a
// fallback marker written when the real capture can't happen, and the capture context
// may be exactly what died (deadline exceeded / cancelled) — reusing it would drop the
// marker just when it is needed. The fresh budget still bounds the metaDB write so it
// can't hang on a contended lock.
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
		Schemas:       source.GetSchemaList(),
		IsPlaceholder: true,
	}
	if _, err := schemasnapshot.SavePlaceholder(ctx, metaDB, h); err != nil {
		log.Warnf("schema-snapshot placeholder for label %q failed: %v", label, err)
	}
}
