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
	"sync"
	"time"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
)

// QueryExecutor is the minimal read interface the loaders run against.
// It is satisfied by *sql.Tx (and by *sql.DB), allowing Capture to run
// TakeSnapshot inside a managed REPEATABLE READ transaction.
type QueryExecutor interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// SnapshotProvider is the per-database-type seam that keeps the library
// multi-engine. Each databases/<dbtype>/ sub-package registers a factory in
// its init() via RegisterProvider.
type SnapshotProvider interface {
	// DatabaseType returns the lowercase database type string, e.g. "postgresql".
	DatabaseType() string

	// TakeSnapshot captures the schema for the given schemas via db and returns
	// a populated *SchemaSnapshot. The header fields (CapturedAt, CaptureSource,
	// StableIdentity, etc.) are stamped by the Capture orchestrator after this
	// call returns, so the provider must not set them.
	TakeSnapshot(ctx context.Context, db QueryExecutor, schemas []string) (*SchemaSnapshot, error)

	// HasStableIdentity reports whether ID fields in the snapshot are reliable
	// enough for rename detection across snapshots (true for PostgreSQL).
	HasStableIdentity() bool
}

// ─── Provider registry ────────────────────────────────────────────────────────

// providerRegistry maps a DatabaseType string to a factory function.
var (
	providerRegistryMu sync.RWMutex
	providerRegistry   = map[string]func() SnapshotProvider{}
)

// RegisterProvider registers a provider factory for the given databaseType.
// Packages under databases/<dbtype>/ call this in their init() functions.
func RegisterProvider(databaseType string, factory func() SnapshotProvider) {
	providerRegistryMu.Lock()
	defer providerRegistryMu.Unlock()
	providerRegistry[databaseType] = factory
}

// NewSnapshotProvider looks up the factory registered for databaseType and
// returns a fresh provider. If no factory has been registered for the given
// type it returns a clear error.
func NewSnapshotProvider(databaseType string) (SnapshotProvider, error) {
	providerRegistryMu.RLock()
	factory, ok := providerRegistry[databaseType]
	providerRegistryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("schemasnapshot: no provider registered for database type %q (import the matching databases/<dbtype> package)", databaseType)
	}
	return factory(), nil
}

// ─── Capture orchestrator ─────────────────────────────────────────────────────

// Capture is the library entry point for taking a schema snapshot. It:
//  1. Resolves the SnapshotProvider for source.DatabaseType.
//  2. Opens a read-only REPEATABLE READ transaction on db.
//  3. Calls provider.TakeSnapshot inside that transaction.
//  4. Commits on success (rollback on any error) — atomic, never partial.
//  5. Stamps all header fields onto the returned snapshot.
func Capture(ctx context.Context, db *sql.DB, source CaptureSource, schemas []string) (*SchemaSnapshot, error) {
	provider, err := NewSnapshotProvider(source.DatabaseType)
	if err != nil {
		return nil, err
	}

	tx, err := db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelRepeatableRead,
		ReadOnly:  true,
	})
	if err != nil {
		return nil, fmt.Errorf("schemasnapshot: opening snapshot transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	snap, err := provider.TakeSnapshot(ctx, tx, schemas)
	if err != nil {
		return nil, fmt.Errorf("schemasnapshot: taking snapshot: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("schemasnapshot: committing snapshot transaction: %w", err)
	}

	// Stamp header fields.
	snap.Version = 1
	snap.CaptureSource = source
	snap.CapturedAt = time.Now().UTC()
	snap.DatabaseType = source.DatabaseType
	snap.StableIdentity = provider.HasStableIdentity()
	snap.Schemas = schemas

	return snap, nil
}

// CaptureAndSaveSnapshot captures the source schema and persists it. On capture failure,
// if placeholderOnFailure is true it writes a metadata-only placeholder marker (so the
// lifecycle moment still appears on the timeline) and returns the original capture error;
// if false it returns the capture error without writing anything.
func CaptureAndSaveSnapshot(ctx context.Context, db *sql.DB, mdb *metadb.MetaDB,
	source CaptureSource, schemas []string, label, reason string,
	placeholderOnFailure bool) (name string, err error) {

	snap, captureErr := Capture(ctx, db, source, schemas)
	if captureErr != nil {
		if placeholderOnFailure {
			// placeholder dbVersion is "" (the version probe was part of the failed capture).
			_, _ = SavePlaceholder(mdb, label, reason, source.Role, time.Now().UTC(), "", schemas)
		}
		return "", captureErr
	}

	return SaveSnapshot(mdb, snap, label, reason)
}
