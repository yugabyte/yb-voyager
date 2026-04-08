//go:build failpoint_export_cdc

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
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

const (
	// Debezium PostgreSQL connector timing configuration
	//
	// Verification: yb-voyager/src/dbzm/config.go lines 127-139 (postgresSrcConfigTemplate)
	// - PostgreSQL config does NOT explicitly set: poll.interval.ms, max.batch.size, max.queue.size
	// - Only sets: offset.flush.interval.ms=0 (immediate flush, line 97)
	// - Therefore uses Debezium defaults: 500ms poll interval, 2048 max batch, 8192 queue
	//
	// Test batching strategy:
	// - Insert 20 rows per batch (well under 2048 limit)
	// - Wait 5x poll interval (2.5s) between INSERTs
	// - This gives Debezium time to: poll → process → commit offset → start next batch
	//
	debeziumDefaultPollIntervalMs = 500                                                               // Default poll interval
	batchSeparationWaitTime       = time.Duration(debeziumDefaultPollIntervalMs*5) * time.Millisecond // 2.5 seconds
)

// TestCDCBatchFailureAndResume verifies that live migration `export data` can resume after
// a mid-batch failure during CDC streaming with mixed INSERT/UPDATE/DELETE operations,
// and that `import data` correctly applies all events to the target with no anomalies.
//
// Scenario:
//  1. Start `export data` (snapshot-and-changes mode) with 100 snapshot rows.
//  2. Start `import data` concurrently (imports snapshot + streams CDC to target).
//  3. Generate 3 CDC batches (20 events each: 10 INSERTs + 5 UPDATEs + 5 DELETEs).
//  4. Inject failure on 2nd CDC batch via before-batch-streaming marker.
//  5. Export crashes after batch 1 committed; batch 2 and 3 are lost.
//  6. Resume `export data` without failure injection.
//  7. Verify all 60 CDC events recovered with no duplicates via event_id dedup.
//  8. Verify import consumed all CDC events and source == target row counts match.
//
// This test validates:
// - Batch processing failure recovery with mixed operation types
// - CDC offset replay (batch 2 and 3 replayed from offsets)
// - Event deduplication (batch 1 events already written, not duplicated on resume)
// - End-to-end: import data correctly applies recovered events to the target
//
// Injection point:
//   - Byteman rule on Debezium's `YbExporterConsumer.handleBatch` (2nd invocation),
//     triggered via before-batch-streaming marker file.
func TestCDCBatchFailureAndResume(t *testing.T) {
	if os.Getenv("BYTEMAN_JAR") == "" {
		t.Skip("Skipping test: BYTEMAN_JAR environment variable not set. Install Byteman to run this test.")
	}

	ctx := context.Background()

	tableName := "test_schema.cdc_test"

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:    ContainerConfig{Type: "postgresql", ForLive: true, DatabaseName: "postgres"},
		TargetDB:    ContainerConfig{Type: "yugabytedb", DatabaseName: "test_batch_failure"},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			"CREATE SCHEMA IF NOT EXISTS test_schema;",
			`CREATE TABLE test_schema.cdc_test (
				id SERIAL PRIMARY KEY,
				name TEXT,
				value INTEGER,
				created_at TIMESTAMP DEFAULT NOW()
			);`,
		},
		SourceSetupSchemaSQL: []string{
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
		testutils.NewRule("fail_cdc_batch_2").
			AtMarker(testutils.MarkerCDC, "before-batch-streaming").
			If("incrementCounter(\"cdc_batch\") == 2").
			ThrowException("java.lang.RuntimeException", "TEST: Simulated batch processing failure on batch 2"),
	)
	require.NoError(t, bytemanHelper.WriteRules())

	// Run 1: export with Byteman injection - should fail on 2nd CDC batch
	err = lm.StartExportDataWithEnv(true, nil, bytemanHelper.GetEnv())
	require.NoError(t, err, "Failed to start export")

	err = lm.StartImportData(true, nil)
	require.NoError(t, err, "Failed to start import data")

	time.Sleep(10 * time.Second)

	for batch := 1; batch <= 3; batch++ {
		updateStart := (batch-1)*5 + 1
		deleteStart := (batch-1)*5 + 51
		lm.ExecuteOnSource(
			fmt.Sprintf(`INSERT INTO test_schema.cdc_test (name, value)
			SELECT 'batch%d_ins_' || i, %d + i FROM generate_series(1, 10) i;`, batch, batch*100),
		)
		lm.ExecuteOnSource(
			fmt.Sprintf(`UPDATE test_schema.cdc_test SET name = 'batch%d_upd_' || id, value = value + 1000
			WHERE id BETWEEN %d AND %d;`, batch, updateStart, updateStart+4),
		)
		lm.ExecuteOnSource(
			fmt.Sprintf(`DELETE FROM test_schema.cdc_test WHERE id BETWEEN %d AND %d;`, deleteStart, deleteStart+4),
		)
		time.Sleep(batchSeparationWaitTime)
	}

	matched, err := bytemanHelper.WaitForInjection(">>> BYTEMAN: fail_cdc_batch_2", 90*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs")
	require.True(t, matched, "Byteman injection should have occurred and been logged")

	err = lm.WaitForExportDataExitTimeout(120 * time.Second)
	require.Error(t, err, "Export should exit with error after Byteman injection")

	time.Sleep(3 * time.Second)

	eventCountAfterFailure, err := countEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count CDC events after failure")
	require.Equal(t, 20, eventCountAfterFailure, "Should have exactly 20 events (batch 1) before failure")

	verifyNoEventIDDuplicates(t, exportDir)

	// Run 2: resume export without Byteman
	err = lm.StartExportData(true, nil)
	require.NoError(t, err, "Failed to start resumed export")

	finalEventCount := lm.WaitForCDCEventCount(t, 60, 120*time.Second, 5*time.Second)
	require.Equal(t, 60, finalEventCount, "Expected 60 CDC events after resume")

	verifyNoEventIDDuplicates(t, exportDir)

	// Validate: import (started alongside run 1) should have snapshot + all CDC events
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		reportTableName(tableName): {Inserts: 30, Updates: 15, Deletes: 15},
	}, 120, 5)
	require.NoError(t, err, "Forward streaming did not complete")

	err = lm.ValidateRowCount([]string{tableName})
	require.NoError(t, err, "Source and target row counts don't match after batch failure recovery")
}

