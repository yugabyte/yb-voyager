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
	Type:        UNSUPPORTED_DATATYPE_XML,
	Name:        UNSUPPORTED_DATATYPE_XML_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_3,
	Description: UNSUPPORTED_DATATYPE_ISSUE_DESCRIPTION,
	Suggestion:  XML_DATATYPE_ISSUE_SUGGESTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/1043",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#data-ingestion-on-xml-data-type-is-not-supported",
}

// ============================= Unsupported Datatypes Issues ========================================

func NewXMLDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := xmlDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var xidDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_XID,
	Name:        UNSUPPORTED_DATATYPE_XID_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_3,
	Description: UNSUPPORTED_DATATYPE_ISSUE_DESCRIPTION,
	Suggestion:  XID_DATATYPE_ISSUE_SUGGESTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/15638",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#xid-functions-is-not-supported",
}

func NewXIDDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := xidDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var geometryDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_GEOMETRY,
	Name:        UNSUPPORTED_DATATYPE_GEOMETRY_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_3,
	Description: UNSUPPORTED_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/11323",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-yugabytedb",
}

func NewGeometryDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := geometryDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var geographyDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_GEOGRAPHY,
	Name:        UNSUPPORTED_DATATYPE_GEOGRAPHY_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_3,
	Description: UNSUPPORTED_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/11323",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-yugabytedb",
}

func NewGeographyDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := geographyDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var box2dDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_BOX2D,
	Name:        UNSUPPORTED_DATATYPE_BOX2D_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_3,
	Description: UNSUPPORTED_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/11323",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-yugabytedb",
}

func NewBox2DDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := box2dDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var box3dDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_BOX3D,
	Name:        UNSUPPORTED_DATATYPE_BOX3D_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_3,
	Description: UNSUPPORTED_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/11323",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-yugabytedb",
}

func NewBox3DDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := box3dDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var topogeometryDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_TOPOGEOMETRY,
	Name:        UNSUPPORTED_DATATYPE_TOPOGEOMETRY_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_3,
	Description: UNSUPPORTED_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/11323",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-yugabytedb",
}

func NewTopogeometryDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := topogeometryDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var loDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LARGE_OBJECT,
	Name:        UNSUPPORTED_DATATYPE_LARGE_OBJECT_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_3,
	Description: UNSUPPORTED_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25318",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#large-objects-and-its-functions-are-currently-not-supported", // TODO
}

func NewLODatatypeIssue(objectType string, objectName string, SqlStatement string, typeName string, colName string) QueryIssue {
	issue := loDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, SqlStatement, map[string]interface{}{})
}

var rasterDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_RASTER,
	Name:        UNSUPPORTED_DATATYPE_RASTER_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_3,
	Description: UNSUPPORTED_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-yugabytedb",
}

func NewRasterDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := rasterDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var pgLsnDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_PG_LSN,
	Name:        UNSUPPORTED_DATATYPE_PG_LSN_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_3,
	Description: UNSUPPORTED_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-yugabytedb",
}

func NewPgLsnDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := pgLsnDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var txidSnapshotDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_TXID_SNAPSHOT,
	Name:        UNSUPPORTED_DATATYPE_TXID_SNAPSHOT_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_3,
	Description: UNSUPPORTED_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-yugabytedb",
}

func NewTxidSnapshotDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := txidSnapshotDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var int8MultirangeDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_INT8MULTIRANGE,
	Name:        UNSUPPORTED_DATATYPE_INT8MULTIRANGE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_3,
	Description: UNSUPPORTED_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#postgresql-12-and-later-features",
	MinimumVersionsFixedIn: map[string]*ybversion.YBVersion{
		ybversion.SERIES_2_25: ybversion.V2_25_0_0,
	},
}

func NewInt8MultiRangeDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := int8MultirangeDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var int4MultirangeDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_INT4MULTIRANGE,
	Name:        UNSUPPORTED_DATATYPE_INT4MULTIRANGE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_3,
	Description: UNSUPPORTED_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#postgresql-12-and-later-features",
	MinimumVersionsFixedIn: map[string]*ybversion.YBVersion{
		ybversion.SERIES_2_25: ybversion.V2_25_0_0,
	},
}

func NewInt4MultiRangeDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := int4MultirangeDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var dateMultirangeDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_DATEMULTIRANGE,
	Name:        UNSUPPORTED_DATATYPE_DATEMULTIRANGE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_3,
	Description: UNSUPPORTED_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#postgresql-12-and-later-features",
	MinimumVersionsFixedIn: map[string]*ybversion.YBVersion{
		ybversion.SERIES_2_25: ybversion.V2_25_0_0,
	},
}

func NewDateMultiRangeDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := dateMultirangeDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var numMultirangeDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_NUMMULTIRANGE,
	Name:        UNSUPPORTED_DATATYPE_NUMMULTIRANGE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_3,
	Description: UNSUPPORTED_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#postgresql-12-and-later-features",
	MinimumVersionsFixedIn: map[string]*ybversion.YBVersion{
		ybversion.SERIES_2_25: ybversion.V2_25_0_0,
	},
}

func NewNumMultiRangeDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := numMultirangeDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var tsMultirangeDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_TSMULTIRANGE,
	Name:        UNSUPPORTED_DATATYPE_TSMULTIRANGE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_3,
	Description: UNSUPPORTED_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#postgresql-12-and-later-features",
	MinimumVersionsFixedIn: map[string]*ybversion.YBVersion{
		ybversion.SERIES_2_25: ybversion.V2_25_0_0,
	},
}

func NewTSMultiRangeDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := tsMultirangeDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var tstzMultirangeDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_TSTZMULTIRANGE,
	Name:        UNSUPPORTED_DATATYPE_TSTZMULTIRANGE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_3,
	Description: UNSUPPORTED_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#postgresql-12-and-later-features",
	MinimumVersionsFixedIn: map[string]*ybversion.YBVersion{
		ybversion.SERIES_2_25: ybversion.V2_25_0_0,
	},
}

func NewTSTZMultiRangeDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := tstzMultirangeDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var pointDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_POINT,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_POINT_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewPointDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := pointDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var lineDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_LINE,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_LINE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewLineDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := lineDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var lsegDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_LSEG,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_LSEG_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewLsegDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := lsegDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var boxDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_BOX,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_BOX_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewBoxDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := boxDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var pathDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_PATH,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_PATH_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewPathDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := pathDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var polygonDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_POLYGON,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_POLYGON_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewPolygonDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := polygonDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var circleDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_CIRCLE,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_CIRCLE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewCircleDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := circleDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var arrayOfEnumDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_WITH_FF_FB_ARRAY_OF_ENUM,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_WITH_FF_FB_ARRAY_OF_ENUM_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_WITH_FF_FB_ARRAY_OF_ENUM_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewArrayOfEnumDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := arrayOfEnumDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var userDefinedDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_WITH_FF_FB_USER_DEFINED,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_WITH_FF_FB_USER_DEFINED_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_WITH_FF_FB_USER_DEFINED_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewUserDefinedDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := userDefinedDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var tsQueryDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_WITH_FF_FB_TSQUERY,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_WITH_FF_FB_TSQUERY_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_WITH_FF_FB_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewTsQueryDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := tsQueryDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var tsVectorDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_WITH_FF_FB_TSVECTOR,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_WITH_FF_FB_TSVECTOR_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_WITH_FF_FB_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewTsVectorDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := tsVectorDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var hstoreDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_WITH_FF_FB_HSTORE,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_WITH_FF_FB_HSTORE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_WITH_FF_FB_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewHstoreDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := hstoreDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

// ============================= PK and UK Constraints on Unsupported Datatypes Issues =================

var primaryOrUniqueConstraintOnCitextDatatypeIssue = issue.Issue{
	Type:        PK_UK_ON_CITEXT_DATATYPE,
	Name:        PK_UK_ON_CITEXT_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: PK_UK_ON_CITEXT_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  PK_UK_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported", //Keeping it similar for now, will see if we need to a separate issue on docs,
}

func NewPrimaryOrUniqueConstraintOnCitextDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	issue := primaryOrUniqueConstraintOnCitextDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var primaryOrUniqueConstraintOnTsVectorDatatypeIssue = issue.Issue{
	Type:        PK_UK_ON_TSVECTOR_DATATYPE,
	Name:        PK_UK_ON_TSVECTOR_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: PK_UK_ON_TSVECTOR_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  PK_UK_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported", //Keeping it similar for now, will see if we need to a separate issue on docs,
}

func NewPrimaryOrUniqueConstraintOnTsVectorDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	issue := primaryOrUniqueConstraintOnTsVectorDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var primaryOrUniqueConstraintOnTsQueryDatatypeIssue = issue.Issue{
	Type:        PK_UK_ON_TSQUERY_DATATYPE,
	Name:        PK_UK_ON_TSQUERY_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: PK_UK_ON_TSQUERY_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  PK_UK_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported", //Keeping it similar for now, will see if we need to a separate issue on docs,
}

func NewPrimaryOrUniqueConstraintOnTsQueryDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	issue := primaryOrUniqueConstraintOnTsQueryDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var primaryOrUniqueConstraintOnJsonbDatatypeIssue = issue.Issue{
	Type:        PK_UK_ON_JSONB_DATATYPE,
	Name:        PK_UK_ON_JSONB_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: PK_UK_ON_JSONB_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  PK_UK_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported", //Keeping it similar for now, will see if we need to a separate issue on docs,
}

func NewPrimaryOrUniqueConstraintOnJsonbDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	issue := primaryOrUniqueConstraintOnJsonbDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var primaryOrUniqueConstraintOnInetDatatypeIssue = issue.Issue{
	Type:        PK_UK_ON_INET_DATATYPE,
	Name:        PK_UK_ON_INET_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: PK_UK_ON_INET_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  PK_UK_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported", //Keeping it similar for now, will see if we need to a separate issue on docs,
}

func NewPrimaryOrUniqueConstraintOnInetDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	issue := primaryOrUniqueConstraintOnInetDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var primaryOrUniqueConstraintOnJsonDatatypeIssue = issue.Issue{
	Type:        PK_UK_ON_JSON_DATATYPE,
	Name:        PK_UK_ON_JSON_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: PK_UK_ON_JSON_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  PK_UK_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported", //Keeping it similar for now, will see if we need to a separate issue on docs,
}

