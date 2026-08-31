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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/cmd"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
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

	require.Greater(t, conflictStats.Total, 0, "true-positive delta should produce UK conflicts")
	require.Greater(t, conflictStats.ByTable[`"test_schema"."test_multi_column_unique_index"`], 0, "test_multi_column_unique_index should produce UK conflicts")
	require.Greater(t, conflictStats.ByTable[`"test_schema"."test_multi_column_unique_index_part"`], 0, "test_multi_column_unique_index_part should produce UK conflicts")

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

	require.Greater(t, conflictStats.Total, 0, "true-positive delta should produce UK conflicts")
	require.Greater(t, conflictStats.ByTable[`"test_schema"."test_live"`], 0, "test_live should produce UK conflicts")

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

func TestLiveMigrationWithUniqueKeyConflictWithNullValuesDetectionCasesNULLSNOTDISTINCT(t *testing.T) {
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
	require.Greater(t, conflictStats.Total, 0, "null unique delta should produce UK conflicts")
	require.Greater(t, conflictStats.ByTable[`"test_schema"."test_live_null_unique_values"`], 0, "test_live_null_unique_values should produce UK conflicts")

	err = lm.ValidateDataConsistency([]string{`"test_schema"."test_live_null_unique_values"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = lm.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")

	err = lm.WaitForCutoverComplete(0, 30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

}

func TestLiveMigrationWithUniqueKeyConflictWithNullValuesCAseWithDefaultUniqueIndexNULLSDistinct(t *testing.T) {
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

	err = lm.StartImportDataWithEnv(true, nil, []string{uniqueKeyConflictFailpointEnv})
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
	require.Greater(t, conflictStats.Total, 0, "partial unique delta should produce UK conflicts")
	require.Greater(t, conflictStats.ByTable[`"test_schema"."test_live_null_partial_unique_values"`], 0, "test_live_null_partial_unique_values should produce UK conflicts")

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
	require.Greater(t, conflictStats.Total, 0, "case-sensitive UK delta should produce unique-key conflicts")
	require.Greater(t, conflictStats.ByTable[table], 0, "case-sensitive UK conflicts should be attributed to the table")

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

// TestLiveMigrationCdcPartitionKeyConfigs covers three import-data config cases:
//  1. invalid overrides fail before snapshot
//  2. global pk succeeds; changing overrides on resume is rejected
//  3. start-clean with mixed pk/table overrides completes live migration through cutover
func TestLiveMigrationCdcPartitionKeyConfigs(t *testing.T) {
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "cdc_part_key",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "cdc_part_key",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.orders (
				id SERIAL PRIMARY KEY,
				name TEXT
			);
			CREATE TABLE test_schema.events (
				id SERIAL PRIMARY KEY,
				payload TEXT
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.orders REPLICA IDENTITY FULL;
			ALTER TABLE test_schema.events REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.orders (name) SELECT md5(random()::text) FROM generate_series(1, 10);`,
			`INSERT INTO test_schema.events (payload) SELECT md5(random()::text) FROM generate_series(1, 10);`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema.orders (name) SELECT md5(random()::text) FROM generate_series(1, 5);`,
			`INSERT INTO test_schema.events (payload) SELECT md5(random()::text) FROM generate_series(1, 5);`,
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

	// Case 1: unknown table in overrides must fail during prepare (before snapshot).
	err = lm.StartImportData(false, map[string]string{
		"--cdc-partition-key":           "pk",
		"--cdc-partition-key-overrides": "test_schema.missing_table:table",
	})
	require.Error(t, err, "import with unknown override table should fail")
	stderr := lm.GetImportCommandStderr() + lm.GetImportCommandStdout()
	assert.True(t,
		strings.Contains(stderr, "not in the import table list") || strings.Contains(stderr, "not found in name registry"),
		"expected table-list/namereg error, got: %s", stderr)

	// Case 2: global pk succeeds; stored map should be pk for both tables.
	err = lm.StartImportData(true, map[string]string{
		"--cdc-partition-key": "pk",
	})
	testutils.FatalIfError(t, err, "failed to start import data with global pk")

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"test_schema"."orders"`: 10,
		`"test_schema"."events"`: 10,
	}, 30)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	err = lm.InitMetaDB()
	testutils.FatalIfError(t, err, "failed to initialize meta db")

	importDataStatus, err := lm.GetMetaDB().GetImportDataStatusRecord()
	testutils.FatalIfError(t, err, "failed to get import data status record")

	assert.Equal(t, "pk", importDataStatus.CdcPartitioningStrategyConfig)
	assert.Equal(t, "", importDataStatus.CdcPartitionKeyOverridesConfig)
	assertStrategyInMap(t, importDataStatus.TableToCDCPartitionKey, "orders", cmd.PARTITION_BY_PK)
	assertStrategyInMap(t, importDataStatus.TableToCDCPartitionKey, "events", cmd.PARTITION_BY_PK)

	err = lm.StopImportData()
	testutils.FatalIfError(t, err, "failed to stop import data")

	// Resume without start-clean must reject newly introduced overrides. The change is
	// caught by the semantic per-table comparison in prepareCdcPartitionKey (orders flips
	// pk -> table), not a raw string compare.
	_ = lm.ResumeImportData(false, map[string]string{
		"--cdc-partition-key":           "pk",
		"--cdc-partition-key-overrides": "test_schema.orders:table",
	})
	rejectOutput := lm.GetImportCommandStderr() + lm.GetImportCommandStdout()
	assert.True(t,
		strings.Contains(rejectOutput, "changing cdc-partition-key") &&
			strings.Contains(rejectOutput, "is not allowed"),
		"expected overrides change-guard error, got: %s", rejectOutput)

	// Case 3: mixed strategies — global pk, override orders to table.
	mixedOverrides := "test_schema.orders:table;test_schema.events:pk"
	err = lm.ResumeImportData(true, map[string]string{
		"--cdc-partition-key":           "pk",
		"--cdc-partition-key-overrides": mixedOverrides,
		"--start-clean":                 "true",
		"--truncate-tables":             "true",
	})
	testutils.FatalIfError(t, err, "failed to resume import data with mixed partition-key configs")

	time.Sleep(30 * time.Second)

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"test_schema"."orders"`: 10,
		`"test_schema"."events"`: 10,
	}, 30)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete after start-clean")

	importDataStatus, err = lm.GetMetaDB().GetImportDataStatusRecord()
	testutils.FatalIfError(t, err, "failed to get import data status record")
	assert.Equal(t, "pk", importDataStatus.CdcPartitioningStrategyConfig)
	assert.Equal(t, mixedOverrides, importDataStatus.CdcPartitionKeyOverridesConfig)
	assertStrategyInMap(t, importDataStatus.TableToCDCPartitionKey, "orders", cmd.PARTITION_BY_TABLE)
	assertStrategyInMap(t, importDataStatus.TableToCDCPartitionKey, "events", cmd.PARTITION_BY_PK)

	err = lm.ValidateDataConsistency([]string{`"test_schema"."orders"`, `"test_schema"."events"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate snapshot data consistency")

	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"test_schema"."orders"`: {Inserts: 5},
		`"test_schema"."events"`: {Inserts: 5},
	}, 60, 1)
	testutils.FatalIfError(t, err, "failed to wait for forward streaming complete")

	err = lm.ValidateDataConsistency([]string{`"test_schema"."orders"`, `"test_schema"."events"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate streaming data consistency")

	err = lm.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")

	err = lm.WaitForCutoverComplete(0, 30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")
}

func assertStrategyInMap(t *testing.T, partitionKeyMap map[string]metadb.CDCPartitionKey, tableSubstring, want string) {
	t.Helper()
	for key, partitionKey := range partitionKeyMap {
		if strings.Contains(strings.ToLower(key), strings.ToLower(tableSubstring)) {
			require.Equal(t, want, partitionKey.Strategy, "strategy for key %q", key)
			return
		}
	}
	t.Fatalf("table %q not found in partition key map: %v", tableSubstring, partitionKeyMap)
}

// TestLiveMigrationCdcPartitionKeyOverridesEquivalentOnResume verifies the semantic
// resume guard for cdc-partition-key-overrides: resuming (without --start-clean) with an
// overrides string written differently (quoting / whitespace) but resolving to the SAME
// effective per-table strategy must be ACCEPTED. A raw string compare (the old behaviour)
// would have wrongly rejected it; prepareCdcPartitionKey now compares the resolved
// per-table map instead.
func TestLiveMigrationCdcPartitionKeyOverridesEquivalentOnResume(t *testing.T) {
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "cdc_part_key_equiv",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "cdc_part_key_equiv",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.orders (
				id SERIAL PRIMARY KEY,
				name TEXT
			);
			CREATE TABLE test_schema.events (
				id SERIAL PRIMARY KEY,
				payload TEXT
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.orders REPLICA IDENTITY FULL;
			ALTER TABLE test_schema.events REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.orders (name) SELECT md5(random()::text) FROM generate_series(1, 10);`,
			`INSERT INTO test_schema.events (payload) SELECT md5(random()::text) FROM generate_series(1, 10);`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema.orders (name) SELECT md5(random()::text) FROM generate_series(1, 5);`,
			`INSERT INTO test_schema.events (payload) SELECT md5(random()::text) FROM generate_series(1, 5);`,
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

	// First run: global pk, override orders -> table (events keeps pk).
	err = lm.StartImportData(true, map[string]string{
		"--cdc-partition-key":           "pk",
		"--cdc-partition-key-overrides": "test_schema.orders:table",
	})
	testutils.FatalIfError(t, err, "failed to start import data")

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"test_schema"."orders"`: 10,
		`"test_schema"."events"`: 10,
	}, 30)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	err = lm.InitMetaDB()
	testutils.FatalIfError(t, err, "failed to initialize meta db")

	importDataStatus, err := lm.GetMetaDB().GetImportDataStatusRecord()
	testutils.FatalIfError(t, err, "failed to get import data status record")
	assert.Equal(t, "pk", importDataStatus.CdcPartitioningStrategyConfig)
	assertStrategyInMap(t, importDataStatus.TableToCDCPartitionKey, "orders", cmd.PARTITION_BY_TABLE)
	assertStrategyInMap(t, importDataStatus.TableToCDCPartitionKey, "events", cmd.PARTITION_BY_PK)
	// Expression-UK tables are captured on the first run (none here, but non-nil) so the
	// resume comparison needs no target-DB re-query.
	require.NotNil(t, importDataStatus.CdcExpressionUniqueIndexTables,
		"expression-UK tables should be captured on the first prepare")

	err = lm.StopImportData()
	testutils.FatalIfError(t, err, "failed to stop import data")

	// Case 1 (reject): resume WITHOUT start-clean flipping the strategies
	// (orders -> pk, events -> table) differs from the persisted map (orders -> table,
	// events -> pk), so the semantic change-guard must reject it.
	_ = lm.ResumeImportData(false, map[string]string{
		"--cdc-partition-key":           "pk",
		"--cdc-partition-key-overrides": "test_schema.orders:pk;test_schema.events:table",
	})
	flipRejectOutput := lm.GetImportCommandStderr() + lm.GetImportCommandStdout()
	assert.True(t,
		strings.Contains(flipRejectOutput, "changing cdc-partition-key") &&
			strings.Contains(flipRejectOutput, "is not allowed"),
		"flipped strategies must trip the resume change-guard, got: %s", flipRejectOutput)

	// Case 2 (accept): resume WITHOUT start-clean with a differently-written but
	// semantically-equivalent overrides string (quoted identifiers, reordered, extra
	// whitespace). Same effective strategy (orders -> table, events -> pk), so it must be
	// accepted and streaming resumes.
	equivalentOverrides := ` "test_schema"."events":pk ; "test_schema"."orders":table `
	err = lm.ResumeImportData(true, map[string]string{
		"--cdc-partition-key":           "pk",
		"--cdc-partition-key-overrides": equivalentOverrides,
	})
	testutils.FatalIfError(t, err, "resume with semantically-equivalent overrides should succeed")

	// Give the resumed import time to run prepareCdcPartitionKey; it must not trip the
	// change-guard for an equivalent overrides string.
	time.Sleep(15 * time.Second)
	resumeOutput := lm.GetImportCommandStderr() + lm.GetImportCommandStdout()
	assert.NotContains(t, resumeOutput, "is not allowed",
		"semantically-equivalent overrides must not trip the resume change-guard, got: %s", resumeOutput)

	// The effective per-table map is preserved across the resume.
	importDataStatus, err = lm.GetMetaDB().GetImportDataStatusRecord()
	testutils.FatalIfError(t, err, "failed to get import data status record after resume")
	assert.Equal(t, "pk", importDataStatus.CdcPartitioningStrategyConfig)
	assertStrategyInMap(t, importDataStatus.TableToCDCPartitionKey, "orders", cmd.PARTITION_BY_TABLE)
	assertStrategyInMap(t, importDataStatus.TableToCDCPartitionKey, "events", cmd.PARTITION_BY_PK)

	err = lm.ValidateDataConsistency([]string{`"test_schema"."orders"`, `"test_schema"."events"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate snapshot data consistency after resume")

	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"test_schema"."orders"`: {Inserts: 5},
		`"test_schema"."events"`: {Inserts: 5},
	}, 60, 1)
	testutils.FatalIfError(t, err, "failed to wait for forward streaming complete")

	err = lm.ValidateDataConsistency([]string{`"test_schema"."orders"`, `"test_schema"."events"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate streaming data consistency")

	err = lm.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")

	err = lm.WaitForCutoverComplete(0, 30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")
}

// TestLiveMigrationCdcPartitionKeyRejectsPkOnExpressionUniqueIndex verifies the
// expression-UK guardrail end-to-end: pk-partitioning cannot detect unique-key
// conflicts on an expression unique index, so `--cdc-partition-key pk` on such a
// table must be rejected during prepare (before snapshot import). It then confirms
// the documented remedy — overriding just that table to `table` — is accepted and
// the snapshot completes. This exercises the real getExpressionUniqueIndexTables
// query on the target, which the pure unit tests cannot cover.
func TestLiveMigrationCdcPartitionKeyRejectsPkOnExpressionUniqueIndex(t *testing.T) {
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "cdc_expr_uk",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "cdc_expr_uk",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.users (
				id SERIAL PRIMARY KEY,
				email TEXT
			);
			CREATE UNIQUE INDEX users_lower_email_uidx ON test_schema.users (lower(email));`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.users REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.users (email) SELECT 'user_' || i || '@example.com' FROM generate_series(1, 10) i;`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema.users (email) SELECT 'user_' || i || '@example.com' FROM generate_series(11, 20) i;`,
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

	// pk on a table with an expression unique index must fail during prepare,
	// before snapshot import (prepareCdcPartitionKey runs before ImportDataStarted
	// is persisted, so this failed attempt leaves no state behind).
	err = lm.StartImportData(false, map[string]string{
		"--cdc-partition-key": "pk",
	})
	require.Error(t, err, "import with pk partition-key on expression-UK table should fail")
	stderr := lm.GetImportCommandStderr() + lm.GetImportCommandStdout()
	assert.Contains(t, stderr, "expression-based unique index",
		"expected expression-UK rejection, got: %s", stderr)

	// Remedy: override just the expression-UK table to `table`. The global pk still
	// applies elsewhere, but users is table-partitioned, so prepare succeeds and the
	// snapshot completes.
	err = lm.StartImportData(false, map[string]string{
		"--cdc-partition-key-overrides": "test_schema.users:pk",
	})
	require.Error(t, err, "import with pk partition-key on expression-UK table should fail")
	stderr = lm.GetImportCommandStderr() + lm.GetImportCommandStdout()
	assert.Contains(t, stderr, "expression-based unique index",
		"expected expression-UK rejection, got: %s", stderr)

	err = lm.ResumeImportData(true, map[string]string{
		"--cdc-partition-key-overrides": "test_schema.users:table",
	})
	require.NoError(t, err, "failed to resume import data with pk partition-key override on expression-UK table")

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"test_schema"."users"`: 10,
	}, 30)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	err = lm.InitMetaDB()
	testutils.FatalIfError(t, err, "failed to initialize meta db")

	importDataStatus, err := lm.GetMetaDB().GetImportDataStatusRecord()
	testutils.FatalIfError(t, err, "failed to get import data status record")
	require.Equal(t, "auto", importDataStatus.CdcPartitioningStrategyConfig)
	assertStrategyInMap(t, importDataStatus.TableToCDCPartitionKey, "users", cmd.PARTITION_BY_TABLE)

	// Resume WITHOUT start-clean trying to switch the expression-UK table from table to pk
	// must be rejected: pk-partitioning cannot detect unique-key conflicts on an expression
	// unique index. The expression-UK set captured on the first run (global auto) drives the
	// same guard on resume, so no target-DB re-query is needed.
	err = lm.StopImportData()
	testutils.FatalIfError(t, err, "failed to stop import data before pk-on-expr-UK resume")

	_ = lm.ResumeImportData(false, map[string]string{
		"--cdc-partition-key-overrides": "test_schema.users:pk",
	})
	pkOnExprRejectOutput := lm.GetImportCommandStderr() + lm.GetImportCommandStdout()
	assert.Contains(t, pkOnExprRejectOutput, "expression-based unique index",
		"pk on an expression-UK table must be rejected on resume, got: %s", pkOnExprRejectOutput)

	// Resume back to the valid table strategy to continue the migration.
	err = lm.ResumeImportData(true, map[string]string{
		"--cdc-partition-key-overrides": "test_schema.users:table",
	})
	testutils.FatalIfError(t, err, "failed to resume import data back to table strategy")
	time.Sleep(15 * time.Second)

	err = lm.ValidateDataConsistency([]string{`"test_schema"."users"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"test_schema"."users"`: {Inserts: 10},
	}, 60, 1)
	testutils.FatalIfError(t, err, "failed to wait for forward streaming complete")

	err = lm.ValidateDataConsistency([]string{`"test_schema"."users"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate streaming data consistency")

	err = lm.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")

	err = lm.WaitForCutoverComplete(0, 30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

}

func TestLiveMigrationWithCoveringUniqueKeyIndex(t *testing.T) {
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "covering_uk",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "covering_uk",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.users (
				id int PRIMARY KEY,
				email TEXT
			);
			CREATE UNIQUE INDEX users_lower_email_uidx ON test_schema.users (email) INCLUDE (id);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.users REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.users (id, email) SELECT i, 'user_' || i || '@example.com' FROM generate_series(1, 10) i;`,
		},
		SourceDeltaSQL: []string{
			`
		  DO $$
		  BEGIN
			FOR i IN 11..510 LOOP
			  UPDATE test_schema.users SET email = 'user_' || i || '@example.com' WHERE id = i-1;
			  INSERT INTO test_schema.users (id, email) SELECT i, 'user_' || 10 || '@example.com';

			  DELETE FROM test_schema.users WHERE id = i-1;
			  UPDATE test_schema.users SET email = 'user_' || i || '@example.com' WHERE id = i;

			  INSERT INTO test_schema.users (id, email) SELECT i-1, 'user_' || 10 || '@example.com';

			  DELETE FROM test_schema.users WHERE id = i;
			  UPDATE test_schema.users SET email = 'user_' || i || '@example.com' WHERE id = i-1;
			  INSERT INTO test_schema.users (id, email) SELECT i, 'user_' || 10 || '@example.com';

			END LOOP;
		  END $$;
		  `,
		},
		CleanupSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema CASCADE;`,
		},
	})
	defer lm.Cleanup()

	uniqueKeyConflictCountFailpointEnv := testutils.GetFailpointEnvVar(
		`github.com/yugabyte/yb-voyager/yb-voyager/cmd/uniqueKeyConflictDetected=return("count")`,
	)
	uniqueKeyConflictStatsPath := filepath.Join(
		lm.GetCurrentExportDir(), "failpoints", "unique-key-conflict-stats.json")

	err := lm.SetupContainers(context.Background())
	testutils.FatalIfError(t, err, "failed to setup containers")

	err = lm.SetupSchema()
	testutils.FatalIfError(t, err, "failed to setup schema")

	err = lm.StartExportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start export data")

	err = lm.StartImportDataWithEnv(true, nil, []string{uniqueKeyConflictCountFailpointEnv})
	testutils.FatalIfError(t, err, "failed to start import data")

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"test_schema"."users"`: 10,
	}, 30)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	err = lm.ValidateDataConsistency([]string{`"test_schema"."users"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"test_schema"."users"`: {Inserts: 1500, Updates: 1500, Deletes: 1000},
	}, 60, 1)
	testutils.FatalIfError(t, err, "failed to wait for forward streaming complete")

	conflictStats, err := testutils.ReadUniqueKeyConflictStats(uniqueKeyConflictStatsPath)
	testutils.FatalIfError(t, err, "failed to read unique key conflict stats")
	require.Greater(t, conflictStats.Total, 0, "covering UK delta should produce unique-key conflicts")
	require.Greater(t, conflictStats.ByTable[`"test_schema"."users"`], 0, "covering UK conflicts should be attributed to the table")

	err = lm.ValidateDataConsistency([]string{`"test_schema"."users"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate streaming data consistency")

	err = lm.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")

	err = lm.WaitForCutoverComplete(0, 30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")
}

// TestLiveMigrationWithCustomCdcPartitionKey is a basic end-to-end live migration that
// mixes CDC partition strategies via --cdc-partition-key-overrides: the "orders" table is
// routed by a custom column (customer_id) while "events" uses the global auto strategy
// (which resolves to pk). It asserts the persisted per-table strategy + custom-columns
// maps, then streams snapshot + CDC (custom key column customer_id is kept immutable in
// the delta) and verifies target data matches source after cutover.
func TestLiveMigrationWithCustomCdcPartitionKey(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test_custom_key",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test_custom_key",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.orders (
				id SERIAL PRIMARY KEY,
				customer_id TEXT NOT NULL,
				amount INT
			);
			CREATE TABLE test_schema.events (
				id SERIAL PRIMARY KEY,
				name TEXT,
				value INT
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.orders REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.events REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			// customer_id has cardinality 5 (C1..C5); id 1..10
			`INSERT INTO test_schema.orders (customer_id, amount)
			 SELECT 'C' || ((i % 5) + 1), i * 10 FROM generate_series(1, 10) i;`,
			`INSERT INTO test_schema.events (name, value)
			 SELECT 'evt_' || i, i FROM generate_series(1, 10) i;`,
		},
		SourceDeltaSQL: []string{
			// orders: 5 inserts, 5 updates (amount only - customer_id is immutable), 2 deletes
			`INSERT INTO test_schema.orders (customer_id, amount)
			 SELECT 'C' || ((i % 5) + 1), 1000 + i FROM generate_series(11, 15) i;`,
			`UPDATE test_schema.orders SET amount = amount + 5000 WHERE id BETWEEN 1 AND 5;`,
			`DELETE FROM test_schema.orders WHERE id BETWEEN 6 AND 7;`,
			// events: 5 inserts, 5 updates, 2 deletes
			`INSERT INTO test_schema.events (name, value)
			 SELECT 'evt_' || i, i FROM generate_series(11, 15) i;`,
			`UPDATE test_schema.events SET value = value + 5000 WHERE id BETWEEN 1 AND 5;`,
			`DELETE FROM test_schema.events WHERE id BETWEEN 6 AND 7;`,
			// High-churn loop (500 iterations) to stress the custom-key routing and generate
			// heavy same-key traffic/conflicts. Each iteration touches one order and one event
			// with an insert -> update -> delete on the same row. customer_id is chosen from the
			// low-cardinality set C1..C5 and is NEVER changed by the update (custom key must stay
			// immutable), so all three events for an order share the same customer_id and route to
			// the same channel where queue order serializes them. Explicit ids (1000+i) are used so
			// each iteration's events target a single row deterministically.
			// Net effect on both tables is zero rows added; per table this adds exactly
			// 500 inserts, 500 updates and 500 deletes.
			`DO $$
			DECLARE
				i INTEGER;
			BEGIN
				FOR i IN 1..500 LOOP
					-- orders: insert -> update amount (customer_id immutable) -> delete, same row
					INSERT INTO test_schema.orders (id, customer_id, amount)
						VALUES (1000 + i, 'C' || ((i % 5) + 1), i);
					UPDATE test_schema.orders SET amount = amount + 1 WHERE id = 1000 + i;
					DELETE FROM test_schema.orders WHERE id = 1000 + i;

					-- events: insert -> update value -> delete, same row (pk-routed)
					INSERT INTO test_schema.events (id, name, value)
						VALUES (1000 + i, 'evt_' || i, i);
					UPDATE test_schema.events SET value = value + 1 WHERE id = 1000 + i;
					DELETE FROM test_schema.events WHERE id = 1000 + i;
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

	err = lm.StartImportData(true, map[string]string{
		"--cdc-partition-key":           "auto",
		"--cdc-partition-key-overrides": "test_schema.orders:(customer_id)",
	})
	testutils.FatalIfError(t, err, "failed to start import data")
	defer lm.StopImportData()

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"test_schema"."orders"`: 10,
		`"test_schema"."events"`: 10,
	}, 120)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	// Assert the persisted per-table strategy + custom-columns maps.
	err = lm.InitMetaDB()
	testutils.FatalIfError(t, err, "failed to initialize meta db")
	testMetaDB := lm.GetMetaDB()
	importDataStatus, err := testMetaDB.GetImportDataStatusRecord()
	testutils.FatalIfError(t, err, "failed to get import data status record")

	assert.Equal(t, cmd.PARTITION_BY_CUSTOM, importDataStatus.TableToCDCPartitionKey[`"test_schema"."orders"`].Strategy,
		"orders should use the custom partition strategy")
	assert.Equal(t, cmd.PARTITION_BY_PK, importDataStatus.TableToCDCPartitionKey[`"test_schema"."events"`].Strategy,
		"events should resolve to pk under auto")
	assert.Equal(t, []string{"customer_id"}, importDataStatus.TableToCDCPartitionKey[`"test_schema"."orders"`].Columns,
		"orders custom key columns should be persisted")

	// Stream CDC changes and validate.
	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	// Base delta (5 ins / 5 upd / 2 del per table) plus the 500-iteration churn loop
	// (500 ins / 500 upd / 500 del per table) => 505 / 505 / 502 per table.
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"test_schema"."orders"`: {Inserts: 505, Updates: 505, Deletes: 502},
		`"test_schema"."events"`: {Inserts: 505, Updates: 505, Deletes: 502},
	}, 180, 5)
	testutils.FatalIfError(t, err, "failed to wait for forward streaming complete")

	err = lm.ValidateDataConsistency([]string{`"test_schema"."orders"`, `"test_schema"."events"`}, "id")
	testutils.FatalIfError(t, err, "target does not match source after streaming")

	err = lm.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")

	err = lm.WaitForCutoverComplete(0, 30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")
}

// TestLiveMigrationCustomCdcPartitionKeyNoConflict verifies that a table routed by a custom
// partition key never trips unique-key conflict detection for events that share that key.
//
// test_live has a partial unique index on (custom_key) WHERE most_recent and is routed by
// custom_key. A single-transaction delta repeatedly frees and re-uses custom_key=1 across
// different primary keys:
//
//	INSERT (1, 1, true); UPDATE most_recent=false WHERE id=1;
//	INSERT (2, 1, true); UPDATE most_recent=false WHERE id=2; ... INSERT (7, 1, true);
//
// Each "UPDATE ... false" then "INSERT ... true" pair is the classic UI conflict on the
// partial unique index (the same (custom_key=1) slot is vacated and re-claimed). Under pk
// routing these land on different channels and would be flagged as conflicts, but because
// every event carries the same custom_key (the custom partition key) they all route to one
// channel and are applied in commit order. The count failpoint must therefore record ZERO
// detected conflicts.
func TestLiveMigrationCustomCdcPartitionKeyNoConflict(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test_custom_key_no_conflict",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test_custom_key_no_conflict",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.test_live (
				id int PRIMARY KEY,
				custom_key int,
				most_recent boolean
			);
			-- Partial unique index on the custom partition key column. Any conflict on it is
			-- necessarily between rows that share the same custom_key => same custom partition
			-- key => same channel, so conflict detection must skip them.
			CREATE UNIQUE INDEX idx_test_live_custom_key ON test_schema.test_live (custom_key) WHERE most_recent;`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.test_live REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			// Snapshot rows with distinct custom_keys and most_recent=false so they don't occupy
			// the partial unique index and don't collide with the delta's ids/custom_key.
			`INSERT INTO test_schema.test_live (id, custom_key, most_recent)
			 SELECT i, i, false FROM generate_series(100, 104) i;`,
		},
		SourceDeltaSQL: []string{
			// Single transaction (DO block): all events share custom_key=1.
			`DO $$
			BEGIN
				INSERT INTO test_schema.test_live (id, custom_key, most_recent) VALUES (1, 1, true);
				UPDATE test_schema.test_live SET most_recent = false WHERE id = 1;
				INSERT INTO test_schema.test_live (id, custom_key, most_recent) VALUES (2, 1, true);
				UPDATE test_schema.test_live SET most_recent = false WHERE id = 2;
				INSERT INTO test_schema.test_live (id, custom_key, most_recent) VALUES (3, 1, true);
				UPDATE test_schema.test_live SET most_recent = false WHERE id = 3;
				INSERT INTO test_schema.test_live (id, custom_key, most_recent) VALUES (4, 1, true);
				UPDATE test_schema.test_live SET most_recent = false WHERE id = 4;
				INSERT INTO test_schema.test_live (id, custom_key, most_recent) VALUES (5, 1, true);
				UPDATE test_schema.test_live SET most_recent = false WHERE id = 5;
				INSERT INTO test_schema.test_live (id, custom_key, most_recent) VALUES (6, 1, true);
				UPDATE test_schema.test_live SET most_recent = false WHERE id = 6;
				INSERT INTO test_schema.test_live (id, custom_key, most_recent) VALUES (7, 1, true);
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

	// count-only failpoint: any detected UK conflict is recorded in the stats file.
	uniqueKeyConflictCountFailpointEnv := testutils.GetFailpointEnvVar(
		`github.com/yugabyte/yb-voyager/yb-voyager/cmd/uniqueKeyConflictDetected=return("count")`,
	)
	uniqueKeyConflictStatsPath := filepath.Join(
		lm.GetCurrentExportDir(), "failpoints", "unique-key-conflict-stats.json")

	err = lm.StartImportDataWithEnv(true, map[string]string{
		"--cdc-partition-key":           "auto",
		"--cdc-partition-key-overrides": "test_schema.test_live:(custom_key)",
	}, []string{uniqueKeyConflictCountFailpointEnv})
	testutils.FatalIfError(t, err, "failed to start import data")
	defer lm.StopImportData()

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"test_schema"."test_live"`: 5,
	}, 120)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	// Assert the persisted per-table custom strategy + columns.
	err = lm.InitMetaDB()
	testutils.FatalIfError(t, err, "failed to initialize meta db")
	importDataStatus, err := lm.GetMetaDB().GetImportDataStatusRecord()
	testutils.FatalIfError(t, err, "failed to get import data status record")
	assert.Equal(t, cmd.PARTITION_BY_CUSTOM, importDataStatus.TableToCDCPartitionKey[`"test_schema"."test_live"`].Strategy,
		"test_live should use the custom partition strategy")
	assert.Equal(t, []string{"custom_key"}, importDataStatus.TableToCDCPartitionKey[`"test_schema"."test_live"`].Columns,
		"test_live custom key columns should be persisted")

	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	// Delta: 7 inserts, 6 updates, 0 deletes.
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"test_schema"."test_live"`: {Inserts: 7, Updates: 6, Deletes: 0},
	}, 120, 5)
	testutils.FatalIfError(t, err, "failed to wait for forward streaming complete")

	conflicts, err := testutils.ReadUniqueKeyConflictStats(uniqueKeyConflictStatsPath)
	if err != nil && !os.IsNotExist(err) {
		testutils.FatalIfError(t, err, "failed to read unique key conflict stats")
	}
	require.Nil(t, conflicts, "no unique-key conflicts should be detected: all events share the custom key => same channel")

	err = lm.ValidateDataConsistency([]string{`"test_schema"."test_live"`}, "id")
	testutils.FatalIfError(t, err, "target does not match source after streaming")

	err = lm.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")

	err = lm.WaitForCutoverComplete(0, 30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")
}


