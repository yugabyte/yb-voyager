package schemadiff

import (
	"testing"
)

func TestBtreeToLSMRule(t *testing.T) {
	entries := []DiffEntry{
		{ObjectType: ObjectIndex, ObjectName: "public.orders_pkey", DiffType: DiffModified, Property: "METHOD_CHANGED", OldValue: "btree", NewValue: "lsm"},
		{ObjectType: ObjectIndex, ObjectName: "public.idx_date", DiffType: DiffModified, Property: "METHOD_CHANGED", OldValue: "btree", NewValue: "lsm"},
		{ObjectType: ObjectColumn, ObjectName: "public.orders.amount", DiffType: DiffModified, Property: "TYPE_CHANGED", OldValue: "numeric(10,2)", NewValue: "numeric(12,4)"},
	}

	rules := []IgnoreRule{BtreeToLSMRule}
	result := FilterDiff(entries, rules)

	if len(result.Real) != 1 {
		t.Fatalf("expected 1 real diff, got %d", len(result.Real))
	}
	if result.Real[0].ObjectName != "public.orders.amount" {
		t.Errorf("expected column diff to pass through, got %s", result.Real[0].ObjectName)
	}
	if len(result.Suppressed) != 2 {
		t.Fatalf("expected 2 suppressed, got %d", len(result.Suppressed))
	}
	for _, s := range result.Suppressed {
		if s.Reason != "YugabyteDB uses LSM storage for all btree indexes." {
			t.Errorf("wrong reason: %s", s.Reason)
		}
	}
}

func TestBtreeToLSMRule_DoesNotMatchHash(t *testing.T) {
	entries := []DiffEntry{
		{ObjectType: ObjectIndex, ObjectName: "public.idx_hash", DiffType: DiffModified, Property: "METHOD_CHANGED", OldValue: "hash", NewValue: "lsm"},
	}
	result := FilterDiff(entries, []IgnoreRule{BtreeToLSMRule})
	if len(result.Real) != 1 {
		t.Fatalf("expected hash→lsm to NOT be suppressed, got %d real", len(result.Real))
	}
}

func TestRedundantIndexRule(t *testing.T) {
	redundant := map[string]bool{
		"public.idx_orders_redundant": true,
	}
	entries := []DiffEntry{
		{ObjectType: ObjectIndex, ObjectName: "public.idx_orders_redundant", DiffType: DiffDropped},
		{ObjectType: ObjectIndex, ObjectName: "public.idx_orders_date", DiffType: DiffDropped},
	}

	rules := []IgnoreRule{RedundantIndexRule(redundant)}
	result := FilterDiff(entries, rules)

	if len(result.Real) != 1 {
		t.Fatalf("expected 1 real, got %d", len(result.Real))
	}
	if result.Real[0].ObjectName != "public.idx_orders_date" {
		t.Errorf("wrong real diff: %s", result.Real[0].ObjectName)
	}
	if len(result.Suppressed) != 1 {
		t.Fatalf("expected 1 suppressed, got %d", len(result.Suppressed))
	}
}

func TestPendingPostDataImportRule(t *testing.T) {
	deferred := map[string]bool{
		"public.idx_orders_customer_id": true,
		"public.chk_amount":            true,
	}
	entries := []DiffEntry{
		{ObjectType: ObjectIndex, ObjectName: "public.idx_orders_customer_id", DiffType: DiffDropped},
		{ObjectType: ObjectConstraint, ObjectName: "public.chk_amount", DiffType: DiffDropped},
		{ObjectType: ObjectIndex, ObjectName: "public.idx_users_email", DiffType: DiffDropped},
		{ObjectType: ObjectTable, ObjectName: "public.orders", DiffType: DiffDropped},
	}

	rules := []IgnoreRule{PendingPostDataImportRule(deferred)}
	result := FilterDiff(entries, rules)

	if len(result.Real) != 2 {
		t.Fatalf("expected 2 real, got %d", len(result.Real))
	}
	if len(result.Suppressed) != 2 {
		t.Fatalf("expected 2 suppressed, got %d", len(result.Suppressed))
	}
}

func TestPendingPostDataImportRule_DoesNotMatchAdded(t *testing.T) {
	deferred := map[string]bool{"public.idx_foo": true}
	entries := []DiffEntry{
		{ObjectType: ObjectIndex, ObjectName: "public.idx_foo", DiffType: DiffAdded},
	}
	rules := []IgnoreRule{PendingPostDataImportRule(deferred)}
	result := FilterDiff(entries, rules)

	if len(result.Real) != 1 {
		t.Fatalf("ADDED index should not be suppressed by pending rule, got %d real", len(result.Real))
	}
}

func TestDefaultPGvsYBRules_Combined(t *testing.T) {
	redundant := map[string]bool{"public.idx_redundant": true}
	deferred := map[string]bool{"public.idx_deferred": true}

	entries := []DiffEntry{
		{ObjectType: ObjectIndex, DiffType: DiffModified, ObjectName: "public.idx_pk", Property: "METHOD_CHANGED", OldValue: "btree", NewValue: "lsm"},
		{ObjectType: ObjectIndex, DiffType: DiffDropped, ObjectName: "public.idx_redundant"},
		{ObjectType: ObjectIndex, DiffType: DiffDropped, ObjectName: "public.idx_deferred"},
		{ObjectType: ObjectColumn, DiffType: DiffModified, ObjectName: "public.t.col", Property: "TYPE_CHANGED", OldValue: "int4", NewValue: "int8"},
	}

	rules := DefaultPGvsYBRules(redundant, deferred)
	result := FilterDiff(entries, rules)

	if len(result.Real) != 1 {
		t.Fatalf("expected 1 real diff (column type change), got %d", len(result.Real))
	}
	if result.Real[0].ObjectName != "public.t.col" {
		t.Errorf("wrong real diff: %s", result.Real[0].ObjectName)
	}
	if len(result.Suppressed) != 3 {
		t.Fatalf("expected 3 suppressed, got %d", len(result.Suppressed))
	}
}

func TestFilterDiff_NoRules(t *testing.T) {
	entries := []DiffEntry{
		{ObjectType: ObjectTable, ObjectName: "public.t1", DiffType: DiffAdded},
		{ObjectType: ObjectTable, ObjectName: "public.t2", DiffType: DiffDropped},
	}
	result := FilterDiff(entries, nil)
	if len(result.Real) != 2 {
		t.Fatalf("expected 2 real with no rules, got %d", len(result.Real))
	}
	if len(result.Suppressed) != 0 {
		t.Fatalf("expected 0 suppressed with no rules, got %d", len(result.Suppressed))
	}
}

func TestFilterDiff_Empty(t *testing.T) {
	result := FilterDiff(nil, []IgnoreRule{BtreeToLSMRule})
	if len(result.Real) != 0 || len(result.Suppressed) != 0 {
		t.Fatalf("expected empty result for empty input")
	}
}
