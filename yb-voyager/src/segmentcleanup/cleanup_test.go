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
	"path/filepath"
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

// #############################################################################################
//
//	DELETE POLICY TESTS
//
// #############################################################################################

// =============================================================================================
// WORKFLOW 1: LIVE MIGRATION
//   - importCount = 1, exporter_role = "source_db_exporter"
//   - Processed when imported_by_target = 1
// =============================================================================================

func TestDeletePolicy_LiveMigration(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = false
		r.FallbackEnabled = false
		r.ExportDataSourceDebeziumStarted = true
	})

	//   Segs 1-5: processed (imported_by_target=1)
	//   Segs 6-7: pending   (imported_by_target=0)
	paths := createSegmentFiles(t, exportDir, 7)
	for i := 1; i <= 5; i++ {
		insertSegment(t, exportDir, SegmentRow{i, paths[i-1], "source_db_exporter", 1, 0, 0, 0, 0})
	}
	for i := 6; i <= 7; i++ {
		insertSegment(t, exportDir, SegmentRow{i, paths[i-1], "source_db_exporter", 0, 0, 0, 0, 0})
	}

	cleaner := newDeleteCleaner(exportDir, mdb)

	// Normal operation: 5 processed − buffer(3) = 2 eligible (segs 1,2).
	processed, pending, deleted, err := cleaner.DeleteProcessedSegments()
	require.NoError(t, err)
	assert.Len(t, processed, 5)
	assert.Len(t, pending, 2)
	assert.Len(t, deleted, 2)
	assert.NoFileExists(t, paths[0], "Seg 1 should be deleted")
	assert.NoFileExists(t, paths[1], "Seg 2 should be deleted")
	for i := 2; i < 7; i++ {
		assert.FileExists(t, paths[i])
	}

	// SignalStop (cutover): buffer disabled, remaining 3 processed drained.
	cleaner.SignalStop()
	processed, pending, deleted, err = cleaner.DeleteProcessedSegments()
	require.NoError(t, err)
	assert.Len(t, processed, 3)
	assert.Len(t, pending, 2)
	assert.Len(t, deleted, 3)
	for i := 0; i < 5; i++ {
		assert.NoFileExists(t, paths[i])
	}
	assert.FileExists(t, paths[5], "Seg 6 (pending) untouched")
	assert.FileExists(t, paths[6], "Seg 7 (pending) untouched")
}

// =============================================================================================
// WORKFLOW 2: LIVE-FALL-FORWARD MIGRATION
//   - Pre-cutover importCount = 2 (target + sr must both process)
//   - Post-cutover: target_db_exporter_ff segments, processed when sr+source = 1
// =============================================================================================

func TestDeletePolicy_LiveFallForward_PreCutover(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = true
		r.FallbackEnabled = false
		r.ExportDataSourceDebeziumStarted = true
	})

	//   Segs 1-5: fully processed (target=1, sr=1, sum=2)
	//   Seg 6:    partially processed (target=1, sr=0, sum=1 < 2) → pending
	//   Seg 7:    unprocessed (target=0, sr=0) → pending
	paths := createSegmentFiles(t, exportDir, 7)
	for i := 1; i <= 5; i++ {
		insertSegment(t, exportDir, SegmentRow{i, paths[i-1], "source_db_exporter", 1, 1, 0, 0, 0})
	}
	insertSegment(t, exportDir, SegmentRow{6, paths[5], "source_db_exporter", 1, 0, 0, 0, 0})
	insertSegment(t, exportDir, SegmentRow{7, paths[6], "source_db_exporter", 0, 0, 0, 0, 0})

	cleaner := newDeleteCleaner(exportDir, mdb)

	// Normal operation: 5 processed − buffer(3) = 2 eligible (segs 1,2).
	processed, pending, deleted, err := cleaner.DeleteProcessedSegments()
	require.NoError(t, err)
	assert.Len(t, processed, 5)
	assert.Len(t, pending, 2, "seg 6 (partial) + seg 7 (unprocessed)")
	assert.Len(t, deleted, 2)
	assert.NoFileExists(t, paths[0])
	assert.NoFileExists(t, paths[1])
	for i := 2; i < 7; i++ {
		assert.FileExists(t, paths[i])
	}

	// SignalStop: remaining 3 processed drained, partial seg 6 stays pending.
	cleaner.SignalStop()
	processed, pending, deleted, err = cleaner.DeleteProcessedSegments()
	require.NoError(t, err)
	assert.Len(t, processed, 3)
	assert.Len(t, pending, 2)
	assert.Len(t, deleted, 3)
	for i := 0; i < 5; i++ {
		assert.NoFileExists(t, paths[i])
	}
	assert.FileExists(t, paths[5], "Seg 6 (target=1 but sr=0) still pending — FF needs both")
	assert.FileExists(t, paths[6], "Seg 7 (unprocessed) still pending")
}

