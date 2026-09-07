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
	"time"

	goerrors "github.com/go-errors/errors"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
)

// schemaSnapshotCaptureEnabled reports whether capture is live at all: PostgreSQL
// source, not disabled. It also returns why it is not, so a caller that wants to say
// so logs the specific reason rather than a generic one. Role gating stays at the call
// sites that know the role.
func schemaSnapshotCaptureEnabled() (bool, string) {
	if source.DBType != POSTGRESQL {
		return false, "only PostgreSQL sources are supported"
	}
	if bool(disableSchemaSnapshotCapture) {
		return false, "disabled via --disable-schema-snapshot-capture"
	}
	return true, ""
}

// captureSourceSchemaSnapshot captures the source schema and persists it as a snapshot
// for the given label/reason, returning why it could not.
//
// A skip is not an error: a non-PostgreSQL source or --disable-schema-snapshot-capture
// returns nil, since nothing went wrong. Exporter-role gating is the caller's.
//
// The caller decides what a failure means. Today every caller is an export hook that
// logs and carries on, because schema capture is off the data path and must never fail
// or stall an export.
func captureSourceSchemaSnapshot(ctx context.Context, label, reason string, placeholderOnFailure bool) error {
	if enabled, why := schemaSnapshotCaptureEnabled(); !enabled {
		log.Infof("schema-snapshot capture skipped for label %q: %s", label, why)
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
// Best-effort: a no-op when disabled, when interval <= 0, or off the source exporter.
func startPeriodicSourceSchemaSnapshotCapture(ctx context.Context, interval time.Duration) {
	enabled, _ := schemaSnapshotCaptureEnabled()
	if exporterRole != SOURCE_DB_EXPORTER_ROLE || !enabled {
		return
	}
	if interval <= 0 {
		return
	}
	// Logged once per started ticker, so a second start is visible in the log (and
	// asserted by the live E2E): two tickers would double every periodic snapshot.
	log.Infof("starting periodic schema-snapshot capture every %s", interval)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := captureSourceSchemaSnapshot(ctx, schemasnapshot.LabelExportDataFromSourcePeriodic, "", false); err != nil {
					log.Warnf("periodic schema-snapshot capture failed, migration unaffected: %v", err)
				}
			}
		}
	}()
}

// saveSourceSchemaSnapshotPlaceholder records a metadata-only timeline marker for a
// moment we can't fully capture. Best-effort; honors the disable flag.
//
// It uses its OWN fresh, bounded context: the capture context may be exactly what
// died, and reusing it would drop the marker just when it is needed.
func saveSourceSchemaSnapshotPlaceholder(label, reason string) {
	if enabled, _ := schemaSnapshotCaptureEnabled(); !enabled {
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
