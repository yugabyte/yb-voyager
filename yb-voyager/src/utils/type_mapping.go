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
// This function ONLY handles internal type name mapping (e.g., "int4" → "integer", "bpchar" → "char")
// It does NOT apply type modifiers - use ApplyModifiersToDatatype for that.
//
// Examples:
//   - GetSQLTypeName("int4") → "integer"
//   - GetSQLTypeName("bpchar") → "char"
//   - GetSQLTypeName("varchar") → "varchar" (no mapping needed)
//   - GetSQLTypeName("text") → "text" (no mapping needed)
//   - GetSQLTypeName("unknown_type") → "unknown_type" (no mapping exists)
//
// Note: This function only handles non-array, non-user-defined types.
func GetSQLTypeName(internalTypeName string) string {
	// Get the SQL type name (either mapped or original)
	if mapped, exists := PostgreSQLTypeMapping[internalTypeName]; exists {
		return mapped
	}
	return internalTypeName
}

// ApplyModifiersToDatatype applies type modifiers to a datatype name
// This function can be called independently when you need to apply typmods
//
// Examples:
//   - ApplyModifiersToDatatype("varchar", []int32{255}) → "varchar(255)"
//   - ApplyModifiersToDatatype("numeric", []int32{8, 2}) → "numeric(8,2)"
//   - ApplyModifiersToDatatype("timestamp", []int32{6}) → "timestamp(6)"
//   - ApplyModifiersToDatatype("bit", []int32{8}) → "bit(8)"
//   - ApplyModifiersToDatatype("text", []int32{}) → "text" (no modifiers)
//
// Supported types with single parameter:
//   - char, varchar, bit, varbit, time, timetz, timestamp, timestamptz, interval
//
// Supported types with two parameters:
//   - numeric, decimal
func ApplyModifiersToDatatype(typeName string, typmods []int32) string {
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
	case "interval":
		switch len(typmods) {
		case 2:
			// PostgreSQL stores interval typmods as (field_precision, fractional_precision)
			// where 32767 means "no limit" for field precision
			if typmods[0] == 32767 {
				// Only fractional precision specified: INTERVAL(6)
				return fmt.Sprintf("%s(%d)", typeName, typmods[1])
			} else {
				// Both precisions specified: INTERVAL(3,6)
				return fmt.Sprintf("%s(%d,%d)", typeName, typmods[0], typmods[1])
			}
		case 1:
			return fmt.Sprintf("%s(%d)", typeName, typmods[0])
		}
	case "char", "varchar", "bit", "varbit", "time", "timetz", "timestamp", "timestamptz":
		if len(typmods) == 1 {
			return fmt.Sprintf("%s(%d)", typeName, typmods[0])
		}
	}

	return typeName
}
