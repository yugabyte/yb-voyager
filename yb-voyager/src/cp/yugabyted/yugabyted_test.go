package yugabyted

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/stretchr/testify/assert"
	controlPlane "github.com/yugabyte/yb-voyager/yb-voyager/src/cp"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

func TestDatabaseTablesYugabyteD(t *testing.T) {
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
	// Export the database connection string to env variable YUGABYTED_DB_CONN_STRING
	err = os.Setenv("YUGABYTED_DB_CONN_STRING", dsn)

	exportDir := "./yugabyted_export_dir"
	controlPlane := New(exportDir)
	controlPlane.eventChan = make(chan MigrationEvent, 100)
	controlPlane.rowCountUpdateEventChan = make(chan []VisualizerTableMetrics, 200)

	err = controlPlane.connect()
	assert.NoError(t, err, "Failed to connect to YugabyteDB")

	err = controlPlane.setupDatabase()
	assert.NoError(t, err, "Failed to setup YugabyteDB database")

	expectedTables := map[string][]utils.ColumnPropertiesPGYB{
		QUALIFIED_YUGABYTED_METADATA_TABLE_NAME: {
			{
				Name:       "migration_uuid",
				DataType:   "uuid",
				IsNullable: "NO",
				Default:    nil,
				IsPrimary:  true,
			},
			{
				Name:       "migration_phase",
				DataType:   "integer",
				IsNullable: "NO",
				Default:    nil,
				IsPrimary:  true,
			},
			{
				Name:       "invocation_sequence",
				DataType:   "integer",
				IsNullable: "NO",
				Default:    nil,
				IsPrimary:  true,
			},
			{
				Name:       "migration_dir",
				DataType:   "character varying",
				IsNullable: "YES",
				Default:    nil,
				IsPrimary:  false,
			},
			{
				Name:       "database_name",
				DataType:   "character varying",
				IsNullable: "YES",
				Default:    nil,
				IsPrimary:  false,
			},
			{
				Name:       "schema_name",
				DataType:   "character varying",
				IsNullable: "YES",
				Default:    nil,
				IsPrimary:  false,
			},
			{
				Name:       "payload",
				DataType:   "text",
				IsNullable: "YES",
				Default:    nil,
				IsPrimary:  false,
			},
			{
				Name:       "complexity",
				DataType:   "character varying",
				IsNullable: "YES",
				Default:    nil,
				IsPrimary:  false,
			},
			{
				Name:       "db_type",
				DataType:   "character varying",
				IsNullable: "YES",
				Default:    nil,
				IsPrimary:  false,
			},
			{
				Name:       "status",
				DataType:   "character varying",
				IsNullable: "YES",
				Default:    nil,
				IsPrimary:  false,
			},
			{
				Name:       "invocation_timestamp",
				DataType:   "timestamp with time zone",
				IsNullable: "YES",
				Default:    nil,
				IsPrimary:  false,
			},
			{
				Name:       "host_ip",
				DataType:   "character varying",
				IsNullable: "YES",
				Default:    nil,
				IsPrimary:  false,
			},
			{
				Name:       "port",
				DataType:   "integer",
				IsNullable: "YES",
				Default:    nil,
				IsPrimary:  false,
			},
			{
				Name:       "db_version",
				DataType:   "character varying",
				IsNullable: "YES",
				Default:    nil,
				IsPrimary:  false,
			},
			{
				Name:       "voyager_info",
				DataType:   "character varying",
				IsNullable: "YES",
				Default:    nil,
				IsPrimary:  false,
			},
		},
		YUGABYTED_TABLE_METRICS_TABLE_NAME: {
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
				Name:       "schema_name",
				DataType:   "character varying",
				IsNullable: "NO",
				Default:    nil,
				IsPrimary:  true,
			},
			{
				Name:       "migration_phase",
				DataType:   "integer",
				IsNullable: "NO",
				Default:    nil,
				IsPrimary:  true,
			},
			{
				Name:       "status",
				DataType:   "integer",
				IsNullable: "YES",
				Default:    nil,
				IsPrimary:  false,
			},
			{
				Name:       "count_live_rows",
				DataType:   "integer",
				IsNullable: "YES",
				Default:    nil,
				IsPrimary:  false,
			},
			{
				Name:       "count_total_rows",
				DataType:   "integer",
				IsNullable: "YES",
				Default:    nil,
				IsPrimary:  false,
			},
			{
				Name:       "invocation_timestamp",
				DataType:   "timestamp with time zone",
				IsNullable: "YES",
				Default:    nil,
				IsPrimary:  false,
			},
		},
	}

	// Validate the schema and tables
	t.Run("Check all the expected tables and no extra tables", func(t *testing.T) {
		utils.CheckTableExistencePGYB(t, db, VISUALIZER_METADATA_SCHEMA, expectedTables)
	})

	// Validate columns for each table
	for tableName, expectedColumns := range expectedTables {
		t.Run(fmt.Sprintf("Check columns for %s table", tableName), func(t *testing.T) {
			table := strings.Split(tableName, ".")[1]
			utils.CheckTableStructurePGYB(t, db, VISUALIZER_METADATA_SCHEMA, table, expectedColumns)
		})
	}
}

