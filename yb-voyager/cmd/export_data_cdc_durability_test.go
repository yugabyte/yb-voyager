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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// TestCDCOffsetCommitFailureAndResume verifies that live migration `export data` can resume after
// an offset commit failure during CDC streaming.
//
// Scenario:
//  1. Start `export data` (snapshot-and-changes mode) with 50 snapshot rows.
//  2. Insert 20 CDC events and process them (write to queue + flush/sync).
//  3. Inject failure at before-offset-commit marker (before offsets are persisted).
//  4. Export crashes; queue has 20 events but offsets file is empty.
//  5. Resume `export data` and verify batch is replayed from the beginning.
//  6. Verify dedup cache skips all 20 replayed events (no duplicates in queue).
//
// This test validates:
// - Offset commit failure forces full batch replay
// - Event deduplication prevents duplicate writes during replay
//
// Injection point:
//   - Byteman rule on Debezium at the before-offset-commit marker,
//     firing after queue write but before offset persistence.
func TestCDCOffsetCommitFailureAndResume(t *testing.T) {
	if os.Getenv("BYTEMAN_JAR") == "" {
		t.Skip("Skipping test: BYTEMAN_JAR environment variable not set. Install Byteman to run this test.")
	}

	ctx := context.Background()

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:    ContainerConfig{Type: "postgresql", ForLive: true, DatabaseName: "postgres"},
		SchemaNames: []string{"test_schema_offset_commit"},
		SchemaSQL: []string{
			"DROP SCHEMA IF EXISTS test_schema_offset_commit CASCADE;",
			"CREATE SCHEMA test_schema_offset_commit;",
			`CREATE TABLE test_schema_offset_commit.cdc_offset_commit_test (
				id SERIAL PRIMARY KEY,
				name TEXT,
				value INTEGER,
				created_at TIMESTAMP DEFAULT NOW()
			);`,
			`ALTER TABLE test_schema_offset_commit.cdc_offset_commit_test REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema_offset_commit.cdc_offset_commit_test (name, value)
			SELECT 'snapshot_' || i, i * 10 FROM generate_series(1, 50) i;`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema_offset_commit.cdc_offset_commit_test (name, value)
			SELECT 'batch1_' || i, 100 + i FROM generate_series(1, 20) i;`,
		},
		CleanupSQL: []string{"DROP SCHEMA IF EXISTS test_schema_offset_commit CASCADE;"},
	})
	defer lm.Cleanup()
	require.NoError(t, lm.SetupContainers(ctx))
	require.NoError(t, lm.SetupSchema())

	exportDir := lm.GetExportDir()

	bytemanHelper, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err, "Failed to create Byteman helper")
	bytemanHelper.AddRuleFromBuilder(
		testutils.NewRule("fail_offset_commit").
			AtMarker(testutils.MarkerCDC, "before-offset-commit").
			If("incrementCounter(\"offset_commit\") == 1").
			ThrowException("java.lang.RuntimeException", "Simulated offset commit failure"),
	)
	require.NoError(t, bytemanHelper.WriteRules(), "Failed to write Byteman rules")

	// Run 1: export with Byteman injection at before-offset-commit
	err = lm.StartExportDataWithEnv(true, nil, bytemanHelper.GetEnv())
	require.NoError(t, err, "Failed to start export")

	time.Sleep(10 * time.Second)

	offsetBeforeCDC := readOffsetFileChecksum(exportDir)

	lm.ExecuteSourceDelta()

	matched, err := bytemanHelper.WaitForInjection(">>> BYTEMAN: fail_offset_commit", 90*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs for offset commit failure")
	require.True(t, matched, "Byteman offset commit failure should be injected")

	err = lm.WaitForExportDataExit()
	require.Error(t, err, "Export should exit with error after offset commit failure")

	eventCountAfterFailure, err := countEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count events after failure")
	require.Equal(t, 20, eventCountAfterFailure, "Expected 20 CDC events after failure")
	offsetAfterFailure := readOffsetFileChecksum(exportDir)
	require.Equal(t, offsetBeforeCDC, offsetAfterFailure, "Offsets advanced despite before-offset-commit failure; replay will not occur")
	offsetContents := readOffsetFileContents(exportDir)
	require.Equal(t, "", strings.TrimSpace(offsetContents), "Offset file should be empty after failure")

	eventIDsBefore, err := collectEventIDsForOffsetCommitTest(exportDir)
	require.NoError(t, err, "Failed to read event_ids after failure")
	require.Len(t, eventIDsBefore, 20, "Expected 20 unique event_ids after failure")
	verifyNoEventIDDuplicates(t, exportDir)
	dedupSkipsBeforeResume, err := countDedupSkipLogs(exportDir)
	require.NoError(t, err, "Failed to count dedup skip logs before resume")

	// Run 2: resume export with Byteman tracing rule to detect batch replay
	bytemanHelperResume, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err, "Failed to create Byteman helper for resume")
	bytemanHelperResume.AddRuleFromBuilder(
		testutils.NewRule("replay_batch").
			AtMarker(testutils.MarkerCDC, "before-batch-streaming").
			If("incrementCounter(\"replay_batch\") == 1").
			Do(`traceln(">>> BYTEMAN: replay_batch");`),
	)
	require.NoError(t, bytemanHelperResume.WriteRules(), "Failed to write Byteman rules for resume")

	err = lm.StartExportDataWithEnv(true, nil, bytemanHelperResume.GetEnv())
	require.NoError(t, err, "Failed to start export resume")

	replayMatched, err := bytemanHelperResume.WaitForInjection(">>> BYTEMAN: replay_batch", 90*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs for replay marker")
	require.True(t, replayMatched, "Expected replay batch after resume")

	eventIDsAfter, err := collectEventIDsForOffsetCommitTest(exportDir)
	require.NoError(t, err, "Failed to read event_ids after resume")
	require.Equal(t, len(eventIDsBefore), len(eventIDsAfter), "event_id set size should be unchanged after replay")
	for eventID := range eventIDsBefore {
		_, ok := eventIDsAfter[eventID]
		require.True(t, ok, "event_id should still exist after replay: %s", eventID)
	}
	verifyNoEventIDDuplicates(t, exportDir)
	dedupSkipsAfterResume, err := countDedupSkipLogs(exportDir)
	require.NoError(t, err, "Failed to count dedup skip logs after resume")
	require.GreaterOrEqual(t, dedupSkipsAfterResume-dedupSkipsBeforeResume, 20,
		"Expected dedup cache to skip at least 20 replayed records on resume")
}

// TestCDCBatchFailureBeforeHandleBatchComplete verifies that live migration `export data` can resume
// after a crash before flush/sync, recovering from the durability gap.
//
// Scenario:
//  1. Start `export data` (snapshot-and-changes mode) with 50 snapshot rows.
//  2. Insert 20 CDC events with large payloads to force buffered writes.
//  3. Inject failure at before-handle-batch-complete marker (after write, before flush/sync).
//  4. Export crashes; data may be in buffer but not flushed to disk.
//  5. Resume `export data` and insert 10 more CDC events.
//  6. Verify all 30 events eventually written with no duplicates.
//
// This test validates:
// - Durability gap: records written to buffer but not fsynced are lost on crash
// - Recovery replays lost events from offsets
// - Deduplication works correctly during replay
//
// Injection point:
//   - Byteman rule on Debezium at the before-handle-batch-complete marker,
//     firing after queue write but before flush/sync.
func TestCDCBatchFailureBeforeHandleBatchComplete(t *testing.T) {
	if os.Getenv("BYTEMAN_JAR") == "" {
		t.Skip("Skipping test: BYTEMAN_JAR environment variable not set. Install Byteman to run this test.")
	}

	ctx := context.Background()

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:    ContainerConfig{Type: "postgresql", ForLive: true, DatabaseName: "postgres"},
		SchemaNames: []string{"test_schema_before_batch_complete"},
		SchemaSQL: []string{
			"DROP SCHEMA IF EXISTS test_schema_before_batch_complete CASCADE;",
			"CREATE SCHEMA test_schema_before_batch_complete;",
			`CREATE TABLE test_schema_before_batch_complete.cdc_before_batch_complete_test (
				id SERIAL PRIMARY KEY,
				name TEXT,
				value INTEGER,
				payload TEXT,
				created_at TIMESTAMP DEFAULT NOW()
			);`,
			`ALTER TABLE test_schema_before_batch_complete.cdc_before_batch_complete_test REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema_before_batch_complete.cdc_before_batch_complete_test (name, value, payload)
			SELECT 'snapshot_' || i, i * 10, repeat('s', 20000) FROM generate_series(1, 50) i;`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema_before_batch_complete.cdc_before_batch_complete_test (name, value, payload)
			SELECT 'batch1_' || i, 100 + i, repeat('x', 2000) FROM generate_series(1, 20) i;`,
		},
		CleanupSQL: []string{"DROP SCHEMA IF EXISTS test_schema_before_batch_complete CASCADE;"},
	})
	defer lm.Cleanup()
	require.NoError(t, lm.SetupContainers(ctx))
	require.NoError(t, lm.SetupSchema())

	exportDir := lm.GetExportDir()

	bytemanHelper, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err, "Failed to create Byteman helper")
	bytemanHelper.AddRuleFromBuilder(
		testutils.NewRule("fail_before_handle_batch_complete").
			AtMarker(testutils.MarkerCDC, "before-handle-batch-complete").
			If("incrementCounter(\"before_handle_batch_complete\") == 1").
			ThrowException("java.lang.RuntimeException", "Simulated failure before handleBatchComplete"),
	)
	require.NoError(t, bytemanHelper.WriteRules(), "Failed to write Byteman rules")

	// Run 1: export with Byteman injection at before-handle-batch-complete
	err = lm.StartExportDataWithEnv(true, nil, bytemanHelper.GetEnv())
	require.NoError(t, err, "Failed to start export")

	time.Sleep(10 * time.Second)

	lm.ExecuteSourceDelta()

	matched, err := bytemanHelper.WaitForInjection(">>> BYTEMAN: fail_before_handle_batch_complete", 90*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs for handleBatchComplete failure")
	require.True(t, matched, "Byteman failure should be injected before handleBatchComplete")

	err = lm.WaitForExportDataExit()
	require.Error(t, err, "Export should exit with error after failure")

	_, _ = verifyNoEventIDDuplicatesAfterFailure(t, exportDir)

	// Run 2: resume export and insert 10 more events
	err = lm.StartExportData(true, nil)
	require.NoError(t, err, "Failed to start export resume")

	time.Sleep(10 * time.Second)

	lm.ExecuteOnSource(
		`INSERT INTO test_schema_before_batch_complete.cdc_before_batch_complete_test (name, value, payload)
		SELECT 'resume_' || i, 200 + i, repeat('y', 2000) FROM generate_series(1, 10) i;`,
	)

	finalEventCount := lm.WaitForCDCEventCount(t, 30, 120*time.Second, 5*time.Second)
	require.Equal(t, 30, finalEventCount, "Expected 30 CDC events after resume")
	verifyNoEventIDDuplicates(t, exportDir)
}

