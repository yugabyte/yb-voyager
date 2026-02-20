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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// TestCDCBatchProcessingFailure injects at YbExporterConsumer.handleBatch entry (2nd batch)
// to validate Byteman attach and batch failure behavior.
func TestCDCBatchProcessingFailure(t *testing.T) {
	ctx := context.Background()

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:    ContainerConfig{Type: "postgresql", ForLive: true},
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

	exportDir = lm.GetExportDir()
	postgresContainer := lm.GetSourceContainer()

	bytemanHelper, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err, "Failed to create Byteman helper")

	// Target YbExporterConsumer.handleBatch - fails on 2nd batch
	bytemanHelper.AddRuleFromBuilder(
		testutils.NewRule("fail_handle_batch").
			Class("io.debezium.server.ybexporter.YbExporterConsumer").
			Method("handleBatch").
			AtEntry().
			If("incrementCounter(\"batch\") == 2").
			ThrowException("java.lang.RuntimeException", "Simulated batch processing failure on batch 2"),
	)

	err = bytemanHelper.WriteRules()
	require.NoError(t, err, "Failed to write Byteman rules")

	testutils.LogTest(t, "Running CDC export with batch processing failure injection...")

	generateCDCEvents := func() {
		time.Sleep(10 * time.Second)
		testutils.LogTest(t, "Generating CDC events...")
		for batch := 0; batch < 5; batch++ {
			postgresContainer.ExecuteSqls(
				fmt.Sprintf(`INSERT INTO test_schema.cdc_test (name, value)
					SELECT 'batch%d_' || i, %d * 100 + i FROM generate_series(1, 50) i;`, batch, batch),
			)
			time.Sleep(2 * time.Second)
		}
		testutils.LogTest(t, "Finished generating CDC events")
	}

	// Run export data with Byteman injection - should fail on 2nd batch
	exportRunner := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema",
		"--disable-pb", "true",
		"--yes",
	}, generateCDCEvents, true).WithEnv(bytemanHelper.GetEnv()...) // async=true, with concurrent CDC event generation

	err = exportRunner.Run()
	require.NoError(t, err, "Failed to start export")
	defer exportRunner.Kill()

	matched, err := bytemanHelper.WaitForInjection("fail_handle_batch|Simulated batch processing failure", 60*time.Second)
	assert.True(t, matched, "Byteman injection should be logged in Debezium logs")
	assert.NoError(t, err, "Should be able to read debezium logs for verification")
}

// TestCDCBatchProcessing_WithMarkers injects at cdc("before-batch") to validate marker usage.
func TestCDCBatchProcessing_WithMarkers(t *testing.T) {
	ctx := context.Background()

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:    ContainerConfig{Type: "postgresql", ForLive: true},
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

	exportDir = lm.GetExportDir()
	postgresContainer := lm.GetSourceContainer()

	bytemanHelper, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err)

	// Target marker for batch processing - fails on 2nd batch
	// batch_count is a variable counter maintained by Byteman; counter name can be anything here
	bytemanHelper.AddRuleFromBuilder(
		testutils.NewRule("fail_during_batch").
			AtMarker(testutils.MarkerCDC, "before-batch").
			If("incrementCounter(\"batch_count\") == 2").
			ThrowException("java.lang.RuntimeException", "Simulated batch processing failure"),
	)

	err = bytemanHelper.WriteRules()
	require.NoError(t, err)

	testutils.LogTest(t, "Running CDC export with marker-based batch processing failure on 2nd batch...")

	generateCDCEvents := func() {
		time.Sleep(10 * time.Second)
		testutils.LogTest(t, "Generating CDC events...")
		for batch := 0; batch < 5; batch++ {
			postgresContainer.ExecuteSqls(
				fmt.Sprintf(`INSERT INTO test_schema.cdc_test (name, value)
					SELECT 'marker_batch%d_' || i, %d * 100 + i FROM generate_series(1, 50) i;`, batch, batch),
			)
			time.Sleep(2 * time.Second)
		}
		testutils.LogTest(t, "Finished generating CDC events")
	}

	exportRunner := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema",
		"--disable-pb", "true",
		"--yes",
	}, generateCDCEvents, true).WithEnv(bytemanHelper.GetEnv()...)

	err = exportRunner.Run()
	require.NoError(t, err, "Failed to start export")
	defer exportRunner.Kill()

	matched, err := bytemanHelper.WaitForInjection("fail_during_batch|Simulated batch processing failure", 60*time.Second)
	assert.True(t, matched, "Byteman marker injection should be logged")
	assert.NoError(t, err, "Should be able to read debezium logs for verification")
}
