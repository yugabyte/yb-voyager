//go:build integration

/*
Copyright (c) YugabyteDB, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package yugabyted

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	controlPlane "github.com/yugabyte/yb-voyager/yb-voyager/src/cp"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

func TestYugabyteDTableSchema(t *testing.T) {
	ctx := context.Background()

	yugabyteDBContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	err := yugabyteDBContainer.Start(ctx)
	if err != nil {
		utils.ErrExit("Failed to start yugabytedb container: %v", err)
	}
	defer testcontainers.TerminateAllContainers()
	assert.NoError(t, err, "Failed to start YugabyteDB container")

	// Connect to the database
	dsn := yugabyteDBContainer.GetConnectionString()
	db, err := sql.Open("pgx", dsn)
	assert.NoError(t, err)
	defer db.Close()

	// Wait for the database to be ready
	err = testutils.WaitForDBToBeReady(db)
	assert.NoError(t, err)
	// Export the database connection string to env variable YUGABYTED_DB_CONN_STRING
	err = os.Setenv("YUGABYTED_DB_CONN_STRING", dsn)

	exportDir := filepath.Join(os.TempDir(), "yugabyted")

	// Create a temporary export directory for testing
	err = os.MkdirAll(exportDir, 0755)
	assert.NoError(t, err, "Failed to create temporary export directory")
	// Ensure the directory is removed after the test
	defer func() {
		err := os.RemoveAll(exportDir)
		assert.NoError(t, err, "Failed to remove temporary export directory")
	}()

	controlPlane := New(exportDir)
	controlPlane.eventChan = make(chan MigrationEvent, 100)
	controlPlane.rowCountUpdateEventChan = make(chan []VisualizerTableMetrics, 200)

	err = controlPlane.connect()
	assert.NoError(t, err, "Failed to connect to YugabyteDB")

	err = controlPlane.setupDatabase()
	assert.NoError(t, err, "Failed to setup YugabyteDB database")

	expectedTables := map[string]map[string]testutils.ColumnPropertiesPG{
		QUALIFIED_YUGABYTED_METADATA_TABLE_NAME: {
			"migration_uuid":       {Type: "uuid", IsNullable: "NO", Default: sql.NullString{Valid: false}, IsPrimary: true},
			"migration_phase":      {Type: "integer", IsNullable: "NO", Default: sql.NullString{Valid: false}, IsPrimary: true},
			"invocation_sequence":  {Type: "integer", IsNullable: "NO", Default: sql.NullString{Valid: false}, IsPrimary: true},
			"migration_dir":        {Type: "character varying", IsNullable: "YES", Default: sql.NullString{Valid: false}, IsPrimary: false},
			"database_name":        {Type: "character varying", IsNullable: "YES", Default: sql.NullString{Valid: false}, IsPrimary: false},
			"schema_name":          {Type: "character varying", IsNullable: "YES", Default: sql.NullString{Valid: false}, IsPrimary: false},
			"payload":              {Type: "text", IsNullable: "YES", Default: sql.NullString{Valid: false}, IsPrimary: false},
			"complexity":           {Type: "character varying", IsNullable: "YES", Default: sql.NullString{Valid: false}, IsPrimary: false},
			"db_type":              {Type: "character varying", IsNullable: "YES", Default: sql.NullString{Valid: false}, IsPrimary: false},
			"status":               {Type: "character varying", IsNullable: "YES", Default: sql.NullString{Valid: false}, IsPrimary: false},
			"invocation_timestamp": {Type: "timestamp with time zone", IsNullable: "YES", Default: sql.NullString{Valid: false}, IsPrimary: false},
			"host_ip":              {Type: "character varying", IsNullable: "YES", Default: sql.NullString{Valid: false}, IsPrimary: false},
			"port":                 {Type: "integer", IsNullable: "YES", Default: sql.NullString{Valid: false}, IsPrimary: false},
			"db_version":           {Type: "character varying", IsNullable: "YES", Default: sql.NullString{Valid: false}, IsPrimary: false},
			"voyager_info":         {Type: "character varying", IsNullable: "YES", Default: sql.NullString{Valid: false}, IsPrimary: false},
		},
		YUGABYTED_TABLE_METRICS_TABLE_NAME: {
			"migration_uuid":       {Type: "uuid", IsNullable: "NO", Default: sql.NullString{Valid: false}, IsPrimary: true},
			"table_name":           {Type: "character varying", IsNullable: "NO", Default: sql.NullString{Valid: false}, IsPrimary: true},
			"schema_name":          {Type: "character varying", IsNullable: "NO", Default: sql.NullString{Valid: false}, IsPrimary: true},
			"migration_phase":      {Type: "integer", IsNullable: "NO", Default: sql.NullString{Valid: false}, IsPrimary: true},
			"status":               {Type: "integer", IsNullable: "YES", Default: sql.NullString{Valid: false}, IsPrimary: false},
			"count_live_rows":      {Type: "integer", IsNullable: "YES", Default: sql.NullString{Valid: false}, IsPrimary: false},
			"count_total_rows":     {Type: "integer", IsNullable: "YES", Default: sql.NullString{Valid: false}, IsPrimary: false},
			"invocation_timestamp": {Type: "timestamp with time zone", IsNullable: "YES", Default: sql.NullString{Valid: false}, IsPrimary: false},
		},
	}

	// Validate the schema and tables
	t.Run("Check all the expected tables and no extra tables", func(t *testing.T) {
		testutils.CheckTableExistencePG(t, db, VISUALIZER_METADATA_SCHEMA, expectedTables)
	})

	// Validate columns for each table
	for tableName, expectedColumns := range expectedTables {
		t.Run(fmt.Sprintf("Check columns for %s table", tableName), func(t *testing.T) {
			table := strings.Split(tableName, ".")[1]
			testutils.CheckTableStructurePG(t, db, VISUALIZER_METADATA_SCHEMA, table, expectedColumns)
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

	t.Run("Validate VoyagerInstance Struct Definition", func(t *testing.T) {
		testutils.CompareStructs(t, reflect.TypeOf(controlPlane.VoyagerInstance{}), reflect.TypeOf(expectedVoyagerInstance), "VoyagerInstance")
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

	t.Run("Validate MigrationEvent Struct Definition", func(t *testing.T) {
		testutils.CompareStructs(t, reflect.TypeOf(MigrationEvent{}), reflect.TypeOf(expectedMigrationEvent), "MigrationEvent")
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

	t.Run("Validate VisualizerTableMetrics Struct Definition", func(t *testing.T) {
		testutils.CompareStructs(t, reflect.TypeOf(VisualizerTableMetrics{}), reflect.TypeOf(expectedVisualizerTableMetrics), "VisualizerTableMetrics")
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

	t.Run("Validate YugabyteD Struct Definition", func(t *testing.T) {
		testutils.CompareStructs(t, reflect.TypeOf(&YugabyteD{}).Elem(), reflect.TypeOf(&expectedYugabyteD).Elem(), "YugabyteD")
	})
}
