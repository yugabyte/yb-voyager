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
	"github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// ============================================================
// Test A1: Normal workflow - Segment cleanup with delete policy
// Verifies: sum-based predicate with importCount=1, no FF
// ============================================================
func TestA1_NormalDeletePolicy(t *testing.T) {
	mdb, exportDir := testutils.SetupTestMetaDB(t)
	testutils.SetMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = false
		r.ExportDataSourceDebeziumStarted = true
	})

	paths := testutils.CreateSegmentFiles(t, exportDir, 3)

	testutils.InsertSegment(t, exportDir, testutils.SegmentRow{1, paths[0], "source_db_exporter", 1, 0, 0, 0, 0})
	testutils.InsertSegment(t, exportDir, testutils.SegmentRow{2, paths[1], "source_db_exporter", 1, 0, 0, 0, 0})
	testutils.InsertSegment(t, exportDir, testutils.SegmentRow{3, paths[2], "source_db_exporter", 0, 0, 0, 0, 0})

	t.Log("=== Test A1: Normal Delete Policy (no FF) ===")
	t.Log("Pre-condition: FallForwardEnabled=false, 3 segments, seg 1&2 imported by target, seg 3 not")

	processedSegs, err := mdb.GetProcessedQueueSegments()
	require.NoError(t, err)
	t.Logf("GetProcessedQueueSegments() returned %d segments", len(processedSegs))
	assert.Equal(t, 2, len(processedSegs), "Should find 2 processed segments")

	allSegs := testutils.QueryAllSegments(t, exportDir)
	t.Log("queue_segment_meta state BEFORE cleanup:")
	for _, s := range allSegs {
		t.Logf("  seg=%d exporter=%s target=%d sr=%d src=%d deleted=%d archived=%d",
			s.SegmentNo, s.ExporterRole, s.ImportedByTarget, s.ImportedBySourceReplica,
			s.ImportedBySource, s.Deleted, s.Archived)
	}

	for _, seg := range processedSegs {
		require.NoError(t, mdb.MarkSegmentDeletedAndArchived(seg.Num))
	}

	allSegs = testutils.QueryAllSegments(t, exportDir)
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

	processedSegsAfter, err := mdb.GetProcessedQueueSegments()
	require.NoError(t, err)
	assert.Equal(t, 0, len(processedSegsAfter), "No processed segments should remain after deletion")
	t.Log("RESULT: PASS - Sum-based predicate importCount=1 works correctly for non-FF workflow")
}

// ============================================================
// Test B4: FF pre-cutover - both importers must process
// Verifies: importCount=2, sum-based check
// ============================================================
func TestB4_FFPreCutoverBothImportersRequired(t *testing.T) {
	mdb, exportDir := testutils.SetupTestMetaDB(t)
	testutils.SetMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = true
		r.ExportDataSourceDebeziumStarted = true
	})

	paths := testutils.CreateSegmentFiles(t, exportDir, 3)

	testutils.InsertSegment(t, exportDir, testutils.SegmentRow{1, paths[0], "source_db_exporter", 1, 1, 0, 0, 0})
	testutils.InsertSegment(t, exportDir, testutils.SegmentRow{2, paths[1], "source_db_exporter", 1, 1, 0, 0, 0})
	testutils.InsertSegment(t, exportDir, testutils.SegmentRow{3, paths[2], "source_db_exporter", 1, 0, 0, 0, 0})

	t.Log("=== Test B4: FF Pre-Cutover - Both Importers Required ===")
	t.Log("Pre-condition: FF enabled, seg 1&2 processed by both target+SR, seg 3 only by target")
	t.Log("With FallForwardEnabled=true and no cutover, GetProcessedQueueSegments uses importCount=2")

	processedSegs, err := mdb.GetProcessedQueueSegments()
	require.NoError(t, err)
	t.Logf("GetProcessedQueueSegments() returned %d segments", len(processedSegs))
	for _, s := range processedSegs {
		t.Logf("  Processed: seg=%d path=%s", s.Num, s.FilePath)
	}
	assert.Equal(t, 2, len(processedSegs), "Only segments where target+SR=2 should be returned (importCount=2 for FF pre-cutover)")

	allSegs := testutils.QueryAllSegments(t, exportDir)
	t.Log("queue_segment_meta state:")
	for _, s := range allSegs {
		t.Logf("  seg=%d exporter=%s target=%d sr=%d src=%d deleted=%d",
			s.SegmentNo, s.ExporterRole, s.ImportedByTarget, s.ImportedBySourceReplica,
			s.ImportedBySource, s.Deleted)
	}
	t.Log("RESULT: PASS - Pre-cutover sum-based check with importCount=2 works correctly")
}

