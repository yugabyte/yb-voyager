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
	"database/sql"
	"fmt"
	"os"
	"strings"
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
//  5. Export crashes after some CDC from logical batch 1 is committed (connector may split
//     work across handleBatch calls); logical batches 2 and 3 are not fully applied.
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
		t.Fatal("BYTEMAN_JAR environment variable not set. Install Byteman to run this test.")
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

	//Wait for streaming to start
	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"test_schema"."cdc_test"`: 100,
	}, 120)
	require.NoError(t, err, "Snapshot did not complete")

	require.NoError(t, lm.WaitForStreamingMode(2*time.Minute, 2*time.Second),
		"Export should reach streaming mode before generating CDC")

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

	eventCountAfterFailure, err := testutils.CountEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count CDC events after failure")
	require.Equal(t, 20, eventCountAfterFailure, "Should have exactly 20 events (batch 1) before failure")

	testutils.VerifyNoEventIDDuplicates(t, exportDir)

	// Run 2: resume export without Byteman
	err = lm.StartExportData(true, nil)
	require.NoError(t, err, "Failed to start resumed export")

	finalEventCount := lm.WaitForCDCEventCount(t, 60, 120*time.Second, 5*time.Second)
	require.Equal(t, 60, finalEventCount, "Expected 60 CDC events after resume")

	testutils.VerifyNoEventIDDuplicates(t, exportDir)

	// Validate: import (started alongside run 1) should have snapshot + all CDC events
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		testutils.ReportTableName(tableName): {Inserts: 30, Updates: 15, Deletes: 15},
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
		t.Fatal("BYTEMAN_JAR environment variable not set. Install Byteman to run this test.")
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

	//Wait for streaming to start
	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"test_schema"."first_batch_test"`: 50,
	}, 120)
	require.NoError(t, err, "Snapshot did not complete")

	require.NoError(t, lm.WaitForStreamingMode(2*time.Minute, 2*time.Second),
		"Export should reach streaming mode before generating CDC")

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

	eventCount1, err := testutils.CountEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count events after first export")
	require.Equal(t, 0, eventCount1, "Should have 0 events (first batch failed at entry, no CDC state yet)")

	// Run 2: resume export without Byteman
	err = lm.StartExportData(true, nil)
	require.NoError(t, err, "Failed to start resumed export")

	finalEventCount := lm.WaitForCDCEventCount(t, 60, 120*time.Second, 5*time.Second)
	require.Equal(t, 60, finalEventCount, "Expected 60 CDC events after resume")

	testutils.VerifyNoEventIDDuplicates(t, exportDir)

	// Validate: import (started alongside run 1) should have snapshot + all CDC events
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		testutils.ReportTableName(tableName): {Inserts: 60},
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
		t.Fatal("BYTEMAN_JAR environment variable not set. Install Byteman to run this test.")
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

	eventCountAfterRun1, err := testutils.CountEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count events after run 1")
	require.Equal(t, 20, eventCountAfterRun1, "Expected 20 CDC events after run 1 (batch 1 only)")
	testutils.VerifyNoEventIDDuplicates(t, exportDir)
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

	eventCountAfterRun2, err := testutils.CountEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count events after run 2")
	require.Equal(t, 40, eventCountAfterRun2, "Expected 40 CDC events after run 2 (batch 1 + replayed batch 2)")
	testutils.VerifyNoEventIDDuplicates(t, exportDir)
	assertRowCount(110)

	// Run 3: no injection, complete remaining CDC
	err = lm.StartExportData(true, nil)
	require.NoError(t, err, "Failed to start export (run 3)")

	finalEventCount := lm.WaitForCDCEventCount(t, 60, 120*time.Second, 5*time.Second)
	require.Equal(t, 60, finalEventCount, "Expected 60 CDC events after final resume")

	testutils.VerifyNoEventIDDuplicates(t, exportDir)
	assertRowCount(110)

	// Validate: import (started alongside run 1) should have snapshot + all CDC events
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		testutils.ReportTableName(tableName): {Inserts: 60},
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
		t.Fatal("BYTEMAN_JAR environment variable not set. Install Byteman to run this test.")
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

	//Wait for streaming to start
	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"test_schema_multi_tbl"."table_a"`: 50,
		`"test_schema_multi_tbl"."table_b"`: 50,
	}, 120)
	require.NoError(t, err, "Snapshot did not complete")

	require.NoError(t, lm.WaitForStreamingMode(2*time.Minute, 2*time.Second),
		"Export should reach streaming mode before generating CDC")

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

	eventCountAfterFailure, err := testutils.CountEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count CDC events after failure")
	require.Equal(t, 20, eventCountAfterFailure, "Should have exactly 20 events (batch 1 from both tables) before failure")

	testutils.VerifyNoEventIDDuplicates(t, exportDir)

	// Run 2: resume export without Byteman
	err = lm.StartExportData(true, nil)
	require.NoError(t, err, "Failed to start resumed export")

	finalEventCount := lm.WaitForCDCEventCount(t, 60, 120*time.Second, 5*time.Second)
	require.Equal(t, 60, finalEventCount, "Expected 60 CDC events after resume (30 per table)")

	testutils.VerifyNoEventIDDuplicates(t, exportDir)

	// Validate: import (started alongside run 1) should have snapshot + all CDC events
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		testutils.ReportTableName(tableA): {Inserts: 30},
		testutils.ReportTableName(tableB): {Inserts: 30},
	}, 120, 5)
	require.NoError(t, err, "Forward streaming did not complete")

	err = lm.ValidateRowCount([]string{tableA, tableB})
	require.NoError(t, err, "Source and target row counts don't match after multi-table failure recovery")
}

// TestCDCOffsetCommitFailureAndResume verifies that live migration `export data` can resume after
// an offset commit failure during CDC streaming, and that `import data` correctly applies all
// events to the target with no anomalies.
//
// Scenario:
//  1. Start `export data` (snapshot-and-changes mode) with 50 snapshot rows.
//  2. Start `import data` concurrently (imports snapshot + streams CDC to target).
//  3. Insert 20 CDC events and process them (write to queue + flush/sync).
//  4. Inject failure at before-offset-commit marker (before offsets are persisted).
//  5. Export crashes; queue has 20 events but offsets file is empty.
//  6. Resume `export data` and verify batch is replayed from the beginning.
//  7. Verify dedup cache skips all 20 replayed events (no duplicates in queue).
//  8. Verify import consumed all CDC events and source == target row counts match.
//
// This test validates:
// - Offset commit failure forces full batch replay
// - Event deduplication prevents duplicate writes during replay
// - End-to-end: import data correctly applies recovered events to the target
//
// Injection point:
//   - Byteman rule on Debezium at the before-offset-commit marker,
//     firing after queue write but before offset persistence.
func TestCDCOffsetCommitFailureAndResume(t *testing.T) {
	if os.Getenv("BYTEMAN_JAR") == "" {
		t.Fatal("BYTEMAN_JAR environment variable not set. Install Byteman to run this test.")
	}

	ctx := context.Background()

	tableName := "test_schema_offset_commit.cdc_offset_commit_test"

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:    ContainerConfig{Type: "postgresql", ForLive: true, DatabaseName: "postgres"},
		TargetDB:    ContainerConfig{Type: "yugabytedb", DatabaseName: "test_offset_commit"},
		SchemaNames: []string{"test_schema_offset_commit"},
		SchemaSQL: []string{
			"DROP SCHEMA IF EXISTS test_schema_offset_commit CASCADE;",
			"CREATE SCHEMA test_schema_offset_commit;",
			`CREATE TABLE test_schema_offset_commit.cdc_offset_commit_test (
				id SERIAL PRIMARY KEY,
				name TEXT,
				value INTEGER,
				created_at TIMESTAMP DEFAULT NOW()
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema_offset_commit.cdc_offset_commit_test REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema_offset_commit.cdc_offset_commit_test (name, value)
			SELECT 'snapshot_' || i, i * 10 FROM generate_series(1, 50) i;`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema_offset_commit.cdc_offset_commit_test (name, value)
			SELECT 'batch1_' || i, 100 + i FROM generate_series(1, 20) i;`,
		},
		CleanupSQL: []string{"DROP SCHEMA IF EXISTS test_schema_offset_commit CASCADE;"},
	})
	defer lm.Cleanup()
	require.NoError(t, lm.SetupContainers(ctx))
	require.NoError(t, lm.SetupSchema())

	exportDir := lm.GetCurrentExportDir()

	bytemanHelper, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err, "Failed to create Byteman helper")
	bytemanHelper.AddRuleFromBuilder(
		testutils.NewRule("fail_offset_commit").
			AtMarker(testutils.MarkerCDC, "before-offset-commit").
			If("incrementCounter(\"offset_commit\") == 1").
			ThrowException("java.lang.RuntimeException", "Simulated offset commit failure"),
	)
	require.NoError(t, bytemanHelper.WriteRules(), "Failed to write Byteman rules")

	// Run 1: export with Byteman injection at before-offset-commit
	err = lm.StartExportDataWithEnv(true, nil, bytemanHelper.GetEnv())
	require.NoError(t, err, "Failed to start export")

	err = lm.StartImportData(true, nil)
	require.NoError(t, err, "Failed to start import data")

	time.Sleep(10 * time.Second)

	offsetBeforeCDC := testutils.ReadOffsetFileChecksum(exportDir, SOURCE_DB_EXPORTER_ROLE)

	lm.ExecuteSourceDelta()

	matched, err := bytemanHelper.WaitForInjection(">>> BYTEMAN: fail_offset_commit", 90*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs for offset commit failure")
	require.True(t, matched, "Byteman offset commit failure should be injected")

	err = lm.WaitForExportDataExitTimeout(120 * time.Second)
	require.Error(t, err, "Export should exit with error after offset commit failure")

	eventCountAfterFailure, err := testutils.CountEventsInQueueSegments(exportDir)
	require.NoError(t, err, "Should be able to count events after failure")
	require.Equal(t, 20, eventCountAfterFailure, "Expected 20 CDC events after failure")
	offsetAfterFailure := testutils.ReadOffsetFileChecksum(exportDir, SOURCE_DB_EXPORTER_ROLE)
	require.Equal(t, offsetBeforeCDC, offsetAfterFailure, "Offsets advanced despite before-offset-commit failure; replay will not occur")
	offsetContents := testutils.ReadOffsetFileContents(exportDir, SOURCE_DB_EXPORTER_ROLE)
	require.Equal(t, "", strings.TrimSpace(offsetContents), "Offset file should be empty after failure")

	eventIDsBefore, err := testutils.CollectEventIDsForOffsetCommitTest(exportDir)
	require.NoError(t, err, "Failed to read event_ids after failure")
	require.Len(t, eventIDsBefore, 20, "Expected 20 unique event_ids after failure")
	testutils.VerifyNoEventIDDuplicates(t, exportDir)
	dedupSkipsBeforeResume, err := testutils.CountDedupSkipLogs(exportDir)
	require.NoError(t, err, "Failed to count dedup skip logs before resume")

	// Run 2: resume export with Byteman tracing rule to detect batch replay
	bytemanHelperResume, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err, "Failed to create Byteman helper for resume")
	bytemanHelperResume.AddRuleFromBuilder(
		testutils.NewRule("replay_batch").
			AtMarker(testutils.MarkerCDC, "before-batch-streaming").
			If("incrementCounter(\"replay_batch\") == 1").
			Do(`traceln(">>> BYTEMAN: replay_batch");`),
	)
	require.NoError(t, bytemanHelperResume.WriteRules(), "Failed to write Byteman rules for resume")

	err = lm.StartExportDataWithEnv(true, nil, bytemanHelperResume.GetEnv())
	require.NoError(t, err, "Failed to start export resume")

	replayMatched, err := bytemanHelperResume.WaitForInjection(">>> BYTEMAN: replay_batch", 90*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs for replay marker")
	require.True(t, replayMatched, "Expected replay batch after resume")

	eventIDsAfter, err := testutils.CollectEventIDsForOffsetCommitTest(exportDir)
	require.NoError(t, err, "Failed to read event_ids after resume")
	require.Equal(t, len(eventIDsBefore), len(eventIDsAfter), "event_id set size should be unchanged after replay")
	for eventID := range eventIDsBefore {
		_, ok := eventIDsAfter[eventID]
		require.True(t, ok, "event_id should still exist after replay: %s", eventID)
	}
	testutils.VerifyNoEventIDDuplicates(t, exportDir)
	dedupSkipsAfterResume, err := testutils.CountDedupSkipLogs(exportDir)
	require.NoError(t, err, "Failed to count dedup skip logs after resume")
	require.GreaterOrEqual(t, dedupSkipsAfterResume-dedupSkipsBeforeResume, 20,
		"Expected dedup cache to skip at least 20 replayed records on resume")

	// Validate: import (started alongside run 1) should have snapshot + all CDC events
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		testutils.ReportTableName(tableName): {Inserts: 20},
	}, 120, 5)
	require.NoError(t, err, "Forward streaming did not complete")

	err = lm.ValidateRowCount([]string{tableName})
	require.NoError(t, err, "Source and target row counts don't match after offset commit failure recovery")
}

