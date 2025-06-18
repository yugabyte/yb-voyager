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
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/importdata"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/jsonfile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

func TestIsDataLine(t *testing.T) {
	assert := assert.New(t)
	testcases := []struct {
		line     string
		expected bool
	}{
		{`SET client_encoding TO 'UTF8';`, false},
		{`set client_encoding to 'UTF8';`, false},
		{`COPY "Foo" ("v") FROM STDIN;`, false},
		{"", false},
		{"\n", false},
		{"\\.\n", false},
		{"\\.", false},
	}
	insideCopyStmt := false
	for _, tc := range testcases {
		assert.Equal(tc.expected, isDataLine(tc.line, "oracle", &insideCopyStmt), "%q", tc.line)
	}
	insideCopyStmt = false
	for i := 3; i < len(testcases); i++ {
		assert.Equal(testcases[i].expected, isDataLine(testcases[i].line, "postgresql", &insideCopyStmt), "%q", testcases[i].line)
	}
}

// TestResumableImportWithInterruptions tests resumability by exporting data,
// then starting the import command in async mode, interrupting it by sending a kill signal,
// and finally resuming with a synchronous run.
// After completion, the test compares the complete table data from source and target.
func TestImportDataResumptionWithInterruptions(t *testing.T) {
	ctx := context.Background()

	// Create a temporary export directory.
	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	// Start Postgres container.
	postgresContainer := testcontainers.NewTestContainer("postgresql", nil)
	err := postgresContainer.Start(ctx)
	testutils.FatalIfError(t, err, "Failed to start Postgres container")

	// Start YugabyteDB container.
	yugabytedbContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	err = yugabytedbContainer.Start(ctx)
	testutils.FatalIfError(t, err, "Failed to start YugabyteDB container")

	createSchemaSQL := `CREATE SCHEMA IF NOT EXISTS test_schema;`
	createTableSQL := `
CREATE TABLE test_schema.test_data (
	id SERIAL PRIMARY KEY,
	name TEXT,
	email TEXT,
	description TEXT
);`
	insertDataSQL := `
INSERT INTO test_schema.test_data (name, email, description)
SELECT
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 500000);`
	dropSchemaSQL := `DROP SCHEMA IF EXISTS test_schema CASCADE;`

	// Create the test table and insert 1M rows in Postgres.
	postgresContainer.ExecuteSqls(createSchemaSQL, createTableSQL, insertDataSQL)
	defer postgresContainer.ExecuteSqls(dropSchemaSQL)

	// Create the same table in YugabyteDB.
	yugabytedbContainer.ExecuteSqls(createSchemaSQL, createTableSQL)
	defer yugabytedbContainer.ExecuteSqls(dropSchemaSQL)

	// Export data from Postgres (synchronous run).
	_, err = testutils.RunVoyagerCommand(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--source-db-schema", "test_schema",
		"--disable-pb", "true",
		"--yes",
	}, nil, false)
	testutils.FatalIfError(t, err, "Export command failed")

	importDataCmdArgs := []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--yes",
	}

	// Simulate multiple interruptions during the import to YugabyteDB.
	// We will run the import command asynchronously and then, after a delay, kill the process.
	interruptionRuns := 2
	for i := 0; i < interruptionRuns; i++ {
		fmt.Printf("\n\nStarting async import run #%d with interruption...\n", i+1)

		// Start the import command in async mode.
		cmd, err := testutils.RunVoyagerCommand(yugabytedbContainer, "import data", importDataCmdArgs, nil, true)
		testutils.FatalIfError(t, err, fmt.Sprintf("Failed to start async import command (run #%d)", i+1))

		// Wait a short while to ensure that the command has gotten underway.
		time.Sleep(2 * time.Second)

		t.Log("Simulating interruption by sending SIGKILL to the import command process...")
		if err := testutils.KillVoyagerCommand(cmd); err != nil {
			t.Errorf("Failed to kill import command process on run #%d: %v", i+1, err)
		}

		// Wait for the command to exit.
		if err := cmd.Wait(); err != nil {
			t.Logf("Async import run #%d exited with error (expected): %v", i+1, err)
		} else {
			t.Logf("Async import run #%d completed unexpectedly", i+1)
		}
	}

	// Now, resume the import without interruption (synchronous mode) to complete the data import.
	t.Log("Resuming import command to complete data import...")
	_, err = testutils.RunVoyagerCommand(yugabytedbContainer, "import data", importDataCmdArgs, nil, false)
	testutils.FatalIfError(t, err, "Final import command failed")

	// Connect to both Postgres and YugabyteDB.
	pgConn, err := postgresContainer.GetConnection()
	testutils.FatalIfError(t, err, "connecting to Postgres")
	ybConn, err := yugabytedbContainer.GetConnection()
	testutils.FatalIfError(t, err, "Error connecting to YugabyteDB")

	// Compare the full table data between Postgres and YugabyteDB.
	// We assume the table "test_data" has a primary key "id" so we order by it.
	if err := testutils.CompareTableData(ctx, pgConn, ybConn, "test_schema.test_data", "id"); err != nil {
		t.Errorf("Table data mismatch between Postgres and YugabyteDB: %v", err)
	}
}

// Test import data command with interruptions and fast path, with primary key conflict action as IGNORE.
func TestImportDataResumptionWithInterruptions_FastPath_OnPrimaryKeyConflictActionAsIgnore(t *testing.T) {
	ctx := context.Background()

	// Create a temporary export directory.
	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	// Start Postgres container.
	postgresContainer := testcontainers.NewTestContainer("postgresql", nil)
	if err := postgresContainer.Start(ctx); err != nil {
		utils.ErrExit("Failed to start Postgres container: %v", err)
	}

	// Start YugabyteDB container.
	yugabytedbContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	if err := yugabytedbContainer.Start(ctx); err != nil {
		utils.ErrExit("Failed to start YugabyteDB container: %v", err)
	}

	createSchemaSQL := `CREATE SCHEMA IF NOT EXISTS test_schema;`
	createTableSQL := `
CREATE TABLE test_schema.test_data (
	id SERIAL PRIMARY KEY,
	name TEXT,
	email TEXT,
	description TEXT
);`
	insertDataSQL := `
INSERT INTO test_schema.test_data (name, email, description)
SELECT
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 500000);`
	dropSchemaSQL := `DROP SCHEMA IF EXISTS test_schema CASCADE;`

	// Create the test table and insert 1M rows in Postgres.
	postgresContainer.ExecuteSqls(createSchemaSQL, createTableSQL, insertDataSQL)
	defer postgresContainer.ExecuteSqls(dropSchemaSQL)

	// Create the same table in YugabyteDB.
	yugabytedbContainer.ExecuteSqls(createSchemaSQL, createTableSQL)
	defer yugabytedbContainer.ExecuteSqls(dropSchemaSQL)

	// Export data from Postgres (synchronous run).
	_, err := testutils.RunVoyagerCommand(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--source-db-schema", "test_schema",
		"--disable-pb", "true",
		"--yes",
	}, nil, false)
	testutils.FatalIfError(t, err, "Export command failed")

	importDataCmdArgs := []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--on-primary-key-conflict", "ignore",
		"--yes",
	}

	// Simulate multiple interruptions during the import to YugabyteDB.
	// We will run the import command asynchronously and then, after a delay, kill the process.
	interruptionRuns := 2
	for i := 0; i < interruptionRuns; i++ {
		t.Logf("\n\nStarting async import run #%d with interruption...\n", i+1)

		// Start the import command in async mode.
		cmd, err := testutils.RunVoyagerCommand(yugabytedbContainer, "import data", importDataCmdArgs, nil, true)
		testutils.FatalIfError(t, err, fmt.Sprintf("Failed to start async import command (run #%d)", i+1))

		// Wait a short while to ensure that the command has gotten underway.
		time.Sleep(2 * time.Second)

		t.Log("Simulating interruption by sending SIGKILL to the import command process...")
		if err := testutils.KillVoyagerCommand(cmd); err != nil {
			t.Errorf("Failed to kill import command process on run #%d: %v", i+1, err)
		}

		// Wait for the command to exit.
		if err := cmd.Wait(); err != nil {
			t.Logf("Async import run #%d exited with error (expected): %v", i+1, err)
		} else {
			t.Logf("Async import run #%d completed unexpectedly", i+1)
		}
	}

	// Now, resume the import without interruption (synchronous mode) to complete the data import.
	t.Log("Resuming import command to complete data import...")
	_, err = testutils.RunVoyagerCommand(yugabytedbContainer, "import data", importDataCmdArgs, nil, false)
	testutils.FatalIfError(t, err, "Final import command failed")

	// Connect to both Postgres and YugabyteDB.
	pgConn, err := postgresContainer.GetConnection()
	testutils.FatalIfError(t, err, "connecting to Postgres")
	ybConn, err := yugabytedbContainer.GetConnection()
	testutils.FatalIfError(t, err, "Error connecting to YugabyteDB")

	// Compare the full table data between Postgres and YugabyteDB.
	// We assume the table "test_data" has a primary key "id" so we order by it.
	if err := testutils.CompareTableData(ctx, pgConn, ybConn, "test_schema.test_data", "id"); err != nil {
		t.Errorf("Table data mismatch between Postgres and YugabyteDB: %v", err)
	}
}

