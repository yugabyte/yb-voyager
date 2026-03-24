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
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// TestImportCDCTransformFailureAndResume verifies that live migration `import data` can resume after
// a CDC-transform failure during the streaming apply phase.
//
// Scenario:
// 1. Start `export data` and `import data` concurrently (import runs with failpoint enabled).
// 2. Wait for the snapshot phase to complete via the framework's WaitForSnapshotComplete.
// 3. Generate CDC changes (INSERT/UPDATE/DELETE) on the source via SourceDeltaSQL.
// 4. Wait for the failpoint to fire during CDC streaming, causing import to crash.
// 5. Verify partial CDC progress on target (export keeps running throughout).
// 6. Resume `import data` without failpoint and verify target matches source.
//
// This test validates:
// - CDC apply resumability after transformation failure
// - Idempotent resume (final target state matches source exactly)
//
// Notes on determinism:
//   - Failpoint triggers in `cmd/live_migration.go` inside `handleEvent()` after ConvertEvent().
//   - We set small buffering/batching via env to ensure `processEvents()` commits some CDC before
//     the failpoint triggers, so `last_applied_vsn > 0` is observable:
//   - MAX_EVENTS_PER_BATCH=1
//   - MAX_INTERVAL_BETWEEN_BATCHES=1
//   - EVENT_CHANNEL_SIZE=2
func TestImportCDCTransformFailureAndResume(t *testing.T) {
	ctx := context.Background()
	tableName := "test_schema_import_cdc.cdc_import_test"

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test_import_cdc_transform",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test_import_cdc_transform",
		},
		SchemaNames: []string{"test_schema_import_cdc"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema_import_cdc;`,
			`CREATE TABLE test_schema_import_cdc.cdc_import_test (
				id SERIAL PRIMARY KEY,
				name TEXT,
				value INTEGER
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema_import_cdc.cdc_import_test REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema_import_cdc.cdc_import_test (name, value)
			 SELECT 'snapshot_' || i, i FROM generate_series(1, 30) i;`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema_import_cdc.cdc_import_test (name, value)
			 SELECT 'cdc_ins_' || i, 1000 + i FROM generate_series(1, 200) i;`,
			`UPDATE test_schema_import_cdc.cdc_import_test
			 SET value = value + 10000
			 WHERE id BETWEEN 1 AND 200;`,
			`DELETE FROM test_schema_import_cdc.cdc_import_test WHERE id BETWEEN 11 AND 110;`,
		},
		CleanupSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema_import_cdc CASCADE;`,
		},
	})
	defer lm.Cleanup()

	err := lm.SetupContainers(ctx)
	require.NoError(t, err, "failed to setup containers")
	err = lm.SetupSchema()
	require.NoError(t, err, "failed to setup schema")

	// --- Phase 1: Start export and import concurrently, wait for snapshot, then generate CDC ---
	err = lm.StartExportData(true, nil)
	require.NoError(t, err, "failed to start export")

	failpointEnv := testutils.GetFailpointEnvVar(
		// Skip the first 250 transform calls (let them succeed), then trigger a
		// transform failure on the 251st call. 250 is chosen to be large enough
		// that meaningful CDC progress is observable before the crash.
		"github.com/yugabyte/yb-voyager/yb-voyager/cmd/importCDCTransformFailure=250*off->return()",
	)
	err = lm.StartImportDataWithEnv(true, nil, []string{
		failpointEnv,
		"MAX_EVENTS_PER_BATCH=1",
		"MAX_INTERVAL_BETWEEN_BATCHES=1",
		"EVENT_CHANNEL_SIZE=2",
	})
	require.NoError(t, err, "failed to start import with failpoint")

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"test_schema_import_cdc"."cdc_import_test"`: 30,
	}, 120)
	require.NoError(t, err, "snapshot phase did not complete")
	t.Log("Snapshot complete; generating CDC changes...")

	lm.ExecuteSourceDelta()

	// --- Phase 2: Wait for failpoint to fire and import to crash ---
	failMarkerPath := filepath.Join(lm.GetCurrentExportDir(), "failpoints", "failpoint-import-cdc-transform.log")
	err = lm.WaitForImportFailpointAndProcessCrash(t, failMarkerPath, 120*time.Second, 60*time.Second)
	require.NoError(t, err, "Import should crash after CDC transform failpoint")

	// Verify partial CDC progress: some events applied, but not all queued ones.
	migrationUUIDStr, err := lm.ReadMigrationUUID()
	require.NoError(t, err, "Failed to read migration UUID")
	require.NotEmpty(t, migrationUUIDStr)

	maxQueuedVsn, err := maxVsnInQueueSegments(lm.GetCurrentExportDir())
	require.NoError(t, err, "Failed to compute max queued vsn from queue segments")
	t.Logf("Max queued vsn in CDC segments: %d", maxQueuedVsn)

	err = lm.WithTargetConn(func(ybConn *sql.DB) error {
		lastAppliedVsn, err := maxLastAppliedVsn(ybConn, migrationUUIDStr)
		require.NoError(t, err, "Failed to read last_applied_vsn from event channels metadata")
		t.Logf("Max last_applied_vsn on target after failure: %d", lastAppliedVsn)
		require.Greater(t, lastAppliedVsn, int64(0), "Expected some CDC progress before failure")
		require.Less(t, lastAppliedVsn, maxQueuedVsn, "Expected partial progress (not all events applied)")
		return nil
	})
	require.NoError(t, err)

	// --- Phase 3: Resume import without failpoint (export still running) ---
	t.Log("Resuming import without failpoint...")
	err = lm.StartImportDataWithEnv(true, nil, []string{
		"MAX_EVENTS_PER_BATCH=1",
		"MAX_INTERVAL_BETWEEN_BATCHES=1",
		"EVENT_CHANNEL_SIZE=2",
	})
	require.NoError(t, err, "Failed to start import resume")
	defer lm.StopImportData()

	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		reportTableName(tableName): {Inserts: 200, Updates: 200, Deletes: 100},
	}, 120, 5)
	require.NoError(t, err, "Migration report did not match expected CDC counts")

	err = lm.ValidateDataConsistency([]string{tableName}, "id")
	require.NoError(t, err, "Target does not match source after resume")
}

