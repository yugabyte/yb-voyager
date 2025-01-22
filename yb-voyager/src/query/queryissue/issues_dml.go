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
	"sort"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/issue"
)

var advisoryLocksIssue = issue.Issue{
	Type:        ADVISORY_LOCKS,
	Name:        "Advisory Locks",
	Impact:      constants.IMPACT_LEVEL_2,
	Description: ADVISORY_LOCKS_ISSUE_DESCRIPTION,
	Suggestion:  "",
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/3642",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#advisory-locks-is-not-yet-implemented",
}

func NewAdvisoryLocksIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(advisoryLocksIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var systemColumnsIssue = issue.Issue{
	Type:        SYSTEM_COLUMNS,
	Name:        "System Columns",
	Impact:      constants.IMPACT_LEVEL_2,
	Description: SYSTEM_COLUMNS_ISSUE_DESCRIPTION,
	Suggestion:  "",
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/24843",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#system-columns-is-not-yet-supported",
}

func NewSystemColumnsIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(systemColumnsIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var xmlFunctionsIssue = issue.Issue{
	Type:        XML_FUNCTIONS,
	Name:        "XML Functions",
	Impact:      constants.IMPACT_LEVEL_2,
	Description: XML_FUNCTIONS_ISSUE_DESCRIPTION,
	Suggestion:  "",
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/1043",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#xml-functions-is-not-yet-supported",
}

func NewXmlFunctionsIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(xmlFunctionsIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var regexFunctionsIssue = issue.Issue{
	Type:        REGEX_FUNCTIONS,
	Name:        "Regex Functions",
	Impact:      constants.IMPACT_LEVEL_2,
	Description: REGEX_FUNCTIONS_ISSUE_DESCRIPTION,
	Suggestion:  "",
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#postgresql-12-and-later-features",
}

func NewRegexFunctionsIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(regexFunctionsIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var aggregateFunctionIssue = issue.Issue{
	Type:        AGGREGATE_FUNCTION,
	Name:        AGGREGATION_FUNCTIONS_NAME,
	Impact:      constants.IMPACT_LEVEL_2,
	Description: AGGREGATE_FUNCTION_ISSUE_DESCRIPTION,
	Suggestion:  "",
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#postgresql-12-and-later-features",
}

func NewAggregationFunctionIssue(objectType string, objectName string, sqlStatement string, funcNames []string) QueryIssue {
	sort.Strings(funcNames)
	details := map[string]interface{}{
		FUNCTION_NAMES: funcNames, //TODO USE it later when we start putting these in reports
	}
	return newQueryIssue(aggregateFunctionIssue, objectType, objectName, sqlStatement, details)
}

var jsonConstructorFunctionsIssue = issue.Issue{
	Type:        JSON_CONSTRUCTOR_FUNCTION,
	Name:        JSON_CONSTRUCTOR_FUNCTION_NAME,
	Impact:      constants.IMPACT_LEVEL_2,
	Description: JSON_CONSTRUCTOR_FUNCTION_ISSUE_DESCRIPTION,
	Suggestion:  "",
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#postgresql-12-and-later-features",
}

func NewJsonConstructorFunctionIssue(objectType string, objectName string, sqlStatement string, funcNames []string) QueryIssue {
	sort.Strings(funcNames)
	details := map[string]interface{}{
		FUNCTION_NAMES: funcNames, //TODO USE it later when we start putting these in reports
	}
	return newQueryIssue(jsonConstructorFunctionsIssue, objectType, objectName, sqlStatement, details)
}

var jsonQueryFunctionIssue = issue.Issue{
	Type:        JSON_QUERY_FUNCTION,
	Name:        JSON_QUERY_FUNCTIONS_NAME,
	Impact:      constants.IMPACT_LEVEL_2,
	Description: JSON_QUERY_FUNCTION_ISSUE_DESCRIPTION,
	Suggestion:  "",
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#postgresql-12-and-later-features",
}

func NewJsonQueryFunctionIssue(objectType string, objectName string, sqlStatement string, funcNames []string) QueryIssue {
	sort.Strings(funcNames)
	details := map[string]interface{}{
		FUNCTION_NAMES: funcNames, //TODO USE it later when we start putting these in reports
	}
	return newQueryIssue(jsonQueryFunctionIssue, objectType, objectName, sqlStatement, details)
}

var loFunctionsIssue = issue.Issue{
	Type:        LARGE_OBJECT_FUNCTIONS,
	Name:        LARGE_OBJECT_FUNCTIONS_NAME,
	Impact:      constants.IMPACT_LEVEL_2,
	Description: LO_FUNCTIONS_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25318",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#large-objects-and-its-functions-are-currently-not-supported",
}

func NewLOFuntionsIssue(objectType string, objectName string, sqlStatement string, funcNames []string) QueryIssue {
	sort.Strings(funcNames)
	details := map[string]interface{}{
		FUNCTION_NAMES: funcNames, //TODO USE it later when we start putting these in reports
	}
	return newQueryIssue(loFunctionsIssue, objectType, objectName, sqlStatement, details)
}

var jsonbSubscriptingIssue = issue.Issue{
	Type:        JSONB_SUBSCRIPTING,
	Name:        JSONB_SUBSCRIPTING_NAME,
	Impact:      constants.IMPACT_LEVEL_2,
	Description: JSONB_SUBSCRIPTING_ISSUE_DESCRIPTION,
	Suggestion:  "Use Arrow operators (-> / ->>) to access the jsonb fields.",
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#jsonb-subscripting",
}

func NewJsonbSubscriptingIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(jsonbSubscriptingIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var jsonPredicateIssue = issue.Issue{
	Type:        JSON_TYPE_PREDICATE,
	Name:        JSON_TYPE_PREDICATE_NAME,
	Impact:      constants.IMPACT_LEVEL_2,
	Description: JSON_PREDICATE_ISSUE_DESCRIPTION,
	Suggestion:  "",
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#postgresql-12-and-later-features",
}

func NewJsonPredicateIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(jsonPredicateIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var copyFromWhereIssue = issue.Issue{
	Type:        COPY_FROM_WHERE,
	Name:        "COPY FROM ... WHERE",
	Impact:      constants.IMPACT_LEVEL_2,
	Description: COPY_FROM_WHERE_ISSUE_DESCRIPTION,
	Suggestion:  "",
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#postgresql-12-and-later-features",
}

func NewCopyFromWhereIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(copyFromWhereIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var copyOnErrorIssue = issue.Issue{
	Type:        COPY_ON_ERROR,
	Name:        "COPY ... ON_ERROR",
	Impact:      constants.IMPACT_LEVEL_2,
	Description: COPY_ON_ERROR_ISSUE_DESCRIPTION,
	Suggestion:  "",
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#postgresql-12-and-later-features",
}

func NewCopyOnErrorIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(copyOnErrorIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var fetchWithTiesIssue = issue.Issue{
	Type:        FETCH_WITH_TIES,
	Name:        "FETCH .. WITH TIES",
	Impact:      constants.IMPACT_LEVEL_2,
	Description: FETCH_WITH_TIES_ISSUE_DESCRIPTION,
	Suggestion:  "No workaround available right now",
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#postgresql-12-and-later-features",
}

func NewFetchWithTiesIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(fetchWithTiesIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var cteWithMaterializedIssue = issue.Issue{
	Type:        CTE_WITH_MATERIALIZED_CLAUSE,
	Name:        CTE_WITH_MATERIALIZED_CLAUSE_NAME,
	Impact:      constants.IMPACT_LEVEL_2,
	Description: "Modifying the materialization of CTE is not supported yet in YugabyteDB.",
	Suggestion:  "No workaround available right now",
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#postgresql-12-and-later-features",
}

func NewCTEWithMaterializedIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(cteWithMaterializedIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var mergeStatementIssue = issue.Issue{
	Type:        MERGE_STATEMENT,
	Name:        "Merge Statement",
	Impact:      constants.IMPACT_LEVEL_2,
	Description: MERGE_STATEMENT_ISSUE_DESCRIPTION,
	Suggestion:  "Use PL/pgSQL to write the logic to get this functionality",
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25574",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#merge-command",
}

func NewMergeStatementIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	//MERGE STATEMENT is PG15 feature but  MERGE .... RETURNING clause is PG17 feature so need to report it separately later.
	return newQueryIssue(mergeStatementIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}
