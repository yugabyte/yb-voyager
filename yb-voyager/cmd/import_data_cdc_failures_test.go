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

	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"database/sql"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// TestImportCDCTransformFailureAndResume verifies that live migration `import data` can resume after
// a CDC-transform failure during the streaming apply phase.
//
// Scenario:
// 1. Start `export data --export-type snapshot-and-changes` and wait for streaming mode.
// 2. Generate CDC changes (INSERT/UPDATE/DELETE) and wait until they are queued to segment files.
// 3. Stop export to freeze the queue (avoid new queue writes racing with import assertions).
// 4. Run `import data` with failpoint injection in CDC transform path (expect crash mid-stream).
// 5. After crash, verify partial progress by checking:
//    - max queued vsn from queue segments
//    - max last_applied_vsn from target voyager metadata
// 6. Resume `import data` without failpoint and verify target matches source.
//
// This test validates:
// - CDC apply resumability after transformation failure
// - Idempotent resume (final target state matches source exactly)
//
// Notes on determinism:
// - Failpoint triggers in `cmd/live_migration.go` inside `handleEvent()` after ConvertEvent().
// - We set small buffering/batching via env to ensure `processEvents()` commits some CDC before
//   the failpoint triggers, so `last_applied_vsn > 0` is observable:
//   - MAX_EVENTS_PER_BATCH=1
//   - MAX_INTERVAL_BETWEEN_BATCHES=1
//   - EVENT_CHANNEL_SIZE=2
func TestImportCDCTransformFailureAndResume(t *testing.T) {
	ctx := context.Background()

	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	logTestf(t, "Using exportDir=%s", exportDir)
	migrationUUIDStr := ""

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
		"DROP SCHEMA IF EXISTS test_schema_import_cdc CASCADE;",
		"CREATE SCHEMA test_schema_import_cdc;",
		`CREATE TABLE test_schema_import_cdc.cdc_import_test (
			id SERIAL PRIMARY KEY,
			name TEXT,
			value INTEGER
		);`,
		`ALTER TABLE test_schema_import_cdc.cdc_import_test REPLICA IDENTITY FULL;`,
		`INSERT INTO test_schema_import_cdc.cdc_import_test (name, value)
		 SELECT 'snapshot_' || i, i FROM generate_series(1, 30) i;`,
	)
	defer postgresContainer.ExecuteSqls("DROP SCHEMA IF EXISTS test_schema_import_cdc CASCADE;")

	yugabytedbContainer.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema_import_cdc CASCADE;",
		"CREATE SCHEMA test_schema_import_cdc;",
		`CREATE TABLE test_schema_import_cdc.cdc_import_test (
			id SERIAL PRIMARY KEY,
			name TEXT,
			value INTEGER
		);`,
	)
	defer yugabytedbContainer.ExecuteSqls("DROP SCHEMA IF EXISTS test_schema_import_cdc CASCADE;")

	cdcQueued := make(chan bool, 1)
	generateCDC := func() {
		logTest(t, "Waiting for export to enter streaming mode...")
		require.NoError(t, waitForStreamingModeImportTest(exportDir, 120*time.Second, 2*time.Second), "Export should enter streaming mode")
		logTest(t, "Export reached streaming mode; generating CDC changes...")

		// Batch 1: INSERT 200 rows
		logTest(t, "CDC batch 1: INSERT 200 rows")
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema_import_cdc.cdc_import_test (name, value)
			 SELECT 'cdc_ins_' || i, 1000 + i FROM generate_series(1, 200) i;`,
		)
		time.Sleep(5 * time.Second)

		// Batch 2: UPDATE 200 rows (forces update-path conversions)
		logTest(t, "CDC batch 2: UPDATE 200 rows")
		postgresContainer.ExecuteSqls(
			`UPDATE test_schema_import_cdc.cdc_import_test
			 SET value = value + 10000
			 WHERE id BETWEEN 1 AND 200;`,
		)
		time.Sleep(5 * time.Second)

		// Batch 3: DELETE 100 rows
		logTest(t, "CDC batch 3: DELETE 100 rows")
		postgresContainer.ExecuteSqls(
			`DELETE FROM test_schema_import_cdc.cdc_import_test WHERE id BETWEEN 11 AND 110;`,
		)

		// Expect 200 inserts + 200 updates + 100 deletes = 500 CDC events in queue.
		logTest(t, "Waiting for 500 CDC events to be queued to segment files...")
		waitForCDCEventCountImportTest(t, exportDir, 500, 240*time.Second, 5*time.Second)
		logTest(t, "Verifying no duplicate event_id values in queued CDC...")
		verifyNoEventIDDuplicatesImportTest(t, exportDir)
		logTest(t, "CDC queued and verified")
		cdcQueued <- true
	}

	exportRunner := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_import_cdc",
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

	// Stop export to avoid further queue writes during import assertions.
	logTest(t, "Stopping export after CDC has been queued")
	_ = exportRunner.Kill()
	killDebeziumForExportDirImportTest(t, exportDir)
	_ = os.Remove(filepath.Join(exportDir, ".export-dataLockfile.lck"))
	time.Sleep(2 * time.Second)

	migrationUUIDStr, err = readMigrationUUIDFromExportDirImportTest(exportDir)
	require.NoError(t, err, "Failed to read migration UUID from exportDir")
	require.NotEmpty(t, migrationUUIDStr, "migration UUID should not be empty")
	logTestf(t, "Migration UUID: %s", migrationUUIDStr)

	failpointEnv := testutils.GetFailpointEnvVar(
		// Trigger late enough that some CDC progress is committed before the crash.
		"github.com/yugabyte/yb-voyager/yb-voyager/cmd/importCDCTransformFailure=250*off->return()",
	)
	logTest(t, "Running import with CDC transform failpoint (expected to crash)...")

	importWithFailpoint := testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--yes",
	}, nil, true).WithEnv(
		failpointEnv,
		// Force frequent batch commits AND reduce channel buffering so processEvents makes progress
		// before the failpoint triggers in handleEvent().
		"MAX_EVENTS_PER_BATCH=1",
		"MAX_INTERVAL_BETWEEN_BATCHES=1",
		"EVENT_CHANNEL_SIZE=2",
	)

	err = importWithFailpoint.Run()
	require.NoError(t, err, "Failed to start import with failpoint")

	failMarkerPath := filepath.Join(exportDir, "logs", "failpoint-import-cdc-transform.log")
	logTestf(t, "Waiting for failpoint marker: %s", failMarkerPath)
	matched, err := waitForMarkerFileImportTest(failMarkerPath, 60*time.Second, 2*time.Second)
	require.NoError(t, err, "Should be able to read CDC transform failure marker")
	if !matched {
		_ = importWithFailpoint.Kill()
		require.Fail(t, "CDC transform failure marker did not trigger (failpoints may not be enabled). "+
			"Make sure to run `failpoint-ctl enable` before `go test -tags=failpoint`.")
	}

	logTest(t, "Failpoint marker detected; waiting for import process to exit with error...")
	_, waitErr := waitForProcessExitOrKillImportTest(importWithFailpoint, 60*time.Second)
	require.Error(t, waitErr, "Import should exit with error after CDC transform failpoint")
	_ = os.Remove(filepath.Join(exportDir, ".import-dataLockfile.lck"))

	// Mid-test verification: ensure we made partial CDC progress before failing, so resume work is meaningful.
	// We do this by comparing queued CDC max vsn with the target importer channels last_applied_vsn.
	maxQueuedVsn, err := maxVsnInQueueSegmentsImportTest(exportDir)
	require.NoError(t, err, "Failed to compute max queued vsn from queue segments")
	logTestf(t, "Max queued vsn in CDC segments: %d", maxQueuedVsn)

	ybConnForChecks, err := yugabytedbContainer.GetConnection()
	require.NoError(t, err, "Failed to get YugabyteDB connection for mid-test checks")
	defer ybConnForChecks.Close()

	lastAppliedVsn, err := maxLastAppliedVsnImportTest(ybConnForChecks, migrationUUIDStr)
	require.NoError(t, err, "Failed to read last_applied_vsn from event channels metadata")
	logTestf(t, "Max last_applied_vsn on target after failure: %d", lastAppliedVsn)

	var targetRowCount int
	err = ybConnForChecks.QueryRow("SELECT COUNT(*) FROM test_schema_import_cdc.cdc_import_test").Scan(&targetRowCount)
	require.NoError(t, err, "Failed to query target row count after failure")
	logTestf(t, "Target row count after failure: %d", targetRowCount)

	require.Greater(t, lastAppliedVsn, int64(0), "Expected some CDC progress before failure (last_applied_vsn > 0)")
	require.Less(t, lastAppliedVsn, maxQueuedVsn, "Expected partial progress (last_applied_vsn < max queued vsn)")

	// Resume import without failpoint. Live migration import does not naturally terminate,
	// so we run async, wait until data matches, then kill the process.
	logTest(t, "Resuming import without failpoint and waiting for target to match source...")
	importResume := testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--yes",
	}, nil, true).WithEnv(
		"MAX_EVENTS_PER_BATCH=1",
		"MAX_INTERVAL_BETWEEN_BATCHES=1",
		"EVENT_CHANNEL_SIZE=2",
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
		// CompareTableData returns nil only when fully caught up.
		return testutils.CompareTableData(ctx, pgConn, ybConn, "test_schema_import_cdc.cdc_import_test", "id") == nil
	}, 180*time.Second, 5*time.Second, "Timed out waiting for target to match source after resume")

	logTest(t, "✓ Target matches source after resume")

	// best-effort shutdown
	_ = importResume.Kill()
	_ = os.Remove(filepath.Join(exportDir, ".import-dataLockfile.lck"))
}

// TestImportCDCDbErrorAndResume verifies that live migration `import data` can resume after
// a CDC apply failure caused by a simulated target DB error during the streaming apply phase.
//
// Scenario:
// 1. Run `export data --export-type snapshot-and-changes` and wait for streaming mode.
// 2. Generate a CDC workload and wait until the queue segments persist all events.
// 3. Stop export to freeze the queue (avoid new queue writes racing with import assertions).
// 4. Run `import data` with a failpoint that injects a non-retryable SQLSTATE error inside
//    `processEvents()` right before `tdb.ExecuteBatch()` (expect crash mid-stream).
// 5. After crash, verify partial progress by checking:
//    - max queued vsn from queue segments
//    - max last_applied_vsn from target voyager metadata
// 6. Resume `import data` without failpoint and verify target matches source.
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

	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)
	logTestf(t, "Using exportDir=%s", exportDir)

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
		"DROP SCHEMA IF EXISTS test_schema_import_cdc_db_err CASCADE;",
		"CREATE SCHEMA test_schema_import_cdc_db_err;",
		`CREATE TABLE test_schema_import_cdc_db_err.cdc_import_test (
			id SERIAL PRIMARY KEY,
			name TEXT,
			value INTEGER
		);`,
		`ALTER TABLE test_schema_import_cdc_db_err.cdc_import_test REPLICA IDENTITY FULL;`,
		`INSERT INTO test_schema_import_cdc_db_err.cdc_import_test (name, value)
		 SELECT 'snapshot_' || i, i FROM generate_series(1, 30) i;`,
	)
	defer postgresContainer.ExecuteSqls("DROP SCHEMA IF EXISTS test_schema_import_cdc_db_err CASCADE;")

	yugabytedbContainer.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema_import_cdc_db_err CASCADE;",
		"CREATE SCHEMA test_schema_import_cdc_db_err;",
		`CREATE TABLE test_schema_import_cdc_db_err.cdc_import_test (
			id SERIAL PRIMARY KEY,
			name TEXT,
			value INTEGER
		);`,
	)
	defer yugabytedbContainer.ExecuteSqls("DROP SCHEMA IF EXISTS test_schema_import_cdc_db_err CASCADE;")

	cdcQueued := make(chan bool, 1)
	generateCDC := func() {
		logTest(t, "Waiting for export to enter streaming mode...")
		require.NoError(t, waitForStreamingModeImportTest(exportDir, 120*time.Second, 2*time.Second), "Export should enter streaming mode")
		logTest(t, "Export reached streaming mode; generating CDC changes...")

		logTest(t, "CDC batch 1: INSERT 200 rows")
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema_import_cdc_db_err.cdc_import_test (name, value)
			 SELECT 'cdc_ins_' || i, 1000 + i FROM generate_series(1, 200) i;`,
		)
		time.Sleep(5 * time.Second)

		logTest(t, "CDC batch 2: UPDATE 200 rows")
		postgresContainer.ExecuteSqls(
			`UPDATE test_schema_import_cdc_db_err.cdc_import_test
			 SET value = value + 10000
			 WHERE id BETWEEN 1 AND 200;`,
		)
		time.Sleep(5 * time.Second)

		logTest(t, "CDC batch 3: DELETE 100 rows")
		postgresContainer.ExecuteSqls(
			`DELETE FROM test_schema_import_cdc_db_err.cdc_import_test WHERE id BETWEEN 11 AND 110;`,
		)

		logTest(t, "Waiting for 500 CDC events to be queued to segment files...")
		waitForCDCEventCountImportTest(t, exportDir, 500, 240*time.Second, 5*time.Second)
		logTest(t, "Verifying no duplicate event_id values in queued CDC...")
		verifyNoEventIDDuplicatesImportTest(t, exportDir)
		logTest(t, "CDC queued and verified")
		cdcQueued <- true
	}

	exportRunner := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_import_cdc_db_err",
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

	logTest(t, "Stopping export after CDC has been queued")
	_ = exportRunner.Kill()
	killDebeziumForExportDirImportTest(t, exportDir)
	_ = os.Remove(filepath.Join(exportDir, ".export-dataLockfile.lck"))
	time.Sleep(2 * time.Second)

	migrationUUIDStr, err := readMigrationUUIDFromExportDirImportTest(exportDir)
	require.NoError(t, err, "Failed to read migration UUID from exportDir")
	require.NotEmpty(t, migrationUUIDStr, "migration UUID should not be empty")
	logTestf(t, "Migration UUID: %s", migrationUUIDStr)

	failpointEnv := testutils.GetFailpointEnvVar(
		// Crash after 100 successful CDC batches have been committed, so `last_applied_vsn > 0`
		// is observable and we can prove resume work is meaningful.
		"github.com/yugabyte/yb-voyager/yb-voyager/cmd/importCDCBatchDBError=100*off->return(true)",
	)

	logTest(t, "Running import with CDC DB-error failpoint (expected to crash)...")
	importWithFailpoint := testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--max-retries-streaming", "1",
		"--yes",
	}, nil, true).WithEnv(
		failpointEnv,
		// Make "Nth batch" deterministic and keep progress visible.
		"NUM_EVENT_CHANNELS=1",
		"MAX_EVENTS_PER_BATCH=1",
		"MAX_INTERVAL_BETWEEN_BATCHES=1",
		"EVENT_CHANNEL_SIZE=2",
	)
	err = importWithFailpoint.Run()
	require.NoError(t, err, "Failed to start import with failpoint")

	failMarkerPath := filepath.Join(exportDir, "logs", "failpoint-import-cdc-db-error.log")
	logTestf(t, "Waiting for failpoint marker: %s", failMarkerPath)
	matched, err := waitForMarkerFileImportTest(failMarkerPath, 60*time.Second, 2*time.Second)
	require.NoError(t, err, "Should be able to read CDC DB-error marker")
	if !matched {
		_ = importWithFailpoint.Kill()
		require.Fail(t, "CDC DB-error marker did not trigger (failpoints may not be enabled). "+
			"Make sure to run `failpoint-ctl enable` before `go test -tags=failpoint`.")
	}

	logTest(t, "Failpoint marker detected; waiting for import process to exit with error...")
	_, waitErr := waitForProcessExitOrKillImportTest(importWithFailpoint, 60*time.Second)
	require.Error(t, waitErr, "Import should exit with error after CDC DB-error failpoint")
	_ = os.Remove(filepath.Join(exportDir, ".import-dataLockfile.lck"))

	maxQueuedVsn, err := maxVsnInQueueSegmentsImportTest(exportDir)
	require.NoError(t, err, "Failed to compute max queued vsn from queue segments")
	logTestf(t, "Max queued vsn in CDC segments: %d", maxQueuedVsn)

	ybConnForChecks, err := yugabytedbContainer.GetConnection()
	require.NoError(t, err, "Failed to get YugabyteDB connection for mid-test checks")
	defer ybConnForChecks.Close()

	lastAppliedVsn, err := maxLastAppliedVsnImportTest(ybConnForChecks, migrationUUIDStr)
	require.NoError(t, err, "Failed to read last_applied_vsn from event channels metadata")
	logTestf(t, "Max last_applied_vsn on target after failure: %d", lastAppliedVsn)

	var targetRowCount int
	err = ybConnForChecks.QueryRow("SELECT COUNT(*) FROM test_schema_import_cdc_db_err.cdc_import_test").Scan(&targetRowCount)
	require.NoError(t, err, "Failed to query target row count after failure")
	logTestf(t, "Target row count after failure: %d", targetRowCount)

	require.Greater(t, lastAppliedVsn, int64(0), "Expected some CDC progress before failure (last_applied_vsn > 0)")
	require.Less(t, lastAppliedVsn, maxQueuedVsn, "Expected partial progress (last_applied_vsn < max queued vsn)")

	logTest(t, "Resuming import without failpoint and waiting for target to match source...")
	importResume := testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--yes",
	}, nil, true).WithEnv(
		"NUM_EVENT_CHANNELS=1",
		"MAX_EVENTS_PER_BATCH=1",
		"MAX_INTERVAL_BETWEEN_BATCHES=1",
		"EVENT_CHANNEL_SIZE=2",
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
		// CompareTableData returns nil only when fully caught up.
		return testutils.CompareTableData(ctx, pgConn, ybConn, "test_schema_import_cdc_db_err.cdc_import_test", "id") == nil
	}, 180*time.Second, 5*time.Second, "Timed out waiting for target to match source after resume")

	logTest(t, "✓ Target matches source after resume (CDC DB error)")

	// best-effort shutdown
	_ = importResume.Kill()
	_ = os.Remove(filepath.Join(exportDir, ".import-dataLockfile.lck"))
}