// TestFirstCDCBatchFailure verifies that live migration `export data` can resume after
// the very first CDC batch fails before any offsets are committed, and that `import data`
// correctly applies all events to the target with no anomalies.
//
// Scenario:
//  1. Start `export data` (snapshot-and-changes mode) with 50 snapshot rows.
//  2. Start `import data` concurrently (imports snapshot + streams CDC to target).
//  3. Generate 3 CDC batches (20 rows each, 60 total events).
//  4. Inject failure on 1st CDC batch via before-batch-streaming marker.
//  5. Export crashes before any CDC offsets are committed (0 events written).
//  6. Resume `export data` without failure injection.
//  7. Verify all 60 CDC events recovered via full replay from zero offset state.
//  8. Verify import consumed all CDC events and source == target row counts match.
//
// This test validates:
// - "Cold start" CDC recovery (no offsets file exists)
// - Full CDC replay capability from the beginning
// - End-to-end: import data correctly applies recovered events to the target
//
// Injection point:
//   - Byteman rule on Debezium's `YbExporterConsumer.handleBatch` (1st invocation),
//     triggered via before-batch-streaming marker file.

func TestFirstCDCBatchFailure(t *testing.T) {
	if os.Getenv("BYTEMAN_JAR") == "" {
		t.Skip("Skipping test: BYTEMAN_JAR environment variable not set. Install Byteman to run this test.")
	}

	ctx := context.Background()

	tableName := "test_schema.first_batch_test"

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:    ContainerConfig{Type: "postgresql", ForLive: true, DatabaseName: "postgres"},
		TargetDB:    ContainerConfig{Type: "yugabytedb", DatabaseName: "test_first_batch"},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			"CREATE SCHEMA IF NOT EXISTS test_schema;",
			`CREATE TABLE test_schema.first_batch_test (
				id SERIAL PRIMARY KEY,
				name TEXT,
				value INTEGER,
				created_at TIMESTAMP DEFAULT NOW()
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.first_batch_test REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.first_batch_test (name, value)
			SELECT 'initial_' || i, i * 10 FROM generate_series(1, 50) i;`,
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
		testutils.NewRule("fail_first_cdc_batch").
			AtMarker(testutils.MarkerCDC, "before-batch-streaming").
			If("incrementCounter(\"first_batch_counter\") == 1").
			ThrowException("java.lang.RuntimeException", "TEST: Simulated failure on first CDC batch"),
	)
	require.NoError(t, bytemanHelper.WriteRules())

	// Run 1: export with Byteman injection - should fail on 1st CDC batch
	err = lm.StartExportDataWithEnv(true, nil, bytemanHelper.GetEnv())
	require.NoError(t, err, "Failed to start export")

	err = lm.StartImportData(true, nil)
	require.NoError(t, err, "Failed to start import data")

	time.Sleep(10 * time.Second)

	for batch := 1; batch <= 3; batch++ {
		lm.ExecuteOnSource(
			fmt.Sprintf(`INSERT INTO test_schema.first_batch_test (name, value)
			SELECT 'batch%d_' || i, %d + i FROM generate_series(1, 20) i;`, batch, batch*100),
		)
		time.Sleep(batchSeparationWaitTime)
	}

	matched, err := bytemanHelper.WaitForInjection(">>> BYTEMAN: fail_first_cdc_batch", 90*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs")
	require.True(t, matched, "Byteman injection should have occurred and been logged")

	err = lm.WaitForExportDataExitTimeout(120 * time.Second)
	require.Error(t, err, "Export should exit with error after Byteman injection")

	time.Sleep(3 * time.Second)

	eventCount1, err := countEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count events after first export")
	require.Equal(t, 0, eventCount1, "Should have 0 events (first batch failed at entry, no CDC state yet)")

	// Run 2: resume export without Byteman
	err = lm.StartExportData(true, nil)
	require.NoError(t, err, "Failed to start resumed export")

	finalEventCount := lm.WaitForCDCEventCount(t, 60, 120*time.Second, 5*time.Second)
	require.Equal(t, 60, finalEventCount, "Expected 60 CDC events after resume")

	verifyNoEventIDDuplicates(t, exportDir)

	// Validate: import (started alongside run 1) should have snapshot + all CDC events
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		reportTableName(tableName): {Inserts: 60},
	}, 120, 5)
	require.NoError(t, err, "Forward streaming did not complete")

	err = lm.ValidateRowCount([]string{tableName})
	require.NoError(t, err, "Source and target row counts don't match after first batch failure recovery")
}

