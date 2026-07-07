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

package schemadiff

import "github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"

// DiffType is a string-typed enumeration of the kinds of schema changes that
// can be detected between two snapshots. Only the V1-emitted set is declared
// here: table-level and column-level findings produced by diffTables and
// diffColumns respectively.
type DiffType string

const (
	// Table-level findings.
	TableAdded               DiffType = "TABLE_ADDED"
	TableDropped             DiffType = "TABLE_DROPPED"
	TableNameChanged         DiffType = "TABLE_NAME_CHANGED"
	TableSchemaChanged       DiffType = "TABLE_SCHEMA_CHANGED"
	TableKindChanged         DiffType = "TABLE_KIND_CHANGED"
	PartitionParentChanged   DiffType = "PARTITION_PARENT_CHANGED"
	PartitionChildrenChanged DiffType = "PARTITION_CHILDREN_CHANGED"
	TableInheritsChanged     DiffType = "TABLE_INHERITS_CHANGED"
	TableInheritedByChanged  DiffType = "TABLE_INHERITED_BY_CHANGED"

	// Column-level findings.
	ColumnAdded              DiffType = "COLUMN_ADDED"
	ColumnDropped            DiffType = "COLUMN_DROPPED"
	ColumnNameChanged        DiffType = "COLUMN_NAME_CHANGED"
	ColumnTypeChanged        DiffType = "COLUMN_TYPE_CHANGED"
	ColumnNullabilityChanged DiffType = "COLUMN_NULLABILITY_CHANGED"
	ColumnDefaultChanged     DiffType = "COLUMN_DEFAULT_CHANGED"
)

// diffTypeDef holds the static, per-DiffType facts derivable from the type alone:
// its scope bucket (for FilterByScope) and the canonical Property name that the
// finding (and the command's JSON "property" field) carries. Single source of
// truth so the Type↔ObjectType and Type↔Property contracts cannot drift.
type diffTypeDef struct {
	ObjectType ObjectType // scope bucket
	Property   string     // canonical property name; "" for *_ADDED / *_DROPPED
}

// diffTypeDefs is an exhaustive registry of every DiffType constant, covering all
// 15 V1-emitted types (tables and columns). It is the single source of truth for
// the Type↔ObjectType and Type↔Property contracts used by FilterByScope and the
// property-change constructors.
//
// Note: only ObjectTypeTable is declared and used here — V1 emits no index,
// sequence, view, function, or type findings, so the other selectors are commented
// out in filter.go and re-enabled alongside the findings they select.
var diffTypeDefs = map[DiffType]diffTypeDef{
	// ── TABLE ────────────────────────────────────────────────────────────────
	TableAdded:               {ObjectTypeTable, ""},
	TableDropped:             {ObjectTypeTable, ""},
	TableNameChanged:         {ObjectTypeTable, "name"},
	TableSchemaChanged:       {ObjectTypeTable, "schema"},
	TableKindChanged:         {ObjectTypeTable, "kind"},
	PartitionParentChanged:   {ObjectTypeTable, "partition_parent"},
	PartitionChildrenChanged: {ObjectTypeTable, "partition_children"},
	TableInheritsChanged:     {ObjectTypeTable, "inherits_from"},
	TableInheritedByChanged:  {ObjectTypeTable, "inherited_by"},

	// ── COLUMN ───────────────────────────────────────────────────────────────
	// Columns "ride with" their parent table and are not independently selectable.
	ColumnAdded:              {ObjectTypeTable, ""},
	ColumnDropped:            {ObjectTypeTable, ""},
	ColumnNameChanged:        {ObjectTypeTable, "name"},
	ColumnTypeChanged:        {ObjectTypeTable, "data_type"},
	ColumnNullabilityChanged: {ObjectTypeTable, "not_null"},
	ColumnDefaultChanged:     {ObjectTypeTable, "default"},
}

// newDifference builds a Difference for any DiffType — added, dropped, or changed.
//
// obj is the identity the finding is reported against and sorted by: the side-B
// ref for *_ADDED, the side-A ref otherwise; for a column finding it is the
// parent table's ref.
//
// anchorTable is the table the finding filters under (--table-list / --exclude-
// table-list). It LOOKS redundant in V1: every finding emitted today anchors to
// its own object, so all callers pass &obj (a table anchors to itself; a column
// to its parent table, which is also obj). It is taken explicitly anyway because
// the findings V1 does not emit yet make obj and the anchor diverge — an INDEX_*
// or owned-SEQUENCE_* finding's obj is the index/sequence while anchorTable is the
// host/owner table, and a top-level view/function/type finding has no anchor (nil).
// Carrying the parameter now means those types slot in without changing this
// signature or any existing call site's shape. It is a pointer so "no anchor" can
// be nil, and it is copied internally so AnchorTable never aliases caller or
// snapshot storage.
//
// subObject is the dependent's name (the column name; "" for table findings).
// Property is derived from diffTypeDefs ("" for *_ADDED / *_DROPPED).
func newDifference(t DiffType, obj schemasnapshot.ObjectRef, anchorTable *schemasnapshot.ObjectRef, subObject string, oldVal, newVal any) Difference {
	var anchor *schemasnapshot.ObjectRef
	if anchorTable != nil {
		a := *anchorTable
		anchor = &a
	}
	return Difference{
		Type:        t,
		Object:      obj,
		AnchorTable: anchor,
		SubObject:   subObject,
		Property:    diffTypeDefs[t].Property,
		OldValue:    oldVal,
		NewValue:    newVal,
	}
}
