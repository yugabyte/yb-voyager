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
package srcdb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsColumnTypeUnsupported(t *testing.T) {
	unsupportedTypeNames := []string{"XML", "PG_LSN"}
	unsupportedTypTypes := []string{pgTypTypeRange}

	testCases := []struct {
		name        string
		columnTypes []columnType
		expected    bool
	}{
		{"supported declared type", []columnType{{"int4", "b"}}, false},
		{"unsupported declared type, case-insensitive", []columnType{{"xml", "b"}}, true},
		{"unsupported array element", []columnType{{"_xml", "b"}, {"xml", "b"}}, true},
		{"unsupported type under stacked domains", []columnType{{"lsn_dom_outer", "d"}, {"lsn_dom", "d"}, {"pg_lsn", "b"}}, true},
		{"unsupported type in the middle of the chain", []columnType{{"range_dom", "d"}, {"public.float_range", "r"}, {"float8", "b"}}, true},
		{"unsupported composite field", []columnType{{"public.geo", "c"}, {"text", "b"}, {"pg_lsn", "b"}}, true},
		{"composite of supported fields", []columnType{{"public.address", "c"}, {"text", "b"}, {"int4", "b"}}, false},
		{"name match is exact, not a prefix", []columnType{{"xml_dom", "d"}, {"text", "b"}}, false},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, isColumnTypeUnsupported(tc.columnTypes, unsupportedTypeNames, unsupportedTypTypes))
		})
	}
}

// Built-in types resolve to their fixed OIDs, an extension type only once its extension is
// installed, and a user-defined type that merely shares the extension type's name never resolves.
// The extension lives in a non-public schema so a search_path-based lookup would miss it.
func runTypeToOidTest(t *testing.T, sourceDB SourceDB, db *sql.DB, executeSqls func(...string)) {
	executeSqls(
		`DROP EXTENSION IF EXISTS hstore CASCADE;`,
		`CREATE SCHEMA user_types;`,
		`CREATE TYPE user_types.hstore AS (k text, v text);`,
	)
	defer executeSqls(
		`DROP SCHEMA IF EXISTS user_types CASCADE;`,
		`DROP SCHEMA IF EXISTS ext_schema CASCADE;`,
	)
	typeNames := []string{"varbit", "tsvector", "tsquery", "hstore"}

	t.Run("extension not installed", func(t *testing.T) {
		typeToOid, err := sourceDB.GetTypeToOid(typeNames)
		require.NoError(t, err)
		require.Equal(t, map[string]int{"varbit": 1562, "tsvector": 3614, "tsquery": 3615}, typeToOid)
	})

	t.Run("extension installed in a non-public schema", func(t *testing.T) {
		executeSqls(
			`CREATE SCHEMA ext_schema;`,
			`CREATE EXTENSION hstore WITH SCHEMA ext_schema;`,
		)
		var hstoreOid int
		require.NoError(t, db.QueryRow(`SELECT 'ext_schema.hstore'::regtype::oid::bigint`).Scan(&hstoreOid))

		typeToOid, err := sourceDB.GetTypeToOid(typeNames)
		require.NoError(t, err)
		require.Equal(t, map[string]int{"varbit": 1562, "tsvector": 3614, "tsquery": 3615, "hstore": hstoreOid}, typeToOid)
	})
}

func TestPostgresGetTypeToOid(t *testing.T) {
	err := testPostgresSource.DB().Connect()
	require.NoError(t, err)
	defer testPostgresSource.DB().Disconnect()

	pgDB := testPostgresSource.DB().(*PostgreSQL)
	runTypeToOidTest(t, pgDB, pgDB.db, testPostgresSource.TestContainer.ExecuteSqls)
}

func TestYugabyteGetTypeToOid(t *testing.T) {
	ybDB := testYugabyteDBSource.DB().(*YugabyteDB)
	runTypeToOidTest(t, ybDB, ybDB.db, testYugabyteDBSource.TestContainer.ExecuteSqls)
}
