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

	"gotest.tools/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

func TestOracleGetAllTableNames(t *testing.T) {
	sqlname.SourceDBType = "oracle"

	// Test GetAllTableNames
	actualTables := testOracleSource.DB().GetAllTableNames()
	expectedTables := []*sqlname.SourceName{
		sqlname.NewSourceName("YBVOYAGER", "FOO"),
		sqlname.NewSourceName("YBVOYAGER", "BAR"),
		sqlname.NewSourceName("YBVOYAGER", "TABLE1"),
		sqlname.NewSourceName("YBVOYAGER", "TABLE2"),
		sqlname.NewSourceName("YBVOYAGER", "UNIQUE_TABLE"),
		sqlname.NewSourceName("YBVOYAGER", "NON_PK1"),
		sqlname.NewSourceName("YBVOYAGER", "NON_PK2"),
	}
	assert.Equal(t, len(expectedTables), len(actualTables), "Expected number of tables to match")

	testutils.AssertEqualSourceNameSlices(t, expectedTables, actualTables)
}

func TestOracleGetTableToUniqueKeyColumnsMap(t *testing.T) {

	// Test GetTableToUniqueKeyColumnsMap
	tableList := []sqlname.NameTuple{
		testutils.CreateNameTupleWithSourceName("YBVOYAGER.UNIQUE_TABLE", "YBVOYAGER", "oracle"),
	}
	_ = testOracleSource.DB().Connect()
	uniqueKeys, err := testOracleSource.DB().GetTableToUniqueKeyColumnsMap(tableList)
	if err != nil {
		t.Fatalf("Error retrieving unique keys: %v", err)
	}

	expectedKeys := utils.NewStructMap[sqlname.NameTuple, []string]()
	expectedKeys.Put(testutils.CreateNameTupleWithSourceName("YBVOYAGER.UNIQUE_TABLE", "YBVOYAGER", "oracle"), []string{"EMAIL", "PHONE", "ADDRESS"})

	// Compare the maps by iterating over each table and asserting the columns list
	expectedKeys.IterKV(func(table sqlname.NameTuple, expectedColumns []string) (bool, error) {
		actualColumns, exists := uniqueKeys.Get(table)
		if !exists {
			t.Errorf("Expected table %s not found in uniqueKeys", table)
		}

		testutils.AssertEqualStringSlices(t, expectedColumns, actualColumns)
		return true, nil
	})
}

func TestOracleGetNonPKTables(t *testing.T) {
	_ = testOracleSource.DB().Connect()

	sqlname.SourceDBType = "oracle"
	actualTables, err := testOracleSource.DB().GetNonPKTables()
	assert.NilError(t, err, "Expected nil but non nil error: %v", err)

	expectedTables := []string{`"YBVOYAGER"."NON_PK1"`, `"YBVOYAGER"."NON_PK2"`}
	testutils.AssertEqualStringSlices(t, expectedTables, actualTables)
}
