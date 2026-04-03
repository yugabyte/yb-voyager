package schemadiff

import (
	"testing"
)

func TestDiffTables_Added(t *testing.T) {
	left := &SchemaSnapshot{}
	right := &SchemaSnapshot{
		Tables: []Table{{Schema: "public", Name: "orders"}},
	}
	result := Diff(left, right)
	assertEntry(t, result, ObjectTable, "public.orders", DiffAdded, "")
}

func TestDiffTables_Dropped(t *testing.T) {
	left := &SchemaSnapshot{
		Tables: []Table{{Schema: "public", Name: "orders"}},
	}
	right := &SchemaSnapshot{}
	result := Diff(left, right)
	assertEntry(t, result, ObjectTable, "public.orders", DiffDropped, "")
}

func TestDiffTables_Modified_PartitionStrategy(t *testing.T) {
	left := &SchemaSnapshot{
		Tables: []Table{{Schema: "public", Name: "events", IsPartitioned: true, PartitionStrategy: "r"}},
	}
	right := &SchemaSnapshot{
		Tables: []Table{{Schema: "public", Name: "events", IsPartitioned: true, PartitionStrategy: "l"}},
	}
	result := Diff(left, right)
	assertEntry(t, result, ObjectTable, "public.events", DiffModified, "PARTITION_STRATEGY")
}

func TestDiffTables_Unchanged(t *testing.T) {
	snap := &SchemaSnapshot{
		Tables: []Table{{Schema: "public", Name: "users"}},
	}
	result := Diff(snap, snap)
	if len(result.Entries) != 0 {
		t.Errorf("expected no diffs, got %d", len(result.Entries))
	}
}

func TestDiffColumns_TypeChanged(t *testing.T) {
	left := &SchemaSnapshot{
		Columns: []Column{{Schema: "public", TableName: "orders", Name: "amount", DataType: "numeric", TypeModifier: "10,2"}},
	}
	right := &SchemaSnapshot{
		Columns: []Column{{Schema: "public", TableName: "orders", Name: "amount", DataType: "numeric", TypeModifier: "12,4"}},
	}
	result := Diff(left, right)
	e := assertEntry(t, result, ObjectColumn, "public.orders.amount", DiffModified, "TYPE_CHANGED")
	if e.OldValue != "numeric(10,2)" || e.NewValue != "numeric(12,4)" {
		t.Errorf("wrong values: old=%q, new=%q", e.OldValue, e.NewValue)
	}
}

func TestDiffColumns_NullableChanged(t *testing.T) {
	left := &SchemaSnapshot{
		Columns: []Column{{Schema: "public", TableName: "users", Name: "email", DataType: "text", IsNullable: false}},
	}
	right := &SchemaSnapshot{
		Columns: []Column{{Schema: "public", TableName: "users", Name: "email", DataType: "text", IsNullable: true}},
	}
	result := Diff(left, right)
	assertEntry(t, result, ObjectColumn, "public.users.email", DiffModified, "NULLABLE_CHANGED")
}

func TestDiffColumns_DefaultChanged(t *testing.T) {
	left := &SchemaSnapshot{
		Columns: []Column{{Schema: "public", TableName: "users", Name: "status", DataType: "text", DefaultExpr: "'active'"}},
	}
	right := &SchemaSnapshot{
		Columns: []Column{{Schema: "public", TableName: "users", Name: "status", DataType: "text", DefaultExpr: "'pending'"}},
	}
	result := Diff(left, right)
	assertEntry(t, result, ObjectColumn, "public.users.status", DiffModified, "DEFAULT_CHANGED")
}

func TestDiffColumns_AddedAndDropped(t *testing.T) {
	left := &SchemaSnapshot{
		Columns: []Column{
			{Schema: "public", TableName: "orders", Name: "old_col", DataType: "int4"},
		},
	}
	right := &SchemaSnapshot{
		Columns: []Column{
			{Schema: "public", TableName: "orders", Name: "new_col", DataType: "text"},
		},
	}
	result := Diff(left, right)
	assertEntry(t, result, ObjectColumn, "public.orders.old_col", DiffDropped, "")
	assertEntry(t, result, ObjectColumn, "public.orders.new_col", DiffAdded, "")
}

