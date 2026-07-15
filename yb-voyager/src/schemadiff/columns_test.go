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

// tbl builds an ordinary Table with the given ID, schema, name, and nested columns.
// It is the columns-focused counterpart of makeTable (tables_test.go), which takes
// an explicit Kind; these tests only ever need TableKindOrdinary.
func tbl(id, schema, name string, cols ...schemasnapshot.Column) schemasnapshot.Table {
	return schemasnapshot.Table{
		ObjectRef: schemasnapshot.ObjectRef{Schema: schema, Name: name},
		ID:        id,
		Kind:      schemasnapshot.TableKindOrdinary,
		Columns:   cols,
	}
}

// snap builds a SnapshotContent containing the given tables (each carrying its own
// nested columns). DatabaseType is set to "postgresql" because these helpers model
// PG snapshots whose IDs (OIDs) are stable and comparable within the same engine.
func snap(tables ...schemasnapshot.Table) *schemasnapshot.SnapshotContent {
	return &schemasnapshot.SnapshotContent{
		Version:      1,
		DatabaseType: "postgresql",
		Tables:       tables,
	}
}

// colOpt is a functional option for makeColumn.
type colOpt func(*schemasnapshot.Column)

// notNull sets NotNull=true on a column.
func notNull() colOpt { return func(c *schemasnapshot.Column) { c.NotNull = true } }

// withDefault sets Default on a column.
func withDefault(d string) colOpt { return func(c *schemasnapshot.Column) { c.Default = d } }

