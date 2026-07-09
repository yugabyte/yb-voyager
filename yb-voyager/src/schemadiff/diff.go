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

// QualifiedObject is the object a Difference is about, pre-rendered at diff time
// (Difference is ephemeral, never serialized). Key is the unquoted canonical name
// (ForKey) used for sorting/internal keys; Display is the minimally-quoted,
// case-correct name (ForDisplay) for user-facing reports.
type QualifiedObject struct {
	Key     string
	Display string
}

// Difference describes a single detected schema change between two snapshots,
// where A is the old (side-A) snapshot and B is the new (side-B) snapshot. It is
// the unit element of the slice Diff returns. One shape covers added, dropped, and
// changed findings; which fields are populated depends on Type — see each field.
type Difference struct {
	// Type identifies what changed (e.g. TABLE_ADDED, COLUMN_TYPE_CHANGED). The
	// trailing verb governs which fields are set: *_ADDED uses NewValue, *_DROPPED
	// uses OldValue, *_CHANGED uses both.
	Type DiffType

	// Object is the rendered identity of the changed object: the table itself for
	// a table-level finding, or schema.table.column for a column-level finding.
	// It is rendered from the old (side-A) identity, except *_ADDED which uses the
	// new (side-B) identity; a rename keeps the old identity with the new name in
	// NewValue.
	Object QualifiedObject

	// AnchorTable is the table this finding filters under (--table-list /
	// --exclude-table-list) — nil for top-level findings not yet emitted
	// (VIEW_*/FUNCTION_*/TYPE_*, in the future).
	AnchorTable *schemasnapshot.ObjectRef

	// OldValue is the value on side A (nil for *_ADDED). For *_CHANGED it is the
	// previous value of the changed attribute; for COLUMN_DROPPED the whole
	// dropped Column; for TABLE_DROPPED the dropped table's []Column. Its dynamic
	// type depends on Type (string, bool, ObjectRef, []ObjectRef,
	// schemasnapshot.Column, or []schemasnapshot.Column).
	OldValue any

	// NewValue is the value on side B (nil for *_DROPPED). For *_CHANGED it is the
	// new value of the changed attribute; for COLUMN_ADDED the whole added Column;
	// for TABLE_ADDED the added table's []Column. Same dynamic-type rules as OldValue.
	NewValue any
}

// Diff computes the schema differences between snapshot a (old/side-A) and b (new/side-B).
// It returns a sorted slice of Difference values.
func Diff(a, b *schemasnapshot.SnapshotContent) []Difference {
	diffs := diffTables(a, b)
	sortDifferences(diffs)
	return diffs
}

// sortDifferences sorts a slice of Difference values in place by a deterministic
// key: sorted by Object.Key, then Type. Object.Key groups columns under their
// table because e.g. "public.orders" < "public.orders.email".
func sortDifferences(diffs []Difference) {
	sort.Slice(diffs, func(i, j int) bool {
		a, b := diffs[i], diffs[j]
		if a.Object.Key != b.Object.Key {
			return a.Object.Key < b.Object.Key
		}
		return string(a.Type) < string(b.Type)
	})
}
