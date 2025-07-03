//go:build integration_voyager_command

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
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/importdata"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// TestImportDataFileReport verifies the snapshot report after importing data using the import-data-file command.
func TestImportDataFileReport(t *testing.T) {
	// Create a temporary export directory.
	tempExportDir := testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(tempExportDir)

	// Start YugabyteDB container.
	setupYugabyteTestDb(t)

	// Create table in the default public schema in YugabyteDB.
	createTableSQL := `CREATE TABLE public.test_data (id INTEGER PRIMARY KEY, name TEXT);`
	testYugabyteDBTarget.TestContainer.ExecuteSqls(createTableSQL)
	t.Cleanup(func() {
		testYugabyteDBTarget.TestContainer.ExecuteSqls("DROP TABLE IF EXISTS public.test_data;")
	})

	// Generate a CSV file with test data.
	dataFilePath := filepath.Join("/tmp", "test_data.csv")
	f, err := os.Create(dataFilePath)
	if err != nil {
		t.Fatalf("Failed to create data file: %v", err)
	}
	defer os.Remove(dataFilePath)
	defer f.Close()

	w := csv.NewWriter(f)

	// Write header and rows to the CSV file.
	w.Write([]string{"id", "name"})
	for i := 1; i <= 100; i++ {
		w.Write([]string{fmt.Sprintf("%d", i), fmt.Sprintf("name_%d", i)})
	}
	w.Flush()

	// Import data from the CSV file into YugabyteDB.
	importDataFileCmdArgs := []string{
		"--export-dir", tempExportDir,
		"--disable-pb", "true",
		"--batch-size", "10",
		"--target-db-schema", "public",
		"--data-dir", filepath.Dir(dataFilePath),
		"--file-table-map", "test_data.csv:public.test_data",
		"--format", "CSV",
		"--has-header", "true",
		"--yes",
	}

	err = testutils.NewVoyagerCommandRunner(testYugabyteDBTarget.TestContainer, "import data file", importDataFileCmdArgs, nil, false).Run()
	testutils.FatalIfError(t, err, "Import command failed")

	// Verify snapshot report.
	exportDir = tempExportDir
	yb, ok := testYugabyteDBTarget.TargetDB.(*tgtdb.TargetYugabyteDB)
	if !ok {
		t.Fatalf("TargetDB is not of type TargetYugabyteDB")
	}
	err = InitNameRegistry(exportDir, IMPORT_FILE_ROLE, nil, nil, &testYugabyteDBTarget.Tconf, yb, false)
	testutils.FatalIfError(t, err, "Failed to initialize name registry")
	metaDB = initMetaDB(exportDir)
	state := NewImportDataState(exportDir)
	dataFileDescriptor, err = prepareDummyDescriptor(state)
	testutils.FatalIfError(t, err, "Failed to prepare dummy descriptor")
	snapshotRowsMap, err := getImportedSnapshotRowsMap("target-file")
	if err != nil {
		t.Fatalf("Failed to get imported snapshot rows map: %v", err)
	}

	// Ensure the snapshotRowsMap contains the expected data.
	tblName := sqlname.NameTuple{
		SourceName:  sqlname.NewObjectNameWithQualifiedName(POSTGRESQL, "public", "public.test_data"),
		CurrentName: sqlname.NewObjectNameWithQualifiedName(POSTGRESQL, "public", "public.test_data"),
	}
	rowCountPair, _ := snapshotRowsMap.Get(tblName)
	assert.Equal(t, int64(100), rowCountPair.Imported, "Imported row count mismatch")
	assert.Equal(t, int64(0), rowCountPair.Errored, "Errored row count mismatch")

	// Verify import data status command output
	err = testutils.NewVoyagerCommandRunner(testYugabyteDBTarget.TestContainer, "import data status", []string{
		"--export-dir", tempExportDir,
		"--output-format", "json",
	}, nil, false).Run()
	testutils.FatalIfError(t, err, "Import data status command failed")

	// Verify the report file content
	reportPath := filepath.Join(tempExportDir, "reports", "import-data-status-report.json")
	assert.FileExists(t, reportPath, "Import data status report file should exist")

	reportData, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("Failed to read import data status report file: %v", err)
	}
	var statusReport []*tableMigStatusOutputRow
	err = json.Unmarshal(reportData, &statusReport)
	testutils.FatalIfError(t, err, "Failed to unmarshal import data status report JSON")
	assert.Equal(t, 1, len(statusReport), "Report should contain exactly one entry")

	assert.Equal(t, &tableMigStatusOutputRow{
		TableName:          `public."test_data"`,
		FileName:           "test_data.csv",
		ImportedCount:      1092,
		ErroredCount:       0,
		TotalCount:         1092,
		Status:             "DONE",
		PercentageComplete: 100,
	}, statusReport[0], "Status report row mismatch")
}