// makeColumn builds a Column with the given parent table, ID, name, and type.
// Optional colOpts can be passed to set additional fields (e.g. notNull(), withDefault(...)).
func makeColumn(tableSchema, tableName, id, name, dataType string, opts ...colOpt) schemasnapshot.Column {
	c := schemasnapshot.Column{
		TableScopedRef: schemasnapshot.TableScopedRef{
			Table: schemasnapshot.ObjectRef{Schema: tableSchema, Name: tableName},
			Name:  name,
		},
		ID:       id,
		DataType: dataType,
	}
	for _, o := range opts {
		o(&c)
	}
	return c
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: Column added (table gains a new column)
// ──────────────────────────────────────────────────────────────────────────────

func TestDiffColumns_ColumnAdded(t *testing.T) {
	newCol := makeColumn("public", "orders", "101:2", "email", "text", notNull(), withDefault("'unknown'"))
	// Same parent table (id "101") on both sides so it matches; only the column differs.
	a := snap(tbl("101", "public", "orders"))
	b := snap(tbl("101", "public", "orders", newCol))

	got := Diff(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 difference, got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	d := got[0]
	if d.Type != ColumnAdded {
		t.Errorf("expected ColumnAdded, got %v", d.Type)
	}
	if identKey(d, "postgresql") != "public.orders.email" {
		t.Errorf("expected Object.Key=public.orders.email, got %v", identKey(d, "postgresql"))
	}
	if d.SideAValue != nil {
		t.Errorf("expected OldValue=nil, got %v", d.SideAValue)
	}
	nv, ok := d.SideBValue.(schemasnapshot.Column)
	if !ok {
		t.Fatalf("expected NewValue to be a schemasnapshot.Column, got %T: %v", d.SideBValue, d.SideBValue)
	}
	if nv != newCol {
		t.Errorf("expected NewValue=%v, got %v", newCol, nv)
	}
	if nv.DataType != "text" {
		t.Errorf("expected NewValue.DataType='text', got %v", nv.DataType)
	}
	if nv.NotNull != true {
		t.Errorf("expected NewValue.NotNull=true, got %v", nv.NotNull)
	}
	if nv.Default != "'unknown'" {
		t.Errorf("expected NewValue.Default=\"'unknown'\", got %v", nv.Default)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: Column dropped (column only in A)
// ──────────────────────────────────────────────────────────────────────────────

func TestDiffColumns_ColumnDropped(t *testing.T) {
	oldCol := makeColumn("public", "orders", "101:3", "legacy_field", "integer", notNull(), withDefault("0"))
	a := snap(tbl("101", "public", "orders", oldCol))
	b := snap(tbl("101", "public", "orders"))

	got := Diff(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 difference, got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	d := got[0]
	if d.Type != ColumnDropped {
		t.Errorf("expected ColumnDropped, got %v", d.Type)
	}
	if identKey(d, "postgresql") != "public.orders.legacy_field" {
		t.Errorf("expected Object.Key=public.orders.legacy_field, got %v", identKey(d, "postgresql"))
	}
	ov, ok := d.SideAValue.(schemasnapshot.Column)
	if !ok {
		t.Fatalf("expected OldValue to be a schemasnapshot.Column, got %T: %v", d.SideAValue, d.SideAValue)
	}
	if ov != oldCol {
		t.Errorf("expected OldValue=%v, got %v", oldCol, ov)
	}
	if ov.DataType != "integer" {
		t.Errorf("expected OldValue.DataType='integer', got %v", ov.DataType)
	}
	if ov.NotNull != true {
		t.Errorf("expected OldValue.NotNull=true, got %v", ov.NotNull)
	}
	if ov.Default != "0" {
		t.Errorf("expected OldValue.Default='0', got %v", ov.Default)
	}
	if d.SideBValue != nil {
		t.Errorf("expected NewValue=nil, got %v", d.SideBValue)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: Column renamed (same ID, different Name)
// ──────────────────────────────────────────────────────────────────────────────

func TestDiffColumns_ColumnRenamed(t *testing.T) {
	oldCol := makeColumn("public", "users", "200:1", "usr_name", "text")
	newCol := makeColumn("public", "users", "200:1", "username", "text") // same ID, new name

	a := snap(tbl("200", "public", "users", oldCol))
	b := snap(tbl("200", "public", "users", newCol))

	got := Diff(a, b)

	// Must have exactly ONE finding: ColumnNameChanged
	if len(got) != 1 {
		t.Fatalf("expected 1 difference, got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	d := got[0]
	if d.Type != ColumnNameChanged {
		t.Errorf("expected ColumnNameChanged, got %v", d.Type)
	}
	if identKey(d, "postgresql") != "public.users.usr_name" {
		t.Errorf("expected Object.Key=public.users.usr_name (old name), got %v", identKey(d, "postgresql"))
	}
	if d.SideAValue.(string) != "usr_name" {
		t.Errorf("expected OldValue='usr_name', got %v", d.SideAValue)
	}
	if d.SideBValue.(string) != "username" {
		t.Errorf("expected NewValue='username', got %v", d.SideBValue)
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

	a := snap(tbl("300", "public", "products", oldCol))
	b := snap(tbl("300", "public", "products", newCol))

	got := Diff(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 difference, got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	d := got[0]
	if d.Type != ColumnTypeChanged {
		t.Errorf("expected ColumnTypeChanged, got %v", d.Type)
	}
	if identKey(d, "postgresql") != "public.products.price" {
		t.Errorf("expected Object.Key=public.products.price, got %v", identKey(d, "postgresql"))
	}
	if d.SideAValue.(string) != "integer" {
		t.Errorf("expected OldValue='integer', got %v", d.SideAValue)
	}
	if d.SideBValue.(string) != "numeric(10,2)" {
		t.Errorf("expected NewValue='numeric(10,2)', got %v", d.SideBValue)
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

	a := snap(tbl("101", "public", "orders", oldCol))
	b := snap(tbl("101", "public", "orders", newCol))

	got := Diff(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 difference, got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	d := got[0]
	if d.Type != ColumnNullabilityChanged {
		t.Errorf("expected ColumnNullabilityChanged, got %v", d.Type)
	}
	if d.SideAValue.(bool) != false {
		t.Errorf("expected OldValue=false, got %v", d.SideAValue)
	}
	if d.SideBValue.(bool) != true {
		t.Errorf("expected NewValue=true, got %v", d.SideBValue)
	}
}

func TestDiffColumns_ColumnNullabilityChanged_TrueToFalse(t *testing.T) {
	oldCol := makeColumn("public", "orders", "101:5", "qty", "integer")
	oldCol.NotNull = true
	newCol := makeColumn("public", "orders", "101:5", "qty", "integer")
	newCol.NotNull = false

	a := snap(tbl("101", "public", "orders", oldCol))
	b := snap(tbl("101", "public", "orders", newCol))

	got := Diff(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 difference, got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	d := got[0]
	if d.Type != ColumnNullabilityChanged {
		t.Errorf("expected ColumnNullabilityChanged, got %v", d.Type)
	}
	if d.SideAValue.(bool) != true {
		t.Errorf("expected OldValue=true, got %v", d.SideAValue)
	}
	if d.SideBValue.(bool) != false {
		t.Errorf("expected NewValue=false, got %v", d.SideBValue)
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

	a := snap(tbl("101", "public", "orders", oldCol))
	b := snap(tbl("101", "public", "orders", newCol))

	got := Diff(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 difference, got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	d := got[0]
	if d.Type != ColumnDefaultChanged {
		t.Errorf("expected ColumnDefaultChanged, got %v", d.Type)
	}
	if d.SideAValue.(string) != "pending" {
		t.Errorf("expected OldValue='pending', got %v", d.SideAValue)
	}
	if d.SideBValue.(string) != "active" {
		t.Errorf("expected NewValue='active', got %v", d.SideBValue)
	}
}

func TestDiffColumns_ColumnDefaultChanged_EmptyToSet(t *testing.T) {
	oldCol := makeColumn("public", "orders", "101:7", "note", "text")
	oldCol.Default = ""
	newCol := makeColumn("public", "orders", "101:7", "note", "text")
	newCol.Default = "n/a"

	a := snap(tbl("101", "public", "orders", oldCol))
	b := snap(tbl("101", "public", "orders", newCol))

	got := Diff(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 difference, got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
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

	a := snap(tbl("101", "public", "orders", oldCol))
	b := snap(tbl("101", "public", "orders", newCol))

	got := Diff(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 difference, got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
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

	a := snap(tbl("101", "public", "orders", col))
	b := snap(tbl("101", "public", "orders", col))

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

	a := snap(tbl("10", "public", "alpha"), tbl("20", "public", "zeta"))
	b := snap(tbl("10", "public", "alpha", colAlpha), tbl("20", "public", "zeta", colZeta))

	got := Diff(a, b)
	if len(got) != 2 {
		t.Fatalf("expected 2 differences, got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}

	// After sort: alpha comes before zeta (Object.Key = "public.alpha..." < "public.zeta...")
	if identKey(got[0], "postgresql") != "public.alpha.col1" {
		t.Errorf("expected first finding for public.alpha.col1, got %q", identKey(got[0], "postgresql"))
	}
	if identKey(got[1], "postgresql") != "public.zeta.col1" {
		t.Errorf("expected second finding for public.zeta.col1, got %q", identKey(got[1], "postgresql"))
	}

	// Within same table, the column tail of Object.Key sorts columns under their parent
	colAlpha2 := makeColumn("public", "alpha", "10:2", "zzz", "text")
	colAlpha3 := makeColumn("public", "alpha", "10:3", "aaa", "text")

	a2 := snap(tbl("10", "public", "alpha"))
	b2 := snap(tbl("10", "public", "alpha", colAlpha2, colAlpha3))

	got2 := Diff(a2, b2)
	if len(got2) != 2 {
		t.Fatalf("expected 2 differences, got %d: %v", len(got2), got2)
	}
	for _, d := range got2 {
		assertAnchoredToObject(t, d)
	}
	// aaa should come before zzz
	if identKey(got2[0], "postgresql") != "public.alpha.aaa" {
		t.Errorf("expected first Object.Key='public.alpha.aaa', got %q", identKey(got2[0], "postgresql"))
	}
	if identKey(got2[1], "postgresql") != "public.alpha.zzz" {
		t.Errorf("expected second Object.Key='public.alpha.zzz', got %q", identKey(got2[1], "postgresql"))
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: cross-engine gate — ID matching requires a.DatabaseType == b.DatabaseType.
// When it differs, matching falls back to (table, name), so same-ID+different-
// name produces ColumnDropped+ColumnAdded, not ColumnNameChanged.
// ──────────────────────────────────────────────────────────────────────────────

// TestDiffColumns_DifferentDatabaseType_RenameBecomesAddDrop: same column ID on
// both sides but different DatabaseType must NOT produce ColumnNameChanged —
// cross-type ID comparison is illegal, so matching falls back to name. The
// parent table (same schema.name on both sides) still matches by name, since
// table matching is also gated by DatabaseType equality.
func TestDiffColumns_DifferentDatabaseType_RenameBecomesAddDrop(t *testing.T) {
	oldCol := makeColumn("public", "users", "200:1", "old_col", "text")
	newCol := makeColumn("public", "users", "200:1", "new_col", "text") // same ID, different name
	a := &schemasnapshot.SnapshotContent{
		Version:      1,
		DatabaseType: "postgresql",
		Tables:       []schemasnapshot.Table{tbl("200", "public", "users", oldCol)},
	}
	b := &schemasnapshot.SnapshotContent{
		Version:      1,
		DatabaseType: "mysql",
		Tables:       []schemasnapshot.Table{tbl("200", "public", "users", newCol)},
	}

	got := Diff(a, b)

	// Must NOT produce ColumnNameChanged — cross-type ID matching is illegal.
	// Must produce ColumnDropped(old_col) + ColumnAdded(new_col).
	if len(got) != 2 {
		t.Fatalf("expected 2 differences (ColumnDropped+ColumnAdded), got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	var hasDropped, hasAdded bool
	for _, d := range got {
		if d.Type == ColumnNameChanged {
			t.Errorf("unexpected ColumnNameChanged when DatabaseType differs: %v", d)
		}
		if d.Type == ColumnDropped && identKey(d, "postgresql") == "public.users.old_col" {
			hasDropped = true
		}
		if d.Type == ColumnAdded && identKey(d, "postgresql") == "public.users.new_col" {
			hasAdded = true
		}
	}
	if !hasDropped {
		t.Errorf("expected ColumnDropped for old_col; got: %v", got)
	}
	if !hasAdded {
		t.Errorf("expected ColumnAdded for new_col; got: %v", got)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: ID-empty fallback — match by (table, name) composite key
// ──────────────────────────────────────────────────────────────────────────────

func TestDiffColumns_IDEmptyFallback_MatchedByTableAndName(t *testing.T) {
	// Both sides have same table+name, empty ID → should match, no findings if identical.
	colA := makeColumn("public", "orders", "", "status", "text")
	colB := makeColumn("public", "orders", "", "status", "text")

	a := snap(tbl("101", "public", "orders", colA))
	b := snap(tbl("101", "public", "orders", colB))

	got := Diff(a, b)
	if len(got) != 0 {
		t.Errorf("expected no differences for identical empty-ID columns matched by name, got %d: %v", len(got), got)
	}

	// Now change the type — should produce one finding
	colBChanged := colB
	colBChanged.DataType = "varchar(255)"
	bChanged := snap(tbl("101", "public", "orders", colBChanged))

	got2 := Diff(a, bChanged)
	if len(got2) != 1 {
		t.Fatalf("expected 1 difference for empty-ID columns with type change, got %d: %v", len(got2), got2)
	}
	for _, d := range got2 {
		assertAnchoredToObject(t, d)
	}
	if got2[0].Type != ColumnTypeChanged {
		t.Errorf("expected ColumnTypeChanged, got %v", got2[0].Type)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Tests: cross-engine gate for columns — ID matching only when DatabaseType matches
// ──────────────────────────────────────────────────────────────────────────────

// (StableIdentity=false test deleted: unexpressible after the API refactor.
// Cross-engine add+drop is covered by
// TestDiffColumns_DifferentDatabaseType_RenameBecomesAddDrop.)

// ──────────────────────────────────────────────────────────────────────────────
// Tests: table-lifecycle column findings — a wholly added/dropped table carries
// its columns on the TABLE_ADDED/TABLE_DROPPED finding, with NO separate
// per-column COLUMN_ADDED/COLUMN_DROPPED findings (see emitTableAdded/
// emitTableDropped in tables.go, which read the table's own nested Columns).
// ──────────────────────────────────────────────────────────────────────────────

// TestDiff_TableAdded_SuppressesColumnAdds: when a table is wholly new (TABLE_ADDED),
// its columns are carried on the TABLE_ADDED payload, not emitted as separate
// per-column COLUMN_ADDED findings (diffColumnsIn never runs for an unmatched table).
func TestDiff_TableAdded_SuppressesColumnAdds(t *testing.T) {
	colID := makeColumn("public", "orders", "200:1", "id", "integer")
	colEmail := makeColumn("public", "orders", "200:2", "email", "text")

	a := snap()
	b := snap(tbl("200", "public", "orders", colID, colEmail))

	got := Diff(a, b)

	// Must have exactly 1 finding: TABLE_ADDED for public.orders
	if len(got) != 1 {
		t.Fatalf("expected 1 finding (TableAdded), got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	if got[0].Type != TableAdded {
		t.Errorf("expected TableAdded, got %v", got[0].Type)
	}
	if identKey(got[0], "postgresql") != "public.orders" {
		t.Errorf("expected Object.Key=public.orders, got %v", identKey(got[0], "postgresql"))
	}
	// The columns must survive on the TABLE_ADDED finding's NewValue Table.
	newTbl, ok := got[0].SideBValue.(schemasnapshot.Table)
	if !ok {
		t.Fatalf("expected NewValue to be schemasnapshot.Table, got %T: %v", got[0].SideBValue, got[0].SideBValue)
	}
	cols := newTbl.Columns
	if len(cols) != 2 || cols[0] != colID || cols[1] != colEmail {
		t.Errorf("expected NewValue=[%v, %v], got %v", colID, colEmail, cols)
	}
	for _, d := range got {
		if d.Type == ColumnAdded {
			t.Errorf("unexpected ColumnAdded finding for wholly-added table: %v", d)
		}
	}
}

// TestDiff_TableDropped_SuppressesColumnDrops: when a table is wholly dropped (TABLE_DROPPED),
// its columns are carried on the TABLE_DROPPED payload, not emitted as separate
// per-column COLUMN_DROPPED findings.
func TestDiff_TableDropped_SuppressesColumnDrops(t *testing.T) {
	colID := makeColumn("public", "orders", "200:1", "id", "integer")
	colEmail := makeColumn("public", "orders", "200:2", "email", "text")

	a := snap(tbl("200", "public", "orders", colID, colEmail))
	b := snap()

	got := Diff(a, b)

	// Must have exactly 1 finding: TABLE_DROPPED for public.orders
	if len(got) != 1 {
		t.Fatalf("expected 1 finding (TableDropped), got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	if got[0].Type != TableDropped {
		t.Errorf("expected TableDropped, got %v", got[0].Type)
	}
	if identKey(got[0], "postgresql") != "public.orders" {
		t.Errorf("expected Object.Key=public.orders, got %v", identKey(got[0], "postgresql"))
	}
	// The columns must survive on the TABLE_DROPPED finding's OldValue Table.
	oldTbl, ok := got[0].SideAValue.(schemasnapshot.Table)
	if !ok {
		t.Fatalf("expected OldValue to be schemasnapshot.Table, got %T: %v", got[0].SideAValue, got[0].SideAValue)
	}
	cols := oldTbl.Columns
	if len(cols) != 2 || cols[0] != colID || cols[1] != colEmail {
		t.Errorf("expected OldValue=[%v, %v], got %v", colID, colEmail, cols)
	}
	for _, d := range got {
		if d.Type == ColumnDropped {
			t.Errorf("unexpected ColumnDropped finding for wholly-dropped table: %v", d)
		}
	}
}

// TestDiff_ColumnAddedToExistingTable_NotSuppressed: when a column is added to a matched
// (existing) table, the COLUMN_ADDED finding must NOT be suppressed — there is no TABLE_ADDED.
func TestDiff_ColumnAddedToExistingTable_NotSuppressed(t *testing.T) {
	colID := makeColumn("public", "orders", "200:1", "id", "integer")
	colEmail := makeColumn("public", "orders", "200:2", "email", "text")

	// A: same table + only "id" column
	a := snap(tbl("200", "public", "orders", colID))
	// B: same table (matched) + both "id" and "email" columns
	b := snap(tbl("200", "public", "orders", colID, colEmail))

	got := Diff(a, b)

	// Exactly one finding: COLUMN_ADDED for email
	if len(got) != 1 {
		t.Fatalf("expected 1 finding (ColumnAdded for email), got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	if got[0].Type != ColumnAdded {
		t.Errorf("expected ColumnAdded, got %v", got[0].Type)
	}
	if identKey(got[0], "postgresql") != "public.orders.email" {
		t.Errorf("expected Object.Key=public.orders.email, got %q", identKey(got[0], "postgresql"))
	}
}

// TestDiff_ColumnDroppedFromExistingTable_NotSuppressed: when a column is dropped from a
// matched (existing) table, the COLUMN_DROPPED finding must NOT be suppressed.
func TestDiff_ColumnDroppedFromExistingTable_NotSuppressed(t *testing.T) {
	colID := makeColumn("public", "orders", "200:1", "id", "integer")
	colEmail := makeColumn("public", "orders", "200:2", "email", "text")

	// A: same table + both columns
	a := snap(tbl("200", "public", "orders", colID, colEmail))
	// B: same table (matched) + only "id" column
	b := snap(tbl("200", "public", "orders", colID))

	got := Diff(a, b)

	// Exactly one finding: COLUMN_DROPPED for email
	if len(got) != 1 {
		t.Fatalf("expected 1 finding (ColumnDropped for email), got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	if got[0].Type != ColumnDropped {
		t.Errorf("expected ColumnDropped, got %v", got[0].Type)
	}
	if identKey(got[0], "postgresql") != "public.orders.email" {
		t.Errorf("expected Object.Key=public.orders.email, got %q", identKey(got[0], "postgresql"))
	}
}

// TestDiff_TableDropped_PreservesOtherTableColumnChanges guards against over-suppression:
// dropping table X must not suppress column findings on unrelated matched table Y.
func TestDiff_TableDropped_PreservesOtherTableColumnChanges(t *testing.T) {
	// Table X is dropped; table Y is present on both sides but has a column type change.
	colXA := makeColumn("public", "x", "300:1", "a", "integer")

	colYOld := makeColumn("public", "y", "400:1", "val", "integer")
	colYNew := makeColumn("public", "y", "400:1", "val", "text") // type changed

	a := snap(
		tbl("300", "public", "x", colXA),
		tbl("400", "public", "y", colYOld),
	)
	b := snap(
		tbl("400", "public", "y", colYNew), // X dropped, Y's column type changed
	)

	got := Diff(a, b)

	// Expect: TableDropped(x) + ColumnTypeChanged(y.val) — exactly 2 findings.
	if len(got) != 2 {
		t.Fatalf("expected 2 findings, got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}

	var hasTableDropped, hasColTypeChanged bool
	for _, d := range got {
		if d.Type == TableDropped && identKey(d, "postgresql") == "public.x" {
			hasTableDropped = true
		}
		if d.Type == ColumnTypeChanged && identKey(d, "postgresql") == "public.y.val" {
			hasColTypeChanged = true
		}
		if d.Type == ColumnDropped {
			t.Errorf("unexpected ColumnDropped (should be suppressed for table x): %v", d)
		}
	}

	if !hasTableDropped {
		t.Errorf("expected TableDropped for x, not found in: %v", got)
	}
	if !hasColTypeChanged {
		t.Errorf("expected ColumnTypeChanged for y.val, not found in: %v", got)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: ID matches but parent table was renamed — under nesting, the table
// itself is matched by ID (same ID both sides), so the rename surfaces as
// TableNameChanged; the column is diffed within it and its finding's Object
// is side-A's Column.Table.
// ──────────────────────────────────────────────────────────────────────────────

func TestDiffColumns_IDMatchTableRenamed_ObjectIsOldTable(t *testing.T) {
	// Column with same ID but different Column.Table (parent was renamed).
	// The table itself keeps the same ID ("500") on both sides, so it matches
	// as a rename (TableNameChanged) rather than a drop+add.
	oldCol := makeColumn("public", "old_table", "500:1", "id", "integer")
	newCol := makeColumn("public", "new_table", "500:1", "id", "integer") // parent renamed

	a := snap(tbl("500", "public", "old_table", oldCol))
	b := snap(tbl("500", "public", "new_table", newCol))

	// Identical column content (only parent table ref differs) → just the
	// table-level rename, no column-level finding.
	got := Diff(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 difference (TableNameChanged only), got %d: %v", len(got), got)
	}
	if got[0].Type != TableNameChanged {
		t.Errorf("expected TableNameChanged, got %v", got[0].Type)
	}

	// Now change the type to get an additional column finding; its Object must
	// be side-A's table.
	newColChanged := newCol
	newColChanged.DataType = "bigint"
	bChanged := snap(tbl("500", "public", "new_table", newColChanged))

	got2 := Diff(a, bChanged)
	if len(got2) != 2 {
		t.Fatalf("expected 2 differences (TableNameChanged + ColumnTypeChanged), got %d: %v", len(got2), got2)
	}
	for _, d := range got2 {
		assertAnchoredToObject(t, d)
	}
	var foundColChange bool
	for _, d := range got2 {
		if d.Type == ColumnTypeChanged {
			foundColChange = true
			if identKey(d, "postgresql") != "public.old_table.id" {
				t.Errorf("expected Object.Key=public.old_table.id (side-A), got %v", identKey(d, "postgresql"))
			}
		}
	}
	if !foundColChange {
		t.Errorf("expected ColumnTypeChanged in %v", got2)
	}
}

// IDMissingTableRenamed: without a stable column ID, a column on a renamed
// parent table can't be tracked across the rename — the name-fallback key
// embeds the table name, so old/new keys differ — and degrades to
// COLUMN_DROPPED + COLUMN_ADDED. The parent table itself keeps a stable ID
// so it still matches as a rename (TableNameChanged) alongside the column
// drop+add. Contrast TestDiffColumns_IDMatchTableRenamed_ObjectIsOldTable,
// where a stable column ID tracks the column across the rename.
func TestDiffColumns_IDMissingTableRenamed_BecomesDropAdd(t *testing.T) {
	oldCol := makeColumn("public", "old_table", "", "id", "integer") // empty ID
	newCol := makeColumn("public", "new_table", "", "id", "integer") // parent renamed, still empty ID

	// Same table ID ("600") on both sides → table rename is tracked.
	got := Diff(snap(tbl("600", "public", "old_table", oldCol)), snap(tbl("600", "public", "new_table", newCol)))
	if len(got) != 3 {
		t.Fatalf("expected 3 findings (TableNameChanged + drop+add — column rename untrackable without a stable ID), got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	objByType := map[DiffType]string{}
	for _, d := range got {
		objByType[d.Type] = identKey(d, "postgresql")
	}
	if _, ok := objByType[TableNameChanged]; !ok {
		t.Errorf("expected TableNameChanged, got: %v", got)
	}
	if objByType[ColumnDropped] != "public.old_table.id" {
		t.Errorf("COLUMN_DROPPED should anchor to public.old_table.id, got %v", objByType[ColumnDropped])
	}
	if objByType[ColumnAdded] != "public.new_table.id" {
		t.Errorf("COLUMN_ADDED should anchor to public.new_table.id, got %v", objByType[ColumnAdded])
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Tests: hybrid ID-then-name matching for columns. A column with a stable ID on
// one side but empty on the other must reconcile by name (not a spurious
// drop+add); a genuine drop-and-recreate reusing the table+name with a
// DIFFERENT id stays an add+drop.
// ──────────────────────────────────────────────────────────────────────────────

// IDInAEmptyInB: same column (same table+name, type integer), ID "5:1" in A but
// "" in B. The hybrid residue pass must reconcile them by name → 0 findings.
func TestDiffColumns_HybridResidue_IDInAEmptyInB_Matched(t *testing.T) {
	cA := makeColumn("public", "orders", "5:1", "qty", "integer")
	cB := makeColumn("public", "orders", "", "qty", "integer")

	a := snap(tbl("101", "public", "orders", cA))
	b := snap(tbl("101", "public", "orders", cB))

	got := Diff(a, b)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings (column reconciled by name despite missing ID in B), got %d: %v", len(got), got)
	}
}

// EmptyInAIDInB: symmetric — empty in A, ID in B → still reconciled by name → 0 findings.
func TestDiffColumns_HybridResidue_EmptyInAIDInB_Matched(t *testing.T) {
	cA := makeColumn("public", "orders", "", "qty", "integer")
	cB := makeColumn("public", "orders", "5:1", "qty", "integer")

	a := snap(tbl("101", "public", "orders", cA))
	b := snap(tbl("101", "public", "orders", cB))

	got := Diff(a, b)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings (column reconciled by name despite missing ID in A), got %d: %v", len(got), got)
	}
}

// MixedID with a type change: ID "5:1" in A, "" in B, type integer→bigint.
// Reconciled by name AND the type change surfaces as exactly 1 ColumnTypeChanged.
func TestDiffColumns_HybridResidue_MixedID_TypeChangeSurfaces(t *testing.T) {
	cA := makeColumn("public", "orders", "5:1", "qty", "integer")
	cB := makeColumn("public", "orders", "", "qty", "bigint")

	a := snap(tbl("101", "public", "orders", cA))
	b := snap(tbl("101", "public", "orders", cB))

	got := Diff(a, b)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 finding (ColumnTypeChanged on reconciled column), got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	if got[0].Type != ColumnTypeChanged {
		t.Errorf("expected ColumnTypeChanged, got %v", got[0].Type)
	}
	if identKey(got[0], "postgresql") != "public.orders.qty" {
		t.Errorf("expected Object.Key=public.orders.qty, got %q", identKey(got[0], "postgresql"))
	}
}

// DropRecreateSameNameDifferentID: same table+name, DIFFERENT non-empty IDs ("5:1"
// vs "5:2"), matchByID on. These are genuinely different columns (drop-and-recreate)
// and must NOT be collapsed — expect exactly ColumnDropped + ColumnAdded.
func TestDiffColumns_HybridResidue_DropRecreateSameNameDifferentID_NotCollapsed(t *testing.T) {
	cA := makeColumn("public", "orders", "5:1", "qty", "integer")
	cB := makeColumn("public", "orders", "5:2", "qty", "integer")

	a := snap(tbl("101", "public", "orders", cA))
	b := snap(tbl("101", "public", "orders", cB))

	got := Diff(a, b)
	if len(got) != 2 {
		t.Fatalf("expected 2 findings (ColumnDropped + ColumnAdded for distinct IDs), got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	types := map[DiffType]int{}
	for _, d := range got {
		types[d.Type]++
		if identKey(d, "postgresql") != "public.orders.qty" {
			t.Errorf("expected Object.Key=public.orders.qty, got %q", identKey(d, "postgresql"))
		}
	}
	if types[ColumnDropped] != 1 || types[ColumnAdded] != 1 {
		t.Errorf("expected exactly one ColumnDropped and one ColumnAdded, got %v", types)
	}
}
