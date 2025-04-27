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
package cmd

const (
	KB                              = 1024
	MB                              = 1024 * 1024
	META_INFO_DIR_NAME              = "metainfo"
	NEWLINE                         = '\n'
	ORACLE_DEFAULT_PORT             = 1521
	MYSQL_DEFAULT_PORT              = 3306
	POSTGRES_DEFAULT_PORT           = 5432
	YUGABYTEDB_YSQL_DEFAULT_PORT    = 5433
	YUGABYTEDB_DEFAULT_DATABASE     = "yugabyte"
	YUGABYTEDB_DEFAULT_SCHEMA       = "public"
	ORACLE                          = "oracle"
	MYSQL                           = "mysql"
	POSTGRESQL                      = "postgresql"
	YUGABYTEDB                      = "yugabytedb"
	LAST_SPLIT_NUM                  = 0
	SPLIT_INFO_PATTERN              = "[0-9]*.[0-9]*.[0-9]*.[0-9]*"
	LAST_SPLIT_PATTERN              = "0.[0-9]*.[0-9]*.[0-9]*"
	COPY_MAX_RETRY_COUNT            = 10
	MAX_SLEEP_SECOND                = 60
	DEFAULT_BATCH_SIZE_ORACLE       = 10000000
	DEFAULT_BATCH_SIZE_YUGABYTEDB   = 20000
	DEFAULT_BATCH_SIZE_POSTGRESQL   = 100000
	INDEX_RETRY_COUNT               = 5
	DDL_MAX_RETRY_COUNT             = 5
	SCHEMA_VERSION_MISMATCH_ERR     = "Query error: schema version mismatch for table"
	SNAPSHOT_ONLY                   = "snapshot-only"
	SNAPSHOT_AND_CHANGES            = "snapshot-and-changes"
	CHANGES_ONLY                    = "changes-only"
	TARGET_DB                       = "target"
	FF_DB                           = "ff"
	SOURCE_REPLICA_DB_IMPORTER_ROLE = "source_replica_db_importer"
	SOURCE_DB_IMPORTER_ROLE         = "source_db_importer"
	TARGET_DB_IMPORTER_ROLE         = "target_db_importer"
	SOURCE_DB_EXPORTER_ROLE         = "source_db_exporter"
	TARGET_DB_EXPORTER_FF_ROLE      = "target_db_exporter_ff"
	TARGET_DB_EXPORTER_FB_ROLE      = "target_db_exporter_fb"
	IMPORT_FILE_ROLE                = "import_file"
	ROW_UPDATE_STATUS_NOT_STARTED   = 0
	ROW_UPDATE_STATUS_IN_PROGRESS   = 1
	ROW_UPDATE_STATUS_COMPLETED     = 3
	COLOCATION_CLAUSE               = "colocation"
	LOW                             = "LOW"
	MEDIUM                          = "MEDIUM"
	HIGH                            = "HIGH"
	//phase names used in call-home payload
	ANALYZE_PHASE                    = "analyze-schema"
	EXPORT_SCHEMA_PHASE              = "export-schema"
	EXPORT_DATA_PHASE                = "export-data"
	EXPORT_DATA_FROM_TARGET_PHASE    = "export-data-from-target"
	IMPORT_SCHEMA_PHASE              = "import-schema"
	IMPORT_DATA_PHASE                = "import-data"
	IMPORT_DATA_SOURCE_REPLICA_PHASE = "import-data-to-source-repilca"
	IMPORT_DATA_SOURCE_PHASE         = "import-data-to-source"
	END_MIGRATION_PHASE              = "end-migration"
	ASSESS_MIGRATION_PHASE           = "assess-migration"
	ASSESS_MIGRATION_BULK_PHASE      = "assess-migration-bulk"
	IMPORT_DATA_FILE_PHASE           = "import-data-file"
	//...more phases
	OFFLINE        = "offline"
	LIVE_MIGRATION = "live-migration"
	BULK_DATA_LOAD = "bulk-data-load-from-flat-files"
	FALL_BACK      = "fall-back"
	FALL_FORWARD   = "fall-forward"
	AWS_S3         = "aws-s3"
	GCS_BUCKETS    = "gcs-buckets"
	AZURE_BLOBS    = "azure-blob-storage"
	LOCAL_DISK     = "local-disk"
	//status
	ERROR                     = "ERROR"
	EXIT                      = "EXIT"
	COMPLETE                  = "COMPLETE"
	COMPLETE_WITH_ERRORS      = "COMPLETE-WITH-ERRORS"
	INPROGRESS                = "IN-PROGRESS"
	CUTOVER_TO_TARGET         = "cutover-to-target"
	CUTOVER_TO_SOURCE         = "cutover-to-source"
	CUTOVER_TO_SOURCE_REPLICA = "cutover-to-source-replica"

	// unsupported features of assess migration
	VIRTUAL_COLUMN      = "VIRTUAL COLUMN"
	INHERITED_TYPE      = "INHERITED TYPE"
	REFERENCE_PARTITION = "REFERENCE PARTITION"
	SYSTEM_PARTITION    = "SYSTEM PARTITION"

	UNSUPPORTED_FEATURES                = "unsupported_features"
	UNSUPPORTED_DATATYPES               = "unsupported_datatypes"
	UNSUPPORTED_PLPGSQL_OBEJCTS         = "unsupported_plpgsql_objects"
	REPORT_UNSUPPORTED_QUERY_CONSTRUCTS = "REPORT_UNSUPPORTED_QUERY_CONSTRUCTS"

	HTML = "html"
	JSON = "json"

	TABLE     = "TABLE"
	MVIEW     = "MVIEW"
	YUGABYTED = "yugabyted"

	// assess-migration-bulk
	SOURCE_DB_TYPE     = "source-db-type"
	SOURCE_DB_HOST     = "source-db-host"
	SOURCE_DB_PORT     = "source-db-port"
	SOURCE_DB_NAME     = "source-db-name"
	ORACLE_DB_SID      = "oracle-db-sid"
	ORACLE_TNS_ALIAS   = "oracle-tns-alias"
	SOURCE_DB_USER     = "source-db-user"
	SOURCE_DB_PASSWORD = "source-db-password"
	SOURCE_DB_SCHEMA   = "source-db-schema"

	HTML_EXTENSION            = ".html"
	JSON_EXTENSION            = ".json"
	ASSESSMENT_FILE_NAME      = "migration_assessment_report"
	ANALYSIS_REPORT_FILE_NAME = "schema_analysis_report"
	BULK_ASSESSMENT_FILE_NAME = "bulk_assessment_report"

	//adding constants for docs link
	DOCS_LINK_PREFIX                              = "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/"
	POSTGRESQL_PREFIX                             = "postgresql/"
	MYSQL_PREFIX                                  = "mysql/"
	ORACLE_PREFIX                                 = "oracle/"
	ADDING_PK_TO_PARTITIONED_TABLE_DOC_LINK       = DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#adding-primary-key-to-a-partitioned-table-results-in-an-error"
	CREATE_CONVERSION_DOC_LINK                    = DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#create-or-alter-conversion-is-not-supported"
	GENERATED_STORED_COLUMN_DOC_LINK              = DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#generated-always-as-stored-type-column-is-not-supported"
	UNSUPPORTED_ALTER_VARIANTS_DOC_LINK           = DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#unsupported-alter-table-ddl-variants-in-source-schema"
	STORAGE_PARAMETERS_DDL_STMT_DOC_LINK          = DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#storage-parameters-on-indexes-or-constraints-in-the-source-postgresql"
	FOREIGN_TABLE_DOC_LINK                        = DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#foreign-table-in-the-source-database-requires-server-and-user-mapping"
	EXCLUSION_CONSTRAINT_DOC_LINK                 = DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#exclusion-constraints-is-not-supported"
	EXTENSION_DOC_LINK                            = "https://docs.yugabyte.com/preview/explore/ysql-language-features/pg-extensions/"
	DEFERRABLE_CONSTRAINT_DOC_LINK                = DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#deferrable-constraint-on-constraints-other-than-foreign-keys-is-not-supported"
	XML_DATATYPE_DOC_LINK                         = DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#data-ingestion-on-xml-data-type-is-not-supported"
	UNSUPPORTED_INDEX_METHODS_DOC_LINK            = DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#gist-brin-and-spgist-index-types-are-not-supported"
	CONSTRAINT_TRIGGER_DOC_LINK                   = DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#constraint-trigger-is-not-supported"
	REFERENCING_CLAUSE_TRIGGER_DOC_LINK           = DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#referencing-clause-for-triggers"
	BEFORE_ROW_TRIGGER_PARTITIONED_TABLE_DOC_LINK = DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#before-row-triggers-on-partitioned-tables"
	INHERITANCE_DOC_LINK                          = DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#table-inheritance-is-not-supported"
	GIN_INDEX_MULTI_COLUMN_DOC_LINK               = DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#gin-indexes-on-multiple-columns-are-not-supported"
	GIN_INDEX_DIFFERENT_ISSUE_DOC_LINK            = DOCS_LINK_PREFIX + ORACLE_PREFIX + "#issue-in-some-unsupported-cases-of-gin-indexes"
	POLICY_DOC_LINK                               = DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#policies-on-users-in-source-require-manual-user-creation"
	VIEW_CHECK_OPTION_DOC_LINK                    = DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#view-with-check-option-is-not-supported"
	EXPRESSION_PARTIITON_DOC_LINK                 = DOCS_LINK_PREFIX + MYSQL_PREFIX + "#tables-partitioned-with-expressions-cannot-contain-primary-unique-keys"
	LIST_PARTIION_MULTI_COLUMN_DOC_LINK           = DOCS_LINK_PREFIX + MYSQL_PREFIX + "#multi-column-partition-by-list-is-not-supported"
	PARTITION_KEY_NOT_PK_DOC_LINK                 = DOCS_LINK_PREFIX + ORACLE_PREFIX + "#partition-key-column-not-part-of-primary-key-columns"
	DROP_TEMP_TABLE_DOC_LINK                      = DOCS_LINK_PREFIX + MYSQL_PREFIX + "#drop-temporary-table-statements-are-not-supported"
	INDEX_ON_UNSUPPORTED_TYPE                     = DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#indexes-on-some-complex-data-types-are-not-supported"
	PK_UK_CONSTRAINT_ON_UNSUPPORTED_TYPE          = DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#indexes-on-some-complex-data-types-are-not-supported" //Keeping it similar for now, will see if we need to a separate issue on docs
	UNLOGGED_TABLE_DOC_LINK                       = DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#unlogged-table-is-not-supported"
	XID_DATATYPE_DOC_LINK                         = DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#xid-functions-is-not-supported"
	UNSUPPORTED_DATATYPES_DOC_LINK                = DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#unsupported-datatypes-by-yugabytedb"
	UNSUPPORTED_DATATYPE_LIVE_MIGRATION_DOC_LINK  = DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#unsupported-datatypes-by-voyager-during-live-migration"
	// temporary, till unsupported datatype documentation is shifted to known issues for Oracle
	UNSUPPORTED_DATATYPES_DOC_LINK_ORACLE = "https://docs.yugabyte.com/preview/yugabyte-voyager/reference/datatype-mapping-oracle"
)

