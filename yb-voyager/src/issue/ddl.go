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

var generatedColumnsIssue = Issue{
	Type:       STORED_GENERATED_COLUMNS,
	TypeName:   "Stored generated columns are not supported.",
	GH:         "https://github.com/yugabyte/yugabyte-db/issues/10695",
	Suggestion: "Using Triggers to update the generated columns is one way to work around this issue, refer docs link for more details.",
	DocsLink:   "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#generated-always-as-stored-type-column-is-not-supported",
}

func NewGeneratedColumnsIssue(objectType string, objectName string, sqlStatement string, details map[string]interface{}) IssueInstance {
	return newIssueInstance(generatedColumnsIssue, objectType, objectName, sqlStatement, details)
}
