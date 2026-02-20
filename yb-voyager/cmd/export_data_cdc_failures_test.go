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
)

// TestCDCBatchFailureAndResume verifies mid-batch failure recovery and deduplication.
//
// Scenario:
// 1. Start CDC export (snapshot-and-changes mode) with 100 snapshot rows
// 2. Generate 3 CDC batches (20 rows each, 2.5s wait between batches)
// 3. Inject failure on 2nd CDC batch via before-batch-streaming marker
// 4. Export crashes after batch 1 committed; batch 2 and 3 are lost
// 5. Resume export without failure injection
// 6. Verify all 60 CDC events recovered with no duplicates via event_id dedup
//
// This test validates:
// - Batch processing failure recovery
// - CDC offset replay (batch 2 and 3 replayed from offsets)
// - Event deduplication (batch 1 events already written, not duplicated on resume)
func TestCDCBatchFailureAndResume(t *testing.T) {
	if os.Getenv("BYTEMAN_JAR") == "" {
		t.Skip("Skipping test: BYTEMAN_JAR environment variable not set. Install Byteman to run this test.")
	}

	ctx := context.Background()

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:    ContainerConfig{Type: "postgresql", ForLive: true},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			"CREATE SCHEMA IF NOT EXISTS test_schema;",
			`CREATE TABLE test_schema.cdc_test (
				id SERIAL PRIMARY KEY,
				name TEXT,
				value INTEGER,
				created_at TIMESTAMP DEFAULT NOW()
			);`,
			`ALTER TABLE test_schema.cdc_test REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.cdc_test (name, value)
			SELECT 'initial_' || i, i * 10 FROM generate_series(1, 100) i;`,
		},
		CleanupSQL: []string{"DROP SCHEMA IF EXISTS test_schema CASCADE;"},
	})
	defer lm.Cleanup()
	require.NoError(t, lm.SetupContainers(ctx))
	require.NoError(t, lm.SetupSchema())

	exportDir = lm.GetExportDir()
	postgresContainer := lm.GetSourceContainer()

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

	testutils.LogTest(t, "Running CDC export with failure injection on 2nd CDC batch (streaming-only)...")

	// Generate CDC events in background
	cdcEventsGenerated := make(chan bool, 1)
	generateCDCEvents := func() {
		time.Sleep(10 * time.Second) // Wait for snapshot to complete
		testutils.LogTestf(t, "Generating CDC events (3 batches of 20 rows, %v between batches)...", batchSeparationWaitTime)

		// Batch 1: Should succeed
		testutils.LogTest(t, "Inserting batch 1 (20 rows)...")
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema.cdc_test (name, value)
			SELECT 'batch1_' || i, 100 + i FROM generate_series(1, 20) i;`,
		)
		testutils.LogTestf(t, "Batch 1 inserted, waiting %v for Debezium to process...", batchSeparationWaitTime)
		time.Sleep(batchSeparationWaitTime)

		// Best-effort check: batch 1 should result in 20 CDC events.
		batch1Count, err := countEventsInQueueSegments(exportDir)
		if err == nil {
			testutils.LogTestf(t, "Events in queue after batch 1: %d", batch1Count)
			if batch1Count != 20 {
				testutils.LogTestf(t, "Warning: expected 20 events after batch 1, got %d (may still be processing)", batch1Count)
			}
		} else {
			testutils.LogTestf(t, "Could not count events after batch 1 (queue may not exist yet): %v", err)
		}

		// Batch 2: Should FAIL due to injection
		testutils.LogTest(t, "Inserting batch 2 (20 rows) - expected to fail via Byteman...")
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema.cdc_test (name, value)
			SELECT 'batch2_' || i, 200 + i FROM generate_series(1, 20) i;`,
		)
		testutils.LogTestf(t, "Batch 2 inserted, waiting %v...", batchSeparationWaitTime)
		time.Sleep(batchSeparationWaitTime)

		// Batch 3: Will be processed after recovery
		testutils.LogTest(t, "Inserting batch 3 (20 rows) - processed after recovery...")
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema.cdc_test (name, value)
			SELECT 'batch3_' || i, 300 + i FROM generate_series(1, 20) i;`,
		)
		testutils.LogTest(t, "Batch 3 inserted")

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
	testutils.LogTest(t, "Waiting for Byteman injection...")
	matched, err := bytemanHelper.WaitForInjection(">>> BYTEMAN: fail_cdc_batch_2", 90*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs")
	require.True(t, matched, "Byteman injection should have occurred and been logged")
	testutils.LogTest(t, "Byteman injection detected")

	// Wait a bit to ensure all CDC events are generated
	select {
	case <-cdcEventsGenerated:
		testutils.LogTest(t, "CDC events generation completed")
	case <-time.After(60 * time.Second):
		require.Fail(t, "CDC event generation timed out")
	}

	// Wait for the export process to crash (Byteman exception should abort Debezium)
	testutils.LogTest(t, "Waiting for export process to exit after Byteman injection...")
	err = exportRunner.Wait()
	require.Error(t, err, "Export should exit with error after Byteman injection")

	time.Sleep(3 * time.Second) // Additional wait for cleanup

	// Verify that some CDC events were written before failure
	testutils.LogTest(t, "Counting CDC events in queue segments after failed export...")
	eventCountAfterFailure, err := countEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count CDC events after failure")
	testutils.LogTestf(t, "CDC events in queue after failed export: %d (expected: 20)", eventCountAfterFailure)
	require.Equal(t, 20, eventCountAfterFailure, "Should have exactly 20 events (batch 1) before failure")

	verifyNoEventIDDuplicates(t, exportDir)

	testutils.LogTest(t, "Resuming CDC export without failure injection...")

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

	testutils.LogTest(t, "CDC batch failure and resume test completed successfully")
}

// TestFirstCDCBatchFailure verifies recovery when the first CDC batch fails before any offsets are committed.
//
// Scenario:
// 1. Start CDC export (snapshot-and-changes mode) with 50 snapshot rows
// 2. Generate 3 CDC batches (20 rows each, 60 total events)
// 3. Inject failure on 1st CDC batch via before-batch-streaming marker
// 4. Export crashes before any CDC offsets are committed (0 events written)
// 5. Resume export without failure injection
// 6. Verify all 60 CDC events recovered via full replay from zero offset state
//
// This test validates:
// - "Cold start" CDC recovery (no offsets file exists)
// - Full CDC replay capability from the beginning

func TestFirstCDCBatchFailure(t *testing.T) {
	if os.Getenv("BYTEMAN_JAR") == "" {
		t.Skip("Skipping test: BYTEMAN_JAR environment variable not set. Install Byteman to run this test.")
	}

	ctx := context.Background()

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:    ContainerConfig{Type: "postgresql", ForLive: true},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			"CREATE SCHEMA IF NOT EXISTS test_schema;",
			`CREATE TABLE test_schema.first_batch_test (
				id SERIAL PRIMARY KEY,
				name TEXT,
				value INTEGER,
				created_at TIMESTAMP DEFAULT NOW()
			);`,
			`ALTER TABLE test_schema.first_batch_test REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.first_batch_test (name, value)
			SELECT 'initial_' || i, i * 10 FROM generate_series(1, 50) i;`,
		},
		CleanupSQL: []string{"DROP SCHEMA IF EXISTS test_schema CASCADE;"},
	})
	defer lm.Cleanup()
	require.NoError(t, lm.SetupContainers(ctx))
	require.NoError(t, lm.SetupSchema())

	exportDir = lm.GetExportDir()
	postgresContainer := lm.GetSourceContainer()

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

	testutils.LogTest(t, "Running CDC export with failure injection on 1st CDC batch (streaming-only)...")

	// Generate CDC events in background
	cdcEventsGenerated := make(chan bool, 1)
	generateCDCEvents := func() {
		time.Sleep(10 * time.Second) // Wait for snapshot to complete
		testutils.LogTestf(t, "Generating CDC events (3 batches of 20 rows, %v between batches)...", batchSeparationWaitTime)

		// Batch 1: Should FAIL due to injection (counter == 1)
		testutils.LogTest(t, "Inserting batch 1 (20 rows) - expected to fail via Byteman...")
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema.first_batch_test (name, value)
			SELECT 'batch1_' || i, 100 + i FROM generate_series(1, 20) i;`,
		)
		testutils.LogTestf(t, "Batch 1 inserted, waiting %v...", batchSeparationWaitTime)
		time.Sleep(batchSeparationWaitTime)

		// Batch 2: Will be processed after recovery
		testutils.LogTest(t, "Inserting batch 2 (20 rows) - processed after recovery...")
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema.first_batch_test (name, value)
			SELECT 'batch2_' || i, 200 + i FROM generate_series(1, 20) i;`,
		)
		testutils.LogTestf(t, "Batch 2 inserted, waiting %v...", batchSeparationWaitTime)
		time.Sleep(batchSeparationWaitTime)

		// Batch 3: Will be processed after recovery
		testutils.LogTest(t, "Inserting batch 3 (20 rows) - processed after recovery...")
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema.first_batch_test (name, value)
			SELECT 'batch3_' || i, 300 + i FROM generate_series(1, 20) i;`,
		)
		testutils.LogTest(t, "Batch 3 inserted")

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
	testutils.LogTest(t, "Waiting for Byteman injection...")
	matched, err := bytemanHelper.WaitForInjection(">>> BYTEMAN: fail_first_cdc_batch", 90*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs")
	require.True(t, matched, "Byteman injection should have occurred and been logged")
	testutils.LogTest(t, "Byteman injection detected")

	// Wait a bit to ensure all CDC events are generated
	select {
	case <-cdcEventsGenerated:
		testutils.LogTest(t, "CDC events generation completed")
	case <-time.After(60 * time.Second):
		require.Fail(t, "CDC event generation timed out")
	}

	// Wait for the export process to crash (Byteman exception should abort Debezium)
	testutils.LogTest(t, "Waiting for export process to exit after Byteman injection...")
	err = exportRunner.Wait()
	require.Error(t, err, "Export should exit with error after Byteman injection")

	time.Sleep(3 * time.Second) // Additional wait for cleanup

	// Verify 0 CDC events written before failure (first batch failed at entry)
	testutils.LogTest(t, "Counting CDC events in queue segments after failed export...")
	eventCount1, err := countEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count events after first export")
	testutils.LogTestf(t, "CDC events in queue after failed export: %d (expected: 0)", eventCount1)
	require.Equal(t, 0, eventCount1, "Should have 0 events (first batch failed at entry, no CDC state yet)")

	testutils.LogTest(t, "Resuming CDC export without failure injection...")

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

	testutils.LogTest(t, "First CDC batch failure test completed successfully")
}

