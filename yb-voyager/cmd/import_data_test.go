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
		"--source-db-schema", "test_schema", // override schema
		"--disable-pb", "true",
		"--yes",
	}, nil, false)
	if err != nil {
		t.Fatalf("Export command failed: %v", err)
	}

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
		if err != nil {
			t.Fatalf("Failed to start async import command (run #%d): %v", i+1, err)
		}

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
	if err != nil {
		t.Fatalf("Final import command failed: %v", err)
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
	} else {
		t.Log("Success: Table data in Postgres and YugabyteDB match exactly.")
	}
}

// Test import data command with interruptions and non-transactional path, with primary key conflict action as UPDATE.
func TestImportDataResumptionWithInterruptions_NonTransactionalPath_OnPrimaryKeyConflictActionAsUpdate(t *testing.T) {
	ctx := context.Background()

	// Create a temporary export directory.
	exportDir = testutils.CreateTempExportDir()
	// defer testutils.RemoveTempExportDir(exportDir)

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
		"--source-db-schema", "public,test_schema", // override schema
		"--disable-pb", "true",
		"--yes",
	}, nil, false)
	if err != nil {
		t.Fatalf("Export command failed: %v", err)
	}

	importDataCmdArgs := []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--enable-fast-path", "true",
		"--on-primary-key-conflict", "UPDATE",
		"--yes",
	}

	// Simulate multiple interruptions during the import to YugabyteDB.
	// We will run the import command asynchronously and then, after a delay, kill the process.
	interruptionRuns := 2
	for i := 0; i < interruptionRuns; i++ {
		fmt.Printf("Starting async import run #%d with interruption...\n", i+1)

		// Start the import command in async mode.
		cmd, err := testutils.RunVoyagerCommand(yugabytedbContainer, "import data", importDataCmdArgs, nil, true)
		if err != nil {
			t.Fatalf("Failed to start async import command (run #%d): %v", i+1, err)
		}

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
	if err != nil {
		t.Fatalf("Final import command failed: %v", err)
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
	} else {
		t.Log("Success: Table data in Postgres and YugabyteDB match exactly.")
	}
}

// Test import data command with interruptions and non-transactional path, with primary key conflict action as IGNORE.
func TestImportDataResumptionWithInterruptions_NonTransactionalPath_OnPrimaryKeyConflictActionAsIgnore(t *testing.T) {
	ctx := context.Background()

	// Create a temporary export directory.
	exportDir = testutils.CreateTempExportDir()
	// defer testutils.RemoveTempExportDir(exportDir)

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
		"--source-db-schema", "public,test_schema", // override schema
		"--disable-pb", "true",
		"--yes",
	}, nil, false)
	if err != nil {
		t.Fatalf("Export command failed: %v", err)
	}

	importDataCmdArgs := []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--enable-fast-path", "true",
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
		if err != nil {
			t.Fatalf("Failed to start async import command (run #%d): %v", i+1, err)
		}

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
	if err != nil {
		t.Fatalf("Final import command failed: %v", err)
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
	} else {
		t.Log("Success: Table data in Postgres and YugabyteDB match exactly.")
	}
}
