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
package migassessment

import (
	"database/sql"
	"fmt"
	"os"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

func TestInitAssessmentDB(t *testing.T) {
	expectedTables := map[string]map[string]testutils.ColumnPropertiesSqlite{
		TABLE_INDEX_IOPS: {
			"schema_name":      {Type: "TEXT", PrimaryKey: 1},
			"object_name":      {Type: "TEXT", PrimaryKey: 2},
			"object_type":      {Type: "TEXT"},
			"seq_reads":        {Type: "INTEGER"},
			"row_writes":       {Type: "INTEGER"},
			"measurement_type": {Type: "TEXT", PrimaryKey: 3},
		},
		TABLE_INDEX_SIZES: {
			"schema_name":   {Type: "TEXT", PrimaryKey: 1},
			"object_name":   {Type: "TEXT", PrimaryKey: 2},
			"object_type":   {Type: "TEXT"},
			"size_in_bytes": {Type: "INTEGER"},
		},
		TABLE_ROW_COUNTS: {
			"schema_name": {Type: "TEXT", PrimaryKey: 1},
			"table_name":  {Type: "TEXT", PrimaryKey: 2},
			"row_count":   {Type: "INTEGER"},
		},
		TABLE_COLUMNS_COUNT: {
			"schema_name":  {Type: "TEXT", PrimaryKey: 1},
			"object_name":  {Type: "TEXT", PrimaryKey: 2},
			"object_type":  {Type: "TEXT"},
			"column_count": {Type: "INTEGER"},
		},
		INDEX_TO_TABLE_MAPPING: {
			"index_schema": {Type: "TEXT", PrimaryKey: 1},
			"index_name":   {Type: "TEXT", PrimaryKey: 2},
			"table_schema": {Type: "TEXT"},
			"table_name":   {Type: "TEXT"},
		},
		OBJECT_TYPE_MAPPING: {
			"schema_name": {Type: "TEXT", PrimaryKey: 1},
			"object_name": {Type: "TEXT", PrimaryKey: 2},
			"object_type": {Type: "TEXT"},
		},
		TABLE_COLUMNS_DATA_TYPES: {
			"schema_name": {Type: "TEXT", PrimaryKey: 1},
			"table_name":  {Type: "TEXT", PrimaryKey: 2},
			"column_name": {Type: "TEXT", PrimaryKey: 3},
			"data_type":   {Type: "TEXT"},
		},
		TABLE_INDEX_STATS: {
			"schema_name":       {Type: "TEXT", PrimaryKey: 1},
			"object_name":       {Type: "TEXT", PrimaryKey: 2},
			"row_count":         {Type: "INTEGER"},
			"column_count":      {Type: "INTEGER"},
			"reads_per_second":  {Type: "INTEGER"},
			"writes_per_second": {Type: "INTEGER"},
			"is_index":          {Type: "BOOLEAN"},
			"object_type":       {Type: "TEXT"},
			"parent_table_name": {Type: "TEXT"},
			"size_in_bytes":     {Type: "INTEGER"},
		},
		DB_QUERIES_SUMMARY: {
			"queryid": {Type: "BIGINT"},
			"query":   {Type: "TEXT"},
			"calls":   {Type: "BIGINT"},
			"rows":    {Type: "BIGINT"},
			"total_exec_time": {Type: "REAL"},
			"mean_exec_time": {Type: "REAL"},
			"min_exec_time": {Type: "REAL"},
			"max_exec_time": {Type: "REAL"},
			"stddev_exec_time": {Type: "REAL"},
		},
		REDUNDANT_INDEXES: {
			"redundant_schema_name": {Type: "TEXT", PrimaryKey: 1},
			"redundant_table_name":  {Type: "TEXT", PrimaryKey: 2},
			"redundant_index_name":  {Type: "TEXT", PrimaryKey: 3},
			"existing_schema_name":  {Type: "TEXT"},
			"existing_table_name":   {Type: "TEXT"},
			"existing_index_name":   {Type: "TEXT"},
			"redundant_ddl":         {Type: "TEXT"},
			"existing_ddl":          {Type: "TEXT"},
		},
		COLUMN_STATISTICS: {
			"schema_name":          {Type: "TEXT", PrimaryKey: 1},
			"table_name":           {Type: "TEXT", PrimaryKey: 2},
			"column_name":          {Type: "TEXT", PrimaryKey: 3},
			"null_frac":            {Type: "REAL"},
			"effective_n_distinct": {Type: "INTEGER"},
			"most_common_freq":     {Type: "REAL"},
			"most_common_val":      {Type: "TEXT"},
		},
	}

	// Create a temporary SQLite database file for testing
	tempFile, err := os.CreateTemp(os.TempDir(), "test_assessment_db_*.db")
	if err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}
	// Ensure the file is removed after the test
	defer func() {
		err := os.Remove(tempFile.Name())
		if err != nil {
			t.Fatalf("Failed to remove temporary file: %v", err)
		}
	}()

	GetSourceMetadataDBFilePath = func() string {
		return tempFile.Name()
	}

	err = InitAssessmentDB()
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

// Helper function to create a string pointer
func stringPointer(s string) *string {
	return &s
}