/*
Test import data command with fast path primary key conflict action as IGNORE
with table already has same rows which are going to be imported
Expectation: import process should ignore the rows which are already present in the table
*/
func TestImportData_FastPath_OnPrimaryKeyConflictAsIgnore_TableAlreadyHasData(t *testing.T) {
	ctx := context.Background()

	// Create a temporary export directory.
	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	// Start Postgres container.
	postgresContainer := testcontainers.NewTestContainer("postgresql", nil)
	if err := postgresContainer.Start(ctx); err != nil {
		utils.ErrExit("Failed to start Postgres container: %v", err)
	}

	// Start YugabyteDB container.
	yugabytedbContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	if err := yugabytedbContainer.Start(ctx); err != nil {
		utils.ErrExit("Failed to start YugabyteDB container: %v", err)
	}

	createSchemaSQL := `CREATE SCHEMA IF NOT EXISTS test_schema;`
	dropSchemaSQL := `DROP SCHEMA IF EXISTS test_schema CASCADE;`
	createTableSQL := `
CREATE TABLE test_schema.test_data (
	id SERIAL PRIMARY KEY,
	name TEXT
);`

	// insert 100 rows in the table
	var pgInsertStatements, ybInsertStatements []string
	for i := 0; i < 100; i++ {
		insertStatement := fmt.Sprintf(`INSERT INTO test_schema.test_data (id, name) VALUES (%d, 'name_%d');`, i+1, i)

		if (i % 2) == 0 {
			ybInsertStatements = append(ybInsertStatements, insertStatement)
		}
		pgInsertStatements = append(pgInsertStatements, insertStatement)
	}

	postgresContainer.ExecuteSqls(createSchemaSQL, createTableSQL)
	postgresContainer.ExecuteSqls(pgInsertStatements...)
	defer postgresContainer.ExecuteSqls(dropSchemaSQL)

	// Import some of the rows so that import data will have conflicts
	yugabytedbContainer.ExecuteSqls(createSchemaSQL, createTableSQL)
	yugabytedbContainer.ExecuteSqls(ybInsertStatements...)
	defer yugabytedbContainer.ExecuteSqls(dropSchemaSQL)

	_, err := testutils.RunVoyagerCommand(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--source-db-schema", "test_schema",
		"--disable-pb", "true",
		"--yes",
	}, nil, false)
	testutils.FatalIfError(t, err, "Export command failed")

	importDataCmdArgs := []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--on-primary-key-conflict", "IGNORE",
		"--yes",
	}

	_, err = testutils.RunVoyagerCommand(yugabytedbContainer, "import data", importDataCmdArgs, nil, false)
	testutils.FatalIfError(t, err, "Import command failed")

	// Connect to both Postgres and YugabyteDB.
	pgConn, err := postgresContainer.GetConnection()
	testutils.FatalIfError(t, err, "connecting to Postgres")

	ybConn, err := yugabytedbContainer.GetConnection()
	testutils.FatalIfError(t, err, "Error connecting to YugabyteDB")

	// Compare the full table data between Postgres and YugabyteDB.
	// We assume the table "test_data" has a primary key "id" so we order by it.
	if err := testutils.CompareTableData(ctx, pgConn, ybConn, "test_schema.test_data", "id"); err != nil {
		t.Errorf("Table data mismatch between Postgres and YugabyteDB: %v", err)
	}
}

// This test uses Fast Path import with --on-primary-key-conflict=IGNORE
// Focuses on testing the import of all data types via import batch recovery mode(INSERT ON CONFLICT DO NOTHING statements)
func TestImportData_FastPath_OnPrimaryKeyConflictsAsIgnore_AllDatatypesTest(t *testing.T) {
	ctx := context.Background()

	// Create a temporary export directory.
	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	// Start Postgres container.
	postgresContainer := testcontainers.NewTestContainer("postgresql", nil)
	if err := postgresContainer.Start(ctx); err != nil {
		utils.ErrExit("Failed to start Postgres container: %v", err)
	}

	// Start YugabyteDB container.
	yugabytedbContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	if err := yugabytedbContainer.Start(ctx); err != nil {
		utils.ErrExit("Failed to start YugabyteDB container: %v", err)
	}

	createSchemaSQL := `CREATE SCHEMA IF NOT EXISTS test_schema;`
	dropSchemaSQL := `DROP SCHEMA IF EXISTS test_schema CASCADE;`
	createTableSQL := testutils.AllTypesSchemaDDL("test_schema.all_types")
	insertDataSQL := testutils.AllTypesInsertSQL("test_schema.all_types", 10000)

	postgresContainer.ExecuteSqls(createSchemaSQL, createTableSQL)
	postgresContainer.ExecuteSqls(insertDataSQL)
	defer postgresContainer.ExecuteSqls(dropSchemaSQL)

	// create the same table in YugabyteDB without data.
	yugabytedbContainer.ExecuteSqls(createSchemaSQL, createTableSQL)
	defer yugabytedbContainer.ExecuteSqls(dropSchemaSQL)

	_, err := testutils.RunVoyagerCommand(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--source-db-schema", "test_schema",
		"--disable-pb", "true",
		"--yes",
	}, nil, false)
	testutils.FatalIfError(t, err, "Export command failed")

	importDataCmdArgs := []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--on-primary-key-conflict", "ERROR",
		"--yes",
	}

	// first run: import data command to load data from PG
	_, err = testutils.RunVoyagerCommand(yugabytedbContainer, "import data", importDataCmdArgs, nil, false)
	testutils.FatalIfError(t, err, "Import command failed")

	// second run: test IGNORE on primary key conflict
	importDataCmdArgs = []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--on-primary-key-conflict", "IGNORE",
		"--batch-size", "10",
		"--start-clean", "true",
		"--yes",
	}
	// second run: to test INSERT ON CONFLICT DO NOTHING statements with all datatypes
	_, err = testutils.RunVoyagerCommand(yugabytedbContainer, "import data", importDataCmdArgs, nil, false)
	testutils.FatalIfError(t, err, "Import command failed")

	// Connect to both Postgres and YugabyteDB.
	pgConn, err := postgresContainer.GetConnection()
	testutils.FatalIfError(t, err, "connecting to Postgres")

	ybConn, err := yugabytedbContainer.GetConnection()
	testutils.FatalIfError(t, err, "Error connecting to YugabyteDB")

	// Compare the full table data between Postgres and YugabyteDB.
	// We assume the table "test_data" has a primary key "id" so we order by it.
	if err := testutils.CompareTableData(ctx, pgConn, ybConn, "test_schema.all_types", "id"); err != nil {
		t.Errorf("Table data mismatch between Postgres and YugabyteDB: %v", err)
	}
}

