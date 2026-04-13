//go:build failpoint_export

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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// TestCDCBatchProcessingFailure verifies that Byteman can inject a failure at
// Debezium's `YbExporterConsumer.handleBatch` entry on the 2nd batch invocation.
//
// Scenario:
//  1. Start `export data` (snapshot-and-changes mode) with 100 snapshot rows.
//  2. Generate CDC batches (50 rows each, 2s apart) to trigger multiple handleBatch calls.
//  3. Byteman injects a RuntimeException on the 2nd handleBatch call.
//  4. Verify the injection is logged in Debezium logs.
//
// This test validates:
// - Byteman attach and rule injection into Debezium JVM via method entry
//
// Injection point:
//   - Byteman rule on `YbExporterConsumer.handleBatch` entry (2nd invocation).
func TestCDCBatchProcessingFailure(t *testing.T) {
	if os.Getenv("BYTEMAN_JAR") == "" {
		t.Fatal("BYTEMAN_JAR environment variable not set. Install Byteman to run this test.")
	}
	ctx := context.Background()

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:    ContainerConfig{Type: "postgresql", ForLive: true, DatabaseName: "postgres"},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			"CREATE SCHEMA IF NOT EXISTS test_schema;",
			`CREATE TABLE test_schema.cdc_test (
				id SERIAL PRIMARY KEY,
				name TEXT,
				value INTEGER,
				created_at TIMESTAMP DEFAULT NOW()
			);`,
			`ALTER TABLE test_schema.cdc_test REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.cdc_test (name, value)
			SELECT 'initial_' || i, i * 10 FROM generate_series(1, 100) i;`,
		},
		CleanupSQL: []string{"DROP SCHEMA IF EXISTS test_schema CASCADE;"},
	})
	defer lm.Cleanup()
	require.NoError(t, lm.SetupContainers(ctx))
	require.NoError(t, lm.SetupSchema())

	exportDir := lm.GetCurrentExportDir()

	bytemanHelper, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err, "Failed to create Byteman helper")

	bytemanHelper.AddRuleFromBuilder(
		testutils.NewRule("fail_handle_batch").
			Class("io.debezium.server.ybexporter.YbExporterConsumer").
			Method("handleBatch").
			AtEntry().
			If("incrementCounter(\"batch\") == 2").
			ThrowException("java.lang.RuntimeException", "Simulated batch processing failure on batch 2"),
	)
	require.NoError(t, bytemanHelper.WriteRules())

	err = lm.StartExportDataWithEnv(true, nil, bytemanHelper.GetEnv())
	require.NoError(t, err, "Failed to start export")

	time.Sleep(10 * time.Second)
	for batch := 0; batch < 5; batch++ {
		lm.ExecuteOnSource(
			fmt.Sprintf(`INSERT INTO test_schema.cdc_test (name, value)
				SELECT 'batch%d_' || i, %d * 100 + i FROM generate_series(1, 50) i;`, batch, batch),
		)
		time.Sleep(2 * time.Second)
	}

	matched, err := bytemanHelper.WaitForInjection("fail_handle_batch|Simulated batch processing failure", 60*time.Second)
	assert.True(t, matched, "Byteman injection should be logged in Debezium logs")
	assert.NoError(t, err, "Should be able to read debezium logs for verification")
}

// TestCDCBatchProcessing_WithMarkers verifies that Byteman can inject a failure at
// a CDC marker checkpoint (`before-batch`) instead of a raw Java method entry.
//
// Scenario:
//  1. Start `export data` (snapshot-and-changes mode) with 100 snapshot rows.
//  2. Generate CDC batches (50 rows each, 2s apart) to trigger multiple marker hits.
//  3. Byteman fires at the before-batch-streaming marker on the 2nd invocation.
//  4. Verify the injection is logged in Debezium logs.
//
// This test validates:
// - Marker-based Byteman injection (cdc("before-batch") checkpoint)
//
// Injection point:
//   - Byteman rule on Debezium at the `cdc("before-batch")` marker (2nd invocation).
func TestCDCBatchProcessing_WithMarkers(t *testing.T) {
	if os.Getenv("BYTEMAN_JAR") == "" {
		t.Fatal("BYTEMAN_JAR environment variable not set. Install Byteman to run this test.")
	}
	ctx := context.Background()

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:    ContainerConfig{Type: "postgresql", ForLive: true, DatabaseName: "postgres"},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			"CREATE SCHEMA IF NOT EXISTS test_schema;",
			`CREATE TABLE test_schema.cdc_test (
				id SERIAL PRIMARY KEY,
				name TEXT,
				value INTEGER,
				created_at TIMESTAMP DEFAULT NOW()
			);`,
			`ALTER TABLE test_schema.cdc_test REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.cdc_test (name, value)
			SELECT 'initial_' || i, i * 10 FROM generate_series(1, 100) i;`,
		},
		CleanupSQL: []string{"DROP SCHEMA IF EXISTS test_schema CASCADE;"},
	})
	defer lm.Cleanup()
	require.NoError(t, lm.SetupContainers(ctx))
	require.NoError(t, lm.SetupSchema())

	exportDir := lm.GetCurrentExportDir()

	bytemanHelper, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err)

	bytemanHelper.AddRuleFromBuilder(
		testutils.NewRule("fail_during_batch").
			AtMarker(testutils.MarkerCDC, "before-batch").
			If("incrementCounter(\"batch_count\") == 2").
			ThrowException("java.lang.RuntimeException", "Simulated batch processing failure"),
	)
	require.NoError(t, bytemanHelper.WriteRules())

	err = lm.StartExportDataWithEnv(true, nil, bytemanHelper.GetEnv())
	require.NoError(t, err, "Failed to start export")

	time.Sleep(10 * time.Second)
	for batch := 0; batch < 5; batch++ {
		lm.ExecuteOnSource(
			fmt.Sprintf(`INSERT INTO test_schema.cdc_test (name, value)
				SELECT 'marker_batch%d_' || i, %d * 100 + i FROM generate_series(1, 50) i;`, batch, batch),
		)
		time.Sleep(2 * time.Second)
	}

	matched, err := bytemanHelper.WaitForInjection("fail_during_batch|Simulated batch processing failure", 60*time.Second)
	assert.True(t, matched, "Byteman marker injection should be logged")
	assert.NoError(t, err, "Should be able to read debezium logs for verification")
}
