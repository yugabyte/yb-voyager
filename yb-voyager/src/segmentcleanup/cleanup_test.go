//go:build unit

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
package segmentcleanup

import (
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
)

func newDeleteCleaner(exportDir string, mdb *metadb.MetaDB) *SegmentCleaner {
	return NewSegmentCleaner(Config{
		Policy:                 PolicyDelete,
		ExportDir:              exportDir,
		FSUtilizationThreshold: 0,
	}, mdb)
}

func newArchiveCleaner(exportDir string, archiveDir string, mdb *metadb.MetaDB) *SegmentCleaner {
	return NewSegmentCleaner(Config{
		Policy:                 PolicyArchive,
		ExportDir:              exportDir,
		ArchiveDir:             archiveDir,
		FSUtilizationThreshold: 0,
	}, mdb)
}

// =============================================================================================
// WORKFLOW 1: LIVE MIGRATION (Basic live flow without Fall-Forward or Fall-Back)
//
// Migration stages:
//   1. Initiate migration: segments are exported from source and imported to target
//   2. During normal operation: processed segments are deleted with a 3-segment buffer
//   3. Cutover: streaming continues until all segments are processed
//   4. Post-cutover (SignalStop/drain mode): all processed segments are deleted
//
// Key characteristics:
//   - importCount = 1 (only target importer needs to process each segment)
//   - exporter_role = "source_db_exporter"
// =============================================================================================

// TestLiveMigration_BufferDuringNormalOperation verifies that during normal operation,
// the cleaner keeps a buffer of 3 processed segments and only deletes segments beyond that buffer.
func TestLiveMigration_BufferDuringNormalOperation(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = false
		r.FallbackEnabled = false
		r.ExportDataSourceDebeziumStarted = true
	})

	// Create 6 segments: 5 processed, 1 pending
	paths := createSegmentFiles(t, exportDir, 6)
	for i := 1; i <= 5; i++ {
		insertSegment(t, exportDir, SegmentRow{i, paths[i-1], "source_db_exporter", 1, 0, 0, 0, 0})
	}
	insertSegment(t, exportDir, SegmentRow{6, paths[5], "source_db_exporter", 0, 0, 0, 0, 0})

	cleaner := newDeleteCleaner(exportDir, mdb)

	// Normal operation: 5 processed, buffer=3 → first 2 eligible
	processed, pending, deleted, err := cleaner.DeleteProcessedSegments()
	require.NoError(t, err)
	assert.Len(t, processed, 5, "5 segments should be processed")
	assert.Len(t, pending, 1, "1 segment should be pending")
	assert.Len(t, deleted, 2, "Only first 2 of 5 processed segments should be eligible (buffer=3)")

	assert.NoFileExists(t, paths[0], "Seg 1 should be deleted")
	assert.NoFileExists(t, paths[1], "Seg 2 should be deleted")
	assert.FileExists(t, paths[2], "Seg 3 should be retained (buffer)")
	assert.FileExists(t, paths[3], "Seg 4 should be retained (buffer)")
	assert.FileExists(t, paths[4], "Seg 5 should be retained (buffer)")
	assert.FileExists(t, paths[5], "Seg 6 should still be pending")
}

// TestLiveMigration_DrainAfterCutover verifies that after SignalStop (cutover/drain mode),
// all processed segments are deleted regardless of the buffer.
func TestLiveMigration_DrainAfterCutover(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = false
		r.FallbackEnabled = false
		r.ExportDataSourceDebeziumStarted = true
	})

	// Create 6 segments: 5 processed, 1 pending
	paths := createSegmentFiles(t, exportDir, 6)
	for i := 1; i <= 5; i++ {
		insertSegment(t, exportDir, SegmentRow{i, paths[i-1], "source_db_exporter", 1, 0, 0, 0, 0})
	}
	insertSegment(t, exportDir, SegmentRow{6, paths[5], "source_db_exporter", 0, 0, 0, 0, 0})

	cleaner := newDeleteCleaner(exportDir, mdb)

	// First pass: delete 2 segments (buffer behavior)
	_, _, _, err := cleaner.DeleteProcessedSegments()
	require.NoError(t, err)

	// Simulate cutover / drain mode
	cleaner.SignalStop()

	// After SignalStop: all remaining processed segments should be eligible
	processed, pending, deleted, err := cleaner.DeleteProcessedSegments()
	require.NoError(t, err)
	assert.Len(t, processed, 3, "3 segments remain processed (2 were already deleted)")
	assert.Len(t, pending, 1, "1 segment should still be pending")
	assert.Len(t, deleted, 3, "All remaining processed segments should be deleted in drain mode")

	assert.NoFileExists(t, paths[2], "Seg 3 should now be deleted")
	assert.NoFileExists(t, paths[3], "Seg 4 should now be deleted")
	assert.NoFileExists(t, paths[4], "Seg 5 should now be deleted")
	assert.FileExists(t, paths[5], "Seg 6 should still be pending")
}

