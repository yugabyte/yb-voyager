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

// TestCDCOffsetCommitFailureAndResume verifies that a failed offset commit causes replay
// and deduplication prevents duplicate events from being added.
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

	exportRunner := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_offset_commit",
		"--disable-pb", "true",
		"--yes",
	}, generateCDCEvents, true).WithEnv(append(bytemanHelper.GetEnv(), "YB_VOYAGER_SKIP_MARK_PROCESSED_FOR_SKIPPED=1")...)

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
	logHighlight(t, "Offset file contents after failure: %q", offsetContents)
	require.Equal(t, "", strings.TrimSpace(offsetContents), "Offset file should be empty after failure")

	eventIDsBefore, err := collectEventIDsForOffsetCommitTest(exportDir)
	require.NoError(t, err, "Failed to read event_ids after failure")
	require.Len(t, eventIDsBefore, 20, "Expected 20 unique event_ids after failure")
	verifyNoEventIDDuplicates(t, exportDir)
	eventCountBeforeResume := eventCountAfterFailure
	logHighlight(t, "Queue count before resume: %d", eventCountBeforeResume)
	dedupSkipsBeforeResume, err := countDedupSkipLogs(exportDir)
	require.NoError(t, err, "Failed to count dedup skip logs before resume")
	logHighlight(t, "Dedup skip logs before resume: %d", dedupSkipsBeforeResume)

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
	}, nil, true).WithEnv(append(bytemanHelperResume.GetEnv(), "YB_VOYAGER_SKIP_MARK_PROCESSED_FOR_SKIPPED=1")...)

	err = exportRunnerResume.Run()
	require.NoError(t, err, "Failed to start export resume")

	replayMatched, err := bytemanHelperResume.WaitForInjection(">>> BYTEMAN: replay_batch", 90*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs for replay marker")
	require.True(t, replayMatched, "Expected replay batch after resume")

	eventCountAfterReplay, err := countEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count events after replay marker")
	logHighlight(t, "Queue count after replay marker: %d", eventCountAfterReplay)
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
	logHighlight(t, "Dedup skip logs after resume: %d", dedupSkipsAfterResume)
	require.GreaterOrEqual(t, dedupSkipsAfterResume-dedupSkipsBeforeResume, 20,
		"Expected dedup cache to skip at least 20 replayed records on resume")

	_ = exportRunnerResume.Kill()
	_ = killDebeziumForExportDir(exportDir)
	_ = os.Remove(filepath.Join(exportDir, ".export-dataLockfile.lck"))
}

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

	container.ExecuteSqls(
		`INSERT INTO test_schema_offset_commit.cdc_offset_commit_test (name, value)
		SELECT 'snapshot_' || i, i * 10 FROM generate_series(1, 50) i;`,
	)

	t.Log("Offset commit test schema created with 50 snapshot rows")
}

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

func readOffsetFileChecksum(exportDir string) string {
	offsetPath := filepath.Join(exportDir, "data", fmt.Sprintf("offsets.%s.dat", SOURCE_DB_EXPORTER_ROLE))
	data, err := os.ReadFile(offsetPath)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func readOffsetFileContents(exportDir string) string {
	offsetPath := filepath.Join(exportDir, "data", fmt.Sprintf("offsets.%s.dat", SOURCE_DB_EXPORTER_ROLE))
	data, err := os.ReadFile(offsetPath)
	if err != nil {
		return ""
	}
	return string(data)
}

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