func NewPrimaryOrUniqueConstraintOnJsonDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	issue := primaryOrUniqueConstraintOnJsonDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var primaryOrUniqueConstraintOnMacaddrDatatypeIssue = issue.Issue{
	Type:        PK_UK_ON_MACADDR_DATATYPE,
	Name:        PK_UK_ON_MACADDR_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: PK_UK_ON_MACADDR_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  PK_UK_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported", //Keeping it similar for now, will see if we need to a separate issue on docs,
}

func NewPrimaryOrUniqueConstraintOnMacaddrDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	issue := primaryOrUniqueConstraintOnMacaddrDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var primaryOrUniqueConstraintOnMacaddr8DatatypeIssue = issue.Issue{
	Type:        PK_UK_ON_MACADDR8_DATATYPE,
	Name:        PK_UK_ON_MACADDR8_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: PK_UK_ON_MACADDR8_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  PK_UK_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported", //Keeping it similar for now, will see if we need to a separate issue on docs,
}

func NewPrimaryOrUniqueConstraintOnMacaddr8DatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	issue := primaryOrUniqueConstraintOnMacaddr8DatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var primaryOrUniqueConstraintOnCidrDatatypeIssue = issue.Issue{
	Type:        PK_UK_ON_CIDR_DATATYPE,
	Name:        PK_UK_ON_CIDR_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: PK_UK_ON_CIDR_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  PK_UK_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported", //Keeping it similar for now, will see if we need to a separate issue on docs,
}

func NewPrimaryOrUniqueConstraintOnCidrDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	issue := primaryOrUniqueConstraintOnCidrDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var primaryOrUniqueConstraintOnBitDatatypeIssue = issue.Issue{
	Type:        PK_UK_ON_BIT_DATATYPE,
	Name:        PK_UK_ON_BIT_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: PK_UK_ON_BIT_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  PK_UK_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported", //Keeping it similar for now, will see if we need to a separate issue on docs,
}

func NewPrimaryOrUniqueConstraintOnBitDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	issue := primaryOrUniqueConstraintOnBitDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var primaryOrUniqueConstraintOnVarbitDatatypeIssue = issue.Issue{
	Type:        PK_UK_ON_VARBIT_DATATYPE,
	Name:        PK_UK_ON_VARBIT_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: PK_UK_ON_VARBIT_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  PK_UK_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported", //Keeping it similar for now, will see if we need to a separate issue on docs,
}

func NewPrimaryOrUniqueConstraintOnVarbitDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	issue := primaryOrUniqueConstraintOnVarbitDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var primaryOrUniqueConstraintOnDaterangeDatatypeIssue = issue.Issue{
	Type:        PK_UK_ON_DATERANGE_DATATYPE,
	Name:        PK_UK_ON_DATERANGE_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: PK_UK_ON_DATERANGE_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  PK_UK_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported", //Keeping it similar for now, will see if we need to a separate issue on docs,
}

func NewPrimaryOrUniqueConstraintOnDaterangeDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	issue := primaryOrUniqueConstraintOnDaterangeDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var primaryOrUniqueConstraintOnTsrangeDatatypeIssue = issue.Issue{
	Type:        PK_UK_ON_TSRANGE_DATATYPE,
	Name:        PK_UK_ON_TSRANGE_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: PK_UK_ON_TSRANGE_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  PK_UK_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported", //Keeping it similar for now, will see if we need to a separate issue on docs,
}

func NewPrimaryOrUniqueConstraintOnTsrangeDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	issue := primaryOrUniqueConstraintOnTsrangeDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var primaryOrUniqueConstraintOnTstzrangeDatatypeIssue = issue.Issue{
	Type:        PK_UK_ON_TSTZRANGE_DATATYPE,
	Name:        PK_UK_ON_TSTZRANGE_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: PK_UK_ON_TSTZRANGE_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  PK_UK_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported", //Keeping it similar for now, will see if we need to a separate issue on docs,
}

func NewPrimaryOrUniqueConstraintOnTstzrangeDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	issue := primaryOrUniqueConstraintOnTstzrangeDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var primaryOrUniqueConstraintOnNumrangeDatatypeIssue = issue.Issue{
	Type:        PK_UK_ON_NUMRANGE_DATATYPE,
	Name:        PK_UK_ON_NUMRANGE_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: PK_UK_ON_NUMRANGE_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  PK_UK_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported", //Keeping it similar for now, will see if we need to a separate issue on docs,
}

func NewPrimaryOrUniqueConstraintOnNumrangeDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	issue := primaryOrUniqueConstraintOnNumrangeDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var primaryOrUniqueConstraintOnInt4rangeDatatypeIssue = issue.Issue{
	Type:        PK_UK_ON_INT4RANGE_DATATYPE,
	Name:        PK_UK_ON_INT4RANGE_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: PK_UK_ON_INT4RANGE_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  PK_UK_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported", //Keeping it similar for now, will see if we need to a separate issue on docs,
}

func NewPrimaryOrUniqueConstraintOnInt4rangeDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	issue := primaryOrUniqueConstraintOnInt4rangeDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var primaryOrUniqueConstraintOnInt8rangeDatatypeIssue = issue.Issue{
	Type:        PK_UK_ON_INT8RANGE_DATATYPE,
	Name:        PK_UK_ON_INT8RANGE_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: PK_UK_ON_INT8RANGE_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  PK_UK_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported", //Keeping it similar for now, will see if we need to a separate issue on docs,
}