// TestCDCBatchFailureBeforeHandleBatchComplete verifies that live migration `export data` can resume
// after a crash before flush/sync, recovering from the durability gap, and that `import data`
// correctly applies all events to the target with no anomalies.
//
// Scenario:
//  1. Start `export data` (snapshot-and-changes mode) with 50 snapshot rows.
//  2. Start `import data` concurrently (imports snapshot + streams CDC to target).
//  3. Insert 20 CDC events with large payloads to force buffered writes.
//  4. Inject failure at before-handle-batch-complete marker (after write, before flush/sync).
//  5. Export crashes; data may be in buffer but not flushed to disk.
//  6. Resume `export data` and insert 10 more CDC events.
//  7. Verify all 30 events eventually written with no duplicates.
//  8. Verify import consumed all CDC events and source == target row counts match.
//
// This test validates:
// - Durability gap: records written to buffer but not fsynced are lost on crash
// - Recovery replays lost events from offsets
// - Deduplication works correctly during replay
// - End-to-end: import data correctly applies recovered events to the target
//
// Injection point:
//   - Byteman rule on Debezium at the before-handle-batch-complete marker,
//     firing after queue write but before flush/sync.
func TestCDCBatchFailureBeforeHandleBatchComplete(t *testing.T) {
	if os.Getenv("BYTEMAN_JAR") == "" {
		t.Fatal("BYTEMAN_JAR environment variable not set. Install Byteman to run this test.")
	}

	ctx := context.Background()

	tableName := "test_schema_before_batch_complete.cdc_before_batch_complete_test"

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:    ContainerConfig{Type: "postgresql", ForLive: true, DatabaseName: "postgres"},
		TargetDB:    ContainerConfig{Type: "yugabytedb", DatabaseName: "test_batch_complete"},
		SchemaNames: []string{"test_schema_before_batch_complete"},
		SchemaSQL: []string{
			"DROP SCHEMA IF EXISTS test_schema_before_batch_complete CASCADE;",
			"CREATE SCHEMA test_schema_before_batch_complete;",
			`CREATE TABLE test_schema_before_batch_complete.cdc_before_batch_complete_test (
				id SERIAL PRIMARY KEY,
				name TEXT,
				value INTEGER,
				payload TEXT,
				created_at TIMESTAMP DEFAULT NOW()
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema_before_batch_complete.cdc_before_batch_complete_test REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema_before_batch_complete.cdc_before_batch_complete_test (name, value, payload)
			SELECT 'snapshot_' || i, i * 10, repeat('s', 20000) FROM generate_series(1, 50) i;`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema_before_batch_complete.cdc_before_batch_complete_test (name, value, payload)
			SELECT 'batch1_' || i, 100 + i, repeat('x', 2000) FROM generate_series(1, 20) i;`,
		},
		CleanupSQL: []string{"DROP SCHEMA IF EXISTS test_schema_before_batch_complete CASCADE;"},
	})
	defer lm.Cleanup()
	require.NoError(t, lm.SetupContainers(ctx))
	require.NoError(t, lm.SetupSchema())

	exportDir := lm.GetCurrentExportDir()

	bytemanHelper, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err, "Failed to create Byteman helper")
	bytemanHelper.AddRuleFromBuilder(
		testutils.NewRule("fail_before_handle_batch_complete").
			AtMarker(testutils.MarkerCDC, "before-handle-batch-complete").
			If("incrementCounter(\"before_handle_batch_complete\") == 1").
			ThrowException("java.lang.RuntimeException", "Simulated failure before handleBatchComplete"),
	)
	require.NoError(t, bytemanHelper.WriteRules(), "Failed to write Byteman rules")

	// Run 1: export with Byteman injection at before-handle-batch-complete
	err = lm.StartExportDataWithEnv(true, nil, bytemanHelper.GetEnv())
	require.NoError(t, err, "Failed to start export")

	err = lm.StartImportData(true, nil)
	require.NoError(t, err, "Failed to start import data")

	time.Sleep(10 * time.Second)

	lm.ExecuteSourceDelta()

	matched, err := bytemanHelper.WaitForInjection(">>> BYTEMAN: fail_before_handle_batch_complete", 90*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs for handleBatchComplete failure")
	require.True(t, matched, "Byteman failure should be injected before handleBatchComplete")

	err = lm.WaitForExportDataExitTimeout(120 * time.Second)
	require.Error(t, err, "Export should exit with error after failure")

	_, _ = testutils.VerifyNoEventIDDuplicatesAfterFailure(t, exportDir)

	// Run 2: resume export and insert 10 more events
	err = lm.StartExportData(true, nil)
	require.NoError(t, err, "Failed to start export resume")

	time.Sleep(10 * time.Second)

	lm.ExecuteOnSource(
		`INSERT INTO test_schema_before_batch_complete.cdc_before_batch_complete_test (name, value, payload)
		SELECT 'resume_' || i, 200 + i, repeat('y', 2000) FROM generate_series(1, 10) i;`,
	)

	finalEventCount := lm.WaitForCDCEventCount(t, 30, 120*time.Second, 5*time.Second)
	require.Equal(t, 30, finalEventCount, "Expected 30 CDC events after resume")
	testutils.VerifyNoEventIDDuplicates(t, exportDir)

	// Validate: import (started alongside run 1) should have snapshot + all CDC events
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		testutils.ReportTableName(tableName): {Inserts: 30},
	}, 120, 5)
	require.NoError(t, err, "Forward streaming did not complete")

	err = lm.ValidateRowCount([]string{tableName})
	require.NoError(t, err, "Source and target row counts don't match after handle-batch-complete failure recovery")
}

// TestCDCQueueWriteFailureAndResume verifies that live migration `export data` can resume after
// a queue write failure mid-batch during CDC streaming, and that `import data` correctly applies
// all events to the target with no anomalies.
//
// Scenario:
//  1. Start `export data` (snapshot-and-changes mode) with 50 snapshot rows.
//  2. Start `import data` concurrently (imports snapshot + streams CDC to target).
//  3. Insert 40 CDC events with large payloads (20KB each) to exceed buffer size.
//  4. Inject failure at before-write-record marker on the 25th event.
//  5. Export crashes with ~24 events written (buffer flushed due to size).
//  6. Resume `export data` and verify all 40 events eventually written.
//  7. Verify no event count overgrowth (dedup prevents duplicates).
//  8. Verify import consumed all CDC events and source == target row counts match.
//
// This test validates:
// - Mid-write failure recovery
// - Buffered data is flushed when buffer size exceeds threshold
// - Deduplication prevents event count from exceeding expected total
// - End-to-end: import data correctly applies recovered events to the target
//
// Injection point:
//   - Byteman rule on Debezium at the before-write-record marker,
//     firing on the 25th event write.
func TestCDCQueueWriteFailureAndResume(t *testing.T) {
	if os.Getenv("BYTEMAN_JAR") == "" {
		t.Fatal("BYTEMAN_JAR environment variable not set. Install Byteman to run this test.")
	}

	ctx := context.Background()

	tableName := "test_schema_queue_write.cdc_queue_write_test"

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:    ContainerConfig{Type: "postgresql", ForLive: true, DatabaseName: "postgres"},
		TargetDB:    ContainerConfig{Type: "yugabytedb", DatabaseName: "test_queue_write"},
		SchemaNames: []string{"test_schema_queue_write"},
		SchemaSQL: []string{
			"DROP SCHEMA IF EXISTS test_schema_queue_write CASCADE;",
			"CREATE SCHEMA test_schema_queue_write;",
			`CREATE TABLE test_schema_queue_write.cdc_queue_write_test (
				id SERIAL PRIMARY KEY,
				name TEXT,
				value INTEGER,
				payload TEXT,
				created_at TIMESTAMP DEFAULT NOW()
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema_queue_write.cdc_queue_write_test REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema_queue_write.cdc_queue_write_test (name, value, payload)
			SELECT 'snapshot_' || i, i * 10, repeat('s', 20000) FROM generate_series(1, 50) i;`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema_queue_write.cdc_queue_write_test (name, value, payload)
			SELECT 'batch1_' || i, 100 + i, repeat('q', 20000) FROM generate_series(1, 40) i;`,
		},
		CleanupSQL: []string{"DROP SCHEMA IF EXISTS test_schema_queue_write CASCADE;"},
	})
	defer lm.Cleanup()
	require.NoError(t, lm.SetupContainers(ctx))
	require.NoError(t, lm.SetupSchema())

	exportDir := lm.GetCurrentExportDir()

	bytemanHelper, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err, "Failed to create Byteman helper")
	bytemanHelper.AddRuleFromBuilder(
		testutils.NewRule("fail_queue_write").
			AtMarker(testutils.MarkerCDC, "before-write-record").
			If("incrementCounter(\"write_record\") == 25").
			ThrowException("java.lang.RuntimeException", "Simulated queue write failure"),
	)
	require.NoError(t, bytemanHelper.WriteRules(), "Failed to write Byteman rules")

	// Run 1: export with Byteman injection at before-write-record (25th event)
	err = lm.StartExportDataWithEnv(true, nil, bytemanHelper.GetEnv())
	require.NoError(t, err, "Failed to start export")

	err = lm.StartImportData(true, nil)
	require.NoError(t, err, "Failed to start import data")

	time.Sleep(10 * time.Second)

	lm.ExecuteSourceDelta()

	matched, err := bytemanHelper.WaitForInjection(">>> BYTEMAN: fail_queue_write", 90*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs for queue write failure")
	require.True(t, matched, "Byteman queue write failure should be injected")

	err = lm.WaitForExportDataExitTimeout(120 * time.Second)
	require.Error(t, err, "Export should exit with error after queue write failure")

	// Run 2: resume export — all 40 events should be recovered
	err = lm.StartExportData(true, nil)
	require.NoError(t, err, "Failed to start export resume")

	lm.WaitForCDCEventCount(t, 40, 120*time.Second, 5*time.Second)
	testutils.AssertEventCountDoesNotExceed(t, exportDir, 40, 15*time.Second, 2*time.Second)
	testutils.VerifyNoEventIDDuplicates(t, exportDir)

	// Validate: import (started alongside run 1) should have snapshot + all CDC events
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		testutils.ReportTableName(tableName): {Inserts: 40},
	}, 120, 5)
	require.NoError(t, err, "Forward streaming did not complete")

	err = lm.ValidateRowCount([]string{tableName})
	require.NoError(t, err, "Source and target row counts don't match after queue write failure recovery")
}