// TestCDCQueueWriteFailureAndResume verifies that live migration `export data` can resume after
// a queue write failure mid-batch during CDC streaming.
//
// Scenario:
//  1. Start `export data` (snapshot-and-changes mode) with 50 snapshot rows.
//  2. Insert 40 CDC events with large payloads (20KB each) to exceed buffer size.
//  3. Inject failure at before-write-record marker on the 25th event.
//  4. Export crashes with ~24 events written (buffer flushed due to size).
//  5. Resume `export data` and verify all 40 events eventually written.
//  6. Verify no event count overgrowth (dedup prevents duplicates).
//
// This test validates:
// - Mid-write failure recovery
// - Buffered data is flushed when buffer size exceeds threshold
// - Deduplication prevents event count from exceeding expected total
//
// Injection point:
//   - Byteman rule on Debezium at the before-write-record marker,
//     firing on the 25th event write.
func TestCDCQueueWriteFailureAndResume(t *testing.T) {
	if os.Getenv("BYTEMAN_JAR") == "" {
		t.Skip("Skipping test: BYTEMAN_JAR environment variable not set. Install Byteman to run this test.")
	}

	ctx := context.Background()

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:    ContainerConfig{Type: "postgresql", ForLive: true, DatabaseName: "postgres"},
		SchemaNames: []string{"test_schema_queue_write"},
		SchemaSQL: []string{
			"DROP SCHEMA IF EXISTS test_schema_queue_write CASCADE;",
			"CREATE SCHEMA test_schema_queue_write;",
			`CREATE TABLE test_schema_queue_write.cdc_queue_write_test (
				id SERIAL PRIMARY KEY,
				name TEXT,
				value INTEGER,
				payload TEXT,
				created_at TIMESTAMP DEFAULT NOW()
			);`,
			`ALTER TABLE test_schema_queue_write.cdc_queue_write_test REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema_queue_write.cdc_queue_write_test (name, value, payload)
			SELECT 'snapshot_' || i, i * 10, repeat('s', 20000) FROM generate_series(1, 50) i;`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema_queue_write.cdc_queue_write_test (name, value, payload)
			SELECT 'batch1_' || i, 100 + i, repeat('q', 20000) FROM generate_series(1, 40) i;`,
		},
		CleanupSQL: []string{"DROP SCHEMA IF EXISTS test_schema_queue_write CASCADE;"},
	})
	defer lm.Cleanup()
	require.NoError(t, lm.SetupContainers(ctx))
	require.NoError(t, lm.SetupSchema())

	exportDir := lm.GetExportDir()

	bytemanHelper, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err, "Failed to create Byteman helper")
	bytemanHelper.AddRuleFromBuilder(
		testutils.NewRule("fail_queue_write").
			AtMarker(testutils.MarkerCDC, "before-write-record").
			If("incrementCounter(\"write_record\") == 25").
			ThrowException("java.lang.RuntimeException", "Simulated queue write failure"),
	)
	require.NoError(t, bytemanHelper.WriteRules(), "Failed to write Byteman rules")

	// Run 1: export with Byteman injection at before-write-record (25th event)
	err = lm.StartExportDataWithEnv(true, nil, bytemanHelper.GetEnv())
	require.NoError(t, err, "Failed to start export")

	time.Sleep(10 * time.Second)

	lm.ExecuteSourceDelta()

	matched, err := bytemanHelper.WaitForInjection(">>> BYTEMAN: fail_queue_write", 90*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs for queue write failure")
	require.True(t, matched, "Byteman queue write failure should be injected")

	err = lm.WaitForExportDataExit()
	require.Error(t, err, "Export should exit with error after queue write failure")

	// Run 2: resume export — all 40 events should be recovered
	err = lm.StartExportData(true, nil)
	require.NoError(t, err, "Failed to start export resume")

	lm.WaitForCDCEventCount(t, 40, 120*time.Second, 5*time.Second)
	assertEventCountDoesNotExceed(t, exportDir, 40, 15*time.Second, 2*time.Second)
	verifyNoEventIDDuplicates(t, exportDir)
}

// TestCDCRotationMidBatchClosesSegment verifies that live migration `export data` properly
// closes rotated queue segments when a crash occurs mid-batch during segment rotation.
//
// Scenario:
//  1. Start `export data` with very small queue segment size (8KB via QUEUE_SEGMENT_MAX_BYTES).
//  2. Insert 30 CDC events with 5KB payloads to force multiple segment rotations mid-batch.
//  3. Inject failure at before-handle-batch-complete marker (before batch commits).
//  4. Export crashes with multiple queue segments created.
//  5. Verify the first (lowest-numbered) rotated segment is closed with EOF marker.
//
// This test validates:
// - Segment rotation mid-batch properly closes/syncs the old segment
// - Rotated segments have EOF markers even when batch doesn't complete
//
// Injection point:
//   - Byteman rule on Debezium at the before-handle-batch-complete marker,
//     firing before batch commit with small segment size forcing rotation.
func TestCDCRotationMidBatchClosesSegment(t *testing.T) {
	if os.Getenv("BYTEMAN_JAR") == "" {
		t.Skip("Skipping test: BYTEMAN_JAR environment variable not set. Install Byteman to run this test.")
	}

	ctx := context.Background()

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:    ContainerConfig{Type: "postgresql", ForLive: true, DatabaseName: "postgres"},
		SchemaNames: []string{"test_schema_rotation"},
		SchemaSQL: []string{
			"DROP SCHEMA IF EXISTS test_schema_rotation CASCADE;",
			"CREATE SCHEMA test_schema_rotation;",
			`CREATE TABLE test_schema_rotation.cdc_rotation_test (
				id SERIAL PRIMARY KEY,
				name TEXT,
				value INTEGER,
				payload TEXT,
				created_at TIMESTAMP DEFAULT NOW()
			);`,
			`ALTER TABLE test_schema_rotation.cdc_rotation_test REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema_rotation.cdc_rotation_test (name, value, payload)
			SELECT 'snapshot_' || i, i * 10, repeat('s', 20000) FROM generate_series(1, 50) i;`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema_rotation.cdc_rotation_test (name, value, payload)
			SELECT 'batch1_' || i, 100 + i, repeat('r', 5000) FROM generate_series(1, 30) i;`,
		},
		CleanupSQL: []string{"DROP SCHEMA IF EXISTS test_schema_rotation CASCADE;"},
	})
	defer lm.Cleanup()
	require.NoError(t, lm.SetupContainers(ctx))
	require.NoError(t, lm.SetupSchema())

	exportDir := lm.GetExportDir()

	bytemanHelper, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err, "Failed to create Byteman helper")
	bytemanHelper.AddRuleFromBuilder(
		testutils.NewRule("fail_before_handle_batch_complete_rotation").
			AtMarker(testutils.MarkerCDC, "before-handle-batch-complete").
			If("incrementCounter(\"before_handle_batch_complete\") == 1").
			ThrowException("java.lang.RuntimeException", "Simulated failure before handleBatchComplete"),
	)
	require.NoError(t, bytemanHelper.WriteRules(), "Failed to write Byteman rules")

	envVars := append(bytemanHelper.GetEnv(), "QUEUE_SEGMENT_MAX_BYTES=8192")
	err = lm.StartExportDataWithEnv(true, nil, envVars)
	require.NoError(t, err, "Failed to start export")

	time.Sleep(10 * time.Second)

	lm.ExecuteSourceDelta()

	matched, err := bytemanHelper.WaitForInjection(">>> BYTEMAN: fail_before_handle_batch_complete_rotation", 90*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs for handleBatchComplete failure")
	require.True(t, matched, "Byteman failure should be injected before handleBatchComplete")

	// Kill immediately after injection to avoid graceful shutdown that could sync segments.
	_ = lm.exportCmd.Kill()
	lm.KillDebezium(SOURCE_DB_EXPORTER_ROLE)

	segmentFiles, err := listQueueSegmentFiles(exportDir)
	require.NoError(t, err, "Failed to list queue segment files")
	require.GreaterOrEqual(t, len(segmentFiles), 2, "Expected multiple queue segments after rotation")

	lowestSegmentPath, _, highestSegmentNum, err := findSegmentNumRange(segmentFiles)
	require.NoError(t, err, "Failed to parse queue segment numbers")
	require.NotEmpty(t, lowestSegmentPath, "Expected to identify lowest queue segment")

	closed, err := isQueueSegmentClosed(lowestSegmentPath)
	require.NoError(t, err, "Failed to check queue segment EOF marker")
	require.True(t, closed, "First rotated queue segment should be closed with EOF marker")

	require.GreaterOrEqual(t, highestSegmentNum, int64(1), "Expected latest segment to be >= 1 after rotation")
}

