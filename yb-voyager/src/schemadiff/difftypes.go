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
// can be detected between two snapshots. Only the types the diff engine emits
// are declared here.
type DiffType string

const (
	// Table-level findings.
	TableAdded                    DiffType = "TABLE_ADDED"
	TableDropped                  DiffType = "TABLE_DROPPED"
	TableNameChanged              DiffType = "TABLE_NAME_CHANGED"
	TableSchemaChanged            DiffType = "TABLE_SCHEMA_CHANGED"
	TableKindChanged              DiffType = "TABLE_KIND_CHANGED"
	TablePartitionAttached        DiffType = "TABLE_PARTITION_ATTACHED"
	TablePartitionDetached        DiffType = "TABLE_PARTITION_DETACHED"
	TablePartitionChildrenChanged DiffType = "TABLE_PARTITION_CHILDREN_CHANGED"
	TableInheritAdded             DiffType = "TABLE_INHERIT_ADDED"
	TableInheritRemoved           DiffType = "TABLE_INHERIT_REMOVED"
	TableInheritedByAdded         DiffType = "TABLE_INHERITED_BY_ADDED"
	TableInheritedByRemoved       DiffType = "TABLE_INHERITED_BY_REMOVED"

	// Column-level findings.
	ColumnAdded          DiffType = "COLUMN_ADDED"
	ColumnDropped        DiffType = "COLUMN_DROPPED"
	ColumnNameChanged    DiffType = "COLUMN_NAME_CHANGED"
	ColumnTypeChanged    DiffType = "COLUMN_TYPE_CHANGED"
	ColumnNotNullSet     DiffType = "COLUMN_NOT_NULL_SET"
	ColumnNotNullDropped DiffType = "COLUMN_NOT_NULL_DROPPED"
	ColumnDefaultChanged DiffType = "COLUMN_DEFAULT_CHANGED"

	// Key findings.
	PrimaryKeyAdded       DiffType = "PRIMARY_KEY_ADDED"
	PrimaryKeyDropped     DiffType = "PRIMARY_KEY_DROPPED"
	PrimaryKeyNameChanged DiffType = "PRIMARY_KEY_NAME_CHANGED"
	UniqueAdded           DiffType = "UNIQUE_ADDED"
	UniqueDropped         DiffType = "UNIQUE_DROPPED"
	UniqueNameChanged     DiffType = "UNIQUE_NAME_CHANGED"
	CheckAdded            DiffType = "CHECK_ADDED"
	CheckDropped          DiffType = "CHECK_DROPPED"
	CheckNameChanged      DiffType = "CHECK_NAME_CHANGED"
	ForeignKeyAdded       DiffType = "FOREIGN_KEY_ADDED"
	ForeignKeyDropped     DiffType = "FOREIGN_KEY_DROPPED"
	ForeignKeyNameChanged DiffType = "FOREIGN_KEY_NAME_CHANGED"
)

// AllDiffTypes returns every DiffType the engine can emit, so consumers keyed by
// DiffType can be checked for completeness.
func AllDiffTypes() []DiffType {
	return []DiffType{
		TableAdded, TableDropped, TableNameChanged, TableSchemaChanged, TableKindChanged,
		TablePartitionAttached, TablePartitionDetached, TablePartitionChildrenChanged,
		TableInheritAdded, TableInheritRemoved, TableInheritedByAdded, TableInheritedByRemoved,
		ColumnAdded, ColumnDropped, ColumnNameChanged, ColumnTypeChanged,
		ColumnNotNullSet, ColumnNotNullDropped, ColumnDefaultChanged,
		PrimaryKeyAdded, PrimaryKeyDropped, PrimaryKeyNameChanged,
		UniqueAdded, UniqueDropped, UniqueNameChanged,
		CheckAdded, CheckDropped, CheckNameChanged,
		ForeignKeyAdded, ForeignKeyDropped, ForeignKeyNameChanged,
	}
}

// Operation is the kind of change a Difference records: an object added or
// dropped, or one of its attributes changed. It is the decomposed verb of Type
// (e.g. COLUMN_TYPE_CHANGED → OpChanged), surfaced as a first-class field so
// consumers can group/filter by verb without parsing Type. Type's suffix is not
// always the verb: the split attribute types (COLUMN_NOT_NULL_DROPPED,
// TABLE_PARTITION_ATTACHED, TABLE_INHERIT_ADDED, ...) are OpChanged.
type Operation string

const (
	OpAdded   Operation = "ADDED"
	OpDropped Operation = "DROPPED"
	OpChanged Operation = "CHANGED"
)

// ObjectType is the honest object type of the changed object and the user-facing
// selector matched by --object-type-list. It is the decomposed noun of Type
// (e.g. COLUMN_TYPE_CHANGED → ObjectTypeColumn), a first-class facet alongside
// Operation and Attribute. Only the object types the engine emits are declared;
// the remaining selectors are re-enabled as the engine starts emitting their
// findings.
//
// COLUMN is its own selector: a column change is filtered directly by
// --object-type-list=COLUMN (it is NOT swept in under TABLE). Table-scoping is
// orthogonal — a column finding still anchors to its host table for --table-list.
type ObjectType string

const (
	ObjectTypeTable  ObjectType = "TABLE"
	ObjectTypeColumn ObjectType = "COLUMN"
	// ObjectTypeConstraint covers primary keys now, and the unique, check and foreign
	// key constraints as their findings land. Like COLUMN, it anchors to its host table
	// for --table-list.
	ObjectTypeConstraint ObjectType = "CONSTRAINT"
	// Not yet emitted by the diff engine — uncomment each as its findings land
	// (INDEX is its own selector but stays anchored to its host table for
	// --table-list, like COLUMN):
	// ObjectTypeIndex    ObjectType = "INDEX"
	// ObjectTypeSequence ObjectType = "SEQUENCE"
	// ObjectTypeView     ObjectType = "VIEW"
	// ObjectTypeFunction ObjectType = "FUNCTION"
	// ObjectTypeType     ObjectType = "TYPE"
)

// Attribute names the specific object attribute an OpChanged finding is about
// (e.g. COLUMN_TYPE_CHANGED → AttrType). It is AttrNone for OpAdded/OpDropped
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
// and attr are its decomposed facets (attr is AttrNone for OpAdded/OpDropped). objA/objB
// are the side-A/side-B identities (nil on the absent side: objA nil for OpAdded,
// objB nil for OpDropped). Identities are value types, so they are copied into the
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
