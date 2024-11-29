package tgtdb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Column represents a column's expected metadata
type Column struct {
	Name       string
	DataType   string
	IsNullable string
	Default    interface{}
	IsPrimary  bool
}

func startYugabyteDBContainer(ctx context.Context) (testcontainers.Container, error) {
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

func TestCreateVoyagerSchemaYB(t *testing.T) {
	ctx := context.Background()

	// Start a YugabyteDB container
	ybContainer, err := startYugabyteDBContainer(ctx)
	assert.NoError(t, err, "Failed to start YugabyteDB container")
	defer ybContainer.Terminate(ctx)

	// Get the container's host and port
	host, err := ybContainer.Host(ctx)
	assert.NoError(t, err)
	port, err := ybContainer.MappedPort(ctx, "5433")
	assert.NoError(t, err)

	// Connect to the database
	dsn := fmt.Sprintf("host=%s port=%s user=yugabyte password=yugabyte dbname=yugabyte sslmode=disable", host, port.Port())
	db, err := sql.Open("postgres", dsn)
	assert.NoError(t, err)
	defer db.Close()

	// Wait for the database to be ready
	err = waitForDBConnection(db)
	assert.NoError(t, err)

	// Initialize the TargetYugabyteDB instance
	yb := &TargetYugabyteDB{
		db: db,
	}

	// Call CreateVoyagerSchema
	err = yb.CreateVoyagerSchema()
	assert.NoError(t, err, "CreateVoyagerSchema failed")

	expectedTables := map[string][]Column{
		BATCH_METADATA_TABLE_NAME: {
			{"migration_uuid", "uuid", "NO", nil, true},
			{"data_file_name", "character varying", "NO", nil, true},
			{"batch_number", "integer", "NO", nil, true},
			{"schema_name", "character varying", "NO", nil, true},
			{"table_name", "character varying", "NO", nil, true},
			{"rows_imported", "bigint", "YES", nil, false},
		},
		EVENT_CHANNELS_METADATA_TABLE_NAME: {
			{"migration_uuid", "uuid", "NO", nil, true},
			{"channel_no", "integer", "NO", nil, true},
			{"last_applied_vsn", "bigint", "YES", nil, false},
			{"num_inserts", "bigint", "YES", nil, false},
			{"num_deletes", "bigint", "YES", nil, false},
			{"num_updates", "bigint", "YES", nil, false},
		},
		EVENTS_PER_TABLE_METADATA_TABLE_NAME: {
			{"migration_uuid", "uuid", "NO", nil, true},
			{"table_name", "character varying", "NO", nil, true},
			{"channel_no", "integer", "NO", nil, true},
			{"total_events", "bigint", "YES", nil, false},
			{"num_inserts", "bigint", "YES", nil, false},
			{"num_deletes", "bigint", "YES", nil, false},
			{"num_updates", "bigint", "YES", nil, false},
		},
	}

	// Validate the schema and tables
	t.Run("Check all the expected tables and no extra tables", func(t *testing.T) {
		validateSchema(t, db, BATCH_METADATA_TABLE_SCHEMA, expectedTables)
	})

	// Validate columns for each table
	for tableName, expectedColumns := range expectedTables {
		t.Run(fmt.Sprintf("Check columns for %s table", tableName), func(t *testing.T) {
			table := strings.Split(tableName, ".")[1]
			validateColumns(t, db, BATCH_METADATA_TABLE_SCHEMA, table, expectedColumns)
		})
	}
}

// waitForDBConnection waits until the database is ready for connections.
func waitForDBConnection(db *sql.DB) error {
	for i := 0; i < 12; i++ {
		if err := db.Ping(); err == nil {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("database did not become ready in time")
}

// validateSchema validates the schema, tables, and columns
func validateSchema(t *testing.T, db *sql.DB, schema string, expectedTables map[string][]Column) {
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

// validateColumns validates the columns of a specific table
func validateColumns(t *testing.T, db *sql.DB, schema, table string, expectedColumns []Column) {
	queryColumns := `
		SELECT column_name, data_type, is_nullable, column_default
		FROM information_schema.columns
		WHERE table_name = $1;`

	rows, err := db.Query(queryColumns, table)
	if err != nil {
		t.Fatalf("Failed to query columns for table %s.%s: %v", schema, table, err)
	}
	defer rows.Close()

	actualColumns := make(map[string]Column)
	for rows.Next() {
		var col Column
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
	checkPrimaryKeyOfTable(t, db, schema, table, expectedColumns)
}

func checkPrimaryKeyOfTable(t *testing.T, db *sql.DB, schema, table string, expectedColumns []Column) {
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
		columns := parsePrimaryKeyColumns(constraintDef)
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
func parsePrimaryKeyColumns(constraintDef string) []string {
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