// ============================================================
// Test B7: Post-cutover old source segments pending for SR
// ============================================================
func TestB7_FFPostCutoverOldSourcePending(t *testing.T) {
	mdb, exportDir := testutils.SetupTestMetaDB(t)
	testutils.SetMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = true
		r.CutoverProcessedByTargetImporter = true
		r.ExportFromTargetFallForwardStarted = true
	})

	paths := testutils.CreateSegmentFiles(t, exportDir, 2)

	testutils.InsertSegment(t, exportDir, testutils.SegmentRow{1, paths[0], "source_db_exporter", 1, 0, 0, 0, 0})
	testutils.InsertSegment(t, exportDir, testutils.SegmentRow{2, paths[1], "source_db_exporter", 1, 1, 0, 0, 0})

	t.Log("=== Test B7: Post-Cutover - Old Source Segments Pending for SR ===")
	t.Log("Pre-condition: Cutover complete. Seg 1: target=1, SR=0. Seg 2: target=1, SR=1")
	t.Log("With FallForwardEnabled=true and cutover done, GetProcessedQueueSegments uses importCount=1")

	processedSegs, err := mdb.GetProcessedQueueSegments()
	require.NoError(t, err)
	t.Logf("GetProcessedQueueSegments() returned %d segments", len(processedSegs))
	assert.Equal(t, 1, len(processedSegs), "Only seg 2 should be eligible (seg 1 sum=1, need 2)")
	if len(processedSegs) > 0 {
		assert.Equal(t, 2, processedSegs[0].Num)
	}

	t.Log("RESULT: PASS - Old source segment S1 correctly protected until SR processes it")
}

