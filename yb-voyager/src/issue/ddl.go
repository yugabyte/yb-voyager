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

var percentTypeSyntax = Issue{
	Type:     REFERENCED_TYPE_DECLARATION,
	TypeName: "Referenced type declaration of variables",
	TypeDescription: "",
	Suggestion: "Fix the syntax to include the actual type name instead of referencing the type of a column",
	GH: "https://github.com/yugabyte/yugabyte-db/issues/23619",
	DocsLink: "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#type-syntax-is-not-supported",
}

func NewPercentTypeSyntaxIssue(objectType string, objectName string, sqlStatement string) IssueInstance {
	return newIssueInstance(percentTypeSyntax, objectType, objectName, sqlStatement, map[string]interface{}{})
}

