package anon

import (
	_ "embed"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// Embedded function lists from different sources
//
// postgresql_17_catalog_functions.txt contains all functions, aggregates, and window functions
// from PostgreSQL 17 pg_catalog schema. Generated using:
//
// SELECT DISTINCT proname FROM pg_catalog.pg_proc p
// JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace
// WHERE n.nspname = 'pg_catalog'
// AND p.prokind IN ('f', 'a', 'w')  -- Functions, Aggregates, Window functions
// ORDER BY proname;
//
//go:embed data/postgresql_17_catalog_functions.txt
var postgresql17CatalogFunctions string

//go:embed data/sql_standard_functions.txt
var sqlStandardFunctions string

//go:embed data/operator_functions.txt
var operatorFunctions string

// AllBuiltinFunctions contains all built-in functions that should NOT be anonymized
var AllBuiltinFunctions map[string]bool

// System schemas that contain built-in functions
var SystemSchemas = map[string]bool{
	"pg_catalog":         true,
	"information_schema": true,
	"pg_toast":           true,
	"pg_temp":            true,
	"pg_toast_temp":      true,
}

// Known extension schemas - functions from these schemas should not be anonymized
var KnownExtensionSchemas = map[string]bool{
	"uuid-ossp":          true,
	"pgcrypto":           true,
	"ltree":              true,
	"hstore":             true,
	"postgis":            true,
	"timescaledb":        true,
	"pg_stat_statements": true,
	"btree_gin":          true,
	"btree_gist":         true,
	"citext":             true,
	"cube":               true,
	"dblink":             true,
	"earthdistance":      true,
	"fuzzystrmatch":      true,
	"isn":                true,
	"lo":                 true,
	"pg_buffercache":     true,
	"pg_freespacemap":    true,
	"pg_prewarm":         true,
	"pg_trgm":            true,
	"pgrowlocks":         true,
	"pgstattuple":        true,
	"tablefunc":          true,
	"unaccent":           true,
	"xml2":               true,
}

func init() {
	AllBuiltinFunctions = make(map[string]bool)

	// Load functions from all sources
	loadFunctionsFromData(postgresql17CatalogFunctions)
	loadFunctionsFromData(sqlStandardFunctions)
	loadFunctionsFromData(operatorFunctions)
}

// loadFunctionsFromData loads functions from embedded text data
func loadFunctionsFromData(data string) {
	lines := strings.Split(strings.TrimSpace(data), "\n")
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			AllBuiltinFunctions[strings.ToLower(line)] = true
		}
	}
}

// Builtin Sequence functions that require argument anonymization
// Note: lastval() takes no arguments rest all take sequence name as argument
var SequenceFunctions = map[string]bool{
	"nextval": true,
	"currval": true,
	"setval":  true,
	"lastval": true,
}

// == Helper functions for function classification ==

func IsSequenceFunction(funcname []*pg_query.Node) bool {
	if len(funcname) == 1 {
		if funcStr := funcname[0].GetString_(); funcStr != nil {
			// TODO: think about case sensitivity; do we need to handle that in this scenario or rely on pg parser
			return SequenceFunctions[funcStr.Sval]
		}
	}
	return false
}

// IsBuiltinFunction checks if a function call represents a built-in function
func IsBuiltinFunction(funcname []*pg_query.Node) bool {
	if len(funcname) == 0 {
		return false
	}

	if len(funcname) == 1 { // Unqualified function - only option is to check function name
		if funcname[0] != nil {
			if funcStr := funcname[0].GetString_(); funcStr != nil {
				return AllBuiltinFunctions[funcStr.Sval]
			}
		}
	} else { // Qualified function name - checking schema name is enough
		if funcname[0] != nil {
			if schemaStr := funcname[0].GetString_(); schemaStr != nil {
				schema := schemaStr.Sval
				return SystemSchemas[schema] || KnownExtensionSchemas[schema]
			}
		}
	}
	return false
}

// ShouldAnonymizeFunction returns true if the function should be anonymized
func ShouldAnonymizeFunction(funcname []*pg_query.Node) bool {
	return !IsBuiltinFunction(funcname)
}
