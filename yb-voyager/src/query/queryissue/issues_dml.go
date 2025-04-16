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

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/issue"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/ybversion"
)

var advisoryLocksIssue = issue.Issue{
	Type:        ADVISORY_LOCKS,
	Name:        ADVISORY_LOCKS_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_2,
	Description: ADVISORY_LOCKS_ISSUE_DESCRIPTION,
	Suggestion:  "",
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/3642",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#advisory-locks-is-not-yet-implemented",
}

func NewAdvisoryLocksIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(advisoryLocksIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

// ------------------------------------------- System Columns Issue ------------------------------------------------

var xminSystemColumnIssue = issue.Issue{
	Type:       SYSTEM_COLUMN_XMIN,
	Name:       SYSTEM_COLUMN_XMIN_ISSUE_NAME,
	Impact:     constants.IMPACT_LEVEL_2,
	GH:         "https://github.com/yugabyte/yugabyte-db/issues/24843",
	DocsLink:   "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#system-columns-is-not-yet-supported",
}

func NewXminSystemColumnIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	xminSystemColumnIssue.Description = fmt.Sprintf(SYSTEM_COLUMNS_ISSUE_DESCRIPTION, "xmin")
	return newQueryIssue(xminSystemColumnIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var xmaxSystemColumnIssue = issue.Issue{
	Type:       SYSTEM_COLUMN_XMAX,
	Name:       SYSTEM_COLUMN_XMAX_ISSUE_NAME,
	Impact:     constants.IMPACT_LEVEL_2,
	Suggestion: "",
	GH:         "https://github.com/yugabyte/yugabyte-db/issues/24843",
	DocsLink:   "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#system-columns-is-not-yet-supported",
}

func NewXmaxSystemColumnIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	xmaxSystemColumnIssue.Description = fmt.Sprintf(SYSTEM_COLUMNS_ISSUE_DESCRIPTION, "xmax")
	return newQueryIssue(xmaxSystemColumnIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var cminSystemColumnIssue = issue.Issue{
	Type:       SYSTEM_COLUMN_CMIN,
	Name:       SYSTEM_COLUMN_CMIN_ISSUE_NAME,
	Impact:     constants.IMPACT_LEVEL_2,
	Suggestion: "",
	GH:         "https://github.com/yugabyte/yugabyte-db/issues/24843",
	DocsLink:   "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#system-columns-is-not-yet-supported",
}

func NewCminSystemColumnIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	cminSystemColumnIssue.Description = fmt.Sprintf(SYSTEM_COLUMNS_ISSUE_DESCRIPTION, "cmin")
	return newQueryIssue(cminSystemColumnIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var cmaxSystemColumnIssue = issue.Issue{
	Type:       SYSTEM_COLUMN_CMAX,
	Name:       SYSTEM_COLUMN_CMAX_ISSUE_NAME,
	Impact:     constants.IMPACT_LEVEL_2,
	Suggestion: "",
	GH:         "https://github.com/yugabyte/yugabyte-db/issues/24843",
	DocsLink:   "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#system-columns-is-not-yet-supported",
}

func NewCmaxSystemColumnIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	cmaxSystemColumnIssue.Description = fmt.Sprintf(SYSTEM_COLUMNS_ISSUE_DESCRIPTION, "cmax")
	return newQueryIssue(cmaxSystemColumnIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var ctidSystemColumnIssue = issue.Issue{
	Type:       SYSTEM_COLUMN_CTID,
	Name:       SYSTEM_COLUMN_CTID_ISSUE_NAME,
	Impact:     constants.IMPACT_LEVEL_2,
	Suggestion: "",
	GH:         "https://github.com/yugabyte/yugabyte-db/issues/24843",
	DocsLink:   "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#system-columns-is-not-yet-supported",
}

func NewCtidSystemColumnIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	ctidSystemColumnIssue.Description = fmt.Sprintf(SYSTEM_COLUMNS_ISSUE_DESCRIPTION, "ctid")
	return newQueryIssue(ctidSystemColumnIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

// -------------------------------------------------------------------------------------------------------------

var xmlFunctionsIssue = issue.Issue{
	Type:        XML_FUNCTIONS,
	Name:        XML_FUNCTIONS_ISSUE_NAME,
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
	Name:        REGEX_FUNCTIONS_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_2,
	Description: REGEX_FUNCTIONS_ISSUE_DESCRIPTION,
	Suggestion:  "",
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#postgresql-12-and-later-features",
	MinimumVersionsFixedIn: map[string]*ybversion.YBVersion{
		ybversion.SERIES_2_25: ybversion.V2_25_0_0,
	},
}

func NewRegexFunctionsIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(regexFunctionsIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var anyValueAggregateFunction = issue.Issue{
	Type:        ANY_VALUE_AGGREGATE_FUNCTION,
	Name:        ANY_VALUE_AGGREGATE_FUNCTION_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_2,
	Description: ANY_VALUE_AGGREGATE_FUNCTION_ISSUE_DESCRIPTION,
	Suggestion:  "",
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#postgresql-12-and-later-features",
}

func NewAnyValueAggregateFunctionIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(anyValueAggregateFunction, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var rangeAggregateFunctionIssue = issue.Issue{
	Type:        RANGE_AGGREGATE_FUNCTION,
	Name:        RANGE_AGGREGATE_FUNCTION_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_2,
	Description: RANGE_AGGREGATE_FUNCTION_ISSUE_DESCRIPTION,
	Suggestion:  "",
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#postgresql-12-and-later-features",
	MinimumVersionsFixedIn: map[string]*ybversion.YBVersion{
		ybversion.SERIES_2_25: ybversion.V2_25_0_0,
	},
}

func NewRangeAggregateFunctionIssue(objectType string, objectName string, sqlStatement string, funcNames []string) QueryIssue {
	sort.Strings(funcNames)
	details := map[string]interface{}{
		FUNCTION_NAMES: funcNames, //TODO USE it later when we start putting these in reports
	}
	return newQueryIssue(rangeAggregateFunctionIssue, objectType, objectName, sqlStatement, details)
}

var jsonConstructorFunctionsIssue = issue.Issue{
	Type:        JSON_CONSTRUCTOR_FUNCTION,
	Name:        JSON_CONSTRUCTOR_FUNCTION_ISSUE_NAME,
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
	Name:        JSON_QUERY_FUNCTIONS_ISSUE_NAME,
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
	Name:        LARGE_OBJECT_FUNCTIONS_ISSUE_NAME,
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
	Name:        JSONB_SUBSCRIPTING_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_2,
	Description: JSONB_SUBSCRIPTING_ISSUE_DESCRIPTION,
	Suggestion:  "Use Arrow operators (-> / ->>) to access the jsonb fields.",
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#jsonb-subscripting",
	MinimumVersionsFixedIn: map[string]*ybversion.YBVersion{
		ybversion.SERIES_2_25: ybversion.V2_25_0_0,
	},
}

func NewJsonbSubscriptingIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(jsonbSubscriptingIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var jsonPredicateIssue = issue.Issue{
	Type:        JSON_TYPE_PREDICATE,
	Name:        JSON_TYPE_PREDICATE_ISSUE_NAME,
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
	Name:        COPY_FROM_WHERE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_2,
	Description: COPY_FROM_WHERE_ISSUE_DESCRIPTION,
	Suggestion:  "",
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#postgresql-12-and-later-features",
	MinimumVersionsFixedIn: map[string]*ybversion.YBVersion{
		ybversion.SERIES_2_25: ybversion.V2_25_0_0,
	},
}

func NewCopyFromWhereIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(copyFromWhereIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var copyOnErrorIssue = issue.Issue{
	Type:        COPY_ON_ERROR,
	Name:        COPY_ON_ERROR_ISSUE_NAME,
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
	Name:        FETCH_WITH_TIES_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_2,
	Description: FETCH_WITH_TIES_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#postgresql-12-and-later-features",
}

func NewFetchWithTiesIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(fetchWithTiesIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var cteWithMaterializedIssue = issue.Issue{
	Type:        CTE_WITH_MATERIALIZED_CLAUSE,
	Name:        CTE_WITH_MATERIALIZED_CLAUSE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_2,
	Description: "Modifying the materialization of CTE is not supported yet in YugabyteDB.",
	Suggestion:  "No workaround available right now",
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#postgresql-12-and-later-features",
	MinimumVersionsFixedIn: map[string]*ybversion.YBVersion{
		ybversion.SERIES_2_25: ybversion.V2_25_0_0, //TODO: understand in NOT MATERIALIZED works as expected internally
	},
}

func NewCTEWithMaterializedIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(cteWithMaterializedIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var mergeStatementIssue = issue.Issue{
	Type:        MERGE_STATEMENT,
	Name:        MERGE_STATEMENT_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_2,
	Description: MERGE_STATEMENT_ISSUE_DESCRIPTION,
	Suggestion:  "Use PL/pgSQL to write the logic to get this functionality.",
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25574",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#merge-command",
}

func NewMergeStatementIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	//MERGE STATEMENT is PG15 feature but  MERGE .... RETURNING clause is PG17 feature so need to report it separately later.
	return newQueryIssue(mergeStatementIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var listenNotifyIssue = issue.Issue{
	Type:        LISTEN_NOTIFY,
	Name:        LISTEN_NOTIFY_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_2, //TODO: confirm impact
	Description: "LISTEN / NOTIFY is not supported yet in YugabyteDB.",
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/1872",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#events-listen-notify",
}

func NewListenNotifyIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(listenNotifyIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var nonDecimalIntegerLiteralIssue = issue.Issue{
	Type:        NON_DECIMAL_INTEGER_LITERAL,
	Name:        NON_DECIMAL_INTEGER_LITERAL_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_2,
	Description: "Non decimal integer literals are not supported in YugabyteDB",
	Suggestion:  "",
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/25575",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#postgresql-12-and-later-features",
}

func NewNonDecimalIntegerLiteralIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(nonDecimalIntegerLiteralIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var twoPhaseCommitIssue = issue.Issue{
	Type:        TWO_PHASE_COMMIT,
	Name:        TWO_PHASE_COMMIT_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_3,
	Description: "Tow-Phase Commit is not supported yet in YugabyteDB.",
	Suggestion:  "",
	GH:          "https://github.com/yugabyte/yugabyte-db/issues/11084",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#two-phase-commit",
}

func NewTwoPhaseCommitIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(twoPhaseCommitIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}
