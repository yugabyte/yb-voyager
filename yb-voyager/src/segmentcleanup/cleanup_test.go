//go:build unit

package segmentcleanup

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

// setupTestMetaDB creates a temp export dir with a properly initialized metaDB
// and returns the MetaDB handle plus a cleanup function.
func setupTestMetaDB(t *testing.T) (string, *metadb.MetaDB, func()) {
	t.Helper()

	exportDir, err := os.MkdirTemp("", "segclean_test_*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}

	metaDir := filepath.Join(exportDir, "metainfo")
	if err := os.MkdirAll(metaDir, 0755); err != nil {
		t.Fatalf("mkdir metainfo: %v", err)
	}

	dbPath := filepath.Join(metaDir, "meta.db")
	f, err := os.Create(dbPath)
	if err != nil {
		t.Fatalf("create meta.db: %v", err)
	}
	f.Close()

	conn, err := sql.Open("sqlite3", fmt.Sprintf("%s?_txlock=exclusive&_timeout=30000", dbPath))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	cmds := []string{
		`CREATE TABLE queue_segment_meta 
      (segment_no INTEGER PRIMARY KEY, 
       file_path TEXT, size_committed INTEGER, 
       total_events INTEGER,
       exporter_role TEXT,
       imported_by_target_db_importer INTEGER DEFAULT 0, 
       imported_by_source_replica_db_importer INTEGER DEFAULT 0, 
       imported_by_source_db_importer INTEGER DEFAULT 0, 
       archived INTEGER DEFAULT 0,
       deleted INTEGER DEFAULT 0,
       archive_location TEXT);`,
		`CREATE TABLE json_objects (
			key TEXT PRIMARY KEY,
			json_text TEXT);`,
	}
	for _, cmd := range cmds {
		if _, err := conn.Exec(cmd); err != nil {
			t.Fatalf("init table: %v", err)
		}
	}

	msr := metadb.MigrationStatusRecord{
		ExportDataSourceDebeziumStarted: true,
		MigrationUUID:                   "test-uuid",
	}
	msrJSON, _ := json.Marshal(&msr)
	_, err = conn.Exec(`INSERT INTO json_objects (key, json_text) VALUES (?, ?)`,
		"migration_status", string(msrJSON))
	if err != nil {
		t.Fatalf("insert MSR: %v", err)
	}
	conn.Close()

	mdb, err := metadb.NewMetaDB(exportDir)
	if err != nil {
		t.Fatalf("NewMetaDB: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(exportDir)
	}
	return exportDir, mdb, cleanup
}

func insertSegment(t *testing.T, exportDir string, segNum int, filePath string, exporterRole string, importedByTarget int) {
	t.Helper()
	dbPath := filepath.Join(exportDir, "metainfo", "meta.db")
	conn, err := sql.Open("sqlite3", fmt.Sprintf("%s?_txlock=exclusive&_timeout=30000", dbPath))
	if err != nil {
		t.Fatalf("open sqlite for insert: %v", err)
	}
	defer conn.Close()
	_, err = conn.Exec(`INSERT INTO queue_segment_meta 
		(segment_no, file_path, size_committed, total_events, exporter_role, imported_by_target_db_importer) 
		VALUES (?, ?, 100, 10, ?, ?)`, segNum, filePath, exporterRole, importedByTarget)
	if err != nil {
		t.Fatalf("insert segment %d: %v", segNum, err)
	}
}

func querySegmentMeta(t *testing.T, exportDir string, segNum int) (archived int, deleted int, archiveLocation string) {
	t.Helper()
	dbPath := filepath.Join(exportDir, "metainfo", "meta.db")
	conn, err := sql.Open("sqlite3", fmt.Sprintf("%s?_txlock=exclusive&_timeout=30000", dbPath))
	if err != nil {
		t.Fatalf("open sqlite for query: %v", err)
	}
	defer conn.Close()
	var loc sql.NullString
	err = conn.QueryRow(`SELECT archived, deleted, archive_location FROM queue_segment_meta WHERE segment_no = ?`, segNum).
		Scan(&archived, &deleted, &loc)
	if err != nil {
		t.Fatalf("query segment %d: %v", segNum, err)
	}
	if loc.Valid {
		archiveLocation = loc.String
	}
	return
}

// ========== Section 1.5: IsValidPolicy ===========

func TestIsValidPolicy_Archive(t *testing.T) {
	if !IsValidPolicy("archive") {
		t.Errorf("IsValidPolicy(\"archive\") = false, want true")
	}
}

func TestIsValidPolicy_Delete(t *testing.T) {
	if !IsValidPolicy("delete") {
		t.Errorf("IsValidPolicy(\"delete\") = false, want true")
	}
}

func TestIsValidPolicy_Retain(t *testing.T) {
	if !IsValidPolicy("retain") {
		t.Errorf("IsValidPolicy(\"retain\") = false, want true")
	}
}

func TestIsValidPolicy_Invalid(t *testing.T) {
	for _, p := range []string{"bogus", "ARCHIVE", "Delete", "", "archiv"} {
		if IsValidPolicy(p) {
			t.Errorf("IsValidPolicy(%q) = true, want false", p)
		}
	}
}

// ========== Section 2: Core Archival Behavior ===========

func TestArchivePolicy_ProcessedSegmentsArchivedAndDeleted(t *testing.T) {
	exportDir, mdb, cleanup := setupTestMetaDB(t)
	defer cleanup()

	archiveDir, err := os.MkdirTemp("", "archive_test_*")
	if err != nil {
		t.Fatalf("create archive dir: %v", err)
	}
	defer os.RemoveAll(archiveDir)

	segFile := filepath.Join(exportDir, "segment_1.dat")
	os.WriteFile(segFile, []byte("segment-1-content"), 0644)

	insertSegment(t, exportDir, 1, segFile, "source_db_exporter", 1)

	cfg := Config{
		Policy:     PolicyArchive,
		ExportDir:  exportDir,
		ArchiveDir: archiveDir,
	}
	sc := NewSegmentCleaner(cfg, mdb)

	go func() {
		time.Sleep(7 * time.Second)
		sc.SignalStop()
	}()

	err = sc.Run()
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	archived, deleted, archiveLoc := querySegmentMeta(t, exportDir, 1)
	if archived != 1 {
		t.Errorf("archived = %d, want 1", archived)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}
	expectedLoc := filepath.Join(archiveDir, "segment_1.dat")
	if archiveLoc != expectedLoc {
		t.Errorf("archive_location = %q, want %q", archiveLoc, expectedLoc)
	}

	if utils.FileOrFolderExists(segFile) {
		t.Errorf("original segment file still exists after archival")
	}
	if !utils.FileOrFolderExists(expectedLoc) {
		t.Errorf("archived file does not exist at %s", expectedLoc)
	}
}

func TestArchivePolicy_MultipleSegmentsInOneTick(t *testing.T) {
	exportDir, mdb, cleanup := setupTestMetaDB(t)
	defer cleanup()

	archiveDir, err := os.MkdirTemp("", "archive_multi_*")
	if err != nil {
		t.Fatalf("create archive dir: %v", err)
	}
	defer os.RemoveAll(archiveDir)

	for i := 1; i <= 4; i++ {
		segFile := filepath.Join(exportDir, fmt.Sprintf("segment_%d.dat", i))
		os.WriteFile(segFile, []byte(fmt.Sprintf("content-%d", i)), 0644)
		insertSegment(t, exportDir, i, segFile, "source_db_exporter", 1)
	}

	cfg := Config{
		Policy:     PolicyArchive,
		ExportDir:  exportDir,
		ArchiveDir: archiveDir,
	}
	sc := NewSegmentCleaner(cfg, mdb)

	go func() {
		time.Sleep(7 * time.Second)
		sc.SignalStop()
	}()

	err = sc.Run()
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	for i := 1; i <= 4; i++ {
		archived, deleted, _ := querySegmentMeta(t, exportDir, i)
		if archived != 1 || deleted != 1 {
			t.Errorf("segment %d: archived=%d deleted=%d, want both 1", i, archived, deleted)
		}
		archivePath := filepath.Join(archiveDir, fmt.Sprintf("segment_%d.dat", i))
		if !utils.FileOrFolderExists(archivePath) {
			t.Errorf("archived file for segment %d does not exist", i)
		}
	}
}

func TestArchivePolicy_NoFSUtilizationGating(t *testing.T) {
	exportDir, mdb, cleanup := setupTestMetaDB(t)
	defer cleanup()

	archiveDir, err := os.MkdirTemp("", "archive_nofs_*")
	if err != nil {
		t.Fatalf("create archive dir: %v", err)
	}
	defer os.RemoveAll(archiveDir)

	segFile := filepath.Join(exportDir, "seg_nofs.dat")
	os.WriteFile(segFile, []byte("nofs-content"), 0644)
	insertSegment(t, exportDir, 1, segFile, "source_db_exporter", 1)

	cfg := Config{
		Policy:                 PolicyArchive,
		ExportDir:              exportDir,
		ArchiveDir:             archiveDir,
		FSUtilizationThreshold: 99,
	}
	sc := NewSegmentCleaner(cfg, mdb)

	go func() {
		time.Sleep(7 * time.Second)
		sc.SignalStop()
	}()

	err = sc.Run()
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	archived, deleted, _ := querySegmentMeta(t, exportDir, 1)
	if archived != 1 || deleted != 1 {
		t.Errorf("segment archived=%d deleted=%d, want both 1 (FS utilization should not gate archive)", archived, deleted)
	}
}

func TestArchivePolicy_FileContentIntegrity(t *testing.T) {
	exportDir, mdb, cleanup := setupTestMetaDB(t)
	defer cleanup()

	archiveDir, err := os.MkdirTemp("", "archive_integrity_*")
	if err != nil {
		t.Fatalf("create archive dir: %v", err)
	}
	defer os.RemoveAll(archiveDir)

	content := []byte("known-content-for-integrity-check-1234567890")
	segFile := filepath.Join(exportDir, "integrity_seg.dat")
	os.WriteFile(segFile, content, 0644)
	insertSegment(t, exportDir, 1, segFile, "source_db_exporter", 1)

	cfg := Config{
		Policy:     PolicyArchive,
		ExportDir:  exportDir,
		ArchiveDir: archiveDir,
	}
	sc := NewSegmentCleaner(cfg, mdb)

	go func() {
		time.Sleep(7 * time.Second)
		sc.SignalStop()
	}()

	err = sc.Run()
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	archivePath := filepath.Join(archiveDir, "integrity_seg.dat")
	archivedContent, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("read archived file: %v", err)
	}
	if string(archivedContent) != string(content) {
		t.Errorf("archived content mismatch: got %q, want %q", archivedContent, content)
	}
}

// ========== Section 3: MetaDB State Transitions ===========

func TestMetaDB_ArchiveLocationPathFormat(t *testing.T) {
	exportDir, mdb, cleanup := setupTestMetaDB(t)
	defer cleanup()

	archiveDir, err := os.MkdirTemp("", "archive_path_*")
	if err != nil {
		t.Fatalf("create archive dir: %v", err)
	}
	defer os.RemoveAll(archiveDir)

	segFile := filepath.Join(exportDir, "subdir", "seg_path.dat")
	os.MkdirAll(filepath.Dir(segFile), 0755)
	os.WriteFile(segFile, []byte("path-test"), 0644)
	insertSegment(t, exportDir, 1, segFile, "source_db_exporter", 1)

	cfg := Config{
		Policy:     PolicyArchive,
		ExportDir:  exportDir,
		ArchiveDir: archiveDir,
	}
	sc := NewSegmentCleaner(cfg, mdb)

	go func() {
		time.Sleep(7 * time.Second)
		sc.SignalStop()
	}()

	err = sc.Run()
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	_, _, archiveLoc := querySegmentMeta(t, exportDir, 1)
	expected := filepath.Join(archiveDir, "seg_path.dat")
	if archiveLoc != expected {
		t.Errorf("archive_location = %q, want %q (should use basename only)", archiveLoc, expected)
	}
	// Verify no double slashes
	if filepath.Clean(archiveLoc) != archiveLoc {
		t.Errorf("archive_location has unclean path: %q", archiveLoc)
	}
}

func TestMetaDB_SegmentNotReturnedAfterArchival(t *testing.T) {
	exportDir, mdb, cleanup := setupTestMetaDB(t)
	defer cleanup()

	archiveDir, err := os.MkdirTemp("", "archive_notret_*")
	if err != nil {
		t.Fatalf("create archive dir: %v", err)
	}
	defer os.RemoveAll(archiveDir)

	segFile := filepath.Join(exportDir, "notret_seg.dat")
	os.WriteFile(segFile, []byte("notret"), 0644)
	insertSegment(t, exportDir, 1, segFile, "source_db_exporter", 1)

	cfg := Config{
		Policy:     PolicyArchive,
		ExportDir:  exportDir,
		ArchiveDir: archiveDir,
	}
	sc := NewSegmentCleaner(cfg, mdb)

	go func() {
		time.Sleep(7 * time.Second)
		sc.SignalStop()
	}()

	err = sc.Run()
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	segments, err := mdb.GetProcessedQueueSegments()
	if err != nil {
		t.Fatalf("GetProcessedQueueSegments: %v", err)
	}
	if len(segments) != 0 {
		t.Errorf("GetProcessedQueueSegments returned %d segments, want 0 (deleted=1 should filter them)", len(segments))
	}
}

// ========== Section 4: Edge Cases ===========

func TestArchivePolicy_SegmentFileMissing(t *testing.T) {
	exportDir, mdb, cleanup := setupTestMetaDB(t)
	defer cleanup()

	archiveDir, err := os.MkdirTemp("", "archive_miss_*")
	if err != nil {
		t.Fatalf("create archive dir: %v", err)
	}
	defer os.RemoveAll(archiveDir)

	segFile := filepath.Join(exportDir, "missing_seg.dat")
	insertSegment(t, exportDir, 1, segFile, "source_db_exporter", 1)

	cfg := Config{
		Policy:     PolicyArchive,
		ExportDir:  exportDir,
		ArchiveDir: archiveDir,
	}
	sc := NewSegmentCleaner(cfg, mdb)

	go func() {
		time.Sleep(7 * time.Second)
		sc.SignalStop()
	}()

	err = sc.Run()
	if err != nil {
		t.Fatalf("Run() should succeed even with missing file, got: %v", err)
	}

	archived, deleted, _ := querySegmentMeta(t, exportDir, 1)
	if archived != 1 || deleted != 1 {
		t.Errorf("missing file: archived=%d deleted=%d, want both 1 (idempotent)", archived, deleted)
	}
}

func TestArchivePolicy_ArchiveFileAlreadyExists(t *testing.T) {
	exportDir, mdb, cleanup := setupTestMetaDB(t)
	defer cleanup()

	archiveDir, err := os.MkdirTemp("", "archive_exist_*")
	if err != nil {
		t.Fatalf("create archive dir: %v", err)
	}
	defer os.RemoveAll(archiveDir)

	segFile := filepath.Join(exportDir, "exist_seg.dat")
	os.WriteFile(segFile, []byte("new-content"), 0644)
	insertSegment(t, exportDir, 1, segFile, "source_db_exporter", 1)

	preExist := filepath.Join(archiveDir, "exist_seg.dat")
	os.WriteFile(preExist, []byte("old-content"), 0644)

	cfg := Config{
		Policy:     PolicyArchive,
		ExportDir:  exportDir,
		ArchiveDir: archiveDir,
	}
	sc := NewSegmentCleaner(cfg, mdb)

	go func() {
		time.Sleep(7 * time.Second)
		sc.SignalStop()
	}()

	err = sc.Run()
	if err != nil {
		t.Fatalf("Run() with pre-existing archive file failed: %v", err)
	}

	archivedContent, _ := os.ReadFile(preExist)
	if string(archivedContent) != "new-content" {
		t.Errorf("archive file content = %q, want %q (pre-existing should be overwritten)", archivedContent, "new-content")
	}
}

func TestArchivePolicy_NoProcessedSegments(t *testing.T) {
	exportDir, mdb, cleanup := setupTestMetaDB(t)
	defer cleanup()

	archiveDir, err := os.MkdirTemp("", "archive_noseg_*")
	if err != nil {
		t.Fatalf("create archive dir: %v", err)
	}
	defer os.RemoveAll(archiveDir)

	segFile := filepath.Join(exportDir, "pending_seg.dat")
	os.WriteFile(segFile, []byte("pending"), 0644)
	insertSegment(t, exportDir, 1, segFile, "source_db_exporter", 0)

	cfg := Config{
		Policy:     PolicyArchive,
		ExportDir:  exportDir,
		ArchiveDir: archiveDir,
	}
	sc := NewSegmentCleaner(cfg, mdb)

	done := make(chan error, 1)
	go func() {
		done <- sc.Run()
	}()

	time.Sleep(6 * time.Second)

	entries, _ := os.ReadDir(archiveDir)
	if len(entries) != 0 {
		t.Errorf("archive dir has %d files, want 0 (no processed segments yet)", len(entries))
	}

	sc.SignalStop()

	// Mark segment as processed so the cleaner can drain
	dbPath := filepath.Join(exportDir, "metainfo", "meta.db")
	conn, _ := sql.Open("sqlite3", fmt.Sprintf("%s?_txlock=exclusive&_timeout=30000", dbPath))
	conn.Exec(`UPDATE queue_segment_meta SET imported_by_target_db_importer = 1 WHERE segment_no = 1`)
	conn.Close()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error after stop: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("Run() did not exit after SignalStop + marking segment processed")
	}
}

