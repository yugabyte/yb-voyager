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
	d := got[0]
	if d.Type != TableAdded {
		t.Errorf("expected TableAdded, got %v", d.Type)
	}
	if d.Object != ref("public", "users") {
		t.Errorf("expected Object=public.users, got %v", d.Object)
	}
	if d.AnchorTable == nil || *d.AnchorTable != ref("public", "users") {
		t.Errorf("expected AnchorTable=public.users, got %v", d.AnchorTable)
	}
	if d.OldValue != nil {
		t.Errorf("expected OldValue=nil, got %v", d.OldValue)
	}
	if d.NewValue != nil {
		t.Errorf("expected NewValue=nil, got %v", d.NewValue)
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
	d := got[0]
	if d.Type != TableDropped {
		t.Errorf("expected TableDropped, got %v", d.Type)
	}
	if d.Object != ref("public", "orders") {
		t.Errorf("expected Object=public.orders, got %v", d.Object)
	}
	if d.AnchorTable == nil || *d.AnchorTable != ref("public", "orders") {
		t.Errorf("expected AnchorTable=public.orders, got %v", d.AnchorTable)
	}
	if d.OldValue != nil {
		t.Errorf("expected OldValue=nil, got %v", d.OldValue)
	}
	if d.NewValue != nil {
		t.Errorf("expected NewValue=nil, got %v", d.NewValue)
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
	d := got[0]
	if d.Type != TableNameChanged {
		t.Errorf("expected TableNameChanged, got %v", d.Type)
	}
	if d.Object != ref("public", "old_name") {
		t.Errorf("expected Object=public.old_name (old ref), got %v", d.Object)
	}
	if d.Property != "name" {
		t.Errorf("expected Property='name', got %q", d.Property)
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
	d := got[0]
	if d.Type != TableSchemaChanged {
		t.Errorf("expected TableSchemaChanged, got %v", d.Type)
	}
	if d.Object != ref("old_schema", "my_table") {
		t.Errorf("expected Object=old_schema.my_table, got %v", d.Object)
	}
	if d.Property != "schema" {
		t.Errorf("expected Property='schema', got %q", d.Property)
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
	d := got[0]
	if d.Type != TableKindChanged {
		t.Errorf("expected TableKindChanged, got %v", d.Type)
	}
	if d.Property != "kind" {
		t.Errorf("expected Property='kind', got %q", d.Property)
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
	d := got[0]
	if d.Type != PartitionParentChanged {
		t.Errorf("expected PartitionParentChanged, got %v", d.Type)
	}
	if d.Property != "partition_parent" {
		t.Errorf("expected Property='partition_parent', got %q", d.Property)
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
	d := got[0]
	if d.Type != PartitionParentChanged {
		t.Errorf("expected PartitionParentChanged, got %v", d.Type)
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
	d := got[0]
	if d.Type != PartitionParentChanged {
		t.Errorf("expected PartitionParentChanged, got %v", d.Type)
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
	if got[0].Type != PartitionChildrenChanged {
		t.Errorf("expected PartitionChildrenChanged, got %v", got[0].Type)
	}
	if got[0].Property != "partition_children" {
		t.Errorf("expected Property='partition_children', got %q", got[0].Property)
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
	if got[0].Type != PartitionChildrenChanged {
		t.Errorf("expected PartitionChildrenChanged, got %v", got[0].Type)
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
	if got[0].Type != TableInheritsChanged {
		t.Errorf("expected TableInheritsChanged, got %v", got[0].Type)
	}
	if got[0].Property != "inherits_from" {
		t.Errorf("expected Property='inherits_from', got %q", got[0].Property)
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
	if got[0].Type != TableInheritedByChanged {
		t.Errorf("expected TableInheritedByChanged, got %v", got[0].Type)
	}
	if got[0].Property != "inherited_by" {
		t.Errorf("expected Property='inherited_by', got %q", got[0].Property)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Tests: StableIdentity gate — ID-based matching only when BOTH snapshots declare
// StableIdentity: true. When either is false, fall back to name-based matching.
// ──────────────────────────────────────────────────────────────────────────────

// unstableSnapWithTables builds a snapshot with StableIdentity:false.
func unstableSnapWithTables(tables ...schemasnapshot.Table) *schemasnapshot.SchemaSnapshot {
	return &schemasnapshot.SchemaSnapshot{
		Version:        1,
		StableIdentity: false,
		Tables:         tables,
	}
}

// TestDiffTables_UnstableIdentity_RenameBecomesAddDrop: when both snapshots have
// StableIdentity:false, objects with same ID but different names must NOT be
// rename-detected. Matching falls back to name, producing TableDropped (old name) +
// TableAdded (new name).
func TestDiffTables_UnstableIdentity_RenameBecomesAddDrop(t *testing.T) {
	oldTbl := makeTable("55", "public", "old_name", schemasnapshot.TableKindOrdinary)
	newTbl := makeTable("55", "public", "new_name", schemasnapshot.TableKindOrdinary)
	a := unstableSnapWithTables(oldTbl)
	b := unstableSnapWithTables(newTbl)

	got := Diff(a, b)

	// Expect TableDropped(old_name) + TableAdded(new_name) — no TableNameChanged.
	if len(got) != 2 {
		t.Fatalf("expected 2 differences (TableDropped+TableAdded), got %d: %v", len(got), got)
	}
	var hasDropped, hasAdded bool
	for _, d := range got {
		if d.Type == TableNameChanged {
			t.Errorf("unexpected TableNameChanged when StableIdentity=false: %v", d)
		}
		if d.Type == TableDropped && d.Object == ref("public", "old_name") {
			hasDropped = true
		}
		if d.Type == TableAdded && d.Object == ref("public", "new_name") {
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

// TestDiffTables_UnstableIdentity_SameNameMatches: same ID AND same name, both
// StableIdentity:false → tables match by name → no findings.
func TestDiffTables_UnstableIdentity_SameNameMatches(t *testing.T) {
	tbl := makeTable("55", "public", "orders", schemasnapshot.TableKindOrdinary)
	a := unstableSnapWithTables(tbl)
	b := unstableSnapWithTables(tbl)

	got := Diff(a, b)
	if len(got) != 0 {
		t.Errorf("expected no differences for unstable-identity snapshots with same-named tables, got %d: %v", len(got), got)
	}
}

// TestDiffTables_MixedStability_FallsBackToName: a.StableIdentity=true but
// b.StableIdentity=false (or vice-versa). The gate is AND, so a rename (same ID,
// different name) must produce add+drop, not TableNameChanged.
func TestDiffTables_MixedStability_FallsBackToName(t *testing.T) {
	oldTbl := makeTable("77", "public", "old_name", schemasnapshot.TableKindOrdinary)
	newTbl := makeTable("77", "public", "new_name", schemasnapshot.TableKindOrdinary)

	// a=stable, b=unstable
	a := &schemasnapshot.SchemaSnapshot{Version: 1, StableIdentity: true, Tables: []schemasnapshot.Table{oldTbl}}
	b := &schemasnapshot.SchemaSnapshot{Version: 1, StableIdentity: false, Tables: []schemasnapshot.Table{newTbl}}

	got := Diff(a, b)

	if len(got) != 2 {
		t.Fatalf("mixed stability: expected 2 differences (TableDropped+TableAdded), got %d: %v", len(got), got)
	}
	for _, d := range got {
		if d.Type == TableNameChanged {
			t.Errorf("mixed stability: unexpected TableNameChanged; got: %v", got)
		}
	}

	// Also verify the symmetric case: a=unstable, b=stable
	a2 := &schemasnapshot.SchemaSnapshot{Version: 1, StableIdentity: false, Tables: []schemasnapshot.Table{oldTbl}}
	b2 := &schemasnapshot.SchemaSnapshot{Version: 1, StableIdentity: true, Tables: []schemasnapshot.Table{newTbl}}

	got2 := Diff(a2, b2)
	if len(got2) != 2 {
		t.Fatalf("mixed stability (reversed): expected 2 differences (TableDropped+TableAdded), got %d: %v", len(got2), got2)
	}
	for _, d := range got2 {
		if d.Type == TableNameChanged {
			t.Errorf("mixed stability (reversed): unexpected TableNameChanged; got: %v", got2)
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test: Different DatabaseType gate — ID comparison is legal only when
// a.DatabaseType == b.DatabaseType AND both StableIdentity=true.
// When DatabaseType differs, ID matching must be skipped and matching falls
// back to name, so same-ID+different-Name produces TableDropped+TableAdded
// instead of TableNameChanged.
// ──────────────────────────────────────────────────────────────────────────────

// TestDiffTables_DifferentDatabaseType_RenameBecomesAddDrop: snapshots with the
// same table ID and StableIdentity:true on both sides, but different DatabaseType,
// must NOT produce TableNameChanged. IDs are only comparable within one database
// type; cross-type ID comparison is illegal, so matching falls back to name.
func TestDiffTables_DifferentDatabaseType_RenameBecomesAddDrop(t *testing.T) {
	oldTbl := makeTable("55", "public", "old_name", schemasnapshot.TableKindOrdinary)
	newTbl := makeTable("55", "public", "new_name", schemasnapshot.TableKindOrdinary)
	a := &schemasnapshot.SchemaSnapshot{
		Version:        1,
		DatabaseType:   "postgresql",
		StableIdentity: true,
		Tables:         []schemasnapshot.Table{oldTbl},
	}
	b := &schemasnapshot.SchemaSnapshot{
		Version:        1,
		DatabaseType:   "mysql",
		StableIdentity: true,
		Tables:         []schemasnapshot.Table{newTbl},
	}

	got := Diff(a, b)

	// Must NOT produce TableNameChanged — cross-type ID matching is illegal.
	// Must produce TableDropped(old_name) + TableAdded(new_name).
	if len(got) != 2 {
		t.Fatalf("expected 2 differences (TableDropped+TableAdded), got %d: %v", len(got), got)
	}
	var hasDropped, hasAdded bool
	for _, d := range got {
		if d.Type == TableNameChanged {
			t.Errorf("unexpected TableNameChanged when DatabaseType differs: %v", d)
		}
		if d.Type == TableDropped && d.Object == ref("public", "old_name") {
			hasDropped = true
		}
		if d.Type == TableAdded && d.Object == ref("public", "new_name") {
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
	d := got[0]
	if d.Type != TableDropped {
		t.Errorf("expected TableDropped, got %v", d.Type)
	}
	if d.Object != ref("public", "gone_table") {
		t.Errorf("expected Object=public.gone_table, got %v", d.Object)
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
