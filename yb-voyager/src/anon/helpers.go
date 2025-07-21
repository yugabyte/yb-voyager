package anon

import (
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// BuiltinTypeNames contains PostgreSQL built-in data types that should not be anonymized
var BuiltinTypeNames = map[string]bool{
	// Numeric types
	"int":      true,
	"int2":     true,
	"int4":     true,
	"int8":     true,
	"integer":  true,
	"smallint": true,
	"bigint":   true,
	"decimal":  true,
	"numeric":  true,
	"real":     true,
	"float":    true,
	"float4":   true,
	"float8":   true,
	"double":   true,
	"money":    true,

	// Serial types
	"serial":      true,
	"serial2":     true,
	"serial4":     true,
	"serial8":     true,
	"bigserial":   true,
	"smallserial": true,

	// String/Character types
	"text":              true,
	"varchar":           true,
	"char":              true,
	"character":         true,
	"character varying": true,
	"bpchar":            true,
	"name":              true,

	// Date/Time types
	"date":        true,
	"time":        true,
	"timestamp":   true,
	"timestamptz": true,
	"timetz":      true,
	"interval":    true,
	"abstime":     true,
	"reltime":     true,
	"tinterval":   true,

	// Boolean type
	"boolean": true,
	"bool":    true,

	// JSON types
	"json":  true,
	"jsonb": true,

	// UUID type
	"uuid": true,

	// Binary types
	"bytea":  true,
	"bit":    true,
	"varbit": true,

	// XML type
	"xml": true,

	// Geometric types
	"point":   true,
	"line":    true,
	"lseg":    true,
	"box":     true,
	"path":    true,
	"polygon": true,
	"circle":  true,

	// Network types
	"cidr":     true,
	"inet":     true,
	"macaddr":  true,
	"macaddr8": true,

	// Text search types
	"tsvector": true,
	"tsquery":  true,

	// Pseudo types
	"any":                   true,
	"anyarray":              true,
	"anyelement":            true,
	"anyenum":               true,
	"anynonarray":           true,
	"anyrange":              true,
	"anycompatible":         true,
	"anycompatiblearray":    true,
	"anycompatiblenonarray": true,
	"anycompatiblerange":    true,
	"cstring":               true,
	"internal":              true,
	"language_handler":      true,
	"fdw_handler":           true,
	"table_am_handler":      true,
	"index_am_handler":      true,
	"tsm_handler":           true,
	"void":                  true,
	"unknown":               true,
	"opaque":                true,
	"trigger":               true,
	"event_trigger":         true,
	"pg_ddl_command":        true,
	"record":                true,

	// OID types
	"oid":           true,
	"regproc":       true,
	"regprocedure":  true,
	"regoper":       true,
	"regoperator":   true,
	"regclass":      true,
	"regtype":       true,
	"regconfig":     true,
	"regdictionary": true,
	"regnamespace":  true,
	"regrole":       true,
	"regcollation":  true,

	// Range types
	"int4range": true,
	"int8range": true,
	"numrange":  true,
	"tsrange":   true,
	"tstzrange": true,
	"daterange": true,

	// Multirange types (PostgreSQL 14+)
	"int4multirange": true,
	"int8multirange": true,
	"nummultirange":  true,
	"tsmultirange":   true,
	"tstzmultirange": true,
	"datemultirange": true,
}

// IsBuiltinType checks if a TypeName represents a built-in PostgreSQL type that should not be anonymized
func IsBuiltinType(typeName *pg_query.TypeName) bool {
	if typeName == nil || len(typeName.Names) == 0 {
		return false
	}

	// Check if it's in pg_catalog schema (built-in types are in pg_catalog)
	if len(typeName.Names) > 1 {
		if schemaName := typeName.Names[0].GetString_(); schemaName != nil {
			if schemaName.Sval == "pg_catalog" {
				return true
			}
		}
	}

	// Check for common built-in type names (unqualified)
	// Get the last element which is the actual type name
	lastIndex := len(typeName.Names) - 1
	if typeNameStr := typeName.Names[lastIndex].GetString_(); typeNameStr != nil && typeNameStr.Sval != "" {
		return BuiltinTypeNames[typeNameStr.Sval]
	}

	return false
}

// IsBuiltinTypeName checks if a string represents a built-in PostgreSQL type name
func IsBuiltinTypeName(typeName string) bool {
	return BuiltinTypeNames[typeName]
}
