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

	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
)

// captureSourceSchemaSnapshot captures the source schema and persists it as a
// snapshot for the given label/reason. It is BEST-EFFORT and off the data path:
// it never returns an error and never blocks the migration — every failure is
// logged and swallowed (FS: capture must never interrupt export). Honors
// --suppress-schema-snapshot-capture. PostgreSQL only (no-op with a log for
// other engines). Callers are responsible for exporter-role gating.
func captureSourceSchemaSnapshot(ctx context.Context, label, reason string, placeholderOnFailure bool) {
	if bool(suppressSchemaSnapshotCapture) {
		log.Infof("schema-snapshot capture suppressed (--suppress-schema-snapshot-capture); skipping %s", label)
		return
	}
	pg, ok := source.DB().(*srcdb.PostgreSQL)
	if !ok {
		log.Infof("schema-snapshot capture skipped for label %q: only PostgreSQL sources are supported", label)
		return
	}
	db := pg.GetDB()
	if db == nil {
		log.Warnf("schema-snapshot capture skipped for label %q: no active database handle", label)
		return
	}
	req := schemasnapshot.CaptureRequest{
		CaptureParams: schemasnapshot.CaptureParams{
			DatabaseType: source.DBType,
			DBMetadata:   schemasnapshot.DBMetadata{Host: source.Host, Port: source.Port, Database: source.DBName, User: source.User},
			Schemas:      source.GetSchemaList(),
			Label:        label,
			Reason:       reason,
		},
		PlaceholderOnFailure: placeholderOnFailure,
	}
	name, err := schemasnapshot.CaptureAndSaveSnapshot(ctx, db, metaDB, req)
	if err != nil {
		log.Warnf("schema-snapshot capture for label %q failed (continuing, migration unaffected): %v", label, err)
		return
	}
	log.Infof("captured schema snapshot %q", name)
}

// saveSourceSchemaSnapshotPlaceholder records a metadata-only timeline marker
// (no schema read) for a lifecycle moment we can't fully capture, e.g. an
// error/interrupt exit. Best-effort; never fails the caller. Honors suppression.
func saveSourceSchemaSnapshotPlaceholder(label, reason string) {
	if bool(suppressSchemaSnapshotCapture) {
		return
	}
	if source.DBType != POSTGRESQL {
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
