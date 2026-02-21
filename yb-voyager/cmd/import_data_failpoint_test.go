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
	"github.com/stretchr/testify/require"

	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// TestImportDataBatchCommitFailure tests failpoint infrastructure by injecting a commit failure
// during batch import, then verifying the import can resume successfully without the failpoint.
func TestImportDataBatchCommitFailure(t *testing.T) {
	ctx := context.Background()

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:    ContainerConfig{Type: "postgresql", DatabaseName: "test_failpoint"},
		TargetDB:    ContainerConfig{Type: "yugabytedb", DatabaseName: "test_failpoint"},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			"CREATE SCHEMA IF NOT EXISTS test_schema;",
			"CREATE TABLE test_schema.test_failpoint (id SERIAL PRIMARY KEY, name TEXT, value INTEGER);",
		},
		InitialDataSQL: []string{
			"INSERT INTO test_schema.test_failpoint (name, value) SELECT 'test_' || i, i * 100 FROM generate_series(1, 20) i;",
		},
		CleanupSQL: []string{"DROP SCHEMA IF EXISTS test_schema CASCADE;"},
	})
	defer lm.Cleanup()
	require.NoError(t, lm.SetupContainers(ctx))
	require.NoError(t, lm.SetupSchema())

	// Snapshot-only export: the framework StartExportData uses snapshot-and-changes,
	// so we use a custom runner here.
	exportRunner := testutils.NewVoyagerCommandRunner(lm.GetSourceContainer(), "export data", []string{
		"--export-dir", lm.GetExportDir(),
		"--source-db-schema", "test_schema",
		"--source-db-name", "test_failpoint",
		"--disable-pb", "true",
		"--yes",
	}, nil, false)
	err := exportRunner.Run()
	testutils.FatalIfError(t, err, "Failed to export data")

	// Enable failpoint to inject commit error
	fpEnv := testutils.GetFailpointEnvVar(
		"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb/importBatchCommitError=4*off->return()",
	)

	t.Log("Running import with failpoint enabled...")
	lm.WithEnv(fpEnv, "YB_VOYAGER_COPY_MAX_RETRY_COUNT=1")
	err = lm.StartImportData(false, map[string]string{
		"--batch-size": "2", "--parallel-jobs": "1", "--adaptive-parallelism": "disabled",
	})
	assert.Error(t, err, "Expected import to fail due to failpoint injection")
	assert.Contains(t, lm.GetImportCommandStderr(), "failpoint", "Error should mention failpoint")

	// Verify partial progress on target after failpoint-induced failure
	ybConn, err := lm.GetTargetConnection()
	testutils.FatalIfError(t, err, "Failed to get YugabyteDB connection")
	defer ybConn.Close()

	var count int
	err = ybConn.QueryRow("SELECT COUNT(*) FROM test_schema.test_failpoint").Scan(&count)
	testutils.FatalIfError(t, err, "Failed to query row count")
	assert.Equal(t, 8, count, "Expected 8 rows (4 batches succeeded before failpoint triggered)")

	// Now resume import WITHOUT failpoint (should succeed)
	t.Log("Resuming import without failpoint...")
	lm.ClearEnv()
	err = lm.StartImportData(false, nil)
	testutils.FatalIfError(t, err, "Failed to resume import")

	// Verify data was successfully imported this time
	err = ybConn.QueryRow("SELECT COUNT(*) FROM test_schema.test_failpoint").Scan(&count)
	testutils.FatalIfError(t, err, "Failed to query row count after resume")
	assert.Equal(t, 20, count, "All 20 rows should be imported after successful resume")

	// Compare data between source and target
	pgConn, err := lm.GetSourceConnection()
	testutils.FatalIfError(t, err, "Failed to get Postgres connection")
	defer pgConn.Close()

	err = testutils.CompareTableData(ctx, pgConn, ybConn, "test_schema.test_failpoint", "id")
	assert.NoError(t, err, "Data should match between source and target after recovery")
	t.Logf("All data correctly imported and matches source")
}
