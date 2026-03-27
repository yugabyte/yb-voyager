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
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// TestVSNResetAfterSegmentRotationCrashAndArchival demonstrates a data-loss bug
// where the Voyager Sequence Number (VSN) resets to 0 on export resume after:
//
//  1. Debezium rotates a queue segment (closes segment N, creates segment N+1).
//  2. Export crashes immediately after rotation — segment N+1 is empty.
//  3. Import ingests segment N successfully; archiver deletes segment N.
//  4. Export resumes: recovery reads segment N+1 (empty), falls back to segment N
//     to find the last VSN — but segment N was deleted, so a fresh empty file is
//     created, getSequenceNumberOfLastRecord() returns -1, and nextVSN becomes 0.
//  5. New events get VSN starting from 0, but import's last_applied_vsn is already
//     > 0 from segment N — so the importer skips these events → data loss.
//
// Injection point:
//   - Byteman AFTER INVOKE rotateQueueSegment inside EventQueue.writeRecord,
//     causing a crash right after the new segment is created but before any
//     event is written to it.
func TestVSNResetAfterSegmentRotationCrashAndArchival(t *testing.T) {
	if os.Getenv("BYTEMAN_JAR") == "" {
		t.Skip("Skipping test: BYTEMAN_JAR environment variable not set.")
	}

	ctx := context.Background()
	tableName := "test_schema_vsn_reset.vsn_reset_test"

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:    ContainerConfig{Type: "postgresql", ForLive: true, DatabaseName: "test_vsn_reset"},
		TargetDB:    ContainerConfig{Type: "yugabytedb", DatabaseName: "test_vsn_reset"},
		SchemaNames: []string{"test_schema_vsn_reset"},
		SchemaSQL: []string{
			"DROP SCHEMA IF EXISTS test_schema_vsn_reset CASCADE;",
			"CREATE SCHEMA test_schema_vsn_reset;",
			`CREATE TABLE test_schema_vsn_reset.vsn_reset_test (
				id SERIAL PRIMARY KEY,
				name TEXT,
				value INTEGER,
				payload TEXT
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema_vsn_reset.vsn_reset_test REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema_vsn_reset.vsn_reset_test (name, value, payload)
			SELECT 'snapshot_' || i, i, repeat('s', 100) FROM generate_series(1, 30) i;`,
		},
		SourceDeltaSQL: []string{
			// Batch 1: events with large payloads to force segment rotation at small max bytes.
			`INSERT INTO test_schema_vsn_reset.vsn_reset_test (name, value, payload)
			SELECT 'batch1_' || i, 1000 + i, repeat('a', 5000) FROM generate_series(1, 30) i;`,
		},
		CleanupSQL: []string{"DROP SCHEMA IF EXISTS test_schema_vsn_reset CASCADE;"},
	})
	defer lm.Cleanup()
	require.NoError(t, lm.SetupContainers(ctx))
	require.NoError(t, lm.SetupSchema())

	exportDir := lm.exportDir

	// ---------------------------------------------------------------
	// Phase 1: Start export with Byteman crash-after-rotation rule.
	// Small QUEUE_SEGMENT_MAX_BYTES forces rotation after a few events.
	// ---------------------------------------------------------------
	bytemanHelper, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err, "Failed to create Byteman helper")

	// Raw rule: AFTER INVOKE targets the point right after rotateQueueSegment()
	// returns inside writeRecord(), before any data is written to the new segment.
	bytemanHelper.AddRule(`RULE crash_after_rotation
CLASS io.debezium.server.ybexporter.EventQueue
METHOD writeRecord
AFTER INVOKE rotateQueueSegment
IF incrementCounter("rotate_count") == 1
DO traceln(">>> BYTEMAN: crash_after_rotation"); throw new java.lang.RuntimeException("Simulated crash after queue segment rotation")
ENDRULE`)
	require.NoError(t, bytemanHelper.WriteRules(), "Failed to write Byteman rules")

	envVars := append(bytemanHelper.GetEnv(), "QUEUE_SEGMENT_MAX_BYTES=8192")
	err = lm.StartExportDataWithEnv(true, nil, envVars)
	require.NoError(t, err, "Failed to start export")

	err = lm.StartImportData(true, nil)
	require.NoError(t, err, "Failed to start import")

	time.Sleep(10 * time.Second)

	// ---------------------------------------------------------------
	// Phase 2: Generate CDC events (batch 1) to trigger rotation + crash.
	// ---------------------------------------------------------------
	lm.ExecuteSourceDelta()

	matched, err := bytemanHelper.WaitForInjection(">>> BYTEMAN: crash_after_rotation", 90*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs for rotation crash")
	require.True(t, matched, "Byteman crash-after-rotation should fire")

	err = lm.WaitForExportDataExit()
	require.Error(t, err, "Export should exit with error after crash")

	// Verify at least 2 segments were created (rotation happened).
	segmentFiles, err := listQueueSegmentFiles(exportDir)
	require.NoError(t, err, "Failed to list queue segment files")
	require.GreaterOrEqual(t, len(segmentFiles), 2, "Expected >= 2 segments after rotation")

	_, lowestSegNum, highestSegNum, err := findSegmentNumRange(segmentFiles)
	require.NoError(t, err)
	t.Logf("Segments after crash: lowest=%d highest=%d count=%d", lowestSegNum, highestSegNum, len(segmentFiles))

	// ---------------------------------------------------------------
	// Phase 3: Wait for import to process the first segment (segment 0).
	// ---------------------------------------------------------------
	migrationUUID, err := lm.ReadMigrationUUID()
	require.NoError(t, err, "Failed to read migration UUID")
	require.NotEmpty(t, migrationUUID)

	// Wait for the importer to fully consume segment 0 by checking metaDB.
	testMetaDB, err := metadb.NewMetaDB(exportDir)
	require.NoError(t, err, "Failed to open metaDB")

	require.Eventually(t, func() bool {
		processed, err := testMetaDB.GetProcessedQueueSegments()
		if err != nil {
			t.Logf("Waiting for segment 0 to be processed: %v", err)
			return false
		}
		for _, seg := range processed {
			if seg.Num == 0 {
				t.Logf("Segment 0 is fully processed by importer")
				return true
			}
		}
		t.Logf("Segment 0 not yet processed (processed segments: %d)", len(processed))
		return false
	}, 120*time.Second, 3*time.Second, "Expected importer to fully process segment 0")

	// Read last_applied_vsn so we can report it in the final assertion message.
	var lastAppliedVsnBeforeArchival int64
	err = lm.WithTargetConn(func(conn *sql.DB) error {
		lastAppliedVsnBeforeArchival, err = maxLastAppliedVsn(conn, migrationUUID)
		return err
	})
	require.NoError(t, err)
	require.Greater(t, lastAppliedVsnBeforeArchival, int64(0),
		"Expected last_applied_vsn > 0 after processing segment 0")
	t.Logf("last_applied_vsn after import processes segment 0: %d", lastAppliedVsnBeforeArchival)

	// Stop importer.
	err = lm.StopImportData()
	require.NoError(t, err, "Failed to stop import")

	// ---------------------------------------------------------------
	// Phase 4: Simulate archiver — delete segment 0 file and mark it
	// as archived+deleted in the metaDB.
	// ---------------------------------------------------------------
	segment0Path := filepath.Join(exportDir, "data", "queue", "segment.0.ndjson")
	err = os.Remove(segment0Path)
	require.NoError(t, err, "Failed to delete segment 0 file (simulating archiver)")

	err = testMetaDB.MarkSegmentDeletedAndArchived(0)
	require.NoError(t, err, "Failed to mark segment 0 as deleted in metaDB")
	t.Log("Simulated archiver: segment 0 deleted and marked archived")

	// ---------------------------------------------------------------
	// Phase 5: Resume export. On recovery, the exporter will:
	//   - Open segment 1 (latest in metaDB, empty)
	//   - Try to read last VSN from segment 0 (previous segment)
	//   - Segment 0 file was deleted → creates empty file → VSN = -1
	//   - nextVSN = -1 + 1 = 0  ← BUG: VSN reset!
	// ---------------------------------------------------------------
	err = lm.StartExportData(true, nil)
	require.NoError(t, err, "Failed to resume export")

	// Insert batch 2 on source. These will get VSN starting from 0.
	lm.ExecuteOnSource(
		`INSERT INTO test_schema_vsn_reset.vsn_reset_test (name, value, payload)
		 SELECT 'batch2_' || i, 2000 + i, repeat('b', 100) FROM generate_series(1, 20) i;`,
	)

	// Wait for batch 2 events to be exported.
	time.Sleep(20 * time.Second)

	// ---------------------------------------------------------------
	// Phase 6: Resume import and verify data loss.
	// The importer has last_applied_vsn > 0 from segment 0. New events
	// in segment 1 have VSN starting from 0, so many will be skipped.
	// ---------------------------------------------------------------
	err = lm.StartImportData(true, nil)
	require.NoError(t, err, "Failed to resume import")
	defer lm.StopImportData()

	// Give the importer time to process whatever it can from segment 1.
	time.Sleep(30 * time.Second)

	// The assertion: source and target should have the same data.
	// This WILL FAIL due to the VSN reset bug — proving the data loss.
	err = lm.ValidateRowCount([]string{tableName})
	require.NoError(t, err,
		"Row count mismatch: VSN reset after segment rotation + archival caused data loss. "+
			"last_applied_vsn was %d but new events started from VSN 0, so events with VSN <= %d were skipped.",
		lastAppliedVsnBeforeArchival, lastAppliedVsnBeforeArchival)
}