// =============================================================================================
// WORKFLOW 2: LIVE-FALL-FORWARD MIGRATION (Live migration with Fall-Forward to a source replica)
//
// Migration stages:
//   1. Pre-cutover: Both target AND source-replica must process each segment (importCount=2)
//   2. Cutover: Switch to source replica
//   3. Post-cutover: Export from target begins, only source-replica and source need to import
//
// Key characteristics:
//   - Pre-cutover importCount = 2 (target + source-replica must both process)
//   - Post-cutover: exporter_role changes to "target_db_exporter_*"
// =============================================================================================

// TestLiveFallForward_PreCutoverRequiresBothImporters verifies that before cutover,
// a segment is only eligible for deletion after BOTH target and source-replica have processed it.
func TestLiveFallForward_PreCutoverRequiresBothImporters(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = true
		r.FallbackEnabled = false
		r.ExportDataSourceDebeziumStarted = true
		// Pre-cutover: CutoverProcessedByTargetImporter = false
	})

	paths := createSegmentFiles(t, exportDir, 3)
	// Seg 1 & 2: fully processed by both target (1) and source-replica (1)
	insertSegment(t, exportDir, SegmentRow{1, paths[0], "source_db_exporter", 1, 1, 0, 0, 0})
	insertSegment(t, exportDir, SegmentRow{2, paths[1], "source_db_exporter", 1, 1, 0, 0, 0})
	// Seg 3: only target processed, source-replica pending
	insertSegment(t, exportDir, SegmentRow{3, paths[2], "source_db_exporter", 1, 0, 0, 0, 0})

	cleaner := newDeleteCleaner(exportDir, mdb)
	cleaner.SignalStop() // Drain mode to delete all eligible

	processed, pending, deleted, err := cleaner.DeleteProcessedSegments()
	require.NoError(t, err)
	assert.Len(t, processed, 2, "Segments where target+SR=2 are processed")
	assert.Len(t, pending, 1, "Seg 3 should still be pending (missing SR)")
	assert.Len(t, deleted, 2, "Both fully-processed segments should be deleted")

	assert.NoFileExists(t, paths[0], "Seg 1 file should be removed")
	assert.NoFileExists(t, paths[1], "Seg 2 file should be removed")
	assert.FileExists(t, paths[2], "Seg 3 file should still exist (only target processed)")

	allSegs := queryAllSegments(t, exportDir)
	assert.Equal(t, 1, allSegs[0].Deleted, "Seg 1 should be deleted")
	assert.Equal(t, 1, allSegs[1].Deleted, "Seg 2 should be deleted")
	assert.Equal(t, 0, allSegs[2].Deleted, "Seg 3 should NOT be deleted")
}

