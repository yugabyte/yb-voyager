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

	UNSUPPORTED_FEATURES  = "unsupported_features"
	UNSUPPORTED_DATATYPES = "unsupported_datatypes"
	MIGRATION_CAVEATS     = "migration_caveats"

	TABLE     = "TABLE"
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
	DOCS_LINK_PREFIX                        = "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/"
	ADDING_PK_TO_PARTITIONED_TABLE_DOC_LINK = DOCS_LINK_PREFIX + "postgresql/#adding-primary-key-to-a-partitioned-table-results-in-an-error"
	CREATE_CONVERSION_DOC_LINK              = DOCS_LINK_PREFIX + "postgresql/#create-or-alter-conversion-is-not-supported"
	GENERATED_STORED_COLUMN_DOC_LINK        = DOCS_LINK_PREFIX + "postgresql/#generated-always-as-stored-type-column-is-not-supported"
	UNSUPPORTED_ALTER_VARIANTS_DOC_LINK     = DOCS_LINK_PREFIX + "postgresql/#unsupported-alter-table-ddl-variants-in-source-schema"
	STORAGE_PARAMETERS_DDL_STMT_DOC_LINK    = DOCS_LINK_PREFIX + "postgresql/#storage-parameters-on-indexes-or-constraints-in-the-source-postgresql"
	FOREIGN_TABLE_DOC_LINK                  = DOCS_LINK_PREFIX + "postgresql/#foreign-table-in-the-source-database-requires-server-and-user-mapping"
	EXCLUSION_CONSTRAINT_DOC_LINK           = DOCS_LINK_PREFIX + "postgresql/#exclusion-constraints-is-not-supported"
	EXTENSION_DOC_LINK                      = "https://docs.yugabyte.com/preview/explore/ysql-language-features/pg-extensions/"
	DEFERRABLE_CONSTRAINT_DOC_LINK          = DOCS_LINK_PREFIX + "postgresql/#deferrable-constraint-on-constraints-other-than-foreign-keys-is-not-supported"
	XML_DATATYPE_DOC_LINK                   = DOCS_LINK_PREFIX + "postgresql/#data-ingestion-on-xml-data-type-is-not-supported"
	GIST_INDEX_DOC_LINK                     = DOCS_LINK_PREFIX + "postgresql/#gist-index-type-is-not-supported"
	CONSTRAINT_TRIGGER_DOC_LINK             = DOCS_LINK_PREFIX + "postgresql/#constraint-trigger-is-not-supported"
	INHERITANCE_DOC_LINK                    = DOCS_LINK_PREFIX + "postgresql/#table-inheritance-is-not-supported"
	GIN_INDEX_MULTI_COLUMN_DOC_LINK         = DOCS_LINK_PREFIX + "postgresql/#gin-indexes-on-multiple-columns-are-not-supported"
	GIN_INDEX_DIFFERENT_ISSUE_DOC_LINK      = DOCS_LINK_PREFIX + "oracle/#issue-in-some-unsupported-cases-of-gin-indexes"
	POLICY_DOC_LINK                         = DOCS_LINK_PREFIX + "postgresql/#policies-on-users-in-source-require-manual-user-creation"
	VIEW_CHECK_OPTION_DOC_LINK              = DOCS_LINK_PREFIX + "postgresql/#view-with-check-option-is-not-supported"
	EXPRESSION_PARTIITON_DOC_LINK           = DOCS_LINK_PREFIX + "mysql-oracle/#tables-partitioned-with-expressions-cannot-contain-primary-unique-keys"
	LIST_PARTIION_MULTI_COLUMN_DOC_LINK     = DOCS_LINK_PREFIX + "mysql-oracle/#multi-column-partition-by-list-is-not-supported"
	PARTITION_KEY_NOT_PK_DOC_LINK           = DOCS_LINK_PREFIX + "oracle/#partition-key-column-not-part-of-primary-key-columns"
	DROP_TEMP_TABLE_DOC_LINK                = DOCS_LINK_PREFIX + "mysql/#drop-temporary-table-statements-are-not-supported"
	UNLOGGED_TABLE_DOC_LINK                 = DOCS_LINK_PREFIX + "postgresql/#unlogged-table-is-not-supported"
)

var supportedSourceDBTypes = []string{ORACLE, MYSQL, POSTGRESQL, YUGABYTEDB}
var validExportTypes = []string{SNAPSHOT_ONLY, CHANGES_ONLY, SNAPSHOT_AND_CHANGES}

var validSSLModes = map[string][]string{
	"mysql":      {"disable", "prefer", "require", "verify-ca", "verify-full"},
	"postgresql": {"disable", "allow", "prefer", "require", "verify-ca", "verify-full"},
	"yugabytedb": {"disable", "allow", "prefer", "require", "verify-ca", "verify-full"},
}

var EVENT_BATCH_MAX_RETRY_COUNT = 50
