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

package issue

// Types
const (
	ADVISORY_LOCKS           = "ADVISORY_LOCKS"
	SYSTEM_COLUMNS           = "SYSTEM_COLUMNS"
	XML_FUNCTIONS            = "XML_FUNCTIONS"
	STORED_GENERATED_COLUMNS = "STORED_GENERATED_COLUMNS"
	UNLOGGED_TABLE           = "UNLOGGED_TABLE"
	UNSUPPORTED_INDEX_METHOD  = "UNSUPPORTED_INDEX_METHOD"
)

// Object types
const (
	TABLE_OBJECT_TYPE     = "TABLE"
	FUNCTION_OBJECT_TYPE  = "FUNCTION"
	DML_QUERY_OBJECT_TYPE = "DML_QUERY"
)

const (
	CONVERSION_ISSUE_REASON                           = "CREATE CONVERSION is not supported yet"
	GIN_INDEX_MULTI_COLUMN_ISSUE_REASON               = "Schema contains gin index on multi column which is not supported."
	ADDING_PK_TO_PARTITIONED_TABLE_ISSUE_REASON       = "Adding primary key to a partitioned table is not supported yet."
	INHERITANCE_ISSUE_REASON                          = "TABLE INHERITANCE not supported in YugabyteDB"
	CONSTRAINT_TRIGGER_ISSUE_REASON                   = "CONSTRAINT TRIGGER not supported yet."
	REFERENCING_CLAUSE_FOR_TRIGGERS                   = "REFERENCING clause (transition tables) not supported yet."
	BEFORE_FOR_EACH_ROW_TRIGGERS_ON_PARTITIONED_TABLE = "Partitioned tables cannot have BEFORE / FOR EACH ROW triggers."
	COMPOUND_TRIGGER_ISSUE_REASON                     = "COMPOUND TRIGGER not supported in YugabyteDB."

	STORED_GENERATED_COLUMN_ISSUE_REASON           = "Stored generated columns are not supported."
	UNSUPPORTED_EXTENSION_ISSUE                    = "This extension is not supported in YugabyteDB by default."
	EXCLUSION_CONSTRAINT_ISSUE                     = "Exclusion constraint is not supported yet"
	ALTER_TABLE_DISABLE_RULE_ISSUE                 = "ALTER TABLE name DISABLE RULE not supported yet"
	STORAGE_PARAMETERS_DDL_STMT_ISSUE              = "Storage parameters are not supported yet."
	ALTER_TABLE_SET_ATTRIBUTE_ISSUE                = "ALTER TABLE .. ALTER COLUMN .. SET ( attribute = value )	 not supported yet"
	FOREIGN_TABLE_ISSUE_REASON                     = "Foreign tables require manual intervention."
	ALTER_TABLE_CLUSTER_ON_ISSUE                   = "ALTER TABLE CLUSTER not supported yet."
	DEFERRABLE_CONSTRAINT_ISSUE                    = "DEFERRABLE constraints not supported yet"
	POLICY_ROLE_ISSUE                              = "Policy require roles to be created."
	VIEW_CHECK_OPTION_ISSUE                        = "Schema containing VIEW WITH CHECK OPTION is not supported yet."
	ISSUE_INDEX_WITH_COMPLEX_DATATYPES             = `INDEX on column '%s' not yet supported`
	ISSUE_PK_UK_CONSTRAINT_WITH_COMPLEX_DATATYPES  = `Primary key and Unique constraint on column '%s' not yet supported`
	ISSUE_UNLOGGED_TABLE                           = "UNLOGGED tables are not supported yet."
	UNSUPPORTED_DATATYPE                           = "Unsupported datatype"
	UNSUPPORTED_DATATYPE_LIVE_MIGRATION            = "Unsupported datatype for Live migration"
	UNSUPPORTED_DATATYPE_LIVE_MIGRATION_WITH_FF_FB = "Unsupported datatype for Live migration with fall-forward/fallback"
	UNSUPPORTED_PG_SYNTAX                          = "Unsupported PG syntax"

	INDEX_METHOD_ISSUE_REASON                = "Schema contains %s index which is not supported."
	INSUFFICIENT_COLUMNS_IN_PK_FOR_PARTITION = "insufficient columns in the PRIMARY KEY constraint definition in CREATE TABLE"
	GIN_INDEX_DETAILS                        = "There are some GIN indexes present in the schema, but GIN indexes are partially supported in YugabyteDB as mentioned in (https://github.com/yugabyte/yugabyte-db/issues/7850) so take a look and modify them if not supported."
)
