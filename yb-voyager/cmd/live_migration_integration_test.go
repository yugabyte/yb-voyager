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

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/jsonfile"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

////=========================================

// HELPER functions for live

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
func streamingPhaseCompleted(t *testing.T, postgresPass string, targetPass string, streamingInserts int64, streamingUpdates int64, streamingDeletes int64, tableName string) bool {
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
		exportUpdates := 0
		importUpdates := 0
		exportDeletes := 0
		importDeletes := 0
		for _, row := range *rowData {
			if row.TableName == tableName {
				if row.DBType == "source" {
					exportInserts = int(row.ExportedInserts)
					exportUpdates = int(row.ExportedUpdates)
					exportDeletes = int(row.ExportedDeletes)
				}
				if row.DBType == "target" {
					importInserts = int(row.ImportedInserts)
					importUpdates = int(row.ImportedUpdates)
					importDeletes = int(row.ImportedDeletes)
				}

			}
		}
		if exportInserts == int(streamingInserts) && exportInserts == importInserts &&
			exportUpdates == int(streamingUpdates) && exportUpdates == importUpdates &&
			exportDeletes == int(streamingDeletes) && exportDeletes == importDeletes {
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

	ids := []int{}
	for i := startID; i <= endId; i++ {
		ids = append(ids, i)
	}
	query := fmt.Sprintf("SELECT id from %s where id IN (%s) ORDER BY id;", tableName, strings.Join(lo.Map(ids, func(id int, _ int) string {
		return strconv.Itoa(id)
	}), ", "))
	rows, err := ybConn.Query(query)
	testutils.FatalIfError(t, err, "failed to read data")
	var resIds []int
	for rows.Next() {
		var id int
		err = rows.Scan(&id)
		testutils.FatalIfError(t, err, "error scanning rows")
		resIds = append(resIds, id)
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
	}, func() {
		time.Sleep(5 * time.Second) // Wait for the export to start
	}, true).Run()
	testutils.FatalIfError(t, err, "Export command failed")

	err = testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--yes",
	}, func() {
		time.Sleep(5 * time.Second)
	}, true).Run()

	testutils.FatalIfError(t, err, "Import command failed")

	ok := utils.RetryWorkWithTimeout(1, 30, func() bool {
		return snapshotPhaseCompleted(t, postgresContainer.GetConfig().Password,
			yugabytedbContainer.GetConfig().Password, 10, `test_schema."test_live"`)
	})
	assert.True(t, ok)

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

	ok = utils.RetryWorkWithTimeout(1, 30, func() bool {
		return streamingPhaseCompleted(t, postgresContainer.GetConfig().Password,
			yugabytedbContainer.GetConfig().Password, 5, 0, 0, `test_schema."test_live"`)
	})
	assert.True(t, ok)

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

	ok = utils.RetryWorkWithTimeout(1, 30, func() bool {
		return getCutoverStatus() == COMPLETED
	})
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
	}, func() {
		time.Sleep(5 * time.Second) // Wait for the export to start
	}, true).Run()
	testutils.FatalIfError(t, err, "Export command failed")

	importCmd := testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--yes",
	}, func() {
		time.Sleep(5 * time.Second)
	}, true)
	err = importCmd.Run()
	testutils.FatalIfError(t, err, "Import command failed")

	ok := utils.RetryWorkWithTimeout(1, 30, func() bool {
		return snapshotPhaseCompleted(t, postgresContainer.GetConfig().Password,
			yugabytedbContainer.GetConfig().Password, 20, `test_schema."test_live"`)
	})
	assert.True(t, ok)
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

	ok = utils.RetryWorkWithTimeout(1, 30, func() bool {
		return streamingPhaseCompleted(t, postgresContainer.GetConfig().Password,
			yugabytedbContainer.GetConfig().Password, 15, 0, 0, `test_schema."test_live"`)
	})
	assert.True(t, ok)

	// Compare the full table data between Postgres and YugabyteDB for streaming part.
	if err := testutils.CompareTableData(ctx, pgConn, ybConn, "test_schema.test_live", "id"); err != nil {
		t.Errorf("Table data mismatch between Postgres and YugabyteDB after streaming: %v", err)
	}

	//Stopping import command
	if err := importCmd.Kill(); err != nil {
		testutils.FatalIfError(t, err, "killing the import data process errored")
	}

	// Wait for the command to exit.
	if err := importCmd.Wait(); err != nil {
		t.Logf("Async import run exited with error (expected): %v", err)
	} else {
		t.Logf("Async import run completed unexpectedly")
	}

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
	importCmd.SetAsync(false)
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
	importCmd.SetAsync(true)
	err = importCmd.Run()
	testutils.FatalIfError(t, err, "import data failed")

	metaDB, err = metadb.NewMetaDB(exportDir)
	testutils.FatalIfError(t, err, "Failed to initialize meta db")

	ok = utils.RetryWorkWithTimeout(1, 30, func() bool {
		return getCutoverStatus() == COMPLETED
	})
	assert.True(t, ok)
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
	}, func() {
		time.Sleep(5 * time.Second) // Wait for the export to start
	}, true).Run()
	testutils.FatalIfError(t, err, "Export command failed")

	importCmd := testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--yes",
	}, nil, true)
	err = importCmd.Run()
	testutils.FatalIfError(t, err, "Import command failed")

	time.Sleep(5 * time.Second)

	ok := utils.RetryWorkWithTimeout(1, 30, func() bool {
		return snapshotPhaseCompleted(t, postgresContainer.GetConfig().Password,
			yugabytedbContainer.GetConfig().Password, 20, `test_schema."test_live"`)
	})
	assert.True(t, ok)
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

	ok = utils.RetryWorkWithTimeout(1, 30, func() bool {
		return streamingPhaseCompleted(t, postgresContainer.GetConfig().Password,
			yugabytedbContainer.GetConfig().Password, 15, 0, 0, `test_schema."test_live"`)
	})
	assert.True(t, ok)

	// Compare the full table data between Postgres and YugabyteDB for streaming part.
	if err := testutils.CompareTableData(ctx, pgConn, ybConn, "test_schema.test_live", "id"); err != nil {
		t.Errorf("Table data mismatch between Postgres and YugabyteDB after streaming: %v", err)
	}

	//Stopping import command
	if err := importCmd.Kill(); err != nil {
		testutils.FatalIfError(t, err, "killing the import data process errored")
	}

	// Wait for the command to exit.
	if err := importCmd.Wait(); err != nil {
		t.Logf("Async import run exited with error (expected): %v", err)
	} else {
		t.Logf("Async import run completed unexpectedly")
	}

	// Perform cutover
	err = testutils.NewVoyagerCommandRunner(nil, "initiate cutover to target", []string{
		"--export-dir", exportDir,
		"--yes",
		"--prepare-for-fall-back", "false",
	}, nil, false).Run()
	testutils.FatalIfError(t, err, "Cutover command failed")

	//Resume import command to finish the cutover
	err = importCmd.Run()
	testutils.FatalIfError(t, err, "import data failed")

	metaDB, err = metadb.NewMetaDB(exportDir)
	testutils.FatalIfError(t, err, "Failed to initialize meta db")

	ok = utils.RetryWorkWithTimeout(1, 30, func() bool {
		return getCutoverStatus() == COMPLETED
	})
	assert.True(t, ok)
	//Check if always is restored back
	query := fmt.Sprintf(`SELECT column_name FROM information_schema.columns where table_schema='test_schema' AND
		table_name='test_live' AND is_identity='YES' AND identity_generation='ALWAYS'`)

	var col string
	err = ybConn.QueryRow(query).Scan(&col)
	testutils.FatalIfError(t, err, "error checking if table has always or not")
	assert.Equal(t, col, "id")
}

