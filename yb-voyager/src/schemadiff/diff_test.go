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
	"testing"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
)

// helper: build a minimal SchemaSnapshot with the given tables
func snapWithTables(tables ...schemasnapshot.Table) *schemasnapshot.SchemaSnapshot {
	return &schemasnapshot.SchemaSnapshot{
		Version: 1,
		Tables:  tables,
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

// ──────────────────────────────────────────────────────────────────────────────
// Test: Diff of two empty snapshots returns nil/empty
// ──────────────────────────────────────────────────────────────────────────────

func TestDiff_EmptySnapshots_ReturnsNilOrEmpty(t *testing.T) {
	a := &schemasnapshot.SchemaSnapshot{Version: 1}
	b := &schemasnapshot.SchemaSnapshot{Version: 1}
	got := Diff(a, b)
	if len(got) != 0 {
		t.Errorf("expected no differences, got %d: %v", len(got), got)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: Diff of identical snapshots returns no findings
// ──────────────────────────────────────────────────────────────────────────────

func TestDiff_IdenticalSnapshots_NoFindings(t *testing.T) {
	tbl := makeTable("101", "public", "orders", schemasnapshot.TableKindOrdinary)
	col := schemasnapshot.Column{
		Table:    ref("public", "orders"),
		ID:       "101:1",
		Name:     "id",
		DataType: "integer",
		NotNull:  true,
	}
	a := &schemasnapshot.SchemaSnapshot{Version: 1, Tables: []schemasnapshot.Table{tbl}, Columns: []schemasnapshot.Column{col}}
	b := &schemasnapshot.SchemaSnapshot{Version: 1, Tables: []schemasnapshot.Table{tbl}, Columns: []schemasnapshot.Column{col}}

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
	// Build diffs that exercise all 5 sort keys in turn
	diffs := []Difference{
		// same schema/name/subobj/type; differ only on Property
		{Object: ref("z_schema", "z_name"), SubObject: "z_sub", Type: TableKindChanged, Property: "z_prop"},
		{Object: ref("z_schema", "z_name"), SubObject: "z_sub", Type: TableKindChanged, Property: "a_prop"},
		// same schema/name/subobj; differ only on Type
		{Object: ref("z_schema", "z_name"), SubObject: "a_sub", Type: TableSchemaChanged, Property: ""},
		{Object: ref("z_schema", "z_name"), SubObject: "a_sub", Type: TableAdded, Property: ""},
		// same schema/name; differ only on SubObject
		{Object: ref("z_schema", "a_name"), SubObject: "z_sub", Type: TableAdded, Property: ""},
		{Object: ref("z_schema", "a_name"), SubObject: "a_sub", Type: TableAdded, Property: ""},
		// same schema; differ only on Name
		{Object: ref("a_schema", "z_name"), SubObject: "", Type: TableAdded, Property: ""},
		{Object: ref("a_schema", "a_name"), SubObject: "", Type: TableAdded, Property: ""},
	}

	sortDifferences(diffs)

	want := []Difference{
		{Object: ref("a_schema", "a_name"), SubObject: "", Type: TableAdded, Property: ""},
		{Object: ref("a_schema", "z_name"), SubObject: "", Type: TableAdded, Property: ""},
		{Object: ref("z_schema", "a_name"), SubObject: "a_sub", Type: TableAdded, Property: ""},
		{Object: ref("z_schema", "a_name"), SubObject: "z_sub", Type: TableAdded, Property: ""},
		{Object: ref("z_schema", "z_name"), SubObject: "a_sub", Type: TableAdded, Property: ""},
		{Object: ref("z_schema", "z_name"), SubObject: "a_sub", Type: TableSchemaChanged, Property: ""},
		{Object: ref("z_schema", "z_name"), SubObject: "z_sub", Type: TableKindChanged, Property: "a_prop"},
		{Object: ref("z_schema", "z_name"), SubObject: "z_sub", Type: TableKindChanged, Property: "z_prop"},
	}

	if !reflect.DeepEqual(diffs, want) {
		t.Errorf("wrong sort order\ngot:  %v\nwant: %v", diffs, want)
	}
}
