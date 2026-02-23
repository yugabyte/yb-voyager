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

	"database/sql"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
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
	defer killDebeziumForExportDirImportTest(t, lm.GetExportDir())

	failpointEnv := testutils.GetFailpointEnvVar(
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
	testutils.LogTest(t, "Snapshot complete; generating CDC changes...")

	lm.ExecuteSourceDelta()

	// --- Phase 2: Wait for failpoint to fire and import to crash ---
	failMarkerPath := filepath.Join(lm.GetExportDir(), "logs", "failpoint-import-cdc-transform.log")
	err = testutils.WaitForFailpointAndProcessCrash(t, lm.GetImportRunner(), failMarkerPath, 120*time.Second, 60*time.Second)
	require.NoError(t, err, "Import should crash after CDC transform failpoint")

	// Verify partial CDC progress: some events applied, but not all queued ones.
	migrationUUIDStr, err := lm.ReadMigrationUUID()
	require.NoError(t, err, "Failed to read migration UUID")
	require.NotEmpty(t, migrationUUIDStr)

	maxQueuedVsn, err := maxVsnInQueueSegments(lm.GetExportDir())
	require.NoError(t, err, "Failed to compute max queued vsn from queue segments")
	testutils.LogTestf(t, "Max queued vsn in CDC segments: %d", maxQueuedVsn)

	err = lm.WithTargetConn(func(ybConn *sql.DB) error {
		lastAppliedVsn, err := maxLastAppliedVsn(ybConn, migrationUUIDStr)
		require.NoError(t, err, "Failed to read last_applied_vsn from event channels metadata")
		testutils.LogTestf(t, "Max last_applied_vsn on target after failure: %d", lastAppliedVsn)
		require.Greater(t, lastAppliedVsn, int64(0), "Expected some CDC progress before failure")
		require.Less(t, lastAppliedVsn, maxQueuedVsn, "Expected partial progress (not all events applied)")
		return nil
	})
	require.NoError(t, err)

	// --- Phase 3: Resume import without failpoint (export still running) ---
	testutils.LogTest(t, "Resuming import without failpoint...")
	err = lm.StartImportDataWithEnv(true, nil, []string{
		"MAX_EVENTS_PER_BATCH=1",
		"MAX_INTERVAL_BETWEEN_BATCHES=1",
		"EVENT_CHANNEL_SIZE=2",
	})
	require.NoError(t, err, "Failed to start import resume")
	defer lm.StopImportData()

	err = lm.WithSourceTargetConn(func(pgConn, ybConn *sql.DB) error {
		require.Eventually(t, func() bool {
			return testutils.CompareTableData(ctx, pgConn, ybConn, tableName, "id") == nil
		}, 180*time.Second, 5*time.Second, "Timed out waiting for target to match source after resume")
		return nil
	})
	require.NoError(t, err)

	testutils.LogTest(t, "Target matches source after resume")
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
	defer killDebeziumForExportDirImportTest(t, lm.GetExportDir())

	failpointEnv := testutils.GetFailpointEnvVar(
		// Crash after 100 successful CDC batches have been committed, so `last_applied_vsn > 0`
		// is observable and we can prove resume work is meaningful.
		"github.com/yugabyte/yb-voyager/yb-voyager/cmd/importCDCBatchDBError=100*off->return(true)",
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
	testutils.LogTest(t, "Snapshot complete; generating CDC changes...")

	lm.ExecuteSourceDelta()

	// --- Phase 2: Wait for failpoint to fire and import to crash ---
	failMarkerPath := filepath.Join(lm.GetExportDir(), "logs", "failpoint-import-cdc-db-error.log")
	err = testutils.WaitForFailpointAndProcessCrash(t, lm.GetImportRunner(), failMarkerPath, 120*time.Second, 60*time.Second)
	require.NoError(t, err, "Import should crash after CDC DB-error failpoint")

	// Verify partial CDC progress: some events applied, but not all queued ones.
	migrationUUIDStr, err := lm.ReadMigrationUUID()
	require.NoError(t, err, "Failed to read migration UUID")
	require.NotEmpty(t, migrationUUIDStr)

	maxQueuedVsn, err := maxVsnInQueueSegments(lm.GetExportDir())
	require.NoError(t, err, "Failed to compute max queued vsn from queue segments")
	testutils.LogTestf(t, "Max queued vsn in CDC segments: %d", maxQueuedVsn)

	err = lm.WithTargetConn(func(ybConn *sql.DB) error {
		lastAppliedVsn, err := maxLastAppliedVsn(ybConn, migrationUUIDStr)
		require.NoError(t, err, "Failed to read last_applied_vsn from event channels metadata")
		testutils.LogTestf(t, "Max last_applied_vsn on target after failure: %d", lastAppliedVsn)
		require.Greater(t, lastAppliedVsn, int64(0), "Expected some CDC progress before failure")
		require.Less(t, lastAppliedVsn, maxQueuedVsn, "Expected partial progress (not all events applied)")
		return nil
	})
	require.NoError(t, err)

	// --- Phase 3: Resume import without failpoint (export still running) ---
	testutils.LogTest(t, "Resuming import without failpoint...")
	err = lm.StartImportDataWithEnv(true, nil, []string{
		"NUM_EVENT_CHANNELS=1",
		"MAX_EVENTS_PER_BATCH=1",
		"MAX_INTERVAL_BETWEEN_BATCHES=1",
		"EVENT_CHANNEL_SIZE=2",
	})
	require.NoError(t, err, "Failed to start import resume")
	defer lm.StopImportData()

	err = lm.WithSourceTargetConn(func(pgConn, ybConn *sql.DB) error {
		require.Eventually(t, func() bool {
			return testutils.CompareTableData(ctx, pgConn, ybConn, tableName, "id") == nil
		}, 180*time.Second, 5*time.Second, "Timed out waiting for target to match source after resume")
		return nil
	})
	require.NoError(t, err)

	testutils.LogTest(t, "Target matches source after resume (CDC DB error)")
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
	defer killDebeziumForExportDirImportTest(t, lm.GetExportDir())

	failpointEnv := testutils.GetFailpointEnvVar(
		"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb/importCDCExecEventError=50*off->return(true)",
	)
	err = lm.StartImportDataWithEnv(true, map[string]string{
		"--max-retries-streaming": "1",
	}, []string{
		failpointEnv,
		fmt.Sprintf("YB_VOYAGER_FAILPOINT_MARKER_DIR=%s", filepath.Join(lm.GetExportDir(), "logs")),
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
	testutils.LogTest(t, "Snapshot complete; generating CDC inserts...")

	lm.ExecuteSourceDelta()

	// --- Phase 2: Wait for failpoint to fire and import to crash ---
	failMarkerPath := filepath.Join(lm.GetExportDir(), "logs", "failpoint-import-cdc-exec-event-error.log")
	err = testutils.WaitForFailpointAndProcessCrash(t, lm.GetImportRunner(), failMarkerPath, 120*time.Second, 60*time.Second)
	require.NoError(t, err, "Import should crash after CDC exec-event failpoint")
	require.Contains(t, lm.GetImportCommandStderr(), "failpoint", "Expected failpoint mention in import stderr")

	// Verify partial CDC progress: some events applied, but not all queued ones.
	migrationUUIDStr, err := lm.ReadMigrationUUID()
	require.NoError(t, err, "Failed to read migration UUID")
	require.NotEmpty(t, migrationUUIDStr)

	maxQueuedVsn, err := maxVsnInQueueSegments(lm.GetExportDir())
	require.NoError(t, err, "Failed to compute max queued vsn from queue segments")
	testutils.LogTestf(t, "Max queued vsn in CDC segments: %d", maxQueuedVsn)

	err = lm.WithTargetConn(func(ybConn *sql.DB) error {
		lastAppliedVsn, err := maxLastAppliedVsn(ybConn, migrationUUIDStr)
		require.NoError(t, err, "Failed to read last_applied_vsn from event channels metadata")
		testutils.LogTestf(t, "Max last_applied_vsn on target after failure: %d", lastAppliedVsn)
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
	testutils.LogTest(t, "Resuming import without failpoint...")
	err = lm.StartImportDataWithEnv(true, nil, []string{
		"NUM_EVENT_CHANNELS=1",
		"MAX_EVENTS_PER_BATCH=10",
		"MAX_INTERVAL_BETWEEN_BATCHES=1",
		"EVENT_CHANNEL_SIZE=20",
	})
	require.NoError(t, err, "Failed to start import resume")
	defer lm.StopImportData()

	err = lm.WithSourceTargetConn(func(pgConn, ybConn *sql.DB) error {
		require.Eventually(t, func() bool {
			return testutils.CompareTableData(ctx, pgConn, ybConn, tableName, "id") == nil
		}, 240*time.Second, 5*time.Second, "Timed out waiting for target to match source after resume")
		return nil
	})
	require.NoError(t, err)

	testutils.LogTest(t, "Target matches source after resume (CDC event execution failure)")
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
	defer killDebeziumForExportDirImportTest(t, lm.GetExportDir())

	failpointEnv := testutils.GetFailpointEnvVar(
		"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb/importCDCRetryableExecuteBatchError=1*return(true)",
	)
	err = lm.StartImportDataWithEnv(true, nil, []string{
		failpointEnv,
		fmt.Sprintf("YB_VOYAGER_FAILPOINT_MARKER_DIR=%s", filepath.Join(lm.GetExportDir(), "logs")),
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
	testutils.LogTest(t, "Snapshot complete; generating CDC inserts...")

	lm.ExecuteSourceDelta()

	// --- Phase 2: Wait for failpoint marker and verify import stays alive (retries in-process) ---
	failMarkerPath := filepath.Join(lm.GetExportDir(), "logs", "failpoint-import-cdc-retryable-exec-batch-error.log")
	testutils.LogTestf(t, "Waiting for failpoint marker: %s", failMarkerPath)
	matched, err := testutils.WaitForFailpointMarker(failMarkerPath, 120*time.Second, 2*time.Second)
	require.NoError(t, err, "Should be able to read retryable batch failure marker")
	require.True(t, matched, "Retryable batch failpoint marker did not trigger")
	testutils.LogTest(t, "Verified retryable batch failpoint marker was written")

	// The key property: importer should NOT exit; it should retry and keep running.
	exitedEarly := make(chan error, 1)
	go func() {
		exitedEarly <- lm.GetImportRunner().Wait()
	}()
	select {
	case waitErr := <-exitedEarly:
		require.Failf(t, "Import exited unexpectedly after retryable failure",
			"err=%v\nstderr=%s", waitErr, lm.GetImportCommandStderr())
	case <-time.After(3 * time.Second):
		// still running as expected
	}

	// --- Phase 3: Verify target matches source (no resume needed) ---
	err = lm.WithSourceTargetConn(func(pgConn, ybConn *sql.DB) error {
		require.Eventually(t, func() bool {
			return testutils.CompareTableData(ctx, pgConn, ybConn, tableName, "id") == nil
		}, 240*time.Second, 5*time.Second, "Timed out waiting for target to match source after retry")
		return nil
	})
	require.NoError(t, err)

	testutils.LogTest(t, "Target matches source after in-process retry (no resume run needed)")
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
//  4. Verify `last_applied_vsn` advances to max queued vsn (proving the retry short-circuited).
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
	defer killDebeziumForExportDirImportTest(t, lm.GetExportDir())

	failpointEnv := testutils.GetFailpointEnvVar(
		"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb/importCDCRetryableAfterCommitError=1*return(true)",
	)
	err = lm.StartImportDataWithEnv(true, nil, []string{
		failpointEnv,
		fmt.Sprintf("YB_VOYAGER_FAILPOINT_MARKER_DIR=%s", filepath.Join(lm.GetExportDir(), "logs")),
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
	testutils.LogTest(t, "Snapshot complete; generating CDC inserts...")

	lm.ExecuteSourceDelta()

	// --- Phase 2: Wait for failpoint marker and verify import stays alive (retries in-process) ---
	failMarkerPath := filepath.Join(lm.GetExportDir(), "logs", "failpoint-import-cdc-retryable-after-commit-error.log")
	testutils.LogTestf(t, "Waiting for failpoint marker: %s", failMarkerPath)
	matched, err := testutils.WaitForFailpointMarker(failMarkerPath, 120*time.Second, 2*time.Second)
	require.NoError(t, err, "Should be able to read retryable-after-commit marker")
	require.True(t, matched, "Retryable-after-commit failpoint marker did not trigger")
	testutils.LogTest(t, "Verified retryable-after-commit failpoint marker was written")

	// Import should NOT exit; it should retry in-process and continue.
	exitedEarly := make(chan error, 1)
	go func() {
		exitedEarly <- lm.GetImportRunner().Wait()
	}()
	select {
	case waitErr := <-exitedEarly:
		require.Failf(t, "Import exited unexpectedly after retryable-after-commit failure",
			"err=%v\nstderr=%s", waitErr, lm.GetImportCommandStderr())
	case <-time.After(3 * time.Second):
		// still running as expected
	}

	// --- Phase 3: Verify vsn catches up and target matches source (no resume needed) ---
	migrationUUIDStr, err := lm.ReadMigrationUUID()
	require.NoError(t, err, "Failed to read migration UUID")
	require.NotEmpty(t, migrationUUIDStr)

	maxQueuedVsn, err := maxVsnInQueueSegments(lm.GetExportDir())
	require.NoError(t, err, "Failed to compute max queued vsn from queue segments")
	testutils.LogTestf(t, "Max queued vsn in CDC segments: %d", maxQueuedVsn)

	// Verify voyager progress metadata catches up to the queue.
	err = lm.WithTargetConn(func(ybConn *sql.DB) error {
		require.Eventually(t, func() bool {
			v, verr := maxLastAppliedVsn(ybConn, migrationUUIDStr)
			return verr == nil && v >= maxQueuedVsn
		}, 120*time.Second, 2*time.Second, "Expected last_applied_vsn to reach max queued vsn after retryable-after-commit error")
		testutils.LogTest(t, "Verified last_applied_vsn caught up after retryable-after-commit error")
		return nil
	})
	require.NoError(t, err)

	err = lm.WithSourceTargetConn(func(pgConn, ybConn *sql.DB) error {
		require.Eventually(t, func() bool {
			return testutils.CompareTableData(ctx, pgConn, ybConn, tableName, "id") == nil
		}, 240*time.Second, 5*time.Second, "Timed out waiting for target to match source after retryable-after-commit error")
		return nil
	})
	require.NoError(t, err)

	testutils.LogTest(t, "Target matches source after in-process retryable-after-commit handling")
}

// TestImportCDCMultiChannelBatchFailureAndResume verifies that CDC import resume works correctly
// when multiple event channels are enabled (NUM_EVENT_CHANNELS>1).
//
// Scenario:
//  1. Export snapshot-and-changes, wait for streaming mode, and queue a CDC workload with enough
//     INSERT + UPDATE + DELETE events to spread across all channels.
//  2. Stop export to freeze the queue.
//  3. Run `import data` with NUM_EVENT_CHANNELS=4 and a failpoint that crashes after N successful
//     CDC batches (expect crash mid-stream).
//  4. After crash, verify:
//     - partial progress (max last_applied_vsn < max queued vsn)
//     - per-channel progress metadata exists for all channels (rows present) and at least some channels advanced
//  5. Resume `import data` and verify:
//     - final target matches source
//     - each channel's last_applied_vsn advanced (>-1), proving multi-channel progress tracking worked
func TestImportCDCMultiChannelBatchFailureAndResume(t *testing.T) {
	ctx := context.Background()

	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)
	testutils.LogTestf(t, "Using exportDir=%s", exportDir)

	postgresContainer := testcontainers.NewTestContainer("postgresql", &testcontainers.ContainerConfig{ForLive: true})
	err := postgresContainer.Start(ctx)
	require.NoError(t, err, "Failed to start PostgreSQL container")
	defer postgresContainer.Stop(ctx)

	yugabytedbContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	err = yugabytedbContainer.Start(ctx)
	require.NoError(t, err, "Failed to start YugabyteDB container")
	defer yugabytedbContainer.Stop(ctx)

	postgresContainer.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema_import_cdc_multi_chan CASCADE;",
		"CREATE SCHEMA test_schema_import_cdc_multi_chan;",
		`CREATE TABLE test_schema_import_cdc_multi_chan.cdc_import_test (
			id INTEGER PRIMARY KEY,
			name TEXT,
			value INTEGER
		);`,
		`ALTER TABLE test_schema_import_cdc_multi_chan.cdc_import_test REPLICA IDENTITY FULL;`,
		`INSERT INTO test_schema_import_cdc_multi_chan.cdc_import_test (id, name, value)
		 SELECT i, 'snapshot_' || i, i FROM generate_series(1, 30) i;`,
	)
	defer postgresContainer.ExecuteSqls("DROP SCHEMA IF EXISTS test_schema_import_cdc_multi_chan CASCADE;")

	yugabytedbContainer.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema_import_cdc_multi_chan CASCADE;",
		"CREATE SCHEMA test_schema_import_cdc_multi_chan;",
		`CREATE TABLE test_schema_import_cdc_multi_chan.cdc_import_test (
			id INTEGER PRIMARY KEY,
			name TEXT,
			value INTEGER
		);`,
	)
	defer yugabytedbContainer.ExecuteSqls("DROP SCHEMA IF EXISTS test_schema_import_cdc_multi_chan CASCADE;")

	numChans := 4
	eventsPerChan := 60 // 4*60 = 240 CDC events
	updatesPerChan := 30
	deletesPerChan := 15
	tableFQN := "test_schema_import_cdc_multi_chan.cdc_import_test"

	cdcQueued := make(chan bool, 1)
	generateCDC := func() {
		testutils.LogTest(t, "Waiting for export to enter streaming mode...")
		require.NoError(t, waitForStreamingModeImportTest(exportDir, 120*time.Second, 2*time.Second), "Export should enter streaming mode")
		testutils.LogTest(t, "Export reached streaming mode; generating CDC inserts...")

		// Multi-channel determinism:
		// CDC import partitions events into `NUM_EVENT_CHANNELS` by hashing (table + key) and taking modulo N.
		// To ensure this test actually exercises all channels (and doesn't accidentally generate events for
		// only 1-2 channels), we *pre-select* INSERT primary keys whose hash maps into each channel.
		//
		// Our `computeChanForIDImportTest` uses the same FNV64a scheme used by the importer:
		// - hash the table FQN
		// - hash the primary key value (as a string)
		// - channel = hash % NUM_EVENT_CHANNELS
		//
		// Then we generate:
		// - INSERT events for all selected IDs (spread across channels)
		// - UPDATE events on a per-channel subset of those IDs
		// - DELETE events on a smaller per-channel subset (subset of updated IDs)
		idsByChan := make([][]int, numChans)
		for id := 1000; id < 100000; id++ {
			ch := computeChanForIDImportTest(tableFQN, id, numChans)
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

		// Insert all chosen IDs in one shot.
		vals := make([]string, 0, numChans*eventsPerChan)
		for c := 0; c < numChans; c++ {
			for _, id := range idsByChan[c] {
				vals = append(vals, fmt.Sprintf("(%d, 'cdc_ins_%d', %d)", id, id, 10000+id))
			}
		}
		insertStmt := fmt.Sprintf(
			"INSERT INTO %s (id, name, value) VALUES %s;",
			tableFQN,
			strings.Join(vals, ","),
		)
		postgresContainer.ExecuteSqls(insertStmt)

		time.Sleep(5 * time.Second)

		// UPDATE a subset of inserted rows in each channel.
		updateIDs := make([]string, 0, numChans*updatesPerChan)
		for c := 0; c < numChans; c++ {
			for i := 0; i < updatesPerChan; i++ {
				updateIDs = append(updateIDs, strconv.Itoa(idsByChan[c][i]))
			}
		}
		updateStmt := fmt.Sprintf(
			"UPDATE %s SET value = value + 50000, name = 'cdc_upd_' || id WHERE id IN (%s);",
			tableFQN,
			strings.Join(updateIDs, ","),
		)
		testutils.LogTestf(t, "Generating CDC updates: %d rows", len(updateIDs))
		postgresContainer.ExecuteSqls(updateStmt)

		time.Sleep(5 * time.Second)

		// DELETE a smaller subset (also per channel). These IDs are a subset of the updated IDs.
		deleteIDs := make([]string, 0, numChans*deletesPerChan)
		for c := 0; c < numChans; c++ {
			for i := 0; i < deletesPerChan; i++ {
				deleteIDs = append(deleteIDs, strconv.Itoa(idsByChan[c][i]))
			}
		}
		deleteStmt := fmt.Sprintf(
			"DELETE FROM %s WHERE id IN (%s);",
			tableFQN,
			strings.Join(deleteIDs, ","),
		)
		testutils.LogTestf(t, "Generating CDC deletes: %d rows", len(deleteIDs))
		postgresContainer.ExecuteSqls(deleteStmt)

		expected := (numChans * eventsPerChan) + (numChans * updatesPerChan) + (numChans * deletesPerChan)
		testutils.LogTestf(t, "Waiting for %d CDC events to be queued to segment files...", expected)
		waitForCDCEventCountImportTest(t, exportDir, expected, 240*time.Second, 5*time.Second)
		testutils.LogTest(t, "Verifying no duplicate event_id values in queued CDC...")
		verifyNoEventIDDuplicatesImportTest(t, exportDir)
		testutils.LogTest(t, "CDC queued and verified")
		cdcQueued <- true
	}

	exportRunner := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_import_cdc_multi_chan",
		"--disable-pb", "true",
		"--yes",
	}, generateCDC, true)
	err = exportRunner.Run()
	require.NoError(t, err, "Failed to start export")
	defer killDebeziumForExportDirImportTest(t, exportDir)

	select {
	case <-cdcQueued:
	case <-time.After(180 * time.Second):
		_ = exportRunner.Kill()
		require.Fail(t, "Timed out waiting for CDC events to be queued")
	}

	testutils.LogTest(t, "Stopping export after CDC has been queued")
	_ = exportRunner.Kill()
	killDebeziumForExportDirImportTest(t, exportDir)
	_ = os.Remove(filepath.Join(exportDir, ".export-dataLockfile.lck"))
	time.Sleep(2 * time.Second)

	migrationUUIDStr, err := readMigrationUUID(exportDir)
	require.NoError(t, err, "Failed to read migration UUID from exportDir")
	require.NotEmpty(t, migrationUUIDStr, "migration UUID should not be empty")
	testutils.LogTestf(t, "Migration UUID: %s", migrationUUIDStr)

	failpointEnv := testutils.GetFailpointEnvVar(
		// Crash after 100 successful CDC batches across all channels.
		"github.com/yugabyte/yb-voyager/yb-voyager/cmd/importCDCBatchDBError=100*off->return(true)",
	)

	testutils.LogTest(t, "Running import with multi-channel batch failure failpoint (expected to crash)...")
	importWithFailpoint := testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--max-retries-streaming", "1",
		"--yes",
	}, nil, true).WithEnv(
		failpointEnv,
		fmt.Sprintf("YB_VOYAGER_FAILPOINT_MARKER_DIR=%s", filepath.Join(exportDir, "logs")),
		fmt.Sprintf("NUM_EVENT_CHANNELS=%d", numChans),
		"MAX_EVENTS_PER_BATCH=1",
		"MAX_INTERVAL_BETWEEN_BATCHES=1",
		"EVENT_CHANNEL_SIZE=20",
	)
	err = importWithFailpoint.Run()
	require.NoError(t, err, "Failed to start import with failpoint")

	failMarkerPath := filepath.Join(exportDir, "logs", "failpoint-import-cdc-db-error.log")
	testutils.LogTestf(t, "Waiting for failpoint marker: %s", failMarkerPath)
	matched, err := testutils.WaitForFailpointMarker(failMarkerPath, 60*time.Second, 2*time.Second)
	require.NoError(t, err, "Should be able to read multi-channel batch failure marker")
	require.True(t, matched, "Multi-channel batch failure marker did not trigger")
	testutils.LogTest(t, "✓ Verified multi-channel batch failure failpoint marker was written")

	testutils.LogTest(t, "Failpoint marker detected; waiting for import process to exit with error...")
	_, waitErr := testutils.WaitForProcessExitOrKill(importWithFailpoint, 60*time.Second)
	require.Error(t, waitErr, "Import should exit with error after multi-channel batch failure failpoint")
	_ = os.Remove(filepath.Join(exportDir, ".import-dataLockfile.lck"))

	maxQueuedVsn, err := maxVsnInQueueSegments(exportDir)
	require.NoError(t, err, "Failed to compute max queued vsn from queue segments")
	testutils.LogTestf(t, "Max queued vsn in CDC segments: %d", maxQueuedVsn)

	ybConnForChecks, err := yugabytedbContainer.GetConnection()
	require.NoError(t, err, "Failed to get YugabyteDB connection for mid-test checks")
	defer ybConnForChecks.Close()

	lastAppliedVsn, err := maxLastAppliedVsn(ybConnForChecks, migrationUUIDStr)
	require.NoError(t, err, "Failed to read last_applied_vsn from event channels metadata")
	testutils.LogTestf(t, "Max last_applied_vsn on target after failure: %d", lastAppliedVsn)
	require.Less(t, lastAppliedVsn, maxQueuedVsn, "Expected partial progress (max last_applied_vsn < max queued vsn)")

	byChan, err := lastAppliedVsnsByChannelImportTest(ybConnForChecks, migrationUUIDStr)
	require.NoError(t, err, "Failed to read per-channel last_applied_vsn")
	require.Equal(t, numChans, len(byChan), "Expected one metadata row per channel")
	progressed := 0
	for c := 0; c < numChans; c++ {
		if byChan[c] >= 0 {
			progressed++
		}
	}
	require.GreaterOrEqual(t, progressed, 2, "Expected at least two channels to have applied some events before failure")

	pgConnForChecks, err := postgresContainer.GetConnection()
	require.NoError(t, err, "Failed to get PostgreSQL connection for mid-test mismatch check")
	defer pgConnForChecks.Close()
	require.Error(t,
		testutils.CompareTableData(ctx, pgConnForChecks, ybConnForChecks, tableFQN, "id"),
		"Expected source != target after crash",
	)

	testutils.LogTest(t, "Resuming import without failpoint and waiting for target to match source...")
	importResume := testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--yes",
	}, nil, true).WithEnv(
		fmt.Sprintf("NUM_EVENT_CHANNELS=%d", numChans),
		"MAX_EVENTS_PER_BATCH=1",
		"MAX_INTERVAL_BETWEEN_BATCHES=1",
		"EVENT_CHANNEL_SIZE=20",
	)
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
		return testutils.CompareTableData(ctx, pgConn, ybConn, tableFQN, "id") == nil
	}, 240*time.Second, 5*time.Second, "Timed out waiting for target to match source after resume")
	testutils.LogTest(t, "✓ Target matches source after resume (multi-channel)")

	byChanAfter, err := lastAppliedVsnsByChannelImportTest(ybConn, migrationUUIDStr)
	require.NoError(t, err, "Failed to read per-channel last_applied_vsn after resume")
	require.Equal(t, numChans, len(byChanAfter), "Expected one metadata row per channel after resume")
	for c := 0; c < numChans; c++ {
		require.Greater(t, byChanAfter[c], int64(-1), "Expected channel %d to have applied at least one event", c)
		require.LessOrEqual(t, byChanAfter[c], maxQueuedVsn, "Expected channel %d last_applied_vsn <= max queued vsn", c)
	}

	// best-effort shutdown
	_ = importResume.Kill()
	_ = os.Remove(filepath.Join(exportDir, ".import-dataLockfile.lck"))
}

