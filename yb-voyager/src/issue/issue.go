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

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/version"
)

type Issue struct {
	Type                         string // (advisory_locks, index_not_supported, etc)
	TypeName                     string // for display
	TypeDescription              string
	Suggestion                   string
	GH                           string
	DocsLink                     string
	MinimumFixedVersionStable    *version.YBVersion // should be fully specified A.B.C.D (4 segments)
	MinimumFixedVersionPreview   *version.YBVersion // should be fully specified A.B.C.D (4 segments)
	MinimumFixedVersionStableOld *version.YBVersion // should be fully specified A.B.C.D (4 segments)
}

func (i Issue) IsFixedIn(v *version.YBVersion) (bool, error) {
	switch v.ReleaseType() {
	case version.STABLE:
		if i.MinimumFixedVersionStable == nil {
			return false, nil
		}
		greaterThanMin, err := v.CommonPrefixGreaterThanOrEqual(i.MinimumFixedVersionStable)
		if err != nil {
			return false, fmt.Errorf("comparing versions %s and %s: %w", v, i.MinimumFixedVersionStable, err)
		}
		return greaterThanMin, nil
	case version.PREVIEW:
		if i.MinimumFixedVersionPreview == nil {
			return false, nil
		}
		greaterThanMin, err := v.CommonPrefixGreaterThanOrEqual(i.MinimumFixedVersionPreview)
		if err != nil {
			return false, fmt.Errorf("comparing versions %s and %s: %w", v, i.MinimumFixedVersionPreview, err)
		}
		return greaterThanMin, nil
	case version.STABLE_OLD:
		if i.MinimumFixedVersionStableOld == nil {
			return false, nil
		}
		greaterThanMin, err := v.CommonPrefixGreaterThanOrEqual(i.MinimumFixedVersionStableOld)
		if err != nil {
			return false, fmt.Errorf("comparing versions %s and %s: %w", v, i.MinimumFixedVersionStableOld, err)
		}
		return greaterThanMin, nil
	default:
		return false, fmt.Errorf("unsupported release type: %s", v.ReleaseType())
	}
}

type IssueInstance struct {
	Issue
	ObjectType   string // TABLE, FUNCTION, DML_QUERY?
	ObjectName   string // table name/function name/etc
	SqlStatement string
	Details      map[string]interface{} // additional details about the issue
}

func newIssueInstance(issue Issue, objectType string, objectName string, sqlStatement string, details map[string]interface{}) IssueInstance {
	// We want the full version to be specified in issues.
	// Consider this example:
	// Actual fixed version = 2024.1.4.2
	// Specified MinimumFixedVersionStable = 2024.1.4 // this is what we want to avoid.
	// IsFixedIn("2024.1.4.1") should return false ideally, but it will return true in this case
	// becaus we will only compare the common prefix 2024.1.4 and ignore the rest.

	// Ideally we should have the validations done at init time,
	// but doing that for all the issues defined as variables is not possible to do in golang
	// when all of them are declared as simple variables.
	// We would have to list all the issues in a slice/map and loop over them to validate,
	// which is not required/fool-proof (someone might just declare an issue outside of that list).
	// So, we will do the validations here assuming that issue creation is tested somewhere via unit tests.
	if issue.MinimumFixedVersionPreview != nil {
		if issue.MinimumFixedVersionPreview.OriginalSegmentsLen() != 4 {
			utils.ErrExit("ERROR: MinimumFixedVersionPreview in %v must have 4 segments", issue)
		}
	}
	if issue.MinimumFixedVersionStable != nil {
		if issue.MinimumFixedVersionStable.OriginalSegmentsLen() != 4 {
			utils.ErrExit("ERROR: MinimumFixedVersionStable in %v must have 4 segments", issue)
		}
	}
	if issue.MinimumFixedVersionStableOld != nil {
		if issue.MinimumFixedVersionStableOld.OriginalSegmentsLen() != 4 {
			utils.ErrExit("ERROR: MinimumFixedVersionStableOld in %v must have 4 segments", issue)
		}
	}

	return IssueInstance{
		Issue:        issue,
		ObjectType:   objectType,
		ObjectName:   objectName,
		SqlStatement: sqlStatement,
		Details:      details,
	}
}