func TestImportData_FastPath_OnPrimaryKeyConflictAsIgnore_TableAlreadyHasData_MultiSchema(t *testing.T) {
	ctx := context.Background()

	// Create a temporary export directory.
	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	// Start Postgres container.
	postgresContainer := testcontainers.NewTestContainer("postgresql", nil)
	if err := postgresContainer.Start(ctx); err != nil {
		utils.ErrExit("Failed to start Postgres container: %v", err)
	}

	// Start YugabyteDB container.
	yugabytedbContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	if err := yugabytedbContainer.Start(ctx); err != nil {
		utils.ErrExit("Failed to start YugabyteDB container: %v", err)
	}

	createSchemaSQL := `CREATE SCHEMA IF NOT EXISTS test_schema;
	CREATE SCHEMA IF NOT EXISTS public;`
	dropSchemaSQL := `DROP SCHEMA IF EXISTS test_schema CASCADE;
	DROP TABLE IF EXISTS public.foo CASCADE;`
	createTableSQL := `
CREATE TABLE test_schema.test_data (
	id INTEGER PRIMARY KEY,
	name TEXT
);
CREATE TABLE public.foo (
	id INTEGER PRIMARY KEY,
	name TEXT
);
`
	// insert 100 rows in the table
	var pgInsertStmts, ybInsertStmts []string
	for i := 0; i < 100; i++ {
		// need same rows to be Inserted in both Postgres and YugabyteDB
		stmt1 := fmt.Sprintf(`
INSERT INTO test_schema.test_data (id, name) VALUES (%d, 'name_%d');`, i+1, i)
		stmt2 := fmt.Sprintf(`
INSERT INTO public.foo (id, name) VALUES (%d, 'name_%d');`, i+1, i)

		pgInsertStmts = append(pgInsertStmts, stmt1, stmt2)
		if i%2 == 0 {
			ybInsertStmts = append(ybInsertStmts, stmt1, stmt2)
		}
	}

	postgresContainer.ExecuteSqls(createSchemaSQL, createTableSQL)
	postgresContainer.ExecuteSqls(pgInsertStmts...)
	defer postgresContainer.ExecuteSqls(dropSchemaSQL)

	// Inserting the same data so that import data will have conflicts
	yugabytedbContainer.ExecuteSqls(createSchemaSQL, createTableSQL)
	yugabytedbContainer.ExecuteSqls(ybInsertStmts...)
	defer yugabytedbContainer.ExecuteSqls(dropSchemaSQL)

	_, err := testutils.RunVoyagerCommand(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--source-db-schema", "public,test_schema",
		"--disable-pb", "true",
		"--yes",
	}, nil, false)
	if err != nil {
		t.Fatalf("Export command failed: %v", err)
	}

	importDataCmdArgs := []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--on-primary-key-conflict", "IGNORE",
		"--yes",
	}

	_, err = testutils.RunVoyagerCommand(yugabytedbContainer, "import data", importDataCmdArgs, nil, false)
	if err != nil {
		t.Fatalf("Import command failed: %v", err)
	}

	// Connect to both Postgres and YugabyteDB.
	pgConn, err := postgresContainer.GetConnection()
	if err != nil {
		t.Fatalf("Error connecting to Postgres: %v", err)
	}
	ybConn, err := yugabytedbContainer.GetConnection()
	if err != nil {
		t.Fatalf("Error connecting to YugabyteDB: %v", err)
	}

	cases := []struct {
		table string
		pk    string
	}{
		{"test_schema.test_data", "id"},
		{"public.foo", "id"},
	}

	for _, tc := range cases {
		if err := testutils.CompareTableData(ctx, pgConn, ybConn, tc.table, tc.pk); err != nil {
			t.Errorf("table %q mismatch: %v", tc.table, err)
		} else {
			t.Logf("table %q matches exactly", tc.table)
		}
	}
}

// ============ similar tests for import data file command =============

// Data file without header
func TestImportDataFile_FastPath_OnPrimaryKeyConflictAsIgnore_AlreadyHasData1(t *testing.T) {
	ctx := context.Background()

	// Create a temporary export directory.
	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	// Start Postgres container.
	postgresContainer := testcontainers.NewTestContainer("postgresql", nil)
	if err := postgresContainer.Start(ctx); err != nil {
		utils.ErrExit("Failed to start Postgres container: %v", err)
	}

	// Start YugabyteDB container.
	yugabytedbContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	if err := yugabytedbContainer.Start(ctx); err != nil {
		utils.ErrExit("Failed to start YugabyteDB container: %v", err)
	}

	createSchemaSQL := `CREATE SCHEMA IF NOT EXISTS test_schema;`
	dropSchemaSQL := `DROP SCHEMA IF EXISTS test_schema CASCADE;`
	createTableSQL := `
CREATE TABLE test_schema.test_data (
	id INTEGER PRIMARY KEY,
	name VARCHAR(255)
);`

	// insert 100 rows in the table
	var pgInsertStmts, ybInsertStmts []string
	for i := 0; i < 100; i++ {
		// need same rows to be Inserted in both Postgres and YugabyteDB
		stmt := fmt.Sprintf("INSERT INTO test_schema.test_data (id, name) VALUES (%d, 'name_%d');", i+1, i)

		pgInsertStmts = append(pgInsertStmts, stmt)
		if i%2 == 0 {
			ybInsertStmts = append(ybInsertStmts, stmt)
		}
	}

	postgresContainer.ExecuteSqls(createSchemaSQL, createTableSQL)
	postgresContainer.ExecuteSqls(pgInsertStmts...)
	defer postgresContainer.ExecuteSqls(dropSchemaSQL)

	// Inserting only subset of data to better mimick real world scenarios
	yugabytedbContainer.ExecuteSqls(createSchemaSQL, createTableSQL)
	yugabytedbContainer.ExecuteSqls(ybInsertStmts...)
	defer yugabytedbContainer.ExecuteSqls(dropSchemaSQL)

	// Export data from Postgres (synchronous run).
	_, err := testutils.RunVoyagerCommand(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--source-db-schema", "test_schema",
		"--disable-pb", "true",
		"--yes",
	}, nil, false)

	// create new export dir and run import data file command on this data file
	exportDir2 := testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir2)

	importDataFileCmdArgs := []string{
		"--export-dir", exportDir2,
		"--disable-pb", "true",
		"--batch-size", "10",
		"--target-db-schema", "test_schema",
		"--on-primary-key-conflict", "IGNORE",
		"--data-dir", filepath.Join(exportDir, "data"),
		"--file-table-map", "test_data_data.sql:test_schema.test_data",
		"--format", "TEXT", // by default hasHeader is false
		"--yes",
	}

	_, err = testutils.RunVoyagerCommand(yugabytedbContainer, "import data file", importDataFileCmdArgs, nil, false)
	if err != nil {
		t.Fatalf("Import command failed: %v", err)
	}

	// Connect to both Postgres and YugabyteDB.
	pgConn, err := postgresContainer.GetConnection()
	if err != nil {
		t.Fatalf("Error connecting to Postgres: %v", err)
	}

	ybConn, err := yugabytedbContainer.GetConnection()
	if err != nil {
		t.Fatalf("Error connecting to YugabyteDB: %v", err)
	}

	// Compare the full table data between Postgres and YugabyteDB.
	// We assume the table "test_data" has a primary key "id" so we order by it.
	if err := testutils.CompareTableData(ctx, pgConn, ybConn, "test_schema.test_data", "id"); err != nil {
		t.Errorf("Table data mismatch between Postgres and YugabyteDB: %v", err)
	}
}

// Data file with header - create a csv file with header so that import data file picks that up
func TestImportDataFile_FastPath_OnPrimaryKeyConflictAsIgnore_AlreadyHasData2(t *testing.T) {
	ctx := context.Background()
	// Start YugabyteDB container.
	yugabytedbContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	if err := yugabytedbContainer.Start(ctx); err != nil {
		utils.ErrExit("Failed to start YugabyteDB container: %v", err)
	}

	createSchemaSQL := `CREATE SCHEMA IF NOT EXISTS test_schema;`
	dropSchemaSQL := `DROP SCHEMA IF EXISTS test_schema CASCADE;`
	createTableSQL := `
CREATE table test_schema.test_data (
	id INTEGER PRIMARY KEY,
	name VARCHAR(255),
	email VARCHAR(255)
);`

	yugabytedbContainer.ExecuteSqls(createSchemaSQL, createTableSQL)
	defer yugabytedbContainer.ExecuteSqls(dropSchemaSQL)

	// generate CSV file with header + data in /tmp/data-dir/test_data.csv
	dataFilePath := filepath.Join("/tmp", "data-dir", "test_data.csv")

	// Ensure the directory exists
	dataDir := filepath.Dir(dataFilePath)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create directory %q: %v", dataDir, err)
	}

	f, err := os.Create(dataFilePath) // truncate or create new
	if err != nil {
		t.Fatalf("Error creating data file: %v", err)
	}
	defer func() {
		f.Close()
		os.Remove(dataFilePath)
	}()

	// use csv.Writer to write header and rows
	w := csv.NewWriter(f)
	if err := w.Write([]string{"id", "name", "email"}); err != nil { // write header
		t.Fatalf("Error writing CSV header: %v", err)
	}

	// write 100 rows
	for i := 1; i <= 100; i++ {
		record := []string{
			fmt.Sprintf("%d", i),
			fmt.Sprintf("user%d", i),
			fmt.Sprintf("user%d@example.com", i),
		}
		if err := w.Write(record); err != nil {
			t.Fatalf("Error writing CSV record %d: %v", i, err)
		}

		// insert only even rows to mimic real world scenarios
		if i%2 == 0 {
			yugabytedbContainer.ExecuteSqls(fmt.Sprintf("INSERT INTO test_schema.test_data(id, name, email) VALUES (%s, '%s', '%s');",
				record[0], record[1], record[2]))
		}
	}
	w.Flush() // flush the writer to ensure all data is written
	if err := w.Error(); err != nil {
		t.Fatalf("Error flushing CSV writer: %v", err)
	}

	// Create a temporary export directory.
	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	// now run import data file command using this data file
	importDataFileCmdArgs := []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--batch-size", "10",
		"--target-db-schema", "test_schema",
		"--on-primary-key-conflict", "IGNORE",
		"--data-dir", dataDir,
		"--file-table-map", "test_data.csv:test_schema.test_data",
		"--format", "CSV",
		"--has-header", "true",
		"--yes",
	}

	_, err = testutils.RunVoyagerCommand(yugabytedbContainer, "import data file", importDataFileCmdArgs, nil, false)
	if err != nil {
		t.Fatalf("Import command failed: %v", err)
	}

	// Connect to YugabyteDB.
	ybConn, err := yugabytedbContainer.GetConnection()
	if err != nil {
		t.Fatalf("Error connecting to YugabyteDB: %v", err)
	}

	// verify the row count
	var rowCount int
	err = ybConn.QueryRow("SELECT COUNT(*) FROM test_schema.test_data").Scan(&rowCount)
	if err != nil {
		t.Fatalf("Error querying row count: %v", err)
	}
	assert.Equal(t, 100, rowCount, "Row count mismatch: expected 100, got %d", rowCount)
}

