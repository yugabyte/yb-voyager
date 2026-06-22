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

// Difference describes a single detected schema change between two snapshots.
type Difference struct {
	Type        DiffType
	Object      schemasnapshot.ObjectRef  // anchor: side-A (old) for most; side-B (new) for *_ADDED
	AnchorTable *schemasnapshot.ObjectRef // table this finding filters under; nil for non-table-anchored
	SubObject   string                    // dependent's name (column, etc.); "" for object-level
	Property    string                    // changed attribute name; "" for object-level
	OldValue    any                       // value on side A; nil if absent on A
	NewValue    any                       // value on side B; nil if absent on B
	Details     string                    // optional human-readable summary
}
