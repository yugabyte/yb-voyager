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


	const (
		batchSizeRows      = 2
		successBatchesThen = 10 // inject on (successBatchesThen+1)-th commit attempt
	)
	expectedRowsAfterFailure := batchSizeRows * successBatchesThen

	failpointEnv := testutils.GetFailpointEnvVar(
		// Skip the first 10 batch commits (let them succeed), then inject a commit
		// error on the 11th. With batch-size=2 and 60 rows, 10 successful batches
		// import 20 rows before the crash.
		"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb/importBatchCommitError=10*off->return()",
	)

	t.Logf("Starting import with snapshot commit failpoint (expected to crash after %d committed batches)...", successBatchesThen)
	err = lm.StartImportDataWithEnv(true, map[string]string{
		"--batch-size":           "2",
		"--parallel-jobs":        "1",
		"--adaptive-parallelism": "disabled",
	}, []string{
		failpointEnv,
		"YB_VOYAGER_COPY_MAX_RETRY_COUNT=1",
	})
	require.NoError(t, err, "failed to start import with failpoint")

	t.Log("Waiting for import to crash due to snapshot commit failpoint...")
	_, waitErr := testutils.WaitForProcessExitOrKill(lm.GetImportRunner(), 120*time.Second)
	require.Error(t, waitErr, "Expected import to exit with error after snapshot commit failpoint")
	require.Contains(t, lm.GetImportCommandStderr(), "failpoint", "Expected failpoint mention in import stderr")

	// --- Phase 2: Verify partial snapshot progress on target ---
	err = lm.WithTargetConn(func(ybConn *sql.DB) error {
		var rowsAfterFailure int
		require.NoError(t, ybConn.QueryRow("SELECT COUNT(*) FROM "+tableName).Scan(&rowsAfterFailure))
		t.Logf("Target snapshot row count after failure: %d (expected %d)", rowsAfterFailure, expectedRowsAfterFailure)
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
		t.Logf("After snapshot failure: imported=%d missing=%d", rowsAfterFailure, missingIDs)
		require.Equal(t, 60-rowsAfterFailure, missingIDs, "Expected missing ids count to match partial snapshot row count")
		return nil
	})
	require.NoError(t, err)

	// --- Phase 3: Generate CDC changes and resume import ---
	t.Log("Generating CDC changes on source...")
	lm.ExecuteSourceDelta()
	t.Log("Resuming import without failpoint...")
	err = lm.StartImportData(true, nil)
	require.NoError(t, err, "Failed to start import resume")
	defer lm.StopImportData()

	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		reportTableName(tableName): {Inserts: 3, Updates: 2, Deletes: 1},
	}, 120, 5)
	require.NoError(t, err, "Migration report did not match expected CDC counts")

	err = lm.ValidateDataConsistency([]string{tableName}, "id")
	require.NoError(t, err, "Target does not match source after resume")
}

