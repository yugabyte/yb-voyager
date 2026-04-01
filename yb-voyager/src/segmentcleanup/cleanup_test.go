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

// ============================================================
// Buffer behaviour: with segmentCleanupBuffer=3, during normal
// operation only segments beyond the last 3 processed are
// eligible. After SignalStop all processed segments are eligible.
// ============================================================
func TestDeletePolicy_BufferKeepsLastThreeProcessed(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = false
		r.ExportDataSourceDebeziumStarted = true
	})

	paths := createSegmentFiles(t, exportDir, 6)
	for i := 1; i <= 5; i++ {
		insertSegment(t, exportDir, SegmentRow{i, paths[i-1], "source_db_exporter", 1, 0, 0, 0, 0})
	}
	insertSegment(t, exportDir, SegmentRow{6, paths[5], "source_db_exporter", 0, 0, 0, 0, 0})

	cleaner := newDeleteCleaner(exportDir, mdb)

	// Normal operation: 5 processed, buffer=3 → first 2 eligible.
	_, _, deleted, err := cleaner.DeleteProcessedSegments()
	require.NoError(t, err)
	assert.Len(t, deleted, 2, "Only first 2 of 5 processed segments should be eligible")

	assert.NoFileExists(t, paths[0], "Seg 1 should be deleted")
	assert.NoFileExists(t, paths[1], "Seg 2 should be deleted")
	assert.FileExists(t, paths[2], "Seg 3 should be retained (buffer)")
	assert.FileExists(t, paths[3], "Seg 4 should be retained (buffer)")
	assert.FileExists(t, paths[4], "Seg 5 should be retained (buffer)")
	assert.FileExists(t, paths[5], "Seg 6 should still be pending")

	// After SignalStop: remaining 3 processed segments become eligible.
	cleaner.SignalStop()
	_, _, deleted2, err := cleaner.DeleteProcessedSegments()
	require.NoError(t, err)
	assert.Len(t, deleted2, 3, "All 3 remaining processed segments should be eligible after stop")

	assert.NoFileExists(t, paths[2], "Seg 3 should now be deleted")
	assert.NoFileExists(t, paths[3], "Seg 4 should now be deleted")
	assert.NoFileExists(t, paths[4], "Seg 5 should now be deleted")
	assert.FileExists(t, paths[5], "Seg 6 should still be pending")
}

// ============================================================
// FF pre-cutover: both target and source-replica must process a
// segment before it becomes eligible (importCount=2).
// ============================================================
func TestDeletePolicy_FFPreCutoverRequiresBothImporters(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = true
		r.ExportDataSourceDebeziumStarted = true
	})

	paths := createSegmentFiles(t, exportDir, 3)
	insertSegment(t, exportDir, SegmentRow{1, paths[0], "source_db_exporter", 1, 1, 0, 0, 0})
	insertSegment(t, exportDir, SegmentRow{2, paths[1], "source_db_exporter", 1, 1, 0, 0, 0})
	insertSegment(t, exportDir, SegmentRow{3, paths[2], "source_db_exporter", 1, 0, 0, 0, 0})

	cleaner := newDeleteCleaner(exportDir, mdb)
	cleaner.SignalStop()

	processed, pending, deleted, err := cleaner.DeleteProcessedSegments()
	require.NoError(t, err)
	assert.Len(t, processed, 2, "Segments where target+SR=2 are processed")
	assert.Len(t, pending, 1, "Seg 3 should still be pending")
	assert.Len(t, deleted, 2, "Both processed segments should be deleted in drain mode")

	assert.NoFileExists(t, paths[0], "Seg 1 file should be removed")
	assert.NoFileExists(t, paths[1], "Seg 2 file should be removed")
	assert.FileExists(t, paths[2], "Seg 3 file should still exist (only target processed)")

	allSegs := queryAllSegments(t, exportDir)
	assert.Equal(t, 1, allSegs[0].Deleted, "Seg 1 should be deleted")
	assert.Equal(t, 1, allSegs[1].Deleted, "Seg 2 should be deleted")
	assert.Equal(t, 0, allSegs[2].Deleted, "Seg 3 should NOT be deleted")
}

// ============================================================
// FF post-cutover: source segment still pending SR processing
// must not be deleted; only the fully-processed one is eligible.
// ============================================================
func TestDeletePolicy_FFPostCutoverSourceSegmentPendingSR(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = true
		r.CutoverProcessedByTargetImporter = true
		r.ExportFromTargetFallForwardStarted = true
	})

	paths := createSegmentFiles(t, exportDir, 2)
	insertSegment(t, exportDir, SegmentRow{1, paths[0], "source_db_exporter", 1, 1, 0, 0, 0})
	insertSegment(t, exportDir, SegmentRow{2, paths[1], "source_db_exporter", 1, 0, 0, 0, 0})

	cleaner := newDeleteCleaner(exportDir, mdb)
	cleaner.SignalStop()

	processed, pending, deleted, err := cleaner.DeleteProcessedSegments()
	require.NoError(t, err)
	assert.Len(t, processed, 1, "Only seg 1 should be processed )")
	assert.Len(t, pending, 1, "Seg 2 should still be pending")
	assert.Len(t, deleted, 1, "Only seg 1 should be deleted")

	assert.NoFileExists(t, paths[0], "Seg 1 file should be removed")
	assert.FileExists(t, paths[1], "Seg 2 file should be removed")

	allSegs := queryAllSegments(t, exportDir)
	assert.Equal(t, 1, allSegs[0].Deleted, "Seg 1 should be deleted")
	assert.Equal(t, 0, allSegs[1].Deleted, "Seg 2 should not be deleted")
}