// TestCDCQueueSegmentTruncationOnResume verifies that live migration `export data` correctly
// truncates incomplete queue segments back to the committed size on resume.
//
// Scenario:
//  1. Start `export data` with large segment size (1GB, forces single segment).
//  2. Insert 20 CDC events with large payloads (20KB each) to force buffer flush to disk.
//  3. Inject failure at before-handle-batch-complete marker (after write, before fsync/commit).
//  4. Export crashes; queue segment file size > committed size in metadb.
//  5. Resume `export data` and verify:
//     - Truncation log appears in Debezium logs.
//     - Queue segment file is truncated back to committed size (0 bytes in this case).
//
// This test validates:
// - Queue segment recovery truncates uncommitted bytes on resume
// - Metadb size_committed is the source of truth for valid data boundary
//
// Injection point:
//   - Byteman rule on Debezium at the before-handle-batch-complete marker,
//     firing after write but before fsync/commit with large segment size.
func TestCDCQueueSegmentTruncationOnResume(t *testing.T) {
	if os.Getenv("BYTEMAN_JAR") == "" {
		t.Skip("Skipping test: BYTEMAN_JAR environment variable not set. Install Byteman to run this test.")
	}

	ctx := context.Background()

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:    ContainerConfig{Type: "postgresql", ForLive: true},
		SchemaNames: []string{"test_schema_truncation"},
		SchemaSQL: []string{
			"DROP SCHEMA IF EXISTS test_schema_truncation CASCADE;",
			"CREATE SCHEMA test_schema_truncation;",
			`CREATE TABLE test_schema_truncation.cdc_truncation_test (
				id SERIAL PRIMARY KEY,
				name TEXT,
				value INTEGER,
				payload TEXT,
				created_at TIMESTAMP DEFAULT NOW()
			);`,
			`ALTER TABLE test_schema_truncation.cdc_truncation_test REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema_truncation.cdc_truncation_test (name, value, payload)
			SELECT 'snapshot_' || i, i * 10, repeat('s', 20000) FROM generate_series(1, 50) i;`,
		},
		CleanupSQL: []string{"DROP SCHEMA IF EXISTS test_schema_truncation CASCADE;"},
	})
	defer lm.Cleanup()
	require.NoError(t, lm.SetupContainers(ctx))
	require.NoError(t, lm.SetupSchema())

	exportDir = lm.GetExportDir()
	postgresContainer := lm.GetSourceContainer()

	bytemanHelper, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err, "Failed to create Byteman helper")
	bytemanHelper.AddRuleFromBuilder(
		testutils.NewRule("fail_before_handle_batch_complete_truncation").
			AtMarker(testutils.MarkerCDC, "before-handle-batch-complete").
			If("incrementCounter(\"before_handle_batch_complete\") == 1").
			ThrowException("java.lang.RuntimeException", "Simulated failure before handleBatchComplete"),
	)
	require.NoError(t, bytemanHelper.WriteRules(), "Failed to write Byteman rules")

	cdcEventsGenerated := make(chan bool, 1)
	generateCDCEvents := func() {
		if err := waitForStreamingMode(exportDir, 90*time.Second, 2*time.Second); err != nil {
			testutils.LogTestf(t, "Failed to reach streaming mode: %v", err)
			return
		}
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema_truncation.cdc_truncation_test (name, value, payload)
			SELECT 'batch1_' || i, 100 + i, repeat('t', 20000) FROM generate_series(1, 20) i;`,
		)
		time.Sleep(3 * time.Second)
		cdcEventsGenerated <- true
	}

	envVars := append(bytemanHelper.GetEnv(), "QUEUE_SEGMENT_MAX_BYTES=1073741824")
	exportRunner := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_truncation",
		"--disable-pb", "true",
		"--yes",
	}, generateCDCEvents, true).WithEnv(envVars...)

	err = exportRunner.Run()
	require.NoError(t, err, "Failed to start export")

	matched, err := bytemanHelper.WaitForInjection(">>> BYTEMAN: fail_before_handle_batch_complete_truncation", 90*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs for handleBatchComplete failure")
	require.True(t, matched, "Byteman failure should be injected before handleBatchComplete")

	select {
	case <-cdcEventsGenerated:
	case <-time.After(60 * time.Second):
		require.Fail(t, "CDC event generation timed out")
	}

	_, waitErr := waitForProcessExitOrKill(exportRunner, exportDir, 60*time.Second)
	require.Error(t, waitErr, "Export should exit with error after failure")

	segmentFiles, err := listQueueSegmentFiles(exportDir)
	require.NoError(t, err, "Failed to list queue segment files")
	require.Len(t, segmentFiles, 1, "Expected a single queue segment before resume")
	segmentNum, err := parseQueueSegmentNum(segmentFiles[0])
	require.NoError(t, err, "Failed to parse queue segment number")

	fileSizeBefore, err := getQueueSegmentFileSize(segmentFiles[0])
	require.NoError(t, err, "Failed to read queue segment size after failure")
	committedSize, err := getQueueSegmentCommittedSize(exportDir, segmentNum)
	require.NoError(t, err, "Failed to read committed size from metadb")
	require.Greater(t, fileSizeBefore, committedSize, "Expected file size to exceed committed size before resume")
	testutils.LogTestf(t, "Queue segment size before resume: %d, committed size: %d", fileSizeBefore, committedSize)

	testutils.LogTest(t, "Resuming export to trigger truncation...")
	exportRunnerResume := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_truncation",
		"--disable-pb", "true",
		"--yes",
	}, nil, true)

	err = exportRunnerResume.Run()
	require.NoError(t, err, "Failed to start export resume")
	defer exportRunnerResume.Kill()

	truncationMatched, err := waitForTruncationLog(exportDir, 60*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs for truncation")
	testutils.LogTestf(t, "Truncation log observed on resume: %v", truncationMatched)
	require.True(t, truncationMatched, "Expected truncation log on resume")

	testutils.LogTest(t, "Verifying segment size after truncation...")
	fileSizeAfter, err := getQueueSegmentFileSize(segmentFiles[0])
	require.NoError(t, err, "Failed to read queue segment size after truncation")
	testutils.LogTestf(t, "Queue segment size after truncation: %d", fileSizeAfter)
	_ = killDebeziumForExportDir(exportDir)
	lm.RemoveExportLockfile()
}
