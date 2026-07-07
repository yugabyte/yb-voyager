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

// diffColumns computes column-level differences between snapshots a and b using a
// hybrid two-pass match keyed on Column.ForKey(dbType), the collision-safe,
// case-sensitive, per-part-quoted composite key "schema"."table"."col":
//
//  1. ID pass — columns with a usable stable ID are matched by ID. Enabled only
//     when a.DatabaseType == b.DatabaseType, since IDs are comparable only
//     within the same engine. Same engine ⇒ match by ID, which lets a rename
//     surface as COLUMN_NAME_CHANGED instead of an add+drop pair; different
//     engine ⇒ fall back to name matching.
//  2. Name pass — columns left unmatched by the ID pass (no usable ID, or an ID
//     present on only one side) are reconciled by the composite key
//     Column.ForKey(dbType).
//
// Falling through to the name pass instead of declaring ID-unmatched columns
// dropped/added immediately avoids a spurious drop+add when an ID is missing on
// one side. The name pass guards the inverse case: two same-named columns with
// real but DIFFERENT IDs are a genuine drop-and-recreate and stay an add+drop
// (see nameMatchAllowed).
func diffColumns(a, b *schemasnapshot.SnapshotContent) []Difference {
	var diffs []Difference

	// Same engine ⇒ IDs are stable/comparable; match by ID. Different engine ⇒
	// fall back to name matching.
	matchByID := a.DatabaseType == b.DatabaseType

	// Pass 1: ID-based matching for columns that carry a usable stable ID.
	// Columns without one start life in the name-pass residue.
	byIDA := make(map[string]schemasnapshot.Column)
	byIDB := make(map[string]schemasnapshot.Column)
	var residueA, residueB []schemasnapshot.Column

	for _, c := range a.Columns {
		if matchByID && c.ID != "" {
			byIDA[c.ID] = c
		} else {
			residueA = append(residueA, c)
		}
	}
	for _, c := range b.Columns {
		if matchByID && c.ID != "" {
			byIDB[c.ID] = c
		} else {
			residueB = append(residueB, c)
		}
	}

	for id, cA := range byIDA {
		if cB, ok := byIDB[id]; ok {
			diffs = append(diffs, compareMatchedColumns(cA, cB)...)
		} else {
			residueA = append(residueA, cA) // unmatched by ID → reconcile by name
		}
	}
	for id, cB := range byIDB {
		if _, ok := byIDA[id]; !ok {
			residueB = append(residueB, cB) // unmatched by ID → reconcile by name
		}
	}

	// Pass 2: name-based reconciliation of the residue.
	diffs = append(diffs, diffColumnsByName(residueA, residueB, matchByID, a.DatabaseType, b.DatabaseType)...)

	return diffs
}

// diffColumnsByName reconciles the columns left unmatched by the ID pass, keyed
// on the collision-safe composite key Column.ForKey(dbType) (unique within a
// snapshot). A same-named pair is treated as the same column only when
// nameMatchAllowed permits it; otherwise the A-side is dropped and the B-side
// added.
func diffColumnsByName(residueA, residueB []schemasnapshot.Column, matchByID bool, dbTypeA, dbTypeB string) []Difference {
	var diffs []Difference

	// identity seam — the single point where a column's name-match key is
	// derived. Today: ForKey (case-sensitive, collision-safe, per-part-quoted
	// composite key). When cross-engine diffing lands, this becomes a
	// NameRegistry handle — change only here; the match loop stays generic
	// over the key string.
	keyA := func(c schemasnapshot.Column) string { return c.ForKey(dbTypeA) }
	keyB := func(c schemasnapshot.Column) string { return c.ForKey(dbTypeB) }

	byNameA := make(map[string]schemasnapshot.Column, len(residueA))
	byNameB := make(map[string]schemasnapshot.Column, len(residueB))
	for _, c := range residueA {
		byNameA[keyA(c)] = c
	}
	for _, c := range residueB {
		byNameB[keyB(c)] = c
	}

	matchedB := make(map[string]bool, len(byNameB))
	for key, cA := range byNameA {
		if cB, ok := byNameB[key]; ok && nameMatchAllowed(matchByID, cA.ID, cB.ID) {
			diffs = append(diffs, compareMatchedColumns(cA, cB)...)
			matchedB[key] = true
			continue
		}
		// Only in A (or a name collision we must not collapse) → dropped.
		diffs = append(diffs, newDifference(ColumnDropped, cA.Table, &cA.Table, cA.Name, cA, nil))
	}
	for key, cB := range byNameB {
		if matchedB[key] {
			continue
		}
		// Only in B (or the un-collapsed half of a drop-and-recreate) → added.
		diffs = append(diffs, newDifference(ColumnAdded, cB.Table, &cB.Table, cB.Name, nil, cB))
	}

	return diffs
}

// compareMatchedColumns emits all field-level differences for a pair of columns
// matched by ID (or composite key for the ID-empty fallback). Object in each
// Difference is always the side-A (old) Column.Table.
func compareMatchedColumns(cA, cB schemasnapshot.Column) []Difference {
	var diffs []Difference

	if cA.Name != cB.Name {
		diffs = append(diffs, newDifference(ColumnNameChanged, cA.Table, &cA.Table, cA.Name, cA.Name, cB.Name))
	}

	if cA.DataType != cB.DataType {
		diffs = append(diffs, newDifference(ColumnTypeChanged, cA.Table, &cA.Table, cA.Name, cA.DataType, cB.DataType))
	}

	if cA.NotNull != cB.NotNull {
		diffs = append(diffs, newDifference(ColumnNullabilityChanged, cA.Table, &cA.Table, cA.Name, cA.NotNull, cB.NotNull))
	}

	if cA.Default != cB.Default {
		diffs = append(diffs, newDifference(ColumnDefaultChanged, cA.Table, &cA.Table, cA.Name, cA.Default, cB.Default))
	}

	return diffs
}
