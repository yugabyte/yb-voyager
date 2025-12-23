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
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"

	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

////=========================================

// This inserts some rows in target table having sequence and validates if the ids ingested are correct or not
func assertSequenceValues(t *testing.T, startID int, endId int, ybConn *sql.DB, tableName string) error {
	_, err := ybConn.Exec(fmt.Sprintf(`INSERT INTO test_schema.test_live (name, email, description)
SELECT
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(%d, %d);`, startID, endId))
	if err != nil {
		return fmt.Errorf("failed to insert into target: %w", err)
	}

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
	return nil
}

// Basic Test for live migration with cutover
// cutover -> validate sequence restoration
//
//export data -> import data (streaming for some events) -> once all data is streamed to target
func TestBasicLiveMigrationWithCutover(t *testing.T) {

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:    "postgresql",
			ForLive: true,
		},
		TargetDB: ContainerConfig{
			Type: "yugabytedb",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.test_live (
				id SERIAL PRIMARY KEY,
				name TEXT,
				email TEXT,
				description TEXT
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.test_live REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.test_live (name, email, description)
SELECT
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 10);`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema.test_live (name, email, description)
SELECT
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 5);`,
		},
		CleanupSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema CASCADE;`,
		},
	})

	defer lm.Cleanup()

	err := lm.SetupContainers(context.Background())
	testutils.FatalIfError(t, err, "failed to setup containers")

	err = lm.SetupSchema()
	testutils.FatalIfError(t, err, "failed to setup schema")

	err = lm.StartExportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start export data")

	err = lm.StartImportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start import data")

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`test_schema."test_live"`: 10,
	}, 30)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	//validate snapshot data
	err = lm.ValidateDataConsistency([]string{`test_schema."test_live"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	//execute source delta
	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`test_schema."test_live"`: {
			Inserts: 5,
			Updates: 0,
			Deletes: 0,
		},
	}, 30, 1)
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	//validate streaming data
	err = lm.ValidateDataConsistency([]string{`test_schema."test_live"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = lm.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")

	err = lm.WaitForCutoverComplete(50)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

	//validate sequence restoration
	err = lm.WithTargetConn(func(target *sql.DB) error {
		return assertSequenceValues(t, 16, 25, target, `test_schema.test_live`)
	})
	testutils.FatalIfError(t, err, "failed to validate sequence restoration")

}

func TestBasicLiveMigrationWithFallback(t *testing.T) {

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:    "postgresql",
			ForLive: true,
		},
		TargetDB: ContainerConfig{
			Type: "yugabytedb",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema CASCADE;`,
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.test_live (
				id SERIAL PRIMARY KEY,
				name TEXT,
				email TEXT,
				description TEXT
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.test_live REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.test_live (name, email, description)
SELECT
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 10);`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema.test_live (name, email, description)
SELECT
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 5);`,
		},
		CleanupSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema CASCADE;`,
		},
		TargetDeltaSQL: []string{
			`INSERT INTO test_schema.test_live (name, email, description)
SELECT
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 5);`,
		},
	})

	defer lm.Cleanup()

	err := lm.SetupContainers(context.Background())
	testutils.FatalIfError(t, err, "failed to setup containers")

	err = lm.SetupSchema()
	testutils.FatalIfError(t, err, "failed to setup schema")

	// run select from yb_servers() and print output
	err = lm.WithTargetConn(func(target *sql.DB) error {
		rows, err := target.Query(`SELECT host, port, num_connections, node_type, cloud, region, zone, public_ip FROM yb_servers();`)
		if err != nil {
			return fmt.Errorf("failed to query yb_servers: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var host, nodeType, cloud, region, zone, publicIP string
			var port, numConnections int
			err := rows.Scan(&host, &port, &numConnections, &nodeType, &cloud, &region, &zone, &publicIP)
			if err != nil {
				return fmt.Errorf("failed to scan row: %w", err)
			}
			fmt.Printf("host=%s port=%d num_connections=%d node_type=%s cloud=%s region=%s zone=%s public_ip=%s\n",
				host, port, numConnections, nodeType, cloud, region, zone, publicIP)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("failed to get rows: %w", err)
		}
		return nil
	})
	testutils.FatalIfError(t, err, "failed to query yb_servers and print output")

	err = lm.StartExportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start export data")

	err = lm.StartImportData(true, map[string]string{
		"--log-level": "debug",
	})
	testutils.FatalIfError(t, err, "failed to start import data")

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`test_schema."test_live"`: 10,
	}, 30)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	//validate snapshot data
	err = lm.ValidateDataConsistency([]string{`test_schema."test_live"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	//execute source delta
	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`test_schema."test_live"`: {
			Inserts: 5,
			Updates: 0,
			Deletes: 0,
		},
	}, 30, 1)
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	//validate streaming data
	err = lm.ValidateDataConsistency([]string{`test_schema."test_live"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	//RIGHT NOW not using the fallback true
	err = lm.InitiateCutoverToTarget(true, nil)
	// err = lm.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")

	err = lm.WaitForCutoverComplete(50)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

	//validate sequence restoration
	// err = lm.WithTargetConn(func(target *sql.DB) error {
	// 	return assertSequenceValues(t, 16, 25, target, `test_schema.test_live`)
	// })
	// testutils.FatalIfError(t, err, "failed to validate sequence restoration")

	err = lm.ExecuteTargetDelta()
	testutils.FatalIfError(t, err, "failed to execute target delta")

	err = lm.WaitForFallbackStreamingComplete(map[string]ChangesCount{
		`test_schema."test_live"`: {
			Inserts: 5,
			Updates: 0,
			Deletes: 0,
		},
	}, 30, 1)
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	//validate streaming data
	err = lm.ValidateDataConsistency([]string{`test_schema."test_live"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = lm.InitiateCutoverToSource(nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to source")

	err = lm.WaitForCutoverSourceComplete(100)
	testutils.FatalIfError(t, err, "failed to wait for cutover to source complete")

}

// test for live migration with resumption and failure during restore sequences
// cutover -> drop sequence on target -> start import again (validate its failing at restore sequences)
// create sequence back on target -> re-run import again
// validate sequence by inserting data
//
//export data -> import data (streaming some data) -> once done kill import
func TestLiveMigrationWithImportResumptionOnFailureAtRestoreSequences(t *testing.T) {

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:    "postgresql",
			ForLive: true,
		},
		TargetDB: ContainerConfig{
			Type: "yugabytedb",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.test_live (
				id SERIAL PRIMARY KEY,
				name TEXT,
				email TEXT,
				description TEXT
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.test_live REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.test_live (name, email, description)
SELECT
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 20);`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema.test_live (name, email, description)
SELECT
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 15);`,
		},
		CleanupSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema CASCADE;`,
		},
	})
	defer lm.Cleanup()

	err := lm.SetupContainers(context.Background())
	testutils.FatalIfError(t, err, "failed to setup containers")

	err = lm.SetupSchema()
	testutils.FatalIfError(t, err, "failed to setup schema")

	err = lm.StartExportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start export data")

	err = lm.StartImportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start import data")

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`test_schema."test_live"`: 20,
	}, 30)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	err = lm.ValidateDataConsistency([]string{`test_schema."test_live"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`test_schema."test_live"`: {
			Inserts: 15,
			Updates: 0,
			Deletes: 0,
		},
	}, 30, 1)
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	err = lm.ValidateDataConsistency([]string{`test_schema."test_live"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = lm.StopImportData()
	testutils.FatalIfError(t, err, "failed to stop import data")

	err = lm.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")

	//drop sequence on target to simulate the failure at restore sequences during cutover
	err = lm.WithTargetConn(func(target *sql.DB) error {
		_, err := target.Exec(`DROP SEQUENCE test_schema.test_live_id_seq CASCADE;`)
		if err != nil {
			return fmt.Errorf("failed to drop sequence: %w", err)
		}
		return nil
	})
	testutils.FatalIfError(t, err, "failed to drop sequence")

	time.Sleep(10 * time.Second)

	//Resume import command after deleting a sequence of the table column idand import should fail while restoring sequences as cutover is already triggered
	err = lm.ResumeImportData(false, nil)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(lm.GetImportCommandStderr(), "failed to restore sequences:"))

	//Create sequence back on yb to resume import and finish cutover
	err = lm.WithTargetConn(func(target *sql.DB) error {
		statements := []string{
			`CREATE SEQUENCE test_schema.test_live_id_seq;`,
			`ALTER SEQUENCE test_schema.test_live_id_seq OWNED BY test_schema.test_live.id;`,
			`ALTER TABLE test_schema.test_live ALTER COLUMN id SET DEFAULT nextval('test_schema.test_live_id_seq');`,
		}
		for _, statement := range statements {
			_, err := target.Exec(statement)
			if err != nil {
				return fmt.Errorf("failed to execute statement: %w", err)
			}
		}
		return nil
	})
	testutils.FatalIfError(t, err, "failed to create sequence")

	//Resume import command after deleting a sequence of the table column idand import should pass while restoring sequences as cutover is already triggered
	err = lm.ResumeImportData(true, nil)
	testutils.FatalIfError(t, err, "failed to resume import data")

	err = lm.WaitForCutoverComplete(30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

	//Check if ids from 36-45 are present in target this is to verify the sequence serial col is restored properly till last value
	err = lm.WithTargetConn(func(target *sql.DB) error {
		return assertSequenceValues(t, 36, 45, target, `test_schema.test_live`)
	})
	testutils.FatalIfError(t, err, "failed to validate sequence restoration")

}

// test live migration with import resumption with  generated always schema
// cutover -> start import again
// validate ALWAYS type on the target
//
//export data -> import data (streaming some data) -> once done kill import
func TestLiveMigrationWithImportResumptionWithGeneratedAlwaysColumn(t *testing.T) {
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:    "postgresql",
			ForLive: true,
		},
		TargetDB: ContainerConfig{
			Type: "yugabytedb",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.test_live (
				id int GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
				name TEXT,
				email TEXT,
				description TEXT
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.test_live REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.test_live (name, email, description)
SELECT
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 20);`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema.test_live (name, email, description)
SELECT
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 15);`,
		},
		CleanupSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema CASCADE;`,
		},
	})
	defer lm.Cleanup()

	err := lm.SetupContainers(context.Background())
	testutils.FatalIfError(t, err, "failed to setup containers")

	err = lm.SetupSchema()
	testutils.FatalIfError(t, err, "failed to setup schema")

	err = lm.StartExportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start export data")

	err = lm.StartImportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start import data")

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`test_schema."test_live"`: 20,
	}, 30)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	err = lm.ValidateDataConsistency([]string{`test_schema."test_live"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`test_schema."test_live"`: {
			Inserts: 15,
			Updates: 0,
			Deletes: 0,
		},
	}, 30, 1)
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	err = lm.ValidateDataConsistency([]string{`test_schema."test_live"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = lm.StopImportData()
	testutils.FatalIfError(t, err, "failed to stop import data")

	// Perform cutover
	err = lm.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")

	err = lm.ResumeImportData(true, nil)
	testutils.FatalIfError(t, err, "failed to resume import data")

	err = lm.WaitForCutoverComplete(30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

	err = lm.WithTargetConn(func(target *sql.DB) error {
		//Check if always is restored back
		query := fmt.Sprintf(`SELECT column_name FROM information_schema.columns where table_schema='test_schema' AND
		table_name='test_live' AND is_identity='YES' AND identity_generation='ALWAYS'`)

		var col string
		err = target.QueryRow(query).Scan(&col)
		testutils.FatalIfError(t, err, "error checking if table has always or not")
		assert.Equal(t, col, "id")
		return nil
	})
	testutils.FatalIfError(t, err, "failed to validate generated always column")

}

func TestLiveMigrationResumptionWithChangeInCDCPartitioningStrategy(t *testing.T) {
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:    "postgresql",
			ForLive: true,
		},
		TargetDB: ContainerConfig{
			Type: "yugabytedb",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.test_live (
				id SERIAL PRIMARY KEY,
				name TEXT,
				email TEXT,
				description TEXT
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.test_live REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.test_live (name, email, description)
SELECT
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 10);`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema.test_live (name, email, description)
SELECT
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 10);`,
		},
		CleanupSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema CASCADE;`,
		},
	})
	defer lm.Cleanup()

	err := lm.SetupContainers(context.Background())
	testutils.FatalIfError(t, err, "failed to setup containers")

	err = lm.SetupSchema()
	testutils.FatalIfError(t, err, "failed to setup schema")

	err = lm.StartExportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start export data")

	err = lm.StartImportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start import data")

	err = lm.StopImportData()
	testutils.FatalIfError(t, err, "failed to stop import data")

	err = lm.ResumeImportData(false, map[string]string{
		"--cdc-partitioning-strategy": "pk",
	})

	assert.True(t, strings.Contains(lm.GetImportCommandStderr(), "changing the cdc partitioning strategy is not allowed after the import data has started. Current strategy: auto, new strategy: pk"))

	err = lm.InitMetaDB()
	testutils.FatalIfError(t, err, "failed to initialize meta db")
	metaDB = lm.metaDB

	//check if the cdc partitioning strategy is auto after the first import
	importDataStatus, err := metaDB.GetImportDataStatusRecord()
	testutils.FatalIfError(t, err, "Failed to get import data status record")
	assert.Equal(t, importDataStatus.CdcPartitioningStrategyConfig, "auto")

	err = lm.ResumeImportData(true, map[string]string{
		"--cdc-partitioning-strategy": "pk",
		"--start-clean":               "true",
		"--truncate-tables":           "true",
	})
	testutils.FatalIfError(t, err, "failed to resume import data")

	importDataStatus, err = metaDB.GetImportDataStatusRecord()
	testutils.FatalIfError(t, err, "Failed to get import data status record")
	assert.Equal(t, importDataStatus.CdcPartitioningStrategyConfig, PARTITION_BY_PK)

	// Perform cutover
	err = lm.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")

}

func TestLiveMigrationWithUniqueKeyValuesWithPartialPredicateConflictDetectionCases(t *testing.T) {
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:    "postgresql",
			ForLive: true,
		},
		TargetDB: ContainerConfig{
			Type: "yugabytedb",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.test_live (
				id int PRIMARY KEY,
				name TEXT,
				check_id int,
				most_recent boolean,
				description TEXT
			);
			CREATE UNIQUE INDEX idx_test_live_id_check_id ON test_schema.test_live (check_id) WHERE most_recent;
			`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.test_live REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.test_live (id, name, check_id, most_recent, description)
SELECT
	i,
	md5(random()::text),                                      -- name
    i,                                                     -- check_id
	i%2=0,                                                     -- most_recent
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 20) as i;`,
		},
		SourceDeltaSQL: []string{
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
		},
		CleanupSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema CASCADE;`,
		},
	})
	defer lm.Cleanup()

	err := lm.SetupContainers(context.Background())
	testutils.FatalIfError(t, err, "failed to setup containers")

	err = lm.SetupSchema()
	testutils.FatalIfError(t, err, "failed to setup schema")

	err = lm.StartExportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start export data")

	err = lm.StartImportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start import data")

	time.Sleep(5 * time.Second)

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`test_schema."test_live"`: 20,
	}, 30)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	err = lm.ValidateDataConsistency([]string{`test_schema."test_live"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`test_schema."test_live"`: {
			Inserts: 1500,
			Updates: 2500,
			Deletes: 1000,
		},
	}, 100, 5)
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	err = lm.ValidateDataConsistency([]string{`test_schema."test_live"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = lm.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")

}

func TestLiveMigrationWithUniqueKeyConflictWithNullValuesDetectionCases(t *testing.T) {
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:    "postgresql",
			ForLive: true,
		},
		TargetDB: ContainerConfig{
			Type: "yugabytedb",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.test_live_null_unique_values (
				id int PRIMARY KEY,
				name TEXT,
				check_id int UNIQUE,
				check_id_null_unique int UNIQUE NULLS NOT DISTINCT
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.test_live_null_unique_values REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.test_live_null_unique_values (id, name, check_id, check_id_null_unique)
SELECT
	i,
	md5(random()::text),                                   -- name
    CASE WHEN i%2=0 THEN i ELSE NULL END,                  -- check_id
    i                                                 -- check_id_null_unique
FROM generate_series(1, 20) as i;`,
		},
		SourceDeltaSQL: []string{
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
		},
		CleanupSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema CASCADE;`,
		},
	})
	defer lm.Cleanup()

	err := lm.SetupContainers(context.Background())
	testutils.FatalIfError(t, err, "failed to setup containers")

	err = lm.SetupSchema()
	testutils.FatalIfError(t, err, "failed to setup schema")

	err = lm.StartExportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start export data")

	err = lm.StartImportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start import data")

	time.Sleep(5 * time.Second)

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`test_schema."test_live_null_unique_values"`: 20,
	}, 30)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	err = lm.ValidateDataConsistency([]string{`test_schema."test_live_null_unique_values"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`test_schema."test_live_null_unique_values"`: {
			Inserts: 1500,
			Updates: 3000,
			Deletes: 1000,
		},
	}, 120, 5)
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	err = lm.ValidateDataConsistency([]string{`test_schema."test_live_null_unique_values"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = lm.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")

}

func TestLiveMigrationWithUniqueKeyConflictWithUniqueIndexOnlyOnLeafPartitions(t *testing.T) {
	liveMigrationTest := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:    "postgresql",
			ForLive: true,
		},
		TargetDB: ContainerConfig{
			Type: "yugabytedb",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.test_partitions (
				id int,
				name TEXT,
				region TEXT,
				branch TEXT,
				PRIMARY KEY(id, region)
			) PARTITION BY LIST (region);

			CREATE TABLE test_schema.test_partitions_part1 PARTITION OF test_schema.test_partitions FOR VALUES IN ('London');
			CREATE TABLE test_schema.test_partitions_part2 PARTITION OF test_schema.test_partitions FOR VALUES IN ('Sydney');
			CREATE TABLE test_schema.test_partitions_part3 PARTITION OF test_schema.test_partitions FOR VALUES IN ('Boston');
			CREATE UNIQUE INDEX idx_1 ON test_schema.test_partitions_part1 (branch); -- This is the unique index only on part1
			CREATE UNIQUE INDEX idx_2 ON test_schema.test_partitions_part2 (branch); -- This is the unique index only on part2
			CREATE UNIQUE INDEX idx_3 ON test_schema.test_partitions_part3 (branch); -- This is the unique index only on part3`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.test_partitions (id, name, region, branch)
	SELECT i, md5(random()::text), CASE WHEN i%3=1 THEN 'London' WHEN i%3=2 THEN 'Sydney' ELSE 'Boston' END, 'Branch ' || i FROM generate_series(1, 20) as i;`,
		},
		SourceSetupSchemaSQL: []string{
			"ALTER TABLE test_schema.test_partitions REPLICA IDENTITY FULL;",
			"ALTER TABLE test_schema.test_partitions_part1 REPLICA IDENTITY FULL;",
			"ALTER TABLE test_schema.test_partitions_part2 REPLICA IDENTITY FULL;",
			"ALTER TABLE test_schema.test_partitions_part3 REPLICA IDENTITY FULL;",
		},
		SourceDeltaSQL: []string{
			/*
				conflict events
				1 London Branch1
				2 Sydney Branch2
				3 Boston Branch3
				...
				20 Sydney Branch20
				i=21
				UI conflict
				U 20 Sydney Branch20->Branch 21
				I 21 Boston Branch20

				U 21 Boston Branch20->Branch 521
				UU conflict
				U 20 Sydney Branch21->Branch 20
				U 21 Boston Branch521->Branch 21

				DU conflict
				D 20 Sydney Branch20
				U 21 Boston Branch21->Branch 20

				DI conflict
				D 21 Boston Branch21
				I 20 Sydney Branch 21

				U 20 Sydney Branch21->Branch 20
				I 21 Boston Branch20->Branch 21

				..so on since the branch is same for all the events it will be conflict with each other
			*/
			`
		DO $$
		DECLARE
		i INTEGER;
		BEGIN
			FOR i IN 21..520 LOOP
				UPDATE test_schema.test_partitions SET branch = 'Branch ' || i WHERE id = i - 1;
				INSERT INTO test_schema.test_partitions(id, name, region, branch)
				SELECT i, md5(random()::text), 'London', 'Branch ' || i-1;
		
				UPDATE test_schema.test_partitions SET branch = 'Branch ' || i+500 WHERE id = i;
		
				UPDATE test_schema.test_partitions SET branch = 'Branch ' || i-1 WHERE id = i - 1;
				UPDATE test_schema.test_partitions SET branch = 'Branch ' || i WHERE id = i;
		
				DELETE FROM test_schema.test_partitions WHERE id = i-1;
				UPDATE test_schema.test_partitions SET branch = 'Branch ' || i-1 WHERE id = i;
		
				DELETE FROM test_schema.test_partitions WHERE id = i;
				INSERT INTO test_schema.test_partitions(id, name, region, branch)
				SELECT i-1, md5(random()::text), 'London', 'Branch ' || i;
		
				UPDATE test_schema.test_partitions SET branch = 'Branch ' || i-1 WHERE id = i - 1;
				INSERT INTO test_schema.test_partitions(id, name, region, branch)
				SELECT i, md5(random()::text), 'London', 'Branch ' || i;
		
			END LOOP;
		END $$;`,
		},
		CleanupSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema CASCADE;`,
		},
	})

	defer liveMigrationTest.Cleanup()

	err := liveMigrationTest.SetupContainers(context.Background())
	testutils.FatalIfError(t, err, "failed to setup containers")

	err = liveMigrationTest.SetupSchema()
	testutils.FatalIfError(t, err, "failed to setup schema")

	err = liveMigrationTest.StartExportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start export data")

	err = liveMigrationTest.StartImportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start import data")

	time.Sleep(5 * time.Second)
	err = liveMigrationTest.WaitForSnapshotComplete(map[string]int64{
		`test_schema."test_partitions"`: 20,
	}, 30)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	err = liveMigrationTest.ValidateDataConsistency([]string{`test_schema."test_partitions"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = liveMigrationTest.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	err = liveMigrationTest.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`test_schema."test_partitions"`: {
			Inserts: 1500,
			Updates: 3000,
			Deletes: 1000,
		},
	}, 120, 5)
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	//streaming events 10000 events
	err = liveMigrationTest.ValidateDataConsistency([]string{`test_schema."test_partitions"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	// Perform cutover
	err = liveMigrationTest.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to target")

}

func TestLiveMigrationWithUniqueKeyConflictWithNullValueAndPartialPredicatesDetectionCases(t *testing.T) {
	liveMigrationTest := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:    "postgresql",
			ForLive: true,
		},
		TargetDB: ContainerConfig{
			Type: "yugabytedb",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.test_live_null_partial_unique_values (
				id int PRIMARY KEY,
				name TEXT,
				check_id int,
				most_recent boolean
			);

			CREATE UNIQUE INDEX idx_test_live_null_partial_unique_values_id_check_id ON test_schema.test_live_null_partial_unique_values (check_id) WHERE most_recent;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.test_live_null_partial_unique_values (id, name, check_id, most_recent)
SELECT
	i,
	md5(random()::text),                                   -- name
    CASE WHEN i%2=0 THEN i ELSE NULL END,                  -- check_id
    i%2=0                                                 -- most_recent
FROM generate_series(1, 20) as i;`,
		},
		SourceSetupSchemaSQL: []string{
			"ALTER TABLE test_schema.test_live_null_partial_unique_values REPLICA IDENTITY FULL;",
		},
		SourceDeltaSQL: []string{
			/*
				The below test covering  the null cases
				1  NULL f
				2  2 t
				...

				i=21
				UI conflict
				U 20 20->NULL t->f
				I 21 20 t

				UU conflict
				U 21 20->NULL t
				U 20 NULL->20 f->t

				U 20 20->NULL t
				DU conflict - false positive
				D 21 NULL t
				U 20 NULL->20 t

				I 21 NULL f
				D 20 20 t

				DI conflict - false positive
				D 21 NULL f
				I 20 NULL f

				I 21 20 t
			*/
			`DO $$
		DECLARE	
			i INTEGER;
		BEGIN
			FOR i IN 21..520 LOOP
				UPDATE test_schema.test_live_null_partial_unique_values SET most_recent = false AND check_id = NULL WHERE id = i - 1;
				INSERT INTO test_schema.test_live_null_partial_unique_values(id, name, check_id, most_recent) VALUES (i, md5(random()::text), 20, true);
		
				UPDATE test_schema.test_live_null_partial_unique_values SET check_id = NULL WHERE id = i;
				UPDATE test_schema.test_live_null_partial_unique_values SET most_recent = true WHERE id = i - 1;
		
				UPDATE test_schema.test_live_null_partial_unique_values SET check_id = NULL WHERE id = i-1;
		
				DELETE FROM test_schema.test_live_null_partial_unique_values WHERE id = i;
				UPDATE test_schema.test_live_null_partial_unique_values SET check_id = 20 WHERE id = i-1;
		
				INSERT INTO test_schema.test_live_null_partial_unique_values(id, name, check_id, most_recent) VALUES (i, md5(random()::text), NULL, false);
				DELETE FROM test_schema.test_live_null_partial_unique_values WHERE id = i-1;
		
				DELETE FROM test_schema.test_live_null_partial_unique_values WHERE id = i;
				INSERT INTO test_schema.test_live_null_partial_unique_values(id, name, check_id, most_recent) VALUES (i-1, md5(random()::text), NULL, false);
		
				INSERT INTO test_schema.test_live_null_partial_unique_values(id, name, check_id, most_recent) VALUES (i, md5(random()::text), 20, true);
		
			END LOOP;
		END $$;`,
		},
		CleanupSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema CASCADE;`,
		},
	})

	defer liveMigrationTest.Cleanup()

	err := liveMigrationTest.SetupContainers(context.Background())
	testutils.FatalIfError(t, err, "failed to setup containers")

	err = liveMigrationTest.SetupSchema()
	testutils.FatalIfError(t, err, "failed to setup schema")

	err = liveMigrationTest.StartExportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start export data")

	err = liveMigrationTest.StartImportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start import data")

	time.Sleep(5 * time.Second)

	err = liveMigrationTest.WaitForSnapshotComplete(map[string]int64{
		`test_schema."test_live_null_partial_unique_values"`: 20,
	}, 30)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	err = liveMigrationTest.ValidateDataConsistency([]string{`test_schema."test_live_null_partial_unique_values"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = liveMigrationTest.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	err = liveMigrationTest.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`test_schema."test_live_null_partial_unique_values"`: {
			Inserts: 2000,
			Updates: 2500,
			Deletes: 1500,
		},
	}, 120, 5)
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	err = liveMigrationTest.ValidateDataConsistency([]string{`test_schema."test_live_null_partial_unique_values"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = liveMigrationTest.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to target")

}

func TestLiveMigrationWithUniqueKeyConflictWithExpressionIndexOnPartitions(t *testing.T) {
	liveMigrationTest := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:    "postgresql",
			ForLive: true,
		},
		TargetDB: ContainerConfig{
			Type: "yugabytedb",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.test_partitions(
		id int,
		region text,
		created_at date,
		email text,
		username text,
		status text,
		PRIMARY KEY(id, region)
	) PARTITION BY LIST (region);
	 
	CREATE TABLE test_schema.test_partitions_l PARTITION OF test_schema.test_partitions FOR VALUES IN ('London');
	CREATE TABLE test_schema.test_partitions_s PARTITION OF test_schema.test_partitions FOR VALUES IN ('Sydney');
	CREATE TABLE test_schema.test_partitions_b PARTITION OF test_schema.test_partitions FOR VALUES IN ('Boston');
	CREATE TABLE test_schema.test_partitions_t PARTITION OF test_schema.test_partitions FOR VALUES IN ('Tokyo');
	
	CREATE UNIQUE INDEX idx_test_partitions_email_l ON test_schema.test_partitions_l (lower(email));
	CREATE UNIQUE INDEX idx_test_partitions_email_s ON test_schema.test_partitions_s (lower(email));
	CREATE UNIQUE INDEX idx_test_partitions_email_b ON test_schema.test_partitions_b (lower(email));
	CREATE UNIQUE INDEX idx_test_partitions_email_t ON test_schema.test_partitions_t (lower(email));
	CREATE UNIQUE INDEX idx_test_expression_index_partitions_username_t ON test_schema.test_partitions_t (upper(username));`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.test_partitions REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.test_partitions_l REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.test_partitions_s REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.test_partitions_b REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.test_partitions_t REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.test_partitions (id, region, email, username, created_at, status)
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
	FROM generate_series(1, 20) as i;`,
		},
		SourceDeltaSQL: []string{
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
		},
		CleanupSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema CASCADE;`,
		},
	})

	defer liveMigrationTest.Cleanup()

	err := liveMigrationTest.SetupContainers(context.Background())
	testutils.FatalIfError(t, err, "failed to setup containers")

	err = liveMigrationTest.SetupSchema()
	testutils.FatalIfError(t, err, "failed to setup schema")

	err = liveMigrationTest.StartExportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start export data")

	err = liveMigrationTest.StartImportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start import data")

	time.Sleep(5 * time.Second)

	err = liveMigrationTest.WaitForSnapshotComplete(map[string]int64{
		`test_schema."test_partitions"`: 20,
	}, 30)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	err = liveMigrationTest.ValidateDataConsistency([]string{`test_schema."test_partitions"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = liveMigrationTest.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	err = liveMigrationTest.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`test_schema."test_partitions"`: {
			Inserts: 1500,
			Updates: 2500,
			Deletes: 1000,
		},
	}, 120, 5)
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	err = liveMigrationTest.ValidateDataConsistency([]string{`test_schema."test_partitions"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	// Perform cutover
	err = liveMigrationTest.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to target")

	err = liveMigrationTest.WaitForCutoverComplete(50)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

}
