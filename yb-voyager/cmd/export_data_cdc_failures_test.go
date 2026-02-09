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

const (
	// Debezium PostgreSQL connector timing configuration
	//
	// Verification: yb-voyager/src/dbzm/config.go lines 127-139 (postgresSrcConfigTemplate)
	// - PostgreSQL config does NOT explicitly set: poll.interval.ms, max.batch.size, max.queue.size
	// - Only sets: offset.flush.interval.ms=0 (immediate flush, line 97)
	// - Therefore uses Debezium defaults: 500ms poll interval, 2048 max batch, 8192 queue
	//
	// Test batching strategy:
	// - Insert 20 rows per batch (well under 2048 limit)
	// - Wait 5x poll interval (2.5s) between INSERTs
	// - This gives Debezium time to: poll → process → commit offset → start next batch
	//
	debeziumDefaultPollIntervalMs = 500                                                               // Default poll interval
	batchSeparationWaitTime       = time.Duration(debeziumDefaultPollIntervalMs*5) * time.Millisecond // 2.5 seconds
	multiFailurePollIntervalMs    = 500
	multiFailureBatchWait         = time.Duration(multiFailurePollIntervalMs*5) * time.Millisecond // 2.5s
)

// TestCDCBatchFailureAndResume injects at before-batch-streaming (2nd batch) and verifies resume/dedup.
//
// Scenario:
// 1. Start CDC export (snapshot-and-changes mode)
// 2. Complete snapshot phase (100 rows)
// 3. Generate 3 CDC batches (20 rows each)
// 4. Inject failure on 2nd CDC batch
// 5. Resume export
// 6. Verify all 60 CDC events recovered with no duplicates
// 1. Start CDC export (snapshot-and-changes) with failure injection on 2nd CDC batch
// 2. Verify the failure occurs and export crashes (after snapshot completes)
// 3. Resume export WITHOUT failure injection
// 4. Verify all CDC events are eventually received and processed correctly
// 5. Validate no duplicate events (event deduplication works)
//
// Debezium Configuration:
// - PostgreSQL connector uses Debezium defaults (500ms poll, 2048 max batch)
// - Only explicit config: offset.flush.interval.ms=0 (immediate offset commits)
// - Test waits 2.5s (5x poll interval) between INSERTs to encourage separate batches
//
// Batching Behavior:
// - Each INSERT creates 20 rows in a single transaction
// - 2.5s wait between INSERTs allows Debezium to: poll → process → write → commit offset
// - Byteman counter triggers on 2nd CDC batch (whenever it occurs)
// - Test validates recovery and durability, not exact batch boundaries
//
// Note: This test verifies CDC event durability. Snapshot data is in separate files,
// while CDC events are in queue segments. We focus on CDC event recovery and deduplication.
func TestCDCBatchFailureAndResume(t *testing.T) {
	// Skip if Byteman is not available
	if os.Getenv("BYTEMAN_JAR") == "" {
		t.Skip("Skipping test: BYTEMAN_JAR environment variable not set. Install Byteman to run this test.")
	}

	ctx := context.Background()

	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	postgresContainer := testcontainers.NewTestContainer("postgresql", &testcontainers.ContainerConfig{
		ForLive: true,
	})
	err := postgresContainer.Start(ctx)
	require.NoError(t, err, "Failed to start PostgreSQL container")
	defer postgresContainer.Stop(ctx)

	// Setup test schema and initial data
	setupCDCTestDataForResume(t, postgresContainer)
	defer postgresContainer.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema CASCADE;",
	)

	bytemanHelper, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err, "Failed to create Byteman helper")

	// Target marker for batch processing - fails on 2nd CDC batch (after snapshot)
	// Use streaming-only marker to avoid snapshot batch ambiguity
	bytemanHelper.AddRuleFromBuilder(
		testutils.NewRule("fail_cdc_batch_2").
			AtMarker(testutils.MarkerCDC, "before-batch-streaming").
			If("incrementCounter(\"cdc_batch\") == 2").
			ThrowException("java.lang.RuntimeException", "TEST: Simulated batch processing failure on batch 2"),
	)

	err = bytemanHelper.WriteRules()
	require.NoError(t, err, "Failed to write Byteman rules")

	logTest(t, "Running CDC export with failure injection on 2nd CDC batch (streaming-only)...")

	// Generate CDC events in background
	cdcEventsGenerated := make(chan bool, 1)
	generateCDCEvents := func() {
		time.Sleep(10 * time.Second) // Wait for snapshot to complete
		logTestf(t, "Generating CDC events (3 batches of 20 rows, %v between batches)...", batchSeparationWaitTime)

		// Batch 1: Should succeed
		logTest(t, "Inserting batch 1 (20 rows)...")
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema.cdc_test (name, value)
			SELECT 'batch1_' || i, 100 + i FROM generate_series(1, 20) i;`,
		)
		logTestf(t, "Batch 1 inserted, waiting %v for Debezium to process...", batchSeparationWaitTime)
		time.Sleep(batchSeparationWaitTime)

		// Best-effort check: batch 1 should result in 20 CDC events.
		batch1Count, err := countEventsInQueueSegments(exportDir)
		if err == nil {
			logTestf(t, "Events in queue after batch 1: %d", batch1Count)
			if batch1Count != 20 {
				logTestf(t, "Warning: expected 20 events after batch 1, got %d (may still be processing)", batch1Count)
			}
		} else {
			logTestf(t, "Could not count events after batch 1 (queue may not exist yet): %v", err)
		}

		// Batch 2: Should FAIL due to injection
		logTest(t, "Inserting batch 2 (20 rows) - expected to fail via Byteman...")
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema.cdc_test (name, value)
			SELECT 'batch2_' || i, 200 + i FROM generate_series(1, 20) i;`,
		)
		logTestf(t, "Batch 2 inserted, waiting %v...", batchSeparationWaitTime)
		time.Sleep(batchSeparationWaitTime)

		// Batch 3: Will be processed after recovery
		logTest(t, "Inserting batch 3 (20 rows) - processed after recovery...")
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema.cdc_test (name, value)
			SELECT 'batch3_' || i, 300 + i FROM generate_series(1, 20) i;`,
		)
		logTest(t, "Batch 3 inserted")

		cdcEventsGenerated <- true
	}

	// Run export with Byteman injection - should fail on 2nd CDC batch
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
	logTest(t, "Waiting for Byteman injection...")
	matched, err := bytemanHelper.WaitForInjection(">>> BYTEMAN: fail_cdc_batch_2", 90*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs")
	require.True(t, matched, "Byteman injection should have occurred and been logged")
	logTest(t, "Byteman injection detected")

	// Wait a bit to ensure all CDC events are generated
	select {
	case <-cdcEventsGenerated:
		logTest(t, "CDC events generation completed")
	case <-time.After(60 * time.Second):
		require.Fail(t, "CDC event generation timed out")
	}

	// Wait for the export process to crash (Byteman exception should abort Debezium)
	logTest(t, "Waiting for export process to exit after Byteman injection...")
	err = exportRunner.Wait()
	require.Error(t, err, "Export should exit with error after Byteman injection")

	time.Sleep(3 * time.Second) // Additional wait for cleanup

	// Verify that some CDC events were written before failure
	logTest(t, "Counting CDC events in queue segments after failed export...")
	eventCountAfterFailure, err := countEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count CDC events after failure")
	logTestf(t, "CDC events in queue after failed export: %d (expected: 20)", eventCountAfterFailure)
	require.Equal(t, 20, eventCountAfterFailure, "Should have exactly 20 events (batch 1) before failure")

	verifyNoEventIDDuplicates(t, exportDir)

	logTest(t, "Resuming CDC export without failure injection...")

	// Resume export WITHOUT Byteman (no failure injection)
	exportRunnerResume := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema",
		"--disable-pb", "true",
		"--yes",
	}, nil, true)

	err = exportRunnerResume.Run()
	require.NoError(t, err, "Failed to start resumed export")
	defer exportRunnerResume.Kill()

	finalEventCount := waitForCDCEventCount(t, exportDir, 60, 120*time.Second, 5*time.Second)
	require.Equal(t, 60, finalEventCount, "Expected 60 CDC events after resume")

	verifyNoEventIDDuplicates(t, exportDir)

	logTest(t, "CDC batch failure and resume test completed successfully")
}

// TestFirstCDCBatchFailure injects at before-batch-streaming (1st batch) and verifies full replay.
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
	// Use streaming-only marker to avoid snapshot batch ambiguity
	bytemanHelper.AddRuleFromBuilder(
		testutils.NewRule("fail_first_cdc_batch").
			AtMarker(testutils.MarkerCDC, "before-batch-streaming").
			If("incrementCounter(\"first_batch_counter\") == 1").
			ThrowException("java.lang.RuntimeException", "TEST: Simulated failure on first CDC batch"),
	)

	err = bytemanHelper.WriteRules()
	require.NoError(t, err, "Failed to write Byteman rules")

	logTest(t, "Running CDC export with failure injection on 1st CDC batch (streaming-only)...")

	// Generate CDC events in background
	cdcEventsGenerated := make(chan bool, 1)
	generateCDCEvents := func() {
		time.Sleep(10 * time.Second) // Wait for snapshot to complete
		logTestf(t, "Generating CDC events (3 batches of 20 rows, %v between batches)...", batchSeparationWaitTime)

		// Batch 1: Should FAIL due to injection (counter == 1)
		logTest(t, "Inserting batch 1 (20 rows) - expected to fail via Byteman...")
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema.first_batch_test (name, value)
			SELECT 'batch1_' || i, 100 + i FROM generate_series(1, 20) i;`,
		)
		logTestf(t, "Batch 1 inserted, waiting %v...", batchSeparationWaitTime)
		time.Sleep(batchSeparationWaitTime)

		// Batch 2: Will be processed after recovery
		logTest(t, "Inserting batch 2 (20 rows) - processed after recovery...")
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema.first_batch_test (name, value)
			SELECT 'batch2_' || i, 200 + i FROM generate_series(1, 20) i;`,
		)
		logTestf(t, "Batch 2 inserted, waiting %v...", batchSeparationWaitTime)
		time.Sleep(batchSeparationWaitTime)

		// Batch 3: Will be processed after recovery
		logTest(t, "Inserting batch 3 (20 rows) - processed after recovery...")
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema.first_batch_test (name, value)
			SELECT 'batch3_' || i, 300 + i FROM generate_series(1, 20) i;`,
		)
		logTest(t, "Batch 3 inserted")

		cdcEventsGenerated <- true
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
	logTest(t, "Waiting for Byteman injection...")
	matched, err := bytemanHelper.WaitForInjection(">>> BYTEMAN: fail_first_cdc_batch", 90*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs")
	require.True(t, matched, "Byteman injection should have occurred and been logged")
	logTest(t, "Byteman injection detected")

	// Wait a bit to ensure all CDC events are generated
	select {
	case <-cdcEventsGenerated:
		logTest(t, "CDC events generation completed")
	case <-time.After(60 * time.Second):
		require.Fail(t, "CDC event generation timed out")
	}

	// Wait for the export process to crash (Byteman exception should abort Debezium)
	logTest(t, "Waiting for export process to exit after Byteman injection...")
	err = exportRunner.Wait()
	require.Error(t, err, "Export should exit with error after Byteman injection")

	time.Sleep(3 * time.Second) // Additional wait for cleanup

	// Verify 0 CDC events written before failure (first batch failed at entry)
	logTest(t, "Counting CDC events in queue segments after failed export...")
	eventCount1, err := countEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count events after first export")
	logTestf(t, "CDC events in queue after failed export: %d (expected: 0)", eventCount1)
	require.Equal(t, 0, eventCount1, "Should have 0 events (first batch failed at entry, no CDC state yet)")

	logTest(t, "Resuming CDC export without failure injection...")

	// Resume export WITHOUT Byteman (no failure injection)
	exportRunnerResume := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema",
		"--disable-pb", "true",
		"--yes",
	}, nil, true)

	err = exportRunnerResume.Run()
	require.NoError(t, err, "Failed to start resumed export")
	defer exportRunnerResume.Kill()

	finalEventCount := waitForCDCEventCount(t, exportDir, 60, 120*time.Second, 5*time.Second)
	require.Equal(t, 60, finalEventCount, "Expected 60 CDC events after resume")

	verifyNoEventIDDuplicates(t, exportDir)

	logTest(t, "First CDC batch failure test completed successfully")
}

