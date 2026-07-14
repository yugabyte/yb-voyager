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

// Operation is the kind of change a Difference records: an object added or
// dropped, or one of its attributes changed. It is the decomposed verb of Type
// (e.g. COLUMN_TYPE_CHANGED → OpChanged), surfaced as a first-class field so
// consumers can group/filter by verb without parsing Type.
type Operation string

const (
	OpAdded   Operation = "ADDED"
	OpDropped Operation = "DROPPED"
	OpChanged Operation = "CHANGED"
)

// Attribute names the specific object attribute a *_CHANGED finding is about
// (e.g. COLUMN_TYPE_CHANGED → AttrType). It is AttrNone for ADDED/DROPPED
// findings, which concern the whole object rather than one attribute.
type Attribute string

const (
	AttrNone              Attribute = "" // added/dropped: no single attribute
	AttrName              Attribute = "NAME"
	AttrSchema            Attribute = "SCHEMA"
	AttrKind              Attribute = "KIND"
	AttrType              Attribute = "TYPE"
	AttrNullability       Attribute = "NULLABILITY"
	AttrDefault           Attribute = "DEFAULT"
	AttrPartitionParent   Attribute = "PARTITION_PARENT"
	AttrPartitionChildren Attribute = "PARTITION_CHILDREN"
	AttrInherits          Attribute = "INHERITS"
	AttrInheritedBy       Attribute = "INHERITED_BY"
)

// newDifference builds a Difference. Type is the single-string kind; op, objType,
// and attr are its decomposed facets (attr is AttrNone for ADDED/DROPPED). objA/objB
// are the side-A/side-B identities (nil on the absent side: objA nil for *_ADDED,
// objB nil for *_DROPPED). Identities are value types, so they are copied into the
// interface — no aliasing with snapshot storage.
func newDifference(t DiffType, op Operation, objType ObjectType, attr Attribute, objA, objB ObjectIdent, sideAVal, sideBVal any) Difference {
	return Difference{
		Type:       t,
		Operation:  op,
		ObjectType: objType,
		Attribute:  attr,
		ObjectA:    objA,
		ObjectB:    objB,
		SideAValue: sideAVal,
		SideBValue: sideBVal,
	}
}
