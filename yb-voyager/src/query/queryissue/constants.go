/*
Copyright (c) YugabyteDB, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package queryissue

// Types
const (
	REFERENCED_TYPE_DECLARATION                    = "REFERENCED_TYPE_DECLARATION"
	STORED_GENERATED_COLUMNS                       = "STORED_GENERATED_COLUMNS"
	UNLOGGED_TABLES                                = "UNLOGGED_TABLES"
	UNSUPPORTED_INDEX_METHOD                       = "UNSUPPORTED_INDEX_METHOD"
	STORAGE_PARAMETERS                             = "STORAGE_PARAMETERS"
	ALTER_TABLE_SET_COLUMN_ATTRIBUTE               = "ALTER_TABLE_SET_COLUMN_ATTRIBUTE"
	ALTER_TABLE_CLUSTER_ON                         = "ALTER_TABLE_CLUSTER_ON"
	ALTER_TABLE_DISABLE_RULE                       = "ALTER_TABLE_DISABLE_RULE"
	EXCLUSION_CONSTRAINTS                          = "EXCLUSION_CONSTRAINTS"
	DEFERRABLE_CONSTRAINTS                         = "DEFERRABLE_CONSTRAINTS"
	MULTI_COLUMN_GIN_INDEX                         = "MULTI_COLUMN_GIN_INDEX"
	ORDERED_GIN_INDEX                              = "ORDERED_GIN_INDEX"
	POLICY_WITH_ROLES                              = "POLICY_WITH_ROLES"
	CONSTRAINT_TRIGGER                             = "CONSTRAINT_TRIGGER"
	REFERENCING_CLAUSE_IN_TRIGGER                  = "REFERENCING_CLAUSE_IN_TRIGGER"
	BEFORE_ROW_TRIGGER_ON_PARTITIONED_TABLE        = "BEFORE_ROW_TRIGGER_ON_PARTITIONED_TABLE"
	ALTER_TABLE_ADD_PK_ON_PARTITIONED_TABLE        = "ALTER_TABLE_ADD_PK_ON_PARTITIONED_TABLE"
	EXPRESSION_PARTITION_WITH_PK_UK                = "EXPRESSION_PARTITION_WITH_PK_UK"
	MULTI_COLUMN_LIST_PARTITION                    = "MULTI_COLUMN_LIST_PARTITION"
	INSUFFICIENT_COLUMNS_IN_PK_FOR_PARTITION       = "INSUFFICIENT_COLUMNS_IN_PK_FOR_PARTITION"
	XML_DATATYPE                                   = "XML_DATATYPE"
	XID_DATATYPE                                   = "XID_DATATYPE"
	POSTGIS_DATATYPE                               = "POSTGIS_DATATYPE"
	UNSUPPORTED_DATATYPE                           = "UNSUPPORTED_DATATYPE"
	UNSUPPORTED_DATATYPE_LIVE_MIGRATION            = "UNSUPPORTED_DATATYPE_LIVE_MIGRATION"
	UNSUPPORTED_DATATYPE_LIVE_MIGRATION_WITH_FF_FB = "UNSUPPORTED_DATATYPE_LIVE_MIGRATION_WITH_FF_FB"
	PK_UK_ON_COMPLEX_DATATYPE                      = "PK_UK_ON_COMPLEX_DATATYPE"
	INDEX_ON_COMPLEX_DATATYPE                      = "INDEX_ON_COMPLEX_DATATYPE"
	FOREIGN_TABLE                                  = "FOREIGN_TABLE"
	INHERITANCE                                    = "INHERITANCE"

	AGGREGATE_FUNCTION             = "AGGREGATE_FUNCTION"
	AGGREGATION_FUNCTIONS_NAME     = "Aggregate Functions"
	JSON_TYPE_PREDICATE            = "JSON_TYPE_PREDICATE"
	JSON_TYPE_PREDICATE_NAME       = "Json Type Predicate"
	JSON_CONSTRUCTOR_FUNCTION      = "JSON_CONSTRUCTOR_FUNCTION"
	JSON_CONSTRUCTOR_FUNCTION_NAME = "Json Constructor Functions"
	JSON_QUERY_FUNCTION            = "JSON_QUERY_FUNCTION"
	JSON_QUERY_FUNCTIONS_NAME      = "Json Query Functions"
	LARGE_OBJECT_DATATYPE          = "LARGE_OBJECT_DATATYPE"
	LARGE_OBJECT_FUNCTIONS         = "LARGE_OBJECT_FUNCTIONS"
	LARGE_OBJECT_FUNCTIONS_NAME    = "Large Object Functions"

	SECURITY_INVOKER_VIEWS      = "SECURITY_INVOKER_VIEWS"
	SECURITY_INVOKER_VIEWS_NAME = "Security Invoker Views"

	ADVISORY_LOCKS      = "ADVISORY_LOCKS"
	SYSTEM_COLUMNS      = "SYSTEM_COLUMNS"
	XML_FUNCTIONS       = "XML_FUNCTIONS"
	ADVISORY_LOCKS_NAME = "Advisory Locks"
	SYSTEM_COLUMNS_NAME = "System Columns"
	XML_FUNCTIONS_NAME  = "XML Functions"
	FETCH_WITH_TIES     = "FETCH_WITH_TIES"
	REGEX_FUNCTIONS     = "REGEX_FUNCTIONS"

	JSONB_SUBSCRIPTING      = "JSONB_SUBSCRIPTING"
	JSONB_SUBSCRIPTING_NAME = "Jsonb Subscripting"
	MULTI_RANGE_DATATYPE    = "MULTI_RANGE_DATATYPE"
	COPY_FROM_WHERE         = "COPY FROM ... WHERE"
	COPY_ON_ERROR           = "COPY ... ON_ERROR"

	DETERMINISTIC_OPTION_WITH_COLLATION      = "DETERMINISTIC_OPTION_WITH_COLLATION"
	DETERMINISTIC_OPTION_WITH_COLLATION_NAME = "Deterministic attribute in collation"

	MERGE_STATEMENT                               = "MERGE_STATEMENT"
	MERGE_STATEMENT_NAME                          = "Merge Statement"
	FOREIGN_KEY_REFERENCES_PARTITIONED_TABLE      = "FOREIGN_KEY_REFERENCED_PARTITIONED_TABLE"
	FOREIGN_KEY_REFERENCES_PARTITIONED_TABLE_NAME = "Foreign key constraint references partitioned table"

	UNIQUE_NULLS_NOT_DISTINCT      = "UNIQUE_NULLS_NOT_DISTINCT"
	UNIQUE_NULLS_NOT_DISTINCT_NAME = "Unique Nulls Not Distinct"
)

const (
	// Issue Names
	INDEX_ON_COMPLEX_DATATYPE_ISSUE_NAME                      = "Index on column with complex datatype"
	UNSUPPORTED_INDEX_METHOD_ISSUE_NAME                       = "Index with access method"
	PK_UK_ON_COMPLEX_DATATYPE_ISSUE_NAME                      = "Primary/Unique key on column with complex datatype"
	UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_NAME            = "Unsupported datatype for Live migration"
	UNSUPPORTED_DATATYPE_LIVE_MIGRATION_WITH_FF_FB_ISSUE_NAME = "Unsupported datatype for Live migration with fall-forward/fallback"
)

// Issues Description
// Note: Any issue description added here should be updated in reasonsIncludingSensitiveInformationToCallhome slice in analyzeSchema.go
const (
	// for DMLs
	ADVISORY_LOCKS_ISSUE_DESCRIPTION            = "Advisory locks are not yet implemented in YugabyteDB"
	SYSTEM_COLUMNS_ISSUE_DESCRIPTION            = "System columns are not yet supported in YugabyteDB"
	XML_FUNCTIONS_ISSUE_DESCRIPTION             = "XML functions are not yet supported in YugabyteDB"
	REGEX_FUNCTIONS_ISSUE_DESCRIPTION           = "Regex functions are not yet supported in YugabyteDB"
	AGGREGATE_FUNCTION_ISSUE_DESCRIPTION        = "any_value, range_agg and range_intersect_agg functions are not supported yet in YugabyteDB"
	JSON_CONSTRUCTOR_FUNCTION_ISSUE_DESCRIPTION = "JSON constructor functions from PostgreSQL 17 are not yet supported in YugabyteDB"
	JSON_QUERY_FUNCTION_ISSUE_DESCRIPTION       = "JSON query functions from PostgreSQL 17 are not yet supported in YugabyteDB"
	LO_FUNCTIONS_ISSUE_DESCRIPTION              = "Large Objects functions are not supported in YugabyteDB"
	JSONB_SUBSCRIPTING_ISSUE_DESCRIPTION        = "Jsonb subscripting is not yet supported in YugabyteDB"
	JSON_PREDICATE_ISSUE_DESCRIPTION            = "IS JSON predicate expressions are not yet supported in YugabyteDB"
	COPY_FROM_WHERE_ISSUE_DESCRIPTION           = "COPY FROM ... WHERE is not yet supported in YugabyteDB"
	COPY_ON_ERROR_ISSUE_DESCRIPTION             = "COPY ... ON_ERROR is not yet supported in YugabyteDB"
	FETCH_WITH_TIES_ISSUE_DESCRIPTION           = "FETCH .. WITH TIES is not yet supported in YugabyteDB"
	MERGE_STATEMENT_ISSUE_DESCRIPTION           = "MERGE statement is not yet supported in YugabyteDB"

	// for DDLs
	STORED_GENERATED_COLUMNS_ISSUE_DESCRIPTION                       = "Stored generated columns are not supported in YugabyteDB. Detected columns are (%s)."
	UNLOGGED_TABLES_ISSUE_DESCRIPTION                                = "UNLOGGED tables are not yet supported in YugabyteDB"
	UNSUPPORTED_INDEX_METHOD_DESCRIPTION                             = "The schema contains an index with an access method '%s' which is not supported in YugabyteDB."
	STORAGE_PARAMETERS_ISSUE_DESCRIPTION                             = "Storage parameters in tables, indexes, and constraints are not yet supported in YugabyteDB"
	ALTER_TABLE_SET_COLUMN_ATTRIBUTE_ISSUE_DESCRIPTION               = "ALTER TABLE .. ALTER COLUMN .. SET ( attribute = value ) is not yet supported in YugabyteDB"
	ALTER_TABLE_CLUSTER_ON_ISSUE_DESCRIPTION                         = "ALTER TABLE CLUSTER is not yet supported in YugabyteDB"
	ALTER_TABLE_DISABLE_RULE_ISSUE_DESCRIPTION                       = "ALTER TABLE name DISABLE RULE is not yet supported in YugabyteDB"
	EXCLUSION_CONSTRAINT_ISSUE_DESCRIPTION                           = "Exclusion constraints are not yet supported in YugabyteDB"
	DEFERRABLE_CONSTRAINT_ISSUE_DESCRIPTION                          = "Deferrable constraints are not yet supported in YugabyteDB"
	MULTI_COLUMN_GIN_INDEX_ISSUE_DESCRIPTION                         = "GIN indexes on multiple columns are not supported in YugabyteDB"
	ORDERED_GIN_INDEX_ISSUE_DESCRIPTION                              = "GIN indexes on columns with ASC/DESC/HASH clause are not yet supported in YugabyteDB"
	POLICY_ROLE_ISSUE_DESCRIPTION                                    = "Policies require specific roles (%s) to be created in the target database, as roles are not migrated during schema migration. Manually create these roles before running the import schema."
	CONSTRAINT_TRIGGER_ISSUE_DESCRIPTION                             = "CONSTRAINT TRIGGER is not yet supported in YugabyteDB"
	REFERENCING_CLAUSE_IN_TRIGGER_ISSUE_DESCRIPTION                  = "REFERENCING clause (transition tables) in triggers is not yet supported in YugabyteDB"
	BEFORE_ROW_TRIGGER_ON_PARTITION_TABLE_ISSUE_DESCRIPTION          = "BEFORE ROW triggers on partitioned tables are not yet supported in YugabyteDB"
	ALTER_TABLE_ADD_PK_ON_PARTITION_ISSUE_DESCRIPTION                = "Adding primary key using ALTER TABLE to a partitioned table is not yet supported in YugabyteDB. After export schema, the ALTER table should be merged with CREATE table for partitioned tables"
	EXPRESSION_PARTITION_ISSUE_DESCRIPTION                           = "Tables partitioned using expressions cannot contain primary or unique keys in YugabyteDB"
	MULTI_COLUMN_LIST_PARTITION_ISSUE_DESCRIPTION                    = "Multi-column partition by list i.e. PARTITION BY LIST (col1, col2) is not supported in YugabyteDB"
	INSUFFICIENT_COLUMNS_IN_PK_FOR_PARTITION_ISSUE_DESCRIPTION       = "Partition key columns(%s) are not part of Primary Key columns which is not supported in YugabyteDB"
	XML_DATATYPE_ISSUE_DESCRIPTION                                   = "XML datatype is not yet supported in YugabyteDB. Affected column: %s."
	XID_DATATYPE_ISSUE_DESCRIPTION                                   = "XID datatype is not yet supported in YugabyteDB. Affected column: %s."
	POSTGIS_DATATYPE_ISSUE_DESCRIPTION                               = "PostGIS datatypes are not yet supported in YugabyteDB. Affected column: %s and type: %s."
	UNSUPPORTED_DATATYPE_ISSUE_DESCRIPTION                           = "Datatype not yet supported in YugabyteDB. Affected column: %s and type: %s."
	UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_DESCRIPTION            = "Datatype not yet supported by voyager in live migration. Affected column: %s and type: %s. These columns will be excluded when exporting and importing data in live migration workflows."
	UNSUPPORTED_DATATYPE_LIVE_MIGRATION_WITH_FF_FB_ISSUE_DESCRIPTION = "Datatype not yet supported by voyager in live migration with fall-forward/fallback. Affected column: %s and type: %s. These columns will be excluded when exporting and importing data in live migration workflows."
	PK_UK_ON_COMPLEX_DATATYPE_ISSUE_DESCRIPTION                      = "Primary key and Unique constraints on columns with complex data types like '%s' are not yet supported in YugabyteDB."
	INDEX_ON_COMPLEX_DATATYPE_ISSUE_DESCRIPTION                      = "Indexes on columns with complex data types like '%s' are not yet supported in YugabyteDB"
	FOREIGN_TABLE_ISSUE_DESCRIPTION                                  = "Foreign table creation fails as SERVER and USER MAPPING objects are not exported by voyager. These should be manually created to make the foreign tables work."
	INHERITANCE_ISSUE_DESCRIPTION                                    = "Table inheritance is not yet supported in YugabyteDB"
	REFERENCED_TYPE_DECLARATION_ISSUE_DESCRIPTION                    = "Referencing the type of a column instead of the actual type name is not supported in YugabyteDB"
	LARGE_OBJECT_DATATYPE_ISSUE_DESCRIPTION                          = "Large Objects are not yet supported in YugabyteDB. Affected column: %s."
	MULTI_RANGE_DATATYPE_ISSUE_DESCRIPTION                           = "Multi-range data type is not yet supported in YugabyteDB. Affected column: %s and type: %s."
	SECURITY_INVOKER_VIEWS_ISSUE_DESCRIPTION                         = "Security invoker views are not yet supported in YugabyteDB"
	DETERMINISTIC_OPTION_WITH_COLLATION_ISSUE_DESCRIPTION            = "Deterministic option/attribute with collation is not yet supported in YugabyteDB"
	FOREIGN_KEY_REFERENCES_PARTITIONED_TABLE_ISSUE_DESCRIPTION       = "Foreign key references to partitioned table are not yet supported in YugabyteDB"
	UNIQUE_NULLS_NOT_DISTINCT_ISSUE_DESCRIPTION                      = "Unique constraint on columns with NULL values is not yet supported in YugabyteDB"
)

// Object types
const (
	CONSTRAINT_NAME           = "ConstraintName"
	FUNCTION_NAMES            = "FunctionNames"
	TABLE_OBJECT_TYPE         = "TABLE"
	FOREIGN_TABLE_OBJECT_TYPE = "FOREIGN TABLE"
	FUNCTION_OBJECT_TYPE      = "FUNCTION"
	INDEX_OBJECT_TYPE         = "INDEX"
	POLICY_OBJECT_TYPE        = "POLICY"
	TRIGGER_OBJECT_TYPE       = "TRIGGER"
	DML_QUERY_OBJECT_TYPE     = "DML_QUERY"
)
