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
	"sort"
	"strings"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/issue"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/ybversion"
)

var generatedColumnsIssue = issue.Issue{
	Type:        STORED_GENERATED_COLUMNS,
	Name:        "Stored Generated Columns",
	Impact:      constants.IMPACT_LEVEL_1,
	Description: STORED_GENERATED_COLUMNS_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/10695",
	Suggestion:  STORED_GENERATED_COLUMN_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#generated-always-as-stored-type-column-is-not-supported",
	MinimumVersionsFixedIn: map[string]*ybversion.YBVersion{
		ybversion.SERIES_2_25: ybversion.V2_25_0_0,
	},
}

func NewGeneratedColumnsIssue(objectType string, objectName string, sqlStatement string, generatedColumns []string) QueryIssue {
	issue := generatedColumnsIssue
	issue.Description = fmt.Sprintf(issue.Description, strings.Join(generatedColumns, ", "))
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var unloggedTableIssue = issue.Issue{
	Type:        UNLOGGED_TABLES,
	Name:        "UNLOGGED tables",
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNLOGGED_TABLES_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/1129/",
	Suggestion:  UNLOGGED_TABLES_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unlogged-table-is-not-supported",
	MinimumVersionsFixedIn: map[string]*ybversion.YBVersion{
		ybversion.SERIES_2024_2: ybversion.V2024_2_0_0,
		ybversion.SERIES_2_25:   ybversion.V2_25_0_0,
	},
}

func NewUnloggedTableIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	details := map[string]interface{}{}
	return newQueryIssue(unloggedTableIssue, objectType, objectName, sqlStatement, details)
}

var unsupportedIndexMethodIssue = issue.Issue{
	Type:        UNSUPPORTED_INDEX_METHOD,
	Name:        UNSUPPORTED_INDEX_METHOD_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_INDEX_METHOD_DESCRIPTION,
	GH:          "https://github.com/YugaByte/yugabyte-db/issues/1337",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#gist-brin-and-spgist-index-types-are-not-supported",
}

func NewUnsupportedIndexMethodIssue(objectType string, objectName string, sqlStatement string, indexAccessMethod string) QueryIssue {
	issue := unsupportedIndexMethodIssue
	issue.Description = fmt.Sprintf(issue.Description, strings.ToUpper(indexAccessMethod))
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var storageParameterIssue = issue.Issue{
	Type:        STORAGE_PARAMETERS,
	Name:        "Storage Parameters",
	Impact:      constants.IMPACT_LEVEL_1,
	Description: STORAGE_PARAMETERS_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/23467",
	Suggestion:  STORAGE_PARAMETERS_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#storage-parameters-on-indexes-or-constraints-in-the-source-postgresql",
}

func NewStorageParameterIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	details := map[string]interface{}{}
	return newQueryIssue(storageParameterIssue, objectType, objectName, sqlStatement, details)
}

var setColumnAttributeIssue = issue.Issue{
	Type:        ALTER_TABLE_SET_COLUMN_ATTRIBUTE,
	Name:        "ALTER TABLE .. ALTER COLUMN .. SET ( attribute = value )",
	Impact:      constants.IMPACT_LEVEL_1,
	Description: ALTER_TABLE_SET_COLUMN_ATTRIBUTE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/1124",
	Suggestion:  ALTER_TABLE_SET_COLUMN_ATTRIBUTE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-alter-table-ddl-variants-in-source-schema",
}

func NewSetColumnAttributeIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	details := map[string]interface{}{}
	return newQueryIssue(setColumnAttributeIssue, objectType, objectName, sqlStatement, details)
}

var alterTableClusterOnIssue = issue.Issue{
	Type:        ALTER_TABLE_CLUSTER_ON,
	Name:        "ALTER TABLE CLUSTER ON",
	Impact:      constants.IMPACT_LEVEL_1,
	Description: ALTER_TABLE_CLUSTER_ON_ISSUE_DESCRIPTION,
	GH:          "https://github.com/YugaByte/yugabyte-db/issues/1124",
	Suggestion:  ALTER_TABLE_CLUSTER_ON_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-alter-table-ddl-variants-in-source-schema",
}

func NewClusterONIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	details := map[string]interface{}{}
	return newQueryIssue(alterTableClusterOnIssue, objectType, objectName, sqlStatement, details)
}