// Import data file with fast path and primary key conflict action as IGNORE with default schema(public)
func TestImportDataFile_FastPath_OnPrimaryKeyConflictAsIgnore_AlreadyHasData_DefaultSchema(t *testing.T) {
	ctx := context.Background()

	// Create a temporary export directory.
	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	// Start Postgres container.
	postgresContainer := testcontainers.NewTestContainer("postgresql", nil)
	if err := postgresContainer.Start(ctx); err != nil {
		utils.ErrExit("Failed to start Postgres container: %v", err)
	}

	// Start YugabyteDB container.
	yugabytedbContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	if err := yugabytedbContainer.Start(ctx); err != nil {
		utils.ErrExit("Failed to start YugabyteDB container: %v", err)
	}

	createSchemaSQL := `CREATE SCHEMA IF NOT EXISTS public;`
	dropSchemaSQL := `DROP TABLE IF EXISTS public.foo CASCADE;`
	createTableSQL := `
CREATE TABLE public.foo (
	id INTEGER PRIMARY KEY,
	name VARCHAR(255)
);`

	// insert 100 rows in the table
	var pgInsertStmts, ybInsertStmts []string
	for i := 0; i < 100; i++ {
		stmt := fmt.Sprintf("INSERT INTO public.foo (id, name) VALUES (%d, 'name_%d');", i+1, i)

		pgInsertStmts = append(pgInsertStmts, stmt)
		if i%2 == 0 {
			ybInsertStmts = append(ybInsertStmts, stmt)
		}
	}

	postgresContainer.ExecuteSqls(createSchemaSQL, createTableSQL)
	postgresContainer.ExecuteSqls(pgInsertStmts...)
	defer postgresContainer.ExecuteSqls(dropSchemaSQL)

	// Inserting the same data so that import data will have conflicts
	yugabytedbContainer.ExecuteSqls(createSchemaSQL, createTableSQL)
	yugabytedbContainer.ExecuteSqls(ybInsertStmts...)
	defer yugabytedbContainer.ExecuteSqls(dropSchemaSQL)

	// Export data from Postgres (synchronous run).
	_, err := testutils.RunVoyagerCommand(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--source-db-schema", "public",
		"--disable-pb", "true",
		"--yes",
	}, nil, false)

	// create new export dir and run import data file command on this data file
	exportDir2 := testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir2)

	importDataFileCmdArgs := []string{
		"--export-dir", exportDir2,
		"--disable-pb", "true",
		"--batch-size", "10",
		"--on-primary-key-conflict", "IGNORE",
		"--data-dir", filepath.Join(exportDir, "data"),
		"--file-table-map", "foo_data.sql:public.foo",
		"--format", "TEXT", // by default hasHeader is false
		"--yes",
	}

	_, err = testutils.RunVoyagerCommand(yugabytedbContainer, "import data file", importDataFileCmdArgs, nil, false)
	if err != nil {
		t.Fatalf("Import command failed: %v", err)
	}

	// Connect to both Postgres and YugabyteDB.
	pgConn, err := postgresContainer.GetConnection()
	if err != nil {
		t.Fatalf("Error connecting to Postgres: %v", err)
	}

	ybConn, err := yugabytedbContainer.GetConnection()
	if err != nil {
		t.Fatalf("Error connecting to YugabyteDB: %v", err)
	}

	// Compare the full table data between Postgres and YugabyteDB.
	if err := testutils.CompareTableData(ctx, pgConn, ybConn, "public.foo", "id"); err != nil {
		t.Errorf("Table data mismatch between Postgres and YugabyteDB: %v", err)
	}
}

// Test to ensure that only unique constraint violation errors by Primary Key are ignored
// If caused by other constraints, the import should fail and not retry/ignore
// Testing with custom csv file with import data file command
func TestImportDataFile_FastPath_OnPrimaryKeyConflictAsIgnore_UniqueConstraintViolationErrorExit(t *testing.T) {
	ctx := context.Background()

	// Create a temporary export directory.
	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	// Start YugabyteDB container.
	yugabytedbContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	if err := yugabytedbContainer.Start(ctx); err != nil {
		utils.ErrExit("Failed to start YugabyteDB container: %v", err)
	}

	createSchemaSQL := `CREATE SCHEMA IF NOT EXISTS test_schema;`
	dropSchemaSQL := `DROP SCHEMA IF EXISTS test_schema CASCADE;`
	createTableSQL := `CREATE TABLE test_schema.test_data (
		id SERIAL PRIMARY KEY,
		name TEXT,
		email TEXT UNIQUE
	);`

	// generate CSV file with header + data in /tmp/data-dir/test_data.csv
	dataFilePath := filepath.Join("/tmp", "data-dir", "test_data.csv")

	// Ensure the directory exists
	dataDir := filepath.Dir(dataFilePath)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create directory %q: %v", dataDir, err)
	}

	// create and write data to the csv file
	f, err := os.Create(dataFilePath)
	if err != nil {
		t.Fatalf("Error creating data file: %v", err)
	}
	defer func() {
		f.Close()
		os.Remove(dataFilePath)
	}()

	w := csv.NewWriter(f)
	if err := w.Write([]string{"id", "name", "email"}); err != nil { // write header
		t.Fatalf("Error writing CSV header: %v", err)
	}
	for i := 1; i <= 100; i++ {
		w.Write([]string{
			fmt.Sprintf("%d", i),
			fmt.Sprintf("user%d", i),
			fmt.Sprintf("user%d@gmail.com", i),
		})
	}
	// insert one more row with duplicate email to trigger unique constraint violation
	w.Write([]string{
		fmt.Sprintf("%d", 101),
		"user101",
		fmt.Sprintf("user100@gmail.com"), // duplicate email
	})

	w.Flush() // flush the writer to ensure all data is written
	if err := w.Error(); err != nil {
		t.Fatalf("Error flushing CSV writer: %v", err)
	}

	// create the table in YugabyteDB
	yugabytedbContainer.ExecuteSqls(createSchemaSQL, createTableSQL)
	defer yugabytedbContainer.ExecuteSqls(dropSchemaSQL)

	// now run import data file command using this data file
	importDataFileCmdArgs := []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--batch-size", "10",
		"--target-db-schema", "test_schema",
		"--on-primary-key-conflict", "IGNORE",
		"--data-dir", dataDir,
		"--file-table-map", fmt.Sprintf("%s:test_schema.test_data", filepath.Base(dataFilePath)),
		"--format", "CSV",
		"--has-header", "true",
		"--truncate-splits", "false",
		"--yes",
	}

	// TODO: planning to enhance RunVoyagerCommand to return the actual error messages, currently it returns a exit status
	_, err = testutils.RunVoyagerCommand(yugabytedbContainer, "import data file", importDataFileCmdArgs, nil, false)
	if err == nil {
		t.Fatalf(`Expected import command to fail due to "unique constraint violation", but it succeeded`)
	} else {
		t.Logf("Import command failed as expected with error: %v", err)
	}
}

