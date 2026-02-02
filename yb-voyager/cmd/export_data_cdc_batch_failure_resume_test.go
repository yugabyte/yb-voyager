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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
)

// TestCDCBatchFailureAndResume tests the complete failure injection and recovery flow:
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
	// Note: Snapshot batches will complete first, then CDC batches start
	bytemanHelper.AddRuleFromBuilder(
		testutils.NewRule("fail_cdc_batch_2").
			AtMarker(testutils.MarkerCDC, "before-batch").
			If("incrementCounter(\"cdc_batch\") == 2").
			ThrowException("java.lang.RuntimeException", "TEST: Simulated batch processing failure on batch 2"),
	)

	err = bytemanHelper.WriteRules()
	require.NoError(t, err, "Failed to write Byteman rules")

	t.Log("Phase 1: Running CDC export with failure injection on 2nd batch...")

	// Generate CDC events in background
	cdcEventsGenerated := make(chan bool, 1)
	generateCDCEvents := func() {
		time.Sleep(10 * time.Second) // Wait for snapshot to complete
		t.Logf("Generating CDC events (3 batches of 20 rows each, waiting %v between batches)...", batchSeparationWaitTime)

		// NOTE: Batching behavior relies on Debezium's internal logic:
		// - Each INSERT is a separate transaction (20 rows each)
		// - 2.5s wait (5x poll interval) encourages separate Debezium batches
		// - Byteman counter triggers on the 2nd CDC batch
		// - Test validates recovery and durability, not exact batch boundaries

		// Batch 1: Should succeed
		t.Log("Inserting batch 1 (20 rows)...")
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema.cdc_test (name, value)
			SELECT 'batch1_' || i, 100 + i FROM generate_series(1, 20) i;`,
		)
		t.Logf("Batch 1 inserted, waiting %v for Debezium to process...", batchSeparationWaitTime)
		time.Sleep(batchSeparationWaitTime)

		// Verify batch 1 was processed as a single batch with exactly 20 events
		// Since we:
		// - Inserted 20 rows (well under max batch size of 2048)
		// - Waited 5x poll interval (2.5s) before next insert
		// - Debezium flushes offsets immediately (offset.flush.interval.ms=0)
		// We expect exactly 20 CDC events in the queue
		batch1Count, err := countEventsInQueueSegments(exportDir)
		if err == nil {
			t.Logf("✓ Events in queue after batch 1: %d", batch1Count)
			assert.Equal(t, 20, batch1Count, "Batch 1 should contain exactly 20 events (validates batching behavior)")
		} else {
			t.Logf("Could not count events after batch 1 (queue may not exist yet): %v", err)
		}

		// Batch 2: Should FAIL due to injection
		t.Log("Inserting batch 2 (20 rows) - Byteman should fail this batch...")
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema.cdc_test (name, value)
			SELECT 'batch2_' || i, 200 + i FROM generate_series(1, 20) i;`,
		)
		t.Logf("Batch 2 inserted, waiting %v...", batchSeparationWaitTime)
		time.Sleep(batchSeparationWaitTime)

		// Batch 3: Will be processed after recovery
		t.Log("Inserting batch 3 (20 rows) - will be processed after recovery...")
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema.cdc_test (name, value)
			SELECT 'batch3_' || i, 300 + i FROM generate_series(1, 20) i;`,
		)
		t.Log("Batch 3 inserted")

		cdcEventsGenerated <- true
		t.Log("Finished generating CDC events")
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
	t.Log("Waiting for Byteman injection to occur...")
	matched, err := bytemanHelper.WaitForInjection(">>> BYTEMAN: fail_cdc_batch_2", 90*time.Second)
	assert.NoError(t, err, "Should be able to read debezium logs")
	assert.True(t, matched, "Byteman injection should have occurred and been logged")
	t.Log("✓ Byteman injection detected - batch 2 processing failed as expected")

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

	// Verify exactly batch 1 events were written before failure
	// Note: Queue segments only contain CDC events, not snapshot data
	t.Log("Counting CDC events in queue segments after failed export...")
	eventCount1, err := countEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count events after first export")
	t.Logf("CDC events in queue after failed export: %d (expected: 20)", eventCount1)

	// We expect exactly 20 events (batch 1 only)
	// - Batch 1: 20 events, processed successfully, offset committed
	// - Batch 2: Failed at entry (before-batch marker), no events written
	// - Batch 3: Not yet processed (will be replayed on resume)
	// This validates our batching strategy: 20 rows + 2.5s wait = separate batch
	assert.Equal(t, 20, eventCount1, "Should have exactly 20 events from batch 1 (validates batching: batch 2 failed at entry, batch 3 not processed yet)")

	t.Log("================================================================================")
	t.Log("Phase 2: Resuming CDC export WITHOUT failure injection...")
	t.Log("================================================================================")
	t.Log("Expected behavior:")
	t.Log("  - Debezium resumes from last committed offset (after batch 1)")
	t.Log("  - Will replay batch 2 (20 events) + batch 3 (20 events) = 40 events to replay")
	t.Log("  - Total expected: 60 CDC events (batch 1: 20, batch 2: 20, batch 3: 20)")
	t.Log("  - Note: Snapshot data (50 rows) is in separate data files, not queue segments")

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

	// Wait for CDC events to be re-processed and catch up
	// We expect: 60 CDC events total (3 batches of 20 each)
	// Note: Snapshot data is in separate files, not in queue segments
	t.Log("Waiting for resumed export to process all remaining CDC events...")
	maxWait := 120 * time.Second
	pollInterval := 5 * time.Second
	deadline := time.Now().Add(maxWait)

	var finalEventCount int
	for time.Now().Before(deadline) {
		finalEventCount, err = countEventsInQueueSegments(exportDir)
		require.NoError(t, err, "Should be able to count CDC events")

		t.Logf("Current CDC event count: %d / 60 expected", finalEventCount)

		// We expect 60 CDC events (3 batches of 20 each)
		if finalEventCount >= 60 {
			t.Logf("✓ All expected CDC events received: %d", finalEventCount)
			break
		}

		time.Sleep(pollInterval)
	}

	// Final assertions
	t.Log("================================================================================")
	t.Log("Verifying final event counts and data integrity...")
	t.Log("================================================================================")
	// We expect exactly 60 CDC events (3 batches of 20 rows each)
	// Transaction metadata events are filtered out (marked as "unsupported" in parser)
	assert.Equal(t, 60, finalEventCount, "Should have exactly 60 CDC events (3 batches of 20 each) after recovery")
	t.Logf("✓ Final CDC event count: %d (expected: 60)", finalEventCount)

	// Verify data completeness in source database
	pgConn, err := postgresContainer.GetConnection()
	require.NoError(t, err, "Failed to get Postgres connection")
	defer pgConn.Close()

	var sourceRowCount int
	err = pgConn.QueryRow("SELECT COUNT(*) FROM test_schema.cdc_test").Scan(&sourceRowCount)
	require.NoError(t, err, "Failed to query source row count")
	t.Logf("Source database row count: %d", sourceRowCount)
	assert.Equal(t, 110, sourceRowCount, "Source should have 50 snapshot + 60 CDC rows")

	// Verify no duplicate events by checking VSN uniqueness
	verifyNoEventDuplicates(t, exportDir)

	t.Log("✓ CDC batch failure and resume test completed successfully")
	t.Log("✓ CDC events were re-received after failure (Debezium offset replay)")
	t.Log("✓ Event deduplication prevented duplicates")
	t.Log("✓ All CDC data exported correctly after recovery")
	t.Logf("✓ Final CDC event count: %d", finalEventCount)
}

// setupCDCTestDataForResume creates test schema and initial snapshot data.
// The snapshot will be exported first, then CDC events are generated and tested for failure/recovery.
func setupCDCTestDataForResume(t *testing.T, container testcontainers.TestContainer) {
	container.ExecuteSqls(
		"CREATE SCHEMA IF NOT EXISTS test_schema;",
		`CREATE TABLE test_schema.cdc_test (
			id SERIAL PRIMARY KEY,
			name TEXT,
			value INTEGER,
			created_at TIMESTAMP DEFAULT NOW()
		);`,
		`ALTER TABLE test_schema.cdc_test REPLICA IDENTITY FULL;`,
		`INSERT INTO test_schema.cdc_test (name, value)
		SELECT 'snapshot_' || i, i * 10 FROM generate_series(1, 50) i;`,
	)
}

// countEventsInQueueSegments counts the total number of events in all queue segment files
func countEventsInQueueSegments(exportDir string) (int, error) {
	queueDir := filepath.Join(exportDir, "data", "queue")
	if _, err := os.Stat(queueDir); os.IsNotExist(err) {
		return 0, nil
	}

	files, err := filepath.Glob(filepath.Join(queueDir, "segment.*.ndjson"))
	if err != nil {
		return 0, fmt.Errorf("failed to glob queue segment files: %w", err)
	}

	totalEvents := 0
	for _, file := range files {
		count, err := countEventsInFile(file)
		if err != nil {
			return 0, fmt.Errorf("failed to count events in %s: %w", file, err)
		}
		totalEvents += count
	}

	return totalEvents, nil
}

// countEventsInFile counts events in a single queue segment file
func countEventsInFile(filePath string) (int, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to read file: %w", err)
	}

	lines := 0
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			lines++
		}
	}

	// Each line is an event (NDJSON format)
	return lines, nil
}

// verifyNoEventDuplicates checks that all VSN (version sequence numbers) are unique
func verifyNoEventDuplicates(t *testing.T, exportDir string) {
	queueDir := filepath.Join(exportDir, "data", "queue")
	files, err := filepath.Glob(filepath.Join(queueDir, "segment.*.ndjson"))
	require.NoError(t, err, "Failed to glob queue segment files")

	if len(files) == 0 {
		t.Log("WARNING: No queue segment files found for VSN deduplication check")
		return
	}

	vsnSet := make(map[int64]bool)
	duplicateCount := 0
	totalLines := 0
	parseErrors := 0

	for _, file := range files {
		data, err := os.ReadFile(file)
		require.NoError(t, err, "Failed to read queue segment file")

		// Parse NDJSON format
		start := 0
		for i := 0; i < len(data); i++ {
			if data[i] == '\n' {
				line := data[start:i]
				start = i + 1

				if len(line) == 0 {
					continue
				}

				totalLines++

				// Parse as generic JSON to check VSN field
				var eventData map[string]interface{}
				err := json.Unmarshal(line, &eventData)
				if err != nil {
					parseErrors++
					if parseErrors <= 3 {
						t.Logf("Failed to parse line %d: %v (line preview: %.80s...)", totalLines, err, string(line))
					}
					continue
				}

				// Extract VSN (might be "vsn" or "Vsn")
				var vsn int64
				if v, ok := eventData["vsn"]; ok {
					if vsnFloat, ok := v.(float64); ok {
						vsn = int64(vsnFloat)
					}
				} else if v, ok := eventData["Vsn"]; ok {
					if vsnFloat, ok := v.(float64); ok {
						vsn = int64(vsnFloat)
					}
				} else {
					// VSN field not found, might be a different event type
					continue
				}

				if vsnSet[vsn] {
					duplicateCount++
					t.Logf("WARNING: Duplicate VSN found: %d", vsn)
				} else {
					vsnSet[vsn] = true
				}
			}
		}
	}

	if parseErrors > 0 {
		t.Logf("WARNING: Failed to parse %d out of %d lines in queue segments", parseErrors, totalLines)
	}

	assert.Equal(t, 0, duplicateCount, "No duplicate events should exist (event deduplication should work)")
	t.Logf("✓ Verified %d unique events with no duplicates (out of %d lines)", len(vsnSet), totalLines)
}
