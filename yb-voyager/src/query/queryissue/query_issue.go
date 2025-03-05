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

/*
This package has all logic related to detecting issues in queries (DDL or DML).
Entry point is ParserIssueDetector, which makes use queryparser pkg to parse
the query and multiple detectors to figure out issues in the parseTree.
*/
package queryissue

import (
	"github.com/yugabyte/yb-voyager/yb-voyager/src/issue"
)

type QueryIssue struct {
	issue.Issue
	ObjectType   string // TABLE, FUNCTION, DML_QUERY?
	ObjectName   string // table name/function name/etc
	SqlStatement string
	Details      map[string]interface{} // additional details about the issue
}

func newQueryIssue(issue issue.Issue, objectType string, objectName string, sqlStatement string, details map[string]interface{}) QueryIssue {
	return QueryIssue{
		Issue:        issue,
		ObjectType:   objectType,
		ObjectName:   objectName,
		SqlStatement: sqlStatement,
		Details:      details,
	}
}
