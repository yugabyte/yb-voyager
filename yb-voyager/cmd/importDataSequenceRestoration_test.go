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

func TestQuoteIdentifierIfUnquoted(t *testing.T) {
	tests := []struct {
		name       string
		identifier string
		expected   string
	}{
		{
			name:       "unquoted lower case column is quoted",
			identifier: "id",
			expected:   `"id"`,
		},
		{
			name:       "unquoted mixed case column keeps its case once quoted",
			identifier: "Id",
			expected:   `"Id"`,
		},
		{
			name:       "already quoted column is left as is",
			identifier: `"Id"`,
			expected:   `"Id"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, quoteIdentifierIfUnquoted(test.identifier))
		})
	}
}
