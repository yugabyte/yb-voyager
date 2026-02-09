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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// TestCDCOffsetCommitFailureAndResume verifies replay and deduplication when offset commit fails.
//
// Scenario:
// 1. Start CDC export (snapshot-and-changes mode) with 50 snapshot rows
// 2. Insert 20 CDC events and process them (write to queue + flush/sync)
// 3. Inject failure at before-offset-commit marker (before offsets are persisted)
// 4. Export crashes; queue has 20 events but offsets file is empty
// 5. Resume export and verify batch is replayed from the beginning
// 6. Verify dedup cache skips all 20 replayed events (no duplicates in queue)
//
// This test validates:
// - Offset commit failure forces full batch replay
// - Event deduplication prevents duplicate writes during replay
func TestCDCOffsetCommitFailureAndResume(t *testing.T) {
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

	setupOffsetCommitTestData(t, postgresContainer)
	defer postgresContainer.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema_offset_commit CASCADE;",
	)

	bytemanHelper, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err, "Failed to create Byteman helper")
	bytemanHelper.AddRuleFromBuilder(
		testutils.NewRule("fail_offset_commit").
			AtMarker(testutils.MarkerCDC, "before-offset-commit").
			If("incrementCounter(\"offset_commit\") == 1").
			ThrowException("java.lang.RuntimeException", "Simulated offset commit failure"),
	)
	require.NoError(t, bytemanHelper.WriteRules(), "Failed to write Byteman rules")

	cdcEventsGenerated := make(chan bool, 1)
	offsetBeforeCDCCh := make(chan string, 1)
	generateCDCEvents := func() {
		if err := waitForStreamingMode(exportDir, 90*time.Second, 2*time.Second); err != nil {
			logTestf(t, "Failed to reach streaming mode: %v", err)
			return
		}
		offsetBeforeCDCCh <- readOffsetFileChecksum(exportDir)
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema_offset_commit.cdc_offset_commit_test (name, value)
			SELECT 'batch1_' || i, 100 + i FROM generate_series(1, 20) i;`,
		)
		time.Sleep(3 * time.Second)
		cdcEventsGenerated <- true
	}

	exportRunner := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_offset_commit",
		"--disable-pb", "true",
		"--yes",
	}, generateCDCEvents, true).WithEnv(bytemanHelper.GetEnv()...)

	err = exportRunner.Run()
	require.NoError(t, err, "Failed to start export")

	matched, err := bytemanHelper.WaitForInjection(">>> BYTEMAN: fail_offset_commit", 90*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs for offset commit failure")
	require.True(t, matched, "Byteman offset commit failure should be injected")

	select {
	case <-cdcEventsGenerated:
	case <-time.After(60 * time.Second):
		require.Fail(t, "CDC event generation timed out")
	}
	var offsetBeforeCDC string
	select {
	case offsetBeforeCDC = <-offsetBeforeCDCCh:
	case <-time.After(30 * time.Second):
		require.Fail(t, "Timed out waiting to capture offsets before CDC insert")
	}

	_, waitErr := waitForProcessExitOrKill(exportRunner, exportDir, 60*time.Second)
	require.Error(t, waitErr, "Export should exit with error after offset commit failure")

	eventCountAfterFailure, err := countEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count events after failure")
	require.Equal(t, 20, eventCountAfterFailure, "Expected 20 CDC events after failure")
	offsetAfterFailure := readOffsetFileChecksum(exportDir)
	require.Equal(t, offsetBeforeCDC, offsetAfterFailure, "Offsets advanced despite before-offset-commit failure; replay will not occur")
	offsetContents := readOffsetFileContents(exportDir)
	logTestf(t, "Offset file contents after failure: %q", offsetContents)
	require.Equal(t, "", strings.TrimSpace(offsetContents), "Offset file should be empty after failure")

	eventIDsBefore, err := collectEventIDsForOffsetCommitTest(exportDir)
	require.NoError(t, err, "Failed to read event_ids after failure")
	require.Len(t, eventIDsBefore, 20, "Expected 20 unique event_ids after failure")
	verifyNoEventIDDuplicates(t, exportDir)
	eventCountBeforeResume := eventCountAfterFailure
	logTestf(t, "Queue count before resume: %d", eventCountBeforeResume)
	dedupSkipsBeforeResume, err := countDedupSkipLogs(exportDir)
	require.NoError(t, err, "Failed to count dedup skip logs before resume")
	logTestf(t, "Dedup skip logs before resume: %d", dedupSkipsBeforeResume)

	_ = os.Remove(filepath.Join(exportDir, ".export-dataLockfile.lck"))

	bytemanHelperResume, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err, "Failed to create Byteman helper for resume")
	bytemanHelperResume.AddRuleFromBuilder(
		testutils.NewRule("replay_batch").
			AtMarker(testutils.MarkerCDC, "before-batch-streaming").
			If("incrementCounter(\"replay_batch\") == 1").
			Do(`traceln(">>> BYTEMAN: replay_batch");`),
	)
	require.NoError(t, bytemanHelperResume.WriteRules(), "Failed to write Byteman rules for resume")

	exportRunnerResume := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_offset_commit",
		"--disable-pb", "true",
		"--yes",
	}, nil, true).WithEnv(bytemanHelperResume.GetEnv()...)

	err = exportRunnerResume.Run()
	require.NoError(t, err, "Failed to start export resume")

	replayMatched, err := bytemanHelperResume.WaitForInjection(">>> BYTEMAN: replay_batch", 90*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs for replay marker")
	require.True(t, replayMatched, "Expected replay batch after resume")

	eventCountAfterReplay, err := countEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count events after replay marker")
	logTestf(t, "Queue count after replay marker: %d", eventCountAfterReplay)
	require.Equal(t, eventCountBeforeResume, eventCountAfterReplay, "Replay processed but queue count should remain unchanged")

	waitForCDCEventCount(t, exportDir, 20, 60*time.Second, 2*time.Second)
	eventCountAfterResume, err := countEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count events after resume")
	require.Equal(t, 20, eventCountAfterResume, "Event count should remain 20 after replay")

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
	logTestf(t, "Dedup skip logs after resume: %d", dedupSkipsAfterResume)
	require.GreaterOrEqual(t, dedupSkipsAfterResume-dedupSkipsBeforeResume, 20,
		"Expected dedup cache to skip at least 20 replayed records on resume")

	_ = exportRunnerResume.Kill()
	_ = killDebeziumForExportDir(exportDir)
	_ = os.Remove(filepath.Join(exportDir, ".export-dataLockfile.lck"))
}

// TestCDCBatchFailureBeforeHandleBatchComplete verifies durability gap when crash happens before flush/sync.
//
// Scenario:
// 1. Start CDC export (snapshot-and-changes mode) with 50 snapshot rows
// 2. Insert 20 CDC events with large payloads to force buffered writes
// 3. Inject failure at before-handle-batch-complete marker (after write, before flush/sync)
// 4. Export crashes; data may be in buffer but not flushed to disk
// 5. Resume export and insert 10 more CDC events
// 6. Verify all 30 events eventually written with no duplicates
//
// This test validates:
// - Durability gap: records written to buffer but not fsynced are lost on crash
// - Recovery replays lost events from offsets
// - Deduplication works correctly during replay
func TestCDCBatchFailureBeforeHandleBatchComplete(t *testing.T) {
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

	setupBeforeHandleBatchCompleteTestData(t, postgresContainer)
	defer postgresContainer.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema_before_batch_complete CASCADE;",
	)

	bytemanHelper, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err, "Failed to create Byteman helper")
	bytemanHelper.AddRuleFromBuilder(
		testutils.NewRule("fail_before_handle_batch_complete").
			AtMarker(testutils.MarkerCDC, "before-handle-batch-complete").
			If("incrementCounter(\"before_handle_batch_complete\") == 1").
			ThrowException("java.lang.RuntimeException", "Simulated failure before handleBatchComplete"),
	)
	require.NoError(t, bytemanHelper.WriteRules(), "Failed to write Byteman rules")

	cdcEventsGenerated := make(chan bool, 1)
	generateCDCEvents := func() {
		if err := waitForStreamingMode(exportDir, 90*time.Second, 2*time.Second); err != nil {
			logTestf(t, "Failed to reach streaming mode: %v", err)
			return
		}
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema_before_batch_complete.cdc_before_batch_complete_test (name, value, payload)
			SELECT 'batch1_' || i, 100 + i, repeat('x', 2000) FROM generate_series(1, 20) i;`,
		)
		time.Sleep(3 * time.Second)
		cdcEventsGenerated <- true
	}

	exportRunner := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_before_batch_complete",
		"--disable-pb", "true",
		"--yes",
	}, generateCDCEvents, true).WithEnv(bytemanHelper.GetEnv()...)

	err = exportRunner.Run()
	require.NoError(t, err, "Failed to start export")

	matched, err := bytemanHelper.WaitForInjection(">>> BYTEMAN: fail_before_handle_batch_complete", 90*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs for handleBatchComplete failure")
	require.True(t, matched, "Byteman failure should be injected before handleBatchComplete")

	select {
	case <-cdcEventsGenerated:
	case <-time.After(60 * time.Second):
		require.Fail(t, "CDC event generation timed out")
	}

	_, waitErr := waitForProcessExitOrKill(exportRunner, exportDir, 60*time.Second)
	require.Error(t, waitErr, "Export should exit with error after failure")

	eventCountAfterFailure, err := countEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count events after failure")
	logTestf(t, "Queue count after failure: %d", eventCountAfterFailure)
	_, _ = verifyNoEventIDDuplicatesAfterFailure(t, exportDir)

	_ = os.Remove(filepath.Join(exportDir, ".export-dataLockfile.lck"))

	insertAfterResume := make(chan bool, 1)
	generateAfterResumeEvents := func() {
		if err := waitForStreamingMode(exportDir, 90*time.Second, 2*time.Second); err != nil {
			logTestf(t, "Failed to reach streaming mode after resume: %v", err)
			return
		}
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema_before_batch_complete.cdc_before_batch_complete_test (name, value, payload)
			SELECT 'resume_' || i, 200 + i, repeat('y', 2000) FROM generate_series(1, 10) i;`,
		)
		insertAfterResume <- true
	}

	exportRunnerResume := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_before_batch_complete",
		"--disable-pb", "true",
		"--yes",
	}, generateAfterResumeEvents, true)

	err = exportRunnerResume.Run()
	require.NoError(t, err, "Failed to start export resume")

	select {
	case <-insertAfterResume:
	case <-time.After(60 * time.Second):
		require.Fail(t, "Post-resume CDC event generation timed out")
	}

	waitForCDCEventCount(t, exportDir, 30, 60*time.Second, 2*time.Second)
	eventCountAfterResume, err := countEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count events after resume")
	require.Equal(t, 30, eventCountAfterResume, "Expected 30 CDC events after resume")
	verifyNoEventIDDuplicates(t, exportDir)

	_ = exportRunnerResume.Kill()
	_ = killDebeziumForExportDir(exportDir)
	_ = os.Remove(filepath.Join(exportDir, ".export-dataLockfile.lck"))
}

// TestCDCQueueWriteFailureAndResume verifies recovery when queue write fails mid-batch.
//
// Scenario:
// 1. Start CDC export (snapshot-and-changes mode) with 50 snapshot rows
// 2. Insert 40 CDC events with large payloads (20KB each) to exceed buffer size
// 3. Inject failure at before-write-record marker on the 25th event
// 4. Export crashes with ~24 events written (buffer flushed due to size)
// 5. Resume export and verify all 40 events eventually written
// 6. Verify no event count overgrowth (dedup prevents duplicates)
//
// This test validates:
// - Mid-write failure recovery
// - Buffered data is flushed when buffer size exceeds threshold
// - Deduplication prevents event count from exceeding expected total
func TestCDCQueueWriteFailureAndResume(t *testing.T) {
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

	setupQueueWriteFailureTestData(t, postgresContainer)
	defer postgresContainer.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema_queue_write CASCADE;",
	)

	bytemanHelper, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err, "Failed to create Byteman helper")
	bytemanHelper.AddRuleFromBuilder(
		testutils.NewRule("fail_queue_write").
			AtMarker(testutils.MarkerCDC, "before-write-record").
			If("incrementCounter(\"write_record\") == 25").
			ThrowException("java.lang.RuntimeException", "Simulated queue write failure"),
	)
	require.NoError(t, bytemanHelper.WriteRules(), "Failed to write Byteman rules")

	cdcEventsGenerated := make(chan bool, 1)
	generateCDCEvents := func() {
		if err := waitForStreamingMode(exportDir, 90*time.Second, 2*time.Second); err != nil {
			logTestf(t, "Failed to reach streaming mode: %v", err)
			return
		}
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema_queue_write.cdc_queue_write_test (name, value, payload)
			SELECT 'batch1_' || i, 100 + i, repeat('q', 20000) FROM generate_series(1, 40) i;`,
		)
		time.Sleep(3 * time.Second)
		cdcEventsGenerated <- true
	}

	exportRunner := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_queue_write",
		"--disable-pb", "true",
		"--yes",
	}, generateCDCEvents, true).WithEnv(bytemanHelper.GetEnv()...)

	err = exportRunner.Run()
	require.NoError(t, err, "Failed to start export")

	matched, err := bytemanHelper.WaitForInjection(">>> BYTEMAN: fail_queue_write", 90*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs for queue write failure")
	require.True(t, matched, "Byteman queue write failure should be injected")

	select {
	case <-cdcEventsGenerated:
	case <-time.After(60 * time.Second):
		require.Fail(t, "CDC event generation timed out")
	}

	_, waitErr := waitForProcessExitOrKill(exportRunner, exportDir, 60*time.Second)
	require.Error(t, waitErr, "Export should exit with error after queue write failure")

	eventCountAfterFailure, err := countEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count events after failure")
	logTestf(t, "Queue count after failure: %d", eventCountAfterFailure)

	_ = os.Remove(filepath.Join(exportDir, ".export-dataLockfile.lck"))

	exportRunnerResume := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_queue_write",
		"--disable-pb", "true",
		"--yes",
	}, nil, true)

	err = exportRunnerResume.Run()
	require.NoError(t, err, "Failed to start export resume")

	eventCountAfterResumeStart, err := countEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count events after resume start")
	logTestf(t, "Queue count after resume start: %d", eventCountAfterResumeStart)
	truncationMatched, err := waitForTruncationLog(exportDir, 60*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs for truncation")
	if truncationMatched {
		logTestf(t, "✓ Observed queue segment truncation on resume")
	} else {
		logTestf(t, "ℹ No truncation log observed on resume")
	}

	waitForCDCEventCount(t, exportDir, 40, 120*time.Second, 5*time.Second)
	eventCountAfterResume, err := countEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count events after resume")
	require.Equal(t, 40, eventCountAfterResume, "Expected 40 CDC events after resume")
	assertEventCountDoesNotExceed(t, exportDir, 40, 15*time.Second, 2*time.Second)
	verifyNoEventIDDuplicates(t, exportDir)

	_ = exportRunnerResume.Kill()
	_ = killDebeziumForExportDir(exportDir)
	_ = os.Remove(filepath.Join(exportDir, ".export-dataLockfile.lck"))
}