// Similar to the above test, but with a unique constraint violation on primary key which should be ignored
func TestImportDataFile_FastPath_OnPrimaryKeyConflictAsIgnore_UniqueConstraintViolationErrorIgnore(t *testing.T) {
	ctx := context.Background()

	// Create a temporary export directory.
	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	// Start YugabyteDB container.
	yugabytedbContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	if err := yugabytedbContainer.Start(ctx); err != nil {
		utils.ErrExit("Failed to start YugabyteDB container: %v", err)
	}

	createSchemaSQL := `CREATE SCHEMA IF NOT EXISTS test_schema;`
	dropSchemaSQL := `DROP SCHEMA IF EXISTS test_schema CASCADE;`
	createTableSQL := `CREATE TABLE test_schema.test_data (
		id SERIAL PRIMARY KEY,
		name TEXT,
		email TEXT UNIQUE
	);`

	// generate CSV file with header + data in /tmp/data-dir/test_data.csv
	dataFilePath := filepath.Join("/tmp", "data-dir", "test_data.csv")

	// Ensure the directory exists
	dataDir := filepath.Dir(dataFilePath)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create directory %q: %v", dataDir, err)
	}

	// create and write data to the csv file
	f, err := os.Create(dataFilePath)
	if err != nil {
		t.Fatalf("Error creating data file: %v", err)
	}
	defer func() {
		f.Close()
		os.Remove(dataFilePath)
	}()

	w := csv.NewWriter(f)
	if err := w.Write([]string{"id", "name", "email"}); err != nil { // write header
		t.Fatalf("Error writing CSV header: %v", err)
	}
	for i := 1; i <= 100; i++ {
		w.Write([]string{
			fmt.Sprintf("%d", i),
			fmt.Sprintf("user%d", i),
			fmt.Sprintf("user%d@gmail.com", i),
		})
	}
	// insert one more row with duplicate id(PK)
	w.Write([]string{
		fmt.Sprintf("%d", 100), // duplicate id
		"user101",
		fmt.Sprintf("user101@gmail.com"),
	})

	w.Flush() // flush the writer to ensure all data is written
	if err := w.Error(); err != nil {
		t.Fatalf("Error flushing CSV writer: %v", err)
	}

	// create the table in YugabyteDB
	yugabytedbContainer.ExecuteSqls(createSchemaSQL, createTableSQL)
	defer yugabytedbContainer.ExecuteSqls(dropSchemaSQL)

	// now run import data file command using this data file
	importDataFileCmdArgs := []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--batch-size", "10",
		"--target-db-schema", "test_schema",
		"--on-primary-key-conflict", "IGNORE",
		"--data-dir", dataDir,
		"--file-table-map", fmt.Sprintf("%s:test_schema.test_data", filepath.Base(dataFilePath)),
		"--format", "CSV",
		"--has-header", "true",
		"--yes",
	}

	_, err = testutils.RunVoyagerCommand(yugabytedbContainer, "import data file", importDataFileCmdArgs, nil, false)
	if err != nil {
		t.Fatalf("Import command failed unexpectedly: %v", err)
	} else {
		t.Logf("Import command succeeded as expected, ignoring unique constraint violation on primary key")
	}
}

// TestImportDataResumptionWithInterruptions_FastPath_ForTransientDBErrors
// Simulates a transient YugabyteDB outage mid-import.
// Expect: import process fails, but a subsequent resume succeeds.

/*
	Add tests:
	1. TestImportDataResumptionWithInterruptions_FastPath_ForTransientDBErrors 	(retryable errors)
	2. TestImportDataResumptionWithInterruptions_FastPath_SyntaxError			(non-retryable errors)
	3. Add/Enable test for --on-primary-key-conflict=UPDATE
	4. Import Data File tests
*/

////=========================================

// HELPER functions for live
func isExportDataStreamingStarted(t *testing.T) bool {
	//check for status until its streaming
	statusFilePath := filepath.Join(exportDir, "data", "export_status.json")
	status, err := dbzm.ReadExportStatus(statusFilePath)
	if err != nil {
		utils.ErrExit("Failed to read export status file: %s: %v", statusFilePath, err)
	}
	testutils.FatalIfError(t, err, "reading dbzm export status")
	if status != nil && status.Mode == dbzm.MODE_STREAMING {
		return true
	}
	return false
}

func snapshotPhaseCompleted(t *testing.T, postgresPass string, targetPass string, snapshotRows int64, tableName string) bool {
	_, err := testutils.RunVoyagerCommand(nil, "get data-migration-report", []string{
		"--export-dir", exportDir,
		"--output-format", "json",
		"--source-db-password", postgresPass,
		"--target-db-password", targetPass,
	}, nil, true)
	testutils.FatalIfError(t, err, "get data-migration-report command failed")

	reportFilePath := filepath.Join(exportDir, "reports", "data-migration-report.json")
	// Check if the report file exists and is not empty.
	if ok := utils.FileOrFolderExists(reportFilePath); ok {
		jsonFile := jsonfile.NewJsonFile[[]*rowData](reportFilePath)
		rowData, err := jsonFile.Read()
		testutils.FatalIfError(t, err, "error reading get data-migration-report")
		exportSnapshot := 0
		importSnapshot := 0
		for _, row := range *rowData {
			if row.TableName == tableName {
				if row.DBType == "source" {
					exportSnapshot = int(row.ExportedSnapshotRows)
				}
				if row.DBType == "target" {
					importSnapshot = int(row.ImportedSnapshotRows)
				}
			}
		}
		if exportSnapshot == int(snapshotRows) && exportSnapshot == importSnapshot {
			return true
		}

	}
	return false
}

func streamingPhaseCompleted(t *testing.T, postgresPass string, targetPass string, streamingInserts int64, tableName string) bool {
	_, err := testutils.RunVoyagerCommand(nil, "get data-migration-report", []string{
		"--export-dir", exportDir,
		"--output-format", "json",
		"--source-db-password", postgresPass,
		"--target-db-password", targetPass,
	}, nil, true)
	testutils.FatalIfError(t, err, "get data-migration-report command failed")

	reportFilePath := filepath.Join(exportDir, "reports", "data-migration-report.json")
	// Check if the report file exists and is not empty.
	if ok := utils.FileOrFolderExists(reportFilePath); ok {
		jsonFile := jsonfile.NewJsonFile[[]*rowData](reportFilePath)
		rowData, err := jsonFile.Read()
		testutils.FatalIfError(t, err, "error reading get data-migration-report")
		exportInserts := 0
		importInserts := 0
		for _, row := range *rowData {
			if row.TableName == `test_schema."test_live"` {
				if row.DBType == "source" {
					exportInserts = int(row.ExportedInserts)
				}
				if row.DBType == "target" {
					importInserts = int(row.ImportedInserts)
				}

			}
		}
		if exportInserts == int(streamingInserts) && exportInserts == importInserts {
			return true
		}

	}
	return false
}

func validateSequence(t *testing.T, startID int64, endId int64, ybConn *sql.DB, tableName string) {
	ids := []string{}
	for i := 16; i <= 25; i++ {
		ids = append(ids, strconv.Itoa(i))
	}
	query := fmt.Sprintf("SELECT id from %s where id IN (%s) ORDER BY id;", tableName, strings.Join(ids, ", "))
	rows, err := ybConn.Query(query)
	testutils.FatalIfError(t, err, "failed to read data")
	var resIds []string
	for rows.Next() {
		var id int
		err = rows.Scan(&id)
		testutils.FatalIfError(t, err, "error scanning rows")
		resIds = append(resIds, strconv.Itoa(id))
	}

	assert.Equal(t, ids, resIds)
}

