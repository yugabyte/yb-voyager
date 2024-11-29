package tgtdb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func startPostgresContainer(ctx context.Context) (testcontainers.Container, error) {
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

func TestCreateVoyagerSchemaPG(t *testing.T) {
	ctx := context.Background()

	// Start a YugabyteDB container
	pgContainer, err := startPostgresContainer(ctx)
	assert.NoError(t, err, "Failed to start YugabyteDB container")
	defer pgContainer.Terminate(ctx)

	// Get the container's host and port
	host, err := pgContainer.Host(ctx)
	assert.NoError(t, err)
	port, err := pgContainer.MappedPort(ctx, "5432")
	assert.NoError(t, err)

	// Connect to the database
	dsn := fmt.Sprintf("host=%s port=%s user=testuser password=testpassword dbname=testdb sslmode=disable", host, port.Port())
	db, err := sql.Open("postgres", dsn)
	assert.NoError(t, err)
	defer db.Close()

	// Wait for the database to be ready
	err = waitForDBConnection(db)
	assert.NoError(t, err)

	// Initialize the TargetYugabyteDB instance
	pg := &TargetPostgreSQL{
		db: db,
	}

	// Call CreateVoyagerSchema
	err = pg.CreateVoyagerSchema()
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
