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
	"fmt"
	"time"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

// SchemaSnapshot is a point-in-time capture of a database schema: its header (metadata,
// stored in metadb columns) and its content (serialized to the blob).
type SchemaSnapshot struct {
	Header  SnapshotHeader
	Content *SnapshotContent
}

// ─── Header: per-snapshot metadata (stored in metadb columns) ───────────────────

// SnapshotHeader is the per-snapshot metadata stored in metadb COLUMNS (not in the blob).
// It is produced by Capture, written to columns by SaveSnapshot/SavePlaceholder, and
// returned by ListSnapshots.
type SnapshotHeader struct {
	Label           string    // capture label (a labels.go constant); the filing key
	Reason          string    // capture reason; "" when none
	Side            string    // migration side (SideSource in v1)
	CapturedAt      time.Time // capture time (UTC)
	DatabaseVersion string    // server version, e.g. "16.4"
	Schemas         []string  // schemas in scope
	// IsPlaceholder marks a failed-capture marker row (no schema content). It is set at
	// header construction (false for a real capture, true on the placeholder path),
	// persisted to its own column, and read back directly.
	IsPlaceholder bool
}

// Name is the derived primary-key handle "{label}_{second-precision-timestamp}".
// Derived (not stored) so the struct carries no write-vs-read-asymmetric identity field.
func (h SnapshotHeader) Name() string { return deriveName(h.Label, h.CapturedAt) }

// deriveName builds the primary-key name "{label}_{timestamp-at-second-precision}".
func deriveName(label string, capturedAt time.Time) string {
	return fmt.Sprintf("%s_%s", label, capturedAt.UTC().Format("20060102T150405Z"))
}

// SideSource is the capture side for the migration source database. v1 only ever
// captures the source; SnapshotHeader.Side exists so other sides (e.g. target,
// source-replica) can be added later.
const SideSource = "source"

// ─── Content: the schema itself (serialized to the blob) ────────────────────────

// SnapshotContent is the schema CONTENT — this is what gets serialized to the metadb blob.
// The per-object lists are flat; each record keys back to its parent via an ObjectRef field.
// v1 carries tables and columns only.
type SnapshotContent struct {
	Version      int        `json:"version"`           // format/compat gate
	DatabaseType string     `json:"database_type"`     // engine; the diff reads this
	DBMetadata   DBMetadata `json:"db_metadata"`       // display coordinates; provenance for reports
	Tables       []Table    `json:"tables,omitempty"`  // captured tables (ordinary/partitioned/foreign).
	Columns      []Column   `json:"columns,omitempty"` // captured columns; each keyed back to its parent table via Column.Table.
}

// DBMetadata holds the display coordinates of the captured database (for report generation).
// It is display-only identity — never connection secrets. No DatabaseType or Side here;
// those live at the SnapshotContent and SnapshotHeader levels respectively.
type DBMetadata struct {
	Host     string `json:"host"`     // source host, for display.
	Port     int    `json:"port"`     // source port, for display.
	Database string `json:"database"` // source database name, for display.
	User     string `json:"user"`     // connecting user, for display.
}

// ObjectRef is the (schema, name) identity embedded in every per-object struct.
//
// Schema and Name hold the raw, unquoted catalog names exactly as returned by the
// database catalog (e.g. pg_class.relname). Rendering and matching are engine-aware:
// use ForDisplay(dbType) for human-facing output and ForKey(dbType) for collision-safe
// map keys. The struct itself is comparable and can be used as a Go map key.
type ObjectRef struct {
	Schema string `json:"schema"` // schema (namespace) the object lives in, e.g. "public".
	Name   string `json:"name"`   // object name within the schema, e.g. "orders".
}

// sqlName builds the sqlname view for dbType. Passing defaultSchema="" means
// MinQualified == Qualified, so String() yields the fully-qualified form.
func (o ObjectRef) sqlName(dbType string) *sqlname.ObjectName {
	return sqlname.NewObjectName(dbType, "", o.Schema, o.Name)
}