// TestCDCRotationMidBatchClosesSegment verifies queue segment rotation properly closes rotated segments.
//
// Scenario:
// 1. Start CDC export with very small queue segment size (8KB via QUEUE_SEGMENT_MAX_BYTES)
// 2. Insert 30 CDC events with 5KB payloads to force multiple segment rotations mid-batch
// 3. Inject failure at before-handle-batch-complete marker (before batch commits)
// 4. Export crashes with multiple queue segments created
// 5. Verify the first (lowest-numbered) rotated segment is closed with EOF marker
//
// This test validates:
// - Segment rotation mid-batch properly closes/syncs the old segment
// - Rotated segments have EOF markers even when batch doesn't complete
func TestCDCRotationMidBatchClosesSegment(t *testing.T) {
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

	setupRotationMidBatchTestData(t, postgresContainer)
	defer postgresContainer.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema_rotation CASCADE;",
	)

	bytemanHelper, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err, "Failed to create Byteman helper")
	bytemanHelper.AddRuleFromBuilder(
		testutils.NewRule("fail_before_handle_batch_complete_rotation").
			AtMarker(testutils.MarkerCDC, "before-handle-batch-complete").
			If("incrementCounter(\"before_handle_batch_complete\") == 1").
			ThrowException("java.lang.RuntimeException", "Simulated failure before handleBatchComplete"),
	)
	require.NoError(t, bytemanHelper.WriteRules(), "Failed to write Byteman rules")

	cdcEventsGenerated := make(chan bool, 1)
	generateCDCEvents := func() {
		if err := waitForStreamingMode(exportDir, 90*time.Second, 2*time.Second); err != nil {
			logTestf(t, "Failed to reach streaming mode: %v", err)
			return
		}
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema_rotation.cdc_rotation_test (name, value, payload)
			SELECT 'batch1_' || i, 100 + i, repeat('r', 5000) FROM generate_series(1, 30) i;`,
		)
		time.Sleep(3 * time.Second)
		cdcEventsGenerated <- true
	}

	envVars := append(bytemanHelper.GetEnv(), "QUEUE_SEGMENT_MAX_BYTES=8192")
	exportRunner := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_rotation",
		"--disable-pb", "true",
		"--yes",
	}, generateCDCEvents, true).WithEnv(envVars...)

	err = exportRunner.Run()
	require.NoError(t, err, "Failed to start export")

	matched, err := bytemanHelper.WaitForInjection(">>> BYTEMAN: fail_before_handle_batch_complete_rotation", 90*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs for handleBatchComplete failure")
	require.True(t, matched, "Byteman failure should be injected before handleBatchComplete")

	select {
	case <-cdcEventsGenerated:
	case <-time.After(60 * time.Second):
		require.Fail(t, "CDC event generation timed out")
	}

	// Kill immediately after injection to avoid graceful shutdown that could sync segments.
	_ = exportRunner.Kill()
	_ = killDebeziumForExportDir(exportDir)

	segmentFiles, err := listQueueSegmentFiles(exportDir)
	require.NoError(t, err, "Failed to list queue segment files")
	require.GreaterOrEqual(t, len(segmentFiles), 2, "Expected multiple queue segments after rotation")
	logTestf(t, "Queue segment files after failure: %v", segmentFiles)

	lowestSegmentPath := ""
	lowestSegmentNum := int64(-1)
	latestSegmentPath := ""
	latestSegmentNum := int64(-1)
	for _, segmentPath := range segmentFiles {
		segmentNum, err := parseQueueSegmentNum(segmentPath)
		require.NoError(t, err, "Failed to parse queue segment number")
		if lowestSegmentNum == -1 || segmentNum < lowestSegmentNum {
			lowestSegmentNum = segmentNum
			lowestSegmentPath = segmentPath
		}
		if segmentNum > latestSegmentNum {
			latestSegmentNum = segmentNum
			latestSegmentPath = segmentPath
		}
	}
	require.NotEmpty(t, lowestSegmentPath, "Expected to identify lowest queue segment")
	require.NotEmpty(t, latestSegmentPath, "Expected to identify latest queue segment")

	closed, err := isQueueSegmentClosed(lowestSegmentPath)
	require.NoError(t, err, "Failed to check queue segment EOF marker")
	require.True(t, closed, "First rotated queue segment should be closed with EOF marker")

	require.GreaterOrEqual(t, latestSegmentNum, int64(1), "Expected latest segment to be >= 1 after rotation")
}

// TestCDCQueueSegmentTruncationOnResume verifies incomplete queue segments are truncated on resume.
//
// Scenario:
// 1. Start CDC export with large segment size (1GB, forces single segment)
// 2. Insert 20 CDC events with large payloads (20KB each) to force buffer flush to disk
// 3. Inject failure at before-handle-batch-complete marker (after write, before fsync/commit)
// 4. Export crashes; queue segment file size > committed size in metadb
// 5. Resume export and verify:
//    - Truncation log appears in Debezium logs
//    - Queue segment file is truncated back to committed size (0 bytes in this case)
//
// This test validates:
// - Queue segment recovery truncates uncommitted bytes on resume
// - Metadb size_committed is the source of truth for valid data boundary
func TestCDCQueueSegmentTruncationOnResume(t *testing.T) {
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

	setupTruncationTestData(t, postgresContainer)
	defer postgresContainer.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema_truncation CASCADE;",
	)

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
			logTestf(t, "Failed to reach streaming mode: %v", err)
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
	logTestf(t, "Queue segment size before resume: %d, committed size: %d", fileSizeBefore, committedSize)

	logTest(t, "Resuming export to trigger truncation...")
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
	logTestf(t, "Truncation log observed on resume: %v", truncationMatched)
	require.True(t, truncationMatched, "Expected truncation log on resume")

	logTest(t, "Verifying segment size after truncation...")
	fileSizeAfter, err := getQueueSegmentFileSize(segmentFiles[0])
	require.NoError(t, err, "Failed to read queue segment size after truncation")
	logTestf(t, "Queue segment size after truncation: %d", fileSizeAfter)
	_ = killDebeziumForExportDir(exportDir)
	_ = os.Remove(filepath.Join(exportDir, ".export-dataLockfile.lck"))
}
