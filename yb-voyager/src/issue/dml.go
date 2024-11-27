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

import "github.com/yugabyte/yb-voyager/yb-voyager/src/version"

var advisoryLocksIssue = Issue{
	Type:            ADVISORY_LOCKS,
	TypeName:        "Advisory Locks",
	TypeDescription: "",
	Suggestion:      "",
	GH:              "",
	DocsLink:        "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#advisory-locks-is-not-yet-implemented",
}

func NewAdvisoryLocksIssue(objectType string, objectName string, sqlStatement string) IssueInstance {
	return newIssueInstance(advisoryLocksIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var systemColumnsIssue = Issue{
	Type:            SYSTEM_COLUMNS,
	TypeName:        "System Columns",
	TypeDescription: "",
	Suggestion:      "",
	GH:              "",
	DocsLink:        "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#system-columns-is-not-yet-supported",
}

func NewSystemColumnsIssue(objectType string, objectName string, sqlStatement string) IssueInstance {
	return newIssueInstance(systemColumnsIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

var xmlFunctionsIssue = Issue{
	Type:            XML_FUNCTIONS,
	TypeName:        "XML Functions",
	TypeDescription: "",
	Suggestion:      "",
	GH:              "",
	DocsLink:        "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#xml-functions-is-not-yet-supported",
	MinimumVersionsFixedIn: map[string]*version.YBVersion{
		version.SERIES_2024_1: version.V2024_1_4_0,
		version.SERIES_2_23:   version.V2_23_5_0,
	},
}

func NewXmlFunctionsIssue(objectType string, objectName string, sqlStatement string) IssueInstance {
	return newIssueInstance(xmlFunctionsIssue, objectType, objectName, sqlStatement, map[string]interface{}{})
}
