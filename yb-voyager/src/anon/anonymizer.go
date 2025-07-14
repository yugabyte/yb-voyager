package anon

const (
	DATABASE_KIND_PREFIX   = "db_"
	SCHEMA_KIND_PREFIX     = "schema_"
	SEQUENCE_KIND_PREFIX   = "seq_"
	TABLE_KIND_PREFIX      = "table_"
	ENUM_LABEL_PREFIX      = "enum_"
	COLUMN_KIND_PREFIX     = "col_"
	FUNCTION_KIND_PREFIX   = "func_"
	TRIGGER_KIND_PREFIX    = "trigger_"
	PROCEDURE_KIND_PREFIX  = "proc_"
	INDEX_KIND_PREFIX      = "index_"
	CONSTRAINT_KIND_PREFIX = "constraint_"
	ALIAS_KIND_PREFIX      = "alias_"
	TYPE_KIND_PREFIX       = "type_"
	ROLE_KIND_PREFIX       = "role_"
	CONST_KIND_PREFIX      = "const_"
	COLLATION_KIND_PREFIX = "collation_"
	DEFAULT_KIND_PREFIX    = "anon_" // fallback for any other identifiers
)

// interface to be implemented by any new anonymizer
type Anonymizer interface {
	Anonymize(input string) (string, error)
}
