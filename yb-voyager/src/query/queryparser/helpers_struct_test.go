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
package queryparser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsAlterSequenceStmt(t *testing.T) {
	tests := []struct {
		sql      string
		expected bool
	}{
		{
			sql:      "ALTER SEQUENCE IF EXISTS Case_Sensitive_always_ID_seq RESTART WITH 4;",
			expected: true,
		},
		{
			sql:      "ALTER SEQUENCE Case_Sensitive_always_ID_seq RESTART WITH 4;",
			expected: true,
		},
		{
			sql:      "SELECT pg_catalog.setval('Case_Sensitive_always_ID_seq', 4, true);",
			expected: false,
		},
	}
	for _, test := range tests {

		parseTree, err := Parse(test.sql)
		if err != nil {
			t.Fatalf("error parsing sql: %v", err)
		}
		actual := IsAlterSequenceStmt(parseTree)
		assert.Equal(t, test.expected, actual)

	}
}

func TestGetSequenceNameAndLastValueFromAlterSequenceStmt(t *testing.T) {
	tests := []struct {
		sql                  string
		expectedSequenceName string
		expectedLastValue    int64
	}{
		{
			sql:                  `ALTER SEQUENCE IF EXISTS "Case_Sensitive_always_ID_seq" RESTART WITH 4;`,
			expectedSequenceName: "Case_Sensitive_always_ID_seq",
			expectedLastValue:    4,
		},
		{
			sql:                  "ALTER SEQUENCE case_sensitive_always_id_seq RESTART WITH 1290;",
			expectedSequenceName: "case_sensitive_always_id_seq",
			expectedLastValue:    1290,
		},
	}
	for _, test := range tests {
		parseTree, err := Parse(test.sql)
		if err != nil {
			t.Fatalf("error parsing sql: %v", err)
		}
		actualSequenceName, actualLastValue, err := GetSequenceNameAndLastValueFromAlterSequenceStmt(parseTree)
		if err != nil {
			t.Fatalf("error getting sequence name and last value: %v", err)
		}
		assert.Equal(t, test.expectedSequenceName, actualSequenceName)
		assert.Equal(t, test.expectedLastValue, actualLastValue)

	}
}

func TestIsSelectSetValStmt(t *testing.T) {
	tests := []struct {
		sql      string
		expected bool
	}{
		{
			sql:      "SELECT pg_catalog.setval('Case_Sensitive_always_ID_seq', 4, true);",
			expected: true,
		},
		{
			sql:      "SELECT pg_catalog.setval('Case_Sensitive_always_ID_seq', 4);",
			expected: true,
		},
		{
			sql:      "SELECT pg_catalog.setval('Case_Sensitive_always_ID_seq'::regclass, 4, false);",
			expected: true,
		},
	}
	for _, test := range tests {
		parseTree, err := Parse(test.sql)
		if err != nil {
			t.Fatalf("error parsing sql: %v", err)
		}
		actual := IsSelectSetValStmt(parseTree)
		assert.Equal(t, test.expected, actual)
	}
}

func TestGetSequenceNameAndLastValueFromSetValStmt(t *testing.T) {
	tests := []struct {
		sql      string
		expectedSequenceName string
		expectedLastValue    int64
	}{
		{
			sql:      "SELECT pg_catalog.setval('Case_Sensitive_always_ID_seq', 4, true);",
			expectedSequenceName: "Case_Sensitive_always_ID_seq",
			expectedLastValue:    4,
		},
		{
			sql:      "SELECT pg_catalog.setval('Case_Sensitive_always_ID_seq', 4);",
			expectedSequenceName: "Case_Sensitive_always_ID_seq",
			expectedLastValue:    4,
		},
		{
			sql:      "SELECT pg_catalog.setval('Case_Sensitive_always_ID_seq'::regclass, 4, false);",
			expectedSequenceName: "Case_Sensitive_always_ID_seq",
			expectedLastValue:    4,
		},
	}
	for _, test := range tests {
		parseTree, err := Parse(test.sql)
		if err != nil {
			t.Fatalf("error parsing sql: %v", err)
		}
		sequenceName, lastValue, err := GetSequenceNameAndLastValueFromSetValStmt(parseTree)
		if err != nil {
			t.Fatalf("error getting sequence name and last value: %v", err)
		}
		assert.Equal(t, test.expectedSequenceName, sequenceName)
		assert.Equal(t, test.expectedLastValue, lastValue)
	}
}