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
//  1. ID pass — columns carrying a usable stable ID are matched by ID. This is
//     enabled only when a.DatabaseType == b.DatabaseType — IDs (e.g. PG OIDs) are
//     only comparable within the same database engine; cross-type ID comparison is
//     illegal. Same engine ⇒ IDs are stable and comparable, so match by ID to detect
//     renames; different engine ⇒ IDs aren't comparable, fall back to name matching.
//     ID matching is what lets a rename surface as COLUMN_NAME_CHANGED rather than
//     an add+drop pair.
//  2. Name pass — every column left unmatched by the ID pass (columns with no
//     usable ID, PLUS any whose ID was present on one side but absent on the
//     other) is reconciled by the collision-safe composite key
//     Column.ForKey(dbType).
//
// Letting ID-unmatched columns fall through to the name pass — instead of
// declaring them dropped/added immediately — is what keeps a column whose ID is
// present on one side but missing on the other from surfacing as a spurious
// drop+add. The name pass guards against the inverse mistake: two same-named
// columns that each carry a real but DIFFERENT ID are a genuine drop-and-recreate
// and stay an add+drop rather than collapsing into one match (see nameMatchAllowed).
func diffColumns(a, b *schemasnapshot.SnapshotContent) []Difference {
	var diffs []Difference

	// Same engine ⇒ IDs (e.g. PG OIDs) are stable and comparable, so match by ID
	// to detect renames; different engine ⇒ IDs aren't comparable, fall back to name matching.
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
	// derived. Today: the case-sensitive, collision-safe per-part-quoted
	// composite key (ForKey). When cross-engine diffing lands, this becomes a
	// NameRegistry-canonicalized handle — change only here; the match loop
	// stays generic over the key string.
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
		diffs = append(diffs, newDifference(ColumnDropped, cA.Table, &cA.Table, cA.Name, cA.DataType, nil))
	}
	for key, cB := range byNameB {
		if matchedB[key] {
			continue
		}
		// Only in B (or the un-collapsed half of a drop-and-recreate) → added.
		diffs = append(diffs, newDifference(ColumnAdded, cB.Table, &cB.Table, cB.Name, nil, cB.DataType))
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