func TestDeletePolicy_LiveFallForward_PostCutover(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = true
		r.FallbackEnabled = false
		r.CutoverProcessedByTargetImporter = true
		r.ExportFromTargetFallForwardStarted = true
	})

	//   Segs 1-5: SR has processed (sr=1) → processed
	//   Segs 6-7: SR hasn't processed yet (sr=0) → pending
	paths := createSegmentFiles(t, exportDir, 7)
	for i := 1; i <= 5; i++ {
		insertSegment(t, exportDir, SegmentRow{i, paths[i-1], "target_db_exporter_ff", 0, 1, 0, 0, 0})
	}
	for i := 6; i <= 7; i++ {
		insertSegment(t, exportDir, SegmentRow{i, paths[i-1], "target_db_exporter_ff", 0, 0, 0, 0, 0})
	}

	cleaner := newDeleteCleaner(exportDir, mdb)

	// Normal operation: 5 processed − buffer(3) = 2 eligible (segs 1,2).
	processed, pending, deleted, err := cleaner.DeleteProcessedSegments()
	require.NoError(t, err)
	assert.Len(t, processed, 5)
	assert.Len(t, pending, 2)
	assert.Len(t, deleted, 2)
	assert.NoFileExists(t, paths[0])
	assert.NoFileExists(t, paths[1])
	for i := 2; i < 7; i++ {
		assert.FileExists(t, paths[i])
	}

	// SignalStop: remaining 3 processed (segs 3,4,5) all eligible.
	cleaner.SignalStop()
	processed, pending, deleted, err = cleaner.DeleteProcessedSegments()
	require.NoError(t, err)
	assert.Len(t, processed, 3)
	assert.Len(t, pending, 2)
	assert.Len(t, deleted, 3)
	for i := 0; i < 5; i++ {
		assert.NoFileExists(t, paths[i])
	}
	assert.FileExists(t, paths[5], "Seg 6 (SR pending) should remain")
	assert.FileExists(t, paths[6], "Seg 7 (SR pending) should remain")
}

// =============================================================================================
// WORKFLOW 3: LIVE-FALL-BACK MIGRATION
//   - Pre-cutover: importCount = 1, source_db_exporter, processed when target = 1
//   - Post-cutover: target_db_exporter_fb segments, processed when sr+source = 1
// =============================================================================================

func TestDeletePolicy_LiveFallBack_PreCutover(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = false
		r.FallbackEnabled = true
		r.ExportDataSourceDebeziumStarted = true
	})

	//   Segs 1-5: processed (imported_by_target=1)
	//   Segs 6-7: pending   (imported_by_target=0)
	paths := createSegmentFiles(t, exportDir, 7)
	for i := 1; i <= 5; i++ {
		insertSegment(t, exportDir, SegmentRow{i, paths[i-1], "source_db_exporter", 1, 0, 0, 0, 0})
	}
	for i := 6; i <= 7; i++ {
		insertSegment(t, exportDir, SegmentRow{i, paths[i-1], "source_db_exporter", 0, 0, 0, 0, 0})
	}

	cleaner := newDeleteCleaner(exportDir, mdb)

	// Normal operation: 5 processed − buffer(3) = 2 eligible (segs 1,2).
	processed, pending, deleted, err := cleaner.DeleteProcessedSegments()
	require.NoError(t, err)
	assert.Len(t, processed, 5)
	assert.Len(t, pending, 2)
	assert.Len(t, deleted, 2)
	assert.NoFileExists(t, paths[0])
	assert.NoFileExists(t, paths[1])
	for i := 2; i < 7; i++ {
		assert.FileExists(t, paths[i])
	}

	// SignalStop: remaining 3 processed drained.
	cleaner.SignalStop()
	processed, pending, deleted, err = cleaner.DeleteProcessedSegments()
	require.NoError(t, err)
	assert.Len(t, processed, 3)
	assert.Len(t, pending, 2)
	assert.Len(t, deleted, 3)
	for i := 0; i < 5; i++ {
		assert.NoFileExists(t, paths[i])
	}
	assert.FileExists(t, paths[5], "Seg 6 (pending) untouched")
	assert.FileExists(t, paths[6], "Seg 7 (pending) untouched")
}