func TestImportDataFileReport_ErrorPolicyStashAndContinue(t *testing.T) {
	// Create a temporary export directory.
	tempExportDir := testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(tempExportDir)

	// create backupDIr
	backupDir := testutils.CreateBackupDir(t)

	// Start YugabyteDB container.
	setupYugabyteTestDb(t)

	// Create table in the default public schema in YugabyteDB.
	createTableSQL := `CREATE TABLE public.test_data (id INTEGER PRIMARY KEY, name TEXT);`
	testYugabyteDBTarget.TestContainer.ExecuteSqls(createTableSQL)
	t.Cleanup(func() {
		testYugabyteDBTarget.TestContainer.ExecuteSqls("DROP TABLE IF EXISTS public.test_data;")
	})

	// Generate a CSV file with test data.
	dataFilePath := filepath.Join("/tmp", "test_data.csv")
	f, err := os.Create(dataFilePath)
	if err != nil {
		t.Fatalf("Failed to create data file: %v", err)
	}
	defer os.Remove(dataFilePath)
	defer f.Close()

	w := csv.NewWriter(f)

	// Write header and rows to the CSV file.
	w.Write([]string{"id", "name"})
	for i := 1; i <= 100; i++ {
		w.Write([]string{fmt.Sprintf("%d", i), fmt.Sprintf("name_%d", i)})
	}
	w.Flush()

	// Insert conflicting rows into YugabyteDB to simulate errors.
	for i := 1; i <= 10; i++ {
		stmt := fmt.Sprintf("INSERT INTO public.test_data (id, name) VALUES (%d, 'conflict_name_%d');", i, i)
		testYugabyteDBTarget.TestContainer.ExecuteSqls(stmt)
	}

	// Import data from the CSV file into YugabyteDB with --error-policy stash-and-continue.
	importDataFileCmdArgs := []string{
		"--export-dir", tempExportDir,
		"--disable-pb", "true",
		"--batch-size", "10",
		"--target-db-schema", "public",
		"--data-dir", filepath.Dir(dataFilePath),
		"--file-table-map", "test_data.csv:public.test_data",
		"--format", "CSV",
		"--has-header", "true",
		"--error-policy", "stash-and-continue",
		"--yes",
	}

	err = testutils.NewVoyagerCommandRunner(testYugabyteDBTarget.TestContainer, "import data file", importDataFileCmdArgs, nil, false).Run()
	testutils.FatalIfError(t, err, "Import data file command failed")

	// Verify snapshot report.
	exportDir = tempExportDir
	yb, ok := testYugabyteDBTarget.TargetDB.(*tgtdb.TargetYugabyteDB)
	if !ok {
		t.Fatalf("TargetDB is not of type TargetYugabyteDB")
	}
	err = InitNameRegistry(exportDir, IMPORT_FILE_ROLE, nil, nil, &testYugabyteDBTarget.Tconf, yb, false)
	testutils.FatalIfError(t, err, "Failed to initialize name registry")
	metaDB = initMetaDB(exportDir)
	state := NewImportDataState(exportDir)
	dataFileDescriptor, err = prepareDummyDescriptor(state)
	testutils.FatalIfError(t, err, "Failed to prepare dummy descriptor")

	snapshotRowsMap, err := getImportedSnapshotRowsMap("target-file")
	if err != nil {
		t.Fatalf("Failed to get imported snapshot rows map: %v", err)
	}
	// Ensure the snapshotRowsMap contains the expected data.
	tblName := sqlname.NameTuple{
		SourceName:  sqlname.NewObjectNameWithQualifiedName(POSTGRESQL, "public", "public.test_data"),
		CurrentName: sqlname.NewObjectNameWithQualifiedName(POSTGRESQL, "public", "public.test_data"),
	}
	rowCountPair, _ := snapshotRowsMap.Get(tblName)
	assert.Equal(t, int64(90), rowCountPair.Imported, "Imported row count mismatch")
	assert.Equal(t, int64(10), rowCountPair.Errored, "Errored row count mismatch")

	// Verify import data status command output
	err = testutils.NewVoyagerCommandRunner(testYugabyteDBTarget.TestContainer, "import data status", []string{
		"--export-dir", tempExportDir,
		"--output-format", "json",
	}, nil, false).Run()
	testutils.FatalIfError(t, err, "Import data status command failed")

	// Verify the report file content
	reportPath := filepath.Join(tempExportDir, "reports", "import-data-status-report.json")
	assert.FileExists(t, reportPath, "Import data status report file should exist")

	reportData, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("Failed to read import data status report file: %v", err)
	}
	var statusReport []*tableMigStatusOutputRow
	err = json.Unmarshal(reportData, &statusReport)
	testutils.FatalIfError(t, err, "Failed to unmarshal import data status report JSON")
	assert.Equal(t, 1, len(statusReport), "Report should contain exactly one entry")

	assert.Equal(t, &tableMigStatusOutputRow{
		TableName:          `public."test_data"`,
		FileName:           "test_data.csv",
		ImportedCount:      992,
		ErroredCount:       100,
		TotalCount:         1092,
		Status:             "DONE_WITH_ERRORS",
		PercentageComplete: 100,
	}, statusReport[0], "Status report row mismatch")

	// Run end-migration to ensure that the errored files are backed up properly
	os.Setenv("SOURCE_DB_PASSWORD", "postgres")
	os.Setenv("TARGET_DB_PASSWORD", "yugabyte")
	err = testutils.NewVoyagerCommandRunner(testYugabyteDBTarget.TestContainer, "end migration", []string{
		"--export-dir", tempExportDir,
		"--backup-data-files", "true",
		"--backup-dir", backupDir,
		"--backup-log-files", "true",
		"--backup-schema-files", "false",
		"--save-migration-reports", "false",
		"--yes",
	}, nil, false).Run()
	testutils.FatalIfError(t, err, "End migration command failed")

	// Verify that the backup directory contains the expected error files.
	// error file is expected to be under dir table::test_data/file::test_data_data.sql:1960b25c and of the name ingestion-error.batch::1.10.10.92.E
	tableDir := fmt.Sprintf("table::%s", "test_data")
	fileDir := fmt.Sprintf("file::%s:%s", filepath.Base(dataFilePath), importdata.ComputePathHash(dataFilePath))
	tableFileErrorsDir := filepath.Join(backupDir, "data", "errors", tableDir, fileDir)
	errorFilePath := filepath.Join(tableFileErrorsDir, "ingestion-error.batch::1.10.10.100.E")
	assert.FileExistsf(t, errorFilePath, "Expected error file %s to exist", errorFilePath)

	// Verify the content of the error file
	testutils.AssertFileContains(t, errorFilePath, "duplicate key value violates unique constraint")
}