func TestArchivePolicy_SegmentsArriveIncrementally(t *testing.T) {
	exportDir, mdb, cleanup := setupTestMetaDB(t)
	defer cleanup()

	archiveDir, err := os.MkdirTemp("", "archive_incr_*")
	if err != nil {
		t.Fatalf("create archive dir: %v", err)
	}
	defer os.RemoveAll(archiveDir)

	cfg := Config{
		Policy:     PolicyArchive,
		ExportDir:  exportDir,
		ArchiveDir: archiveDir,
	}
	sc := NewSegmentCleaner(cfg, mdb)

	done := make(chan error, 1)
	go func() {
		done <- sc.Run()
	}()

	time.Sleep(6 * time.Second)

	segFile := filepath.Join(exportDir, "incr_seg.dat")
	os.WriteFile(segFile, []byte("incremental"), 0644)
	insertSegment(t, exportDir, 1, segFile, "source_db_exporter", 1)

	time.Sleep(7 * time.Second)

	archived, deleted, _ := querySegmentMeta(t, exportDir, 1)
	if archived != 1 || deleted != 1 {
		t.Errorf("incremental segment: archived=%d deleted=%d, want both 1", archived, deleted)
	}

	sc.SignalStop()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("Run() did not exit after SignalStop")
	}
}

