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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// setupCDCTestData creates test schema and data for CDC testing
func setupCDCTestData(t *testing.T, container testcontainers.TestContainer) {
	container.ExecuteSqls(
		"CREATE SCHEMA IF NOT EXISTS test_schema;",
		`CREATE TABLE test_schema.cdc_test (
			id SERIAL PRIMARY KEY,
			name TEXT,
			value INTEGER,
			created_at TIMESTAMP DEFAULT NOW()
		);`,
		`INSERT INTO test_schema.cdc_test (name, value)
		SELECT 'initial_' || i, i * 10 FROM generate_series(1, 100) i;`,
	)
}

// TestCDCReplicationSlotFailure_WithoutMarkers demonstrates testing CDC failures
// by targeting existing Java methods without modifying Debezium code.
// This test injects a failure into PostgreSQL replication slot creation.
func TestCDCReplicationSlotFailure_WithoutMarkers(t *testing.T) {
	ctx := context.Background()

	// Create temporary export directory
	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	// Start PostgreSQL container with logical replication enabled
	postgresContainer := testcontainers.NewTestContainer("postgresql", nil)
	err := postgresContainer.Start(ctx)
	require.NoError(t, err, "Failed to start PostgreSQL container")
	defer postgresContainer.Stop(ctx)

	// Create test schema and data
	setupCDCTestData(t, postgresContainer)

	// Setup Byteman for replication slot creation failure
	bytemanHelper, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err, "Failed to create Byteman helper")

	// Target existing PostgreSQL replication method (no markers needed in code)
	bytemanHelper.AddRuleFromBuilder(
		testutils.NewRule("fail_replication_slot_creation").
			Class("org.postgresql.replication.PGReplicationConnection").
			Method("createReplicationSlot").
			AtEntry().
			ThrowException("org.postgresql.util.PSQLException", "Replication slot already exists"),
	)

	// Write rules to file
	err = bytemanHelper.WriteRules()
	require.NoError(t, err, "Failed to write Byteman rules")

	t.Log("Running CDC export with replication slot creation failure...")

	// Run export data with Byteman injection (should fail)
	exportRunner := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "changes-only",
		"--source-db-schema", "test_schema",
		"--disable-pb", "true",
		"--yes",
	}, nil, false).WithEnvMap(bytemanHelper.GetEnv())

	err = exportRunner.Run()
	assert.Error(t, err, "Expected export to fail due to replication slot creation failure")

	// Verify Byteman injection occurred
	stderr := exportRunner.Stderr()
	assert.Contains(t, stderr, "Replication slot", "Error should mention replication slot")

	t.Logf("✓ Replication slot creation failure correctly detected")
	t.Logf("✓ Test demonstrates targeting existing Java methods without code changes")
}

// TestCDCBatchProcessing_WithMarkers demonstrates testing CDC with marker-based injection.
// This test targets BytemanMarkers added to the Debezium code for stable injection points.
func TestCDCBatchProcessing_WithMarkers(t *testing.T) {
	ctx := context.Background()

	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	postgresContainer := testcontainers.NewTestContainer("postgresql", nil)
	err := postgresContainer.Start(ctx)
	require.NoError(t, err)
	defer postgresContainer.Stop(ctx)

	setupCDCTestData(t, postgresContainer)

	// Setup Byteman to fail during batch processing (using markers)
	bytemanHelper, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err)

	// Target marker for batch processing - stable and self-documenting
	bytemanHelper.AddRuleFromBuilder(
		testutils.NewRule("fail_during_batch").
			AtCDCMarker("before-batch").
			If("incrementCounter(\"batch_count\") == 2").
			ThrowException("java.lang.RuntimeException", "Simulated batch processing failure"),
	)

	err = bytemanHelper.WriteRules()
	require.NoError(t, err)

	t.Log("Running export with batch processing failure on 2nd batch...")

	// Run export - should fail on second batch
	exportRunner := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-only",
		"--source-db-schema", "test_schema",
		"--disable-pb", "true",
		"--yes",
	}, nil, false).WithEnvMap(bytemanHelper.GetEnv())

	err = exportRunner.Run()
	assert.Error(t, err, "Expected export to fail on 2nd batch")

	// Verify injection happened at the marker
	matched, _ := bytemanHelper.VerifyInjection(">>> BYTEMAN.*fail_during_batch")
	assert.True(t, matched, "Byteman marker injection should be logged")

	t.Logf("✓ Batch processing failure injected at marker checkpoint")
	t.Logf("✓ Test demonstrates marker-based stable injection points")
}

// TestCDCSnapshotTransition_Combined demonstrates combining both approaches:
// using markers where available and targeting existing methods where needed.
func TestCDCSnapshotTransition_Combined(t *testing.T) {
	ctx := context.Background()

	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	postgresContainer := testcontainers.NewTestContainer("postgresql", nil)
	err := postgresContainer.Start(ctx)
	require.NoError(t, err)
	defer postgresContainer.Stop(ctx)

	// Create larger dataset to ensure multiple batches
	postgresContainer.ExecuteSqls(
		"CREATE SCHEMA test_schema;",
		`CREATE TABLE test_schema.snapshot_test (
			id SERIAL PRIMARY KEY,
			data TEXT
		);`,
		`INSERT INTO test_schema.snapshot_test (data)
		SELECT 'row_' || i FROM generate_series(1, 1000) i;`,
	)

	bytemanHelper, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err)

	// Rule 1: Use marker for snapshot completion detection
	bytemanHelper.AddRuleFromBuilder(
		testutils.NewRule("trace_snapshot_complete").
			AtSnapshotMarker("detected-complete").
			Do(`traceln(">>> BYTEMAN: Snapshot completion detected via marker")`),
	)

	// Rule 2: Use existing method for lower-level failure injection
	bytemanHelper.AddRuleFromBuilder(
		testutils.NewRule("delay_snapshot_read").
			Class("java.sql.ResultSet").
			Method("next").
			AtEntry().
			If("incrementCounter(\"rows\") == 500").
			Delay(2), // 2 second delay at row 500
	)

	err = bytemanHelper.WriteRules()
	require.NoError(t, err)

	t.Log("Running snapshot export with combined marker and method targeting...")

	exportRunner := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-only",
		"--source-db-schema", "test_schema",
		"--disable-pb", "true",
		"--yes",
	}, nil, false).WithEnvMap(bytemanHelper.GetEnv())

	err = exportRunner.Run()
	// Should succeed despite the delay
	assert.NoError(t, err, "Export should succeed after delay")

	// Verify both injections occurred
	markerMatched, _ := bytemanHelper.VerifyInjection("Snapshot completion detected via marker")
	assert.True(t, markerMatched, "Marker-based injection should be logged")

	delayMatched, _ := bytemanHelper.VerifyInjection("delay_snapshot_read")
	assert.True(t, delayMatched, "Method-based delay should be logged")

	t.Logf("✓ Both marker-based and method-based injections worked together")
	t.Logf("✓ Test demonstrates hybrid approach for maximum flexibility")
}
