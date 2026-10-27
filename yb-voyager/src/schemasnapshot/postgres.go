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

// PostgreSQL SnapshotProvider for the schemasnapshot package.
// Loads tables (including partition and inheritance wiring), columns, constraints and indexes.
// Activate PostgreSQL support by importing this package (schemasnapshot); no
// separate sub-package import is required. Add more engines by adding a file
// and a switch case in newProvider (provider.go).

package schemasnapshot

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	goerrors "github.com/go-errors/errors"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
)

// ─── SQL catalog constants ─────────────────────────────────────────────────────

// sqlClearSearchPath sets search_path to empty for the current transaction only.
const sqlClearSearchPath = `SELECT set_config('search_path', '', true)`

// sqlServerVersion probes the PostgreSQL server version string.
const sqlServerVersion = `SHOW server_version`

// sqlLoadTablesFmt is the format template for loading tables. It expects %s to be
// replaced with a comma-separated list of $N placeholders (one per schema).
// Columns returned: oid, schema, name, relkind
const sqlLoadTablesFmt = `
SELECT c.oid::text       AS oid,
       n.nspname         AS schema,
       c.relname         AS name,
       c.relkind         AS relkind
FROM   pg_class     c
JOIN   pg_namespace n ON n.oid = c.relnamespace
WHERE  c.relkind IN ('r','p','f')
  AND  n.nspname IN (%s)
ORDER  BY n.nspname, c.relname
`

// sqlLoadTableLinksFmt is the format template for loading BOTH declarative-partition
// and legacy-inheritance parent/child relationships from pg_inherits. It expects %s to
// be replaced with a comma-separated list of $N placeholders (one per schema). Rows are
// distinguished by the is_partition flag (c.relispartition): true for declarative
// partitions, false for legacy INHERITS hierarchies.
// Columns returned: child_oid, child_schema, child_name, parent_oid, parent_schema, parent_name, is_partition
const sqlLoadTableLinksFmt = `
SELECT i.inhrelid::text AS child_oid, cn.nspname AS child_schema, c.relname AS child_name,
       i.inhparent::text AS parent_oid, pn.nspname AS parent_schema, p.relname AS parent_name,
       c.relispartition AS is_partition
FROM   pg_inherits i
JOIN   pg_class     c  ON c.oid = i.inhrelid
JOIN   pg_namespace cn ON cn.oid = c.relnamespace
JOIN   pg_class     p  ON p.oid = i.inhparent
JOIN   pg_namespace pn ON pn.oid = p.relnamespace
WHERE  cn.nspname IN (%s)
ORDER  BY i.inhparent, i.inhrelid
`

// sqlLoadColumnsFmt is the format template for loading columns. It expects %s to be
// replaced with a comma-separated list of $N placeholders (one per schema).
// Columns returned: table_oid, attnum, schema, table_name, col_name, data_type, not_null, col_default
const sqlLoadColumnsFmt = `
SELECT a.attrelid::text                                   AS table_oid,
       a.attnum::text                                     AS attnum,
       n.nspname                                          AS schema,
       c.relname                                          AS table_name,
       a.attname                                          AS col_name,
       format_type(a.atttypid, a.atttypmod)               AS data_type,
       a.attnotnull                                       AS not_null,
       COALESCE(pg_get_expr(d.adbin, d.adrelid), '')      AS col_default
FROM   pg_attribute  a
JOIN   pg_class      c ON c.oid = a.attrelid
JOIN   pg_namespace  n ON n.oid = c.relnamespace
LEFT   JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
WHERE  a.attnum > 0
  AND  NOT a.attisdropped
  AND  c.relkind IN ('r','p','f')
  AND  n.nspname IN (%s)
ORDER  BY a.attrelid, a.attnum
`