// TestImportCDCProgressMetadataWriteFailureAndResume verifies that live migration `import data` can
// resume successfully even if a CDC batch's DML is committed on the target but voyager progress
// metadata (last_applied_vsn + per-table event stats) is not updated for that batch.
//
// Scenario:
// 1. Run `export data --export-type snapshot-and-changes` and wait for streaming mode.
// 2. Generate a CDC workload with INSERTs + UPDATEs + DELETEs and wait until the queue persists all events.
// 3. Stop export to freeze the queue.
// 4. Run `import data` with a failpoint injected inside `tgtdb.TargetYugabyteDB.ExecuteBatch()` that:
//    - commits the CDC DML
//    - skips updating `ybvoyager_metadata.ybvoyager_import_data_event_channels_metainfo`
//    - exits with an error
// 5. After crash, verify:
//    - `last_applied_vsn > 0` and `< max queued vsn`
//    - the CDC event with VSN = last_applied_vsn+1 was actually applied (DELETE reflected on target),
//      proving that data progressed further than progress metadata.
// 6. Resume `import data` without failpoint and verify the target matches source.
//
// Notes on determinism:
// - We force NUM_EVENT_CHANNELS=1 and MAX_EVENTS_PER_BATCH=1 so "Nth batch" aligns with "Nth CDC event".
// - We deliberately fail on the first DELETE event so the probe event is a DELETE that will be re-applied on resume.
func TestImportCDCProgressMetadataWriteFailureAndResume(t *testing.T) {
	ctx := context.Background()

	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)
	logTestf(t, "Using exportDir=%s", exportDir)

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
		"DROP SCHEMA IF EXISTS test_schema_import_cdc_progress_meta CASCADE;",
		"CREATE SCHEMA test_schema_import_cdc_progress_meta;",
		`CREATE TABLE test_schema_import_cdc_progress_meta.cdc_import_test (
			id SERIAL PRIMARY KEY,
			name TEXT,
			value INTEGER
		);`,
		`ALTER TABLE test_schema_import_cdc_progress_meta.cdc_import_test REPLICA IDENTITY FULL;`,
		`INSERT INTO test_schema_import_cdc_progress_meta.cdc_import_test (name, value)
		 SELECT 'snapshot_' || i, i FROM generate_series(1, 30) i;`,
	)
	defer postgresContainer.ExecuteSqls("DROP SCHEMA IF EXISTS test_schema_import_cdc_progress_meta CASCADE;")

	yugabytedbContainer.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema_import_cdc_progress_meta CASCADE;",
		"CREATE SCHEMA test_schema_import_cdc_progress_meta;",
		`CREATE TABLE test_schema_import_cdc_progress_meta.cdc_import_test (
			id SERIAL PRIMARY KEY,
			name TEXT,
			value INTEGER
		);`,
	)
	defer yugabytedbContainer.ExecuteSqls("DROP SCHEMA IF EXISTS test_schema_import_cdc_progress_meta CASCADE;")

	cdcQueued := make(chan bool, 1)
	generateCDC := func() {
		logTest(t, "Waiting for export to enter streaming mode...")
		require.NoError(t, waitForStreamingModeImportTest(exportDir, 120*time.Second, 2*time.Second), "Export should enter streaming mode")
		logTest(t, "Export reached streaming mode; generating CDC inserts...")

		// Mixed workload:
		// - 150 INSERTs (VSN 1..150)
		// - 100 UPDATEs (VSN 151..250)
		// - 50 DELETEs  (VSN 251..300)
		//
		// With NUM_EVENT_CHANNELS=1 and MAX_EVENTS_PER_BATCH=1, we can deterministically target
		// the first DELETE event (VSN 251) for the failpoint.
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema_import_cdc_progress_meta.cdc_import_test (name, value)
			 SELECT 'cdc_ins_' || i, 10000 + i FROM generate_series(1, 150) i;`,
		)
		time.Sleep(5 * time.Second)

		logTest(t, "Generating CDC updates...")
		postgresContainer.ExecuteSqls(
			`UPDATE test_schema_import_cdc_progress_meta.cdc_import_test
			 SET value = value + 100000,
			     name  = 'cdc_upd_' || id
			 WHERE id BETWEEN 31 AND 130;`,
		)
		time.Sleep(5 * time.Second)

		logTest(t, "Generating CDC deletes...")
		postgresContainer.ExecuteSqls(
			`DELETE FROM test_schema_import_cdc_progress_meta.cdc_import_test
			 WHERE id BETWEEN 31 AND 80;`,
		)

		logTest(t, "Waiting for 300 CDC events to be queued to segment files...")
		waitForCDCEventCountImportTest(t, exportDir, 300, 240*time.Second, 5*time.Second)
		logTest(t, "Verifying no duplicate event_id values in queued CDC...")
		verifyNoEventIDDuplicatesImportTest(t, exportDir)
		logTest(t, "CDC queued and verified")
		cdcQueued <- true
	}

	exportRunner := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_import_cdc_progress_meta",
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

	logTest(t, "Stopping export after CDC has been queued")
	_ = exportRunner.Kill()
	killDebeziumForExportDirImportTest(t, exportDir)
	_ = os.Remove(filepath.Join(exportDir, ".export-dataLockfile.lck"))
	time.Sleep(2 * time.Second)

	migrationUUIDStr, err := readMigrationUUIDFromExportDirImportTest(exportDir)
	require.NoError(t, err, "Failed to read migration UUID from exportDir")
	require.NotEmpty(t, migrationUUIDStr, "migration UUID should not be empty")
	logTestf(t, "Migration UUID: %s", migrationUUIDStr)

	failpointEnv := testutils.GetFailpointEnvVar(
		// Fail on the first DELETE (VSN 251). After failure:
		// - last_applied_vsn should be 250
		// - the DELETE DML is committed (row absent on target)
		"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb/importCDCProgressMetadataWriteFailure=250*off->return(true)",
	)

	logTest(t, "Running import with progress-metadata write failure failpoint (expected to crash)...")
	importWithFailpoint := testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--max-retries-streaming", "1",
		"--yes",
	}, nil, true).WithEnv(
		failpointEnv,
		fmt.Sprintf("YB_VOYAGER_FAILPOINT_MARKER_DIR=%s", filepath.Join(exportDir, "logs")),
		// Make "Nth batch" deterministic and keep progress visible.
		"NUM_EVENT_CHANNELS=1",
		"MAX_EVENTS_PER_BATCH=1",
		"MAX_INTERVAL_BETWEEN_BATCHES=1",
		"EVENT_CHANNEL_SIZE=2",
	)
	err = importWithFailpoint.Run()
	require.NoError(t, err, "Failed to start import with failpoint")

	failMarkerPath := filepath.Join(exportDir, "logs", "failpoint-import-cdc-progress-metadata-write-failure.log")
	logTestf(t, "Waiting for failpoint marker: %s", failMarkerPath)
	matched, err := waitForMarkerFileImportTest(failMarkerPath, 60*time.Second, 2*time.Second)
	require.NoError(t, err, "Should be able to read progress-metadata failure marker")
	if !matched {
		_ = importWithFailpoint.Kill()
		require.Fail(t, "Progress-metadata failure marker did not trigger (failpoints may not be enabled). "+
			"Make sure to run `failpoint-ctl enable` before `go test -tags=failpoint`.")
	}
	logTest(t, "✓ Verified progress-metadata write failure failpoint marker was written")

	logTest(t, "Failpoint marker detected; waiting for import process to exit with error...")
	_, waitErr := waitForProcessExitOrKillImportTest(importWithFailpoint, 60*time.Second)
	require.Error(t, waitErr, "Import should exit with error after progress-metadata write failpoint")
	_ = os.Remove(filepath.Join(exportDir, ".import-dataLockfile.lck"))

	maxQueuedVsn, err := maxVsnInQueueSegmentsImportTest(exportDir)
	require.NoError(t, err, "Failed to compute max queued vsn from queue segments")
	logTestf(t, "Max queued vsn in CDC segments: %d", maxQueuedVsn)

	ybConnForChecks, err := yugabytedbContainer.GetConnection()
	require.NoError(t, err, "Failed to get YugabyteDB connection for mid-test checks")
	defer ybConnForChecks.Close()

	lastAppliedVsn, err := maxLastAppliedVsnImportTest(ybConnForChecks, migrationUUIDStr)
	require.NoError(t, err, "Failed to read last_applied_vsn from event channels metadata")
	logTestf(t, "Max last_applied_vsn on target after failure: %d", lastAppliedVsn)

	require.Greater(t, lastAppliedVsn, int64(0), "Expected some CDC progress before failure (last_applied_vsn > 0)")
	require.Less(t, lastAppliedVsn, maxQueuedVsn, "Expected partial progress (last_applied_vsn < max queued vsn)")

	// Key verification: VSN=last_applied_vsn+1 should be present on target even though metadata didn't advance past last_applied_vsn.
	probeVsn := lastAppliedVsn + 1
	ev, err := eventByVsnInQueueSegmentsImportTest(exportDir, probeVsn)
	require.NoError(t, err, "Failed to read CDC event for probe VSN=%d", probeVsn)
	require.NotNil(t, ev, "Expected to find a CDC event for probe VSN=%d", probeVsn)
	require.Equal(t, int64(probeVsn), ev.Vsn, "Probe event VSN mismatch")
	require.Equal(t, "d", ev.Op, "Expected probe event to be a DELETE (we fail on first DELETE)")
	require.Equal(t, "test_schema_import_cdc_progress_meta.cdc_import_test", ev.TableFQN(), "Unexpected probe event table")

	probeID, err := ev.IntID()
	require.NoError(t, err, "Failed to get probe id from CDC event")

	// Control check: ensure UPDATE semantics are visible on the target post-failure.
	// id=81 is updated (31..130) but not deleted (31..80).
	var controlName string
	var controlValue int
	err = ybConnForChecks.QueryRow(
		"SELECT name, value FROM test_schema_import_cdc_progress_meta.cdc_import_test WHERE id = 81",
	).Scan(&controlName, &controlValue)
	require.NoError(t, err, "Expected control row (id=81) to exist on target after failure")
	require.Equal(t, "cdc_upd_81", controlName, "Control row name should reflect UPDATE")
	require.Equal(t, 110051, controlValue, "Control row value should reflect INSERT+UPDATE")

	// Probe check: the DELETE at VSN=last_applied_vsn+1 should be committed even though metadata did not advance.
	var probeCount int
	err = ybConnForChecks.QueryRow(
		"SELECT COUNT(*) FROM test_schema_import_cdc_progress_meta.cdc_import_test WHERE id = $1",
		probeID,
	).Scan(&probeCount)
	require.NoError(t, err, "Failed to query probe row existence on target")
	require.Equal(t, 0, probeCount, "Expected probe DELETE row to be absent on target (CDC DML committed)")
	logTestf(t, "✓ Verified probe VSN=%d (DELETE id=%d) is reflected on target while last_applied_vsn=%d", probeVsn, probeID, lastAppliedVsn)

	// And source should not match target yet (we crashed mid-stream).
	pgConnForChecks, err := postgresContainer.GetConnection()
	require.NoError(t, err, "Failed to get PostgreSQL connection for mid-test mismatch check")
	defer pgConnForChecks.Close()
	require.Error(t,
		testutils.CompareTableData(ctx, pgConnForChecks, ybConnForChecks, "test_schema_import_cdc_progress_meta.cdc_import_test", "id"),
		"Expected source != target after crash",
	)

	logTest(t, "Resuming import without failpoint and waiting for target to match source...")
	importResume := testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--yes",
	}, nil, true).WithEnv(
		"NUM_EVENT_CHANNELS=1",
		"MAX_EVENTS_PER_BATCH=1",
		"MAX_INTERVAL_BETWEEN_BATCHES=1",
		"EVENT_CHANNEL_SIZE=2",
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
		return testutils.CompareTableData(ctx, pgConn, ybConn, "test_schema_import_cdc_progress_meta.cdc_import_test", "id") == nil
	}, 240*time.Second, 5*time.Second, "Timed out waiting for target to match source after resume")

	logTest(t, "✓ Target matches source after resume (progress metadata write failure)")

	// best-effort shutdown
	_ = importResume.Kill()
	_ = os.Remove(filepath.Join(exportDir, ".import-dataLockfile.lck"))
}

// TestImportCDCEventExecutionFailureAndResume verifies that live migration `import data` can resume
// after a non-retryable target DB error occurs while applying a CDC event inside a batch.
//
// Scenario:
// 1. Run `export data --export-type snapshot-and-changes` and wait for streaming mode.
// 2. Generate CDC inserts and wait until queue segments persist all events.
// 3. Stop export to freeze the queue.
// 4. Run `import data` with failpoint injection inside `tgtdb.TargetYugabyteDB.ExecuteBatch()`
//    that fails on the (N+1)-th CDC event execution attempt (expect crash mid-stream).
// 5. After crash, verify partial progress (last_applied_vsn > 0) and confirm source != target.
// 6. Resume `import data` without failpoint and verify target matches source.
//
// Notes on determinism:
// - We set NUM_EVENT_CHANNELS=1 and MAX_EVENTS_PER_BATCH=10 so event execution ordering is stable.
// - Failpoint uses a hit-counter pattern: `50*off->return(true)` fails on the 51st event execution attempt.
func TestImportCDCEventExecutionFailureAndResume(t *testing.T) {
	ctx := context.Background()

	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)
	logTestf(t, "Using exportDir=%s", exportDir)

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
		"DROP SCHEMA IF EXISTS test_schema_import_cdc_event_fail CASCADE;",
		"CREATE SCHEMA test_schema_import_cdc_event_fail;",
		`CREATE TABLE test_schema_import_cdc_event_fail.cdc_import_test (
			id INTEGER PRIMARY KEY,
			name TEXT
		);`,
		`ALTER TABLE test_schema_import_cdc_event_fail.cdc_import_test REPLICA IDENTITY FULL;`,
		`INSERT INTO test_schema_import_cdc_event_fail.cdc_import_test (id, name)
		 SELECT i, 'snapshot_' || i FROM generate_series(1, 30) i;`,
	)
	defer postgresContainer.ExecuteSqls("DROP SCHEMA IF EXISTS test_schema_import_cdc_event_fail CASCADE;")

	yugabytedbContainer.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema_import_cdc_event_fail CASCADE;",
		"CREATE SCHEMA test_schema_import_cdc_event_fail;",
		`CREATE TABLE test_schema_import_cdc_event_fail.cdc_import_test (
			id INTEGER PRIMARY KEY,
			name TEXT
		);`,
	)
	defer yugabytedbContainer.ExecuteSqls("DROP SCHEMA IF EXISTS test_schema_import_cdc_event_fail CASCADE;")

	cdcQueued := make(chan bool, 1)
	generateCDC := func() {
		logTest(t, "Waiting for export to enter streaming mode...")
		require.NoError(t, waitForStreamingModeImportTest(exportDir, 120*time.Second, 2*time.Second), "Export should enter streaming mode")
		logTest(t, "Export reached streaming mode; generating CDC inserts...")

		// 120 inserts => 120 CDC events in queue.
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema_import_cdc_event_fail.cdc_import_test (id, name)
			 SELECT 1000 + i, 'cdc_ins_' || i FROM generate_series(1, 120) i;`,
		)

		logTest(t, "Waiting for 120 CDC events to be queued to segment files...")
		waitForCDCEventCountImportTest(t, exportDir, 120, 240*time.Second, 5*time.Second)
		logTest(t, "Verifying no duplicate event_id values in queued CDC...")
		verifyNoEventIDDuplicatesImportTest(t, exportDir)
		logTest(t, "CDC queued and verified")
		cdcQueued <- true
	}

	exportRunner := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_import_cdc_event_fail",
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

	logTest(t, "Stopping export after CDC has been queued")
	_ = exportRunner.Kill()
	killDebeziumForExportDirImportTest(t, exportDir)
	_ = os.Remove(filepath.Join(exportDir, ".export-dataLockfile.lck"))
	time.Sleep(2 * time.Second)

	migrationUUIDStr, err := readMigrationUUIDFromExportDirImportTest(exportDir)
	require.NoError(t, err, "Failed to read migration UUID from exportDir")
	require.NotEmpty(t, migrationUUIDStr, "migration UUID should not be empty")
	logTestf(t, "Migration UUID: %s", migrationUUIDStr)

	failpointEnv := testutils.GetFailpointEnvVar(
		"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb/importCDCExecEventError=50*off->return(true)",
	)

	logTest(t, "Running import with CDC event execution failpoint (expected to crash)...")
	importWithFailpoint := testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--max-retries-streaming", "1",
		"--yes",
	}, nil, true).WithEnv(
		failpointEnv,
		fmt.Sprintf("YB_VOYAGER_FAILPOINT_MARKER_DIR=%s", filepath.Join(exportDir, "logs")),
		"NUM_EVENT_CHANNELS=1",
		"MAX_EVENTS_PER_BATCH=10",
		"MAX_INTERVAL_BETWEEN_BATCHES=1",
		"EVENT_CHANNEL_SIZE=20",
	)
	err = importWithFailpoint.Run()
	require.NoError(t, err, "Failed to start import with failpoint")

	failMarkerPath := filepath.Join(exportDir, "logs", "failpoint-import-cdc-exec-event-error.log")
	logTestf(t, "Waiting for failpoint marker: %s", failMarkerPath)
	matched, err := waitForMarkerFileImportTest(failMarkerPath, 60*time.Second, 2*time.Second)
	require.NoError(t, err, "Should be able to read CDC exec-event failure marker")
	if !matched {
		_ = importWithFailpoint.Kill()
		require.Fail(t, "CDC exec-event failure marker did not trigger (failpoints may not be enabled). "+
			"Make sure to run `failpoint-ctl enable` before `go test -tags=failpoint` and use a failpoint-enabled yb-voyager binary.")
	}

	_, waitErr := waitForProcessExitOrKillImportTest(importWithFailpoint, 60*time.Second)
	require.Error(t, waitErr, "Import should exit with error after CDC event execution failpoint")
	require.Contains(t, importWithFailpoint.Stderr(), "failpoint", "Expected failpoint mention in import stderr")
	_ = os.Remove(filepath.Join(exportDir, ".import-dataLockfile.lck"))

	maxQueuedVsn, err := maxVsnInQueueSegmentsImportTest(exportDir)
	require.NoError(t, err, "Failed to compute max queued vsn from queue segments")
	logTestf(t, "Max queued vsn in CDC segments: %d", maxQueuedVsn)

	ybConnForChecks, err := yugabytedbContainer.GetConnection()
	require.NoError(t, err, "Failed to get YugabyteDB connection for mid-test checks")
	defer ybConnForChecks.Close()

	lastAppliedVsn, err := maxLastAppliedVsnImportTest(ybConnForChecks, migrationUUIDStr)
	require.NoError(t, err, "Failed to read last_applied_vsn from event channels metadata")
	logTestf(t, "Max last_applied_vsn on target after failure: %d", lastAppliedVsn)
	require.Greater(t, lastAppliedVsn, int64(0), "Expected some CDC progress before failure (last_applied_vsn > 0)")
	require.Less(t, lastAppliedVsn, maxQueuedVsn, "Expected partial progress (last_applied_vsn < max queued vsn)")

	// After failure, target must not match source yet.
	pgConnForMismatch, err := postgresContainer.GetConnection()
	require.NoError(t, err, "Failed to get PostgreSQL connection for mismatch check")
	defer pgConnForMismatch.Close()
	require.Error(t, testutils.CompareTableData(ctx, pgConnForMismatch, ybConnForChecks, "test_schema_import_cdc_event_fail.cdc_import_test", "id"),
		"Expected source != target after CDC failure (resume should be required)")

	logTest(t, "Resuming import without failpoint and waiting for target to match source...")
	importResume := testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--yes",
	}, nil, true).WithEnv(
		"NUM_EVENT_CHANNELS=1",
		"MAX_EVENTS_PER_BATCH=10",
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
		return testutils.CompareTableData(ctx, pgConn, ybConn, "test_schema_import_cdc_event_fail.cdc_import_test", "id") == nil
	}, 240*time.Second, 5*time.Second, "Timed out waiting for target to match source after resume")

	logTest(t, "✓ Target matches source after resume (CDC event execution failure)")

	// best-effort shutdown
	_ = importResume.Kill()
	_ = os.Remove(filepath.Join(exportDir, ".import-dataLockfile.lck"))
}