/*
List of all the features we are reporting as part of Unsupported features and Migration caveats
*/
const (
	// Types for AssessmentIssue
	FEATURE           = "feature"
	DATATYPE          = "datatype"
	QUERY_CONSTRUCT   = "query_construct" // confused: in json for some values we are using space separated and for some snake_case
	MIGRATION_CAVEATS = "migration_caveats"
	PLPGSQL_OBJECT    = "plpgsql_object"

	// Description
	FEATURE_ISSUE_TYPE_DESCRIPTION          = "Features of the source database that are not supported on the target YugabyteDB."
	DATATYPE_ISSUE_TYPE_DESCRIPTION         = "Data types of the source database that are not supported on the target YugabyteDB."
	MIGRATION_CAVEATS_TYPE_DESCRIPTION      = "Migration Caveats highlights the current limitations with the migration workflow."
	UNSUPPORTED_QUERY_CONSTRUTS_DESCRIPTION = "Source database queries not supported in YugabyteDB, identified by scanning system tables."
	UNSUPPPORTED_PLPGSQL_OBJECT_DESCRIPTION = "Source schema objects having unsupported statements on the target YugabyteDB in PL/pgSQL code block"
	SCHEMA_SUMMARY_DESCRIPTION              = "Objects that will be created on the target YugabyteDB."
	SCHEMA_SUMMARY_DESCRIPTION_ORACLE       = SCHEMA_SUMMARY_DESCRIPTION + " Some of the index and sequence names might be different from those in the source database."

	//Unsupported Features

	//Oracle
	UNSUPPORTED_INDEXES_FEATURE              = "Unsupported Indexes"
	VIRTUAL_COLUMNS_FEATURE                  = "Virtual Columns"
	INHERITED_TYPES_FEATURE                  = "Inherited Types"
	UNSUPPORTED_PARTITIONING_METHODS_FEATURE = "Unsupported Partitioning Methods"
	COMPOUND_TRIGGER_FEATURE                 = "Compound Triggers"

	//POSTGRESQL
	CONSTRAINT_TRIGGERS_FEATURE                               = "Constraint triggers"
	INHERITED_TABLES_FEATURE                                  = "Inherited tables"
	GENERATED_COLUMNS_FEATURE                                 = "Tables with stored generated columns"
	CONVERSIONS_OBJECTS_FEATURE                               = "Conversion objects"
	MULTI_COLUMN_GIN_INDEX_FEATURE                            = "Gin indexes on multi-columns"
	ALTER_SETTING_ATTRIBUTE_FEATURE                           = "Setting attribute=value on column"
	DISABLING_TABLE_RULE_FEATURE                              = "Disabling rule on table"
	CLUSTER_ON_FEATURE                                        = "Clustering table on index"
	STORAGE_PARAMETERS_FEATURE                                = "Storage parameters in DDLs"
	EXTENSION_FEATURE                                         = "Extensions"
	EXCLUSION_CONSTRAINT_FEATURE                              = "Exclusion constraints"
	DEFERRABLE_CONSTRAINT_FEATURE                             = "Deferrable constraints"
	VIEW_CHECK_FEATURE                                        = "View with check option"
	UNLOGGED_TABLE_FEATURE                                    = "Unlogged tables"
	REFERENCING_TRIGGER_FEATURE                               = "REFERENCING clause for triggers"
	BEFORE_FOR_EACH_ROW_TRIGGERS_ON_PARTITIONED_TABLE_FEATURE = "BEFORE ROW triggers on Partitioned tables"
	PK_UK_CONSTRAINT_ON_COMPLEX_DATATYPES_FEATURE             = "Primary / Unique key constraints on complex datatypes"
	REGEX_FUNCTIONS_FEATURE                                   = "Regex Functions"

	// Migration caveats

	//POSTGRESQL
	ALTER_PARTITION_ADD_PK_CAVEAT_FEATURE                     = "Alter partitioned tables to add Primary Key"
	FOREIGN_TABLE_CAVEAT_FEATURE                              = "Foreign tables"
	POLICIES_CAVEAT_FEATURE                                   = "Policies"
	UNSUPPORTED_DATATYPES_LIVE_CAVEAT_FEATURE                 = "Unsupported Data Types for Live Migration"
	UNSUPPORTED_DATATYPES_LIVE_WITH_FF_FB_CAVEAT_FEATURE      = "Unsupported Data Types for Live Migration with Fall-forward/Fallback"
	DESCRIPTION_ADD_PK_TO_PARTITION_TABLE                     = `After export schema, the ALTER table should be merged with CREATE table for partitioned tables as alter of partitioned tables to add primary key is not supported.`
	DESCRIPTION_FOREIGN_TABLES                                = `During the export schema phase, SERVER and USER MAPPING objects are not exported. These should be manually created to make the foreign tables work.`
	DESCRIPTION_POLICY_ROLE_ISSUE                             = `There are some policies that are created for certain users/roles. During the export schema phase, USERs and GRANTs are not exported. Therefore, they will have to be manually created before running import schema.`
	UNSUPPORTED_DATATYPES_FOR_LIVE_MIGRATION_ISSUE            = "There are some data types in the schema that are not supported by live migration of data. These columns will be excluded when exporting and importing data in live migration workflows."
	UNSUPPORTED_DATATYPES_FOR_LIVE_MIGRATION_WITH_FF_FB_ISSUE = "There are some data types in the schema that are not supported by live migration with fall-forward/fall-back. These columns will be excluded when exporting and importing data in live migration workflows."
)

var supportedSourceDBTypes = []string{ORACLE, MYSQL, POSTGRESQL, YUGABYTEDB}
var validExportTypes = []string{SNAPSHOT_ONLY, CHANGES_ONLY, SNAPSHOT_AND_CHANGES}

var validSSLModes = map[string][]string{
	"mysql":      {"disable", "prefer", "require", "verify-ca", "verify-full"},
	"postgresql": {"disable", "allow", "prefer", "require", "verify-ca", "verify-full"},
	"yugabytedb": {"disable", "allow", "prefer", "require", "verify-ca", "verify-full"},
}

var EVENT_BATCH_MAX_RETRY_COUNT = 50
