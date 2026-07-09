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
	"encoding/json"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
)

// exportDataExitSnapshotCaptured records whether the export-data run already
// captured (or attempted to capture) its LabelExportDataFromSourceExit
// snapshot via the normal complete/cutover code path. The atexit-registered
// error/interrupt handler checks this flag so it doesn't also fire a
// placeholder for a run that already exited cleanly.
var exportDataExitSnapshotCaptured bool

// schemaSnapshotExitCaptureTimeout bounds the best-effort schema capture attempted on
// an abnormal export-data exit (error/signal). The run's own context is already
// cancelled by then, so this fresh budget lets a healthy catalog read finish (seconds)
// while capping the delay if the source DB is wedged. Matches main.go's post-shutdown
// cleanup window.
const schemaSnapshotExitCaptureTimeout = 2 * time.Minute

// captureSourceSchemaSnapshot captures the source schema and persists it as a
// snapshot for the given label/reason. It is BEST-EFFORT and off the data path:
// it never returns an error and never blocks the migration — every failure is
// logged and swallowed (FS: capture must never interrupt export). Honors
// --suppress-schema-snapshot-capture. PostgreSQL only (no-op with a log for
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

	if label != schemasnapshot.LabelExportDataFromSourcePeriodic {
		// Lifecycle markers (start/exit/export-schema) are always saved, never deduped.
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
		return
	}

	// Periodic capture: dedup against the last stored (non-placeholder) snapshot so an
	// unchanged schema doesn't grow the timeline with identical rows. placeholderOnFailure
	// is always false for the periodic label, so a capture failure needs no placeholder.
	snap, err := schemasnapshot.Capture(ctx, db, captureParams)
	if err != nil {
		log.Warnf("schema-snapshot capture for label %q failed (continuing, migration unaffected): %v", label, err)
		return
	}

	prev, err := latestStoredSnapshotContent()
	if err != nil {
		log.Debugf("schema-snapshot dedup check for label %q failed (continuing, treating as changed): %v", label, err)
	} else if snapshotContentEqual(prev, snap.Content) {
		log.Infof("source schema unchanged since last snapshot; skipping periodic capture")
		return
	}

	name, err := schemasnapshot.SaveSnapshot(metaDB, snap)
	if err != nil {
		log.Warnf("schema-snapshot capture for label %q failed (continuing, migration unaffected): %v", label, err)
		return
	}
	log.Infof("captured schema snapshot %q", name)
}

// snapshotContentEqual reports whether a and b marshal to byte-identical JSON. It is the
// dedup comparison used by periodic capture. Nil needs no explicit guard: json.Marshal
// renders a nil *SnapshotContent as "null", so nil == nil compares equal and nil vs
// non-nil does not. A marshal error on either side is treated as "changed" (never blocks
// capture). The first periodic capture has no prior stored snapshot, so a is nil then and
// this correctly reports "changed".
func snapshotContentEqual(a, b *schemasnapshot.SnapshotContent) bool {
	aBytes, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bBytes, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(aBytes) == string(bBytes)
}

// latestStoredSnapshotContent returns the content of the most recently persisted
// non-placeholder snapshot, or (nil, nil) if none exists. ListSnapshots returns headers
// ordered oldest-first, so this scans from the end to find the most recent real capture.
func latestStoredSnapshotContent() (*schemasnapshot.SnapshotContent, error) {
	headers, err := schemasnapshot.ListSnapshots(metaDB)
	if err != nil {
		return nil, err
	}
	for i := len(headers) - 1; i >= 0; i-- {
		if headers[i].IsPlaceholder {
			continue
		}
		return schemasnapshot.LoadSnapshotByName(metaDB, headers[i].Name())
	}
	return nil, nil
}

// startPeriodicSourceSchemaSnapshotCapture launches a background ticker that
// captures a source schema snapshot every --schema-snapshot-capture-interval
// minutes for the full duration of the export — both the snapshot and streaming
// phases — so schema drift is tracked throughout the migration, not just during
// the initial snapshot. It returns a stop function to be invoked via defer.
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
