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
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/importdata"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
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
	exportRunner := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--source-db-schema", "test_schema",
		"--disable-pb", "true",
		"--yes",
	}, nil, false)
	err = exportRunner.Run()
	testutils.FatalIfError(t, err, "Failed to run export command")

	importDataCmdArgs := []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--yes",
	}
	runner := testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", importDataCmdArgs, nil, true)

	// Simulate multiple interruptions during the import to YugabyteDB.
	// We will run the import command asynchronously and then, after a delay, kill the process.
	interruptionRuns := 2
	for i := 0; i < interruptionRuns; i++ {
		fmt.Printf("\n\nStarting async import run #%d with interruption...\n", i+1)

		// Start the import command in async mode.
		err = runner.Run()
		testutils.FatalIfError(t, err, fmt.Sprintf("Failed to run async import command (run #%d)", i+1))

		// Wait a short while to ensure that the command has gotten underway.
		time.Sleep(2 * time.Second)

		t.Log("Simulating interruption by sending SIGKILL to the import command process...")
		if err := runner.Kill(); err != nil {
			t.Errorf("Failed to kill import command process on run #%d: %v", i+1, err)
		}

		// Wait for the command to exit.
		if err := runner.Wait(); err != nil {
			t.Logf("Async import run #%d exited with error (expected): %v", i+1, err)
		} else {
			t.Logf("Async import run #%d completed unexpectedly", i+1)
		}
	}

	// Now, resume the import without interruption (synchronous mode) to complete the data import.
	t.Log("Resuming import command to complete data import...")
	runner.SetAsync(false) // Set to synchronous mode for the final run
	err = runner.Run()
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
	runner := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--source-db-schema", "test_schema",
		"--disable-pb", "true",
		"--yes",
	}, nil, false)
	err := runner.Run()
	testutils.FatalIfError(t, err, "Export command failed")

	importDataCmdArgs := []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--on-primary-key-conflict", "ignore",
		"--yes",
	}

	importDataCmdRunner := testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", importDataCmdArgs, nil, true) //async mode

	// Simulate multiple interruptions during the import to YugabyteDB.
	// We will run the import command asynchronously and then, after a delay, kill the process.
	interruptionRuns := 2
	for i := 0; i < interruptionRuns; i++ {
		t.Logf("\n\nStarting async import run #%d with interruption...\n", i+1)

		// Start the import command in async mode.
		err = importDataCmdRunner.Run()
		testutils.FatalIfError(t, err, fmt.Sprintf("Failed to run async import command (run #%d)", i+1))

		// Wait a short while to ensure that the command has gotten underway.
		time.Sleep(2 * time.Second)

		t.Log("Simulating interruption by sending SIGKILL to the import command process...")
		if err = importDataCmdRunner.Kill(); err != nil {
			t.Errorf("Failed to kill import command process on run #%d: %v", i+1, err)
		}

		// Wait for the command to exit.
		if err = importDataCmdRunner.Wait(); err != nil {
			t.Logf("Async import run #%d exited with error (expected): %v", i+1, err)
		} else {
			t.Logf("Async import run #%d completed unexpectedly", i+1)
		}
	}

	// Now, resume the import without interruption (synchronous mode) to complete the data import.
	t.Log("Resuming import command to complete data import...")
	importDataCmdRunner.SetAsync(false) // Set to synchronous mode for the final run
	err = importDataCmdRunner.Run()
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

	exportRunner := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--source-db-schema", "test_schema",
		"--disable-pb", "true",
		"--yes",
	}, nil, false)
	err := exportRunner.Run()
	testutils.FatalIfError(t, err, "Export command failed")

	importRunner := testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--on-primary-key-conflict", "IGNORE",
		"--yes",
	}, nil, false)
	err = importRunner.Run()
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

