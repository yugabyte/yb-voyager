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

import (
	"fmt"
	"strings"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/issue"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/ybversion"
)

var generatedColumnsIssue = issue.Issue{
	Type:       STORED_GENERATED_COLUMNS,
	TypeName:   "Stored generated columns are not supported.",
	GH:         "https://github.com/yugabyte/yugabyte-db/issues/10695",
	Suggestion: "Using Triggers to update the generated columns is one way to work around this issue, refer docs link for more details.",
	DocsLink:   "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#generated-always-as-stored-type-column-is-not-supported",
}

func NewGeneratedColumnsIssue(objectType string, objectName string, sqlStatement string, generatedColumns []string) QueryIssue {
	issue := generatedColumnsIssue
	issue.TypeName = issue.TypeName + fmt.Sprintf(" Generated Columns: (%s)", strings.Join(generatedColumns, ","))
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var unloggedTableIssue = issue.Issue{
	Type:       UNLOGGED_TABLE,
	TypeName:   "UNLOGGED tables are not supported yet.",
	GH:         "https://github.com/yugabyte/yugabyte-db/issues/1129/",
	Suggestion: "Remove UNLOGGED keyword to make it work",
	DocsLink:   "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unlogged-table-is-not-supported",
	MinimumVersionsFixedIn: map[string]*ybversion.YBVersion{
		ybversion.SERIES_2024_2: ybversion.V2024_2_0_0,
	},
}

func NewUnloggedTableIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	details := map[string]interface{}{}
	return newQueryIssue(unloggedTableIssue, objectType, objectName, sqlStatement, details)
}

var unsupportedIndexMethodIssue = issue.Issue{
	Type:     UNSUPPORTED_INDEX_METHOD,
	TypeName: "Schema contains %s index which is not supported.",
	GH:       "https://github.com/YugaByte/yugabyte-db/issues/1337",
	DocsLink: "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#gist-brin-and-spgist-index-types-are-not-supported",
}

func NewUnsupportedIndexMethodIssue(objectType string, objectName string, sqlStatement string, indexAccessMethod string) QueryIssue {
	issue := unsupportedIndexMethodIssue
	issue.TypeName = fmt.Sprintf(unsupportedIndexMethodIssue.TypeName, strings.ToUpper(indexAccessMethod))
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var storageParameterIssue = issue.Issue{
	Type:       STORAGE_PARAMETER,
	TypeName:   "Storage parameters are not supported yet.",
	GH:         "https://github.com/yugabyte/yugabyte-db/issues/23467",
	Suggestion: "Remove the storage parameters from the DDL",
	DocsLink:   "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#storage-parameters-on-indexes-or-constraints-in-the-source-postgresql",
}

func NewStorageParameterIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	details := map[string]interface{}{}
	return newQueryIssue(storageParameterIssue, objectType, objectName, sqlStatement, details)
}

var setAttributeIssue = issue.Issue{
	Type:       SET_ATTRIBUTES,
	TypeName:   "ALTER TABLE .. ALTER COLUMN .. SET ( attribute = value )	 not supported yet",
	GH:         "https://github.com/yugabyte/yugabyte-db/issues/1124",
	Suggestion: "Remove it from the exported schema",
	DocsLink:   "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-alter-table-ddl-variants-in-source-schema",
}

func NewSetAttributeIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	details := map[string]interface{}{}
	return newQueryIssue(setAttributeIssue, objectType, objectName, sqlStatement, details)
}

var clusterOnIssue = issue.Issue{
	Type:       CLUSTER_ON,
	TypeName:   "ALTER TABLE CLUSTER not supported yet.",
	GH:         "https://github.com/YugaByte/yugabyte-db/issues/1124",
	Suggestion: "Remove it from the exported schema.",
	DocsLink:   "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-alter-table-ddl-variants-in-source-schema",
}

func NewClusterONIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	details := map[string]interface{}{}
	return newQueryIssue(clusterOnIssue, objectType, objectName, sqlStatement, details)
}

var disableRuleIssue = issue.Issue{
	Type:       DISABLE_RULE,
	TypeName:   "ALTER TABLE name DISABLE RULE not supported yet",
	GH:         "https://github.com/yugabyte/yugabyte-db/issues/1124",
	Suggestion: "Remove this and the rule '%s' from the exported schema to be not enabled on the table.",
	DocsLink:   "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-alter-table-ddl-variants-in-source-schema",
}

