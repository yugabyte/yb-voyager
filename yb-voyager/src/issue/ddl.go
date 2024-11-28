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
