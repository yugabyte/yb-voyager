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
	"gotest.tools/assert"
)

// func TestPostgresExportData(t *testing.T) {
// 	ctx := context.Background()

// 	tempExportDir := os.TempDir()
// 	defer os.RemoveAll(tempExportDir) // Clean up after test

// 	dataDir := filepath.Join(tempExportDir, "data")
// 	err := os.Mkdir(dataDir, 0755)
// 	assert.NilError(t, err, "Failed to create 'data' subdirectory")

// 	tableList := []sqlname.NameTuple{
// 		{
// 			CurrentName: sqlname.NewObjectName("postgresql", "public", "public", "foo"),
// 			SourceName:  sqlname.NewObjectName("postgresql", "public", "public", "foo"),
// 		},
// 		{
// 			CurrentName: sqlname.NewObjectName("postgresql", "public", "public", "bar"),
// 			SourceName:  sqlname.NewObjectName("postgresql", "public", "public", "bar"),
// 		},
// 		{
// 			CurrentName: sqlname.NewObjectName("postgresql", "public", "public", "unique_table"),
// 			SourceName:  sqlname.NewObjectName("postgresql", "public", "public", "unique_table"),
// 		},
// 		{
// 			CurrentName: sqlname.NewObjectName("postgresql", "public", "public", "table1"),
// 			SourceName:  sqlname.NewObjectName("postgresql", "public", "public", "table1"),
// 		},
// 		{
// 			CurrentName: sqlname.NewObjectName("postgresql", "public", "public", "table2"),
// 			SourceName:  sqlname.NewObjectName("postgresql", "public", "public", "table2"),
// 		},
// 		// {
// 		// 	CurrentName: sqlname.NewObjectName("postgresql", "public", "public", "non_pk1"),
// 		// 	SourceName:  sqlname.NewObjectName("postgresql", "public", "public", "non_pk1"),
// 		// },
// 		// {
// 		// 	CurrentName: sqlname.NewObjectName("postgresql", "public", "public", "non_pk2"),
// 		// 	SourceName:  sqlname.NewObjectName("postgresql", "public", "public", "non_pk2"),
// 		// },
// 	}

// 	tablesColumnList := utils.NewStructMap[sqlname.NameTuple, []string]()
// 	for _, table := range tableList {
// 		// Assuming you want to export all columns for each table
// 		tablesColumnList.Put(table, []string{"*"})
// 	}

// 	// Initialize channels if your function requires them
// 	quitChan := make(chan bool)
// 	exportDataStart := make(chan bool)
// 	exportSuccessChan := make(chan bool)

// 	postgresTestDB.NumConnections = 2
// 	// Call ExportData with initialized parameters
// 	go postgresTestDB.DB().ExportData(ctx, tempExportDir, tableList, quitChan, exportDataStart, exportSuccessChan, tablesColumnList, "")

// 	// Wait for export to start
// 	select {
// 	case <-exportDataStart:
// 		fmt.Println("Data export started signal received.")
// 	case <-time.After(10 * time.Second):
// 		t.Fatalf("Data export did not start in time.")
// 	}

// 	// Wait for export to complete
// 	select {
// 	case <-exportSuccessChan:
// 		fmt.Println("Data export succeeded.")
// 	case <-quitChan:
// 		t.Fatalf("Data export failed.")
// 	case <-time.After(120 * time.Second):
// 		t.Fatalf("Data export timed out.")
// 	}

// 	entries, err := os.ReadDir(filepath.Join(tempExportDir, "data"))
// 	assert.NilError(t, err, "Error while reading contents of export dir(temp): %s", err)
// 	for _, entry := range entries {
// 		log.Infof("file name: %s", entry.Name())
// 	}

// 	expected := len(tableList)
// 	actual := len(entries) - 1
// 	assert.Equal(t, len(tableList), len(entries)-1, "Expected data files: %d, actual data files: %d", expected, actual)

// 	/*
// 		for _, table := range tableList {
// 			// Assuming that exportDir/data contains the exported files named as <table>.sql
// 			exportedFilePath := filepath.Join(dataDir, table.CurrentName.Unqualified.Unquoted+".sql")

// 			// Check if the file exists
// 			_, err := os.Stat(exportedFilePath)
// 			assert.NilError(t, err, "Exported file for table %s does not exist", table.CurrentName.Unqualified.Unquoted)

// 			// Check if the file is not empty
// 			fileInfo, err := os.Stat(exportedFilePath)
// 			assert.NilError(t, err, "Failed to stat file %s", exportedFilePath)
// 			assert.Check(t, fileInfo.Size() > 0, "Exported file %s is empty", exportedFilePath)

// 			// Optionally, read and verify file contents
// 			content, err := os.ReadFile(exportedFilePath)
// 			assert.NilError(t, err, "Failed to read exported file %s", exportedFilePath)
// 			assert.Check(t, string(content), "CREATE TABLE", "Exported file %s does not contain 'CREATE TABLE'", exportedFilePath)
// 		}
// 	*/
// }

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

	assertEqualSourceNameSlices(t, expectedTables, actualTables)
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

		assertEqualStringSlices(t, expectedColumns, actualColumns)
	}
}

func TestPostgresGetNonPKTables(t *testing.T) {
	actualTables, err := testPostgresSource.DB().GetNonPKTables()
	assert.NilError(t, err, "Expected nil but non nil error: %v", err)

	expectedTables := []string{`public."non_pk2"`, `public."non_pk1"`} // func returns table.Qualified.Quoted
	assertEqualStringSlices(t, expectedTables, actualTables)
}
