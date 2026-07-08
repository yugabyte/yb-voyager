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

// diffTables computes table-level differences between snapshots a and b using a
// hybrid two-pass match:
//
//  1. ID pass — tables with a usable stable ID (OID) are matched by ID. Enabled
//     only when a.DatabaseType == b.DatabaseType, since IDs are comparable only
//     within the same engine. Same engine ⇒ match by ID, which lets a rename
//     surface as TABLE_NAME_CHANGED instead of an add+drop pair; different
//     engine ⇒ fall back to name matching.
//  2. Name pass — tables left unmatched by the ID pass (no usable ID, or an ID
//     present on only one side) are reconciled by ObjectRef.ForKey(dbType), the
//     case-sensitive, unquoted dot-joined canonical key (see diffTablesByName,
//     and ForKey's own doc for its dot-collision limitation).
//
// Falling through to the name pass instead of declaring ID-unmatched tables
// dropped/added immediately avoids a spurious drop+add when an ID is missing on
// one side. The name pass guards the inverse case: two same-named tables with
// real but DIFFERENT IDs are a genuine drop-and-recreate and stay an add+drop
// (see nameMatchAllowed).
func diffTables(a, b *schemasnapshot.SnapshotContent) []Difference {
	var diffs []Difference

	// Same engine ⇒ IDs are stable/comparable; match by ID. Different engine ⇒
	// fall back to name matching.
	matchByID := a.DatabaseType == b.DatabaseType

	// Pass 1: ID-based matching for tables that carry a usable stable ID.
	// Tables without one start life in the name-pass residue.
	byIDA := make(map[string]schemasnapshot.Table)
	byIDB := make(map[string]schemasnapshot.Table)
	var residueA, residueB []schemasnapshot.Table

	for _, t := range a.Tables {
		if matchByID && t.ID != "" {
			byIDA[t.ID] = t
		} else {
			residueA = append(residueA, t)
		}
	}
	for _, t := range b.Tables {
		if matchByID && t.ID != "" {
			byIDB[t.ID] = t
		} else {
			residueB = append(residueB, t)
		}
	}

	for id, tA := range byIDA {
		if tB, ok := byIDB[id]; ok {
			diffs = append(diffs, compareMatchedTables(tA, tB)...)
		} else {
			residueA = append(residueA, tA) // unmatched by ID → reconcile by name
		}
	}
	for id, tB := range byIDB {
		if _, ok := byIDA[id]; !ok {
			residueB = append(residueB, tB) // unmatched by ID → reconcile by name
		}
	}

	// Group each snapshot's columns by their parent table's canonical key, so
	// TABLE_ADDED/TABLE_DROPPED findings can carry the added/dropped table's
	// columns. Iterate Columns in order and append so each table's columns
	// preserve their original (attnum) order.
	colsByTableA := make(map[string][]schemasnapshot.Column)
	for _, col := range a.Columns {
		key := col.Table.ForKey(a.DatabaseType)
		colsByTableA[key] = append(colsByTableA[key], col)
	}
	colsByTableB := make(map[string][]schemasnapshot.Column)
	for _, col := range b.Columns {
		key := col.Table.ForKey(b.DatabaseType)
		colsByTableB[key] = append(colsByTableB[key], col)
	}

	// Pass 2: name-based reconciliation of the residue.
	diffs = append(diffs, diffTablesByName(residueA, residueB, matchByID, a.DatabaseType, b.DatabaseType, colsByTableA, colsByTableB)...)

	return diffs
}

// diffTablesByName reconciles the tables left unmatched by the ID pass, keyed on
// the case-sensitive, unquoted dot-joined canonical key returned by
// ObjectRef.ForKey(dbType) (unique within a snapshot in practice; see ForKey's
// doc for its dot-collision limitation). A same-named pair is treated as the
// same table only when nameMatchAllowed permits it; otherwise the A-side is
// dropped and the B-side added.
//
// colsByTableA and colsByTableB map a table's ForKey(dbType) to its columns (in
// original snapshot order) and are used to attach the added/dropped table's
// columns onto the TABLE_ADDED/TABLE_DROPPED finding.
func diffTablesByName(residueA, residueB []schemasnapshot.Table, matchByID bool, dbTypeA, dbTypeB string, colsByTableA, colsByTableB map[string][]schemasnapshot.Column) []Difference {
	var diffs []Difference

	// identity seam — the single point where a table's name-match key is derived.
	// Today: ForKey (case-sensitive, unquoted dot-joined; see its doc for the
	// dot-collision limitation). When cross-engine diffing lands, this becomes a
	// NameRegistry handle — change only here; the match loop stays generic over
	// the key string.
	keyA := func(t schemasnapshot.Table) string { return t.ForKey(dbTypeA) }
	keyB := func(t schemasnapshot.Table) string { return t.ForKey(dbTypeB) }

	byNameA := make(map[string]schemasnapshot.Table, len(residueA))
	byNameB := make(map[string]schemasnapshot.Table, len(residueB))
	for _, t := range residueA {
		byNameA[keyA(t)] = t
	}
	for _, t := range residueB {
		byNameB[keyB(t)] = t
	}

	matchedB := make(map[string]bool, len(byNameB))
	for name, tA := range byNameA {
		if tB, ok := byNameB[name]; ok && nameMatchAllowed(matchByID, tA.ID, tB.ID) {
			diffs = append(diffs, compareMatchedTables(tA, tB)...)
			matchedB[name] = true
			continue
		}
		// Only in A (or a name collision we must not collapse) → dropped.
		diffs = append(diffs, newDifference(TableDropped, tA.ObjectRef, &tA.ObjectRef, "", cloneColumns(colsByTableA[tA.ForKey(dbTypeA)]), nil))
	}
	for name, tB := range byNameB {
		if matchedB[name] {
			continue
		}
		// Only in B (or the un-collapsed half of a drop-and-recreate) → added.
		diffs = append(diffs, newDifference(TableAdded, tB.ObjectRef, &tB.ObjectRef, "", nil, cloneColumns(colsByTableB[tB.ForKey(dbTypeB)])))
	}

	return diffs
}

// nameMatchAllowed reports whether two same-named residue objects may be treated
// as the same object during the name pass.
//
//   - ID matching off (different DatabaseType, or no stable identity): name is
//     the only signal, so any same-named pair matches.
//   - ID matching on: a residue pair is the same object only if at least one
//     side lacked a usable ID. If BOTH sides carry real, distinct IDs that
//     simply didn't match, they're genuinely different objects — a
//     drop-and-recreate that reused the name — and stay an add + drop.
func nameMatchAllowed(matchByID bool, idA, idB string) bool {
	return !matchByID || idA == "" || idB == ""
}

// compareMatchedTables emits all field-level differences for a pair of tables
// matched by ID (or name for the ID-empty fallback). Object in each Difference
// is always the side-A (old) ObjectRef.
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