func TestImportData_FastPath_OnPrimaryKeyConflictAsIgnore_PartitionedTableAlreadyHasData(t *testing.T) {
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
CREATE TABLE test_schema.emp (
	emp_id   INT,
	emp_name TEXT,
	dep_code INT,
	PRIMARY KEY (emp_id)
) PARTITION BY HASH (emp_id);

CREATE TABLE test_schema.emp_0 PARTITION OF test_schema.emp FOR VALUES WITH (MODULUS 3, REMAINDER 0);
CREATE TABLE test_schema.emp_1 PARTITION OF test_schema.emp FOR VALUES WITH (MODULUS 3, REMAINDER 1);
CREATE TABLE test_schema.emp_2 PARTITION OF test_schema.emp FOR VALUES WITH (MODULUS 3, REMAINDER 2);
`

	// insert 100 rows in the table
	var pgInsertStatements, ybInsertStatements []string
	for i := 0; i < 100; i++ {
		empID := i + 1
		depCode := i % 5
		insertStatement := fmt.Sprintf(
			`INSERT INTO test_schema.emp (emp_id, emp_name, dep_code) VALUES (%d, 'name_%d', %d);`,
			empID, i, depCode,
		)

		if (i % 2) == 0 {
			ybInsertStatements = append(ybInsertStatements, insertStatement)
		}
		pgInsertStatements = append(pgInsertStatements, insertStatement)
	}

	// prepare Postgres with full data
	postgresContainer.ExecuteSqls(createSchemaSQL, createTableSQL)
	postgresContainer.ExecuteSqls(pgInsertStatements...)
	defer postgresContainer.ExecuteSqls(dropSchemaSQL)

	// pre-load half the data into Yugabyte to cause PK conflicts
	yugabytedbContainer.ExecuteSqls(createSchemaSQL, createTableSQL)
	yugabytedbContainer.ExecuteSqls(ybInsertStatements...)
	defer yugabytedbContainer.ExecuteSqls(dropSchemaSQL)

	// Export data from Postgres.
	exportRunner := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--source-db-schema", "test_schema",
		"--disable-pb", "true",
		"--yes",
	}, nil, false)
	err := exportRunner.Run()
	testutils.FatalIfError(t, err, "Export command failed")

	// Import into Yugabyte with IGNORE on PK conflict.
	importRunner := testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--on-primary-key-conflict", "IGNORE",
		"--yes",
	}, nil, false)
	err = importRunner.Run()
	testutils.FatalIfError(t, err, "Import command failed")

	// Connect to both Postgres and YugabyteDB.
	pgConn, err := postgresContainer.GetConnection()
	testutils.FatalIfError(t, err, "connecting to Postgres")

	ybConn, err := yugabytedbContainer.GetConnection()
	testutils.FatalIfError(t, err, "Error connecting to YugabyteDB")

	// Compare the full table data between Postgres and YugabyteDB.
	// We assume the table "emp" has a primary key "emp_id" so we order by it.
	if err := testutils.CompareTableData(ctx, pgConn, ybConn, "test_schema.emp", "emp_id"); err != nil {
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

	err := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--source-db-schema", "test_schema",
		"--disable-pb", "true",
		"--yes",
	}, nil, false).Run()
	testutils.FatalIfError(t, err, "Export command failed")

	importDataCmdArgs := []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--on-primary-key-conflict", "ERROR",
		"--yes",
	}
	// first run: import data command to load data from PG
	err = testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", importDataCmdArgs, nil, false).Run()
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
	err = testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", importDataCmdArgs, nil, false).Run()
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

	err := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--source-db-schema", "public,test_schema",
		"--disable-pb", "true",
		"--yes",
	}, nil, false).Run()
	if err != nil {
		t.Fatalf("Export command failed: %v", err)
	}

	importDataCmdArgs := []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--on-primary-key-conflict", "IGNORE",
		"--yes",
	}

	err = testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", importDataCmdArgs, nil, false).Run()
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
	err := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--source-db-schema", "test_schema",
		"--disable-pb", "true",
		"--yes",
	}, nil, false).Run()
	testutils.FatalIfError(t, err, "Export command failed")

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

	err = testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data file", importDataFileCmdArgs, nil, false).Run()
	testutils.FatalIfError(t, err, "Import command failed")

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

	err = testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data file", importDataFileCmdArgs, nil, false).Run()
	testutils.FatalIfError(t, err, "Import command failed")

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
	exportDataCmdRunner := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--source-db-schema", "public",
		"--disable-pb", "true",
		"--yes",
	}, nil, false)

	err := exportDataCmdRunner.Run()
	testutils.FatalIfError(t, err, "Export command failed")

	// create new export dir and run import data file command on this data file
	exportDir2 := testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir2)

	importDataFileCmdRunner := testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data file", []string{
		"--export-dir", exportDir2,
		"--disable-pb", "true",
		"--batch-size", "10",
		"--on-primary-key-conflict", "IGNORE",
		"--data-dir", filepath.Join(exportDir, "data"),
		"--file-table-map", "foo_data.sql:public.foo",
		"--format", "TEXT", // by default hasHeader is false
		"--yes",
	}, nil, false)

	// Run the import command
	err = importDataFileCmdRunner.Run()
	testutils.FatalIfError(t, err, "Import command failed")

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

	importDataFileCmdRunner := testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data file", []string{
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
	}, nil, false)

	// Run the import command
	err = importDataFileCmdRunner.Run()
	expectedErr := tgtdb.VIOLATES_UNIQUE_CONSTRAINT_ERROR
	if err != nil && !strings.Contains(importDataFileCmdRunner.Stderr(), expectedErr) {
		// err message from Run just contains the ExitCode, refer Stderr for actual error message
		t.Fatalf("Import command failed with expected error: %v\n actual error: %s", expectedErr, importDataFileCmdRunner.Stderr())
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

	importDataFileCmdRunner := testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data file", importDataFileCmdArgs, nil, false)

	// Run the import command
	if err := importDataFileCmdRunner.Run(); err != nil {
		testutils.FatalIfError(t, errors.New(importDataFileCmdRunner.Stderr()), "Import command failed unexpectedly")
	}

	// Connect to YugabyteDB.
	ybConn, err := yugabytedbContainer.GetConnection()
	testutils.FatalIfError(t, err, "Error connecting to YugabyteDB")

	// verify the row count
	var rowCount int
	err = ybConn.QueryRow("SELECT COUNT(*) FROM test_schema.test_data").Scan(&rowCount)
	testutils.FatalIfError(t, err, "Error querying row count")

	assert.Equal(t, 100, rowCount, "Row count mismatch: expected 100, got %d", rowCount)
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

// TestExportAndImportDataSnapshotReport verifies the snapshot report after exporting and importing data.
func TestExportAndImportDataSnapshotReport(t *testing.T) {
	ctx := context.Background()

	// Create a temporary export directory.
	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

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
	err := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--source-db-schema", "public",
		"--disable-pb", "true",
		"--yes",
	}, nil, false).Run()
	testutils.FatalIfError(t, err, "Export command failed")

	// Import data into YugabyteDB.
	err = testutils.NewVoyagerCommandRunner(testYugabyteDBTarget.TestContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--yes",
	}, nil, false).Run()
	testutils.FatalIfError(t, err, "Import Data command failed")

	yb, ok := testYugabyteDBTarget.TargetDB.(*tgtdb.TargetYugabyteDB)
	if !ok {
		t.Fatalf("TargetDB is not of type TargetYugabyteDB")
	}
	err = InitNameRegistry(exportDir, TARGET_DB_IMPORTER_ROLE, nil, nil, &testYugabyteDBTarget.Tconf, yb, false)
	testutils.FatalIfError(t, err, "Failed to initialize name registry")

	metaDB = initMetaDB(exportDir)
	errorHandler, err := getImportDataErrorHandlerUsed()
	if err != nil {
		t.Fatalf("Failed to get import data error handler: %v", err)
	}
	tblName := sqlname.NameTuple{
		SourceName:  sqlname.NewObjectNameWithQualifiedName(POSTGRESQL, "public", "public.test_data"),
		CurrentName: sqlname.NewObjectNameWithQualifiedName(POSTGRESQL, "public", "public.test_data"),
	}
	tableList := []sqlname.NameTuple{
		tblName,
	}

	snapshotRowsMap, err := getImportedSnapshotRowsMap("target", tableList, errorHandler)
	if err != nil {
		t.Fatalf("Failed to get imported snapshot rows map: %v", err)
	}

	rowCountPair, _ := snapshotRowsMap.Get(tblName)
	assert.Equal(t, int64(10), rowCountPair.Imported, "Imported row count mismatch")
	assert.Equal(t, int64(0), rowCountPair.Errored, "Errored row count mismatch")

	// Verify import data status command output
	err = testutils.NewVoyagerCommandRunner(testYugabyteDBTarget.TestContainer, "import data status", []string{
		"--export-dir", exportDir,
		"--output-format", "json",
	}, nil, false).Run()
	testutils.FatalIfError(t, err, "Import data status command failed")

	// Verify the report file content
	reportPath := filepath.Join(exportDir, "reports", "import-data-status-report.json")
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
func TestExportAndImportDataSnapshotReport_ErrorPolicyStashAndContinue_BatchIngestionError(t *testing.T) {
	ctx := context.Background()

	// Create a temporary export directory.
	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

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
	err := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--source-db-schema", "public",
		"--disable-pb", "true",
		"--yes",
	}, nil, false).Run()
	testutils.FatalIfError(t, err, "Export command failed")

	// Import data into YugabyteDB with --error-policy stash-and-continue.
	err = testutils.NewVoyagerCommandRunner(testYugabyteDBTarget.TestContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--error-policy-snapshot", "stash-and-continue",
		"--batch-size", "10",
		"--yes",
	}, nil, false).Run()
	testutils.FatalIfError(t, err, "Import command failed")

	// Verify snapshot report.
	yb, ok := testYugabyteDBTarget.TargetDB.(*tgtdb.TargetYugabyteDB)
	if !ok {
		t.Fatalf("TargetDB is not of type TargetYugabyteDB")
	}
	err = InitNameRegistry(exportDir, TARGET_DB_IMPORTER_ROLE, nil, nil, &testYugabyteDBTarget.Tconf, yb, false)
	testutils.FatalIfError(t, err, "Failed to initialize name registry")

	metaDB = initMetaDB(exportDir)
	errorHandler, err := getImportDataErrorHandlerUsed()
	if err != nil {
		t.Fatalf("Failed to get import data error handler: %v", err)
	}

	// Ensure the snapshotRowsMap contains the expected data.
	tblName := sqlname.NameTuple{
		SourceName:  sqlname.NewObjectNameWithQualifiedName(POSTGRESQL, "public", "public.test_data"),
		CurrentName: sqlname.NewObjectNameWithQualifiedName(POSTGRESQL, "public", "public.test_data"),
	}
	tableList := []sqlname.NameTuple{
		tblName,
	}

	snapshotRowsMap, err := getImportedSnapshotRowsMap("target", tableList, errorHandler)
	if err != nil {
		t.Fatalf("Failed to get imported snapshot rows map: %v", err)
	}

	rowCountPair, _ := snapshotRowsMap.Get(tblName)
	assert.Equal(t, int64(90), rowCountPair.Imported, "Imported row count mismatch")
	assert.Equal(t, int64(10), rowCountPair.Errored, "Errored row count mismatch")

	// Verify import data status command output
	err = testutils.NewVoyagerCommandRunner(testYugabyteDBTarget.TestContainer, "import data status", []string{
		"--export-dir", exportDir,
		"--output-format", "json",
	}, nil, false).Run()
	testutils.FatalIfError(t, err, "Import data status command failed")

	// Verify the report file content
	reportPath := filepath.Join(exportDir, "reports", "import-data-status-report.json")
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
	err = testutils.NewVoyagerCommandRunner(testYugabyteDBTarget.TestContainer, "end migration", []string{
		"--export-dir", exportDir,
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
	tableDir := fmt.Sprintf("table::%s", tblName.ForKey())
	fileDir := fmt.Sprintf("file::test_data_data.sql:%s", importdata.ComputePathHash(filepath.Join(exportDir, "data", "test_data_data.sql")))
	tableFileErrorsDir := filepath.Join(backupDir, "data", "errors", tableDir, fileDir)
	errorFilePath := filepath.Join(tableFileErrorsDir, "ingestion-error.batch::1.10.10.92.E")
	assert.FileExistsf(t, errorFilePath, "Expected error file %s to exist", errorFilePath)

	// Verify the content of the error file
	testutils.AssertFileContains(t, errorFilePath, "duplicate key value violates unique constraint")
}

func TestImportOfSubsetOfExportedTables(t *testing.T) {
	ctx := context.Background()

	// Create a temporary export directory.
	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	createSchemaSQL := `CREATE SCHEMA IF NOT EXISTS test_schema;`
	createTableSQL := `
CREATE TABLE test_schema.test_migration (
	id SERIAL PRIMARY KEY,
	name TEXT,
	email TEXT,
	description TEXT
);`
	insertDataSQL := `
INSERT INTO test_schema.test_migration (name, email, description)
SELECT
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 10);`
	createTable1SQL := `