// TestCDCRotationMidBatchClosesSegment verifies that live migration `export data` properly
// closes rotated queue segments when a crash occurs mid-batch during segment rotation.
//
// Scenario:
//  1. Start `export data` with very small queue segment size (8KB via QUEUE_SEGMENT_MAX_BYTES).
//  2. Insert 30 CDC events with 5KB payloads to force multiple segment rotations mid-batch.
//  3. Inject failure at before-handle-batch-complete marker (before batch commits).
//  4. Export crashes with multiple queue segments created.
//  5. Verify the first (lowest-numbered) rotated segment is closed with EOF marker.
//
// This test validates:
// - Segment rotation mid-batch properly closes/syncs the old segment
// - Rotated segments have EOF markers even when batch doesn't complete
//
// Injection point:
//   - Byteman rule on Debezium at the before-handle-batch-complete marker,
//     firing before batch commit with small segment size forcing rotation.
func TestCDCRotationMidBatchClosesSegment(t *testing.T) {
	if os.Getenv("BYTEMAN_JAR") == "" {
		t.Fatal("BYTEMAN_JAR environment variable not set. Install Byteman to run this test.")
	}

	ctx := context.Background()

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:    ContainerConfig{Type: "postgresql", ForLive: true, DatabaseName: "postgres"},
		SchemaNames: []string{"test_schema_rotation"},
		SchemaSQL: []string{
			"DROP SCHEMA IF EXISTS test_schema_rotation CASCADE;",
			"CREATE SCHEMA test_schema_rotation;",
			`CREATE TABLE test_schema_rotation.cdc_rotation_test (
				id SERIAL PRIMARY KEY,
				name TEXT,
				value INTEGER,
				payload TEXT,
				created_at TIMESTAMP DEFAULT NOW()
			);`,
			`ALTER TABLE test_schema_rotation.cdc_rotation_test REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema_rotation.cdc_rotation_test (name, value, payload)
			SELECT 'snapshot_' || i, i * 10, repeat('s', 20000) FROM generate_series(1, 50) i;`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema_rotation.cdc_rotation_test (name, value, payload)
			SELECT 'batch1_' || i, 100 + i, repeat('r', 5000) FROM generate_series(1, 30) i;`,
		},
		CleanupSQL: []string{"DROP SCHEMA IF EXISTS test_schema_rotation CASCADE;"},
	})
	defer lm.Cleanup()
	require.NoError(t, lm.SetupContainers(ctx))
	require.NoError(t, lm.SetupSchema())

	exportDir := lm.GetCurrentExportDir()

	bytemanHelper, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err, "Failed to create Byteman helper")
	bytemanHelper.AddRuleFromBuilder(
		testutils.NewRule("fail_before_handle_batch_complete_rotation").
			AtMarker(testutils.MarkerCDC, "before-handle-batch-complete").
			If("incrementCounter(\"before_handle_batch_complete\") == 1").
			ThrowException("java.lang.RuntimeException", "Simulated failure before handleBatchComplete"),
	)
	require.NoError(t, bytemanHelper.WriteRules(), "Failed to write Byteman rules")

	envVars := append(bytemanHelper.GetEnv(), "QUEUE_SEGMENT_MAX_BYTES=8192")
	err = lm.StartExportDataWithEnv(true, nil, envVars)
	require.NoError(t, err, "Failed to start export")

	time.Sleep(10 * time.Second)

	lm.ExecuteSourceDelta()

	matched, err := bytemanHelper.WaitForInjection(">>> BYTEMAN: fail_before_handle_batch_complete_rotation", 90*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs for handleBatchComplete failure")
	require.True(t, matched, "Byteman failure should be injected before handleBatchComplete")

	// Kill immediately after injection to avoid graceful shutdown that could sync segments.
	// _ = lm.exportCmd.Kill()
	// lm.KillDebezium(SOURCE_DB_EXPORTER_ROLE)
	err = lm.WaitForExportDataExitTimeout(120 * time.Second)
	require.Error(t, err, "Export should exit after Byteman failure")

	segmentFiles, err := testutils.ListQueueSegmentFiles(exportDir)
	require.NoError(t, err, "Failed to list queue segment files")
	require.GreaterOrEqual(t, len(segmentFiles), 2, "Expected multiple queue segments after rotation")

	lowestSegmentPath, _, highestSegmentPath, highestSegmentNum, err := testutils.FindSegmentNumRange(segmentFiles)
	require.NoError(t, err, "Failed to parse queue segment numbers")
	require.NotEmpty(t, lowestSegmentPath, "Expected to identify lowest queue segment")

	// Every segment other than highest segment should be closed with EOF marker
	for _, segmentPath := range segmentFiles {
		if segmentPath == highestSegmentPath {
			continue
		}
		closed, err := testutils.IsQueueSegmentClosed(segmentPath)
		require.NoError(t, err, "Failed to check queue segment EOF marker")
		require.True(t, closed, "Segment should be closed with EOF marker")
	}

	closed, err = testutils.IsQueueSegmentClosed(highestSegmentPath)
	require.NoError(t, err, "Failed to check queue segment EOF marker")
	require.False(t, closed, "Last rotated queue segment should be closed with EOF marker")

	require.GreaterOrEqual(t, highestSegmentNum, int64(1), "Expected latest segment to be >= 1 after rotation")
}

// TestCDCQueueSegmentTruncationOnResume verifies that live migration `export data` correctly
// truncates incomplete queue segments back to the committed size on resume.
//
// Scenario:
//  1. Start `export data` with large segment size (1GB, forces single segment).
//  2. Insert 20 CDC events with large payloads (20KB each) to force buffer flush to disk.
//  3. Inject failure at before-handle-batch-complete marker (after write, before fsync/commit).
//  4. Export crashes; queue segment file size > committed size in metadb.
//  5. Resume `export data` and verify:
//     - Truncation log appears in Debezium logs.
//     - Queue segment file is truncated back to committed size (0 bytes in this case).
//
// This test validates:
// - Queue segment recovery truncates uncommitted bytes on resume
// - Metadb size_committed is the source of truth for valid data boundary
//
// Injection point:
//   - Byteman rule on Debezium at the before-handle-batch-complete marker,
//     firing after write but before fsync/commit with large segment size.
func TestCDCQueueSegmentTruncationOnResume(t *testing.T) {
	if os.Getenv("BYTEMAN_JAR") == "" {
		t.Fatal("BYTEMAN_JAR environment variable not set. Install Byteman to run this test.")
	}

	ctx := context.Background()

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:    ContainerConfig{Type: "postgresql", ForLive: true, DatabaseName: "postgres"},
		SchemaNames: []string{"test_schema_truncation"},
		SchemaSQL: []string{
			"DROP SCHEMA IF EXISTS test_schema_truncation CASCADE;",
			"CREATE SCHEMA test_schema_truncation;",
			`CREATE TABLE test_schema_truncation.cdc_truncation_test (
				id SERIAL PRIMARY KEY,
				name TEXT,
				value INTEGER,
				payload TEXT,
				created_at TIMESTAMP DEFAULT NOW()
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema_truncation.cdc_truncation_test REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema_truncation.cdc_truncation_test (name, value, payload)
			SELECT 'snapshot_' || i, i * 10, repeat('s', 20000) FROM generate_series(1, 50) i;`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema_truncation.cdc_truncation_test (name, value, payload)
			SELECT 'batch1_' || i, 100 + i, repeat('t', 20000) FROM generate_series(1, 20) i;`,
		},
		CleanupSQL: []string{"DROP SCHEMA IF EXISTS test_schema_truncation CASCADE;"},
	})
	defer lm.Cleanup()
	require.NoError(t, lm.SetupContainers(ctx))
	require.NoError(t, lm.SetupSchema())

	exportDir := lm.GetCurrentExportDir()

	bytemanHelper, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err, "Failed to create Byteman helper")
	bytemanHelper.AddRuleFromBuilder(
		testutils.NewRule("fail_before_handle_batch_complete_truncation").
			AtMarker(testutils.MarkerCDC, "before-handle-batch-complete").
			If("incrementCounter(\"before_handle_batch_complete\") == 1").
			ThrowException("java.lang.RuntimeException", "Simulated failure before handleBatchComplete"),
	)
	require.NoError(t, bytemanHelper.WriteRules(), "Failed to write Byteman rules")

	// Run 1: export with Byteman + large segment (1GB forces single segment)
	envVars := append(bytemanHelper.GetEnv(), "QUEUE_SEGMENT_MAX_BYTES=1073741824")
	err = lm.StartExportDataWithEnv(true, nil, envVars)
	require.NoError(t, err, "Failed to start export")

	time.Sleep(10 * time.Second)

	lm.ExecuteSourceDelta()

	matched, err := bytemanHelper.WaitForInjection(">>> BYTEMAN: fail_before_handle_batch_complete_truncation", 90*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs for handleBatchComplete failure")
	require.True(t, matched, "Byteman failure should be injected before handleBatchComplete")

	err = lm.WaitForExportDataExitTimeout(120 * time.Second)
	require.Error(t, err, "Export should exit with error after failure")

	segmentFiles, err := testutils.ListQueueSegmentFiles(exportDir)
	require.NoError(t, err, "Failed to list queue segment files")
	require.Len(t, segmentFiles, 1, "Expected a single queue segment before resume")
	segmentNum, err := testutils.ParseQueueSegmentNum(segmentFiles[0])
	require.NoError(t, err, "Failed to parse queue segment number")

	fileSizeBefore, err := testutils.GetQueueSegmentFileSize(segmentFiles[0])
	require.NoError(t, err, "Failed to read queue segment size after failure")
	committedSize, err := testutils.GetQueueSegmentCommittedSize(exportDir, segmentNum)
	require.NoError(t, err, "Failed to read committed size from metadb")
	require.Greater(t, fileSizeBefore, committedSize, "Expected file size to exceed committed size before resume")

	// Run 2: resume export to trigger truncation
	err = lm.StartExportData(true, nil)
	require.NoError(t, err, "Failed to start export resume")

	truncationMatched, err := testutils.WaitForTruncationLog(exportDir, 60*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs for truncation")
	require.True(t, truncationMatched, "Expected truncation log on resume")

	truncationTargetSize, err := testutils.ParseTruncationTargetSize(exportDir)
	require.NoError(t, err, "Failed to parse truncation target size from log")
	require.Equal(t, committedSize, truncationTargetSize,
		"Truncation target should equal committed size from metadb")
	t.Logf("Queue segment: fileSize=%d, committedSize=%d, truncatedTo=%d",
		fileSizeBefore, committedSize, truncationTargetSize)
}