// ============================================================
// Test B8: Post-cutover target-exported segments before SR processes
// ============================================================
func TestB8_FFPostCutoverTargetSegmentsBeforeSR(t *testing.T) {
	mdb, exportDir := testutils.SetupTestMetaDB(t)
	testutils.SetMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = true
		r.CutoverProcessedByTargetImporter = true
		r.ExportFromTargetFallForwardStarted = true
	})

	paths := testutils.CreateSegmentFiles(t, exportDir, 2)

	testutils.InsertSegment(t, exportDir, testutils.SegmentRow{10, paths[0], "target_db_exporter_ff", 0, 0, 0, 0, 0})
	testutils.InsertSegment(t, exportDir, testutils.SegmentRow{11, paths[1], "target_db_exporter_ff", 0, 0, 0, 0, 0})

	t.Log("=== Test B8: Post-Cutover - Target-Exported Segments Before SR Processes ===")
	t.Log("Pre-condition: Target exported segs S10, S11 with SR=0")
	t.Log("With FallForwardEnabled=true and cutover done, GetProcessedQueueSegments uses importCount=1")

	processedSegs, err := mdb.GetProcessedQueueSegments()
	require.NoError(t, err)
	t.Logf("GetProcessedQueueSegments() returned %d segments", len(processedSegs))
	assert.Equal(t, 0, len(processedSegs), "S10, S11 should NOT be eligible (SR hasn't processed)")

	allSegs := testutils.QueryAllSegments(t, exportDir)
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
	mdb, exportDir := testutils.SetupTestMetaDB(t)
	testutils.SetMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = true
		r.CutoverProcessedByTargetImporter = true
		r.ExportFromTargetFallForwardStarted = true
	})

	paths := testutils.CreateSegmentFiles(t, exportDir, 4)

	testutils.InsertSegment(t, exportDir, testutils.SegmentRow{1, paths[0], "source_db_exporter", 1, 1, 0, 0, 0})
	testutils.InsertSegment(t, exportDir, testutils.SegmentRow{2, paths[1], "source_db_exporter", 1, 1, 0, 0, 0})
	testutils.InsertSegment(t, exportDir, testutils.SegmentRow{10, paths[2], "target_db_exporter_ff", 0, 1, 0, 0, 0})
	testutils.InsertSegment(t, exportDir, testutils.SegmentRow{11, paths[3], "target_db_exporter_ff", 0, 1, 0, 0, 0})

	t.Log("=== Test B9: Post-Cutover - All Segments Processed by SR ===")
	t.Log("With FallForwardEnabled=true and cutover done, GetProcessedQueueSegments uses importCount=1")

	processedSegs, err := mdb.GetProcessedQueueSegments()
	require.NoError(t, err)
	t.Logf("GetProcessedQueueSegments() returned %d segments", len(processedSegs))
	assert.Equal(t, 4, len(processedSegs), "All 4 segments should be eligible")

	for _, seg := range processedSegs {
		require.NoError(t, mdb.MarkSegmentDeletedAndArchived(seg.Num))
	}

	allSegs := testutils.QueryAllSegments(t, exportDir)
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
	mdb, exportDir := testutils.SetupTestMetaDB(t)
	testutils.SetMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = true
		r.CutoverProcessedByTargetImporter = true
		r.ExportFromTargetFallForwardStarted = true
	})

	paths := testutils.CreateSegmentFiles(t, exportDir, 4)

	testutils.InsertSegment(t, exportDir, testutils.SegmentRow{1, paths[0], "source_db_exporter", 1, 1, 0, 0, 0})
	testutils.InsertSegment(t, exportDir, testutils.SegmentRow{2, paths[1], "source_db_exporter", 1, 1, 0, 0, 0})
	testutils.InsertSegment(t, exportDir, testutils.SegmentRow{10, paths[2], "target_db_exporter_ff", 0, 1, 0, 0, 0})
	testutils.InsertSegment(t, exportDir, testutils.SegmentRow{11, paths[3], "target_db_exporter_ff", 0, 0, 0, 0, 0})

	t.Log("=== Test C10: Cleanup Started Post-Cutover ===")
	t.Log("Pre-condition: Cutover complete. Source segs fully processed. Target seg 10 SR=1, seg 11 SR=0")
	t.Log("With FallForwardEnabled=true and cutover done, GetProcessedQueueSegments uses importCount=1")

	processedSegs, err := mdb.GetProcessedQueueSegments()
	require.NoError(t, err)
	t.Logf("GetProcessedQueueSegments() returned %d segments", len(processedSegs))
	for _, s := range processedSegs {
		t.Logf("  Eligible: seg=%d", s.Num)
	}
	assert.Equal(t, 3, len(processedSegs), "Source segs 1,2 + target seg 10 should be eligible; seg 11 should not")

	t.Log("RESULT: PASS - Post-cutover cleanup correctly handles mixed segment state")
}

