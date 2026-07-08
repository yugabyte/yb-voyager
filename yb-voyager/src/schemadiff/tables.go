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

// diffTables computes table-level differences between snapshots a and b. Tables
// are matched by chooseMatchKeys: by stable OID when IDs are usable (same engine,
// all present) so a rename surfaces as TABLE_NAME_CHANGED, else by the unquoted
// dot-joined canonical key (see ObjectRef.ForKey and its dot-collision limitation).
// Unmatched side-A tables become TABLE_DROPPED and unmatched side-B tables
// TABLE_ADDED, each carrying the table's columns as payload.
func diffTables(a, b *schemasnapshot.SnapshotContent) []Difference {
	// Group columns by their parent table's canonical key so a wholly added/dropped
	// table can carry its columns on the finding, in original (attnum) order.
	colsByTableA := columnsByTable(a.Columns, a.DatabaseType)
	colsByTableB := columnsByTable(b.Columns, b.DatabaseType)

	keyA, keyB := chooseMatchKeys(a.DatabaseType, b.DatabaseType, a.Tables, b.Tables,
		tableID, schemasnapshot.Table.ForKey)

	return matchByKey(a.Tables, b.Tables, keyA, keyB,
		compareMatchedTables,
		tableDropped(colsByTableA, a.DatabaseType),
		tableAdded(colsByTableB, b.DatabaseType))
}

// tableID extracts a table's stable ID (OID) for ID-based matching.
func tableID(t schemasnapshot.Table) string { return t.ID }

// tableDropped returns the matchByKey onDropped callback for tables: it emits a
// TABLE_DROPPED finding carrying the dropped table's columns (looked up from
// colsByTable, keyed by ForKey(dbType)).
func tableDropped(colsByTable map[string][]schemasnapshot.Column, dbType string) func(schemasnapshot.Table) []Difference {
	return func(t schemasnapshot.Table) []Difference {
		return []Difference{newDifference(TableDropped, t.ObjectRef, &t.ObjectRef, "", cloneColumns(colsByTable[t.ForKey(dbType)]), nil)}
	}
}

// tableAdded returns the matchByKey onAdded callback for tables: it emits a
// TABLE_ADDED finding carrying the added table's columns (looked up from
// colsByTable, keyed by ForKey(dbType)).
func tableAdded(colsByTable map[string][]schemasnapshot.Column, dbType string) func(schemasnapshot.Table) []Difference {
	return func(t schemasnapshot.Table) []Difference {
		return []Difference{newDifference(TableAdded, t.ObjectRef, &t.ObjectRef, "", nil, cloneColumns(colsByTable[t.ForKey(dbType)]))}
	}
}

// columnsByTable groups columns by their parent table's canonical key, preserving
// each table's original column (attnum) order.
func columnsByTable(cols []schemasnapshot.Column, dbType string) map[string][]schemasnapshot.Column {
	out := make(map[string][]schemasnapshot.Column)
	for _, col := range cols {
		key := col.Table.ForKey(dbType)
		out[key] = append(out[key], col)
	}
	return out
}

// compareMatchedTables emits all field-level differences for a pair of tables
// matched by chooseMatchKeys (ID or name). Object in each Difference is always
// the side-A (old) ObjectRef.
func compareMatchedTables(tA, tB schemasnapshot.Table) []Difference {
	var diffs []Difference

	if tA.Name != tB.Name {
		diffs = append(diffs, newDifference(TableNameChanged, tA.ObjectRef, &tA.ObjectRef, "", tA.Name, tB.Name))
	}

	if tA.Schema != tB.Schema {
		diffs = append(diffs, newDifference(TableSchemaChanged, tA.ObjectRef, &tA.ObjectRef, "", tA.Schema, tB.Schema))
	}

	if tA.Kind != tB.Kind {
		diffs = append(diffs, newDifference(TableKindChanged, tA.ObjectRef, &tA.ObjectRef, "", string(tA.Kind), string(tB.Kind)))
	}

	if !partitionParentEqual(tA.PartitionParent, tB.PartitionParent) {
		var ov, nv any
		if tA.PartitionParent != nil {
			ov = *tA.PartitionParent
		}
		if tB.PartitionParent != nil {
			nv = *tB.PartitionParent
		}
		diffs = append(diffs, newDifference(TablePartitionParentChanged, tA.ObjectRef, &tA.ObjectRef, "", ov, nv))
	}

	if !objectRefSetEqual(tA.PartitionChildren, tB.PartitionChildren) {
		diffs = append(diffs, newDifference(TablePartitionChildrenChanged, tA.ObjectRef, &tA.ObjectRef, "", cloneObjectRefs(tA.PartitionChildren), cloneObjectRefs(tB.PartitionChildren)))
	}

	if !objectRefSetEqual(tA.InheritsFrom, tB.InheritsFrom) {
		diffs = append(diffs, newDifference(TableInheritsChanged, tA.ObjectRef, &tA.ObjectRef, "", cloneObjectRefs(tA.InheritsFrom), cloneObjectRefs(tB.InheritsFrom)))
	}

	if !objectRefSetEqual(tA.InheritedBy, tB.InheritedBy) {
		diffs = append(diffs, newDifference(TableInheritedByChanged, tA.ObjectRef, &tA.ObjectRef, "", cloneObjectRefs(tA.InheritedBy), cloneObjectRefs(tB.InheritedBy)))
	}

	return diffs
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

// cloneObjectRefs returns a fresh copy of an []ObjectRef so a Difference never
// shares backing storage with the input snapshot — a shallow copy fully decouples
// it since ObjectRef is a value type (nil input stays nil). Without this, a
// consumer mutating a link finding's OldValue/NewValue slice would write through
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