func TestDiffConstraints_PKDropped(t *testing.T) {
	left := &SchemaSnapshot{
		Constraints: []Constraint{
			{Schema: "public", TableName: "orders", Name: "orders_pkey", Type: ConstraintPK, Columns: []string{"id"}},
		},
	}
	right := &SchemaSnapshot{}
	result := Diff(left, right)
	assertEntry(t, result, ObjectConstraint, "public.orders.orders_pkey", DiffDropped, "p")
}

func TestDiffConstraints_ColumnsChanged(t *testing.T) {
	left := &SchemaSnapshot{
		Constraints: []Constraint{
			{Schema: "public", TableName: "orders", Name: "orders_pkey", Type: ConstraintPK, Columns: []string{"id"}},
		},
	}
	right := &SchemaSnapshot{
		Constraints: []Constraint{
			{Schema: "public", TableName: "orders", Name: "orders_pkey", Type: ConstraintPK, Columns: []string{"id", "tenant_id"}},
		},
	}
	result := Diff(left, right)
	assertEntry(t, result, ObjectConstraint, "public.orders.orders_pkey", DiffModified, "COLUMNS_CHANGED")
}

func TestDiffIndexes_MethodChanged(t *testing.T) {
	left := &SchemaSnapshot{
		Indexes: []Index{
			{Schema: "public", Name: "idx_orders_date", AccessMethod: "btree", Columns: []string{"created_at"}},
		},
	}
	right := &SchemaSnapshot{
		Indexes: []Index{
			{Schema: "public", Name: "idx_orders_date", AccessMethod: "lsm", Columns: []string{"created_at"}},
		},
	}
	result := Diff(left, right)
	e := assertEntry(t, result, ObjectIndex, "public.idx_orders_date", DiffModified, "METHOD_CHANGED")
	if e.OldValue != "btree" || e.NewValue != "lsm" {
		t.Errorf("wrong values: old=%q, new=%q", e.OldValue, e.NewValue)
	}
}

func TestDiffIndexes_Dropped(t *testing.T) {
	left := &SchemaSnapshot{
		Indexes: []Index{
			{Schema: "public", Name: "idx_orders_redundant", AccessMethod: "btree", Columns: []string{"customer_id", "amount"}},
		},
	}
	right := &SchemaSnapshot{}
	result := Diff(left, right)
	assertEntry(t, result, ObjectIndex, "public.idx_orders_redundant", DiffDropped, "")
}

func TestDiffSequences_Modified(t *testing.T) {
	left := &SchemaSnapshot{
		Sequences: []Sequence{{Schema: "public", Name: "orders_id_seq", DataType: "int4", Increment: 1}},
	}
	right := &SchemaSnapshot{
		Sequences: []Sequence{{Schema: "public", Name: "orders_id_seq", DataType: "int8", Increment: 1}},
	}
	result := Diff(left, right)
	assertEntry(t, result, ObjectSequence, "public.orders_id_seq", DiffModified, "TYPE_CHANGED")
}

func TestDiffEnumTypes_ValuesChanged(t *testing.T) {
	left := &SchemaSnapshot{
		EnumTypes: []EnumType{{Schema: "public", Name: "status_t", Values: []string{"active", "inactive"}}},
	}
	right := &SchemaSnapshot{
		EnumTypes: []EnumType{{Schema: "public", Name: "status_t", Values: []string{"active", "inactive", "pending"}}},
	}
	result := Diff(left, right)
	assertEntry(t, result, ObjectEnumType, "public.status_t", DiffModified, "VALUES_CHANGED")
}

func TestDiffViews_DefinitionChanged(t *testing.T) {
	left := &SchemaSnapshot{
		Views: []View{{Schema: "public", Name: "active_users", Definition: "SELECT * FROM users WHERE active"}},
	}
	right := &SchemaSnapshot{
		Views: []View{{Schema: "public", Name: "active_users", Definition: "SELECT * FROM users WHERE active AND verified"}},
	}
	result := Diff(left, right)
	assertEntry(t, result, ObjectView, "public.active_users", DiffModified, "DEFINITION_CHANGED")
}