// sqlLoadConstraintsFmt is the format template for loading primary key, unique, check
// and foreign key constraints. The first %s is replaced with a comma-separated list of
// $N placeholders (one per schema), the second with sqlSkipReferencedPartitionFKs or "".
// Columns returned: table_oid, name, oid, contype, definition, from_parent
const sqlLoadConstraintsFmt = `
SELECT con.conrelid::text                     AS table_oid,
       con.conname                            AS name,
       con.oid::text                          AS oid,
       con.contype                            AS contype,
       pg_get_constraintdef(con.oid, true)    AS definition,
       -- conislocal is false for a partition's or inheritance child's copy, and true for a
       -- check declared both locally and inherited, which is the table's own.
       NOT con.conislocal                     AS from_parent
FROM   pg_constraint con
JOIN   pg_class      c ON c.oid = con.conrelid
JOIN   pg_namespace  n ON n.oid = c.relnamespace
WHERE  con.contype IN ('p','u','c','f')
  AND  c.relkind IN ('r','p','f')
  AND  n.nspname IN (%s)
  %s
ORDER  BY con.conrelid, con.conname, con.oid
`

// sqlSkipReferencedPartitionFKs drops the per-partition clones of a foreign key to a partitioned
// table: unlike a partition's own copy, a clone's parent is on the same table. Needs PostgreSQL 11+.
const sqlSkipReferencedPartitionFKs = `AND  NOT EXISTS (SELECT 1 FROM pg_constraint pc
                   WHERE  pc.oid = con.conparentid
                     AND  pc.conrelid = con.conrelid)`

// sqlLoadIndexesFmt is the format template for loading indexes that back no constraint.
// It expects %s to be replaced with a comma-separated list of $N placeholders (one per
// schema).
// Columns returned: table_oid, name, oid, is_unique, quoted_name, index_def, table_regclass, from_parent
const sqlLoadIndexesFmt = `
SELECT i.indrelid::text         AS table_oid,
       ic.relname               AS name,
       i.indexrelid::text       AS oid,
       i.indisunique            AS is_unique,
       quote_ident(ic.relname)  AS quoted_name,
       pg_get_indexdef(i.indexrelid) AS index_def,
       i.indrelid::regclass::text    AS table_regclass,
       ic.relispartition        AS from_parent
FROM   pg_index     i
JOIN   pg_class     ic ON ic.oid = i.indexrelid
JOIN   pg_class     c  ON c.oid = i.indrelid
JOIN   pg_namespace n  ON n.oid = c.relnamespace
-- An invalid index (a failed CREATE INDEX CONCURRENTLY) enforces nothing.
WHERE  i.indisvalid
  -- The constraint reports its own index.
  AND  NOT EXISTS (SELECT 1 FROM pg_constraint k
                   WHERE  k.conindid = i.indexrelid
                     AND  k.contype IN ('p','u','x'))
  AND  c.relkind IN ('r','p','f')
  AND  n.nspname IN (%s)
ORDER  BY i.indrelid, ic.relname, i.indexrelid
`

// ─── PostgreSQL provider ───────────────────────────────────────────────────────

// postgresSnapshotProvider implements SnapshotProvider for PostgreSQL.
// It is unexported; callers obtain it via newProvider (provider.go).
type postgresSnapshotProvider struct {
	db QueryExecutor
}

// DatabaseType returns the canonical database type string for PostgreSQL.
func (p *postgresSnapshotProvider) DatabaseType() string { return constants.POSTGRESQL }

