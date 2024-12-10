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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/ybversion"
)

func TestIssueFixedInStable(t *testing.T) {
	fixedVersion, err := ybversion.NewYBVersion("2024.1.1.0")
	assert.NoError(t, err)
	issue := Issue{
		Type: "ADVISORY_LOCKS",
		MinimumVersionsFixedIn: map[string]*ybversion.YBVersion{
			ybversion.SERIES_2024_1: fixedVersion,
		},
	}

	versionsToCheck := map[string]bool{
		"2024.1.1.0": true,
		"2024.1.1.1": true,
		"2024.1.0.0": false,
	}
	for v, expected := range versionsToCheck {
		ybVersion, err := ybversion.NewYBVersion(v)
		assert.NoError(t, err)

		fixed, err := issue.IsFixedIn(ybVersion)
		assert.NoError(t, err)
		assert.Equalf(t, expected, fixed, "comparing ybv %s to fixed %s", ybVersion, fixedVersion)
	}
}

func TestIssueFixedInPreview(t *testing.T) {
	fixedVersion, err := ybversion.NewYBVersion("2.21.4.5")
	assert.NoError(t, err)
	issue := Issue{
		Type: "ADVISORY_LOCKS",
		MinimumVersionsFixedIn: map[string]*ybversion.YBVersion{
			ybversion.SERIES_2_21: fixedVersion,
		},
	}

	versionsToCheck := map[string]bool{
		"2.21.4.5": true,
		"2.21.5.5": true,
		"2.21.4.1": false,
	}
	for v, expected := range versionsToCheck {
		ybVersion, err := ybversion.NewYBVersion(v)
		assert.NoError(t, err)

		fixed, err := issue.IsFixedIn(ybVersion)
		assert.NoError(t, err)
		assert.Equalf(t, expected, fixed, "comparing ybv %s to fixed %s", ybVersion, fixedVersion)
	}
}

func TestIssueFixedInStableOld(t *testing.T) {
	fixedVersionStableOld, err := ybversion.NewYBVersion("2.20.7.1")
	assert.NoError(t, err)
	fixedVersionStable, err := ybversion.NewYBVersion("2024.1.1.1")
	assert.NoError(t, err)

	issue := Issue{
		Type: "ADVISORY_LOCKS",
		MinimumVersionsFixedIn: map[string]*ybversion.YBVersion{
			ybversion.SERIES_2024_1: fixedVersionStable,
			ybversion.SERIES_2_20:   fixedVersionStableOld,
		},
	}

	versionsToCheck := map[string]bool{
		"2.20.0.0":   false,
		"2.20.7.0":   false,
		"2.20.7.1":   true,
		"2024.1.1.1": true,
		"2024.1.1.2": true,
		"2024.1.1.0": false,
	}
	for v, expected := range versionsToCheck {
		ybVersion, err := ybversion.NewYBVersion(v)
		assert.NoError(t, err)

		fixed, err := issue.IsFixedIn(ybVersion)
		assert.NoError(t, err)
		assert.Equalf(t, expected, fixed, "comparing ybv %s to fixed [%s, %s]", ybVersion, fixedVersionStableOld, fixedVersionStable)
	}
}

func TestIssueFixedFalseWhenMinimumNotSpecified(t *testing.T) {
	issue := Issue{
		Type: "ADVISORY_LOCKS",
	}

	versionsToCheck := []string{"2024.1.0.0", "2.20.7.4", "2.21.1.1"}

	for _, v := range versionsToCheck {
		ybVersion, err := ybversion.NewYBVersion(v)
		assert.NoError(t, err)

		fixed, err := issue.IsFixedIn(ybVersion)
		assert.NoError(t, err)
		// If the minimum fixed version is not specified, the issue is not fixed in any version.
		assert.Falsef(t, fixed, "comparing ybv %s to fixed should be false", ybVersion)
	}
}
