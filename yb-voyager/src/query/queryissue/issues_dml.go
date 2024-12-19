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
)

var advisoryLocksIssue = issue.Issue{
	Type:            ADVISORY_LOCKS,
	TypeName:        "Advisory Locks",
	TypeDescription: "",
	Suggestion:      "",
	GH:              "",
	DocsLink:        "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#advisory-locks-is-not-yet-implemented",
}

func NewAdvisoryLocksIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(advisoryLocksIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var systemColumnsIssue = issue.Issue{
	Type:            SYSTEM_COLUMNS,
	TypeName:        "System Columns",
	TypeDescription: "",
	Suggestion:      "",
	GH:              "",
	DocsLink:        "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#system-columns-is-not-yet-supported",
}

func NewSystemColumnsIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(systemColumnsIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var xmlFunctionsIssue = issue.Issue{
	Type:            XML_FUNCTIONS,
	TypeName:        "XML Functions",
	TypeDescription: "",
	Suggestion:      "",
	GH:              "",
	DocsLink:        "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#xml-functions-is-not-yet-supported",
}

func NewXmlFunctionsIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(xmlFunctionsIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var anyValueAggFunctionIssue = issue.Issue{
	Type: AGGREGATE_FUNCTION,
	TypeName: "ANY_VALUE() aggregate function is not supported",
	TypeDescription: "",
	Suggestion: "",
	GH: "",
	DocsLink: "",
}

func NewAnyValueAGGFunctionIssue(objectType string, objectName string, sqlStatement string) QueryIssue {
	return newQueryIssue(anyValueAggFunctionIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var jsonConstructorFunctionsIssue = issue.Issue{
	Type: JSON_CONSTRUCTOR_FUNCTION,
	TypeName: "%s() function is not supported",
	TypeDescription: "",
	Suggestion: "",
	GH: "",
	DocsLink: "",
}

func NewJsonConstructorFunctionIssue(objectType string, objectName string, sqlStatement string, jsonConstructorFnName string) QueryIssue {
	issue := jsonConstructorFunctionsIssue
	issue.TypeName = fmt.Sprintf(issue.TypeName, strings.ToUpper(jsonConstructorFnName))
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var jsonQueryFunctionIssue = issue.Issue{
	Type: JSON_QUERY_FUNCTION,
	TypeName: "%s() function is not supported",
	TypeDescription: "",
	Suggestion: "",
	GH: "",
	DocsLink: "",
}

func NewJsonQueryFunction(objectType string, objectName string, sqlStatement string, jsonQueryFnName string) QueryIssue {
	issue := jsonQueryFunctionIssue
	issue.TypeName = fmt.Sprintf(issue.TypeName, strings.ToUpper(jsonQueryFnName))
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}