func TestDiffFunctions_ReturnTypeChanged(t *testing.T) {
	left := &SchemaSnapshot{
		Functions: []Function{{Schema: "public", Name: "get_count", Arguments: "", ReturnType: "int4", Language: "sql", Kind: "f"}},
	}
	right := &SchemaSnapshot{
		Functions: []Function{{Schema: "public", Name: "get_count", Arguments: "", ReturnType: "int8", Language: "sql", Kind: "f"}},
	}
	result := Diff(left, right)
	assertEntry(t, result, ObjectFunction, "public.get_count()", DiffModified, "RETURN_TYPE_CHANGED")
}

func TestDiffTriggers_Added(t *testing.T) {
	left := &SchemaSnapshot{}
	right := &SchemaSnapshot{
		Triggers: []Trigger{{Schema: "public", TableName: "orders", Name: "trg_audit", Timing: "AFTER", Events: "INSERT"}},
	}
	result := Diff(left, right)
	assertEntry(t, result, ObjectTrigger, "public.orders.trg_audit", DiffAdded, "")
}

func TestDiffExtensions_VersionChanged(t *testing.T) {
	left := &SchemaSnapshot{
		Extensions: []Extension{{Name: "pgcrypto", Version: "1.3"}},
	}
	right := &SchemaSnapshot{
		Extensions: []Extension{{Name: "pgcrypto", Version: "1.4"}},
	}
	result := Diff(left, right)
	assertEntry(t, result, ObjectExtension, "pgcrypto", DiffModified, "VERSION_CHANGED")
}

func TestDiffComplex_MultipleChanges(t *testing.T) {
	left := &SchemaSnapshot{
		Tables: []Table{
			{Schema: "public", Name: "orders"},
			{Schema: "public", Name: "old_table"},
		},
		Columns: []Column{
			{Schema: "public", TableName: "orders", Name: "amount", DataType: "numeric", TypeModifier: "10,2"},
		},
		Indexes: []Index{
			{Schema: "public", Name: "idx_orders_date", AccessMethod: "btree", Columns: []string{"created_at"}},
			{Schema: "public", Name: "idx_orders_redundant", AccessMethod: "btree", Columns: []string{"customer_id"}},
		},
	}
	right := &SchemaSnapshot{
		Tables: []Table{
			{Schema: "public", Name: "orders"},
			{Schema: "public", Name: "events_2026_04"},
		},
		Columns: []Column{
			{Schema: "public", TableName: "orders", Name: "amount", DataType: "numeric", TypeModifier: "12,4"},
		},
		Indexes: []Index{
			{Schema: "public", Name: "idx_orders_date", AccessMethod: "lsm", Columns: []string{"created_at"}},
		},
	}
	result := Diff(left, right)

	if len(result.Entries) < 4 {
		t.Fatalf("expected at least 4 diffs, got %d", len(result.Entries))
	}
	assertEntry(t, result, ObjectTable, "public.old_table", DiffDropped, "")
	assertEntry(t, result, ObjectTable, "public.events_2026_04", DiffAdded, "")
	assertEntry(t, result, ObjectColumn, "public.orders.amount", DiffModified, "TYPE_CHANGED")
	assertEntry(t, result, ObjectIndex, "public.idx_orders_date", DiffModified, "METHOD_CHANGED")
	assertEntry(t, result, ObjectIndex, "public.idx_orders_redundant", DiffDropped, "")
}

func assertEntry(t *testing.T, result *DiffResult, objType ObjectType, objName string, diffType DiffType, property string) DiffEntry {
	t.Helper()
	for _, e := range result.Entries {
		if e.ObjectType == objType && e.ObjectName == objName && e.DiffType == diffType {
			if property != "" && e.Property != property {
				continue
			}
			return e
		}
	}
	t.Errorf("expected diff entry: type=%s name=%s diff=%s property=%s\nGot entries:", objType, objName, diffType, property)
	for _, e := range result.Entries {
		t.Errorf("  %s %s %s property=%s", e.ObjectType, e.ObjectName, e.DiffType, e.Property)
	}
	t.FailNow()
	return DiffEntry{}
}