CREATE TABLE test_schema.test_migration1 (
	id SERIAL PRIMARY KEY,
	name TEXT,
	email TEXT,
	description TEXT
);`
	insertData1SQL := `
INSERT INTO test_schema.test_migration1 (name, email, description)
SELECT
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 10);`
	dropSchemaSQL := `DROP SCHEMA IF EXISTS test_schema CASCADE;`

	// Start Postgres container for live migration
	postgresContainer := testcontainers.NewTestContainer("postgresql", &testcontainers.ContainerConfig{
		ForLive: true,
	})
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
		createTable1SQL,
		insertData1SQL,
	}...)

	yugabytedbContainer.ExecuteSqls([]string{
		createSchemaSQL,
		createTableSQL,
		//Not creating second table in yb to test the import of subset of exported tables
	}...)

	defer postgresContainer.ExecuteSqls(dropSchemaSQL)
	defer yugabytedbContainer.ExecuteSqls(dropSchemaSQL)

	err := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--source-db-schema", "test_schema",
		"--disable-pb", "true",
		"--yes",
	}, func() {
		time.Sleep(15 * time.Second) // Wait for the export to start
	}, true).Run()
	testutils.FatalIfError(t, err, "Export command failed")

	exportStatus := testutils.NewVoyagerCommandRunner(nil, "export data status", []string{
		"--export-dir", exportDir,
		"--output-format", "json",
	}, nil, false)
	err = exportStatus.Run()
	testutils.FatalIfError(t, err, "Export data status command failed")

	//verify the report file content
	reportPath := filepath.Join(exportDir, "reports", "export-data-status-report.json")
	reportData, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("Failed to read import data status report file: %v", err)
	}
	var exportReportData []*exportTableMigStatusOutputRow
	err = json.Unmarshal(reportData, &exportReportData)
	testutils.FatalIfError(t, err, "Failed to read export data status report file")

	assert.Equal(t, 2, len(exportReportData), "Report should contain exactly one entry")
	assert.Equal(t, &exportTableMigStatusOutputRow{
		TableName:     `test_migration`,
		ExportedCount: 10,
		Status:        "DONE",
	}, exportReportData[0], "Status report row mismatch")

	assert.Equal(t, &exportTableMigStatusOutputRow{
		TableName:     `test_migration1`,
		ExportedCount: 10,
		Status:        "DONE",
	}, exportReportData[1], "Status report row mismatch")

	importCmd := testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--yes",
	}, func() {
		time.Sleep(15 * time.Second)
	}, false)

	err = importCmd.Run()

	//assert error contains table not found
	assert.NotNil(t, err)
	assert.Contains(t, importCmd.Stdout(), `Following tables are not present in the target database:
