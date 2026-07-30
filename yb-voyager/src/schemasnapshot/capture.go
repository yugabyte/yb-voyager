// Copyright (c) YugabyteDB, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package schemasnapshot

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	goerrors "github.com/go-errors/errors"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
)

// CaptureParams describes a single capture: what to capture and how to file it.
// The old single Source DBMetadata is split into DatabaseType + Side + DBMetadata coords.
// Label is required at every call site — it is the snapshot's filing key (a labels.go
// constant) and the basis of its persisted name.
type CaptureParams struct {
	DatabaseType string     // selects the provider; recorded in SnapshotContent.DatabaseType
	Side         string     // migration side; recorded in the header (defaults to SideSource if "")
	DBMetadata   DBMetadata // display coordinates; recorded in SnapshotContent.DBMetadata
	// Schemas are RAW, UNQUOTED identifier values, matched as data against catalog
	// values (pg_namespace.nspname). Passing the SQL-quoted form breaks any schema
	// that needs quoting: `"Odd Schema"` never equals nspname `Odd Schema`, so the
	// capture silently records zero tables. Callers holding sqlname identifiers
	// should use srcdb.Source.GetSchemaListUnquoted, not GetSchemaList.
	Schemas []string
	Label   string
	Reason  string
}

// Capture is the library entry point for taking a schema snapshot. It:
//  1. Guards against empty Schemas.
//  2. Validates Label/Reason against the known vocabulary.
//  3. Opens a read-only REPEATABLE READ transaction on db.
//  4. Resolves the SnapshotProvider for p.DatabaseType, bound to the tx.
//  5. Calls provider.TakeSnapshot inside that transaction.
//  6. Commits on success (rollback on any error) — atomic, never partial.
//  7. Stamps all header fields onto the returned SchemaSnapshot.
//
// The SchemaSnapshot returned is fully populated and ready for SaveSnapshot without any
// further mutation.
func Capture(ctx context.Context, db *sql.DB, p CaptureParams) (*SchemaSnapshot, error) {
	if len(p.Schemas) == 0 {
		return nil, goerrors.Errorf("schemasnapshot: no schemas in scope for capture")
	}

	if err := ValidateLabelReason(p.Label, p.Reason); err != nil {
		return nil, err
	}

	// REPEATABLE READ so every loader query (tables, columns, links) sees one
	// consistent point-in-time catalog — concurrent DDL mid-capture can't make
	// the multi-query snapshot internally inconsistent.
	tx, err := db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelRepeatableRead,
		ReadOnly:  true,
	})
	if err != nil {
		return nil, fmt.Errorf("schemasnapshot: opening snapshot transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	provider, err := newProvider(p.DatabaseType, tx)
	if err != nil {
		return nil, err
	}

	schema, dbVersion, err := provider.TakeSnapshot(ctx, p.Schemas)
	if err != nil {
		return nil, fmt.Errorf("schemasnapshot: taking snapshot: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("schemasnapshot: committing snapshot transaction: %w", err)
	}

	// Stamp schema-level fields (content).
	schema.Version = 1
	schema.DatabaseType = p.DatabaseType
	schema.DBMetadata = p.DBMetadata

	// Build the header (metadata).
	header := newHeader(p, time.Now().UTC(), dbVersion, false)

	return &SchemaSnapshot{Header: header, Content: schema}, nil
}

// CaptureRequest describes a single capture-and-persist operation. It bundles the
// fields that describe *what* to capture and *how* to record it, keeping the
// infrastructure handles (ctx, db, mdb) as direct parameters of CaptureAndSaveSnapshot.
// Zero values are sane defaults (no placeholder, empty reason).
type CaptureRequest struct {
	CaptureParams
	PlaceholderOnFailure bool // when true, a failed capture still records a metadata-only timeline marker.
}

// CaptureTimeout is the single best-effort budget for ALL schema-snapshot DB work: the
// catalog read plus the metaDB write of a real snapshot, and equally the fallback
// placeholder write. Schema capture is best-effort and off the data path, so this caps
// how long a slow or wedged source DB — or a contended metaDB write lock — can delay the
// migration. The work is small (a metadata-only read plus one insert; the placeholder is
// metadata-only and does NOT scale with schema size), so it is generous headroom, not a
// tight SLA. Both cmd (the capture wrap and the source-side placeholder) and this package
// (the fallback placeholder in CaptureAndSaveSnapshot) use this one value.
const CaptureTimeout = 10 * time.Second

// CaptureAndSaveSnapshot captures the source schema and persists it. On capture failure,
// if req.PlaceholderOnFailure is true it writes a metadata-only placeholder marker (so the
// lifecycle moment still appears on the timeline) and returns the original capture error;
// if false it returns the capture error without writing anything.
func CaptureAndSaveSnapshot(ctx context.Context, db *sql.DB, mdb *metadb.MetaDB,
	req CaptureRequest) (name string, err error) {

	snap, captureErr := Capture(ctx, db, req.CaptureParams)
	if captureErr != nil {
		if req.PlaceholderOnFailure {
			// placeholder dbVersion is "" (the version probe was part of the failed capture).
			h := newHeader(req.CaptureParams, time.Now().UTC(), "", true)
			// Fresh context: the capture context may be the very thing that failed
			// (deadline exceeded / cancelled), so the marker must not reuse it.
			phCtx, cancel := context.WithTimeout(context.Background(), CaptureTimeout)
			defer cancel()
			// Best-effort timeline marker: we still return the original capture error,
			// but a failed placeholder insert must not vanish silently (BUGBOT.md).
			if _, perr := SavePlaceholder(phCtx, mdb, h); perr != nil {
				log.Warnf("schemasnapshot: failed to write placeholder marker for label %q: %v", req.Label, perr)
			}
		}
		return "", captureErr
	}

	return SaveSnapshot(ctx, mdb, snap)
}

// newHeader builds a SnapshotHeader from capture params + the capture-time facts.
// Applies the SideSource default when Side is empty.
// Used by both the success path (Capture) and the failure/placeholder path
// (CaptureAndSaveSnapshot), so header construction lives in one place.
func newHeader(p CaptureParams, capturedAt time.Time, dbVersion string, isPlaceholder bool) SnapshotHeader {
	side := p.Side
	if side == "" {
		side = SideSource
	}
	return SnapshotHeader{
		Label:           p.Label,
		Reason:          p.Reason,
		Side:            side,
		CapturedAt:      capturedAt,
		DatabaseVersion: dbVersion,
		Schemas:         p.Schemas,
		IsPlaceholder:   isPlaceholder,
	}
}
