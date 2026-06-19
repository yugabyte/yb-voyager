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
	"testing"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
)

// snapWithColumns builds a snapshot containing the given columns (no tables).
func snapWithColumns(cols ...schemasnapshot.Column) *schemasnapshot.SchemaSnapshot {
	return &schemasnapshot.SchemaSnapshot{
		Version: 1,
		Columns: cols,
	}
}

// snapWithTablesAndColumns builds a snapshot with both tables and columns.
func snapWithTablesAndColumns(tables []schemasnapshot.Table, cols []schemasnapshot.Column) *schemasnapshot.SchemaSnapshot {
	return &schemasnapshot.SchemaSnapshot{
		Version: 1,
		Tables:  tables,
		Columns: cols,
	}
}

// makeColumn builds a Column with the given parent table, ID, name, and type.
func makeColumn(tableSchema, tableName, id, name, dataType string) schemasnapshot.Column {
	return schemasnapshot.Column{
		Table:    schemasnapshot.ObjectRef{Schema: tableSchema, Name: tableName},
		ID:       id,
		Name:     name,
		DataType: dataType,
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: Column added (table gains a new column)
// ──────────────────────────────────────────────────────────────────────────────

func TestDiffColumns_ColumnAdded(t *testing.T) {
	newCol := makeColumn("public", "orders", "101:2", "email", "text")
	a := snapWithColumns()
	b := snapWithColumns(newCol)

	got := Diff(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 difference, got %d: %v", len(got), got)
	}
	d := got[0]
	if d.Type != ColumnAdded {
		t.Errorf("expected ColumnAdded, got %v", d.Type)
	}
	if d.Object != ref("public", "orders") {
		t.Errorf("expected Object=public.orders (parent table), got %v", d.Object)
	}
	if d.AnchorTable == nil || *d.AnchorTable != ref("public", "orders") {
		t.Errorf("expected AnchorTable=public.orders, got %v", d.AnchorTable)
	}
	if d.SubObject != "email" {
		t.Errorf("expected SubObject='email', got %q", d.SubObject)
	}
	if d.OldValue != nil {
		t.Errorf("expected OldValue=nil, got %v", d.OldValue)
	}
	if d.NewValue.(string) != "text" {
		t.Errorf("expected NewValue='text', got %v", d.NewValue)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: Column dropped (column only in A)
// ──────────────────────────────────────────────────────────────────────────────

func TestDiffColumns_ColumnDropped(t *testing.T) {
	oldCol := makeColumn("public", "orders", "101:3", "legacy_field", "integer")
	a := snapWithColumns(oldCol)
	b := snapWithColumns()

	got := Diff(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 difference, got %d: %v", len(got), got)
	}
	d := got[0]
	if d.Type != ColumnDropped {
		t.Errorf("expected ColumnDropped, got %v", d.Type)
	}
	if d.Object != ref("public", "orders") {
		t.Errorf("expected Object=public.orders, got %v", d.Object)
	}
	if d.AnchorTable == nil || *d.AnchorTable != ref("public", "orders") {
		t.Errorf("expected AnchorTable=public.orders, got %v", d.AnchorTable)
	}
	if d.SubObject != "legacy_field" {
		t.Errorf("expected SubObject='legacy_field', got %q", d.SubObject)
	}
	if d.OldValue.(string) != "integer" {
		t.Errorf("expected OldValue='integer' (type), got %v", d.OldValue)
	}
	if d.NewValue != nil {
		t.Errorf("expected NewValue=nil, got %v", d.NewValue)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: Column renamed (same ID, different Name)
// ──────────────────────────────────────────────────────────────────────────────

func TestDiffColumns_ColumnRenamed(t *testing.T) {
	oldCol := makeColumn("public", "users", "200:1", "usr_name", "text")
	newCol := makeColumn("public", "users", "200:1", "username", "text") // same ID, new name

	a := snapWithColumns(oldCol)
	b := snapWithColumns(newCol)

	got := Diff(a, b)

	// Must have exactly ONE finding: ColumnNameChanged
	if len(got) != 1 {
		t.Fatalf("expected 1 difference, got %d: %v", len(got), got)
	}
	d := got[0]
	if d.Type != ColumnNameChanged {
		t.Errorf("expected ColumnNameChanged, got %v", d.Type)
	}
	if d.Object != ref("public", "users") {
		t.Errorf("expected Object=public.users, got %v", d.Object)
	}
	if d.SubObject != "usr_name" {
		t.Errorf("expected SubObject='usr_name' (old name), got %q", d.SubObject)
	}
	if d.Property != "name" {
		t.Errorf("expected Property='name', got %q", d.Property)
	}
	if d.OldValue.(string) != "usr_name" {
		t.Errorf("expected OldValue='usr_name', got %v", d.OldValue)
	}
	if d.NewValue.(string) != "username" {
		t.Errorf("expected NewValue='username', got %v", d.NewValue)
	}

	// No ColumnAdded or ColumnDropped
	for _, diff := range got {
		if diff.Type == ColumnAdded || diff.Type == ColumnDropped {
			t.Errorf("unexpected %v in result for a rename", diff.Type)
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: Column type changed
// ──────────────────────────────────────────────────────────────────────────────

func TestDiffColumns_ColumnTypeChanged(t *testing.T) {
	oldCol := makeColumn("public", "products", "300:1", "price", "integer")
	newCol := makeColumn("public", "products", "300:1", "price", "numeric(10,2)")

	a := snapWithColumns(oldCol)
	b := snapWithColumns(newCol)

	got := Diff(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 difference, got %d: %v", len(got), got)
	}
	d := got[0]
	if d.Type != ColumnTypeChanged {
		t.Errorf("expected ColumnTypeChanged, got %v", d.Type)
	}
	if d.Object != ref("public", "products") {
		t.Errorf("expected Object=public.products, got %v", d.Object)
	}
	if d.SubObject != "price" {
		t.Errorf("expected SubObject='price', got %q", d.SubObject)
	}
	if d.Property != "data_type" {
		t.Errorf("expected Property='data_type', got %q", d.Property)
	}
	if d.OldValue.(string) != "integer" {
		t.Errorf("expected OldValue='integer', got %v", d.OldValue)
	}
	if d.NewValue.(string) != "numeric(10,2)" {
		t.Errorf("expected NewValue='numeric(10,2)', got %v", d.NewValue)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: Column nullability changed (false→true and true→false)
// ──────────────────────────────────────────────────────────────────────────────

func TestDiffColumns_ColumnNullabilityChanged_FalseToTrue(t *testing.T) {
	oldCol := makeColumn("public", "orders", "101:5", "qty", "integer")
	oldCol.NotNull = false
	newCol := makeColumn("public", "orders", "101:5", "qty", "integer")
	newCol.NotNull = true

	a := snapWithColumns(oldCol)
	b := snapWithColumns(newCol)

	got := Diff(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 difference, got %d: %v", len(got), got)
	}
	d := got[0]
	if d.Type != ColumnNullabilityChanged {
		t.Errorf("expected ColumnNullabilityChanged, got %v", d.Type)
	}
	if d.Property != "not_null" {
		t.Errorf("expected Property='not_null', got %q", d.Property)
	}
	if d.OldValue.(bool) != false {
		t.Errorf("expected OldValue=false, got %v", d.OldValue)
	}
	if d.NewValue.(bool) != true {
		t.Errorf("expected NewValue=true, got %v", d.NewValue)
	}
}

func TestDiffColumns_ColumnNullabilityChanged_TrueToFalse(t *testing.T) {
	oldCol := makeColumn("public", "orders", "101:5", "qty", "integer")
	oldCol.NotNull = true
	newCol := makeColumn("public", "orders", "101:5", "qty", "integer")
	newCol.NotNull = false

	a := snapWithColumns(oldCol)
	b := snapWithColumns(newCol)

	got := Diff(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 difference, got %d: %v", len(got), got)
	}
	d := got[0]
	if d.Type != ColumnNullabilityChanged {
		t.Errorf("expected ColumnNullabilityChanged, got %v", d.Type)
	}
	if d.OldValue.(bool) != true {
		t.Errorf("expected OldValue=true, got %v", d.OldValue)
	}
	if d.NewValue.(bool) != false {
		t.Errorf("expected NewValue=false, got %v", d.NewValue)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: Column default changed (including ""→"x" and "x"→"")
// ──────────────────────────────────────────────────────────────────────────────

func TestDiffColumns_ColumnDefaultChanged_SetToNew(t *testing.T) {
	oldCol := makeColumn("public", "orders", "101:6", "status", "text")
	oldCol.Default = "pending"
	newCol := makeColumn("public", "orders", "101:6", "status", "text")
	newCol.Default = "active"

	a := snapWithColumns(oldCol)
	b := snapWithColumns(newCol)

	got := Diff(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 difference, got %d: %v", len(got), got)
	}
	d := got[0]
	if d.Type != ColumnDefaultChanged {
		t.Errorf("expected ColumnDefaultChanged, got %v", d.Type)
	}
	if d.Property != "default" {
		t.Errorf("expected Property='default', got %q", d.Property)
	}
	if d.OldValue.(string) != "pending" {
		t.Errorf("expected OldValue='pending', got %v", d.OldValue)
	}
	if d.NewValue.(string) != "active" {
		t.Errorf("expected NewValue='active', got %v", d.NewValue)
	}
}

func TestDiffColumns_ColumnDefaultChanged_EmptyToSet(t *testing.T) {
	oldCol := makeColumn("public", "orders", "101:7", "note", "text")
	oldCol.Default = ""
	newCol := makeColumn("public", "orders", "101:7", "note", "text")
	newCol.Default = "n/a"

	a := snapWithColumns(oldCol)
	b := snapWithColumns(newCol)

	got := Diff(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 difference, got %d: %v", len(got), got)
	}
	d := got[0]
	if d.Type != ColumnDefaultChanged {
		t.Errorf("expected ColumnDefaultChanged, got %v", d.Type)
	}
}

func TestDiffColumns_ColumnDefaultChanged_SetToEmpty(t *testing.T) {
	oldCol := makeColumn("public", "orders", "101:8", "note2", "text")
	oldCol.Default = "some_value"
	newCol := makeColumn("public", "orders", "101:8", "note2", "text")
	newCol.Default = ""

	a := snapWithColumns(oldCol)
	b := snapWithColumns(newCol)

	got := Diff(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 difference, got %d: %v", len(got), got)
	}
	d := got[0]
	if d.Type != ColumnDefaultChanged {
		t.Errorf("expected ColumnDefaultChanged, got %v", d.Type)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: Identical columns → no findings
// ──────────────────────────────────────────────────────────────────────────────

func TestDiffColumns_IdenticalColumns_NoFindings(t *testing.T) {
	col := makeColumn("public", "orders", "101:1", "id", "integer")
	col.NotNull = true
	col.Default = "nextval('orders_id_seq')"

	a := snapWithColumns(col)
	b := snapWithColumns(col)

	got := Diff(a, b)
	if len(got) != 0 {
		t.Errorf("expected no differences, got %d: %v", len(got), got)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: Multiple columns on multiple tables sort under their parent table
// ──────────────────────────────────────────────────────────────────────────────

func TestDiffColumns_MultipleTablesSort(t *testing.T) {
	// Two tables: "alpha" and "zeta". Each has a column added.
	// alpha.col1 added, zeta.col1 added
	// We expect alpha's finding to come before zeta's in the sorted output.

	colAlpha := makeColumn("public", "alpha", "10:1", "col1", "text")
	colZeta := makeColumn("public", "zeta", "20:1", "col1", "text")

	a := snapWithColumns()
	b := snapWithColumns(colAlpha, colZeta)

	got := Diff(a, b)
	if len(got) != 2 {
		t.Fatalf("expected 2 differences, got %d: %v", len(got), got)
	}

	// After sort: alpha comes before zeta (Object.Name = "alpha" < "zeta")
	if got[0].Object.Name != "alpha" {
		t.Errorf("expected first finding for 'alpha', got %q", got[0].Object.Name)
	}
	if got[1].Object.Name != "zeta" {
		t.Errorf("expected second finding for 'zeta', got %q", got[1].Object.Name)
	}

	// Within same table, SubObject sorts columns under their parent
	colAlpha2 := makeColumn("public", "alpha", "10:2", "zzz", "text")
	colAlpha3 := makeColumn("public", "alpha", "10:3", "aaa", "text")

	a2 := snapWithColumns()
	b2 := snapWithColumns(colAlpha2, colAlpha3)

	got2 := Diff(a2, b2)
	if len(got2) != 2 {
		t.Fatalf("expected 2 differences, got %d: %v", len(got2), got2)
	}
	// aaa should come before zzz
	if got2[0].SubObject != "aaa" {
		t.Errorf("expected first SubObject='aaa', got %q", got2[0].SubObject)
	}
	if got2[1].SubObject != "zzz" {
		t.Errorf("expected second SubObject='zzz', got %q", got2[1].SubObject)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: ID-empty fallback — match by (table, name) composite key
// ──────────────────────────────────────────────────────────────────────────────

func TestDiffColumns_IDEmptyFallback_MatchedByTableAndName(t *testing.T) {
	// Both sides have same table+name, empty ID → should match, no findings if identical.
	colA := schemasnapshot.Column{
		Table:    schemasnapshot.ObjectRef{Schema: "public", Name: "orders"},
		ID:       "",
		Name:     "status",
		DataType: "text",
	}
	colB := schemasnapshot.Column{
		Table:    schemasnapshot.ObjectRef{Schema: "public", Name: "orders"},
		ID:       "",
		Name:     "status",
		DataType: "text",
	}

	a := snapWithColumns(colA)
	b := snapWithColumns(colB)

	got := Diff(a, b)
	if len(got) != 0 {
		t.Errorf("expected no differences for identical empty-ID columns matched by name, got %d: %v", len(got), got)
	}

	// Now change the type — should produce one finding
	colBChanged := colB
	colBChanged.DataType = "varchar(255)"
	bChanged := snapWithColumns(colBChanged)

	got2 := Diff(a, bChanged)
	if len(got2) != 1 {
		t.Fatalf("expected 1 difference for empty-ID columns with type change, got %d: %v", len(got2), got2)
	}
	if got2[0].Type != ColumnTypeChanged {
		t.Errorf("expected ColumnTypeChanged, got %v", got2[0].Type)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: ID matches but parent table was renamed — match by ID, Object = side-A Column.Table
// ──────────────────────────────────────────────────────────────────────────────

func TestDiffColumns_IDMatchTableRenamed_ObjectIsOldTable(t *testing.T) {
	// Column with same ID but different Column.Table (parent was renamed)
	oldCol := schemasnapshot.Column{
		Table:    schemasnapshot.ObjectRef{Schema: "public", Name: "old_table"},
		ID:       "500:1",
		Name:     "id",
		DataType: "integer",
	}
	newCol := schemasnapshot.Column{
		Table:    schemasnapshot.ObjectRef{Schema: "public", Name: "new_table"}, // parent renamed
		ID:       "500:1",                                                       // same ID
		Name:     "id",
		DataType: "integer",
	}

	a := snapWithColumns(oldCol)
	b := snapWithColumns(newCol)

	// Identical column content (only parent table ref differs) → no column findings
	got := Diff(a, b)
	if len(got) != 0 {
		t.Errorf("expected no column-level differences (table rename tracked separately), got %d: %v", len(got), got)
	}

	// Now change the type to get one finding; Object must be side-A's table
	newColChanged := newCol
	newColChanged.DataType = "bigint"
	bChanged := snapWithColumns(newColChanged)

	got2 := Diff(a, bChanged)
	if len(got2) != 1 {
		t.Fatalf("expected 1 difference, got %d: %v", len(got2), got2)
	}
	d := got2[0]
	if d.Object != ref("public", "old_table") {
		t.Errorf("expected Object=public.old_table (side-A), got %v", d.Object)
	}
	if d.AnchorTable == nil || *d.AnchorTable != ref("public", "old_table") {
		t.Errorf("expected AnchorTable=public.old_table (side-A), got %v", d.AnchorTable)
	}
}
