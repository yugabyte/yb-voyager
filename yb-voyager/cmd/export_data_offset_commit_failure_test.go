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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// TestCDCOffsetCommitFailureAndResume verifies proper durability guarantees:
// 1. Events are NOT marked as processed until after batch fsync completes
// 2. Failed offset commits cause event replay on resume
// 3. Deduplication prevents duplicate events from being written
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

	logHighlight(t, "")
	logHighlight(t, "========================================================================")
	logHighlight(t, "TEST: CDC Offset Commit Failure and Resume with Deduplication")
	logHighlight(t, "========================================================================")
	logHighlight(t, "")

	// ============================================================================
	// SETUP: Configure Byteman to fail offset commit on first CDC batch
	// ============================================================================

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
			t.Logf("Failed to reach streaming mode: %v", err)
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

	// ============================================================================
	// RUN INITIAL EXPORT: Process snapshot, then CDC events with injected failure
	// ============================================================================

	exportRunner := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_offset_commit",
		"--disable-pb", "true",
		"--yes",
	}, generateCDCEvents, true).WithEnv(bytemanHelper.GetEnv()...)

	err = exportRunner.Run()
	require.NoError(t, err, "Failed to start export")

	// ============================================================================
	// WAIT FOR FAILURE INJECTION: Confirm Byteman injected failure
	// ============================================================================

	logHighlight(t, "▶ FAILURE INJECTION: Waiting for offset commit failure...")

	matched, err := bytemanHelper.WaitForInjection(">>> BYTEMAN: fail_offset_commit", 90*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs")
	require.True(t, matched, "Byteman offset commit failure should be injected")
	logHighlight(t, "  ✓ Byteman injected offset commit failure")

	select {
	case <-cdcEventsGenerated:
		logHighlight(t, "  ✓ CDC events generated (20 inserts)")
	case <-time.After(60 * time.Second):
		require.Fail(t, "CDC event generation timed out")
	}

	var offsetBeforeCDC string
	select {
	case offsetBeforeCDC = <-offsetBeforeCDCCh:
		if offsetBeforeCDC == "" {
			logHighlight(t, "  ✓ Captured offset before CDC: (empty/no offset file)")
		} else if len(offsetBeforeCDC) > 16 {
			logHighlight(t, "  ✓ Captured offset before CDC: %s...", offsetBeforeCDC[:16])
		} else {
			logHighlight(t, "  ✓ Captured offset before CDC: %s", offsetBeforeCDC)
		}
	case <-time.After(30 * time.Second):
		require.Fail(t, "Timed out waiting to capture offset before CDC")
	}

	_, waitErr := waitForProcessExitOrKill(exportRunner, exportDir, 60*time.Second)
	require.Error(t, waitErr, "Export should exit with error after offset commit failure")
	logHighlight(t, "  ✓ Export process exited with error (expected)")

	// ============================================================================
	// VERIFY DURABILITY GUARANTEE: Events written but offsets NOT committed
	// This is the core fix - events should only be marked processed AFTER fsync
	// ============================================================================

	logHighlight(t, "▶ DURABILITY CHECKS: Verifying events written but offsets not committed...")

	// Check 1: Events are written to queue
	eventCountAfterFailure, err := countEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count events after failure")
	require.Equal(t, 20, eventCountAfterFailure, "Expected 20 CDC events written to queue")
	logHighlight(t, "  ✓ Check 1 PASSED: 20 events written to queue")

	// Check 2: Offsets NOT advanced (ensures replay will occur)
	offsetAfterFailure := readOffsetFileChecksum(exportDir)
	require.Equal(t, offsetBeforeCDC, offsetAfterFailure,
		"Offsets should NOT advance when offset commit fails")
	logHighlight(t, "  ✓ Check 2 PASSED: Offsets NOT advanced (checksum unchanged)")

	offsetContents := readOffsetFileContents(exportDir)
	require.Equal(t, "", strings.TrimSpace(offsetContents), "Offset file should be empty")
	logHighlight(t, "  ✓ Check 3 PASSED: Offset file is empty (%q)", offsetContents)

	// ============================================================================
	// CAPTURE BASELINE STATE: Collect metrics before resume for comparison
	// ============================================================================

	logHighlight(t, "▶ BASELINE STATE: Capturing metrics before resume...")

	eventIDsBefore, err := collectEventIDsForOffsetCommitTest(exportDir)
	require.NoError(t, err, "Failed to collect event_ids after failure")
	require.Len(t, eventIDsBefore, 20, "Expected 20 unique event_ids")
	verifyNoEventIDDuplicates(t, exportDir)
	logHighlight(t, "  ✓ Captured: 20 unique event_ids, no duplicates")
	logHighlight(t, "  ✓ Captured: Queue count = %d", eventCountAfterFailure)

	dedupSkipsBeforeResume, err := countDedupSkipLogs(exportDir)
	require.NoError(t, err, "Failed to count dedup skip logs")
	logHighlight(t, "  ✓ Captured: Dedup skips = %d", dedupSkipsBeforeResume)

	logHighlight(t, "")
	logHighlight(t, "========================================================================")
	logHighlight(t, "PHASE 2: Resume Export and Verify Replay with Deduplication")
	logHighlight(t, "========================================================================")
	logHighlight(t, "")

	// ============================================================================
	// PREPARE FOR RESUME: Setup Byteman to detect replay, release lock
	// ============================================================================

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

	// ============================================================================
	// RESUME EXPORT: Should replay events from last committed offset
	// ============================================================================

	exportRunnerResume := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_offset_commit",
		"--disable-pb", "true",
		"--yes",
	}, nil, true).WithEnv(bytemanHelperResume.GetEnv()...)

	err = exportRunnerResume.Run()
	require.NoError(t, err, "Failed to start export resume")

	// ============================================================================
	// VERIFY REPLAY OCCURRED: Events replayed from last committed offset
	// ============================================================================

	logHighlight(t, "▶ REPLAY CHECKS: Verifying events replayed from last committed offset...")

	replayMatched, err := bytemanHelperResume.WaitForInjection(">>> BYTEMAN: replay_batch", 90*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs")
	require.True(t, replayMatched, "Replay batch should occur after resume")
	logHighlight(t, "  ✓ REPLAY CONFIRMED: Batch replayed after resume")

	// ============================================================================
	// VERIFY DEDUPLICATION: Replayed events recognized and NOT written again
	// ============================================================================

	logHighlight(t, "▶ DEDUPLICATION CHECKS: Verifying replayed events not written again...")

	// Check 1: Event count remains unchanged
	waitForCDCEventCount(t, exportDir, 20, 60*time.Second, 2*time.Second)
	eventCountAfterResume, err := countEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count events after resume")
	require.Equal(t, eventCountAfterFailure, eventCountAfterResume,
		"Event count should remain 20 (no duplicates added)")
	logHighlight(t, "  ✓ Check 1 PASSED: Event count unchanged (%d events)", eventCountAfterResume)

	// Check 2: Same event IDs present (no new IDs added)
	eventIDsAfter, err := collectEventIDsForOffsetCommitTest(exportDir)
	require.NoError(t, err, "Failed to collect event_ids after resume")
	require.Equal(t, len(eventIDsBefore), len(eventIDsAfter),
		"Event ID count should be unchanged")
	logHighlight(t, "  ✓ Check 2 PASSED: Same %d unique event_ids", len(eventIDsAfter))

	// Check 3: All original event IDs still present
	for eventID := range eventIDsBefore {
		_, ok := eventIDsAfter[eventID]
		require.True(t, ok, "Event ID should still exist: %s", eventID)
	}
	verifyNoEventIDDuplicates(t, exportDir)
	logHighlight(t, "  ✓ Check 3 PASSED: All original event IDs preserved, no duplicates")

	// Check 4: Deduplication logs confirm skipped events
	dedupSkipsAfterResume, err := countDedupSkipLogs(exportDir)
	require.NoError(t, err, "Failed to count dedup skip logs")
	dedupSkipDelta := dedupSkipsAfterResume - dedupSkipsBeforeResume
	require.GreaterOrEqual(t, dedupSkipDelta, 20,
		"At least 20 replayed events should be skipped by dedup cache")
	logHighlight(t, "  ✓ Check 4 PASSED: Dedup cache skipped %d events (baseline: %d, after: %d)",
		dedupSkipDelta, dedupSkipsBeforeResume, dedupSkipsAfterResume)

	// ============================================================================
	logHighlight(t, "")
	logHighlight(t, "✅ ALL CHECKS PASSED - Durability guarantee verified!")
	logHighlight(t, "   • Events written but offsets NOT committed on failure")
	logHighlight(t, "   • Events replayed on resume")
	logHighlight(t, "   • Deduplication prevented duplicate writes")
	logHighlight(t, "")
	// ============================================================================

	// Cleanup
	_ = exportRunnerResume.Kill()
	_ = killDebeziumForExportDir(exportDir)
	_ = os.Remove(filepath.Join(exportDir, ".export-dataLockfile.lck"))
}