test_schema."test_migration1"`)

	err = testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--exclude-table-list", "test_schema.test_migration1",
		"--yes",
	}, func() {
		time.Sleep(15 * time.Second)
	}, true).Run()

	testutils.FatalIfError(t, err, "Import command failed")

	//verify import data status command output
	err = testutils.NewVoyagerCommandRunner(nil, "import data status", []string{
		"--export-dir", exportDir,
		"--output-format", "json",
	}, nil, false).Run()
	testutils.FatalIfError(t, err, "Import data status command failed")

	//verify the import report file content
	reportPath = filepath.Join(exportDir, "reports", "import-data-status-report.json")
	reportData, err = os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("Failed to read import data status report file: %v", err)
	}
	var importReportData []*tableMigStatusOutputRow
	err = json.Unmarshal(reportData, &importReportData)
	testutils.FatalIfError(t, err, "Failed to read import data status report file")

	assert.Equal(t, 2, len(importReportData), "Report should contain exactly one entry")
	assert.Equal(t, &tableMigStatusOutputRow{
		TableName:          `test_schema."test_migration"`,
		FileName:           "",
		ImportedCount:      10,
		ErroredCount:       0,
		TotalCount:         10,
		Status:             "DONE",
		PercentageComplete: 100,
	}, importReportData[0], "Status report row mismatch")

	assert.Equal(t, &tableMigStatusOutputRow{
		TableName:          `test_schema."test_migration1"`,
		FileName:           "",
		ImportedCount:      0,
		ErroredCount:       0,
		TotalCount:         10,
		Status:             "NOT_STARTED",
		PercentageComplete: 0,
	}, importReportData[1], "Status report row mismatch")

	//verify the export report content
	err = exportStatus.Run()
	testutils.FatalIfError(t, err, "Export data status command failed")
	reportPath = filepath.Join(exportDir, "reports", "export-data-status-report.json")
	reportData, err = os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("Failed to read import data status report file: %v", err)
	}
	err = json.Unmarshal(reportData, &exportReportData)
	testutils.FatalIfError(t, err, "Failed to read export data status report file")

	assert.Equal(t, 2, len(exportReportData), "Report should contain exactly one entry")
	assert.Equal(t, &exportTableMigStatusOutputRow{
		TableName:     `test_migration`,
		ExportedCount: 10,
		Status:        "DONE",
	}, exportReportData[0], "Status report row mismatch")

	assert.Equal(t, &exportTableMigStatusOutputRow{
		TableName:     `test_migration1`,
		ExportedCount: 10,
		Status:        "DONE",
	}, exportReportData[1], "Status report row mismatch")

}


func TestImportOfSubsetOfExportedTablesDebeziumOffline(t *testing.T) {
	ctx := context.Background()

	// Create a temporary export directory.
	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	createSchemaSQL := `CREATE SCHEMA IF NOT EXISTS test_schema;`
	createTableSQL := `