// TestCDCMultipleBatchFailures verifies that live migration `export data` can resume correctly
// across multiple consecutive batch failures, and that `import data` correctly applies all
// events to the target with no anomalies.
//
// Scenario:
//  1. Start `export data` (snapshot-and-changes mode) with 50 snapshot rows.
//  2. Start `import data` concurrently (imports snapshot + streams CDC to target).
//  3. Run 1: Insert batch1 (20 rows), batch2 (20 rows) -> fail on 2nd batch -> 20 events written.
//  4. Run 2 (resume): Insert batch3 (20 rows) -> fail on 2nd batch again -> 40 events total.
//  5. Run 3 (resume): No failure -> all batches replay -> 60 events total with no duplicates.
//  6. Verify import consumed all CDC events and source == target row counts match.
//
// This test validates:
// - Recovery across multiple consecutive failures
// - Incremental progress after each failed run
// - Final full recovery with deduplication
// - End-to-end: import data correctly applies recovered events to the target
//
// Injection point:
//   - Byteman rule on Debezium's `YbExporterConsumer.handleBatch` (2nd invocation per run),
//     triggered via before-batch-streaming marker file.
func TestCDCMultipleBatchFailures(t *testing.T) {
	if os.Getenv("BYTEMAN_JAR") == "" {
		t.Skip("Skipping test: BYTEMAN_JAR environment variable not set. Install Byteman to run this test.")
	}

	ctx := context.Background()
	tableName := "test_schema_multi_fail.cdc_multi_fail_test"

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:    ContainerConfig{Type: "postgresql", ForLive: true, DatabaseName: "postgres"},
		TargetDB:    ContainerConfig{Type: "yugabytedb", DatabaseName: "test_multi_fail"},
		SchemaNames: []string{"test_schema_multi_fail"},
		SchemaSQL: []string{
			"DROP SCHEMA IF EXISTS test_schema_multi_fail CASCADE;",
			"CREATE SCHEMA test_schema_multi_fail;",
			fmt.Sprintf(`CREATE TABLE %s (
				id SERIAL PRIMARY KEY,
				name TEXT,
				value INTEGER,
				created_at TIMESTAMP DEFAULT NOW()
			);`, tableName),
		},
		SourceSetupSchemaSQL: []string{
			fmt.Sprintf(`ALTER TABLE %s REPLICA IDENTITY FULL;`, tableName),
		},
		InitialDataSQL: []string{
			fmt.Sprintf(`INSERT INTO %s (name, value)
			SELECT 'snapshot_' || i, i * 10 FROM generate_series(1, 50) i;`, tableName),
		},
		CleanupSQL: []string{"DROP SCHEMA IF EXISTS test_schema_multi_fail CASCADE;"},
	})
	defer lm.Cleanup()
	require.NoError(t, lm.SetupContainers(ctx))
	require.NoError(t, lm.SetupSchema())

	exportDir := lm.GetCurrentExportDir()

	assertRowCount := func(expected int) {
		err := lm.WithSourceConn(func(db *sql.DB) error {
			return testutils.AssertRowCount(ctx, db, tableName, expected)
		})
		require.NoError(t, err)
	}

	// Run 1: fail on 2nd streaming batch
	bytemanHelperRun1, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err, "Failed to create Byteman helper (run 1)")
	bytemanHelperRun1.AddRuleFromBuilder(
		testutils.NewRule("fail_cdc_batch_run1").
			AtMarker(testutils.MarkerCDC, "before-batch-streaming").
			If("incrementCounter(\"cdc_batch\") == 2").
			ThrowException("java.lang.RuntimeException", "TEST: Simulated batch failure on run 1"),
	)
	require.NoError(t, bytemanHelperRun1.WriteRules())

	err = lm.StartExportDataWithEnv(true, nil, bytemanHelperRun1.GetEnv())
	require.NoError(t, err, "Failed to start export (run 1)")

	err = lm.StartImportData(true, nil)
	require.NoError(t, err, "Failed to start import data")

	time.Sleep(10 * time.Second)

	for batch := 1; batch <= 2; batch++ {
		lm.ExecuteOnSource(
			fmt.Sprintf(`INSERT INTO %s (name, value)
			SELECT 'batch%d_' || i, %d + i FROM generate_series(1, 20) i;`, tableName, batch, batch*100),
		)
		time.Sleep(batchSeparationWaitTime)
	}

	matched, err := bytemanHelperRun1.WaitForInjection(">>> BYTEMAN: fail_cdc_batch_run1", 90*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs (run 1)")
	require.True(t, matched, "Byteman injection should have occurred (run 1)")

	err = lm.WaitForExportDataExitTimeout(120 * time.Second)
	require.Error(t, err, "Export should exit with error after run 1 injection")

	time.Sleep(3 * time.Second)

	eventCountAfterRun1, err := countEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count events after run 1")
	require.Equal(t, 20, eventCountAfterRun1, "Expected 20 CDC events after run 1 (batch 1 only)")
	verifyNoEventIDDuplicates(t, exportDir)
	assertRowCount(90)

	// Run 2: fail on 2nd streaming batch again (replay batch2 succeeds, batch3 fails)
	bytemanHelperRun2, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err, "Failed to create Byteman helper (run 2)")
	bytemanHelperRun2.AddRuleFromBuilder(
		testutils.NewRule("fail_cdc_batch_run2").
			AtMarker(testutils.MarkerCDC, "before-batch-streaming").
			If("incrementCounter(\"cdc_batch\") == 2").
			ThrowException("java.lang.RuntimeException", "TEST: Simulated batch failure on run 2"),
	)
	require.NoError(t, bytemanHelperRun2.WriteRules())

	err = lm.StartExportDataWithEnv(true, nil, bytemanHelperRun2.GetEnv())
	require.NoError(t, err, "Failed to start export (run 2)")

	time.Sleep(10 * time.Second)

	lm.ExecuteOnSource(
		fmt.Sprintf(`INSERT INTO %s (name, value)
		SELECT 'batch3_' || i, 300 + i FROM generate_series(1, 20) i;`, tableName),
	)
	time.Sleep(batchSeparationWaitTime)

	matched, err = bytemanHelperRun2.WaitForInjection(">>> BYTEMAN: fail_cdc_batch_run2", 90*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs (run 2)")
	require.True(t, matched, "Byteman injection should have occurred (run 2)")

	err = lm.WaitForExportDataExitTimeout(120 * time.Second)
	require.Error(t, err, "Export should exit with error after run 2 injection")

	time.Sleep(3 * time.Second)

	eventCountAfterRun2, err := countEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count events after run 2")
	require.Equal(t, 40, eventCountAfterRun2, "Expected 40 CDC events after run 2 (batch 1 + replayed batch 2)")
	verifyNoEventIDDuplicates(t, exportDir)
	assertRowCount(110)

	// Run 3: no injection, complete remaining CDC
	err = lm.StartExportData(true, nil)
	require.NoError(t, err, "Failed to start export (run 3)")

	finalEventCount := lm.WaitForCDCEventCount(t, 60, 120*time.Second, 5*time.Second)
	require.Equal(t, 60, finalEventCount, "Expected 60 CDC events after final resume")

	verifyNoEventIDDuplicates(t, exportDir)
	assertRowCount(110)

	// Validate: import (started alongside run 1) should have snapshot + all CDC events
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		reportTableName(tableName): {Inserts: 60},
	}, 120, 5)
	require.NoError(t, err, "Forward streaming did not complete")

	err = lm.ValidateRowCount([]string{tableName})
	require.NoError(t, err, "Source and target row counts don't match after multiple batch failures recovery")
}

