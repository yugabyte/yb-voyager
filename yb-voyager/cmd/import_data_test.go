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
)

func TestIsDataLine(t *testing.T) {
	assert := assert.New(t)
	testcases := []struct {
		line     string
		expected bool
	}{
		{`SET client_encoding TO 'UTF8';`, false},
		{`set client_encoding to 'UTF8';`, false},
		{`COPY "Foo" ("v") FROM STDIN;`, false},
		{"", false},
		{"\n", false},
		{"\\.\n", false},
		{"\\.", false},
	}
	insideCopyStmt := false
	for _, tc := range testcases {
		assert.Equal(tc.expected, isDataLine(tc.line, "oracle", &insideCopyStmt), "%q", tc.line)
	}
	insideCopyStmt = false
	for i := 3; i < len(testcases); i++ {
		assert.Equal(testcases[i].expected, isDataLine(testcases[i].line, "postgresql", &insideCopyStmt), "%q", testcases[i].line)
	}
}