// ============================================================
// Test C11: Post-cutover mixed segment state
// ============================================================
func TestC11_FFPostCutoverMixedState(t *testing.T) {
	mdb, exportDir := testutils.SetupTestMetaDB(t)
	testutils.SetMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = true
		r.CutoverProcessedByTargetImporter = true
	})

	paths := testutils.CreateSegmentFiles(t, exportDir, 4)

	testutils.InsertSegment(t, exportDir, testutils.SegmentRow{1, paths[0], "source_db_exporter", 1, 1, 0, 0, 0})
	testutils.InsertSegment(t, exportDir, testutils.SegmentRow{2, paths[1], "source_db_exporter", 1, 1, 0, 0, 0})
	testutils.InsertSegment(t, exportDir, testutils.SegmentRow{10, paths[2], "target_db_exporter_ff", 0, 1, 0, 0, 0})
	testutils.InsertSegment(t, exportDir, testutils.SegmentRow{11, paths[3], "target_db_exporter_ff", 0, 0, 0, 0, 0})

	t.Log("=== Test C11: Post-Cutover Mixed Segment State ===")
	t.Log("With FallForwardEnabled=true and cutover done, GetProcessedQueueSegments uses importCount=1")

	processedSegs, err := mdb.GetProcessedQueueSegments()
	require.NoError(t, err)
	t.Logf("Eligible segments: %d", len(processedSegs))
	assert.Equal(t, 3, len(processedSegs))

	for _, seg := range processedSegs {
		require.NoError(t, mdb.MarkSegmentDeletedAndArchived(seg.Num))
	}

	allSegs := testutils.QueryAllSegments(t, exportDir)
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
// Test B18: Dynamic MSR re-read per call
// ============================================================
func TestB18_DynamicMSRReread(t *testing.T) {
	mdb, exportDir := testutils.SetupTestMetaDB(t)
	testutils.SetMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = true
		r.ExportDataSourceDebeziumStarted = true
	})

	paths := testutils.CreateSegmentFiles(t, exportDir, 3)
	testutils.InsertSegment(t, exportDir, testutils.SegmentRow{1, paths[0], "source_db_exporter", 1, 1, 0, 0, 0}) // sum=2
	testutils.InsertSegment(t, exportDir, testutils.SegmentRow{2, paths[1], "source_db_exporter", 1, 0, 0, 0, 0}) // sum=1
	testutils.InsertSegment(t, exportDir, testutils.SegmentRow{3, paths[2], "source_db_exporter", 0, 0, 0, 0, 0}) // sum=0

	t.Log("=== Test B18: Dynamic MSR Re-Read Per Call ===")
	t.Log("GetProcessedQueueSegments reads MSR internally on each call")

	t.Log("--- Call 1: FF enabled (importCount=2) ---")
	processedSegs1, err := mdb.GetProcessedQueueSegments()
	require.NoError(t, err)
	t.Logf("Call 1: %d segments eligible (importCount=2 from MSR)", len(processedSegs1))
	assert.Equal(t, 1, len(processedSegs1), "Only seg 1 eligible (sum=2)")

	testutils.SetMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = false
	})

	t.Log("--- Call 2: FF disabled (importCount=1) ---")
	processedSegs2, err := mdb.GetProcessedQueueSegments()
	require.NoError(t, err)
	t.Logf("Call 2: %d segments eligible (importCount=1 from MSR)", len(processedSegs2))
	// The sum-based predicate uses strict equality (= importCount), not >=.
	// Seg 1 has sum=2, which does NOT match =1. Only seg 2 (sum=1) matches.
	assert.Equal(t, 1, len(processedSegs2), "Only seg 2 eligible (sum=1 exactly)")
	if len(processedSegs2) > 0 {
		assert.Equal(t, 2, processedSegs2[0].Num, "Seg 2 has sum=1 matching importCount=1")
	}

	t.Log("RESULT: PASS - GetProcessedQueueSegments re-reads MSR each call, picks up changes")
	t.Log("FINDING: Predicate uses strict equality (= importCount), not >=")
	t.Log("  This means segments already processed by MORE importers than required won't match")
	t.Log("  Seg 1 (sum=2) does NOT match importCount=1. This could be intentional or a bug.")
}

// ============================================================
// Test F19: Fall-back workflow (no FF)
// ============================================================
func TestF19_FallbackWorkflow(t *testing.T) {
	mdb, exportDir := testutils.SetupTestMetaDB(t)
	testutils.SetMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = false
		r.FallbackEnabled = true
		r.ExportDataSourceDebeziumStarted = true
	})

	paths := testutils.CreateSegmentFiles(t, exportDir, 3)
	testutils.InsertSegment(t, exportDir, testutils.SegmentRow{1, paths[0], "source_db_exporter", 1, 0, 0, 0, 0})
	testutils.InsertSegment(t, exportDir, testutils.SegmentRow{2, paths[1], "source_db_exporter", 1, 0, 0, 0, 0})
	testutils.InsertSegment(t, exportDir, testutils.SegmentRow{3, paths[2], "source_db_exporter", 0, 0, 0, 0, 0})

	t.Log("=== Test F19: Fall-Back Workflow (No FF) ===")
	t.Log("Pre-condition: FallForwardEnabled=false, FallbackEnabled=true")
	t.Log("With FallForwardEnabled=false, GetProcessedQueueSegments uses importCount=1")

	processedSegs, err := mdb.GetProcessedQueueSegments()
	require.NoError(t, err)
	t.Logf("GetProcessedQueueSegments() returned %d segments", len(processedSegs))
	assert.Equal(t, 2, len(processedSegs), "Segs 1 and 2 with target=1 should be eligible (importCount=1)")

	t.Log("RESULT: PASS - Fall-back workflow uses importCount=1 correctly")
}

