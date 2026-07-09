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

// ──────────────────────────────────────────────────────────────────────────────
// Test: Table added (ID only in B)
// ──────────────────────────────────────────────────────────────────────────────

func TestDiffTables_TableAdded(t *testing.T) {
	a := snapWithTables()
	newTbl := makeTable("42", "public", "users", schemasnapshot.TableKindOrdinary)
	b := snapWithTables(newTbl)

	got := Diff(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 difference, got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	d := got[0]
	if d.Type != TableAdded {
		t.Errorf("expected TableAdded, got %v", d.Type)
	}
	if d.Object.Key != "public.users" {
		t.Errorf("expected Object.Key=public.users, got %v", d.Object.Key)
	}
	if d.OldValue != nil {
		t.Errorf("expected OldValue=nil, got %v", d.OldValue)
	}
	// No columns were added alongside the table (snapWithTables sets no Columns),
	// so NewValue must be an empty []schemasnapshot.Column.
	newCols, ok := d.NewValue.([]schemasnapshot.Column)
	if !ok {
		t.Fatalf("expected NewValue to be []schemasnapshot.Column, got %T: %v", d.NewValue, d.NewValue)
	}
	if len(newCols) != 0 {
		t.Errorf("expected NewValue to be empty, got %v", newCols)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: Table dropped (ID only in A)
// ──────────────────────────────────────────────────────────────────────────────

func TestDiffTables_TableDropped(t *testing.T) {
	oldTbl := makeTable("99", "public", "orders", schemasnapshot.TableKindOrdinary)
	a := snapWithTables(oldTbl)
	b := snapWithTables()

	got := Diff(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 difference, got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	d := got[0]
	if d.Type != TableDropped {
		t.Errorf("expected TableDropped, got %v", d.Type)
	}
	if d.Object.Key != "public.orders" {
		t.Errorf("expected Object.Key=public.orders, got %v", d.Object.Key)
	}
	// No columns existed on the dropped table (snapWithTables sets no Columns),
	// so OldValue must be an empty []schemasnapshot.Column.
	oldCols, ok := d.OldValue.([]schemasnapshot.Column)
	if !ok {
		t.Fatalf("expected OldValue to be []schemasnapshot.Column, got %T: %v", d.OldValue, d.OldValue)
	}
	if len(oldCols) != 0 {
		t.Errorf("expected OldValue to be empty, got %v", oldCols)
	}
	if d.NewValue != nil {
		t.Errorf("expected NewValue=nil, got %v", d.NewValue)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: Table added carries its columns, in original (attnum) order
// ──────────────────────────────────────────────────────────────────────────────

func TestDiffTables_TableAdded_CarriesColumnsInOrder(t *testing.T) {
	newTbl := makeTable("42", "public", "users", schemasnapshot.TableKindOrdinary)
	col1 := makeColumn("public", "users", "42:1", "id", "integer", notNull())
	col2 := makeColumn("public", "users", "42:2", "email", "text")
	col3 := makeColumn("public", "users", "42:3", "created_at", "timestamp", withDefault("now()"))
	newTbl.Columns = []schemasnapshot.Column{col1, col2, col3}

	a := snapWithTables()
	b := snapWithTables(newTbl)

	got := Diff(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 difference (TableAdded; per-column adds must be suppressed), got %d: %v", len(got), got)
	}
	d := got[0]
	if d.Type != TableAdded {
		t.Fatalf("expected TableAdded, got %v", d.Type)
	}
	if d.OldValue != nil {
		t.Errorf("expected OldValue=nil, got %v", d.OldValue)
	}
	cols, ok := d.NewValue.([]schemasnapshot.Column)
	if !ok {
		t.Fatalf("expected NewValue to be []schemasnapshot.Column, got %T: %v", d.NewValue, d.NewValue)
	}
	wantCols := []schemasnapshot.Column{col1, col2, col3}
	if len(cols) != len(wantCols) {
		t.Fatalf("expected %d columns, got %d: %v", len(wantCols), len(cols), cols)
	}
	for i := range wantCols {
		if cols[i] != wantCols[i] {
			t.Errorf("column order mismatch at index %d: expected %v, got %v", i, wantCols[i], cols[i])
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: Table dropped carries its columns, in original (attnum) order
// ──────────────────────────────────────────────────────────────────────────────

func TestDiffTables_TableDropped_CarriesColumnsInOrder(t *testing.T) {
	oldTbl := makeTable("99", "public", "orders", schemasnapshot.TableKindOrdinary)
	col1 := makeColumn("public", "orders", "99:1", "id", "integer", notNull())
	col2 := makeColumn("public", "orders", "99:2", "amount", "numeric(10,2)")
	col3 := makeColumn("public", "orders", "99:3", "status", "text", withDefault("'pending'"))
	oldTbl.Columns = []schemasnapshot.Column{col1, col2, col3}

	a := snapWithTables(oldTbl)
	b := snapWithTables()

	got := Diff(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 difference (TableDropped; per-column drops must be suppressed), got %d: %v", len(got), got)
	}
	d := got[0]
	if d.Type != TableDropped {
		t.Fatalf("expected TableDropped, got %v", d.Type)
	}
	if d.NewValue != nil {
		t.Errorf("expected NewValue=nil, got %v", d.NewValue)
	}
	cols, ok := d.OldValue.([]schemasnapshot.Column)
	if !ok {
		t.Fatalf("expected OldValue to be []schemasnapshot.Column, got %T: %v", d.OldValue, d.OldValue)
	}
	wantCols := []schemasnapshot.Column{col1, col2, col3}
	if len(cols) != len(wantCols) {
		t.Fatalf("expected %d columns, got %d: %v", len(wantCols), len(cols), cols)
	}
	for i := range wantCols {
		if cols[i] != wantCols[i] {
			t.Errorf("column order mismatch at index %d: expected %v, got %v", i, wantCols[i], cols[i])
		}
	}
}

// TestCloneColumns_Independence verifies cloneColumns decouples the Difference's
// column slice from the source snapshot: mutating the returned slice must not
// write through into the snapshot's original Columns.
func TestCloneColumns_Independence(t *testing.T) {
	newTbl := makeTable("42", "public", "users", schemasnapshot.TableKindOrdinary)
	col1 := makeColumn("public", "users", "42:1", "id", "integer")
	newTbl.Columns = []schemasnapshot.Column{col1}

	a := snapWithTables()
	b := snapWithTables(newTbl)

	got := Diff(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 difference, got %d: %v", len(got), got)
	}
	cols, ok := got[0].NewValue.([]schemasnapshot.Column)
	if !ok || len(cols) != 1 {
		t.Fatalf("expected NewValue to be a 1-element []schemasnapshot.Column, got %T: %v", got[0].NewValue, got[0].NewValue)
	}
	cols[0].Name = "mutated"
	if b.Tables[0].Columns[0].Name != "id" {
		t.Errorf("mutating the returned column slice must not affect the source snapshot; source became %v", b.Tables[0].Columns[0].Name)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: Table renamed — single TableNameChanged, no TableAdded/TableDropped
// ──────────────────────────────────────────────────────────────────────────────

func TestDiffTables_TableRenamed(t *testing.T) {
	oldTbl := makeTable("55", "public", "old_name", schemasnapshot.TableKindOrdinary)
	newTbl := makeTable("55", "public", "new_name", schemasnapshot.TableKindOrdinary)
	a := snapWithTables(oldTbl)
	b := snapWithTables(newTbl)

	got := Diff(a, b)

	// Must have exactly ONE finding
	if len(got) != 1 {
		t.Fatalf("expected 1 difference, got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	d := got[0]
	if d.Type != TableNameChanged {
		t.Errorf("expected TableNameChanged, got %v", d.Type)
	}
	if d.Object.Key != "public.old_name" {
		t.Errorf("expected Object.Key=public.old_name (old ref), got %v", d.Object.Key)
	}
	if d.OldValue.(string) != "old_name" {
		t.Errorf("expected OldValue='old_name', got %v", d.OldValue)
	}
	if d.NewValue.(string) != "new_name" {
		t.Errorf("expected NewValue='new_name', got %v", d.NewValue)
	}

	// No TableAdded or TableDropped
	for _, diff := range got {
		if diff.Type == TableAdded || diff.Type == TableDropped {
			t.Errorf("unexpected %v in result", diff.Type)
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: Table schema moved
// ──────────────────────────────────────────────────────────────────────────────

func TestDiffTables_TableSchemaMoved(t *testing.T) {
	oldTbl := makeTable("77", "old_schema", "my_table", schemasnapshot.TableKindOrdinary)
	newTbl := makeTable("77", "new_schema", "my_table", schemasnapshot.TableKindOrdinary)
	a := snapWithTables(oldTbl)
	b := snapWithTables(newTbl)

	got := Diff(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 difference, got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	d := got[0]
	if d.Type != TableSchemaChanged {
		t.Errorf("expected TableSchemaChanged, got %v", d.Type)
	}
	if d.Object.Key != "old_schema.my_table" {
		t.Errorf("expected Object.Key=old_schema.my_table, got %v", d.Object.Key)
	}
	if d.OldValue.(string) != "old_schema" {
		t.Errorf("expected OldValue='old_schema', got %v", d.OldValue)
	}
	if d.NewValue.(string) != "new_schema" {
		t.Errorf("expected NewValue='new_schema', got %v", d.NewValue)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: Table kind changed
// ──────────────────────────────────────────────────────────────────────────────

func TestDiffTables_TableKindChanged(t *testing.T) {
	oldTbl := makeTable("88", "public", "my_table", schemasnapshot.TableKindOrdinary)
	newTbl := makeTable("88", "public", "my_table", schemasnapshot.TableKindPartitioned)
	a := snapWithTables(oldTbl)
	b := snapWithTables(newTbl)

	got := Diff(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 difference, got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	d := got[0]
	if d.Type != TableKindChanged {
		t.Errorf("expected TableKindChanged, got %v", d.Type)
	}
	// Kind is stored as string
	if d.OldValue.(string) != string(schemasnapshot.TableKindOrdinary) {
		t.Errorf("expected OldValue='ordinary', got %v", d.OldValue)
	}
	if d.NewValue.(string) != string(schemasnapshot.TableKindPartitioned) {
		t.Errorf("expected NewValue='partitioned', got %v", d.NewValue)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: PartitionParent changed — nil→set, set→nil, set→different
// ──────────────────────────────────────────────────────────────────────────────

func TestDiffTables_PartitionParent_NilToSet(t *testing.T) {
	oldTbl := makeTable("10", "public", "part", schemasnapshot.TableKindOrdinary)
	// old has no PartitionParent
	newTbl := oldTbl
	newTbl.PartitionParent = refPtr("public", "parent_table")

	a := snapWithTables(oldTbl)
	b := snapWithTables(newTbl)

	got := Diff(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 difference, got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	d := got[0]
	if d.Type != TablePartitionParentChanged {
		t.Errorf("expected TablePartitionParentChanged, got %v", d.Type)
	}
	if d.OldValue != nil {
		t.Errorf("expected OldValue=nil, got %v", d.OldValue)
	}
	if d.NewValue == nil {
		t.Errorf("expected NewValue non-nil")
	}
}

func TestDiffTables_PartitionParent_SetToNil(t *testing.T) {
	oldTbl := makeTable("11", "public", "part", schemasnapshot.TableKindOrdinary)
	oldTbl.PartitionParent = refPtr("public", "parent_table")
	newTbl := makeTable("11", "public", "part", schemasnapshot.TableKindOrdinary)
	// new has no PartitionParent

	a := snapWithTables(oldTbl)
	b := snapWithTables(newTbl)

	got := Diff(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 difference, got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	d := got[0]
	if d.Type != TablePartitionParentChanged {
		t.Errorf("expected TablePartitionParentChanged, got %v", d.Type)
	}
	if d.OldValue == nil {
		t.Errorf("expected OldValue non-nil")
	}
	if d.NewValue != nil {
		t.Errorf("expected NewValue=nil, got %v", d.NewValue)
	}
}

func TestDiffTables_PartitionParent_SetToDifferent(t *testing.T) {
	oldTbl := makeTable("12", "public", "part", schemasnapshot.TableKindOrdinary)
	oldTbl.PartitionParent = refPtr("public", "parent_a")
	newTbl := makeTable("12", "public", "part", schemasnapshot.TableKindOrdinary)
	newTbl.PartitionParent = refPtr("public", "parent_b")

	a := snapWithTables(oldTbl)
	b := snapWithTables(newTbl)

	got := Diff(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 difference, got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	d := got[0]
	if d.Type != TablePartitionParentChanged {
		t.Errorf("expected TablePartitionParentChanged, got %v", d.Type)
	}
	if d.OldValue.(schemasnapshot.ObjectRef) != ref("public", "parent_a") {
		t.Errorf("unexpected OldValue %v", d.OldValue)
	}
	if d.NewValue.(schemasnapshot.ObjectRef) != ref("public", "parent_b") {
		t.Errorf("unexpected NewValue %v", d.NewValue)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: PartitionChildren changed — add member, remove member, order-insensitive
// ──────────────────────────────────────────────────────────────────────────────

func TestDiffTables_PartitionChildren_MemberAdded(t *testing.T) {
	oldTbl := makeTable("20", "public", "parent", schemasnapshot.TableKindPartitioned)
	oldTbl.PartitionChildren = []schemasnapshot.ObjectRef{ref("public", "child1")}
	newTbl := makeTable("20", "public", "parent", schemasnapshot.TableKindPartitioned)
	newTbl.PartitionChildren = []schemasnapshot.ObjectRef{ref("public", "child1"), ref("public", "child2")}

	a := snapWithTables(oldTbl)
	b := snapWithTables(newTbl)

	got := Diff(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 difference, got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	if got[0].Type != TablePartitionChildrenChanged {
		t.Errorf("expected TablePartitionChildrenChanged, got %v", got[0].Type)
	}
}

func TestDiffTables_PartitionChildren_MemberRemoved(t *testing.T) {
	oldTbl := makeTable("21", "public", "parent", schemasnapshot.TableKindPartitioned)
	oldTbl.PartitionChildren = []schemasnapshot.ObjectRef{ref("public", "child1"), ref("public", "child2")}
	newTbl := makeTable("21", "public", "parent", schemasnapshot.TableKindPartitioned)
	newTbl.PartitionChildren = []schemasnapshot.ObjectRef{ref("public", "child1")}

	a := snapWithTables(oldTbl)
	b := snapWithTables(newTbl)

	got := Diff(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 difference, got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	if got[0].Type != TablePartitionChildrenChanged {
		t.Errorf("expected TablePartitionChildrenChanged, got %v", got[0].Type)
	}
}

func TestDiffTables_PartitionChildren_OrderInsensitive(t *testing.T) {
	// Same members, different order → NO finding
	oldTbl := makeTable("22", "public", "parent", schemasnapshot.TableKindPartitioned)
	oldTbl.PartitionChildren = []schemasnapshot.ObjectRef{ref("public", "child1"), ref("public", "child2")}
	newTbl := makeTable("22", "public", "parent", schemasnapshot.TableKindPartitioned)
	newTbl.PartitionChildren = []schemasnapshot.ObjectRef{ref("public", "child2"), ref("public", "child1")}

	a := snapWithTables(oldTbl)
	b := snapWithTables(newTbl)

	got := Diff(a, b)
	if len(got) != 0 {
		t.Errorf("expected no differences (order-insensitive), got %d: %v", len(got), got)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: InheritsFrom changed
// ──────────────────────────────────────────────────────────────────────────────

func TestDiffTables_InheritsFromChanged(t *testing.T) {
	oldTbl := makeTable("30", "public", "child", schemasnapshot.TableKindOrdinary)
	oldTbl.InheritsFrom = []schemasnapshot.ObjectRef{ref("public", "parent_a")}
	newTbl := makeTable("30", "public", "child", schemasnapshot.TableKindOrdinary)
	newTbl.InheritsFrom = []schemasnapshot.ObjectRef{ref("public", "parent_a"), ref("public", "parent_b")}

	a := snapWithTables(oldTbl)
	b := snapWithTables(newTbl)

	got := Diff(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 difference, got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	if got[0].Type != TableInheritsChanged {
		t.Errorf("expected TableInheritsChanged, got %v", got[0].Type)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: InheritedBy changed
// ──────────────────────────────────────────────────────────────────────────────

func TestDiffTables_InheritedByChanged(t *testing.T) {
	oldTbl := makeTable("31", "public", "parent", schemasnapshot.TableKindOrdinary)
	oldTbl.InheritedBy = []schemasnapshot.ObjectRef{ref("public", "child_a")}
	newTbl := makeTable("31", "public", "parent", schemasnapshot.TableKindOrdinary)
	// InheritedBy is now empty

	a := snapWithTables(oldTbl)
	b := snapWithTables(newTbl)

	got := Diff(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 difference, got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	if got[0].Type != TableInheritedByChanged {
		t.Errorf("expected TableInheritedByChanged, got %v", got[0].Type)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Tests: cross-engine gate — ID-based matching only when DatabaseType matches.
// ──────────────────────────────────────────────────────────────────────────────

// crossEngineSnapWithTables builds a SnapshotContent with DatabaseType="mysql" for
// use in tests that need two snapshots with differing DatabaseType values to verify
// that ID-based matching is disabled across engines.
func crossEngineSnapWithTables(tables ...schemasnapshot.Table) *schemasnapshot.SnapshotContent {
	return &schemasnapshot.SnapshotContent{
		Version:      1,
		DatabaseType: "mysql",
		Tables:       tables,
	}
}

// Note: TestDiffTables_UnstableIdentity_RenameBecomesAddDrop and
// TestDiffTables_UnstableIdentity_SameNameMatches were deleted — "same
// DatabaseType but StableIdentity=false" is unexpressible after the API
// refactor (gate is now DatabaseType equality). Cross-engine add+drop is
// covered by TestDiffTables_DifferentDatabaseType_RenameBecomesAddDrop.

// TestDiffTables_MixedDatabaseType_FallsBackToName: a="postgresql", b="mysql"
// (or vice-versa). The gate is equality, so a rename (same ID, different name)
// must produce add+drop, not TableNameChanged.
func TestDiffTables_MixedDatabaseType_FallsBackToName(t *testing.T) {
	oldTbl := makeTable("77", "public", "old_name", schemasnapshot.TableKindOrdinary)
	newTbl := makeTable("77", "public", "new_name", schemasnapshot.TableKindOrdinary)

	// a=postgresql, b=mysql
	a := &schemasnapshot.SnapshotContent{Version: 1, DatabaseType: "postgresql", Tables: []schemasnapshot.Table{oldTbl}}
	b := &schemasnapshot.SnapshotContent{Version: 1, DatabaseType: "mysql", Tables: []schemasnapshot.Table{newTbl}}

	got := Diff(a, b)

	if len(got) != 2 {
		t.Fatalf("mixed db type: expected 2 differences (TableDropped+TableAdded), got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	for _, d := range got {
		if d.Type == TableNameChanged {
			t.Errorf("mixed db type: unexpected TableNameChanged; got: %v", got)
		}
	}

	// Also verify the symmetric case: a=mysql, b=postgresql
	a2 := &schemasnapshot.SnapshotContent{Version: 1, DatabaseType: "mysql", Tables: []schemasnapshot.Table{oldTbl}}
	b2 := &schemasnapshot.SnapshotContent{Version: 1, DatabaseType: "postgresql", Tables: []schemasnapshot.Table{newTbl}}

	got2 := Diff(a2, b2)
	if len(got2) != 2 {
		t.Fatalf("mixed db type (reversed): expected 2 differences (TableDropped+TableAdded), got %d: %v", len(got2), got2)
	}
	for _, d := range got2 {
		assertAnchoredToObject(t, d)
	}
	for _, d := range got2 {
		if d.Type == TableNameChanged {
			t.Errorf("mixed db type (reversed): unexpected TableNameChanged; got: %v", got2)
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: cross-engine gate — ID matching requires a.DatabaseType == b.DatabaseType.
// When it differs, matching falls back to name, so same-ID+different-name
// produces TableDropped+TableAdded, not TableNameChanged.
// ──────────────────────────────────────────────────────────────────────────────

// TestDiffTables_DifferentDatabaseType_RenameBecomesAddDrop: same table ID on
// both sides but different DatabaseType must NOT produce TableNameChanged —
// cross-type ID comparison is illegal, so matching falls back to name.
func TestDiffTables_DifferentDatabaseType_RenameBecomesAddDrop(t *testing.T) {
	oldTbl := makeTable("55", "public", "old_name", schemasnapshot.TableKindOrdinary)
	newTbl := makeTable("55", "public", "new_name", schemasnapshot.TableKindOrdinary)
	a := &schemasnapshot.SnapshotContent{
		Version:      1,
		DatabaseType: "postgresql",
		Tables:       []schemasnapshot.Table{oldTbl},
	}
	b := &schemasnapshot.SnapshotContent{
		Version:      1,
		DatabaseType: "mysql",
		Tables:       []schemasnapshot.Table{newTbl},
	}

	got := Diff(a, b)

	// Must NOT produce TableNameChanged — cross-type ID matching is illegal.
	// Must produce TableDropped(old_name) + TableAdded(new_name).
	if len(got) != 2 {
		t.Fatalf("expected 2 differences (TableDropped+TableAdded), got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	var hasDropped, hasAdded bool
	for _, d := range got {
		if d.Type == TableNameChanged {
			t.Errorf("unexpected TableNameChanged when DatabaseType differs: %v", d)
		}
		if d.Type == TableDropped && d.Object.Key == "public.old_name" {
			hasDropped = true
		}
		if d.Type == TableAdded && d.Object.Key == "public.new_name" {
			hasAdded = true
		}
	}
	if !hasDropped {
		t.Errorf("expected TableDropped for public.old_name; got: %v", got)
	}
	if !hasAdded {
		t.Errorf("expected TableAdded for public.new_name; got: %v", got)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: ID-empty fallback — matched by schema.name; name-only-in-A → TableDropped
// ──────────────────────────────────────────────────────────────────────────────

func TestDiffTables_IDEmptyFallback_MatchedByName(t *testing.T) {
	// Table with empty ID, matched by name → should produce NO finding if identical
	tblA := schemasnapshot.Table{
		ObjectRef: ref("public", "shared"),
		ID:        "",
		Kind:      schemasnapshot.TableKindOrdinary,
	}
	tblB := schemasnapshot.Table{
		ObjectRef: ref("public", "shared"),
		ID:        "",
		Kind:      schemasnapshot.TableKindOrdinary,
	}
	// Table only in A (ID="") → TableDropped
	tblOnlyInA := schemasnapshot.Table{
		ObjectRef: ref("public", "gone_table"),
		ID:        "",
		Kind:      schemasnapshot.TableKindOrdinary,
	}

	a := snapWithTables(tblA, tblOnlyInA)
	b := snapWithTables(tblB)

	got := Diff(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 difference (TableDropped for gone_table), got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	d := got[0]
	if d.Type != TableDropped {
		t.Errorf("expected TableDropped, got %v", d.Type)
	}
	if d.Object.Key != "public.gone_table" {
		t.Errorf("expected Object.Key=public.gone_table, got %v", d.Object.Key)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: hybrid ID-then-name matching. A table with a stable ID on one side but
// empty on the other must reconcile by name (not a spurious drop+add); a
// genuine drop-and-recreate reusing a name with a DIFFERENT id stays add+drop.
// ──────────────────────────────────────────────────────────────────────────────

// IDInAEmptyInB: same table, ID "123" in A but empty in B; identical otherwise.
// The hybrid residue pass must reconcile them by name → no findings.
func TestDiffTables_HybridResidue_IDInAEmptyInB_Matched(t *testing.T) {
	a := snapWithTables(makeTable("123", "public", "t", schemasnapshot.TableKindOrdinary))
	b := snapWithTables(makeTable("", "public", "t", schemasnapshot.TableKindOrdinary))

	got := Diff(a, b)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings (table reconciled by name despite missing ID in B), got %d: %v", len(got), got)
	}
}

// EmptyInAIDInB: the symmetric case — empty in A, ID in B.
func TestDiffTables_HybridResidue_EmptyInAIDInB_Matched(t *testing.T) {
	a := snapWithTables(makeTable("", "public", "t", schemasnapshot.TableKindOrdinary))
	b := snapWithTables(makeTable("456", "public", "t", schemasnapshot.TableKindOrdinary))

	got := Diff(a, b)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings (table reconciled by name despite missing ID in A), got %d: %v", len(got), got)
	}
}

// MixedID with a real property change: reconciled by name AND the change
// surfaces as a single TABLE_KIND_CHANGED (proving it matched, not drop+add).
func TestDiffTables_HybridResidue_MixedID_PropertyChangeSurfaces(t *testing.T) {
	a := snapWithTables(makeTable("123", "public", "t", schemasnapshot.TableKindOrdinary))
	b := snapWithTables(makeTable("", "public", "t", schemasnapshot.TableKindPartitioned))

	got := Diff(a, b)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 finding (TABLE_KIND_CHANGED on the reconciled table), got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	if got[0].Type != TableKindChanged {
		t.Errorf("expected TableKindChanged, got %v", got[0].Type)
	}
	if got[0].Object.Key != "public.t" {
		t.Errorf("expected Object.Key=public.t, got %v", got[0].Object.Key)
	}
}

// Drop-and-recreate guard: same name, DIFFERENT id, both ids present, matchByID
// on. These are genuinely different objects and must NOT be collapsed into a
// match — the residue name-match is only a fallback for tables lacking an id.
func TestDiffTables_HybridResidue_DropRecreateSameNameDifferentID_NotCollapsed(t *testing.T) {
	a := snapWithTables(makeTable("1", "public", "foo", schemasnapshot.TableKindOrdinary))
	b := snapWithTables(makeTable("2", "public", "foo", schemasnapshot.TableKindOrdinary))

	got := Diff(a, b)
	if len(got) != 2 {
		t.Fatalf("expected 2 findings (TABLE_DROPPED + TABLE_ADDED for distinct OIDs), got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}
	types := map[DiffType]int{}
	for _, d := range got {
		types[d.Type]++
		if d.Object.Key != "public.foo" {
			t.Errorf("expected Object.Key=public.foo, got %v", d.Object.Key)
		}
	}
	if types[TableDropped] != 1 || types[TableAdded] != 1 {
		t.Errorf("expected exactly one TableDropped and one TableAdded, got %v", types)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: link-change OldValue/NewValue slices are independent of the input
// snapshots (a consumer mutating a returned []ObjectRef must not reach back
// into the source snapshot's slice).
// ──────────────────────────────────────────────────────────────────────────────

func TestDiffTables_LinkSlicesAreDefensivelyCopied(t *testing.T) {
	oldTbl := makeTable("70", "public", "t", schemasnapshot.TableKindPartitioned)
	oldTbl.PartitionChildren = []schemasnapshot.ObjectRef{ref("public", "child1")}
	oldTbl.InheritsFrom = []schemasnapshot.ObjectRef{ref("public", "parent_a")}
	oldTbl.InheritedBy = []schemasnapshot.ObjectRef{ref("public", "sub_a")}

	newTbl := makeTable("70", "public", "t", schemasnapshot.TableKindPartitioned)
	newTbl.PartitionChildren = []schemasnapshot.ObjectRef{ref("public", "child1"), ref("public", "child2")}
	newTbl.InheritsFrom = []schemasnapshot.ObjectRef{ref("public", "parent_a"), ref("public", "parent_b")}
	newTbl.InheritedBy = []schemasnapshot.ObjectRef{ref("public", "sub_a"), ref("public", "sub_b")}

	a := snapWithTables(oldTbl)
	b := snapWithTables(newTbl)

	got := Diff(a, b)
	if len(got) != 3 {
		t.Fatalf("expected 3 link-change findings, got %d: %v", len(got), got)
	}
	for _, d := range got {
		assertAnchoredToObject(t, d)
	}

	// Snapshot the source slices' first elements so we can detect write-through.
	wantOldA := a.Tables[0].PartitionChildren[0]
	wantNewB := b.Tables[0].PartitionChildren[0]
	wantInhA := a.Tables[0].InheritsFrom[0]
	wantInhB := b.Tables[0].InheritsFrom[0]
	wantBesA := a.Tables[0].InheritedBy[0]
	wantBesB := b.Tables[0].InheritedBy[0]

	// Mutate every returned link slice's first element through the any-typed value.
	for _, d := range got {
		if s, ok := d.OldValue.([]schemasnapshot.ObjectRef); ok && len(s) > 0 {
			s[0] = ref("MUTATED", "MUTATED")
		}
		if s, ok := d.NewValue.([]schemasnapshot.ObjectRef); ok && len(s) > 0 {
			s[0] = ref("MUTATED", "MUTATED")
		}
	}

	// The source snapshots must be untouched.
	if a.Tables[0].PartitionChildren[0] != wantOldA {
		t.Errorf("PartitionChildren OldValue aliased input: source A mutated to %v", a.Tables[0].PartitionChildren[0])
	}
	if b.Tables[0].PartitionChildren[0] != wantNewB {
		t.Errorf("PartitionChildren NewValue aliased input: source B mutated to %v", b.Tables[0].PartitionChildren[0])
	}
	if a.Tables[0].InheritsFrom[0] != wantInhA {
		t.Errorf("InheritsFrom OldValue aliased input: source A mutated to %v", a.Tables[0].InheritsFrom[0])
	}
	if b.Tables[0].InheritsFrom[0] != wantInhB {
		t.Errorf("InheritsFrom NewValue aliased input: source B mutated to %v", b.Tables[0].InheritsFrom[0])
	}
	if a.Tables[0].InheritedBy[0] != wantBesA {
		t.Errorf("InheritedBy OldValue aliased input: source A mutated to %v", a.Tables[0].InheritedBy[0])
	}
	if b.Tables[0].InheritedBy[0] != wantBesB {
		t.Errorf("InheritedBy NewValue aliased input: source B mutated to %v", b.Tables[0].InheritedBy[0])
	}
}