func TestDeletePolicy_LiveFallBack_PostCutover(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = false
		r.FallbackEnabled = true
		r.ExportDataSourceDebeziumStarted = true
		r.CutoverProcessedByTargetImporter = true
		r.ExportFromTargetFallBackStarted = true
	})

	paths := createSegmentFiles(t, exportDir, 7)

	// source_db_exporter segments still in DB (processed by target, awaiting cleanup):
	//   Segs 1-2: processed (target=1, importCount=1 → sum=1)
	insertSegment(t, exportDir, SegmentRow{1, paths[0], "source_db_exporter", 1, 0, 0, 0, 0})
	insertSegment(t, exportDir, SegmentRow{2, paths[1], "source_db_exporter", 1, 0, 0, 0, 0})

	// target_db_exporter_fb segments (processed when sr+source=1):
	//   Segs 3-5: processed (imported_by_source=1)
	//   Segs 6-7: pending   (imported_by_source=0)
	for i := 3; i <= 5; i++ {
		insertSegment(t, exportDir, SegmentRow{i, paths[i-1], "target_db_exporter_fb", 0, 0, 1, 0, 0})
	}
	for i := 6; i <= 7; i++ {
		insertSegment(t, exportDir, SegmentRow{i, paths[i-1], "target_db_exporter_fb", 0, 0, 0, 0, 0})
	}

	cleaner := newDeleteCleaner(exportDir, mdb)

	// Normal operation: processed=[1,2,3,4,5](5) − buffer(3) = 2 eligible (segs 1,2).
	processed, pending, deleted, err := cleaner.DeleteProcessedSegments()
	require.NoError(t, err)
	assert.Len(t, processed, 5)
	assert.Len(t, pending, 2)
	assert.Len(t, deleted, 2)
	assert.NoFileExists(t, paths[0], "Seg 1 (source, processed) deleted")
	assert.NoFileExists(t, paths[1], "Seg 2 (source, processed) deleted")
	for i := 2; i < 7; i++ {
		assert.FileExists(t, paths[i])
	}

	// SignalStop: remaining processed=[3,4,5](3), all eligible.
	cleaner.SignalStop()
	processed, pending, deleted, err = cleaner.DeleteProcessedSegments()
	require.NoError(t, err)
	assert.Len(t, processed, 3)
	assert.Len(t, pending, 2)
	assert.Len(t, deleted, 3)
	for i := 0; i < 5; i++ {
		assert.NoFileExists(t, paths[i])
	}
	assert.FileExists(t, paths[5], "Seg 6 (fb, source pending) untouched")
	assert.FileExists(t, paths[6], "Seg 7 (fb, source pending) untouched")
}

// #############################################################################################
//
//	ARCHIVE POLICY TESTS
//
// #############################################################################################

// archivePath returns the expected archive location for a given export-dir segment path.
func archivePath(archiveDir, segPath string) string {
	return filepath.Join(archiveDir, filepath.Base(segPath))
}

// =============================================================================================
// WORKFLOW 1: LIVE MIGRATION (archive)
// =============================================================================================

