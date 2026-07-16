//go:build unit

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
	"reflect"
	"strings"
	"testing"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
)

// helper: build a minimal SnapshotContent with the given tables.
// DatabaseType is set to constants.POSTGRESQL because these helpers model PG snapshots
// whose IDs (OIDs) are stable and comparable within the same engine.
func snapWithTables(tables ...schemasnapshot.Table) *schemasnapshot.SnapshotContent {
	return &schemasnapshot.SnapshotContent{
		Version:      1,
		DatabaseType: "postgresql",
		Tables:       tables,
	}
}

// helper: build a table with ID, schema, name, kind
func makeTable(id, schema, name string, kind schemasnapshot.TableKind) schemasnapshot.Table {
	return schemasnapshot.Table{
		ObjectRef: schemasnapshot.ObjectRef{Schema: schema, Name: name},
		ID:        id,
		Kind:      kind,
	}
}

// helper: ref shorthand
func ref(schema, name string) schemasnapshot.ObjectRef {
	return schemasnapshot.ObjectRef{Schema: schema, Name: name}
}

// refPtr returns a pointer to an ObjectRef
func refPtr(schema, name string) *schemasnapshot.ObjectRef {
	r := ref(schema, name)
	return &r
}

// identKey renders a finding's own identity key: side-A's key, falling back to
// side-B for *_ADDED (where ObjectA is nil), in dbType's dialect. This is the
// direct replacement for the old pre-rendered Object.Key.
func identKey(d Difference, dbType string) string {
	if d.ObjectA != nil {
		return d.ObjectA.ForKey(dbType)
	}
	return d.ObjectB.ForKey(dbType)
}

