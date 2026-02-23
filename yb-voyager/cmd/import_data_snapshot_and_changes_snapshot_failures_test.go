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
	"database/sql"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// TestImportSnapshotCommitFailureAndResume verifies that live migration `import data`
// can resume after a snapshot batch commit failure during the snapshot apply phase.
//
// Scenario:
//  1. Start export and import concurrently. Import hits a failpoint that injects a snapshot
//     batch commit error after N successful batches, and crashes mid-snapshot.
//  2. Verify partial snapshot progress on target (row count reflects committed batches).
//  3. Generate CDC changes on source (export is still streaming).
//  4. Resume `import data` without failpoint and verify target matches source.
//
// Injection point:
// - `src/tgtdb/yugabytedb.go` in transactional COPY path, right before txn commit:
//   failpoint `importBatchCommitError`.
func TestImportSnapshotCommitFailureAndResume(t *testing.T) {
	ctx := context.Background()
	tableName := "test_schema_import_snap_fail.snapshot_import_test"

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test_import_snap_fail",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test_import_snap_fail",
		},
		SchemaNames: []string{"test_schema_import_snap_fail"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema_import_snap_fail;`,
			`CREATE TABLE test_schema_import_snap_fail.snapshot_import_test (
				id INTEGER PRIMARY KEY,
				name TEXT
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema_import_snap_fail.snapshot_import_test REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema_import_snap_fail.snapshot_import_test (id, name)
			 SELECT i, 'row_' || i FROM generate_series(1, 60) i;`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema_import_snap_fail.snapshot_import_test (id, name)
			 VALUES (1001, 'cdc_ins_1001'), (1002, 'cdc_ins_1002'), (1003, 'cdc_ins_1003');`,
			`UPDATE test_schema_import_snap_fail.snapshot_import_test SET name='cdc_upd_1' WHERE id=1;`,
			`UPDATE test_schema_import_snap_fail.snapshot_import_test SET name='cdc_upd_3' WHERE id=3;`,
			`DELETE FROM test_schema_import_snap_fail.snapshot_import_test WHERE id=2;`,
		},
		CleanupSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema_import_snap_fail CASCADE;`,
		},
	})
	defer lm.Cleanup()

	err := lm.SetupContainers(ctx)
	require.NoError(t, err, "failed to setup containers")
	err = lm.SetupSchema()
	require.NoError(t, err, "failed to setup schema")

	// --- Phase 1: Start export and import concurrently ---
	err = lm.StartExportData(true, nil)
	require.NoError(t, err, "failed to start export")
	defer killDebeziumForExportDir(t, lm.GetExportDir())

	const (
		batchSizeRows      = 2
		successBatchesThen = 10 // inject on (successBatchesThen+1)-th commit attempt
	)
	expectedRowsAfterFailure := batchSizeRows * successBatchesThen

	failpointEnv := testutils.GetFailpointEnvVar(
		"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb/importBatchCommitError=10*off->return()",
	)

	testutils.LogTestf(t, "Starting import with snapshot commit failpoint (expected to crash after %d committed batches)...", successBatchesThen)
	err = lm.StartImportDataWithEnv(true, map[string]string{
		"--batch-size":           "2",
		"--parallel-jobs":        "1",
		"--adaptive-parallelism": "disabled",
	}, []string{
		failpointEnv,
		"YB_VOYAGER_COPY_MAX_RETRY_COUNT=1",
	})
	require.NoError(t, err, "failed to start import with failpoint")

	testutils.LogTest(t, "Waiting for import to crash due to snapshot commit failpoint...")
	_, waitErr := testutils.WaitForProcessExitOrKill(lm.GetImportRunner(), 120*time.Second)
	require.Error(t, waitErr, "Expected import to exit with error after snapshot commit failpoint")
	require.Contains(t, lm.GetImportCommandStderr(), "failpoint", "Expected failpoint mention in import stderr")

	// --- Phase 2: Verify partial snapshot progress on target ---
	err = lm.WithTargetConn(func(ybConn *sql.DB) error {
		var rowsAfterFailure int
		require.NoError(t, ybConn.QueryRow("SELECT COUNT(*) FROM "+tableName).Scan(&rowsAfterFailure))
		testutils.LogTestf(t, "Target snapshot row count after failure: %d (expected %d)", rowsAfterFailure, expectedRowsAfterFailure)
		require.Equal(t, expectedRowsAfterFailure, rowsAfterFailure, "Expected deterministic partial snapshot progress before failure")

		var distinctIDs int
		require.NoError(t, ybConn.QueryRow("SELECT COUNT(DISTINCT id) FROM "+tableName).Scan(&distinctIDs))
		require.Equal(t, rowsAfterFailure, distinctIDs, "Expected no duplicate ids in partially imported snapshot")

		var outOfRange int
		require.NoError(t, ybConn.QueryRow("SELECT COUNT(*) FROM "+tableName+" WHERE id < 1 OR id > 60").Scan(&outOfRange))
		require.Equal(t, 0, outOfRange, "Expected imported ids to be within [1,60]")

		var missingIDs int
		require.NoError(t, ybConn.QueryRow(`SELECT COUNT(*) FROM (
			SELECT i FROM generate_series(1, 60) AS i
			EXCEPT SELECT id FROM `+tableName+`) AS missing`).Scan(&missingIDs))
		testutils.LogTestf(t, "After snapshot failure: imported=%d missing=%d", rowsAfterFailure, missingIDs)
		require.Equal(t, 60-rowsAfterFailure, missingIDs, "Expected missing ids count to match partial snapshot row count")
		return nil
	})
	require.NoError(t, err)

	// --- Phase 3: Generate CDC changes and resume import ---
	testutils.LogTest(t, "Generating CDC changes on source...")
	lm.ExecuteSourceDelta()
	testutils.LogTest(t, "Resuming import without failpoint...")
	err = lm.StartImportData(true, nil)
	require.NoError(t, err, "Failed to start import resume")
	defer lm.StopImportData()

	err = lm.WithSourceTargetConn(func(pgConn, ybConn *sql.DB) error {
		require.Eventually(t, func() bool {
			return testutils.CompareTableData(ctx, pgConn, ybConn, tableName, "id") == nil
		}, 180*time.Second, 5*time.Second, "Timed out waiting for snapshot import resume to catch up")
		return nil
	})
	require.NoError(t, err)

	testutils.LogTest(t, "Target matches source after resume (snapshot commit failure)")
}

// TestImportSnapshotTransformFailureAndResume verifies that live migration `import data` can resume
// after a deterministic snapshot row "transform" failure during snapshot batch production.
//
// Scenario:
// 1. Run `export data --export-type snapshot-and-changes` and wait for streaming mode (snapshot exported).
// 2. Generate a small CDC workload and wait until those events are queued.
// 3. Stop export to freeze the exportDir.
// 4. Run `import data` with a failpoint that injects a per-row failure while producing snapshot batches.
// 5. Verify partial snapshot progress + source != target after failure.
// 6. Resume `import data` without failpoint and verify target matches source (includes CDC changes).
//
// Injection point:
// - `cmd/importDataSequentialFileBatchProducer.go` in `transformRow()` via failpoint `importSnapshotTransformError`.
func TestImportSnapshotTransformFailureAndResume(t *testing.T) {
	ctx := context.Background()

	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)
	testutils.LogTestf(t, "Using exportDir=%s", exportDir)

	postgresContainer := testcontainers.NewTestContainer("postgresql", &testcontainers.ContainerConfig{
		ForLive: true,
	})
	err := postgresContainer.Start(ctx)
	require.NoError(t, err, "Failed to start PostgreSQL container")
	defer postgresContainer.Stop(ctx)

	yugabytedbContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	err = yugabytedbContainer.Start(ctx)
	require.NoError(t, err, "Failed to start YugabyteDB container")
	defer yugabytedbContainer.Stop(ctx)

	postgresContainer.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema_import_snap_transform_fail CASCADE;",
		"CREATE SCHEMA test_schema_import_snap_transform_fail;",
		`CREATE TABLE test_schema_import_snap_transform_fail.snapshot_import_test (
			id INTEGER PRIMARY KEY,
			name TEXT
		);`,
		`ALTER TABLE test_schema_import_snap_transform_fail.snapshot_import_test REPLICA IDENTITY FULL;`,
		`INSERT INTO test_schema_import_snap_transform_fail.snapshot_import_test (id, name)
		 SELECT i, 'row_' || i FROM generate_series(1, 60) i;`,
	)
	defer postgresContainer.ExecuteSqls("DROP SCHEMA IF EXISTS test_schema_import_snap_transform_fail CASCADE;")

	yugabytedbContainer.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema_import_snap_transform_fail CASCADE;",
		"CREATE SCHEMA test_schema_import_snap_transform_fail;",
		`CREATE TABLE test_schema_import_snap_transform_fail.snapshot_import_test (
			id INTEGER PRIMARY KEY,
			name TEXT
		);`,
	)
	defer yugabytedbContainer.ExecuteSqls("DROP SCHEMA IF EXISTS test_schema_import_snap_transform_fail CASCADE;")

	exportReady := make(chan bool, 1)
	waitForExport := func() {
		testutils.LogTest(t, "Waiting for export to enter streaming mode (snapshot exported)...")
		require.NoError(t, waitForStreamingMode(exportDir, 120*time.Second, 2*time.Second), "Export should enter streaming mode")

		testutils.LogTest(t, "Export reached streaming mode; generating a few CDC changes...")
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema_import_snap_transform_fail.snapshot_import_test (id, name)
			 VALUES (1001, 'cdc_ins_1001'), (1002, 'cdc_ins_1002'), (1003, 'cdc_ins_1003');`,
			`UPDATE test_schema_import_snap_transform_fail.snapshot_import_test SET name='cdc_upd_1' WHERE id=1;`,
			`UPDATE test_schema_import_snap_transform_fail.snapshot_import_test SET name='cdc_upd_3' WHERE id=3;`,
			`DELETE FROM test_schema_import_snap_transform_fail.snapshot_import_test WHERE id=2;`,
		)

		testutils.LogTest(t, "Waiting for CDC events to be queued...")
		waitForCDCEventCount(t, exportDir, 6, 180*time.Second, 5*time.Second)
		testutils.LogTest(t, "Verifying no duplicate event_id values in queued CDC...")
		verifyNoEventIDDuplicates(t, exportDir)

		exportReady <- true
	}

	exportRunner := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_import_snap_transform_fail",
		"--disable-pb", "true",
		"--yes",
	}, waitForExport, true)
	err = exportRunner.Run()
	require.NoError(t, err, "Failed to start export")
	defer killDebeziumForExportDir(t, exportDir)

	select {
	case <-exportReady:
	case <-time.After(180 * time.Second):
		_ = exportRunner.Kill()
		require.Fail(t, "Timed out waiting for export streaming mode")
	}

	testutils.LogTest(t, "Stopping export to freeze exportDir before snapshot import")
	_ = exportRunner.Kill()
	killDebeziumForExportDir(t, exportDir)
	_ = os.Remove(filepath.Join(exportDir, ".export-dataLockfile.lck"))
	time.Sleep(2 * time.Second)

	const expectedRowsTarget = 0

	failpointEnv := testutils.GetFailpointEnvVar(
		"github.com/yugabyte/yb-voyager/yb-voyager/cmd/importSnapshotTransformError=20*off->return(true)",
	)

	testutils.LogTest(t, "Running import with snapshot transform failpoint (expected to crash mid-snapshot)...")
	importWithFailpoint := testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--batch-size", "2",
		"--parallel-jobs", "1",
		"--adaptive-parallelism", "disabled",
		"--yes",
	}, nil, true).WithEnv(
		failpointEnv,
		"YB_VOYAGER_COPY_MAX_RETRY_COUNT=1",
		"YB_VOYAGER_FAILPOINT_MARKER_DIR="+filepath.Join(exportDir, "logs"),
	)
	err = importWithFailpoint.Run()
	require.NoError(t, err, "Failed to start import with failpoint")

	failMarkerPath := filepath.Join(exportDir, "logs", "failpoint-import-snapshot-transform-error.log")
	testutils.LogTestf(t, "Waiting for failpoint marker: %s", failMarkerPath)
	matched, err := testutils.WaitForFailpointMarker(failMarkerPath, 60*time.Second, 2*time.Second)
	require.NoError(t, err, "Should be able to read snapshot transform failure marker")
	if !matched {
		_ = importWithFailpoint.Kill()
		require.Fail(t, "Snapshot transform failure marker did not trigger (failpoints may not be enabled). "+
			"Make sure to run `failpoint-ctl enable` before `go test -tags=failpoint` and use a failpoint-enabled yb-voyager binary.")
	}

	_, waitErr := testutils.WaitForProcessExitOrKill(importWithFailpoint, 60*time.Second)
	require.Error(t, waitErr, "Import should exit with error after snapshot transform failpoint")
	require.Contains(t, importWithFailpoint.Stderr(), "failpoint", "Expected failpoint mention in import stderr")
	_ = os.Remove(filepath.Join(exportDir, ".import-dataLockfile.lck"))

	// Production-path verification: batch production writes temp batch files under import_data_state before
	// a batch is finalized. For a transform failure mid-production, we expect at least one `tmp::<N>` file.
	stateRoot := filepath.Join(exportDir, "metainfo", "import_data_state")
	tmpFiles := []string{}
	err = filepath.WalkDir(stateRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasPrefix(d.Name(), "tmp::") {
			tmpFiles = append(tmpFiles, path)
		}
		return nil
	})
	require.NoError(t, err, "Failed to walk import_data_state for tmp batch files")
	require.Greater(t, len(tmpFiles), 0, "Expected at least one tmp::<N> batch file after transform failure")
	info, statErr := os.Stat(tmpFiles[0])
	require.NoError(t, statErr, "Failed to stat tmp batch file")
	testutils.LogTestf(t, "Found %d tmp batch files; example=%s size=%d", len(tmpFiles), tmpFiles[0], info.Size())
	// Note: the tmp batch file is written via a buffered writer. On a mid-batch failure (like our
	// transform failpoint), the file may not be flushed/closed, so its on-disk size can be 0 even
	// though rows were read and processing progressed. Existence is the reliable signal here.
	require.True(t, info.Mode().IsRegular(), "Expected tmp batch file to be a regular file")
	require.Contains(t, importWithFailpoint.Stderr(), "transforming line number=", "Expected stderr to show transform progressed to a specific line")

	ybConnForChecks, err := yugabytedbContainer.GetConnection()
	require.NoError(t, err, "Failed to get YugabyteDB connection for mid-test checks")
	defer ybConnForChecks.Close()

	var rowsAfterFailure int
	err = ybConnForChecks.QueryRow("SELECT COUNT(*) FROM test_schema_import_snap_transform_fail.snapshot_import_test").Scan(&rowsAfterFailure)
	require.NoError(t, err, "Failed to query target row count after snapshot transform failure")
	testutils.LogTestf(t, "Target snapshot row count after failure: %d (expected %d)", rowsAfterFailure, expectedRowsTarget)
	require.Equal(t, expectedRowsTarget, rowsAfterFailure, "Expected no snapshot rows imported when batch production fails")

	var distinctIDs int
	err = ybConnForChecks.QueryRow("SELECT COUNT(DISTINCT id) FROM test_schema_import_snap_transform_fail.snapshot_import_test").Scan(&distinctIDs)
	require.NoError(t, err, "Failed to query distinct id count after snapshot transform failure")
	require.Equal(t, rowsAfterFailure, distinctIDs, "Expected no duplicate ids in partially imported snapshot")

	var outOfRange int
	err = ybConnForChecks.QueryRow(
		"SELECT COUNT(*) FROM test_schema_import_snap_transform_fail.snapshot_import_test WHERE id < 1 OR id > 1003",
	).Scan(&outOfRange)
	require.NoError(t, err, "Failed to query out-of-range id count after snapshot transform failure")
	require.Equal(t, 0, outOfRange, "Expected imported ids to be within expected range")

	var missingIDs int
	err = ybConnForChecks.QueryRow(
		`SELECT COUNT(*) FROM (
			SELECT i FROM generate_series(1, 60) AS i
			EXCEPT
			SELECT id FROM test_schema_import_snap_transform_fail.snapshot_import_test
		) AS missing`,
	).Scan(&missingIDs)
	require.NoError(t, err, "Failed to compute missing ids after snapshot transform failure")
	testutils.LogTestf(t, "After snapshot transform failure: imported=%d missing=%d", rowsAfterFailure, missingIDs)
	require.Equal(t, 60-rowsAfterFailure, missingIDs, "Expected missing ids count to match partial snapshot row count")

	pgConnForMismatch, err := postgresContainer.GetConnection()
	require.NoError(t, err, "Failed to get PostgreSQL connection for mismatch check")
	defer pgConnForMismatch.Close()
	require.Error(t, testutils.CompareTableData(ctx, pgConnForMismatch, ybConnForChecks, "test_schema_import_snap_transform_fail.snapshot_import_test", "id"),
		"Expected source != target after snapshot transform failure (resume should be required)")

	testutils.LogTest(t, "Resuming import without failpoint and waiting for target to match source...")
	importResume := testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--yes",
	}, nil, true)
	err = importResume.Run()
	require.NoError(t, err, "Failed to start import resume")
	defer importResume.Kill()

	pgConn, err := postgresContainer.GetConnection()
	require.NoError(t, err, "Failed to get PostgreSQL connection")
	defer pgConn.Close()

	ybConn, err := yugabytedbContainer.GetConnection()
	require.NoError(t, err, "Failed to get YugabyteDB connection")
	defer ybConn.Close()

	require.Eventually(t, func() bool {
		return testutils.CompareTableData(ctx, pgConn, ybConn, "test_schema_import_snap_transform_fail.snapshot_import_test", "id") == nil
	}, 240*time.Second, 5*time.Second, "Timed out waiting for snapshot import resume to catch up")

	testutils.LogTest(t, "✓ Target matches source after resume (snapshot transform failure)")

	// best-effort shutdown
	_ = importResume.Kill()
	_ = os.Remove(filepath.Join(exportDir, ".import-dataLockfile.lck"))
}