// TestImportCDCDbErrorAndResume verifies that live migration `import data` can resume after
// a CDC apply failure caused by a simulated target DB error during the streaming apply phase.
//
// Scenario:
//  1. Start `export data` and `import data` concurrently (import runs with failpoint enabled).
//  2. Wait for the snapshot phase to complete via the framework's WaitForSnapshotComplete.
//  3. Generate CDC changes (INSERT/UPDATE/DELETE) on the source via SourceDeltaSQL.
//  4. Wait for the failpoint to fire during CDC streaming, causing import to crash.
//  5. Verify partial CDC progress on target (export keeps running throughout).
//  6. Resume `import data` without failpoint and verify target matches source.
//
// This test validates:
// - CDC apply resumability after a target DB error during batch execution
// - Idempotent resume (final target state matches source exactly)
//
// Notes on determinism:
// - Failpoint triggers in `cmd/live_migration.go` inside `processEvents()` before ExecuteBatch.
// - We set NUM_EVENT_CHANNELS=1 and MAX_EVENTS_PER_BATCH=1 so "Nth batch" is deterministic.
func TestImportCDCDbErrorAndResume(t *testing.T) {
	ctx := context.Background()
	tableName := "test_schema_import_cdc_db_err.cdc_import_test"

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test_import_cdc_db_err",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test_import_cdc_db_err",
		},
		SchemaNames: []string{"test_schema_import_cdc_db_err"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema_import_cdc_db_err;`,
			`CREATE TABLE test_schema_import_cdc_db_err.cdc_import_test (
				id SERIAL PRIMARY KEY,
				name TEXT,
				value INTEGER
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema_import_cdc_db_err.cdc_import_test REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema_import_cdc_db_err.cdc_import_test (name, value)
			 SELECT 'snapshot_' || i, i FROM generate_series(1, 30) i;`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema_import_cdc_db_err.cdc_import_test (name, value)
			 SELECT 'cdc_ins_' || i, 1000 + i FROM generate_series(1, 200) i;`,
			`UPDATE test_schema_import_cdc_db_err.cdc_import_test
			 SET value = value + 10000
			 WHERE id BETWEEN 1 AND 200;`,
			`DELETE FROM test_schema_import_cdc_db_err.cdc_import_test WHERE id BETWEEN 11 AND 110;`,
		},
		CleanupSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema_import_cdc_db_err CASCADE;`,
		},
	})
	defer lm.Cleanup()

	err := lm.SetupContainers(ctx)
	require.NoError(t, err, "failed to setup containers")
	err = lm.SetupSchema()
	require.NoError(t, err, "failed to setup schema")

	// --- Phase 1: Start export and import concurrently, wait for snapshot, then generate CDC ---
	err = lm.StartExportData(true, nil)
	require.NoError(t, err, "failed to start export")

	failpointEnv := testutils.GetFailpointEnvVar(
		// Skip the first 100 batch-commit calls (let them succeed), then inject a
		// DB error on the 101st. 100 is chosen so `last_applied_vsn > 0` is
		// observable and we can prove resume work is meaningful.
		"github.com/yugabyte/yb-voyager/yb-voyager/cmd/importCDCNonRetryableBatchDBError=100*off->return(true)",
	)
	err = lm.StartImportDataWithEnv(true, map[string]string{
		"--max-retries-streaming": "1",
	}, []string{
		failpointEnv,
		"NUM_EVENT_CHANNELS=1",
		"MAX_EVENTS_PER_BATCH=1",
		"MAX_INTERVAL_BETWEEN_BATCHES=1",
		"EVENT_CHANNEL_SIZE=2",
	})
	require.NoError(t, err, "failed to start import with failpoint")

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"test_schema_import_cdc_db_err"."cdc_import_test"`: 30,
	}, 120)
	require.NoError(t, err, "snapshot phase did not complete")
	t.Log("Snapshot complete; generating CDC changes...")

	lm.ExecuteSourceDelta()

	// --- Phase 2: Wait for failpoint to fire and import to crash ---
	failMarkerPath := filepath.Join(lm.GetCurrentExportDir(), "failpoints", "failpoint-import-cdc-non-retryable-batch-db-error.log")
	err = lm.WaitForImportFailpointAndProcessCrash(t, failMarkerPath, 120*time.Second, 60*time.Second)
	require.NoError(t, err, "Import should crash after CDC DB-error failpoint")

	// Verify partial CDC progress: some events applied, but not all queued ones.
	migrationUUIDStr, err := lm.ReadMigrationUUID()
	require.NoError(t, err, "Failed to read migration UUID")
	require.NotEmpty(t, migrationUUIDStr)

	maxQueuedVsn, err := maxVsnInQueueSegments(lm.GetCurrentExportDir())
	require.NoError(t, err, "Failed to compute max queued vsn from queue segments")
	t.Logf("Max queued vsn in CDC segments: %d", maxQueuedVsn)

	err = lm.WithTargetConn(func(ybConn *sql.DB) error {
		lastAppliedVsn, err := maxLastAppliedVsn(ybConn, migrationUUIDStr)
		require.NoError(t, err, "Failed to read last_applied_vsn from event channels metadata")
		t.Logf("Max last_applied_vsn on target after failure: %d", lastAppliedVsn)
		require.Greater(t, lastAppliedVsn, int64(0), "Expected some CDC progress before failure")
		require.Less(t, lastAppliedVsn, maxQueuedVsn, "Expected partial progress (not all events applied)")
		return nil
	})
	require.NoError(t, err)

	// --- Phase 3: Resume import without failpoint (export still running) ---
	t.Log("Resuming import without failpoint...")
	err = lm.StartImportDataWithEnv(true, nil, []string{
		"NUM_EVENT_CHANNELS=1",
		"MAX_EVENTS_PER_BATCH=1",
		"MAX_INTERVAL_BETWEEN_BATCHES=1",
		"EVENT_CHANNEL_SIZE=2",
	})
	require.NoError(t, err, "Failed to start import resume")
	defer lm.StopImportData()

	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		reportTableName(tableName): {Inserts: 200, Updates: 200, Deletes: 100},
	}, 120, 5)
	require.NoError(t, err, "Migration report did not match expected CDC counts")

	err = lm.ValidateDataConsistency([]string{tableName}, "id")
	require.NoError(t, err, "Target does not match source after resume")
}

