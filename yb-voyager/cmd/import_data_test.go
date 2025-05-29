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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
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
		insertStatement := fmt.Sprintf(`INSERT INTO test_schema.test_data (name) VALUES ('name_%d');`, i)

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

// TestImportDataResumptionWithInterruptions_FastPath_ForTransientDBErrors
// Simulates a transient YugabyteDB outage mid-import.
// Expect: import process fails, but a subsequent resume succeeds.
// f

/*
	Add tests:
	1. TestImportDataResumptionWithInterruptions_FastPath_ForTransientDBErrors 	(retryable errors)
	2. TestImportDataResumptionWithInterruptions_FastPath_SyntaxError			(non-retryable errors)
	3. Add/Enable test for --on-primary-key-conflict=UPDATE
*/