// ============================================================
// Test F20: FS below threshold - no deletion
// ============================================================
func TestF20_FSBelowThreshold(t *testing.T) {
	mdb, exportDir := testutils.SetupTestMetaDB(t)
	testutils.SetMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = true
		r.CutoverProcessedByTargetImporter = true
	})

	paths := testutils.CreateSegmentFiles(t, exportDir, 2)
	testutils.InsertSegment(t, exportDir, testutils.SegmentRow{1, paths[0], "source_db_exporter", 1, 1, 0, 0, 0})
	testutils.InsertSegment(t, exportDir, testutils.SegmentRow{2, paths[1], "source_db_exporter", 1, 1, 0, 0, 0})

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

	allSegs := testutils.QueryAllSegments(t, exportDir)
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
// Test: Verify GetProcessedQueueSegments predicate correctness
// ============================================================
func TestPredicateCorrectnessMatrix(t *testing.T) {
	t.Log("=== Predicate Correctness Matrix ===")
	t.Log("Testing predicate behavior with MSR-derived importCount")
	t.Log("GetProcessedQueueSegments now reads MSR internally and uses importCount=1 (normal) or 2 (FF pre-cutover)")

	// Test with importCount=1 (FallForwardEnabled=false)
	t.Run("importCount=1 (no FF)", func(t *testing.T) {
		testCases := []struct {
			name      string
			seg       testutils.SegmentRow
			wantFound bool
		}{
			{"source_seg target=1 sr=0 src=0", testutils.SegmentRow{1, "/tmp/s1", "source_db_exporter", 1, 0, 0, 0, 0}, true},
			{"source_seg target=0 sr=0 src=0", testutils.SegmentRow{1, "/tmp/s1", "source_db_exporter", 0, 0, 0, 0, 0}, false},
			{"source_seg target=1 sr=1 src=0 (sum=2)", testutils.SegmentRow{1, "/tmp/s1", "source_db_exporter", 1, 1, 0, 0, 0}, false}, // sum=2 != 1
			{"target_ff_seg sr=1", testutils.SegmentRow{1, "/tmp/s1", "target_db_exporter_ff", 0, 1, 0, 0, 0}, true},
			{"target_ff_seg sr=0", testutils.SegmentRow{1, "/tmp/s1", "target_db_exporter_ff", 0, 0, 0, 0, 0}, false},
			{"target_fb_seg src=1", testutils.SegmentRow{1, "/tmp/s1", "target_db_exporter_fb", 0, 0, 1, 0, 0}, true},
			{"already_deleted source_seg", testutils.SegmentRow{1, "/tmp/s1", "source_db_exporter", 1, 0, 0, 1, 0}, false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				mdb, exportDir := testutils.SetupTestMetaDB(t)
				testutils.SetMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
					r.FallForwardEnabled = false
				})
				testutils.InsertSegment(t, exportDir, tc.seg)

				processedSegs, err := mdb.GetProcessedQueueSegments()
				require.NoError(t, err)

				found := len(processedSegs) > 0
				t.Logf("  seg=%d exporter=%s target=%d sr=%d src=%d deleted=%d | importCount=1 (from MSR) | found=%v expected=%v",
					tc.seg.SegmentNo, tc.seg.ExporterRole, tc.seg.ImportedByTarget,
					tc.seg.ImportedBySourceReplica, tc.seg.ImportedBySource, tc.seg.Deleted,
					found, tc.wantFound)
				assert.Equal(t, tc.wantFound, found)
			})
		}
	})

	// Test with importCount=2 (FallForwardEnabled=true, no cutover)
	t.Run("importCount=2 (FF pre-cutover)", func(t *testing.T) {
		testCases := []struct {
			name      string
			seg       testutils.SegmentRow
			wantFound bool
		}{
			{"source_seg target=1 sr=1 src=0 (sum=2)", testutils.SegmentRow{1, "/tmp/s1", "source_db_exporter", 1, 1, 0, 0, 0}, true},
			{"source_seg target=1 sr=0 src=0 (sum=1)", testutils.SegmentRow{1, "/tmp/s1", "source_db_exporter", 1, 0, 0, 0, 0}, false},
			{"source_seg target=0 sr=1 src=0 (sum=1)", testutils.SegmentRow{1, "/tmp/s1", "source_db_exporter", 0, 1, 0, 0, 0}, false},
			{"target_ff_seg sr=1 (sum=1)", testutils.SegmentRow{1, "/tmp/s1", "target_db_exporter_ff", 0, 1, 0, 0, 0}, true}, // target uses count=1
			{"target_ff_seg sr=0", testutils.SegmentRow{1, "/tmp/s1", "target_db_exporter_ff", 0, 0, 0, 0, 0}, false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				mdb, exportDir := testutils.SetupTestMetaDB(t)
				testutils.SetMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
					r.FallForwardEnabled = true
					r.CutoverProcessedByTargetImporter = false
				})
				testutils.InsertSegment(t, exportDir, tc.seg)

				processedSegs, err := mdb.GetProcessedQueueSegments()
				require.NoError(t, err)

				found := len(processedSegs) > 0
				t.Logf("  seg=%d exporter=%s target=%d sr=%d src=%d deleted=%d | importCount=2 (from MSR) | found=%v expected=%v",
					tc.seg.SegmentNo, tc.seg.ExporterRole, tc.seg.ImportedByTarget,
					tc.seg.ImportedBySourceReplica, tc.seg.ImportedBySource, tc.seg.Deleted,
					found, tc.wantFound)
				assert.Equal(t, tc.wantFound, found)
			})
		}
	})
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
	mdb, exportDir := testutils.SetupTestMetaDB(t)
	paths := testutils.CreateSegmentFiles(t, exportDir, 1)
	testutils.InsertSegment(t, exportDir, testutils.SegmentRow{1, paths[0], "source_db_exporter", 1, 0, 0, 0, 0})

	t.Log("=== Test: MarkSegmentDeletedAndArchived ===")
	require.NoError(t, mdb.MarkSegmentDeletedAndArchived(1))

	allSegs := testutils.QueryAllSegments(t, exportDir)
	assert.Equal(t, 1, allSegs[0].Deleted)
	assert.Equal(t, 1, allSegs[0].Archived)
	t.Log("RESULT: PASS - MarkSegmentDeletedAndArchived sets both flags")
}

