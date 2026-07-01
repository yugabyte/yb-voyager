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

import "time"

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
// TODO(schemadiff): handle case sensitivity in a coming PR. Schema/Name are stored
// as the raw catalog identifiers and String() joins them as "schema.name" by plain
// concatenation. This is consistent for source-vs-source matching, but quoted /
// case-sensitive identifiers (and the --table-list matching boundary) need proper
// handling — likely via the sqlname helpers rather than ad-hoc string join.
type ObjectRef struct {
	Schema string `json:"schema"` // schema (namespace) the object lives in, e.g. "public".
	Name   string `json:"name"`   // object name within the schema, e.g. "orders".
}

// String returns "schema.name".
func (o ObjectRef) String() string { return o.Schema + "." + o.Name }

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