func TestImportDataFile_MultipleTasksForATable(t *testing.T) {
	// Create a temporary export directory.
	tempExportDir := testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(tempExportDir)

	// Start YugabyteDB container.
	setupYugabyteTestDb(t)

	// Create table in the default public schema in YugabyteDB.
	createTableSQL := `CREATE TABLE public.test_data (id INTEGER PRIMARY KEY, name TEXT);`
	testYugabyteDBTarget.TestContainer.ExecuteSqls(createTableSQL)
	t.Cleanup(func() {
		testYugabyteDBTarget.TestContainer.ExecuteSqls("DROP TABLE IF EXISTS public.test_data;")
	})

	// Generate a CSV file with test data.
	dataFilePath := filepath.Join("/tmp", "test_data.csv")
	f, err := os.Create(dataFilePath)
	if err != nil {
		t.Fatalf("Failed to create data file: %v", err)
	}
	defer os.Remove(dataFilePath)
	defer f.Close()

	// Generate a CSV file with test data.
	dataFilePath1 := filepath.Join("/tmp", "test_data1.csv")
	f1, err := os.Create(dataFilePath1)
	if err != nil {
		t.Fatalf("Failed to create data file: %v", err)
	}
	defer os.Remove(dataFilePath1)
	defer f1.Close()

	w := csv.NewWriter(f)

	// Write header and rows to the CSV file.
	w.Write([]string{"id", "name"})
	for i := 1; i <= 100; i++ {
		w.Write([]string{fmt.Sprintf("%d", i), fmt.Sprintf("name_%d", i)})
	}
	w.Flush()

	w = csv.NewWriter(f1)
	w.Write([]string{"id", "name"})
	for i := 101; i <= 200; i++ {
		w.Write([]string{fmt.Sprintf("%d", i), fmt.Sprintf("name_%d", i)})
	}
	w.Flush()
	// Import data from the CSV file into YugabyteDB.
	importDataFileCmdArgs := []string{
		"--export-dir", tempExportDir,
		"--disable-pb", "true",
		"--batch-size", "10",
		"--target-db-schema", "public",
		"--data-dir", filepath.Dir(dataFilePath),
		"--file-table-map", "test_data.csv:public.test_data,test_data1.csv:public.test_data",
		"--format", "CSV",
		"--has-header", "true",
		"--yes",
	}

	err = testutils.NewVoyagerCommandRunner(testYugabyteDBTarget.TestContainer, "import data file", importDataFileCmdArgs, nil, false).Run()
	testutils.FatalIfError(t, err, "Import command failed")

	// Verify snapshot report.
	exportDir = tempExportDir
	yb, ok := testYugabyteDBTarget.TargetDB.(*tgtdb.TargetYugabyteDB)
	if !ok {
		t.Fatalf("TargetDB is not of type TargetYugabyteDB")
	}
	err = InitNameRegistry(exportDir, IMPORT_FILE_ROLE, nil, nil, &testYugabyteDBTarget.Tconf, yb, false)
	testutils.FatalIfError(t, err, "Failed to initialize name registry")
	metaDB = initMetaDB(exportDir)
	state := NewImportDataState(exportDir)
	dataFileDescriptor, err = prepareDummyDescriptor(state)
	testutils.FatalIfError(t, err, "Failed to prepare dummy descriptor")
	snapshotRowsMap, err := getImportedSnapshotRowsMap("target-file")
	if err != nil {
		t.Fatalf("Failed to get imported snapshot rows map: %v", err)
	}

	// Ensure the snapshotRowsMap contains the expected data.
	tblName := sqlname.NameTuple{
		SourceName:  sqlname.NewObjectNameWithQualifiedName(POSTGRESQL, "public", "public.test_data"),
		CurrentName: sqlname.NewObjectNameWithQualifiedName(POSTGRESQL, "public", "public.test_data"),
	}
	rowCountPair, _ := snapshotRowsMap.Get(tblName)
	assert.Equal(t, int64(200), rowCountPair.Imported, "Imported row count mismatch")
	assert.Equal(t, int64(0), rowCountPair.Errored, "Errored row count mismatch")

	// Verify import data status command output
	err = testutils.NewVoyagerCommandRunner(testYugabyteDBTarget.TestContainer, "import data status", []string{
		"--export-dir", tempExportDir,
		"--output-format", "json",
	}, nil, false).Run()
	testutils.FatalIfError(t, err, "Import data status command failed")

	// Verify the report file content
	reportPath := filepath.Join(tempExportDir, "reports", "import-data-status-report.json")
	assert.FileExists(t, reportPath, "Import data status report file should exist")

	reportData, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("Failed to read import data status report file: %v", err)
	}
	var statusReport []*tableMigStatusOutputRow
	err = json.Unmarshal(reportData, &statusReport)
	testutils.FatalIfError(t, err, "Failed to unmarshal import data status report JSON")
	assert.Equal(t, 2, len(statusReport), "Report should contain exactly one entry")

	assert.Equal(t, &tableMigStatusOutputRow{
		TableName:          `public."test_data"`,
		FileName:           "test_data.csv",
		ImportedCount:      1092,
		ErroredCount:       0,
		TotalCount:         1092,
		Status:             "DONE",
		PercentageComplete: 100,
	}, statusReport[0], "Status report row mismatch")

	assert.Equal(t, &tableMigStatusOutputRow{
		TableName:          `public."test_data"`,
		FileName:           "test_data1.csv",
		ImportedCount:      1308,
		ErroredCount:       0,
		TotalCount:         1308,
		Status:             "DONE",
		PercentageComplete: 100,
	}, statusReport[1], "Status report row mismatch")
}

