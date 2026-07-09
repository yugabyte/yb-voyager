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
	TableAdded                    DiffType = "TABLE_ADDED"
	TableDropped                  DiffType = "TABLE_DROPPED"
	TableNameChanged              DiffType = "TABLE_NAME_CHANGED"
	TableSchemaChanged            DiffType = "TABLE_SCHEMA_CHANGED"
	TableKindChanged              DiffType = "TABLE_KIND_CHANGED"
	TablePartitionParentChanged   DiffType = "TABLE_PARTITION_PARENT_CHANGED"
	TablePartitionChildrenChanged DiffType = "TABLE_PARTITION_CHILDREN_CHANGED"
	TableInheritsChanged          DiffType = "TABLE_INHERITS_CHANGED"
	TableInheritedByChanged       DiffType = "TABLE_INHERITED_BY_CHANGED"

	// Column-level findings.
	ColumnAdded              DiffType = "COLUMN_ADDED"
	ColumnDropped            DiffType = "COLUMN_DROPPED"
	ColumnNameChanged        DiffType = "COLUMN_NAME_CHANGED"
	ColumnTypeChanged        DiffType = "COLUMN_TYPE_CHANGED"
	ColumnNullabilityChanged DiffType = "COLUMN_NULLABILITY_CHANGED"
	ColumnDefaultChanged     DiffType = "COLUMN_DEFAULT_CHANGED"
)

// diffTypeDefs is an exhaustive registry mapping every DiffType constant to its
// scope bucket (ObjectType), covering all 15 V1-emitted types (tables and
// columns). Single source of truth for the Type↔ObjectType contract used by
// FilterByScope.
//
// Note: only ObjectTypeTable is declared and used here — V1 emits no index,
// sequence, view, function, or type findings, so the other selectors are commented
// out in filter.go and re-enabled alongside the findings they select.
var diffTypeDefs = map[DiffType]ObjectType{
	// ── TABLE ────────────────────────────────────────────────────────────────
	TableAdded:                    ObjectTypeTable,
	TableDropped:                  ObjectTypeTable,
	TableNameChanged:              ObjectTypeTable,
	TableSchemaChanged:            ObjectTypeTable,
	TableKindChanged:              ObjectTypeTable,
	TablePartitionParentChanged:   ObjectTypeTable,
	TablePartitionChildrenChanged: ObjectTypeTable,
	TableInheritsChanged:          ObjectTypeTable,
	TableInheritedByChanged:       ObjectTypeTable,

	// ── COLUMN ───────────────────────────────────────────────────────────────
	// Columns "ride with" their parent table and are not independently selectable.
	ColumnAdded:              ObjectTypeTable,
	ColumnDropped:            ObjectTypeTable,
	ColumnNameChanged:        ObjectTypeTable,
	ColumnTypeChanged:        ObjectTypeTable,
	ColumnNullabilityChanged: ObjectTypeTable,
	ColumnDefaultChanged:     ObjectTypeTable,
}

// newDifference builds a Difference for any DiffType — added, dropped, or changed.
//
// obj is the finding's rendered/sorted identity: the side-B rendering for
// *_ADDED, the side-A rendering otherwise; for a column finding, schema.table.column.
//
// anchorTable is the table the finding filters under (--table-list / --exclude-
// table-list). It LOOKS redundant in V1 — every finding anchors to its own object,
// so all callers pass &obj's underlying table ref — but is taken explicitly because
// future findings (INDEX_*/owned SEQUENCE_* anchor to their host table; top-level
// VIEW_*/FUNCTION_*/TYPE_* have no anchor) will make obj and anchor diverge.
// Carrying the parameter now means those types slot in without a signature
// change. It's a pointer so "no anchor" can be nil, and copied internally so
// AnchorTable never aliases caller or snapshot storage.
func newDifference(t DiffType, obj QualifiedObject, anchorTable *schemasnapshot.ObjectRef, oldVal, newVal any) Difference {
	var anchor *schemasnapshot.ObjectRef
	if anchorTable != nil {
		a := *anchorTable
		anchor = &a
	}
	return Difference{
		Type:        t,
		Object:      obj,
		AnchorTable: anchor,
		OldValue:    oldVal,
		NewValue:    newVal,
	}
}
