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

func TestPostgresGetAllTableNames(t *testing.T) {
	sqlname.SourceDBType = "postgresql"

	// Test GetAllTableNames
	actualTables := testPostgresSource.DB().GetAllTableNames()
	expectedTables := []*sqlname.SourceName{
		sqlname.NewSourceName("public", "foo"),
		sqlname.NewSourceName("public", "bar"),
		sqlname.NewSourceName("public", "table1"),
		sqlname.NewSourceName("public", "table2"),
		sqlname.NewSourceName("public", "unique_table"),
		sqlname.NewSourceName("public", "non_pk1"),
		sqlname.NewSourceName("public", "non_pk2"),
	}
	assert.Equal(t, len(expectedTables), len(actualTables), "Expected number of tables to match")

	testutils.AssertEqualSourceNameSlices(t, expectedTables, actualTables)
}

func TestPostgresGetTableToUniqueKeyColumnsMap(t *testing.T) {
	objectName := sqlname.NewObjectName("postgresql", "public", "public", "unique_table")

	// Test GetTableToUniqueKeyColumnsMap
	tableList := []sqlname.NameTuple{
		{CurrentName: objectName},
	}
	uniqueKeys, err := testPostgresSource.DB().GetTableToUniqueKeyColumnsMap(tableList)
	if err != nil {
		t.Fatalf("Error retrieving unique keys: %v", err)
	}

	expectedKeys := map[string][]string{
		"unique_table": {"email", "phone", "address"},
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

func TestPostgresGetNonPKTables(t *testing.T) {
	actualTables, err := testPostgresSource.DB().GetNonPKTables()
	assert.NilError(t, err, "Expected nil but non nil error: %v", err)

	expectedTables := []string{`public."non_pk2"`, `public."non_pk1"`} // func returns table.Qualified.Quoted
	testutils.AssertEqualStringSlices(t, expectedTables, actualTables)
}
