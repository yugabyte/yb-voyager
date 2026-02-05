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

// TestCDCQueueWriteFailureAndResume injects a failure after many writes to ensure
// partial buffered writes don't get committed, and replay + dedup recovers events.
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
			t.Logf("Failed to reach streaming mode: %v", err)
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
	}, generateCDCEvents, true).WithEnv(append(bytemanHelper.GetEnv(), "YB_VOYAGER_SKIP_MARK_PROCESSED_FOR_SKIPPED=1")...)

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
	logHighlight(t, "Queue count after failure: %d", eventCountAfterFailure)

	_ = os.Remove(filepath.Join(exportDir, ".export-dataLockfile.lck"))

	exportRunnerResume := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_queue_write",
		"--disable-pb", "true",
		"--yes",
	}, nil, true).WithEnv("YB_VOYAGER_SKIP_MARK_PROCESSED_FOR_SKIPPED=1")

	err = exportRunnerResume.Run()
	require.NoError(t, err, "Failed to start export resume")

	eventCountAfterResumeStart, err := countEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count events after resume start")
	logHighlight(t, "Queue count after resume start: %d", eventCountAfterResumeStart)
	truncationMatched, err := waitForTruncationLog(exportDir, 60*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs for truncation")
	if truncationMatched {
		logHighlight(t, "✓ Observed queue segment truncation on resume")
	} else {
		logHighlight(t, "ℹ No truncation log observed on resume")
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

func setupQueueWriteFailureTestData(t *testing.T, container testcontainers.TestContainer) {
	container.ExecuteSqls(
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
	)

	container.ExecuteSqls(
		`INSERT INTO test_schema_queue_write.cdc_queue_write_test (name, value, payload)
		SELECT 'snapshot_' || i, i * 10, repeat('s', 20000) FROM generate_series(1, 50) i;`,
	)

	t.Log("Queue write failure test schema created with 50 snapshot rows")
}

func waitForTruncationLog(exportDir string, timeout time.Duration) (bool, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		matched, err := checkTruncationLog(exportDir)
		if err == nil && matched {
			return true, nil
		}
		time.Sleep(2 * time.Second)
	}
	return checkTruncationLog(exportDir)
}

func checkTruncationLog(exportDir string) (bool, error) {
	logPattern := filepath.Join(exportDir, "logs", "debezium-*.log")
	matches, err := filepath.Glob(logPattern)
	if err != nil {
		return false, err
	}
	if len(matches) == 0 {
		return false, nil
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		return false, err
	}
	return strings.Contains(string(data), "Truncating queue segment"), nil
}

func assertEventCountDoesNotExceed(t *testing.T, exportDir string, max int, timeout, pollInterval time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		count, err := countEventsInQueueSegments(exportDir)
		require.NoError(t, err, "Failed to count events while checking upper bound")
		if count > max {
			require.Fail(t, "Event count exceeded expected max", "count=%d max=%d", count, max)
		}
		time.Sleep(pollInterval)
	}
}
