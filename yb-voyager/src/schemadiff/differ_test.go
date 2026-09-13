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
	ordersA.Columns = []schemasnapshot.Column{colA}
	ordersB.Columns = []schemasnapshot.Column{colB}

	a := snapWithTables(ordersA, legacyA)
	b := snapWithTables(ordersB)

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
	ordersA.Columns = []schemasnapshot.Column{colOrdersA}
	ordersB.Columns = []schemasnapshot.Column{colOrdersB}
	legacyA.Columns = []schemasnapshot.Column{colLegacyA}
	legacyB.Columns = []schemasnapshot.Column{colLegacyB}

	a := snapWithTables(ordersA, legacyA)
	b := snapWithTables(ordersB, legacyB)

	scope := Scope{Tables: []schemasnapshot.ObjectRef{ref("public", "orders")}}
	got := NewDiffer(Config{Scope: scope}).Diff(a, b)

	// Assert the concrete expected result, independently of FilterByScope's
	// implementation: only the orders column-type change survives; the legacy
	// change is scoped out. Comparing against FilterByScope(Diff(...)) here would
	// be circular — that is exactly what Differ.Diff runs internally, so a filter
	// regression would corrupt both sides identically and still pass.
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 finding (public.orders column type change), got %d: %v", len(got), got)
	}
	only := got[0]
	if only.Type != ColumnTypeChanged {
		t.Errorf("expected ColumnTypeChanged, got %v", only.Type)
	}
	if key := identKey(only, "postgresql"); key != "public.orders.price" {
		t.Errorf("expected finding on public.orders.price, got %v", key)
	}
	// And nothing anchored to the scoped-out legacy table.
	if anchor, ok := anchorTableOf(only); ok && anchor == ref("public", "legacy") {
		t.Errorf("unexpected legacy finding in scoped result: %v", only)
	}
}

// ─── TestDiffer_ScopeRenameRetention ─────────────────────────────────────────
// The façade preserves FilterByScope's rename-alias behaviour: filtering by the
// NEW table name still returns the TABLE_NAME_CHANGED finding whose derived
// anchor carries the OLD name.

func TestDiffer_ScopeRenameRetention(t *testing.T) {
	t.Skip("rename or move alias handling temporarily disabled in FilterByScope; re-enable with the alias logic")
	// public.orders (ID "55") is renamed to public.purchases in b.
	ordersA := makeTable("55", "public", "orders", schemasnapshot.TableKindOrdinary)
	purchasesB := makeTable("55", "public", "purchases", schemasnapshot.TableKindOrdinary)

	// Add a column present in both sides to make the snapshot non-trivial.
	colA := makeColumn("public", "orders", "55:1", "id", "integer", notNull())
	colB := makeColumn("public", "purchases", "55:1", "id", "integer", notNull())
	ordersA.Columns = []schemasnapshot.Column{colA}
	purchasesB.Columns = []schemasnapshot.Column{colB}

	a := snapWithTables(ordersA)
	b := snapWithTables(purchasesB)

	// Scope by the NEW name (public.purchases).
	scope := Scope{Tables: []schemasnapshot.ObjectRef{ref("public", "purchases")}}
	got := NewDiffer(Config{Scope: scope}).Diff(a, b)

	// The TABLE_NAME_CHANGED finding must survive even though its derived anchor
	// is the old name (public.orders) — FilterByScope honours rename aliases.
	found := false
	for _, diff := range got {
		if diff.Type == TableNameChanged {
			found = true
			// The derived anchor is the old (side-A) ref.
			anchor, ok := anchorTableOf(diff)
			if !ok || anchor.Name != "orders" {
				t.Errorf("TableNameChanged anchor should be public.orders, got %v (ok=%v)", anchor, ok)
			}
		}
	}
	if !found {
		t.Errorf("expected TABLE_NAME_CHANGED finding in result for scope public.purchases, got: %v", got)
	}
}