// TestImportCDCRetryableDbErrorThenSucceed verifies that live migration `import data` retries and
// continues successfully when a transient (retryable) target DB error occurs during CDC apply.
//
// Scenario:
// 1. Run `export data --export-type snapshot-and-changes` and wait for streaming mode.
// 2. Generate CDC inserts and wait until queue segments persist the events.
// 3. Stop export to freeze the queue.
// 4. Run `import data` with a failpoint that injects a retryable SQLSTATE error at the start of
//    `TargetYugabyteDB.ExecuteBatch()` for one attempt.
// 5. Verify the failpoint marker is written and the import keeps running (i.e. it retried instead of exiting).
// 6. Verify target eventually matches source without needing a separate resume run.
//
// Notes on determinism/speed:
// - We cap retry sleeps using YB_VOYAGER_MAX_SLEEP_SECOND=0 so the retry is immediate.
// - We set NUM_EVENT_CHANNELS=1 and MAX_EVENTS_PER_BATCH=10 to keep ordering stable.
func TestImportCDCRetryableDbErrorThenSucceed(t *testing.T) {
	ctx := context.Background()

	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)
	logTestf(t, "Using exportDir=%s", exportDir)

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
		"DROP SCHEMA IF EXISTS test_schema_import_cdc_retry CASCADE;",
		"CREATE SCHEMA test_schema_import_cdc_retry;",
		`CREATE TABLE test_schema_import_cdc_retry.cdc_import_test (
			id INTEGER PRIMARY KEY,
			name TEXT
		);`,
		`ALTER TABLE test_schema_import_cdc_retry.cdc_import_test REPLICA IDENTITY FULL;`,
		`INSERT INTO test_schema_import_cdc_retry.cdc_import_test (id, name)
		 SELECT i, 'snapshot_' || i FROM generate_series(1, 30) i;`,
	)
	defer postgresContainer.ExecuteSqls("DROP SCHEMA IF EXISTS test_schema_import_cdc_retry CASCADE;")

	yugabytedbContainer.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema_import_cdc_retry CASCADE;",
		"CREATE SCHEMA test_schema_import_cdc_retry;",
		`CREATE TABLE test_schema_import_cdc_retry.cdc_import_test (
			id INTEGER PRIMARY KEY,
			name TEXT
		);`,
	)
	defer yugabytedbContainer.ExecuteSqls("DROP SCHEMA IF EXISTS test_schema_import_cdc_retry CASCADE;")

	cdcQueued := make(chan bool, 1)
	generateCDC := func() {
		logTest(t, "Waiting for export to enter streaming mode...")
		require.NoError(t, waitForStreamingModeImportTest(exportDir, 120*time.Second, 2*time.Second), "Export should enter streaming mode")
		logTest(t, "Export reached streaming mode; generating CDC inserts...")

		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema_import_cdc_retry.cdc_import_test (id, name)
			 SELECT 1000 + i, 'cdc_ins_' || i FROM generate_series(1, 120) i;`,
		)

		logTest(t, "Waiting for 120 CDC events to be queued to segment files...")
		waitForCDCEventCountImportTest(t, exportDir, 120, 240*time.Second, 5*time.Second)
		logTest(t, "Verifying no duplicate event_id values in queued CDC...")
		verifyNoEventIDDuplicatesImportTest(t, exportDir)
		logTest(t, "CDC queued and verified")
		cdcQueued <- true
	}

	exportRunner := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_import_cdc_retry",
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

	logTest(t, "Stopping export after CDC has been queued")
	_ = exportRunner.Kill()
	killDebeziumForExportDirImportTest(t, exportDir)
	_ = os.Remove(filepath.Join(exportDir, ".export-dataLockfile.lck"))
	time.Sleep(2 * time.Second)

	failpointEnv := testutils.GetFailpointEnvVar(
		"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb/importCDCRetryableExecuteBatchError=1*return(true)",
	)

	logTest(t, "Running import with retryable CDC error failpoint (should retry and continue)...")
	importRunner := testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--max-retries-streaming", "2",
		"--yes",
	}, nil, true).WithEnv(
		failpointEnv,
		fmt.Sprintf("YB_VOYAGER_FAILPOINT_MARKER_DIR=%s", filepath.Join(exportDir, "logs")),
		"YB_VOYAGER_MAX_SLEEP_SECOND=0",
		"NUM_EVENT_CHANNELS=1",
		"MAX_EVENTS_PER_BATCH=10",
		"MAX_INTERVAL_BETWEEN_BATCHES=1",
		"EVENT_CHANNEL_SIZE=20",
	)
	err = importRunner.Run()
	require.NoError(t, err, "Failed to start import")
	defer importRunner.Kill()

	failMarkerPath := filepath.Join(exportDir, "logs", "failpoint-import-cdc-retryable-exec-batch-error.log")
	logTestf(t, "Waiting for failpoint marker: %s", failMarkerPath)
	matched, err := waitForMarkerFileImportTest(failMarkerPath, 60*time.Second, 2*time.Second)
	require.NoError(t, err, "Should be able to read retryable batch failure marker")
	require.True(t, matched, "Retryable batch failpoint marker did not trigger")
	logTest(t, "✓ Verified retryable batch failpoint marker was written")

	// The key property: importer should NOT exit; it should retry and keep running.
	exitedEarly := make(chan error, 1)
	go func() {
		exitedEarly <- importRunner.Wait()
	}()
	select {
	case err := <-exitedEarly:
		require.Failf(t, "Import exited unexpectedly after retryable failure", "err=%v\nstderr=%s", err, importRunner.Stderr())
	case <-time.After(3 * time.Second):
		// still running as expected
	}

	pgConn, err := postgresContainer.GetConnection()
	require.NoError(t, err, "Failed to get PostgreSQL connection")
	defer pgConn.Close()

	ybConn, err := yugabytedbContainer.GetConnection()
	require.NoError(t, err, "Failed to get YugabyteDB connection")
	defer ybConn.Close()

	require.Eventually(t, func() bool {
		return testutils.CompareTableData(ctx, pgConn, ybConn, "test_schema_import_cdc_retry.cdc_import_test", "id") == nil
	}, 240*time.Second, 5*time.Second, "Timed out waiting for target to match source after retry")

	logTest(t, "✓ Target matches source after in-process retry (no resume run needed)")

	// best-effort shutdown
	_ = importRunner.Kill()
	_ = os.Remove(filepath.Join(exportDir, ".import-dataLockfile.lck"))
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
// 1. Export snapshot-and-changes and queue a small CDC workload, then stop export to freeze the queue.
// 2. Run `import data` with a failpoint in `tgtdb.TargetYugabyteDB.ExecuteBatch()` that:
//    - commits the CDC batch normally (including voyager metadata updates)
//    - then returns a retryable SQLSTATE error
// 3. Verify:
//    - the failpoint marker is written
//    - importer does not exit (it retries in-process)
//    - `last_applied_vsn` advances to max queued vsn (proving the retry short-circuited correctly)
//    - final target matches source without a resume run
func TestImportCDCRetryableAfterCommitErrorSkipsRetry(t *testing.T) {
	ctx := context.Background()

	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)
	logTestf(t, "Using exportDir=%s", exportDir)

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
		"DROP SCHEMA IF EXISTS test_schema_import_cdc_retry_after_commit CASCADE;",
		"CREATE SCHEMA test_schema_import_cdc_retry_after_commit;",
		`CREATE TABLE test_schema_import_cdc_retry_after_commit.cdc_import_test (
			id SERIAL PRIMARY KEY,
			name TEXT,
			value INTEGER
		);`,
		`ALTER TABLE test_schema_import_cdc_retry_after_commit.cdc_import_test REPLICA IDENTITY FULL;`,
		`INSERT INTO test_schema_import_cdc_retry_after_commit.cdc_import_test (name, value)
		 SELECT 'snapshot_' || i, i FROM generate_series(1, 30) i;`,
	)
	defer postgresContainer.ExecuteSqls("DROP SCHEMA IF EXISTS test_schema_import_cdc_retry_after_commit CASCADE;")

	yugabytedbContainer.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema_import_cdc_retry_after_commit CASCADE;",
		"CREATE SCHEMA test_schema_import_cdc_retry_after_commit;",
		`CREATE TABLE test_schema_import_cdc_retry_after_commit.cdc_import_test (
			id SERIAL PRIMARY KEY,
			name TEXT,
			value INTEGER
		);`,
	)
	defer yugabytedbContainer.ExecuteSqls("DROP SCHEMA IF EXISTS test_schema_import_cdc_retry_after_commit CASCADE;")

	cdcQueued := make(chan bool, 1)
	generateCDC := func() {
		logTest(t, "Waiting for export to enter streaming mode...")
		require.NoError(t, waitForStreamingModeImportTest(exportDir, 120*time.Second, 2*time.Second), "Export should enter streaming mode")
		logTest(t, "Export reached streaming mode; generating CDC inserts...")

		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema_import_cdc_retry_after_commit.cdc_import_test (name, value)
			 SELECT 'cdc_ins_' || i, 1000 + i FROM generate_series(1, 60) i;`,
		)

		logTest(t, "Waiting for 60 CDC events to be queued to segment files...")
		waitForCDCEventCountImportTest(t, exportDir, 60, 240*time.Second, 5*time.Second)
		logTest(t, "Verifying no duplicate event_id values in queued CDC...")
		verifyNoEventIDDuplicatesImportTest(t, exportDir)
		logTest(t, "CDC queued and verified")
		cdcQueued <- true
	}

	exportRunner := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_import_cdc_retry_after_commit",
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

	logTest(t, "Stopping export after CDC has been queued")
	_ = exportRunner.Kill()
	killDebeziumForExportDirImportTest(t, exportDir)
	_ = os.Remove(filepath.Join(exportDir, ".export-dataLockfile.lck"))
	time.Sleep(2 * time.Second)

	migrationUUIDStr, err := readMigrationUUIDFromExportDirImportTest(exportDir)
	require.NoError(t, err, "Failed to read migration UUID from exportDir")
	require.NotEmpty(t, migrationUUIDStr, "migration UUID should not be empty")
	logTestf(t, "Migration UUID: %s", migrationUUIDStr)

	failpointEnv := testutils.GetFailpointEnvVar(
		"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb/importCDCRetryableAfterCommitError=1*return(true)",
	)

	importRunner := testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--yes",
	}, nil, true).WithEnv(
		failpointEnv,
		fmt.Sprintf("YB_VOYAGER_FAILPOINT_MARKER_DIR=%s", filepath.Join(exportDir, "logs")),
		"YB_VOYAGER_MAX_SLEEP_SECOND=0", // immediate retry
		"NUM_EVENT_CHANNELS=1",
		"MAX_EVENTS_PER_BATCH=1",
		"MAX_INTERVAL_BETWEEN_BATCHES=1",
		"EVENT_CHANNEL_SIZE=2",
	)
	err = importRunner.Run()
	require.NoError(t, err, "Failed to start import")
	defer importRunner.Kill()

	failMarkerPath := filepath.Join(exportDir, "logs", "failpoint-import-cdc-retryable-after-commit-error.log")
	logTestf(t, "Waiting for failpoint marker: %s", failMarkerPath)
	matched, err := waitForMarkerFileImportTest(failMarkerPath, 60*time.Second, 2*time.Second)
	require.NoError(t, err, "Should be able to read retryable-after-commit marker")
	require.True(t, matched, "Retryable-after-commit failpoint marker did not trigger")
	logTest(t, "✓ Verified retryable-after-commit failpoint marker was written")

	// Import should NOT exit; it should retry in-process and continue.
	exitedEarly := make(chan error, 1)
	go func() {
		exitedEarly <- importRunner.Wait()
	}()
	select {
	case err := <-exitedEarly:
		require.Failf(t, "Import exited unexpectedly after retryable-after-commit failure", "err=%v\nstderr=%s", err, importRunner.Stderr())
	case <-time.After(3 * time.Second):
		// still running as expected
	}

	maxQueuedVsn, err := maxVsnInQueueSegmentsImportTest(exportDir)
	require.NoError(t, err, "Failed to compute max queued vsn from queue segments")
	logTestf(t, "Max queued vsn in CDC segments: %d", maxQueuedVsn)

	// Stronger-than-log verification: ensure voyager progress metadata catches up to the queue.
	ybConnForChecks, err := yugabytedbContainer.GetConnection()
	require.NoError(t, err, "Failed to get YugabyteDB connection for progress check")
	defer ybConnForChecks.Close()

	require.Eventually(t, func() bool {
		v, verr := maxLastAppliedVsnImportTest(ybConnForChecks, migrationUUIDStr)
		return verr == nil && v >= maxQueuedVsn
	}, 120*time.Second, 2*time.Second, "Expected last_applied_vsn to reach max queued vsn after retryable-after-commit error")
	logTest(t, "✓ Verified last_applied_vsn caught up after retryable-after-commit error")

	pgConn, err := postgresContainer.GetConnection()
	require.NoError(t, err, "Failed to get PostgreSQL connection")
	defer pgConn.Close()

	ybConn, err := yugabytedbContainer.GetConnection()
	require.NoError(t, err, "Failed to get YugabyteDB connection")
	defer ybConn.Close()

	require.Eventually(t, func() bool {
		return testutils.CompareTableData(ctx, pgConn, ybConn, "test_schema_import_cdc_retry_after_commit.cdc_import_test", "id") == nil
	}, 240*time.Second, 5*time.Second, "Timed out waiting for target to match source after retryable-after-commit error")
	logTest(t, "✓ Target matches source after in-process retryable-after-commit handling")

	// best-effort shutdown
	_ = importRunner.Kill()
	_ = os.Remove(filepath.Join(exportDir, ".import-dataLockfile.lck"))
}

