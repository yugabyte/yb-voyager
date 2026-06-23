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
package metadb

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// Test the initMetaDB function
func TestInitMetaDB(t *testing.T) {
	// Define the expected columns and their types for each table
	expectedTables := map[string]map[string]testutils.ColumnPropertiesSqlite{
		QUEUE_SEGMENT_META_TABLE_NAME: {
			"segment_no":                             {Type: "INTEGER", PrimaryKey: 1},
			"file_path":                              {Type: "TEXT"},
			"size_committed":                         {Type: "INTEGER"},
			"total_events":                           {Type: "INTEGER"},
			"exporter_role":                          {Type: "TEXT"},
			"imported_by_target_db_importer":         {Type: "INTEGER", Default: sql.NullString{String: "0", Valid: true}},
			"imported_by_source_replica_db_importer": {Type: "INTEGER", Default: sql.NullString{String: "0", Valid: true}},
			"imported_by_source_db_importer":         {Type: "INTEGER", Default: sql.NullString{String: "0", Valid: true}},
			"archived":                               {Type: "INTEGER", Default: sql.NullString{String: "0", Valid: true}},
			"deleted":                                {Type: "INTEGER", Default: sql.NullString{String: "0", Valid: true}},
			"archive_location":                       {Type: "TEXT"},
		},
		EXPORTED_EVENTS_STATS_TABLE_NAME: {
			// TODO: We have a composite primary key here (run_id, exporter_role, timestamp)
			"run_id":        {Type: "TEXT", PrimaryKey: 1},
			"exporter_role": {Type: "TEXT", PrimaryKey: 2},
			"timestamp":     {Type: "INTEGER", PrimaryKey: 3},
			"num_total":     {Type: "INTEGER"},
			"num_inserts":   {Type: "INTEGER"},
			"num_updates":   {Type: "INTEGER"},
			"num_deletes":   {Type: "INTEGER"},
		},
		EXPORTED_EVENTS_STATS_PER_TABLE_TABLE_NAME: {
			"exporter_role": {Type: "TEXT", PrimaryKey: 1},
			"schema_name":   {Type: "TEXT", PrimaryKey: 2},
			"table_name":    {Type: "TEXT", PrimaryKey: 3},
			"num_total":     {Type: "INTEGER"},
			"num_inserts":   {Type: "INTEGER"},
			"num_updates":   {Type: "INTEGER"},
			"num_deletes":   {Type: "INTEGER"},
		},
		JSON_OBJECTS_TABLE_NAME: {
			"key":       {Type: "TEXT", PrimaryKey: 1},
			"json_text": {Type: "TEXT"},
		},
	}

	// Create a temporary SQLite database file for testing
	tempFile, err := os.CreateTemp(os.TempDir(), "test_meta_db_*.db")
	if err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}

	// remove the temporary file
	defer func() {
		err := os.Remove(tempFile.Name())
		if err != nil {
			t.Fatalf("Failed to remove temporary file: %v", err)
		}
	}()

	// Call initMetaDB with the path to the temporary file
	err = initMetaDB(tempFile.Name()) // Pass the temp file path to initMetaDB
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	} else {
		t.Logf("Database initialized successfully")
	}

	// Open the temporary database for verification
	db, err := sql.Open("sqlite3", tempFile.Name())
	if err != nil {
		t.Fatalf("Failed to open temporary database: %v", err)
	}
	defer db.Close()

	// Verify the existence of each table and no extra tables
	t.Run("Check table existence and no extra tables", func(t *testing.T) {
		err := testutils.CheckTableExistenceSqlite(t, db, expectedTables)
		if err != nil {
			t.Errorf("Table existence mismatch: %v", err)
		}
	})

	// Verify the structure of each table
	for table, expectedColumns := range expectedTables {
		t.Run(fmt.Sprintf("Check structure of %s table", table), func(t *testing.T) {
			err := testutils.CheckTableStructureSqlite(db, table, expectedColumns)
			if err != nil {
				t.Errorf("Table %s structure mismatch: %v", table, err)
			}
		})
	}
}

// testSourceDBExporterRole is redeclared here to avoid an import cycle with the
// cmd package. It must stay in sync with cmd.SOURCE_DB_EXPORTER_ROLE, which the
// AnySegmentsDeletedOrArchived callers (importData.go guardrail and
// eventQueue.go resolveSegmentToResumeFrom) pass in.
const testSourceDBExporterRole = "source_db_exporter"

func newTestMetaDB(t *testing.T) *MetaDB {
	t.Helper()
	exportDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(exportDir, "metainfo"), 0755))
	require.NoError(t, CreateAndInitMetaDBIfRequired(exportDir))
	mdb, err := NewMetaDB(exportDir)
	require.NoError(t, err)
	t.Cleanup(func() { mdb.db.Close() })
	return mdb
}

func insertQueueSegment(t *testing.T, mdb *MetaDB, segmentNo int, exporterRole string, archived int, deleted int) {
	t.Helper()
	query := fmt.Sprintf(`INSERT INTO %s (segment_no, file_path, exporter_role, archived, deleted) VALUES (?, ?, ?, ?, ?)`,
		QUEUE_SEGMENT_META_TABLE_NAME)
	_, err := mdb.db.Exec(query, segmentNo,
		fmt.Sprintf("segment.%d.ndjson", segmentNo), exporterRole, archived, deleted)
	require.NoError(t, err)
}

// TestAnySegmentsDeletedOrArchived covers the start-clean guardrail backing
// AnySegmentsDeletedOrArchived. The guardrail must agree with
// resolveSegmentToResumeFrom in eventQueue.go on which segment is the resume
// point for importers that support --start-clean: the earliest segment_no.
func TestAnySegmentsDeletedOrArchived(t *testing.T) {
	t.Run("no queue segments returns false", func(t *testing.T) {
		mdb := newTestMetaDB(t)

		deleted, err := mdb.AnySegmentsDeletedOrArchived()
		require.NoError(t, err)
		assert.False(t, deleted, "with no queue segments the guardrail must not block start-clean")
	})

	t.Run("earliest segment deleted returns true", func(t *testing.T) {
		mdb := newTestMetaDB(t)
		insertQueueSegment(t, mdb, 1, testSourceDBExporterRole, 0, 1)
		insertQueueSegment(t, mdb, 2, testSourceDBExporterRole, 0, 0)

		deleted, err := mdb.AnySegmentsDeletedOrArchived()
		require.NoError(t, err)
		assert.True(t, deleted, "earliest segment (resume point) is deleted, so re-streaming from the beginning is impossible")
	})

	t.Run("earliest segment archived returns true", func(t *testing.T) {
		mdb := newTestMetaDB(t)
		insertQueueSegment(t, mdb, 1, testSourceDBExporterRole, 1, 0)
		insertQueueSegment(t, mdb, 2, testSourceDBExporterRole, 0, 0)

		deletedOrArchived, err := mdb.AnySegmentsDeletedOrArchived("")
		require.NoError(t, err)
		assert.True(t, deletedOrArchived, "earliest segment (resume point) is archived, so re-streaming from the beginning is impossible")
	})

}