func TestLiveMigrationResumptionWithChangeInCDCPartitioningStrategy(t *testing.T) {
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
	}, func() {
		time.Sleep(5 * time.Second) // Wait for the export to start
	}, true).Run()
	testutils.FatalIfError(t, err, "Export command failed")

	importCmd := testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--yes",
	}, func() {
		time.Sleep(5 * time.Second)
	}, true)
	err = importCmd.Run()

	testutils.FatalIfError(t, err, "Import command failed")

	if err := importCmd.Kill(); err != nil {
		testutils.FatalIfError(t, err, "killing the import data process errored")
	}
	if err := importCmd.Wait(); err != nil {
		t.Logf("Async import run exited with error (expected): %v", err)
	} else {
		t.Logf("Async import run completed unexpectedly")
	}

	importCmd = testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--cdc-partitioning-strategy", "pk",
		"--yes",
	}, func() {
		time.Sleep(15 * time.Second)
	}, false)
	err = importCmd.Run()

	assert.True(t, strings.Contains(importCmd.Stderr(), "changing the cdc partitioning strategy is not allowed after the import data has started. Current strategy: auto, new strategy: pk"))

	metaDB, err = metadb.NewMetaDB(exportDir)
	testutils.FatalIfError(t, err, "Failed to initialize meta db")

	//check if the cdc partitioning strategy is auto after the first import
	importDataStatus, err := metaDB.GetImportDataStatusRecord()
	testutils.FatalIfError(t, err, "Failed to get import data status record")
	assert.Equal(t, importDataStatus.CdcPartitioningStrategyConfig, "auto")

	err = testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--cdc-partitioning-strategy", "pk",
		"--start-clean", "true",
		"--truncate-tables", "true",
		"--yes",
	}, func() {
		time.Sleep(15 * time.Second)
	}, true).Run()

	testutils.FatalIfError(t, err, "Import command failed")

	importDataStatus, err = metaDB.GetImportDataStatusRecord()
	testutils.FatalIfError(t, err, "Failed to get import data status record")
	assert.Equal(t, importDataStatus.CdcPartitioningStrategyConfig, PARTITION_BY_PK)

	// Perform cutover
	err = testutils.NewVoyagerCommandRunner(nil, "initiate cutover to target", []string{
		"--export-dir", exportDir,
		"--yes",
		"--prepare-for-fall-back", "false",
	}, nil, false).Run()
	testutils.FatalIfError(t, err, "Cutover command failed")

}

