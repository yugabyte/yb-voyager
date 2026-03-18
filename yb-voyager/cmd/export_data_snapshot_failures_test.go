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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// TestSnapshotFailureAndResume verifies that live migration `export data` can resume after
// a pg_dump snapshot failure during the snapshot phase.
//
// Scenario:
//  1. Start `export data` (snapshot-and-changes mode) with failpoint enabled and 100 initial rows.
//  2. pg_dump fails; export crashes with no snapshot descriptor created.
//  3. Insert 20 rows into the source after failure, before resume.
//  4. Resume `export data` without failpoint.
//  5. Verify snapshot completes and captures all 120 rows (100 initial + 20 inserted before resume).
//  6. Insert 10 CDC rows after snapshot and verify CDC export works correctly.
//  7. Verify total: 120 snapshot rows + 10 CDC events = 130 total rows.
//
// This test validates:
// - Snapshot failure recovery restarts pg_dump from scratch
// - Resumed snapshot captures the current database state, not a stale snapshot
// - CDC export resumes correctly after snapshot completes
//
// Injection point:
//   - `cmd/exportData.go` before pg_dump via failpoint `pgDumpSnapshotFailure`.
//   - Delay controlled by `YB_VOYAGER_PGDUMP_FAIL_DELAY_MS` env var.
func TestSnapshotFailureAndResume(t *testing.T) {
	ctx := context.Background()

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test_export_snapshot_fail",
		},
		SchemaNames: []string{"test_schema_snapshot_fail"},
		SchemaSQL: []string{
			"DROP SCHEMA IF EXISTS test_schema_snapshot_fail CASCADE;",
			"CREATE SCHEMA test_schema_snapshot_fail;",
			`CREATE TABLE test_schema_snapshot_fail.cdc_snapshot_fail_test (
				id SERIAL PRIMARY KEY,
				name TEXT,
				value INTEGER,
				created_at TIMESTAMP DEFAULT NOW()
			);`,
			`ALTER TABLE test_schema_snapshot_fail.cdc_snapshot_fail_test REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema_snapshot_fail.cdc_snapshot_fail_test (name, value)
			SELECT 'snapshot_' || i, i * 10 FROM generate_series(1, 100) i;`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema_snapshot_fail.cdc_snapshot_fail_test (name, value)
			SELECT 'cdc_after_' || i, 2000 + i FROM generate_series(1, 10) i;`,
		},
		CleanupSQL: []string{"DROP SCHEMA IF EXISTS test_schema_snapshot_fail CASCADE;"},
	})
	defer lm.Cleanup()
	require.NoError(t, lm.SetupContainers(ctx))
	require.NoError(t, lm.SetupSchema())

	// --- Phase 1: Start export with failpoint, let it crash ---
	failpointEnv := testutils.GetFailpointEnvVar(
		"github.com/yugabyte/yb-voyager/yb-voyager/cmd/pgDumpSnapshotFailure=1*return()",
	)
	err := lm.StartExportDataWithEnv(true, nil, []string{failpointEnv, "YB_VOYAGER_PGDUMP_FAIL_DELAY_MS=8000"})
	require.NoError(t, err, "Failed to start export")

	failMarkerPath := filepath.Join(lm.GetExportDir(), "logs", "failpoint-pg-dump-snapshot.log")
	err = lm.WaitForExportFailpointAndProcessCrash(t, failMarkerPath, 60*time.Second, 60*time.Second)
	require.NoError(t, err, "Export should crash due to pg_dump snapshot failpoint")

	// --- Phase 2: Verify state after failure, insert rows before resume ---
	descriptorPath := filepath.Join(lm.GetExportDir(), datafile.DESCRIPTOR_PATH)
	_, err = os.Stat(descriptorPath)
	require.Error(t, err, "Snapshot descriptor should not exist after failed snapshot")

	lm.ExecuteOnSource(
		`INSERT INTO test_schema_snapshot_fail.cdc_snapshot_fail_test (name, value)
		SELECT 'between_failure_' || i, 500 + i FROM generate_series(1, 20) i;`,
	)

	// --- Phase 3: Resume export without failpoint ---
	err = lm.StartExportData(true, nil)
	require.NoError(t, err, "Failed to start resumed export")
	defer lm.StopExportData()

	require.NoError(t, lm.WaitForStreamingMode(2*time.Minute, 2*time.Second),
		"Export should reach streaming mode after resume")

	snapshotRowCount, err := getSnapshotRowCountForTable(lm.GetExportDir(), "cdc_snapshot_fail_test")
	require.NoError(t, err, "Failed to get snapshot row count from descriptor")
	require.Equal(t, int64(120), snapshotRowCount, "Snapshot should include initial rows + rows inserted before resume")

	lm.ExecuteSourceDelta()

	cdcEventCount := lm.WaitForCDCEventCount(t, 10, 60*time.Second, 2*time.Second)

	verifyNoEventIDDuplicates(t, lm.GetExportDir())

	var sourceRowCount int
	err = lm.WithSourceConn(func(conn *sql.DB) error {
		return conn.QueryRow("SELECT COUNT(*) FROM test_schema_snapshot_fail.cdc_snapshot_fail_test").Scan(&sourceRowCount)
	})
	require.NoError(t, err, "Failed to query source row count")
	require.Equal(t, 130, sourceRowCount, "Source should have snapshot + CDC rows after resume")
	require.Equal(t, sourceRowCount, int(snapshotRowCount)+cdcEventCount,
		"Snapshot rows + CDC events should equal source row count")
}

// TestSnapshotToCDCTransitionFailure verifies that live migration `export data` can resume after
// a failure at the snapshot-to-CDC transition point.
//
// Scenario:
//  1. Start `export data` (snapshot-and-changes mode) with failpoint enabled and 30 snapshot rows.
//  2. Snapshot phase completes successfully and creates descriptor.
//  3. Failpoint fires at the transition point (after snapshot, before CDC starts), crashing export.
//  4. Verify no CDC events were emitted before failure.
//  5. Resume `export data` without failpoint.
//  6. Insert 20 CDC rows after resume and verify CDC export works correctly.
//  7. Verify final state: 30 snapshot rows + 20 CDC events (snapshot not re-run).
//
// This test validates:
// - Snapshot-to-CDC transition failure does not corrupt snapshot data
// - Resume correctly starts CDC from where transition left off
// - Snapshot is not re-executed on resume (descriptor hash remains stable)
//
// Injection point:
//   - `cmd/exportDataDebezium.go` after snapshot, before CDC start:
//     failpoint `snapshotToCDCTransitionError`.
func TestSnapshotToCDCTransitionFailure(t *testing.T) {
	ctx := context.Background()

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test_export_snapshot_transition",
		},
		SchemaNames: []string{"test_schema_transition"},
		SchemaSQL: []string{
			"DROP SCHEMA IF EXISTS test_schema_transition CASCADE;",
			"CREATE SCHEMA test_schema_transition;",
			`CREATE TABLE test_schema_transition.cdc_transition_test (
				id SERIAL PRIMARY KEY,
				name TEXT,
				value INTEGER,
				created_at TIMESTAMP DEFAULT NOW()
			);`,
			`ALTER TABLE test_schema_transition.cdc_transition_test REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema_transition.cdc_transition_test (name, value)
			SELECT 'snapshot_' || i, i * 10 FROM generate_series(1, 30) i;`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema_transition.cdc_transition_test (name, value)
			SELECT 'batch1_' || i, 100 + i FROM generate_series(1, 20) i;`,
		},
		CleanupSQL: []string{"DROP SCHEMA IF EXISTS test_schema_transition CASCADE;"},
	})
	defer lm.Cleanup()
	require.NoError(t, lm.SetupContainers(ctx))
	require.NoError(t, lm.SetupSchema())

	// --- Phase 1: Start export with failpoint at snapshot->CDC transition ---
	failpointEnv := testutils.GetFailpointEnvVar(
		"github.com/yugabyte/yb-voyager/yb-voyager/cmd/snapshotToCDCTransitionError=1*return()",
	)
	err := lm.StartExportDataWithEnv(true, nil, []string{failpointEnv})
	require.NoError(t, err, "Failed to start export")

	failMarkerPath := filepath.Join(lm.GetExportDir(), "logs", "failpoint-snapshot-to-cdc.log")
	err = lm.WaitForExportFailpointAndProcessCrash(t, failMarkerPath, 60*time.Second, 60*time.Second)
	require.NoError(t, err, "Export should crash after snapshot->CDC transition failpoint")

	// --- Phase 2: Verify state after failure ---
	time.Sleep(3 * time.Second)

	eventCountAfterFailure, err := countEventsInQueueSegments(lm.GetExportDir())
	require.NoError(t, err, "Should be able to count CDC events after failure")
	require.Equal(t, 0, eventCountAfterFailure, "Expected 0 CDC events before resume")

	descriptorHashBefore, err := hashSnapshotDescriptor(lm.GetExportDir())
	require.NoError(t, err, "Should be able to hash snapshot descriptor before resume")

	// --- Phase 3: Resume export without failpoint, generate CDC events ---
	err = lm.StartExportData(true, nil)
	require.NoError(t, err, "Failed to start resumed export")
	defer lm.StopExportData()

	require.NoError(t, lm.WaitForStreamingMode(2*time.Minute, 2*time.Second),
		"Export should reach streaming mode after resume")

	lm.ExecuteSourceDelta()

	finalEventCount := lm.WaitForCDCEventCount(t, 20, 60*time.Second, 2*time.Second)
	require.Equal(t, 20, finalEventCount, "Expected 20 CDC events after recovery")

	verifyNoEventIDDuplicates(t, lm.GetExportDir())

	descriptorHashAfter, err := hashSnapshotDescriptor(lm.GetExportDir())
	require.NoError(t, err, "Should be able to hash snapshot descriptor after resume")
	require.Equal(t, descriptorHashBefore, descriptorHashAfter, "Snapshot descriptor should not change after resume")

	var sourceRowCount int
	err = lm.WithSourceConn(func(conn *sql.DB) error {
		return conn.QueryRow("SELECT COUNT(*) FROM test_schema_transition.cdc_transition_test").Scan(&sourceRowCount)
	})
	require.NoError(t, err, "Failed to query source row count")
	require.Equal(t, 50, sourceRowCount, "Source should have 30 snapshot + 20 CDC rows")
}
