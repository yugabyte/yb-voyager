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

// cloneObjectRefs returns a fresh copy of an []ObjectRef so a Difference never
// shares backing storage with the input snapshot. ObjectRef is a value type, so
// a shallow copy fully decouples the result; a nil input stays nil (no needless
// allocation). Without this, a consumer mutating a link finding's OldValue /
// NewValue slice would write through into the source snapshot.
func cloneObjectRefs(refs []schemasnapshot.ObjectRef) []schemasnapshot.ObjectRef {
	if refs == nil {
		return nil
	}
	out := make([]schemasnapshot.ObjectRef, len(refs))
	copy(out, refs)
	return out
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

// diffTables computes table-level differences between snapshots a and b using a
// hybrid two-pass match:
//
//  1. ID pass — tables carrying a usable stable ID (OID) are matched by ID. This
//     is enabled only when a.DatabaseType == b.DatabaseType — IDs (e.g. PG OIDs)
//     are only comparable within the same database engine; cross-type ID comparison
//     is illegal. Same engine ⇒ IDs are stable and comparable, so match by ID to
//     detect renames; different engine ⇒ IDs aren't comparable, fall back to name
//     matching. ID matching is what lets a rename surface as TABLE_NAME_CHANGED
//     rather than an add+drop pair.
//  2. Name pass — every table left unmatched by the ID pass (tables with no
//     usable ID, PLUS any whose ID was present on one side but absent on the
//     other) is reconciled by ObjectRef.String() (schema.name).
//
// Letting ID-unmatched tables fall through to the name pass — instead of
// declaring them dropped/added immediately — is what keeps a table whose ID is
// present on one side but missing on the other from surfacing as a spurious
// drop+add. The name pass guards against the inverse mistake: two same-named
// tables that each carry a real but DIFFERENT ID are a genuine drop-and-recreate
// and stay an add+drop rather than collapsing into one match (see nameMatchAllowed).
func diffTables(a, b *schemasnapshot.SnapshotContent) []Difference {
	var diffs []Difference

	// Same engine ⇒ IDs (e.g. PG OIDs) are stable and comparable, so match by ID
	// to detect renames; different engine ⇒ IDs aren't comparable, fall back to name matching.
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

	// Pass 2: name-based reconciliation of the residue.
	diffs = append(diffs, diffTablesByName(residueA, residueB, matchByID)...)

	return diffs
}

// diffTablesByName reconciles the tables left unmatched by the ID pass, keyed on
// ObjectRef.String() (schema.name, which is unique within a snapshot). A
// same-named pair is treated as the same table only when nameMatchAllowed
// permits it; otherwise the A-side is dropped and the B-side added.
func diffTablesByName(residueA, residueB []schemasnapshot.Table, matchByID bool) []Difference {
	var diffs []Difference

	byNameA := make(map[string]schemasnapshot.Table, len(residueA))
	byNameB := make(map[string]schemasnapshot.Table, len(residueB))
	for _, t := range residueA {
		byNameA[t.ObjectRef.String()] = t
	}
	for _, t := range residueB {
		byNameB[t.ObjectRef.String()] = t
	}

	matchedB := make(map[string]bool, len(byNameB))
	for name, tA := range byNameA {
		if tB, ok := byNameB[name]; ok && nameMatchAllowed(matchByID, tA.ID, tB.ID) {
			diffs = append(diffs, compareMatchedTables(tA, tB)...)
			matchedB[name] = true
			continue
		}
		// Only in A (or a name collision we must not collapse) → dropped.
		diffs = append(diffs, newDifference(TableDropped, tA.ObjectRef, &tA.ObjectRef, "", nil, nil))
	}
	for name, tB := range byNameB {
		if matchedB[name] {
			continue
		}
		// Only in B (or the un-collapsed half of a drop-and-recreate) → added.
		diffs = append(diffs, newDifference(TableAdded, tB.ObjectRef, &tB.ObjectRef, "", nil, nil))
	}

	return diffs
}

// nameMatchAllowed reports whether two same-named residue objects may be treated
// as the same object during the name pass.
//
//   - When ID matching is off entirely (different DatabaseType, or a snapshot
//     without stable identity), name is the only signal we have, so any
//     same-named pair matches — the documented name-only fallback.
//   - When ID matching is on, a residue pair is the same object only if at least
//     one side lacked a usable ID (and so could not have been ID-matched). If
//     BOTH sides carry real, distinct IDs that simply did not match, they are
//     genuinely different objects — a drop-and-recreate that reused the name —
//     and must stay an add + drop.
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
		diffs = append(diffs, newDifference(PartitionParentChanged, tA.ObjectRef, &tA.ObjectRef, "", ov, nv))
	}

	if !objectRefSetEqual(tA.PartitionChildren, tB.PartitionChildren) {
		diffs = append(diffs, newDifference(PartitionChildrenChanged, tA.ObjectRef, &tA.ObjectRef, "", cloneObjectRefs(tA.PartitionChildren), cloneObjectRefs(tB.PartitionChildren)))
	}

	if !objectRefSetEqual(tA.InheritsFrom, tB.InheritsFrom) {
		diffs = append(diffs, newDifference(TableInheritsChanged, tA.ObjectRef, &tA.ObjectRef, "", cloneObjectRefs(tA.InheritsFrom), cloneObjectRefs(tB.InheritsFrom)))
	}

	if !objectRefSetEqual(tA.InheritedBy, tB.InheritedBy) {
		diffs = append(diffs, newDifference(TableInheritedByChanged, tA.ObjectRef, &tA.ObjectRef, "", cloneObjectRefs(tA.InheritedBy), cloneObjectRefs(tB.InheritedBy)))
	}

	return diffs
}