func NewPrimaryOrUniqueConstraintOnInt8rangeDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	issue := primaryOrUniqueConstraintOnInt8rangeDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var primaryOrUniqueConstraintOnIntervalDatatypeIssue = issue.Issue{
	Type:        PK_UK_ON_INTERVAL_DATATYPE,
	Name:        PK_UK_ON_INTERVAL_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: PK_UK_ON_INTERVAL_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  PK_UK_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported", //Keeping it similar for now, will see if we need to a separate issue on docs,
}

func NewPrimaryOrUniqueConstraintOnIntervalDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	issue := primaryOrUniqueConstraintOnIntervalDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var primaryOrUniqueConstraintOnCircleDatatypeIssue = issue.Issue{
	Type:        PK_UK_ON_CIRCLE_DATATYPE,
	Name:        PK_UK_ON_CIRCLE_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: PK_UK_ON_CIRCLE_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  PK_UK_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported", //Keeping it similar for now, will see if we need to a separate issue on docs,
}

func NewPrimaryOrUniqueConstraintOnCircleDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	issue := primaryOrUniqueConstraintOnCircleDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var primaryOrUniqueConstraintOnBoxDatatypeIssue = issue.Issue{
	Type:        PK_UK_ON_BOX_DATATYPE,
	Name:        PK_UK_ON_BOX_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: PK_UK_ON_BOX_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  PK_UK_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported", //Keeping it similar for now, will see if we need to a separate issue on docs,
}

func NewPrimaryOrUniqueConstraintOnBoxDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	issue := primaryOrUniqueConstraintOnBoxDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var primaryOrUniqueConstraintOnLineDatatypeIssue = issue.Issue{
	Type:        PK_UK_ON_LINE_DATATYPE,
	Name:        PK_UK_ON_LINE_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: PK_UK_ON_LINE_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  PK_UK_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported", //Keeping it similar for now, will see if we need to a separate issue on docs,
}

func NewPrimaryOrUniqueConstraintOnLineDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	issue := primaryOrUniqueConstraintOnLineDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var primaryOrUniqueConstraintOnLsegDatatypeIssue = issue.Issue{
	Type:        PK_UK_ON_LSEG_DATATYPE,
	Name:        PK_UK_ON_LSEG_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: PK_UK_ON_LSEG_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  PK_UK_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported", //Keeping it similar for now, will see if we need to a separate issue on docs,
}

func NewPrimaryOrUniqueConstraintOnLsegDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	issue := primaryOrUniqueConstraintOnLsegDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var primaryOrUniqueConstraintOnPointDatatypeIssue = issue.Issue{
	Type:        PK_UK_ON_POINT_DATATYPE,
	Name:        PK_UK_ON_POINT_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: PK_UK_ON_POINT_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  PK_UK_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported", //Keeping it similar for now, will see if we need to a separate issue on docs,
}

func NewPrimaryOrUniqueConstraintOnPointDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	issue := primaryOrUniqueConstraintOnPointDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var primaryOrUniqueConstraintOnPgLsnDatatypeIssue = issue.Issue{
	Type:        PK_UK_ON_PGLSN_DATATYPE,
	Name:        PK_UK_ON_PGLSN_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: PK_UK_ON_PGLSN_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  PK_UK_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported", //Keeping it similar for now, will see if we need to a separate issue on docs,
}

func NewPrimaryOrUniqueConstraintOnPgLsnDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	issue := primaryOrUniqueConstraintOnPgLsnDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var primaryOrUniqueConstraintOnPathDatatypeIssue = issue.Issue{
	Type:        PK_UK_ON_PATH_DATATYPE,
	Name:        PK_UK_ON_PATH_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: PK_UK_ON_PATH_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  PK_UK_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported", //Keeping it similar for now, will see if we need to a separate issue on docs,
}

func NewPrimaryOrUniqueConstraintOnPathDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	issue := primaryOrUniqueConstraintOnPathDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var primaryOrUniqueConstraintOnPolygonDatatypeIssue = issue.Issue{
	Type:        PK_UK_ON_POLYGON_DATATYPE,
	Name:        PK_UK_ON_POLYGON_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: PK_UK_ON_POLYGON_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  PK_UK_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported", //Keeping it similar for now, will see if we need to a separate issue on docs,
}

func NewPrimaryOrUniqueConstraintOnPolygonDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	issue := primaryOrUniqueConstraintOnPolygonDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var primaryOrUniqueConstraintOnTxidSnapshotDatatypeIssue = issue.Issue{
	Type:        PK_UK_ON_TXID_SNAPSHOT_DATATYPE,
	Name:        PK_UK_ON_TXID_SNAPSHOT_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: PK_UK_ON_TXID_SNAPSHOT_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  PK_UK_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported", //Keeping it similar for now, will see if we need to a separate issue on docs,
}

func NewPrimaryOrUniqueConstraintOnTxidSnapshotDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	issue := primaryOrUniqueConstraintOnTxidSnapshotDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var primaryOrUniqueConstraintOnArrayDatatypeIssue = issue.Issue{
	Type:        PK_UK_ON_ARRAY_DATATYPE,
	Name:        PK_UK_ON_ARRAY_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: PK_UK_ON_ARRAY_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  PK_UK_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported", //Keeping it similar for now, will see if we need to a separate issue on docs,
}

