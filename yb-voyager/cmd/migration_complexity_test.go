//go:build unit

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
package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
)

func TestGetComplexityForLevel(t *testing.T) {
	testCases := []struct {
		level    string
		count    int
		expected string
		desc     string
	}{
		// -------------------------------
		// LEVEL_1 test cases
		// -------------------------------
		{
			level:    constants.IMPACT_LEVEL_1,
			count:    0,
			expected: constants.MIGRATION_COMPLEXITY_LOW,
			desc:     "L1, count=0 => LOW",
		},
		{
			level:    constants.IMPACT_LEVEL_1,
			count:    20,
			expected: constants.MIGRATION_COMPLEXITY_LOW,
			desc:     "L1, count=20 => LOW",
		},
		{
			level:    constants.IMPACT_LEVEL_1,
			count:    21,
			expected: constants.MIGRATION_COMPLEXITY_MEDIUM,
			desc:     "L1, count=21 => MEDIUM",
		},
		{
			level:    constants.IMPACT_LEVEL_1,
			count:    999999999,
			expected: constants.MIGRATION_COMPLEXITY_MEDIUM,
			desc:     "L1, big count => MEDIUM",
		},

		// -------------------------------
		// LEVEL_2 test cases
		// -------------------------------
		{
			level:    constants.IMPACT_LEVEL_2,
			count:    0,
			expected: constants.MIGRATION_COMPLEXITY_LOW,
			desc:     "L2, count=0 => LOW",
		},
		{
			level:    constants.IMPACT_LEVEL_2,
			count:    10,
			expected: constants.MIGRATION_COMPLEXITY_LOW,
			desc:     "L2, count=10 => LOW",
		},
		{
			level:    constants.IMPACT_LEVEL_2,
			count:    11,
			expected: constants.MIGRATION_COMPLEXITY_MEDIUM,
			desc:     "L2, count=11 => MEDIUM",
		},
		{
			level:    constants.IMPACT_LEVEL_2,
			count:    100,
			expected: constants.MIGRATION_COMPLEXITY_MEDIUM,
			desc:     "L2, count=100 => MEDIUM",
		},
		{
			level:    constants.IMPACT_LEVEL_2,
			count:    101,
			expected: constants.MIGRATION_COMPLEXITY_HIGH,
			desc:     "L2, count=101 => HIGH",
		},

		// -------------------------------
		// LEVEL_3 test cases
		// -------------------------------
		{
			level:    constants.IMPACT_LEVEL_3,
			count:    0,
			expected: constants.MIGRATION_COMPLEXITY_LOW,
			desc:     "L3, count=0 => LOW",
		},
		{
			level:    constants.IMPACT_LEVEL_3,
			count:    1,
			expected: constants.MIGRATION_COMPLEXITY_MEDIUM,
			desc:     "L3, count=1 => MEDIUM",
		},
		{
			level:    constants.IMPACT_LEVEL_3,
			count:    4,
			expected: constants.MIGRATION_COMPLEXITY_MEDIUM,
			desc:     "L3, count=4 => MEDIUM",
		},
		{
			level:    constants.IMPACT_LEVEL_3,
			count:    5,
			expected: constants.MIGRATION_COMPLEXITY_HIGH,
			desc:     "L3, count=5 => HIGH",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			actual := getComplexityForLevel(tc.level, tc.count)
			assert.Equal(t, tc.expected, actual,
				"Level=%s, Count=%d => Expected %s, Got %s",
				tc.level, tc.count, tc.expected, actual,
			)
		})
	}
}