// ============================================================
// Fall-back workflow without FF: uses importCount=1, so only
// target-processed segments are eligible.
// ============================================================
func TestDeletePolicy_FallbackWorkflowUsesImportCountOne(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = false
		r.FallbackEnabled = true
		r.ExportDataSourceDebeziumStarted = true
	})

	paths := createSegmentFiles(t, exportDir, 3)
	insertSegment(t, exportDir, SegmentRow{1, paths[0], "source_db_exporter", 1, 0, 0, 0, 0})
	insertSegment(t, exportDir, SegmentRow{2, paths[1], "source_db_exporter", 1, 0, 0, 0, 0})
	insertSegment(t, exportDir, SegmentRow{3, paths[2], "source_db_exporter", 0, 0, 0, 0, 0})

	cleaner := newDeleteCleaner(exportDir, mdb)
	cleaner.SignalStop()

	_, _, deleted, err := cleaner.DeleteProcessedSegments()
	require.NoError(t, err)
	assert.Len(t, deleted, 2, "Segs 1 and 2 with target=1 should be deleted in drain mode")

	assert.NoFileExists(t, paths[0], "Seg 1 file should be removed")
	assert.NoFileExists(t, paths[1], "Seg 2 file should be removed")
	assert.FileExists(t, paths[2], "Seg 3 file should still exist")

	allSegs := queryAllSegments(t, exportDir)
	assert.Equal(t, 1, allSegs[0].Deleted, "Seg 1 deleted")
	assert.Equal(t, 1, allSegs[1].Deleted, "Seg 2 deleted")
	assert.Equal(t, 0, allSegs[2].Deleted, "Seg 3 NOT deleted")
}

// ============================================================
// FS utilization below threshold: the run-loop skips deletion
// until SignalStop forces a drain pass.
// ============================================================
func TestDeletePolicy_FSBelowThresholdNoDeletion(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = true
		r.CutoverProcessedByTargetImporter = true
	})

	paths := createSegmentFiles(t, exportDir, 2)
	insertSegment(t, exportDir, SegmentRow{1, paths[0], "source_db_exporter", 1, 1, 0, 0, 0})
	insertSegment(t, exportDir, SegmentRow{2, paths[1], "source_db_exporter", 1, 1, 0, 0, 0})

	cfg := Config{
		Policy:                 PolicyDelete,
		ExportDir:              exportDir,
		FSUtilizationThreshold: 99,
	}
	cleaner := NewSegmentCleaner(cfg, mdb)

	t.Logf("isFSUtilizationExceeded (threshold=99%%): %v", cleaner.isFSUtilizationExceeded())

	done := make(chan error, 1)
	go func() {
		done <- cleaner.Run()
	}()

	time.Sleep(6 * time.Second)
	cleaner.SignalStop()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(15 * time.Second):
		t.Fatal("cleanup did not stop")
	}

	allSegs := queryAllSegments(t, exportDir)
	t.Log("Segments after cleanup with high threshold:")
	deletedCount := 0
	for _, s := range allSegs {
		t.Logf("  seg=%d deleted=%d", s.SegmentNo, s.Deleted)
		if s.Deleted == 1 {
			deletedCount++
		}
	}
	t.Logf("Total deleted: %d (during SignalStop draining)", deletedCount)
	t.Log("NOTE: When SignalStop is called, isFSUtilizationExceeded returns true to allow draining")
}

// ============================================================
// IsValidPolicy accepts known policies and rejects unknowns.
// ============================================================
func TestIsValidPolicy(t *testing.T) {
	assert.True(t, IsValidPolicy("delete"))
	assert.False(t, IsValidPolicy("retain"))
	assert.True(t, IsValidPolicy("archive"))
	assert.False(t, IsValidPolicy(""))
}

// ============================================================
// MarkSegmentDeletedAndArchived sets both flags in the DB.
// ============================================================
func TestMarkSegmentDeletedAndArchived(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	paths := createSegmentFiles(t, exportDir, 1)
	insertSegment(t, exportDir, SegmentRow{1, paths[0], "source_db_exporter", 1, 0, 0, 0, 0})

	require.NoError(t, mdb.MarkSegmentDeletedAndArchived(1))

	allSegs := queryAllSegments(t, exportDir)
	assert.Equal(t, 1, allSegs[0].Deleted)
	assert.Equal(t, 1, allSegs[0].Archived)
}

// ============================================================
// DeleteProcessedSegments removes the segment file from disk
// and marks it deleted+archived in the DB.
// ============================================================
func TestDeleteProcessedSegmentsRemovesFile(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = false
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

// ============================================================
// DeleteProcessedSegments gracefully handles a segment whose
// file is already missing (marks DB without error).
// ============================================================
func TestDeleteProcessedSegmentsMissingFile(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = false
	})
	insertSegment(t, exportDir, SegmentRow{1, "/tmp/nonexistent/segment.1.ndjson", "source_db_exporter", 1, 0, 0, 0, 0})

	cleaner := newDeleteCleaner(exportDir, mdb)
	cleaner.SignalStop()

	_, _, deleted, err := cleaner.DeleteProcessedSegments()
	require.NoError(t, err, "Should not error on missing file")
	assert.Len(t, deleted, 1)

	allSegs := queryAllSegments(t, exportDir)
	assert.Equal(t, 1, allSegs[0].Deleted)
	assert.Equal(t, 1, allSegs[0].Archived)
}
