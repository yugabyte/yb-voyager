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
	"cmp"
	"slices"

	"github.com/samber/lo"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
)

// diffTables computes table-level differences between snapshots a and b. Tables
// are matched by chooseMatchKeys: by stable OID for a same-engine Postgres/
// YugabyteDB diff so a rename surfaces as TABLE_NAME_CHANGED, else by the unquoted
// dot-joined canonical key (see ObjectRef.ForKey and its dot-collision limitation).
// Unmatched side-A tables become TABLE_DROPPED and unmatched side-B tables
// TABLE_ADDED, each carrying the table's columns as payload.
func diffTables(a, b *schemasnapshot.SnapshotContent) []Difference {
	keyA, keyB := chooseMatchKeys(a.DatabaseType, b.DatabaseType,
		idOfTable, schemasnapshot.Table.ForKey)

	return matchByKey(a.Tables, b.Tables, keyA, keyB,
		compareMatchedTables(a.DatabaseType, b.DatabaseType, linkKeyFunc(a, keyA), linkKeyFunc(b, keyB)),
		emitTableDropped, emitTableAdded)
}

// linkKeyFunc keys a partition or inheritance link the way tables are matched, so
// a renamed or moved parent or child is still the same link on both sides. A link
// to a table outside the snapshot has no ID, so it is keyed by name.
func linkKeyFunc(s *schemasnapshot.SnapshotContent, tableKey func(schemasnapshot.Table) string) func(schemasnapshot.ObjectRef) string {
	keyByRef := make(map[schemasnapshot.ObjectRef]string, len(s.Tables))
	for _, t := range s.Tables {
		keyByRef[t.ObjectRef] = tableKey(t)
	}
	return func(r schemasnapshot.ObjectRef) string {
		if k, ok := keyByRef[r]; ok {
			return k
		}
		return r.ForKey(s.DatabaseType)
	}
}

// idOfTable extracts a table's stable ID (OID) for ID-based matching.
func idOfTable(t schemasnapshot.Table) string { return t.ID }

// emitTableDropped is the matchByKey onDropped callback for tables: it emits a
// TABLE_DROPPED finding carrying the whole dropped Table (columns plus kind,
// partitioning, and inheritance metadata) as SideAValue.
func emitTableDropped(t schemasnapshot.Table) []Difference {
	return []Difference{newDifference(TableDropped, OpDropped, ObjectTypeTable, AttrNone, t.ObjectRef, nil, cloneTable(t), nil)}
}

// emitTableAdded is the matchByKey onAdded callback for tables: it emits a
// TABLE_ADDED finding carrying the whole added Table (columns plus kind,
// partitioning, and inheritance metadata) as SideBValue.
func emitTableAdded(t schemasnapshot.Table) []Difference {
	return []Difference{newDifference(TableAdded, OpAdded, ObjectTypeTable, AttrNone, nil, t.ObjectRef, nil, cloneTable(t))}
}

// compareMatchedTables returns the matchByKey onMatch callback for tables: it
// emits the table's field-level differences and then diffs its nested columns.
// dbTypeA/dbTypeB select ID- vs name-based column matching (see diffColumnsIn).
func compareMatchedTables(dbTypeA, dbTypeB string, linkKeyA, linkKeyB func(schemasnapshot.ObjectRef) string) func(tA, tB schemasnapshot.Table) []Difference {
	return func(tA, tB schemasnapshot.Table) []Difference {
		var diffs []Difference

		if tA.Name != tB.Name {
			diffs = append(diffs, newDifference(TableNameChanged, OpChanged, ObjectTypeTable, AttrName, tA.ObjectRef, tB.ObjectRef, tA.Name, tB.Name))
		}

		if tA.Schema != tB.Schema {
			diffs = append(diffs, newDifference(TableSchemaChanged, OpChanged, ObjectTypeTable, AttrSchema, tA.ObjectRef, tB.ObjectRef, tA.Schema, tB.Schema))
		}

		if tA.Kind != tB.Kind {
			diffs = append(diffs, newDifference(TableKindChanged, OpChanged, ObjectTypeTable, AttrKind, tA.ObjectRef, tB.ObjectRef, string(tA.Kind), string(tB.Kind)))
		}

		if !partitionParentEqual(tA.PartitionParent, tB.PartitionParent, linkKeyA, linkKeyB) {
			if tA.PartitionParent != nil {
				diffs = append(diffs, newDifference(TablePartitionDetached, OpChanged, ObjectTypeTable, AttrPartitionParent, tA.ObjectRef, tB.ObjectRef, *tA.PartitionParent, nil))
			}
			if tB.PartitionParent != nil {
				diffs = append(diffs, newDifference(TablePartitionAttached, OpChanged, ObjectTypeTable, AttrPartitionParent, tA.ObjectRef, tB.ObjectRef, nil, *tB.PartitionParent))
			}
		}

		if !linkSetEqual(tA.PartitionChildren, tB.PartitionChildren, linkKeyA, linkKeyB) {
			diffs = append(diffs, newDifference(TablePartitionChildrenChanged, OpChanged, ObjectTypeTable, AttrPartitionChildren, tA.ObjectRef, tB.ObjectRef, cloneObjectRefs(tA.PartitionChildren), cloneObjectRefs(tB.PartitionChildren)))
		}

		diffs = append(diffs, diffLinkSet(tA, tB, tA.InheritsFrom, tB.InheritsFrom, linkKeyA, linkKeyB, TableInheritAdded, TableInheritRemoved, AttrInherits)...)
		diffs = append(diffs, diffLinkSet(tA, tB, tA.InheritedBy, tB.InheritedBy, linkKeyA, linkKeyB, TableInheritedByAdded, TableInheritedByRemoved, AttrInheritedBy)...)

		diffs = append(diffs, diffColumnsIn(tA.Columns, tB.Columns, dbTypeA, dbTypeB)...)

		return diffs
	}
}