// TestImportCDCEventExecutionFailureAndResume verifies that live migration `import data` can resume
// after a non-retryable target DB error occurs while applying a CDC event inside a batch.
//
// Scenario:
//  1. Start `export data` and `import data` concurrently (import runs with failpoint enabled).
//  2. Wait for the snapshot phase to complete via the framework's WaitForSnapshotComplete.
//  3. Generate CDC inserts on the source via SourceDeltaSQL.
//  4. Wait for the failpoint to fire during CDC streaming, causing import to crash.
//  5. Verify partial CDC progress on target and confirm source != target (export keeps running).
//  6. Resume `import data` without failpoint and verify target matches source.
//
// Notes on determinism:
// - We set NUM_EVENT_CHANNELS=1 and MAX_EVENTS_PER_BATCH=10 so event execution ordering is stable.
// - Failpoint uses a hit-counter pattern: `50*off->return(true)` fails on the 51st event execution attempt.
func TestImportCDCEventExecutionFailureAndResume(t *testing.T) {
	ctx := context.Background()
	tableName := "test_schema_import_cdc_event_fail.cdc_import_test"

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test_import_cdc_event_fail",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test_import_cdc_event_fail",
		},
		SchemaNames: []string{"test_schema_import_cdc_event_fail"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema_import_cdc_event_fail;`,
			`CREATE TABLE test_schema_import_cdc_event_fail.cdc_import_test (
				id INTEGER PRIMARY KEY,
				name TEXT
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema_import_cdc_event_fail.cdc_import_test REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema_import_cdc_event_fail.cdc_import_test (id, name)
			 SELECT i, 'snapshot_' || i FROM generate_series(1, 30) i;`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema_import_cdc_event_fail.cdc_import_test (id, name)
			 SELECT 1000 + i, 'cdc_ins_' || i FROM generate_series(1, 120) i;`,
		},
		CleanupSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema_import_cdc_event_fail CASCADE;`,
		},
	})
	defer lm.Cleanup()

	err := lm.SetupContainers(ctx)
	require.NoError(t, err, "failed to setup containers")
	err = lm.SetupSchema()
	require.NoError(t, err, "failed to setup schema")

	// --- Phase 1: Start export and import concurrently, wait for snapshot, then generate CDC ---
	err = lm.StartExportData(true, nil)
	require.NoError(t, err, "failed to start export")

	failpointEnv := testutils.GetFailpointEnvVar(
		// Skip the first 50 event-execution attempts (let them succeed), then
		// inject a non-retryable DB error on the 51st attempt.
		"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb/importCDCExecEventError=50*off->return(true)",
	)
	err = lm.StartImportDataWithEnv(true, map[string]string{
		"--max-retries-streaming": "1",
	}, []string{
		failpointEnv,
		fmt.Sprintf("YB_VOYAGER_FAILPOINT_MARKER_DIR=%s", filepath.Join(lm.GetCurrentExportDir(), "failpoints")),
		"NUM_EVENT_CHANNELS=1",
		"MAX_EVENTS_PER_BATCH=10",
		"MAX_INTERVAL_BETWEEN_BATCHES=1",
		"EVENT_CHANNEL_SIZE=20",
	})
	require.NoError(t, err, "failed to start import with failpoint")

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"test_schema_import_cdc_event_fail"."cdc_import_test"`: 30,
	}, 120)
	require.NoError(t, err, "snapshot phase did not complete")
	t.Log("Snapshot complete; generating CDC inserts...")

	lm.ExecuteSourceDelta()

	// --- Phase 2: Wait for failpoint to fire and import to crash ---
	failMarkerPath := filepath.Join(lm.GetCurrentExportDir(), "failpoints", "failpoint-import-cdc-exec-event-error.log")
	err = lm.WaitForImportFailpointAndProcessCrash(t, failMarkerPath, 120*time.Second, 60*time.Second)
	require.NoError(t, err, "Import should crash after CDC exec-event failpoint")
	require.Contains(t, lm.GetImportCommandStderr(), "failpoint", "Expected failpoint mention in import stderr")

	// Verify partial CDC progress: some events applied, but not all queued ones.
	migrationUUIDStr, err := lm.ReadMigrationUUID()
	require.NoError(t, err, "Failed to read migration UUID")
	require.NotEmpty(t, migrationUUIDStr)

	maxQueuedVsn, err := maxVsnInQueueSegments(lm.GetCurrentExportDir())
	require.NoError(t, err, "Failed to compute max queued vsn from queue segments")
	t.Logf("Max queued vsn in CDC segments: %d", maxQueuedVsn)

	err = lm.WithTargetConn(func(ybConn *sql.DB) error {
		lastAppliedVsn, err := maxLastAppliedVsn(ybConn, migrationUUIDStr)
		require.NoError(t, err, "Failed to read last_applied_vsn from event channels metadata")
		t.Logf("Max last_applied_vsn on target after failure: %d", lastAppliedVsn)
		require.Greater(t, lastAppliedVsn, int64(0), "Expected some CDC progress before failure")
		require.Less(t, lastAppliedVsn, maxQueuedVsn, "Expected partial progress (not all events applied)")
		return nil
	})
	require.NoError(t, err)

	// After failure, target must not match source yet.
	err = lm.WithSourceTargetConn(func(pgConn, ybConn *sql.DB) error {
		require.Error(t, testutils.CompareTableData(ctx, pgConn, ybConn, tableName, "id"),
			"Expected source != target after CDC failure (resume should be required)")
		return nil
	})
	require.NoError(t, err)

	// --- Phase 3: Resume import without failpoint (export still running) ---
	t.Log("Resuming import without failpoint...")
	err = lm.StartImportDataWithEnv(true, nil, []string{
		"NUM_EVENT_CHANNELS=1",
		"MAX_EVENTS_PER_BATCH=10",
		"MAX_INTERVAL_BETWEEN_BATCHES=1",
		"EVENT_CHANNEL_SIZE=20",
	})
	require.NoError(t, err, "Failed to start import resume")
	defer lm.StopImportData()

	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		reportTableName(tableName): {Inserts: 120, Updates: 0, Deletes: 0},
	}, 120, 5)
	require.NoError(t, err, "Migration report did not match expected CDC counts")

	err = lm.ValidateDataConsistency([]string{tableName}, "id")
	require.NoError(t, err, "Target does not match source after resume")
}