func NewPrimaryOrUniqueConstraintOnArrayDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	issue := primaryOrUniqueConstraintOnArrayDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var primaryOrUniqueConstraintOnUserDefinedTypeIssue = issue.Issue{
	Type:        PK_UK_ON_USER_DEFINED_DATATYPE,
	Name:        PK_UK_ON_USER_DEFINED_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: PK_UK_ON_USER_DEFINED_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  PK_UK_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported", //Keeping it similar for now, will see if we need to a separate issue on docs,
}

func NewPrimaryOrUniqueConstraintOnUserDefinedTypeIssue(objectType string, objectName string, sqlStatement string, typeName string, constraintName string) QueryIssue {
	details := map[string]interface{}{
		CONSTRAINT_NAME: constraintName,
	}
	issue := primaryOrUniqueConstraintOnUserDefinedTypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

// ============================= Index on Unsupported Datatypes Issues =================

var indexOnArrayDatatypeIssue = issue.Issue{
	Type:        INDEX_ON_ARRAY_DATATYPE,
	Name:        INDEX_ON_ARRAY_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: INDEX_ON_ARRAY_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  INDEX_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported",
}

func NewIndexOnArrayDatatypeIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	issue := indexOnArrayDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var indexOnUserDefinedDatatypeIssue = issue.Issue{
	Type:        INDEX_ON_USER_DEFINED_DATATYPE,
	Name:        INDEX_ON_USER_DEFINED_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: INDEX_ON_USER_DEFINED_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  INDEX_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported",
}

func NewIndexOnUserDefinedTypeIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	issue := indexOnUserDefinedDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var indexOnCitextDatatypeIssue = issue.Issue{
	Type:        INDEX_ON_CITEXT_DATATYPE,
	Name:        INDEX_ON_CITEXT_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: INDEX_ON_CITEXT_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  INDEX_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported",
}

func NewIndexOnCitextDatatypeIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	issue := indexOnCitextDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var indexOnTsVectorDatatypeIssue = issue.Issue{
	Type:        INDEX_ON_TSVECTOR_DATATYPE,
	Name:        INDEX_ON_TSVECTOR_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: INDEX_ON_TSVECTOR_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  INDEX_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported",
}

func NewIndexOnTsVectorDatatypeIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	issue := indexOnTsVectorDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var indexOnTsQueryDatatypeIssue = issue.Issue{
	Type:        INDEX_ON_TSQUERY_DATATYPE,
	Name:        INDEX_ON_TSQUERY_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: INDEX_ON_TSQUERY_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  INDEX_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported",
}

func NewIndexOnTsQueryDatatypeIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	issue := indexOnTsQueryDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var indexOnJsonbDatatypeIssue = issue.Issue{
	Type:        INDEX_ON_JSONB_DATATYPE,
	Name:        INDEX_ON_JSONB_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: INDEX_ON_JSONB_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  INDEX_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported",
}

func NewIndexOnJsonbDatatypeIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	issue := indexOnJsonbDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var indexOnInetDatatypeIssue = issue.Issue{
	Type:        INDEX_ON_INET_DATATYPE,
	Name:        INDEX_ON_INET_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: INDEX_ON_INET_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  INDEX_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported",
}

func NewIndexOnInetDatatypeIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	issue := indexOnInetDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var indexOnJsonDatatypeIssue = issue.Issue{
	Type:        INDEX_ON_JSON_DATATYPE,
	Name:        INDEX_ON_JSON_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: INDEX_ON_JSON_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  INDEX_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported",
}

func NewIndexOnJsonDatatypeIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	issue := indexOnJsonDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var indexOnMacaddrDatatypeIssue = issue.Issue{
	Type:        INDEX_ON_MACADDR_DATATYPE,
	Name:        INDEX_ON_MACADDR_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: INDEX_ON_MACADDR_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  INDEX_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported",
}

func NewIndexOnMacaddrDatatypeIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	issue := indexOnMacaddrDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var indexOnMacaddr8DatatypeIssue = issue.Issue{
	Type:        INDEX_ON_MACADDR8_DATATYPE,
	Name:        INDEX_ON_MACADDR8_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: INDEX_ON_MACADDR8_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  INDEX_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported",
}

func NewIndexOnMacaddr8DatatypeIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	issue := indexOnMacaddr8DatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var indexOnCidrDatatypeIssue = issue.Issue{
	Type:        INDEX_ON_CIDR_DATATYPE,
	Name:        INDEX_ON_CIDR_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: INDEX_ON_CIDR_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  INDEX_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported",
}

func NewIndexOnCidrDatatypeIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	issue := indexOnCidrDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var indexOnBitDatatypeIssue = issue.Issue{
	Type:        INDEX_ON_BIT_DATATYPE,
	Name:        INDEX_ON_BIT_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: INDEX_ON_BIT_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  INDEX_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported",
}

func NewIndexOnBitDatatypeIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	issue := indexOnBitDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var indexOnVarbitDatatypeIssue = issue.Issue{
	Type:        INDEX_ON_VARBIT_DATATYPE,
	Name:        INDEX_ON_VARBIT_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: INDEX_ON_VARBIT_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  INDEX_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported",
}

func NewIndexOnVarbitDatatypeIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	issue := indexOnVarbitDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var indexOnDaterangeDatatypeIssue = issue.Issue{
	Type:        INDEX_ON_DATERANGE_DATATYPE,
	Name:        INDEX_ON_DATERANGE_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: INDEX_ON_DATERANGE_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  INDEX_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported",
}

func NewIndexOnDaterangeDatatypeIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	issue := indexOnDaterangeDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var indexOnTsrangeDatatypeIssue = issue.Issue{
	Type:        INDEX_ON_TSRANGE_DATATYPE,
	Name:        INDEX_ON_TSRANGE_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: INDEX_ON_TSRANGE_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  INDEX_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported",
}

func NewIndexOnTsrangeDatatypeIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	issue := indexOnTsrangeDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var indexOnTstzrangeDatatypeIssue = issue.Issue{
	Type:        INDEX_ON_TSTZRANGE_DATATYPE,
	Name:        INDEX_ON_TSTZRANGE_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: INDEX_ON_TSTZRANGE_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  INDEX_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported",
}

func NewIndexOnTstzrangeDatatypeIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	issue := indexOnTstzrangeDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var indexOnNumrangeDatatypeIssue = issue.Issue{
	Type:        INDEX_ON_NUMRANGE_DATATYPE,
	Name:        INDEX_ON_NUMRANGE_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: INDEX_ON_NUMRANGE_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  INDEX_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported",
}

func NewIndexOnNumrangeDatatypeIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	issue := indexOnNumrangeDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var indexOnInt4rangeDatatypeIssue = issue.Issue{
	Type:        INDEX_ON_INT4RANGE_DATATYPE,
	Name:        INDEX_ON_INT4RANGE_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: INDEX_ON_INT4RANGE_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  INDEX_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported",
}

func NewIndexOnInt4rangeDatatypeIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	issue := indexOnInt4rangeDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var indexOnInt8rangeDatatypeIssue = issue.Issue{
	Type:        INDEX_ON_INT8RANGE_DATATYPE,
	Name:        INDEX_ON_INT8RANGE_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: INDEX_ON_INT8RANGE_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  INDEX_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported",
}

func NewIndexOnInt8rangeDatatypeIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	issue := indexOnInt8rangeDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var indexOnIntervalDatatypeIssue = issue.Issue{
	Type:        INDEX_ON_INTERVAL_DATATYPE,
	Name:        INDEX_ON_INTERVAL_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: INDEX_ON_INTERVAL_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  INDEX_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported",
}

func NewIndexOnIntervalDatatypeIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	issue := indexOnIntervalDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var indexOnCircleDatatypeIssue = issue.Issue{
	Type:        INDEX_ON_CIRCLE_DATATYPE,
	Name:        INDEX_ON_CIRCLE_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: INDEX_ON_CIRCLE_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  INDEX_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported",
}

func NewIndexOnCircleDatatypeIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	issue := indexOnCircleDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var indexOnBoxDatatypeIssue = issue.Issue{
	Type:        INDEX_ON_BOX_DATATYPE,
	Name:        INDEX_ON_BOX_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: INDEX_ON_BOX_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  INDEX_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported",
}

func NewIndexOnBoxDatatypeIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	issue := indexOnBoxDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var indexOnLineDatatypeIssue = issue.Issue{
	Type:        INDEX_ON_LINE_DATATYPE,
	Name:        INDEX_ON_LINE_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: INDEX_ON_LINE_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  INDEX_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported",
}

func NewIndexOnLineDatatypeIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	issue := indexOnLineDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var indexOnLsegDatatypeIssue = issue.Issue{
	Type:        INDEX_ON_LSEG_DATATYPE,
	Name:        INDEX_ON_LSEG_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: INDEX_ON_LSEG_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  INDEX_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported",
}

func NewIndexOnLsegDatatypeIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	issue := indexOnLsegDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var indexOnPointDatatypeIssue = issue.Issue{
	Type:        INDEX_ON_POINT_DATATYPE,
	Name:        INDEX_ON_POINT_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: INDEX_ON_POINT_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  INDEX_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported",
}

func NewIndexOnPointDatatypeIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	issue := indexOnPointDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var indexOnPgLsnDatatypeIssue = issue.Issue{
	Type:        INDEX_ON_PG_LSN_DATATYPE,
	Name:        INDEX_ON_PG_LSN_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: INDEX_ON_PG_LSN_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  INDEX_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported",
}

func NewIndexOnPgLsnDatatypeIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	issue := indexOnPgLsnDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var indexOnPathDatatypeIssue = issue.Issue{
	Type:        INDEX_ON_PATH_DATATYPE,
	Name:        INDEX_ON_PATH_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: INDEX_ON_PATH_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  INDEX_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported",
}

func NewIndexOnPathDatatypeIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	issue := indexOnPathDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var indexOnPolygonDatatypeIssue = issue.Issue{
	Type:        INDEX_ON_POLYGON_DATATYPE,
	Name:        INDEX_ON_POLYGON_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: INDEX_ON_POLYGON_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  INDEX_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported",
}

func NewIndexOnPolygonDatatypeIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	issue := indexOnPolygonDatatypeIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var indexOnTxidSnapshotDatatypeIssue = issue.Issue{
	Type:        INDEX_ON_TXID_SNAPSHOT_DATATYPE,
	Name:        INDEX_ON_TXID_SNAPSHOT_DATATYPE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: INDEX_ON_TXID_SNAPSHOT_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25003",
	Suggestion:  INDEX_ON_COMPLEX_DATATYPE_ISSUE_SUGGESTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#indexes-on-some-complex-data-types-are-not-supported",
}

func NewIndexOnTxidSnapshotDatatypeIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	issue := indexOnTxidSnapshotDatatypeIssue
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

var securityInvokerViewIssue = issue.Issue{
	Type:        SECURITY_INVOKER_VIEWS,
	Name:        SECURITY_INVOKER_VIEWS_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: SECURITY_INVOKER_VIEWS_ISSUE_DESCRIPTION,
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
	Name:        DETERMINISTIC_OPTION_WITH_COLLATION_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: DETERMINISTIC_OPTION_WITH_COLLATION_ISSUE_DESCRIPTION,
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
	Name:        NON_DETERMINISTIC_COLLATION_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: NON_DETERMINISTIC_COLLATION_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#postgresql-12-and-later-features",
}

func NewNonDeterministicCollationIssue(objectType string, objectName string, SqlStatement string) QueryIssue {
	return newQueryIssue(nonDeterministicCollationIssue, objectType, objectName, SqlStatement, map[string]interface{}{})
}

var foreignKeyReferencesPartitionedTableIssue = issue.Issue{
	Type:        FOREIGN_KEY_REFERENCES_PARTITIONED_TABLE,
	Name:        FOREIGN_KEY_REFERENCES_PARTITIONED_TABLE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: FOREIGN_KEY_REFERENCES_PARTITIONED_TABLE_ISSUE_DESCRIPTION,
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
	Name:        SQL_BODY_IN_FUNCTION_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: SQL_BODY_IN_FUNCTION_ISSUE_DESCRIPTION,
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
	Name:        UNIQUE_NULLS_NOT_DISTINCT_ISSUE_NAME,
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
Compression clause is the PG14 feature. So it doesn't work on PG11-YB versions
CREATE TABLE with this COMPRESSION clause works on 2.25 by chance but the actual functionality is not there
e.g.
yugabyte=# CREATE TABLE tbl_comp (id int, val text COMPRESSION pglz);
CREATE TABLE
yugabyte=# SELECT relname AS main_table, reltoastrelid,

	(SELECT relname FROM pg_class WHERE oid = c.reltoastrelid) AS toast_table

FROM pg_class c
WHERE relname = 'tbl_comp';

	main_table | reltoastrelid | toast_table

------------+---------------+-------------

	tbl_comp   |             0 |

(1 row)

Refer this issue comments for more details - https://github.com/yugabyte/yugabyte-db/issues/12845#issuecomment-1152261234
Hence this feature is not completely not supported so not marking it supported in 2.25
*/
var compressionClauseForToasting = issue.Issue{
	Type:        COMPRESSION_CLAUSE_IN_TABLE,
	Name:        COMPRESSION_CLAUSE_IN_TABLE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: "TOASTing is disabled internally in YugabyteDB and hence this clause is not relevant.",
	Suggestion:  "Remove the clause from the DDL.",
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#postgresql-12-and-later-features",
}

func NewCompressionClauseForToasting(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(compressionClauseForToasting, objectType, objectName, sqlStatement, map[string]interface{}{})
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

var extensionsIssue = issue.Issue{
	Type:        UNSUPPORTED_EXTENSION,
	Name:        UNSUPPORTED_EXTENSION_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_3,
	Description: UNSUPPORTED_EXTENSION_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1538",
	DocsLink:    "https://docs.yugabyte.com/preview/explore/ysql-language-features/pg-extensions/",
	// TODO: main version specific list of unsupported extension; based on that we can figure out MinimumVersionsFixedIn dynamically for each extension
}

func NewExtensionsIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	issue := extensionsIssue
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var hotspotsOnDateIndexes = issue.Issue{
	Type:        HOTSPOTS_ON_DATE_INDEX,
	Name:        HOTSPOTS_ON_DATE_INDEX_ISSUE,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: HOTSPOTS_ON_RANGE_SHARDED_INDEX_ISSUE_DESCRIPTION,
	GH:          "",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#hotspots-with-range-sharded-timestamp-date-indexes",
}

func NewHotspotOnDateIndexIssue(objectType string, objectName string, sqlStatement string, colName string) QueryIssue {
	issue := hotspotsOnDateIndexes
	if colName != "" {
		issue.Description = fmt.Sprintf("%s Affected column: %s", issue.Description, colName)
	}
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var hotspotsOnTimestampIndexes = issue.Issue{
	Type:        HOTSPOTS_ON_TIMESTAMP_INDEX,
	Name:        HOTSPOTS_ON_TIMESTAMP_INDEX_ISSUE,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: HOTSPOTS_ON_RANGE_SHARDED_INDEX_ISSUE_DESCRIPTION,
	GH:          "",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#hotspots-with-range-sharded-timestamp-date-indexes",
}

func NewHotspotOnTimestampIndexIssue(objectType string, objectName string, sqlStatement string, colName string) QueryIssue {
	issue := hotspotsOnTimestampIndexes
	if colName != "" {
		issue.Description = fmt.Sprintf("%s Affected column: %s", issue.Description, colName)
	}
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var suggestionOnDateIndexesForRangeSharding = issue.Issue{
	Type:        HASH_SHARDING_DATE_INDEX,
	Name:        HASH_SHARDING_DATE_INDEX_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: HASH_SHARDING_RECOMMENDATION_ON_DATE_TIMESTAMP_INDEXES,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/49",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#hash-sharding-with-indexes-on-the-timestamp-date-columns",
}

func NewSuggestionOnDateIndexesForRangeSharding(objectType string, objectName string, sqlStatement string, colName string) QueryIssue {
	issue := suggestionOnDateIndexesForRangeSharding
	if colName != "" {
		issue.Description = fmt.Sprintf("%s Affected column: %s", issue.Description, colName)
	}
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var suggestionOnTimestampIndexesForRangeSharding = issue.Issue{
	Type:        HASH_SHARDING_TIMESTAMP_INDEX,
	Name:        HASH_SHARDING_TIMESTAMP_INDEX_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: HASH_SHARDING_RECOMMENDATION_ON_DATE_TIMESTAMP_INDEXES,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/49",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#hash-sharding-with-indexes-on-the-timestamp-date-columns",
}

func NewSuggestionOnTimestampIndexesForRangeSharding(objectType string, objectName string, sqlStatement string, colName string) QueryIssue {
	issue := suggestionOnTimestampIndexesForRangeSharding
	if colName != "" {
		issue.Description = fmt.Sprintf("%s Affected column: %s", issue.Description, colName)
	}
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var redundantIndexesIssue = issue.Issue{
	Name:        REDUNDANT_INDEXES_ISSUE_NAME,
	Type:        REDUNDANT_INDEXES,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: REDUNDANT_INDEXES_DESCRIPTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#redundant-indexes",
}

func NewRedundantIndexIssue(objectType string, objectName string, sqlStatement string, existingDDL string) QueryIssue {
	issue := redundantIndexesIssue
	if existingDDL != "" {
		issue.Description = fmt.Sprintf("%s\nExisting Index SQL Statement: %s", issue.Description, existingDDL)
	}
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var lowCardinalityIndexIssue = issue.Issue{
	Name:     LOW_CARDINALITY_INDEX_ISSUE_NAME,
	Type:     LOW_CARDINALITY_INDEXES,
	Impact:   constants.IMPACT_LEVEL_1,
	DocsLink: "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#low-cardinality-indexes",
}

func NewLowCardinalityIndexesIssue(objectType string, objectName string, sqlStatement string, numOfColumns int, cardinality int, columnName string) QueryIssue {
	issue := lowCardinalityIndexIssue
	if numOfColumns > 1 {
		issue.Description = fmt.Sprintf("%s %s\nCardinality of the column '%s' is %d.", LOW_CARDINALITY_DESCRIPTION, LOW_CARDINALITY_DESCRIPTION_MULTI_COLUMN, columnName, cardinality)
	} else {
		issue.Description = fmt.Sprintf("%s %s\nCardinality of the column '%s' is %d.", LOW_CARDINALITY_DESCRIPTION, LOW_CARDINALITY_DESCRIPTION_SINGLE_COLUMN, columnName, cardinality)
	}
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var nullValueIndexes = issue.Issue{
	Name:     NULL_VALUE_INDEXES_ISSUE_NAME,
	Type:     NULL_VALUE_INDEXES,
	Impact:   constants.IMPACT_LEVEL_1,
	DocsLink: "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#null-value-indexes",
}

func NewNullValueIndexesIssue(objectType string, objectName string, sqlStatement string, numOfColumns int, nullFrequency float64, columnName string) QueryIssue {
	issue := nullValueIndexes
	perc := int(nullFrequency * 100)
	if numOfColumns > 1 {
		issue.Description = fmt.Sprintf("%s %s\nFrequency of NULLs on the column '%s' is %d%%.", NULL_VALUE_INDEXES_DESCRIPTION, NULL_VALUE_INDEXES_DESCRIPTION_MULTI_COLUMN, columnName, perc)
	} else {
		issue.Description = fmt.Sprintf("%s %s\nFrequency of NULLs on the column '%s' is %d%%.", NULL_VALUE_INDEXES_DESCRIPTION, NULL_VALUE_INDEXES_DESCRIPTION_SINGLE_COLUMN, columnName, perc)
	}
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var mostFrequentValueIndexIssue = issue.Issue{
	Name:     MOST_FREQUENT_VALUE_INDEXES_ISSUE_NAME,
	Type:     MOST_FREQUENT_VALUE_INDEXES,
	Impact:   constants.IMPACT_LEVEL_1,
	DocsLink: "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#low-cardinality-indexes",
}

func NewMostFrequentValueIndexesIssue(objectType string, objectName string, sqlStatement string, numOfColumns int, value string, frequency float64, columnName string) QueryIssue {
	issue := mostFrequentValueIndexIssue
	perc := int(frequency * 100)
	if numOfColumns > 1 {
		issue.Description = fmt.Sprintf("%s %s\nFrequently occuring value '%s' with frequency %d%% on the column '%s'.", MOST_FREQUENT_VALUE_INDEX_DESCRIPTION, MOST_FREQUENT_VALUE_INDEX_DESCRIPTION_MULTI_COLUMN, value, perc, columnName)
	} else {
		issue.Description = fmt.Sprintf("%s %s\nFrequently occuring value '%s' with frequency %d%% on the column '%s'.", MOST_FREQUENT_VALUE_INDEX_DESCRIPTION, MOST_FREQUENT_VALUE_INDEX_DESCRIPTION_SINGLE_COLUMN, value, perc, columnName)
	}
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}
