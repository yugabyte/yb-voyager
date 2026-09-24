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

package versions

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================== Connector Versions Tests ===============================

func TestLoadConnectorVersions(t *testing.T) {
	cv := LoadConnectorVersions()
	assert.NotEmpty(t, cv.LogicalConnector.Tag, "logical connector tag must not be empty")
	assert.NotEmpty(t, cv.GRPCConnector.Tag, "grpc connector tag must not be empty")
}

func TestGetLogicalConnectorTag(t *testing.T) {
	tag := GetLogicalConnectorTag()
	assert.NotEmpty(t, tag, "logical connector tag must not be empty")
	assert.Regexp(t, regexp.MustCompile(`^dz\..*\.yb\.`), tag,
		"logical connector tag %q must start with 'dz.' and contain '.yb.'", tag)
}

func TestGetGRPCConnectorTag(t *testing.T) {
	tag := GetGRPCConnectorTag()
	assert.NotEmpty(t, tag, "gRPC connector tag must not be empty")
	assert.Regexp(t, regexp.MustCompile(`\.yb\.grpc\.`), tag,
		"gRPC connector tag %q must contain '.yb.grpc.'", tag)
}

func TestGetLatestStableYBVersionWithoutBuildNumber(t *testing.T) {
	// Test the actual function with current data
	fullVersion := GetLatestStableYBVersion()
	baseVersion := GetLatestStableYBVersionWithoutBuildNumber()

	// Verify that base version is a prefix of full version
	assert.True(t, strings.HasPrefix(fullVersion, baseVersion),
		"Base version %s should be a prefix of full version %s", baseVersion, fullVersion)

	// Verify that base version doesn't contain build number
	assert.False(t, strings.Contains(baseVersion, "-b"),
		"Base version %s should not contain build number (dash)", baseVersion)
}