func TestYugabyteDStructs(t *testing.T) {
	// Test the structs used in YugabyteD

	expectedVoyagerInstance := struct {
		IP                 string
		OperatingSystem    string
		DiskSpaceAvailable uint64
		ExportDirectory    string
	}{}

	t.Run("Check VoyagerInstance structure", func(t *testing.T) {
		utils.CompareStructs(t, reflect.TypeOf(controlPlane.VoyagerInstance{}), reflect.TypeOf(expectedVoyagerInstance), "VoyagerInstance")
	})

	expectedMigrationEvent := struct {
		MigrationUUID       uuid.UUID `json:"migration_uuid"`
		MigrationPhase      int       `json:"migration_phase"`
		InvocationSequence  int       `json:"invocation_sequence"`
		MigrationDirectory  string    `json:"migration_dir"`
		DatabaseName        string    `json:"database_name"`
		SchemaName          string    `json:"schema_name"`
		DBIP                string    `json:"db_ip"`
		Port                int       `json:"port"`
		DBVersion           string    `json:"db_version"`
		Payload             string    `json:"payload"`
		VoyagerInfo         string    `json:"voyager_info"`
		DBType              string    `json:"db_type"`
		Status              string    `json:"status"`
		InvocationTimestamp string    `json:"invocation_timestamp"`
	}{}

	t.Run("Check MigrationEvent structure", func(t *testing.T) {
		utils.CompareStructs(t, reflect.TypeOf(MigrationEvent{}), reflect.TypeOf(expectedMigrationEvent), "MigrationEvent")
	})

	expectedVisualizerTableMetrics := struct {
		MigrationUUID       uuid.UUID `json:"migration_uuid"`
		TableName           string    `json:"table_name"`
		Schema              string    `json:"schema_name"`
		MigrationPhase      int       `json:"migration_phase"`
		Status              int       `json:"status"`
		CountLiveRows       int64     `json:"count_live_rows"`
		CountTotalRows      int64     `json:"count_total_rows"`
		InvocationTimestamp string    `json:"invocation_timestamp"`
	}{}

	t.Run("Check VisualizerTableMetrics structure", func(t *testing.T) {
		utils.CompareStructs(t, reflect.TypeOf(VisualizerTableMetrics{}), reflect.TypeOf(expectedVisualizerTableMetrics), "VisualizerTableMetrics")
	})

	expectedYugabyteD := struct {
		sync.Mutex
		migrationDirectory       string
		voyagerInfo              *controlPlane.VoyagerInstance
		waitGroup                sync.WaitGroup
		eventChan                chan (MigrationEvent)
		rowCountUpdateEventChan  chan ([]VisualizerTableMetrics)
		connPool                 *pgxpool.Pool
		lastRowCountUpdate       map[string]time.Time
		latestInvocationSequence int
	}{}

	t.Run("Check YugabyteD structure", func(t *testing.T) {
		utils.CompareStructs(t, reflect.TypeOf(&YugabyteD{}).Elem(), reflect.TypeOf(&expectedYugabyteD).Elem(), "YugabyteD")
	})
}
