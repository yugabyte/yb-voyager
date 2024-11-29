package tgtdb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

func TestCreateVoyagerSchemaYB(t *testing.T) {
	ctx := context.Background()

	// Start a YugabyteDB container
	ybContainer, err := utils.StartYugabyteDBContainer(ctx)
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
	err = utils.WaitForPGYBDBConnection(db)
	assert.NoError(t, err)

	// Initialize the TargetYugabyteDB instance
	yb := &TargetYugabyteDB{
		db: db,
	}

	// Call CreateVoyagerSchema
	err = yb.CreateVoyagerSchema()
	assert.NoError(t, err, "CreateVoyagerSchema failed")

	expectedTables := map[string][]utils.ColumnPropertiesPGYB{
		BATCH_METADATA_TABLE_NAME: {
			{
				Name:       "migration_uuid",
				DataType:   "uuid",
				IsNullable: "NO",
				Default:    nil,
				IsPrimary:  true,
			},
			{
				Name:       "data_file_name",
				DataType:   "character varying",
				IsNullable: "NO",
				Default:    nil,
				IsPrimary:  true,
			},
			{
				Name:       "batch_number",
				DataType:   "integer",
				IsNullable: "NO",
				Default:    nil,
				IsPrimary:  true,
			},
			{
				Name:       "schema_name",
				DataType:   "character varying",
				IsNullable: "NO",
				Default:    nil,
				IsPrimary:  true,
			},
			{
				Name:       "table_name",
				DataType:   "character varying",
				IsNullable: "NO",
				Default:    nil,
				IsPrimary:  true,
			},
			{
				Name:       "rows_imported",
				DataType:   "bigint",
				IsNullable: "YES",
				Default:    nil,
				IsPrimary:  false,
			},
		},
		EVENT_CHANNELS_METADATA_TABLE_NAME: {
			{
				Name:       "migration_uuid",
				DataType:   "uuid",
				IsNullable: "NO",
				Default:    nil,
				IsPrimary:  true,
			},
			{
				Name:       "channel_no",
				DataType:   "integer",
				IsNullable: "NO",
				Default:    nil,
				IsPrimary:  true,
			},
			{
				Name:       "last_applied_vsn",
				DataType:   "bigint",
				IsNullable: "YES",
				Default:    nil,
				IsPrimary:  false,
			},
			{
				Name:       "num_inserts",
				DataType:   "bigint",
				IsNullable: "YES",
				Default:    nil,
				IsPrimary:  false,
			},
			{
				Name:       "num_deletes",
				DataType:   "bigint",
				IsNullable: "YES",
				Default:    nil,
				IsPrimary:  false,
			},
			{
				Name:       "num_updates",
				DataType:   "bigint",
				IsNullable: "YES",
				Default:    nil,
				IsPrimary:  false,
			},
		},
		EVENTS_PER_TABLE_METADATA_TABLE_NAME: {
			{
				Name:       "migration_uuid",
				DataType:   "uuid",
				IsNullable: "NO",
				Default:    nil,
				IsPrimary:  true,
			},
			{
				Name:       "table_name",
				DataType:   "character varying",
				IsNullable: "NO",
				Default:    nil,
				IsPrimary:  true,
			},
			{
				Name:       "channel_no",
				DataType:   "integer",
				IsNullable: "NO",
				Default:    nil,
				IsPrimary:  true,
			},
			{
				Name:       "total_events",
				DataType:   "bigint",
				IsNullable: "YES",
				Default:    nil,
				IsPrimary:  false,
			},
			{
				Name:       "num_inserts",
				DataType:   "bigint",
				IsNullable: "YES",
				Default:    nil,
				IsPrimary:  false,
			},
			{
				Name:       "num_deletes",
				DataType:   "bigint",
				IsNullable: "YES",
				Default:    nil,
				IsPrimary:  false,
			},
			{
				Name:       "num_updates",
				DataType:   "bigint",
				IsNullable: "YES",
				Default:    nil,
				IsPrimary:  false,
			},
		},
	}

	// Validate the schema and tables
	t.Run("Check all the expected tables and no extra tables", func(t *testing.T) {
		utils.CheckTableExistencePGYB(t, db, BATCH_METADATA_TABLE_SCHEMA, expectedTables)
	})

	// Validate columns for each table
	for tableName, expectedColumns := range expectedTables {
		t.Run(fmt.Sprintf("Check columns for %s table", tableName), func(t *testing.T) {
			table := strings.Split(tableName, ".")[1]
			utils.CheckTableStructurePGYB(t, db, BATCH_METADATA_TABLE_SCHEMA, table, expectedColumns)
		})
	}
}
