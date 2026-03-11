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
package sqlname

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
)

func TestSplitQualifiedNameSimpleCases(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"schema.table", []string{"schema", "table"}},
		{"table", []string{"table"}},
		{`"schema"."table"`, []string{`"schema"`, `"table"`}},
		{`"public"."my_table"`, []string{`"public"`, `"my_table"`}},
		{"`schema`.`table`", []string{"`schema`", "`table`"}},
	}

	for _, tt := range tests {
		parts, err := SplitQualifiedName(tt.input)
		require.NoError(t, err, "input: %s", tt.input)
		assert.Equal(t, tt.expected, parts, "input: %s", tt.input)
	}
}

func TestSplitQualifiedNameDotsInQuotedIdentifiers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "dot in quoted table name",
			input:    `"public"."my.table"`,
			expected: []string{`"public"`, `"my.table"`},
		},
		{
			name:     "dot in quoted schema name",
			input:    `"my.schema"."table1"`,
			expected: []string{`"my.schema"`, `"table1"`},
		},
		{
			name:     "dots in both schema and table",
			input:    `"my.schema"."my.table"`,
			expected: []string{`"my.schema"`, `"my.table"`},
		},
		{
			name:     "dot in backtick-quoted name (MySQL style)",
			input:    "`my.db`.`table1`",
			expected: []string{"`my.db`", "`table1`"},
		},
		{
			name:     "multiple dots in quoted table name",
			input:    `"schema"."a.b.c"`,
			expected: []string{`"schema"`, `"a.b.c"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts, err := SplitQualifiedName(tt.input)
			require.NoError(t, err, "input: %s", tt.input)
			assert.Equal(t, tt.expected, parts, "input: %s", tt.input)
		})
	}
}

func TestSplitQualifiedNameErrorCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"unterminated double quote", `"schema`},
		{"unterminated backtick", "`schema"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SplitQualifiedName(tt.input)
			assert.Error(t, err, "input: %s", tt.input)
		})
	}
}

func TestSplitQualifiedNameMixedQuoting(t *testing.T) {
	parts, err := SplitQualifiedName(`"schema".table`)
	require.NoError(t, err)
	assert.Equal(t, []string{`"schema"`, "table"}, parts)

	parts, err = SplitQualifiedName(`schema."table"`)
	require.NoError(t, err)
	assert.Equal(t, []string{"schema", `"table"`}, parts)
}

func TestNewObjectNameWithQualifiedNameDotsInName(t *testing.T) {
	SourceDBType = constants.POSTGRESQL

	obj := NewObjectNameWithQualifiedName(constants.POSTGRESQL, "public", `"public"."my.table"`)
	assert.Equal(t, "my.table", obj.Unqualified.Unquoted)
	assert.Equal(t, "public", obj.SchemaName.Unquoted)
}

func TestNewSourceNameFromQualifiedNameDotsInName(t *testing.T) {
	SourceDBType = constants.POSTGRESQL

	sn := NewSourceNameFromQualifiedName(`"public"."my.table"`)
	assert.Equal(t, "my.table", sn.ObjectName.Unquoted)
	assert.Equal(t, "public", sn.SchemaName.Unquoted)
}

func TestNewObjectNameWithQualifiedNameMySQLDotInDB(t *testing.T) {
	SourceDBType = constants.MYSQL

	obj := NewObjectNameWithQualifiedName(constants.MYSQL, "my.db", `"my.db"."table1"`)
	assert.Equal(t, "table1", obj.Unqualified.Unquoted)
	assert.Equal(t, "my.db", obj.SchemaName.Unquoted)
}

func TestMatchesPatternWithDotsInQuotedName(t *testing.T) {
	SourceDBType = constants.POSTGRESQL

	obj := NewObjectName(constants.POSTGRESQL, "public", "public", "my.table")
	require.NotNil(t, obj)

	match, err := obj.MatchesPattern(`public."my.table"`)
	require.NoError(t, err)
	assert.True(t, match)

	match, err = obj.MatchesPattern(`"public"."my.table"`)
	require.NoError(t, err)
	assert.True(t, match)
}
