//go:build integration_voyager_command

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
package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/jsonfile"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

////=========================================

// HELPER functions for live

// Checks for streaming started or not using dbzm status
func isExportDataStreamingStarted(t *testing.T) bool {
	//check for status until its streaming
	statusFilePath := filepath.Join(exportDir, "data", "export_status.json")
	status, err := dbzm.ReadExportStatus(statusFilePath)
	if err != nil {
		utils.ErrExit("Failed to read export status file: %s: %v", statusFilePath, err)
	}
	testutils.FatalIfError(t, err, "reading dbzm export status")
	if status != nil && status.Mode == dbzm.MODE_STREAMING {
		return true
	}
	return false
}

// checks for the snapshot phase completed in export and import both by checking the get data migration report
func snapshotPhaseCompleted(t *testing.T, postgresPass string, targetPass string, snapshotRows int64, tableName string) bool {
	err := testutils.NewVoyagerCommandRunner(nil, "get data-migration-report", []string{
		"--export-dir", exportDir,
		"--output-format", "json",
		"--source-db-password", postgresPass,
		"--target-db-password", targetPass,
	}, nil, true).Run()
	testutils.FatalIfError(t, err, "get data-migration-report command failed")

	reportFilePath := filepath.Join(exportDir, "reports", "data-migration-report.json")
	// Check if the report file exists and is not empty.
	if ok := utils.FileOrFolderExists(reportFilePath); ok {
		jsonFile := jsonfile.NewJsonFile[[]*rowData](reportFilePath)
		rowData, err := jsonFile.Read()
		testutils.FatalIfError(t, err, "error reading get data-migration-report")
		exportSnapshot := 0
		importSnapshot := 0
		for _, row := range *rowData {
			if row.TableName == tableName {
				if row.DBType == "source" {
					exportSnapshot = int(row.ExportedSnapshotRows)
				}
				if row.DBType == "target" {
					importSnapshot = int(row.ImportedSnapshotRows)
				}
			}
		}
		if exportSnapshot == int(snapshotRows) && exportSnapshot == importSnapshot {
			return true
		}

	}
	return false
}

// checks for the streaming phase completed in export and import both by checking the get data migration report
func streamingPhaseCompleted(t *testing.T, postgresPass string, targetPass string, streamingInserts int64, tableName string) bool {
	err := testutils.NewVoyagerCommandRunner(nil, "get data-migration-report", []string{
		"--export-dir", exportDir,
		"--output-format", "json",
		"--source-db-password", postgresPass,
		"--target-db-password", targetPass,
	}, nil, true).Run()
	testutils.FatalIfError(t, err, "get data-migration-report command failed")

	reportFilePath := filepath.Join(exportDir, "reports", "data-migration-report.json")
	// Check if the report file exists and is not empty.
	if ok := utils.FileOrFolderExists(reportFilePath); ok {
		jsonFile := jsonfile.NewJsonFile[[]*rowData](reportFilePath)
		rowData, err := jsonFile.Read()
		testutils.FatalIfError(t, err, "error reading get data-migration-report")
		exportInserts := 0
		importInserts := 0
		for _, row := range *rowData {
			if row.TableName == tableName {
				if row.DBType == "source" {
					exportInserts = int(row.ExportedInserts)
				}
				if row.DBType == "target" {
					importInserts = int(row.ImportedInserts)
				}

			}
		}
		if exportInserts == int(streamingInserts) && exportInserts == importInserts {
			return true
		}

	}
	return false
}

// This inserts some rows in target table having sequence and validates if the ids ingested are correct or not
func assertSequenceValues(t *testing.T, startID int, endId int, ybConn *sql.DB, tableName string) {
	_, err := ybConn.Exec(fmt.Sprintf(`INSERT INTO test_schema.test_live (name, email, description)
SELECT
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(%d, %d);`, startID, endId))
	testutils.FatalIfError(t, err, "inserting into target")

	ids := []string{}
	for i := startID; i <= endId; i++ {
		ids = append(ids, strconv.Itoa(i))
	}
	query := fmt.Sprintf("SELECT id from %s where id IN (%s) ORDER BY id;", tableName, strings.Join(ids, ", "))
	rows, err := ybConn.Query(query)
	testutils.FatalIfError(t, err, "failed to read data")
	var resIds []string
	for rows.Next() {
		var id int
		err = rows.Scan(&id)
		testutils.FatalIfError(t, err, "error scanning rows")
		resIds = append(resIds, strconv.Itoa(id))
	}

	assert.Equal(t, ids, resIds)
}