CREATE TABLE test_schema.test_migration (
	id int PRIMARY KEY,
	name TEXT,
	email TEXT,
	description TEXT
);`
	insertDataSQL := `
INSERT INTO test_schema.test_migration (id, name, email, description)
SELECT
	i,
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 10) as i;`
	createTable1SQL := `
CREATE TABLE test_schema.test_migration1 (
	id int PRIMARY KEY,
	name TEXT,
	email TEXT,
	description TEXT
);`
	insertData1SQL := `
INSERT INTO test_schema.test_migration1 (id, name, email, description)
SELECT
	i,
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 10) as i;`
	dropSchemaSQL := `DROP SCHEMA IF EXISTS test_schema CASCADE;`

	// Start Postgres container for live migration
	postgresContainer := testcontainers.NewTestContainer("postgresql", &testcontainers.ContainerConfig{
		ForLive: true,
	})
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
		createTable1SQL,
		insertData1SQL,
	}...)

	yugabytedbContainer.ExecuteSqls([]string{
		createSchemaSQL,
		createTableSQL,
		//Not creating second table in yb to test the import of subset of exported tables
	}...)

	defer postgresContainer.ExecuteSqls(dropSchemaSQL)
	defer yugabytedbContainer.ExecuteSqls(dropSchemaSQL)

	//run an export variable before this command
	os.Setenv("BETA_FAST_DATA_EXPORT", "true")
	defer os.Unsetenv("BETA_FAST_DATA_EXPORT")

	err := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--source-db-schema", "test_schema",
		"--disable-pb", "true",
		"--yes",
	}, func() {
		time.Sleep(15 * time.Second) // Wait for the export to start
	}, true).Run()
	testutils.FatalIfError(t, err, "Export command failed")

	exportStatus := testutils.NewVoyagerCommandRunner(nil, "export data status", []string{
		"--export-dir", exportDir,
		"--output-format", "json",
	}, nil, false)
	err = exportStatus.Run()
	testutils.FatalIfError(t, err, "Export data status command failed")

	//verify the report file content
	reportPath := filepath.Join(exportDir, "reports", "export-data-status-report.json")
	reportData, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("Failed to read import data status report file: %v", err)
	}
	var exportReportData []*exportTableMigStatusOutputRow
	err = json.Unmarshal(reportData, &exportReportData)
	testutils.FatalIfError(t, err, "Failed to read export data status report file")

	assert.Equal(t, 2, len(exportReportData), "Report should contain exactly one entry")
	assert.Equal(t, &exportTableMigStatusOutputRow{
		TableName:     `test_migration`,
		ExportedCount: 10,
		Status:        "DONE",
	}, exportReportData[1], "Status report row mismatch")

	assert.Equal(t, &exportTableMigStatusOutputRow{
		TableName:     `test_migration1`,
		ExportedCount: 10,
		Status:        "DONE",
	}, exportReportData[0], "Status report row mismatch")

	importCmd := testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--yes",
	}, func() {
		time.Sleep(15 * time.Second)
	}, false)

	err = importCmd.Run()

	//assert error contains table not found
	assert.NotNil(t, err)
	assert.Contains(t, importCmd.Stdout(), `Following tables are not present in the target database:
