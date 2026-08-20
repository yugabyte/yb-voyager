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
	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/ybversion"
)

type Issue struct {
	// Type acts as ID for the issue; should be unique across all issues
	// for example: UNSUPPORTED_DATATYPE_PG_LSN, UNLOGGED_TABLES, etc
	// This field is not to be changed frequently; if changed, be careful about impact on callhome and yugabyted
	Type string

	// readable name for the issue; used in UI, logs or any print statements
	Name                     string
	Description              string
	Impact                   string
	Suggestion               string
	GH                       string
	DocsLink                 string
	MinimumVersionsFixedIn   map[string]*ybversion.YBVersion // key: series; GA
	MinimumVersionsFixedInTP map[string]*ybversion.YBVersion // key: series; available as Tech Preview
	MinimumVersionsFixedInEA map[string]*ybversion.YBVersion // key: series; available as Early Access

	// Flags to set in order to enable the feature when it is available only as
	// Tech Preview / Early Access in the target version (e.g. "yb_enable_derived_saops=true").
	EnablingFlags []string
}

func isFixedInVersionMap(versionsFixedIn map[string]*ybversion.YBVersion, v *ybversion.YBVersion) (bool, error) {
	if versionsFixedIn == nil {
		return false, nil
	}
	minVersionFixedInSeries, ok := versionsFixedIn[v.Series()]
	if !ok {
		return false, nil
	}
	return v.GreaterThanOrEqual(minVersionFixedInSeries), nil
}

func (i Issue) IsFixedIn(v *ybversion.YBVersion) (bool, error) {
	return isFixedInVersionMap(i.MinimumVersionsFixedIn, v)
}

func (i Issue) IsFixedInTP(v *ybversion.YBVersion) (bool, error) {
	return isFixedInVersionMap(i.MinimumVersionsFixedInTP, v)
}

func (i Issue) IsFixedInEA(v *ybversion.YBVersion) (bool, error) {
	return isFixedInVersionMap(i.MinimumVersionsFixedInEA, v)
}

// GetMaturityInTarget returns the maturity of the feature in the given target version:
// GA / EA / TP / UNSUPPORTED. Precedence is GA > EA > TP: if the feature qualifies at
// multiple maturities in the target series, the highest maturity wins.
func (i Issue) GetMaturityInTarget(v *ybversion.YBVersion) (string, error) {
	ga, err := i.IsFixedIn(v)
	if err != nil {
		return "", err
	}
	if ga {
		return constants.MATURITY_GA, nil
	}
	ea, err := i.IsFixedInEA(v)
	if err != nil {
		return "", err
	}
	if ea {
		return constants.MATURITY_EA, nil
	}
	tp, err := i.IsFixedInTP(v)
	if err != nil {
		return "", err
	}
	if tp {
		return constants.MATURITY_TP, nil
	}
	return constants.MATURITY_UNSUPPORTED, nil
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
