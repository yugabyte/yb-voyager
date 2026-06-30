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

// Package postgres provides the schemasnapshot SnapshotProvider for PostgreSQL.
// It self-registers in init() — import it (or import databases/all) to activate
// PostgreSQL support. v1 loads tables and columns only; no Attr types registered.
package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
)

// buildInPlaceholders returns a comma-separated list of n PostgreSQL positional
// placeholders, e.g. n=3 → "$1,$2,$3", and a corresponding []interface{} of the
// schema names. PostgreSQL (pgx / lib/pq) uses $N placeholders, not "?".
func buildInPlaceholders(schemas []string) (string, []interface{}) {
	placeholders := make([]string, len(schemas))
	args := make([]interface{}, len(schemas))
	for i, s := range schemas {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = s
	}
	return strings.Join(placeholders, ","), args
}

// PostgresSnapshotProvider implements schemasnapshot.SnapshotProvider for PostgreSQL.
type PostgresSnapshotProvider struct{}

// DatabaseType returns the canonical database type string for PostgreSQL.
func (p *PostgresSnapshotProvider) DatabaseType() string { return constants.POSTGRESQL }

// HasStableIdentity reports that PostgreSQL OIDs are stable enough for rename
// detection across consecutive snapshots.
func (p *PostgresSnapshotProvider) HasStableIdentity() bool { return true }

// TakeSnapshot captures the PostgreSQL schema for the given schemas via db.
// It loads tables (including hierarchy links) and columns (v1 scope).
// The header fields (CapturedAt, CaptureSource, etc.) are stamped by the
// Capture orchestrator in provider.go after this call returns.
// Query order: SHOW server_version → pg_class → pg_inherits → pg_attribute.
func (p *PostgresSnapshotProvider) TakeSnapshot(
	ctx context.Context,
	db schemasnapshot.QueryExecutor,
	schemas []string,
) (*schemasnapshot.SchemaSnapshot, error) {
	snap := &schemasnapshot.SchemaSnapshot{}

	placeholders, args := buildInPlaceholders(schemas)

	// Probe database version.
	dbVersion, err := detectDatabaseVersion(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("postgres: detecting database version: %w", err)
	}
	snap.DatabaseVersion = dbVersion

	// Load tables (includes partition and inheritance wiring via pg_inherits).
	tables, err := loadTables(ctx, db, placeholders, args)
	if err != nil {
		return nil, fmt.Errorf("postgres: loading tables: %w", err)
	}
	snap.Tables = tables

	// Load columns.
	columns, err := loadColumns(ctx, db, placeholders, args)
	if err != nil {
		return nil, fmt.Errorf("postgres: loading columns: %w", err)
	}
	snap.Columns = columns

	return snap, nil
}

// detectDatabaseVersion probes the PostgreSQL server_version and returns just
// the version number (truncated at the first space).
func detectDatabaseVersion(ctx context.Context, db schemasnapshot.QueryExecutor) (string, error) {
	row := db.QueryRowContext(ctx, sqlServerVersion)
	var version string
	if err := row.Scan(&version); err != nil {
		return "", fmt.Errorf("SHOW server_version: %w", err)
	}
	// Truncate at first space: "16.4 (Ubuntu ...)" → "16.4"
	if idx := strings.Index(version, " "); idx >= 0 {
		version = version[:idx]
	}
	return version, nil
}

// tableLink holds a single row returned by sqlLoadTableLinksFmt.
type tableLink struct {
	childOID     string
	childSchema  string
	childName    string
	parentOID    string
	parentSchema string
	parentName   string
	isPartition  bool
}

// loadTableLinks queries pg_inherits for BOTH declarative-partition and
// legacy-inheritance parent/child relationships within the given schemas and
// returns them in parent/child OID order.
func loadTableLinks(ctx context.Context, db schemasnapshot.QueryExecutor, placeholders string, args []interface{}) ([]tableLink, error) {
	query := fmt.Sprintf(sqlLoadTableLinksFmt, placeholders)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query pg_inherits: %w", err)
	}
	defer rows.Close()

	var links []tableLink
	for rows.Next() {
		var lnk tableLink
		if err := rows.Scan(
			&lnk.childOID, &lnk.childSchema, &lnk.childName,
			&lnk.parentOID, &lnk.parentSchema, &lnk.parentName,
			&lnk.isPartition,
		); err != nil {
			return nil, fmt.Errorf("scan table link row: %w", err)
		}
		links = append(links, lnk)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate table link rows: %w", err)
	}
	return links, nil
}