// TestImportCDCRetryableDbErrorThenSucceed verifies that live migration `import data` retries and
// continues successfully when a transient (retryable) target DB error occurs during CDC apply.
//
// Scenario:
//  1. Start `export data` and `import data` concurrently (import runs with failpoint enabled).
//  2. Wait for the snapshot phase to complete via the framework's WaitForSnapshotComplete.
//  3. Generate CDC inserts on the source via SourceDeltaSQL.
//  4. Verify the failpoint marker is written and the import keeps running (i.e. it retried
//     instead of exiting).
//  5. Verify target eventually matches source without needing a separate resume run.
//
// Notes on determinism/speed:
// - We cap retry sleeps using YB_VOYAGER_MAX_SLEEP_SECOND=0 so the retry is immediate.
// - We set NUM_EVENT_CHANNELS=1 and MAX_EVENTS_PER_BATCH=10 to keep ordering stable.
func TestImportCDCRetryableDbErrorThenSucceed(t *testing.T) {
	ctx := context.Background()
	tableName := "test_schema_import_cdc_retry.cdc_import_test"

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test_import_cdc_retry",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test_import_cdc_retry",
		},
		SchemaNames: []string{"test_schema_import_cdc_retry"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema_import_cdc_retry;`,
			`CREATE TABLE test_schema_import_cdc_retry.cdc_import_test (
				id INTEGER PRIMARY KEY,
				name TEXT
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema_import_cdc_retry.cdc_import_test REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema_import_cdc_retry.cdc_import_test (id, name)
			 SELECT i, 'snapshot_' || i FROM generate_series(1, 30) i;`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema_import_cdc_retry.cdc_import_test (id, name)
			 SELECT 1000 + i, 'cdc_ins_' || i FROM generate_series(1, 120) i;`,
		},
		CleanupSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema_import_cdc_retry CASCADE;`,
		},
	})
	defer lm.Cleanup()

	err := lm.SetupContainers(ctx)
	require.NoError(t, err, "failed to setup containers")
	err = lm.SetupSchema()
	require.NoError(t, err, "failed to setup schema")

	// --- Phase 1: Start export and import concurrently, wait for snapshot, then generate CDC ---
	err = lm.StartExportData(true, nil)
	require.NoError(t, err, "failed to start export")

	failpointEnv := testutils.GetFailpointEnvVar(
		// Inject a retryable error on the very first batch execution, then disable
		// the failpoint (1*return = fire once). The importer should retry in-process
		// and continue without crashing.
		"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb/importCDCRetryableExecuteBatchError=1*return(true)",
	)
	err = lm.StartImportDataWithEnv(true, nil, []string{
		failpointEnv,
		fmt.Sprintf("YB_VOYAGER_FAILPOINT_MARKER_DIR=%s", filepath.Join(lm.GetCurrentExportDir(), "failpoints")),
		"YB_VOYAGER_MAX_SLEEP_SECOND=0",
		"NUM_EVENT_CHANNELS=1",
		"MAX_EVENTS_PER_BATCH=10",
		"MAX_INTERVAL_BETWEEN_BATCHES=1",
		"EVENT_CHANNEL_SIZE=20",
	})
	require.NoError(t, err, "failed to start import with failpoint")
	defer lm.GetImportRunner().Kill()

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"test_schema_import_cdc_retry"."cdc_import_test"`: 30,
	}, 120)
	require.NoError(t, err, "snapshot phase did not complete")
	t.Log("Snapshot complete; generating CDC inserts...")

	lm.ExecuteSourceDelta()

	// --- Phase 2: Wait for failpoint marker and verify import stays alive (retries in-process) ---
	failMarkerPath := filepath.Join(lm.GetCurrentExportDir(), "failpoints", "failpoint-import-cdc-retryable-exec-batch-error.log")
	t.Logf("Waiting for failpoint marker: %s", failMarkerPath)
	matched, err := testutils.WaitForFailpointMarker(failMarkerPath, 120*time.Second, 2*time.Second)
	require.NoError(t, err, "Should be able to read retryable batch failure marker")
	require.True(t, matched, "Retryable batch failpoint marker did not trigger")
	t.Log("Verified retryable batch failpoint marker was written")

	// The key property: importer should NOT exit; it should retry and keep running.
	time.Sleep(3 * time.Second)
	require.False(t, lm.GetImportRunner().IsStopped(),
		"Import exited unexpectedly after retryable failure; stderr=%s", lm.GetImportCommandStderr())

	// --- Phase 3: Verify target matches source (no resume needed) ---
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		reportTableName(tableName): {Inserts: 120, Updates: 0, Deletes: 0},
	}, 120, 5)
	require.NoError(t, err, "Migration report did not match expected CDC counts")

	err = lm.ValidateDataConsistency([]string{tableName}, "id")
	require.NoError(t, err, "Target does not match source after retry")
}

