package utils

import (
	"database/sql"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
)

type ColumnProperties struct {
	Type       string  // Data type (e.g., INTEGER, TEXT)
	PrimaryKey int     // Whether it's a primary key values can be 0,1,2,3. If 1 then it is primary key. 2,3 etc. are used for composite primary key
	NotNull    bool    // Whether the column has a NOT NULL constraint
	Default    *string // Default value, if any (nil means no default)
}

// CompareStructs compares two struct types and reports any mismatches.
func CompareStructs(t *testing.T, actual, expected reflect.Type, structName string) {
	if actual.Kind() != reflect.Struct || expected.Kind() != reflect.Struct {
		t.Fatalf("Both %s and expected type must be structs. There is some breaking change!", structName)
	}

	if actual.NumField() != expected.NumField() {
		t.Errorf("%s: Number of fields mismatch. Got %d, expected %d. There is some breaking change!", structName, actual.NumField(), expected.NumField())
	}

	for i := 0; i < max(actual.NumField(), expected.NumField()); i++ {
		var actualField, expectedField reflect.StructField
		var actualExists, expectedExists bool

		if i < actual.NumField() {
			actualField = actual.Field(i)
			actualExists = true
		}
		if i < expected.NumField() {
			expectedField = expected.Field(i)
			expectedExists = true
		}

		// Compare field names
		if actualExists && expectedExists && actualField.Name != expectedField.Name {
			t.Errorf("%s: Field name mismatch at position %d. Got %s, expected %s. There is some breaking change!", structName, i, actualField.Name, expectedField.Name)
		}

		// Compare field types
		if actualExists && expectedExists && actualField.Type != expectedField.Type {
			t.Errorf("%s: Field type mismatch for %s. Got %s, expected %s. There is some breaking change!", structName, actualField.Name, actualField.Type, expectedField.Type)
		}

		// Compare tags
		if actualExists && expectedExists && actualField.Tag != expectedField.Tag {
			t.Errorf("%s: Field tag mismatch for %s. Got %s, expected %s. There is some breaking change!", structName, actualField.Name, actualField.Tag, expectedField.Tag)
		}

		// Report missing fields
		if !actualExists && expectedExists {
			t.Errorf("%s: Missing field %s of type %s. There is some breaking change!", structName, expectedField.Name, expectedField.Type)
		}
		if actualExists && !expectedExists {
			t.Errorf("%s: Unexpected field %s of type %s. There is some breaking change!", structName, actualField.Name, actualField.Type)
		}
	}
}

// CompareJson compares two structs by marshalling them into JSON and reports any differences.
func CompareJson(t *testing.T, outputFilePath string, expectedJSON string, exportDir string) {
	// Read the output JSON file
	outputBytes, err := os.ReadFile(outputFilePath)
	if err != nil {
		t.Fatalf("Failed to read output JSON file: %v", err)
	}

	// Compare the output JSON with the expected JSON
	if diff := cmp.Diff(expectedJSON, string(outputBytes)); diff != "" {
		t.Errorf("JSON file mismatch (-expected +actual):\n%s", diff)
	}

	// Remove the test directory
	if err := os.RemoveAll(exportDir); err != nil {
		t.Logf("Failed to remove test export directory: %v", err)
	}
}

// Helper function to check table structure
func CheckTableStructure(db *sql.DB, tableName string, expectedColumns map[string]ColumnProperties) error {
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
func CheckTableExistence(t *testing.T, db *sql.DB, expectedTables map[string]map[string]ColumnProperties) error {
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
