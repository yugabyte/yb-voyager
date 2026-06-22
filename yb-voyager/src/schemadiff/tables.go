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