func TestLiveMigrationWithUniqueKeyValuesWithPartialPredicateConflictDetectionCases(t *testing.T) {
	ctx := context.Background()

	// Create a temporary export directory.
	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	createSchemaSQL := `CREATE SCHEMA IF NOT EXISTS test_schema;`
	createTableSQL := `
CREATE TABLE test_schema.test_live (
	id int PRIMARY KEY,
	name TEXT,
	check_id int,
	most_recent boolean,
	description TEXT
);`
	uniqueIndexDDL := `CREATE UNIQUE INDEX idx_test_live_id_check_id ON test_schema.test_live (check_id) WHERE most_recent;`
	insertDataSQL := `
INSERT INTO test_schema.test_live (id, name, check_id, most_recent, description)
SELECT
	i,
	md5(random()::text),                                      -- name
    i,                                                     -- check_id
	i%2=0,                                                     -- most_recent
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 20) as i;`
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
		"ALTER TABLE test_schema.test_live REPLICA IDENTITY FULL;",
		uniqueIndexDDL,
		insertDataSQL,
	}...)

	yugabytedbContainer.ExecuteSqls([]string{
		createSchemaSQL,
		createTableSQL,
		uniqueIndexDDL,
	}...)

	defer postgresContainer.ExecuteSqls(dropSchemaSQL)
	defer yugabytedbContainer.ExecuteSqls(dropSchemaSQL)

	err := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--source-db-schema", "test_schema",
		"--disable-pb", "true",
		"--export-type", SNAPSHOT_AND_CHANGES,
		"--yes",
	}, func() {
		time.Sleep(5 * time.Second) // Wait for the export to start
	}, true).Run()
	testutils.FatalIfError(t, err, "Export command failed")

	importCmd := testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--yes",
	}, nil, true)
	err = importCmd.Run()
	testutils.FatalIfError(t, err, "Import command failed")

	time.Sleep(5 * time.Second)

	ok := utils.RetryWorkWithTimeout(1, 30, func() bool {
		return snapshotPhaseCompleted(t, postgresContainer.GetConfig().Password,
			yugabytedbContainer.GetConfig().Password, 20, `test_schema."test_live"`)
	})
	assert.True(t, ok)
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

	//streaming events 10000 events
	postgresContainer.ExecuteSqls([]string{
		/*
			conflict events
			1 1 t
			...
			20 20 t
			i=21
			UI conflict
			U 20 20 t->f
			I 21 20 true

			UU conflict
			U 21 20 t->f
			U 20 20 f->t

			DU conflict
			D 20 20 t
			U 21 20 f->t

			DI conflict
			D 21 20 t
			I 20 20 true

			//set the required values back as first UI confict
			U 20 20 t->f
			I 21 20 true


			i=22
			U 21 20 t->f
			I 22 20 true
			..so on since the check_id is same for all the events it will be conflict with each other
		*/
		`DO $$
DECLARE
    i INTEGER;
BEGIN
    FOR i IN 21..520 LOOP
        UPDATE test_schema.test_live SET most_recent = false WHERE id = i - 1;
        INSERT INTO test_schema.test_live(id, name, check_id, most_recent, description) VALUES (i, md5(random()::text), 20, true, repeat(md5(random()::text), 10));

		UPDATE test_schema.test_live SET most_recent = false WHERE id = i;
		UPDATE test_schema.test_live SET most_recent = true WHERE id = i - 1;

		DELETE FROM test_schema.test_live WHERE id = i-1;
		UPDATE test_schema.test_live SET most_recent = true WHERE id = i;

		DELETE FROM test_schema.test_live WHERE id = i;
		INSERT INTO test_schema.test_live(id, name, check_id, most_recent, description) VALUES (i-1, md5(random()::text), 20, true, repeat(md5(random()::text), 10));

		UPDATE test_schema.test_live SET most_recent = false WHERE id = i-1;
		INSERT INTO test_schema.test_live(id, name, check_id, most_recent, description) VALUES (i, md5(random()::text), 20, true, repeat(md5(random()::text), 10));
    END LOOP;
END $$;`,
	}...)
	ok = utils.RetryWorkWithTimeout(5, 100, func() bool {
		return streamingPhaseCompleted(t, postgresContainer.GetConfig().Password,
			yugabytedbContainer.GetConfig().Password, 1500, 2500, 1000, `test_schema."test_live"`)
	})
	assert.True(t, ok)

	// Compare the full table data between Postgres and YugabyteDB for streaming part.
	if err := testutils.CompareTableData(ctx, pgConn, ybConn, "test_schema.test_live", "id"); err != nil {
		t.Errorf("Table data mismatch between Postgres and YugabyteDB after streaming: %v", err)
	}
	// Perform cutover
	err = testutils.NewVoyagerCommandRunner(nil, "initiate cutover to target", []string{
		"--export-dir", exportDir,
		"--yes",
		"--prepare-for-fall-back", "false",
	}, nil, false).Run()
	testutils.FatalIfError(t, err, "Cutover command failed")

}