// setupOffsetCommitTestData creates test schema and inserts initial snapshot data
func setupOffsetCommitTestData(t *testing.T, container testcontainers.TestContainer) {
	container.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema_offset_commit CASCADE;",
		"CREATE SCHEMA test_schema_offset_commit;",
		`CREATE TABLE test_schema_offset_commit.cdc_offset_commit_test (
			id SERIAL PRIMARY KEY,
			name TEXT,
			value INTEGER,
			created_at TIMESTAMP DEFAULT NOW()
		);`,
		`ALTER TABLE test_schema_offset_commit.cdc_offset_commit_test REPLICA IDENTITY FULL;`,
	)

	// Insert 50 rows for snapshot phase
	container.ExecuteSqls(
		`INSERT INTO test_schema_offset_commit.cdc_offset_commit_test (name, value)
		SELECT 'snapshot_' || i, i * 10 FROM generate_series(1, 50) i;`,
	)

	t.Log("Offset commit test schema created with 50 snapshot rows")
}

// collectEventIDsForOffsetCommitTest extracts all event_ids from queue segment files
// Returns a set of unique event_ids for deduplication verification
func collectEventIDsForOffsetCommitTest(exportDir string) (map[string]struct{}, error) {
	queueDir := filepath.Join(exportDir, "data", "queue")
	files, err := filepath.Glob(filepath.Join(queueDir, "segment.*.ndjson"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return map[string]struct{}{}, nil
	}

	eventIDs := make(map[string]struct{})
	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			return nil, err
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || line == `\.` {
				continue
			}
			var eventData map[string]interface{}
			if err := json.Unmarshal([]byte(line), &eventData); err != nil {
				continue
			}
			eventIDRaw, ok := eventData["event_id"]
			if !ok || eventIDRaw == nil {
				continue
			}
			eventID, ok := eventIDRaw.(string)
			if !ok || eventID == "" || eventID == "null" {
				continue
			}
			eventIDs[eventID] = struct{}{}
		}
		if err := scanner.Err(); err != nil {
			f.Close()
			return nil, err
		}
		f.Close()
	}
	return eventIDs, nil
}