// TakeSnapshot captures the PostgreSQL schema for the given schemas.
// It loads tables (including hierarchy links), columns, constraints and indexes.
// Returns the schema content, the probed database version string (e.g. "16.4"), and any error.
// The header fields (Version, DatabaseType, DBMetadata) are stamped by the
// Capture orchestrator in capture.go after this call returns.
// Query order: set_config(search_path) → SHOW server_version → pg_class → pg_inherits → pg_attribute →
// pg_constraint → pg_index.
func (p *postgresSnapshotProvider) TakeSnapshot(
	ctx context.Context,
	schemas []string,
) (*SnapshotContent, string, error) {
	snap := &SnapshotContent{}

	placeholders, args := buildInPlaceholders(schemas)

	// Empty search_path makes format_type, pg_get_expr, pg_get_constraintdef, pg_get_indexdef
	// and regclass text always schema-qualified, so captured text never depends on the session.
	// is_local=true scopes it to the transaction Capture runs this in.
	var ignored string
	if err := p.db.QueryRowContext(ctx, sqlClearSearchPath).Scan(&ignored); err != nil {
		return nil, "", fmt.Errorf("postgres: clearing search_path: %w", err)
	}

	dbVersion, err := detectDatabaseVersion(ctx, p.db)
	if err != nil {
		return nil, "", fmt.Errorf("postgres: detecting database version: %w", err)
	}
	majorVersion, err := parseMajorVersion(dbVersion)
	if err != nil {
		return nil, "", fmt.Errorf("postgres: parsing database version: %w", err)
	}

	// Load tables (includes partition and inheritance wiring via pg_inherits).
	tables, err := loadTables(ctx, p.db, placeholders, args)
	if err != nil {
		return nil, "", fmt.Errorf("postgres: loading tables: %w", err)
	}
	snap.Tables = tables

	// Load columns and nest each under its parent table.
	columns, err := loadColumns(ctx, p.db, placeholders, args)
	if err != nil {
		return nil, "", fmt.Errorf("postgres: loading columns: %w", err)
	}
	attachColumns(snap.Tables, columns)

	constraints, err := loadConstraints(ctx, p.db, placeholders, args, majorVersion >= 11)
	if err != nil {
		return nil, "", fmt.Errorf("postgres: loading constraints: %w", err)
	}
	if err := attachConstraints(snap.Tables, constraints); err != nil {
		return nil, "", fmt.Errorf("postgres: attaching constraints: %w", err)
	}

	indexes, err := loadIndexes(ctx, p.db, placeholders, args)
	if err != nil {
		return nil, "", fmt.Errorf("postgres: loading indexes: %w", err)
	}
	if err := attachIndexes(snap.Tables, indexes); err != nil {
		return nil, "", fmt.Errorf("postgres: attaching indexes: %w", err)
	}

	return snap, dbVersion, nil
}

// attachColumns groups columns under their parent table (by parent OID, parsed
// from the column's "{tableOID}:{attnum}" ID) and sets Table.Columns in place,
// preserving each table's original column (attnum) order. Columns whose parent
// table is not in scope are dropped.
func attachColumns(tables []Table, columns []Column) {
	oidToIdx := make(map[string]int, len(tables))
	for i, t := range tables {
		oidToIdx[t.ID] = i
	}
	for _, c := range columns {
		tableOID := c.ID
		if idx := strings.IndexByte(tableOID, ':'); idx >= 0 {
			tableOID = tableOID[:idx]
		}
		if ti, ok := oidToIdx[tableOID]; ok {
			tables[ti].Columns = append(tables[ti].Columns, c)
		}
	}
}

// constraintRow is one row of sqlLoadConstraintsFmt.
type constraintRow struct {
	tableOID string
	contype  string
	Constraint
}

// indexRow is one row of sqlLoadIndexesFmt, with the definition already reduced to the
// part after the table name.
type indexRow struct {
	tableOID string
	isUnique bool
	Index
}

// attachConstraints routes each constraint to the Table field for its contype, by table
// OID. A constraint whose table is not in tables is an error.
func attachConstraints(tables []Table, rows []constraintRow) error {
	oidToIdx := make(map[string]int, len(tables))
	for i, t := range tables {
		oidToIdx[t.ID] = i
	}
	for _, r := range rows {
		ti, ok := oidToIdx[r.tableOID]
		if !ok {
			return goerrors.Errorf("constraint %q belongs to table OID %s, which is not in the snapshot", r.Name, r.tableOID)
		}
		tbl := &tables[ti]
		con := r.Constraint
		con.Table = tbl.ObjectRef
		switch r.contype {
		case "p":
			if tbl.PrimaryKey != nil {
				return goerrors.Errorf("table %s.%s has two primary keys: %q and %q",
					tbl.Schema, tbl.Name, tbl.PrimaryKey.Name, con.Name)
			}
			tbl.PrimaryKey = &con
		case "u":
			tbl.UniqueKeys = append(tbl.UniqueKeys, con)
		case "c":
			tbl.Checks = append(tbl.Checks, con)
		case "f":
			tbl.ForeignKeys = append(tbl.ForeignKeys, con)
		default:
			return goerrors.Errorf("constraint %q on table %s.%s has unexpected contype %q",
				con.Name, tbl.Schema, tbl.Name, r.contype)
		}
	}
	return nil
}

