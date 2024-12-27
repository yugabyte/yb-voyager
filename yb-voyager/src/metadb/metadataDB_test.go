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
	"testing"

	_ "github.com/mattn/go-sqlite3"
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
