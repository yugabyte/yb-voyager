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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// TestSnapshotFailureAndResume validates recovery when pg_dump snapshot fails.
func TestSnapshotFailureAndResume(t *testing.T) {
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

	setupSnapshotFailureTestData(t, postgresContainer)
	defer postgresContainer.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema_snapshot_fail CASCADE;",
	)

	failpointEnv := testutils.GetFailpointEnvVar(
		"github.com/yugabyte/yb-voyager/yb-voyager/cmd/pgDumpSnapshotFailure=1*return()",
	)
	cdcEventsGenerated := make(chan bool, 1)
	generateCDCEvents := func() {
		// Insert CDC rows while snapshot is in progress (pg_dump running)
		time.Sleep(3 * time.Second)
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema_snapshot_fail.cdc_snapshot_fail_test (name, value)
			SELECT 'cdc_' || i, 1000 + i FROM generate_series(1, 20) i;`,
		)
		t.Log("CDC rows inserted during snapshot phase")
		cdcEventsGenerated <- true
	}

	exportRunner := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_snapshot_fail",
		"--disable-pb", "true",
		"--yes",
	}, generateCDCEvents, true).WithEnv(failpointEnv, "YB_VOYAGER_PGDUMP_FAIL_DELAY_MS=8000")

	err = exportRunner.Run()
	require.NoError(t, err, "Failed to start export")

	err = exportRunner.Wait()
	require.Error(t, err, "Export should fail due to pg_dump snapshot failpoint")

	failMarkerPath := filepath.Join(exportDir, "logs", "failpoint-pg-dump-snapshot.log")
	matched, err := waitForMarkerFile(failMarkerPath, 60*time.Second, 2*time.Second)
	require.NoError(t, err, "Should be able to read snapshot failure marker")
	require.True(t, matched, "Snapshot failure marker did not trigger")

	descriptorPath := filepath.Join(exportDir, datafile.DESCRIPTOR_PATH)
	_, err = os.Stat(descriptorPath)
	require.Error(t, err, "Snapshot descriptor should not exist after failed snapshot")

	t.Log("Verifying CDC events inserted before failure are accounted for...")
	eventCountAfterFailure, err := countEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count CDC events after failure")
	if eventCountAfterFailure > 0 {
		t.Logf("CDC events captured before failure: %d", eventCountAfterFailure)
	}

	t.Log("Resuming export without failure injection...")
	exportRunnerResume := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_snapshot_fail",
		"--disable-pb", "true",
		"--yes",
	}, nil, true)

	err = exportRunnerResume.Run()
	require.NoError(t, err, "Failed to start resumed export")
	defer exportRunnerResume.Kill()

	select {
	case <-cdcEventsGenerated:
	case <-time.After(30 * time.Second):
		require.Fail(t, "CDC event generation timed out")
	}

	descriptorHash, err := waitForSnapshotDescriptorHashSnapshotFailure(exportDir, 120*time.Second, 2*time.Second)
	require.NoError(t, err, "Snapshot descriptor should be created after resume")
	require.NotEmpty(t, descriptorHash, "Snapshot descriptor hash should not be empty")

	t.Log("Inserting CDC rows after snapshot completion to validate CDC export...")
	postgresContainer.ExecuteSqls(
		`INSERT INTO test_schema_snapshot_fail.cdc_snapshot_fail_test (name, value)
		SELECT 'cdc_after_' || i, 2000 + i FROM generate_series(1, 10) i;`,
	)

	snapshotRowCount, err := getSnapshotRowCountForTable(exportDir, "cdc_snapshot_fail_test")
	require.NoError(t, err, "Failed to get snapshot row count from descriptor")
	require.Equal(t, int64(120), snapshotRowCount, "Snapshot should include initial + during-snapshot rows")

	cdcEventCount := waitForCDCEventCount(t, exportDir, 10, 60*time.Second, 2*time.Second)

	pgConn, err := postgresContainer.GetConnection()
	require.NoError(t, err, "Failed to get PostgreSQL connection")
	defer pgConn.Close()

	var sourceRowCount int
	err = pgConn.QueryRow("SELECT COUNT(*) FROM test_schema_snapshot_fail.cdc_snapshot_fail_test").Scan(&sourceRowCount)
	require.NoError(t, err, "Failed to query source row count")
	require.Equal(t, 130, sourceRowCount, "Source should have snapshot + CDC rows after resume")
	require.Equal(t, sourceRowCount, int(snapshotRowCount)+cdcEventCount,
		"Snapshot rows + CDC events should equal source row count")

	t.Log("Snapshot failure and resume test completed successfully")
}

func setupSnapshotFailureTestData(t *testing.T, container testcontainers.TestContainer) {
	container.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema_snapshot_fail CASCADE;",
		"CREATE SCHEMA test_schema_snapshot_fail;",
		`CREATE TABLE test_schema_snapshot_fail.cdc_snapshot_fail_test (
			id SERIAL PRIMARY KEY,
			name TEXT,
			value INTEGER,
			created_at TIMESTAMP DEFAULT NOW()
		);`,
		`ALTER TABLE test_schema_snapshot_fail.cdc_snapshot_fail_test REPLICA IDENTITY FULL;`,
	)

	container.ExecuteSqls(
		`INSERT INTO test_schema_snapshot_fail.cdc_snapshot_fail_test (name, value)
		SELECT 'snapshot_' || i, i * 10 FROM generate_series(1, 100) i;`,
	)

	t.Log("Snapshot failure test schema created with 100 snapshot rows")
}

func waitForSnapshotDescriptorHashSnapshotFailure(exportDir string, timeout time.Duration, pollInterval time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		hash, err := hashSnapshotDescriptorSnapshotFailure(exportDir)
		if err == nil && hash != "" {
			return hash, nil
		}
		time.Sleep(pollInterval)
	}
	return hashSnapshotDescriptorSnapshotFailure(exportDir)
}

func hashSnapshotDescriptorSnapshotFailure(exportDir string) (string, error) {
	descriptorPath := filepath.Join(exportDir, datafile.DESCRIPTOR_PATH)
	data, err := os.ReadFile(descriptorPath)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}

func getSnapshotRowCountForTable(exportDir string, tableName string) (int64, error) {
	descriptor := datafile.OpenDescriptor(exportDir)
	if descriptor == nil || descriptor.DataFileList == nil {
		return 0, fmt.Errorf("data file descriptor not found")
	}
	var total int64
	for _, entry := range descriptor.DataFileList {
		if strings.Contains(entry.TableName, tableName) {
			total += entry.RowCount
		}
	}
	return total, nil
}
