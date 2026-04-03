package schemadiff

import (
	"testing"
)

func TestClassifyImpact_TableDropped(t *testing.T) {
	entry := DiffEntry{ObjectType: ObjectTable, DiffType: DiffDropped}
	if got := ClassifyImpact(entry); got != Level3 {
		t.Errorf("table dropped: expected LEVEL_3, got %s", got)
	}
}

func TestClassifyImpact_TableAdded(t *testing.T) {
	entry := DiffEntry{ObjectType: ObjectTable, DiffType: DiffAdded}
	if got := ClassifyImpact(entry); got != Level2 {
		t.Errorf("table added: expected LEVEL_2, got %s", got)
	}
}

func TestClassifyImpact_ColumnTypeChanged(t *testing.T) {
	entry := DiffEntry{ObjectType: ObjectColumn, DiffType: DiffModified, Property: "TYPE_CHANGED"}
	if got := ClassifyImpact(entry); got != Level3 {
		t.Errorf("column type changed: expected LEVEL_3, got %s", got)
	}
}

func TestClassifyImpact_ColumnNullableChanged(t *testing.T) {
	entry := DiffEntry{ObjectType: ObjectColumn, DiffType: DiffModified, Property: "NULLABLE_CHANGED"}
	if got := ClassifyImpact(entry); got != Level2 {
		t.Errorf("column nullable changed: expected LEVEL_2, got %s", got)
	}
}

func TestClassifyImpact_ColumnDefaultChanged(t *testing.T) {
	entry := DiffEntry{ObjectType: ObjectColumn, DiffType: DiffModified, Property: "DEFAULT_CHANGED"}
	if got := ClassifyImpact(entry); got != Level1 {
		t.Errorf("column default changed: expected LEVEL_1, got %s", got)
	}
}

func TestClassifyImpact_PKDropped(t *testing.T) {
	entry := DiffEntry{ObjectType: ObjectConstraint, DiffType: DiffDropped, Property: "p"}
	if got := ClassifyImpact(entry); got != Level3 {
		t.Errorf("PK dropped: expected LEVEL_3, got %s", got)
	}
}

func TestClassifyImpact_UKDropped(t *testing.T) {
	entry := DiffEntry{ObjectType: ObjectConstraint, DiffType: DiffDropped, Property: "u"}
	if got := ClassifyImpact(entry); got != Level2 {
		t.Errorf("UK dropped: expected LEVEL_2, got %s", got)
	}
}

func TestClassifyImpact_FKDropped(t *testing.T) {
	entry := DiffEntry{ObjectType: ObjectConstraint, DiffType: DiffDropped, Property: "f"}
	if got := ClassifyImpact(entry); got != Level1 {
		t.Errorf("FK dropped: expected LEVEL_1, got %s", got)
	}
}

func TestClassifyImpact_IndexDropped(t *testing.T) {
	entry := DiffEntry{ObjectType: ObjectIndex, DiffType: DiffDropped}
	if got := ClassifyImpact(entry); got != Level1 {
		t.Errorf("index dropped: expected LEVEL_1, got %s", got)
	}
}

func TestClassifyImpact_TriggerChanged(t *testing.T) {
	entry := DiffEntry{ObjectType: ObjectTrigger, DiffType: DiffModified, Property: "FUNCTION_CHANGED"}
	if got := ClassifyImpact(entry); got != Level2 {
		t.Errorf("trigger function changed: expected LEVEL_2, got %s", got)
	}
}

func TestClassifyImpact_EnumValuesChanged(t *testing.T) {
	entry := DiffEntry{ObjectType: ObjectEnumType, DiffType: DiffModified, Property: "VALUES_CHANGED"}
	if got := ClassifyImpact(entry); got != Level2 {
		t.Errorf("enum values changed: expected LEVEL_2, got %s", got)
	}
}

func TestClassifyImpact_UnknownFallsToLevel1(t *testing.T) {
	entry := DiffEntry{ObjectType: "UNKNOWN_TYPE", DiffType: DiffModified, Property: "SOMETHING"}
	if got := ClassifyImpact(entry); got != Level1 {
		t.Errorf("unknown type: expected LEVEL_1 fallback, got %s", got)
	}
}

func TestClassifyImpact_DiffRunsClassification(t *testing.T) {
	left := &SchemaSnapshot{
		Tables: []Table{{Schema: "public", Name: "orders"}},
		Columns: []Column{
			{Schema: "public", TableName: "orders", Name: "amount", DataType: "numeric", TypeModifier: "10,2"},
		},
	}
	right := &SchemaSnapshot{
		Columns: []Column{
			{Schema: "public", TableName: "orders", Name: "amount", DataType: "numeric", TypeModifier: "12,4"},
		},
	}
	result := Diff(left, right)

	for _, e := range result.Entries {
		if e.ImpactLevel == LevelUnset {
			t.Errorf("entry %s has unset impact level", e.Summary())
		}
	}
}

func TestImpactLevel_String(t *testing.T) {
	tests := []struct {
		level    ImpactLevel
		expected string
	}{
		{Level3, "LEVEL_3"},
		{Level2, "LEVEL_2"},
		{Level1, "LEVEL_1"},
		{LevelUnset, "UNKNOWN"},
	}
	for _, tc := range tests {
		if got := tc.level.String(); got != tc.expected {
			t.Errorf("ImpactLevel(%d).String() = %q, want %q", tc.level, got, tc.expected)
		}
	}
}
