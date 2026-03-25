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

package testutils

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
)

// SegmentRow represents a row in the queue_segment_meta table for testing.
type SegmentRow struct {
	SegmentNo               int
	FilePath                string
	ExporterRole            string
	ImportedByTarget        int
	ImportedBySourceReplica int
	ImportedBySource        int
	Deleted                 int
	Archived                int
}

// SetupTestMetaDB creates a temporary meta database for testing segment cleanup.
// Returns the MetaDB instance and the export directory path.
func SetupTestMetaDB(t *testing.T) (*metadb.MetaDB, string) {
	tmpDir := t.TempDir()
	exportDir := tmpDir
	metainfoDir := filepath.Join(exportDir, "metainfo")
	require.NoError(t, os.MkdirAll(metainfoDir, 0755))
	require.NoError(t, metadb.CreateAndInitMetaDBIfRequired(exportDir))
	mdb, err := metadb.NewMetaDB(exportDir)
	require.NoError(t, err)
	return mdb, exportDir
}

// InsertSegment inserts a row into the queue_segment_meta table in the test meta database.
// It is a helper for creating mock segment metadata for tests.
// The seg argument defines values for all columns in queue_segment_meta.
// Example usage:
//
//	InsertSegment(t, exportDir, SegmentRow{1, "/tmp/seg1", "source_db_exporter", 1, 0, 0, 0, 0})
func InsertSegment(t *testing.T, exportDir string, seg SegmentRow) {
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

// QueryAllSegments retrieves all segments from the queue_segment_meta table for verification.
func QueryAllSegments(t *testing.T, exportDir string) []SegmentRow {
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
	var result []SegmentRow
	for rows.Next() {
		var r SegmentRow
		require.NoError(t, rows.Scan(&r.SegmentNo, &r.FilePath, &r.ExporterRole,
			&r.ImportedByTarget, &r.ImportedBySourceReplica, &r.ImportedBySource,
			&r.Deleted, &r.Archived))
		result = append(result, r)
	}
	require.NoError(t, rows.Err())
	return result
}

// SetMSR updates the MigrationStatusRecord in the test database.
func SetMSR(t *testing.T, mdb *metadb.MetaDB, updateFn func(*metadb.MigrationStatusRecord)) {
	require.NoError(t, mdb.UpdateMigrationStatusRecord(updateFn))
}

// CreateSegmentFiles creates mock segment files in the data directory.
// Returns the paths of the created files.
func CreateSegmentFiles(t *testing.T, exportDir string, count int) []string {
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