// Basic Test for live migration with cutover
func TestBasicLiveMigrationWithCutover(t *testing.T) {
	ctx := context.Background()

	// Create a temporary export directory.
	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	createSchemaSQL := `CREATE SCHEMA IF NOT EXISTS test_schema;`
	createTableSQL := `
CREATE TABLE test_schema.test_live (
	id SERIAL PRIMARY KEY,
	name TEXT,
	email TEXT,
	description TEXT
);`
	insertDataSQL := `
INSERT INTO test_schema.test_live (name, email, description)
SELECT
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 10);`
	dropSchemaSQL := `DROP SCHEMA IF EXISTS test_schema CASCADE;`

	// Start Postgres container for live migration
	postgresContainer := testcontainers.NewTestContainerForLiveMigration("postgresql", nil)
	if err := postgresContainer.Start(ctx); err != nil {
		utils.ErrExit("Failed to start Postgres container: %v", err)
	}

	// Start YugabyteDB container.
	yugabytedbContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	if err := yugabytedbContainer.Start(ctx); err != nil {
		utils.ErrExit("Failed to start YugabyteDB container: %v", err)
	}
	postgresContainer.ExecuteSqls([]string{
		createSchemaSQL,
		createTableSQL,
		insertDataSQL,
	}...)

	yugabytedbContainer.ExecuteSqls([]string{
		createSchemaSQL,
		createTableSQL,
	}...)

	defer postgresContainer.ExecuteSqls(dropSchemaSQL)
	defer yugabytedbContainer.ExecuteSqls(dropSchemaSQL)

	_, err := testutils.RunVoyagerCommand(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--source-db-schema", "test_schema",
		"--disable-pb", "true",
		"--export-type", SNAPSHOT_AND_CHANGES,
		"--yes",
	}, nil, true)
	testutils.FatalIfError(t, err, "Export command failed")

	for {
		if isExportDataStreamingStarted(t) {
			break
		}
		time.Sleep(1 * time.Second)
	}

	_, err = testutils.RunVoyagerCommand(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--yes",
	}, nil, true)
	testutils.FatalIfError(t, err, "Import command failed")
	time.Sleep(5 * time.Second)

	for {
		if snapshotPhaseCompleted(t, postgresContainer.GetConfig().Password,
			yugabytedbContainer.GetConfig().Password, 10, `test_schema."test_live"`) {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// Connect to both Postgres and YugabyteDB.
	pgConn, err := postgresContainer.GetConnection()
	testutils.FatalIfError(t, err, "connecting to Postgres")

	ybConn, err := yugabytedbContainer.GetConnection()
	testutils.FatalIfError(t, err, "Error connecting to YugabyteDB")

	// Compare the full table data between Postgres and YugabyteDB for snapshot part.
	// We assume the table "test_data" has a primary key "id" so we order by it.
	if err := testutils.CompareTableData(ctx, pgConn, ybConn, "test_schema.test_live", "id"); err != nil {
		t.Errorf("Table data mismatch between Postgres and YugabyteDB: %v", err)
	}

	//streaming events 5 events
	postgresContainer.ExecuteSqls([]string{
		`INSERT INTO test_schema.test_live (name, email, description)
SELECT
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 5);`,
	}...)

	for {
		if streamingPhaseCompleted(t, postgresContainer.GetConfig().Password,
			yugabytedbContainer.GetConfig().Password, 5, `test_schema."test_live"`) {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// Compare the full table data between Postgres and YugabyteDB for streaming part.
	if err := testutils.CompareTableData(ctx, pgConn, ybConn, "test_schema.test_live", "id"); err != nil {
		t.Errorf("Table data mismatch between Postgres and YugabyteDB after streaming: %v", err)
	}

	// Run cutover
	_, err = testutils.RunVoyagerCommand(nil, "initiate cutover to target", []string{
		"--export-dir", exportDir,
		"--yes",
		"--prepare-for-fall-back", "false",
	}, nil, false)
	testutils.FatalIfError(t, err, "Cutover command failed")

	metaDB, err = metadb.NewMetaDB(exportDir)
	testutils.FatalIfError(t, err, "Failed to initialize meta db")

	for {
		status := getCutoverStatus()
		if status == COMPLETED {
			break
		}
		time.Sleep(1 * time.Second)
	}

	yugabytedbContainer.ExecuteSqls([]string{
		`INSERT INTO test_schema.test_live (name, email, description)
SELECT
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 10);`,
	}...)

	//Check if ids from 16-25 are present in target this is to verify the sequence serial col is restored properly till last value
	validateSequence(t, 16, 25, ybConn, `test_schema.test_live`)

}

// test for live migration with resumption and failure during restore sequences
func TestLiveMigrationWithImportResumptionOnFailureAtRestoreSequences(t *testing.T) {
	ctx := context.Background()

	// Create a temporary export directory.
	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	createSchemaSQL := `CREATE SCHEMA IF NOT EXISTS test_schema;`
	createTableSQL := `
CREATE TABLE test_schema.test_live (
	id SERIAL PRIMARY KEY,
	name TEXT,
	email TEXT,
	description TEXT
);`
	insertDataSQL := `
INSERT INTO test_schema.test_live (name, email, description)
SELECT
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 20);`
	dropSchemaSQL := `DROP SCHEMA IF EXISTS test_schema CASCADE;`

	// Start Postgres container for live migration
	postgresContainer := testcontainers.NewTestContainerForLiveMigration("postgresql", nil)
	if err := postgresContainer.Start(ctx); err != nil {
		utils.ErrExit("Failed to start Postgres container: %v", err)
	}

	// Start YugabyteDB container.
	yugabytedbContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	if err := yugabytedbContainer.Start(ctx); err != nil {
		utils.ErrExit("Failed to start YugabyteDB container: %v", err)
	}
	postgresContainer.ExecuteSqls([]string{
		createSchemaSQL,
		createTableSQL,
		insertDataSQL,
	}...)

	yugabytedbContainer.ExecuteSqls([]string{
		createSchemaSQL,
		createTableSQL,
	}...)

	defer postgresContainer.ExecuteSqls(dropSchemaSQL)
	defer yugabytedbContainer.ExecuteSqls(dropSchemaSQL)

	_, err := testutils.RunVoyagerCommand(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--source-db-schema", "test_schema",
		"--disable-pb", "true",
		"--export-type", SNAPSHOT_AND_CHANGES,
		"--yes",
	}, nil, true)
	testutils.FatalIfError(t, err, "Export command failed")

	for {
		if isExportDataStreamingStarted(t) {
			break
		}
		time.Sleep(1 * time.Second)
	}

	importCmd, err := testutils.RunVoyagerCommand(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--yes",
	}, nil, true)
	testutils.FatalIfError(t, err, "Import command failed")

	time.Sleep(5 * time.Second)

	for {
		if snapshotPhaseCompleted(t, postgresContainer.GetConfig().Password,
			yugabytedbContainer.GetConfig().Password, 20, `test_schema."test_live"`) {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// Connect to both Postgres and YugabyteDB.
	pgConn, err := postgresContainer.GetConnection()
	testutils.FatalIfError(t, err, "connecting to Postgres")

	ybConn, err := yugabytedbContainer.GetConnection()
	testutils.FatalIfError(t, err, "Error connecting to YugabyteDB")

	// Compare the full table data between Postgres and YugabyteDB for snapshot part.
	// We assume the table "test_data" has a primary key "id" so we order by it.
	if err := testutils.CompareTableData(ctx, pgConn, ybConn, "test_schema.test_live", "id"); err != nil {
		t.Errorf("Table data mismatch between Postgres and YugabyteDB: %v", err)
	}

	//streaming events 15 events
	postgresContainer.ExecuteSqls([]string{
		`INSERT INTO test_schema.test_live (name, email, description)
SELECT
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 15);`,
	}...)

	for {
		if streamingPhaseCompleted(t, postgresContainer.GetConfig().Password,
			yugabytedbContainer.GetConfig().Password, 15, `test_schema."test_live"`) {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// Compare the full table data between Postgres and YugabyteDB for streaming part.
	if err := testutils.CompareTableData(ctx, pgConn, ybConn, "test_schema.test_live", "id"); err != nil {
		t.Errorf("Table data mismatch between Postgres and YugabyteDB after streaming: %v", err)
	}

	//Stopping import command
	if err := testutils.KillVoyagerCommand(importCmd); err != nil {
		testutils.FatalIfError(t, err, "killing the import data process errored")
	}

	time.Sleep(5 * time.Second)

	// Wait for the command to exit.
	if err := importCmd.Wait(); err != nil {
		t.Logf("Async import run exited with error (expected): %v", err)
	} else {
		t.Logf("Async import run completed unexpectedly")
	}

	time.Sleep(10 * time.Second)

	// Perform cutover
	_, err = testutils.RunVoyagerCommand(nil, "initiate cutover to target", []string{
		"--export-dir", exportDir,
		"--yes",
		"--prepare-for-fall-back", "false",
	}, nil, false)
	testutils.FatalIfError(t, err, "Cutover command failed")

	//Dropping sequence on yugabyte
	yugabytedbContainer.ExecuteSqls([]string{
		`DROP SEQUENCE test_schema.test_live_id_seq CASCADE;`,
	}...)

	time.Sleep(10 * time.Second)

	//Resume import command after deleting a sequence of the table column idand import should fail while restoring sequences as cutover is already triggered
	importCmd, err = testutils.RunVoyagerCommand(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--yes",
	}, nil, false)
	assert.NotNil(t, err) //Can't validate error "failed to restore sequences:" as we don't get the exact err msg

	//Create sequence back on yb to resume import and finish cutover

	yugabytedbContainer.ExecuteSqls([]string{
		`CREATE SEQUENCE test_schema.test_live_id_seq;`,
		`ALTER SEQUENCE test_schema.test_live_id_seq OWNED BY test_schema.test_live.id;`,
		`ALTER TABLE test_schema.test_live ALTER COLUMN id SET DEFAULT nextval('test_schema.test_live_id_seq');`,
	}...)

	//Resume import command after deleting a sequence of the table column idand import should pass while restoring sequences as cutover is already triggered
	importCmd, err = testutils.RunVoyagerCommand(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--yes",
	}, nil, true)
	testutils.FatalIfError(t, err, "import data failed")

	metaDB, err = metadb.NewMetaDB(exportDir)
	testutils.FatalIfError(t, err, "Failed to initialize meta db")

	for {
		status := getCutoverStatus()
		if status == COMPLETED {
			break
		}
		time.Sleep(1 * time.Second)
	}

	yugabytedbContainer.ExecuteSqls([]string{
		`INSERT INTO test_schema.test_live (name, email, description)
SELECT
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 10);`,
	}...)

	//Check if ids from 36-45 are present in target this is to verify the sequence serial col is restored properly till last value
	validateSequence(t, 36, 45, ybConn, `test_schema.test_live`)

}

func TestLiveMigrationWithImportResumptionWithGeneratedAlwaysColumn(t *testing.T) {
	ctx := context.Background()

	// Create a temporary export directory.
	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	createSchemaSQL := `CREATE SCHEMA IF NOT EXISTS test_schema;`
	createTableSQL := `
CREATE TABLE test_schema.test_live (
	id int GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	name TEXT,
	email TEXT,
	description TEXT
);`
	insertDataSQL := `
INSERT INTO test_schema.test_live (name, email, description)
SELECT
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 20);`
	dropSchemaSQL := `DROP SCHEMA IF EXISTS test_schema CASCADE;`

	// Start Postgres container with live migration
	postgresContainer := testcontainers.NewTestContainerForLiveMigration("postgresql", nil)
	if err := postgresContainer.Start(ctx); err != nil {
		utils.ErrExit("Failed to start Postgres container: %v", err)
	}

	// Start YugabyteDB container.
	yugabytedbContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	if err := yugabytedbContainer.Start(ctx); err != nil {
		utils.ErrExit("Failed to start YugabyteDB container: %v", err)
	}
	postgresContainer.ExecuteSqls([]string{
		createSchemaSQL,
		createTableSQL,
		insertDataSQL,
	}...)

	yugabytedbContainer.ExecuteSqls([]string{
		createSchemaSQL,
		createTableSQL,
	}...)

	defer postgresContainer.ExecuteSqls(dropSchemaSQL)
	defer yugabytedbContainer.ExecuteSqls(dropSchemaSQL)

	_, err := testutils.RunVoyagerCommand(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--source-db-schema", "test_schema",
		"--disable-pb", "true",
		"--export-type", SNAPSHOT_AND_CHANGES,
		"--yes",
	}, nil, true)
	testutils.FatalIfError(t, err, "Export command failed")

	for {
		//check for status until its streaming
		if isExportDataStreamingStarted(t) {
			break
		}
		time.Sleep(1 * time.Second)
	}

	importCmd, err := testutils.RunVoyagerCommand(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--yes",
	}, nil, true)
	testutils.FatalIfError(t, err, "Import command failed")

	time.Sleep(5 * time.Second)

	for {
		if snapshotPhaseCompleted(t, postgresContainer.GetConfig().Password,
			yugabytedbContainer.GetConfig().Password, 20, `test_schema."test_live"`) {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// Connect to both Postgres and YugabyteDB.
	pgConn, err := postgresContainer.GetConnection()
	testutils.FatalIfError(t, err, "connecting to Postgres")

	ybConn, err := yugabytedbContainer.GetConnection()
	testutils.FatalIfError(t, err, "Error connecting to YugabyteDB")

	// Compare the full table data between Postgres and YugabyteDB for snapshot part.
	// We assume the table "test_data" has a primary key "id" so we order by it.
	if err := testutils.CompareTableData(ctx, pgConn, ybConn, "test_schema.test_live", "id"); err != nil {
		t.Errorf("Table data mismatch between Postgres and YugabyteDB: %v", err)
	}

	//streaming events 15 events
	postgresContainer.ExecuteSqls([]string{
		`INSERT INTO test_schema.test_live (name, email, description)
SELECT
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 15);`,
	}...)

	for {
		if streamingPhaseCompleted(t, postgresContainer.GetConfig().Password,
			yugabytedbContainer.GetConfig().Password, 15, `test_schema."test_live"`) {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// Compare the full table data between Postgres and YugabyteDB for streaming part.
	if err := testutils.CompareTableData(ctx, pgConn, ybConn, "test_schema.test_live", "id"); err != nil {
		t.Errorf("Table data mismatch between Postgres and YugabyteDB after streaming: %v", err)
	}

	//Stopping import command
	if err := testutils.KillVoyagerCommand(importCmd); err != nil {
		testutils.FatalIfError(t, err, "killing the import data process errored")
	}

	time.Sleep(5 * time.Second)

	// Wait for the command to exit.
	if err := importCmd.Wait(); err != nil {
		t.Logf("Async import run exited with error (expected): %v", err)
	} else {
		t.Logf("Async import run completed unexpectedly")
	}

	time.Sleep(10 * time.Second)

	// Perform cutover
	_, err = testutils.RunVoyagerCommand(nil, "initiate cutover to target", []string{
		"--export-dir", exportDir,
		"--yes",
		"--prepare-for-fall-back", "false",
	}, nil, false)
	testutils.FatalIfError(t, err, "Cutover command failed")

	//Resume import command to finish the cutover
	importCmd, err = testutils.RunVoyagerCommand(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--yes",
	}, nil, true)
	testutils.FatalIfError(t, err, "import data failed")

	metaDB, err = metadb.NewMetaDB(exportDir)
	testutils.FatalIfError(t, err, "Failed to initialize meta db")

	for {
		status := getCutoverStatus()
		if status == COMPLETED {
			break
		}
		time.Sleep(1 * time.Second)
	}

	//Check if always is restored back
	query := fmt.Sprintf(`SELECT column_name FROM information_schema.columns where table_schema='test_schema' AND
		table_name='test_live' AND is_identity='YES' AND identity_generation='ALWAYS'`)

	var col string
	err = ybConn.QueryRow(query).Scan(&col)
	testutils.FatalIfError(t, err, "error checking if table has always or not")

	assert.Equal(t, col, "id")
}

// TestExportAndImportDataSnapshotReport verifies the snapshot report after exporting and importing data.
func TestExportAndImportDataSnapshotReport(t *testing.T) {
	ctx := context.Background()

	// Create a temporary export directory.
	tempExportDir := testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(tempExportDir)

	// Start Postgres container.
	postgresContainer := testcontainers.NewTestContainer("postgresql", nil)
	if err := postgresContainer.Start(ctx); err != nil {
		t.Fatalf("Failed to start Postgres container: %v", err)
	}

	setupYugabyteTestDb(t)

	// Create table in the default public schema in Postgres and YugabyteDB.
	createTableSQL := `CREATE TABLE public.test_data (id INTEGER PRIMARY KEY, name TEXT);`
	postgresContainer.ExecuteSqls(createTableSQL)
	testYugabyteDBTarget.TestContainer.ExecuteSqls(createTableSQL)
	t.Cleanup(func() {
		postgresContainer.ExecuteSqls("DROP TABLE IF EXISTS public.test_data;")
		testYugabyteDBTarget.TestContainer.ExecuteSqls("DROP TABLE IF EXISTS public.test_data;")
	})

	// Insert data into Postgres.
	for i := 1; i <= 10; i++ {
		stmt := fmt.Sprintf("INSERT INTO public.test_data (id, name) VALUES (%d, 'name_%d');", i, i)
		postgresContainer.ExecuteSqls(stmt)
	}

	// Export data from Postgres.
	_, err := testutils.RunVoyagerCommand(postgresContainer, "export data", []string{
		"--export-dir", tempExportDir,
		"--source-db-schema", "public",
		"--disable-pb", "true",
		"--yes",
	}, nil, false)
	if err != nil {
		t.Fatalf("Export command failed: %v", err)
	}

	// Import data into YugabyteDB.
	_, err = testutils.RunVoyagerCommand(testYugabyteDBTarget.TestContainer, "import data", []string{
		"--export-dir", tempExportDir,
		"--disable-pb", "true",
		"--yes",
	}, nil, false)
	if err != nil {
		t.Fatalf("Import command failed: %v", err)
	}

	// Verify snapshot report.
	exportDir = tempExportDir
	yb, ok := testYugabyteDBTarget.TargetDB.(*tgtdb.TargetYugabyteDB)
	if !ok {
		t.Fatalf("TargetDB is not of type TargetYugabyteDB")
	}
	err = InitNameRegistry(exportDir, TARGET_DB_IMPORTER_ROLE, nil, nil, &testYugabyteDBTarget.Tconf, yb, false)
	testutils.FatalIfError(t, err, "Failed to initialize name registry")
	snapshotRowsMap, err := getImportedSnapshotRowsMap("target")
	if err != nil {
		t.Fatalf("Failed to get imported snapshot rows map: %v", err)
	}

	// Ensure the snapshotRowsMap contains the expected data.
	tblName := sqlname.NameTuple{
		SourceName:  sqlname.NewObjectNameWithQualifiedName(POSTGRESQL, "public", "public.test_data"),
		CurrentName: sqlname.NewObjectNameWithQualifiedName(POSTGRESQL, "public", "public.test_data"),
	}
	rowCountPair, _ := snapshotRowsMap.Get(tblName)
	assert.Equal(t, int64(10), rowCountPair.Imported, "Imported row count mismatch")
	assert.Equal(t, int64(0), rowCountPair.Errored, "Errored row count mismatch")

	// Verify import data status command output
	_, err = testutils.RunVoyagerCommand(testYugabyteDBTarget.TestContainer, "import data status", []string{
		"--export-dir", tempExportDir,
		"--output-format", "json",
	}, nil, false)
	if err != nil {
		t.Fatalf("Import data status command failed: %v", err)
	}

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
		FileName:           "",
		ImportedCount:      10,
		ErroredCount:       0,
		TotalCount:         10,
		Status:             "DONE",
		PercentageComplete: 100,
	}, statusReport[0], "Status report row mismatch")

}

// TestExportAndImportDataSnapshotReport_ErrorPolicyStashAndContinue verifies the behavior of the --error-policy stash-and-continue flag.
func TestExportAndImportDataSnapshotReport_ErrorPolicyStashAndContinue(t *testing.T) {
	ctx := context.Background()

	// Create a temporary export directory.
	tempExportDir := testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(tempExportDir)

	// create backupDIr
	backupDir := testutils.CreateBackupDir(t)

	// Start Postgres container.
	postgresContainer := testcontainers.NewTestContainer("postgresql", nil)
	if err := postgresContainer.Start(ctx); err != nil {
		t.Fatalf("Failed to start Postgres container: %v", err)
	}

	// Start YugabyteDB container.
	setupYugabyteTestDb(t)

	// Create table in the default public schema in Postgres and YugabyteDB.
	createTableSQL := `CREATE TABLE public.test_data (id INTEGER PRIMARY KEY, name TEXT);`
	postgresContainer.ExecuteSqls(createTableSQL)
	testYugabyteDBTarget.TestContainer.ExecuteSqls(createTableSQL)
	t.Cleanup(func() {
		postgresContainer.ExecuteSqls("DROP TABLE IF EXISTS public.test_data;")
		testYugabyteDBTarget.TestContainer.ExecuteSqls("DROP TABLE IF EXISTS public.test_data;")
	})

	// Insert data into Postgres.
	for i := 1; i <= 100; i++ {
		stmt := fmt.Sprintf("INSERT INTO public.test_data (id, name) VALUES (%d, 'name_%d');", i, i)
		postgresContainer.ExecuteSqls(stmt)
	}

	// Insert conflicting rows into YugabyteDB to simulate errors.
	for i := 1; i <= 10; i++ {
		stmt := fmt.Sprintf("INSERT INTO public.test_data (id, name) VALUES (%d, 'conflict_name_%d');", i, i)
		testYugabyteDBTarget.TestContainer.ExecuteSqls(stmt)
	}

	// Export data from Postgres.
	_, err := testutils.RunVoyagerCommand(postgresContainer, "export data", []string{
		"--export-dir", tempExportDir,
		"--source-db-schema", "public",
		"--disable-pb", "true",
		"--yes",
	}, nil, false)
	if err != nil {
		t.Fatalf("Export command failed: %v", err)
	}

	// Import data into YugabyteDB with --error-policy stash-and-continue.
	_, err = testutils.RunVoyagerCommand(testYugabyteDBTarget.TestContainer, "import data", []string{
		"--export-dir", tempExportDir,
		"--disable-pb", "true",
		"--error-policy-snapshot", "stash-and-continue",
		"--batch-size", "10",
		"--yes",
	}, nil, false)
	if err != nil {
		t.Fatalf("Import command failed: %v", err)
	}

	// Verify snapshot report.
	exportDir = tempExportDir
	yb, ok := testYugabyteDBTarget.TargetDB.(*tgtdb.TargetYugabyteDB)
	if !ok {
		t.Fatalf("TargetDB is not of type TargetYugabyteDB")
	}
	err = InitNameRegistry(tempExportDir, TARGET_DB_IMPORTER_ROLE, nil, nil, &testYugabyteDBTarget.Tconf, yb, false)
	testutils.FatalIfError(t, err, "Failed to initialize name registry")
	snapshotRowsMap, err := getImportedSnapshotRowsMap("target")
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
	_, err = testutils.RunVoyagerCommand(testYugabyteDBTarget.TestContainer, "import data status", []string{
		"--export-dir", tempExportDir,
		"--output-format", "json",
	}, nil, false)
	if err != nil {
		t.Fatalf("Import data status command failed: %v", err)
	}

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
		FileName:           "",
		ImportedCount:      90,
		ErroredCount:       10,
		TotalCount:         100,
		Status:             "DONE_WITH_ERRORS",
		PercentageComplete: 100,
	}, statusReport[0], "Status report row mismatch")

	// Run end-migration to ensure that the errored files are backed up properly
	os.Setenv("SOURCE_DB_PASSWORD", "postgres")
	os.Setenv("TARGET_DB_PASSWORD", "yugabyte")
	_, err = testutils.RunVoyagerCommand(testYugabyteDBTarget.TestContainer, "end migration", []string{
		"--export-dir", tempExportDir,
		"--backup-data-files", "true",
		"--backup-dir", backupDir,
		"--backup-log-files", "true",
		"--backup-schema-files", "false",
		"--save-migration-reports", "false",
		"--yes",
	}, nil, false)
	testutils.FatalIfError(t, err, "End migration command failed")

	// Verify that the backup directory contains the expected error files.
	// error file is expected to be under dir table::test_data/file::test_data_data.sql:1960b25c and of the name ingestion-error.batch::1.10.10.92.E
	tableDir := fmt.Sprintf("table::%s", "test_data")
	fileDir := fmt.Sprintf("file::test_data_data.sql:%s", importdata.ComputePathHash(filepath.Join(tempExportDir, "data", "test_data_data.sql")))
	tableFileErrorsDir := filepath.Join(backupDir, "data", "errors", tableDir, fileDir)
	errorFilePath := filepath.Join(tableFileErrorsDir, "ingestion-error.batch::1.10.10.92.E")
	assert.FileExistsf(t, errorFilePath, "Expected error file %s to exist", errorFilePath)

	// Verify the content of the error file
	testutils.AssertFileContains(t, errorFilePath, "duplicate key value violates unique constraint")
}

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

	_, err = testutils.RunVoyagerCommand(testYugabyteDBTarget.TestContainer, "import data file", importDataFileCmdArgs, nil, false)
	if err != nil {
		t.Fatalf("Import data file command failed: %v", err)
	}

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
	_, err = testutils.RunVoyagerCommand(testYugabyteDBTarget.TestContainer, "import data status", []string{
		"--export-dir", tempExportDir,
		"--output-format", "json",
	}, nil, false)
	if err != nil {
		t.Fatalf("Import data status command failed: %v", err)
	}

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

	_, err = testutils.RunVoyagerCommand(testYugabyteDBTarget.TestContainer, "import data file", importDataFileCmdArgs, nil, false)
	if err != nil {
		t.Fatalf("Import data file command failed: %v", err)
	}
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
	_, err = testutils.RunVoyagerCommand(testYugabyteDBTarget.TestContainer, "import data status", []string{
		"--export-dir", tempExportDir,
		"--output-format", "json",
	}, nil, false)
	if err != nil {
		t.Fatalf("Import data status command failed: %v", err)
	}

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
	_, err = testutils.RunVoyagerCommand(testYugabyteDBTarget.TestContainer, "end migration", []string{
		"--export-dir", tempExportDir,
		"--backup-data-files", "true",
		"--backup-dir", backupDir,
		"--backup-log-files", "true",
		"--backup-schema-files", "false",
		"--save-migration-reports", "false",
		"--yes",
	}, nil, false)
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
