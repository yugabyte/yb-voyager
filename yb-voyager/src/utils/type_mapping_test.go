//go:build unit_test

package utils

import (
	"fmt"
	"testing"
)

func TestGetSQLTypeName(t *testing.T) {
	tests := []struct {
		name           string
		internalType   string
		expectedResult string
	}{
		// Integer types that need conversion
		{"int2 to smallint", "int2", "smallint"},
		{"int4 to integer", "int4", "integer"},
		{"int8 to bigint", "int8", "bigint"},
		{"serial2 to smallserial", "serial2", "smallserial"},
		{"serial4 to serial", "serial4", "serial"},
		{"serial8 to bigserial", "serial8", "bigserial"},

		// Character types that need conversion
		{"bpchar to char", "bpchar", "char"},

		// Numeric types that need conversion
		{"float4 to real", "float4", "real"},
		{"float8 to double precision", "float8", "double precision"},

		// Boolean type that needs conversion
		{"bool to boolean", "bool", "boolean"},

		// Types that don't need conversion (should return original)
		{"char to char", "char", "char"},
		{"varchar to varchar", "varchar", "varchar"},
		{"text to text", "text", "text"},
		{"numeric to numeric", "numeric", "numeric"},
		{"decimal to decimal", "decimal", "decimal"},
		{"real to real", "real", "real"},
		{"timestamp to timestamp", "timestamp", "timestamp"},
		{"timestamptz to timestamptz", "timestamptz", "timestamptz"},
		{"date to date", "date", "date"},
		{"time to time", "time", "time"},
		{"timetz to timetz", "timetz", "timetz"},
		{"interval to interval", "interval", "interval"},
		{"json to json", "json", "json"},
		{"jsonb to jsonb", "jsonb", "jsonb"},
		{"uuid to uuid", "uuid", "uuid"},
		{"inet to inet", "inet", "inet"},
		{"bit to bit", "bit", "bit"},
		{"varbit to varbit", "varbit", "varbit"},

		// Unknown types (should return original)
		{"unknown_type to unknown_type", "unknown_type", "unknown_type"},
		{"custom_type to custom_type", "custom_type", "custom_type"},
		{"user_defined_type to user_defined_type", "user_defined_type", "user_defined_type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetSQLTypeName(tt.internalType)
			if result != tt.expectedResult {
				t.Errorf("GetSQLTypeName(%q) = %q, want %q",
					tt.internalType, result, tt.expectedResult)
			}
		})
	}
}

func TestApplyModifiersToDatatype(t *testing.T) {
	tests := []struct {
		name           string
		typeName       string
		typmods        []int32
		expectedResult string
	}{
		// Character types with modifiers
		{"char with length", "char", []int32{10}, "char(10)"},
		{"varchar with length", "varchar", []int32{255}, "varchar(255)"},

		// Bit types with modifiers
		{"bit with length", "bit", []int32{8}, "bit(8)"},
		{"varbit with length", "varbit", []int32{16}, "varbit(16)"},

		// Time types with precision
		{"time with precision", "time", []int32{6}, "time(6)"},
		{"timetz with precision", "timetz", []int32{3}, "timetz(3)"},
		{"timestamp with precision", "timestamp", []int32{6}, "timestamp(6)"},
		{"timestamptz with precision", "timestamptz", []int32{3}, "timestamptz(3)"},
		{"interval with precision (internal 32767)", "interval", []int32{32767, 6}, "interval(6)"},
		{"interval with precision and scale", "interval", []int32{3, 6}, "interval(3,6)"},
		{"interval with single typmod", "interval", []int32{6}, "interval(6)"},

		// Numeric types with precision/scale
		{"numeric with precision", "numeric", []int32{8}, "numeric(8)"},
		{"numeric with precision and scale", "numeric", []int32{8, 2}, "numeric(8,2)"},
		{"decimal with precision and scale", "decimal", []int32{10, 4}, "decimal(10,4)"},

		// Types without modifiers
		{"text without modifiers", "text", []int32{}, "text"},
		{"integer without modifiers", "integer", []int32{}, "integer"},
		{"boolean without modifiers", "boolean", []int32{}, "boolean"},

		// Edge cases
		{"empty typmods", "varchar", []int32{}, "varchar"},
		{"nil typmods", "varchar", nil, "varchar"},
		{"unsupported type with typmods", "text", []int32{100}, "text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ApplyModifiersToDatatype(tt.typeName, tt.typmods)
			if result != tt.expectedResult {
				t.Errorf("ApplyModifiersToDatatype(%q, %v) = %q, want %q",
					tt.typeName, tt.typmods, result, tt.expectedResult)
			}
		})
	}
}

func TestTypeModifierMappingCompleteness(t *testing.T) {
	// Test that our switch statement handles all expected types correctly
	singleParamTypes := []string{
		"char", "varchar", "bit", "varbit",
		"time", "timetz", "timestamp", "timestamptz", "interval",
	}

	for _, typ := range singleParamTypes {
		t.Run("single_param_modifier_works_"+typ, func(t *testing.T) {
			result := ApplyModifiersToDatatype(typ, []int32{10})
			expected := fmt.Sprintf("%s(10)", typ)
			if result != expected {
				t.Errorf("Expected %q, got %q", expected, result)
			}
		})
	}

	numericTypes := []string{"numeric", "decimal"}

	for _, typ := range numericTypes {
		t.Run("numeric_modifier_works_"+typ, func(t *testing.T) {
			result := ApplyModifiersToDatatype(typ, []int32{10, 2})
			expected := fmt.Sprintf("%s(10,2)", typ)
			if result != expected {
				t.Errorf("Expected %q, got %q", expected, result)
			}
		})
	}
}
