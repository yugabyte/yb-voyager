package metadb

import (
	"database/sql"
	"fmt"
	"os"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

type ColumnProperties struct {
	Type       string  // Data type (e.g., INTEGER, TEXT)
	PrimaryKey int     // Whether it's a primary key values can be 0,1,2,3. If 1 then it is primary key. 2,3 etc. are used for composite primary key
	NotNull    bool    // Whether the column has a NOT NULL constraint
	Default    *string // Default value, if any (nil means no default)
}

// Helper function to check table structure
func checkTableStructure(db *sql.DB, tableName string, expectedColumns map[string]ColumnProperties) error {
	// Query to get table info
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s);", tableName))
	if err != nil {
		return fmt.Errorf("failed to get table info for %s: %w", tableName, err)
	}
	defer rows.Close()

	// Check if columns match expected ones
	actualColumns := make(map[string]ColumnProperties)
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt_value sql.NullString // Default value can be NULL
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt_value, &pk); err != nil {
			return err
		}
		actualColumns[name] = ColumnProperties{
			Type:       ctype,
			PrimaryKey: pk,
			NotNull:    notnull == 1,
			Default: func() *string { // Inline function to set the Default field conditionally
				if dflt_value.Valid {
					return &dflt_value.String
				}
				return nil
			}(),
		}
	}

	// Compare actual columns with expected columns
	for colName, expectedProps := range expectedColumns {
		actualProps, exists := actualColumns[colName]
		if !exists {
			return fmt.Errorf("table %s missing expected column: %s. There is some breaking change!", tableName, colName)
		}

		// Check type
		if actualProps.Type != expectedProps.Type {
			return fmt.Errorf("table %s column %s: expected type %s, got %s. There is some breaking change!", tableName, colName, expectedProps.Type, actualProps.Type)
		}

		// Check if it's part of the primary key
		if actualProps.PrimaryKey != expectedProps.PrimaryKey {
			return fmt.Errorf("table %s column %s: expected primary key to be %v, got %v. There is some breaking change!", tableName, colName, expectedProps.PrimaryKey, actualProps.PrimaryKey)
		}

		// Check NOT NULL constraint
		if actualProps.NotNull != expectedProps.NotNull {
			return fmt.Errorf("table %s column %s: expected NOT NULL to be %v, got %v. There is some breaking change!", tableName, colName, expectedProps.NotNull, actualProps.NotNull)
		}

		// Check default value
		if (expectedProps.Default == nil && actualProps.Default != nil) || (expectedProps.Default != nil && (actualProps.Default == nil || *expectedProps.Default != *actualProps.Default)) {
			return fmt.Errorf("table %s column %s: expected default value %v, got %v. There is some breaking change!", tableName, colName, expectedProps.Default, actualProps.Default)
		}
	}

	// Check for any additional unexpected columns
	for colName := range actualColumns {
		if _, exists := expectedColumns[colName]; !exists {
			return fmt.Errorf("table %s has unexpected additional column: %s. There is some breaking change!", tableName, colName)
		}
	}

	return nil
}

// Helper function to check table existence and no extra tables
func checkTableExistence(t *testing.T, db *sql.DB, expectedTables map[string]map[string]ColumnProperties) error {
	// Query to get table names
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table';")
	if err != nil {
		return fmt.Errorf("failed to get table names: %w", err)
	}
	defer rows.Close()

	// Check if tables match expected ones
	actualTables := make(map[string]struct{})
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return err
		}
		actualTables[tableName] = struct{}{}
	}

	// Compare actual tables with expected tables
	for tableName := range expectedTables {
		if _, exists := actualTables[tableName]; !exists {
			return fmt.Errorf("expected table %s not found. There is some breaking change!", tableName)
		} else {
			t.Logf("Found table: %s", tableName)
		}
	}

	// Check for any additional unexpected tables
	for tableName := range actualTables {
		if _, exists := expectedTables[tableName]; !exists {
			return fmt.Errorf("unexpected additional table: %s. There is some breaking change!", tableName)
		}
	}

	return nil
}

// Test the initMetaDB function
func TestInitMetaDB(t *testing.T) {
	// Define the expected columns and their types for each table
	expectedTables := map[string]map[string]ColumnProperties{
		QUEUE_SEGMENT_META_TABLE_NAME: {
			"segment_no":                             {Type: "INTEGER", PrimaryKey: 1},
			"file_path":                              {Type: "TEXT"},
			"size_committed":                         {Type: "INTEGER"},
			"total_events":                           {Type: "INTEGER"},
			"exporter_role":                          {Type: "TEXT"},
			"imported_by_target_db_importer":         {Type: "INTEGER", Default: stringPointer("0")},
			"imported_by_source_replica_db_importer": {Type: "INTEGER", Default: stringPointer("0")},
			"imported_by_source_db_importer":         {Type: "INTEGER", Default: stringPointer("0")},
			"archived":                               {Type: "INTEGER", Default: stringPointer("0")},
			"deleted":                                {Type: "INTEGER", Default: stringPointer("0")},
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
	tempFile, err := os.CreateTemp(".", "test_meta_db_*.db")
	if err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}
	defer os.Remove(tempFile.Name()) // Ensure the file is removed after the test

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
		err := checkTableExistence(t, db, expectedTables)
		if err != nil {
			t.Errorf("Table existence mismatch: %v", err)
		}
	})

	// Verify the structure of each table
	for table, expectedColumns := range expectedTables {
		t.Run(fmt.Sprintf("Check structure of %s table", table), func(t *testing.T) {
			err := checkTableStructure(db, table, expectedColumns)
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
