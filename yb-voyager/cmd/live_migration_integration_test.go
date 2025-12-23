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
	if !assert.Equal(t, ids, resIds) {
		return fmt.Errorf("ids do not match %v != %v", ids, resIds)
	}
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

// TestLiveMigrationWithEventsOnSamePkOrdered tests that INSERT/UPDATE/DELETE events
// for the same primary key are applied in the correct order during live migration.
//
// Coverage Matrix (all meaningful I-U-D ordering transitions):
//
//	| Transition | Table 1 | Table 2 | Table 3 |
//	|------------|---------|---------|---------|
//	| I→U        |         |         | 1000x   |
//	| I→D        | 1000x   |         |         |
//	| U→U        |         | 1000x   |         |
//	| U→D        |         |         | 1000x   |
//	| D→I        | 999x    |         | 1000x   |
//
// Invalid transitions not tested (not meaningful for ordering):
//   - I→I: Impossible (duplicate key error)
//   - U→I: Requires DELETE first (covered by D→I)
//   - D→U: UPDATE on non-existent row (no-op)
//   - D→D: Second DELETE is no-op
//
// Table 1 (test_insert_delete_ordering): Tests I→D and D→I ordering on same PK
//   - Pattern: INSERT(id=1) → DELETE(id=1) → INSERT(id=1) → ... (1000 cycles, skip last DELETE)
//   - If D executes before I: DELETE is no-op (row doesn't exist), I succeeds, next I fails with duplicate key
//   - If I executes before previous D: Duplicate key error (row still exists from previous cycle)
//   - Validation: row exists with iteration=1000
//
// Table 2 (test_update_ordering): Tests U→U ordering on same PK
//   - Pattern: UPDATE version=1 WHERE version=0 → UPDATE version=2 WHERE version=1 → ...
//   - If any U executes out of order: WHERE clause doesn't match, UPDATE is no-op, chain breaks
//   - Validation: row exists with version=1000 (any break → version stuck at break point)
//
// Table 3 (test_insert_update_delete_ordering): Tests I→U, U→D, D→I ordering on same PK
//   - Pattern: INSERT → UPDATE WHERE state='inserted' → DELETE WHERE state='updated' → (repeat)
//   - If U before I: UPDATE finds no row, no-op → DELETE fails → next INSERT may hit duplicate key
//   - If D before U: DELETE finds no state='updated', no-op → row persists → next INSERT fails
//   - If I before D: Duplicate key error
//   - Validation: row exists with state='final', iteration=1001
func TestLiveMigrationWithEventsOnSamePkOrdered(t *testing.T) {
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
			`CREATE SCHEMA IF NOT EXISTS test_schema;`,
			// Table 1: INSERT-DELETE ordering test (same PK, id=1)
			// Tests: I→D (1000x), D→I (999x)
			`CREATE TABLE test_schema.test_insert_delete_ordering (
				id INT PRIMARY KEY,
				iteration INT
			);`,
			// Table 2: UPDATE ordering test (same PK, id=1, chained WHERE)
			// Tests: U→U (1000x)
			`CREATE TABLE test_schema.test_update_ordering (
				id INT PRIMARY KEY,
				version INT
			);`,
			// Table 3: INSERT-UPDATE-DELETE ordering test (same PK, id=1, chained WHERE)
			// Tests: I→U (1000x), U→D (1000x), D→I (1000x)
			`CREATE TABLE test_schema.test_insert_update_delete_ordering (
				id INT PRIMARY KEY,
				state TEXT,
				iteration INT
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.test_insert_delete_ordering REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.test_update_ordering REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.test_insert_update_delete_ordering REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			// Seed row for update ordering test (Table 2)
			`INSERT INTO test_schema.test_update_ordering (id, version) VALUES (1, 0);`,
		},
		SourceDeltaSQL: []string{
			`DO $$
			DECLARE
				i INTEGER;
			BEGIN
				FOR i IN 1..1000 LOOP
					-- Table 1: INSERT-DELETE cycle on same PK (skip DELETE on last iteration)
					-- Tests I→D ordering: if DELETE runs before INSERT, it's a no-op
					-- Tests D→I ordering: if INSERT runs before DELETE, duplicate key error
					INSERT INTO test_schema.test_insert_delete_ordering (id, iteration) VALUES (1, i);
					IF i < 1000 THEN
						DELETE FROM test_schema.test_insert_delete_ordering WHERE id = 1;
					END IF;
					
					-- Table 2: Chained UPDATE on same PK
					-- Tests U→U ordering: UPDATE only succeeds if previous UPDATE completed
					-- WHERE version=i-1 ensures ordering - if previous UPDATE didn't run, this is no-op
					UPDATE test_schema.test_update_ordering SET version = i WHERE id = 1 AND version = i - 1;
					
					-- Table 3: INSERT-UPDATE-DELETE cycle on same PK with chained WHERE
					-- Tests I→U: UPDATE only matches state='inserted'
					-- Tests U→D: DELETE only matches state='updated'
					-- Tests D→I: INSERT fails with duplicate key if DELETE didn't run
					INSERT INTO test_schema.test_insert_update_delete_ordering (id, state, iteration) VALUES (1, 'inserted', i);
					UPDATE test_schema.test_insert_update_delete_ordering SET state = 'updated' WHERE id = 1 AND state = 'inserted' AND iteration = i;
					DELETE FROM test_schema.test_insert_update_delete_ordering WHERE id = 1 AND state = 'updated' AND iteration = i;
				END LOOP;
				
				-- Final INSERT for Table 3 validation (also tests D→I from last DELETE)
				INSERT INTO test_schema.test_insert_update_delete_ordering (id, state, iteration) VALUES (1, 'final', 1001);
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

	// Wait for snapshot (only test_update_ordering has initial data)
	err = lm.WaitForSnapshotComplete(map[string]int64{
		`test_schema."test_update_ordering"`: 1,
	}, 30)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	time.Sleep(5 * time.Second)

	// Validate snapshot data
	err = lm.ValidateDataConsistency([]string{`test_schema."test_update_ordering"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate snapshot data consistency")

	// Execute delta SQL with ordering-sensitive operations
	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	// Wait for streaming to complete
	// Table 1: 1000 inserts, 999 deletes (same PK, id=1)
	// Table 2: 1000 updates (same PK, id=1)
	// Table 3: 1001 inserts, 1000 updates, 1000 deletes (same PK, id=1)
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`test_schema."test_insert_delete_ordering"`: {
			Inserts: 1000,
			Updates: 0,
			Deletes: 999,
		},
		`test_schema."test_update_ordering"`: {
			Inserts: 0,
			Updates: 1000,
			Deletes: 0,
		},
		`test_schema."test_insert_update_delete_ordering"`: {
			Inserts: 1001,
			Updates: 1000,
			Deletes: 1000,
		},
	}, 180, 5)
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	// Validate data consistency for all three tables
	err = lm.ValidateDataConsistency([]string{
		`test_schema."test_insert_delete_ordering"`,
		`test_schema."test_update_ordering"`,
		`test_schema."test_insert_update_delete_ordering"`,
	}, "id")
	testutils.FatalIfError(t, err, "failed to validate streaming data consistency")

	// Additional validation: verify expected final values
	err = lm.WithTargetConn(func(target *sql.DB) error {
		// Table 1: should have iteration=1000
		var iteration int
		err := target.QueryRow(`SELECT iteration FROM test_schema.test_insert_delete_ordering WHERE id = 1`).Scan(&iteration)
		if err != nil {
			return fmt.Errorf("failed to query test_insert_delete_ordering: %w", err)
		}
		if iteration != 1000 {
			return fmt.Errorf("INSERT-DELETE ordering failed: expected iteration=1000, got iteration=%d", iteration)
		}

		// Table 2: should have version=1000
		var version int
		err = target.QueryRow(`SELECT version FROM test_schema.test_update_ordering WHERE id = 1`).Scan(&version)
		if err != nil {
			return fmt.Errorf("failed to query test_update_ordering: %w", err)
		}
		if version != 1000 {
			return fmt.Errorf("UPDATE ordering failed: expected version=1000, got version=%d", version)
		}

		// Table 3: should have state='final', iteration=1001
		var state string
		var iter int
		err = target.QueryRow(`SELECT state, iteration FROM test_schema.test_insert_update_delete_ordering WHERE id = 1`).Scan(&state, &iter)
		if err != nil {
			return fmt.Errorf("failed to query test_insert_update_delete_ordering: %w", err)
		}
		if state != "final" || iter != 1001 {
			return fmt.Errorf("INSERT-UPDATE-DELETE ordering failed: expected state='final', iteration=1001, got state='%s', iteration=%d", state, iter)
		}

		return nil
	})
	testutils.FatalIfError(t, err, "failed to validate ordering")

	err = lm.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")

	err = lm.WaitForCutoverComplete(50)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")
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

	err = lm.InitiateCutoverToTarget(true, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")

	err = lm.WaitForCutoverComplete(50)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

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

	//validate sequence restoration
	err = lm.WithSourceConn(func(source *sql.DB) error {
		return assertSequenceValues(t, 21, 30, source, `test_schema.test_live`)
	})
	testutils.FatalIfError(t, err, "failed to validate sequence restoration")

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

func TestLiveMigrationWithBytesColumn(t *testing.T) {
	liveMigrationTest := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test12",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test12",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.large_test (
				id SERIAL PRIMARY KEY,
				created_at TIMESTAMP DEFAULT now(),
				metadata TEXT,
				payload BYTEA -- This will hold our 5MB
			);
			
			CREATE OR REPLACE FUNCTION generate_large_rows(num_rows INT, size_mb INT)
RETURNS VOID AS $$
DECLARE
    byte_size INT := size_mb * 1024 * 1024;
BEGIN
    INSERT INTO test_schema.large_test (metadata, payload)
    SELECT 
        'Test row ' || i,
        decode(repeat('00', byte_size), 'hex') -- Generates a zero-filled byte array
    FROM generate_series(1, num_rows) AS i;
END;
$$ LANGUAGE plpgsql;`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.large_test REPLICA IDENTITY FULL;`,
			`-- Force Postgres to NOT compress the data 
			-- This ensures the row stays ~5MB and doesn't shrink if the data is repetitive.
			ALTER TABLE test_schema.large_test ALTER COLUMN payload SET STORAGE EXTERNAL;`,
		},
		InitialDataSQL: []string{
			`SELECT generate_large_rows(5, 5);`,
		},
		SourceDeltaSQL: []string{
			`SELECT generate_large_rows(10, 10);`,
		},
		TargetDeltaSQL: []string{
			`SELECT generate_large_rows(5, 5);`,
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
		`test_schema."large_test"`: 5,
	}, 80)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	err = liveMigrationTest.ValidateDataConsistency([]string{`test_schema."large_test"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = liveMigrationTest.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	err = liveMigrationTest.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`test_schema."large_test"`: {
			Inserts: 10,
			Updates: 0,
			Deletes: 0,
		},
	}, 120, 5)
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	err = liveMigrationTest.ValidateRowCount([]string{`test_schema."large_test"`})
	testutils.FatalIfError(t, err, "failed to validate row count")

	err = liveMigrationTest.ValidateDataConsistency([]string{`test_schema."large_test"`}, "id")
	testutils.FatalIfError(t, err, "failed to verify data consistency")

	err = liveMigrationTest.InitiateCutoverToTarget(true, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to target")

	err = liveMigrationTest.WaitForCutoverComplete(50)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

	err = liveMigrationTest.ExecuteTargetDelta()
	testutils.FatalIfError(t, err, "failed to execute target delta")

	err = liveMigrationTest.WaitForFallbackStreamingComplete(map[string]ChangesCount{
		`test_schema."large_test"`: {
			Inserts: 5,
			Updates: 0,
			Deletes: 0,
		},
	}, 120, 5)
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	err = liveMigrationTest.ValidateDataConsistency([]string{`test_schema."large_test"`}, "id")
	testutils.FatalIfError(t, err, "failed to verify data consistency")

	err = liveMigrationTest.InitiateCutoverToSource(nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to source")

	err = liveMigrationTest.WaitForCutoverSourceComplete(150)
	testutils.FatalIfError(t, err, "failed to wait for cutover source complete")

}

// TestLiveMigrationWithDatatypeEdgeCases tests live migration with various datatypes
// containing special characters and edge cases that require proper escaping.
// Currently testing: STRING datatype with backslashes, quotes, newlines, tabs, Unicode, etc.
// This test verifies that the datatype converter properly handles edge cases during CDC streaming.
// Aligned with unit tests in yugabytedbSuite_test.go
func TestLiveMigrationWithDatatypeEdgeCases(t *testing.T) {
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
			`CREATE SCHEMA test_schema;

			CREATE TABLE test_schema.string_edge_cases (
				id SERIAL PRIMARY KEY,
				text_with_backslash TEXT,
				text_with_quote TEXT,
				text_with_newline TEXT,
				text_with_tab TEXT,
				text_with_mixed TEXT,
				text_windows_path TEXT,
				text_sql_injection TEXT,
				text_unicode TEXT,
				text_empty TEXT,
				text_null_string TEXT
			);

			CREATE TABLE test_schema.json_edge_cases (
				id SERIAL PRIMARY KEY,
				json_with_escaped_chars JSON,
				json_with_unicode JSON,
				json_nested JSON,
				json_array JSON,
				json_with_null JSON,
				json_empty JSON,
				json_formatted JSONB,
				json_with_numbers JSON,
				json_complex JSONB
			);

			CREATE TYPE test_schema.status_enum AS ENUM ('active', 'inactive', 'pending', 'enum''value', 'enum"value', 'enum\value', 'with space', 'with-dash', 'with_underscore', 'café', '🎉emoji', '123start');
			
			CREATE TABLE test_schema.enum_edge_cases (
				id SERIAL PRIMARY KEY,
				status_simple test_schema.status_enum,
				status_with_quote test_schema.status_enum,
				status_with_special test_schema.status_enum,
				status_unicode test_schema.status_enum,
				status_array test_schema.status_enum[],
				status_null test_schema.status_enum
			);

			CREATE TABLE test_schema.bytes_edge_cases (
				id SERIAL PRIMARY KEY,
				bytes_empty BYTEA,
				bytes_single BYTEA,
				bytes_ascii BYTEA,
				bytes_null_byte BYTEA,
				bytes_all_zeros BYTEA,
				bytes_all_ff BYTEA,
				bytes_special_chars BYTEA,
				bytes_mixed BYTEA
			);

			CREATE TABLE test_schema.datetime_edge_cases (
				id SERIAL PRIMARY KEY,
				date_epoch DATE,
				date_negative DATE,
				date_future DATE,
				timestamp_epoch TIMESTAMP,
				timestamp_negative TIMESTAMP,
				timestamp_with_tz TIMESTAMPTZ,
				time_midnight TIME,
				time_noon TIME,
				time_with_micro TIME(6)
			);

			CREATE EXTENSION IF NOT EXISTS ltree;

			CREATE TABLE test_schema.uuid_ltree_edge_cases (
				id SERIAL PRIMARY KEY,
				uuid_standard UUID,
				uuid_all_zeros UUID,
				uuid_all_fs UUID,
				uuid_random UUID,
				ltree_simple LTREE,
				ltree_quoted LTREE,
				ltree_deep LTREE,
				ltree_single LTREE
			);

			CREATE EXTENSION IF NOT EXISTS hstore;

			CREATE TABLE test_schema.map_edge_cases (
				id SERIAL PRIMARY KEY,
				map_simple HSTORE,
				map_with_arrow HSTORE,
				map_with_quotes HSTORE,
				map_empty_values HSTORE,
				map_multiple_pairs HSTORE,
				map_special_chars HSTORE
			);

			CREATE TABLE test_schema.interval_edge_cases (
				id SERIAL PRIMARY KEY,
				interval_positive INTERVAL,
				interval_negative INTERVAL,
				interval_zero INTERVAL,
				interval_years INTERVAL,
				interval_days INTERVAL,
				interval_hours INTERVAL,
				interval_mixed INTERVAL
			);

			CREATE TABLE test_schema.zonedtimestamp_edge_cases (
				id SERIAL PRIMARY KEY,
				ts_utc TIMESTAMPTZ,
				ts_positive_offset TIMESTAMPTZ,
				ts_negative_offset TIMESTAMPTZ,
				ts_epoch TIMESTAMPTZ,
				ts_future TIMESTAMPTZ,
				ts_midnight TIMESTAMPTZ
			);

			CREATE TABLE test_schema.decimal_edge_cases (
				id SERIAL PRIMARY KEY,
				decimal_large NUMERIC(38, 9),
				decimal_negative NUMERIC(15, 3),
				decimal_zero NUMERIC(10, 2),
				decimal_high_precision NUMERIC(30, 15),
				decimal_scientific NUMERIC,
				decimal_small NUMERIC(5, 2)
			);
			`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.string_edge_cases REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.json_edge_cases REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.enum_edge_cases REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.bytes_edge_cases REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.datetime_edge_cases REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.uuid_ltree_edge_cases REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.map_edge_cases REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.interval_edge_cases REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.zonedtimestamp_edge_cases REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.decimal_edge_cases REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			// Row 1: Basic edge cases + Unicode separators (TODO 6)
			`INSERT INTO test_schema.string_edge_cases (
				text_with_backslash,
				text_with_quote,
				text_with_newline,
				text_with_tab,
				text_with_mixed,
				text_windows_path,
				text_sql_injection,
				text_unicode,
				text_empty,
				text_null_string
			) VALUES
			(
				'path\to\file',                          -- literal backslash-t, backslash-o
				'It''s a test',                          -- single quote (SQL escaped)
				'line1' || E'\u2028' || 'line2',         -- TODO 6: Unicode line separator (U+2028)
				'para1' || E'\u2029' || 'para2',         -- TODO 6: Unicode paragraph separator (U+2029)
				'word' || E'\u200B' || 'word',           -- TODO 6: Zero-width space (U+200B)
				'word' || E'\u00A0' || 'word',           -- TODO 6: Non-breaking space (U+00A0)
				'''; DROP TABLE users--',                -- SQL injection
				'café 日本語',                           -- Unicode
				'',                                      -- empty string
				'NULL'                                   -- literal string "NULL"
			);`,

			// Row 2: Actual control characters with E-strings (TODOs 7, 9)
			`INSERT INTO test_schema.string_edge_cases (
				text_with_backslash,
				text_with_quote,
				text_with_newline,
				text_with_tab,
				text_with_mixed,
				text_windows_path,
				text_sql_injection,
				text_unicode,
				text_empty,
				text_null_string
			) VALUES
			(
				'\\server\share',                        -- UNC path (double backslash)
				'O''Reilly''s book',                    -- multiple single quotes
				E'line1\nline2',                        -- TODO 7: Actual newline character (E-string)
				E'col1\tcol2',                          -- TODO 7: Actual tab character (E-string)
				E'text\rmore',                          -- TODO 7: Actual carriage return (E-string)
				'C:\Program Files\MyApp\bin',           -- Windows path
				''' OR ''1''=''1',                      -- SQL injection
				'café''s specialty',                     -- TODO 1: Unicode with single quote
				E'\t',                                  -- TODO 9: Tab only (E-string)
				E'\n'                                   -- TODO 9: Newline only (E-string)
			);`,

			// Row 3: Extreme cases + Advanced Unicode (TODOs 2-5)
			`INSERT INTO test_schema.string_edge_cases (
				text_with_backslash,
				text_with_quote,
				text_with_newline,
				text_with_tab,
				text_with_mixed,
				text_windows_path,
				text_sql_injection,
				text_unicode,
				text_empty,
				text_null_string
			) VALUES
			(
				'path\to\日本語',                        -- TODO 2: Unicode with backslash (backslash + Japanese)
				'English مرحبا English',                 -- TODO 5: Bidirectional text (LTR + RTL)
				'Hello 世界 🌍',                         -- TODO 3: Mixed ASCII+Unicode (English + Chinese + emoji)
				'tab',                                  -- simple text
				'All: ''""\\ text',                     -- all special chars
				'C:\new\test\report.txt',               -- path
				'--comment',                            -- SQL comment
				'👨‍👩‍👧 family',                           -- TODO 4: Zero-width joiner emoji (composite emoji)
				' ',                                    -- single space only (critical edge case)
				'This is NULL value'                    -- NULL as part of string
			);`,

			// JSON Row 1: Basic JSON edge cases
			`INSERT INTO test_schema.json_edge_cases (
				json_with_escaped_chars,
				json_with_unicode,
				json_nested,
				json_array,
				json_with_null,
				json_empty,
				json_formatted,
				json_with_numbers,
				json_complex
			) VALUES
			(
				'{"key": "value\"test", "path": "C:\\\\path"}',
				'{"message": "Hello 世界 🎉 café"}',
				'{"outer": {"inner": "value"}}',
				'["item1", "item2", "item\"3"]',
				'{"key": null}',
				'{}',
				'{"formatted": "value"}',
				'{"num": 123, "float": 45.67, "bool": true}',
				'{"str": "test", "num": 123, "bool": true, "null": null, "arr": [1,2]}'
			);`,

			// JSON Row 2: Complex JSON structures
			`INSERT INTO test_schema.json_edge_cases (
				json_with_escaped_chars,
				json_with_unicode,
				json_nested,
				json_array,
				json_with_null,
				json_empty,
				json_formatted,
				json_with_numbers,
				json_complex
			) VALUES
			(
				'{"escapes": "slash:\\\\ newline:\\n tab:\\t return:\\r"}',
				'{"text": "zero\u200Bwidth\u200Djoin"}',
				'{"level1": {"level2": {"level3": "deep"}}}',
				'[1, "two", {"three": 3}]',
				'{"a": null, "b": null}',
				'[]',
				'{"query": "SELECT * FROM users"}',
				'{"int": -999, "float": 3.14159, "exp": 1.23e10}',
				'{"path": "C:\\\\Program Files\\\\App\\\\file.txt", "json": {"nested": true}}'
			);`,

			// JSON Row 3: More JSON edge cases
			`INSERT INTO test_schema.json_edge_cases (
				json_with_escaped_chars,
				json_with_unicode,
				json_nested,
				json_array,
				json_with_null,
				json_empty,
				json_formatted,
				json_with_numbers,
				json_complex
			) VALUES
			(
				'{"key": "line1\nline2"}',
				'{"arabic": "مرحبا", "chinese": "你好"}',
				'{"a": {"b": {"c": {"d": "value"}}}}',
				'[[1,2],[3,4]]',
				'{"result": null}',
				'{"empty": {}}',
				'{"text": "simple value"}',
				'{"zero": 0, "negative": -42, "positive": 42}',
				'{"name": "test", "value": 123}'
			);`,

			// ENUM Row 1: Basic ENUM values
			`INSERT INTO test_schema.enum_edge_cases (
				status_simple,
				status_with_quote,
				status_with_special,
				status_unicode,
				status_array,
				status_null
			) VALUES
			(
				'active',
				'enum''value',
				'with space',
				'café',
				ARRAY['active', 'pending', 'inactive']::test_schema.status_enum[],
				'pending'
			);`,

			// ENUM Row 2: Special character ENUM values
			`INSERT INTO test_schema.enum_edge_cases (
				status_simple,
				status_with_quote,
				status_with_special,
				status_unicode,
				status_array,
				status_null
			) VALUES
			(
				'inactive',
				'enum"value',
				'with-dash',
				'🎉emoji',
				ARRAY['enum''value', 'with space', 'café']::test_schema.status_enum[],
				NULL
			);`,

			// ENUM Row 3: More ENUM edge cases
			`INSERT INTO test_schema.enum_edge_cases (
				status_simple,
				status_with_quote,
				status_with_special,
				status_unicode,
				status_array,
				status_null
			) VALUES
			(
				'pending',
				'enum\value',
				'with_underscore',
				'123start',
				ARRAY['🎉emoji', '123start', 'enum\value']::test_schema.status_enum[],
				'active'
			);`,

			// BYTES Row 1: Basic BYTEA edge cases
			`INSERT INTO test_schema.bytes_edge_cases (
				bytes_empty,
				bytes_single,
				bytes_ascii,
				bytes_null_byte,
				bytes_all_zeros,
				bytes_all_ff,
				bytes_special_chars,
				bytes_mixed
			) VALUES
			(
				E'\\x',
				E'\\x41',
				E'\\x414243',
				E'\\x00',
				E'\\x000000',
				E'\\xffffff',
				E'\\x275c0a',
				E'\\x48656c6c6f'
			);`,

			// BYTES Row 2: More BYTEA patterns
			`INSERT INTO test_schema.bytes_edge_cases (
				bytes_empty,
				bytes_single,
				bytes_ascii,
				bytes_null_byte,
				bytes_all_zeros,
				bytes_all_ff,
				bytes_special_chars,
				bytes_mixed
			) VALUES
			(
				E'\\x',
				E'\\xff',
				E'\\x54657374',
				E'\\x00000000',
				E'\\x0000000000',
				E'\\xffffffffff',
				E'\\x090d',
				E'\\xdeadbeef'
			);`,

			// BYTES Row 3: Special BYTEA patterns
			`INSERT INTO test_schema.bytes_edge_cases (
				bytes_empty,
				bytes_single,
				bytes_ascii,
				bytes_null_byte,
				bytes_all_zeros,
				bytes_all_ff,
				bytes_special_chars,
				bytes_mixed
			) VALUES
			(
				NULL,
				E'\\x7f',
				E'\\x646174',
				E'\\x007465737400',
				E'\\x00',
				E'\\xff',
				E'\\x010203',
				E'\\xcafebabe'
			);`,

			// DATETIME Row 1: Epoch and basic dates
			`INSERT INTO test_schema.datetime_edge_cases (
				date_epoch,
				date_negative,
				date_future,
				timestamp_epoch,
				timestamp_negative,
				timestamp_with_tz,
				time_midnight,
				time_noon,
				time_with_micro
			) VALUES
			(
				'1970-01-01',
				'1969-12-31',
				'2022-01-01',
				'1970-01-01 00:00:00',
				'1969-12-31 00:00:00',
				'2022-01-01 12:00:00+00',
				'00:00:00',
				'12:00:00',
				'12:30:45.123456'
			);`,

			// DATETIME Row 2: Edge case dates
			`INSERT INTO test_schema.datetime_edge_cases (
				date_epoch,
				date_negative,
				date_future,
				timestamp_epoch,
				timestamp_negative,
				timestamp_with_tz,
				time_midnight,
				time_noon,
				time_with_micro
			) VALUES
			(
				'2000-01-01',
				'1900-01-01',
				'2099-12-31',
				'2000-01-01 00:00:00',
				'1900-01-01 12:30:45',
				'2099-12-31 23:59:59+00',
				'23:59:59',
				'06:30:00',
				'00:00:00.000001'
			);`,

			// DATETIME Row 3: Various dates and times
			`INSERT INTO test_schema.datetime_edge_cases (
				date_epoch,
				date_negative,
				date_future,
				timestamp_epoch,
				timestamp_negative,
				timestamp_with_tz,
				time_midnight,
				time_noon,
				time_with_micro
			) VALUES
			(
				'2024-06-15',
				'1950-06-15',
				'2050-06-15',
				'2024-06-15 14:30:00',
				'1950-06-15 08:15:30',
				'2050-06-15 18:45:00-05',
				'18:45:30',
				'09:15:00',
				'23:59:59.999999'
			);`,

			// UUID/LTREE Row 1: Standard values
			`INSERT INTO test_schema.uuid_ltree_edge_cases (
				uuid_standard,
				uuid_all_zeros,
				uuid_all_fs,
				uuid_random,
				ltree_simple,
				ltree_quoted,
				ltree_deep,
				ltree_single
			) VALUES
			(
				'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
				'00000000-0000-0000-0000-000000000000',
				'ffffffff-ffff-ffff-ffff-ffffffffffff',
				'f47ac10b-58cc-4372-a567-0e02b2c3d479',
				'Top.Science.Astronomy',
				'Top.ScienceFiction.Books',
				'Top.Science.Astronomy.Stars.Sun',
				'Top'
			);`,

			// UUID/LTREE Row 2: More values
			`INSERT INTO test_schema.uuid_ltree_edge_cases (
				uuid_standard,
				uuid_all_zeros,
				uuid_all_fs,
				uuid_random,
				ltree_simple,
				ltree_quoted,
				ltree_deep,
				ltree_single
			) VALUES
			(
				'550e8400-e29b-41d4-a716-446655440000',
				'00000000-0000-0000-0000-000000000001',
				'fffffffe-ffff-ffff-ffff-ffffffffffff',
				'6ba7b810-9dad-11d1-80b4-00c04fd430c8',
				'Animals.Mammals.Primates',
				'Products.HomeAppliances.Kitchen',
				'Geography.Continents.Europe.Countries.France.Cities.Paris',
				'Root'
			);`,

			// UUID/LTREE Row 3: Edge cases
			`INSERT INTO test_schema.uuid_ltree_edge_cases (
				uuid_standard,
				uuid_all_zeros,
				uuid_all_fs,
				uuid_random,
				ltree_simple,
				ltree_quoted,
				ltree_deep,
				ltree_single
			) VALUES
			(
				'123e4567-e89b-12d3-a456-426614174000',
				'10000000-0000-0000-0000-000000000000',
				'efffffff-ffff-ffff-ffff-ffffffffffff',
				'00000000-0000-0000-0000-000000000000',
				'Data.Users.Profiles',
				'Items.SpecialCharacters.Test',
				'A.B.C.D.E.F.G.H.I.J',
				'Leaf'
			);`,

			// MAP Row 1: Basic HSTORE values
			`INSERT INTO test_schema.map_edge_cases (
				map_simple,
				map_with_arrow,
				map_with_quotes,
				map_empty_values,
				map_multiple_pairs,
				map_special_chars
			) VALUES
			(
				'"key1" => "value1"',
				'"key=>val" => "test"',
				'"key" => "it''s"',
				'"" => "value"',
				'"a" => "1", "b" => "2", "c" => "3"',
				'"special" => "test@email.com"'
			);`,

			// MAP Row 2: More HSTORE patterns
			`INSERT INTO test_schema.map_edge_cases (
				map_simple,
				map_with_arrow,
				map_with_quotes,
				map_empty_values,
				map_multiple_pairs,
				map_special_chars
			) VALUES
			(
				'"name" => "John"',
				'"key" => "val=>test"',
				'"name" => "O''Reilly"',
				'"key" => ""',
				'"x" => "10", "y" => "20", "z" => "30"',
				'"path" => "C:\\Users\\test"'
			);`,

			// MAP Row 3: Edge case HSTORE
			`INSERT INTO test_schema.map_edge_cases (
				map_simple,
				map_with_arrow,
				map_with_quotes,
				map_empty_values,
				map_multiple_pairs,
				map_special_chars
			) VALUES
			(
				'"status" => "active"',
				'"arrow" => "=>"',
				'"text" => "It''s a test"',
				'"empty" => ""',
				'"one" => "1", "two" => "2"',
				'"data" => "value"'
			);`,

			// INTERVAL Row 1: Positive intervals
			`INSERT INTO test_schema.interval_edge_cases (
				interval_positive,
				interval_negative,
				interval_zero,
				interval_years,
				interval_days,
				interval_hours,
				interval_mixed
			) VALUES
			(
				'1 year 2 months 3 days'::interval,
				'-1 year -2 months'::interval,
				'00:00:00'::interval,
				'5 years'::interval,
				'100 days'::interval,
				'12:30:45'::interval,
				'1 year 6 months 15 days 8 hours 30 minutes'::interval
			);`,

			// INTERVAL Row 2: Various intervals
			`INSERT INTO test_schema.interval_edge_cases (
				interval_positive,
				interval_negative,
				interval_zero,
				interval_years,
				interval_days,
				interval_hours,
				interval_mixed
			) VALUES
			(
				'3 months 7 days'::interval,
				'-5 days -3 hours'::interval,
				'0 seconds'::interval,
				'10 years'::interval,
				'365 days'::interval,
				'23:59:59'::interval,
				'2 years 3 months 10 days 5 hours'::interval
			);`,

			// INTERVAL Row 3: Edge case intervals
			`INSERT INTO test_schema.interval_edge_cases (
				interval_positive,
				interval_negative,
				interval_zero,
				interval_years,
				interval_days,
				interval_hours,
				interval_mixed
			) VALUES
			(
				'6 months'::interval,
				'-1 month -1 day'::interval,
				'0'::interval,
				'1 year'::interval,
				'1 day'::interval,
				'1:00:00'::interval,
				'1 month 1 day 1 hour 1 minute 1 second'::interval
			);`,

			// ZONEDTIMESTAMP Row 1: UTC and various timezones
			`INSERT INTO test_schema.zonedtimestamp_edge_cases (
				ts_utc,
				ts_positive_offset,
				ts_negative_offset,
				ts_epoch,
				ts_future,
				ts_midnight
			) VALUES
			(
				'2024-01-01 00:00:00+00'::timestamptz,
				'2024-06-15 12:30:45+05:30'::timestamptz,
				'2024-12-25 18:00:00-08:00'::timestamptz,
				'1970-01-01 00:00:00+00'::timestamptz,
				'2050-12-31 23:59:59+00'::timestamptz,
				'2024-01-01 00:00:00+00'::timestamptz
			);`,

			// ZONEDTIMESTAMP Row 2: Different timezone offsets
			`INSERT INTO test_schema.zonedtimestamp_edge_cases (
				ts_utc,
				ts_positive_offset,
				ts_negative_offset,
				ts_epoch,
				ts_future,
				ts_midnight
			) VALUES
			(
				'2023-07-04 12:00:00+00'::timestamptz,
				'2023-03-15 08:30:00+01:00'::timestamptz,
				'2023-11-11 22:45:30-05:00'::timestamptz,
				'1969-12-31 23:59:59+00'::timestamptz,
				'2100-01-01 00:00:00+00'::timestamptz,
				'2023-06-21 00:00:00+00'::timestamptz
			);`,

			// ZONEDTIMESTAMP Row 3: Edge case timezones
			`INSERT INTO test_schema.zonedtimestamp_edge_cases (
				ts_utc,
				ts_positive_offset,
				ts_negative_offset,
				ts_epoch,
				ts_future,
				ts_midnight
			) VALUES
			(
				'2025-01-01 06:00:00+00'::timestamptz,
				'2025-05-20 14:15:30+09:00'::timestamptz,
				'2025-08-10 10:20:40-07:00'::timestamptz,
				'1970-01-01 12:00:00+00'::timestamptz,
				'2075-06-15 18:30:00+00'::timestamptz,
				'2025-12-31 00:00:00+00'::timestamptz
			);`,

			// DECIMAL Row 1: Large and negative decimals
			`INSERT INTO test_schema.decimal_edge_cases (
				decimal_large,
				decimal_negative,
				decimal_zero,
				decimal_high_precision,
				decimal_scientific,
				decimal_small
			) VALUES
			(
				123456789.123456789,
				-123.456,
				0.00,
				123.456789012345,
				1000000.00,
				99.99
			);`,

			// DECIMAL Row 2: Various precisions
			`INSERT INTO test_schema.decimal_edge_cases (
				decimal_large,
				decimal_negative,
				decimal_zero,
				decimal_high_precision,
				decimal_scientific,
				decimal_small
			) VALUES
			(
				987654321.987654321,
				-999.999,
				0,
				999.999999999999999,
				0.000001,
				-50.25
			);`,

			// DECIMAL Row 3: Edge case decimals
			`INSERT INTO test_schema.decimal_edge_cases (
				decimal_large,
				decimal_negative,
				decimal_zero,
				decimal_high_precision,
				decimal_scientific,
				decimal_small
			) VALUES
			(
				1.000000001,
				-0.001,
				0.0,
				0.000000000000001,
				99999999999.999,
				12.34
			);`,
		},
		SourceDeltaSQL: []string{
			// INSERT #1: Streaming with Unicode separators (TODO 6)
			`INSERT INTO test_schema.string_edge_cases (
				text_with_backslash,
				text_with_quote,
				text_with_newline,
				text_with_tab,
				text_with_mixed,
				text_windows_path,
				text_sql_injection,
				text_unicode,
				text_empty,
				text_null_string
			) VALUES
			(
				'streaming\path',                       -- backslash path
				'streaming''s test',                    -- single quote
				'first' || E'\u2028' || 'second',       -- TODO 6: Unicode line separator in streaming
				'word' || E'\u200B' || 'word',          -- TODO 6: Zero-width space in streaming
				'text' || E'\u00A0' || 'text',          -- TODO 6: Non-breaking space in streaming
				'D:\streaming\path',                    -- Windows path
				'''; DELETE FROM test',                 -- SQL injection
				'streaming 数据',                        -- Chinese
				'',                                     -- empty
				'NULL'                                  -- NULL literal
			);`,

			// UPDATE #1: Update backslash patterns
			`UPDATE test_schema.string_edge_cases
			SET text_with_backslash = 'updated: \x\y\z',
			    text_with_quote = 'updated''s value'
			WHERE id = 1;`,

			// UPDATE #2: Update with consecutive quotes
			`UPDATE test_schema.string_edge_cases
			SET text_with_quote = 'O''''Reilly',
			    text_sql_injection = '''; DROP TABLE users--'
			WHERE id = 2;`,

			// UPDATE #3: Test critical edge cases - backslash+quote, emoji, single space
			`UPDATE test_schema.string_edge_cases
			SET text_with_backslash = 'updated\''s test',
			    text_unicode = '🎉 emoji test 日本',
			    text_empty = ' '
			WHERE id = 3;`,

			// UPDATE #4: Test advanced Unicode patterns (TODOs 2-5)
			`UPDATE test_schema.string_edge_cases
			SET text_with_backslash = 'updated\path\数据',
			    text_with_quote = 'Hello مرحبا world',
			    text_with_newline = 'Mixed 世界 test 🌏',
			    text_unicode = '👨‍👩‍👧‍👦 emoji family'
			WHERE id = 2;`,

			// UPDATE #5: Test Unicode separators in streaming (TODO 6)
			`UPDATE test_schema.string_edge_cases
			SET text_with_newline = 'updated' || E'\u2028' || 'line',
			    text_with_tab = 'updated' || E'\u2029' || 'para',
			    text_with_mixed = 'zero' || E'\u200B' || 'width',
			    text_windows_path = 'nbsp' || E'\u00A0' || 'here'
			WHERE id = 1;`,

			// UPDATE #6: Test actual control chars in UPDATE (TODOs 7, 8, 9)
			`UPDATE test_schema.string_edge_cases
			SET text_with_newline = E'new\nline\ntest',
			    text_with_tab = E'new\ttab\ttest',
			    text_with_mixed = E'It''s "test" with \n\t\r',
			    text_empty = E' \t\n\r '
			WHERE id = 2;`,

			// INSERT #2: Test actual control chars in streaming (TODOs 7, 8)
			`INSERT INTO test_schema.string_edge_cases (
				text_with_backslash,
				text_with_quote,
				text_with_newline,
				text_with_tab,
				text_with_mixed,
				text_windows_path,
				text_sql_injection,
				text_unicode,
				text_empty,
				text_null_string
			) VALUES
			(
				'another\path\to\文件',                  -- TODO 2: Unicode with backslash (Chinese)
				'café''s specialty Ñoño',               -- TODO 1: Unicode with single quote
				E'first\nsecond\nthird',                -- TODO 7: Multiple actual newlines (E-string)
				E'a\tb\tc\td',                          -- TODO 7: Multiple actual tabs (E-string)
				E'mix: ''"\\\n\t\r',                    -- TODO 8: All special chars with actual control chars
				'C:\path',                              -- Windows path
				'--sql',                                -- SQL comment
				'مرحبا Hello مرحبا',                     -- TODO 5: Bidirectional text
				E'\n',                                  -- TODO 9: Newline only
				'NULL'                                  -- NULL literal
			);`,

			// DELETE #1: Test deletion during streaming
			`DELETE FROM test_schema.string_edge_cases WHERE id = 3;`,

			// JSON INSERT #1: Streaming JSON with special characters
			`INSERT INTO test_schema.json_edge_cases (
				json_with_escaped_chars,
				json_with_unicode,
				json_nested,
				json_array,
				json_with_null,
				json_empty,
				json_formatted,
				json_with_numbers,
				json_complex
			) VALUES
			(
				'{"streaming": "value with backslash\\\\ test"}',
				'{"stream": "数据流 🚀"}',
				'{"new": {"nested": "stream"}}',
				'["stream1", "stream2"]',
				'{"stream": null}',
				'{}',
				'{"stream": "formatted"}',
				'{"count": 999}',
				'{"streaming": true, "data": "test"}'
			);`,

			// JSON UPDATE #1: Update with complex JSON
			`UPDATE test_schema.json_edge_cases
			SET json_with_escaped_chars = '{"updated": "simple value"}',
			    json_with_unicode = '{"updated": "café 世界 🎉"}',
			    json_nested = '{"updated": {"deep": {"nesting": "value"}}}'
			WHERE id = 1;`,

			// JSON UPDATE #2: Update with empty and null values
			`UPDATE test_schema.json_edge_cases
			SET json_empty = '{"now": "not_empty"}',
			    json_with_null = '{"was": null, "now": "value"}',
			    json_array = '[1, 2, 3, 4, 5]'
			WHERE id = 2;`,

			// JSON DELETE #1: Test JSON row deletion
			`DELETE FROM test_schema.json_edge_cases WHERE id = 3;`,

			// ENUM INSERT #1: Streaming ENUM with special characters
			`INSERT INTO test_schema.enum_edge_cases (
				status_simple,
				status_with_quote,
				status_with_special,
				status_unicode,
				status_array,
				status_null
			) VALUES
			(
				'active',
				'enum''value',
				'with-dash',
				'🎉emoji',
				ARRAY['active', 'café', '123start']::test_schema.status_enum[],
				NULL
			);`,

			// ENUM UPDATE #1: Update ENUM values
			`UPDATE test_schema.enum_edge_cases
			SET status_simple = 'pending',
			    status_with_quote = 'enum"value',
			    status_unicode = '123start'
			WHERE id = 1;`,

			// ENUM UPDATE #2: Update ENUM array
			`UPDATE test_schema.enum_edge_cases
			SET status_array = ARRAY['enum\value', 'with_underscore', 'with space']::test_schema.status_enum[],
			    status_null = 'inactive'
			WHERE id = 2;`,

			// ENUM DELETE #1: Test ENUM row deletion
			`DELETE FROM test_schema.enum_edge_cases WHERE id = 3;`,

			// BYTES INSERT #1: Streaming BYTEA with special patterns
			`INSERT INTO test_schema.bytes_edge_cases (
				bytes_empty,
				bytes_single,
				bytes_ascii,
				bytes_null_byte,
				bytes_all_zeros,
				bytes_all_ff,
				bytes_special_chars,
				bytes_mixed
			) VALUES
			(
				E'\\x',
				E'\\x42',
				E'\\x53747265616d',
				E'\\x0000',
				E'\\x00000000',
				E'\\xffffffff',
				E'\\x5c27',
				E'\\x0123456789abcdef'
			);`,

			// BYTES UPDATE #1: Update BYTEA values
			`UPDATE test_schema.bytes_edge_cases
			SET bytes_single = E'\\xaa',
			    bytes_ascii = E'\\x557064617465',
			    bytes_mixed = E'\\xfeedface'
			WHERE id = 1;`,

			// BYTES UPDATE #2: Update with null bytes and special patterns
			`UPDATE test_schema.bytes_edge_cases
			SET bytes_null_byte = E'\\x00ff00ff',
			    bytes_all_zeros = E'\\x0000',
			    bytes_all_ff = E'\\xffff'
			WHERE id = 2;`,

			// BYTES DELETE #1: Test BYTEA row deletion
			`DELETE FROM test_schema.bytes_edge_cases WHERE id = 3;`,

			// DATETIME INSERT #1: Streaming datetime values
			`INSERT INTO test_schema.datetime_edge_cases (
				date_epoch,
				date_negative,
				date_future,
				timestamp_epoch,
				timestamp_negative,
				timestamp_with_tz,
				time_midnight,
				time_noon,
				time_with_micro
			) VALUES
			(
				'2023-01-15',
				'1980-03-20',
				'2030-08-10',
				'2023-01-15 10:20:30',
				'1980-03-20 15:45:00',
				'2030-08-10 20:00:00+02',
				'10:20:30',
				'15:45:00',
				'08:15:30.654321'
			);`,

			// DATETIME UPDATE #1: Update datetime values
			`UPDATE test_schema.datetime_edge_cases
			SET date_epoch = '2025-12-25',
			    timestamp_epoch = '2025-12-25 18:30:00',
			    time_midnight = '01:02:03'
			WHERE id = 1;`,

			// DATETIME UPDATE #2: Update with edge case times
			`UPDATE test_schema.datetime_edge_cases
			SET date_future = '2099-01-01',
			    timestamp_with_tz = '2099-01-01 00:00:00-08',
			    time_with_micro = '12:34:56.789012'
			WHERE id = 2;`,

			// DATETIME DELETE #1: Test datetime row deletion
			`DELETE FROM test_schema.datetime_edge_cases WHERE id = 3;`,

			// UUID/LTREE INSERT #1: Streaming UUID/LTREE values
			`INSERT INTO test_schema.uuid_ltree_edge_cases (
				uuid_standard,
				uuid_all_zeros,
				uuid_all_fs,
				uuid_random,
				ltree_simple,
				ltree_quoted,
				ltree_deep,
				ltree_single
			) VALUES
			(
				'7c9e6679-7425-40de-944b-e07fc1f90ae7',
				'00000000-0000-0000-0000-000000000002',
				'fffffffd-ffff-ffff-ffff-ffffffffffff',
				'9b2c8f5d-1234-5678-9abc-def012345678',
				'Stream.Data.Live',
				'Test.StreamingPath.Values',
				'Deep.Path.To.Stream.Data.Node',
				'Stream'
			);`,

			// UUID/LTREE UPDATE #1: Update UUID/LTREE values
			`UPDATE test_schema.uuid_ltree_edge_cases
			SET uuid_standard = 'c2a9c8d0-1234-5678-9abc-def123456789',
			    ltree_simple = 'Updated.Path.Node'
			WHERE id = 1;`,

			// UUID/LTREE UPDATE #2: Update with edge case values
			`UPDATE test_schema.uuid_ltree_edge_cases
			SET uuid_all_zeros = '00000000-0000-0000-0000-000000000003',
			    ltree_deep = 'Very.Deep.Path.With.Many.Levels.To.Test'
			WHERE id = 2;`,

			// UUID/LTREE DELETE #1: Test UUID/LTREE row deletion
			`DELETE FROM test_schema.uuid_ltree_edge_cases WHERE id = 3;`,

			// MAP INSERT #1: Streaming HSTORE values
			`INSERT INTO test_schema.map_edge_cases (
				map_simple,
				map_with_arrow,
				map_with_quotes,
				map_empty_values,
				map_multiple_pairs,
				map_special_chars
			) VALUES
			(
				'"stream" => "data"',
				'"test=>key" => "value"',
				'"quote" => "test''s"',
				'"" => "empty"',
				'"s1" => "v1", "s2" => "v2"',
				'"special" => "data"'
			);`,

			// MAP UPDATE #1: Update HSTORE values
			`UPDATE test_schema.map_edge_cases
			SET map_simple = '"updated" => "value"',
			    map_with_arrow = '"arrow=>test" => "updated"'
			WHERE id = 1;`,

			// MAP UPDATE #2: Update with special characters
			`UPDATE test_schema.map_edge_cases
			SET map_with_quotes = '"name" => "O''Brien"',
			    map_multiple_pairs = '"x" => "100", "y" => "200"'
			WHERE id = 2;`,

			// MAP DELETE #1: Test HSTORE row deletion
			`DELETE FROM test_schema.map_edge_cases WHERE id = 3;`,

			// INTERVAL INSERT #1: Streaming INTERVAL values
			`INSERT INTO test_schema.interval_edge_cases (
				interval_positive,
				interval_negative,
				interval_zero,
				interval_years,
				interval_days,
				interval_hours,
				interval_mixed
			) VALUES
			(
				'2 years 5 months'::interval,
				'-10 days'::interval,
				'0 minutes'::interval,
				'50 years'::interval,
				'7 days'::interval,
				'6:15:30'::interval,
				'3 years 2 months 20 days 10 hours'::interval
			);`,

			// INTERVAL UPDATE #1: Update INTERVAL values
			`UPDATE test_schema.interval_edge_cases
			SET interval_positive = '8 months 15 days'::interval,
			    interval_years = '25 years'::interval
			WHERE id = 1;`,

			// INTERVAL UPDATE #2: Update with negative and zero
			`UPDATE test_schema.interval_edge_cases
			SET interval_negative = '-3 months -7 days'::interval,
			    interval_mixed = '5 months 10 days 2 hours 30 minutes'::interval
			WHERE id = 2;`,

			// INTERVAL DELETE #1: Test INTERVAL row deletion
			`DELETE FROM test_schema.interval_edge_cases WHERE id = 3;`,

			// ZONEDTIMESTAMP INSERT #1: Streaming TIMESTAMPTZ values
			`INSERT INTO test_schema.zonedtimestamp_edge_cases (
				ts_utc,
				ts_positive_offset,
				ts_negative_offset,
				ts_epoch,
				ts_future,
				ts_midnight
			) VALUES
			(
				'2024-08-20 15:45:30+00'::timestamptz,
				'2024-09-10 09:15:00+03:00'::timestamptz,
				'2024-10-05 20:30:15-06:00'::timestamptz,
				'1970-01-02 00:00:00+00'::timestamptz,
				'2060-05-15 12:00:00+00'::timestamptz,
				'2024-07-01 00:00:00+00'::timestamptz
			);`,

			// ZONEDTIMESTAMP UPDATE #1: Update TIMESTAMPTZ values
			`UPDATE test_schema.zonedtimestamp_edge_cases
			SET ts_utc = '2024-02-14 10:30:00+00'::timestamptz,
			    ts_positive_offset = '2024-03-20 16:45:00+08:00'::timestamptz
			WHERE id = 1;`,

			// ZONEDTIMESTAMP UPDATE #2: Update with different timezones
			`UPDATE test_schema.zonedtimestamp_edge_cases
			SET ts_negative_offset = '2023-09-30 11:11:11-04:00'::timestamptz,
			    ts_future = '2090-12-31 23:59:59+00'::timestamptz
			WHERE id = 2;`,

			// ZONEDTIMESTAMP DELETE #1: Test TIMESTAMPTZ row deletion
			`DELETE FROM test_schema.zonedtimestamp_edge_cases WHERE id = 3;`,

			// DECIMAL INSERT #1: Streaming DECIMAL values
			`INSERT INTO test_schema.decimal_edge_cases (
				decimal_large,
				decimal_negative,
				decimal_zero,
				decimal_high_precision,
				decimal_scientific,
				decimal_small
			) VALUES
			(
				555555555.555555555,
				-777.777,
				0.000,
				888.888888888888888,
				12345.6789,
				75.50
			);`,

			// DECIMAL UPDATE #1: Update DECIMAL values
			`UPDATE test_schema.decimal_edge_cases
			SET decimal_large = 999999999.999999999,
			    decimal_negative = -1000.001
			WHERE id = 1;`,

			// DECIMAL UPDATE #2: Update with high precision
			`UPDATE test_schema.decimal_edge_cases
			SET decimal_high_precision = 0.123456789012345,
			    decimal_scientific = 999999.999999
			WHERE id = 2;`,

			// DECIMAL DELETE #1: Test DECIMAL row deletion
			`DELETE FROM test_schema.decimal_edge_cases WHERE id = 3;`,
		},
		CleanupSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema CASCADE;`,
		},
	})

	defer lm.Cleanup()

	t.Log("=== Setting up containers ===")
	err := lm.SetupContainers(context.Background())
	testutils.FatalIfError(t, err, "failed to setup containers")

	t.Log("=== Setting up schema ===")
	err = lm.SetupSchema()
	testutils.FatalIfError(t, err, "failed to setup schema")

	t.Log("=== Starting export data ===")
	err = lm.StartExportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start export data")

	t.Log("=== Starting import data ===")
	err = lm.StartImportData(true, map[string]string{
		"--log-level": "debug",
	})
	testutils.FatalIfError(t, err, "failed to start import data")

	t.Log("=== Waiting for snapshot complete (10 datatypes × 3 rows = 30 rows) ===")
	err = lm.WaitForSnapshotComplete(map[string]int64{
		`test_schema."string_edge_cases"`:         3, // 3 rows with STRING edge cases
		`test_schema."json_edge_cases"`:           3, // 3 rows with JSON edge cases
		`test_schema."enum_edge_cases"`:           3, // 3 rows with ENUM edge cases
		`test_schema."bytes_edge_cases"`:          3, // 3 rows with BYTES edge cases
		`test_schema."datetime_edge_cases"`:       3, // 3 rows with DATETIME edge cases
		`test_schema."uuid_ltree_edge_cases"`:     3, // 3 rows with UUID/LTREE edge cases
		`test_schema."map_edge_cases"`:            3, // 3 rows with MAP edge cases
		`test_schema."interval_edge_cases"`:       3, // 3 rows with INTERVAL edge cases
		`test_schema."zonedtimestamp_edge_cases"`: 3, // 3 rows with ZONEDTIMESTAMP edge cases
		`test_schema."decimal_edge_cases"`:        3, // 3 rows with DECIMAL edge cases
	}, 60) // 1 minute should be plenty for small rows
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	t.Log("=== Validating snapshot data ===")
	err = lm.ValidateDataConsistency([]string{`test_schema."string_edge_cases"`, `test_schema."json_edge_cases"`, `test_schema."enum_edge_cases"`, `test_schema."bytes_edge_cases"`, `test_schema."datetime_edge_cases"`, `test_schema."uuid_ltree_edge_cases"`, `test_schema."map_edge_cases"`, `test_schema."interval_edge_cases"`, `test_schema."zonedtimestamp_edge_cases"`, `test_schema."decimal_edge_cases"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate snapshot data consistency")

	t.Log("=== Executing source delta (streaming operations) ===")
	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	t.Log("=== Waiting for streaming complete (10 datatypes: STRING: 2/6/1, Others: 1/2/1) ===")
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`test_schema."string_edge_cases"`: {
			Inserts: 2, // 2 INSERT operations: basic + actual control chars
			Updates: 6, // 6 UPDATE operations: backslash, consecutive quotes, critical edge cases, advanced Unicode, Unicode separators, actual control chars
			Deletes: 1, // 1 DELETE operation: delete row 3
		},
		`test_schema."json_edge_cases"`: {
			Inserts: 1, // 1 INSERT operation: JSON with special characters
			Updates: 2, // 2 UPDATE operations: complex JSON, empty/null updates
			Deletes: 1, // 1 DELETE operation: delete JSON row 3
		},
		`test_schema."enum_edge_cases"`: {
			Inserts: 1, // 1 INSERT operation: ENUM with special characters
			Updates: 2, // 2 UPDATE operations: ENUM value updates, ENUM array updates
			Deletes: 1, // 1 DELETE operation: delete ENUM row 3
		},
		`test_schema."bytes_edge_cases"`: {
			Inserts: 1, // 1 INSERT operation: BYTES with special patterns
			Updates: 2, // 2 UPDATE operations: BYTES value updates, null byte patterns
			Deletes: 1, // 1 DELETE operation: delete BYTES row 3
		},
		`test_schema."datetime_edge_cases"`: {
			Inserts: 1, // 1 INSERT operation: DATETIME with various dates/times
			Updates: 2, // 2 UPDATE operations: date/timestamp updates, timezone updates
			Deletes: 1, // 1 DELETE operation: delete DATETIME row 3
		},
		`test_schema."uuid_ltree_edge_cases"`: {
			Inserts: 1, // 1 INSERT operation: UUID/LTREE with edge cases
			Updates: 2, // 2 UPDATE operations: UUID updates, LTREE path updates
			Deletes: 1, // 1 DELETE operation: delete UUID/LTREE row 3
		},
		`test_schema."map_edge_cases"`: {
			Inserts: 1, // 1 INSERT operation: MAP/HSTORE with arrow operator and quotes
			Updates: 2, // 2 UPDATE operations: MAP value updates, special character updates
			Deletes: 1, // 1 DELETE operation: delete MAP row 3
		},
		`test_schema."interval_edge_cases"`: {
			Inserts: 1, // 1 INSERT operation: INTERVAL with positive/negative/zero
			Updates: 2, // 2 UPDATE operations: INTERVAL value updates, mixed intervals
			Deletes: 1, // 1 DELETE operation: delete INTERVAL row 3
		},
		`test_schema."zonedtimestamp_edge_cases"`: {
			Inserts: 1, // 1 INSERT operation: TIMESTAMPTZ with various timezones
			Updates: 2, // 2 UPDATE operations: TIMESTAMPTZ value updates, timezone offset updates
			Deletes: 1, // 1 DELETE operation: delete TIMESTAMPTZ row 3
		},
		`test_schema."decimal_edge_cases"`: {
			Inserts: 1, // 1 INSERT operation: DECIMAL with large, negative, high precision
			Updates: 2, // 2 UPDATE operations: DECIMAL value updates, precision updates
			Deletes: 1, // 1 DELETE operation: delete DECIMAL row 3
		},
	}, 60, 1) // 1 minute timeout, 1 second poll interval
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	t.Log("=== Validating streaming data ===")
	err = lm.ValidateDataConsistency([]string{`test_schema."string_edge_cases"`, `test_schema."json_edge_cases"`, `test_schema."enum_edge_cases"`, `test_schema."bytes_edge_cases"`, `test_schema."datetime_edge_cases"`, `test_schema."uuid_ltree_edge_cases"`, `test_schema."map_edge_cases"`, `test_schema."interval_edge_cases"`, `test_schema."zonedtimestamp_edge_cases"`, `test_schema."decimal_edge_cases"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate streaming data consistency")

	t.Log("=== Initiating cutover ===")
	err = lm.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")

	t.Log("=== Waiting for cutover complete ===")
	err = lm.WaitForCutoverComplete(60)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

	t.Log("=== Final validation ===")
	err = lm.ValidateDataConsistency([]string{`test_schema."string_edge_cases"`, `test_schema."json_edge_cases"`, `test_schema."enum_edge_cases"`, `test_schema."bytes_edge_cases"`, `test_schema."datetime_edge_cases"`, `test_schema."uuid_ltree_edge_cases"`, `test_schema."map_edge_cases"`, `test_schema."interval_edge_cases"`, `test_schema."zonedtimestamp_edge_cases"`, `test_schema."decimal_edge_cases"`}, "id")
	testutils.FatalIfError(t, err, "failed final data consistency check")

	t.Log("✅ All datatype edge cases test PASSED - STRING, JSON, ENUM, BYTES, DATETIME, UUID, LTREE!")
}

// Test INTERVAL columns with different casings during fallback streaming
// This validates the fix for case-sensitivity bug in INTERVAL column lookup
// Tests: unquoted lowercase, quoted mixed-case, and multiple INTERVAL columns
func TestLiveMigrationIntervalColumnsFallback(t *testing.T) {
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test_interval_fallback",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test_interval_fallback",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema CASCADE;`,
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.interval_test (
				id SERIAL PRIMARY KEY,
				-- Unquoted lowercase (most common case)
				interval_years INTERVAL,
				interval_days INTERVAL,
				interval_hours INTERVAL,
				-- Quoted identifiers (edge cases for case sensitivity)
				"IntervalMixed" INTERVAL,
				"INTERVAL_UPPER" INTERVAL,
				name TEXT
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.interval_test REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.interval_test (interval_years, interval_days, interval_hours, "IntervalMixed", "INTERVAL_UPPER", name)
VALUES
	(INTERVAL '1 year', INTERVAL '30 days', INTERVAL '5 hours', INTERVAL '2 months', INTERVAL '1 day 3 hours', 'row1'),
	(INTERVAL '2 years 6 months', INTERVAL '45 days', INTERVAL '10 hours', INTERVAL '3 months 15 days', INTERVAL '2 days', 'row2'),
	(INTERVAL '5 years', INTERVAL '90 days', INTERVAL '24 hours', INTERVAL '1 year', INTERVAL '5 hours 30 minutes', 'row3');`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema.interval_test (interval_years, interval_days, interval_hours, "IntervalMixed", "INTERVAL_UPPER", name)
VALUES
	(INTERVAL '3 years', INTERVAL '60 days', INTERVAL '8 hours', INTERVAL '6 months', INTERVAL '3 days', 'row4');`,
		},
		TargetDeltaSQL: []string{
			// These updates during fallback will test the fix for case-sensitivity
			`UPDATE test_schema.interval_test SET interval_years = INTERVAL '10 years' WHERE id = 1;`,
			`UPDATE test_schema.interval_test SET interval_days = INTERVAL '100 days', interval_hours = INTERVAL '20 hours' WHERE id = 2;`,
			`UPDATE test_schema.interval_test SET "IntervalMixed" = INTERVAL '12 months', "INTERVAL_UPPER" = INTERVAL '7 days' WHERE id = 3;`,
			// Insert with all INTERVAL columns to test case sensitivity
			`INSERT INTO test_schema.interval_test (interval_years, interval_days, interval_hours, "IntervalMixed", "INTERVAL_UPPER", name)
VALUES (INTERVAL '7 years', INTERVAL '120 days', INTERVAL '15 hours', INTERVAL '8 months', INTERVAL '4 days', 'fallback_row');`,
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

	err = lm.StartImportData(true, map[string]string{
		"--log-level": "debug",
	})
	testutils.FatalIfError(t, err, "failed to start import data")

	time.Sleep(10 * time.Second)

	// Wait for snapshot to complete (3 initial rows)
	err = lm.WaitForSnapshotComplete(map[string]int64{
		`test_schema."interval_test"`: 3,
	}, 30)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	// Validate snapshot data consistency
	err = lm.ValidateDataConsistency([]string{`test_schema."interval_test"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate snapshot data consistency")

	// Execute source delta (forward streaming: PG→YB)
	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	// Wait for forward streaming to complete (1 insert)
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`test_schema."interval_test"`: {
			Inserts: 1,
			Updates: 0,
			Deletes: 0,
		},
	}, 30, 1)
	testutils.FatalIfError(t, err, "failed to wait for forward streaming complete")

	// Validate forward streaming data
	err = lm.ValidateDataConsistency([]string{`test_schema."interval_test"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate forward streaming data consistency")

	// Initiate cutover to target (YB becomes primary)
	err = lm.InitiateCutoverToTarget(true, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to target")

	err = lm.WaitForCutoverComplete(50)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

	// Execute target delta (fallback streaming: YB→PG)
	// This is where the bug was: INTERVAL columns with different casings failed during fallback
	err = lm.ExecuteTargetDelta()
	testutils.FatalIfError(t, err, "failed to execute target delta")

	// Wait for fallback streaming to complete (3 updates + 1 insert)
	err = lm.WaitForFallbackStreamingComplete(map[string]ChangesCount{
		`test_schema."interval_test"`: {
			Inserts: 1,
			Updates: 3,
			Deletes: 0,
		},
	}, 30, 1)
	testutils.FatalIfError(t, err, "failed to wait for fallback streaming complete")

	// Validate fallback streaming data consistency
	// This ensures all INTERVAL columns (lowercase and mixed-case) were correctly replicated
	// CompareTableData does a full SELECT * comparison of all rows and columns
	err = lm.ValidateDataConsistency([]string{`test_schema."interval_test"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate fallback data consistency")

	// Complete cutover to source
	err = lm.InitiateCutoverToSource(nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to source")

	err = lm.WaitForCutoverSourceComplete(100)
	testutils.FatalIfError(t, err, "failed to wait for cutover source complete")

}