// TestImportCDCMultiChannelBatchFailureAndResume verifies that CDC import resume works correctly
// when multiple event channels are enabled (NUM_EVENT_CHANNELS>1).
//
// Scenario:
// 1. Export snapshot-and-changes, wait for streaming mode, and queue a CDC workload with enough
//    INSERT + UPDATE + DELETE events to spread across all channels.
// 2. Stop export to freeze the queue.
// 3. Run `import data` with NUM_EVENT_CHANNELS=4 and a failpoint that crashes after N successful
//    CDC batches (expect crash mid-stream).
// 4. After crash, verify:
//    - partial progress (max last_applied_vsn < max queued vsn)
//    - per-channel progress metadata exists for all channels (rows present) and at least some channels advanced
// 5. Resume `import data` and verify:
//    - final target matches source
//    - each channel's last_applied_vsn advanced (>-1), proving multi-channel progress tracking worked
func TestImportCDCMultiChannelBatchFailureAndResume(t *testing.T) {
	ctx := context.Background()

	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)
	logTestf(t, "Using exportDir=%s", exportDir)

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
		logTest(t, "Waiting for export to enter streaming mode...")
		require.NoError(t, waitForStreamingModeImportTest(exportDir, 120*time.Second, 2*time.Second), "Export should enter streaming mode")
		logTest(t, "Export reached streaming mode; generating CDC inserts...")

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
		logTestf(t, "Generating CDC updates: %d rows", len(updateIDs))
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
		logTestf(t, "Generating CDC deletes: %d rows", len(deleteIDs))
		postgresContainer.ExecuteSqls(deleteStmt)

		expected := (numChans * eventsPerChan) + (numChans * updatesPerChan) + (numChans * deletesPerChan)
		logTestf(t, "Waiting for %d CDC events to be queued to segment files...", expected)
		waitForCDCEventCountImportTest(t, exportDir, expected, 240*time.Second, 5*time.Second)
		logTest(t, "Verifying no duplicate event_id values in queued CDC...")
		verifyNoEventIDDuplicatesImportTest(t, exportDir)
		logTest(t, "CDC queued and verified")
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

	logTest(t, "Stopping export after CDC has been queued")
	_ = exportRunner.Kill()
	killDebeziumForExportDirImportTest(t, exportDir)
	_ = os.Remove(filepath.Join(exportDir, ".export-dataLockfile.lck"))
	time.Sleep(2 * time.Second)

	migrationUUIDStr, err := readMigrationUUIDFromExportDirImportTest(exportDir)
	require.NoError(t, err, "Failed to read migration UUID from exportDir")
	require.NotEmpty(t, migrationUUIDStr, "migration UUID should not be empty")
	logTestf(t, "Migration UUID: %s", migrationUUIDStr)

	failpointEnv := testutils.GetFailpointEnvVar(
		// Crash after 100 successful CDC batches across all channels.
		"github.com/yugabyte/yb-voyager/yb-voyager/cmd/importCDCBatchDBError=100*off->return(true)",
	)

	logTest(t, "Running import with multi-channel batch failure failpoint (expected to crash)...")
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
	logTestf(t, "Waiting for failpoint marker: %s", failMarkerPath)
	matched, err := waitForMarkerFileImportTest(failMarkerPath, 60*time.Second, 2*time.Second)
	require.NoError(t, err, "Should be able to read multi-channel batch failure marker")
	require.True(t, matched, "Multi-channel batch failure marker did not trigger")
	logTest(t, "✓ Verified multi-channel batch failure failpoint marker was written")

	logTest(t, "Failpoint marker detected; waiting for import process to exit with error...")
	_, waitErr := waitForProcessExitOrKillImportTest(importWithFailpoint, 60*time.Second)
	require.Error(t, waitErr, "Import should exit with error after multi-channel batch failure failpoint")
	_ = os.Remove(filepath.Join(exportDir, ".import-dataLockfile.lck"))

	maxQueuedVsn, err := maxVsnInQueueSegmentsImportTest(exportDir)
	require.NoError(t, err, "Failed to compute max queued vsn from queue segments")
	logTestf(t, "Max queued vsn in CDC segments: %d", maxQueuedVsn)

	ybConnForChecks, err := yugabytedbContainer.GetConnection()
	require.NoError(t, err, "Failed to get YugabyteDB connection for mid-test checks")
	defer ybConnForChecks.Close()

	lastAppliedVsn, err := maxLastAppliedVsnImportTest(ybConnForChecks, migrationUUIDStr)
	require.NoError(t, err, "Failed to read last_applied_vsn from event channels metadata")
	logTestf(t, "Max last_applied_vsn on target after failure: %d", lastAppliedVsn)
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

	logTest(t, "Resuming import without failpoint and waiting for target to match source...")
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
	logTest(t, "✓ Target matches source after resume (multi-channel)")

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
		logTestf(t, "WARNING: failed to read Debezium PID from exportDir: %v", err)
		return
	}

	pid, err := strconv.Atoi(strings.TrimSpace(pidStr))
	if err != nil {
		logTestf(t, "WARNING: failed to parse Debezium PID %q: %v", pidStr, err)
		return
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		logTestf(t, "WARNING: failed to find Debezium process pid=%d: %v", pid, err)
		return
	}
	if err := proc.Kill(); err != nil {
		logTestf(t, "WARNING: failed to kill Debezium process pid=%d: %v", pid, err)
		return
	}
	logTestf(t, "Killed Debezium process pid=%d", pid)
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

