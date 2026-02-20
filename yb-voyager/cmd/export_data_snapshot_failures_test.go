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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// TestSnapshotFailureAndResume verifies recovery when pg_dump snapshot fails mid-export.
//
// Scenario:
// 1. Start CDC export (snapshot-and-changes mode) with 100 initial rows
// 2. Inject pgDumpSnapshotFailure failpoint with 8s delay before pg_dump starts
// 3. Insert 20 CDC rows during the delay window (before failure triggers)
// 4. pg_dump fails; export crashes with no snapshot descriptor created
// 5. Resume export without failure injection
// 6. Verify snapshot completes and includes both initial (100) and during-snapshot (20) rows
// 7. Insert 10 more CDC rows after snapshot and verify CDC export works correctly
// 8. Verify total: 120 snapshot rows + 10 CDC events = 130 total rows
//
// This test validates:
// - Snapshot failure recovery restarts pg_dump from scratch
// - CDC rows inserted during failed snapshot attempt are captured in the new snapshot
// - CDC export resumes correctly after snapshot completes
func TestSnapshotFailureAndResume(t *testing.T) {
	if os.Getenv("BYTEMAN_JAR") == "" {
		t.Skip("Skipping test: BYTEMAN_JAR environment variable not set. Install Byteman to run this test.")
	}

	ctx := context.Background()

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:    ContainerConfig{Type: "postgresql", ForLive: true},
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
		CleanupSQL: []string{"DROP SCHEMA IF EXISTS test_schema_snapshot_fail CASCADE;"},
	})
	defer lm.Cleanup()
	require.NoError(t, lm.SetupContainers(ctx))
	require.NoError(t, lm.SetupSchema())

	exportDir = lm.GetExportDir()
	postgresContainer := lm.GetSourceContainer()

	failpointEnv := testutils.GetFailpointEnvVar(
		"github.com/yugabyte/yb-voyager/yb-voyager/cmd/pgDumpSnapshotFailure=1*return()",
	)
	cdcEventsGenerated := make(chan bool, 1)
	generateCDCEvents := func() {
		// Insert CDC rows while snapshot is in progress (pg_dump running)
		time.Sleep(3 * time.Second)
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema_snapshot_fail.cdc_snapshot_fail_test (name, value)
			SELECT 'cdc_' || i, 1000 + i FROM generate_series(1, 20) i;`,
		)
		testutils.LogTest(t, "CDC rows inserted during snapshot phase")
		cdcEventsGenerated <- true
	}

	exportRunner := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_snapshot_fail",
		"--disable-pb", "true",
		"--yes",
	}, generateCDCEvents, true).WithEnv(failpointEnv, "YB_VOYAGER_PGDUMP_FAIL_DELAY_MS=8000")

	err := exportRunner.Run()
	require.NoError(t, err, "Failed to start export")

	err = exportRunner.Wait()
	require.Error(t, err, "Export should fail due to pg_dump snapshot failpoint")

	failMarkerPath := filepath.Join(exportDir, "logs", "failpoint-pg-dump-snapshot.log")
	matched, err := waitForMarkerFile(failMarkerPath, 60*time.Second, 2*time.Second)
	require.NoError(t, err, "Should be able to read snapshot failure marker")
	require.True(t, matched, "Snapshot failure marker did not trigger")

	descriptorPath := filepath.Join(exportDir, datafile.DESCRIPTOR_PATH)
	_, err = os.Stat(descriptorPath)
	require.Error(t, err, "Snapshot descriptor should not exist after failed snapshot")

	testutils.LogTest(t, "Verifying CDC events inserted before failure are accounted for...")
	eventCountAfterFailure, err := countEventsInQueueSegments(exportDir)
	if err != nil {
		if os.IsNotExist(err) {
			testutils.LogTest(t, "Queue directory missing after failure; no CDC events persisted yet")
			eventCountAfterFailure = 0
		} else {
			require.NoError(t, err, "Should be able to count CDC events after failure")
		}
	}
	if eventCountAfterFailure > 0 {
		testutils.LogTestf(t, "CDC events captured before failure: %d", eventCountAfterFailure)
	}

	testutils.LogTest(t, "Resuming export without failure injection...")
	exportRunnerResume := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_snapshot_fail",
		"--disable-pb", "true",
		"--yes",
	}, nil, true)

	err = exportRunnerResume.Run()
	require.NoError(t, err, "Failed to start resumed export")
	defer exportRunnerResume.Kill()

	select {
	case <-cdcEventsGenerated:
	case <-time.After(30 * time.Second):
		require.Fail(t, "CDC event generation timed out")
	}

	descriptorHash, err := waitForSnapshotDescriptorHashSnapshotFailure(exportDir, 120*time.Second, 2*time.Second)
	require.NoError(t, err, "Snapshot descriptor should be created after resume")
	require.NotEmpty(t, descriptorHash, "Snapshot descriptor hash should not be empty")

	testutils.LogTest(t, "Inserting CDC rows after snapshot completion to validate CDC export...")
	postgresContainer.ExecuteSqls(
		`INSERT INTO test_schema_snapshot_fail.cdc_snapshot_fail_test (name, value)
		SELECT 'cdc_after_' || i, 2000 + i FROM generate_series(1, 10) i;`,
	)

	snapshotRowCount, err := getSnapshotRowCountForTable(exportDir, "cdc_snapshot_fail_test")
	require.NoError(t, err, "Failed to get snapshot row count from descriptor")
	require.Equal(t, int64(120), snapshotRowCount, "Snapshot should include initial + during-snapshot rows")

	cdcEventCount := waitForCDCEventCount(t, exportDir, 10, 60*time.Second, 2*time.Second)

	pgConn, err := postgresContainer.GetConnection()
	require.NoError(t, err, "Failed to get PostgreSQL connection")
	defer pgConn.Close()

	var sourceRowCount int
	err = pgConn.QueryRow("SELECT COUNT(*) FROM test_schema_snapshot_fail.cdc_snapshot_fail_test").Scan(&sourceRowCount)
	require.NoError(t, err, "Failed to query source row count")
	require.Equal(t, 130, sourceRowCount, "Source should have snapshot + CDC rows after resume")
	require.Equal(t, sourceRowCount, int(snapshotRowCount)+cdcEventCount,
		"Snapshot rows + CDC events should equal source row count")

	testutils.LogTest(t, "Snapshot failure and resume test completed successfully")
}

// TestSnapshotToCDCTransitionFailure verifies recovery when snapshot-to-CDC transition fails.
//
// Scenario:
// 1. Start CDC export (snapshot-and-changes mode) with 30 snapshot rows
// 2. Snapshot phase completes successfully and creates descriptor
// 3. Inject snapshotToCDCTransitionError failpoint at transition point (after snapshot, before CDC starts)
// 4. Export crashes after snapshot is complete but before CDC starts
// 5. Verify no CDC events were emitted before failure
// 6. Resume export without failure injection
// 7. Insert 20 CDC rows after resume and verify CDC export works correctly
// 8. Verify final state: 30 snapshot rows + 20 CDC events (snapshot not re-run)
//
// This test validates:
// - Snapshot-to-CDC transition failure does not corrupt snapshot data
// - Resume correctly starts CDC from where transition left off
// - Snapshot is not re-executed on resume (descriptor hash remains stable)
func TestSnapshotToCDCTransitionFailure(t *testing.T) {
	if os.Getenv("BYTEMAN_JAR") == "" {
		t.Skip("Skipping test: BYTEMAN_JAR environment variable not set. Install Byteman to run this test.")
	}

	ctx := context.Background()

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:    ContainerConfig{Type: "postgresql", ForLive: true},
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
		CleanupSQL: []string{"DROP SCHEMA IF EXISTS test_schema_transition CASCADE;"},
	})
	defer lm.Cleanup()
	require.NoError(t, lm.SetupContainers(ctx))
	require.NoError(t, lm.SetupSchema())

	exportDir = lm.GetExportDir()
	postgresContainer := lm.GetSourceContainer()

	testutils.LogTest(t, "Running export with Go-side failure injection at snapshot->CDC transition...")
	failpointEnv := testutils.GetFailpointEnvVar(
		"github.com/yugabyte/yb-voyager/yb-voyager/cmd/snapshotToCDCTransitionError=1*return()",
	)
	exportRunner := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_transition",
		"--disable-pb", "true",
		"--yes",
	}, nil, true).WithEnv(failpointEnv)

	err := exportRunner.Run()
	require.NoError(t, err, "Failed to start export")

	failMarkerPath := filepath.Join(exportDir, "logs", "failpoint-snapshot-to-cdc.log")
	testutils.LogTest(t, "Waiting for failpoint marker after injection...")
	matched, err := waitForMarkerFile(failMarkerPath, 60*time.Second, 2*time.Second)
	require.NoError(t, err, "Should be able to read failpoint marker")
	if !matched {
		_ = exportRunner.Kill()
		_ = killDebeziumForExportDir(exportDir)
		require.Fail(t, "Snapshot->CDC failpoint marker did not trigger")
	}
	testutils.LogTest(t, "✓ Failpoint marker detected: snapshot->CDC transition failure injected")

	testutils.LogTest(t, "Waiting for export process to exit after injection...")
	wasKilled, waitErr := waitForProcessExitOrKill(exportRunner, exportDir, 60*time.Second)
	if wasKilled {
		testutils.LogTest(t, "Export did not exit after injection; process was killed")
	} else {
		require.Error(t, waitErr, "Export should exit with error after failpoint injection")
	}

	time.Sleep(3 * time.Second)

	testutils.LogTest(t, "Verifying no CDC events were emitted before failure...")
	eventCountAfterFailure, err := countEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count CDC events after failure")
	require.Equal(t, 0, eventCountAfterFailure, "Expected 0 CDC events before resume")

	testutils.LogTest(t, "Capturing snapshot descriptor before resume...")
	descriptorHashBefore, err := hashSnapshotDescriptor(exportDir)
	require.NoError(t, err, "Should be able to hash snapshot descriptor before resume")

	testutils.LogTest(t, "Resuming export without failure injection...")
	cdcEventsGenerated := make(chan bool, 1)
	generateCDCEvents := func() {
		if err := waitForStreamingMode(exportDir, 2*time.Minute, 2*time.Second); err != nil {
			testutils.LogTestf(t, "Failed waiting for streaming mode before CDC inserts: %v", err)
			cdcEventsGenerated <- true
			return
		}
		testutils.LogTestf(t, "Generating CDC events (1 batch of 20 rows, %v between batches)...", batchSeparationWaitTime)
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema_transition.cdc_transition_test (name, value)
			SELECT 'batch1_' || i, 100 + i FROM generate_series(1, 20) i;`,
		)
		testutils.LogTestf(t, "Batch 1 inserted, waiting %v for Debezium to process...", batchSeparationWaitTime)
		time.Sleep(batchSeparationWaitTime)
		cdcEventsGenerated <- true
	}

	exportRunnerResume := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_transition",
		"--disable-pb", "true",
		"--yes",
	}, generateCDCEvents, true)

	err = exportRunnerResume.Run()
	require.NoError(t, err, "Failed to start resumed export")
	defer exportRunnerResume.Kill()

	select {
	case <-cdcEventsGenerated:
		testutils.LogTest(t, "CDC events generation completed")
	case <-time.After(60 * time.Second):
		require.Fail(t, "CDC event generation timed out")
	}

	finalEventCount := waitForCDCEventCount(t, exportDir, 20, 60*time.Second, 2*time.Second)
	require.Equal(t, 20, finalEventCount, "Expected 20 CDC events after recovery")

	verifyNoEventIDDuplicates(t, exportDir)

	testutils.LogTest(t, "Verifying snapshot descriptor unchanged after resume...")
	descriptorHashAfter, err := hashSnapshotDescriptor(exportDir)
	require.NoError(t, err, "Should be able to hash snapshot descriptor after resume")
	require.Equal(t, descriptorHashBefore, descriptorHashAfter, "Snapshot descriptor should not change after resume")

	pgConn, err := postgresContainer.GetConnection()
	require.NoError(t, err, "Failed to get PostgreSQL connection")
	defer pgConn.Close()

	var sourceRowCount int
	err = pgConn.QueryRow("SELECT COUNT(*) FROM test_schema_transition.cdc_transition_test").Scan(&sourceRowCount)
	require.NoError(t, err, "Failed to query source row count")
	require.Equal(t, 50, sourceRowCount, "Source should have 30 snapshot + 20 CDC rows")

	testutils.LogTest(t, "Snapshot->CDC transition failure test completed successfully")
}