// ========== Section 5: Stop/Drain Behavior ===========

func TestSignalStop_NoRemainingSegments(t *testing.T) {
	exportDir, mdb, cleanup := setupTestMetaDB(t)
	defer cleanup()

	archiveDir, err := os.MkdirTemp("", "archive_stop1_*")
	if err != nil {
		t.Fatalf("create archive dir: %v", err)
	}
	defer os.RemoveAll(archiveDir)

	segFile := filepath.Join(exportDir, "stop1_seg.dat")
	os.WriteFile(segFile, []byte("stop1"), 0644)
	insertSegment(t, exportDir, 1, segFile, "source_db_exporter", 1)

	cfg := Config{
		Policy:     PolicyArchive,
		ExportDir:  exportDir,
		ArchiveDir: archiveDir,
	}
	sc := NewSegmentCleaner(cfg, mdb)

	done := make(chan error, 1)
	go func() {
		done <- sc.Run()
	}()

	time.Sleep(7 * time.Second)

	archived, _, _ := querySegmentMeta(t, exportDir, 1)
	if archived != 1 {
		t.Fatalf("segment should be archived before SignalStop test")
	}

	sc.SignalStop()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("cleaner did not exit cleanly after SignalStop with no remaining segments")
	}
}

func TestSignalStop_WithProcessedSegmentsNotYetArchived(t *testing.T) {
	exportDir, mdb, cleanup := setupTestMetaDB(t)
	defer cleanup()

	archiveDir, err := os.MkdirTemp("", "archive_stop3_*")
	if err != nil {
		t.Fatalf("create archive dir: %v", err)
	}
	defer os.RemoveAll(archiveDir)

	for i := 1; i <= 5; i++ {
		segFile := filepath.Join(exportDir, fmt.Sprintf("stop3_seg_%d.dat", i))
		os.WriteFile(segFile, []byte(fmt.Sprintf("stop3-content-%d", i)), 0644)
		insertSegment(t, exportDir, i, segFile, "source_db_exporter", 1)
	}

	cfg := Config{
		Policy:     PolicyArchive,
		ExportDir:  exportDir,
		ArchiveDir: archiveDir,
	}
	sc := NewSegmentCleaner(cfg, mdb)

	sc.SignalStop()

	done := make(chan error, 1)
	go func() {
		done <- sc.Run()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error: %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("cleaner did not exit after SignalStop with 5 processed segments")
	}

	for i := 1; i <= 5; i++ {
		archived, deleted, _ := querySegmentMeta(t, exportDir, i)
		if archived != 1 || deleted != 1 {
			t.Errorf("segment %d: archived=%d deleted=%d, want both 1", i, archived, deleted)
		}
	}
}

