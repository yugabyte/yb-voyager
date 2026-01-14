//go:build integration_live_migration

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
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test1",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test1",
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
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test2",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test2",
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

// TestLiveMigrationWithEventsOnSamePkOrderedFallback tests that INSERT/UPDATE/DELETE events
// for the same primary key are applied in the correct order during fall-back streaming (target→source).
//
// This is the fall-back counterpart to TestLiveMigrationWithEventsOnSamePkOrdered.
// Events are generated on the target (YugabyteDB) and streamed back to the source (PostgreSQL).
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
// Table 1 (test_insert_delete_ordering): Tests I→D and D→I ordering on same PK
// Table 2 (test_update_ordering): Tests U→U ordering on same PK
// Table 3 (test_insert_update_delete_ordering): Tests I→U, U→D, D→I ordering on same PK
func TestLiveMigrationWithEventsOnSamePkOrderedFallback(t *testing.T) {
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test_live_same_pk_ordered_fallback",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test_live_same_pk_ordered_fallback",
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
		// No SourceDeltaSQL - forward streaming already covered by other tests
		TargetDeltaSQL: []string{
			// Ordering-sensitive events executed on target (YugabyteDB) and streamed back to source
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

	// Initiate cutover to target with fall-back enabled
	err = lm.InitiateCutoverToTarget(true, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")

	err = lm.WaitForCutoverComplete(50)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

	// Execute ordering-sensitive delta SQL on target (YugabyteDB)
	err = lm.ExecuteTargetDelta()
	testutils.FatalIfError(t, err, "failed to execute target delta")

	// Wait for fall-back streaming to complete (target → source)
	// Table 1: 1000 inserts, 999 deletes (same PK, id=1)
	// Table 2: 1000 updates (same PK, id=1)
	// Table 3: 1001 inserts, 1000 updates, 1000 deletes (same PK, id=1)
	err = lm.WaitForFallbackStreamingComplete(map[string]ChangesCount{
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
	testutils.FatalIfError(t, err, "failed to wait for fall-back streaming complete")

	// Validate data consistency for all three tables
	err = lm.ValidateDataConsistency([]string{
		`test_schema."test_insert_delete_ordering"`,
		`test_schema."test_update_ordering"`,
		`test_schema."test_insert_update_delete_ordering"`,
	}, "id")
	testutils.FatalIfError(t, err, "failed to validate streaming data consistency")

	// Additional validation: verify expected final values on source
	err = lm.WithSourceConn(func(source *sql.DB) error {
		// Table 1: should have iteration=1000
		var iteration int
		err := source.QueryRow(`SELECT iteration FROM test_schema.test_insert_delete_ordering WHERE id = 1`).Scan(&iteration)
		if err != nil {
			return fmt.Errorf("failed to query test_insert_delete_ordering: %w", err)
		}
		if iteration != 1000 {
			return fmt.Errorf("INSERT-DELETE ordering failed: expected iteration=1000, got iteration=%d", iteration)
		}

		// Table 2: should have version=1000
		var version int
		err = source.QueryRow(`SELECT version FROM test_schema.test_update_ordering WHERE id = 1`).Scan(&version)
		if err != nil {
			return fmt.Errorf("failed to query test_update_ordering: %w", err)
		}
		if version != 1000 {
			return fmt.Errorf("UPDATE ordering failed: expected version=1000, got version=%d", version)
		}

		// Table 3: should have state='final', iteration=1001
		var state string
		var iter int
		err = source.QueryRow(`SELECT state, iteration FROM test_schema.test_insert_update_delete_ordering WHERE id = 1`).Scan(&state, &iter)
		if err != nil {
			return fmt.Errorf("failed to query test_insert_update_delete_ordering: %w", err)
		}
		if state != "final" || iter != 1001 {
			return fmt.Errorf("INSERT-UPDATE-DELETE ordering failed: expected state='final', iteration=1001, got state='%s', iteration=%d", state, iter)
		}

		return nil
	})
	testutils.FatalIfError(t, err, "failed to validate ordering on source")

	// Cutover back to source
	err = lm.InitiateCutoverToSource(nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to source")

	err = lm.WaitForCutoverSourceComplete(100)
	testutils.FatalIfError(t, err, "failed to wait for cutover to source complete")
}

func TestBasicLiveMigrationWithFallback(t *testing.T) {

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test3",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test3",
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

	time.Sleep(10 * time.Second)

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
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test4",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test4",
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

	time.Sleep(10 * time.Second)

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
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test5",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test5",
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
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test6",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test6",
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

	err = lm.WaitForCutoverComplete(30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

}

func TestLiveMigrationWithUniqueKeyValuesWithPartialPredicateConflictDetectionCases(t *testing.T) {
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test7",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test7",
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

	err = lm.WaitForCutoverComplete(30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

}

func TestLiveMigrationWithUniqueKeyConflictWithNullValuesDetectionCases(t *testing.T) {
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test8",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test8",
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

	err = lm.WaitForCutoverComplete(30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

}

func TestLiveMigrationWithUniqueKeyConflictWithUniqueIndexOnlyOnLeafPartitions(t *testing.T) {
	liveMigrationTest := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test9",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test9",
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

	err = liveMigrationTest.WaitForCutoverComplete(30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

}

func TestLiveMigrationWithUniqueKeyConflictWithNullValueAndPartialPredicatesDetectionCases(t *testing.T) {
	liveMigrationTest := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test10",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test10",
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

	err = liveMigrationTest.WaitForCutoverComplete(30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

}

func TestLiveMigrationWithUniqueKeyConflictWithExpressionIndexOnPartitions(t *testing.T) {
	liveMigrationTest := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test11",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test11",
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

func TestLiveMigrationWithLargeNumberOfColumns(t *testing.T) {
	liveMigrationTest := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test13",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test13",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.test_large_number_of_columns (
				id int PRIMARY KEY,
				column1 text,
				column2 text,
				column3 text,
				column4 text,
				column5 text,
				column6 text,
				column7 text,
				column8 text,
				column9 text,
				column10 text,
				column11 text,
				column12 text,
				column13 text,
				column14 text,
				column15 text,
				column16 text,
				column17 text,
				column18 text,
				column19 text,
				column20 text,
				column21 text,
				column22 text,
				column23 text,
				column24 text,
				column25 text,
				column26 text,
				column27 text,
				column28 text,
				column29 text,
				column30 text,
				column31 text,
				column32 text,
				column33 text,
				column34 text,
				column35 text,
				column36 text,
				column37 text,
				column38 text,
				column39 text,
				column40 text,
				column41 text,
				column42 text,
				column43 text,
				column44 text,
				column45 text,
				column46 text,
				column47 text,
				column48 text,
				column49 text,
				column50 text
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.test_large_number_of_columns REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.test_large_number_of_columns (id, column1, column2, column3, column4, column5, column6, column7, column8, column9, column10, column11, column12, column13, column14, column15, column16, column17, column18, column19, column20, column21, column22, column23, column24, column25,
			column26, column27, column28, column29, column30, column31, column32, column33, column34, column35, column36, column37, column38, column39, column40, column41, column42, column43, column44, column45, column46, column47, column48, column49, column50)
			SELECT i, md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), 
			md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), 
			md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text)
			FROM generate_series(1, 20) as i;`,
		},
		SourceDeltaSQL: []string{
			/*
				changes
				Inserting 500 events

				I event with  25 columns
				U above row and all 25 columns
				U above row and 20 columns
				U above row and 15 columns
				U above row and 10 columns
				U above row and 8 columns
				U above row and 6 columns

				D

				I
				and same updates again
			*/
			`
DO $$
DECLARE
    i INTEGER;
BEGIN
    FOR i IN 21..220 LOOP
        INSERT INTO test_schema.test_large_number_of_columns (id, column1, column2, column3, column4, column5, column6, column7, column8, column9, column10, column11, column12, column13, column14, column15, column16, column17, column18, column19, column20, column21, column22, column23, column24, column25,
		column26, column27, column28, column29, column30, column31, column32, column33, column34, column35, column36, column37, column38, column39, column40, column41, column42, column43, column44, column45, column46, column47, column48, column49, column50)
		SELECT i, md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), 
			md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), 
			md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text);
		
		UPDATE test_schema.test_large_number_of_columns SET column1 = 'updated_column1' || i , column2 = 'updated_column2' || i , column3 = 'updated_column3' || i , column4 = 'updated_column4' || i , column5 = 'updated_column5' || i , column6 = 'updated_column6' || i , 
		column7 = 'updated_column7' || i , column8 = 'updated_column8' || i , column9 = 'updated_column9' || i , column10 = 'updated_column10' || i , column11 = 'updated_column11' || i , column12 = 'updated_column12' || i , column13 = 'updated_column13' || i , column14 = 'updated_column14' || i , column15 = 'updated_column15' || i , column16 = 'updated_column16' || i , 
		column17 = 'updated_column17' || i , column18 = 'updated_column18' || i , column19 = 'updated_column19' || i , column20 = 'updated_column20' || i , column21 = 'updated_column21' || i , 
		column22 = 'updated_column22' || i , column23 = 'updated_column23' || i , column24 = 'updated_column24' || i , column25 = 'updated_column25' || i , column26 = 'updated_column26' || i , column27 = 'updated_column27' || i , column28 = 'updated_column28' || i , column29 = 'updated_column29' || i , column30 = 'updated_column30' || i , column31 = 'updated_column31' || i , column32 = 'updated_column32' || i , 
		column33 = 'updated_column33' || i , column34 = 'updated_column34' || i , column35 = 'updated_column35' || i , column36 = 'updated_column36' || i , column37 = 'updated_column37' || i , column38 = 'updated_column38' || i , column39 = 'updated_column39' || i , column40 = 'updated_column40' || i , column41 = 'updated_column41' || i , column42 = 'updated_column42' || i , column43 = 'updated_column43' || i , 
		column44 = 'updated_column44' || i , column45 = 'updated_column45' || i , column46 = 'updated_column46' || i , column47 = 'updated_column47' || i , column48 = 'updated_column48' || i , column49 = 'updated_column49' || i , column50 = 'updated_column50' || i WHERE id = i;

		UPDATE test_schema.test_large_number_of_columns SET column1 = 'updated_column1' || i+1 , column2 = 'updated_column2' || i+1 , column3 = 'updated_column3' || i+1 , column4 = 'updated_column4' || i+1 , column5 = 'updated_column5' || i+1 , column6 = 'updated_column6' || i+1 , 
		column7 = 'updated_column7' || i+1 , column8 = 'updated_column8' || i+1 , column9 = 'updated_column9' || i+1 , column10 = 'updated_column10' || i+1 , column11 = 'updated_column11' || i+1 , column12 = 'updated_column12' || i+1 , column13 = 'updated_column13' || i+1 , column14 = 'updated_column14' || i+1 , column15 = 'updated_column15' || i+1 , 
		column16 = 'updated_column16' || i+1 , column17 = 'updated_column17' || i+1 , column18 = 'updated_column18' || i+1 , column19 = 'updated_column19' || i+1 , column20 = 'updated_column20' || i+1 , column21 = 'updated_column21' || i+1 , 
		column22 = 'updated_column22' || i+1 , column23 = 'updated_column23' || i+1 , column24 = 'updated_column24' || i+1 , column25 = 'updated_column25' || i+1 , column26 = 'updated_column26' || i+1 , column27 = 'updated_column27' || i+1 , column28 = 'updated_column28' || i+1 , column29 = 'updated_column29' || i+1 , column30 = 'updated_column30' || i+1 , column31 = 'updated_column31' || i+1 , column32 = 'updated_column32' || i+1 , 
		column33 = 'updated_column33' || i+1 , column34 = 'updated_column34' || i+1 , column35 = 'updated_column35' || i+1 , column36 = 'updated_column36' || i+1 , column37 = 'updated_column37' || i+1 , column38 = 'updated_column38' || i+1 , column39 = 'updated_column39' || i+1 , column40 = 'updated_column40' || i+1 WHERE id = i;

		UPDATE test_schema.test_large_number_of_columns SET column1 = 'updated_column1' || i+2 , column2 = 'updated_column2' || i+2 , column3 = 'updated_column3' || i+2 , column4 = 'updated_column4' || i+2 , column5 = 'updated_column5' || i+2 , column6 = 'updated_column6' || i+2 , 
		column7 = 'updated_column7' || i+2 , column8 = 'updated_column8' || i+2 , column9 = 'updated_column9' || i+2 , column10 = 'updated_column10' || i+2 , column11 = 'updated_column11' || i+2 , column12 = 'updated_column12' || i+2 , column13 = 'updated_column13' || i+2 , column14 = 'updated_column14' || i+2 , column15 = 'updated_column15' || i+2 , column16 = 'updated_column16' || i+2 , column17 = 'updated_column17' || i+2 , column18 = 'updated_column18' || i+2 , column19 = 'updated_column19' || i+2 , column20 = 'updated_column20' || i+2 , column21 = 'updated_column21' || i+2 , 
		column22 = 'updated_column22' || i+2 , column23 = 'updated_column23' || i+2 , column24 = 'updated_column24' || i+2 , column25 = 'updated_column25' || i+2 , column26 = 'updated_column26' || i+2 , column27 = 'updated_column27' || i+2 , column28 = 'updated_column28' || i+2 , column29 = 'updated_column29' || i+2 , column30 = 'updated_column30' || i+2 , column31 = 'updated_column31' || i+2 , column32 = 'updated_column32' || i+2 , 
		column33 = 'updated_column33' || i+2 , column34 = 'updated_column34' || i+2 , column35 = 'updated_column35' || i+2  WHERE id = i;


		UPDATE test_schema.test_large_number_of_columns SET column1 = 'updated_column1' || i+3 , column2 = 'updated_column2' || i+3 , column3 = 'updated_column3' || i+3 , column4 = 'updated_column4' || i+3 , column5 = 'updated_column5' || i+3 , column6 = 'updated_column6' || i+3 , 
		column7 = 'updated_column7' || i+3 , column8 = 'updated_column8' || i+3 , column9 = 'updated_column9' || i+3 , column10 = 'updated_column10' || i+3 , column11 = 'updated_column11' || i+3 , column12 = 'updated_column12' || i+3 , column13 = 'updated_column13' || i+3 , column14 = 'updated_column14' || i+3 , column15 = 'updated_column15' || i+3 , column16 = 'updated_column16' || i+3 , column17 = 'updated_column17' || i+3 , column18 = 'updated_column18' || i+3 , column19 = 'updated_column19' || i+3 , column20 = 'updated_column20' || i+3 , column21 = 'updated_column21' || i+3 , 
		column22 = 'updated_column22' || i+3 , column23 = 'updated_column23' || i+3 , column24 = 'updated_column24' || i+3 , column25 = 'updated_column25' || i+3 , column26 = 'updated_column26' || i+3 , column27 = 'updated_column27' || i+3 , column28 = 'updated_column28' || i+3 , column29 = 'updated_column29' || i+3 , column30 = 'updated_column30' || i+3 WHERE id = i;

		UPDATE test_schema.test_large_number_of_columns SET column1 = 'updated_column1' || i+4 , column2 = 'updated_column2' || i+4 , column3 = 'updated_column3' || i+4 , column4 = 'updated_column4' || i+4 , column5 = 'updated_column5' || i+4 , column6 = 'updated_column6' || i+4 , 
		column7 = 'updated_column7' || i+4 , column8 = 'updated_column8' || i+4 , column9 = 'updated_column9' || i+4 , column10 = 'updated_column10' || i+4 , column11 = 'updated_column11' || i+4 , column12 = 'updated_column12' || i+4 , column13 = 'updated_column13' || i+4 , column14 = 'updated_column14' || i+4 , column15 = 'updated_column15' || i+4 , column16 = 'updated_column16' || i+4 , column17 = 'updated_column17' || i+4 , column18 = 'updated_column18' || i+4 , column19 = 'updated_column19' || i+4 , column20 = 'updated_column20' || i+4 , column21 = 'updated_column21' || i+4 , 
		column22 = 'updated_column22' || i+4 , column23 = 'updated_column23' || i+4 , column24 = 'updated_column24' || i+4 , column25 = 'updated_column25' || i+4 , column26 = 'updated_column26' || i+4 , column27 = 'updated_column27' || i+4 WHERE id = i;

		UPDATE test_schema.test_large_number_of_columns SET column1 = 'updated_column1' || i+5 , column2 = 'updated_column2' || i+5 , column3 = 'updated_column3' || i+5 , column4 = 'updated_column4' || i+5 , column5 = 'updated_column5' || i+5 , column6 = 'updated_column6' || i+5 , 
		column7 = 'updated_column7' || i+5 , column8 = 'updated_column8' || i+5 , column9 = 'updated_column9' || i+5 , column10 = 'updated_column10' || i+5 , column11 = 'updated_column11' || i+5 , column12 = 'updated_column12' || i+5 , column13 = 'updated_column13' || i+5 , column14 = 'updated_column14' || i+5 , column15 = 'updated_column15' || i+5 , column16 = 'updated_column16' || i+5 , column17 = 'updated_column17' || i+5 , column18 = 'updated_column18' || i+5 , column19 = 'updated_column19' || i+5 , column20 = 'updated_column20' || i+5 WHERE id = i;

		DELETE FROM test_schema.test_large_number_of_columns WHERE id = i;
		INSERT INTO test_schema.test_large_number_of_columns (id, column1, column2, column3, column4, column5, column6, column7, column8, column9, column10, column11, column12, column13, column14, column15, column16, column17, column18, column19, column20, column21, column22, column23, column24, column25,
		column26, column27, column28, column29, column30, column31, column32, column33, column34, column35, column36, column37, column38, column39, column40, column41, column42, column43, column44, column45, column46, column47, column48, column49, column50)
		SELECT i, md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), 
			md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), 
			md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text);

		UPDATE test_schema.test_large_number_of_columns SET column1 = 'updated_column1' || i+6 , column2 = 'updated_column2' || i+6 , column3 = 'updated_column3' || i+6 , column4 = 'updated_column4' || i+6 , column5 = 'updated_column5' || i+6 , column6 = 'updated_column6' || i+6 , 
		column7 = 'updated_column7' || i+6 , column8 = 'updated_column8' || i+6 , column9 = 'updated_column9' || i+6 , column10 = 'updated_column10' || i+6 , column11 = 'updated_column11' || i+6 , column12 = 'updated_column12' || i+6 , column13 = 'updated_column13' || i+6 , column14 = 'updated_column14' || i+6 , column15 = 'updated_column15' || i+6 , column16 = 'updated_column16' || i+6 , 
		column17 = 'updated_column17' || i+6 , column18 = 'updated_column18' || i+6 , column19 = 'updated_column19' || i+6 , column20 = 'updated_column20' || i+6 , column21 = 'updated_column21' || i+6 , 
		column22 = 'updated_column22' || i+6 , column23 = 'updated_column23' || i+6 , column24 = 'updated_column24' || i+6 , column25 = 'updated_column25' || i+6 , column26 = 'updated_column26' || i+6 , column27 = 'updated_column27' || i+6 , column28 = 'updated_column28' || i+6 , column29 = 'updated_column29' || i+6 , column30 = 'updated_column30' || i+6 , column31 = 'updated_column31' || i+6 , column32 = 'updated_column32' || i+6 , 
		column33 = 'updated_column33' || i+6 , column34 = 'updated_column34' || i+6 , column35 = 'updated_column35' || i+6 , column36 = 'updated_column36' || i+6 , column37 = 'updated_column37' || i+6 , column38 = 'updated_column38' || i+6 , column39 = 'updated_column39' || i+6 , column40 = 'updated_column40' || i+6 , column41 = 'updated_column41' || i+6 , column42 = 'updated_column42' || i+6 , column43 = 'updated_column43' || i+6 , 
		column44 = 'updated_column44' || i+6 , column45 = 'updated_column45' || i+6 , column46 = 'updated_column46' || i+6 , column47 = 'updated_column47' || i+6 , column48 = 'updated_column48' || i+6 , column49 = 'updated_column49' || i+6 , column50 = 'updated_column50' || i+6 WHERE id = i;

		UPDATE test_schema.test_large_number_of_columns SET column1 = 'updated_column1' || i+7 , column2 = 'updated_column2' || i+7 , column3 = 'updated_column3' || i+7 , column4 = 'updated_column4' || i+7 , column5 = 'updated_column5' || i+7 , column6 = 'updated_column6' || i+7 , 
		column7 = 'updated_column7' || i+1 , column8 = 'updated_column8' || i+7 , column9 = 'updated_column9' || i+7 , column10 = 'updated_column10' || i+7 , column11 = 'updated_column11' || i+7 , column12 = 'updated_column12' || i+7 , column13 = 'updated_column13' || i+7 , column14 = 'updated_column14' || i+7 , column15 = 'updated_column15' || i+7 , 
		column16 = 'updated_column16' || i+7 , column17 = 'updated_column17' || i+7 , column18 = 'updated_column18' || i+7 , column19 = 'updated_column19' || i+7 , column20 = 'updated_column20' || i+7 , column21 = 'updated_column21' || i+7 , 
		column22 = 'updated_column22' || i+7 , column23 = 'updated_column23' || i+7 , column24 = 'updated_column24' || i+7 , column25 = 'updated_column25' || i+7, column26 = 'updated_column26' || i+7 , column27 = 'updated_column27' || i+7 , column28 = 'updated_column28' || i+7 , column29 = 'updated_column29' || i+7 , column30 = 'updated_column30' || i+7 , column31 = 'updated_column31' || i+7 , column32 = 'updated_column32' || i+7 , 
		column33 = 'updated_column33' || i+7 , column34 = 'updated_column34' || i+7 , column35 = 'updated_column35' || i+7 , column36 = 'updated_column36' || i+7 , column37 = 'updated_column37' || i+7 , column38 = 'updated_column38' || i+7 , column39 = 'updated_column39' || i+7 , column40 = 'updated_column40' || i+7 WHERE id = i;

		UPDATE test_schema.test_large_number_of_columns SET column1 = 'updated_column1' || i+8 , column2 = 'updated_column2' || i+8 , column3 = 'updated_column3' || i+8 , column4 = 'updated_column4' || i+8 , column5 = 'updated_column5' || i+8 , column6 = 'updated_column6' || i+8 , 
		column7 = 'updated_column7' || i+8 , column8 = 'updated_column8' || i+8 , column9 = 'updated_column9' || i+8 , column10 = 'updated_column10' || i+8 , column11 = 'updated_column11' || i+8 , column12 = 'updated_column12' || i+8 , column13 = 'updated_column13' || i+8 , column14 = 'updated_column14' || i+8 , column15 = 'updated_column15' || i+8, 
		column16 = 'updated_column16' || i+8 , column17 = 'updated_column17' || i+8 , column18 = 'updated_column18' || i+8 , column19 = 'updated_column19' || i+8 , column20 = 'updated_column20' || i+8 , column21 = 'updated_column21' || i+8 , 
		column22 = 'updated_column22' || i+8 , column23 = 'updated_column23' || i+8 , column24 = 'updated_column24' || i+8 , column25 = 'updated_column25' || i+8 WHERE id = i;


		UPDATE test_schema.test_large_number_of_columns SET column1 = 'updated_column1' || i+9 , column2 = 'updated_column2' || i+9 , column3 = 'updated_column3' || i+9 , column4 = 'updated_column4' || i+9 , column5 = 'updated_column5' || i+9 , column6 = 'updated_column6' || i+9 , 
		column7 = 'updated_column7' || i+9 , column8 = 'updated_column8' || i+9 , column9 = 'updated_column9' || i+9 , column10 = 'updated_column10' || i+9, column16 = 'updated_column16' || i+9 , column17 = 'updated_column17' || i+9 , column18 = 'updated_column18' || i+9   WHERE id = i;

    END LOOP;
END $$;
			`,
		},
		TargetDeltaSQL: []string{
			/*
				changes
				Inserting 500 events

				I event with  25 columns
				U above row and all 25 columns
				U above row and 20 columns
				U above row and 15 columns
				U above row and 10 columns
				U above row and 8 columns
				U above row and 6 columns

				D

				I
				and same updates again
			*/
			`
DO $$
DECLARE
    i INTEGER;
BEGIN
    FOR i IN 221..520 LOOP
        INSERT INTO test_schema.test_large_number_of_columns (id, column1, column2, column3, column4, column5, column6, column7, column8, column9, column10, column11, column12, column13, column14, column15, column16, column17, column18, column19, column20, column21, column22, column23, column24, column25,
		column26, column27, column28, column29, column30, column31, column32, column33, column34, column35, column36, column37, column38, column39, column40, column41, column42, column43, column44, column45, column46, column47, column48, column49, column50)
		SELECT i, md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), 
			md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), 
			md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text);
		
		UPDATE test_schema.test_large_number_of_columns SET column1 = 'updated_column1' || i , column2 = 'updated_column2' || i , column3 = 'updated_column3' || i , column4 = 'updated_column4' || i , column5 = 'updated_column5' || i , column6 = 'updated_column6' || i , 
		column7 = 'updated_column7' || i , column8 = 'updated_column8' || i , column9 = 'updated_column9' || i , column10 = 'updated_column10' || i , column11 = 'updated_column11' || i , column12 = 'updated_column12' || i , column13 = 'updated_column13' || i , column14 = 'updated_column14' || i , column15 = 'updated_column15' || i , column16 = 'updated_column16' || i , 
		column17 = 'updated_column17' || i , column18 = 'updated_column18' || i , column19 = 'updated_column19' || i , column20 = 'updated_column20' || i , column21 = 'updated_column21' || i , 
		column22 = 'updated_column22' || i , column23 = 'updated_column23' || i , column24 = 'updated_column24' || i , column25 = 'updated_column25' || i , column26 = 'updated_column26' || i , column27 = 'updated_column27' || i , column28 = 'updated_column28' || i , column29 = 'updated_column29' || i , column30 = 'updated_column30' || i , column31 = 'updated_column31' || i , column32 = 'updated_column32' || i , 
		column33 = 'updated_column33' || i , column34 = 'updated_column34' || i , column35 = 'updated_column35' || i , column36 = 'updated_column36' || i , column37 = 'updated_column37' || i , column38 = 'updated_column38' || i , column39 = 'updated_column39' || i , column40 = 'updated_column40' || i , column41 = 'updated_column41' || i , column42 = 'updated_column42' || i , column43 = 'updated_column43' || i , 
		column44 = 'updated_column44' || i , column45 = 'updated_column45' || i , column46 = 'updated_column46' || i , column47 = 'updated_column47' || i , column48 = 'updated_column48' || i , column49 = 'updated_column49' || i , column50 = 'updated_column50' || i WHERE id = i;

		UPDATE test_schema.test_large_number_of_columns SET column1 = 'updated_column1' || i+1 , column2 = 'updated_column2' || i+1 , column3 = 'updated_column3' || i+1 , column4 = 'updated_column4' || i+1 , column5 = 'updated_column5' || i+1 , column6 = 'updated_column6' || i+1 , 
		column7 = 'updated_column7' || i+1 , column8 = 'updated_column8' || i+1 , column9 = 'updated_column9' || i+1 , column10 = 'updated_column10' || i+1 , column11 = 'updated_column11' || i+1 , column12 = 'updated_column12' || i+1 , column13 = 'updated_column13' || i+1 , column14 = 'updated_column14' || i+1 , column15 = 'updated_column15' || i+1 , 
		column16 = 'updated_column16' || i+1 , column17 = 'updated_column17' || i+1 , column18 = 'updated_column18' || i+1 , column19 = 'updated_column19' || i+1 , column20 = 'updated_column20' || i+1 , column21 = 'updated_column21' || i+1 , 
		column22 = 'updated_column22' || i+1 , column23 = 'updated_column23' || i+1 , column24 = 'updated_column24' || i+1 , column25 = 'updated_column25' || i+1 , column26 = 'updated_column26' || i+1 , column27 = 'updated_column27' || i+1 , column28 = 'updated_column28' || i+1 , column29 = 'updated_column29' || i+1 , column30 = 'updated_column30' || i+1 , column31 = 'updated_column31' || i+1 , column32 = 'updated_column32' || i+1 , 
		column33 = 'updated_column33' || i+1 , column34 = 'updated_column34' || i+1 , column35 = 'updated_column35' || i+1 , column36 = 'updated_column36' || i+1 , column37 = 'updated_column37' || i+1 , column38 = 'updated_column38' || i+1 , column39 = 'updated_column39' || i+1 , column40 = 'updated_column40' || i+1 WHERE id = i;

		UPDATE test_schema.test_large_number_of_columns SET column1 = 'updated_column1' || i+2 , column2 = 'updated_column2' || i+2 , column3 = 'updated_column3' || i+2 , column4 = 'updated_column4' || i+2 , column5 = 'updated_column5' || i+2 , column6 = 'updated_column6' || i+2 , 
		column7 = 'updated_column7' || i+2 , column8 = 'updated_column8' || i+2 , column9 = 'updated_column9' || i+2 , column10 = 'updated_column10' || i+2 , column11 = 'updated_column11' || i+2 , column12 = 'updated_column12' || i+2 , column13 = 'updated_column13' || i+2 , column14 = 'updated_column14' || i+2 , column15 = 'updated_column15' || i+2 , column16 = 'updated_column16' || i+2 , column17 = 'updated_column17' || i+2 , column18 = 'updated_column18' || i+2 , column19 = 'updated_column19' || i+2 , column20 = 'updated_column20' || i+2 , column21 = 'updated_column21' || i+2 , 
		column22 = 'updated_column22' || i+2 , column23 = 'updated_column23' || i+2 , column24 = 'updated_column24' || i+2 , column25 = 'updated_column25' || i+2 , column26 = 'updated_column26' || i+2 , column27 = 'updated_column27' || i+2 , column28 = 'updated_column28' || i+2 , column29 = 'updated_column29' || i+2 , column30 = 'updated_column30' || i+2 , column31 = 'updated_column31' || i+2 , column32 = 'updated_column32' || i+2 , 
		column33 = 'updated_column33' || i+2 , column34 = 'updated_column34' || i+2 , column35 = 'updated_column35' || i+2  WHERE id = i;


		UPDATE test_schema.test_large_number_of_columns SET column1 = 'updated_column1' || i+3 , column2 = 'updated_column2' || i+3 , column3 = 'updated_column3' || i+3 , column4 = 'updated_column4' || i+3 , column5 = 'updated_column5' || i+3 , column6 = 'updated_column6' || i+3 , 
		column7 = 'updated_column7' || i+3 , column8 = 'updated_column8' || i+3 , column9 = 'updated_column9' || i+3 , column10 = 'updated_column10' || i+3 , column11 = 'updated_column11' || i+3 , column12 = 'updated_column12' || i+3 , column13 = 'updated_column13' || i+3 , column14 = 'updated_column14' || i+3 , column15 = 'updated_column15' || i+3 , column16 = 'updated_column16' || i+3 , column17 = 'updated_column17' || i+3 , column18 = 'updated_column18' || i+3 , column19 = 'updated_column19' || i+3 , column20 = 'updated_column20' || i+3 , column21 = 'updated_column21' || i+3 , 
		column22 = 'updated_column22' || i+3 , column23 = 'updated_column23' || i+3 , column24 = 'updated_column24' || i+3 , column25 = 'updated_column25' || i+3 , column26 = 'updated_column26' || i+3 , column27 = 'updated_column27' || i+3 , column28 = 'updated_column28' || i+3 , column29 = 'updated_column29' || i+3 , column30 = 'updated_column30' || i+3 WHERE id = i;

		UPDATE test_schema.test_large_number_of_columns SET column1 = 'updated_column1' || i+4 , column2 = 'updated_column2' || i+4 , column3 = 'updated_column3' || i+4 , column4 = 'updated_column4' || i+4 , column5 = 'updated_column5' || i+4 , column6 = 'updated_column6' || i+4 , 
		column7 = 'updated_column7' || i+4 , column8 = 'updated_column8' || i+4 , column9 = 'updated_column9' || i+4 , column10 = 'updated_column10' || i+4 , column11 = 'updated_column11' || i+4 , column12 = 'updated_column12' || i+4 , column13 = 'updated_column13' || i+4 , column14 = 'updated_column14' || i+4 , column15 = 'updated_column15' || i+4 , column16 = 'updated_column16' || i+4 , column17 = 'updated_column17' || i+4 , column18 = 'updated_column18' || i+4 , column19 = 'updated_column19' || i+4 , column20 = 'updated_column20' || i+4 , column21 = 'updated_column21' || i+4 , 
		column22 = 'updated_column22' || i+4 , column23 = 'updated_column23' || i+4 , column24 = 'updated_column24' || i+4 , column25 = 'updated_column25' || i+4 , column26 = 'updated_column26' || i+4 , column27 = 'updated_column27' || i+4 WHERE id = i;

		UPDATE test_schema.test_large_number_of_columns SET column1 = 'updated_column1' || i+5 , column2 = 'updated_column2' || i+5 , column3 = 'updated_column3' || i+5 , column4 = 'updated_column4' || i+5 , column5 = 'updated_column5' || i+5 , column6 = 'updated_column6' || i+5 , 
		column7 = 'updated_column7' || i+5 , column8 = 'updated_column8' || i+5 , column9 = 'updated_column9' || i+5 , column10 = 'updated_column10' || i+5 , column11 = 'updated_column11' || i+5 , column12 = 'updated_column12' || i+5 , column13 = 'updated_column13' || i+5 , column14 = 'updated_column14' || i+5 , column15 = 'updated_column15' || i+5 , column16 = 'updated_column16' || i+5 , column17 = 'updated_column17' || i+5 , column18 = 'updated_column18' || i+5 , column19 = 'updated_column19' || i+5 , column20 = 'updated_column20' || i+5 WHERE id = i;

		DELETE FROM test_schema.test_large_number_of_columns WHERE id = i;
		INSERT INTO test_schema.test_large_number_of_columns (id, column1, column2, column3, column4, column5, column6, column7, column8, column9, column10, column11, column12, column13, column14, column15, column16, column17, column18, column19, column20, column21, column22, column23, column24, column25,
		column26, column27, column28, column29, column30, column31, column32, column33, column34, column35, column36, column37, column38, column39, column40, column41, column42, column43, column44, column45, column46, column47, column48, column49, column50)
		SELECT i, md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), 
			md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), 
			md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text), md5(random()::text);

		UPDATE test_schema.test_large_number_of_columns SET column1 = 'updated_column1' || i+6 , column2 = 'updated_column2' || i+6 , column3 = 'updated_column3' || i+6 , column4 = 'updated_column4' || i+6 , column5 = 'updated_column5' || i+6 , column6 = 'updated_column6' || i+6 , 
		column7 = 'updated_column7' || i+6 , column8 = 'updated_column8' || i+6 , column9 = 'updated_column9' || i+6 , column10 = 'updated_column10' || i+6 , column11 = 'updated_column11' || i+6 , column12 = 'updated_column12' || i+6 , column13 = 'updated_column13' || i+6 , column14 = 'updated_column14' || i+6 , column15 = 'updated_column15' || i+6 , column16 = 'updated_column16' || i+6 , 
		column17 = 'updated_column17' || i+6 , column18 = 'updated_column18' || i+6 , column19 = 'updated_column19' || i+6 , column20 = 'updated_column20' || i+6 , column21 = 'updated_column21' || i+6 , 
		column22 = 'updated_column22' || i+6 , column23 = 'updated_column23' || i+6 , column24 = 'updated_column24' || i+6 , column25 = 'updated_column25' || i+6 , column26 = 'updated_column26' || i+6 , column27 = 'updated_column27' || i+6 , column28 = 'updated_column28' || i+6 , column29 = 'updated_column29' || i+6 , column30 = 'updated_column30' || i+6 , column31 = 'updated_column31' || i+6 , column32 = 'updated_column32' || i+6 , 
		column33 = 'updated_column33' || i+6 , column34 = 'updated_column34' || i+6 , column35 = 'updated_column35' || i+6 , column36 = 'updated_column36' || i+6 , column37 = 'updated_column37' || i+6 , column38 = 'updated_column38' || i+6 , column39 = 'updated_column39' || i+6 , column40 = 'updated_column40' || i+6 , column41 = 'updated_column41' || i+6 , column42 = 'updated_column42' || i+6 , column43 = 'updated_column43' || i+6 , 
		column44 = 'updated_column44' || i+6 , column45 = 'updated_column45' || i+6 , column46 = 'updated_column46' || i+6 , column47 = 'updated_column47' || i+6 , column48 = 'updated_column48' || i+6 , column49 = 'updated_column49' || i+6 , column50 = 'updated_column50' || i+6 WHERE id = i;

		UPDATE test_schema.test_large_number_of_columns SET column1 = 'updated_column1' || i+7 , column2 = 'updated_column2' || i+7 , column3 = 'updated_column3' || i+7 , column4 = 'updated_column4' || i+7 , column5 = 'updated_column5' || i+7 , column6 = 'updated_column6' || i+7 , 
		column7 = 'updated_column7' || i+1 , column8 = 'updated_column8' || i+7 , column9 = 'updated_column9' || i+7 , column10 = 'updated_column10' || i+7 , column11 = 'updated_column11' || i+7 , column12 = 'updated_column12' || i+7 , column13 = 'updated_column13' || i+7 , column14 = 'updated_column14' || i+7 , column15 = 'updated_column15' || i+7 , 
		column16 = 'updated_column16' || i+7 , column17 = 'updated_column17' || i+7 , column18 = 'updated_column18' || i+7 , column19 = 'updated_column19' || i+7 , column20 = 'updated_column20' || i+7 , column21 = 'updated_column21' || i+7 , 
		column22 = 'updated_column22' || i+7 , column23 = 'updated_column23' || i+7 , column24 = 'updated_column24' || i+7 , column25 = 'updated_column25' || i+7, column26 = 'updated_column26' || i+7 , column27 = 'updated_column27' || i+7 , column28 = 'updated_column28' || i+7 , column29 = 'updated_column29' || i+7 , column30 = 'updated_column30' || i+7 , column31 = 'updated_column31' || i+7 , column32 = 'updated_column32' || i+7 , 
		column33 = 'updated_column33' || i+7 , column34 = 'updated_column34' || i+7 , column35 = 'updated_column35' || i+7 , column36 = 'updated_column36' || i+7 , column37 = 'updated_column37' || i+7 , column38 = 'updated_column38' || i+7 , column39 = 'updated_column39' || i+7 , column40 = 'updated_column40' || i+7 WHERE id = i;

		UPDATE test_schema.test_large_number_of_columns SET column1 = 'updated_column1' || i+8 , column2 = 'updated_column2' || i+8 , column3 = 'updated_column3' || i+8 , column4 = 'updated_column4' || i+8 , column5 = 'updated_column5' || i+8 , column6 = 'updated_column6' || i+8 , 
		column7 = 'updated_column7' || i+8 , column8 = 'updated_column8' || i+8 , column9 = 'updated_column9' || i+8 , column10 = 'updated_column10' || i+8 , column11 = 'updated_column11' || i+8 , column12 = 'updated_column12' || i+8 , column13 = 'updated_column13' || i+8 , column14 = 'updated_column14' || i+8 , column15 = 'updated_column15' || i+8, 
		column16 = 'updated_column16' || i+8 , column17 = 'updated_column17' || i+8 , column18 = 'updated_column18' || i+8 , column19 = 'updated_column19' || i+8 , column20 = 'updated_column20' || i+8 , column21 = 'updated_column21' || i+8 , 
		column22 = 'updated_column22' || i+8 , column23 = 'updated_column23' || i+8 , column24 = 'updated_column24' || i+8 , column25 = 'updated_column25' || i+8 WHERE id = i;


		UPDATE test_schema.test_large_number_of_columns SET column1 = 'updated_column1' || i+9 , column2 = 'updated_column2' || i+9 , column3 = 'updated_column3' || i+9 , column4 = 'updated_column4' || i+9 , column5 = 'updated_column5' || i+9 , column6 = 'updated_column6' || i+9 , 
		column7 = 'updated_column7' || i+9 , column8 = 'updated_column8' || i+9 , column9 = 'updated_column9' || i+9 , column10 = 'updated_column10' || i+9, column16 = 'updated_column16' || i+9 , column17 = 'updated_column17' || i+9 , column18 = 'updated_column18' || i+9   WHERE id = i;

    END LOOP;
