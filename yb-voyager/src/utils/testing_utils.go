package utils

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type ColumnPropertiesSqlite struct {
	Type       string  // Data type (e.g., INTEGER, TEXT)
	PrimaryKey int     // Whether it's a primary key values can be 0,1,2,3. If 1 then it is primary key. 2,3 etc. are used for composite primary key
	NotNull    bool    // Whether the column has a NOT NULL constraint
	Default    *string // Default value, if any (nil means no default)
}

// Column represents a column's expected metadata
type ColumnPropertiesPG struct {
	Name       string
	DataType   string
	IsNullable string
	Default    interface{}
	IsPrimary  bool
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
func CheckTableStructureSqlite(db *sql.DB, tableName string, expectedColumns map[string]ColumnPropertiesSqlite) error {
	// Query to get table info
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s);", tableName))
	if err != nil {
		return fmt.Errorf("failed to get table info for %s: %w", tableName, err)
	}
	defer rows.Close()

	// Check if columns match expected ones
	actualColumns := make(map[string]ColumnPropertiesSqlite)
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt_value sql.NullString // Default value can be NULL
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt_value, &pk); err != nil {
			return err
		}
		actualColumns[name] = ColumnPropertiesSqlite{
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

// Helper function to check table structure
func CheckTableStructurePG(t *testing.T, db *sql.DB, schema, table string, expectedColumns []ColumnPropertiesPG) {
	queryColumns := `
		SELECT column_name, data_type, is_nullable, column_default
		FROM information_schema.columns
		WHERE table_name = $1;`

	rows, err := db.Query(queryColumns, table)
	if err != nil {
		t.Fatalf("Failed to query columns for table %s.%s: %v", schema, table, err)
	}
	defer rows.Close()

	actualColumns := make(map[string]ColumnPropertiesPG)
	for rows.Next() {
		var col ColumnPropertiesPG
		err := rows.Scan(&col.Name, &col.DataType, &col.IsNullable, &col.Default)
		if err != nil {
			t.Fatalf("Failed to scan column metadata: %v", err)
		}
		actualColumns[col.Name] = col
	}

	// Compare columns
	for _, expected := range expectedColumns {
		actual, found := actualColumns[expected.Name]
		if !found {
			t.Errorf("Missing expected column in table %s.%s: %s.\nThere is some breaking change!", schema, table, expected.Name)
			continue
		}
		if actual.DataType != expected.DataType || actual.IsNullable != expected.IsNullable {
			t.Errorf("Column mismatch in table %s.%s: \nexpected %+v, \ngot %+v.\nThere is some breaking change!", schema, table, expected, actual)
		}
	}

	// Check for extra columns
	for actualName := range actualColumns {
		found := false
		for _, expected := range expectedColumns {
			if actualName == expected.Name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Unexpected column in table %s.%s: %s.\nThere is some breaking change!", schema, table, actualName)
		}
	}

	// Check primary keys
	checkPrimaryKeyOfTablePG(t, db, schema, table, expectedColumns)
}

func checkPrimaryKeyOfTablePG(t *testing.T, db *sql.DB, schema, table string, expectedColumns []ColumnPropertiesPG) {
	// Validate primary keys
	queryPrimaryKeys := `
    SELECT conrelid::regclass AS table_name, 
           conname AS primary_key, 
           pg_get_constraintdef(oid)
    FROM   pg_constraint 
    WHERE  contype = 'p'  -- 'p' indicates primary key
    AND    conrelid::regclass::text = $1  -- Use parameterized schema and table name
    ORDER  BY conrelid::regclass::text, contype DESC;`

	rows, err := db.Query(queryPrimaryKeys, fmt.Sprintf("%s.%s", schema, table))
	if err != nil {
		t.Fatalf("Failed to query primary keys for table %s.%s: %v", schema, table, err)
	}
	defer rows.Close()

	// Map to store primary key columns (not just the constraint name)
	// Output is like:
	// table_name                                                   | primary_key                                    | primary_key_definition
	// ybvoyager_metadata.ybvoyager_import_data_batches_metainfo_v3 | ybvoyager_import_data_batches_metainfo_v3_pkey | PRIMARY KEY (migration_uuid, data_file_name, batch_number, schema_name, table_name)
	primaryKeyColumns := map[string]bool{}
	for rows.Next() {
		var tableName, pk, constraintDef string
		if err := rows.Scan(&tableName, &pk, &constraintDef); err != nil {
			t.Fatalf("Failed to scan primary key: %v", err)
		}

		// Parse the columns from the constraint definition (e.g., "PRIMARY KEY (col1, col2, ...)")
		columns := parsePrimaryKeyColumnsPG(constraintDef)
		for _, col := range columns {
			primaryKeyColumns[col] = true
		}
	}

	// Check if the primary key columns match the expected primary key columns
	for _, expected := range expectedColumns {
		if expected.IsPrimary {
			if _, found := primaryKeyColumns[expected.Name]; !found {
				t.Errorf("Missing expected primary key column in table %s.%s: %s.\nThere is some breaking change!", schema, table, expected.Name)
			}
		}
	}

	// Check if there are any extra primary key columns
	for col := range primaryKeyColumns {
		found := false
		for _, expected := range expectedColumns {
			if expected.Name == col && expected.IsPrimary {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Unexpected primary key column in table %s.%s: %s.\nThere is some breaking change!", schema, table, col)
		}
	}
}

// Helper function to parse primary key columns from the constraint definition
func parsePrimaryKeyColumnsPG(constraintDef string) []string {
	// Remove "PRIMARY KEY (" and ")"
	constraintDef = strings.TrimPrefix(constraintDef, "PRIMARY KEY (")
	constraintDef = strings.TrimSuffix(constraintDef, ")")

	// Split by commas to get column names
	columns := strings.Split(constraintDef, ",")
	for i := range columns {
		columns[i] = strings.TrimSpace(columns[i]) // Remove extra spaces around column names
	}

	return columns
}

// Helper function to check table existence and no extra tables
func CheckTableExistenceSqlite(t *testing.T, db *sql.DB, expectedTables map[string]map[string]ColumnPropertiesSqlite) error {
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

// validateSchema validates the schema, tables, and columns
func CheckTableExistencePG(t *testing.T, db *sql.DB, schema string, expectedTables map[string][]ColumnPropertiesPG) {
	// Check all tables in the schema
	queryTables := `SELECT table_schema || '.' || table_name AS qualified_table_name
	FROM information_schema.tables
	WHERE table_schema = $1;`
	rows, err := db.Query(queryTables, schema)
	if err != nil {
		t.Fatalf("Failed to query tables in schema %s: %v", schema, err)
	}
	defer rows.Close()

	actualTables := make(map[string]bool)
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			t.Fatalf("Failed to scan table name: %v", err)
		}
		actualTables[tableName] = true
	}

	// Compare tables
	for expectedTable := range expectedTables {
		if !actualTables[expectedTable] {
			t.Errorf("Missing expected table: %s", expectedTable)
		}
	}

	// Check for extra tables
	for actualTable := range actualTables {
		if _, found := expectedTables[actualTable]; !found {
			t.Errorf("Unexpected table found: %s", actualTable)
		}
	}
}

func StartPostgresContainer(ctx context.Context) (testcontainers.Container, error) {
	// Create a PostgreSQL TestContainer
	req := testcontainers.ContainerRequest{
		Image:        "postgres:latest", // Use the latest PostgreSQL image
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "testuser",     // Set PostgreSQL username
			"POSTGRES_PASSWORD": "testpassword", // Set PostgreSQL password
			"POSTGRES_DB":       "testdb",       // Set PostgreSQL database name
		},
		WaitingFor: wait.ForListeningPort("5432/tcp").WithStartupTimeout(30 * 1e9), // Wait for PostgreSQL to be ready
	}

	// Start the container
	return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
}

func StartYugabyteDBContainer(ctx context.Context) (testcontainers.Container, error) {
	// Create a YugabyteDB TestContainer
	req := testcontainers.ContainerRequest{
		Image:        "yugabytedb/yugabyte:latest",
		ExposedPorts: []string{"5433/tcp"},
		WaitingFor:   wait.ForListeningPort("5433/tcp"),
		Cmd:          []string{"bin/yugabyted", "start", "--daemon=false"},
	}

	// Start the container
	return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
}

// waitForDBConnection waits until the database is ready for connections.
func WaitForPGYBDBConnection(db *sql.DB) error {
	for i := 0; i < 12; i++ {
		if err := db.Ping(); err == nil {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("database did not become ready in time")
}