// readOffsetFileChecksum returns SHA256 hash of offset file for comparison
func readOffsetFileChecksum(exportDir string) string {
	offsetPath := filepath.Join(exportDir, "data", fmt.Sprintf("offsets.%s.dat", SOURCE_DB_EXPORTER_ROLE))
	data, err := os.ReadFile(offsetPath)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

// readOffsetFileContents returns raw contents of offset file
func readOffsetFileContents(exportDir string) string {
	offsetPath := filepath.Join(exportDir, "data", fmt.Sprintf("offsets.%s.dat", SOURCE_DB_EXPORTER_ROLE))
	data, err := os.ReadFile(offsetPath)
	if err != nil {
		return ""
	}
	return string(data)
}

// countDedupSkipLogs counts how many times deduplication skipped a replayed event
func countDedupSkipLogs(exportDir string) (int, error) {
	logPattern := filepath.Join(exportDir, "logs", "debezium-*.log")
	matches, err := filepath.Glob(logPattern)
	if err != nil {
		return 0, err
	}
	if len(matches) == 0 {
		return 0, fmt.Errorf("debezium log not found in %s", logPattern)
	}

	f, err := os.Open(matches[0])
	if err != nil {
		return 0, err
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "Skipping record") && strings.Contains(line, "event dedup cache") {
			count++
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return count, nil
}
