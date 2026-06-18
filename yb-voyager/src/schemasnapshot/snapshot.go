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

// SchemaSnapshot is a point-in-time capture of a database schema.
// The per-object lists are flat; each record keys back to its parent via an ObjectRef field.
// v1 carries tables and columns only.
type SchemaSnapshot struct {
	Version         int           `json:"version"`                    // snapshot JSON format version; gates parse-compatibility on load. Currently 1.
	DatabaseType    string        `json:"database_type"`              // source engine; selects the provider that produced this snapshot, e.g. "postgresql".
	DatabaseVersion string        `json:"database_version,omitempty"` // server version, truncated at the first space (e.g. "16.4"); display-only, never diffed.
	StableIdentity  bool          `json:"stable_identity"`            // true when object IDs are stable enough for rename matching (PostgreSQL OIDs: always true).
	CapturedAt      time.Time     `json:"captured_at"`                // time the snapshot was taken, in UTC.
	CaptureSource   CaptureSource `json:"capture_source"`             // descriptive source coordinates (host/port/db/user/role) for display.
	Schemas         []string      `json:"schemas"`                    // schemas in scope at capture time; fixed once captured and never widened/narrowed.
	Series          string        `json:"series,omitempty"`           // the capture series (== persist label); empty until SaveSnapshot stamps it.
	Reason          string        `json:"reason,omitempty"`           // the capture reason where the series carries one; empty otherwise.

	Tables  []Table  `json:"tables,omitempty"`  // captured tables (ordinary/partitioned/foreign).
	Columns []Column `json:"columns,omitempty"` // captured columns; each keyed back to its parent table via Column.Table.
}

// RoleSource is the capture role for the migration source database. v1 only ever
// captures the source; CaptureSource.Role / SnapshotMetadata.Side exist so other
// roles (e.g. target, source-replica) can be added later without changing the types.
const RoleSource = "source"

// CaptureSource holds the descriptive source coordinates for a snapshot.
// It is display-only identity — never connection secrets.
type CaptureSource struct {
	DatabaseType string `json:"database_type"` // source engine; selects the provider, e.g. "postgresql".
	Host         string `json:"host"`          // source host, for display.
	Port         int    `json:"port"`          // source port, for display.
	Database     string `json:"database"`      // source database name, for display.
	User         string `json:"user"`          // connecting user, for display.
	Role         string `json:"role"`          // logical role of this source; "source" in v1.
}

// ObjectRef is the (schema, name) identity embedded in every per-object struct.
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
	Attrs             Attrs       `json:"attrs,omitempty"`              // engine-specific extension attributes; declared as the seam but empty/uncompared in v1.
}

// Column represents a single column within a table.
// ID encodes the parent table OID and the column attnum as "{parentTableOID}:{attnum}".
type Column struct {
	Table    ObjectRef `json:"table"`             // identity of the parent table this column belongs to.
	ID       string    `json:"id,omitempty"`      // "{parentTableOID}:{attnum}"; matches the column across snapshots even after a rename.
	Name     string    `json:"name"`              // column name.
	DataType string    `json:"data_type"`         // rendered type via format_type(), e.g. "integer", "character varying(255)".
	NotNull  bool      `json:"not_null"`          // true when the column has a NOT NULL constraint.
	Default  string    `json:"default,omitempty"` // default expression text (pg_get_expr); "" when the column has no default.
	Attrs    Attrs     `json:"attrs,omitempty"`   // engine-specific extension attributes; declared as the seam but empty/uncompared in v1.
}