func TestLiveMigrationCustomCaseSensitiveCdcPartitionKeyNoConflict(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test_custom_key_no_conflict",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test_custom_key_no_conflict",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.test_live (
				id int PRIMARY KEY,
				"CustomKey" int,
				most_recent boolean
			);
			CREATE TABLE test_schema.test_live_multi_case (
				id int PRIMARY KEY,
				"customKey" int,
			    "customKey1" int,
				most_recent boolean
			);
			-- Partial unique index on the custom partition key column. Any conflict on it is
			-- necessarily between rows that share the same "customKey" => same custom partition
			-- key => same channel, so conflict detection must skip them.
			CREATE UNIQUE INDEX idx_test_live_custom_key ON test_schema.test_live ("CustomKey") WHERE most_recent;`,
			`CREATE UNIQUE INDEX idx_test_live_multi_case_custom_key ON test_schema.test_live_multi_case ("customKey","customKey1") WHERE most_recent;`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.test_live REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.test_live_multi_case REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			// Snapshot rows with distinct custom_keys and most_recent=false so they don't occupy
			// the partial unique index and don't collide with the delta's ids/custom_key.
			`INSERT INTO test_schema.test_live (id, "CustomKey", most_recent)
			 SELECT i, i, false FROM generate_series(100, 104) i;`,
			 `INSERT INTO test_schema.test_live_multi_case (id, "customKey", "customKey1", most_recent)
			 SELECT i, i, i, false FROM generate_series(100, 104) i;`,
		},
		SourceDeltaSQL: []string{
			// Single transaction (DO block): all events share custom_key=1.
			`DO $$
			BEGIN
				INSERT INTO test_schema.test_live (id, "CustomKey", most_recent) VALUES (1, 1, true);
				UPDATE test_schema.test_live SET most_recent = false WHERE id = 1;
				INSERT INTO test_schema.test_live (id, "CustomKey", most_recent) VALUES (2, 1, true);
				UPDATE test_schema.test_live SET most_recent = false WHERE id = 2;
				INSERT INTO test_schema.test_live (id, "CustomKey", most_recent) VALUES (3, 1, true);
				UPDATE test_schema.test_live SET most_recent = false WHERE id = 3;
				INSERT INTO test_schema.test_live (id, "CustomKey", most_recent) VALUES (4, 1, true);
				UPDATE test_schema.test_live SET most_recent = false WHERE id = 4;
				INSERT INTO test_schema.test_live (id, "CustomKey", most_recent) VALUES (5, 1, true);
				UPDATE test_schema.test_live SET most_recent = false WHERE id = 5;
				INSERT INTO test_schema.test_live (id, "CustomKey", most_recent) VALUES (6, 1, true);
				UPDATE test_schema.test_live SET most_recent = false WHERE id = 6;
				INSERT INTO test_schema.test_live (id, "CustomKey", most_recent) VALUES (7, 1, true);
			END $$;`,

			`DO $$
			BEGIN
				INSERT INTO test_schema.test_live_multi_case (id, "customKey", "customKey1", most_recent) VALUES (1, 1, 1, true);
				UPDATE test_schema.test_live_multi_case SET most_recent = false WHERE id = 1;
				INSERT INTO test_schema.test_live_multi_case (id, "customKey", "customKey1", most_recent) VALUES (2, 1, 1, true);
				UPDATE test_schema.test_live_multi_case SET most_recent = false WHERE id = 2;
				INSERT INTO test_schema.test_live_multi_case (id, "customKey", "customKey1", most_recent) VALUES (3, 1, 1, true);
				UPDATE test_schema.test_live_multi_case SET most_recent = false WHERE id = 3;
				INSERT INTO test_schema.test_live_multi_case (id, "customKey", "customKey1", most_recent) VALUES (4, 1, 1, true);
				UPDATE test_schema.test_live_multi_case SET most_recent = false WHERE id = 4;
				INSERT INTO test_schema.test_live_multi_case (id, "customKey", "customKey1", most_recent) VALUES (5, 1, 1, true);
				UPDATE test_schema.test_live_multi_case SET most_recent = false WHERE id = 5;
				INSERT INTO test_schema.test_live_multi_case (id, "customKey", "customKey1", most_recent) VALUES (6, 1, 1, true);
				UPDATE test_schema.test_live_multi_case SET most_recent = false WHERE id = 6;
				INSERT INTO test_schema.test_live_multi_case (id, "customKey", "customKey1", most_recent) VALUES (7, 1, 1, true);
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

	// count-only failpoint: any detected UK conflict is recorded in the stats file.
	uniqueKeyConflictCountFailpointEnv := testutils.GetFailpointEnvVar(
		`github.com/yugabyte/yb-voyager/yb-voyager/cmd/uniqueKeyConflictDetected=return("count")`,
	)
	uniqueKeyConflictStatsPath := filepath.Join(
		lm.GetCurrentExportDir(), "failpoints", "unique-key-conflict-stats.json")

	err = lm.StartImportDataWithEnv(true, map[string]string{
		"--cdc-partition-key":           "auto",
		"--cdc-partition-key-overrides": "test_schema.test_live:(\"CustomKey\");test_schema.test_live_multi_case:(customkey,customKey1)",
	}, []string{uniqueKeyConflictCountFailpointEnv})
	testutils.FatalIfError(t, err, "failed to start import data")
	defer lm.StopImportData()

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"test_schema"."test_live"`: 5,
		`"test_schema"."test_live_multi_case"`: 5,
	}, 120)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	// Assert the persisted per-table custom strategy + columns.
	err = lm.InitMetaDB()
	testutils.FatalIfError(t, err, "failed to initialize meta db")
	importDataStatus, err := lm.GetMetaDB().GetImportDataStatusRecord()
	testutils.FatalIfError(t, err, "failed to get import data status record")
	assert.Equal(t, cmd.PARTITION_BY_CUSTOM, importDataStatus.TableToCDCPartitionKey[`"test_schema"."test_live"`].Strategy,
		"test_live should use the custom partition strategy")
	assert.Equal(t, []string{"CustomKey"}, importDataStatus.TableToCDCPartitionKey[`"test_schema"."test_live"`].Columns,
		"test_live custom key columns should be persisted")
	assert.Equal(t, cmd.PARTITION_BY_CUSTOM, importDataStatus.TableToCDCPartitionKey[`"test_schema"."test_live_multi_case"`].Strategy,
		"test_live_multi_case should use the custom partition strategy")
	assert.Equal(t, []string{"customKey", "customKey1"}, importDataStatus.TableToCDCPartitionKey[`"test_schema"."test_live_multi_case"`].Columns,
		"test_live_multi_case custom key columns should be persisted")

	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	// Delta: 7 inserts, 6 updates, 0 deletes.
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"test_schema"."test_live"`: {Inserts: 7, Updates: 6, Deletes: 0},
		`"test_schema"."test_live_multi_case"`: {Inserts: 7, Updates: 6, Deletes: 0},
	}, 120, 5)
	testutils.FatalIfError(t, err, "failed to wait for forward streaming complete")

	conflicts, err := testutils.ReadUniqueKeyConflictStats(uniqueKeyConflictStatsPath)
	if err != nil && !os.IsNotExist(err) {
		testutils.FatalIfError(t, err, "failed to read unique key conflict stats")
	}
	require.Nil(t, conflicts, "no unique-key conflicts should be detected: all events share the custom key => same channel")

	err = lm.ValidateDataConsistency([]string{`"test_schema"."test_live"`, `"test_schema"."test_live_multi_case"`}, "id")
	testutils.FatalIfError(t, err, "target does not match source after streaming")

	err = lm.StopImportData()
	testutils.FatalIfError(t, err, "failed to stop import data")

	err = lm.StartImportData(false, map[string]string{
		"--cdc-partition-key":           "auto",
		"--cdc-partition-key-overrides": "test_schema.test_live:(custom_key);test_schema.test_live_multi_case:(customkey,customKey1)",
	})
	require.Error(t, err, "import with a non-existent custom key column should fail")
	output := lm.GetImportCommandStderr() + lm.GetImportCommandStdout()
	require.Contains(t, output, "cdc-partition-key-overrides: custom key column(s) '[custom_key]' do not exist on table 'test_schema.test_live' (available columns: [CustomKey id most_recent]",
		"expected missing-column rejection, got: %s", output)

	err = lm.StartImportData(true, map[string]string{
		"--cdc-partition-key":           "auto",
		"--cdc-partition-key-overrides": "test_schema.test_live:(Customkey);test_schema.test_live_multi_case:(customkey,customKey1)",
	})
	testutils.FatalIfError(t, err, "failed to start import data")

	err = lm.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")

	err = lm.WaitForCutoverComplete(0, 30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")
}