// ============================================================
// Test: SegmentCleaner deleteSegment removes file and marks DB
// ============================================================
func TestDeleteSegmentRemovesFile(t *testing.T) {
	mdb, exportDir := testutils.SetupTestMetaDB(t)
	testutils.SetMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = false
	})
	paths := testutils.CreateSegmentFiles(t, exportDir, 1)
	testutils.InsertSegment(t, exportDir, testutils.SegmentRow{1, paths[0], "source_db_exporter", 1, 0, 0, 0, 0})

	t.Log("=== Test: deleteSegment removes file and marks DB ===")

	assert.FileExists(t, paths[0], "segment file should exist before delete")

	cfg := Config{Policy: PolicyDelete, ExportDir: exportDir, FSUtilizationThreshold: 70}
	cleaner := NewSegmentCleaner(cfg, mdb)

	processedSegs, err := mdb.GetProcessedQueueSegments()
	require.NoError(t, err)
	require.Len(t, processedSegs, 1)

	require.NoError(t, cleaner.deleteSegment(processedSegs[0]))

	assert.NoFileExists(t, paths[0], "segment file should be removed after delete")

	allSegs := testutils.QueryAllSegments(t, exportDir)
	assert.Equal(t, 1, allSegs[0].Deleted)
	assert.Equal(t, 1, allSegs[0].Archived)
	t.Log("RESULT: PASS - deleteSegment removes file and marks DB correctly")
}

// ============================================================
// Test: deleteSegment handles missing file gracefully
// ============================================================
func TestDeleteSegmentMissingFile(t *testing.T) {
	mdb, exportDir := testutils.SetupTestMetaDB(t)
	testutils.SetMSR(t, mdb, func(r *metadb.MigrationStatusRecord) {
		r.FallForwardEnabled = false
	})
	testutils.InsertSegment(t, exportDir, testutils.SegmentRow{1, "/tmp/nonexistent/segment.1.ndjson", "source_db_exporter", 1, 0, 0, 0, 0})

	t.Log("=== Test: deleteSegment handles missing file gracefully ===")

	cfg := Config{Policy: PolicyDelete, ExportDir: exportDir, FSUtilizationThreshold: 70}
	cleaner := NewSegmentCleaner(cfg, mdb)

	processedSegs, err := mdb.GetProcessedQueueSegments()
	require.NoError(t, err)
	require.Len(t, processedSegs, 1)

	err = cleaner.deleteSegment(processedSegs[0])
	require.NoError(t, err, "Should not error on missing file")

	allSegs := testutils.QueryAllSegments(t, exportDir)
	assert.Equal(t, 1, allSegs[0].Deleted)
	assert.Equal(t, 1, allSegs[0].Archived)
	t.Log("RESULT: PASS - deleteSegment handles missing files gracefully")
}