// TestImportCDCRetryableAfterCommitErrorSkipsRetry verifies that live migration `import data`
// continues successfully when `ExecuteBatch` returns a retryable error even though the CDC batch
// transaction actually committed.
//
// This exercises the safety net in `processEvents()`:
// after a retryable error, voyager calls `checkifEventBatchAlreadyImported()` to avoid re-applying
// a batch that committed successfully on the server (e.g. RPC timeout on commit).
//
// Scenario:
//  1. Start `export data` and `import data` concurrently (import runs with failpoint enabled).
//  2. Wait for snapshot, then generate CDC inserts via SourceDeltaSQL.
//  3. Verify the failpoint marker is written and the import keeps running (retries in-process).
//  4. Verify via data-migration-report that imported CDC events catch up to exported events.
//  5. Verify final target matches source without a resume run.
func TestImportCDCRetryableAfterCommitErrorSkipsRetry(t *testing.T) {
	ctx := context.Background()
	tableName := "test_schema_import_cdc_retry_after_commit.cdc_import_test"

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test_import_cdc_retry_commit",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test_import_cdc_retry_commit",
		},
		SchemaNames: []string{"test_schema_import_cdc_retry_after_commit"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema_import_cdc_retry_after_commit;`,
			`CREATE TABLE test_schema_import_cdc_retry_after_commit.cdc_import_test (
				id SERIAL PRIMARY KEY,
				name TEXT,
				value INTEGER
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema_import_cdc_retry_after_commit.cdc_import_test REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema_import_cdc_retry_after_commit.cdc_import_test (name, value)
			 SELECT 'snapshot_' || i, i FROM generate_series(1, 30) i;`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema_import_cdc_retry_after_commit.cdc_import_test (name, value)
			 SELECT 'cdc_ins_' || i, 1000 + i FROM generate_series(1, 60) i;`,
		},
		CleanupSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema_import_cdc_retry_after_commit CASCADE;`,
		},
	})
	defer lm.Cleanup()

	err := lm.SetupContainers(ctx)
	require.NoError(t, err, "failed to setup containers")
	err = lm.SetupSchema()
	require.NoError(t, err, "failed to setup schema")

	// --- Phase 1: Start export and import concurrently, wait for snapshot, then generate CDC ---
	err = lm.StartExportData(true, nil)
	require.NoError(t, err, "failed to start export")

	failpointEnv := testutils.GetFailpointEnvVar(
		// Inject a retryable error on the very first batch execution *after* the
		// batch has already been committed (simulates e.g. RPC timeout on commit).
		// 1*return = fire once, then disable. The importer should detect the batch
		// was already applied and skip re-execution.
		"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb/importCDCRetryableAfterCommitError=1*return(true)",
	)
	err = lm.StartImportDataWithEnv(true, nil, []string{
		failpointEnv,
		fmt.Sprintf("YB_VOYAGER_FAILPOINT_MARKER_DIR=%s", filepath.Join(lm.GetCurrentExportDir(), "failpoints")),
		"YB_VOYAGER_MAX_SLEEP_SECOND=0",
		"NUM_EVENT_CHANNELS=1",
		"MAX_EVENTS_PER_BATCH=1",
		"MAX_INTERVAL_BETWEEN_BATCHES=1",
		"EVENT_CHANNEL_SIZE=2",
	})
	require.NoError(t, err, "failed to start import with failpoint")
	defer lm.GetImportRunner().Kill()

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"test_schema_import_cdc_retry_after_commit"."cdc_import_test"`: 30,
	}, 120)
	require.NoError(t, err, "snapshot phase did not complete")
	t.Log("Snapshot complete; generating CDC inserts...")

	lm.ExecuteSourceDelta()

	// --- Phase 2: Wait for failpoint marker and verify import stays alive (retries in-process) ---
	failMarkerPath := filepath.Join(lm.GetCurrentExportDir(), "failpoints", "failpoint-import-cdc-retryable-after-commit-error.log")
	t.Logf("Waiting for failpoint marker: %s", failMarkerPath)
	matched, err := testutils.WaitForFailpointMarker(failMarkerPath, 120*time.Second, 2*time.Second)
	require.NoError(t, err, "Should be able to read retryable-after-commit marker")
	require.True(t, matched, "Retryable-after-commit failpoint marker did not trigger")
	t.Log("Verified retryable-after-commit failpoint marker was written")

	// Import should NOT exit; it should retry in-process and continue.
	time.Sleep(3 * time.Second)
	require.False(t, lm.GetImportRunner().IsStopped(),
		"Import exited unexpectedly after retryable-after-commit failure; stderr=%s", lm.GetImportCommandStderr())

	// --- Phase 3: Verify CDC catches up and target matches source (no resume needed) ---
	// Verify via data-migration-report that all exported CDC events are imported.
	require.Eventually(t, func() bool {
		report, rerr := lm.GetDataMigrationReport()
		if rerr != nil {
			return false
		}
		imported, exported := getReportCDCCounts(report, reportTableName(tableName))
		return exported > 0 && imported >= exported
	}, 120*time.Second, 2*time.Second, "Expected imported CDC events to catch up to exported events after retryable-after-commit error")
	t.Log("Verified CDC events caught up after retryable-after-commit error")

	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		reportTableName(tableName): {Inserts: 60, Updates: 0, Deletes: 0},
	}, 120, 5)
	require.NoError(t, err, "Migration report did not match expected CDC counts")

	err = lm.ValidateDataConsistency([]string{tableName}, "id")
	require.NoError(t, err, "Target does not match source after retryable-after-commit error")
}