// Basic Test for live migration with cutover
// cutover -> validate sequence restoration
//
//export data -> import data (streaming for some events) -> once all data is streamed to target
func TestBasicLiveMigrationWithCutover(t *testing.T) {
	ctx := context.Background()

	// Create a temporary export directory.
	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	createSchemaSQL := `CREATE SCHEMA IF NOT EXISTS test_schema;`
	createTableSQL := `
CREATE TABLE test_schema.test_live (
	id SERIAL PRIMARY KEY,
	name TEXT,
	email TEXT,
	description TEXT
);`
	insertDataSQL := `
INSERT INTO test_schema.test_live (name, email, description)
SELECT
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 10);`
	dropSchemaSQL := `DROP SCHEMA IF EXISTS test_schema CASCADE;`

	// Start Postgres container for live migration
	postgresContainer := testcontainers.NewTestContainer("postgresql", &testcontainers.ContainerConfig{
		ForLive: true,
	})
	if err := postgresContainer.Start(ctx); err != nil {
		utils.ErrExit("Failed to start Postgres container: %v", err)
	}

	// Start YugabyteDB container.
	yugabytedbContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	if err := yugabytedbContainer.Start(ctx); err != nil {
		utils.ErrExit("Failed to start YugabyteDB container: %v", err)
	}
	postgresContainer.ExecuteSqls([]string{
		createSchemaSQL,
		createTableSQL,
		insertDataSQL,
	}...)

	yugabytedbContainer.ExecuteSqls([]string{
		createSchemaSQL,
		createTableSQL,
	}...)

	defer postgresContainer.ExecuteSqls(dropSchemaSQL)
	defer yugabytedbContainer.ExecuteSqls(dropSchemaSQL)

	err := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--source-db-schema", "test_schema",
		"--disable-pb", "true",
		"--export-type", SNAPSHOT_AND_CHANGES,
		"--yes",
	}, nil, true).Run()
	testutils.FatalIfError(t, err, "Export command failed")

	start := time.Now()
	for time.Since(start) < (30 * time.Second) { // 30 seconds timeout
		if isExportDataStreamingStarted(t) {
			break
		}
		time.Sleep(1 * time.Second)
	}

	err = testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--yes",
	}, func() {
		time.Sleep(5 * time.Second)
	}, true).Run()

	testutils.FatalIfError(t, err, "Import command failed")

	start = time.Now()
	for time.Since(start) < (30 * time.Second) { // 30 seconds timeout
		if snapshotPhaseCompleted(t, postgresContainer.GetConfig().Password,
			yugabytedbContainer.GetConfig().Password, 10, `test_schema."test_live"`) {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// Connect to both Postgres and YugabyteDB.
	pgConn, err := postgresContainer.GetConnection()
	testutils.FatalIfError(t, err, "connecting to Postgres")

	ybConn, err := yugabytedbContainer.GetConnection()
	testutils.FatalIfError(t, err, "Error connecting to YugabyteDB")

	// Compare the full table data between Postgres and YugabyteDB for snapshot part.
	// We assume the table "test_data" has a primary key "id" so we order by it.
	if err := testutils.CompareTableData(ctx, pgConn, ybConn, "test_schema.test_live", "id"); err != nil {
		t.Errorf("Table data mismatch between Postgres and YugabyteDB: %v", err)
	}

	//streaming events 5 events
	postgresContainer.ExecuteSqls([]string{
		`INSERT INTO test_schema.test_live (name, email, description)
SELECT
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 5);`,
	}...)

	start = time.Now()
	for time.Since(start) < (30 * time.Second) { // 30 seconds timeout
		if streamingPhaseCompleted(t, postgresContainer.GetConfig().Password,
			yugabytedbContainer.GetConfig().Password, 5, `test_schema."test_live"`) {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// Compare the full table data between Postgres and YugabyteDB for streaming part.
	if err := testutils.CompareTableData(ctx, pgConn, ybConn, "test_schema.test_live", "id"); err != nil {
		t.Errorf("Table data mismatch between Postgres and YugabyteDB after streaming: %v", err)
	}

	// Run cutover
	err = testutils.NewVoyagerCommandRunner(nil, "initiate cutover to target", []string{
		"--export-dir", exportDir,
		"--yes",
		"--prepare-for-fall-back", "false",
	}, nil, false).Run()
	testutils.FatalIfError(t, err, "Cutover command failed")

	metaDB, err = metadb.NewMetaDB(exportDir)
	testutils.FatalIfError(t, err, "Failed to initialize meta db")

	start = time.Now()
	for time.Since(start) < (30 * time.Second) { // 30 seconds timeout
		status := getCutoverStatus()
		if status == COMPLETED {
			break
		}
		time.Sleep(1 * time.Second)
	}
	//Check if ids from 16-25 are present in target this is to verify the sequence serial col is restored properly till last value
	assertSequenceValues(t, 16, 25, ybConn, `test_schema.test_live`)

}

// test for live migration with resumption and failure during restore sequences
// cutover -> drop sequence on target -> start import again (validate its failing at restore sequences)
// create sequence back on target -> re-run import again
// validate sequence by inserting data
//
//export data -> import data (streaming some data) -> once done kill import
func TestLiveMigrationWithImportResumptionOnFailureAtRestoreSequences(t *testing.T) {
	ctx := context.Background()

	// Create a temporary export directory.
	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	createSchemaSQL := `CREATE SCHEMA IF NOT EXISTS test_schema;`
	createTableSQL := `
CREATE TABLE test_schema.test_live (
	id SERIAL PRIMARY KEY,
	name TEXT,
	email TEXT,
	description TEXT
);`
	insertDataSQL := `
INSERT INTO test_schema.test_live (name, email, description)
SELECT
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 20);`
	dropSchemaSQL := `DROP SCHEMA IF EXISTS test_schema CASCADE;`

	// Start Postgres container for live migration
	postgresContainer := testcontainers.NewTestContainer("postgresql", &testcontainers.ContainerConfig{
		ForLive: true,
	})
	if err := postgresContainer.Start(ctx); err != nil {
		utils.ErrExit("Failed to start Postgres container: %v", err)
	}

	// Start YugabyteDB container.
	yugabytedbContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	if err := yugabytedbContainer.Start(ctx); err != nil {
		utils.ErrExit("Failed to start YugabyteDB container: %v", err)
	}
	postgresContainer.ExecuteSqls([]string{
		createSchemaSQL,
		createTableSQL,
		insertDataSQL,
	}...)

	yugabytedbContainer.ExecuteSqls([]string{
		createSchemaSQL,
		createTableSQL,
	}...)

	defer postgresContainer.ExecuteSqls(dropSchemaSQL)
	defer yugabytedbContainer.ExecuteSqls(dropSchemaSQL)

	err := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--source-db-schema", "test_schema",
		"--disable-pb", "true",
		"--export-type", SNAPSHOT_AND_CHANGES,
		"--yes",
	}, nil, true).Run()
	testutils.FatalIfError(t, err, "Export command failed")

	start := time.Now()
	for time.Since(start) < (30 * time.Second) { // 30 seconds timeout
		if isExportDataStreamingStarted(t) {
			break
		}
		time.Sleep(1 * time.Second)
	}

	importCmd := testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--yes",
	}, func() {
		time.Sleep(5 * time.Second)
	}, true)
	err = importCmd.Run()
	testutils.FatalIfError(t, err, "Import command failed")

	start = time.Now()
	for time.Since(start) < (30 * time.Second) { // 30 seconds timeout
		if snapshotPhaseCompleted(t, postgresContainer.GetConfig().Password,
			yugabytedbContainer.GetConfig().Password, 20, `test_schema."test_live"`) {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// Connect to both Postgres and YugabyteDB.
	pgConn, err := postgresContainer.GetConnection()
	testutils.FatalIfError(t, err, "connecting to Postgres")

	ybConn, err := yugabytedbContainer.GetConnection()
	testutils.FatalIfError(t, err, "Error connecting to YugabyteDB")

	// Compare the full table data between Postgres and YugabyteDB for snapshot part.
	// We assume the table "test_data" has a primary key "id" so we order by it.
	if err := testutils.CompareTableData(ctx, pgConn, ybConn, "test_schema.test_live", "id"); err != nil {
		t.Errorf("Table data mismatch between Postgres and YugabyteDB: %v", err)
	}

	//streaming events 15 events
	postgresContainer.ExecuteSqls([]string{
		`INSERT INTO test_schema.test_live (name, email, description)
SELECT
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 15);`,
	}...)

	start = time.Now()
	for time.Since(start) < (30 * time.Second) { // 30 seconds timeout
		if streamingPhaseCompleted(t, postgresContainer.GetConfig().Password,
			yugabytedbContainer.GetConfig().Password, 15, `test_schema."test_live"`) {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// Compare the full table data between Postgres and YugabyteDB for streaming part.
	if err := testutils.CompareTableData(ctx, pgConn, ybConn, "test_schema.test_live", "id"); err != nil {
		t.Errorf("Table data mismatch between Postgres and YugabyteDB after streaming: %v", err)
	}

	//Stopping import command
	if err := importCmd.Kill(); err != nil {
		testutils.FatalIfError(t, err, "killing the import data process errored")
	}

	time.Sleep(5 * time.Second)

	// Wait for the command to exit.
	if err := importCmd.Wait(); err != nil {
		t.Logf("Async import run exited with error (expected): %v", err)
	} else {
		t.Logf("Async import run completed unexpectedly")
	}

	time.Sleep(10 * time.Second)

	// Perform cutover
	err = testutils.NewVoyagerCommandRunner(nil, "initiate cutover to target", []string{
		"--export-dir", exportDir,
		"--yes",
		"--prepare-for-fall-back", "false",
	}, nil, false).Run()
	testutils.FatalIfError(t, err, "Cutover command failed")

	//Dropping sequence on yugabyte
	yugabytedbContainer.ExecuteSqls([]string{
		`DROP SEQUENCE test_schema.test_live_id_seq CASCADE;`,
	}...)

	time.Sleep(10 * time.Second)

	//Resume import command after deleting a sequence of the table column idand import should fail while restoring sequences as cutover is already triggered
	importCmd = testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--yes",
	}, nil, false)
	err = importCmd.Run()
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(importCmd.Stderr(), "failed to restore sequences:"))

	//Create sequence back on yb to resume import and finish cutover

	yugabytedbContainer.ExecuteSqls([]string{
		`CREATE SEQUENCE test_schema.test_live_id_seq;`,
		`ALTER SEQUENCE test_schema.test_live_id_seq OWNED BY test_schema.test_live.id;`,
		`ALTER TABLE test_schema.test_live ALTER COLUMN id SET DEFAULT nextval('test_schema.test_live_id_seq');`,
	}...)

	//Resume import command after deleting a sequence of the table column idand import should pass while restoring sequences as cutover is already triggered
	err = testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--yes",
	}, nil, true).Run()
	testutils.FatalIfError(t, err, "import data failed")

	metaDB, err = metadb.NewMetaDB(exportDir)
	testutils.FatalIfError(t, err, "Failed to initialize meta db")

	start = time.Now()
	for time.Since(start) < (30 * time.Second) { // 30 seconds timeout
		status := getCutoverStatus()
		if status == COMPLETED {
			break
		}
		time.Sleep(1 * time.Second)
	}

	//Check if ids from 36-45 are present in target this is to verify the sequence serial col is restored properly till last value
	assertSequenceValues(t, 36, 45, ybConn, `test_schema.test_live`)

}