var alterTableDisableRuleIssue = issue.Issue{
	Type:        ALTER_TABLE_DISABLE_RULE,
	Name:        "ALTER TABLE .. DISABLE RULE",
	Impact:      constants.IMPACT_LEVEL_1,
	Description: ALTER_TABLE_DISABLE_RULE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/1124",
	Suggestion:  "Remove this and the rule '%s' from the exported schema to be not enabled on the table.",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-alter-table-ddl-variants-in-source-schema",
}

func NewAlterTableDisableRuleIssue(objectType string, objectName string, sqlStatement string, ruleName string) QueryIssue {
	details := map[string]interface{}{}
	issue := alterTableDisableRuleIssue
	issue.Suggestion = fmt.Sprintf(issue.Suggestion, ruleName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var exclusionConstraintIssue = issue.Issue{
	Type:        EXCLUSION_CONSTRAINTS,
	Name:        "Exclusion Constraints",
	Impact:      constants.IMPACT_LEVEL_1,
	Description: EXCLUSION_CONSTRAINT_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/3944",
	Suggestion:  EXCLUSION_CONSTRAINT_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#exclusion-constraints-is-not-supported",
}

func NewExclusionConstraintIssue(objectType string, objectName string, sqlStatement string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	return newQueryIssue(exclusionConstraintIssue, objectType, objectName, sqlStatement, details)
}

var deferrableConstraintIssue = issue.Issue{
	Type:        DEFERRABLE_CONSTRAINTS,
	Name:        "Deferrable Constraints",
	Impact:      constants.IMPACT_LEVEL_3,
	Description: DEFERRABLE_CONSTRAINT_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/1709",
	Suggestion:  DEFERRABLE_CONSTRAINT_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#deferrable-constraint-on-constraints-other-than-foreign-keys-is-not-supported",
}

func NewDeferrableConstraintIssue(objectType string, objectName string, sqlStatement string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	return newQueryIssue(deferrableConstraintIssue, objectType, objectName, sqlStatement, details)
}

var multiColumnGinIndexIssue = issue.Issue{
	Type:        MULTI_COLUMN_GIN_INDEX,
	Name:        "Multi Column GIN Index",
	Impact:      constants.IMPACT_LEVEL_1,
	Description: MULTI_COLUMN_GIN_INDEX_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/10652",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#gin-indexes-on-multiple-columns-are-not-supported",
}

func NewMultiColumnGinIndexIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(multiColumnGinIndexIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var orderedGinIndexIssue = issue.Issue{
	Type:        ORDERED_GIN_INDEX,
	Name:        "Ordered GIN Index",
	Impact:      constants.IMPACT_LEVEL_1,
	Description: ORDERED_GIN_INDEX_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/10653",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#issue-in-some-unsupported-cases-of-gin-indexes", // TODO: link is not working
}

func NewOrderedGinIndexIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(orderedGinIndexIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var policyRoleIssue = issue.Issue{
	Type:        POLICY_WITH_ROLES,
	Name:        "Policy with Roles",
	Impact:      constants.IMPACT_LEVEL_1,
	Description: POLICY_ROLE_ISSUE_DESCRIPTION,
	Suggestion:  POLICY_ROLE_ISSUE_SUGGESTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1655",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#policies-on-users-in-source-require-manual-user-creation",
}

func NewPolicyRoleIssue(objectType string, objectName string, sqlStatement string, roles []string) QueryIssue {
	issue := policyRoleIssue
	issue.Description = fmt.Sprintf(issue.Description, strings.Join(roles, ", "))
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var constraintTriggerIssue = issue.Issue{
	Type:        CONSTRAINT_TRIGGER,
	Name:        "Constraint Trigger",
	Impact:      constants.IMPACT_LEVEL_1,
	Description: "CONSTRAINT TRIGGER is not yet supported in YugabyteDB.",
	GH:          "https://github.com/YugaByte/yugabyte-db/issues/1709",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#constraint-trigger-is-not-supported",
}

func NewConstraintTriggerIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	details := map[string]interface{}{}
	return newQueryIssue(constraintTriggerIssue, objectType, objectName, sqlStatement, details)
}

var referencingClauseInTriggerIssue = issue.Issue{
	Type:        REFERENCING_CLAUSE_IN_TRIGGER,
	Name:        "Referencing Clause in Triggers",
	Impact:      constants.IMPACT_LEVEL_1,
	Description: REFERENCING_CLAUSE_IN_TRIGGER_ISSUE_DESCRIPTION,
	GH:          "https://github.com/YugaByte/yugabyte-db/issues/1668",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#referencing-clause-for-triggers",
}

func NewReferencingClauseTrigIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(referencingClauseInTriggerIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var beforeRowTriggerOnPartitionTableIssue = issue.Issue{
	Type:        BEFORE_ROW_TRIGGER_ON_PARTITIONED_TABLE,
	Name:        "BEFORE ROW triggers on partitioned tables",
	Impact:      constants.IMPACT_LEVEL_1,
	Description: BEFORE_ROW_TRIGGER_ON_PARTITION_TABLE_ISSUE_DESCRIPTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#before-row-triggers-on-partitioned-tables",
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/24830",
	Suggestion:  BEFORE_ROW_TRIGGER_ON_PARTITION_TABLE_ISSUE_SUGGESTION,
	MinimumVersionsFixedIn: map[string]*ybversion.YBVersion{
		ybversion.SERIES_2_25: ybversion.V2_25_0_0,
	},
}

func NewBeforeRowOnPartitionTableIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(beforeRowTriggerOnPartitionTableIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var alterTableAddPKOnPartitionIssue = issue.Issue{
	Type:        ALTER_TABLE_ADD_PK_ON_PARTITIONED_TABLE,
	Name:        "Adding Primary Key to a partitioned table",
	Impact:      constants.IMPACT_LEVEL_1,
	Description: ALTER_TABLE_ADD_PK_ON_PARTITION_ISSUE_DESCRIPTION,
	Suggestion:  ALTER_TABLE_ADD_PK_ON_PARTITION_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#adding-primary-key-to-a-partitioned-table-results-in-an-error",
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/10074",
	MinimumVersionsFixedIn: map[string]*ybversion.YBVersion{
		ybversion.SERIES_2024_1: ybversion.V2024_1_0_0,
		ybversion.SERIES_2024_2: ybversion.V2024_2_0_0,
		ybversion.SERIES_2_23:   ybversion.V2_23_0_0,
		ybversion.SERIES_2_25:   ybversion.V2_25_0_0,
	},
}

func NewAlterTableAddPKOnPartiionIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	details := map[string]interface{}{}
	return newQueryIssue(alterTableAddPKOnPartitionIssue, objectType, objectName, sqlStatement, details)
}

var expressionPartitionIssue = issue.Issue{
	Type:        EXPRESSION_PARTITION_WITH_PK_UK,
	Name:        "Tables partitioned using expressions containing primary or unique keys",
	Impact:      constants.IMPACT_LEVEL_1,
	Description: EXPRESSION_PARTITION_ISSUE_DESCRIPTION,
	Suggestion:  EXPRESSION_PARTITION_ISSUE_SUGGESTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/698",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/mysql/#tables-partitioned-with-expressions-cannot-contain-primary-unique-keys",
}

func NewExpressionPartitionIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(expressionPartitionIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var multiColumnListPartition = issue.Issue{
	Type:        MULTI_COLUMN_LIST_PARTITION,
	Name:        "Multi-column partition by list",
	Impact:      constants.IMPACT_LEVEL_1,
	Description: MULTI_COLUMN_LIST_PARTITION_ISSUE_DESCRIPTION,
	Suggestion:  MULTI_COLUMN_LIST_PARTITION_ISSUE_SUGGESTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/699",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/mysql/#multi-column-partition-by-list-is-not-supported",
}

func NewMultiColumnListPartition(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(multiColumnListPartition, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var insufficientColumnsInPKForPartition = issue.Issue{
	Type:        INSUFFICIENT_COLUMNS_IN_PK_FOR_PARTITION,
	Name:        "Partition key columns not part of Primary Key",
	Impact:      constants.IMPACT_LEVEL_1,
	Description: INSUFFICIENT_COLUMNS_IN_PK_FOR_PARTITION_ISSUE_DESCRIPTION,
	Suggestion:  INSUFFICIENT_COLUMNS_IN_PK_FOR_PARTITION_ISSUE_SUGGESTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/578",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/oracle/#partition-key-column-not-part-of-primary-key-columns",
}

func NewInsufficientColumnInPKForPartition(objectType string, objectName string, sqlStatement string, partitionColumnsNotInPK []string) QueryIssue {
	issue := insufficientColumnsInPKForPartition
	issue.Description = fmt.Sprintf(issue.Description, strings.Join(partitionColumnsNotInPK, ", "))
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var xmlDatatypeIssue = issue.Issue{
	Type:        XML_DATATYPE,
	Name:        "Unsupported datatype - xml",
	Impact:      constants.IMPACT_LEVEL_3,
	Description: XML_DATATYPE_ISSUE_DESCRIPTION,
	Suggestion:  XML_DATATYPE_ISSUE_SUGGESTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/1043",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#data-ingestion-on-xml-data-type-is-not-supported",
}

func NewXMLDatatypeIssue(objectType string, objectName string, sqlStatement string, colName string) QueryIssue {
	issue := xmlDatatypeIssue
	issue.Description = fmt.Sprintf(issue.Description, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var xidDatatypeIssue = issue.Issue{
	Type:        XID_DATATYPE,
	Name:        "Unsupported datatype - xid",
	Impact:      constants.IMPACT_LEVEL_3,
	Description: XID_DATATYPE_ISSUE_DESCRIPTION,
	Suggestion:  XID_DATATYPE_ISSUE_SUGGESTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/15638",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#xid-functions-is-not-supported",
}

func NewXIDDatatypeIssue(objectType string, objectName string, sqlStatement string, colName string) QueryIssue {
	issue := xidDatatypeIssue
	issue.Description = fmt.Sprintf(issue.Description, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var postgisDatatypeIssue = issue.Issue{
	Type:        POSTGIS_DATATYPE,
	Name:        "Unsupported datatype - POSTGIS",
	Impact:      constants.IMPACT_LEVEL_3,
	Description: POSTGIS_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/11323",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-yugabytedb",
}

func NewPostGisDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := postgisDatatypeIssue
	issue.Description = fmt.Sprintf(issue.Description, colName, typeName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var unsupportedDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE,
	Name:        "Unsupported datatype",
	Impact:      constants.IMPACT_LEVEL_3,
	Description: UNSUPPORTED_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-yugabytedb",
}

func NewUnsupportedDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := unsupportedDatatypeIssue
	issue.Description = fmt.Sprintf(issue.Description, colName, typeName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var unsupportedDatatypeForLiveMigrationIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewUnsupportedDatatypeForLMIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := unsupportedDatatypeForLiveMigrationIssue
	issue.Description = fmt.Sprintf(issue.Description, colName, typeName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var unsupportedDatatypeForLiveMigrationWithFFOrFBIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_WITH_FF_FB,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_WITH_FF_FB_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_WITH_FF_FB_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewUnsupportedDatatypesForLMWithFFOrFBIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := unsupportedDatatypeForLiveMigrationWithFFOrFBIssue
	issue.Description = fmt.Sprintf(issue.Description, colName, typeName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var primaryOrUniqueOnUnsupportedIndexTypesIssue = issue.Issue{
	Type:        PK_UK_ON_COMPLEX_DATATYPE,
	Name:        PK_UK_ON_COMPLEX_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: PK_UK_ON_COMPLEX_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  PK_UK_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported", //Keeping it similar for now, will see if we need to a separate issue on docs,
}

func NewPrimaryOrUniqueConsOnUnsupportedIndexTypesIssue(objectType string, objectName string, sqlStatement string, typeName string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	issue := primaryOrUniqueOnUnsupportedIndexTypesIssue
	issue.Description = fmt.Sprintf(issue.Description, typeName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var indexOnComplexDatatypesIssue = issue.Issue{
	Type:        INDEX_ON_COMPLEX_DATATYPE,
	Name:        INDEX_ON_COMPLEX_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: INDEX_ON_COMPLEX_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  INDEX_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported",
}

func NewIndexOnComplexDatatypesIssue(objectType string, objectName string, sqlStatement string, typeName string) QueryIssue {
	issue := indexOnComplexDatatypesIssue
	issue.Description = fmt.Sprintf(issue.Description, typeName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var foreignTableIssue = issue.Issue{
	Type:        FOREIGN_TABLE,
	Name:        "Foreign Table",
	Impact:      constants.IMPACT_LEVEL_1,
	Description: FOREIGN_TABLE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1627",
	Suggestion:  FOREIGN_TABLE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#foreign-table-in-the-source-database-requires-server-and-user-mapping",
}

func NewForeignTableIssue(objectType string, objectName string, sqlStatement string, serverName string) QueryIssue {
	issue := foreignTableIssue
	issue.Suggestion = fmt.Sprintf(issue.Suggestion, serverName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var inheritanceIssue = issue.Issue{
	Type:        INHERITANCE,
	Name:        "Table Inheritance",
	Impact:      constants.IMPACT_LEVEL_3,
	Description: INHERITANCE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/YugaByte/yugabyte-db/issues/1129",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#table-inheritance-is-not-supported",
}

func NewInheritanceIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(inheritanceIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var percentTypeSyntax = issue.Issue{
	Type:        REFERENCED_TYPE_DECLARATION,
	Name:        "Referencing type declaration of variables",
	Impact:      constants.IMPACT_LEVEL_1,
	Description: REFERENCED_TYPE_DECLARATION_ISSUE_DESCRIPTION,
	Suggestion:  REFERENCED_TYPE_DECLARATION_ISSUE_SUGGESTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/23619",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#type-syntax-is-not-supported",
}

func NewPercentTypeSyntaxIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(percentTypeSyntax, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var loDatatypeIssue = issue.Issue{
	Type:        LARGE_OBJECT_DATATYPE,
	Name:        "Unsupported datatype - lo",
	Impact:      constants.IMPACT_LEVEL_1,
	Description: LARGE_OBJECT_DATATYPE_ISSUE_DESCRIPTION,
	Suggestion:  LARGE_OBJECT_DATATYPE_ISSUE_SUGGESTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25318",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#large-objects-and-its-functions-are-currently-not-supported", // TODO
}

func NewLODatatypeIssue(objectType string, objectName string, SqlStatement string, colName string) QueryIssue {
	issue := loDatatypeIssue
	issue.Description = fmt.Sprintf(issue.Description, colName)
	return newQueryIssue(issue, objectType, objectName, SqlStatement, map[string]interface{}{})
}

var multiRangeDatatypeIssue = issue.Issue{
	Type:        MULTI_RANGE_DATATYPE,
	Name:        "Unsupported datatype - Multirange",
	Impact:      constants.IMPACT_LEVEL_1,
	Description: MULTI_RANGE_DATATYPE_ISSUE_DESCRIPTION,
	Suggestion:  MULTI_RANGE_DATATYPE_ISSUE_SUGGESTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#postgresql-12-and-later-features",
	MinimumVersionsFixedIn: map[string]*ybversion.YBVersion{
		ybversion.SERIES_2_25: ybversion.V2_25_0_0,
	},
}

func NewMultiRangeDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := multiRangeDatatypeIssue
	issue.Description = fmt.Sprintf(issue.Description, colName, typeName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var securityInvokerViewIssue = issue.Issue{
	Type:        SECURITY_INVOKER_VIEWS,
	Name:        "Security Invoker Views",
	Impact:      constants.IMPACT_LEVEL_1,
	Description: SECURITY_INVOKER_VIEWS_ISSUE_DESCRIPTION,
	Suggestion:  SECURITY_INVOKER_VIEWS_ISSUE_SUGGESTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#postgresql-12-and-later-features",
	MinimumVersionsFixedIn: map[string]*ybversion.YBVersion{
		ybversion.SERIES_2_25: ybversion.V2_25_0_0,
	},
}

func NewSecurityInvokerViewIssue(objectType string, objectName string, SqlStatement string) QueryIssue {
	return newQueryIssue(securityInvokerViewIssue, objectType, objectName, SqlStatement, map[string]interface{}{})
}

var deterministicOptionCollationIssue = issue.Issue{
	Type:        DETERMINISTIC_OPTION_WITH_COLLATION,
	Name:        DETERMINISTIC_OPTION_WITH_COLLATION_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: DETERMINISTIC_OPTION_WITH_COLLATION_ISSUE_DESCRIPTION,
	Suggestion:  DETERMINISTIC_OPTION_WITH_COLLATION_ISSUE_SUGGESTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#postgresql-12-and-later-features",
	MinimumVersionsFixedIn: map[string]*ybversion.YBVersion{
		ybversion.SERIES_2_25: ybversion.V2_25_0_0,
	},
}

func NewDeterministicOptionCollationIssue(objectType string, objectName string, SqlStatement string) QueryIssue {
	return newQueryIssue(deterministicOptionCollationIssue, objectType, objectName, SqlStatement, map[string]interface{}{})
}

var nonDeterministicCollationIssue = issue.Issue{
	Type:        NON_DETERMINISTIC_COLLATION,
	Name:        NON_DETERMINISTIC_COLLATION_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: NON_DETERMINISTIC_COLLATION_ISSUE_DESCRIPTION,
	Suggestion:  DETERMINISTIC_OPTION_WITH_COLLATION_ISSUE_SUGGESTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#postgresql-12-and-later-features",
}

func NewNonDeterministicCollationIssue(objectType string, objectName string, SqlStatement string) QueryIssue {
	return newQueryIssue(nonDeterministicCollationIssue, objectType, objectName, SqlStatement, map[string]interface{}{})
}

var foreignKeyReferencesPartitionedTableIssue = issue.Issue{
	Type:        FOREIGN_KEY_REFERENCES_PARTITIONED_TABLE,
	Name:        FOREIGN_KEY_REFERENCES_PARTITIONED_TABLE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: FOREIGN_KEY_REFERENCES_PARTITIONED_TABLE_ISSUE_DESCRIPTION,
	Suggestion:  FOREIGN_KEY_REFERENCES_PARTITIONED_TABLE_ISSUE_SUGGESTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#postgresql-12-and-later-features",
	MinimumVersionsFixedIn: map[string]*ybversion.YBVersion{
		ybversion.SERIES_2_25: ybversion.V2_25_0_0,
	},
}

func NewForeignKeyReferencesPartitionedTableIssue(objectType string, objectName string, SqlStatement string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	return newQueryIssue(foreignKeyReferencesPartitionedTableIssue, objectType, objectName, SqlStatement, details)
}

var sqlBodyInFunctionIssue = issue.Issue{
	Type:        SQL_BODY_IN_FUNCTION,
	Name:        SQL_BODY_IN_FUNCTION_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: "SQL Body for sql languages in function statement is not supported in YugabyteDB",
	Suggestion:  SQL_BODY_IN_FUNCTION_ISSUE_SUGGESTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#postgresql-12-and-later-features",
	MinimumVersionsFixedIn: map[string]*ybversion.YBVersion{
		ybversion.SERIES_2_25: ybversion.V2_25_0_0,
	},
}

func NewSqlBodyInFunctionIssue(objectType string, objectName string, SqlStatement string) QueryIssue {
	return newQueryIssue(sqlBodyInFunctionIssue, objectType, objectName, SqlStatement, map[string]interface{}{})
}

var uniqueNullsNotDistinctIssue = issue.Issue{
	Type:        UNIQUE_NULLS_NOT_DISTINCT,
	Name:        UNIQUE_NULLS_NOT_DISTINCT_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNIQUE_NULLS_NOT_DISTINCT_ISSUE_DESCRIPTION,
	Suggestion:  "",
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#postgresql-12-and-later-features",
	MinimumVersionsFixedIn: map[string]*ybversion.YBVersion{
		ybversion.SERIES_2_25: ybversion.V2_25_0_0,
	},
}

func NewUniqueNullsNotDistinctIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(uniqueNullsNotDistinctIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

/*
Database options works on 2.25 but not marking it as supported in 2.25 for now but as per this ticket

	https://github.com/yugabyte/yugabyte-db/issues/25541, DB will be blocking this support so its not supported technically
*/
var databaseOptionsPG15Issue = issue.Issue{
	Type:        DATABASE_OPTIONS_PG15,
	Name:        "Database options",
	Impact:      constants.IMPACT_LEVEL_2,
	Description: DATABASE_OPTIONS_DESCRIPTION,
	Suggestion:  "",
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#postgresql-12-and-later-features",
	// MinimumVersionsFixedIn: map[string]*ybversion.YBVersion{
	// 	ybversion.SERIES_2_25: ybversion.V2_25_0_0,
	// },// Not marking it as supported for 2.25 as it seems these might be actually not supported at least the locale and collation ones https://github.com/yugabyte/yugabyte-db/issues/25541
}

func NewDatabaseOptionsPG15Issue(objectType string, objectName string, sqlStatement string, options []string) QueryIssue {
	sort.Strings(options)
	issue := databaseOptionsPG15Issue
	issue.Description = fmt.Sprintf(issue.Description, strings.Join(options, ", "))
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var databaseOptionsPG17Issue = issue.Issue{
	Type:        DATABASE_OPTIONS_PG17,
	Name:        "Database options",
	Impact:      constants.IMPACT_LEVEL_2,
	Description: DATABASE_OPTIONS_DESCRIPTION,
	Suggestion:  "",
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#postgresql-12-and-later-features",
}

func NewDatabaseOptionsPG17Issue(objectType string, objectName string, sqlStatement string, options []string) QueryIssue {
	sort.Strings(options)
	issue := databaseOptionsPG17Issue
	issue.Description = fmt.Sprintf(issue.Description, strings.Join(options, ", "))
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}