// TestLiveMigrationCdcPartitionKeyRejectsCustomOnExpressionUniqueIndex verifies the
// expression-UK guardrail for a custom partition key (Follow-up 1.1): a custom key routes
// by column *values*, which cannot protect an expression-based unique index (its
// conflicting value is the expression output, not a stored column). So a
// `--cdc-partition-key-overrides <table>:(cols)` on such a table must be rejected during
// prepare, before snapshot import. This exercises the real getExpressionUniqueIndexTables
// query on the target, which the pure unit tests cannot cover.
func TestLiveMigrationCdcPartitionKeyRejectsCustomOnExpressionUniqueIndex(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "cdc_custom_expr_uk",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "cdc_custom_expr_uk",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.users (
				id SERIAL PRIMARY KEY,
				email TEXT
			);
			CREATE UNIQUE INDEX users_lower_email_uidx ON test_schema.users (lower(email));`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.users REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.users (email) SELECT 'user_' || i || '@example.com' FROM generate_series(1, 10) i;`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema.users (email) SELECT 'user_' || i || '@example.com' FROM generate_series(11, 20) i;`,
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

	// custom on a table with an expression unique index must fail during prepare, before
	// snapshot import. Global stays `table`; only the override picks custom for users.
	err = lm.StartImportData(false, map[string]string{
		"--cdc-partition-key":           "table",
		"--cdc-partition-key-overrides": "test_schema.users:(email)",
	})
	require.Error(t, err, "import with custom partition-key on expression-UK table should fail")
	output := lm.GetImportCommandStderr() + lm.GetImportCommandStdout()
	assert.Contains(t, output, "cdc-partition-key custom is not allowed for table 'test_schema.users' because it has an expression-based unique index; use table (via --cdc-partition-key or --cdc-partition-key-overrides)",
		"expected expression-UK rejection, got: %s", output)

	err = lm.StartImportData(true, map[string]string{
		"--cdc-partition-key": "table",
	})
	testutils.FatalIfError(t, err, "failed to start import data")
	defer lm.StopImportData()

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"test_schema"."users"`: 10,
	}, 120)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"test_schema"."users"`: {Inserts: 10, Updates: 0, Deletes: 0},
	}, 120, 5)
	testutils.FatalIfError(t, err, "failed to wait for forward streaming complete")

	err = lm.ValidateDataConsistency([]string{`"test_schema"."users"`}, "id")
	testutils.FatalIfError(t, err, "target does not match source after streaming")

	err = lm.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")

	err = lm.WaitForCutoverComplete(0, 30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")
}

// TestLiveMigrationCdcPartitionKeyRejectsCustomKeyColumnNotOnTable verifies the
// column-existence guardrail (Follow-up 1.3): every custom key column must exist on the
// table. A misconfigured column name is caught up front during prepare (via
// validateCustomPartitionKeyTables querying the target's columns) instead of erroring
// per-event in hashEvent during streaming.
func TestLiveMigrationCdcPartitionKeyRejectsCustomKeyColumnNotOnTable(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "cdc_custom_missing_col",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "cdc_custom_missing_col",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.orders (
				id int PRIMARY KEY,
				customer_id int,
				amount int
			);
			CREATE UNIQUE INDEX orders_customer_uidx ON test_schema.orders (customer_id);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.orders REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.orders (id, customer_id, amount)
			 SELECT i, i, i * 10 FROM generate_series(1, 10) i;`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema.orders (id, customer_id, amount)
			 SELECT i, i, i * 10 FROM generate_series(11, 20) i;`,
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

	// A custom key column that does not exist on the table must fail during prepare.
	err = lm.StartImportData(false, map[string]string{
		"--cdc-partition-key":           "table",
		"--cdc-partition-key-overrides": "test_schema.orders:(nonexistent_col)",
	})
	require.Error(t, err, "import with a non-existent custom key column should fail")
	output := lm.GetImportCommandStderr() + lm.GetImportCommandStdout()
	assert.Contains(t, output, "cdc-partition-key-overrides: custom key column(s) '[nonexistent_col]' do not exist on table 'test_schema.orders' (available columns: [amount customer_id id]",
		"expected missing-column rejection, got: %s", output)

	err = lm.StartImportData(true, map[string]string{
		"--cdc-partition-key": "table",
	})
	testutils.FatalIfError(t, err, "failed to start import data")
	defer lm.StopImportData()

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"test_schema"."orders"`: 10,
	}, 120)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"test_schema"."orders"`: {Inserts: 10, Updates: 0, Deletes: 0},
	}, 120, 5)
	testutils.FatalIfError(t, err, "failed to wait for forward streaming complete")

	err = lm.ValidateDataConsistency([]string{`"test_schema"."orders"`}, "id")
	testutils.FatalIfError(t, err, "target does not match source after streaming")

	err = lm.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")

	err = lm.WaitForCutoverComplete(0, 30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")
}

func TestLiveMigrationWithSubsetOFPartialUNiqueIndexColumnsBeingChangedInUpdate(t *testing.T) {  
	t.Parallel()
	liveMigrationTest := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test_false_negative",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test_false_negative",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.test_false_negative (
				id int PRIMARY KEY,
				name TEXT,
				c1 int,
				c2 int,
				most_recent boolean
			);
			CREATE UNIQUE INDEX idx_test_false_negative_c1_c2
				ON test_schema.test_false_negative (c1, c2) WHERE most_recent;`,
		},
		SourceSetupSchemaSQL: []string{
			// REPLICA IDENTITY FULL is REQUIRED: the fix reconstructs the unchanged index
			// column (c1) from the update's before-image. Without FULL, c1 is not in
			// before_fields either and the conflict is still missed.
			"ALTER TABLE test_schema.test_false_negative REPLICA IDENTITY FULL;",
		},
		InitialDataSQL: []string{
			// Rows 1..19 are not in the partial index (most_recent=false).
			// Row 20 is the initial anchor occupying the recycled slot (c1=100, c2=1000).
			`INSERT INTO test_schema.test_false_negative (id, name, c1, c2, most_recent)
			 SELECT i, md5(random()::text), 100, i, false FROM generate_series(1, 19) AS i;`,
			`INSERT INTO test_schema.test_false_negative (id, name, c1, c2, most_recent)
			 VALUES (20, md5(random()::text), 100, 1000, true);`,
		},
		SourceDeltaSQL: []string{
			/*
				Composite PARTIAL unique index: (c1, c2) WHERE most_recent.
				Invariant at each loop start: row (i-1) = (c1=100, c2=1000, most_recent=true),
				the only row currently present in the partial index.

				Per loop, one UPDATE-INSERT-style conflict on the recycled slot (100,1000):

				  FREE:  UPDATE id=i-1 SET most_recent=false          -- removes (100,1000) from the index
				  TAKE:  UPDATE id=i   SET c2=1000, most_recent=true  -- row i takes (100,1000)

				The TAKE update's SET clause contains only {c2, most_recent}; c1 is UNCHANGED
				and therefore ABSENT from the CDC after-image (Fields). Because the index is
				composite (c1, c2), the before-after conflict check cannot build the index key
				from the after-image alone -> pre-fix it silently skips the check and MISSES the
				conflict with the freed (100,1000) from row i-1 (different PK -> different channel
				-> can apply out of order -> 23505). The fix merges the unchanged c1 from the
				before-image, reconstructs (100,1000), and detects the conflict.
			*/
			`DO $$
		DECLARE
			i INTEGER;
		BEGIN
			FOR i IN 21..520 LOOP
				-- new row i, not yet in the partial index (most_recent=false)
				INSERT INTO test_schema.test_false_negative(id, name, c1, c2, most_recent)
				VALUES (i, md5(random()::text), 100, i, false);

				-- FREE the slot (only most_recent changes; c1/c2 untouched)
				UPDATE test_schema.test_false_negative SET most_recent = false WHERE id = i - 1;

				-- TAKE the slot via a SUBSET update (c1 stays 100, absent from after-image)
				UPDATE test_schema.test_false_negative SET c2 = 1000, most_recent = true WHERE id = i;

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
		`"test_schema"."test_false_negative"`: 20,
	}, 30)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	err = liveMigrationTest.ValidateDataConsistency([]string{`"test_schema"."test_false_negative"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = liveMigrationTest.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	// 500 loops x {1 insert, 2 updates, 1 delete}
	err = liveMigrationTest.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"test_schema"."test_false_negative"`: {
			Inserts: 500,
			Updates: 1000,
			Deletes: 0,
		},
	}, 120, 5)
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	// Import must not crash on a 23505: with the bug the conflict is missed, the two
	// events race, and import errors out instead of serializing them.
	require.False(t, liveMigrationTest.GetImportRunner().IsStopped(),
		"import should keep running (no unhandled unique-violation) during count failpoint mode")

	conflictStats, err := testutils.ReadUniqueKeyConflictStats(uniqueKeyConflictStatsPath)
	testutils.FatalIfError(t, err, "failed to read unique key conflict stats")

	require.Greater(t, conflictStats.Total, 0, "subset-column partial-index delta should produce UK conflicts")
	require.Greater(t, conflictStats.ByTable[`"test_schema"."test_false_negative"`], 0, "test_false_negative should produce UK conflicts")

	err = liveMigrationTest.ValidateDataConsistency([]string{`"test_schema"."test_false_negative"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = liveMigrationTest.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to target")

	err = liveMigrationTest.WaitForCutoverComplete(0, 30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")
}


// TestLiveMigrationCustomCdcPartitionKeyPKRecycleConflict verifies the primary-key guard for
// custom-key tables (Follow-up 3.2): the primary key is added to the conflict set so a
// recycled primary key across *different* custom keys is detected and serialized.
//
// test_live (id PK, region, val) is routed by the custom key (region) and has NO other unique
// index. A single-transaction delta recycles each primary key with a new region:
//
//	DELETE id=1 (region 'r_old_1'); INSERT (1, 'r_new_1', ...);
//	DELETE id=2 (region 'r_old_2'); INSERT (2, 'r_new_2', ...); ...
//
// The DELETE (before-image region 'r_old_i') and the re-INSERT (region 'r_new_i') carry
// different custom keys, so under custom routing they hash to (potentially) different channels
// and could apply out of order — a duplicate primary key. GetTableToUniqueIndexesMap excludes
// the primary key, so WITHOUT the guard this table would have an empty conflict set and the
// race would go undetected. WITH the guard the PK is a synthetic unique index: conflict
// detection sees the incoming INSERT (id=i) collide with the cached DELETE (id=i) on the PK
// and — because the two events have different custom keys — flags it. The count failpoint must
// therefore record a NON-ZERO number of detected conflicts, and the target must stay
// consistent.
func TestLiveMigrationCustomCdcPartitionKeyPKRecycleConflict(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test_custom_key_pk_recycle",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test_custom_key_pk_recycle",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			-- No unique index other than the primary key: the PK-recycle race is only detectable
			-- because Follow-up 3.2 adds the primary key to the conflict set for custom tables.
			CREATE TABLE test_schema.test_live (
				id int PRIMARY KEY,
				region text,
				val int
			);`,
		},
		SourceSetupSchemaSQL: []string{
			// REPLICA IDENTITY FULL so the DELETE event carries the region (custom key) before
			// image, which custom-key routing needs to place the delete on its channel.
			`ALTER TABLE test_schema.test_live REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.test_live (id, region, val)
			 SELECT i, 'r_old_' || i, i FROM generate_series(1, 5) i;`,
		},
		SourceDeltaSQL: []string{
			// Single transaction: recycle each PK with a new region (a new custom key). Each
			// DELETE+INSERT pair on the same id is a PK-recycle across different custom keys.
			`DO $$
			BEGIN
				DELETE FROM test_schema.test_live WHERE id = 1;
				INSERT INTO test_schema.test_live (id, region, val) VALUES (1, 'r_new_1', 101);
				DELETE FROM test_schema.test_live WHERE id = 2;
				INSERT INTO test_schema.test_live (id, region, val) VALUES (2, 'r_new_2', 102);
				DELETE FROM test_schema.test_live WHERE id = 3;
				INSERT INTO test_schema.test_live (id, region, val) VALUES (3, 'r_new_3', 103);
				DELETE FROM test_schema.test_live WHERE id = 4;
				INSERT INTO test_schema.test_live (id, region, val) VALUES (4, 'r_new_4', 104);
				DELETE FROM test_schema.test_live WHERE id = 5;
				INSERT INTO test_schema.test_live (id, region, val) VALUES (5, 'r_new_5', 105);
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

	// count-only failpoint: any detected UK conflict is recorded in the stats file.
	uniqueKeyConflictCountFailpointEnv := testutils.GetFailpointEnvVar(
		`github.com/yugabyte/yb-voyager/yb-voyager/cmd/uniqueKeyConflictDetected=return("count")`,
	)
	uniqueKeyConflictStatsPath := filepath.Join(
		lm.GetCurrentExportDir(), "failpoints", "unique-key-conflict-stats.json")

	err = lm.StartImportDataWithEnv(true, map[string]string{
		"--cdc-partition-key":           "auto",
		"--cdc-partition-key-overrides": "test_schema.test_live:(region)",
	}, []string{uniqueKeyConflictCountFailpointEnv})
	testutils.FatalIfError(t, err, "failed to start import data")
	defer lm.StopImportData()

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"test_schema"."test_live"`: 5,
	}, 120)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	// Assert the persisted per-table custom strategy + columns.
	err = lm.InitMetaDB()
	testutils.FatalIfError(t, err, "failed to initialize meta db")
	importDataStatus, err := lm.GetMetaDB().GetImportDataStatusRecord()
	testutils.FatalIfError(t, err, "failed to get import data status record")
	assert.Equal(t, cmd.PARTITION_BY_CUSTOM, importDataStatus.TableToCDCPartitionKey[`"test_schema"."test_live"`].Strategy,
		"test_live should use the custom partition strategy")
	assert.Equal(t, []string{"region"}, importDataStatus.TableToCDCPartitionKey[`"test_schema"."test_live"`].Columns,
		"test_live custom key columns should be persisted")

	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	// Delta: 5 inserts, 0 updates, 5 deletes.
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"test_schema"."test_live"`: {Inserts: 5, Updates: 0, Deletes: 5},
	}, 120, 5)
	testutils.FatalIfError(t, err, "failed to wait for forward streaming complete")

	// The PK-recycle race must be detected: each re-INSERT collides with the cached DELETE on
	// the (synthetic) primary-key index, and the two events carry different custom keys.
	conflicts, err := testutils.ReadUniqueKeyConflictStats(uniqueKeyConflictStatsPath)
	testutils.FatalIfError(t, err, "failed to read unique key conflict stats")
	require.NotNil(t, conflicts, "PK-recycle across different custom keys must be detected")
	assert.Greater(t, conflicts.Total, 0,
		"expected at least one detected PK conflict, got stats: %+v", conflicts)

	err = lm.ValidateDataConsistency([]string{`"test_schema"."test_live"`}, "id")
	testutils.FatalIfError(t, err, "target does not match source after streaming")

	err = lm.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")

	err = lm.WaitForCutoverComplete(0, 30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")
}