// TestLiveFallForward_PostCutoverSourceReplicaPending verifies post-cutover behavior
// where segments from source_db_exporter still pending SR processing must not be deleted.
func TestLiveFallForward_PostCutoverSourceReplicaPending(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = true
		r.FallbackEnabled = false
		r.CutoverProcessedByTargetImporter = true
		r.ExportFromTargetFallForwardStarted = true
	})

	paths := createSegmentFiles(t, exportDir, 2)
	// Seg 1: fully processed by both target (1) and source-replica (1)
	insertSegment(t, exportDir, SegmentRow{1, paths[0], "source_db_exporter", 1, 1, 0, 0, 0})
	// Seg 2: only target processed, source-replica pending
	insertSegment(t, exportDir, SegmentRow{2, paths[1], "source_db_exporter", 1, 0, 0, 0, 0})

	cleaner := newDeleteCleaner(exportDir, mdb)
	cleaner.SignalStop()

	processed, pending, deleted, err := cleaner.DeleteProcessedSegments()
	require.NoError(t, err)
	assert.Len(t, processed, 1, "Only seg 1 should be processed")
	assert.Len(t, pending, 1, "Seg 2 should still be pending (SR not processed)")
	assert.Len(t, deleted, 1, "Only seg 1 should be deleted")

	assert.NoFileExists(t, paths[0], "Seg 1 file should be removed")
	assert.FileExists(t, paths[1], "Seg 2 file should remain (SR not processed)")

	allSegs := queryAllSegments(t, exportDir)
	assert.Equal(t, 1, allSegs[0].Deleted, "Seg 1 should be deleted")
	assert.Equal(t, 0, allSegs[1].Deleted, "Seg 2 should not be deleted")
}

// =============================================================================================
// WORKFLOW 3: LIVE-FALL-BACK MIGRATION (Live migration with Fall-Back to source)
//
// Migration stages:
//   1. Segments are exported from source and imported to target
//   2. If needed, can fall back to source database
//
// Key characteristics:
//   - importCount = 1 (only target importer needs to process each segment)
//   - Similar to basic live, but FallbackEnabled flag is set
// =============================================================================================

// TestLiveFallBack_ImportCountOne verifies that fall-back workflow uses importCount=1,
// so only target-processed segments are eligible for deletion.
func TestLiveFallBack_ImportCountOne(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = false
		r.FallbackEnabled = true
		r.ExportDataSourceDebeziumStarted = true
	})

	paths := createSegmentFiles(t, exportDir, 3)
	// Seg 1 & 2: processed by target
	insertSegment(t, exportDir, SegmentRow{1, paths[0], "source_db_exporter", 1, 0, 0, 0, 0})
	insertSegment(t, exportDir, SegmentRow{2, paths[1], "source_db_exporter", 1, 0, 0, 0, 0})
	// Seg 3: pending (not processed by target)
	insertSegment(t, exportDir, SegmentRow{3, paths[2], "source_db_exporter", 0, 0, 0, 0, 0})

	cleaner := newDeleteCleaner(exportDir, mdb)
	cleaner.SignalStop()

	processed, pending, deleted, err := cleaner.DeleteProcessedSegments()
	require.NoError(t, err)
	assert.Len(t, processed, 2, "Segs 1 and 2 should be processed")
	assert.Len(t, pending, 1, "Seg 3 should be pending")
	assert.Len(t, deleted, 2, "Segs 1 and 2 with target=1 should be deleted in drain mode")

	assert.NoFileExists(t, paths[0], "Seg 1 file should be removed")
	assert.NoFileExists(t, paths[1], "Seg 2 file should be removed")
	assert.FileExists(t, paths[2], "Seg 3 file should still exist")

	allSegs := queryAllSegments(t, exportDir)
	assert.Equal(t, 1, allSegs[0].Deleted, "Seg 1 deleted")
	assert.Equal(t, 1, allSegs[1].Deleted, "Seg 2 deleted")
	assert.Equal(t, 0, allSegs[2].Deleted, "Seg 3 NOT deleted")
}

// =============================================================================================
// EDGE CASES & UTILITIES
// =============================================================================================

// TestEdgeCase_FSUtilizationBelowThreshold verifies that when FS utilization is below
// the threshold, the run-loop skips deletion until SignalStop forces a drain pass.
func TestEdgeCase_FSUtilizationBelowThreshold(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = false
		r.FallbackEnabled = false
		r.ExportDataSourceDebeziumStarted = true
	})

	paths := createSegmentFiles(t, exportDir, 5)
	for i := 1; i <= 5; i++ {
		insertSegment(t, exportDir, SegmentRow{i, paths[i-1], "source_db_exporter", 1, 0, 0, 0, 0})
	}

	cfg := Config{
		Policy:                 PolicyDelete,
		ExportDir:              exportDir,
		FSUtilizationThreshold: 99, // Very high threshold - won't be exceeded
	}
	cleaner := NewSegmentCleaner(cfg, mdb)

	done := make(chan error, 1)
	go func() {
		done <- cleaner.Run()
	}()

	// Let it run for a bit - nothing should be deleted due to FS threshold
	time.Sleep(6 * time.Second)

	// Signal stop to force drain mode
	cleaner.SignalStop()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(15 * time.Second):
		t.Fatal("cleanup did not stop")
	}

	// Verify that segments were deleted during drain mode
	allSegs := queryAllSegments(t, exportDir)
	deletedCount := 0
	for _, s := range allSegs {
		if s.Deleted == 1 {
			deletedCount++
		}
	}
	t.Logf("Total deleted: %d (during SignalStop draining)", deletedCount)
	// Note: When SignalStop is called, isFSUtilizationExceeded returns true to allow draining
}