// TestCDCMultiTableBatchFailureAndResume verifies that live migration `export data` can resume
// after a mid-batch failure when streaming CDC events from multiple tables, and that `import data`
// correctly applies all events to the target with no anomalies.
//
// Scenario:
//  1. Start `export data` (snapshot-and-changes mode) with 2 tables (50 snapshot rows each).
//  2. Start `import data` concurrently (imports snapshot + streams CDC to target).
//  3. Generate 3 CDC batches, each inserting 10 rows into both tables (20 events per batch).
//  4. Inject failure on 2nd CDC batch via before-batch-streaming marker.
//  5. Export crashes after batch 1 committed (20 events); batches 2 and 3 are lost.
//  6. Resume `export data` without failure injection.
//  7. Verify all 60 CDC events recovered with no duplicates.
//  8. Verify import consumed all CDC events and source == target row counts match for both tables.
//
// This test validates:
// - Offset tracking correctness across multiple tables during failure/recovery
// - Event deduplication spanning multiple tables on resume
// - End-to-end: import data correctly applies recovered events to the target
//
// Injection point:
//   - Byteman rule on Debezium at `cdc("before-batch-streaming")` marker (2nd invocation).
func TestCDCMultiTableBatchFailureAndResume(t *testing.T) {
	if os.Getenv("BYTEMAN_JAR") == "" {
		t.Skip("Skipping test: BYTEMAN_JAR environment variable not set. Install Byteman to run this test.")
	}

	ctx := context.Background()

	tableA := "test_schema_multi_tbl.table_a"
	tableB := "test_schema_multi_tbl.table_b"

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:    ContainerConfig{Type: "postgresql", ForLive: true, DatabaseName: "postgres"},
		TargetDB:    ContainerConfig{Type: "yugabytedb", DatabaseName: "test_multi_tbl"},
		SchemaNames: []string{"test_schema_multi_tbl"},
		SchemaSQL: []string{
			"DROP SCHEMA IF EXISTS test_schema_multi_tbl CASCADE;",
			"CREATE SCHEMA test_schema_multi_tbl;",
			`CREATE TABLE test_schema_multi_tbl.table_a (
				id SERIAL PRIMARY KEY,
				name TEXT,
				value INTEGER
			);`,
			`CREATE TABLE test_schema_multi_tbl.table_b (
				id SERIAL PRIMARY KEY,
				label TEXT,
				score INTEGER
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema_multi_tbl.table_a REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema_multi_tbl.table_b REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema_multi_tbl.table_a (name, value)
			SELECT 'init_a_' || i, i * 10 FROM generate_series(1, 50) i;`,
			`INSERT INTO test_schema_multi_tbl.table_b (label, score)
			SELECT 'init_b_' || i, i * 20 FROM generate_series(1, 50) i;`,
		},
		CleanupSQL: []string{"DROP SCHEMA IF EXISTS test_schema_multi_tbl CASCADE;"},
	})
	defer lm.Cleanup()
	require.NoError(t, lm.SetupContainers(ctx))
	require.NoError(t, lm.SetupSchema())

	exportDir := lm.GetCurrentExportDir()

	bytemanHelper, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err, "Failed to create Byteman helper")

	bytemanHelper.AddRuleFromBuilder(
		testutils.NewRule("fail_multi_tbl_batch_2").
			AtMarker(testutils.MarkerCDC, "before-batch-streaming").
			If("incrementCounter(\"cdc_batch\") == 2").
			ThrowException("java.lang.RuntimeException", "TEST: Simulated multi-table batch failure on batch 2"),
	)
	require.NoError(t, bytemanHelper.WriteRules())

	// Run 1: export with Byteman injection - should fail on 2nd CDC batch
	err = lm.StartExportDataWithEnv(true, nil, bytemanHelper.GetEnv())
	require.NoError(t, err, "Failed to start export")

	err = lm.StartImportData(true, nil)
	require.NoError(t, err, "Failed to start import data")

	time.Sleep(10 * time.Second)

	for batch := 1; batch <= 3; batch++ {
		lm.ExecuteOnSource(
			fmt.Sprintf(`INSERT INTO test_schema_multi_tbl.table_a (name, value)
			SELECT 'batch%d_a_' || i, %d + i FROM generate_series(1, 10) i;`, batch, batch*100),
		)
		lm.ExecuteOnSource(
			fmt.Sprintf(`INSERT INTO test_schema_multi_tbl.table_b (label, score)
			SELECT 'batch%d_b_' || i, %d + i FROM generate_series(1, 10) i;`, batch, batch*200),
		)
		time.Sleep(batchSeparationWaitTime)
	}

	matched, err := bytemanHelper.WaitForInjection(">>> BYTEMAN: fail_multi_tbl_batch_2", 90*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs")
	require.True(t, matched, "Byteman injection should have occurred and been logged")

	err = lm.WaitForExportDataExitTimeout(120 * time.Second)
	require.Error(t, err, "Export should exit with error after Byteman injection")

	time.Sleep(3 * time.Second)

	eventCountAfterFailure, err := countEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count CDC events after failure")
	require.Equal(t, 20, eventCountAfterFailure, "Should have exactly 20 events (batch 1 from both tables) before failure")

	verifyNoEventIDDuplicates(t, exportDir)

	// Run 2: resume export without Byteman
	err = lm.StartExportData(true, nil)
	require.NoError(t, err, "Failed to start resumed export")

	finalEventCount := lm.WaitForCDCEventCount(t, 60, 120*time.Second, 5*time.Second)
	require.Equal(t, 60, finalEventCount, "Expected 60 CDC events after resume (30 per table)")

	verifyNoEventIDDuplicates(t, exportDir)

	// Validate: import (started alongside run 1) should have snapshot + all CDC events
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		reportTableName(tableA): {Inserts: 30},
		reportTableName(tableB): {Inserts: 30},
	}, 120, 5)
	require.NoError(t, err, "Forward streaming did not complete")

	err = lm.ValidateRowCount([]string{tableA, tableB})
	require.NoError(t, err, "Source and target row counts don't match after multi-table failure recovery")
}