// TestImportSnapshotTransformFailureAndResume verifies that live migration `import data` can resume
// after a deterministic snapshot row "transform" failure during snapshot batch production.
//
// Scenario:
//  1. Start export and import concurrently. Import hits a failpoint that injects a per-row failure
//     while producing snapshot batches, and crashes before any batch is committed.
//  2. Verify zero rows on target + tmp batch files exist in import_data_state.
//  3. Generate CDC changes on source (export is still streaming).
//  4. Resume `import data` without failpoint and verify target matches source.
//
// Injection point:
// - `cmd/importDataSequentialFileBatchProducer.go` in `transformRow()` via failpoint `importSnapshotTransformError`.
func TestImportSnapshotTransformFailureAndResume(t *testing.T) {
	ctx := context.Background()
	tableName := "test_schema_import_snap_transform_fail.snapshot_import_test"

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test_import_snap_transform_fail",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test_import_snap_transform_fail",
		},
		SchemaNames: []string{"test_schema_import_snap_transform_fail"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema_import_snap_transform_fail;`,
			`CREATE TABLE test_schema_import_snap_transform_fail.snapshot_import_test (
				id INTEGER PRIMARY KEY,
				name TEXT
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema_import_snap_transform_fail.snapshot_import_test REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema_import_snap_transform_fail.snapshot_import_test (id, name)
			 SELECT i, 'row_' || i FROM generate_series(1, 60) i;`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema_import_snap_transform_fail.snapshot_import_test (id, name)
			 VALUES (1001, 'cdc_ins_1001'), (1002, 'cdc_ins_1002'), (1003, 'cdc_ins_1003');`,
			`UPDATE test_schema_import_snap_transform_fail.snapshot_import_test SET name='cdc_upd_1' WHERE id=1;`,
			`UPDATE test_schema_import_snap_transform_fail.snapshot_import_test SET name='cdc_upd_3' WHERE id=3;`,
			`DELETE FROM test_schema_import_snap_transform_fail.snapshot_import_test WHERE id=2;`,
		},
		CleanupSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema_import_snap_transform_fail CASCADE;`,
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


	failpointEnv := testutils.GetFailpointEnvVar(
		// Skip the first 20 per-row transform calls (let them succeed), then inject
		// a transform error on the 21st row. With batch-size=100, all 20 rows fit
		// in the first batch which is never finalized, so zero rows reach the target.
		"github.com/yugabyte/yb-voyager/yb-voyager/cmd/importSnapshotTransformError=20*off->return(true)",
	)
	failMarkerPath := filepath.Join(lm.GetCurrentExportDir(), "failpoints", "failpoint-import-snapshot-transform-error.log")

	t.Log("Starting import with snapshot transform failpoint...")
	err = lm.StartImportDataWithEnv(true, map[string]string{
		"--batch-size":           "100",
		"--parallel-jobs":        "1",
		"--adaptive-parallelism": "disabled",
	}, []string{
		failpointEnv,
		"YB_VOYAGER_COPY_MAX_RETRY_COUNT=1",
		"YB_VOYAGER_FAILPOINT_MARKER_DIR=" + filepath.Join(lm.GetCurrentExportDir(), "failpoints"),
	})
	require.NoError(t, err, "failed to start import with failpoint")

	t.Log("Waiting for import to crash via failpoint...")
	require.NoError(t, lm.WaitForImportFailpointAndProcessCrash(t, failMarkerPath, 60*time.Second, 60*time.Second))
	require.Contains(t, lm.GetImportCommandStderr(), "failpoint", "Expected failpoint mention in import stderr")
	require.Contains(t, lm.GetImportCommandStderr(), "transforming line number=", "Expected stderr to show transform progressed to a specific line")

	// --- Phase 2: Verify crash artifacts ---
	// Batch production writes temp batch files under import_data_state before a batch is finalized.
	// For a transform failure mid-production, we expect at least one `tmp::<N>` file.
	stateRoot := filepath.Join(lm.GetCurrentExportDir(), "metainfo", "import_data_state")
	tmpFiles := []string{}
	err = filepath.WalkDir(stateRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() && strings.HasPrefix(d.Name(), "tmp::") {
			tmpFiles = append(tmpFiles, path)
		}
		return nil
	})
	require.NoError(t, err, "Failed to walk import_data_state for tmp batch files")
	require.Greater(t, len(tmpFiles), 0, "Expected at least one tmp::<N> batch file after transform failure")
	info, statErr := os.Stat(tmpFiles[0])
	require.NoError(t, statErr, "Failed to stat tmp batch file")
	t.Logf("Found %d tmp batch files; example=%s size=%d", len(tmpFiles), tmpFiles[0], info.Size())
	require.True(t, info.Mode().IsRegular(), "Expected tmp batch file to be a regular file")

	// Transform failure crashes before any batch is committed, so zero rows on target.
	err = lm.WithTargetConn(func(ybConn *sql.DB) error {
		var rowCount int
		require.NoError(t, ybConn.QueryRow("SELECT COUNT(*) FROM "+tableName).Scan(&rowCount))
		t.Logf("Target snapshot row count after failure: %d (expected 0)", rowCount)
		require.Equal(t, 0, rowCount, "Expected no snapshot rows imported when batch production fails")
		return nil
	})
	require.NoError(t, err)

	// --- Phase 3: Generate CDC changes and resume import ---
	t.Log("Generating CDC changes on source...")
	lm.ExecuteSourceDelta()

	t.Log("Resuming import without failpoint...")
	err = lm.StartImportData(true, nil)
	require.NoError(t, err, "Failed to start import resume")
	defer lm.StopImportData()

	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		reportTableName(tableName): {Inserts: 3, Updates: 2, Deletes: 1},
	}, 120, 5)
	require.NoError(t, err, "Migration report did not match expected CDC counts")

	err = lm.ValidateDataConsistency([]string{tableName}, "id")
	require.NoError(t, err, "Target does not match source after resume")
}
