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
	"os"
	"path/filepath"
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

	select {
	case <-cdcQueued:
	case <-time.After(180 * time.Second):
		_ = exportRunner.Kill()
		require.Fail(t, "Timed out waiting for CDC events to be queued")
	}

	// Stop export to avoid further queue writes during import assertions.
	logTest(t, "Stopping export after CDC has been queued")
	_ = exportRunner.Kill()
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