// TestImportCDCMultiChannelBatchFailureAndResume verifies that CDC import resume works correctly
// when multiple event channels are enabled (NUM_EVENT_CHANNELS>1).
//
// Scenario:
//  1. Start `export data` and `import data` concurrently (import runs with failpoint enabled).
//  2. Wait for snapshot, then generate CDC workload with INSERT/UPDATE/DELETE events spread
//     across all channels using pre-selected IDs whose hash maps into each channel.
//  3. Wait for the failpoint to fire during CDC streaming, causing import to crash.
//  4. After crash, verify:
//     - partial progress (max last_applied_vsn < max queued vsn)
//     - per-channel progress metadata exists for all channels and at least some channels advanced
//  5. Resume `import data` and verify:
//     - final target matches source
//     - each channel's last_applied_vsn advanced (>-1), proving multi-channel progress tracking worked
func TestImportCDCMultiChannelBatchFailureAndResume(t *testing.T) {
	ctx := context.Background()

	numChans := 4
	eventsPerChan := 60
	updatesPerChan := 30
	deletesPerChan := 15
	tableName := "test_schema_import_cdc_multi_chan.cdc_import_test"

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test_import_cdc_multi_chan",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test_import_cdc_multi_chan",
		},
		SchemaNames: []string{"test_schema_import_cdc_multi_chan"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema_import_cdc_multi_chan;`,
			`CREATE TABLE test_schema_import_cdc_multi_chan.cdc_import_test (
				id INTEGER PRIMARY KEY,
				name TEXT,
				value INTEGER
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema_import_cdc_multi_chan.cdc_import_test REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema_import_cdc_multi_chan.cdc_import_test (id, name, value)
			 SELECT i, 'snapshot_' || i, i FROM generate_series(1, 30) i;`,
		},
		CleanupSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema_import_cdc_multi_chan CASCADE;`,
		},
	})
	defer lm.Cleanup()

	err := lm.SetupContainers(ctx)
	require.NoError(t, err, "failed to setup containers")
	err = lm.SetupSchema()
	require.NoError(t, err, "failed to setup schema")

	// --- Phase 1: Start export and import concurrently, wait for snapshot ---
	err = lm.StartExportData(true, nil)
	require.NoError(t, err, "failed to start export")

	failpointEnv := testutils.GetFailpointEnvVar(
		// Skip the first 100 batch-commit calls across all channels (let them
		// succeed), then inject a DB error on the 101st. 100 is large enough that
		// multiple channels make meaningful progress before the crash.
		"github.com/yugabyte/yb-voyager/yb-voyager/cmd/importCDCNonRetryableBatchDBError=100*off->return(true)",
	)
	err = lm.StartImportDataWithEnv(true, map[string]string{
		"--max-retries-streaming": "1",
	}, []string{
		failpointEnv,
		fmt.Sprintf("NUM_EVENT_CHANNELS=%d", numChans),
		"MAX_EVENTS_PER_BATCH=1",
		"MAX_INTERVAL_BETWEEN_BATCHES=1",
		"EVENT_CHANNEL_SIZE=20",
	})
	require.NoError(t, err, "failed to start import with failpoint")

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"test_schema_import_cdc_multi_chan"."cdc_import_test"`: 30,
	}, 120)
	require.NoError(t, err, "snapshot phase did not complete")
	t.Log("Snapshot complete; generating multi-channel CDC changes...")

	// Generate CDC workload with IDs pre-selected to spread across all channels.
	// CDC import partitions events by hashing (table + key) % NUM_EVENT_CHANNELS.
	// We use computeChanForID (same FNV64a scheme as the importer) to
	// pick IDs that map to each channel, then generate INSERT/UPDATE/DELETE for each.
	idsByChan := make([][]int, numChans)
	for id := 1000; id < 100000; id++ {
		ch := computeChanForID(tableName, id, numChans)
		if len(idsByChan[ch]) < eventsPerChan {
			idsByChan[ch] = append(idsByChan[ch], id)
		}
		done := true
		for c := 0; c < numChans; c++ {
			if len(idsByChan[c]) < eventsPerChan {
				done = false
				break
			}
		}
		if done {
			break
		}
	}
	for c := 0; c < numChans; c++ {
		require.Equal(t, eventsPerChan, len(idsByChan[c]), "Failed to find enough ids for channel %d", c)
	}

	vals := make([]string, 0, numChans*eventsPerChan)
	for c := 0; c < numChans; c++ {
		for _, id := range idsByChan[c] {
			vals = append(vals, fmt.Sprintf("(%d, 'cdc_ins_%d', %d)", id, id, 10000+id))
		}
	}
	lm.ExecuteOnSource(fmt.Sprintf(
		"INSERT INTO %s (id, name, value) VALUES %s;", tableName, strings.Join(vals, ",")))

	updateIDs := make([]string, 0, numChans*updatesPerChan)
	for c := 0; c < numChans; c++ {
		for i := 0; i < updatesPerChan; i++ {
			updateIDs = append(updateIDs, strconv.Itoa(idsByChan[c][i]))
		}
	}
	t.Logf("Generating CDC updates: %d rows", len(updateIDs))
	lm.ExecuteOnSource(fmt.Sprintf(
		"UPDATE %s SET value = value + 50000, name = 'cdc_upd_' || id WHERE id IN (%s);",
		tableName, strings.Join(updateIDs, ",")))

	deleteIDs := make([]string, 0, numChans*deletesPerChan)
	for c := 0; c < numChans; c++ {
		for i := 0; i < deletesPerChan; i++ {
			deleteIDs = append(deleteIDs, strconv.Itoa(idsByChan[c][i]))
		}
	}
	t.Logf("Generating CDC deletes: %d rows", len(deleteIDs))
	lm.ExecuteOnSource(fmt.Sprintf(
		"DELETE FROM %s WHERE id IN (%s);", tableName, strings.Join(deleteIDs, ",")))

	// --- Phase 2: Wait for failpoint to fire and import to crash ---
	failMarkerPath := filepath.Join(lm.GetCurrentExportDir(), "failpoints", "failpoint-import-cdc-non-retryable-batch-db-error.log")
	err = lm.WaitForImportFailpointAndProcessCrash(t, failMarkerPath, 120*time.Second, 60*time.Second)
	require.NoError(t, err, "Import should crash after multi-channel batch failure failpoint")

	// Verify partial CDC progress and per-channel metadata.
	migrationUUIDStr, err := lm.ReadMigrationUUID()
	require.NoError(t, err, "Failed to read migration UUID")
	require.NotEmpty(t, migrationUUIDStr)

	maxQueuedVsn, err := maxVsnInQueueSegments(lm.GetCurrentExportDir())
	require.NoError(t, err, "Failed to compute max queued vsn from queue segments")
	t.Logf("Max queued vsn in CDC segments: %d", maxQueuedVsn)

	err = lm.WithTargetConn(func(ybConn *sql.DB) error {
		lastAppliedVsn, err := maxLastAppliedVsn(ybConn, migrationUUIDStr)
		require.NoError(t, err, "Failed to read last_applied_vsn from event channels metadata")
		t.Logf("Max last_applied_vsn on target after failure: %d", lastAppliedVsn)
		require.Less(t, lastAppliedVsn, maxQueuedVsn, "Expected partial progress (max last_applied_vsn < max queued vsn)")

		byChan, err := lastAppliedVsnsByChannel(ybConn, migrationUUIDStr)
		require.NoError(t, err, "Failed to read per-channel last_applied_vsn")
		require.Equal(t, numChans, len(byChan), "Expected one metadata row per channel")
		progressed := 0
		for c := 0; c < numChans; c++ {
			if byChan[c] >= 0 {
				progressed++
			}
		}
		require.GreaterOrEqual(t, progressed, 2, "Expected at least two channels to have applied some events before failure")
		return nil
	})
	require.NoError(t, err)

	// After failure, target must not match source yet.
	err = lm.WithSourceTargetConn(func(pgConn, ybConn *sql.DB) error {
		require.Error(t, testutils.CompareTableData(ctx, pgConn, ybConn, tableName, "id"),
			"Expected source != target after crash")
		return nil
	})
	require.NoError(t, err)

	// --- Phase 3: Resume import without failpoint (export still running) ---
	t.Log("Resuming import without failpoint...")
	err = lm.StartImportDataWithEnv(true, nil, []string{
		fmt.Sprintf("NUM_EVENT_CHANNELS=%d", numChans),
		"MAX_EVENTS_PER_BATCH=1",
		"MAX_INTERVAL_BETWEEN_BATCHES=1",
		"EVENT_CHANNEL_SIZE=20",
	})
	require.NoError(t, err, "Failed to start import resume")
	defer lm.StopImportData()

	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		reportTableName(tableName): {
			Inserts: int64(numChans * eventsPerChan),
			Updates: int64(numChans * updatesPerChan),
			Deletes: int64(numChans * deletesPerChan),
		},
	}, 120, 5)
	require.NoError(t, err, "Migration report did not match expected CDC counts")

	err = lm.ValidateDataConsistency([]string{tableName}, "id")
	require.NoError(t, err, "Target does not match source after resume")

	// Verify all channels advanced after resume.
	err = lm.WithTargetConn(func(ybConn *sql.DB) error {
		byChanAfter, err := lastAppliedVsnsByChannel(ybConn, migrationUUIDStr)
		require.NoError(t, err, "Failed to read per-channel last_applied_vsn after resume")
		require.Equal(t, numChans, len(byChanAfter), "Expected one metadata row per channel after resume")
		for c := 0; c < numChans; c++ {
			require.Greater(t, byChanAfter[c], int64(-1), "Expected channel %d to have applied at least one event", c)
			require.LessOrEqual(t, byChanAfter[c], maxQueuedVsn, "Expected channel %d last_applied_vsn <= max queued vsn", c)
		}
		return nil
	})
	require.NoError(t, err)
}

// reportTableName converts a dot-separated table name (e.g. "schema.table") to the
// quoted format used in the data-migration-report JSON (e.g. `"schema"."table"`).
func reportTableName(dotNotation string) string {
	parts := strings.SplitN(dotNotation, ".", 2)
	if len(parts) == 2 {
		return fmt.Sprintf(`"%s"."%s"`, parts[0], parts[1])
	}
	return fmt.Sprintf(`"%s"`, dotNotation)
}

// getReportCDCCounts extracts the total imported and exported CDC event counts for a
// table from a DataMigrationReport. The tableName must be in the quoted format
// returned by reportTableName().
func getReportCDCCounts(report *DataMigrationReport, tableName string) (imported, exported int64) {
	for _, row := range report.RowData {
		if row.TableName != tableName {
			continue
		}
		if row.DBType == "target" {
			imported = row.ImportedInserts + row.ImportedUpdates + row.ImportedDeletes
		} else if row.DBType == "source" {
			exported = row.ExportedInserts + row.ExportedUpdates + row.ExportedDeletes
		}
	}
	return
}

// computeChanForID returns the importer channel number for a single-row CDC event
// using the same hashing scheme as `cmd/live_migration.go` (FNV64a(tableFQN + key) % N).
func computeChanForID(tableFQN string, id int, numChans int) int {
	h := fnv.New64a()
	h.Write([]byte(tableFQN))
	h.Write([]byte(strconv.Itoa(id)))
	return int(h.Sum64() % uint64(numChans))
}

// lastAppliedVsnsByChannel reads per-channel `last_applied_vsn` for a migration UUID
// from voyager's target-side metadata table.
func lastAppliedVsnsByChannel(ybConn interface {
	Query(query string, args ...any) (*sql.Rows, error)
}, migrationUUIDStr string) (map[int]int64, error) {
	rows, err := ybConn.Query(fmt.Sprintf(
		"SELECT channel_no, last_applied_vsn FROM %s WHERE migration_uuid='%s'",
		tgtdb.EVENT_CHANNELS_METADATA_TABLE_NAME,
		migrationUUIDStr,
	))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := map[int]int64{}
	for rows.Next() {
		var chanNo int
		var v sql.NullInt64
		if err := rows.Scan(&chanNo, &v); err != nil {
			return nil, err
		}
		if v.Valid {
			res[chanNo] = v.Int64
		} else {
			res[chanNo] = -1
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

// maxVsnInQueueSegments returns the maximum VSN present in queue segments.
func maxVsnInQueueSegments(exportDir string) (int64, error) {
	queueDir := filepath.Join(exportDir, "data", "queue")
	entries, err := os.ReadDir(queueDir)
	if err != nil {
		return 0, err
	}

	var maxVsn int64
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "segment.") || !strings.HasSuffix(name, ".ndjson") {
			continue
		}
		f, err := os.Open(filepath.Join(queueDir, name))
		if err != nil {
			return 0, err
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || line == `\.` {
				continue
			}
			var m map[string]any
			if err := json.Unmarshal([]byte(line), &m); err != nil {
				f.Close()
				return 0, err
			}
			vsnRaw, ok := m["vsn"]
			if !ok || vsnRaw == nil {
				continue
			}
			switch v := vsnRaw.(type) {
			case float64:
				if int64(v) > maxVsn {
					maxVsn = int64(v)
				}
			case int64:
				if v > maxVsn {
					maxVsn = v
				}
			}
		}
		if err := scanner.Err(); err != nil {
			f.Close()
			return 0, err
		}
		f.Close()
	}
	if maxVsn == 0 {
		return 0, fmt.Errorf("max vsn not found in queue segments")
	}
	return maxVsn, nil
}

// maxLastAppliedVsn returns the maximum `last_applied_vsn` across all importer channels
// for the given migration UUID, as recorded on the target in voyager metadata.
func maxLastAppliedVsn(ybConn interface {
	Query(query string, args ...any) (*sql.Rows, error)
}, migrationUUIDStr string) (int64, error) {
	// Query the voyager metadata table that tracks CDC progress per channel.
	// The table name is stable across YB versions because it is created by voyager:
	// tgtdb.EVENT_CHANNELS_METADATA_TABLE_NAME = ybvoyager_metadata.ybvoyager_import_data_event_channels_metainfo
	rows, err := ybConn.Query(fmt.Sprintf(
		"SELECT last_applied_vsn FROM %s WHERE migration_uuid='%s'",
		tgtdb.EVENT_CHANNELS_METADATA_TABLE_NAME,
		migrationUUIDStr,
	))
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var max int64
	for rows.Next() {
		var v sql.NullInt64
		if err := rows.Scan(&v); err != nil {
			return 0, err
		}
		if v.Valid && v.Int64 > max {
			max = v.Int64
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return max, nil
}
