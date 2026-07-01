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

	goerrors "github.com/go-errors/errors"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
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
// multi-engine. Providers are constructed by newProvider, which selects the
// right implementation via a switch on the database type string. To add a new
// engine, add its implementation file to this package and add a case to newProvider.
type SnapshotProvider interface {
	// DatabaseType returns the lowercase database type string, e.g. "postgresql".
	DatabaseType() string

	// TakeSnapshot captures the schema for the given schemas and returns
	// a populated *SchemaSnapshot. The provider uses the QueryExecutor it was
	// constructed with (bound at newProvider call time). The header fields
	// (CapturedAt, DBMetadata, StableIdentity, etc.) are stamped by the Capture
	// orchestrator after this call returns, so the provider must not set them.
	//
	// Why: providers produce schema content only; the headers are capture-event
	// metadata computed identically for every engine. Stamping them once in the
	// orchestrator keeps them consistent (one Version/clock/source) and the
	// provider never even receives the DBMetadata, so it can't set them wrong.
	TakeSnapshot(ctx context.Context, schemas []string) (*SchemaSnapshot, error)

	// HasStableIdentity reports whether ID fields in the snapshot are reliable
	// enough for rename detection across snapshots (true for PostgreSQL).
	HasStableIdentity() bool
}

// ─── Provider constructor ──────────────────────────────────────────────────────

// newProvider returns the SnapshotProvider for databaseType, bound to db.
// Unsupported types return a clear error.
func newProvider(databaseType string, db QueryExecutor) (SnapshotProvider, error) {
	switch databaseType {
	case constants.POSTGRESQL:
		return &postgresSnapshotProvider{db: db}, nil
	default:
		return nil, goerrors.Errorf("schemasnapshot: unsupported database type %q", databaseType)
	}
}
