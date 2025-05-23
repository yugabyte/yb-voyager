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

package ybversion

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidNewYBVersion(t *testing.T) {
	validVersionStrings := []string{
		"2024.1.1.0",
		"2.20.7.0",
		"2.21.2.1",
	}
	for _, v := range validVersionStrings {
		_, err := NewYBVersion(v)
		assert.NoError(t, err)
	}
}

func TestInvalidNewYBVersion(t *testing.T) {
	invalidVersionStrings := []string{
		"abc.def",         // has to be numbers
		"2024.0.1-1",      // has to be in supported series
		"2024",            // has to have 4 segments
		"2.20.7",          // has to have 4 segments
		"2024.1.1.1.1.1",  // exactly 4 segments
		"2024.1.0.0-b123", // build number is not allowed.
	}
	for _, v := range invalidVersionStrings {
		_, err := NewYBVersion(v)
		assert.Errorf(t, err, "expected error for %q", v)
	}
}

func TestStableReleaseType(t *testing.T) {
	stableVersionStrings := []string{
		"2024.1.1.0",
		"2024.1.0.0",
		"2024.1.1.1",
		"2024.2.2.3",
	}
	for _, v := range stableVersionStrings {
		ybVersion, _ := NewYBVersion(v)
		assert.Equal(t, STABLE, ybVersion.ReleaseType())
	}
}

func TestPreviewReleaseType(t *testing.T) {
	previewVersionStrings := []string{
		"2.21.1.0",
		"2.21.1.1",
		"2.25.2.0",
	}
	for _, v := range previewVersionStrings {
		ybVersion, _ := NewYBVersion(v)
		assert.Equal(t, PREVIEW, ybVersion.ReleaseType())
	}
}

func TestStableOldReleaseType(t *testing.T) {
	stableOldVersionStrings := []string{
		"2.20.1.0",
		"2.20.0.0",
	}
	for _, v := range stableOldVersionStrings {
		ybVersion, _ := NewYBVersion(v)
		assert.Equal(t, STABLE_OLD, ybVersion.ReleaseType())
	}
}