// attachIndexes routes each index to UniqueIndexes or Indexes by table OID. An index
// whose table is not in tables is an error.
func attachIndexes(tables []Table, rows []indexRow) error {
	oidToIdx := make(map[string]int, len(tables))
	for i, t := range tables {
		oidToIdx[t.ID] = i
	}
	for _, r := range rows {
		ti, ok := oidToIdx[r.tableOID]
		if !ok {
			return goerrors.Errorf("index %q belongs to table OID %s, which is not in the snapshot", r.Name, r.tableOID)
		}
		tbl := &tables[ti]
		idx := r.Index
		idx.Table = tbl.ObjectRef
		if r.isUnique {
			tbl.UniqueIndexes = append(tbl.UniqueIndexes, idx)
		} else {
			tbl.Indexes = append(tbl.Indexes, idx)
		}
	}
	return nil
}

// ─── Helper functions ──────────────────────────────────────────────────────────

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

// detectDatabaseVersion probes the PostgreSQL server_version and returns just
// the version number (truncated at the first space).
func detectDatabaseVersion(ctx context.Context, db QueryExecutor) (string, error) {
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

var leadingDigitsRegex = regexp.MustCompile(`^\d+`)

// parseMajorVersion returns the leading integer of a server version such as "16.4",
// "9.6.24", "17beta1" or "11.2-YB-2.20.1.0-b0".
func parseMajorVersion(version string) (int, error) {
	digits := leadingDigitsRegex.FindString(version)
	if digits == "" {
		return 0, goerrors.Errorf("cannot parse major version from %q", version)
	}
	return strconv.Atoi(digits)
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
func loadTableLinks(ctx context.Context, db QueryExecutor, placeholders string, args []interface{}) ([]tableLink, error) {
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
func linkTableHierarchy(tables []Table, links []tableLink) {
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
				ref := ObjectRef{Schema: lnk.parentSchema, Name: lnk.parentName}
				tables[childIdx].PartitionParent = &ref
			}
			// Only wire PartitionChildren if the parent is also in scope.
			if parentInScope {
				tables[parentIdx].PartitionChildren = append(
					tables[parentIdx].PartitionChildren,
					ObjectRef{Schema: lnk.childSchema, Name: lnk.childName},
				)
			}
		} else {
			// Legacy INHERITS: a child may inherit from multiple parents.
			if childInScope {
				tables[childIdx].InheritsFrom = append(
					tables[childIdx].InheritsFrom,
					ObjectRef{Schema: lnk.parentSchema, Name: lnk.parentName},
				)
			}
			if parentInScope {
				tables[parentIdx].InheritedBy = append(
					tables[parentIdx].InheritedBy,
					ObjectRef{Schema: lnk.childSchema, Name: lnk.childName},
				)
			}
		}
	}
}