func TestArchivePolicy_LiveMigration(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	archiveDir := createArchiveDir(t, exportDir)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = false
		r.FallbackEnabled = false
		r.ExportDataSourceDebeziumStarted = true
	})

	paths := createSegmentFiles(t, exportDir, 7)
	for i := 1; i <= 5; i++ {
		insertSegment(t, exportDir, SegmentRow{i, paths[i-1], "source_db_exporter", 1, 0, 0, 0, 0})
	}
	for i := 6; i <= 7; i++ {
		insertSegment(t, exportDir, SegmentRow{i, paths[i-1], "source_db_exporter", 0, 0, 0, 0, 0})
	}

	cleaner := newArchiveCleaner(exportDir, archiveDir, mdb)

	// Normal operation: 5 processed − buffer(3) = 2 eligible (segs 1,2).
	processed, pending, archived, err := cleaner.ArchiveProcessedSegments()
	require.NoError(t, err)
	assert.Len(t, processed, 5)
	assert.Len(t, pending, 2)
	assert.Len(t, archived, 2)
	assert.NoFileExists(t, paths[0], "Seg 1 removed from export dir")
	assert.NoFileExists(t, paths[1], "Seg 2 removed from export dir")
	assert.FileExists(t, archivePath(archiveDir, paths[0]), "Seg 1 archived")
	assert.FileExists(t, archivePath(archiveDir, paths[1]), "Seg 2 archived")
	for i := 2; i < 7; i++ {
		assert.FileExists(t, paths[i])
		assert.NoFileExists(t, archivePath(archiveDir, paths[i]))
	}

	// SignalStop: remaining 3 processed drained.
	cleaner.SignalStop()
	processed, pending, archived, err = cleaner.ArchiveProcessedSegments()
	require.NoError(t, err)
	assert.Len(t, processed, 3)
	assert.Len(t, pending, 2)
	assert.Len(t, archived, 3)
	for i := 0; i < 5; i++ {
		assert.NoFileExists(t, paths[i])
		assert.FileExists(t, archivePath(archiveDir, paths[i]))
	}
	assert.FileExists(t, paths[5], "Seg 6 (pending) untouched")
	assert.FileExists(t, paths[6], "Seg 7 (pending) untouched")
}

// =============================================================================================
// WORKFLOW 2: LIVE-FALL-FORWARD MIGRATION (archive)
// =============================================================================================

func TestArchivePolicy_LiveFallForward_PreCutover(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	archiveDir := createArchiveDir(t, exportDir)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = true
		r.FallbackEnabled = false
		r.ExportDataSourceDebeziumStarted = true
	})

	paths := createSegmentFiles(t, exportDir, 7)
	for i := 1; i <= 5; i++ {
		insertSegment(t, exportDir, SegmentRow{i, paths[i-1], "source_db_exporter", 1, 1, 0, 0, 0})
	}
	insertSegment(t, exportDir, SegmentRow{6, paths[5], "source_db_exporter", 1, 0, 0, 0, 0})
	insertSegment(t, exportDir, SegmentRow{7, paths[6], "source_db_exporter", 0, 0, 0, 0, 0})

	cleaner := newArchiveCleaner(exportDir, archiveDir, mdb)

	// Normal operation: 5 processed − buffer(3) = 2 eligible.
	processed, pending, archived, err := cleaner.ArchiveProcessedSegments()
	require.NoError(t, err)
	assert.Len(t, processed, 5)
	assert.Len(t, pending, 2, "seg 6 (partial) + seg 7 (unprocessed)")
	assert.Len(t, archived, 2)
	assert.NoFileExists(t, paths[0])
	assert.NoFileExists(t, paths[1])
	assert.FileExists(t, archivePath(archiveDir, paths[0]))
	assert.FileExists(t, archivePath(archiveDir, paths[1]))
	for i := 2; i < 7; i++ {
		assert.FileExists(t, paths[i])
	}

	// SignalStop: remaining 3 processed drained.
	cleaner.SignalStop()
	processed, pending, archived, err = cleaner.ArchiveProcessedSegments()
	require.NoError(t, err)
	assert.Len(t, processed, 3)
	assert.Len(t, pending, 2)
	assert.Len(t, archived, 3)
	for i := 0; i < 5; i++ {
		assert.NoFileExists(t, paths[i])
		assert.FileExists(t, archivePath(archiveDir, paths[i]))
	}
	assert.FileExists(t, paths[5], "Seg 6 still pending — FF needs both")
	assert.FileExists(t, paths[6], "Seg 7 still pending")
}

