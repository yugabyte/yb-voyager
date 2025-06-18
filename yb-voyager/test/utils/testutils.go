package testutils

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

type ColumnPropertiesSqlite struct {
	Type       string         // Data type (e.g., INTEGER, TEXT)
	PrimaryKey int            // Whether it's a primary key values can be 0,1,2,3. If 1 then it is primary key. 2,3 etc. are used for composite primary key
	NotNull    bool           // Whether the column has a NOT NULL constraint
	Default    sql.NullString // Default value, if any (nil means no default)
}

// Column represents a column's expected metadata
type ColumnPropertiesPG struct {
	Type       string
	IsPrimary  bool
	IsNullable string
	Default    sql.NullString
}

// CompareStructs compares two struct types and reports any mismatches.
func CompareStructs(t *testing.T, actual, expected reflect.Type, structName string) {
	assert.Equal(t, reflect.Struct, actual.Kind(), "%s: Actual type must be a struct. There is some breaking change!", structName)
	assert.Equal(t, reflect.Struct, expected.Kind(), "%s: Expected type must be a struct. There is some breaking change!", structName)

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

		// Assert field names match
		if actualExists && expectedExists {
			assert.Equal(t, expectedField.Name, actualField.Name, "%s: Field name mismatch at position %d. There is some breaking change!", structName, i)
			assert.Equal(t, expectedField.Type.String(), actualField.Type.String(), "%s: Field type mismatch for field %s. There is some breaking change!", structName, expectedField.Name)
			assert.Equal(t, expectedField.Tag, actualField.Tag, "%s: Field tag mismatch for field %s. There is some breaking change!", structName, expectedField.Name)
		}

		// Report missing or extra fields
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

	// Can be used if we don't want to compare pretty printed JSON
	// assert.JSONEqf(t, expectedJSON, string(outputBytes), "JSON file mismatch. There is some breaking change!")
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
		var name string
		var cp ColumnPropertiesSqlite
		if err := rows.Scan(&cid, &name, &cp.Type, &cp.NotNull, &cp.Default, &cp.PrimaryKey); err != nil {
			return err
		}
		actualColumns[name] = ColumnPropertiesSqlite{
			Type:       cp.Type,
			PrimaryKey: cp.PrimaryKey,
			NotNull:    cp.NotNull,
			Default:    cp.Default,
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
		if (expectedProps.Default.Valid && !actualProps.Default.Valid) || (!expectedProps.Default.Valid && actualProps.Default.Valid) || (expectedProps.Default.Valid && actualProps.Default.Valid && expectedProps.Default.String != actualProps.Default.String) {
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
func CheckTableStructurePG(t *testing.T, db *sql.DB, schema, table string, expectedColumns map[string]ColumnPropertiesPG) {
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
		var colName string
		var col ColumnPropertiesPG
		err := rows.Scan(&colName, &col.Type, &col.IsNullable, &col.Default)
		if err != nil {
			t.Fatalf("Failed to scan column metadata: %v", err)
		}
		actualColumns[colName] = col
	}

	// Compare columns
	for colName, expectedProps := range expectedColumns {
		actual, found := actualColumns[colName]
		if !found {
			t.Errorf("Missing expected column in table %s.%s: %s.\nThere is some breaking change!", schema, table, colName)
			continue
		}
		if actual.Type != expectedProps.Type || actual.IsNullable != expectedProps.IsNullable || actual.Default != expectedProps.Default {
			t.Errorf("Column mismatch in table %s.%s: \nexpected %+v, \ngot %+v.\nThere is some breaking change!", schema, table, expectedProps, actual)
		}
	}

	// Check for extra columns
	for actualName := range actualColumns {
		found := false
		for expectedName, _ := range expectedColumns {
			if actualName == expectedName {
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

func checkPrimaryKeyOfTablePG(t *testing.T, db *sql.DB, schema, table string, expectedColumns map[string]ColumnPropertiesPG) {
	// Validate primary keys
	queryPrimaryKeys := `
    SELECT conrelid::regclass AS table_name,
        	conname AS primary_key,
           pg_get_constraintdef(oid)
    FROM   pg_constraint
    WHERE  contype = 'p'  -- 'p' indicates primary key
    AND    conrelid::regclass::text = $1
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
	for expectedName, expectedParams := range expectedColumns {
		if expectedParams.IsPrimary {
			if _, found := primaryKeyColumns[expectedName]; !found {
				t.Errorf("Missing expected primary key column in table %s.%s: %s.\nThere is some breaking change!", schema, table, expectedName)
			}
		}
	}

	// Check if there are any extra primary key columns
	for col := range primaryKeyColumns {
		found := false
		for expectedName, expectedParams := range expectedColumns {
			if expectedName == col && expectedParams.IsPrimary {
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
	// Define the regex pattern
	re := regexp.MustCompile(`PRIMARY KEY\s*\((.*?)\)`)

	// Extract the column list inside "PRIMARY KEY(...)"
	matches := re.FindStringSubmatch(constraintDef)
	if len(matches) < 2 {
		return nil // Return nil if no match is found
	}

	// Split by commas to get individual column names
	columns := strings.Split(matches[1], ",")
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
func CheckTableExistencePG(t *testing.T, db *sql.DB, schema string, expectedTables map[string]map[string]ColumnPropertiesPG) {
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

// === assertion helper functions
func AssertEqualStringSlices(t *testing.T, expected, actual []string) {
	t.Helper()
	if len(expected) != len(actual) {
		t.Errorf("Mismatch in slice length. Expected: %v, Actual: %v", expected, actual)
	}

	sort.Strings(expected)
	sort.Strings(actual)
	assert.Equal(t, expected, actual)
}

func AssertEqualSourceNameSlices(t *testing.T, expected, actual []*sqlname.SourceName) {
	SortSourceNames(expected)
	SortSourceNames(actual)
	assert.Equal(t, expected, actual)
}

func SortSourceNames(tables []*sqlname.SourceName) {
	sort.Slice(tables, func(i, j int) bool {
		return tables[i].Qualified.MinQuoted < tables[j].Qualified.MinQuoted
	})
}

func AssertEqualNameTuplesSlice(t *testing.T, expected, actual []sqlname.NameTuple) {
	sortNameTuples(expected)
	sortNameTuples(actual)
	assert.Equal(t, expected, actual)
}

func sortNameTuples(tables []sqlname.NameTuple) {
	sort.Slice(tables, func(i, j int) bool {
		return tables[i].ForOutput() < tables[j].ForOutput()
	})
}

// waitForDBConnection waits until the database is ready for connections.
func WaitForDBToBeReady(db *sql.DB) error {
	for i := 0; i < 12; i++ {
		if err := db.Ping(); err == nil {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("database did not become ready in time")
}

// context is OPTIONAL functional arg for caller but should be appended in error message
func FatalIfError(t *testing.T, err error, context ...string) {
	context = lo.Ternary(len(context) > 1, context, []string{"error"})
	if err != nil {
		t.Fatalf("%s: %v", context[0], err)
	}
}

func CreateTempFile(dir string, fileContents string, fileFormat string) (string, error) {
	// Create a temporary file
	file, err := os.CreateTemp(dir, fmt.Sprintf("temp-*.%s", fileFormat))
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Write some text to the file
	_, err = file.WriteString(fileContents)
	if err != nil {
		return "", err
	}

	return file.Name(), nil
}

func CreateNameTupleWithSourceName(s string, defaultSchema string, dbType string) sqlname.NameTuple {
	return sqlname.NameTuple{
		SourceName:  sqlname.NewObjectNameWithQualifiedName(dbType, defaultSchema, s),
		CurrentName: sqlname.NewObjectNameWithQualifiedName(dbType, defaultSchema, s),
	}
}

// compareTableData queries the specified table from both srcDB and tgtDB, ordered by a given key (e.g. “id”),
// and then compares every row for an exact match.
// It returns an error if there’s any difference.
func CompareTableData(ctx context.Context, srcDB *sql.DB, tgtDB *sql.DB, tableName, orderByClause string) error {
	// Compare row counts first
	err := CompareRowCount(ctx, srcDB, tgtDB, tableName)
	if err != nil {
		return err
	}

	// Construct the query with an ORDER BY clause for deterministic ordering.
	query := fmt.Sprintf("SELECT * FROM %s ORDER BY %s", tableName, orderByClause)

	srcRows, err := srcDB.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("querying source table: %w", err)
	}
	defer srcRows.Close()

	tgtRows, err := tgtDB.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("querying target table: %w", err)
	}
	defer tgtRows.Close()

	// Get the column names from the source.
	srcCols, err := srcRows.Columns()
	if err != nil {
		return fmt.Errorf("getting source columns: %w", err)
	}
	tgtCols, err := tgtRows.Columns()
	if err != nil {
		return fmt.Errorf("getting target columns: %w", err)
	}
	if !reflect.DeepEqual(srcCols, tgtCols) {
		return fmt.Errorf("column mismatch: source %v vs target %v", srcCols, tgtCols)
	}
	colsCount := len(srcCols)

	// Compare rows from each result set.
	rowNum := 0
	for srcRows.Next() {
		rowNum++
		if !tgtRows.Next() {
			return fmt.Errorf("target has fewer rows than source; mismatch found at row %d", rowNum)
		}
		srcRow, err := scanRow(srcRows, colsCount)
		if err != nil {
			return fmt.Errorf("scanning source row %d: %w", rowNum, err)
		}
		tgtRow, err := scanRow(tgtRows, colsCount)
		if err != nil {
			return fmt.Errorf("scanning target row %d: %w", rowNum, err)
		}
		if !reflect.DeepEqual(srcRow, tgtRow) {
			return fmt.Errorf("row %d mismatch: source %v vs target %v", rowNum, srcRow, tgtRow)
		}
	}
	// If target has extra rows, that's a mismatch.
	if tgtRows.Next() {
		return fmt.Errorf("target has more rows than source; extra rows after row %d", rowNum)
	}
	return nil
}

// CompareRowCount compares the row count of a table in two databases.
func CompareRowCount(ctx context.Context, srcDB *sql.DB, tgtDB *sql.DB, tableName string) error {
	srcCountQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)
	tgtCountQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)
	srcCountRow := srcDB.QueryRowContext(ctx, srcCountQuery)
	tgtCountRow := tgtDB.QueryRowContext(ctx, tgtCountQuery)
	var srcCount, tgtCount int
	if err := srcCountRow.Scan(&srcCount); err != nil {
		return fmt.Errorf("counting rows in source table %s: %w", tableName, err)
	}
	if err := tgtCountRow.Scan(&tgtCount); err != nil {
		return fmt.Errorf("counting rows in target table %s: %w", tableName, err)
	}
	if srcCount != tgtCount {
		return fmt.Errorf("row count mismatch for table %s: source has %d rows, target has %d rows", tableName, srcCount, tgtCount)
	}
	return nil
}

// scanRow reads the current row from rows, returning a slice of interface{} for each column.
func scanRow(rows *sql.Rows, colCount int) ([]interface{}, error) {
	values := make([]interface{}, colCount)   // holds the values for each column
	scanArgs := make([]interface{}, colCount) // holds the pointers to the values
	for i := range values {
		scanArgs[i] = &values[i]
	}

	// 'scanArgs' hold pointers to each value (just like &name, &age)
	if err := rows.Scan(scanArgs...); err != nil {
		return nil, err
	}

	return values, nil
}

func AssertFileContains(t *testing.T, filePath string, expectedContent string) {
	t.Helper()
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file %s: %v", filePath, err)
	}
	assert.Containsf(t, string(content), expectedContent,
		"File %s does not contain expected content: %s", filePath, expectedContent)
}
