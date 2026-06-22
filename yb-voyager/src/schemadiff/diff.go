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

import (
	"sort"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
)

// DiffType is a string-typed enumeration of the kinds of schema changes that
// can be detected between two snapshots. The full vocabulary is declared here
// for forward-compat; v1 only emits a subset.
type DiffType string

const (
	// Table-level findings.
	TableAdded               DiffType = "TABLE_ADDED"
	TableDropped             DiffType = "TABLE_DROPPED"
	TableNameChanged         DiffType = "TABLE_NAME_CHANGED"
	TableSchemaChanged       DiffType = "TABLE_SCHEMA_CHANGED"
	TableKindChanged         DiffType = "TABLE_KIND_CHANGED"
	PartitionStrategyChanged DiffType = "PARTITION_STRATEGY_CHANGED"
	PartitionKeyChanged      DiffType = "PARTITION_KEY_CHANGED"
	PartitionChildrenChanged DiffType = "PARTITION_CHILDREN_CHANGED"
	ReplicaIdentityChanged   DiffType = "REPLICA_IDENTITY_CHANGED"
	TablePersistenceChanged  DiffType = "TABLE_PERSISTENCE_CHANGED"
	// New in this implementation.
	PartitionParentChanged  DiffType = "PARTITION_PARENT_CHANGED"
	TableInheritsChanged    DiffType = "TABLE_INHERITS_CHANGED"
	TableInheritedByChanged DiffType = "TABLE_INHERITED_BY_CHANGED"

	// Column-level findings.
	ColumnAdded              DiffType = "COLUMN_ADDED"
	ColumnDropped            DiffType = "COLUMN_DROPPED"
	ColumnNameChanged        DiffType = "COLUMN_NAME_CHANGED"
	ColumnTypeChanged        DiffType = "COLUMN_TYPE_CHANGED"
	ColumnNullabilityChanged DiffType = "COLUMN_NULLABILITY_CHANGED"
	ColumnDefaultChanged     DiffType = "COLUMN_DEFAULT_CHANGED"
	ColumnIdentityChanged    DiffType = "COLUMN_IDENTITY_CHANGED"
	ColumnGeneratedChanged   DiffType = "COLUMN_GENERATED_CHANGED"
	ColumnCollationChanged   DiffType = "COLUMN_COLLATION_CHANGED"

	// Constraint-level findings.
	ConstraintAdded            DiffType = "CONSTRAINT_ADDED"
	ConstraintDropped          DiffType = "CONSTRAINT_DROPPED"
	ConstraintNameChanged      DiffType = "CONSTRAINT_NAME_CHANGED"
	PrimaryKeyChanged          DiffType = "PRIMARY_KEY_CHANGED"
	UniqueConstraintChanged    DiffType = "UNIQUE_CONSTRAINT_CHANGED"
	ForeignKeyChanged          DiffType = "FOREIGN_KEY_CHANGED"
	CheckConstraintChanged     DiffType = "CHECK_CONSTRAINT_CHANGED"
	ExclusionConstraintChanged DiffType = "EXCLUSION_CONSTRAINT_CHANGED"
	NullsNotDistinctChanged    DiffType = "NULLS_NOT_DISTINCT_CHANGED"

	// Index-level findings.
	IndexAdded                  DiffType = "INDEX_ADDED"
	IndexDropped                DiffType = "INDEX_DROPPED"
	IndexNameChanged            DiffType = "INDEX_NAME_CHANGED"
	IndexColumnsChanged         DiffType = "INDEX_COLUMNS_CHANGED"
	IndexAccessMethodChanged    DiffType = "INDEX_ACCESS_METHOD_CHANGED"
	IndexUniqueChanged          DiffType = "INDEX_UNIQUE_CHANGED"
	IndexWhereChanged           DiffType = "INDEX_WHERE_CHANGED"
	IndexIncludedColumnsChanged DiffType = "INDEX_INCLUDED_COLUMNS_CHANGED"

	// Sequence-level findings.
	SequenceAdded             DiffType = "SEQUENCE_ADDED"
	SequenceDropped           DiffType = "SEQUENCE_DROPPED"
	SequenceNameChanged       DiffType = "SEQUENCE_NAME_CHANGED"
	SequenceSchemaChanged     DiffType = "SEQUENCE_SCHEMA_CHANGED"
	SequencePropertiesChanged DiffType = "SEQUENCE_PROPERTIES_CHANGED"
	SequenceOwnedByChanged    DiffType = "SEQUENCE_OWNED_BY_CHANGED"

	// View-level findings.
	ViewAdded             DiffType = "VIEW_ADDED"
	ViewDropped           DiffType = "VIEW_DROPPED"
	ViewNameChanged       DiffType = "VIEW_NAME_CHANGED"
	ViewSchemaChanged     DiffType = "VIEW_SCHEMA_CHANGED"
	ViewDefinitionChanged DiffType = "VIEW_DEFINITION_CHANGED"

	// Materialized view-level findings.
	MaterializedViewAdded             DiffType = "MATERIALIZED_VIEW_ADDED"
	MaterializedViewDropped           DiffType = "MATERIALIZED_VIEW_DROPPED"
	MaterializedViewNameChanged       DiffType = "MATERIALIZED_VIEW_NAME_CHANGED"
	MaterializedViewSchemaChanged     DiffType = "MATERIALIZED_VIEW_SCHEMA_CHANGED"
	MaterializedViewDefinitionChanged DiffType = "MATERIALIZED_VIEW_DEFINITION_CHANGED"

	// Function-level findings.
	FunctionAdded                 DiffType = "FUNCTION_ADDED"
	FunctionDropped               DiffType = "FUNCTION_DROPPED"
	FunctionNameChanged           DiffType = "FUNCTION_NAME_CHANGED"
	FunctionSchemaChanged         DiffType = "FUNCTION_SCHEMA_CHANGED"
	FunctionKindChanged           DiffType = "FUNCTION_KIND_CHANGED"
	FunctionSignatureChanged      DiffType = "FUNCTION_SIGNATURE_CHANGED"
	FunctionReturnTypeChanged     DiffType = "FUNCTION_RETURN_TYPE_CHANGED"
	FunctionVolatilityChanged     DiffType = "FUNCTION_VOLATILITY_CHANGED"
	FunctionParallelSafetyChanged DiffType = "FUNCTION_PARALLEL_SAFETY_CHANGED"
	FunctionStrictChanged         DiffType = "FUNCTION_STRICT_CHANGED"
	FunctionLanguageChanged       DiffType = "FUNCTION_LANGUAGE_CHANGED"
	FunctionSecurityChanged       DiffType = "FUNCTION_SECURITY_CHANGED"

	// Trigger-level findings.
	TriggerAdded               DiffType = "TRIGGER_ADDED"
	TriggerDropped             DiffType = "TRIGGER_DROPPED"
	TriggerNameChanged         DiffType = "TRIGGER_NAME_CHANGED"
	TriggerDefinitionChanged   DiffType = "TRIGGER_DEFINITION_CHANGED"
	TriggerEnabledStateChanged DiffType = "TRIGGER_ENABLED_STATE_CHANGED"

	// Type-level findings.
	TypeAdded            DiffType = "TYPE_ADDED"
	TypeDropped          DiffType = "TYPE_DROPPED"
	TypeNameChanged      DiffType = "TYPE_NAME_CHANGED"
	TypeSchemaChanged    DiffType = "TYPE_SCHEMA_CHANGED"
	TypeKindChanged      DiffType = "TYPE_KIND_CHANGED"
	EnumValueAdded       DiffType = "ENUM_VALUE_ADDED"
	EnumValueRemoved     DiffType = "ENUM_VALUE_REMOVED"
	TypeAttributeChanged DiffType = "TYPE_ATTRIBUTE_CHANGED"

	// Generic attribute change.
	AttrChanged DiffType = "ATTR_CHANGED"
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

// Diff computes the schema differences between snapshot a (old/side-A) and b (new/side-B).
// It returns a sorted slice of Difference values.
func Diff(a, b *schemasnapshot.SchemaSnapshot) []Difference {
	var diffs []Difference
	diffs = append(diffs, diffTables(a, b)...)
	diffs = append(diffs, diffColumns(a, b)...)
	diffs = suppressLifecycleTableColumns(diffs)
	sortDifferences(diffs)
	return diffs
}

// suppressLifecycleTableColumns removes per-column COLUMN_ADDED and COLUMN_DROPPED
// findings whose parent table is itself wholly added or dropped in the same diff.
//
// WHY: When a table is wholly added (TABLE_ADDED) or dropped (TABLE_DROPPED), every
// column in that table appears as COLUMN_ADDED or COLUMN_DROPPED respectively. These
// column-level findings are pure noise — the TABLE_ADDED / TABLE_DROPPED finding
// already conveys the full change. Emitting both creates redundant, confusing output.
//
// A renamed table is a *matched* table (it emits TABLE_NAME_CHANGED, not TABLE_ADDED
// or TABLE_DROPPED), so its real column-level changes (adds, drops, type changes, etc.)
// are intentionally preserved: they describe actual mutations to an existing table.
//
// Only COLUMN_ADDED under TABLE_ADDED and COLUMN_DROPPED under TABLE_DROPPED are
// suppressed; all other finding types (e.g. COLUMN_TYPE_CHANGED on matched tables,
// TABLE_NAME_CHANGED, etc.) pass through unchanged.
func suppressLifecycleTableColumns(diffs []Difference) []Difference {
	// Build sets of table ObjectRef strings for wholly added and wholly dropped tables.
	added := make(map[string]struct{})
	dropped := make(map[string]struct{})
	for _, d := range diffs {
		switch d.Type {
		case TableAdded:
			added[d.Object.String()] = struct{}{}
		case TableDropped:
			dropped[d.Object.String()] = struct{}{}
		}
	}
	// Fast path: nothing to suppress.
	if len(added) == 0 && len(dropped) == 0 {
		return diffs
	}
	// Filter: drop redundant child findings into a fresh slice.
	out := make([]Difference, 0, len(diffs))
	for _, d := range diffs {
		switch d.Type {
		case ColumnAdded:
			if _, ok := added[d.Object.String()]; ok {
				continue // suppress: parent table is wholly added
			}
		case ColumnDropped:
			if _, ok := dropped[d.Object.String()]; ok {
				continue // suppress: parent table is wholly dropped
			}
		}
		out = append(out, d)
	}
	return out
}

// sortDifferences sorts a slice of Difference values in place by a deterministic
// key: Schema → Name → SubObject → Type → Property.
func sortDifferences(diffs []Difference) {
	sort.Slice(diffs, func(i, j int) bool {
		a, b := diffs[i], diffs[j]
		if a.Object.Schema != b.Object.Schema {
			return a.Object.Schema < b.Object.Schema
		}
		if a.Object.Name != b.Object.Name {
			return a.Object.Name < b.Object.Name
		}
		if a.SubObject != b.SubObject {
			return a.SubObject < b.SubObject
		}
		if a.Type != b.Type {
			return string(a.Type) < string(b.Type)
		}
		return a.Property < b.Property
	})
}

// objectRefSetEqual returns true when two []ObjectRef slices have the same members
// regardless of order. Nil and empty slices are treated as equal.
func objectRefSetEqual(a, b []schemasnapshot.ObjectRef) bool {
	if len(a) != len(b) {
		return false
	}
	set := make(map[schemasnapshot.ObjectRef]int, len(a))
	for _, r := range a {
		set[r]++
	}
	for _, r := range b {
		set[r]--
		if set[r] < 0 {
			return false
		}
	}
	return true
}

// partitionParentEqual compares two *ObjectRef values for equality.
// Both nil → equal; one nil → not equal; both set → compare by value.
func partitionParentEqual(a, b *schemasnapshot.ObjectRef) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// diffTables computes table-level differences between snapshots a and b.
// Tables are matched by ID (OID). Tables with empty ID fall back to matching
// by ObjectRef.String() (schema.name).
func diffTables(a, b *schemasnapshot.SchemaSnapshot) []Difference {
	var diffs []Difference

	// Build maps: ID → Table for tables with non-empty ID.
	// For tables with empty ID, use schema.name as key.
	byIDA := make(map[string]schemasnapshot.Table)
	byIDB := make(map[string]schemasnapshot.Table)
	byNameA := make(map[string]schemasnapshot.Table)
	byNameB := make(map[string]schemasnapshot.Table)

	for _, t := range a.Tables {
		if t.ID != "" {
			byIDA[t.ID] = t
		} else {
			byNameA[t.ObjectRef.String()] = t
		}
	}
	for _, t := range b.Tables {
		if t.ID != "" {
			byIDB[t.ID] = t
		} else {
			byNameB[t.ObjectRef.String()] = t
		}
	}

	// Diff ID-matched tables.
	for id, tA := range byIDA {
		tB, ok := byIDB[id]
		if !ok {
			// Only in A → dropped
			objRef := tA.ObjectRef
			diffs = append(diffs, Difference{
				Type:        TableDropped,
				Object:      objRef,
				AnchorTable: &objRef,
			})
			continue
		}
		diffs = append(diffs, compareMatchedTables(tA, tB)...)
	}
	for id, tB := range byIDB {
		if _, ok := byIDA[id]; !ok {
			// Only in B → added
			objRef := tB.ObjectRef
			diffs = append(diffs, Difference{
				Type:        TableAdded,
				Object:      objRef,
				AnchorTable: &objRef,
			})
		}
	}

	// Diff name-matched tables (ID-empty fallback).
	for name, tA := range byNameA {
		tB, ok := byNameB[name]
		if !ok {
			// Only in A → dropped
			objRef := tA.ObjectRef
			diffs = append(diffs, Difference{
				Type:        TableDropped,
				Object:      objRef,
				AnchorTable: &objRef,
			})
			continue
		}
		diffs = append(diffs, compareMatchedTables(tA, tB)...)
	}
	for name, tB := range byNameB {
		if _, ok := byNameA[name]; !ok {
			// Only in B → added
			objRef := tB.ObjectRef
			diffs = append(diffs, Difference{
				Type:        TableAdded,
				Object:      objRef,
				AnchorTable: &objRef,
			})
		}
	}

	return diffs
}

// diffColumns computes column-level differences between snapshots a and b.
// Columns are matched by ID when non-empty; empty-ID columns fall back to
// matching by the composite key Column.Table.String() + "." + Column.Name.
func diffColumns(a, b *schemasnapshot.SchemaSnapshot) []Difference {
	var diffs []Difference

	// Build maps: ID → Column (non-empty ID) and compositeKey → Column (empty ID).
	byIDA := make(map[string]schemasnapshot.Column)
	byIDB := make(map[string]schemasnapshot.Column)
	byNameA := make(map[string]schemasnapshot.Column)
	byNameB := make(map[string]schemasnapshot.Column)

	for _, c := range a.Columns {
		if c.ID != "" {
			byIDA[c.ID] = c
		} else {
			byNameA[c.Table.String()+"."+c.Name] = c
		}
	}
	for _, c := range b.Columns {
		if c.ID != "" {
			byIDB[c.ID] = c
		} else {
			byNameB[c.Table.String()+"."+c.Name] = c
		}
	}

	// Diff ID-matched columns.
	for id, cA := range byIDA {
		cB, ok := byIDB[id]
		if !ok {
			// Only in A → dropped
			objRef := cA.Table
			diffs = append(diffs, Difference{
				Type:        ColumnDropped,
				Object:      objRef,
				AnchorTable: &objRef,
				SubObject:   cA.Name,
				OldValue:    cA.DataType,
			})
			continue
		}
		diffs = append(diffs, compareMatchedColumns(cA, cB)...)
	}
	for id, cB := range byIDB {
		if _, ok := byIDA[id]; !ok {
			// Only in B → added
			objRef := cB.Table
			diffs = append(diffs, Difference{
				Type:        ColumnAdded,
				Object:      objRef,
				AnchorTable: &objRef,
				SubObject:   cB.Name,
				NewValue:    cB.DataType,
			})
		}
	}

	// Diff name-matched columns (ID-empty fallback).
	for key, cA := range byNameA {
		cB, ok := byNameB[key]
		if !ok {
			// Only in A → dropped
			objRef := cA.Table
			diffs = append(diffs, Difference{
				Type:        ColumnDropped,
				Object:      objRef,
				AnchorTable: &objRef,
				SubObject:   cA.Name,
				OldValue:    cA.DataType,
			})
			continue
		}
		diffs = append(diffs, compareMatchedColumns(cA, cB)...)
	}
	for key, cB := range byNameB {
		if _, ok := byNameA[key]; !ok {
			// Only in B → added
			objRef := cB.Table
			diffs = append(diffs, Difference{
				Type:        ColumnAdded,
				Object:      objRef,
				AnchorTable: &objRef,
				SubObject:   cB.Name,
				NewValue:    cB.DataType,
			})
		}
	}

	return diffs
}

// compareMatchedColumns emits all field-level differences for a pair of columns
// matched by ID (or composite key for the ID-empty fallback). Object in each
// Difference is always the side-A (old) Column.Table.
func compareMatchedColumns(cA, cB schemasnapshot.Column) []Difference {
	var diffs []Difference

	oldRef := cA.Table
	anchor := oldRef // copy; we take address of the copy below

	base := Difference{
		Object:      oldRef,
		AnchorTable: &anchor,
		SubObject:   cA.Name, // SubObject is the old (side-A) column name
	}

	if cA.Name != cB.Name {
		d := base
		d.Type = ColumnNameChanged
		d.Property = "name"
		d.OldValue = cA.Name
		d.NewValue = cB.Name
		diffs = append(diffs, d)
	}

	if cA.DataType != cB.DataType {
		d := base
		d.Type = ColumnTypeChanged
		d.Property = "data_type"
		d.OldValue = cA.DataType
		d.NewValue = cB.DataType
		diffs = append(diffs, d)
	}

	if cA.NotNull != cB.NotNull {
		d := base
		d.Type = ColumnNullabilityChanged
		d.Property = "not_null"
		d.OldValue = cA.NotNull
		d.NewValue = cB.NotNull
		diffs = append(diffs, d)
	}

	if cA.Default != cB.Default {
		d := base
		d.Type = ColumnDefaultChanged
		d.Property = "default"
		d.OldValue = cA.Default
		d.NewValue = cB.Default
		diffs = append(diffs, d)
	}

	// DO NOT diff Attrs (empty in v1).

	return diffs
}

// compareMatchedTables emits all field-level differences for a pair of tables
// matched by ID (or name for the ID-empty fallback). Object in each Difference
// is always the side-A (old) ObjectRef.
func compareMatchedTables(tA, tB schemasnapshot.Table) []Difference {
	var diffs []Difference

	oldRef := tA.ObjectRef
	anchor := oldRef // copy; we take address of the copy below

	base := Difference{
		Object:      oldRef,
		AnchorTable: &anchor,
	}

	if tA.Name != tB.Name {
		d := base
		d.Type = TableNameChanged
		d.Property = "name"
		d.OldValue = tA.Name
		d.NewValue = tB.Name
		diffs = append(diffs, d)
	}

	if tA.Schema != tB.Schema {
		d := base
		d.Type = TableSchemaChanged
		d.Property = "schema"
		d.OldValue = tA.Schema
		d.NewValue = tB.Schema
		diffs = append(diffs, d)
	}

	if tA.Kind != tB.Kind {
		d := base
		d.Type = TableKindChanged
		d.Property = "kind"
		d.OldValue = string(tA.Kind)
		d.NewValue = string(tB.Kind)
		diffs = append(diffs, d)
	}

	if !partitionParentEqual(tA.PartitionParent, tB.PartitionParent) {
		d := base
		d.Type = PartitionParentChanged
		d.Property = "partition_parent"
		if tA.PartitionParent != nil {
			d.OldValue = *tA.PartitionParent
		}
		if tB.PartitionParent != nil {
			d.NewValue = *tB.PartitionParent
		}
		diffs = append(diffs, d)
	}

	if !objectRefSetEqual(tA.PartitionChildren, tB.PartitionChildren) {
		d := base
		d.Type = PartitionChildrenChanged
		d.Property = "partition_children"
		d.OldValue = tA.PartitionChildren
		d.NewValue = tB.PartitionChildren
		diffs = append(diffs, d)
	}

	if !objectRefSetEqual(tA.InheritsFrom, tB.InheritsFrom) {
		d := base
		d.Type = TableInheritsChanged
		d.Property = "inherits_from"
		d.OldValue = tA.InheritsFrom
		d.NewValue = tB.InheritsFrom
		diffs = append(diffs, d)
	}

	if !objectRefSetEqual(tA.InheritedBy, tB.InheritedBy) {
		d := base
		d.Type = TableInheritedByChanged
		d.Property = "inherited_by"
		d.OldValue = tA.InheritedBy
		d.NewValue = tB.InheritedBy
		diffs = append(diffs, d)
	}

	// DO NOT diff Attrs (empty in v1).

	return diffs
}
