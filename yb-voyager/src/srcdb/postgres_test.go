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
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"testing"

	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	"gotest.tools/assert"
)

var err error

func TestMain(m *testing.M) {
	var ctx = context.Background()

	// setting soruce dbtype, version and defaults
	source := &Source{
		DBType:    "postgresql",
		DBVersion: "11",
		User:      "postgres",
		Password:  "postgres",
		SSLMode:   "disable",
		Port:      5432,
		Schema:    "public",
	}

	testDB, err = StartTestDB(ctx, source)
	if err != nil {
		utils.ErrExit("Failed to start testDB: %v", err)
	}
	defer StopTestDB(ctx, testDB)

	// disable logs
	log.SetOutput(io.Discard)

	exitCode := m.Run()
	os.Exit(exitCode)
}

func TestPostgresGetAllTableNames(t *testing.T) {
	sqlname.SourceDBType = "postgresql"

	// Test GetAllTableNames
	actualTables := testDB.Source.DB().GetAllTableNames()
	expectedTables := []*sqlname.SourceName{
		sqlname.NewSourceName("public", "foo"),
		sqlname.NewSourceName("public", "bar"),
		sqlname.NewSourceName("public", "table1"),
		sqlname.NewSourceName("public", "table2"),
		sqlname.NewSourceName("public", "unique_table"),
	}
	assert.Equal(t, len(expectedTables), len(actualTables), "Expected number of tables to match")

	// Sort and compare tables
	sortSourceNames(expectedTables)
	sortSourceNames(actualTables)
	for i, expectedTable := range expectedTables {
		assert.Equal(t, expectedTable.String(), actualTables[i].String(), "Expected table names to match")
	}
}

func TestPostgresGetTableToUniqueKeyColumnsMap(t *testing.T) {
	objectName := sqlname.NewObjectName("postgresql", "public", "public", "unique_table2")

	// Test GetTableToUniqueKeyColumnsMap
	tableList := []sqlname.NameTuple{
		{CurrentName: objectName},
	}
	uniqueKeys, err := testDB.Source.DB().GetTableToUniqueKeyColumnsMap(tableList)
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

		err := assertEqualStringSlices(t, expectedColumns, actualColumns)
		assert.NilError(t, err, "Expected no error for matching unique key columns for table %q", table)
	}
}

func sortSourceNames(tables []*sqlname.SourceName) {
	sort.Slice(tables, func(i, j int) bool {
		return tables[i].Qualified.MinQuoted < tables[j].Qualified.MinQuoted
	})
}

func assertEqualStringSlices(t *testing.T, expected, actual []string) error {
	t.Helper()
	if len(expected) != len(actual) {
		return fmt.Errorf("Mismatch in slice length. Expected: %v, Actual: %v", expected, actual)
	}

	sort.Strings(expected)
	sort.Strings(actual)
	for i, _ := range expected {
		if expected[i] != actual[i] {
			return fmt.Errorf("Expected: %q, Actual: %q", expected, actual)
		}
	}
	return nil
}