// loadTables queries pg_class for tables (ordinary, partitioned, foreign) in the
// given schemas, then queries pg_inherits to wire both declarative-partition and
// legacy-inheritance parent/child links onto the returned tables.
func loadTables(ctx context.Context, db QueryExecutor, placeholders string, args []interface{}) ([]Table, error) {
	query := fmt.Sprintf(sqlLoadTablesFmt, placeholders)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query pg_class: %w", err)
	}
	defer rows.Close()

	var tables []Table
	for rows.Next() {
		var oid, schema, name, relkind string
		if err := rows.Scan(&oid, &schema, &name, &relkind); err != nil {
			return nil, fmt.Errorf("scan table row: %w", err)
		}
		kind := relkindToTableKind(relkind)
		tables = append(tables, Table{
			ObjectRef: ObjectRef{Schema: schema, Name: name},
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
// given schemas and returns them as a slice of Column.
func loadColumns(ctx context.Context, db QueryExecutor, placeholders string, args []interface{}) ([]Column, error) {
	query := fmt.Sprintf(sqlLoadColumnsFmt, placeholders)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query pg_attribute: %w", err)
	}
	defer rows.Close()

	var columns []Column
	for rows.Next() {
		var tableOID, attnum, schema, tableName, colName, dataType, colDefault string
		var notNull bool
		if err := rows.Scan(&tableOID, &attnum, &schema, &tableName, &colName, &dataType, &notNull, &colDefault); err != nil {
			return nil, fmt.Errorf("scan column row: %w", err)
		}
		col := Column{
			TableScopedObjectRef: TableScopedObjectRef{
				Table: ObjectRef{Schema: schema, Name: tableName},
				Name:  colName,
			},
			ID:       tableOID + ":" + attnum,
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

func buildConstraintsQuery(placeholders string, skipReferencedPartitionFKs bool) string {
	extraFilter := ""
	if skipReferencedPartitionFKs {
		extraFilter = sqlSkipReferencedPartitionFKs
	}
	return fmt.Sprintf(sqlLoadConstraintsFmt, placeholders, extraFilter)
}

// loadConstraints queries pg_constraint for primary key, unique, check and foreign key
// constraints of tables in the given schemas.
func loadConstraints(ctx context.Context, db QueryExecutor, placeholders string, args []interface{}, skipReferencedPartitionFKs bool) ([]constraintRow, error) {
	query := buildConstraintsQuery(placeholders, skipReferencedPartitionFKs)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query pg_constraint: %w", err)
	}
	defer rows.Close()

	var out []constraintRow
	for rows.Next() {
		var r constraintRow
		if err := rows.Scan(&r.tableOID, &r.Name, &r.ID, &r.contype, &r.Definition, &r.FromParent); err != nil {
			return nil, fmt.Errorf("scan constraint row: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate constraint rows: %w", err)
	}
	return out, nil
}

// loadIndexes queries pg_index for valid indexes, of tables in the given schemas, that
// back no primary key, unique or exclusion constraint.
func loadIndexes(ctx context.Context, db QueryExecutor, placeholders string, args []interface{}) ([]indexRow, error) {
	query := fmt.Sprintf(sqlLoadIndexesFmt, placeholders)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query pg_index: %w", err)
	}
	defer rows.Close()

	var out []indexRow
	for rows.Next() {
		var r indexRow
		var quotedName, indexDef, tableRegclass string
		if err := rows.Scan(&r.tableOID, &r.Name, &r.ID, &r.isUnique, &quotedName, &indexDef, &tableRegclass, &r.FromParent); err != nil {
			return nil, fmt.Errorf("scan index row: %w", err)
		}
		r.Definition, err = stripIndexDefPrefix(indexDef, quotedName, tableRegclass, r.isUnique)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate index rows: %w", err)
	}
	return out, nil
}

// stripIndexDefPrefix removes "CREATE [UNIQUE] INDEX <name> ON [ONLY ]<table> " from a
// pg_get_indexdef result, leaving "USING ..." onward. The prefix is rebuilt from the
// quoted name and the table text the same query returned, so a mismatch means the
// output format changed and is an error rather than a guess.
func stripIndexDefPrefix(indexDef, quotedName, tableRegclass string, isUnique bool) (string, error) {
	create := "CREATE INDEX "
	if isUnique {
		create = "CREATE UNIQUE INDEX "
	}
	prefix := create + quotedName + " ON " + tableRegclass + " "
	if rest, ok := strings.CutPrefix(indexDef, prefix); ok {
		return rest, nil
	}
	onlyPrefix := create + quotedName + " ON ONLY " + tableRegclass + " "
	if rest, ok := strings.CutPrefix(indexDef, onlyPrefix); ok {
		return rest, nil
	}
	return "", goerrors.Errorf("index %s: definition %q does not start with %q or %q",
		quotedName, indexDef, prefix, onlyPrefix)
}

// relkindToTableKind converts a pg_class.relkind character to a TableKind.
func relkindToTableKind(relkind string) TableKind {
	switch relkind {
	case "r":
		return TableKindOrdinary
	case "p":
		return TableKindPartitioned
	case "f":
		return TableKindForeign
	default:
		return TableKindOrdinary
	}
}
