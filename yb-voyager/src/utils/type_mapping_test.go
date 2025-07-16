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
		typmods        []int32
		expectedResult string
	}{
		// Integer types that need conversion
		{"int2 to smallint", "int2", nil, "smallint"},
		{"int4 to integer", "int4", nil, "integer"},
		{"int8 to bigint", "int8", nil, "bigint"},
		{"serial2 to smallserial", "serial2", nil, "smallserial"},
		{"serial4 to serial", "serial4", nil, "serial"},
		{"serial8 to bigserial", "serial8", nil, "bigserial"},

		// Character types that need conversion
		{"bpchar to char", "bpchar", nil, "char"},
		{"bpchar with length", "bpchar", []int32{10}, "char(10)"},

		// Numeric types that need conversion
		{"float4 to real", "float4", nil, "real"},
		{"float8 to double precision", "float8", nil, "double precision"},

		// Boolean type that needs conversion
		{"bool to boolean", "bool", nil, "boolean"},

		// Types that don't need conversion (should return original)
		{"char to char", "char", nil, "char"},
		{"varchar to varchar", "varchar", nil, "varchar"},
		{"text to text", "text", nil, "text"},
		{"numeric to numeric", "numeric", nil, "numeric"},
		{"decimal to decimal", "decimal", nil, "decimal"},
		{"real to real", "real", nil, "real"},
		{"timestamp to timestamp", "timestamp", nil, "timestamp"},
		{"timestamptz to timestamptz", "timestamptz", nil, "timestamptz"},
		{"date to date", "date", nil, "date"},
		{"time to time", "time", nil, "time"},
		{"timetz to timetz", "timetz", nil, "timetz"},
		{"interval to interval", "interval", nil, "interval"},
		{"json to json", "json", nil, "json"},
		{"jsonb to jsonb", "jsonb", nil, "jsonb"},
		{"uuid to uuid", "uuid", nil, "uuid"},
		{"inet to inet", "inet", nil, "inet"},
		{"bit to bit", "bit", nil, "bit"},
		{"varbit to varbit", "varbit", nil, "varbit"},

		// Types with modifiers (types that don't need conversion but have modifiers)
		{"varchar with length", "varchar", []int32{255}, "varchar(255)"},
		{"numeric with precision", "numeric", []int32{8}, "numeric(8)"},
		{"numeric with precision and scale", "numeric", []int32{8, 2}, "numeric(8,2)"},
		{"decimal with precision and scale", "decimal", []int32{10, 4}, "decimal(10,4)"},
		{"timestamp with precision", "timestamp", []int32{6}, "timestamp(6)"},
		{"timestamptz with precision", "timestamptz", []int32{3}, "timestamptz(3)"},
		{"time with precision", "time", []int32{6}, "time(6)"},
		{"timetz with precision", "timetz", []int32{3}, "timetz(3)"},
		{"interval with precision", "interval", []int32{6}, "interval(6)"},
		{"bit with length", "bit", []int32{8}, "bit(8)"},
		{"varbit with length", "varbit", []int32{16}, "varbit(16)"},

		// Unknown types (should return original)
		{"unknown_type to unknown_type", "unknown_type", nil, "unknown_type"},
		{"custom_type to custom_type", "custom_type", nil, "custom_type"},
		{"user_defined_type to user_defined_type", "user_defined_type", nil, "user_defined_type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetSQLTypeName(tt.internalType, tt.typmods)
			if result != tt.expectedResult {
				t.Errorf("GetSQLTypeName(%q, %v) = %q, want %q",
					tt.internalType, tt.typmods, result, tt.expectedResult)
			}
		})
	}
}

func TestTypeMappingCompleteness(t *testing.T) {
	typesThatNeedConversion := []string{
		"int2", "int4", "int8", "serial2", "serial4", "serial8",
		"bpchar", "float4", "float8", "bool",
	}

	for _, typ := range typesThatNeedConversion {
		t.Run("mapping_exists_"+typ, func(t *testing.T) {
			if mappedType, exists := PostgreSQLTypeMapping[typ]; !exists {
				t.Errorf("Type %q should be mapped in PostgreSQLTypeMapping but is missing", typ)
			} else if mappedType == "" {
				t.Errorf("Type %q is mapped to empty string", typ)
			}
		})
	}

	typesThatDontNeedConversion := []string{
		"char", "varchar", "text", "numeric", "decimal", "real",
		"timestamp", "timestamptz", "date", "time", "timetz", "interval",
		"json", "jsonb", "uuid", "bit", "varbit", "inet",
	}

	for _, typ := range typesThatDontNeedConversion {
		t.Run("no_mapping_needed_"+typ, func(t *testing.T) {
			if _, exists := PostgreSQLTypeMapping[typ]; exists {
				t.Errorf("Type %q should NOT be mapped in PostgreSQLTypeMapping since it doesn't need conversion", typ)
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
			result := GetSQLTypeName(typ, []int32{10})
			expected := fmt.Sprintf("%s(10)", typ)
			if result != expected {
				t.Errorf("Expected %q, got %q", expected, result)
			}
		})
	}

	numericTypes := []string{"numeric", "decimal"}

	for _, typ := range numericTypes {
		t.Run("numeric_modifier_works_"+typ, func(t *testing.T) {
			result := GetSQLTypeName(typ, []int32{10, 2})
			expected := fmt.Sprintf("%s(10,2)", typ)
			if result != expected {
				t.Errorf("Expected %q, got %q", expected, result)
			}
		})
	}
}