// TestVSNResetDuringCutoverFallForward demonstrates the VSN-reset data-loss bug
// in the fall-forward path.
//
// The setup creates the broken state BEFORE cutover:
//  1. Source exporter fills one segment (segment 0) with CDC events, then rotates
//     → crash right after rotation leaves segment 1 empty.
//  2. Both importers (target + source-replica) fully process segment 0.
//     source-replica now has last_applied_vsn = X (X > 0).
//  3. Archiver deletes segment 0.
//  4. Resume source export → VSN resets to 0 (same recovery bug as test 1).
//  5. Initiate cutover to target.
//  6. export-data-from-target picks up the segment state, inherits the reset VSN.
//  7. New target events get VSN starting near 0, but source-replica's
//     last_applied_vsn is already X → import-to-source-replica skips them → data loss.
//
// Note: fall-back (import-to-source) is NOT affected because that importer starts
// fresh with last_applied_vsn = -1, so VSN 0 events are still accepted.
//
// Injection point:
//   - Byteman AFTER INVOKE rotateQueueSegment inside EventQueue.writeRecord
//     on the source exporter Debezium (before cutover).
func TestVSNResetDuringCutoverFallForward(t *testing.T) {

	tableName := "test_schema_ff_vsn.ff_vsn_test"
	ctx := context.Background()

	createSchemaSQL := []string{
		"DROP SCHEMA IF EXISTS test_schema_ff_vsn CASCADE;",
		"CREATE SCHEMA test_schema_ff_vsn;",
		`CREATE TABLE test_schema_ff_vsn.ff_vsn_test (
			id SERIAL PRIMARY KEY,
			name TEXT,
			value INTEGER,
			payload TEXT
		);`,
		`ALTER TABLE test_schema_ff_vsn.ff_vsn_test REPLICA IDENTITY FULL;`,
	}

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:                    ContainerConfig{Type: "postgresql", ForLive: true, DatabaseName: "postgres"},
		TargetDB:                    ContainerConfig{Type: "yugabytedb", DatabaseName: "yugabyte"},
		SourceReplicaDB:             ContainerConfig{Type: "postgresql", DatabaseName: "postgres"},
		SchemaNames:                 []string{"test_schema_ff_vsn"},
		SchemaSQL:                   createSchemaSQL,
		SourceReplicaSetupSchemaSQL: createSchemaSQL,
		InitialDataSQL: []string{
			`INSERT INTO test_schema_ff_vsn.ff_vsn_test (name, value, payload)
			SELECT 'snapshot_' || i, i, repeat('s', 100) FROM generate_series(1, 20) i;`,
		},
		SourceDeltaSQL: []string{
			// Large payloads to fill one 8KB segment quickly and force rotation.
			`INSERT INTO test_schema_ff_vsn.ff_vsn_test (name, value, payload)
			SELECT 'cdc_src_' || i, 1000 + i, repeat('a', 5000) FROM generate_series(1, 30) i;`,
		},
		TargetDeltaSQL: []string{
			`INSERT INTO test_schema_ff_vsn.ff_vsn_test (name, value, payload)
			SELECT 'cdc_target_' || i, 2000 + i, repeat('b', 100) FROM generate_series(1, 20) i;`,
		},
		CleanupSQL: []string{"DROP SCHEMA IF EXISTS test_schema_ff_vsn CASCADE;"},
	})
	defer lm.Cleanup()
	require.NoError(t, lm.SetupContainers(ctx))
	require.NoError(t, lm.SetupSchema())

	// exportDir := lm.exportDir

	// ---------------------------------------------------------------
	// Phase 1: Start source export with small segment size + Byteman
	// crash-after-rotation. Also start both importers.
	// ---------------------------------------------------------------

	// envVars := []string{"QUEUE_SEGMENT_MAX_BYTES=8192"}
	err := lm.StartExportDataWithEnv(true, nil, nil)
	require.NoError(t, err, "Failed to start export")

	// Failpoint crashes export-data-from-target before Debezium starts.
	// When import exec's into export-from-target after cutover, this fires once
	// and kills the process — leaving the empty segment state intact for the
	// VSN reset bug to manifest on the next resume.
	failpointEnv := testutils.GetFailpointEnvVar(
		"github.com/yugabyte/yb-voyager/yb-voyager/cmd/beforeDebeziumForTargetExporter=1*return()",
	)
	err = lm.StartImportDataWithEnv(true, nil, []string{failpointEnv})
	require.NoError(t, err, "Failed to start import to target")

	err = lm.WaitForSnapshotComplete(map[string]int64{
		reportTableName(tableName): 20,
	}, 120)
	require.NoError(t, err, "Snapshot phase did not complete")

	// Start source-replica importer (sets FallForwardEnabled).
	err = lm.StartImportDataToSourceReplica(true, nil)
	require.NoError(t, err, "Failed to start import to source-replica")

	err = lm.WaitForFallForwardEnabled(60)
	require.NoError(t, err, "FallForwardEnabled should be set")

	err = lm.StartArchiveChanges(false)
	require.NoError(t, err, "Failed to start archive changes")

	// ---------------------------------------------------------------
	// Phase 2: Generate CDC events
	// ---------------------------------------------------------------
	lm.ExecuteSourceDelta()

	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		reportTableName(tableName): {
			Inserts: 30,
			Updates: 0,
			Deletes: 0,
		},
	}, 60, 3)
	require.NoError(t, err, "Forward streaming did not complete")

	err = lm.WaitForFallForwardStreamingComplete([]string{tableName}, 60, 3)
	require.NoError(t, err, "Fall-forward streaming did not complete")

	// initiate cutover to target
	err = lm.InitiateCutoverToTarget(false, nil)
	require.NoError(t, err, "Failed to initiate cutover to target")

	// wait for beforeDebeziumForTargetExporter failpoint exportDataFromTarget to crash
	failMarkerPath := filepath.Join(lm.GetCurrentExportDir(), "logs", "failpoint-before-debezium-target-exporter.log")
	err = lm.WaitForImportFailpointAndProcessCrash(t, failMarkerPath, 120*time.Second, 60*time.Second)
	require.NoError(t, err, "Failed to wait for beforeDebeziumForTargetExporter failpoint exportDataFromTarget to crash")

	// check queue dir to see that segment 0 is deleted.
	queueDir := filepath.Join(lm.GetCurrentExportDir(), "data", "queue")
	require.NoFileExists(t, filepath.Join(queueDir, "segment.0.ndjson"), "Expected segment 0 to be deleted")

	//

	// run import data again without failpoint to make sure cutover is completed
	err = lm.StartImportData(true, nil)
	require.NoError(t, err, "Failed to start import")

	err = lm.WaitForCutoverComplete(0, 60)
	require.NoError(t, err, "Cutover did not complete")

	// stop import-data-to-source-replica
	err = lm.StopImportDataToSourceReplica()
	require.NoError(t, err, "Failed to stop import to source-replica")

	// start import-data-to-source-replica
	err = lm.StartImportDataToSourceReplica(true, nil)
	require.NoError(t, err, "Failed to start import to source-replica")

	// execute some events on target
	err = lm.ExecuteTargetDelta()
	require.NoError(t, err, "Failed to execute target delta")

	// waitfor fall forward streaming to complete
	err = lm.WaitForFallForwardStreamingComplete([]string{tableName}, 60, 3)
	require.NoError(t, err, "Fall-forward streaming did not complete")

}