// queuedCDCEventImportTest is a minimal representation of the JSON lines in queue segments.
// We intentionally avoid using `tgtdb.Event` here because its JSON unmarshal path depends on
// global name registry initialization in the current process.
type queuedCDCEventImportTest struct {
	Vsn        int64              `json:"vsn"`
	Op         string             `json:"op"`
	SchemaName string             `json:"schema_name"`
	TableName  string             `json:"table_name"`
	Key        map[string]*string `json:"key"`
	Fields     map[string]*string `json:"fields"`
}

// TableFQN returns a schema-qualified table name ("schema.table") for the queued CDC event.
func (e *queuedCDCEventImportTest) TableFQN() string {
	if e == nil {
		return ""
	}
	if e.SchemaName == "" {
		return e.TableName
	}
	return e.SchemaName + "." + e.TableName
}

// IntID returns the integer `id` from either `fields.id` or `key.id`, depending on event type.
func (e *queuedCDCEventImportTest) IntID() (int, error) {
	if e == nil {
		return 0, fmt.Errorf("nil event")
	}
	if e.Fields != nil {
		if v := e.Fields["id"]; v != nil {
			return strconv.Atoi(*v)
		}
	}
	if e.Key != nil {
		if v := e.Key["id"]; v != nil {
			return strconv.Atoi(*v)
		}
	}
	return 0, fmt.Errorf("id not found in event fields/key")
}

