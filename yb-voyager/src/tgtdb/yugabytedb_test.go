package tgtdb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/testutils"
	"github.com/yugabyte/yb-voyager/yb-voyager/testcontainers"
)

func TestCreateVoyagerSchemaYB(t *testing.T) {
	ctx := context.Background()

	// Start a YugabyteDB container
	ybContainer, err := testcontainers.StartDBContainer(ctx, testcontainers.YUGABYTEDB)
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
	err = testcontainers.WaitForPGYBDBConnection(db)
	assert.NoError(t, err)

	// Initialize the TargetYugabyteDB instance
	yb := &TargetYugabyteDB{
		db: db,
	}

	// Call CreateVoyagerSchema
	err = yb.CreateVoyagerSchema()
	assert.NoError(t, err, "CreateVoyagerSchema failed")

	expectedTables := map[string]map[string]testutils.ColumnPropertiesPG{
		BATCH_METADATA_TABLE_NAME: {
			"migration_uuid": {DataType: "uuid", IsNullable: "NO", Default: nil, IsPrimary: true},
			"data_file_name": {DataType: "character varying", IsNullable: "NO", Default: nil, IsPrimary: true},
			"batch_number":   {DataType: "integer", IsNullable: "NO", Default: nil, IsPrimary: true},
			"schema_name":    {DataType: "character varying", IsNullable: "NO", Default: nil, IsPrimary: true},
			"table_name":     {DataType: "character varying", IsNullable: "NO", Default: nil, IsPrimary: true},
			"rows_imported":  {DataType: "bigint", IsNullable: "YES", Default: nil, IsPrimary: false},
		},
		EVENT_CHANNELS_METADATA_TABLE_NAME: {
			"migration_uuid":   {DataType: "uuid", IsNullable: "NO", Default: nil, IsPrimary: true},
			"channel_no":       {DataType: "integer", IsNullable: "NO", Default: nil, IsPrimary: true},
			"last_applied_vsn": {DataType: "bigint", IsNullable: "YES", Default: nil, IsPrimary: false},
			"num_inserts":      {DataType: "bigint", IsNullable: "YES", Default: nil, IsPrimary: false},
			"num_deletes":      {DataType: "bigint", IsNullable: "YES", Default: nil, IsPrimary: false},
			"num_updates":      {DataType: "bigint", IsNullable: "YES", Default: nil, IsPrimary: false},
		},
		EVENTS_PER_TABLE_METADATA_TABLE_NAME: {
			"migration_uuid": {DataType: "uuid", IsNullable: "NO", Default: nil, IsPrimary: true},
			"table_name":     {DataType: "character varying", IsNullable: "NO", Default: nil, IsPrimary: true},
			"channel_no":     {DataType: "integer", IsNullable: "NO", Default: nil, IsPrimary: true},
			"total_events":   {DataType: "bigint", IsNullable: "YES", Default: nil, IsPrimary: false},
			"num_inserts":    {DataType: "bigint", IsNullable: "YES", Default: nil, IsPrimary: false},
			"num_deletes":    {DataType: "bigint", IsNullable: "YES", Default: nil, IsPrimary: false},
			"num_updates":    {DataType: "bigint", IsNullable: "YES", Default: nil, IsPrimary: false},
		},
	}

	// Validate the schema and tables
	t.Run("Check all the expected tables and no extra tables", func(t *testing.T) {
		testutils.CheckTableExistencePG(t, db, BATCH_METADATA_TABLE_SCHEMA, expectedTables)
	})

	// Validate columns for each table
	for tableName, expectedColumns := range expectedTables {
		t.Run(fmt.Sprintf("Check columns for %s table", tableName), func(t *testing.T) {
			table := strings.Split(tableName, ".")[1]
			testutils.CheckTableStructurePG(t, db, BATCH_METADATA_TABLE_SCHEMA, table, expectedColumns)
		})
	}
}