// TestCDCMultipleBatchFailures verifies resilience across multiple consecutive batch failures.
//
// Scenario:
// 1. Start CDC export (snapshot-and-changes mode) with 50 snapshot rows
// 2. Run 1: Insert batch1 (20 rows), batch2 (20 rows) → fail on 2nd batch → 20 events written
// 3. Run 2 (resume): Insert batch3 (20 rows) → fail on 2nd batch again → 40 events total
// 4. Run 3 (resume): No failure → all batches replay → 60 events total with no duplicates
//
// This test validates:
// - Recovery across multiple consecutive failures
// - Incremental progress after each failed run
// - Final full recovery with deduplication
func TestCDCMultipleBatchFailures(t *testing.T) {
	if os.Getenv("BYTEMAN_JAR") == "" {
		t.Skip("Skipping test: BYTEMAN_JAR environment variable not set. Install Byteman to run this test.")
	}

	ctx := context.Background()

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:    ContainerConfig{Type: "postgresql", ForLive: true},
		SchemaNames: []string{"test_schema_multi_fail"},
		SchemaSQL: []string{
			"DROP SCHEMA IF EXISTS test_schema_multi_fail CASCADE;",
			"CREATE SCHEMA test_schema_multi_fail;",
			`CREATE TABLE test_schema_multi_fail.cdc_multi_fail_test (
				id SERIAL PRIMARY KEY,
				name TEXT,
				value INTEGER,
				created_at TIMESTAMP DEFAULT NOW()
			);`,
			`ALTER TABLE test_schema_multi_fail.cdc_multi_fail_test REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema_multi_fail.cdc_multi_fail_test (name, value)
			SELECT 'snapshot_' || i, i * 10 FROM generate_series(1, 50) i;`,
		},
		CleanupSQL: []string{"DROP SCHEMA IF EXISTS test_schema_multi_fail CASCADE;"},
	})
	defer lm.Cleanup()
	require.NoError(t, lm.SetupContainers(ctx))
	require.NoError(t, lm.SetupSchema())

	exportDir = lm.GetExportDir()
	postgresContainer := lm.GetSourceContainer()

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
		time.Sleep(batchSeparationWaitTime)

		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema_multi_fail.cdc_multi_fail_test (name, value)
			SELECT 'batch2_' || i, 200 + i FROM generate_series(1, 20) i;`,
		)
		time.Sleep(batchSeparationWaitTime)
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
		time.Sleep(batchSeparationWaitTime)
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

	testutils.LogTest(t, "CDC multiple batch failures test completed successfully")
}
