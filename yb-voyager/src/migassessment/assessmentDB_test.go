package migassessment

import (
	"database/sql"
	"fmt"
	"os"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/testutils"
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
			"reads":             {Type: "INTEGER"},
			"writes":            {Type: "INTEGER"},
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