// TestLiveMigrationPartitionedTableWithCustomCdcPartitionKeyNoConflict is the partitioned-table
// analogue of TestLiveMigrationCustomCdcPartitionKeyNoConflict: it verifies that a *partitioned*
// table routed by a custom partition key never trips unique-key conflict detection for events
// that share that key.
//
// test_live is LIST-partitioned by region with PRIMARY KEY (id, region) and a per-leaf partial
// unique index on (custom_key) WHERE most_recent. Import references only the root, so this
// exercises several partition-aware paths for a custom-key table:
//   - the primary key ((id, region)) is discovered for the root (declared on the root and
//     inherited by leaves) and added to the conflict set for the custom table,
//   - the per-leaf partial unique indexes are merged up to the root,
//   - custom-key routing (by custom_key) hashes all events with the same custom_key to one
//     channel regardless of which partition/PK they touch.
//
// The delta's r1 loop repeatedly frees and re-uses custom_key=1 across different primary keys
// (the classic UI conflict on the partial unique index). Under pk routing these would land on
// different channels and be flagged, but because every one of those events carries the same
// custom_key they all route to one channel and apply in commit order. The r2 inserts use
// distinct custom_keys with most_recent=false, so they never enter the partial index. There are
// no deletes, so no primary key is recycled. The count failpoint must therefore record ZERO
// detected conflicts, and the target must stay consistent across both partitions.
func TestLiveMigrationPartitionedTableWithCustomCdcPartitionKeyNoConflict(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test_part_custom_key_no_conflict",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test_part_custom_key_no_conflict",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.test_live (
				id int,
				region text,
				custom_key int,
				most_recent boolean,
				PRIMARY KEY (id, region)
			) PARTITION BY LIST (region);
			CREATE TABLE test_schema.test_live_r1 PARTITION OF test_schema.test_live FOR VALUES IN ('r1');
			CREATE TABLE test_schema.test_live_r2 PARTITION OF test_schema.test_live FOR VALUES IN ('r2');
			-- Per-leaf partial unique index on the custom partition key column. Any conflict on it
			-- is necessarily between rows that share the same custom_key => same custom partition
			-- key => same channel, so conflict detection must skip them.
			CREATE UNIQUE INDEX idx_test_live_r1_custom_key ON test_schema.test_live_r1 (custom_key) WHERE most_recent;
			CREATE UNIQUE INDEX idx_test_live_r2_custom_key ON test_schema.test_live_r2 (custom_key) WHERE most_recent;`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.test_live REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.test_live_r1 REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.test_live_r2 REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			// Snapshot rows spread across both partitions with distinct custom_keys and
			// most_recent=false so they neither occupy the partial unique index nor collide with
			// the delta's ids/custom_keys.
			`INSERT INTO test_schema.test_live (id, region, custom_key, most_recent)
			 SELECT i, 'r1', i, false FROM generate_series(100, 104) i;`,
			`INSERT INTO test_schema.test_live (id, region, custom_key, most_recent)
			 SELECT i, 'r2', i, false FROM generate_series(200, 204) i;`,
		},
		SourceDeltaSQL: []string{
			// Single transaction (DO block).
			// r1: all events share custom_key=1 and repeatedly free/re-claim the (custom_key=1)
			// partial-index slot across different primary keys (custom_key stays immutable).
			// r2: two inserts with distinct custom_keys and most_recent=false (never enter the
			// partial index) to prove multi-partition data flow.
			`DO $$
			BEGIN
				INSERT INTO test_schema.test_live (id, region, custom_key, most_recent) VALUES (1, 'r1', 1, true);
				UPDATE test_schema.test_live SET most_recent = false WHERE id = 1 AND region = 'r1';
				INSERT INTO test_schema.test_live (id, region, custom_key, most_recent) VALUES (2, 'r1', 1, true);
				UPDATE test_schema.test_live SET most_recent = false WHERE id = 2 AND region = 'r1';
				INSERT INTO test_schema.test_live (id, region, custom_key, most_recent) VALUES (3, 'r1', 1, true);
				UPDATE test_schema.test_live SET most_recent = false WHERE id = 3 AND region = 'r1';
				INSERT INTO test_schema.test_live (id, region, custom_key, most_recent) VALUES (4, 'r1', 1, true);
				UPDATE test_schema.test_live SET most_recent = false WHERE id = 4 AND region = 'r1';
				INSERT INTO test_schema.test_live (id, region, custom_key, most_recent) VALUES (5, 'r1', 1, true);
				UPDATE test_schema.test_live SET most_recent = false WHERE id = 5 AND region = 'r1';
				INSERT INTO test_schema.test_live (id, region, custom_key, most_recent) VALUES (6, 'r1', 1, true);
				UPDATE test_schema.test_live SET most_recent = false WHERE id = 6 AND region = 'r1';
				INSERT INTO test_schema.test_live (id, region, custom_key, most_recent) VALUES (7, 'r1', 1, true);

				INSERT INTO test_schema.test_live (id, region, custom_key, most_recent) VALUES (300, 'r2', 50, false);
				INSERT INTO test_schema.test_live (id, region, custom_key, most_recent) VALUES (301, 'r2', 51, false);
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

	// count-only failpoint: any detected UK conflict is recorded in the stats file.
	uniqueKeyConflictCountFailpointEnv := testutils.GetFailpointEnvVar(
		`github.com/yugabyte/yb-voyager/yb-voyager/cmd/uniqueKeyConflictDetected=return("count")`,
	)
	uniqueKeyConflictStatsPath := filepath.Join(
		lm.GetCurrentExportDir(), "failpoints", "unique-key-conflict-stats.json")

	err = lm.StartImportDataWithEnv(true, map[string]string{
		"--cdc-partition-key":           "auto",
		"--cdc-partition-key-overrides": "test_schema.test_live:(custom_key)",
	}, []string{uniqueKeyConflictCountFailpointEnv})
	testutils.FatalIfError(t, err, "failed to start import data")
	defer lm.StopImportData()

	// Snapshot count is at the root level (10 rows across the two partitions).
	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"test_schema"."test_live"`: 10,
	}, 120)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	// Assert the persisted per-table custom strategy + columns.
	err = lm.InitMetaDB()
	testutils.FatalIfError(t, err, "failed to initialize meta db")
	importDataStatus, err := lm.GetMetaDB().GetImportDataStatusRecord()
	testutils.FatalIfError(t, err, "failed to get import data status record")
	assert.Equal(t, cmd.PARTITION_BY_CUSTOM, importDataStatus.TableToCDCPartitionKey[`"test_schema"."test_live"`].Strategy,
		"test_live should use the custom partition strategy")
	assert.Equal(t, []string{"custom_key"}, importDataStatus.TableToCDCPartitionKey[`"test_schema"."test_live"`].Columns,
		"test_live custom key columns should be persisted")

	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	// Delta (root-level counts): 9 inserts (7 in r1 + 2 in r2), 6 updates, 0 deletes.
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"test_schema"."test_live"`: {Inserts: 9, Updates: 6, Deletes: 0},
	}, 120, 5)
	testutils.FatalIfError(t, err, "failed to wait for forward streaming complete")

	conflicts, err := testutils.ReadUniqueKeyConflictStats(uniqueKeyConflictStatsPath)
	if err != nil && !os.IsNotExist(err) {
		testutils.FatalIfError(t, err, "failed to read unique key conflict stats")
	}
	require.Nil(t, conflicts, "no unique-key conflicts should be detected: all r1 events share the custom key => same channel")

	
	err = lm.ValidateDataConsistency([]string{`"test_schema"."test_live"`}, "id")
	testutils.FatalIfError(t, err, "target does not match source after streaming")

	err = lm.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")

	err = lm.WaitForCutoverComplete(0, 30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")
}

