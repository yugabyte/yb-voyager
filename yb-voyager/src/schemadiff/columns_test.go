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

// snapWithColumns builds a SnapshotContent containing the given columns (no tables).
// DatabaseType is set to "postgresql" because these helpers model PG snapshots
// whose IDs (OIDs) are stable and comparable within the same engine.
func snapWithColumns(cols ...schemasnapshot.Column) *schemasnapshot.SnapshotContent {
	return &schemasnapshot.SnapshotContent{
		Version:      1,
		DatabaseType: "postgresql",
		Columns:      cols,
	}
}

// snapWithTablesAndColumns builds a SnapshotContent with both tables and columns.
// DatabaseType is set to "postgresql" because these helpers model PG snapshots
// whose IDs (OIDs) are stable and comparable within the same engine.
func snapWithTablesAndColumns(tables []schemasnapshot.Table, cols []schemasnapshot.Column) *schemasnapshot.SnapshotContent {
	return &schemasnapshot.SnapshotContent{
		Version:      1,
		DatabaseType: "postgresql",
		Tables:       tables,
		Columns:      cols,
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
		Table:    schemasnapshot.ObjectRef{Schema: tableSchema, Name: tableName},
		ID:       id,
		Name:     name,
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
	a := snapWithColumns()
	b := snapWithColumns(newCol)

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
	if d.Object != ref("public", "orders") {
		t.Errorf("expected Object=public.orders (parent table), got %v", d.Object)
	}
	if d.SubObject != "email" {
		t.Errorf("expected SubObject='email', got %q", d.SubObject)
	}
	if d.OldValue != nil {
		t.Errorf("expected OldValue=nil, got %v", d.OldValue)
	}
	nv, ok := d.NewValue.(schemasnapshot.Column)
	if !ok {
		t.Fatalf("expected NewValue to be a schemasnapshot.Column, got %T: %v", d.NewValue, d.NewValue)
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
	a := snapWithColumns(oldCol)
	b := snapWithColumns()

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
	if d.Object != ref("public", "orders") {
		t.Errorf("expected Object=public.orders, got %v", d.Object)
	}
	if d.SubObject != "legacy_field" {
		t.Errorf("expected SubObject='legacy_field', got %q", d.SubObject)
	}
	ov, ok := d.OldValue.(schemasnapshot.Column)
	if !ok {
		t.Fatalf("expected OldValue to be a schemasnapshot.Column, got %T: %v", d.OldValue, d.OldValue)
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
	for _, d := range got {
		assertAnchoredToObject(t, d)
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
	for _, d := range got {
		assertAnchoredToObject(t, d)
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
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	d := got[0]
	if d.Type != ColumnNullabilityChanged {
		t.Errorf("expected ColumnNullabilityChanged, got %v", d.Type)
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
	for _, d := range got {
		assertAnchoredToObject(t, d)
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
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	d := got[0]
	if d.Type != ColumnDefaultChanged {
		t.Errorf("expected ColumnDefaultChanged, got %v", d.Type)
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

	a := snapWithColumns(oldCol)
	b := snapWithColumns(newCol)

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
	for _, d := range got {
		assertAnchoredToObject(t, d)
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
	for _, d := range got2 {
		assertAnchoredToObject(t, d)
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
// Test: cross-engine gate — ID matching requires a.DatabaseType == b.DatabaseType.
// When it differs, matching falls back to (table, name), so same-ID+different-
// name produces ColumnDropped+ColumnAdded, not ColumnNameChanged.
// ──────────────────────────────────────────────────────────────────────────────

// TestDiffColumns_DifferentDatabaseType_RenameBecomesAddDrop: same column ID on
// both sides but different DatabaseType must NOT produce ColumnNameChanged —
// cross-type ID comparison is illegal, so matching falls back to name.
func TestDiffColumns_DifferentDatabaseType_RenameBecomesAddDrop(t *testing.T) {
	oldCol := makeColumn("public", "users", "200:1", "old_col", "text")
	newCol := makeColumn("public", "users", "200:1", "new_col", "text") // same ID, different name
	a := &schemasnapshot.SnapshotContent{
		Version:      1,
		DatabaseType: "postgresql",
		Columns:      []schemasnapshot.Column{oldCol},
	}
	b := &schemasnapshot.SnapshotContent{
		Version:      1,
		DatabaseType: "mysql",
		Columns:      []schemasnapshot.Column{newCol},
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
		if d.Type == ColumnDropped && d.SubObject == "old_col" {
			hasDropped = true
		}
		if d.Type == ColumnAdded && d.SubObject == "new_col" {
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
// Tests: suppressLifecycleTableColumns — per-column findings are suppressed when
// the parent table itself is wholly added or dropped.
// ──────────────────────────────────────────────────────────────────────────────

// TestDiff_TableAdded_SuppressesColumnAdds: when a table is wholly new (TABLE_ADDED),
// its per-column COLUMN_ADDED findings are redundant and must be suppressed.
func TestDiff_TableAdded_SuppressesColumnAdds(t *testing.T) {
	tbl := makeTable("200", "public", "orders", schemasnapshot.TableKindOrdinary)
	colID := makeColumn("public", "orders", "200:1", "id", "integer")
	colEmail := makeColumn("public", "orders", "200:2", "email", "text")

	a := snapWithTablesAndColumns(nil, nil)
	b := snapWithTablesAndColumns([]schemasnapshot.Table{tbl}, []schemasnapshot.Column{colID, colEmail})

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
	if got[0].Object != ref("public", "orders") {
		t.Errorf("expected Object=public.orders, got %v", got[0].Object)
	}
	// The suppressed columns must survive on the TABLE_ADDED finding's NewValue.
	cols, ok := got[0].NewValue.([]schemasnapshot.Column)
	if !ok {
		t.Fatalf("expected NewValue to be []schemasnapshot.Column, got %T: %v", got[0].NewValue, got[0].NewValue)
	}
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
// its per-column COLUMN_DROPPED findings are redundant and must be suppressed.
func TestDiff_TableDropped_SuppressesColumnDrops(t *testing.T) {
	tbl := makeTable("200", "public", "orders", schemasnapshot.TableKindOrdinary)
	colID := makeColumn("public", "orders", "200:1", "id", "integer")
	colEmail := makeColumn("public", "orders", "200:2", "email", "text")

	a := snapWithTablesAndColumns([]schemasnapshot.Table{tbl}, []schemasnapshot.Column{colID, colEmail})
	b := snapWithTablesAndColumns(nil, nil)

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
	if got[0].Object != ref("public", "orders") {
		t.Errorf("expected Object=public.orders, got %v", got[0].Object)
	}
	// The suppressed columns must survive on the TABLE_DROPPED finding's OldValue.
	cols, ok := got[0].OldValue.([]schemasnapshot.Column)
	if !ok {
		t.Fatalf("expected OldValue to be []schemasnapshot.Column, got %T: %v", got[0].OldValue, got[0].OldValue)
	}
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
	tbl := makeTable("200", "public", "orders", schemasnapshot.TableKindOrdinary)
	colID := makeColumn("public", "orders", "200:1", "id", "integer")
	colEmail := makeColumn("public", "orders", "200:2", "email", "text")

	// A: same table + only "id" column
	a := snapWithTablesAndColumns([]schemasnapshot.Table{tbl}, []schemasnapshot.Column{colID})
	// B: same table (matched) + both "id" and "email" columns
	b := snapWithTablesAndColumns([]schemasnapshot.Table{tbl}, []schemasnapshot.Column{colID, colEmail})

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
	if got[0].SubObject != "email" {
		t.Errorf("expected SubObject='email', got %q", got[0].SubObject)
	}
}

// TestDiff_ColumnDroppedFromExistingTable_NotSuppressed: when a column is dropped from a
// matched (existing) table, the COLUMN_DROPPED finding must NOT be suppressed.
func TestDiff_ColumnDroppedFromExistingTable_NotSuppressed(t *testing.T) {
	tbl := makeTable("200", "public", "orders", schemasnapshot.TableKindOrdinary)
	colID := makeColumn("public", "orders", "200:1", "id", "integer")
	colEmail := makeColumn("public", "orders", "200:2", "email", "text")

	// A: same table + both columns
	a := snapWithTablesAndColumns([]schemasnapshot.Table{tbl}, []schemasnapshot.Column{colID, colEmail})
	// B: same table (matched) + only "id" column
	b := snapWithTablesAndColumns([]schemasnapshot.Table{tbl}, []schemasnapshot.Column{colID})

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
	if got[0].SubObject != "email" {
		t.Errorf("expected SubObject='email', got %q", got[0].SubObject)
	}
}

// TestDiff_TableDropped_PreservesOtherTableColumnChanges guards against over-suppression:
// dropping table X must not suppress column findings on unrelated matched table Y.
func TestDiff_TableDropped_PreservesOtherTableColumnChanges(t *testing.T) {
	// Table X is dropped; table Y is present on both sides but has a column type change.
	tblX := makeTable("300", "public", "x", schemasnapshot.TableKindOrdinary)
	colXA := makeColumn("public", "x", "300:1", "a", "integer")

	tblY := makeTable("400", "public", "y", schemasnapshot.TableKindOrdinary)
	colYOld := makeColumn("public", "y", "400:1", "val", "integer")
	colYNew := makeColumn("public", "y", "400:1", "val", "text") // type changed

	a := snapWithTablesAndColumns(
		[]schemasnapshot.Table{tblX, tblY},
		[]schemasnapshot.Column{colXA, colYOld},
	)
	b := snapWithTablesAndColumns(
		[]schemasnapshot.Table{tblY},     // X dropped
		[]schemasnapshot.Column{colYNew}, // Y's column type changed
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
		if d.Type == TableDropped && d.Object.Name == "x" {
			hasTableDropped = true
		}
		if d.Type == ColumnTypeChanged && d.SubObject == "val" && d.Object.Name == "y" {
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
// Test: ID matches but parent table was renamed — match by ID, Object = side-A Column.Table
// ──────────────────────────────────────────────────────────────────────────────

func TestDiffColumns_IDMatchTableRenamed_ObjectIsOldTable(t *testing.T) {
	// Column with same ID but different Column.Table (parent was renamed)
	oldCol := makeColumn("public", "old_table", "500:1", "id", "integer")
	newCol := makeColumn("public", "new_table", "500:1", "id", "integer") // parent renamed

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
	for _, d := range got2 {
		assertAnchoredToObject(t, d)
	}
	d := got2[0]
	if d.Object != ref("public", "old_table") {
		t.Errorf("expected Object=public.old_table (side-A), got %v", d.Object)
	}
}

// IDMissingTableRenamed: without a stable ID, a column on a renamed parent table
// can't be tracked across the rename — the name-fallback key embeds the table
// name, so old/new keys differ — and degrades to COLUMN_DROPPED + COLUMN_ADDED.
// Contrast TestDiffColumns_IDMatchTableRenamed_ObjectIsOldTable, where a stable
// ID tracks the column across the rename.
func TestDiffColumns_IDMissingTableRenamed_BecomesDropAdd(t *testing.T) {
	oldCol := makeColumn("public", "old_table", "", "id", "integer") // empty ID
	newCol := makeColumn("public", "new_table", "", "id", "integer") // parent renamed, still empty ID

	got := Diff(snapWithColumns(oldCol), snapWithColumns(newCol))
	if len(got) != 2 {
		t.Fatalf("expected 2 findings (drop+add — rename untrackable without a stable ID), got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	objByType := map[DiffType]schemasnapshot.ObjectRef{}
	for _, d := range got {
		objByType[d.Type] = d.Object
	}
	if objByType[ColumnDropped] != ref("public", "old_table") {
		t.Errorf("COLUMN_DROPPED should anchor to public.old_table, got %v", objByType[ColumnDropped])
	}
	if objByType[ColumnAdded] != ref("public", "new_table") {
		t.Errorf("COLUMN_ADDED should anchor to public.new_table, got %v", objByType[ColumnAdded])
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

	a := snapWithColumns(cA)
	b := snapWithColumns(cB)

	got := Diff(a, b)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings (column reconciled by name despite missing ID in B), got %d: %v", len(got), got)
	}
}

// EmptyInAIDInB: symmetric — empty in A, ID in B → still reconciled by name → 0 findings.
func TestDiffColumns_HybridResidue_EmptyInAIDInB_Matched(t *testing.T) {
	cA := makeColumn("public", "orders", "", "qty", "integer")
	cB := makeColumn("public", "orders", "5:1", "qty", "integer")

	a := snapWithColumns(cA)
	b := snapWithColumns(cB)

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

	a := snapWithColumns(cA)
	b := snapWithColumns(cB)

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
	if got[0].SubObject != "qty" {
		t.Errorf("expected SubObject='qty', got %q", got[0].SubObject)
	}
	if got[0].Object != ref("public", "orders") {
		t.Errorf("expected Object=public.orders, got %v", got[0].Object)
	}
}

// DropRecreateSameNameDifferentID: same table+name, DIFFERENT non-empty IDs ("5:1"
// vs "5:2"), matchByID on. These are genuinely different columns (drop-and-recreate)
// and must NOT be collapsed — expect exactly ColumnDropped + ColumnAdded.
func TestDiffColumns_HybridResidue_DropRecreateSameNameDifferentID_NotCollapsed(t *testing.T) {
	cA := makeColumn("public", "orders", "5:1", "qty", "integer")
	cB := makeColumn("public", "orders", "5:2", "qty", "integer")

	a := snapWithColumns(cA)
	b := snapWithColumns(cB)

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
		if d.SubObject != "qty" {
			t.Errorf("expected SubObject='qty', got %q", d.SubObject)
		}
		if d.Object != ref("public", "orders") {
			t.Errorf("expected Object=public.orders, got %v", d.Object)
		}
	}
	if types[ColumnDropped] != 1 || types[ColumnAdded] != 1 {
		t.Errorf("expected exactly one ColumnDropped and one ColumnAdded, got %v", types)
	}
}