// linkTableHierarchy wires partition and inheritance parent/child refs onto
// tables from pg_inherits rows, routing each row by isPartition. Pure (no DB).
func linkTableHierarchy(tables []schemasnapshot.Table, links []tableLink) {
	// Build an OID→index map for O(1) lookup.
	oidToIdx := make(map[string]int, len(tables))
	for i, tb := range tables {
		oidToIdx[tb.ID] = i
	}

	for _, lnk := range links {
		childIdx, childInScope := oidToIdx[lnk.childOID]
		parentIdx, parentInScope := oidToIdx[lnk.parentOID]

		if lnk.isPartition {
			// Declarative partitioning: child is always in scope (filter by child schema).
			if childInScope {
				ref := schemasnapshot.ObjectRef{Schema: lnk.parentSchema, Name: lnk.parentName}
				tables[childIdx].PartitionParent = &ref
			}
			// Only wire PartitionChildren if the parent is also in scope.
			if parentInScope {
				tables[parentIdx].PartitionChildren = append(
					tables[parentIdx].PartitionChildren,
					schemasnapshot.ObjectRef{Schema: lnk.childSchema, Name: lnk.childName},
				)
			}
		} else {
			// Legacy INHERITS: a child may inherit from multiple parents.
			if childInScope {
				tables[childIdx].InheritsFrom = append(
					tables[childIdx].InheritsFrom,
					schemasnapshot.ObjectRef{Schema: lnk.parentSchema, Name: lnk.parentName},
				)
			}
			if parentInScope {
				tables[parentIdx].InheritedBy = append(
					tables[parentIdx].InheritedBy,
					schemasnapshot.ObjectRef{Schema: lnk.childSchema, Name: lnk.childName},
				)
			}
		}
	}
}

// loadTables queries pg_class for tables (ordinary, partitioned, foreign) in the
// given schemas, then queries pg_inherits to wire both declarative-partition and
// legacy-inheritance parent/child links onto the returned tables.
func loadTables(ctx context.Context, db schemasnapshot.QueryExecutor, placeholders string, args []interface{}) ([]schemasnapshot.Table, error) {
	query := fmt.Sprintf(sqlLoadTablesFmt, placeholders)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query pg_class: %w", err)
	}
	defer rows.Close()

	var tables []schemasnapshot.Table
	for rows.Next() {
		var oid, schema, name, relkind string
		if err := rows.Scan(&oid, &schema, &name, &relkind); err != nil {
			return nil, fmt.Errorf("scan table row: %w", err)
		}
		kind := relkindToTableKind(relkind)
		tables = append(tables, schemasnapshot.Table{
			ObjectRef: schemasnapshot.ObjectRef{Schema: schema, Name: name},
			ID:        oid,
			Kind:      kind,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate table rows: %w", err)
	}

	// Load and wire partition + inheritance links cohesively within table loading.
	links, err := loadTableLinks(ctx, db, placeholders, args)
	if err != nil {
		return nil, fmt.Errorf("loading table links: %w", err)
	}
	linkTableHierarchy(tables, links)

	return tables, nil
}

// loadColumns queries pg_attribute for non-dropped columns of tables in the
// given schemas and returns them as a slice of schemasnapshot.Column.
func loadColumns(ctx context.Context, db schemasnapshot.QueryExecutor, placeholders string, args []interface{}) ([]schemasnapshot.Column, error) {
	query := fmt.Sprintf(sqlLoadColumnsFmt, placeholders)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query pg_attribute: %w", err)
	}
	defer rows.Close()

	var columns []schemasnapshot.Column
	for rows.Next() {
		var tableOID, attnum, schema, tableName, colName, dataType, colDefault string
		var notNull bool
		if err := rows.Scan(&tableOID, &attnum, &schema, &tableName, &colName, &dataType, &notNull, &colDefault); err != nil {
			return nil, fmt.Errorf("scan column row: %w", err)
		}
		col := schemasnapshot.Column{
			Table:    schemasnapshot.ObjectRef{Schema: schema, Name: tableName},
			ID:       tableOID + ":" + attnum,
			Name:     colName,
			DataType: dataType,
			NotNull:  notNull,
			Default:  colDefault,
		}
		columns = append(columns, col)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate column rows: %w", err)
	}
	return columns, nil
}

// relkindToTableKind converts a pg_class.relkind character to a TableKind.
func relkindToTableKind(relkind string) schemasnapshot.TableKind {
	switch relkind {
	case "r":
		return schemasnapshot.TableKindOrdinary
	case "p":
		return schemasnapshot.TableKindPartitioned
	case "f":
		return schemasnapshot.TableKindForeign
	default:
		return schemasnapshot.TableKindOrdinary
	}
}

// init self-registers the PostgreSQL provider.
// This runs when the package is imported (directly or via databases/all).
// Core schemasnapshot never imports this package — the registration is one-way.
func init() {
	schemasnapshot.RegisterProvider(constants.POSTGRESQL, func() schemasnapshot.SnapshotProvider {
		return &PostgresSnapshotProvider{}
	})
}
