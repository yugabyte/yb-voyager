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
	return newIssueInstance(unloggedTableIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
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