func TestLiveMigrationWithUniqueKeyConflictWithNullValuesDetectionCases(t *testing.T) {
	ctx := context.Background()

	// Create a temporary export directory.
	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	createSchemaSQL := `CREATE SCHEMA IF NOT EXISTS test_schema;`

	//check_id_null_unique should be UNIQUE NOT NULLS DISTINCT
	createTableWithNULLUniqueValuesSql := `
CREATE TABLE test_schema.test_live_null_unique_values (
	id int PRIMARY KEY,
	name TEXT,
	check_id int UNIQUE,
	check_id_null_unique int UNIQUE NULLS NOT DISTINCT
);`

	insertDataWithNULLUniqueValuesSQL := `
INSERT INTO test_schema.test_live_null_unique_values (id, name, check_id, check_id_null_unique)
SELECT
	i,
	md5(random()::text),                                   -- name
    CASE WHEN i%2=0 THEN i ELSE NULL END,                  -- check_id
    i                                                 -- check_id_null_unique
FROM generate_series(1, 20) as i;`

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
		createTableWithNULLUniqueValuesSql,
		"ALTER TABLE test_schema.test_live_null_unique_values REPLICA IDENTITY FULL;",
		insertDataWithNULLUniqueValuesSQL,
	}...)

	yugabytedbContainer.ExecuteSqls([]string{
		createSchemaSQL,
		createTableWithNULLUniqueValuesSql,
	}...)

	defer postgresContainer.ExecuteSqls(dropSchemaSQL)
	defer yugabytedbContainer.ExecuteSqls(dropSchemaSQL)

	err := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--source-db-schema", "test_schema",
		"--disable-pb", "true",
		"--export-type", SNAPSHOT_AND_CHANGES,
		"--yes",
	}, func() {
		time.Sleep(5 * time.Second) // Wait for the export to start
	}, true).Run()
	testutils.FatalIfError(t, err, "Export command failed")

	importCmd := testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--yes",
	}, nil, true)
	err = importCmd.Run()
	testutils.FatalIfError(t, err, "Import command failed")

	time.Sleep(5 * time.Second)

	ok := utils.RetryWorkWithTimeout(1, 30, func() bool {
		return snapshotPhaseCompleted(t, postgresContainer.GetConfig().Password,
			yugabytedbContainer.GetConfig().Password, 20, `test_schema."test_live_null_unique_values"`)
	})
	assert.True(t, ok)
	// Connect to both Postgres and YugabyteDB.
	pgConn, err := postgresContainer.GetConnection()
	testutils.FatalIfError(t, err, "connecting to Postgres")

	ybConn, err := yugabytedbContainer.GetConnection()
	testutils.FatalIfError(t, err, "Error connecting to YugabyteDB")

	// Compare the full table data between Postgres and YugabyteDB for snapshot part.
	// We assume the table "test_data" has a primary key "id" so we order by it.
	if err := testutils.CompareTableData(ctx, pgConn, ybConn, "test_schema.test_live_null_unique_values", "id"); err != nil {
		t.Errorf("Table data mismatch between Postgres and YugabyteDB: %v", err)
	}

	//streaming events 10000 events
	postgresContainer.ExecuteSqls([]string{
		/*
			The below test covering  the null cases
			1  NULL 1
			2  2 2
			...

			i=21
			UI conflict
			U 20 20 20->NULL
			I 21 NULL 20

			UU conflict
			U 20 20 NULL->20
			U 21 NULL 20->NULL

			DU conflict
			D 20 20 20
			U 21 NULL NULL->20

			U 21 NULL 20->NULL

			DI conflict
			D 21 NULL NULL
			I 20 20 NULL

			U 20 20 NULL->20
			I 21 NULL 21
		*/
		`DO $$
DECLARE	
    i INTEGER;
BEGIN
    FOR i IN 21..520 LOOP
        UPDATE test_schema.test_live_null_unique_values SET check_id_null_unique = NULL WHERE id = i - 1;
		INSERT INTO test_schema.test_live_null_unique_values(id, name, check_id, check_id_null_unique) 
		SELECT i, md5(random()::text), CASE WHEN i%2=0 THEN i ELSE NULL END, i-1 ;

		UPDATE test_schema.test_live_null_unique_values SET check_id_null_unique = i WHERE id = i - 1;
		UPDATE test_schema.test_live_null_unique_values SET check_id_null_unique = NULL WHERE id = i;

		DELETE FROM test_schema.test_live_null_unique_values WHERE id = i-1;
		UPDATE test_schema.test_live_null_unique_values SET check_id_null_unique = i-1 WHERE id = i;

		UPDATE test_schema.test_live_null_unique_values SET check_id_null_unique = NULL WHERE id = i;
		
		DELETE FROM test_schema.test_live_null_unique_values WHERE id = i;
		INSERT INTO test_schema.test_live_null_unique_values(id, name, check_id, check_id_null_unique) 
		SELECT i-1, md5(random()::text), CASE WHEN (i-1)%2=0 THEN i-1 ELSE NULL END, NULL;


		UPDATE test_schema.test_live_null_unique_values SET check_id_null_unique = i-1 WHERE id = i - 1;
		INSERT INTO test_schema.test_live_null_unique_values(id, name, check_id, check_id_null_unique)
		SELECT i, md5(random()::text), CASE WHEN i%2=0 THEN i ELSE NULL END, i;

    END LOOP;
END $$;`,
	}...)
	ok = utils.RetryWorkWithTimeout(5, 120, func() bool {
		return streamingPhaseCompleted(t, postgresContainer.GetConfig().Password,
			yugabytedbContainer.GetConfig().Password, 1500, 3000, 1000, `test_schema."test_live_null_unique_values"`)
	})
	assert.True(t, ok)

	// Compare the full table data between Postgres and YugabyteDB for streaming part.
	if err := testutils.CompareTableData(ctx, pgConn, ybConn, "test_schema.test_live_null_unique_values", "id"); err != nil {
		t.Errorf("Table data mismatch between Postgres and YugabyteDB after streaming: %v", err)
	}

	// Perform cutover
	err = testutils.NewVoyagerCommandRunner(nil, "initiate cutover to target", []string{
		"--export-dir", exportDir,
		"--yes",
		"--prepare-for-fall-back", "false",
	}, nil, false).Run()
	testutils.FatalIfError(t, err, "Cutover command failed")

}