// waitForStreamingModeImportTest waits until `export data` transitions to streaming mode.
// This is used to ensure snapshot export finished before generating CDC changes.
func waitForStreamingModeImportTest(exportDir string, timeout time.Duration, pollInterval time.Duration) error {
	deadline := time.Now().Add(timeout)
	statusPath := filepath.Join(exportDir, "data", "export_status.json")
	for time.Now().Before(deadline) {
		status, err := dbzm.ReadExportStatus(statusPath)
		if err == nil && status != nil && status.Mode == dbzm.MODE_STREAMING {
			return nil
		}
		time.Sleep(pollInterval)
	}
	return fmt.Errorf("timed out waiting for export streaming mode")
}

// computeChanForIDImportTest returns the importer channel number for a single-row CDC event
// using the same hashing scheme as `cmd/live_migration.go` (FNV64a(tableFQN + key) % N).
func computeChanForIDImportTest(tableFQN string, id int, numChans int) int {
	h := fnv.New64a()
	h.Write([]byte(tableFQN))
	h.Write([]byte(strconv.Itoa(id)))
	return int(h.Sum64() % uint64(numChans))
}

// lastAppliedVsnsByChannelImportTest reads per-channel `last_applied_vsn` for a migration UUID
// from voyager's target-side metadata table.
func lastAppliedVsnsByChannelImportTest(ybConn interface {
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

// killDebeziumForExportDirImportTest force-kills the Debezium Java process for an exportDir.
//
// Import-side CDC tests run `export data --export-type snapshot-and-changes` as a black-box process.
// Debezium runs as a separate child Java process and can outlive the `yb-voyager` parent if the parent
// is SIGKILLed. This helper prevents leaked Java processes between tests.
func killDebeziumForExportDirImportTest(t *testing.T, exportDir string) {
	t.Helper()

	// Best-effort cleanup for black-box tests that run `export data`:
	// Debezium runs as a separate Java process and can outlive the `yb-voyager` parent if it is SIGKILLed.
	pidStr, err := dbzm.GetPIDOfDebeziumOnExportDir(exportDir, SOURCE_DB_EXPORTER_ROLE)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		testutils.LogTestf(t, "WARNING: failed to read Debezium PID from exportDir: %v", err)
		return
	}

	pid, err := strconv.Atoi(strings.TrimSpace(pidStr))
	if err != nil {
		testutils.LogTestf(t, "WARNING: failed to parse Debezium PID %q: %v", pidStr, err)
		return
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		testutils.LogTestf(t, "WARNING: failed to find Debezium process pid=%d: %v", pid, err)
		return
	}
	if err := proc.Kill(); err != nil {
		testutils.LogTestf(t, "WARNING: failed to kill Debezium process pid=%d: %v", pid, err)
		return
	}
	testutils.LogTestf(t, "Killed Debezium process pid=%d", pid)
}

// countEventsInQueueSegmentsImportTest counts CDC events present in `<exportDir>/data/queue/*.ndjson`.
// It ignores empty lines and the `\.` EOF marker lines.
func countEventsInQueueSegmentsImportTest(exportDir string) (int, error) {
	queueDir := filepath.Join(exportDir, "data", "queue")
	entries, err := os.ReadDir(queueDir)
	if err != nil {
		return 0, err
	}

	total := 0
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
			total++
		}
		if err := scanner.Err(); err != nil {
			f.Close()
			return 0, err
		}
		f.Close()
	}
	return total, nil
}

