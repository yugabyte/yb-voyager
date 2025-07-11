package anon

const (
	DATABASE_KIND_PREFIX   = "db_"
	SCHEMA_KIND_PREFIX     = "schema_"
	TABLE_KIND_PREFIX      = "table_"
	COLUMN_KIND_PREFIX     = "col_"
	INDEX_KIND_PREFIX      = "index_"
	CONSTRAINT_KIND_PREFIX = "constraint_"
	ALIAS_KIND_PREFIX      = "alias_"
	TYPE_KIND_PREFIX       = "type_"
	ROLE_KIND_PREFIX       = "role_"
	CONST_KIND_PREFIX      = "const_"
	DEFAULT_KIND_PREFIX    = "anon_" // fallback for any other identifiers
)

// interface to be implemented by any new anonymizer
type Anonymizer interface {
	Anonymize(input string) (string, error)
}