func TestArchivePolicy_LiveFallForward_PostCutover(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	archiveDir := createArchiveDir(t, exportDir)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = true
		r.FallbackEnabled = false
		r.CutoverProcessedByTargetImporter = true
		r.ExportFromTargetFallForwardStarted = true
	})

	paths := createSegmentFiles(t, exportDir, 7)
	for i := 1; i <= 5; i++ {
		insertSegment(t, exportDir, SegmentRow{i, paths[i-1], "target_db_exporter_ff", 0, 1, 0, 0, 0})
	}
	for i := 6; i <= 7; i++ {
		insertSegment(t, exportDir, SegmentRow{i, paths[i-1], "target_db_exporter_ff", 0, 0, 0, 0, 0})
	}

	cleaner := newArchiveCleaner(exportDir, archiveDir, mdb)

	// Normal operation: 5 processed − buffer(3) = 2 eligible.
	processed, pending, archived, err := cleaner.ArchiveProcessedSegments()
	require.NoError(t, err)
	assert.Len(t, processed, 5)
	assert.Len(t, pending, 2)
	assert.Len(t, archived, 2)
	assert.NoFileExists(t, paths[0])
	assert.NoFileExists(t, paths[1])
	assert.FileExists(t, archivePath(archiveDir, paths[0]))
	assert.FileExists(t, archivePath(archiveDir, paths[1]))
	for i := 2; i < 7; i++ {
		assert.FileExists(t, paths[i])
	}

	// SignalStop: remaining 3 processed drained.
	cleaner.SignalStop()
	processed, pending, archived, err = cleaner.ArchiveProcessedSegments()
	require.NoError(t, err)
	assert.Len(t, processed, 3)
	assert.Len(t, pending, 2)
	assert.Len(t, archived, 3)
	for i := 0; i < 5; i++ {
		assert.NoFileExists(t, paths[i])
		assert.FileExists(t, archivePath(archiveDir, paths[i]))
	}
	assert.FileExists(t, paths[5], "Seg 6 (SR pending) should remain")
	assert.FileExists(t, paths[6], "Seg 7 (SR pending) should remain")
}

// =============================================================================================
// WORKFLOW 3: LIVE-FALL-BACK MIGRATION (archive)
// =============================================================================================

func TestArchivePolicy_LiveFallBack_PreCutover(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	archiveDir := createArchiveDir(t, exportDir)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = false
		r.FallbackEnabled = true
		r.ExportDataSourceDebeziumStarted = true
	})

	paths := createSegmentFiles(t, exportDir, 7)
	for i := 1; i <= 5; i++ {
		insertSegment(t, exportDir, SegmentRow{i, paths[i-1], "source_db_exporter", 1, 0, 0, 0, 0})
	}
	for i := 6; i <= 7; i++ {
		insertSegment(t, exportDir, SegmentRow{i, paths[i-1], "source_db_exporter", 0, 0, 0, 0, 0})
	}

	cleaner := newArchiveCleaner(exportDir, archiveDir, mdb)

	// Normal operation: 5 processed − buffer(3) = 2 eligible.
	processed, pending, archived, err := cleaner.ArchiveProcessedSegments()
	require.NoError(t, err)
	assert.Len(t, processed, 5)
	assert.Len(t, pending, 2)
	assert.Len(t, archived, 2)
	assert.NoFileExists(t, paths[0])
	assert.NoFileExists(t, paths[1])
	assert.FileExists(t, archivePath(archiveDir, paths[0]))
	assert.FileExists(t, archivePath(archiveDir, paths[1]))
	for i := 2; i < 7; i++ {
		assert.FileExists(t, paths[i])
	}

	// SignalStop: remaining 3 processed drained.
	cleaner.SignalStop()
	processed, pending, archived, err = cleaner.ArchiveProcessedSegments()
	require.NoError(t, err)
	assert.Len(t, processed, 3)
	assert.Len(t, pending, 2)
	assert.Len(t, archived, 3)
	for i := 0; i < 5; i++ {
		assert.NoFileExists(t, paths[i])
		assert.FileExists(t, archivePath(archiveDir, paths[i]))
	}
	assert.FileExists(t, paths[5], "Seg 6 (pending) untouched")
	assert.FileExists(t, paths[6], "Seg 7 (pending) untouched")
}

