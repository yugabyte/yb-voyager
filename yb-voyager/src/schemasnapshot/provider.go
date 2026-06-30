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
	"sync"

	goerrors "github.com/go-errors/errors"
)

// QueryExecutor is the minimal read interface the loaders run against.
// It is satisfied by *sql.Tx (and by *sql.DB), allowing Capture to run
// TakeSnapshot inside a managed REPEATABLE READ transaction.
//
// Why an interface and not *sql.Tx directly:
//   - Least privilege: providers get read-only access; they cannot Commit,
//     Rollback, Exec, or Close. Capture stays the sole owner of the tx lifecycle.
//   - Trivially fakeable in tests (sqlmock/stub) — no real DB or tx needed.
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
	//
	// Why: providers produce schema content only; the headers are capture-event
	// metadata computed identically for every engine. Stamping them once in the
	// orchestrator keeps them consistent (one Version/clock/source) and the
	// provider never even receives the CaptureSource, so it can't set them wrong.
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
		return nil, goerrors.Errorf("schemasnapshot: no provider registered for database type %q (import the matching databases/<dbtype> package)", databaseType)
	}
	return factory(), nil
}