// test live migration with import resumption with  generated always schema
// cutover -> start import again
// validate ALWAYS type on the target
//
//export data -> import data (streaming some data) -> once done kill import
func TestLiveMigrationWithImportResumptionWithGeneratedAlwaysColumn(t *testing.T) {
	ctx := context.Background()

	// Create a temporary export directory.
	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	createSchemaSQL := `CREATE SCHEMA IF NOT EXISTS test_schema;`
	createTableSQL := `
CREATE TABLE test_schema.test_live (
	id int GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	name TEXT,
	email TEXT,
	description TEXT
);`
	insertDataSQL := `
INSERT INTO test_schema.test_live (name, email, description)
SELECT
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 20);`
	dropSchemaSQL := `DROP SCHEMA IF EXISTS test_schema CASCADE;`

	// Start Postgres container with live migration
	postgresContainer := testcontainers.NewTestContainer("postgresql", &testcontainers.ContainerConfig{
		ForLive: true,
	})
	if err := postgresContainer.Start(ctx); err != nil {
		utils.ErrExit("Failed to start Postgres container: %v", err)
	}

	// Start YugabyteDB container.
	yugabytedbContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	if err := yugabytedbContainer.Start(ctx); err != nil {
		utils.ErrExit("Failed to start YugabyteDB container: %v", err)
	}
	postgresContainer.ExecuteSqls([]string{
		createSchemaSQL,
		createTableSQL,
		insertDataSQL,
	}...)

	yugabytedbContainer.ExecuteSqls([]string{
		createSchemaSQL,
		createTableSQL,
	}...)

	defer postgresContainer.ExecuteSqls(dropSchemaSQL)
	defer yugabytedbContainer.ExecuteSqls(dropSchemaSQL)

	err := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--source-db-schema", "test_schema",
		"--disable-pb", "true",
		"--export-type", SNAPSHOT_AND_CHANGES,
		"--yes",
	}, nil, true).Run()
	testutils.FatalIfError(t, err, "Export command failed")

	start := time.Now()
	for time.Since(start) < (30 * time.Second) { // 30 seconds timeout
		//check for status until its streaming
		if isExportDataStreamingStarted(t) {
			break
		}
		time.Sleep(1 * time.Second)
	}

	importCmd := testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--yes",
	}, nil, true)
	err = importCmd.Run()
	testutils.FatalIfError(t, err, "Import command failed")

	time.Sleep(5 * time.Second)

	start = time.Now()
	for time.Since(start) < (30 * time.Second) { // 30 seconds timeout
		if snapshotPhaseCompleted(t, postgresContainer.GetConfig().Password,
			yugabytedbContainer.GetConfig().Password, 20, `test_schema."test_live"`) {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// Connect to both Postgres and YugabyteDB.
	pgConn, err := postgresContainer.GetConnection()
	testutils.FatalIfError(t, err, "connecting to Postgres")

	ybConn, err := yugabytedbContainer.GetConnection()
	testutils.FatalIfError(t, err, "Error connecting to YugabyteDB")

	// Compare the full table data between Postgres and YugabyteDB for snapshot part.
	// We assume the table "test_data" has a primary key "id" so we order by it.
	if err := testutils.CompareTableData(ctx, pgConn, ybConn, "test_schema.test_live", "id"); err != nil {
		t.Errorf("Table data mismatch between Postgres and YugabyteDB: %v", err)
	}

	//streaming events 15 events
	postgresContainer.ExecuteSqls([]string{
		`INSERT INTO test_schema.test_live (name, email, description)
SELECT
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 15);`,
	}...)

	start = time.Now()
	for time.Since(start) < (30 * time.Second) { // 30 seconds timeout
		if streamingPhaseCompleted(t, postgresContainer.GetConfig().Password,
			yugabytedbContainer.GetConfig().Password, 15, `test_schema."test_live"`) {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// Compare the full table data between Postgres and YugabyteDB for streaming part.
	if err := testutils.CompareTableData(ctx, pgConn, ybConn, "test_schema.test_live", "id"); err != nil {
		t.Errorf("Table data mismatch between Postgres and YugabyteDB after streaming: %v", err)
	}

	//Stopping import command
	if err := importCmd.Kill(); err != nil {
		testutils.FatalIfError(t, err, "killing the import data process errored")
	}

	time.Sleep(5 * time.Second)

	// Wait for the command to exit.
	if err := importCmd.Wait(); err != nil {
		t.Logf("Async import run exited with error (expected): %v", err)
	} else {
		t.Logf("Async import run completed unexpectedly")
	}

	time.Sleep(10 * time.Second)

	// Perform cutover
	err = testutils.NewVoyagerCommandRunner(nil, "initiate cutover to target", []string{
		"--export-dir", exportDir,
		"--yes",
		"--prepare-for-fall-back", "false",
	}, nil, false).Run()
	testutils.FatalIfError(t, err, "Cutover command failed")

	//Resume import command to finish the cutover
	err = testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--yes",
	}, nil, true).Run()
	testutils.FatalIfError(t, err, "import data failed")

	metaDB, err = metadb.NewMetaDB(exportDir)
	testutils.FatalIfError(t, err, "Failed to initialize meta db")

	start = time.Now()
	for time.Since(start) < (30 * time.Second) { // 30 seconds timeout
		status := getCutoverStatus()
		if status == COMPLETED {
			break
		}
		time.Sleep(1 * time.Second)
	}

	//Check if always is restored back
	query := fmt.Sprintf(`SELECT column_name FROM information_schema.columns where table_schema='test_schema' AND
		table_name='test_live' AND is_identity='YES' AND identity_generation='ALWAYS'`)

	var col string
	err = ybConn.QueryRow(query).Scan(&col)
	testutils.FatalIfError(t, err, "error checking if table has always or not")
	assert.Equal(t, col, "id")
}
