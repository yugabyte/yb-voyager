package anon

import pg_query "github.com/pganalyze/pg_query_go/v6"

const (
	DATABASE_KIND_PREFIX      = "db_"
	SCHEMA_KIND_PREFIX        = "schema_"
	SEQUENCE_KIND_PREFIX      = "seq_"
	TABLE_KIND_PREFIX         = "table_"
	ENUM_KIND_PREFIX          = "enum_"
	COLUMN_KIND_PREFIX        = "col_"
	FUNCTION_KIND_PREFIX      = "func_"
	TRIGGER_KIND_PREFIX       = "trigger_"
	PROCEDURE_KIND_PREFIX     = "proc_"
	INDEX_KIND_PREFIX         = "index_"
	CONSTRAINT_KIND_PREFIX    = "constraint_"
	ALIAS_KIND_PREFIX         = "alias_"
	TYPE_KIND_PREFIX          = "type_"
	DOMAIN_KIND_PREFIX        = "domain_"
	ROLE_KIND_PREFIX          = "role_"
	VIEW_KIND_PREFIX          = "view_"
	MVIEW_KIND_PREFIX         = "mview_"
	POLICY_KIND_PREFIX        = "policy_"
	CONST_KIND_PREFIX         = "const_"
	COLLATION_KIND_PREFIX     = "collation_"
	CONVERSION_KIND_PREFIX    = "conversion_"
	FOREIGN_TABLE_KIND_PREFIX = "ftable_"
	RULE_KIND_PREFIX          = "rule_"
	AGGREGATE_KIND_PREFIX     = "agg_"
	OPCLASS_KIND_PREFIX       = "opclass_"
	OPFAMILY_KIND_PREFIX      = "opfamily_"
	OPERATOR_KIND_PREFIX      = "op_"
	DEFAULT_KIND_PREFIX       = "anon_" // fallback for any other identifiers
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
	POLICY_KIND_PREFIX,
	CONST_KIND_PREFIX,
	COLLATION_KIND_PREFIX,
	CONVERSION_KIND_PREFIX,
	FOREIGN_TABLE_KIND_PREFIX,
	RULE_KIND_PREFIX,
	AGGREGATE_KIND_PREFIX,
	OPCLASS_KIND_PREFIX,
	OPFAMILY_KIND_PREFIX,
	OPERATOR_KIND_PREFIX,
	DEFAULT_KIND_PREFIX,
}

// interface to be implemented by any new anonymizer
type Anonymizer interface {
	Anonymize(input string) (string, error)
}

// GetObjectTypePrefix returns the appropriate prefix for anonymization based on the PostgreSQL object type
func GetObjectTypePrefix(objectType pg_query.ObjectType) string {
	switch objectType {
	case pg_query.ObjectType_OBJECT_SCHEMA:
		return SCHEMA_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_TABLE:
		return TABLE_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_VIEW:
		return VIEW_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_MATVIEW:
		return MVIEW_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_SEQUENCE:
		return SEQUENCE_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_INDEX:
		return INDEX_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_TYPE:
		return TYPE_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_DOMAIN:
		return DOMAIN_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_COLLATION:
		return COLLATION_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_TRIGGER:
		return TRIGGER_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_ROLE:
		return ROLE_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_DATABASE:
		return DATABASE_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_FUNCTION:
		return FUNCTION_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_PROCEDURE:
		return PROCEDURE_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_COLUMN:
		return COLUMN_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_POLICY:
		return POLICY_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_TABCONSTRAINT:
		return CONSTRAINT_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_CONVERSION:
		return CONVERSION_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_FOREIGN_TABLE:
		return FOREIGN_TABLE_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_RULE:
		return RULE_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_AGGREGATE:
		return AGGREGATE_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_OPCLASS:
		return OPCLASS_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_OPFAMILY:
		return OPFAMILY_KIND_PREFIX
	default:
		return DEFAULT_KIND_PREFIX // Fallback to default prefix
	}
}