func NewDisableRuleIssue(objectType string, objectName string, sqlStatement string, ruleName string) QueryIssue {
	details := map[string]interface{}{}
	issue := disableRuleIssue
	issue.Suggestion = fmt.Sprintf(issue.Suggestion, ruleName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var exclusionConstraintIssue = issue.Issue{
	Type:       EXCLUSION_CONSTRAINTS,
	TypeName:   "Exclusion constraint is not supported yet",
	GH:         "https://github.com/yugabyte/yugabyte-db/issues/3944",
	Suggestion: "Refer docs link for details on possible workaround",
	DocsLink:   "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#exclusion-constraints-is-not-supported",
}

func NewExclusionConstraintIssue(objectType string, objectName string, sqlStatement string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	return newQueryIssue(exclusionConstraintIssue, objectType, objectName, sqlStatement, details)
}

var deferrableConstraintIssue = issue.Issue{
	Type:       DEFERRABLE_CONSTRAINTS,
	TypeName:   "DEFERRABLE constraints not supported yet",
	GH:         "https://github.com/yugabyte/yugabyte-db/issues/1709",
	Suggestion: "Remove these constraints from the exported schema and make the neccessary changes to the application to work on target seamlessly",
	DocsLink:   "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#deferrable-constraint-on-constraints-other-than-foreign-keys-is-not-supported",
}

func NewDeferrableConstraintIssue(objectType string, objectName string, sqlStatement string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	return newQueryIssue(deferrableConstraintIssue, objectType, objectName, sqlStatement, details)
}

var multiColumnGinIndexIssue = issue.Issue{
	Type:     MULTI_COLUMN_GIN_INDEX,
	TypeName: "Schema contains gin index on multi column which is not supported.",
	GH:       "https://github.com/yugabyte/yugabyte-db/issues/10652",
	DocsLink: "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#gin-indexes-on-multiple-columns-are-not-supported",
}

func NewMultiColumnGinIndexIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(multiColumnGinIndexIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var orderedGinIndexIssue = issue.Issue{
	Type:     ORDERED_GIN_INDEX,
	TypeName: "Schema contains gin index on column with ASC/DESC/HASH Clause which is not supported.",
	GH:       "https://github.com/yugabyte/yugabyte-db/issues/10653",
	DocsLink: "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#issue-in-some-unsupported-cases-of-gin-indexes",
}

func NewOrderedGinIndexIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(orderedGinIndexIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var policyRoleIssue = issue.Issue{
	Type:       POLICY_WITH_ROLES,
	TypeName:   "Policy require roles to be created.",
	Suggestion: "Users/Grants are not migrated during the schema migration. Create the Users manually to make the policies work",
	GH:         "https://github.com/yugabyte/yb-voyager/issues/1655",
	DocsLink:   "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#policies-on-users-in-source-require-manual-user-creation",
}

func NewPolicyRoleIssue(objectType string, objectName string, sqlStatement string, roles []string) QueryIssue {
	issue := policyRoleIssue
	issue.TypeName = fmt.Sprintf("%s Users - (%s)", issue.TypeName, strings.Join(roles, ","))
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var constraintTriggerIssue = issue.Issue{
	Type:     CONSTRAINT_TRIGGER,
	TypeName: "CONSTRAINT TRIGGER not supported yet.",
	GH:       "https://github.com/YugaByte/yugabyte-db/issues/1709",
	DocsLink: "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#constraint-trigger-is-not-supported",
}

func NewConstraintTriggerIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	details := map[string]interface{}{}
	return newQueryIssue(constraintTriggerIssue, objectType, objectName, sqlStatement, details)
}

var referencingClauseInTriggerIssue = issue.Issue{
	Type:     REFERENCING_CLAUSE_FOR_TRIGGERS,
	TypeName: "REFERENCING clause (transition tables) not supported yet.",
	GH:       "https://github.com/YugaByte/yugabyte-db/issues/1668",
	DocsLink: "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#referencing-clause-for-triggers",
}

func NewReferencingClauseTrigIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(referencingClauseInTriggerIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var beforeRowTriggerOnPartitionTableIssue = issue.Issue{
	Type:       BEFORE_ROW_TRIGGER_ON_PARTITIONED_TABLE,
	TypeName:   "Partitioned tables cannot have BEFORE / FOR EACH ROW triggers.",
	DocsLink:   "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#before-row-triggers-on-partitioned-tables",
	GH:         "https://github.com/yugabyte/yugabyte-db/issues/24830",
	Suggestion: "Create the triggers on individual partitions.",
}

func NewBeforeRowOnPartitionTableIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(beforeRowTriggerOnPartitionTableIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var alterTableAddPKOnPartitionIssue = issue.Issue{
	Type:     ALTER_TABLE_ADD_PK_ON_PARTITIONED_TABLE,
	TypeName: "Adding primary key to a partitioned table is not supported yet.",
	DocsLink: "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#adding-primary-key-to-a-partitioned-table-results-in-an-error",
	GH:       "https://github.com/yugabyte/yugabyte-db/issues/10074",
	MinimumVersionsFixedIn: map[string]*ybversion.YBVersion{
		ybversion.SERIES_2024_1: ybversion.V2024_1_0_0,
		ybversion.SERIES_2024_2: ybversion.V2024_2_0_0,
		ybversion.SERIES_2_23:   ybversion.V2_23_0_0,
	},
}

func NewAlterTableAddPKOnPartiionIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	details := map[string]interface{}{}
	return newQueryIssue(alterTableAddPKOnPartitionIssue, objectType, objectName, sqlStatement, details)
}

var expressionPartitionIssue = issue.Issue{
	Type:       EXPRESSION_PARTITION_WITH_PK_UK,
	TypeName:   "Issue with Partition using Expression on a table which cannot contain Primary Key / Unique Key on any column",
	Suggestion: "Remove the Constriant from the table definition",
	GH:         "https://github.com/yugabyte/yb-voyager/issues/698",
	DocsLink:   "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/mysql/#tables-partitioned-with-expressions-cannot-contain-primary-unique-keys",
}

func NewExpressionPartitionIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(expressionPartitionIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var multiColumnListPartition = issue.Issue{
	Type:       MULTI_COLUMN_LIST_PARTITION,
	TypeName:   `cannot use "list" partition strategy with more than one column`,
	Suggestion: "Make it a single column partition by list or choose other supported Partitioning methods",
	GH:         "https://github.com/yugabyte/yb-voyager/issues/699",
	DocsLink:   "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/mysql/#multi-column-partition-by-list-is-not-supported",
}

func NewMultiColumnListPartition(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(multiColumnListPartition, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var insufficientColumnsInPKForPartition = issue.Issue{
	Type:       INSUFFICIENT_COLUMNS_IN_PK_FOR_PARTITION,
	TypeName:   "insufficient columns in the PRIMARY KEY constraint definition in CREATE TABLE",
	Suggestion: "Add all Partition columns to Primary Key",
	GH:         "https://github.com/yugabyte/yb-voyager/issues/578",
	DocsLink:   "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/oracle/#partition-key-column-not-part-of-primary-key-columns",
}

func NewInsufficientColumnInPKForPartition(objectType string, objectName string, sqlStatement string, partitionColumnsNotInPK []string) QueryIssue {
	issue := insufficientColumnsInPKForPartition
	issue.TypeName = fmt.Sprintf("%s - (%s)", issue.TypeName, strings.Join(partitionColumnsNotInPK, ", "))
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var xmlDatatypeIssue = issue.Issue{
	Type:       XML_DATATYPE,
	TypeName:   "Unsupported datatype - xml",
	Suggestion: "Data ingestion is not supported for this type in YugabyteDB so handle this type in different way. Refer link for more details.",
	GH:         "https://github.com/yugabyte/yugabyte-db/issues/1043",
	DocsLink:   "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#data-ingestion-on-xml-data-type-is-not-supported",
}

func NewXMLDatatypeIssue(objectType string, objectName string, sqlStatement string, colName string) QueryIssue {
	issue := xmlDatatypeIssue
	issue.TypeName = fmt.Sprintf("%s on column - %s", issue.TypeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var xidDatatypeIssue = issue.Issue{
	Type:       XID_DATATYPE,
	TypeName:   "Unsupported datatype - xid",
	Suggestion: "Functions for this type e.g. txid_current are not supported in YugabyteDB yet",
	GH:         "https://github.com/yugabyte/yugabyte-db/issues/15638",
	DocsLink:   "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#xid-functions-is-not-supported",
}

func NewXIDDatatypeIssue(objectType string, objectName string, sqlStatement string, colName string) QueryIssue {
	issue := xidDatatypeIssue
	issue.TypeName = fmt.Sprintf("%s on column - %s", issue.TypeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var postgisDatatypeIssue = issue.Issue{
	Type:     POSTGIS_DATATYPES,
	TypeName: "Unsupported datatype",
	GH:       "https://github.com/yugabyte/yugabyte-db/issues/11323",
	DocsLink: "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-yugabytedb",
}

func NewPostGisDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := postgisDatatypeIssue
	issue.TypeName = fmt.Sprintf("%s - %s on column - %s", issue.TypeName, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var unsupportedDatatypesIssue = issue.Issue{
	Type:     UNSUPPORTED_DATATYPES,
	TypeName: "Unsupported datatype",
	GH:       "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink: "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-yugabytedb",
}

func NewUnsupportedDatatypesIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := unsupportedDatatypesIssue
	issue.TypeName = fmt.Sprintf("%s - %s on column - %s", issue.TypeName, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var unsupportedDatatypesForLiveMigrationIssue = issue.Issue{
	Type:     UNSUPPORTED_DATATYPES_LIVE_MIGRATION,
	TypeName: "Unsupported datatype for Live migration",
	GH:       "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink: "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewUnsupportedDatatypesForLMIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := unsupportedDatatypesForLiveMigrationIssue
	issue.TypeName = fmt.Sprintf("%s - %s on column - %s", issue.TypeName, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var unsupportedDatatypesForLiveMigrationWithFFOrFBIssue = issue.Issue{
	Type:     UNSUPPORTED_DATATYPES_LIVE_MIGRATION_WITH_FF_FB,
	TypeName: "Unsupported datatype for Live migration with fall-forward/fallback",
	GH:       "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink: "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewUnsupportedDatatypesForLMWithFFOrFBIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := unsupportedDatatypesForLiveMigrationWithFFOrFBIssue
	issue.TypeName = fmt.Sprintf("%s - %s on column - %s", issue.TypeName, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var primaryOrUniqueOnUnsupportedIndexTypesIssue = issue.Issue{
	Type:       PK_UK_ON_COMPLEX_DATATYPE,
	TypeName:   "Primary key and Unique constraint on column '%s' not yet supported",
	GH:         "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion: "Refer to the docs link for the workaround",
	DocsLink:   "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported", //Keeping it similar for now, will see if we need to a separate issue on docs,
}

func NewPrimaryOrUniqueConsOnUnsupportedIndexTypesIssue(objectType string, objectName string, sqlStatement string, typeName string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	issue := primaryOrUniqueOnUnsupportedIndexTypesIssue
	issue.TypeName = fmt.Sprintf(issue.TypeName, typeName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var indexOnComplexDatatypesIssue = issue.Issue{
	Type:       INDEX_ON_COMPLEX_DATATYPE,
	TypeName:   "INDEX on column '%s' not yet supported",
	GH:         "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion: "Refer to the docs link for the workaround",
	DocsLink:   "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported",
}

func NewIndexOnComplexDatatypesIssue(objectType string, objectName string, sqlStatement string, typeName string) QueryIssue {
	issue := indexOnComplexDatatypesIssue
	issue.TypeName = fmt.Sprintf(issue.TypeName, typeName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var foreignTableIssue = issue.Issue{
	Type:       FOREIGN_TABLE,
	TypeName:   "Foreign tables require manual intervention.",
	GH:         "https://github.com/yugabyte/yb-voyager/issues/1627",
	Suggestion: "SERVER '%s', and USER MAPPING should be created manually on the target to create and use the foreign table",
	DocsLink:   "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#foreign-table-in-the-source-database-requires-server-and-user-mapping",
}

func NewForeignTableIssue(objectType string, objectName string, sqlStatement string, serverName string) QueryIssue {
	issue := foreignTableIssue
	issue.Suggestion = fmt.Sprintf(issue.Suggestion, serverName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var inheritanceIssue = issue.Issue{
	Type:     INHERITANCE,
	TypeName: "TABLE INHERITANCE not supported in YugabyteDB",
	GH:       "https://github.com/YugaByte/yugabyte-db/issues/1129",
	DocsLink: "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#table-inheritance-is-not-supported",
}

func NewInheritanceIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(inheritanceIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var percentTypeSyntax = issue.Issue{
	Type:            REFERENCED_TYPE_DECLARATION,
	TypeName:        "Referenced type declaration of variables",
	TypeDescription: "",
	Suggestion:      "Fix the syntax to include the actual type name instead of referencing the type of a column",
	GH:              "https://github.com/yugabyte/yugabyte-db/issues/23619",
	DocsLink:        "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#type-syntax-is-not-supported",
}

func NewPercentTypeSyntaxIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(percentTypeSyntax, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var loDatatypeIssue = issue.Issue{
	Type:       LARGE_OBJECT_DATATYPE,
	TypeName:   "Unsupported datatype - lo",
	Suggestion: "Large objects are not yet supported in YugabyteDB, no workaround available currently",
	GH:         "https://github.com/yugabyte/yugabyte-db/issues/25318",
	DocsLink:   "", // TODO
}

func NewLODatatypeIssue(objectType string, objectName string, SqlStatement string, colName string) QueryIssue {
	issue := loDatatypeIssue
	issue.TypeName = fmt.Sprintf("%s on column - %s", issue.TypeName, colName)
	return newQueryIssue(issue, objectType, objectName, SqlStatement, map[string]interface{}{})
}

var securityInvokerViewIssue = issue.Issue{
	Type:       SECURITY_INVOKER_VIEWS,
	TypeName:   "Security Invoker Views not supported yet",
	Suggestion: "Security Invoker Views are not yet supported in YugabyteDB, no workaround available currently",
	GH:         "", // TODO
	DocsLink:   "", // TODO
}

func NewSecurityInvokerViewIssue(objectType string, objectName string, SqlStatement string) QueryIssue {
	return newQueryIssue(securityInvokerViewIssue, objectType, objectName, SqlStatement, map[string]interface{}{})
}
