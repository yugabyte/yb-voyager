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

// Difference describes a single detected schema change between two snapshots,
// where A is the old (side-A) snapshot and B is the new (side-B) snapshot. It is
// the unit element of the slice Diff returns. One shape covers added, dropped, and
// changed findings; which fields are populated depends on Type — see each field.
type Difference struct {
	// Type identifies what changed (e.g. TABLE_ADDED, COLUMN_TYPE_CHANGED). The
	// trailing verb governs which fields below are set: *_ADDED is present only on
	// B, *_DROPPED only on A, *_*CHANGED on both with Property naming the attribute.
	Type DiffType

	// Object is the (schema, name) of the object the finding is reported against.
	// For a table-level finding it is the table itself; for a column-level finding
	// it is the column's PARENT table (the column name lives in SubObject).
	// It is the side-A (old) ref for every finding EXCEPT *_ADDED, where the object
	// exists only on B and so this is the side-B (new) ref. For a rename
	// (*_NAME_CHANGED) it stays the OLD ref, with the new name carried in NewValue.
	Object schemasnapshot.ObjectRef

	// AnchorTable is the table this finding is scoped under by --table-list /
	// --exclude-table-list (consumed by FilterByScope). For the table- and
	// column-level findings v1 emits it points at the same table as Object. It is
	// nil for findings anchored to no table — none in v1, reserved for future
	// top-level objects (views, functions, sequences, types). A pointer so that
	// "no anchor" (nil) is distinguishable from the zero ObjectRef.
	AnchorTable *schemasnapshot.ObjectRef

	// SubObject names the dependent object within Object that the finding concerns.
	// In v1 this is a column name (e.g. "status") for column findings, and "" for
	// table-level findings. (Future: constraint name, index column, enum value,
	// type attribute.)
	SubObject string

	// Property names the single attribute that changed. Set ONLY for *_CHANGED
	// findings; "" for object-level findings (*_ADDED / *_DROPPED). v1 values:
	// "name", "schema", "kind" (table); "name", "data_type", "not_null", "default"
	// (column); "partition_parent", "partition_children", "inherits_from",
	// "inherited_by" (table links).
	Property string

	// OldValue is the value on side A, or nil when the object/attribute is absent
	// on A (always nil for *_ADDED). For *_CHANGED it is the previous value of
	// Property; for COLUMN_DROPPED it is the dropped column's data type (string),
	// while table-level drops leave it nil. Its dynamic type tracks Property:
	// string for name/schema/kind/data_type/default, bool for not_null,
	// schemasnapshot.ObjectRef for partition_parent, and []schemasnapshot.ObjectRef
	// for partition_children / inherits_from / inherited_by.
	OldValue any

	// NewValue is the value on side B, or nil when the object/attribute is absent
	// on B (always nil for *_DROPPED). For *_CHANGED it is the new value of
	// Property; for COLUMN_ADDED it is the added column's data type (string), while
	// table-level adds leave it nil. Same dynamic-type rules as OldValue.
	NewValue any

	// Details is an optional human-readable summary for renderers/reporters. It is
	// NOT the value channel — consumers read OldValue/NewValue for the structured
	// values. The diff engine leaves it empty; it is a slot for downstream use.
	Details string
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