// assertAnchoredToObject asserts the V1 invariant that a produced finding is
// anchored under its own object: a table-level finding's identity key equals its
// derived anchor table's key, and a column-level finding's identity key is the
// anchor table's key plus ".<column>". This is the seam that will change only
// when index/sequence/view findings arrive (identity and anchor diverge further).
func assertAnchoredToObject(t *testing.T, d Difference) {
	t.Helper()
	anchor, ok := anchorTableOf(d)
	if !ok {
		t.Errorf("anchorTableOf failed for %v (%q); want a table anchor", d.Type, identKey(d, "postgresql"))
		return
	}
	anchorKey := anchor.ForKey("postgresql")
	objKey := identKey(d, "postgresql")
	if objKey != anchorKey && !strings.HasPrefix(objKey, anchorKey+".") {
		t.Errorf("finding identity %q not anchored under table %q", objKey, anchorKey)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: Diff of two empty snapshots returns nil/empty
// ──────────────────────────────────────────────────────────────────────────────

func TestDiff_EmptySnapshots_ReturnsNilOrEmpty(t *testing.T) {
	a := snapWithTables()
	b := snapWithTables()
	got := Diff(a, b)
	if len(got) != 0 {
		t.Errorf("expected no differences, got %d: %v", len(got), got)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: Diff of identical snapshots returns no findings
// ──────────────────────────────────────────────────────────────────────────────

func TestDiff_IdenticalSnapshots_NoFindings(t *testing.T) {
	table := makeTable("101", "public", "orders", schemasnapshot.TableKindOrdinary)
	col := makeColumn("public", "orders", "101:1", "id", "integer", notNull())
	table.Columns = []schemasnapshot.Column{col}
	a := snapWithTables(table)
	b := snapWithTables(table)

	got := Diff(a, b)
	if len(got) != 0 {
		t.Errorf("expected no differences, got %d: %v", len(got), got)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: Diff does NOT mutate inputs
// ──────────────────────────────────────────────────────────────────────────────

func TestDiff_DoesNotMutateInputs(t *testing.T) {
	tblA := makeTable("1", "public", "foo", schemasnapshot.TableKindOrdinary)
	tblB := makeTable("2", "public", "bar", schemasnapshot.TableKindOrdinary)
	a := snapWithTables(tblA)
	b := snapWithTables(tblB)

	aCopy := *a
	aCopyTables := make([]schemasnapshot.Table, len(a.Tables))
	copy(aCopyTables, a.Tables)
	aCopy.Tables = aCopyTables

	bCopy := *b
	bCopyTables := make([]schemasnapshot.Table, len(b.Tables))
	copy(bCopyTables, b.Tables)
	bCopy.Tables = bCopyTables

	Diff(a, b)

	if !reflect.DeepEqual(a.Tables, aCopy.Tables) {
		t.Errorf("snapshot A was mutated: got %v, want %v", a.Tables, aCopy.Tables)
	}
	if !reflect.DeepEqual(b.Tables, bCopy.Tables) {
		t.Errorf("snapshot B was mutated: got %v, want %v", b.Tables, bCopy.Tables)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: sortDifferences ordering
// ──────────────────────────────────────────────────────────────────────────────

func TestSortDifferences_Ordering(t *testing.T) {
	// Build diffs that exercise both sort keys (ObjectA's key, then Type). Each
	// identity below folds in what used to be a separate SubObject: a
	// column-level identity is a TableScopedObjectRef ("schema.table.column"); a
	// table-level identity is an ObjectRef ("schema.table").
	diffs := []Difference{
		// same identity key; differ only on Type
		{ObjectType: ObjectTypeColumn, ObjectA: schemasnapshot.TableScopedObjectRef{Table: ref("z_schema", "z_name"), Name: "a_sub"}, Type: TableSchemaChanged},
		{ObjectType: ObjectTypeColumn, ObjectA: schemasnapshot.TableScopedObjectRef{Table: ref("z_schema", "z_name"), Name: "a_sub"}, Type: TableAdded},
		// same schema/table; differ only on the column tail of the identity key
		{ObjectType: ObjectTypeColumn, ObjectA: schemasnapshot.TableScopedObjectRef{Table: ref("z_schema", "a_name"), Name: "z_sub"}, Type: TableAdded},
		{ObjectType: ObjectTypeColumn, ObjectA: schemasnapshot.TableScopedObjectRef{Table: ref("z_schema", "a_name"), Name: "a_sub"}, Type: TableAdded},
		// same schema; differ only on the table-name portion of the identity key
		{ObjectType: ObjectTypeTable, ObjectA: ref("a_schema", "z_name"), Type: TableAdded},
		{ObjectType: ObjectTypeTable, ObjectA: ref("a_schema", "a_name"), Type: TableAdded},
	}

	sortDifferences(diffs, "postgresql", "postgresql")

	want := []Difference{
		{ObjectType: ObjectTypeTable, ObjectA: ref("a_schema", "a_name"), Type: TableAdded},
		{ObjectType: ObjectTypeTable, ObjectA: ref("a_schema", "z_name"), Type: TableAdded},
		{ObjectType: ObjectTypeColumn, ObjectA: schemasnapshot.TableScopedObjectRef{Table: ref("z_schema", "a_name"), Name: "a_sub"}, Type: TableAdded},
		{ObjectType: ObjectTypeColumn, ObjectA: schemasnapshot.TableScopedObjectRef{Table: ref("z_schema", "a_name"), Name: "z_sub"}, Type: TableAdded},
		{ObjectType: ObjectTypeColumn, ObjectA: schemasnapshot.TableScopedObjectRef{Table: ref("z_schema", "z_name"), Name: "a_sub"}, Type: TableAdded},
		{ObjectType: ObjectTypeColumn, ObjectA: schemasnapshot.TableScopedObjectRef{Table: ref("z_schema", "z_name"), Name: "a_sub"}, Type: TableSchemaChanged},
	}

	if !reflect.DeepEqual(diffs, want) {
		t.Errorf("wrong sort order\ngot:  %v\nwant: %v", diffs, want)
	}
}

// TestDifferenceDecompositionMatchesType verifies that every finding Diff emits
// carries an (Operation, ObjectType, Attribute) triple consistent with its Type
// string, and that a single rich scenario exercises all 15 V1 DiffTypes. The
// expected facets are derived by parsing Type here — an independent oracle that
// would disagree if any newDifference call site passed the wrong op/objtype/attr.
func TestDifferenceDecompositionMatchesType(t *testing.T) {
	// A matched table (OID "1") with every table attribute and a rich set of
	// column changes, plus one dropped-only and one added-only table, so the diff
	// spans all 15 DiffTypes.
	tA := schemasnapshot.Table{
		ObjectRef:         ref("s1", "t_old"),
		ID:                "1",
		Kind:              schemasnapshot.TableKindOrdinary,
		PartitionParent:   refPtr("s1", "parent_a"),
		PartitionChildren: []schemasnapshot.ObjectRef{ref("s1", "child_a")},
		InheritsFrom:      []schemasnapshot.ObjectRef{ref("s1", "base_a")},
		InheritedBy:       []schemasnapshot.ObjectRef{ref("s1", "derived_a")},
		Columns: []schemasnapshot.Column{
			makeColumn("s1", "t_old", "1:1", "col_keep", "integer", notNull(), withDefault("0")),
			makeColumn("s1", "t_old", "1:2", "col_gone", "text"),
		},
	}
	tB := schemasnapshot.Table{
		ObjectRef:         ref("s2", "t_new"), // name + schema changed
		ID:                "1",
		Kind:              schemasnapshot.TableKindPartitioned,                // kind changed
		PartitionParent:   refPtr("s2", "parent_b"),                           // changed
		PartitionChildren: []schemasnapshot.ObjectRef{ref("s2", "child_b")},   // changed
		InheritsFrom:      []schemasnapshot.ObjectRef{ref("s2", "base_b")},    // changed
		InheritedBy:       []schemasnapshot.ObjectRef{ref("s2", "derived_b")}, // changed
		Columns: []schemasnapshot.Column{
			// col "1:1" matches by ID; name+type+nullability+default all differ.
			makeColumn("s2", "t_new", "1:1", "col_renamed", "bigint"),
			makeColumn("s2", "t_new", "1:3", "col_added", "text"), // added
		},
	}

	a := snapWithTables(tA, makeTable("2", "s1", "dropped_only", schemasnapshot.TableKindOrdinary))
	b := snapWithTables(tB, makeTable("3", "s2", "added_only", schemasnapshot.TableKindOrdinary))

	got := Diff(a, b)

	seen := map[DiffType]bool{}
	for _, d := range got {
		seen[d.Type] = true

		wantOp, wantOT, wantAttr := expectedFacets(t, d.Type)
		if d.Operation != wantOp {
			t.Errorf("%s: Operation=%q, want %q", d.Type, d.Operation, wantOp)
		}
		if d.ObjectType != wantOT {
			t.Errorf("%s: ObjectType=%q, want %q", d.Type, d.ObjectType, wantOT)
		}
		if d.Attribute != wantAttr {
			t.Errorf("%s: Attribute=%q, want %q", d.Type, d.Attribute, wantAttr)
		}
		// Invariant: Attribute is set iff the finding is a *_CHANGED.
		if (d.Operation == OpChanged) != (d.Attribute != AttrNone) {
			t.Errorf("%s: Attribute/Operation mismatch: op=%q attr=%q", d.Type, d.Operation, d.Attribute)
		}
	}

	allTypes := []DiffType{
		TableAdded, TableDropped, TableNameChanged, TableSchemaChanged, TableKindChanged,
		TablePartitionParentChanged, TablePartitionChildrenChanged, TableInheritsChanged,
		TableInheritedByChanged,
		ColumnAdded, ColumnDropped, ColumnNameChanged, ColumnTypeChanged,
		ColumnNullabilityChanged, ColumnDefaultChanged,
	}
	for _, dt := range allTypes {
		if !seen[dt] {
			t.Errorf("scenario did not emit %s — decomposition for it is unverified", dt)
		}
	}
}

// expectedFacets derives the (Operation, ObjectType, Attribute) a Difference of
// the given Type must carry, by parsing the Type string. This is the test's
// independent oracle; production sets these explicitly at each call site.
func expectedFacets(t *testing.T, dt DiffType) (Operation, ObjectType, Attribute) {
	t.Helper()
	s := string(dt)

	var op Operation
	switch {
	case strings.HasSuffix(s, "_ADDED"):
		op = OpAdded
	case strings.HasSuffix(s, "_DROPPED"):
		op = OpDropped
	case strings.HasSuffix(s, "_CHANGED"):
		op = OpChanged
	default:
		t.Fatalf("unrecognized DiffType verb: %s", s)
	}

	var ot ObjectType
	switch {
	case strings.HasPrefix(s, "TABLE_"):
		ot = ObjectTypeTable
	case strings.HasPrefix(s, "COLUMN_"):
		ot = ObjectTypeColumn
	default:
		t.Fatalf("unrecognized DiffType object prefix: %s", s)
	}

	attr := AttrNone
	if op == OpChanged {
		mid := strings.TrimSuffix(strings.TrimPrefix(s, string(ot)+"_"), "_CHANGED")
		attr = Attribute(mid)
	}
	return op, ot, attr
}