// TestLiveMigrationPartitionedTableWithCustomCdcPartitionKeyPKRecycleConflict is the
// partitioned-table analogue of TestLiveMigrationCustomCdcPartitionKeyPKRecycleConflict: it
// verifies the primary-key guard for a *partitioned* custom-key table whose primary key is
// declared on the root (PRIMARY KEY (id, region)).
//
// test_live is LIST-partitioned by region and routed by the custom key (custom_key); it has no
// unique index other than the primary key. Import references only the root, so the root PK
// ((id, region)) is discovered and added to the conflict set for the custom table. A
// single-transaction delta recycles each primary key with a NEW custom key:
//
//	DELETE id=i, region=... (old custom_key); INSERT (i, same region, new custom_key, ...);
//
// The DELETE (before-image custom_key=old) and the re-INSERT (custom_key=new) carry different
// custom keys, so under custom routing they hash to (potentially) different channels and could
// apply out of order — a duplicate primary key. GetTableToUniqueIndexesMap excludes the primary
// key, so WITHOUT the guard this table would have an empty conflict set and the race would go
// undetected. WITH the guard the PK is a synthetic unique index and the collision on
// (id, region) — across different custom keys — is flagged. The count failpoint must therefore
// record a NON-ZERO number of detected conflicts, and the target must stay consistent.
func TestLiveMigrationPartitionedTableWithCustomCdcPartitionKeyPKRecycleConflict(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test_part_custom_key_pk_recycle",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test_part_custom_key_pk_recycle",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			-- No unique index other than the primary key: the PK-recycle race is only detectable
			-- because the primary key ((id, region), declared on the root) is added to the
			-- conflict set for custom tables.
			CREATE TABLE test_schema.test_live (
				id int,
				region text,
				custom_key int,
				val int,
				PRIMARY KEY (id, region)
			) PARTITION BY LIST (region);
			CREATE TABLE test_schema.test_live_r1 PARTITION OF test_schema.test_live FOR VALUES IN ('r1');
			CREATE TABLE test_schema.test_live_r2 PARTITION OF test_schema.test_live FOR VALUES IN ('r2');`,
		},
		SourceSetupSchemaSQL: []string{
			// REPLICA IDENTITY FULL so the DELETE event carries the custom_key (custom key) before
			// image, which custom-key routing needs to place the delete on its channel.
			`ALTER TABLE test_schema.test_live REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.test_live_r1 REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.test_live_r2 REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.test_live (id, region, custom_key, val)
			 SELECT i, 'r1', i, i FROM generate_series(1, 3) i;`,
			`INSERT INTO test_schema.test_live (id, region, custom_key, val)
			 SELECT i, 'r2', 10 + i, i FROM generate_series(1, 2) i;`,
		},
		SourceDeltaSQL: []string{
			// Single transaction: recycle each (id, region) with a new custom_key. Each
			// DELETE+INSERT pair on the same primary key is a PK-recycle across different custom
			// keys.
			`DO $$
			BEGIN
				DELETE FROM test_schema.test_live WHERE id = 1 AND region = 'r1';
				INSERT INTO test_schema.test_live (id, region, custom_key, val) VALUES (1, 'r1', 101, 1001);
				DELETE FROM test_schema.test_live WHERE id = 2 AND region = 'r1';
				INSERT INTO test_schema.test_live (id, region, custom_key, val) VALUES (2, 'r1', 102, 1002);
				DELETE FROM test_schema.test_live WHERE id = 3 AND region = 'r1';
				INSERT INTO test_schema.test_live (id, region, custom_key, val) VALUES (3, 'r1', 103, 1003);
				DELETE FROM test_schema.test_live WHERE id = 1 AND region = 'r2';
				INSERT INTO test_schema.test_live (id, region, custom_key, val) VALUES (1, 'r2', 111, 1011);
				DELETE FROM test_schema.test_live WHERE id = 2 AND region = 'r2';
				INSERT INTO test_schema.test_live (id, region, custom_key, val) VALUES (2, 'r2', 112, 1012);
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

	// count-only failpoint: any detected UK conflict is recorded in the stats file.
	uniqueKeyConflictCountFailpointEnv := testutils.GetFailpointEnvVar(
		`github.com/yugabyte/yb-voyager/yb-voyager/cmd/uniqueKeyConflictDetected=return("count")`,
	)
	uniqueKeyConflictStatsPath := filepath.Join(
		lm.GetCurrentExportDir(), "failpoints", "unique-key-conflict-stats.json")

	err = lm.StartImportDataWithEnv(true, map[string]string{
		"--cdc-partition-key":           "auto",
		"--cdc-partition-key-overrides": "test_schema.test_live:(custom_key)",
	}, []string{uniqueKeyConflictCountFailpointEnv})
	testutils.FatalIfError(t, err, "failed to start import data")
	defer lm.StopImportData()

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"test_schema"."test_live"`: 5,
	}, 120)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	// Assert the persisted per-table custom strategy + columns.
	err = lm.InitMetaDB()
	testutils.FatalIfError(t, err, "failed to initialize meta db")
	importDataStatus, err := lm.GetMetaDB().GetImportDataStatusRecord()
	testutils.FatalIfError(t, err, "failed to get import data status record")
	assert.Equal(t, cmd.PARTITION_BY_CUSTOM, importDataStatus.TableToCDCPartitionKey[`"test_schema"."test_live"`].Strategy,
		"test_live should use the custom partition strategy")
	assert.Equal(t, []string{"custom_key"}, importDataStatus.TableToCDCPartitionKey[`"test_schema"."test_live"`].Columns,
		"test_live custom key columns should be persisted")

	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	// Delta (root-level counts): 5 inserts, 0 updates, 5 deletes.
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"test_schema"."test_live"`: {Inserts: 5, Updates: 0, Deletes: 5},
	}, 120, 5)
	testutils.FatalIfError(t, err, "failed to wait for forward streaming complete")

	// The PK-recycle race must be detected: each re-INSERT collides with the cached DELETE on
	// the (synthetic) primary-key index, and the two events carry different custom keys.
	conflicts, err := testutils.ReadUniqueKeyConflictStats(uniqueKeyConflictStatsPath)
	testutils.FatalIfError(t, err, "failed to read unique key conflict stats")
	require.NotNil(t, conflicts, "PK-recycle across different custom keys must be detected")
	assert.Greater(t, conflicts.Total, 0,
		"expected at least one detected PK conflict, got stats: %+v", conflicts)

	// Order by the full primary key: id alone repeats across partitions (r1/r2).
	err = lm.ValidateDataConsistency([]string{`"test_schema"."test_live"`}, "id, region")
	testutils.FatalIfError(t, err, "target does not match source after streaming")

	err = lm.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")

	err = lm.WaitForCutoverComplete(0, 30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")
}