func TestLiveMigrationWithUniqueKeyConflictWithExpressionIndexOnPartitions(t *testing.T) {
	ctx := context.Background()

	// Create a temporary export directory.
	exportDir = testutils.CreateTempExportDir()
	// defer testutils.RemoveTempExportDir(exportDir)

	createSchemaSQL := `CREATE SCHEMA IF NOT EXISTS test_schema;`

	// Create partitioned table with multiple columns
	createTableSQL := `
	CREATE TABLE test_schema.test_partitions(
		id int,
		region text,
		created_at date,
		email text,
		username text,
		status text,
		PRIMARY KEY(id, region)
	) PARTITION BY LIST (region);`

	// Create multiple partitions
	partitionTableSQL1 := `CREATE TABLE test_schema.test_partitions_l PARTITION OF test_schema.test_partitions FOR VALUES IN ('London');`
	partitionTableSQL2 := `CREATE TABLE test_schema.test_partitions_s PARTITION OF test_schema.test_partitions FOR VALUES IN ('Sydney');`
	partitionTableSQL3 := `CREATE TABLE test_schema.test_partitions_b PARTITION OF test_schema.test_partitions FOR VALUES IN ('Boston');`
	partitionTableSQL4 := `CREATE TABLE test_schema.test_partitions_t PARTITION OF test_schema.test_partitions FOR VALUES IN ('Tokyo');`

	// Create expression unique index ONLY on specific leaf partitions (London and Sydney)
	// This index is NOT created on the parent table, only on individual partitions
	uniqueIndexSQL1 := `CREATE UNIQUE INDEX idx_test_partitions_email_l ON test_schema.test_partitions_l (lower(email));`
	uniqueIndexSQL2 := `CREATE UNIQUE INDEX idx_test_partitions_email_s ON test_schema.test_partitions_s (lower(email));`
	// Note: Boston and Tokyo partitions do NOT have this unique index

	// Optional: Create a different expression index on another partition for variety
	uniqueIndexSQL3 := `CREATE UNIQUE INDEX idx_test_expression_index_partitions_username_t ON test_schema.test_partitions_t (upper(username));`

	insertDataSQL := `INSERT INTO test_schema.test_partitions (id, region, email, username, created_at, status)
	SELECT i, 
		CASE 
			WHEN i%4 = 0 THEN 'London'
			WHEN i%4 = 1 THEN 'Sydney'
			WHEN i%4 = 2 THEN 'Boston'
			ELSE 'Tokyo'
		END,
		'email_' || i || '@example.com',
		'user_' || i,
		now() + (i || ' days')::interval,
		CASE WHEN i%2 = 0 THEN 'active' ELSE 'inactive' END
	FROM generate_series(1, 20) as i;`

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
		partitionTableSQL1,
		partitionTableSQL2,
		partitionTableSQL3,
		partitionTableSQL4,
		uniqueIndexSQL1,
		uniqueIndexSQL2,
		uniqueIndexSQL3,
		insertDataSQL,
		"ALTER TABLE test_schema.test_partitions REPLICA IDENTITY FULL;",
		"ALTER TABLE test_schema.test_partitions_l REPLICA IDENTITY FULL;",
		"ALTER TABLE test_schema.test_partitions_s REPLICA IDENTITY FULL;",
		"ALTER TABLE test_schema.test_partitions_b REPLICA IDENTITY FULL;",
		"ALTER TABLE test_schema.test_partitions_t REPLICA IDENTITY FULL;",
	}...)

	yugabytedbContainer.ExecuteSqls([]string{
		createSchemaSQL,
		createTableSQL,
		partitionTableSQL1,
		partitionTableSQL2,
		partitionTableSQL3,
		partitionTableSQL4,
		uniqueIndexSQL1,
		uniqueIndexSQL2,
		uniqueIndexSQL3,
	}...)

	defer postgresContainer.ExecuteSqls(dropSchemaSQL)
	defer yugabytedbContainer.ExecuteSqls(dropSchemaSQL)

	err := testutils.NewVoyagerCommandRunner(postgresContainer, "export data", []string{
		"--export-dir", exportDir,
		"--source-db-schema", "test_schema",
		"--disable-pb", "true",
		"--export-type", SNAPSHOT_AND_CHANGES,
		"--yes",
	}, func() {
		time.Sleep(5 * time.Second) // Wait for the export to start
	}, true).Run()
	testutils.FatalIfError(t, err, "Export command failed")

	importCmd := testutils.NewVoyagerCommandRunner(yugabytedbContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--yes",
	}, nil, true)
	err = importCmd.Run()
	testutils.FatalIfError(t, err, "Import command failed")

	time.Sleep(5 * time.Second)

	ok := utils.RetryWorkWithTimeout(1, 30, func() bool {
		return snapshotPhaseCompleted(t, postgresContainer.GetConfig().Password,
			yugabytedbContainer.GetConfig().Password, 20, `test_schema."test_partitions"`)
	})
	assert.True(t, ok)
	// Connect to both Postgres and YugabyteDB.
	pgConn, err := postgresContainer.GetConnection()
	testutils.FatalIfError(t, err, "connecting to Postgres")

	ybConn, err := yugabytedbContainer.GetConnection()
	testutils.FatalIfError(t, err, "Error connecting to YugabyteDB")

	// Compare the full table data between Postgres and YugabyteDB for snapshot part.
	// We assume the table "test_data" has a primary key "id" so we order by it.
	if err := testutils.CompareTableData(ctx, pgConn, ybConn, "test_schema.test_partitions", "id"); err != nil {
		t.Errorf("Table data mismatch between Postgres and YugabyteDB: %v", err)
	}

	//streaming events 10000 events
	postgresContainer.ExecuteSqls([]string{
		/*
		1  Sydney email_1@example.com user_1 2021-01-01 active
		2  Boston email_2@example.com user_2 2021-01-02 active
		...
		20 London email_20@example.com user_20 2021-01-20 active


		changes
		UI
		U 20 email_20@example.com -> Email_21@example.com
		I 21 email_20@example.com user_21 2021-01-21 active
		
		UU
		U 21 email_20@example.com -> Email_521@example.com
		U 20 Email_21@example.com -> Email_20@example.com

		DU
		D 20 Email_20@example.com
		U 21 Email_521@example.com -> email_20@example.com

		DI
		D 21 email_20@example.com
		I 20 Email_20@example.com user_20 2021-01-20 active

		U 20 email_20@example.com -> Email_21@example.com
		I 21 email_20@example.com user_21 2021-01-21 active

		*/
		`DO $$
DECLARE
    i INTEGER;
BEGIN
    FOR i IN 21..520 LOOP
        UPDATE test_schema.test_partitions SET email = 'Email_' || i || '@example.com' WHERE id = i - 1;
		INSERT INTO test_schema.test_partitions(id, region, email, username, created_at, status) VALUES 
		(i, 'Sydney', 'email_' || 20 || '@example.com', 'user_' || i, now() + (i || ' days')::interval, 'active');

		UPDATE test_schema.test_partitions SET email = 'Email_' || 500+i || '@example.com' WHERE id = i;
		UPDATE test_schema.test_partitions SET email = 'Email_' || 20 || '@example.com' WHERE id = i - 1;

		DELETE FROM test_schema.test_partitions WHERE id = i-1;
		UPDATE test_schema.test_partitions SET email = 'email_' || 20 || '@example.com' WHERE id = i;

		DELETE FROM test_schema.test_partitions WHERE id = i;
		INSERT INTO test_schema.test_partitions(id, region, email, username, created_at, status) VALUES 
		(i-1, 'London', 'Email_' || 20 || '@example.com', 'user_' || i-1, now() + ((i-1) || ' days')::interval, 'active');

		UPDATE test_schema.test_partitions SET email = 'Email_' || i || '@example.com' WHERE id = i - 1;
		INSERT INTO test_schema.test_partitions(id, region, email, username, created_at, status) VALUES 
		(i, 'Sydney', 'email_' || 20 || '@example.com', 'user_' || i, now() + (i || ' days')::interval, 'active');

    END LOOP;
END $$;`,
	}...)

	ok = utils.RetryWorkWithTimeout(5, 120, func() bool {
		return streamingPhaseCompleted(t, postgresContainer.GetConfig().Password,
			yugabytedbContainer.GetConfig().Password, 1500, 2500, 1000, `test_schema."test_partitions"`)
	})

	assert.True(t, ok)

	// Compare the full table data between Postgres and YugabyteDB for streaming part.
	if err := testutils.CompareTableData(ctx, pgConn, ybConn, "test_schema.test_partitions", "id"); err != nil {
		t.Errorf("Table data mismatch between Postgres and YugabyteDB after streaming: %v", err)
	}

	// Perform cutover
	err = testutils.NewVoyagerCommandRunner(nil, "initiate cutover to target", []string{
		"--export-dir", exportDir,
		"--yes",
		"--prepare-for-fall-back", "false",
	}, nil, false).Run()
	testutils.FatalIfError(t, err, "Cutover command failed")

}
