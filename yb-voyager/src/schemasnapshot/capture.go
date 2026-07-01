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
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
)

// ─── Capture orchestrator ─────────────────────────────────────────────────────

// CaptureParams describes a single capture: what to capture (Source, Schemas) and
// how to file it (Label, Reason). Label is required at every call site — it is the
// snapshot's filing key (a labels.go constant) and the basis of its persisted name.
type CaptureParams struct {
	Source  DBMetadata
	Schemas []string
	Label   string
	Reason  string
}

// Capture is the library entry point for taking a schema snapshot. It:
//  1. Guards against empty Schemas.
//  2. Validates Label/Reason against the known vocabulary.
//  3. Resolves the SnapshotProvider for p.Source.DatabaseType.
//  4. Opens a read-only REPEATABLE READ transaction on db.
//  5. Calls provider.TakeSnapshot inside that transaction.
//  6. Commits on success (rollback on any error) — atomic, never partial.
//  7. Stamps all header fields (including Series and Reason) onto the returned snapshot.
//
// The snapshot returned is fully populated and ready for SaveSnapshot without any
// further mutation.
func Capture(ctx context.Context, db *sql.DB, p CaptureParams) (*SchemaSnapshot, error) {
	if len(p.Schemas) == 0 {
		return nil, goerrors.Errorf("schemasnapshot: no schemas in scope for capture")
	}

	if err := ValidateLabelReason(p.Label, p.Reason); err != nil {
		return nil, err
	}

	provider, err := NewSnapshotProvider(p.Source.DatabaseType)
	if err != nil {
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

	snap, err := provider.TakeSnapshot(ctx, tx, p.Schemas)
	if err != nil {
		return nil, fmt.Errorf("schemasnapshot: taking snapshot: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("schemasnapshot: committing snapshot transaction: %w", err)
	}

	// Stamp header fields.
	snap.Version = 1
	snap.DBMetadata = p.Source
	snap.CapturedAt = time.Now().UTC()
	snap.DatabaseType = p.Source.DatabaseType
	snap.StableIdentity = provider.HasStableIdentity()
	snap.Schemas = p.Schemas
	snap.Series = p.Label
	snap.Reason = p.Reason

	return snap, nil
}

// CaptureRequest describes a single capture-and-persist operation. It bundles the
// fields that describe *what* to capture and *how* to record it, keeping the
// infrastructure handles (ctx, db, mdb) as direct parameters of CaptureAndSaveSnapshot.
// Zero values are sane defaults (no placeholder, empty reason).
type CaptureRequest struct {
	CaptureParams
	PlaceholderOnFailure bool // when true, a failed capture still records a metadata-only timeline marker.
}

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
			_, _ = SavePlaceholder(mdb, req.Label, req.Reason, req.Source.Side, time.Now().UTC(), "", req.Schemas)
		}
		return "", captureErr
	}

	return SaveSnapshot(mdb, snap)
}
