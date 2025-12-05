//go:build failpoint

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
	"testing"

	"github.com/stretchr/testify/assert"

	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// TestImportDataBatchCommitFailure tests failpoint infrastructure by injecting a commit failure
// during batch import, then verifying the import can resume successfully without the failpoint.
func TestImportDataBatchCommitFailure(t *testing.T) {
	ctx := context.Background()

	// Create a temporary export directory
	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	// Start Postgres container for source data
	postgresContainer := testcontainers.NewTestContainer("postgresql", nil)
	err := postgresContainer.Start(ctx)
	testutils.FatalIfError(t, err, "Failed to start Postgres container")
	defer postgresContainer.Stop(ctx)

	// Start YugabyteDB container for target
	yugabytedbContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	err = yugabytedbContainer.Start(ctx)
	testutils.FatalIfError(t, err, "Failed to start YugabyteDB container")
	defer yugabytedbContainer.Stop(ctx)

	// Create test schema and small dataset
	createSchemaSQL := `CREATE SCHEMA IF NOT EXISTS test_schema;`
	createTableSQL := `
CREATE TABLE test_schema.test_failpoint (
	id SERIAL PRIMARY KEY,
	name TEXT,
	value INTEGER
);`
	insertDataSQL := `
INSERT INTO test_schema.test_failpoint (name, value)
SELECT 'test_' || i, i * 100
FROM generate_series(1, 20) i;`
	dropSchemaSQL := `DROP SCHEMA IF EXISTS test_schema CASCADE;`

	// Setup source database
	postgresContainer.ExecuteSqls(createSchemaSQL, createTableSQL, insertDataSQL)
	defer postgresContainer.ExecuteSqls(dropSchemaSQL)

	// Setup target database
	yugabytedbContainer.ExecuteSqls(createSchemaSQL, createTableSQL)
	defer yugabytedbContainer.ExecuteSqls(dropSchemaSQL)

	// Export data from Postgres (creates natural metadata)
	exportRunner := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--source-db-schema", "test_schema",
		"--disable-pb", "true",
		"--yes",
	}, nil, false)
	err = exportRunner.Run()
	testutils.FatalIfError(t, err, "Failed to export data")

	// Enable failpoint to inject commit error
	fpEnv := testutils.GetFailpointEnvVar(
		"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb/importBatchCommitError=4*off->return()",
	)

	// Run import with failpoint enabled (should fail)
	t.Log("Running import with failpoint enabled...")
	importCmdWithFailpoint := testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--batch-size", "2", // set batch size to 2 to trigger commit error
		"--parallel-jobs", "1", // set parallel jobs to 1 to trigger commit error
		"--adaptive-parallelism", "disabled",
		"--yes",
	}, nil, false).WithEnv(fpEnv, "YB_VOYAGER_COPY_MAX_RETRY_COUNT=1")

	err = importCmdWithFailpoint.Run()

	// Verify that the import failed as expected
	assert.Error(t, err, "Expected import to fail due to failpoint injection")
	stderr := importCmdWithFailpoint.Stderr()
	assert.Contains(t, stderr, "failpoint", "Error should mention failpoint")

	// Verify no data was committed to the database
	ybConn, err := yugabytedbContainer.GetConnection()
	testutils.FatalIfError(t, err, "Failed to get YugabyteDB connection")
	defer ybConn.Close()

	var count int
	err = ybConn.QueryRow("SELECT COUNT(*) FROM test_schema.test_failpoint").Scan(&count)
	testutils.FatalIfError(t, err, "Failed to query row count")

	assert.Equal(t, 8, count, "Expected 8 rows (4 batches succeeded before failpoint triggered)")

	// Now resume import WITHOUT failpoint (should succeed)
	t.Log("Resuming import without failpoint...")
	importCmdResume := testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--yes",
	}, nil, false)

	err = importCmdResume.Run()
	testutils.FatalIfError(t, err, "Failed to resume import")

	// Verify data was successfully imported this time
	err = ybConn.QueryRow("SELECT COUNT(*) FROM test_schema.test_failpoint").Scan(&count)
	testutils.FatalIfError(t, err, "Failed to query row count after resume")

	assert.Equal(t, 20, count, "All 20 rows should be imported after successful resume")
	t.Logf("✓ Import successfully resumed and completed after failpoint was disabled")

	// Compare data between source and target
	pgConn, err := postgresContainer.GetConnection()
	testutils.FatalIfError(t, err, "Failed to get Postgres connection")
	defer pgConn.Close()

	err = testutils.CompareTableData(ctx, pgConn, ybConn, "test_schema.test_failpoint", "id")
	assert.NoError(t, err, "Data should match between source and target after recovery")

	t.Logf("✓ All data correctly imported and matches source")
}