// ForDisplay returns the minimally-quoted, always-fully-qualified rendering for
// reports, logs, and user-facing SQL. For example, a lowercase PG table renders
// as public.orders (no quotes needed), while a mixed-case table renders as
// public."Orders".
func (o ObjectRef) ForDisplay(dbType string) string { return o.sqlName(dbType).String() }

// ForKey returns the case-sensitive, per-part-quoted, collision-safe canonical key
// suitable for use as a string map key. For example: "public"."orders".
// Two ObjectRefs that differ only in case produce different ForKey values, and two
// refs that would naively produce the same "schema.name" join (e.g. schema "a.b" /
// name "c" vs schema "a" / name "b.c") produce different ForKey values because each
// part is independently double-quoted.
func (o ObjectRef) ForKey(dbType string) string { return o.sqlName(dbType).Qualified.Quoted }

// TableKind maps pg_class.relkind to a portable enum value.
type TableKind string

const (
	TableKindOrdinary    TableKind = "ordinary"
	TableKindPartitioned TableKind = "partitioned"
	TableKindForeign     TableKind = "foreign"
)

// Table represents a table (ordinary, partitioned, or foreign).
type Table struct {
	ObjectRef                     // embedded (schema, name) identity of the table.
	ID                string      `json:"id"`                           // pg_class.oid as a decimal string; used to match the table across snapshots (rename detection).
	Kind              TableKind   `json:"kind"`                         // relation kind: ordinary, partitioned, or foreign.
	PartitionParent   *ObjectRef  `json:"partition_parent,omitempty"`   // set on a partition child: the table it is a partition of; nil for non-partitions
	PartitionChildren []ObjectRef `json:"partition_children,omitempty"` // set on a partitioned parent: its immediate partition children; nil otherwise
	InheritsFrom      []ObjectRef `json:"inherits_from,omitempty"`      // legacy table inheritance (INHERITS): parent table(s) this table inherits from; can be multiple. Distinct from declarative partitioning.
	InheritedBy       []ObjectRef `json:"inherited_by,omitempty"`       // legacy table inheritance (INHERITS): tables that inherit from this one.
}

// Column represents a single column within a table.
// ID encodes the parent table OID and the column attnum as "{parentTableOID}:{attnum}".
type Column struct {
	Table ObjectRef `json:"table"`        // identity of the parent table this column belongs to.
	ID    string    `json:"id,omitempty"` // "{parentTableOID}:{attnum}"; matches the column across snapshots even after a rename.
	Name  string    `json:"name"`         // column name.
	// TODO(schemadiff): normalize the type before comparison in a future PR. Today
	// this is source-vs-source (same engine's format_type() on both sides), so the
	// raw string compares correctly. PG-vs-YB is also fine — YB shares PostgreSQL's
	// type system and format_type() output. Normalization is needed only once we
	// compare across different engines (e.g. Oracle/MySQL as source), where the same
	// logical type is named/rendered differently and must be mapped to a shared
	// vocabulary (common type enums / a normalizeType helper) before diffing.
	DataType string `json:"data_type"`         // rendered type via format_type(), e.g. "integer", "character varying(255)".
	NotNull  bool   `json:"not_null"`          // true when the column has a NOT NULL constraint.
	Default  string `json:"default,omitempty"` // default expression text (pg_get_expr); "" when the column has no default.
}

// sqlName builds the sqlname view of this column's fully-qualified (schema, table,
// column) identity. defaultSchema="" keeps it always fully qualified.
func (c Column) sqlName(dbType string) *sqlname.ObjectNameQualifiedWithTableName {
	return sqlname.NewObjectNameQualifiedWithTableName(dbType, "", c.Name, c.Table.Schema, c.Table.Name)
}

// ForKey returns a collision-safe, per-part-quoted canonical key for the column:
// "public"."orders"."Col".
func (c Column) ForKey(dbType string) string { return c.sqlName(dbType).Qualified.Quoted }

// ForDisplay returns the minimally-quoted, always-fully-qualified rendering of the
// column for reports, logs, and user-facing SQL: public.orders."Col".
func (c Column) ForDisplay(dbType string) string { return c.sqlName(dbType).MinQualified.MinQuoted }