// TestLiveMigrationPartitionedTableChildPKWithCustomCdcPartitionKeyPKRecycleConflict verifies
// the primary-key guard for a partitioned custom-key table whose primary key lives ONLY on the
// leaf partitions (the root has no primary key of its own). The import table list is always the
// partitioned root (public.orders), so the custom-key override is on the root and its primary key
// must be discovered from a leaf partition (the partition-aware GetPrimaryKeyColumnsForTables
// path) and added to the conflict set for the custom table.
//
// orders is LIST-partitioned by region; each child carries PRIMARY KEY (id). It is routed by the
// custom key (custom_key) and has no unique index other than the child primary keys. Import uses
// --use-partition-root false: the root has no PK constraint, so the upsert must target the leaf
// partition (which owns PRIMARY KEY (id)) via partition_table_name — otherwise ON CONFLICT has no
// matching constraint on the root. Conflict detection still runs against the root table using the
// leaf-discovered PK. A single-transaction delta recycles each child primary key (within one
// partition, so id is unambiguous) with a NEW custom key:
//
//	DELETE id=i, region='US' (old custom_key); INSERT (i, 'US', new custom_key, ...);
//
// The DELETE (before-image custom_key=old) and the re-INSERT (custom_key=new) carry different
// custom keys and could apply out of order across channels — a duplicate primary key. WITHOUT
// discovering the leaf PK for the root, the custom table's conflict set would be empty and the
// race would go undetected. WITH it, the collision on (id) — across different custom keys — is
// flagged. The count failpoint must therefore record a NON-ZERO number of detected conflicts,
// and the target must stay consistent.
func TestLiveMigrationPartitionedTableChildPKWithCustomCdcPartitionKeyPKRecycleConflict(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test_part_child_pk_custom_key_recycle",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test_part_child_pk_custom_key_recycle",
		},
		SchemaNames: []string{"public"},
		SchemaSQL: []string{
			// Partitioned root with NO primary key; each child partition carries PRIMARY KEY (id).
			`CREATE TABLE public.orders (
				id int,
				region text NOT NULL,
				custom_key int,
				amount bigint
			) PARTITION BY LIST (region);`,
			`CREATE TABLE public.orders_us PARTITION OF public.orders FOR VALUES IN ('US');`,
			`ALTER TABLE public.orders_us ADD PRIMARY KEY (id);`,
			`CREATE TABLE public.orders_eu PARTITION OF public.orders FOR VALUES IN ('EU');`,
			`ALTER TABLE public.orders_eu ADD PRIMARY KEY (id);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE public.orders REPLICA IDENTITY FULL;`,
			`ALTER TABLE public.orders_us REPLICA IDENTITY FULL;`,
			`ALTER TABLE public.orders_eu REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO public.orders (id, region, custom_key, amount)
			 SELECT i, 'US', i, i * 100 FROM generate_series(1, 3) i;`,
			`INSERT INTO public.orders (id, region, custom_key, amount)
			 SELECT i, 'EU', 10 + i, i * 100 FROM generate_series(1, 2) i;`,
		},
		SourceDeltaSQL: []string{
			// Single transaction: recycle each child primary key (kept within the 'US' partition
			// so id is unambiguous) with a new custom_key. Each DELETE+INSERT pair on the same id
			// is a PK-recycle across different custom keys.
			`DO $$
			BEGIN
				DELETE FROM public.orders WHERE id = 1 AND region = 'US';
				INSERT INTO public.orders (id, region, custom_key, amount) VALUES (1, 'US', 101, 1001);
				DELETE FROM public.orders WHERE id = 2 AND region = 'US';
				INSERT INTO public.orders (id, region, custom_key, amount) VALUES (2, 'US', 102, 1002);
				DELETE FROM public.orders WHERE id = 3 AND region = 'US';
				INSERT INTO public.orders (id, region, custom_key, amount) VALUES (3, 'US', 103, 1003);
			END $$;`,
		},
		CleanupSQL: []string{
			`DROP TABLE IF EXISTS public.orders CASCADE;`,
		},
	})
	defer lm.Cleanup()

	err := lm.SetupContainers(context.Background())
	testutils.FatalIfError(t, err, "failed to setup containers")

	err = lm.SetupSchema()
	testutils.FatalIfError(t, err, "failed to setup schema")

	err = lm.StartExportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start export data")

	// count-only failpoint: any detected UK conflict is recorded in the stats file.
	uniqueKeyConflictCountFailpointEnv := testutils.GetFailpointEnvVar(
		`github.com/yugabyte/yb-voyager/yb-voyager/cmd/uniqueKeyConflictDetected=return("count")`,
	)
	uniqueKeyConflictStatsPath := filepath.Join(
		lm.GetCurrentExportDir(), "failpoints", "unique-key-conflict-stats.json")

	// The import table list is the partitioned root (public.orders), so the custom-key override
	// is on the root and its primary key must be discovered from a leaf partition (the
	// partition-aware GetPrimaryKeyColumnsForTables path) to seed the conflict set. --use-partition-root
	// false is required because the root itself has no PK constraint: the upsert must target the
	// leaf partition (which owns PRIMARY KEY (id)) via partition_table_name, otherwise ON CONFLICT
	// has no matching constraint on the root.
	err = lm.StartImportDataWithEnv(true, map[string]string{
		"--cdc-partition-key":           "auto",
		"--cdc-partition-key-overrides": "public.orders:(custom_key)",
		"--use-partition-root":          "false",
	}, []string{uniqueKeyConflictCountFailpointEnv})
	testutils.FatalIfError(t, err, "failed to start import data")
	defer lm.StopImportData()

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"public"."orders"`: 5,
	}, 120)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	// Assert the persisted per-table custom strategy + columns.
	err = lm.InitMetaDB()
	testutils.FatalIfError(t, err, "failed to initialize meta db")
	importDataStatus, err := lm.GetMetaDB().GetImportDataStatusRecord()
	testutils.FatalIfError(t, err, "failed to get import data status record")
	assert.Equal(t, cmd.PARTITION_BY_CUSTOM, importDataStatus.TableToCDCPartitionKey[`"public"."orders"`].Strategy,
		"orders should use the custom partition strategy")
	assert.Equal(t, []string{"custom_key"}, importDataStatus.TableToCDCPartitionKey[`"public"."orders"`].Columns,
		"orders custom key columns should be persisted")

	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	// Delta (root-level counts): 3 inserts, 0 updates, 3 deletes.
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"public"."orders"`: {Inserts: 3, Updates: 0, Deletes: 3},
	}, 120, 5)
	testutils.FatalIfError(t, err, "failed to wait for forward streaming complete")

	// The PK-recycle race must be detected via the leaf-discovered primary key: each re-INSERT
	// collides with the cached DELETE on the (synthetic) primary-key index, and the two events
	// carry different custom keys.
	conflicts, err := testutils.ReadUniqueKeyConflictStats(uniqueKeyConflictStatsPath)
	testutils.FatalIfError(t, err, "failed to read unique key conflict stats")
	require.NotNil(t, conflicts, "PK-recycle across different custom keys must be detected")
	assert.Greater(t, conflicts.Total, 0,
		"expected at least one detected PK conflict, got stats: %+v", conflicts)

	// Order by the full primary key: id alone repeats across partitions (US/EU).
	err = lm.ValidateDataConsistency([]string{`"public"."orders"`}, "id, region")
	testutils.FatalIfError(t, err, "target does not match source after streaming")

	err = lm.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")

	err = lm.WaitForCutoverComplete(0, 30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")
}