END $$;
			`,
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
		`test_schema."test_large_number_of_columns"`: 20,
	}, 30)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	err = liveMigrationTest.ValidateDataConsistency([]string{`test_schema."test_large_number_of_columns"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = liveMigrationTest.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	err = liveMigrationTest.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`test_schema."test_large_number_of_columns"`: {
			Inserts: 400,
			Updates: 2000,
			Deletes: 200,
		},
	}, 120, 5)
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	err = liveMigrationTest.ValidateDataConsistency([]string{`test_schema."test_large_number_of_columns"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = liveMigrationTest.InitiateCutoverToTarget(true, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to target")

	err = liveMigrationTest.WaitForCutoverComplete(50)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

	err = liveMigrationTest.ExecuteTargetDelta()
	testutils.FatalIfError(t, err, "failed to execute target delta")

	err = liveMigrationTest.WaitForFallbackStreamingComplete(map[string]ChangesCount{
		`test_schema."test_large_number_of_columns"`: {
			Inserts: 600,
			Updates: 3000,
			Deletes: 300,
		},
	}, 120, 5)
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	err = liveMigrationTest.ValidateDataConsistency([]string{`test_schema."test_large_number_of_columns"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = liveMigrationTest.InitiateCutoverToSource(nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to source")

	err = liveMigrationTest.WaitForCutoverSourceComplete(150)
	testutils.FatalIfError(t, err, "failed to wait for cutover source complete")
}

func TestLiveMigrationWithLargeColumnNames(t *testing.T) {
	liveMigrationTest := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test14",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test14",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			//table column - 60 chars, 55 chars, 40 chars
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.test_large_column_name(
				id int PRIMARY KEY,
				someveryyyverryyyyverryyyverryyyveryylongcolumnnnnnameeeeeee text,
				someveryyyverryyyyverryyyverryyylongcolumnnnnnameeeeeee1 text,
				someveryyyverryyyyverryyyverryyylongcolumnnnnnameeeeeee2 text,
				someveryyverryyylongcolumnnnnnameeeeeee3 text,
				someveryyyverryyyyverryyyverryyylongcolumnnnnnameeeeeee4 text
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.test_large_column_name REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.test_large_column_name
			SELECT i,  md5(random()::text),  md5(random()::text),  md5(random()::text),  md5(random()::text),  md5(random()::text) from generate_series(1,20) as i;`,
		},
		SourceDeltaSQL: []string{
			`
DO $$
DECLARE
    i INTEGER;
BEGIN
    FOR i IN 21..520 LOOP
		INSERT INTO test_schema.test_large_column_name
			SELECT i,  md5(random()::text),  md5(random()::text),  md5(random()::text),  md5(random()::text),  md5(random()::text);

		UPDATE test_schema.test_large_column_name set someveryyyverryyyyverryyyverryyylongcolumnnnnnameeeeeee4 = 'updated_datafor_large_column' || i, someveryyyverryyyyverryyyverryyylongcolumnnnnnameeeeeee1= 'updated_data_for_another_large_columns' || i,
		someveryyverryyylongcolumnnnnnameeeeeee3 = 'updated data for one more column' || i where id=i;


		UPDATE test_schema.test_large_column_name set someveryyyverryyyyverryyyverryyyveryylongcolumnnnnnameeeeeee = 'updated_datafor_large_column' || i+1, someveryyyverryyyyverryyyverryyylongcolumnnnnnameeeeeee1= 'updated_data_for_another_large_columns' || i+1,
		someveryyverryyylongcolumnnnnnameeeeeee3 = 'updated data for one more column' || i+1 where id=i;


		UPDATE test_schema.test_large_column_name set someveryyyverryyyyverryyyverryyylongcolumnnnnnameeeeeee2 = 'updated_datafor_large_column' || i+2, someveryyyverryyyyverryyyverryyylongcolumnnnnnameeeeeee1= 'updated_data_for_another_large_columns' || i+2,
		someveryyverryyylongcolumnnnnnameeeeeee3 = 'updated data for one more column' || i+2, someveryyyverryyyyverryyyverryyylongcolumnnnnnameeeeeee4 = 'updated_datafor_large_column'  || i+2 where id=i;

		DELETE from test_schema.test_large_column_name where id=i;


		INSERT INTO test_schema.test_large_column_name
		SELECT i,  md5(random()::text),  md5(random()::text),  md5(random()::text),  md5(random()::text),  md5(random()::text);

		UPDATE test_schema.test_large_column_name set someveryyyverryyyyverryyyverryyylongcolumnnnnnameeeeeee4 = 'updated_datafor_large_column' || i, someveryyyverryyyyverryyyverryyylongcolumnnnnnameeeeeee1= 'updated_data_for_another_large_columns' || i,
		someveryyverryyylongcolumnnnnnameeeeeee3 = 'updated data for one more column' || i, someveryyyverryyyyverryyyverryyylongcolumnnnnnameeeeeee2 = 'updated_datafor_large_column' || i, someveryyyverryyyyverryyyverryyyveryylongcolumnnnnnameeeeeee = 'updated_datafor_large_column' || i  where id=i;


		UPDATE test_schema.test_large_column_name set someveryyyverryyyyverryyyverryyyveryylongcolumnnnnnameeeeeee = 'updated_datafor_large_column' || i+1, someveryyyverryyyyverryyyverryyylongcolumnnnnnameeeeeee1= 'updated_data_for_another_large_columns' || i+1,
		someveryyverryyylongcolumnnnnnameeeeeee3 = 'updated data for one more column' || i+1 where id=i;


		UPDATE test_schema.test_large_column_name set someveryyyverryyyyverryyyverryyylongcolumnnnnnameeeeeee2 = 'updated_datafor_large_column' || i+2, someveryyyverryyyyverryyyverryyyveryylongcolumnnnnnameeeeeee= 'updated_data_for_another_large_columns' || i+2,
		someveryyverryyylongcolumnnnnnameeeeeee3 = 'updated data for one more column' || i+2, someveryyyverryyyyverryyyverryyylongcolumnnnnnameeeeeee4 = 'updated_datafor_large_column'  || i+2 where id=i;

	END LOOP;
END $$;
			`,
		},
		TargetDeltaSQL: []string{
			`
			DO $$
			DECLARE
				i INTEGER;
			BEGIN
				FOR i IN 521..1020 LOOP
					INSERT INTO test_schema.test_large_column_name
						SELECT i,  md5(random()::text),  md5(random()::text),  md5(random()::text),  md5(random()::text),  md5(random()::text);
			
					UPDATE test_schema.test_large_column_name set someveryyyverryyyyverryyyverryyylongcolumnnnnnameeeeeee4 = 'updated_datafor_large_column' || i, someveryyyverryyyyverryyyverryyylongcolumnnnnnameeeeeee1= 'updated_data_for_another_large_columns' || i,
					someveryyverryyylongcolumnnnnnameeeeeee3 = 'updated data for one more column' || i where id=i;
			
			
					UPDATE test_schema.test_large_column_name set someveryyyverryyyyverryyyverryyyveryylongcolumnnnnnameeeeeee = 'updated_datafor_large_column' || i+1, someveryyyverryyyyverryyyverryyylongcolumnnnnnameeeeeee1= 'updated_data_for_another_large_columns' || i+1,
					someveryyverryyylongcolumnnnnnameeeeeee3 = 'updated data for one more column' || i+1 where id=i;
			
			
					UPDATE test_schema.test_large_column_name set someveryyyverryyyyverryyyverryyylongcolumnnnnnameeeeeee2 = 'updated_datafor_large_column' || i+2, someveryyyverryyyyverryyyverryyylongcolumnnnnnameeeeeee1= 'updated_data_for_another_large_columns' || i+2,
					someveryyverryyylongcolumnnnnnameeeeeee3 = 'updated data for one more column' || i+2, someveryyyverryyyyverryyyverryyylongcolumnnnnnameeeeeee4 = 'updated_datafor_large_column'  || i+2 where id=i;
			
					DELETE from test_schema.test_large_column_name where id=i;
			
			
					INSERT INTO test_schema.test_large_column_name
					SELECT i,  md5(random()::text),  md5(random()::text),  md5(random()::text),  md5(random()::text),  md5(random()::text);
			
					UPDATE test_schema.test_large_column_name set someveryyyverryyyyverryyyverryyylongcolumnnnnnameeeeeee4 = 'updated_datafor_large_column' || i, someveryyyverryyyyverryyyverryyylongcolumnnnnnameeeeeee1= 'updated_data_for_another_large_columns' || i,
					someveryyverryyylongcolumnnnnnameeeeeee3 = 'updated data for one more column' || i, someveryyyverryyyyverryyyverryyylongcolumnnnnnameeeeeee2 = 'updated_datafor_large_column' || i, someveryyyverryyyyverryyyverryyyveryylongcolumnnnnnameeeeeee = 'updated_datafor_large_column' || i  where id=i;
			
			
					UPDATE test_schema.test_large_column_name set someveryyyverryyyyverryyyverryyyveryylongcolumnnnnnameeeeeee = 'updated_datafor_large_column' || i+1, someveryyyverryyyyverryyyverryyylongcolumnnnnnameeeeeee1= 'updated_data_for_another_large_columns' || i+1,
					someveryyverryyylongcolumnnnnnameeeeeee3 = 'updated data for one more column' || i+1 where id=i;
			
			
					UPDATE test_schema.test_large_column_name set someveryyyverryyyyverryyyverryyylongcolumnnnnnameeeeeee2 = 'updated_datafor_large_column' || i+2, someveryyyverryyyyverryyyverryyyveryylongcolumnnnnnameeeeeee= 'updated_data_for_another_large_columns' || i+2,
					someveryyverryyylongcolumnnnnnameeeeeee3 = 'updated data for one more column' || i+2, someveryyyverryyyyverryyyverryyylongcolumnnnnnameeeeeee4 = 'updated_datafor_large_column'  || i+2 where id=i;
			
				END LOOP;
			END $$;
						`,
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
		`test_schema."test_large_column_name"`: 20,
	}, 30)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	err = liveMigrationTest.ValidateDataConsistency([]string{`test_schema."test_large_column_name"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = liveMigrationTest.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	err = liveMigrationTest.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`test_schema."test_large_column_name"`: {
			Inserts: 1000,
			Updates: 3000,
			Deletes: 500,
		},
	}, 120, 5)
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	err = liveMigrationTest.ValidateDataConsistency([]string{`test_schema."test_large_column_name"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = liveMigrationTest.InitiateCutoverToTarget(true, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to target")

	err = liveMigrationTest.WaitForCutoverComplete(50)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

	err = liveMigrationTest.ExecuteTargetDelta()
	testutils.FatalIfError(t, err, "failed to execute target delta")

	err = liveMigrationTest.WaitForFallbackStreamingComplete(map[string]ChangesCount{
		`test_schema."test_large_column_name"`: {
			Inserts: 1000,
			Updates: 3000,
			Deletes: 500,
		},
	}, 120, 5)
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	err = liveMigrationTest.ValidateDataConsistency([]string{`test_schema."test_large_column_name"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = liveMigrationTest.InitiateCutoverToSource(nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to source")

	err = liveMigrationTest.WaitForCutoverSourceComplete(150)
	testutils.FatalIfError(t, err, "failed to wait for cutover source complete")

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
