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
	multiFailurePollIntervalMs = 500
	multiFailureBatchWait      = time.Duration(multiFailurePollIntervalMs*5) * time.Millisecond // 2.5s
)

// TestCDCMultipleBatchFailures validates recovery across multiple consecutive batch failures.
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

	t.Log("CDC multiple batch failures test completed successfully")
}

func setupMultipleFailureTestData(t *testing.T, container testcontainers.TestContainer) {
	container.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema_multi_fail CASCADE;",
		"CREATE SCHEMA test_schema_multi_fail;",
		`CREATE TABLE test_schema_multi_fail.cdc_multi_fail_test (
			id SERIAL PRIMARY KEY,
			name TEXT,
			value INTEGER,
			created_at TIMESTAMP DEFAULT NOW()
		);`,
		`ALTER TABLE test_schema_multi_fail.cdc_multi_fail_test REPLICA IDENTITY FULL;`,
	)

	container.ExecuteSqls(
		`INSERT INTO test_schema_multi_fail.cdc_multi_fail_test (name, value)
		SELECT 'snapshot_' || i, i * 10 FROM generate_series(1, 50) i;`,
	)

	t.Log("Multiple failure test schema created with 50 snapshot rows")
}

func assertSourceRowCount(t *testing.T, container testcontainers.TestContainer, expected int) {
	t.Helper()
	pgConn, err := container.GetConnection()
	require.NoError(t, err, "Failed to get PostgreSQL connection")
	defer pgConn.Close()

	var sourceRowCount int
	err = pgConn.QueryRow("SELECT COUNT(*) FROM test_schema_multi_fail.cdc_multi_fail_test").Scan(&sourceRowCount)
	require.NoError(t, err, "Failed to query source row count")
	require.Equal(t, expected, sourceRowCount, "Source row count should match expected")
}
