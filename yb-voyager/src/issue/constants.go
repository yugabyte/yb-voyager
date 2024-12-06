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
	ADVISORY_LOCKS = "ADVISORY_LOCKS"
	SYSTEM_COLUMNS = "SYSTEM_COLUMNS"
	XML_FUNCTIONS  = "XML_FUNCTIONS"

	REFERENCED_TYPE_DECLARATION                     = "REFERENCED_TYPE_DECLARATION"
	STORED_GENERATED_COLUMNS                        = "STORED_GENERATED_COLUMNS"
	UNLOGGED_TABLE                                  = "UNLOGGED_TABLE"
	UNSUPPORTED_INDEX_METHOD                        = "UNSUPPORTED_INDEX_METHOD"
	STORAGE_PARAMETER                               = "STORAGE_PARAMETER"
	SET_ATTRIBUTES                                  = "SET_ATTRIBUTES"
	CLUSTER_ON                                      = "CLUSTER_ON"
	DISABLE_RULE                                    = "DISABLE_RULE"
	EXCLUSION_CONSTRAINTS                           = "EXCLUSION_CONSTRAINTS"
	DEFERRABLE_CONSTRAINTS                          = "DEFERRABLE_CONSTRAINTS"
	MULTI_COLUMN_GIN_INDEX                          = "MULTI_COLUMN_GIN_INDEX"
	ORDERED_GIN_INDEX                               = "ORDERED_GIN_INDEX"
	POLICY_WITH_ROLES                               = "POLICY_WITH_ROLES"
	CONSTRAINT_TRIGGER                              = "CONSTRAINT_TRIGGER"
	REFERENCING_CLAUSE_FOR_TRIGGERS                 = "REFERENCING_CLAUSE_FOR_TRIGGERS"
	BEFORE_ROW_TRIGGER_ON_PARTITIONED_TABLE         = "BEFORE_ROW_TRIGGER_ON_PARTITIONED_TABLE"
	ALTER_TABLE_ADD_PK_ON_PARTITIONED_TABLE         = "ALTER_TABLE_ADD_PK_ON_PARTITIONED_TABLE"
	EXPRESSION_PARTITION_WITH_PK_UK                 = "EXPRESSION_PARTITION_WITH_PK_UK"
	MULTI_COLUMN_LIST_PARTITION                     = "MULTI_COLUMN_LIST_PARTITION"
	INSUFFICIENT_COLUMNS_IN_PK_FOR_PARTITION        = "INSUFFICIENT_COLUMNS_IN_PK_FOR_PARTITION"
	XML_DATATYPE                                    = "XML_DATATYPE"
	XID_DATATYPE                                    = "XID_DATATYPE"
	POSTGIS_DATATYPES                               = "POSTGIS_DATATYPES"
	UNSUPPORTED_DATATYPES                           = "UNSUPPORTED_TYPES"
	UNSUPPORTED_DATATYPES_LIVE_MIGRATION            = "UNSUPPORTED_DATATYPES_LIVE_MIGRATION"
	UNSUPPORTED_DATATYPES_LIVE_MIGRATION_WITH_FF_FB = "UNSUPPORTED_DATATYPES_LIVE_MIGRATION_WITH_FF_FB"
	PK_UK_ON_COMPLEX_DATATYPE                       = "PK_UK_ON_COMPLEX_DATATYPE"
	INDEX_ON_COMPLEX_DATATYPE                       = "INDEX_ON_COMPLEX_DATATYPE"
	FOREIGN_TABLE                                   = "FOREIGN_TABLE"
	INHERITANCE                                     = "INHERITANCE"
)

// Object types
const (
	TABLE_OBJECT_TYPE         = "TABLE"
	FOREIGN_TABLE_OBJECT_TYPE = "FOREIGN TABLE"
	FUNCTION_OBJECT_TYPE      = "FUNCTION"
	INDEX_OBJECT_TYPE         = "INDEX"
	POLICY_OBJECT_TYPE        = "POLICY"
	TRIGGER_OBJECT_TYPE       = "TRIGGER"
	DML_QUERY_OBJECT_TYPE     = "DML_QUERY"
)
