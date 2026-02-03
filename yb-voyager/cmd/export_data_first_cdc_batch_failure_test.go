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
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// TestFirstCDCBatchFailure tests "cold start" durability when the very first CDC batch fails.
//
// Reference: FAILURE_INJECTION_TEST_PLAN.md - Test 1.2
//
// Scenario:
// 1. Start CDC export (snapshot-and-changes mode)
// 2. Complete snapshot phase (50 rows)
// 3. Generate 3 batches of CDC events (20 rows each, 60 total)
// 4. Inject failure on 1st CDC batch (before processing)
// 5. Process crashes - 0 CDC events written
// 6. Resume export
// 7. Verify all 60 CDC events recovered (full replay from beginning)
// 8. Verify no duplicate events
//
// Validates:
// - "Cold start" recovery (no CDC offsets exist yet)
// - Recovery from zero CDC offset state
// - Full CDC replay capability
//
// Difference from Test 1.1:
// - Test 1.1: Fails on 2nd batch (20 events committed, 40 to replay)
// - Test 1.2: Fails on 1st batch (0 events committed, 60 to replay)
//
// Uses same batching strategy as Test 1.1:
// - Debezium defaults: 500ms poll interval, 2048 max batch size
// - 20 rows per INSERT (well under max batch size)
// - 2.5s wait between INSERTs (5x poll interval)
// - Encourages separate Debezium batches
func TestFirstCDCBatchFailure(t *testing.T) {
	// Skip if Byteman is not available
	if os.Getenv("BYTEMAN_JAR") == "" {
		t.Skip("Skipping test: BYTEMAN_JAR environment variable not set. Install Byteman to run this test.")
	}

	ctx := context.Background()

	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	// Setup PostgreSQL container
	postgresContainer := testcontainers.NewTestContainer("postgresql", &testcontainers.ContainerConfig{
		ForLive: true,
	})
	err := postgresContainer.Start(ctx)
	require.NoError(t, err, "Failed to start PostgreSQL container")
	defer postgresContainer.Stop(ctx)

	// Setup test schema and initial data
	setupFirstBatchTestData(t, postgresContainer)
	defer postgresContainer.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema CASCADE;",
	)

	bytemanHelper, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err, "Failed to create Byteman helper")

	// Inject failure on the FIRST CDC batch
	// This is different from Test 1.1 which fails on the 2nd CDC batch
	// This tests the scenario where NO CDC events have been committed yet when failure occurs
	bytemanHelper.AddRuleFromBuilder(
		testutils.NewRule("fail_first_cdc_batch").
			AtMarker(testutils.MarkerCDC, "before-batch").
			If("incrementCounter(\"first_batch_counter\") == 1").
			ThrowException("java.lang.RuntimeException", "TEST: Simulated failure on first CDC batch"),
	)

	err = bytemanHelper.WriteRules()
	require.NoError(t, err, "Failed to write Byteman rules")

	t.Log("Phase 1: Running CDC export with failure injection on 1st batch...")

	// Batching configuration (same as Test 1.1)
	const (
		debeziumDefaultPollIntervalMs = 500                                    // Debezium's default poll interval
		batchSeparationWaitTime       = time.Duration(2500 * time.Millisecond) // 2.5s = 5x poll interval
	)

	// Generate CDC events in background
	cdcEventsGenerated := make(chan bool, 1)
	generateCDCEvents := func() {
		time.Sleep(10 * time.Second) // Wait for snapshot to complete
		t.Logf("Generating CDC events (3 batches of 20 rows each, waiting %v between batches)...", batchSeparationWaitTime)

		// NOTE: Batching behavior relies on Debezium's internal logic:
		// - Each INSERT is a separate transaction (20 rows each)
		// - 2.5s wait (5x poll interval) encourages separate Debezium batches
		// - Byteman counter triggers on the 1st CDC batch
		// - Test validates recovery when NO CDC offsets exist yet

		// Batch 1: Should FAIL due to injection (counter == 1)
		t.Log("Inserting batch 1 (20 rows) - Byteman should fail this batch...")
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema.first_batch_test (name, value)
			SELECT 'batch1_' || i, 100 + i FROM generate_series(1, 20) i;`,
		)
		t.Logf("Batch 1 inserted, waiting %v...", batchSeparationWaitTime)
		time.Sleep(batchSeparationWaitTime)

		// Batch 2: Will be processed after recovery
		t.Log("Inserting batch 2 (20 rows) - will be processed after recovery...")
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema.first_batch_test (name, value)
			SELECT 'batch2_' || i, 200 + i FROM generate_series(1, 20) i;`,
		)
		t.Logf("Batch 2 inserted, waiting %v...", batchSeparationWaitTime)
		time.Sleep(batchSeparationWaitTime)

		// Batch 3: Will be processed after recovery
		t.Log("Inserting batch 3 (20 rows) - will be processed after recovery...")
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema.first_batch_test (name, value)
			SELECT 'batch3_' || i, 300 + i FROM generate_series(1, 20) i;`,
		)
		t.Log("Batch 3 inserted")

		cdcEventsGenerated <- true
		t.Log("Finished generating CDC events")
	}

	// Run export with Byteman injection - should fail on 1st CDC batch
	exportRunner := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema",
		"--disable-pb", "true",
		"--yes",
	}, generateCDCEvents, true).WithEnv(bytemanHelper.GetEnv()...)

	err = exportRunner.Run()
	require.NoError(t, err, "Failed to start export")

	// Wait for the failure to be injected
	t.Log("Waiting for Byteman injection to occur...")
	matched, err := bytemanHelper.WaitForInjection(">>> BYTEMAN: fail_first_cdc_batch", 90*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs")
	require.True(t, matched, "Byteman injection should have occurred and been logged")
	t.Log("✓ Byteman injection detected - batch 1 processing failed as expected")

	// Wait a bit to ensure all CDC events are generated
	select {
	case <-cdcEventsGenerated:
		t.Log("CDC events generation completed")
	case <-time.After(60 * time.Second):
		t.Log("Warning: CDC event generation timed out")
	}

	// Wait for the export process to crash naturally
	// The RuntimeException from Byteman should propagate to Debezium and cause it to exit
	t.Log("Waiting for export process to crash naturally after Byteman injection...")
	err = exportRunner.Wait()
	if err != nil {
		t.Logf("✓ Export process crashed as expected: %v", err)
	} else {
		t.Log("Warning: Export process exited cleanly (expected an error)")
	}

	time.Sleep(3 * time.Second) // Additional wait for cleanup

	// Verify 0 CDC events written before failure (first batch failed at entry)
	// Note: Queue segments only contain CDC events, not snapshot data
	t.Log("Counting CDC events in queue segments after failed export...")
	eventCount1, err := countEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count events after first export")
	t.Logf("CDC events in queue after failed export: %d (expected: 0)", eventCount1)

	// We expect 0 events because the first batch failed at entry (before-batch marker)
	// - Batch 1: Failed at entry, no events written
	// - Batches 2 & 3: Not yet processed (will be replayed on resume)
	// This validates "cold start" recovery: full replay from zero CDC state
	require.Equal(t, 0, eventCount1, "Should have 0 events (first batch failed at entry, no CDC state yet)")

	t.Log("================================================================================")
	t.Log("Phase 2: Resuming CDC export WITHOUT failure injection...")
	t.Log("================================================================================")
	t.Log("Expected behavior:")
	t.Log("  - Debezium resumes from beginning (no CDC offsets committed)")
	t.Log("  - Will replay ALL 3 batches (batch 1 + batch 2 + batch 3) = 60 events")
	t.Log("  - Total expected: 60 CDC events (20 + 20 + 20)")
	t.Log("  - Note: Snapshot data (50 rows) is in separate data files, not queue segments")
	t.Log("  - This validates 'cold start' durability (recovery from zero CDC state)")

	// Resume export WITHOUT Byteman (no failure injection)
	exportRunnerResume := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema",
		"--disable-pb", "true",
		"--yes",
	}, nil, true) // No concurrent event generation, no Byteman injection

	err = exportRunnerResume.Run()
	require.NoError(t, err, "Failed to start resumed export")
	defer exportRunnerResume.Kill()

	// Wait for resumed export to process all remaining CDC events
	t.Log("Waiting for resumed export to process all remaining CDC events...")
	time.Sleep(5 * time.Second) // Initial wait for startup

	// Poll until we have all expected events (60 total)
	const expectedFinalEvents = 60
	maxWaitTime := 60 * time.Second
	pollInterval := 2 * time.Second
	startTime := time.Now()

	for time.Since(startTime) < maxWaitTime {
		currentCount, err := countEventsInQueueSegments(exportDir)
		if err == nil {
			t.Logf("Current CDC event count: %d / %d expected", currentCount, expectedFinalEvents)
			if currentCount >= expectedFinalEvents {
				t.Logf("✓ All expected CDC events received: %d", currentCount)
				break
			}
		}
		time.Sleep(pollInterval)
	}

	// Give a bit more time for any in-flight processing
	time.Sleep(5 * time.Second)

	t.Log("================================================================================")
	t.Log("Verifying final event counts and data integrity...")
	t.Log("================================================================================")

	// Verify final CDC event count
	finalEventCount, err := countEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count final events")
	t.Logf("✓ Final CDC event count: %d (expected: %d)", finalEventCount, expectedFinalEvents)
	require.Equal(t, expectedFinalEvents, finalEventCount,
		"Should have all 60 CDC events after recovery (3 batches of 20)")

	// Verify source database has correct row count
	pgConn, err := postgresContainer.GetConnection()
	require.NoError(t, err, "Failed to get PostgreSQL connection")
	defer pgConn.Close()

	var sourceRowCount int
	err = pgConn.QueryRow("SELECT COUNT(*) FROM test_schema.first_batch_test").Scan(&sourceRowCount)
	require.NoError(t, err, "Failed to query source row count")
	t.Logf("Source database row count: %d", sourceRowCount)

	// Should have 50 (snapshot) + 60 (CDC) = 110 rows total
	expectedTotalRows := 50 + expectedFinalEvents
	require.Equal(t, expectedTotalRows, sourceRowCount, "Source should have all rows")

	// Verify no duplicate events (all VSNs are unique)
	verifyNoEventDuplicates(t, exportDir)

	t.Log("✓ First CDC batch failure and recovery test completed successfully")
	t.Log("✓ All CDC events were replayed from scratch (no CDC offsets existed)")
	t.Log("✓ Event deduplication prevented duplicates")
	t.Log("✓ All CDC data exported correctly after recovery")
	t.Log("✓ Validates 'cold start' durability when first CDC batch fails")
	t.Logf("✓ Final CDC event count: %d", finalEventCount)
}

// setupFirstBatchTestData creates test schema and initial snapshot data
func setupFirstBatchTestData(t *testing.T, container testcontainers.TestContainer) {
	container.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema CASCADE;",
		"CREATE SCHEMA test_schema;",
		`CREATE TABLE test_schema.first_batch_test (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100),
			value INTEGER
		);`,
		// Enable logical replication
		"ALTER TABLE test_schema.first_batch_test REPLICA IDENTITY FULL;",
	)

	// Insert initial snapshot data (50 rows - same as Test 1.1)
	container.ExecuteSqls(
		`INSERT INTO test_schema.first_batch_test (name, value)
		SELECT 'snapshot_' || i, i FROM generate_series(1, 50) i;`,
	)

	t.Log("Test schema created with 50 snapshot rows")
}
