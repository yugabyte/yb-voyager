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

import (
	"fmt"
	"strings"
)

const (
	DOCS_LINK_PREFIX  = "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/"
	POSTGRESQL_PREFIX = "postgresql/"
	MYSQL_PREFIX      = "mysql/"
	ORACLE_PREFIX     = "oracle/"
)

var generatedColumnsIssue = Issue{
	Type:       STORED_GENERATED_COLUMNS,
	TypeName:   "Stored generated columns are not supported.",
	GH:         "https://github.com/yugabyte/yugabyte-db/issues/10695",
	Suggestion: "Using Triggers to update the generated columns is one way to work around this issue, refer docs link for more details.",
	DocsLink:   DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#generated-always-as-stored-type-column-is-not-supported",
}

func NewGeneratedColumnsIssue(objectType string, objectName string, sqlStatement string, generatedColumns []string) IssueInstance {
	issue := generatedColumnsIssue
	issue.TypeName = issue.TypeName + fmt.Sprintf(" Generated Columns: (%s)", strings.Join(generatedColumns, ","))
	return newIssueInstance(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var unloggedTableIssue = Issue{
	Type:       UNLOGGED_TABLE,
	TypeName:   "UNLOGGED tables are not supported yet.",
	GH:         "https://github.com/yugabyte/yugabyte-db/issues/1129/",
	Suggestion: "Remove UNLOGGED keyword to make it work",
	DocsLink:   DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#unlogged-table-is-not-supported",
}

func NewUnloggedTableIssue(objectType string, objectName string, sqlStatement string) IssueInstance {
	details := map[string]interface{}{}
	//for UNLOGGED TABLE as its not reported in the TABLE objects
	if objectType == "TABLE" {
		details["INCREASE_INVALID_COUNT"] = false
	}
	return newIssueInstance(unloggedTableIssue, objectType, objectName, sqlStatement, details)
}

var unsupportedIndexMethodIssue = Issue{
	Type:     UNSUPPORTED_INDEX_METHOD,
	TypeName: "Schema contains %s index which is not supported.",
	GH:       "https://github.com/YugaByte/yugabyte-db/issues/1337",
	DocsLink: DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#gist-brin-and-spgist-index-types-are-not-supported",
}

func NewUnsupportedIndexMethodIssue(objectType string, objectName string, sqlStatement string, indexAccessMethod string) IssueInstance {
	issue := unsupportedIndexMethodIssue
	issue.TypeName = fmt.Sprintf(unsupportedIndexMethodIssue.TypeName, strings.ToUpper(indexAccessMethod))
	return newIssueInstance(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var storageParameterIssue = Issue{
	Type:       STORAGE_PARAMETER,
	TypeName:   "Storage parameters are not supported yet.",
	GH:         "https://github.com/yugabyte/yugabyte-db/issues/23467",
	Suggestion: "Remove the storage parameters from the DDL",
	DocsLink:   DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#storage-parameters-on-indexes-or-constraints-in-the-source-postgresql",
}

func NewStorageParameterIssue(objectType string, objectName string, sqlStatement string) IssueInstance {
	details := map[string]interface{}{}
	//for ALTER AND INDEX both  same struct now how to differentiate which one to not
	if objectType == "TABLE" {
		details["INCREASE_INVALID_COUNT"] = false
	}
	return newIssueInstance(storageParameterIssue, objectType, objectName, sqlStatement, details)
}

var setAttributeIssue = Issue{
	Type:       SET_ATTRIBUTES,
	TypeName:   "ALTER TABLE .. ALTER COLUMN .. SET ( attribute = value )	 not supported yet",
	GH:         "https://github.com/yugabyte/yugabyte-db/issues/1124",
	Suggestion: "Remove it from the exported schema",
	DocsLink:   DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#unsupported-alter-table-ddl-variants-in-source-schema",
}

func NewSetAttributeIssue(objectType string, objectName string, sqlStatement string) IssueInstance {
	details := map[string]interface{}{}
	//for ALTER AND INDEX both  same struct now how to differentiate which one to not
	if objectType == "TABLE" {
		details["INCREASE_INVALID_COUNT"] = false
	}
	return newIssueInstance(setAttributeIssue, objectType, objectName, sqlStatement, details)
}

var clusterOnIssue = Issue{
	Type:       CLUSTER_ON,
	TypeName:   "ALTER TABLE CLUSTER not supported yet.",
	GH:         "https://github.com/YugaByte/yugabyte-db/issues/1124",
	Suggestion: "Remove it from the exported schema.",
	DocsLink:   DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#unsupported-alter-table-ddl-variants-in-source-schema",
}

func NewClusterONIssue(objectType string, objectName string, sqlStatement string) IssueInstance {
	details := map[string]interface{}{}
	//for ALTER AND INDEX both  same struct now how to differentiate which one to not
	if objectType == "TABLE" {
		details["INCREASE_INVALID_COUNT"] = false
	}
	return newIssueInstance(clusterOnIssue, objectType, objectName, sqlStatement, details)
}

var disableRuleIssue = Issue{
	Type:       DISABLE_RULE,
	TypeName:   "ALTER TABLE name DISABLE RULE not supported yet",
	GH:         "https://github.com/yugabyte/yugabyte-db/issues/1124",
	Suggestion: "Remove this and the rule '%s' from the exported schema to be not enabled on the table.",
	DocsLink:   DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#unsupported-alter-table-ddl-variants-in-source-schema",
}

func NewDisableRuleIssue(objectType string, objectName string, sqlStatement string, ruleName string) IssueInstance {
	details := map[string]interface{}{}
	//for ALTER AND INDEX both  same struct now how to differentiate which one to not
	if objectType == "TABLE" {
		details["INCREASE_INVALID_COUNT"] = false
	}
	issue := disableRuleIssue
	issue.Suggestion = fmt.Sprintf(issue.Suggestion, ruleName)
	return newIssueInstance(issue, objectType, objectName, sqlStatement, details)
}

var exclusionConstraintIssue = Issue{
	Type:       EXCLUSION_CONSTRAINTS,
	TypeName:   "Exclusion constraint is not supported yet",
	GH:         "https://github.com/yugabyte/yugabyte-db/issues/3944",
	Suggestion: "Refer docs link for details on possible workaround",
	DocsLink:   DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#exclusion-constraints-is-not-supported",
}

func NewExclusionConstraintIssue(objectType string, objectName string, sqlStatement string) IssueInstance {
	return newIssueInstance(exclusionConstraintIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var deferrableConstraintIssue = Issue{
	Type:       DEFERRABLE_CONSTRAINTS,
	TypeName:   "DEFERRABLE constraints not supported yet",
	GH:         "https://github.com/yugabyte/yugabyte-db/issues/1709",
	Suggestion: "Remove these constraints from the exported schema and make the neccessary changes to the application to work on target seamlessly",
	DocsLink:   DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#deferrable-constraint-on-constraints-other-than-foreign-keys-is-not-supported",
}

func NewDeferrableConstraintIssue(objectType string, objectName string, sqlStatement string) IssueInstance {
	return newIssueInstance(deferrableConstraintIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var multiColumnGinIndexIssue = Issue{
	Type:     MULTI_COLUMN_GIN_INDEX,
	TypeName: "Schema contains gin index on multi column which is not supported.",
	GH:       "https://github.com/yugabyte/yugabyte-db/issues/10652",
	DocsLink: DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#gin-indexes-on-multiple-columns-are-not-supported",
}

func NewMultiColumnGinIndexIssue(objectType string, objectName string, sqlStatement string) IssueInstance {
	return newIssueInstance(multiColumnGinIndexIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var orderedGinIndexIssue = Issue{
	Type:     ORDERED_GIN_INDEX,
	TypeName: "Schema contains gin index on column with ASC/DESC/HASH Clause which is not supported.",
	GH:       "https://github.com/yugabyte/yugabyte-db/issues/10653",
	DocsLink: DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#issue-in-some-unsupported-cases-of-gin-indexes",
}

func NewOrderedGinIndexIssue(objectType string, objectName string, sqlStatement string) IssueInstance {
	return newIssueInstance(orderedGinIndexIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var policyRoleIssue = Issue{
	Type:       POLICY_WITH_ROLES,
	TypeName:   "Policy require roles to be created.",
	Suggestion: "Users/Grants are not migrated during the schema migration. Create the Users manually to make the policies work",
	GH:         "https://github.com/yugabyte/yb-voyager/issues/1655",
	DocsLink:   DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#policies-on-users-in-source-require-manual-user-creation",
}

func NewPolicyRoleIssue(objectType string, objectName string, SqlStatement string, roles []string) IssueInstance {
	issue := policyRoleIssue
	issue.TypeName = fmt.Sprintf("%s Users - (%s)", issue.TypeName, strings.Join(roles, ","))
	return newIssueInstance(issue, objectType, objectName, SqlStatement, map[string]interface{}{})
}

var constraintTriggerIssue = Issue{
	Type:     CONSTRAINT_TRIGGER,
	TypeName: "CONSTRAINT TRIGGER not supported yet.",
	GH:       "https://github.com/YugaByte/yugabyte-db/issues/1709",
	DocsLink: DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#constraint-trigger-is-not-supported",
}

func NewConstraintTriggerIssue(objectType string, objectName string, SqlStatement string) IssueInstance {
	details := map[string]interface{}{}
	//for CONSTRAINT TRIGGER we don't have separate object type TODO: fix
	if objectType == "TRIGGER" {
		details["INCREASE_INVALID_COUNT"] = false
	}
	return newIssueInstance(constraintTriggerIssue, objectType, objectName, SqlStatement, details)
}

var referencingClauseInTriggerIssue = Issue{
	Type:     REFERENCING_CLAUSE_FOR_TRIGGERS,
	TypeName: "REFERENCING clause (transition tables) not supported yet.",
	GH:       "https://github.com/YugaByte/yugabyte-db/issues/1668",
	DocsLink: DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#referencing-clause-for-triggers",
}

func NewReferencingClauseTrigIssue(objectType string, objectName string, SqlStatement string) IssueInstance {
	return newIssueInstance(referencingClauseInTriggerIssue, objectType, objectName, SqlStatement, map[string]interface{}{})
}

var beforeRowTriggerOnPartitionTableIssue = Issue{
	Type:       BEFORE_ROW_TRIGGER_ON_PARTITIONED_TABLE,
	TypeName:   "Partitioned tables cannot have BEFORE / FOR EACH ROW triggers.",
	DocsLink:   DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#before-row-triggers-on-partitioned-tables",
	GH:         "https://github.com/yugabyte/yugabyte-db/issues/24830",
	Suggestion: "Create the triggers on individual partitions.",
}

func NewBeforeRowOnPartitionTableIssue(objectType string, objectName string, SqlStatement string) IssueInstance {
	return newIssueInstance(beforeRowTriggerOnPartitionTableIssue, objectType, objectName, SqlStatement, map[string]interface{}{})
}

var alterTableAddPKOnPartitionIssue = Issue{
	Type:     ALTER_TABLE_ADD_PK_ON_PARTITIONED_TABLE,
	TypeName: "Adding primary key to a partitioned table is not supported yet.",
	DocsLink: DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#adding-primary-key-to-a-partitioned-table-results-in-an-error",
	GH:       "https://github.com/yugabyte/yugabyte-db/issues/10074",
}

func NewAlterTableAddPKOnPartiionIssue(objectType string, objectName string, SqlStatement string) IssueInstance {
	details := map[string]interface{}{}
	//for ALTER AND INDEX both  same struct now how to differentiate which one to not
	if objectType == "TABLE" {
		details["INCREASE_INVALID_COUNT"] = false
	}
	return newIssueInstance(alterTableAddPKOnPartitionIssue, objectType, objectName, SqlStatement, details)
}

var expressionPartitionIssue = Issue{
	Type:       EXPRESSION_PARTITION_WITH_PK_UK,
	TypeName:   "Issue with Partition using Expression on a table which cannot contain Primary Key / Unique Key on any column",
	Suggestion: "Remove the Constriant from the table definition",
	GH:         "https://github.com/yugabyte/yb-voyager/issues/698",
	DocsLink:   DOCS_LINK_PREFIX + MYSQL_PREFIX + "#tables-partitioned-with-expressions-cannot-contain-primary-unique-keys",
}

func NewExpressionPartitionIssue(objectType string, objectName string, SqlStatement string) IssueInstance {
	return newIssueInstance(expressionPartitionIssue, objectType, objectName, SqlStatement, map[string]interface{}{})
}

var multiColumnListPartition = Issue{
	Type:       MULTI_COLUMN_LIST_PARTITION,
	TypeName:   `cannot use "list" partition strategy with more than one column`,
	Suggestion: "Make it a single column partition by list or choose other supported Partitioning methods",
	GH:         "https://github.com/yugabyte/yb-voyager/issues/699",
	DocsLink:   DOCS_LINK_PREFIX + MYSQL_PREFIX + "#multi-column-partition-by-list-is-not-supported",
}

func NewMultiColumnListPartition(objectType string, objectName string, SqlStatement string) IssueInstance {
	return newIssueInstance(multiColumnListPartition, objectType, objectName, SqlStatement, map[string]interface{}{})
}

var insufficientColumnsInPKForPartition = Issue{
	Type:       INSUFFICIENT_COLUMNS_IN_PK_FOR_PARTITION,
	TypeName:   "insufficient columns in the PRIMARY KEY constraint definition in CREATE TABLE",
	Suggestion: "Add all Partition columns to Primary Key",
	GH:         "https://github.com/yugabyte/yb-voyager/issues/578",
	DocsLink:   DOCS_LINK_PREFIX + ORACLE_PREFIX + "#partition-key-column-not-part-of-primary-key-columns",
}

func NewInsufficientColumnInPKForPartition(objectType string, objectName string, SqlStatement string, partitionColumnsNotInPK []string) IssueInstance {
	issue := insufficientColumnsInPKForPartition
	issue.TypeName = fmt.Sprintf("%s - (%s)", issue.TypeName, strings.Join(partitionColumnsNotInPK, ", "))
	return newIssueInstance(issue, objectType, objectName, SqlStatement, map[string]interface{}{})
}

var xmlDatatypeIssue = Issue{
	Type:       XML_DATATYPE,
	TypeName:   "Unsupported datatype - xml",
	Suggestion: "Data ingestion is not supported for this type in YugabyteDB so handle this type in different way. Refer link for more details.",
	GH:         "https://github.com/yugabyte/yugabyte-db/issues/1043",
	DocsLink:   DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#data-ingestion-on-xml-data-type-is-not-supported",
}

func NewXMLDatatypeIssue(objectType string, objectName string, SqlStatement string, colName string) IssueInstance {
	issue := xmlDatatypeIssue
	issue.TypeName = fmt.Sprintf("%s on column - %s", issue.TypeName, colName)
	return newIssueInstance(issue, objectType, objectName, SqlStatement, map[string]interface{}{})
}

var xidDatatypeIssue = Issue{
	Type:       XID_DATATYPE,
	TypeName:   "Unsupported datatype - xid",
	Suggestion: "Functions for this type e.g. txid_current are not supported in YugabyteDB yet",
	GH:         "https://github.com/yugabyte/yugabyte-db/issues/15638",
	DocsLink:   DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#xid-functions-is-not-supported",
}

func NewXIDDatatypeIssue(objectType string, objectName string, SqlStatement string, colName string) IssueInstance {
	issue := xidDatatypeIssue
	issue.TypeName = fmt.Sprintf("%s on column - %s", issue.TypeName, colName)
	return newIssueInstance(issue, objectType, objectName, SqlStatement, map[string]interface{}{})
}

var postgisDatatypeIssue = Issue{
	Type:     POSTGIS_DATATYPES,
	TypeName: "Unsupported datatype",
	GH:       "https://github.com/yugabyte/yugabyte-db/issues/11323",
	DocsLink: DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#unsupported-datatypes-by-yugabytedb",
}

func NewPostGisDatatypeIssue(objectType string, objectName string, SqlStatement string, typeName string, colName string) IssueInstance {
	issue := postgisDatatypeIssue
	issue.TypeName = fmt.Sprintf("%s - %s on column - %s", issue.TypeName, typeName, colName)
	return newIssueInstance(issue, objectType, objectName, SqlStatement, map[string]interface{}{})
}

var unsupportedDatatypesIssue = Issue{
	Type:     UNSUPPORTED_DATATYPES,
	TypeName: "Unsupported datatype",
	GH:       "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink: DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#unsupported-datatypes-by-yugabytedb",
}

func NewUnsupportedDatatypesIssue(objectType string, objectName string, SqlStatement string, typeName string, colName string) IssueInstance {
	issue := unsupportedDatatypesIssue
	issue.TypeName = fmt.Sprintf("%s - %s on column - %s", issue.TypeName, typeName, colName)
	return newIssueInstance(issue, objectType, objectName, SqlStatement, map[string]interface{}{})
}

var unsupportedDatatypesForLiveMigrationIssue = Issue{
	Type:     UNSUPPORTED_DATATYPES_LIVE_MIGRATION,
	TypeName: "Unsupported datatype for Live migration",
	GH:       "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink: DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewUnsupportedDatatypesForLMIssue(objectType string, objectName string, SqlStatement string, typeName string, colName string) IssueInstance {
	issue := unsupportedDatatypesForLiveMigrationIssue
	issue.TypeName = fmt.Sprintf("%s - %s on column - %s", issue.TypeName, typeName, colName)
	return newIssueInstance(issue, objectType, objectName, SqlStatement, map[string]interface{}{})
}

var unsupportedDatatypesForLiveMigrationWithFFOrFBIssue = Issue{
	Type:     UNSUPPORTED_DATATYPES_LIVE_MIGRATION_WITH_FF_FB,
	TypeName: "Unsupported datatype for Live migration with fall-forward/fallback",
	GH:       "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink: DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewUnsupportedDatatypesForLMWithFFOrFBIssue(objectType string, objectName string, SqlStatement string, typeName string, colName string) IssueInstance {
	issue := unsupportedDatatypesForLiveMigrationWithFFOrFBIssue
	issue.TypeName = fmt.Sprintf("%s - %s on column - %s", issue.TypeName, typeName, colName)
	return newIssueInstance(issue, objectType, objectName, SqlStatement, map[string]interface{}{})
}

var primaryOrUniqueOnUnsupportedIndexTypesIssue = Issue{
	Type:       PK_UK_ON_COMPLEX_DATATYPE,
	TypeName:   "Primary key and Unique constraint on column '%s' not yet supported",
	GH:         "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion: "Refer to the docs link for the workaround",
	DocsLink:   DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#indexes-on-some-complex-data-types-are-not-supported", //Keeping it similar for now, will see if we need to a separate issue on docs,
}

func NewPrimaryOrUniqueConsOnUnsupportedIndexTypesIssue(objectType string, objectName string, SqlStatement string, typeName string, increaseInvalidCnt bool) IssueInstance {
	details := map[string]interface{}{}
	//for ALTER not increasing count, but for Create increasing TODO: fix
	if !increaseInvalidCnt {
		details["INCREASE_INVALID_COUNT"] = false
	}
	issue := primaryOrUniqueOnUnsupportedIndexTypesIssue
	issue.TypeName = fmt.Sprintf(issue.TypeName, typeName)
	return newIssueInstance(issue, objectType, objectName, SqlStatement, details)
}

var indexOnComplexDatatypesIssue = Issue{
	Type:       INDEX_ON_COMPLEX_DATATYPE,
	TypeName:   "INDEX on column '%s' not yet supported",
	GH:         "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion: "Refer to the docs link for the workaround",
	DocsLink:   DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#indexes-on-some-complex-data-types-are-not-supported",
}

func NewIndexOnComplexDatatypesIssue(objectType string, objectName string, SqlStatement string, typeName string) IssueInstance {
	issue := indexOnComplexDatatypesIssue
	issue.TypeName = fmt.Sprintf(issue.TypeName, typeName)
	return newIssueInstance(issue, objectType, objectName, SqlStatement, map[string]interface{}{})
}

var foreignTableIssue = Issue{
	Type:       FOREIGN_TABLE,
	TypeName:   "Foreign tables require manual intervention.",
	GH:         "https://github.com/yugabyte/yb-voyager/issues/1627",
	Suggestion: "SERVER '%s', and USER MAPPING should be created manually on the target to create and use the foreign table",
	DocsLink:   DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#foreign-table-in-the-source-database-requires-server-and-user-mapping",
}

func NewForeignTableIssue(objectType string, objectName string, SqlStatement string, serverName string) IssueInstance {
	issue := foreignTableIssue
	issue.Suggestion = fmt.Sprintf(issue.Suggestion, serverName)
	return newIssueInstance(issue, objectType, objectName, SqlStatement, map[string]interface{}{})
}

var inheritanceIssue = Issue{
	Type:     INHERITANCE,
	TypeName: "TABLE INHERITANCE not supported in YugabyteDB",
	GH:       "https://github.com/YugaByte/yugabyte-db/issues/1129",
	DocsLink: DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#table-inheritance-is-not-supported",
}

func NewInheritanceIssue(objectType string, objectName string, sqlStatement string) IssueInstance {
	return newIssueInstance(inheritanceIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}
var percentTypeSyntax = Issue{
	Type:     REFERENCED_TYPE_DECLARATION,
	TypeName: "Referenced type declaration of variables",
	TypeDescription: "",
	Suggestion: "Fix the syntax to include the actual type name instead of referencing the type of a column",
	GH: "https://github.com/yugabyte/yugabyte-db/issues/23619",
	DocsLink: "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#type-syntax-is-not-supported",
}

func NewPercentTypeSyntaxIssue(objectType string, objectName string, sqlStatement string) IssueInstance {
	return newIssueInstance(percentTypeSyntax, objectType, objectName, sqlStatement, map[string]interface{}{})
}