// eventByVsnInQueueSegmentsImportTest finds and returns the queued CDC event with the given VSN
// by scanning all queue segment files.
func eventByVsnInQueueSegmentsImportTest(exportDir string, vsn int64) (*queuedCDCEventImportTest, error) {
	queueDir := filepath.Join(exportDir, "data", "queue")
	entries, err := os.ReadDir(queueDir)
	if err != nil {
		return nil, err
	}

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
			return nil, err
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || line == `\.` {
				continue
			}
			// Do NOT unmarshal into tgtdb.Event here: its UnmarshalJSON calls namereg, which depends
			// on global source-db type initialization that isn't guaranteed in these black-box tests.
			var ev queuedCDCEventImportTest
			if err := json.Unmarshal([]byte(line), &ev); err != nil {
				f.Close()
				return nil, err
			}
			if ev.Vsn == vsn {
				f.Close()
				return &ev, nil
			}
		}
		if err := scanner.Err(); err != nil {
			f.Close()
			return nil, err
		}
		f.Close()
	}
	return nil, fmt.Errorf("event with vsn=%d not found in queue segments", vsn)
}

// waitForMarkerFileImportTest waits for a failpoint marker file to contain "hit".
// Returns (true, nil) if the marker is observed within the timeout.
func waitForMarkerFileImportTest(path string, timeout, pollInterval time.Duration) (bool, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(data), "hit") {
			return true, nil
		}
		time.Sleep(pollInterval)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return strings.Contains(string(data), "hit"), nil
}

// waitForProcessExitOrKillImportTest waits for the command runner to exit.
// If it doesn't exit within the timeout, it SIGKILLs it and returns (true, nil).
func waitForProcessExitOrKillImportTest(runner *testutils.VoyagerCommandRunner, timeout time.Duration) (bool, error) {
	errCh := make(chan error, 1)
	go func() {
		errCh <- runner.Wait()
	}()

	select {
	case err := <-errCh:
		return false, err
	case <-time.After(timeout):
		_ = runner.Kill()
		return true, nil
	}
}

// maxVsnInQueueSegmentsImportTest returns the maximum VSN present in queue segments.
func maxVsnInQueueSegmentsImportTest(exportDir string) (int64, error) {
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

// readMigrationUUIDFromExportDirImportTest reads the migration UUID from exportDir's metadb.
func readMigrationUUIDFromExportDirImportTest(exportDir string) (string, error) {
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

// maxLastAppliedVsnImportTest returns the maximum `last_applied_vsn` across all importer channels
// for the given migration UUID, as recorded on the target in voyager metadata.
func maxLastAppliedVsnImportTest(ybConn interface {
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