func TestArchivePolicy_LiveFallBack_PostCutover(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	archiveDir := createArchiveDir(t, exportDir)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = false
		r.FallbackEnabled = true
		r.ExportDataSourceDebeziumStarted = true
		r.CutoverProcessedByTargetImporter = true
		r.ExportFromTargetFallBackStarted = true
	})

	paths := createSegmentFiles(t, exportDir, 7)
	insertSegment(t, exportDir, SegmentRow{1, paths[0], "source_db_exporter", 1, 0, 0, 0, 0})
	insertSegment(t, exportDir, SegmentRow{2, paths[1], "source_db_exporter", 1, 0, 0, 0, 0})
	for i := 3; i <= 5; i++ {
		insertSegment(t, exportDir, SegmentRow{i, paths[i-1], "target_db_exporter_fb", 0, 0, 1, 0, 0})
	}
	for i := 6; i <= 7; i++ {
		insertSegment(t, exportDir, SegmentRow{i, paths[i-1], "target_db_exporter_fb", 0, 0, 0, 0, 0})
	}

	cleaner := newArchiveCleaner(exportDir, archiveDir, mdb)

	// Normal operation: processed=[1,2,3,4,5](5) − buffer(3) = 2 eligible (segs 1,2).
	processed, pending, archived, err := cleaner.ArchiveProcessedSegments()
	require.NoError(t, err)
	assert.Len(t, processed, 5)
	assert.Len(t, pending, 2)
	assert.Len(t, archived, 2)
	assert.NoFileExists(t, paths[0])
	assert.NoFileExists(t, paths[1])
	assert.FileExists(t, archivePath(archiveDir, paths[0]))
	assert.FileExists(t, archivePath(archiveDir, paths[1]))
	for i := 2; i < 7; i++ {
		assert.FileExists(t, paths[i])
	}

	// SignalStop: remaining processed=[3,4,5](3), all eligible.
	cleaner.SignalStop()
	processed, pending, archived, err = cleaner.ArchiveProcessedSegments()
	require.NoError(t, err)
	assert.Len(t, processed, 3)
	assert.Len(t, pending, 2)
	assert.Len(t, archived, 3)
	for i := 0; i < 5; i++ {
		assert.NoFileExists(t, paths[i])
		assert.FileExists(t, archivePath(archiveDir, paths[i]))
	}
	assert.FileExists(t, paths[5], "Seg 6 (fb, source pending) untouched")
	assert.FileExists(t, paths[6], "Seg 7 (fb, source pending) untouched")
}

// #############################################################################################
//
//	EDGE CASES & UTILITIES
//
// #############################################################################################

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
		FSUtilizationThreshold: 99,
	}
	cleaner := NewSegmentCleaner(cfg, mdb)

	done := make(chan error, 1)
	go func() {
		done <- cleaner.Run()
	}()

	time.Sleep(6 * time.Second)

	// FS utilization is well below 99%, so no segments should have been deleted yet.
	for _, p := range paths {
		assert.FileExists(t, p, "segment file should still exist — FS utilization below threshold")
	}
	allSegsBeforeStop := queryAllSegments(t, exportDir)
	for _, s := range allSegsBeforeStop {
		assert.Equal(t, 0, s.Deleted, "segment %d should not be marked deleted yet", s.SegmentNo)
	}

	cleaner.SignalStop()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(15 * time.Second):
		t.Fatal("cleanup did not stop")
	}

	allSegs := queryAllSegments(t, exportDir)
	deletedCount := 0
	for _, s := range allSegs {
		if s.Deleted == 1 {
			deletedCount++
		}
	}
	assert.Equal(t, 5, deletedCount, "SignalStop should drain all processed segments")
}

func TestEdgeCase_MissingSegmentFile(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = false
		r.FallbackEnabled = false
	})

	insertSegment(t, exportDir, SegmentRow{1, "/tmp/nonexistent/segment.1.ndjson", "source_db_exporter", 1, 0, 0, 0, 0})

	cleaner := newDeleteCleaner(exportDir, mdb)
	cleaner.SignalStop()

	_, _, deleted, err := cleaner.DeleteProcessedSegments()
	require.NoError(t, err, "should not error on missing file")
	assert.Len(t, deleted, 1)

	allSegs := queryAllSegments(t, exportDir)
	assert.Equal(t, 1, allSegs[0].Deleted)
	assert.Equal(t, 1, allSegs[0].Archived)
}

func TestUtility_IsValidPolicy(t *testing.T) {
	assert.True(t, IsValidPolicy("delete"))
	assert.True(t, IsValidPolicy("archive"))
	assert.False(t, IsValidPolicy("retain"))
	assert.False(t, IsValidPolicy(""))
	assert.False(t, IsValidPolicy("unknown"))
}

func TestUtility_MarkSegmentDeletedAndArchived(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	paths := createSegmentFiles(t, exportDir, 1)
	insertSegment(t, exportDir, SegmentRow{1, paths[0], "source_db_exporter", 1, 0, 0, 0, 0})

	require.NoError(t, mdb.MarkSegmentDeletedAndArchived(1))

	allSegs := queryAllSegments(t, exportDir)
	assert.Equal(t, 1, allSegs[0].Deleted)
	assert.Equal(t, 1, allSegs[0].Archived)
}