// ========== Section 6: Migration Status Record (partial - tested via command) ===========

// ========== Section 8: Regression — Delete Policy ===========

func TestDeletePolicy_FSUtilizationGating(t *testing.T) {
	exportDir, mdb, cleanup := setupTestMetaDB(t)
	defer cleanup()

	segFile := filepath.Join(exportDir, "del_seg.dat")
	os.WriteFile(segFile, []byte("delete-me"), 0644)
	insertSegment(t, exportDir, 1, segFile, "source_db_exporter", 1)

	cfg := Config{
		Policy:                 PolicyDelete,
		ExportDir:              exportDir,
		FSUtilizationThreshold: 99,
	}
	sc := NewSegmentCleaner(cfg, mdb)

	done := make(chan error, 1)
	go func() {
		done <- sc.Run()
	}()

	time.Sleep(7 * time.Second)

	archived, deleted, _ := querySegmentMeta(t, exportDir, 1)
	if deleted != 0 {
		t.Errorf("delete policy should NOT delete segment when FS util < threshold (99%%), deleted=%d", deleted)
	}
	_ = archived

	sc.SignalStop()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("cleaner did not exit")
	}
}

func TestDeletePolicy_DefaultBehavior(t *testing.T) {
	exportDir, mdb, cleanup := setupTestMetaDB(t)
	defer cleanup()

	segFile := filepath.Join(exportDir, "del_def_seg.dat")
	os.WriteFile(segFile, []byte("default-del"), 0644)
	insertSegment(t, exportDir, 1, segFile, "source_db_exporter", 1)

	cfg := Config{
		Policy:                 PolicyDelete,
		ExportDir:              exportDir,
		FSUtilizationThreshold: 0,
	}
	sc := NewSegmentCleaner(cfg, mdb)

	go func() {
		time.Sleep(7 * time.Second)
		sc.SignalStop()
	}()

	err := sc.Run()
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	archived, deleted, _ := querySegmentMeta(t, exportDir, 1)
	if archived != 1 || deleted != 1 {
		t.Errorf("delete policy with threshold=0: archived=%d deleted=%d, want both 1", archived, deleted)
	}
	if utils.FileOrFolderExists(segFile) {
		t.Errorf("segment file should be removed by delete policy")
	}
}

func TestRetainPolicy_NoFileDeletion(t *testing.T) {
	exportDir, mdb, cleanup := setupTestMetaDB(t)
	defer cleanup()

	segFile := filepath.Join(exportDir, "retain_seg.dat")
	os.WriteFile(segFile, []byte("retain-me"), 0644)
	insertSegment(t, exportDir, 1, segFile, "source_db_exporter", 1)

	cfg := Config{
		Policy:                 PolicyRetain,
		ExportDir:              exportDir,
		FSUtilizationThreshold: 0,
	}
	sc := NewSegmentCleaner(cfg, mdb)

	done := make(chan error, 1)
	go func() {
		done <- sc.Run()
	}()

	time.Sleep(3 * time.Second)
	sc.SignalStop()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error: %v", err)
		}
	case <-time.After(40 * time.Second):
		t.Fatal("retain policy did not exit")
	}

	if !utils.FileOrFolderExists(segFile) {
		t.Errorf("retain policy should NOT delete segment files")
	}
}
