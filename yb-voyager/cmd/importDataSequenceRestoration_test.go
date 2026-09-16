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
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

func TestQuoteIdentifierIfUnquoted(t *testing.T) {
	tests := []struct {
		name       string
		identifier string
		expected   string
	}{
		{"unquoted lower case", "id", `"id"`},
		{"unquoted mixed case keeps its case", "Id", `"Id"`},
		{"already quoted is left alone", `"Id"`, `"Id"`},
		{"already quoted lower case is left alone", `"id"`, `"id"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, quoteIdentifierIfUnquoted(test.identifier))
		})
	}
}

func TestParseSequenceColumnMax(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		expected  int64
		isInteger bool
	}{
		{"positive", "42", 42, true},
		{"zero from empty table", "0", 0, true},
		{"negative", "-7", -7, true},
		{"max int64", "9223372036854775807", 9223372036854775807, true},
		{"surrounding whitespace", "  42\n", 42, true},
		{"text column", "abc", 0, false},
		{"numeric column with scale", "42.5", 0, false},
		{"empty", "", 0, false},
		{"overflows int64", "92233720368547758070", 0, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, isInteger := parseSequenceColumnMax(test.raw)
			assert.Equal(t, test.isInteger, isInteger)
			assert.Equal(t, test.expected, value)
		})
	}
}

func testNameTuple(t *testing.T, schemaName, tableName string) sqlname.NameTuple {
	t.Helper()
	objectName := sqlname.NewObjectName(POSTGRESQL, schemaName, schemaName, tableName)
	return sqlname.NameTuple{CurrentName: objectName, SourceName: objectName, TargetName: objectName}
}

// stubSequenceColumnMaxQuerier swaps the querier for the duration of the test and
// records the queries it was asked to run.
func stubSequenceColumnMaxQuerier(t *testing.T, respond func(query string) (string, error)) *[]string {
	t.Helper()
	original := sequenceColumnMaxQuerier
	t.Cleanup(func() { sequenceColumnMaxQuerier = original })

	var queries []string
	sequenceColumnMaxQuerier = func(query string) (string, error) {
		queries = append(queries, query)
		return respond(query)
	}
	return &queries
}

func TestMaxValueAcrossSequenceColumns(t *testing.T) {
	sequence := testNameTuple(t, "public", "t_id_seq")

	t.Run("takes the maximum across every column the sequence feeds", func(t *testing.T) {
		maxByTable := map[string]string{"parent": "10", "child": "37"}
		stubSequenceColumnMaxQuerier(t, func(query string) (string, error) {
			for table, value := range maxByTable {
				if strings.Contains(query, table) {
					return value, nil
				}
			}
			return "", fmt.Errorf("unexpected query %q", query)
		})

		maxValue, err := maxValueAcrossSequenceColumns(sequence, []sequenceOwnerColumn{
			{table: testNameTuple(t, "public", "parent"), column: "id"},
			{table: testNameTuple(t, "public", "child"), column: "id"},
		})

		require.NoError(t, err)
		assert.Equal(t, int64(37), maxValue)
	})

	t.Run("skips a column that does not hold integers", func(t *testing.T) {
		stubSequenceColumnMaxQuerier(t, func(string) (string, error) { return "abc", nil })

		maxValue, err := maxValueAcrossSequenceColumns(sequence, []sequenceOwnerColumn{
			{table: testNameTuple(t, "public", "t"), column: "label"},
		})

		require.NoError(t, err)
		assert.Equal(t, int64(0), maxValue)
	})

	t.Run("propagates a query failure instead of silently skipping", func(t *testing.T) {
		stubSequenceColumnMaxQuerier(t, func(string) (string, error) {
			return "", fmt.Errorf(`ERROR: column "id" does not exist (SQLSTATE 42703)`)
		})

		_, err := maxValueAcrossSequenceColumns(sequence, []sequenceOwnerColumn{
			{table: testNameTuple(t, "public", "t"), column: "id"},
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), `column "id" does not exist`)
		assert.Contains(t, err.Error(), "public")
	})

	t.Run("quotes a case sensitive column name", func(t *testing.T) {
		queries := stubSequenceColumnMaxQuerier(t, func(string) (string, error) { return "1", nil })

		_, err := maxValueAcrossSequenceColumns(sequence, []sequenceOwnerColumn{
			{table: testNameTuple(t, "public", "t"), column: "Id"},
		})

		require.NoError(t, err)
		require.Len(t, *queries, 1)
		assert.Contains(t, (*queries)[0], `MAX("Id")`)
	})
}