func diffLinkSet(tA, tB schemasnapshot.Table, linksA, linksB []schemasnapshot.ObjectRef, linkKeyA, linkKeyB func(schemasnapshot.ObjectRef) string, addedType, removedType DiffType, attr Attribute) []Difference {
	keysA := lo.SliceToMap(linksA, func(r schemasnapshot.ObjectRef) (string, bool) { return linkKeyA(r), true })
	keysB := lo.SliceToMap(linksB, func(r schemasnapshot.ObjectRef) (string, bool) { return linkKeyB(r), true })
	removed := lo.Filter(linksA, func(r schemasnapshot.ObjectRef, _ int) bool { return !keysB[linkKeyA(r)] })
	added := lo.Filter(linksB, func(r schemasnapshot.ObjectRef, _ int) bool { return !keysA[linkKeyB(r)] })
	slices.SortFunc(removed, compareObjectRefs)
	slices.SortFunc(added, compareObjectRefs)

	var diffs []Difference
	if len(removed) > 0 {
		diffs = append(diffs, newDifference(removedType, OpChanged, ObjectTypeTable, attr, tA.ObjectRef, tB.ObjectRef, removed, nil))
	}
	if len(added) > 0 {
		diffs = append(diffs, newDifference(addedType, OpChanged, ObjectTypeTable, attr, tA.ObjectRef, tB.ObjectRef, nil, added))
	}
	return diffs
}

func compareObjectRefs(a, b schemasnapshot.ObjectRef) int {
	return cmp.Or(cmp.Compare(a.Schema, b.Schema), cmp.Compare(a.Name, b.Name))
}

// partitionParentEqual compares two *ObjectRef values by link key.
// Both nil → equal; one nil → not equal; both set → compare keys.
func partitionParentEqual(a, b *schemasnapshot.ObjectRef, linkKeyA, linkKeyB func(schemasnapshot.ObjectRef) string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return linkKeyA(*a) == linkKeyB(*b)
}

// linkSetEqual returns true when two []ObjectRef slices have the same members by
// link key, regardless of order. Nil and empty slices are treated as equal.
func linkSetEqual(a, b []schemasnapshot.ObjectRef, linkKeyA, linkKeyB func(schemasnapshot.ObjectRef) string) bool {
	if len(a) != len(b) {
		return false
	}
	set := make(map[string]int, len(a))
	for _, r := range a {
		set[linkKeyA(r)]++
	}
	for _, r := range b {
		k := linkKeyB(r)
		set[k]--
		if set[k] < 0 {
			return false
		}
	}
	return true
}

// cloneObjectRefs returns a fresh copy of an []ObjectRef so a Difference never
// shares backing storage with the input snapshot — a shallow copy fully decouples
// it since ObjectRef is a value type (nil input stays nil). Without this, a
// consumer mutating a TABLE_PARTITION_CHILDREN_CHANGED finding's OldValue/NewValue slice would write through
// into the source snapshot.
func cloneObjectRefs(refs []schemasnapshot.ObjectRef) []schemasnapshot.ObjectRef {
	if refs == nil {
		return nil
	}
	out := make([]schemasnapshot.ObjectRef, len(refs))
	copy(out, refs)
	return out
}

// cloneColumns returns a fresh copy of a []Column so a Difference never shares
// backing storage with the input snapshot — a shallow copy fully decouples it
// since Column is a value type (nil input stays nil). Without this, a consumer
// mutating a TABLE_ADDED/TABLE_DROPPED finding's OldValue/NewValue slice would
// write through into the source snapshot.
func cloneColumns(cols []schemasnapshot.Column) []schemasnapshot.Column {
	if cols == nil {
		return nil
	}
	out := make([]schemasnapshot.Column, len(cols))
	copy(out, cols)
	return out
}

// cloneTable returns a copy of t whose every slice/pointer field is deep-copied,
// so a TABLE_ADDED/TABLE_DROPPED finding carrying the whole Table never shares
// backing storage with the input snapshot. t is received by value, so the scalar
// and embedded ObjectRef fields are already copied; only the reference-typed
// fields (Columns, the ObjectRef link slices, and the PartitionParent pointer)
// need decoupling. Without this, a consumer mutating the finding's Table would
// write through into the source snapshot.
func cloneTable(t schemasnapshot.Table) schemasnapshot.Table {
	t.Columns = cloneColumns(t.Columns)
	t.PartitionChildren = cloneObjectRefs(t.PartitionChildren)
	t.InheritsFrom = cloneObjectRefs(t.InheritsFrom)
	t.InheritedBy = cloneObjectRefs(t.InheritedBy)
	if t.PartitionParent != nil {
		parent := *t.PartitionParent
		t.PartitionParent = &parent
	}
	return t
}
