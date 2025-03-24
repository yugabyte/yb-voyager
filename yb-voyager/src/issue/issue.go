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
	// Type acts as ID for the issue; should be unique across all issues
	// for example: UNSUPPORTED_DATATYPE_PG_LSN, UNLOGGED_TABLES, etc
	// This field is not to be changed frequently; if changed, be careful about impact on callhome and yugabyted
	Type string

	// readable name for the issue; used in UI, logs or any print statements
	Name                   string
	Description            string
	Impact                 string
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

/*
	Dynamic Impact Determination (TODO)
	- We can define the impact calculator function based on issue type wherever/whenever needed
	- Map will have functions only for issue type with dynamic impact determination

	For example:

	type ImpactCalcFunc func(issue QueryIssue, stats *PgStats) string

	var impactCalculators = map[string]ImpactCalcFunc{
		INHERITED_TABLE: inheritedTableImpactCalc,
		// etc...
	}

	// Example dynamic function
	func inheritedTableImpactCalc(i QueryIssue, stats *PgStats) string {
		usage := stats.GetUsage(i.ObjectName) // e.g. how many reads/writes
		if usage.WritesPerDay > 1000 {
			return "LEVEL_2"
		}
		return "LEVEL_3"
	}

	func (i Issue) GetImpact(stats *PgStats) string {
		if calc, ok := impactCalculators[i.Type]; ok {
			return calc(i, stats)
		}

		return lo.Ternary(i.Impact != "", i.Impact, constants.IMPACT_LEVEL_1)
	}

*/
