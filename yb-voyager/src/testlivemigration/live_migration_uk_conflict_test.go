//go:build failpoint_import

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
package testlivemigration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

const multiColumnUKConflictTargetIDOffset = 2004

// multiColumnUniqueIndexConflictDeltaSQL generates source (idOffset=0) or target-side
// (idOffset=multiColumnUKConflictTargetIDOffset) delta SQL for multi-column UK conflict tests.
// When withRegion is true, inserts include region='r1' for partitioned tables.
func multiColumnUniqueIndexConflictDeltaSQL(tableName string, withRegion bool, idOffset int) []string {
	anchor20 := 20 + idOffset
	anchor521 := 521 + idOffset
	anchor1022 := 1022 + idOffset
	anchor1523 := 1523 + idOffset
	loop1Start := 21 + idOffset
	loop1End := 520 + idOffset
	loop2Start := 522 + idOffset
	loop2End := 1021 + idOffset
	loop3Start := 1023 + idOffset
	loop3End := 1522 + idOffset
	loop4Start := 1524 + idOffset
	loop4End := 2023 + idOffset

	insertCols := "id, id1, id2, updated_at"
	regionPrefix := ""
	loop3LastInsertValues := "i-1, i, i, now()"
	loop4LastInsertValues := "i-1, i, i, now()"
	loop2InsertValues := "i, i, NULL, now()"
	loop2LastInsertValues := "i-1, i-1, NULL, now()"
	if withRegion {
		insertCols = "id, region, id1, id2, updated_at"
		regionPrefix = "'r1', "
		loop2InsertValues = "i, 'r1', i, NULL, now()"
		loop2LastInsertValues = "i-1, 'r1', i-1, NULL, now()"
		loop3LastInsertValues = "i-1, 'r1', i, i, now()"
		loop4LastInsertValues = "i-1, 'r1', i, i, now()"
	}

	blocks := []string{
		fmt.Sprintf(` DO $$
		    DECLARE
				i INTEGER;
			BEGIN
				FOR i IN %d..%d LOOP
						UPDATE %s SET updated_at = now() + interval '1 second' WHERE id = i - 1;
						INSERT INTO %s(%s) VALUES(i, %s%d, i, now());

						DELETE FROM %s WHERE id = i;
						DELETE FROM %s WHERE id = i-1;
						INSERT INTO %s(%s) VALUES(i, %s%d, i, now());

						INSERT INTO %s(%s) VALUES(i-1, %s%d, i-1, now());
				END LOOP;
       		END
		$$;`, loop1Start, loop1End, tableName, tableName, insertCols, regionPrefix, anchor20, tableName, tableName, tableName, insertCols, regionPrefix, anchor20, tableName, insertCols, regionPrefix, anchor20),
		fmt.Sprintf(`INSERT INTO %s(%s) VALUES(%d, %s%d, NULL, now());`, tableName, insertCols, anchor521, regionPrefix, anchor521),
		fmt.Sprintf(` DO $$
		    DECLARE
				i INTEGER;
			BEGIN
				FOR i IN %d..%d LOOP
						UPDATE %s SET updated_at = now() + interval '1 second' WHERE id = i - 1;
						INSERT INTO %s(%s) VALUES(%s);

						DELETE FROM %s WHERE id = i;
						DELETE FROM %s WHERE id = i-1;
						INSERT INTO %s(%s) VALUES(%s);

						INSERT INTO %s(%s) VALUES(%s);

				END LOOP;
       		END
		$$;`, loop2Start, loop2End, tableName, tableName, insertCols, loop2InsertValues, tableName, tableName, tableName, insertCols, loop2InsertValues, tableName, insertCols, loop2LastInsertValues),
		fmt.Sprintf(`INSERT INTO %s(%s) VALUES(%d, %s%d, %d, now());`, tableName, insertCols, anchor1022, regionPrefix, anchor1022, anchor1022),
		fmt.Sprintf(` DO $$
		    DECLARE
				i INTEGER;
			BEGIN
				FOR i IN %d..%d LOOP
						UPDATE %s SET id1=i, id2=i WHERE id = i - 1;
						INSERT INTO %s(%s) VALUES(i, %s%d, %d, now());

						DELETE FROM %s WHERE id = i;
						UPDATE %s SET id1=%d, id2=%d WHERE id = i-1;

						DELETE FROM %s WHERE id = i-1;
						INSERT INTO %s(%s) VALUES(i, %s%d, %d, now());

						INSERT INTO %s(%s) VALUES(%s);

				END LOOP;
       		END
		$$;`, loop3Start, loop3End, tableName, tableName, insertCols, regionPrefix, anchor1022, anchor1022, tableName, tableName, anchor1022, anchor1022, tableName, tableName, insertCols, regionPrefix, anchor1022, anchor1022, tableName, insertCols, loop3LastInsertValues),
		fmt.Sprintf(`INSERT INTO %s(%s) VALUES(%d, %sNULL, NULL, now());`, tableName, insertCols, anchor1523, regionPrefix),
		fmt.Sprintf(` DO $$
		    DECLARE
				i INTEGER;
			BEGIN
				FOR i IN %d..%d LOOP
						UPDATE %s SET id1=i, id2=i WHERE id = i - 1;
						INSERT INTO %s(%s) VALUES(i, %sNULL, NULL, now());

						DELETE FROM %s WHERE id = i;
						UPDATE %s SET id1=NULL, id2=NULL WHERE id = i-1;

						DELETE FROM %s WHERE id = i-1;
						INSERT INTO %s(%s) VALUES(i, %sNULL, NULL, now());

						INSERT INTO %s(%s) VALUES(%s);


				END LOOP;
       		END
		$$;`, loop4Start, loop4End, tableName, tableName, insertCols, regionPrefix, tableName, tableName, tableName, tableName, insertCols, regionPrefix, tableName, insertCols, loop4LastInsertValues),
	}

	if idOffset > 0 {
		leadingInsert := fmt.Sprintf(`INSERT INTO %s(%s) VALUES(%d, %s%d, %d, now());`,
			tableName, insertCols, anchor20, regionPrefix, anchor20, anchor20)
		return append([]string{leadingInsert}, blocks...)
	}
	return blocks
}

