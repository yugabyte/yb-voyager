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

// TestBytemanProbeCDCMarkers verifies which CDC Byteman markers fire without injecting failures.
func TestBytemanProbeCDCMarkers(t *testing.T) {
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

	setupBytemanProbeTestData(t, postgresContainer)
	defer postgresContainer.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema_byteman_probe CASCADE;",
	)

	bytemanHelper, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err, "Failed to create Byteman helper")

	markers := []string{
		"after-connect",
		"before-batch",
		"before-batch-streaming",
		"before-handle-batch-complete",
		"before-write-record",
		"before-offset-commit",
		"skip-record",
		"skip-record-end",
	}

	for _, marker := range markers {
		markerType := testutils.MarkerCDC
		if marker == "after-connect" {
			markerType = testutils.MarkerCheckpoint
		}
		bytemanHelper.AddRuleFromBuilder(
			testutils.NewRule("probe_"+strings.ReplaceAll(marker, "-", "_")).
				AtMarker(markerType, marker).
				Do(fmt.Sprintf(`traceln(">>> BYTEMAN: probe %s");`, marker)),
		)
	}

	err = bytemanHelper.WriteRules()
	require.NoError(t, err, "Failed to write Byteman rules")

	errCh := make(chan error, 1)
	generateCDCEvents := func() {
		if err := waitForStreamingMode(exportDir, 90*time.Second, 2*time.Second); err != nil {
			errCh <- err
			return
		}
		postgresContainer.ExecuteSqls(
			`INSERT INTO test_schema_byteman_probe.cdc_byteman_probe_test (name, value)
			SELECT 'batch1_' || i, 100 + i FROM generate_series(1, 20) i;`,
		)
		errCh <- nil
	}

	exportRunner := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_byteman_probe",
		"--disable-pb", "true",
		"--yes",
	}, generateCDCEvents, true).WithEnv(bytemanHelper.GetEnv()...)

	err = exportRunner.Run()
	require.NoError(t, err, "Failed to start export")

	select {
	case err := <-errCh:
		require.NoError(t, err, "CDC event generation failed")
	case <-time.After(120 * time.Second):
		require.Fail(t, "CDC event generation timed out")
	}

	waitForCDCEventCount(t, exportDir, 20, 60*time.Second, 2*time.Second)

	// after-connect can fire before the Byteman agent attaches, so treat it as informational.
	matchedCount := 0
	for _, marker := range markers {
		pattern := fmt.Sprintf(">>> BYTEMAN: probe %s", marker)
		matched, err := bytemanHelper.WaitForInjection(pattern, 60*time.Second)
		require.NoError(t, err, "Should be able to read debezium logs for %s", marker)
		if matched {
			matchedCount++
			t.Logf("✓ Byteman marker fired: %s", marker)
		} else if marker == "after-connect" {
			t.Log("ℹ Byteman marker did not fire: after-connect (likely before agent attach)")
		} else {
			t.Logf("✗ Byteman marker did not fire: %s", marker)
		}
	}
	require.Greater(t, matchedCount, 0, "No Byteman markers fired; agent may not have attached")

	_ = exportRunner.Kill()
	_ = killDebeziumForExportDir(exportDir)
	_ = os.Remove(filepath.Join(exportDir, ".export-dataLockfile.lck"))
}

func setupBytemanProbeTestData(t *testing.T, container testcontainers.TestContainer) {
	container.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema_byteman_probe CASCADE;",
		"CREATE SCHEMA test_schema_byteman_probe;",
		`CREATE TABLE test_schema_byteman_probe.cdc_byteman_probe_test (
			id SERIAL PRIMARY KEY,
			name TEXT,
			value INTEGER,
			created_at TIMESTAMP DEFAULT NOW()
		);`,
		`ALTER TABLE test_schema_byteman_probe.cdc_byteman_probe_test REPLICA IDENTITY FULL;`,
	)

	container.ExecuteSqls(
		`INSERT INTO test_schema_byteman_probe.cdc_byteman_probe_test (name, value)
		SELECT 'snapshot_' || i, i * 10 FROM generate_series(1, 50) i;`,
	)

	t.Log("Byteman probe test schema created with 50 snapshot rows")
}