// TestCDCMultipleBatchFailures injects at before-batch-streaming on consecutive runs and verifies recovery.
//
// Scenario:
// 1. Run 1: fail on 2nd batch → expect 20 events
// 2. Run 2: fail on 2nd batch → expect 40 events
// 3. Run 3: no failure → expect 60 events
func TestCDCMultipleBatchFailures(t *testing.T) {
	if os.Getenv("BYTEMAN_JAR") == "" {
		t.Skip("Skipping test: BYTEMAN_JAR environment variable not set. Install Byteman to run this test.")
	}

	ctx := context.Background()

	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	postgresContainer := testcontainers.NewTestContainer("postgresql", &testcontainers.ContainerConfig{
		ForLive: true,
	})
	err := postgresContainer.Start(ctx)
	require.NoError(t, err, "Failed to start PostgreSQL container")
	defer postgresContainer.Stop(ctx)

	setupMultipleFailureTestData(t, postgresContainer)
	defer postgresContainer.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema_multi_fail CASCADE;",
	)

	// Run 1: fail on 2nd streaming batch
	bytemanHelperRun1, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err, "Failed to create Byteman helper (run 1)")
	bytemanHelperRun1.AddRuleFromBuilder(
		testutils.NewRule("fail_cdc_batch_run1").
			AtMarker(testutils.MarkerCDC, "before-batch-streaming").
			If("incrementCounter(\"cdc_batch\") == 2").
			ThrowException("java.lang.RuntimeException", "TEST: Simulated batch failure on run 1"),
	)
	err = bytemanHelperRun1.WriteRules()
	require.NoError(t, err, "Failed to write Byteman rules (run 1)")

	cdcEventsGenerated := make(chan bool, 1)
	generateCDCEvents := func() {
		time.Sleep(10 * time.Second) // Wait for snapshot to complete

		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema_multi_fail.cdc_multi_fail_test (name, value)
			SELECT 'batch1_' || i, 100 + i FROM generate_series(1, 20) i;`,
		)
		time.Sleep(multiFailureBatchWait)

		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema_multi_fail.cdc_multi_fail_test (name, value)
			SELECT 'batch2_' || i, 200 + i FROM generate_series(1, 20) i;`,
		)
		time.Sleep(multiFailureBatchWait)
		cdcEventsGenerated <- true
	}

	exportRunner1 := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_multi_fail",
		"--disable-pb", "true",
		"--yes",
	}, generateCDCEvents, true).WithEnv(bytemanHelperRun1.GetEnv()...)

	err = exportRunner1.Run()
	require.NoError(t, err, "Failed to start export (run 1)")

	matched, err := bytemanHelperRun1.WaitForInjection(">>> BYTEMAN: fail_cdc_batch_run1", 90*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs (run 1)")
	require.True(t, matched, "Byteman injection should have occurred (run 1)")

	select {
	case <-cdcEventsGenerated:
	case <-time.After(60 * time.Second):
		require.Fail(t, "CDC event generation timed out")
	}

	err = exportRunner1.Wait()
	require.Error(t, err, "Export should exit with error after run 1 injection")

	time.Sleep(3 * time.Second)

	eventCountAfterRun1, err := countEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count events after run 1")
	require.Equal(t, 20, eventCountAfterRun1, "Expected 20 CDC events after run 1 (batch 1 only)")
	verifyNoEventIDDuplicates(t, exportDir)
	assertSourceRowCount(t, postgresContainer, 90)

	// Run 2: fail on 2nd streaming batch again (replay batch2 succeeds, batch3 fails)
	bytemanHelperRun2, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err, "Failed to create Byteman helper (run 2)")
	bytemanHelperRun2.AddRuleFromBuilder(
		testutils.NewRule("fail_cdc_batch_run2").
			AtMarker(testutils.MarkerCDC, "before-batch-streaming").
			If("incrementCounter(\"cdc_batch\") == 2").
			ThrowException("java.lang.RuntimeException", "TEST: Simulated batch failure on run 2"),
	)
	err = bytemanHelperRun2.WriteRules()
	require.NoError(t, err, "Failed to write Byteman rules (run 2)")

	cdcEventsGeneratedRun2 := make(chan bool, 1)
	generateCDCEventsRun2 := func() {
		time.Sleep(10 * time.Second) // Wait for Debezium resume to start
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema_multi_fail.cdc_multi_fail_test (name, value)
			SELECT 'batch3_' || i, 300 + i FROM generate_series(1, 20) i;`,
		)
		time.Sleep(multiFailureBatchWait)
		cdcEventsGeneratedRun2 <- true
	}

	exportRunner2 := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_multi_fail",
		"--disable-pb", "true",
		"--yes",
	}, generateCDCEventsRun2, true).WithEnv(bytemanHelperRun2.GetEnv()...)

	err = exportRunner2.Run()
	require.NoError(t, err, "Failed to start export (run 2)")

	matched, err = bytemanHelperRun2.WaitForInjection(">>> BYTEMAN: fail_cdc_batch_run2", 90*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs (run 2)")
	require.True(t, matched, "Byteman injection should have occurred (run 2)")

	select {
	case <-cdcEventsGeneratedRun2:
	case <-time.After(60 * time.Second):
		require.Fail(t, "CDC event generation (run 2) timed out")
	}

	err = exportRunner2.Wait()
	require.Error(t, err, "Export should exit with error after run 2 injection")

	time.Sleep(3 * time.Second)

	eventCountAfterRun2, err := countEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count events after run 2")
	require.Equal(t, 40, eventCountAfterRun2, "Expected 40 CDC events after run 2 (batch 1 + replayed batch 2)")
	verifyNoEventIDDuplicates(t, exportDir)
	assertSourceRowCount(t, postgresContainer, 110)

	// Run 3: no injection, complete remaining CDC
	exportRunner3 := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_multi_fail",
		"--disable-pb", "true",
		"--yes",
	}, nil, true)

	err = exportRunner3.Run()
	require.NoError(t, err, "Failed to start export (run 3)")
	defer exportRunner3.Kill()

	finalEventCount := waitForCDCEventCount(t, exportDir, 60, 120*time.Second, 5*time.Second)
	require.Equal(t, 60, finalEventCount, "Expected 60 CDC events after final resume")

	verifyNoEventIDDuplicates(t, exportDir)
	assertSourceRowCount(t, postgresContainer, 110)

	logTest(t, "CDC multiple batch failures test completed successfully")
}
