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

	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// TestCDCBatchFailureBeforeHandleBatchComplete simulates a failure after records are written
// but before batch flush/sync (handleBatchComplete). The goal is to ensure replay recovers events.
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
			t.Logf("Failed to reach streaming mode: %v", err)
			return
		}
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema_before_batch_complete.cdc_before_batch_complete_test (name, value, payload)
			SELECT 'batch1_' || i, 100 + i, repeat('x', 20000) FROM generate_series(1, 20) i;`,
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
	logHighlight(t, "Queue count after failure: %d", eventCountAfterFailure)

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

	insertAfterResume := make(chan bool, 1)
	generateAfterResumeEvents := func() {
		if err := waitForStreamingMode(exportDir, 90*time.Second, 2*time.Second); err != nil {
			t.Logf("Failed to reach streaming mode after resume: %v", err)
			return
		}
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema_before_batch_complete.cdc_before_batch_complete_test (name, value, payload)
			SELECT 'resume_' || i, 200 + i, repeat('y', 20000) FROM generate_series(1, 10) i;`,
		)
		insertAfterResume <- true
	}

	exportRunnerResume := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_before_batch_complete",
		"--disable-pb", "true",
		"--yes",
	}, generateAfterResumeEvents, true).WithEnv(bytemanHelperResume.GetEnv()...)

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

func setupBeforeHandleBatchCompleteTestData(t *testing.T, container testcontainers.TestContainer) {
	container.ExecuteSqls(
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
	)

	container.ExecuteSqls(
		`INSERT INTO test_schema_before_batch_complete.cdc_before_batch_complete_test (name, value, payload)
		SELECT 'snapshot_' || i, i * 10, repeat('s', 20000) FROM generate_series(1, 50) i;`,
	)

	t.Log("Before-handleBatchComplete test schema created with 50 snapshot rows")
}
