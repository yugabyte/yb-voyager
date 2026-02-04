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
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// TestSnapshotToCDCTransitionFailure validates recovery when failure occurs
// during snapshot->CDC transition.
func TestSnapshotToCDCTransitionFailure(t *testing.T) {
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

	setupTransitionFailureTestData(t, postgresContainer)
	defer postgresContainer.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema_transition CASCADE;",
	)

	t.Log("Running export with Go-side failure injection at snapshot->CDC transition...")
	failpointEnv := testutils.GetFailpointEnvVar(
		"github.com/yugabyte/yb-voyager/yb-voyager/cmd/snapshotToCDCTransitionError=1*return()",
	)
	exportRunner := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_transition",
		"--disable-pb", "true",
		"--yes",
	}, nil, true).WithEnv(failpointEnv)

	err = exportRunner.Run()
	require.NoError(t, err, "Failed to start export")

	failMarkerPath := filepath.Join(exportDir, "logs", "failpoint-snapshot-to-cdc.log")
	t.Log("Waiting for failpoint marker after injection...")
	matched, err := waitForMarkerFile(failMarkerPath, 60*time.Second, 2*time.Second)
	require.NoError(t, err, "Should be able to read failpoint marker")
	if !matched {
		_ = exportRunner.Kill()
		_ = killDebeziumForExportDir(exportDir)
		require.Fail(t, "Snapshot->CDC failpoint marker did not trigger")
	}
	t.Log("✓ Failpoint marker detected: snapshot->CDC transition failure injected")

	t.Log("Waiting for export process to exit after injection...")
	wasKilled, waitErr := waitForProcessExitOrKill(exportRunner, exportDir, 60*time.Second)
	if wasKilled {
		t.Log("Export did not exit after injection; process was killed")
	} else {
		require.Error(t, waitErr, "Export should exit with error after failpoint injection")
	}

	time.Sleep(3 * time.Second)

	t.Log("Verifying no CDC events were emitted before failure...")
	eventCountAfterFailure, err := countEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count CDC events after failure")
	require.Equal(t, 0, eventCountAfterFailure, "Expected 0 CDC events before resume")

	t.Log("Capturing snapshot descriptor before resume...")
	descriptorHashBefore, err := hashSnapshotDescriptor(exportDir)
	require.NoError(t, err, "Should be able to hash snapshot descriptor before resume")

	t.Log("Resuming export without failure injection...")
	cdcEventsGenerated := make(chan bool, 1)
	generateCDCEvents := func() {
		if err := waitForStreamingMode(exportDir, 2*time.Minute, 2*time.Second); err != nil {
			t.Logf("Failed waiting for streaming mode before CDC inserts: %v", err)
			cdcEventsGenerated <- true
			return
		}
		t.Logf("Generating CDC events (1 batch of 20 rows, %v between batches)...", batchSeparationWaitTime)
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema_transition.cdc_transition_test (name, value)
			SELECT 'batch1_' || i, 100 + i FROM generate_series(1, 20) i;`,
		)
		t.Logf("Batch 1 inserted, waiting %v for Debezium to process...", batchSeparationWaitTime)
		time.Sleep(batchSeparationWaitTime)
		cdcEventsGenerated <- true
	}

	exportRunnerResume := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_transition",
		"--disable-pb", "true",
		"--yes",
	}, generateCDCEvents, true)

	err = exportRunnerResume.Run()
	require.NoError(t, err, "Failed to start resumed export")
	defer exportRunnerResume.Kill()

	select {
	case <-cdcEventsGenerated:
		t.Log("CDC events generation completed")
	case <-time.After(60 * time.Second):
		require.Fail(t, "CDC event generation timed out")
	}

	finalEventCount := waitForCDCEventCount(t, exportDir, 20, 60*time.Second, 2*time.Second)
	require.Equal(t, 20, finalEventCount, "Expected 20 CDC events after recovery")

	verifyNoEventIDDuplicates(t, exportDir)

	t.Log("Verifying snapshot descriptor unchanged after resume...")
	descriptorHashAfter, err := hashSnapshotDescriptor(exportDir)
	require.NoError(t, err, "Should be able to hash snapshot descriptor after resume")
	require.Equal(t, descriptorHashBefore, descriptorHashAfter, "Snapshot descriptor should not change after resume")

	pgConn, err := postgresContainer.GetConnection()
	require.NoError(t, err, "Failed to get PostgreSQL connection")
	defer pgConn.Close()

	var sourceRowCount int
	err = pgConn.QueryRow("SELECT COUNT(*) FROM test_schema_transition.cdc_transition_test").Scan(&sourceRowCount)
	require.NoError(t, err, "Failed to query source row count")
	require.Equal(t, 50, sourceRowCount, "Source should have 30 snapshot + 20 CDC rows")

	t.Log("Snapshot->CDC transition failure test completed successfully")
}

func setupTransitionFailureTestData(t *testing.T, container testcontainers.TestContainer) {
	container.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema_transition CASCADE;",
		"CREATE SCHEMA test_schema_transition;",
		`CREATE TABLE test_schema_transition.cdc_transition_test (
			id SERIAL PRIMARY KEY,
			name TEXT,
			value INTEGER,
			created_at TIMESTAMP DEFAULT NOW()
		);`,
		`ALTER TABLE test_schema_transition.cdc_transition_test REPLICA IDENTITY FULL;`,
	)

	container.ExecuteSqls(
		`INSERT INTO test_schema_transition.cdc_transition_test (name, value)
		SELECT 'snapshot_' || i, i * 10 FROM generate_series(1, 30) i;`,
	)

	t.Log("Transition test schema created with 30 snapshot rows")
}

func waitForProcessExitOrKill(runner *testutils.VoyagerCommandRunner, exportDir string, timeout time.Duration) (bool, error) {
	errCh := make(chan error, 1)
	go func() {
		errCh <- runner.Wait()
	}()

	select {
	case err := <-errCh:
		return false, err
	case <-time.After(timeout):
		_ = runner.Kill()
		_ = killDebeziumForExportDir(exportDir)
		return true, nil
	}
}

func waitForMarkerFile(path string, timeout time.Duration, pollInterval time.Duration) (bool, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		matched, err := markerFilePresent(path)
		if err == nil && matched {
			return true, nil
		}
		time.Sleep(pollInterval)
	}
	return markerFilePresent(path)
}

func markerFilePresent(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return strings.Contains(string(data), "hit"), nil
}

func waitForStreamingMode(exportDir string, timeout time.Duration, pollInterval time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status, err := dbzm.ReadExportStatus(filepath.Join(exportDir, "data", "export_status.json"))
		if err == nil && status != nil && status.Mode == dbzm.MODE_STREAMING {
			return nil
		}
		time.Sleep(pollInterval)
	}
	return fmt.Errorf("timed out waiting for streaming mode")
}

func killDebeziumForExportDir(exportDir string) error {
	pidStr, err := dbzm.GetPIDOfDebeziumOnExportDir(exportDir, SOURCE_DB_EXPORTER_ROLE)
	if err != nil {
		return err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(pidStr))
	if err != nil {
		return err
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}

func hashSnapshotDescriptor(exportDir string) (string, error) {
	descriptorPath := filepath.Join(exportDir, datafile.DESCRIPTOR_PATH)
	data, err := os.ReadFile(descriptorPath)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}
