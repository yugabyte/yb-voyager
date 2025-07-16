package anon

const (
	DATABASE_KIND_PREFIX   = "db_"
	SCHEMA_KIND_PREFIX     = "schema_"
	SEQUENCE_KIND_PREFIX   = "seq_"
	TABLE_KIND_PREFIX      = "table_"
	ENUM_KIND_PREFIX       = "enum_"
	COLUMN_KIND_PREFIX     = "col_"
	FUNCTION_KIND_PREFIX   = "func_"
	TRIGGER_KIND_PREFIX    = "trigger_"
	PROCEDURE_KIND_PREFIX  = "proc_"
	INDEX_KIND_PREFIX      = "index_"
	CONSTRAINT_KIND_PREFIX = "constraint_"
	ALIAS_KIND_PREFIX      = "alias_"
	TYPE_KIND_PREFIX       = "type_"
	DOMAIN_KIND_PREFIX     = "domain_"
	ROLE_KIND_PREFIX       = "role_"
	VIEW_KIND_PREFIX       = "view_"
	MVIEW_KIND_PREFIX      = "mview_"
	CONST_KIND_PREFIX      = "const_"
	COLLATION_KIND_PREFIX  = "collation_"
	DEFAULT_KIND_PREFIX    = "anon_" // fallback for any other identifiers
)

// AllKindPrefixes - single source of truth for all prefixes possible in anonymization
// NOTE: When adding a new prefix constant above, add it here too
var AllKindPrefixes = []string{
	DATABASE_KIND_PREFIX,
	SCHEMA_KIND_PREFIX,
	SEQUENCE_KIND_PREFIX,
	TABLE_KIND_PREFIX,
	VIEW_KIND_PREFIX,
	MVIEW_KIND_PREFIX,
	ENUM_KIND_PREFIX,
	COLUMN_KIND_PREFIX,
	FUNCTION_KIND_PREFIX,
	TRIGGER_KIND_PREFIX,
	PROCEDURE_KIND_PREFIX,
	INDEX_KIND_PREFIX,
	CONSTRAINT_KIND_PREFIX,
	ALIAS_KIND_PREFIX,
	TYPE_KIND_PREFIX,
	DOMAIN_KIND_PREFIX,
	ROLE_KIND_PREFIX,
	CONST_KIND_PREFIX,
	COLLATION_KIND_PREFIX,
	DEFAULT_KIND_PREFIX,
}

// interface to be implemented by any new anonymizer
type Anonymizer interface {
	Anonymize(input string) (string, error)
}
