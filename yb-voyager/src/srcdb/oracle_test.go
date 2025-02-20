//go:build integration

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

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
	"gotest.tools/assert"
)

func TestOracleGetAllTableNames(t *testing.T) {
	sqlname.SourceDBType = "oracle"

	// Test GetAllTableNames
	actualTables := testOracleSource.DB().GetAllTableNames()
	expectedTables := []*sqlname.SourceName{
		sqlname.NewSourceName("YBVOYAGER", "foo"),
		sqlname.NewSourceName("YBVOYAGER", "bar"),
		sqlname.NewSourceName("YBVOYAGER", "table1"),
		sqlname.NewSourceName("YBVOYAGER", "table2"),
		sqlname.NewSourceName("YBVOYAGER", "unique_table"),
		sqlname.NewSourceName("YBVOYAGER", "non_pk1"),
		sqlname.NewSourceName("YBVOYAGER", "non_pk2"),
	}
	assert.Equal(t, len(expectedTables), len(actualTables), "Expected number of tables to match")

	testutils.AssertEqualSourceNameSlices(t, expectedTables, actualTables)
}

func TestOracleGetTableToUniqueKeyColumnsMap(t *testing.T) {
	objectName := sqlname.NewObjectName("oracle", "YBVOYAGER", "YBVOYAGER", "UNIQUE_TABLE")

	// Test GetTableToUniqueKeyColumnsMap
	tableList := []sqlname.NameTuple{
		{CurrentName: objectName},
	}
	uniqueKeys, err := testOracleSource.DB().GetTableToUniqueKeyColumnsMap(tableList)
	if err != nil {
		t.Fatalf("Error retrieving unique keys: %v", err)
	}

	expectedKeys := map[string][]string{
		"UNIQUE_TABLE": {"EMAIL", "PHONE", "ADDRESS"},
	}

	// Compare the maps by iterating over each table and asserting the columns list
	for table, expectedColumns := range expectedKeys {
		actualColumns, exists := uniqueKeys[table]
		if !exists {
			t.Errorf("Expected table %s not found in uniqueKeys", table)
		}

		testutils.AssertEqualStringSlices(t, expectedColumns, actualColumns)
	}
}

func TestOracleGetNonPKTables(t *testing.T) {
	actualTables, err := testOracleSource.DB().GetNonPKTables()
	assert.NilError(t, err, "Expected nil but non nil error: %v", err)

	expectedTables := []string{`YBVOYAGER."NON_PK1"`, `YBVOYAGER."NON_PK2"`}
	testutils.AssertEqualStringSlices(t, expectedTables, actualTables)
}