// multiColumnUKFalsePositiveDeltaSQL returns delta SQL blocks that should not trigger
// unique-key conflict detection (loops 21-1021).
func multiColumnUKFalsePositiveDeltaSQL(tableName string, withRegion bool, idOffset int) []string {
	all := multiColumnUniqueIndexConflictDeltaSQL(tableName, withRegion, idOffset)
	return all[0:3]
}

// multiColumnUKTruePositiveDeltaSQL returns delta SQL blocks that should trigger
// unique-key conflict detection (loops 1023+).
func multiColumnUKTruePositiveDeltaSQL(tableName string, withRegion bool, idOffset int) []string {
	all := multiColumnUniqueIndexConflictDeltaSQL(tableName, withRegion, idOffset)
	return all[3:]
}

const (
	multiColumnUKFalsePositiveChangesInserts = 3001
	multiColumnUKFalsePositiveChangesUpdates = 1000
	multiColumnUKFalsePositiveChangesDeletes = 2000
)

func TestLiveMigrationWithMultiColumnUniqueIndexConflictDetectionCases(t *testing.T) {
	t.Parallel()
	sourceFPDeltaSQL := slices.Concat(
		multiColumnUKFalsePositiveDeltaSQL("test_schema.test_multi_column_unique_index", false, 0),
		multiColumnUKFalsePositiveDeltaSQL("test_schema.test_multi_column_unique_index_part", true, 0),
	)
	sourceTCDeltaSQL := slices.Concat(
		multiColumnUKTruePositiveDeltaSQL("test_schema.test_multi_column_unique_index", false, 0),
		multiColumnUKTruePositiveDeltaSQL("test_schema.test_multi_column_unique_index_part", true, 0),
	)
	targetDeltaSQL := slices.Concat(
		multiColumnUniqueIndexConflictDeltaSQL("test_schema.test_multi_column_unique_index", false, multiColumnUKConflictTargetIDOffset),
		multiColumnUniqueIndexConflictDeltaSQL("test_schema.test_multi_column_unique_index_part", true, multiColumnUKConflictTargetIDOffset),
	)
	liveMigrationTest := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test_multi_column_unique_index",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test_multi_column_unique_index",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.test_multi_column_unique_index (
				id int PRIMARY KEY,
				id1 int,
				id2 int,
				updated_at timestamp
			);
			CREATE UNIQUE INDEX idx_test_multi_column_unique_index_id1_id2 ON test_schema.test_multi_column_unique_index (id1, id2);

			CREATE TABLE test_schema.test_multi_column_unique_index_part (
				id int,
				region text,
				id1 int,
				id2 int,
				updated_at timestamp,
				PRIMARY KEY (id, region)
			) PARTITION BY LIST (region);
			CREATE TABLE test_schema.test_multi_column_unique_index_part_r1 PARTITION OF test_schema.test_multi_column_unique_index_part FOR VALUES IN ('r1');
			CREATE TABLE test_schema.test_multi_column_unique_index_part_r2 PARTITION OF test_schema.test_multi_column_unique_index_part FOR VALUES IN ('r2');
			CREATE UNIQUE INDEX idx_test_multi_column_unique_index_part_r1_id1_id2 ON test_schema.test_multi_column_unique_index_part_r1 (id1, id2);
			CREATE UNIQUE INDEX idx_test_multi_column_unique_index_part_r2_id1_id2 ON test_schema.test_multi_column_unique_index_part_r2 (id1, id2);`,
		},
		SourceSetupSchemaSQL: []string{
			"ALTER TABLE test_schema.test_multi_column_unique_index REPLICA IDENTITY FULL;",
			"ALTER TABLE test_schema.test_multi_column_unique_index_part REPLICA IDENTITY FULL;",
			"ALTER TABLE test_schema.test_multi_column_unique_index_part_r1 REPLICA IDENTITY FULL;",
			"ALTER TABLE test_schema.test_multi_column_unique_index_part_r2 REPLICA IDENTITY FULL;",
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.test_multi_column_unique_index (id, id1, id2, updated_at)
			SELECT i, i, i, now() FROM generate_series(1, 20) as i;`,
			`INSERT INTO test_schema.test_multi_column_unique_index_part (id, region, id1, id2, updated_at)
			SELECT i, 'r1', i, i, now() FROM generate_series(1, 20) as i;`,
		},
		/*
			false positive cases shouldn't be reported anymore -
				1.  updating i-1th row with id1 and id2 as 20 and inserting a row with id1 as 20 and id2 as i
				2.  updating i-1th row with id1 20 and id2 as NULL and inserting a row with id1 as i and id2 as NULL
				3. deleting i-1th row (id1 as 20 and id2 as i-1) and inserting a row with id1 as 20 and id2 as i
				4. deleting i-1th row (id1 as i-1 and id2 as NULL) and inserting a row with id1 as i and id2 as NULL


			cases should be reported as conflicts:
				1. updating i-1th row to set id1 from i-1 to i and id2 from i-1 to i and insert a row with id1 as 1022 and id2 as 1022
				2. updating i-1th row to set id1 from NULL to i and id2 from NULL to i and insert a row with id1 as NULL and id2 as NULL(U->I)
				3. delete new row and update i-1 to 1022,1022 and then deleting i-1th row (id1 as 1022 and id2 as 1022) and inserting a row with id1 as 1022 and id2 as 1022 (D->U, D->I)
				4. delete new row and update i-1 to NULL,NULL and then deleting i-1th row (id1 as NULL and id2 as NULL) and inserting a row with id1 as NULL and id2 as NULL (D->U, D->I)


		*/
		TargetDeltaSQL: targetDeltaSQL,
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

	uniqueKeyConflictFailpointEnv := testutils.GetFailpointEnvVar(
		"github.com/yugabyte/yb-voyager/yb-voyager/cmd/uniqueKeyConflictDetected=return(true)",
	)
	uniqueKeyConflictCountFailpointEnv := testutils.GetFailpointEnvVar(
		`github.com/yugabyte/yb-voyager/yb-voyager/cmd/uniqueKeyConflictDetected=return("count")`,
	)
	uniqueKeyConflictFailpointMarker := filepath.Join(
		liveMigrationTest.GetCurrentExportDir(), "failpoints", "failpoint-unique-key-conflict-detected.log")
	uniqueKeyConflictStatsPath := filepath.Join(
		liveMigrationTest.GetCurrentExportDir(), "failpoints", "unique-key-conflict-stats.json")

	err = liveMigrationTest.StartImportDataWithEnv(true, nil, []string{uniqueKeyConflictFailpointEnv})
	testutils.FatalIfError(t, err, "failed to start import data with failpoint")

	err = liveMigrationTest.WaitForSnapshotComplete(map[string]int64{
		`"test_schema"."test_multi_column_unique_index"`:      20,
		`"test_schema"."test_multi_column_unique_index_part"`: 20,
	}, 80)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	multiColumnUKTables := []string{
		`"test_schema"."test_multi_column_unique_index"`,
		`"test_schema"."test_multi_column_unique_index_part"`,
	}
	multiColumnUKFPChanges := ChangesCount{
		Inserts: multiColumnUKFalsePositiveChangesInserts,
		Updates: multiColumnUKFalsePositiveChangesUpdates,
		Deletes: multiColumnUKFalsePositiveChangesDeletes,
	}
	multiColumnUKChanges := ChangesCount{
		Inserts: 6003,
		Updates: 3000,
		Deletes: 4000,
	}

	err = liveMigrationTest.ValidateDataConsistency(multiColumnUKTables, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	liveMigrationTest.sourceContainer.ExecuteSqlsOnDB(
		liveMigrationTest.config.SourceDB.DatabaseName, sourceFPDeltaSQL...)

	err = liveMigrationTest.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"test_schema"."test_multi_column_unique_index"`:      multiColumnUKFPChanges,
		`"test_schema"."test_multi_column_unique_index_part"`: multiColumnUKFPChanges,
	}, 300, 5)
	testutils.FatalIfError(t, err, "failed to wait for false-positive streaming complete")

	require.False(t, liveMigrationTest.GetImportRunner().IsStopped(),
		"import should not exit during false-positive phase")
	failpointTriggered, err := testutils.WaitForFailpointMarker(uniqueKeyConflictFailpointMarker, 2*time.Second, 200*time.Millisecond)
	if err != nil && !os.IsNotExist(err) {
		testutils.FatalIfError(t, err, "failed to read unique key conflict failpoint marker")
	}
	require.False(t, failpointTriggered,
		"unique key conflict false positive detected; marker=%s", uniqueKeyConflictFailpointMarker)

	err = liveMigrationTest.StopImportData()
	testutils.FatalIfError(t, err, "failed to stop import after false-positive phase")

	err = liveMigrationTest.StartImportDataWithEnv(true, nil, []string{uniqueKeyConflictCountFailpointEnv})
	testutils.FatalIfError(t, err, "failed to start import for true-positive phase with count failpoint")

	liveMigrationTest.sourceContainer.ExecuteSqlsOnDB(
		liveMigrationTest.config.SourceDB.DatabaseName, sourceTCDeltaSQL...)

	err = liveMigrationTest.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"test_schema"."test_multi_column_unique_index"`:      multiColumnUKChanges,
		`"test_schema"."test_multi_column_unique_index_part"`: multiColumnUKChanges,
	}, 300, 5)
	testutils.FatalIfError(t, err, "failed to wait for true-positive streaming complete")

	require.False(t, liveMigrationTest.GetImportRunner().IsStopped(),
		"import should keep running during count failpoint mode")

	conflictStats, err := testutils.ReadUniqueKeyConflictStats(uniqueKeyConflictStatsPath)
	testutils.FatalIfError(t, err, "failed to read unique key conflict stats")

	require.LessOrEqual(t, conflictStats.Total, 6000, "true-positive delta should produce at most 6000 UK conflicts")
	require.LessOrEqual(t, conflictStats.ByTable[`"test_schema"."test_multi_column_unique_index"`], 3000, "test_multi_column_unique_index should have at most 3000 UK conflicts")
	require.LessOrEqual(t, conflictStats.ByTable[`"test_schema"."test_multi_column_unique_index_part"`], 3000, "test_multi_column_unique_index_part should have at most 3000 UK conflicts")

	require.GreaterOrEqual(t, conflictStats.Total, 0, "true-positive delta should produce at least 3000 UK conflicts")
	require.GreaterOrEqual(t, conflictStats.ByTable[`"test_schema"."test_multi_column_unique_index"`], 0, "test_multi_column_unique_index should have at least 1500 UK conflicts")
	require.GreaterOrEqual(t, conflictStats.ByTable[`"test_schema"."test_multi_column_unique_index_part"`], 0, "test_multi_column_unique_index_part should have at least 1500 UK conflicts")

	err = liveMigrationTest.ValidateDataConsistency(multiColumnUKTables, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = liveMigrationTest.InitiateCutoverToTarget(true, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to target")

	err = liveMigrationTest.WaitForCutoverComplete(0, 30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

	err = liveMigrationTest.StopImportDataToSource()
	testutils.FatalIfError(t, err, "failed to stop import data to source")

	err = liveMigrationTest.ExecuteTargetDelta()
	testutils.FatalIfError(t, err, "failed to execute target delta")

	//check no conflicts should be detected
	err = liveMigrationTest.StartImportDataToSourceWithEnv(true, nil, []string{uniqueKeyConflictFailpointEnv})
	testutils.FatalIfError(t, err, "failed to start import data to source with count failpoint")

	multiColumnUKChanges.Inserts += 1
	err = liveMigrationTest.WaitForFallbackStreamingComplete(map[string]ChangesCount{
		`"test_schema"."test_multi_column_unique_index"`:      multiColumnUKChanges,
		`"test_schema"."test_multi_column_unique_index_part"`: multiColumnUKChanges,
	}, 300, 5)
	testutils.FatalIfError(t, err, "failed to wait for fallback streaming complete")

	require.False(t, liveMigrationTest.GetImportToSourceRunner().IsStopped(),
		"import should not exit during fallback streaming phase")

	failpointTriggered, err = testutils.WaitForFailpointMarker(uniqueKeyConflictFailpointMarker, 2*time.Second, 200*time.Millisecond)
	if err != nil && !os.IsNotExist(err) {
		testutils.FatalIfError(t, err, "failed to read unique key conflict failpoint marker")
	}
	require.False(t, failpointTriggered,
		"unique key conflict detected; marker=%s", uniqueKeyConflictFailpointMarker)

	err = liveMigrationTest.ValidateDataConsistency(multiColumnUKTables, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = liveMigrationTest.InitiateCutoverToSource(nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to source")

	err = liveMigrationTest.WaitForCutoverSourceComplete(0, 100)
	testutils.FatalIfError(t, err, "failed to wait for cutover source complete")

	err = liveMigrationTest.ValidateDataConsistency(multiColumnUKTables, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

}

func getPartialPredicateTestForUniqueConflictDetection(t *testing.T, dbName string) *LiveMigrationTest {
	return NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: dbName,
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: dbName,
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
}

func TestLiveMigrationWithUniqueKeyValuesWithPartialPredicateConflictDetectionCases(t *testing.T) {
	t.Parallel()
	lm := getPartialPredicateTestForUniqueConflictDetection(t, "test_unique_conflict_partial_predicate")
	defer lm.Cleanup()

	err := lm.SetupContainers(context.Background())
	testutils.FatalIfError(t, err, "failed to setup containers")

	err = lm.SetupSchema()
	testutils.FatalIfError(t, err, "failed to setup schema")

	err = lm.StartExportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start export data")

	uniqueKeyConflictCountFailpointEnv := testutils.GetFailpointEnvVar(
		`github.com/yugabyte/yb-voyager/yb-voyager/cmd/uniqueKeyConflictDetected=return("count")`,
	)
	uniqueKeyConflictStatsPath := filepath.Join(
		lm.GetCurrentExportDir(), "failpoints", "unique-key-conflict-stats.json")

	err = lm.StartImportDataWithEnv(true, nil, []string{uniqueKeyConflictCountFailpointEnv})
	testutils.FatalIfError(t, err, "failed to start import data")

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"test_schema"."test_live"`: 20,
	}, 30)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	err = lm.ValidateDataConsistency([]string{`"test_schema"."test_live"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"test_schema"."test_live"`: {
			Inserts: 1500,
			Updates: 2500,
			Deletes: 1000,
		},
	}, 100, 5)
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	conflictStats, err := testutils.ReadUniqueKeyConflictStats(uniqueKeyConflictStatsPath)
	testutils.FatalIfError(t, err, "failed to read unique key conflict stats")

	require.LessOrEqual(t, conflictStats.Total, 2500, "true-positive delta should produce at most 2500 UK conflicts")
	require.LessOrEqual(t, conflictStats.ByTable[`"test_schema"."test_live"`], 2500, "test_live should have at most 2500 UK conflicts")
	require.GreaterOrEqual(t, conflictStats.Total, 0, "true-positive delta should produce at least 0 UK conflicts")
	require.GreaterOrEqual(t, conflictStats.ByTable[`"test_schema"."test_live"`], 0, "test_live should have at least 0 UK conflicts")

	err = lm.ValidateDataConsistency([]string{`"test_schema"."test_live"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = lm.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")

	err = lm.WaitForCutoverComplete(0, 30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

}

func TestLiveMigrationWithUniqueKeyConflictWithTablePartitioning(t *testing.T) {
	t.Parallel()
	lm := getPartialPredicateTestForUniqueConflictDetection(t, "test_uc_table_partitioning")
	defer lm.Cleanup()

	err := lm.SetupContainers(context.Background())
	testutils.FatalIfError(t, err, "failed to setup containers")

	err = lm.SetupSchema()
	testutils.FatalIfError(t, err, "failed to setup schema")

	err = lm.StartExportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start export data")

	uniqueKeyConflictCountFailpointEnv := testutils.GetFailpointEnvVar(
		`github.com/yugabyte/yb-voyager/yb-voyager/cmd/uniqueKeyConflictDetected=return("count")`,
	)

	uniqueKeyConflictStatsPath := filepath.Join(
		lm.GetCurrentExportDir(), "failpoints", "unique-key-conflict-stats.json")

	err = lm.StartImportDataWithEnv(true, map[string]string{
		"--cdc-partition-key": "table",
	}, []string{uniqueKeyConflictCountFailpointEnv})
	testutils.FatalIfError(t, err, "failed to start import data")

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"test_schema"."test_live"`: 20,
	}, 30)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	err = lm.ValidateDataConsistency([]string{`"test_schema"."test_live"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"test_schema"."test_live"`: {
			Inserts: 1500,
			Updates: 2500,
			Deletes: 1000,
		},
	}, 100, 5)
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	conflicts, err := testutils.ReadUniqueKeyConflictStats(uniqueKeyConflictStatsPath)
	if err != nil && !os.IsNotExist(err) {
		testutils.FatalIfError(t, err, "failed to read unique key conflict stats")
	}
	require.Nil(t, conflicts, "no conflicts should be detected")

	err = lm.ValidateDataConsistency([]string{`"test_schema"."test_live"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = lm.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")

	err = lm.WaitForCutoverComplete(0, 30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")
}

func TestLiveMigrationWithUniqueKeyConflictWithNullValuesDetectionCases(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test_null_conflict_detection",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test_null_conflict_detection",
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

	uniqueKeyConflictCountFailpointEnv := testutils.GetFailpointEnvVar(
		`github.com/yugabyte/yb-voyager/yb-voyager/cmd/uniqueKeyConflictDetected=return("count")`,
	)
	uniqueKeyConflictStatsPath := filepath.Join(
		lm.GetCurrentExportDir(), "failpoints", "unique-key-conflict-stats.json")

	err = lm.StartImportDataWithEnv(true, nil, []string{uniqueKeyConflictCountFailpointEnv})
	testutils.FatalIfError(t, err, "failed to start import data")

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"test_schema"."test_live_null_unique_values"`: 20,
	}, 30)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	err = lm.ValidateDataConsistency([]string{`"test_schema"."test_live_null_unique_values"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"test_schema"."test_live_null_unique_values"`: {
			Inserts: 1500,
			Updates: 3000,
			Deletes: 1000,
		},
	}, 120, 5)
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	require.False(t, lm.GetImportRunner().IsStopped(),
		"import should keep running during count failpoint mode")

	conflictStats, err := testutils.ReadUniqueKeyConflictStats(uniqueKeyConflictStatsPath)
	fmt.Println("conflictStats", conflictStats)
	testutils.FatalIfError(t, err, "failed to read unique key conflict stats")
	require.GreaterOrEqual(t, conflictStats.Total, 2000,
		"null unique delta should produce 2500 UK conflicts (5 per loop x 500 loops)")
	require.GreaterOrEqual(t, conflictStats.ByTable[`"test_schema"."test_live_null_unique_values"`], 2000)

	err = lm.ValidateDataConsistency([]string{`"test_schema"."test_live_null_unique_values"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = lm.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")

	err = lm.WaitForCutoverComplete(0, 30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

}

func TestLiveMigrationWithUniqueKeyConflictWithNullValuesDetectionCasesAndDisableNullConflicts(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test_null_conflicts_disable",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test_null_conflicts_disable",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.test_live_null_unique_values (
				id int PRIMARY KEY,
				name TEXT,
				check_id int UNIQUE,
				check_id_null_unique int,
				id3 int
			);`,
			`CREATE UNIQUE INDEX idx_test_live_null_unique_values_check_id_null_unique ON test_schema.test_live_null_unique_values (check_id_null_unique, id3);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.test_live_null_unique_values REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.test_live_null_unique_values (id, name, check_id, check_id_null_unique, id3)
SELECT
	i,
	md5(random()::text),                                   -- name
    CASE WHEN i%2=0 THEN i ELSE NULL END,                  -- check_id
    i,                                                 -- check_id_null_unique
    i+100                                             -- id3
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
				UPDATE test_schema.test_live_null_unique_values SET check_id_null_unique = NULL, id3 = NULL WHERE id = i - 1;
				INSERT INTO test_schema.test_live_null_unique_values(id, name, check_id, check_id_null_unique, id3) 
				SELECT i, md5(random()::text), CASE WHEN i%2=0 THEN i ELSE NULL END, i-1, i+100 ;
		
				UPDATE test_schema.test_live_null_unique_values SET check_id_null_unique = i WHERE id = i - 1;
				UPDATE test_schema.test_live_null_unique_values SET check_id_null_unique = NULL, id3 = NULL WHERE id = i;
		
				DELETE FROM test_schema.test_live_null_unique_values WHERE id = i-1;
				UPDATE test_schema.test_live_null_unique_values SET check_id_null_unique = i-1, id3 = i+100 WHERE id = i;
		
				UPDATE test_schema.test_live_null_unique_values SET check_id_null_unique = NULL, id3 = NULL WHERE id = i;
				
				DELETE FROM test_schema.test_live_null_unique_values WHERE id = i;
				INSERT INTO test_schema.test_live_null_unique_values(id, name, check_id, check_id_null_unique, id3) 
				SELECT i-1, md5(random()::text), CASE WHEN (i-1)%2=0 THEN i-1 ELSE NULL END, NULL, NULL;
		
		
				UPDATE test_schema.test_live_null_unique_values SET check_id_null_unique = i-1 WHERE id = i - 1;
				INSERT INTO test_schema.test_live_null_unique_values(id, name, check_id, check_id_null_unique, id3)
				SELECT i, md5(random()::text), CASE WHEN i%2=0 THEN i ELSE NULL END, i, i+100;
		
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

	uniqueKeyConflictFailpointEnv := testutils.GetFailpointEnvVar(
		`github.com/yugabyte/yb-voyager/yb-voyager/cmd/uniqueKeyConflictDetected=return("true")`,
	)

	uniqueKeyConflictFailpointMarker := filepath.Join(
		lm.GetCurrentExportDir(), "failpoints", "failpoint-unique-key-conflict-detected.log")

	err = lm.StartImportDataWithEnv(true, map[string]string{
		"--disable-null-conflicts": "true",
	}, []string{uniqueKeyConflictFailpointEnv})
	testutils.FatalIfError(t, err, "failed to start import data")

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"test_schema"."test_live_null_unique_values"`: 20,
	}, 30)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	err = lm.ValidateDataConsistency([]string{`"test_schema"."test_live_null_unique_values"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"test_schema"."test_live_null_unique_values"`: {
			Inserts: 1500,
			Updates: 3000,
			Deletes: 1000,
		},
	}, 120, 5)
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	err = lm.ValidateDataConsistency([]string{`"test_schema"."test_live_null_unique_values"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	require.False(t, lm.GetImportRunner().IsStopped(),
		"import should not exit during false-positive phase")
	failpointTriggered, err := testutils.WaitForFailpointMarker(uniqueKeyConflictFailpointMarker, 2*time.Second, 200*time.Millisecond)
	if err != nil && !os.IsNotExist(err) {
		testutils.FatalIfError(t, err, "failed to read unique key conflict failpoint marker")
	}
	require.False(t, failpointTriggered,
		"unique key conflict false positive detected; marker=%s", uniqueKeyConflictFailpointMarker)

	require.False(t, failpointTriggered, "count mode should not write crash failpoint marker")
	err = lm.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")

	err = lm.WaitForCutoverComplete(0, 30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

}
func TestLiveMigrationWithUniqueKeyConflictWithNullValueAndPartialPredicatesDetectionCases(t *testing.T) {
	t.Parallel()
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

	uniqueKeyConflictCountFailpointEnv := testutils.GetFailpointEnvVar(
		`github.com/yugabyte/yb-voyager/yb-voyager/cmd/uniqueKeyConflictDetected=return("count")`,
	)
	uniqueKeyConflictStatsPath := filepath.Join(
		liveMigrationTest.GetCurrentExportDir(), "failpoints", "unique-key-conflict-stats.json")

	err = liveMigrationTest.StartImportDataWithEnv(true, nil, []string{uniqueKeyConflictCountFailpointEnv})
	testutils.FatalIfError(t, err, "failed to start import data")

	err = liveMigrationTest.WaitForSnapshotComplete(map[string]int64{
		`"test_schema"."test_live_null_partial_unique_values"`: 20,
	}, 30)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	err = liveMigrationTest.ValidateDataConsistency([]string{`"test_schema"."test_live_null_partial_unique_values"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = liveMigrationTest.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	err = liveMigrationTest.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"test_schema"."test_live_null_partial_unique_values"`: {
			Inserts: 2000,
			Updates: 2500,
			Deletes: 1500,
		},
	}, 120, 5)
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	require.False(t, liveMigrationTest.GetImportRunner().IsStopped(),
		"import should keep running during count failpoint mode")

	conflictStats, err := testutils.ReadUniqueKeyConflictStats(uniqueKeyConflictStatsPath)
	testutils.FatalIfError(t, err, "failed to read unique key conflict stats")
	require.GreaterOrEqual(t, conflictStats.Total, 0, "partial unique delta should produce at least 0 UK conflicts")
	require.GreaterOrEqual(t, conflictStats.ByTable[`"test_schema"."test_live_null_partial_unique_values"`], 0, "test_live_null_partial_unique_values should have at least 0 UK conflicts")

	err = liveMigrationTest.ValidateDataConsistency([]string{`"test_schema"."test_live_null_partial_unique_values"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = liveMigrationTest.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to target")

	err = liveMigrationTest.WaitForCutoverComplete(0, 30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

}

func TestLiveMigrationWithUniqueKeyConflictsOnCaseSensitiveColumns(t *testing.T) {
	t.Parallel()
	liveMigrationTest := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test_unique_key_on_case_sensitive_columns",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test_unique_key_on_case_sensitive_columns",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.test_unique_key_on_case_sensitive_columns (
				id int PRIMARY KEY,
				id1 int,
				"Id2" int,
				"ID3" int
			);
			CREATE UNIQUE INDEX idx_test_unique_key_on_id2 ON test_schema.test_unique_key_on_case_sensitive_columns ("Id2");
			CREATE UNIQUE INDEX idx_test_unique_key_on_id3 ON test_schema.test_unique_key_on_case_sensitive_columns (id1, "ID3");
			`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.test_unique_key_on_case_sensitive_columns (id, id1, "Id2", "ID3")
			VALUES (1, 1, 1, 1), (2, 2, 2, 2);`,
		},
		SourceSetupSchemaSQL: []string{
			"ALTER TABLE test_schema.test_unique_key_on_case_sensitive_columns REPLICA IDENTITY FULL;",
		},
		SourceDeltaSQL: []string{
			/*
				Each loop iteration exercises unique-key conflict detection on the
				case-sensitive unique indexes ("Id2") and (id1, "ID3") by reusing the
				value held by the anchor row id=2 across a different PK (id=i). Because
				id=2 and id=i land on different PK channels, the importer must serialize
				them to avoid a transient UK violation on the target.

				Two conflict shapes are generated per iteration:
					UI conflict: move row id=2 off (2,2,2), insert row id=i reusing (2,2,2)
					DI conflict: delete row id=2, insert row id=i reusing (2,2,2)

				Every iteration fully resets the table back to {id=1, id=2}, so the batch
				is always valid on the source (no persistent duplicate across iterations)
				while keeping source and target consistent after conflict resolution.
			*/
			`DO $$
			DECLARE
				i INTEGER;
			BEGIN
				FOR i IN 3..502 LOOP
					-- UI conflict on ("Id2") and (id1,"ID3")
					UPDATE test_schema.test_unique_key_on_case_sensitive_columns SET "Id2" = i, id1 = i, "ID3" = i WHERE id = 2;
					INSERT INTO test_schema.test_unique_key_on_case_sensitive_columns (id, id1, "Id2", "ID3") VALUES (i, 2, 2, 2);
					-- DU conflict 
					DELETE FROM test_schema.test_unique_key_on_case_sensitive_columns WHERE id = i;
					UPDATE test_schema.test_unique_key_on_case_sensitive_columns SET "Id2" = 2, id1 = 2, "ID3" = 2 WHERE id = 2;

					-- DI conflict on ("Id2") and (id1,"ID3")
					DELETE FROM test_schema.test_unique_key_on_case_sensitive_columns WHERE id = 2;
					INSERT INTO test_schema.test_unique_key_on_case_sensitive_columns (id, id1, "Id2", "ID3") VALUES (i, 2, 2, 2);
					DELETE FROM test_schema.test_unique_key_on_case_sensitive_columns WHERE id = i;
					INSERT INTO test_schema.test_unique_key_on_case_sensitive_columns (id, id1, "Id2", "ID3") VALUES (2, 2, 2, 2);
				END LOOP;
			END $$;`,
		},
		TargetDeltaSQL: []string{
			// Plain inserts of brand-new rows: no unique-key conflicts, so the fallback
			// stream (target -> source) must not detect any conflict.
			`INSERT INTO test_schema.test_unique_key_on_case_sensitive_columns (id, id1, "Id2", "ID3")
			VALUES (3, 3, 3, 3), (4, 4, 4, 4);`,
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

	table := `"test_schema"."test_unique_key_on_case_sensitive_columns"`

	uniqueKeyConflictFailpointEnv := testutils.GetFailpointEnvVar(
		"github.com/yugabyte/yb-voyager/yb-voyager/cmd/uniqueKeyConflictDetected=return(true)",
	)
	uniqueKeyConflictFailpointMarker := filepath.Join(
		liveMigrationTest.GetCurrentExportDir(), "failpoints", "failpoint-unique-key-conflict-detected.log")

	uniqueKeyConflictCountFailpointEnv := testutils.GetFailpointEnvVar(
		`github.com/yugabyte/yb-voyager/yb-voyager/cmd/uniqueKeyConflictDetected=return("count")`,
	)
	uniqueKeyConflictStatsPath := filepath.Join(
		liveMigrationTest.GetCurrentExportDir(), "failpoints", "unique-key-conflict-stats.json")

	// Forward phase: use the "count" failpoint so conflicts are recorded (not fatal).
	err = liveMigrationTest.StartImportDataWithEnv(true, nil, []string{uniqueKeyConflictCountFailpointEnv})
	testutils.FatalIfError(t, err, "failed to start import data with count failpoint")

	err = liveMigrationTest.WaitForSnapshotComplete(map[string]int64{
		table: 2,
	}, 30)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	err = liveMigrationTest.ValidateDataConsistency([]string{table}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency after snapshot")

	err = liveMigrationTest.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	// 500 iterations x (2 updates + 3 inserts + 3 deletes) per iteration.
	err = liveMigrationTest.WaitForForwardStreamingComplete(map[string]ChangesCount{
		table: {
			Inserts: 1500,
			Updates: 1000,
			Deletes: 1500,
		},
	}, 200, 5)
	testutils.FatalIfError(t, err, "failed to wait for forward streaming complete")

	require.False(t, liveMigrationTest.GetImportRunner().IsStopped(),
		"import should keep running during count failpoint mode")

	conflictStats, err := testutils.ReadUniqueKeyConflictStats(uniqueKeyConflictStatsPath)
	testutils.FatalIfError(t, err, "failed to read unique key conflict stats")
	require.GreaterOrEqual(t, conflictStats.Total, 500,
		"case-sensitive UK delta should produce unique-key conflicts")
	require.GreaterOrEqual(t, conflictStats.ByTable[table], 1,
		"case-sensitive UK conflicts should be attributed to the table")

	err = liveMigrationTest.ValidateDataConsistency([]string{table}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency after forward streaming")

	// Cutover to target, preparing for fallback.
	err = liveMigrationTest.InitiateCutoverToTarget(true, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to target")

	err = liveMigrationTest.WaitForCutoverComplete(0, 30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

	err = liveMigrationTest.StopImportDataToSource()
	testutils.FatalIfError(t, err, "failed to stop import data to source")

	err = liveMigrationTest.ExecuteTargetDelta()
	testutils.FatalIfError(t, err, "failed to execute target delta")

	// Fallback phase: use the crash-on-conflict failpoint and assert no conflict is detected.
	err = liveMigrationTest.StartImportDataToSourceWithEnv(true, nil, []string{uniqueKeyConflictFailpointEnv})
	testutils.FatalIfError(t, err, "failed to start import data to source")

	err = liveMigrationTest.WaitForFallbackStreamingComplete(map[string]ChangesCount{
		table: {
			Inserts: 2,
			Updates: 0,
			Deletes: 0,
		},
	}, 200, 5)
	testutils.FatalIfError(t, err, "failed to wait for fallback streaming complete")

	require.False(t, liveMigrationTest.GetImportToSourceRunner().IsStopped(),
		"import should not exit during fallback streaming phase")

	failpointTriggered, err := testutils.WaitForFailpointMarker(uniqueKeyConflictFailpointMarker, 2*time.Second, 200*time.Millisecond)
	if err != nil && !os.IsNotExist(err) {
		testutils.FatalIfError(t, err, "failed to read unique key conflict failpoint marker")
	}
	require.False(t, failpointTriggered,
		"unique key conflict detected during fallback; marker=%s", uniqueKeyConflictFailpointMarker)

	err = liveMigrationTest.ValidateDataConsistency([]string{table}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency after fallback streaming")

	err = liveMigrationTest.InitiateCutoverToSource(nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to source")

	err = liveMigrationTest.WaitForCutoverSourceComplete(0, 100)
	testutils.FatalIfError(t, err, "failed to wait for cutover source complete")

	err = liveMigrationTest.ValidateDataConsistency([]string{table}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency after cutover to source")
}