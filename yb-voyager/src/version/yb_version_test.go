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

package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidNewYBVersion(t *testing.T) {
	validVersionStrings := []string{
		"2024.1.1",
		"2.20",
		"2.21.2.1",
	}
	for _, v := range validVersionStrings {
		_, err := NewYBVersion(v)
		assert.NoError(t, err)
	}
}

func TestInvalidNewYBVersion(t *testing.T) {
	invalidVersionStrings := []string{
		"abc.def",        // has to be numbers
		"2024.0.1-1",     // has to be in supported series
		"2024",           // has to have at least 2 segments
		"2024.1.1.1.1.1", // max 4 segments
	}
	for _, v := range invalidVersionStrings {
		_, err := NewYBVersion(v)
		assert.Error(t, err)
	}
}

func TestStableReleaseType(t *testing.T) {
	stableVersionStrings := []string{
		"2024.1.1",
		"2024.1",
		"2024.1.1.1",
	}
	for _, v := range stableVersionStrings {
		ybVersion, _ := NewYBVersion(v)
		assert.Equal(t, STABLE, ybVersion.ReleaseType())
	}
}

func TestPreviewReleaseType(t *testing.T) {
	previewVersionStrings := []string{
		"2.21.1",
		"2.21",
	}
	for _, v := range previewVersionStrings {
		ybVersion, _ := NewYBVersion(v)
		assert.Equal(t, PREVIEW, ybVersion.ReleaseType())
	}
}

func TestStableOldReleaseType(t *testing.T) {
	stableOldVersionStrings := []string{
		"2.20.1",
		"2.20",
	}
	for _, v := range stableOldVersionStrings {
		ybVersion, _ := NewYBVersion(v)
		assert.Equal(t, STABLE_OLD, ybVersion.ReleaseType())
	}
}

func TestVersionPrefixGreaterThanOrEqual(t *testing.T) {
	versionsToCompare := [][]string{
		{"2024.1.1", "2024.1.0"},
		{"2024.1", "2024.1.4.0"}, //prefix 2024.1 == 2024.1
	}
	for _, v := range versionsToCompare {
		v1, err := NewYBVersion(v[0])
		assert.NoError(t, err)
		v2, err := NewYBVersion(v[1])
		assert.NoError(t, err)
		greaterThan, err := v1.CommonPrefixGreaterThanOrEqual(v2)
		assert.NoError(t, err)
		assert.True(t, greaterThan, "%s >= %s", v[0], v[1])
	}
}

func TestVersionPrefixLessThan(t *testing.T) {
	versionsToCompare := [][]string{
		{"2024.1.0", "2024.1.2"},
		{"2.21.1", "2.21.7"},
		{"2.20.1", "2.20.7"},
	}
	for _, v := range versionsToCompare {
		v1, err := NewYBVersion(v[0])
		assert.NoError(t, err)
		v2, err := NewYBVersion(v[1])
		assert.NoError(t, err)
		lessThan, err := v1.CommonPrefixLessThan(v2)
		assert.NoError(t, err)
		assert.True(t, lessThan, "%s < %s", v[0], v[1])
	}
}

func TestComparingDifferentSeriesThrowsError(t *testing.T) {
	v1, _ := NewYBVersion("2024.1.1")
	v2, _ := NewYBVersion("2.21.1")
	_, err := v1.CompareCommonPrefix(v2)
	assert.Error(t, err)
}
