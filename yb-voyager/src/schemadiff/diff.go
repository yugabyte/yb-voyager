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

// ObjectIdent is the identity of the object a Difference is about. Both
// schemasnapshot.ObjectRef (schema-level objects: tables, views, …) and
// schemasnapshot.TableScopedRef (table-scoped objects: columns, indexes, …)
// implement it, so ObjectA/ObjectB can hold either. Rendering is deferred and
// engine-aware — call ForKey/ForDisplay with the side's dbType at report time.
type ObjectIdent interface {
	ForKey(dbType string) string
	ForDisplay(dbType string) string
}

// Difference describes a single detected schema change between two snapshots,
// where A is the old (side-A) snapshot and B is the new (side-B) snapshot. It is
// the unit element of the slice Diff returns. One shape covers added, dropped, and
// changed findings; which fields are populated depends on Type — see each field.
type Difference struct {
	// Type identifies what changed (e.g. TABLE_ADDED, COLUMN_TYPE_CHANGED). The
	// trailing verb governs which fields are set: *_ADDED uses ObjectB/SideBValue,
	// *_DROPPED uses ObjectA/SideAValue, *_CHANGED uses both. Operation, ObjectType,
	// and Attribute are Type's decomposed facets, provided so consumers can
	// group/filter without parsing this string.
	Type DiffType

	// Operation is the verb: OpAdded, OpDropped, or OpChanged.
	Operation Operation

	// ObjectType is the honest object type of the changed object (TABLE, COLUMN,
	// …). It is what --object-type-list matches against, so a column change is
	// selected by COLUMN (not TABLE).
	ObjectType ObjectType

	// Attribute names the changed attribute for OpChanged findings (NAME, TYPE,
	// NULLABILITY, …); AttrNone for OpAdded/OpDropped.
	Attribute Attribute

	// ObjectA is the side-A (old) identity of the changed object — nil for
	// *_ADDED, which has no side-A object. For a rename/move ObjectA is the OLD
	// identity and ObjectB is the NEW one; they differ.
	ObjectA ObjectIdent

	// ObjectB is the side-B (new) identity of the changed object — nil for
	// *_DROPPED, which has no side-B object. For a rename/move ObjectB is the NEW
	// identity.
	ObjectB ObjectIdent

	// SideAValue is the value on side A (nil for *_ADDED). For *_CHANGED it is the
	// previous value of the changed attribute; for COLUMN_DROPPED the whole
	// dropped Column; for TABLE_DROPPED the whole dropped Table (columns plus
	// kind/partition/inheritance metadata). Its dynamic type depends on Type
	// (string, bool, schemasnapshot.ObjectRef, []schemasnapshot.ObjectRef,
	// schemasnapshot.Column, or schemasnapshot.Table).
	SideAValue any

	// SideBValue is the value on side B (nil for *_DROPPED). For *_CHANGED it is
	// the new value of the changed attribute; for COLUMN_ADDED the whole added
	// Column; for TABLE_ADDED the whole added Table (columns plus
	// kind/partition/inheritance metadata). Same dynamic-type rules as SideAValue.
	SideBValue any
}

// Diff computes the schema differences between snapshot a (old/side-A) and b (new/side-B).
// It returns a sorted slice of Difference values.
func Diff(a, b *schemasnapshot.SnapshotContent) []Difference {
	diffs := diffTables(a, b)
	sortDifferences(diffs, a.DatabaseType, b.DatabaseType)
	return diffs
}

// sortDifferences sorts a slice of Difference values in place by a deterministic
// key: the finding's side-A identity key (side-B for *_ADDED, where ObjectA is
// nil), then Type. Grouping columns under their table still holds because
// "public.orders" < "public.orders.email".
func sortDifferences(diffs []Difference, dbTypeA, dbTypeB string) {
	keyOf := func(d Difference) string {
		if d.ObjectA != nil {
			return d.ObjectA.ForKey(dbTypeA)
		}
		return d.ObjectB.ForKey(dbTypeB)
	}
	sort.Slice(diffs, func(i, j int) bool {
		ki, kj := keyOf(diffs[i]), keyOf(diffs[j])
		if ki != kj {
			return ki < kj
		}
		return string(diffs[i].Type) < string(diffs[j].Type)
	})
}