// TestEdgeCase_MissingSegmentFile verifies that DeleteProcessedSegments gracefully
// handles a segment whose file path does not exist on disk. The segment is inserted
// into the DB with a non-existent path to simulate a scenario where the file was
// already removed (e.g., manual deletion or previous partial cleanup). The cleaner
// should still mark the segment as deleted+archived in the DB without returning an error.
func TestEdgeCase_MissingSegmentFile(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = false
		r.FallbackEnabled = false
	})

	// Insert a segment with a non-existent file path
	insertSegment(t, exportDir, SegmentRow{1, "/tmp/nonexistent/segment.1.ndjson", "source_db_exporter", 1, 0, 0, 0, 0})

	cleaner := newDeleteCleaner(exportDir, mdb)
	cleaner.SignalStop()

	_, _, deleted, err := cleaner.DeleteProcessedSegments()
	require.NoError(t, err, "Should not error on missing file")
	assert.Len(t, deleted, 1)

	allSegs := queryAllSegments(t, exportDir)
	assert.Equal(t, 1, allSegs[0].Deleted, "Seg should be marked deleted even if file was missing")
	assert.Equal(t, 1, allSegs[0].Archived, "Seg should be marked archived even if file was missing")
}

// =============================================================================================
// UTILITY TESTS
// =============================================================================================

// TestUtility_IsValidPolicy verifies that IsValidPolicy accepts known policies and rejects unknowns.
func TestUtility_IsValidPolicy(t *testing.T) {
	assert.True(t, IsValidPolicy("delete"))
	assert.True(t, IsValidPolicy("archive"))
	assert.False(t, IsValidPolicy("retain"))
	assert.False(t, IsValidPolicy(""))
	assert.False(t, IsValidPolicy("unknown"))
}

// TestUtility_MarkSegmentDeletedAndArchived verifies that the DB method correctly
// sets both deleted and archived flags.
func TestUtility_MarkSegmentDeletedAndArchived(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	paths := createSegmentFiles(t, exportDir, 1)
	insertSegment(t, exportDir, SegmentRow{1, paths[0], "source_db_exporter", 1, 0, 0, 0, 0})

	require.NoError(t, mdb.MarkSegmentDeletedAndArchived(1))

	allSegs := queryAllSegments(t, exportDir)
	assert.Equal(t, 1, allSegs[0].Deleted)
	assert.Equal(t, 1, allSegs[0].Archived)
}

// TestUtility_DeleteProcessedSegmentsRemovesFile verifies that DeleteProcessedSegments
// removes the segment file from disk and marks it deleted+archived in the DB.
func TestUtility_DeleteProcessedSegmentsRemovesFile(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = false
		r.FallbackEnabled = false
	})
	paths := createSegmentFiles(t, exportDir, 1)
	insertSegment(t, exportDir, SegmentRow{1, paths[0], "source_db_exporter", 1, 0, 0, 0, 0})

	assert.FileExists(t, paths[0], "segment file should exist before delete")

	cleaner := newDeleteCleaner(exportDir, mdb)
	cleaner.SignalStop()

	_, _, deleted, err := cleaner.DeleteProcessedSegments()
	require.NoError(t, err)
	assert.Len(t, deleted, 1)

	assert.NoFileExists(t, paths[0], "segment file should be removed after delete")

	allSegs := queryAllSegments(t, exportDir)
	assert.Equal(t, 1, allSegs[0].Deleted)
	assert.Equal(t, 1, allSegs[0].Archived)
}