test_schema."test_migration1"`)

	err = testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--exclude-table-list", "test_schema.test_migration1",
		"--yes",
	}, func() {
		time.Sleep(15 * time.Second)
	}, true).Run()

	testutils.FatalIfError(t, err, "Import command failed")

	//verify import data status command output
	err = testutils.NewVoyagerCommandRunner(nil, "import data status", []string{
		"--export-dir", exportDir,
		"--output-format", "json",
	}, nil, false).Run()
	testutils.FatalIfError(t, err, "Import data status command failed")

	//verify the import report file content
	reportPath = filepath.Join(exportDir, "reports", "import-data-status-report.json")
	reportData, err = os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("Failed to read import data status report file: %v", err)
	}
	var importReportData []*tableMigStatusOutputRow
	err = json.Unmarshal(reportData, &importReportData)
	testutils.FatalIfError(t, err, "Failed to read import data status report file")

	assert.Equal(t, 2, len(importReportData), "Report should contain exactly one entry")
	assert.Equal(t, &tableMigStatusOutputRow{
		TableName:          `test_schema."test_migration"`,
		FileName:           "",
		ImportedCount:      10,
		ErroredCount:       0,
		TotalCount:         10,
		Status:             "DONE",
		PercentageComplete: 100,
	}, importReportData[0], "Status report row mismatch")

	assert.Equal(t, &tableMigStatusOutputRow{
		TableName:          `test_schema."test_migration1"`,
		FileName:           "",
		ImportedCount:      0,
		ErroredCount:       0,
		TotalCount:         10,
		Status:             "NOT_STARTED",
		PercentageComplete: 0,
	}, importReportData[1], "Status report row mismatch")

	//verify the export report content
	err = exportStatus.Run()
	testutils.FatalIfError(t, err, "Export data status command failed")
	reportPath = filepath.Join(exportDir, "reports", "export-data-status-report.json")
	reportData, err = os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("Failed to read import data status report file: %v", err)
	}
	err = json.Unmarshal(reportData, &exportReportData)
	testutils.FatalIfError(t, err, "Failed to read export data status report file")

	assert.Equal(t, 2, len(exportReportData), "Report should contain exactly one entry")
	assert.Equal(t, &exportTableMigStatusOutputRow{
		TableName:     `test_migration`,
		ExportedCount: 10,
		Status:        "DONE",
	}, exportReportData[1], "Status report row mismatch")

	assert.Equal(t, &exportTableMigStatusOutputRow{
		TableName:     `test_migration1`,
		ExportedCount: 10,
		Status:        "DONE",
	}, exportReportData[0], "Status report row mismatch")

}

func TestExportAndImportDataSnapshotReport_ErrorPolicyStashAndContinue_ProcessingError(t *testing.T) {
	ctx := context.Background()

	// Create a temporary export directory.
	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

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
		// if row is no. 50, create a large row with name column exceeding 1000 characters to trigger processing error
		if i == 50 {
			stmt = fmt.Sprintf("INSERT INTO public.test_data (id, name) VALUES (%d, 'name_%s');", i, strings.Repeat("a", 1000))
		}
		postgresContainer.ExecuteSqls(stmt)
	}

	// Export data from Postgres.
	err := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--source-db-schema", "public",
		"--disable-pb", "true",
		"--yes",
	}, nil, false).Run()
	testutils.FatalIfError(t, err, "Export command failed")

	// Import data into YugabyteDB with --error-policy stash-and-continue. and max batch size of 500 bytes
	os.Setenv("MAX_BATCH_SIZE_BYTES", "500")
	defer os.Unsetenv("MAX_BATCH_SIZE_BYTES")
	err = testutils.NewVoyagerCommandRunner(testYugabyteDBTarget.TestContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--error-policy-snapshot", "stash-and-continue",
		"--batch-size", "10",
		"--yes",
	}, nil, false).Run()
	testutils.FatalIfError(t, err, "Import command failed")

	// Verify snapshot report.
	yb, ok := testYugabyteDBTarget.TargetDB.(*tgtdb.TargetYugabyteDB)
	if !ok {
		t.Fatalf("TargetDB is not of type TargetYugabyteDB")
	}
	err = InitNameRegistry(exportDir, TARGET_DB_IMPORTER_ROLE, nil, nil, &testYugabyteDBTarget.Tconf, yb, false)
	testutils.FatalIfError(t, err, "Failed to initialize name registry")
	metaDB = initMetaDB(exportDir)
	errorHandler, err := getImportDataErrorHandlerUsed()
	if err != nil {
		t.Fatalf("Failed to get import data error handler: %v", err)
	}

	// Ensure the snapshotRowsMap contains the expected data.
	tblName := sqlname.NameTuple{
		SourceName:  sqlname.NewObjectNameWithQualifiedName(POSTGRESQL, "public", "public.test_data"),
		CurrentName: sqlname.NewObjectNameWithQualifiedName(POSTGRESQL, "public", "public.test_data"),
	}
	tableList := []sqlname.NameTuple{
		tblName,
	}
	snapshotRowsMap, err := getImportedSnapshotRowsMap("target", tableList, errorHandler)
	if err != nil {
		t.Fatalf("Failed to get imported snapshot rows map: %v", err)
	}
	rowCountPair, _ := snapshotRowsMap.Get(tblName)
	assert.Equal(t, int64(99), rowCountPair.Imported, "Imported row count mismatch")
	assert.Equal(t, int64(1), rowCountPair.Errored, "Errored row count mismatch")

	// Verify import data status command output
	err = testutils.NewVoyagerCommandRunner(testYugabyteDBTarget.TestContainer, "import data status", []string{
		"--export-dir", exportDir,
		"--output-format", "json",
	}, nil, false).Run()
	testutils.FatalIfError(t, err, "Import data status command failed")

	// Verify the report file content
	reportPath := filepath.Join(exportDir, "reports", "import-data-status-report.json")
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
		ImportedCount:      99,
		ErroredCount:       1,
		TotalCount:         100,
		Status:             "DONE_WITH_ERRORS",
		PercentageComplete: 100,
	}, statusReport[0], "Status report row mismatch")

	// Run end-migration to ensure that the errored files are backed up properly
	os.Setenv("SOURCE_DB_PASSWORD", "postgres")
	os.Setenv("TARGET_DB_PASSWORD", "yugabyte")
	err = testutils.NewVoyagerCommandRunner(testYugabyteDBTarget.TestContainer, "end migration", []string{
		"--export-dir", exportDir,
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
	tableDir := fmt.Sprintf("table::%s", tblName.ForKey())
	fileDir := fmt.Sprintf("file::test_data_data.sql:%s", importdata.ComputePathHash(filepath.Join(exportDir, "data", "test_data_data.sql")))
	tableFileErrorsDir := filepath.Join(backupDir, "data", "errors", tableDir, fileDir)
	errorFilePath := filepath.Join(tableFileErrorsDir, "processing-errors.5.1.1009.log")
	assert.FileExistsf(t, errorFilePath, "Expected error file %s to exist", errorFilePath)

	// Verify the content of the error file
	testutils.AssertFileContains(t, errorFilePath, "larger than the max batch size")
}
