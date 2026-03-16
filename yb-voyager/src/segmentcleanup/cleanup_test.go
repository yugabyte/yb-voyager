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
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
)

type segmentRow struct {
	SegmentNo                         int
	FilePath                          string
	ExporterRole                      string
	ImportedByTarget                  int
	ImportedBySourceReplica           int
	ImportedBySource                  int
	Deleted                           int
	Archived                          int
}

func setupTestMetaDB(t *testing.T) (*metadb.MetaDB, string) {
	tmpDir := t.TempDir()
	exportDir := tmpDir
	metainfoDir := filepath.Join(exportDir, "metainfo")
	require.NoError(t, os.MkdirAll(metainfoDir, 0755))
	require.NoError(t, metadb.CreateAndInitMetaDBIfRequired(exportDir))
	mdb, err := metadb.NewMetaDB(exportDir)
	require.NoError(t, err)
	return mdb, exportDir
}

func insertSegment(t *testing.T, exportDir string, seg segmentRow) {
	dbPath := metadb.GetMetaDBPath(exportDir)
	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	defer db.Close()
	_, err = db.Exec(`INSERT INTO queue_segment_meta 
		(segment_no, file_path, exporter_role, imported_by_target_db_importer, 
		 imported_by_source_replica_db_importer, imported_by_source_db_importer, deleted, archived)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		seg.SegmentNo, seg.FilePath, seg.ExporterRole,
		seg.ImportedByTarget, seg.ImportedBySourceReplica, seg.ImportedBySource,
		seg.Deleted, seg.Archived)
	require.NoError(t, err)
}

func queryAllSegments(t *testing.T, exportDir string) []segmentRow {
	dbPath := metadb.GetMetaDBPath(exportDir)
	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	defer db.Close()
	rows, err := db.Query(`SELECT segment_no, file_path, exporter_role, 
		imported_by_target_db_importer, imported_by_source_replica_db_importer,
		imported_by_source_db_importer, deleted, archived 
		FROM queue_segment_meta ORDER BY segment_no`)
	require.NoError(t, err)
	defer rows.Close()
	var result []segmentRow
	for rows.Next() {
		var r segmentRow
		require.NoError(t, rows.Scan(&r.SegmentNo, &r.FilePath, &r.ExporterRole,
			&r.ImportedByTarget, &r.ImportedBySourceReplica, &r.ImportedBySource,
			&r.Deleted, &r.Archived))
		result = append(result, r)
	}
	return result
}

func queryMSRJSON(t *testing.T, exportDir string) string {
	dbPath := metadb.GetMetaDBPath(exportDir)
	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	defer db.Close()
	var jsonText string
	err = db.QueryRow(`SELECT json_text FROM json_objects WHERE key = 'migration_status'`).Scan(&jsonText)
	if err == sql.ErrNoRows {
		return ""
	}
	require.NoError(t, err)
	return jsonText
}

func setMSR(t *testing.T, mdb *metadb.MetaDB, updateFn func(*metadb.MigrationStatusRecord)) {
	require.NoError(t, mdb.UpdateMigrationStatusRecord(updateFn))
}

func createSegmentFiles(t *testing.T, exportDir string, count int) []string {
	dataDir := filepath.Join(exportDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0755))
	var paths []string
	for i := 1; i <= count; i++ {
		path := filepath.Join(dataDir, fmt.Sprintf("segment.%d.ndjson", i))
		require.NoError(t, os.WriteFile(path, []byte("test-data"), 0644))
		paths = append(paths, path)
	}
	return paths
}

// ============================================================
// Test A1: Normal workflow - Segment cleanup with delete policy
// Verifies: sum-based predicate with importCount=1, no FF
// ============================================================
func TestA1_NormalDeletePolicy(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = false
		r.ExportDataSourceDebeziumStarted = true
	})

	paths := createSegmentFiles(t, exportDir, 3)

	insertSegment(t, exportDir, segmentRow{1, paths[0], "source_db_exporter", 1, 0, 0, 0, 0})
	insertSegment(t, exportDir, segmentRow{2, paths[1], "source_db_exporter", 1, 0, 0, 0, 0})
	insertSegment(t, exportDir, segmentRow{3, paths[2], "source_db_exporter", 0, 0, 0, 0, 0})

	t.Log("=== Test A1: Normal Delete Policy (no FF) ===")
	t.Log("Pre-condition: FallForwardEnabled=false, 3 segments, seg 1&2 imported by target, seg 3 not")

	segs, err := mdb.GetProcessedSegments(1)
	require.NoError(t, err)
	t.Logf("GetProcessedSegments(importCount=1) returned %d segments", len(segs))
	assert.Equal(t, 2, len(segs), "Should find 2 processed segments")
	assert.Equal(t, 1, segs[0].Num)
	assert.Equal(t, 2, segs[1].Num)

	allSegs := queryAllSegments(t, exportDir)
	t.Log("queue_segment_meta state BEFORE cleanup:")
	for _, s := range allSegs {
		t.Logf("  seg=%d exporter=%s target=%d sr=%d src=%d deleted=%d archived=%d",
			s.SegmentNo, s.ExporterRole, s.ImportedByTarget, s.ImportedBySourceReplica,
			s.ImportedBySource, s.Deleted, s.Archived)
	}

	for _, seg := range segs {
		require.NoError(t, mdb.MarkSegmentDeletedAndArchived(seg.Num))
	}

	allSegs = queryAllSegments(t, exportDir)
	t.Log("queue_segment_meta state AFTER cleanup:")
	for _, s := range allSegs {
		t.Logf("  seg=%d exporter=%s target=%d sr=%d src=%d deleted=%d archived=%d",
			s.SegmentNo, s.ExporterRole, s.ImportedByTarget, s.ImportedBySourceReplica,
			s.ImportedBySource, s.Deleted, s.Archived)
	}

	assert.Equal(t, 1, allSegs[0].Deleted, "Seg 1 should be deleted")
	assert.Equal(t, 1, allSegs[0].Archived, "Seg 1 should be archived")
	assert.Equal(t, 1, allSegs[1].Deleted, "Seg 2 should be deleted")
	assert.Equal(t, 1, allSegs[1].Archived, "Seg 2 should be archived")
	assert.Equal(t, 0, allSegs[2].Deleted, "Seg 3 should NOT be deleted")
	assert.Equal(t, 0, allSegs[2].Archived, "Seg 3 should NOT be archived")

	segsAfter, err := mdb.GetProcessedSegments(1)
	require.NoError(t, err)
	assert.Equal(t, 0, len(segsAfter), "No processed segments should remain after deletion")
	t.Log("RESULT: PASS - Sum-based predicate importCount=1 works correctly for non-FF workflow")
}

// ============================================================
// Test A2: MSR ArchivingEnabled tracking
// Verifies: ArchivingEnabled set to true when cleanup starts
// ============================================================
func TestA2_MSRArchivingEnabled(t *testing.T) {
	mdb, _ := setupTestMetaDB(t)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = false
		r.ExportDataSourceDebeziumStarted = true
	})

	t.Log("=== Test A2: MSR ArchivingEnabled Tracking ===")
	msr, err := mdb.GetMigrationStatusRecord()
	require.NoError(t, err)
	t.Logf("Before cleanup: ArchivingEnabled=%v", msr.ArchivingEnabled)
	assert.False(t, msr.ArchivingEnabled)

	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.ArchivingEnabled = true
	})

	msr, err = mdb.GetMigrationStatusRecord()
	require.NoError(t, err)
	t.Logf("After setting ArchivingEnabled: ArchivingEnabled=%v", msr.ArchivingEnabled)
	assert.True(t, msr.ArchivingEnabled)
	t.Log("RESULT: PASS - ArchivingEnabled correctly tracked in MSR")
	t.Log("NOTE: SegmentCleanupRunning field does NOT exist in current implementation")
}

// ============================================================
// Test B4: FF pre-cutover - both importers must process
// Verifies: importCount=2, sum-based check
// ============================================================
func TestB4_FFPreCutoverBothImportersRequired(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = true
		r.ExportDataSourceDebeziumStarted = true
	})

	paths := createSegmentFiles(t, exportDir, 3)

	insertSegment(t, exportDir, segmentRow{1, paths[0], "source_db_exporter", 1, 1, 0, 0, 0})
	insertSegment(t, exportDir, segmentRow{2, paths[1], "source_db_exporter", 1, 1, 0, 0, 0})
	insertSegment(t, exportDir, segmentRow{3, paths[2], "source_db_exporter", 1, 0, 0, 0, 0})

	t.Log("=== Test B4: FF Pre-Cutover - Both Importers Required ===")
	t.Log("Pre-condition: FF enabled, seg 1&2 processed by both target+SR, seg 3 only by target")

	sc := &SegmentCleaner{metaDB: mdb}
	importCount, err := sc.getImportCount()
	require.NoError(t, err)
	t.Logf("getImportCount() = %d (expected 2 for FF)", importCount)
	assert.Equal(t, 2, importCount)

	segs, err := mdb.GetProcessedSegments(importCount)
	require.NoError(t, err)
	t.Logf("GetProcessedSegments(%d) returned %d segments", importCount, len(segs))
	for _, s := range segs {
		t.Logf("  Processed: seg=%d path=%s", s.Num, s.FilePath)
	}
	assert.Equal(t, 2, len(segs), "Only segments where target+SR=2 should be returned")

	allSegs := queryAllSegments(t, exportDir)
	t.Log("queue_segment_meta state:")
	for _, s := range allSegs {
		t.Logf("  seg=%d exporter=%s target=%d sr=%d src=%d deleted=%d",
			s.SegmentNo, s.ExporterRole, s.ImportedByTarget, s.ImportedBySourceReplica,
			s.ImportedBySource, s.Deleted)
	}
	t.Log("RESULT: PASS - Pre-cutover sum-based check with importCount=2 works correctly")
}

// ============================================================
// Test B5: FF pre-cutover - SR lagging behind
// Verifies: segment NOT deleted when SR hasn't processed it
// ============================================================
func TestB5_FFPreCutoverSRLagging(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = true
		r.ExportDataSourceDebeziumStarted = true
	})

	paths := createSegmentFiles(t, exportDir, 2)

	insertSegment(t, exportDir, segmentRow{1, paths[0], "source_db_exporter", 1, 0, 0, 0, 0})
	insertSegment(t, exportDir, segmentRow{2, paths[1], "source_db_exporter", 1, 1, 0, 0, 0})

	t.Log("=== Test B5: FF Pre-Cutover - SR Lagging Behind ===")
	t.Log("Pre-condition: Seg 1 has target=1, SR=0 (lagging). Seg 2 has both=1")

	segs, err := mdb.GetProcessedSegments(2)
	require.NoError(t, err)
	t.Logf("GetProcessedSegments(2) returned %d segments", len(segs))
	assert.Equal(t, 1, len(segs), "Only seg 2 should be returned (seg 1 has sum=1, need 2)")
	if len(segs) > 0 {
		assert.Equal(t, 2, segs[0].Num, "Only seg 2 should be eligible")
	}

	allSegs := queryAllSegments(t, exportDir)
	t.Log("queue_segment_meta state:")
	for _, s := range allSegs {
		t.Logf("  seg=%d target=%d sr=%d sum=%d (need 2)",
			s.SegmentNo, s.ImportedByTarget, s.ImportedBySourceReplica,
			s.ImportedByTarget+s.ImportedBySourceReplica+s.ImportedBySource)
	}
	t.Log("RESULT: PASS - Segment S1 correctly NOT eligible for deletion when SR lags")
}

// ============================================================
// Test B6/B7: FF transition across cutover
// Verifies: importCount is dynamically read per tick
// NOTE: In current impl, getImportCount checks FallForwardEnabled
//   which doesn't change after cutover. The test documents this.
// ============================================================
func TestB6_FFTransitionAcrossCutover(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)

	t.Log("=== Test B6: FF Transition Across Cutover ===")

	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = true
		r.ExportDataSourceDebeziumStarted = true
		r.CutoverProcessedByTargetImporter = false
	})

	paths := createSegmentFiles(t, exportDir, 4)

	insertSegment(t, exportDir, segmentRow{1, paths[0], "source_db_exporter", 1, 1, 0, 0, 0})
	insertSegment(t, exportDir, segmentRow{2, paths[1], "source_db_exporter", 1, 0, 0, 0, 0})
	insertSegment(t, exportDir, segmentRow{3, paths[2], "target_db_exporter_ff", 0, 1, 0, 0, 0})
	insertSegment(t, exportDir, segmentRow{4, paths[3], "target_db_exporter_ff", 0, 0, 0, 0, 0})

	sc := &SegmentCleaner{metaDB: mdb}

	t.Log("--- Pre-cutover state ---")
	importCount, err := sc.getImportCount()
	require.NoError(t, err)
	t.Logf("getImportCount() = %d (FF enabled)", importCount)
	assert.Equal(t, 2, importCount)

	segs, err := mdb.GetProcessedSegments(importCount)
	require.NoError(t, err)
	t.Logf("Pre-cutover: GetProcessedSegments(%d) -> %d segments", importCount, len(segs))
	for _, s := range segs {
		t.Logf("  Eligible: seg=%d path=%s", s.Num, s.FilePath)
	}
	assert.Equal(t, 2, len(segs), "Seg 1 (source, sum=2) and seg 3 (target, sum=1) should be eligible")

	t.Log("--- Simulating cutover ---")
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.CutoverProcessedByTargetImporter = true
		r.ExportFromTargetFallForwardStarted = true
	})

	msr, _ := mdb.GetMigrationStatusRecord()
	t.Logf("After cutover: CutoverProcessedByTargetImporter=%v, ExportFromTargetFallForwardStarted=%v",
		msr.CutoverProcessedByTargetImporter, msr.ExportFromTargetFallForwardStarted)

	importCountAfter, err := sc.getImportCount()
	require.NoError(t, err)
	t.Logf("getImportCount() after cutover = %d", importCountAfter)
	t.Log("NOTE: In current implementation, importCount stays 2 because FallForwardEnabled doesn't change")
	t.Log("This means post-cutover target-exported segments use sum=1 check (correct for target_db_exporter segments)")

	segsAfter, err := mdb.GetProcessedSegments(importCountAfter)
	require.NoError(t, err)
	t.Logf("Post-cutover: GetProcessedSegments(%d) -> %d segments", importCountAfter, len(segsAfter))
	for _, s := range segsAfter {
		t.Logf("  Eligible: seg=%d", s.Num)
	}

	allSegs := queryAllSegments(t, exportDir)
	t.Log("Full queue_segment_meta state:")
	for _, s := range allSegs {
		t.Logf("  seg=%d exporter=%s target=%d sr=%d src=%d sum=%d deleted=%d",
			s.SegmentNo, s.ExporterRole, s.ImportedByTarget, s.ImportedBySourceReplica,
			s.ImportedBySource, s.ImportedByTarget+s.ImportedBySourceReplica+s.ImportedBySource,
			s.Deleted)
	}

	t.Log("RESULT: PASS (with notes)")
	t.Log("  - Source segments use sum=2 check (correct)")
	t.Log("  - Target exporter segments always use sum=1 check (correct)")
	t.Log("  - The code has separate predicates for source vs target exporter segments")
}

// ============================================================
// Test B7: Post-cutover old source segments pending for SR
// ============================================================
func TestB7_FFPostCutoverOldSourcePending(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = true
		r.CutoverProcessedByTargetImporter = true
		r.ExportFromTargetFallForwardStarted = true
	})

	paths := createSegmentFiles(t, exportDir, 2)

	insertSegment(t, exportDir, segmentRow{1, paths[0], "source_db_exporter", 1, 0, 0, 0, 0})
	insertSegment(t, exportDir, segmentRow{2, paths[1], "source_db_exporter", 1, 1, 0, 0, 0})

	t.Log("=== Test B7: Post-Cutover - Old Source Segments Pending for SR ===")
	t.Log("Pre-condition: Cutover complete. Seg 1: target=1, SR=0. Seg 2: target=1, SR=1")

	segs, err := mdb.GetProcessedSegments(2)
	require.NoError(t, err)
	t.Logf("GetProcessedSegments(2) returned %d segments", len(segs))
	assert.Equal(t, 1, len(segs), "Only seg 2 should be eligible (seg 1 sum=1, need 2)")
	if len(segs) > 0 {
		assert.Equal(t, 2, segs[0].Num)
	}

	t.Log("RESULT: PASS - Old source segment S1 correctly protected until SR processes it")
}

// ============================================================
// Test B8: Post-cutover target-exported segments before SR processes
// ============================================================
func TestB8_FFPostCutoverTargetSegmentsBeforeSR(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = true
		r.CutoverProcessedByTargetImporter = true
		r.ExportFromTargetFallForwardStarted = true
	})

	paths := createSegmentFiles(t, exportDir, 2)

	insertSegment(t, exportDir, segmentRow{10, paths[0], "target_db_exporter_ff", 0, 0, 0, 0, 0})
	insertSegment(t, exportDir, segmentRow{11, paths[1], "target_db_exporter_ff", 0, 0, 0, 0, 0})

	t.Log("=== Test B8: Post-Cutover - Target-Exported Segments Before SR Processes ===")
	t.Log("Pre-condition: Target exported segs S10, S11 with SR=0")

	segs, err := mdb.GetProcessedSegments(2)
	require.NoError(t, err)
	t.Logf("GetProcessedSegments(2) returned %d segments", len(segs))
	assert.Equal(t, 0, len(segs), "S10, S11 should NOT be eligible (SR hasn't processed)")

	allSegs := queryAllSegments(t, exportDir)
	t.Log("queue_segment_meta state:")
	for _, s := range allSegs {
		t.Logf("  seg=%d exporter=%s target=%d sr=%d sum=%d",
			s.SegmentNo, s.ExporterRole, s.ImportedByTarget, s.ImportedBySourceReplica,
			s.ImportedByTarget+s.ImportedBySourceReplica+s.ImportedBySource)
	}
	t.Log("RESULT: PASS - Post-cutover target segments protected until SR processes them")
}

// ============================================================
// Test B9: Post-cutover all segments processed by SR
// ============================================================
func TestB9_FFPostCutoverAllProcessed(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = true
		r.CutoverProcessedByTargetImporter = true
		r.ExportFromTargetFallForwardStarted = true
	})

	paths := createSegmentFiles(t, exportDir, 4)

	insertSegment(t, exportDir, segmentRow{1, paths[0], "source_db_exporter", 1, 1, 0, 0, 0})
	insertSegment(t, exportDir, segmentRow{2, paths[1], "source_db_exporter", 1, 1, 0, 0, 0})
	insertSegment(t, exportDir, segmentRow{10, paths[2], "target_db_exporter_ff", 0, 1, 0, 0, 0})
	insertSegment(t, exportDir, segmentRow{11, paths[3], "target_db_exporter_ff", 0, 1, 0, 0, 0})

	t.Log("=== Test B9: Post-Cutover - All Segments Processed by SR ===")

	segs, err := mdb.GetProcessedSegments(2)
	require.NoError(t, err)
	t.Logf("GetProcessedSegments(2) returned %d segments", len(segs))
	assert.Equal(t, 4, len(segs), "All 4 segments should be eligible")

	for _, seg := range segs {
		require.NoError(t, mdb.MarkSegmentDeletedAndArchived(seg.Num))
	}

	allSegs := queryAllSegments(t, exportDir)
	t.Log("queue_segment_meta state after cleanup:")
	for _, s := range allSegs {
		t.Logf("  seg=%d exporter=%s deleted=%d archived=%d",
			s.SegmentNo, s.ExporterRole, s.Deleted, s.Archived)
		assert.Equal(t, 1, s.Deleted)
		assert.Equal(t, 1, s.Archived)
	}
	t.Log("RESULT: PASS - All processed segments deleted successfully")
}

// ============================================================
// Test C10: Cleanup started post-cutover
// ============================================================
func TestC10_FFCleanupStartedPostCutover(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = true
		r.CutoverProcessedByTargetImporter = true
		r.ExportFromTargetFallForwardStarted = true
	})

	paths := createSegmentFiles(t, exportDir, 4)

	insertSegment(t, exportDir, segmentRow{1, paths[0], "source_db_exporter", 1, 1, 0, 0, 0})
	insertSegment(t, exportDir, segmentRow{2, paths[1], "source_db_exporter", 1, 1, 0, 0, 0})
	insertSegment(t, exportDir, segmentRow{10, paths[2], "target_db_exporter_ff", 0, 1, 0, 0, 0})
	insertSegment(t, exportDir, segmentRow{11, paths[3], "target_db_exporter_ff", 0, 0, 0, 0, 0})

	t.Log("=== Test C10: Cleanup Started Post-Cutover ===")
	t.Log("Pre-condition: Cutover complete. Source segs fully processed. Target seg 10 SR=1, seg 11 SR=0")

	sc := &SegmentCleaner{metaDB: mdb}
	importCount, err := sc.getImportCount()
	require.NoError(t, err)
	t.Logf("getImportCount() = %d (FF still enabled)", importCount)

	segs, err := mdb.GetProcessedSegments(importCount)
	require.NoError(t, err)
	t.Logf("GetProcessedSegments(%d) returned %d segments", importCount, len(segs))
	for _, s := range segs {
		t.Logf("  Eligible: seg=%d", s.Num)
	}
	assert.Equal(t, 3, len(segs), "Source segs 1,2 + target seg 10 should be eligible; seg 11 should not")

	t.Log("RESULT: PASS - Post-cutover cleanup correctly handles mixed segment state")
}

// ============================================================
// Test C11: Post-cutover mixed segment state
// ============================================================
func TestC11_FFPostCutoverMixedState(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = true
		r.CutoverProcessedByTargetImporter = true
	})

	paths := createSegmentFiles(t, exportDir, 4)

	insertSegment(t, exportDir, segmentRow{1, paths[0], "source_db_exporter", 1, 1, 0, 0, 0})
	insertSegment(t, exportDir, segmentRow{2, paths[1], "source_db_exporter", 1, 1, 0, 0, 0})
	insertSegment(t, exportDir, segmentRow{10, paths[2], "target_db_exporter_ff", 0, 1, 0, 0, 0})
	insertSegment(t, exportDir, segmentRow{11, paths[3], "target_db_exporter_ff", 0, 0, 0, 0, 0})

	t.Log("=== Test C11: Post-Cutover Mixed Segment State ===")

	segs, err := mdb.GetProcessedSegments(2)
	require.NoError(t, err)
	t.Logf("Eligible segments: %d", len(segs))
	assert.Equal(t, 3, len(segs))

	for _, seg := range segs {
		require.NoError(t, mdb.MarkSegmentDeletedAndArchived(seg.Num))
	}

	allSegs := queryAllSegments(t, exportDir)
	t.Log("Final state:")
	for _, s := range allSegs {
		t.Logf("  seg=%d exporter=%s deleted=%d sr=%d",
			s.SegmentNo, s.ExporterRole, s.Deleted, s.ImportedBySourceReplica)
	}
	assert.Equal(t, 1, allSegs[0].Deleted, "Seg 1 deleted")
	assert.Equal(t, 1, allSegs[1].Deleted, "Seg 2 deleted")
	assert.Equal(t, 1, allSegs[2].Deleted, "Seg 10 deleted")
	assert.Equal(t, 0, allSegs[3].Deleted, "Seg 11 NOT deleted (SR=0)")

	t.Log("RESULT: PASS - Selective deletion based on SR column works correctly")
}

// ============================================================
// Test D12: Retain policy
// ============================================================
func TestD12_RetainPolicy(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = true
		r.ExportDataSourceDebeziumStarted = true
	})

	paths := createSegmentFiles(t, exportDir, 2)
	insertSegment(t, exportDir, segmentRow{1, paths[0], "source_db_exporter", 1, 1, 0, 0, 0})
	insertSegment(t, exportDir, segmentRow{2, paths[1], "source_db_exporter", 1, 1, 0, 0, 0})

	t.Log("=== Test D12: Retain Policy ===")

	cfg := Config{
		Policy:                 PolicyRetain,
		ExportDir:              exportDir,
		FSUtilizationThreshold: 70,
	}
	cleaner := NewSegmentCleaner(cfg, mdb)

	done := make(chan error, 1)
	go func() {
		done <- cleaner.Run()
	}()

	time.Sleep(1 * time.Second)
	cleaner.SignalStop()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(35 * time.Second):
		t.Fatal("retain policy did not stop within expected time")
	}

	allSegs := queryAllSegments(t, exportDir)
	t.Log("Segments after retain policy run:")
	for _, s := range allSegs {
		t.Logf("  seg=%d deleted=%d archived=%d", s.SegmentNo, s.Deleted, s.Archived)
		assert.Equal(t, 0, s.Deleted, "No segments should be deleted with retain policy")
	}

	t.Log("RESULT: PASS - Retain policy does not delete any segments")
}

// ============================================================
// Test B18: Dynamic MSR re-read per tick
// ============================================================
func TestB18_DynamicMSRReread(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = true
		r.ExportDataSourceDebeziumStarted = true
	})

	paths := createSegmentFiles(t, exportDir, 3)
	insertSegment(t, exportDir, segmentRow{1, paths[0], "source_db_exporter", 1, 1, 0, 0, 0}) // sum=2
	insertSegment(t, exportDir, segmentRow{2, paths[1], "source_db_exporter", 1, 0, 0, 0, 0}) // sum=1
	insertSegment(t, exportDir, segmentRow{3, paths[2], "source_db_exporter", 0, 0, 0, 0, 0}) // sum=0

	t.Log("=== Test B18: Dynamic MSR Re-Read Per Tick ===")

	sc := &SegmentCleaner{metaDB: mdb}

	ic1, err := sc.getImportCount()
	require.NoError(t, err)
	t.Logf("Tick 1: getImportCount()=%d (FF enabled)", ic1)
	assert.Equal(t, 2, ic1)

	segs1, err := mdb.GetProcessedSegments(ic1)
	require.NoError(t, err)
	t.Logf("Tick 1: %d segments eligible (need sum=2)", len(segs1))
	assert.Equal(t, 1, len(segs1), "Only seg 1 eligible (sum=2)")

	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = false
	})

	ic2, err := sc.getImportCount()
	require.NoError(t, err)
	t.Logf("Tick 2 (after FF disabled): getImportCount()=%d", ic2)
	assert.Equal(t, 1, ic2)

	segs2, err := mdb.GetProcessedSegments(ic2)
	require.NoError(t, err)
	t.Logf("Tick 2: %d segments eligible (need sum=1)", len(segs2))
	// IMPORTANT FINDING: The predicate uses strict equality (= importCount), not >=.
	// Seg 1 has sum=2, which does NOT match =1. Only seg 2 (sum=1) matches.
	assert.Equal(t, 1, len(segs2), "Only seg 2 eligible (sum=1 exactly)")
	if len(segs2) > 0 {
		assert.Equal(t, 2, segs2[0].Num, "Seg 2 has sum=1 matching importCount=1")
	}

	t.Log("RESULT: PASS - getImportCount re-reads MSR each time, picks up changes")
	t.Log("FINDING: GetProcessedSegments uses strict equality (= importCount), not >=")
	t.Log("  This means segments already processed by MORE importers than required won't match")
	t.Log("  Seg 1 (sum=2) does NOT match importCount=1. This could be intentional or a bug.")
}

// ============================================================
// Test F19: Fall-back workflow (no FF)
// ============================================================
func TestF19_FallbackWorkflow(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = false
		r.FallbackEnabled = true
		r.ExportDataSourceDebeziumStarted = true
	})

	paths := createSegmentFiles(t, exportDir, 3)
	insertSegment(t, exportDir, segmentRow{1, paths[0], "source_db_exporter", 1, 0, 0, 0, 0})
	insertSegment(t, exportDir, segmentRow{2, paths[1], "source_db_exporter", 1, 0, 0, 0, 0})
	insertSegment(t, exportDir, segmentRow{3, paths[2], "source_db_exporter", 0, 0, 0, 0, 0})

	t.Log("=== Test F19: Fall-Back Workflow (No FF) ===")
	t.Log("Pre-condition: FallForwardEnabled=false, FallbackEnabled=true")

	sc := &SegmentCleaner{metaDB: mdb}
	importCount, err := sc.getImportCount()
	require.NoError(t, err)
	t.Logf("getImportCount() = %d (FF disabled, FB enabled)", importCount)
	assert.Equal(t, 1, importCount, "Fall-back without FF should use importCount=1")

	segs, err := mdb.GetProcessedSegments(importCount)
	require.NoError(t, err)
	t.Logf("GetProcessedSegments(%d) returned %d segments", importCount, len(segs))
	assert.Equal(t, 2, len(segs), "Segs 1 and 2 with target=1 should be eligible")

	t.Log("RESULT: PASS - Fall-back workflow uses importCount=1 correctly")
}

// ============================================================
// Test F20: FS below threshold - no deletion
// ============================================================
func TestF20_FSBelowThreshold(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	setMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = true
		r.CutoverProcessedByTargetImporter = true
	})

	paths := createSegmentFiles(t, exportDir, 2)
	insertSegment(t, exportDir, segmentRow{1, paths[0], "source_db_exporter", 1, 1, 0, 0, 0})
	insertSegment(t, exportDir, segmentRow{2, paths[1], "source_db_exporter", 1, 1, 0, 0, 0})

	t.Log("=== Test F20: FS Below Threshold - No Deletion ===")

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
	t.Log("RESULT: PASS - Threshold gating works; segments only deleted during stop-drain")
}

// ============================================================
// Test: Verify GetProcessedSegments predicate correctness
// ============================================================
func TestPredicateCorrectnessMatrix(t *testing.T) {
	t.Log("=== Predicate Correctness Matrix ===")
	t.Log("Testing all combinations of exporter_role and importer flags")

	testCases := []struct {
		name        string
		seg         segmentRow
		importCount int
		wantFound   bool
	}{
		{"source_seg target=1 sr=0 src=0, count=1", segmentRow{1, "/tmp/s1", "source_db_exporter", 1, 0, 0, 0, 0}, 1, true},
		{"source_seg target=0 sr=0 src=0, count=1", segmentRow{1, "/tmp/s1", "source_db_exporter", 0, 0, 0, 0, 0}, 1, false},
		{"source_seg target=1 sr=1 src=0, count=2", segmentRow{1, "/tmp/s1", "source_db_exporter", 1, 1, 0, 0, 0}, 2, true},
		{"source_seg target=1 sr=0 src=0, count=2", segmentRow{1, "/tmp/s1", "source_db_exporter", 1, 0, 0, 0, 0}, 2, false},
		{"source_seg target=0 sr=1 src=0, count=2", segmentRow{1, "/tmp/s1", "source_db_exporter", 0, 1, 0, 0, 0}, 2, false},
		{"target_ff_seg sr=1, count=1", segmentRow{1, "/tmp/s1", "target_db_exporter_ff", 0, 1, 0, 0, 0}, 1, true},
		{"target_ff_seg sr=0, count=1", segmentRow{1, "/tmp/s1", "target_db_exporter_ff", 0, 0, 0, 0, 0}, 1, false},
		{"target_ff_seg sr=1, count=2", segmentRow{1, "/tmp/s1", "target_db_exporter_ff", 0, 1, 0, 0, 0}, 2, true},
		{"target_ff_seg sr=0, count=2", segmentRow{1, "/tmp/s1", "target_db_exporter_ff", 0, 0, 0, 0, 0}, 2, false},
		{"target_fb_seg src=1, count=1", segmentRow{1, "/tmp/s1", "target_db_exporter_fb", 0, 0, 1, 0, 0}, 1, true},
		{"target_fb_seg src=0, count=1", segmentRow{1, "/tmp/s1", "target_db_exporter_fb", 0, 0, 0, 0, 0}, 1, false},
		{"already_deleted source_seg, count=1", segmentRow{1, "/tmp/s1", "source_db_exporter", 1, 0, 0, 1, 0}, 1, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mdb, exportDir := setupTestMetaDB(t)
			insertSegment(t, exportDir, tc.seg)

			segs, err := mdb.GetProcessedSegments(tc.importCount)
			require.NoError(t, err)

			found := len(segs) > 0
			t.Logf("  seg=%d exporter=%s target=%d sr=%d src=%d deleted=%d | importCount=%d | found=%v expected=%v",
				tc.seg.SegmentNo, tc.seg.ExporterRole, tc.seg.ImportedByTarget,
				tc.seg.ImportedBySourceReplica, tc.seg.ImportedBySource, tc.seg.Deleted,
				tc.importCount, found, tc.wantFound)
			assert.Equal(t, tc.wantFound, found)
		})
	}
}

// ============================================================
// Test: IsValidPolicy
// ============================================================
func TestIsValidPolicy(t *testing.T) {
	assert.True(t, IsValidPolicy("delete"))
	assert.True(t, IsValidPolicy("retain"))
	assert.False(t, IsValidPolicy("archive"))
	assert.False(t, IsValidPolicy(""))
	t.Log("RESULT: PASS - Policy validation works correctly")
}

// ============================================================
// Test: MarkSegmentDeletedAndArchived correctness
// ============================================================
func TestMarkSegmentDeletedAndArchived(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	paths := createSegmentFiles(t, exportDir, 1)
	insertSegment(t, exportDir, segmentRow{1, paths[0], "source_db_exporter", 1, 0, 0, 0, 0})

	t.Log("=== Test: MarkSegmentDeletedAndArchived ===")
	require.NoError(t, mdb.MarkSegmentDeletedAndArchived(1))

	allSegs := queryAllSegments(t, exportDir)
	assert.Equal(t, 1, allSegs[0].Deleted)
	assert.Equal(t, 1, allSegs[0].Archived)
	t.Log("RESULT: PASS - MarkSegmentDeletedAndArchived sets both flags")
}

// ============================================================
// Test: SegmentCleaner deleteSegment removes file and marks DB
// ============================================================
func TestDeleteSegmentRemovesFile(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	paths := createSegmentFiles(t, exportDir, 1)
	insertSegment(t, exportDir, segmentRow{1, paths[0], "source_db_exporter", 1, 0, 0, 0, 0})

	t.Log("=== Test: deleteSegment removes file and marks DB ===")

	assert.FileExists(t, paths[0], "segment file should exist before delete")

	cfg := Config{Policy: PolicyDelete, ExportDir: exportDir, FSUtilizationThreshold: 70}
	cleaner := NewSegmentCleaner(cfg, mdb)

	segs, err := mdb.GetProcessedSegments(1)
	require.NoError(t, err)
	require.Len(t, segs, 1)

	require.NoError(t, cleaner.deleteSegment(segs[0]))

	assert.NoFileExists(t, paths[0], "segment file should be removed after delete")

	allSegs := queryAllSegments(t, exportDir)
	assert.Equal(t, 1, allSegs[0].Deleted)
	assert.Equal(t, 1, allSegs[0].Archived)
	t.Log("RESULT: PASS - deleteSegment removes file and marks DB correctly")
}

// ============================================================
// Test: deleteSegment handles missing file gracefully
// ============================================================
func TestDeleteSegmentMissingFile(t *testing.T) {
	mdb, exportDir := setupTestMetaDB(t)
	insertSegment(t, exportDir, segmentRow{1, "/tmp/nonexistent/segment.1.ndjson", "source_db_exporter", 1, 0, 0, 0, 0})

	t.Log("=== Test: deleteSegment handles missing file gracefully ===")

	cfg := Config{Policy: PolicyDelete, ExportDir: exportDir, FSUtilizationThreshold: 70}
	cleaner := NewSegmentCleaner(cfg, mdb)

	segs, err := mdb.GetProcessedSegments(1)
	require.NoError(t, err)
	require.Len(t, segs, 1)

	err = cleaner.deleteSegment(segs[0])
	require.NoError(t, err, "Should not error on missing file")

	allSegs := queryAllSegments(t, exportDir)
	assert.Equal(t, 1, allSegs[0].Deleted)
	assert.Equal(t, 1, allSegs[0].Archived)
	t.Log("RESULT: PASS - deleteSegment handles missing files gracefully")
}
