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

package queryparser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTraverseAndExtractDefNamesFromDefElem(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		expected map[string]string
	}{
		{
			name: "table_presence_flag",
			sql:  `CREATE TABLE t(id int) WITH (autovacuum_enabled);`,
			expected: map[string]string{
				"autovacuum_enabled": "true",
			},
		},
		{
			name: "view_boolean_options",
			sql:  `CREATE VIEW v WITH (security_barrier=true, security_invoker=false) AS SELECT 1;`,
			expected: map[string]string{
				"security_barrier": "true",
				"security_invoker": "false",
			},
		},
		{
			name: "collation_provider_typename",
			sql:  `CREATE COLLATION c1 (provider = icu);`,
			expected: map[string]string{
				"provider": "icu",
			},
		},
		{
			name: "extension_schema_identifier",
			sql:  `CREATE EXTENSION hstore WITH SCHEMA public;`,
			expected: map[string]string{
				"schema": "public",
			},
		},
		{
			name: "createdb_integer_option",
			sql:  `CREATE DATABASE db WITH CONNECTION LIMIT 5;`,
			expected: map[string]string{
				// pg_query_go uses defname "connection_limit" (with underscore)
				"connection_limit": "5",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parseTree, err := Parse(tc.sql)
			assert.NoError(t, err)

			msg := GetProtoMessageFromParseTree(parseTree)
			defs, err := TraverseAndExtractDefNamesFromDefElem(msg)
			assert.NoError(t, err)

			for k, v := range tc.expected {
				actual, ok := defs[k]
				assert.True(t, ok, "expected key %q not found in defs: %v", k, defs)
				assert.Equalf(t, v, actual, "expected: %v, actual: %v for key %q", v, actual, k)
			}
		})
	}
}