// waitForCDCEventCountImportTest waits until at least `expected` CDC events are present in the queue.
// Returns the last observed count.
func waitForCDCEventCountImportTest(t *testing.T, exportDir string, expected int, timeout, pollInterval time.Duration) int {
	t.Helper()
	var last int
	require.Eventually(t, func() bool {
		n, err := countEventsInQueueSegmentsImportTest(exportDir)
		if err != nil {
			return false
		}
		last = n
		return n >= expected
	}, timeout, pollInterval, "timed out waiting for CDC events in queue (expected>=%d, last=%d)", expected, last)
	return last
}

// verifyNoEventIDDuplicatesImportTest scans queued CDC events and asserts that every `event_id`
// is present and unique across all queue segment files.
func verifyNoEventIDDuplicatesImportTest(t *testing.T, exportDir string) {
	t.Helper()

	queueDir := filepath.Join(exportDir, "data", "queue")
	entries, err := os.ReadDir(queueDir)
	require.NoError(t, err, "Failed to read queue directory")

	seen := map[string]struct{}{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "segment.") || !strings.HasSuffix(name, ".ndjson") {
			continue
		}
		f, err := os.Open(filepath.Join(queueDir, name))
		require.NoError(t, err, "Failed to open queue segment file")
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || line == `\.` {
				continue
			}
			var m map[string]any
			err := json.Unmarshal([]byte(line), &m)
			require.NoError(t, err, "Malformed CDC JSON in queue segment")
			raw := m["event_id"]
			eventID, _ := raw.(string)
			require.NotEmpty(t, eventID, "Missing event_id in CDC JSON")
			_, dup := seen[eventID]
			require.False(t, dup, "Duplicate event_id found: %s", eventID)
			seen[eventID] = struct{}{}
		}
		require.NoError(t, scanner.Err(), "Failed scanning queue segment file")
		f.Close()
	}
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

// readMigrationUUID reads the migration UUID from exportDir's metadb.
func readMigrationUUID(exportDir string) (string, error) {
	m, err := metadb.NewMetaDB(exportDir)
	if err != nil {
		return "", err
	}
	msr, err := m.GetMigrationStatusRecord()
	if err != nil {
		return "", err
	}
	if msr == nil {
		return "", fmt.Errorf("migration status record not found")
	}
	return msr.MigrationUUID, nil
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