func TestImportDataFile_SameFileForMultipleTables(t *testing.T) {
	// Create a temporary export directory.
	tempExportDir := testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(tempExportDir)

	// Start YugabyteDB container.
	setupYugabyteTestDb(t)

	// Create table in the default public schema in YugabyteDB.
	createTableSQL := `CREATE TABLE test_schema.test_data (id INTEGER PRIMARY KEY, name TEXT);`
	createTableSQL1 := `CREATE TABLE test_schema.test_data1 (id INTEGER PRIMARY KEY, name TEXT);`
	testYugabyteDBTarget.TestContainer.ExecuteSqls([]string{
		"CREATE SCHEMA IF NOT EXISTS test_schema;",
		createTableSQL, 
		createTableSQL1,
	}...)
	defer testYugabyteDBTarget.TestContainer.ExecuteSqls([]string{
		"DROP SCHEMA IF EXISTS test_schema CASCADE;",
	}...)

	// Generate a CSV file with test data.
	dataFilePath := filepath.Join("/tmp", "test_data.csv")
	f, err := os.Create(dataFilePath)
	if err != nil {
		t.Fatalf("Failed to create data file: %v", err)
	}
	defer os.Remove(dataFilePath)
	defer f.Close()

	w := csv.NewWriter(f)

	// Write header and rows to the CSV file.
	w.Write([]string{"id", "name"})
	for i := 1; i <= 100; i++ {
		w.Write([]string{fmt.Sprintf("%d", i), fmt.Sprintf("name_%d", i)})
	}
	w.Flush()

	// Import data from the CSV file into YugabyteDB.
	importDataFileCmdArgs := []string{
		"--export-dir", tempExportDir,
		"--disable-pb", "true",
		"--batch-size", "10",
		"--target-db-schema", "test_schema",
		"--data-dir", filepath.Dir(dataFilePath),
		"--file-table-map", "test_data.csv:test_data,test_data.csv:test_data1",
		"--format", "CSV",
		"--has-header", "true",
		"--yes",
	}

	err = testutils.NewVoyagerCommandRunner(testYugabyteDBTarget.TestContainer, "import data file", importDataFileCmdArgs, nil, false).Run()
	testutils.FatalIfError(t, err, "Import command failed")

	// Verify snapshot report.
	exportDir = tempExportDir
	yb, ok := testYugabyteDBTarget.TargetDB.(*tgtdb.TargetYugabyteDB)
	if !ok {
		t.Fatalf("TargetDB is not of type TargetYugabyteDB")
	}
	err = InitNameRegistry(exportDir, IMPORT_FILE_ROLE, nil, nil, &testYugabyteDBTarget.Tconf, yb, false)
	testutils.FatalIfError(t, err, "Failed to initialize name registry")
	metaDB = initMetaDB(exportDir)
	state := NewImportDataState(exportDir)
	dataFileDescriptor, err = prepareDummyDescriptor(state)
	testutils.FatalIfError(t, err, "Failed to prepare dummy descriptor")
	snapshotRowsMap, err := getImportedSnapshotRowsMap("target-file")
	if err != nil {
		t.Fatalf("Failed to get imported snapshot rows map: %v", err)
	}

	// Ensure the snapshotRowsMap contains the expected data.
	tblName := sqlname.NameTuple{
		SourceName:  sqlname.NewObjectNameWithQualifiedName(POSTGRESQL, "public", "test_schema.test_data"),
		CurrentName: sqlname.NewObjectNameWithQualifiedName(POSTGRESQL, "public", "test_schema.test_data"),
	}
	tblName1 := sqlname.NameTuple{
		SourceName:  sqlname.NewObjectNameWithQualifiedName(POSTGRESQL, "public", "test_schema.test_data1"),
		CurrentName: sqlname.NewObjectNameWithQualifiedName(POSTGRESQL, "public", "test_schema.test_data1"),
	}
	tables := snapshotRowsMap.Keys()
	assert.Equal(t, 2, len(tables), "There should be two tables in the snapshot rows map")

	rowCountPair, ok := snapshotRowsMap.Get(tblName)
	assert.True(t, ok, "Table test_schema.test_data should exist in snapshot rows map")
	assert.Equal(t, int64(100), rowCountPair.Imported, "Imported row count mismatch")
	assert.Equal(t, int64(0), rowCountPair.Errored, "Errored row count mismatch")

	rowCountPair1, ok := snapshotRowsMap.Get(tblName1)
	assert.True(t, ok, "Table test_schema.test_data1 should exist in snapshot rows map")
	assert.Equal(t, int64(100), rowCountPair1.Imported, "Imported row count mismatch")
	assert.Equal(t, int64(0), rowCountPair1.Errored, "Errored row count mismatch")

	// Verify import data status command output
	err = testutils.NewVoyagerCommandRunner(testYugabyteDBTarget.TestContainer, "import data status", []string{
		"--export-dir", tempExportDir,
		"--output-format", "json",
	}, nil, false).Run()
	testutils.FatalIfError(t, err, "Import data status command failed")

	// Verify the report file content
	reportPath := filepath.Join(tempExportDir, "reports", "import-data-status-report.json")
	assert.FileExists(t, reportPath, "Import data status report file should exist")

	reportData, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("Failed to read import data status report file: %v", err)
	}
	var statusReport []*tableMigStatusOutputRow
	err = json.Unmarshal(reportData, &statusReport)
	testutils.FatalIfError(t, err, "Failed to unmarshal import data status report JSON")
	assert.Equal(t, 2, len(statusReport), "Report should contain exactly one entry")

	assert.Equal(t, &tableMigStatusOutputRow{
		TableName:          `test_schema."test_data"`,
		FileName:           "test_data.csv",
		ImportedCount:      1092,
		ErroredCount:       0,
		TotalCount:         1092,
		Status:             "DONE",
		PercentageComplete: 100,
	}, statusReport[0], "Status report row mismatch")

	assert.Equal(t, &tableMigStatusOutputRow{
		TableName:          `test_schema."test_data1"`,
		FileName:           "test_data.csv",
		ImportedCount:      1092,
		ErroredCount:       0,
		TotalCount:         1092,
		Status:             "DONE",
		PercentageComplete: 100,
	}, statusReport[1], "Status report row mismatch")
}
