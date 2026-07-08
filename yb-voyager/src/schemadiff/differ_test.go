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

// ─── TestNewDiffer_ReturnsUsableDiffer ────────────────────────────────────────
// NewDiffer returns a non-nil *Differ and its Diff works on empty snapshots.

func TestNewDiffer_ReturnsUsableDiffer(t *testing.T) {
	d := NewDiffer(Config{})
	if d == nil {
		t.Fatal("NewDiffer returned nil")
	}
	a := snapWithTables()
	b := snapWithTables()
	got := d.Diff(a, b)
	if len(got) != 0 {
		t.Errorf("expected empty diff on identical empty snapshots, got %d findings: %v", len(got), got)
	}
}

// ─── TestDiffer_ZeroConfig_EqualsRawDiff ──────────────────────────────────────
// A Differ with zero Config is a pure pass-through: its result deep-equals the
// package-level Diff. The zero Scope keeps everything (FilterByScope contract).

func TestDiffer_ZeroConfig_EqualsRawDiff(t *testing.T) {
	// Build two snapshots with a column type change and a wholly dropped table so
	// there are multiple findings of different types.
	ordersA := makeTable("101", "public", "orders", schemasnapshot.TableKindOrdinary)
	ordersB := makeTable("101", "public", "orders", schemasnapshot.TableKindOrdinary)
	legacyA := makeTable("202", "public", "legacy", schemasnapshot.TableKindOrdinary)
	// legacy is dropped in b

	colA := makeColumn("public", "orders", "101:1", "amount", "integer", notNull())
	colB := makeColumn("public", "orders", "101:1", "amount", "numeric", notNull()) // type changed

	a := snapWithTablesAndColumns([]schemasnapshot.Table{ordersA, legacyA}, []schemasnapshot.Column{colA})
	b := snapWithTablesAndColumns([]schemasnapshot.Table{ordersB}, []schemasnapshot.Column{colB})

	want := Diff(a, b)
	got := NewDiffer(Config{}).Diff(a, b)

	if !reflect.DeepEqual(got, want) {
		t.Errorf("zero-Config Differ diverged from package Diff\ngot:  %v\nwant: %v", got, want)
	}
}

// ─── TestDiffer_AppliesScope ──────────────────────────────────────────────────
// A Differ with a non-empty Scope applies FilterByScope after diffing.
// Findings from a scoped-out table must not appear in the result.

func TestDiffer_AppliesScope(t *testing.T) {
	// Two tables: public.orders (kept) and public.legacy (filtered out).
	ordersA := makeTable("101", "public", "orders", schemasnapshot.TableKindOrdinary)
	ordersB := makeTable("101", "public", "orders", schemasnapshot.TableKindOrdinary)
	legacyA := makeTable("202", "public", "legacy", schemasnapshot.TableKindOrdinary)
	legacyB := makeTable("202", "public", "legacy", schemasnapshot.TableKindOrdinary)

	colOrdersA := makeColumn("public", "orders", "101:1", "price", "integer")
	colOrdersB := makeColumn("public", "orders", "101:1", "price", "numeric") // type changed → COLUMN_TYPE_CHANGED on orders
	colLegacyA := makeColumn("public", "legacy", "202:1", "old_col", "text")
	colLegacyB := makeColumn("public", "legacy", "202:1", "old_col", "varchar") // type changed → COLUMN_TYPE_CHANGED on legacy

	a := snapWithTablesAndColumns([]schemasnapshot.Table{ordersA, legacyA}, []schemasnapshot.Column{colOrdersA, colLegacyA})
	b := snapWithTablesAndColumns([]schemasnapshot.Table{ordersB, legacyB}, []schemasnapshot.Column{colOrdersB, colLegacyB})

	scope := Scope{IncludeTables: []schemasnapshot.ObjectRef{ref("public", "orders")}}
	d := NewDiffer(Config{Scope: scope})
	got := d.Diff(a, b)

	// The Differ result must equal FilterByScope(Diff(a,b), scope).
	want := FilterByScope(Diff(a, b), scope)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("scoped Differ diverged from FilterByScope(Diff(...))\ngot:  %v\nwant: %v", got, want)
	}

	// There must be at least one finding (the orders column type change).
	if len(got) == 0 {
		t.Fatal("expected at least one finding for public.orders, got none")
	}

	// No finding must be anchored to public.legacy.
	legacyRef := ref("public", "legacy")
	for _, diff := range got {
		if diff.AnchorTable != nil && *diff.AnchorTable == legacyRef {
			t.Errorf("unexpected legacy finding in scoped result: %v", diff)
		}
	}
}

// ─── TestDiffer_ScopeRenameRetention ─────────────────────────────────────────
// The façade preserves FilterByScope's rename-alias behaviour: filtering by the
// NEW table name still returns the TABLE_NAME_CHANGED finding whose AnchorTable
// carries the OLD name.

func TestDiffer_ScopeRenameRetention(t *testing.T) {
	// public.orders (ID "55") is renamed to public.purchases in b.
	ordersA := makeTable("55", "public", "orders", schemasnapshot.TableKindOrdinary)
	purchasesB := makeTable("55", "public", "purchases", schemasnapshot.TableKindOrdinary)

	// Add a column present in both sides to make the snapshot non-trivial.
	colA := makeColumn("public", "orders", "55:1", "id", "integer", notNull())
	colB := makeColumn("public", "purchases", "55:1", "id", "integer", notNull())

	a := snapWithTablesAndColumns([]schemasnapshot.Table{ordersA}, []schemasnapshot.Column{colA})
	b := snapWithTablesAndColumns([]schemasnapshot.Table{purchasesB}, []schemasnapshot.Column{colB})

	// Scope by the NEW name (public.purchases).
	scope := Scope{IncludeTables: []schemasnapshot.ObjectRef{ref("public", "purchases")}}
	got := NewDiffer(Config{Scope: scope}).Diff(a, b)

	// The TABLE_NAME_CHANGED finding must survive even though its AnchorTable is
	// the old name (public.orders) — FilterByScope honours rename aliases.
	found := false
	for _, diff := range got {
		if diff.Type == TableNameChanged {
			found = true
			// AnchorTable is the old (side-A) ref.
			if diff.AnchorTable == nil || diff.AnchorTable.Name != "orders" {
				t.Errorf("TableNameChanged AnchorTable should be public.orders, got %v", diff.AnchorTable)
			}
		}
	}
	if !found {
		t.Errorf("expected TABLE_NAME_CHANGED finding in result for scope public.purchases, got: %v", got)
	}
}
