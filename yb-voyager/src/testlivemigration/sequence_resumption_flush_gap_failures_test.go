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
package testlivemigration

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

func assertSequenceRangeInserted(
	target *sql.DB, tableName string, startID int, endID int) error {
	_, err := target.Exec(fmt.Sprintf(`INSERT INTO %s (name, email, description)
SELECT
	md5(random()::text),
	md5(random()::text) || '@example.com',
	repeat(md5(random()::text), 10)
FROM generate_series(%d, %d);`, tableName, startID, endID))
	if err != nil {
		return fmt.Errorf("failed to insert into target: %w", err)
	}

	rows, err := target.Query(fmt.Sprintf(
		"SELECT id FROM %s WHERE id BETWEEN %d AND %d ORDER BY id",
		tableName, startID, endID))
	if err != nil {
		return fmt.Errorf("failed to query inserted ids: %w", err)
	}
	defer rows.Close()

	var actualIDs []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("failed to scan inserted ids: %w", err)
		}
		actualIDs = append(actualIDs, id)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed while reading inserted ids: %w", err)
	}

	expectedCount := endID - startID + 1
	if len(actualIDs) != expectedCount {
		return fmt.Errorf(
			"expected %d inserted ids in range [%d,%d], got %v",
			expectedCount, startID, endID, actualIDs)
	}
	for i := 0; i < expectedCount; i++ {
		expectedID := startID + i
		if actualIDs[i] != expectedID {
			return fmt.Errorf(
				"sequence mismatch, expected ids [%d..%d], got %v",
				startID, endID, actualIDs)
		}
	}
	return nil
}

// TestLiveMigrationResumption_SequenceRestoreAfterPostCommitCrash reproduces a
// Debezium crash window where queue events are durable but export_status.json
// sequence max is stale, then validates cutover sequence restoration.
func TestLiveMigrationResumption_SequenceRestoreAfterPostCommitCrash(t *testing.T) {
	if os.Getenv("BYTEMAN_JAR") == "" {
		t.Fatal("BYTEMAN_JAR environment variable not set. Install Byteman to run this test.")
	}
	t.Parallel()

	const (
		tableName             = `"test_schema_seq_flush"."test_live_seq"`
		reportTableName       = `"test_schema_seq_flush"."test_live_seq"`
		expectedStreamingRows = int64(20)
	)

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test_seq_flush_gap",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test_seq_flush_gap",
		},
		SchemaNames: []string{"test_schema_seq_flush"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema_seq_flush;
			CREATE TABLE test_schema_seq_flush.test_live_seq (
				id SERIAL PRIMARY KEY,
				name TEXT,
				email TEXT,
				description TEXT
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema_seq_flush.test_live_seq REPLICA IDENTITY FULL;`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema_seq_flush.test_live_seq (name, email, description)
SELECT
	md5(random()::text),
	md5(random()::text) || '@example.com',
	repeat(md5(random()::text), 10)
FROM generate_series(1, 20);`,
		},
		CleanupSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema_seq_flush CASCADE;`,
		},
	})
	defer lm.Cleanup()

	require.NoError(t, lm.SetupContainers(context.Background()))
	require.NoError(t, lm.SetupSchema())

	exportDir := lm.GetCurrentExportDir()
	bytemanHelper, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err, "failed to create Byteman helper")

	// Sleep the periodic flusher thread so sequence max updates do not get
	// persisted quickly during this test.
	bytemanHelper.AddRuleFromBuilder(
		testutils.NewRule("delay_export_status_flusher").
			Class("io.debezium.server.ybexporter.YbExporterConsumer").
			Method("flush").
			AtEntry().
			If("incrementCounter(\"flush_thread_entry\") == 1").
			Do(`traceln(">>> BYTEMAN: delay_export_status_flusher");
Thread.sleep(600000);`),
	)
	// Crash after commit is complete for the first streaming batch.
	bytemanHelper.AddRuleFromBuilder(
		testutils.NewRule("mark_streaming_batch_seen").
			AtMarker(testutils.MarkerCDC, "before-batch-streaming").
			If("true").
			Do(`incrementCounter("streaming_batches_seen");
traceln(">>> BYTEMAN: mark_streaming_batch_seen");`),
	)
	bytemanHelper.AddRuleFromBuilder(
		testutils.NewRule("crash_after_streaming_batch_commit").
			AtMarker(testutils.MarkerCDC, "after-batch").
			If(`readCounter("streaming_batches_seen") > 0 &&
incrementCounter("streaming_post_commit_crash") == 1`).
			ThrowException("java.lang.RuntimeException", "TEST: crash after streaming batch commit"),
	)
	require.NoError(t, bytemanHelper.WriteRules(), "failed to write Byteman rules")

	require.NoError(t, lm.StartExportDataWithEnv(true, nil, bytemanHelper.GetEnv()))
	require.NoError(t, lm.StartImportData(true, nil))

	require.NoError(t, lm.WaitForStreamingMode(2*time.Minute, 2*time.Second))
	require.NoError(t, lm.ExecuteSourceDelta())

	matched, err := bytemanHelper.WaitForInjection(
		">>> BYTEMAN: crash_after_streaming_batch_commit",
		90*time.Second,
	)
	require.NoError(t, err, "should be able to read debezium logs for post-commit crash")
	require.True(t, matched, "post-commit crash injection should be logged")

	err = lm.WaitForExportDataExitTimeout(120 * time.Second)
	require.Error(t, err, "export should exit with error after post-commit crash")

	eventCountAfterCrash, err := testutils.CountEventsInQueueSegments(exportDir)
	require.NoError(t, err, "should be able to count queue events after crash")
	require.GreaterOrEqual(t, eventCountAfterCrash, int(expectedStreamingRows),
		"expected committed CDC events to be present in queue after crash")

	status, err := dbzm.ReadExportStatus(filepath.Join(exportDir, "data", "export_status.json"))
	require.NoError(t, err, "failed to read export_status.json")
	var maxFlushedSeq int64
	for _, value := range status.Sequences {
		if value > maxFlushedSeq {
			maxFlushedSeq = value
		}
	}
	require.Less(t, maxFlushedSeq, expectedStreamingRows,
		"expected stale flushed sequence max to reproduce flush gap")

	require.NoError(t, lm.StartExportData(true, nil))

	require.NoError(t, lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		reportTableName: {Inserts: expectedStreamingRows},
	}, 120, 1))
	require.NoError(t, lm.ValidateDataConsistency([]string{tableName}, "id"))

	require.NoError(t, lm.InitiateCutoverToTarget(false, nil))
	require.NoError(t, lm.WaitForCutoverComplete(0, 120))

	err = lm.WithTargetConn(func(target *sql.DB) error {
		return assertSequenceRangeInserted(target, tableName, 21, 25)
	})
	require.NoError(t, err, "sequence should be restored correctly after cutover")
}
