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
	"github.com/yugabyte/yb-voyager/yb-voyager/src/ybversion"
)

type Issue struct {
	Type                   string // (advisory_locks, index_not_supported, etc)
	TypeName               string // for display
	TypeDescription        string
	Suggestion             string
	GH                     string
	DocsLink               string
	MinimumVersionsFixedIn map[string]*ybversion.YBVersion // key: series (2024.1, 2.21, etc)
}

func (i Issue) IsFixedIn(v *ybversion.YBVersion) (bool, error) {
	if i.MinimumVersionsFixedIn == nil {
		return false, nil
	}
	minVersionFixedInSeries, ok := i.MinimumVersionsFixedIn[v.Series()]
	if !ok {
		return false, nil
	}
	return v.GreaterThanOrEqual(minVersionFixedInSeries), nil
}

type IssueInstance struct {
	Issue
	ObjectType   string // TABLE, FUNCTION, DML_QUERY?
	ObjectName   string // table name/function name/etc
	SqlStatement string
	Details      map[string]interface{} // additional details about the issue
}

func newIssueInstance(issue Issue, objectType string, objectName string, sqlStatement string, details map[string]interface{}) IssueInstance {
	return IssueInstance{
		Issue:        issue,
		ObjectType:   objectType,
		ObjectName:   objectName,
		SqlStatement: sqlStatement,
		Details:      details,
	}
}
