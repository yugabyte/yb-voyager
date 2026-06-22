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
