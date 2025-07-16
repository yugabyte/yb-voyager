package utils

import (
	"fmt"
)

// PostgreSQLTypeMapping maps internal PostgreSQL type names to SQL type names
// Only types that need conversion are included
var PostgreSQLTypeMapping = map[string]string{
	// Integer types that need conversion
	"int2":    "smallint",
	"int4":    "integer",
	"int8":    "bigint",
	"serial2": "smallserial",
	"serial4": "serial",
	"serial8": "bigserial",

	// Character types that need conversion
	"bpchar": "char",

	// Numeric types that need conversion
	"float4": "real",
	"float8": "double precision",

	// Boolean type that needs conversion
	"bool": "boolean",
}

// GetSQLTypeName converts a PostgreSQL internal type name to the SQL type name
// that users write in their statements and that pg_dump exports.
//
// This function handles two main conversions:
// 1. Internal type name mapping (e.g., "int4" → "integer", "bpchar" → "char")
// 2. Type modifier application (e.g., "varchar" + [255] → "varchar(255)")
//
// Examples:
//   - GetSQLTypeName("int4", nil) → "integer"
//   - GetSQLTypeName("bpchar", []int32{10}) → "char(10)"
//   - GetSQLTypeName("numeric", []int32{8, 2}) → "numeric(8,2)"
//   - GetSQLTypeName("timestamp", []int32{6}) → "timestamp(6)"
//   - GetSQLTypeName("text", nil) → "text" (no mapping needed)
//   - GetSQLTypeName("unknown_type", nil) → "unknown_type" (no mapping exists)
//
// Note: This function only handles non-array, non-user-defined types.
func GetSQLTypeName(internalTypeName string, typmods []int32) string {
	// Get the SQL type name (either mapped or original)
	typeName := internalTypeName
	if mapped, exists := PostgreSQLTypeMapping[internalTypeName]; exists {
		typeName = mapped
	}

	// Apply modifiers if any
	return applyModifiers(typeName, typmods)
}

// applyModifiers applies type modifiers to a type name
func applyModifiers(typeName string, typmods []int32) string {
	if len(typmods) == 0 {
		return typeName
	}

	switch typeName {
	case "numeric", "decimal":
		switch len(typmods) {
		case 2:
			return fmt.Sprintf("%s(%d,%d)", typeName, typmods[0], typmods[1])
		case 1:
			return fmt.Sprintf("%s(%d)", typeName, typmods[0])
		}
	case "char", "varchar", "bit", "varbit", "time", "timetz", "timestamp", "timestamptz", "interval":
		if len(typmods) == 1 {
			return fmt.Sprintf("%s(%d)", typeName, typmods[0])
		}
	}

	return typeName
}
