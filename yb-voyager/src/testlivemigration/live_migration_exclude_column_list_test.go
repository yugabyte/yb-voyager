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
package testlivemigration

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

const excludeColumnsOrdersTable = `"test_schema"."orders"`

// reminder (TIMETZ) cannot be streamed from PostgreSQL or by the YugabyteDB logical connector;
// query (TSQUERY) streams from PostgreSQL but not from YugabyteDB, so only export data from target
// has to exclude it.
func newExcludeColumnsTestConfig(databaseName string, sourceDeltaSQL []string, targetDeltaSQL []string) *TestConfig {
	return &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: databaseName,
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: databaseName,
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.orders (
				id SERIAL PRIMARY KEY,
				note TEXT,
				reminder TIMETZ,
				query TSQUERY
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.orders REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.orders (note, reminder, query) VALUES
				('one', '09:00:00+00', 'cat'),
				('two', '09:00:00+00', 'dog'),
				('three', '09:00:00+00', 'owl');`,
		},
		SourceDeltaSQL: sourceDeltaSQL,
		TargetDeltaSQL: targetDeltaSQL,
		CleanupSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema CASCADE;`,
		},
	}
}

type excludeColumnsTestRow struct {
	Note     sql.NullString
	Reminder sql.NullString
	Query    sql.NullString
}

func excludeColumnsTestValue(value string) sql.NullString {
	return sql.NullString{String: value, Valid: true}
}

func readExcludeColumnsTestRow(db *sql.DB, id int) (excludeColumnsTestRow, error) {
	var row excludeColumnsTestRow
	err := db.QueryRow(`SELECT note, reminder::text, query::text FROM test_schema.orders WHERE id = $1`, id).
		Scan(&row.Note, &row.Reminder, &row.Query)
	return row, err
}

func countExcludeColumnsTestRows(db *sql.DB, where string) (int, error) {
	var count int
	err := db.QueryRow(`SELECT count(*) FROM test_schema.orders WHERE ` + where).Scan(&count)
	return count, err
}

// TestExportDataFromSourceExcludeColumnListGuardrails pins the source exporter's guardrails for
// --exclude-column-list, before and during streaming:
//   - without the flag, a column whose datatype cannot be streamed fails export with the value to pass;
//   - an unknown table, an unknown column and a primary key column are all reported in one error;
//   - columns are matched ignoring case, and an excluded column keeps its snapshot value on the
//     target while rows inserted during streaming get NULL in it;
//   - an update that only changes an excluded column is not streamed at all;
//   - on resume the resolved set cannot change, dropping the flag fails on the unsupported column
//     again, and the same set spelt differently resumes.
func TestExportDataFromSourceExcludeColumnListGuardrails(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, newExcludeColumnsTestConfig("test_exclude_column_list_source",
		[]string{
			`INSERT INTO test_schema.orders (note, reminder, query) VALUES ('four', '10:00:00+00', 'fox');`,
			`UPDATE test_schema.orders SET note = note || ' updated', reminder = '10:00:00+00' WHERE id = 1;`,
			// changes only the excluded column, so the exporter has nothing to send for it
			`UPDATE test_schema.orders SET reminder = '10:00:00+00' WHERE id = 2;`,
		}, nil))
	defer lm.Cleanup()

	err := lm.SetupContainers(context.Background())
	testutils.FatalIfError(t, err, "failed to setup containers")
	err = lm.SetupSchema()
	testutils.FatalIfError(t, err, "failed to setup schema")

	err = lm.StartExportData(false, nil)
	require.Error(t, err, "export data must fail while an unsupported column is not excluded")
	assert.Contains(t, lm.GetExportCommandStderr(), "The following columns have datatypes not supported for this migration:\n"+
		"  test_schema.orders: reminder\n"+
		"To proceed, do one of the following:\n"+
		"  1. Exclude these columns with --exclude-column-list:\n"+
		"       --exclude-column-list 'test_schema.orders:reminder'\n")

	err = lm.StartExportData(false, map[string]string{
		"--exclude-column-list": "test_schema.orders:id,nope;test_schema.missing:x",
	})
	require.Error(t, err, "export data must reject an invalid --exclude-column-list")
	assert.Contains(t, lm.GetExportCommandStderr(), "invalid --exclude-column-list:\n"+
		"  test_schema.missing: table not found\n"+
		"  test_schema.orders: column \"nope\" does not exist\n"+
		"  test_schema.orders: column \"id\" is part of the primary key and cannot be excluded")

	err = lm.WithMetaDB(0, func(mdb *metadb.MetaDB) error {
		msr, err := mdb.GetMigrationStatusRecord()
		if err != nil {
			return err
		}
		assert.Nil(t, msr.ExcludedColumns.FromSource, "failed runs must not record an excluded set")
		return nil
	})
	testutils.FatalIfError(t, err, "failed to read migration status record")

	err = lm.StartExportData(true, map[string]string{"--exclude-column-list": "test_schema.orders:REMINDER"})
	testutils.FatalIfError(t, err, "failed to start export data")
	err = lm.StartImportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start import data")

	err = lm.WaitForSnapshotComplete(map[string]int64{excludeColumnsOrdersTable: 3}, 60)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")
	// the update of id 2 only changes reminder, so it is not among the streamed updates
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		excludeColumnsOrdersTable: {Inserts: 1, Updates: 1, Deletes: 0},
	}, 90, 1)
	testutils.FatalIfError(t, err, "failed to wait for forward streaming complete")

	err = lm.WithTargetConn(func(target *sql.DB) error {
		for id, expected := range map[int]excludeColumnsTestRow{
			1: {excludeColumnsTestValue("one updated"), excludeColumnsTestValue("09:00:00+00"), excludeColumnsTestValue("'cat'")},
			2: {excludeColumnsTestValue("two"), excludeColumnsTestValue("09:00:00+00"), excludeColumnsTestValue("'dog'")},
			4: {excludeColumnsTestValue("four"), sql.NullString{}, excludeColumnsTestValue("'fox'")},
		} {
			got, err := readExcludeColumnsTestRow(target, id)
			if err != nil {
				return err
			}
			assert.Equal(t, expected, got, "target row id=%d", id)
		}
		return nil
	})
	testutils.FatalIfError(t, err, "failed to validate target rows")

	err = lm.WithMetaDB(0, func(mdb *metadb.MetaDB) error {
		msr, err := mdb.GetMigrationStatusRecord()
		if err != nil {
			return err
		}
		assert.Equal(t, map[string][]string{excludeColumnsOrdersTable: {"reminder"}}, msr.ExcludedColumns.FromSource)
		return nil
	})
	testutils.FatalIfError(t, err, "failed to read migration status record")

	err = lm.StopExportData()
	testutils.FatalIfError(t, err, "failed to stop export data")

	err = lm.StartExportData(false, map[string]string{"--exclude-column-list": "test_schema.orders:reminder,note"})
	require.Error(t, err, "resuming export data with a different set must fail")
	assert.Contains(t, lm.GetExportCommandStderr(), "--exclude-column-list cannot change once export data has started.\n"+
		"  started with: '\"test_schema\".\"orders\":reminder'\n"+
		"  now:          '\"test_schema\".\"orders\":note,reminder'\n"+
		"Re-run with the value it started with.")

	err = lm.StartExportData(false, nil)
	require.Error(t, err, "resuming export data without the flag must fail on the unsupported column")
	assert.Contains(t, lm.GetExportCommandStderr(), "--exclude-column-list 'test_schema.orders:reminder'")

	err = lm.StartExportData(true, map[string]string{"--exclude-column-list": "TEST_SCHEMA.ORDERS:reminder"})
	testutils.FatalIfError(t, err, "failed to resume export data with the same set")

	err = lm.ExecuteOnSource(`INSERT INTO test_schema.orders (note, reminder, query) VALUES ('five', '10:00:00+00', 'fox');`)
	testutils.FatalIfError(t, err, "failed to insert on source")
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		excludeColumnsOrdersTable: {Inserts: 2, Updates: 1, Deletes: 0},
	}, 90, 1)
	testutils.FatalIfError(t, err, "failed to wait for forward streaming after resume")

	err = lm.EndMigration(nil, false)
	testutils.FatalIfError(t, err, "failed to end migration")
}

// TestExportDataFromTargetExcludeColumnListGuardrails pins the target exporter's guardrails for
// --exclude-column-list in the fall-back flow:
//   - the export data from target that import data starts after cutover passes no flag; it inherits
//     the source's set but fails on query, which only YugabyteDB cannot stream;
//   - a list that leaves out a source-excluded column and an unsupported column fails with one error
//     and one value covering both;
//   - with the full list, rows inserted on the target reach the source with NULL in the excluded
//     columns, and updates leave the source's values of those columns alone;
//   - on resume the target exporter's set cannot change.
func TestExportDataFromTargetExcludeColumnListGuardrails(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, newExcludeColumnsTestConfig("test_exclude_column_list_target",
		[]string{
			`INSERT INTO test_schema.orders (note, reminder, query) VALUES ('from source', '10:00:00+00', 'fox');`,
			`UPDATE test_schema.orders SET note = note || ' +source', reminder = '10:00:00+00' WHERE id = 1;`,
		},
		[]string{
			`INSERT INTO test_schema.orders (note, reminder, query) VALUES ('from target', '11:00:00+00', 'bird');`,
			`UPDATE test_schema.orders SET note = note || ' +target', query = 'bird' WHERE id = 2;`,
		}))
	defer lm.Cleanup()

	err := lm.SetupContainers(context.Background())
	testutils.FatalIfError(t, err, "failed to setup containers")
	err = lm.SetupSchema()
	testutils.FatalIfError(t, err, "failed to setup schema")

	err = lm.StartExportData(true, map[string]string{"--exclude-column-list": "test_schema.orders:reminder"})
	testutils.FatalIfError(t, err, "failed to start export data")
	err = lm.StartImportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start import data")

	err = lm.WaitForSnapshotComplete(map[string]int64{excludeColumnsOrdersTable: 3}, 60)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")
	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		excludeColumnsOrdersTable: {Inserts: 1, Updates: 1, Deletes: 0},
	}, 90, 1)
	testutils.FatalIfError(t, err, "failed to wait for forward streaming complete")

	err = lm.InitiateCutoverToTarget(true, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to target")

	// import data execs into export data from target once it has processed the cutover
	require.Eventually(t, func() bool { return lm.GetImportRunner().IsStopped() }, 3*time.Minute, time.Second,
		"export data from target started by import data must fail on the unsupported column")
	assert.Contains(t, lm.GetImportCommandStderr(), "The following columns have datatypes not supported for this migration:\n"+
		"  test_schema.orders: query\n"+
		"Restart 'export data from target' with these columns excluded:\n"+
		"  --exclude-column-list 'test_schema.orders:reminder,query'\n")

	err = lm.StartExportDataFromTarget(false, map[string]string{"--exclude-column-list": "test_schema.orders:note"})
	require.Error(t, err, "export data from target must fail on a missing source-excluded column and an unsupported column")
	assert.Contains(t, lm.exportFromTargetCmd.Stderr(), "The following columns were excluded by 'export data from source' and must also be in --exclude-column-list, "+
		"since their values on the target are not kept up to date:\n"+
		"  test_schema.orders: reminder\n"+
		"The following columns have datatypes not supported for this migration:\n"+
		"  test_schema.orders: query\n"+
		"Restart 'export data from target' with these columns excluded:\n"+
		"  --exclude-column-list 'test_schema.orders:reminder,note,query'\n")

	err = lm.WithMetaDB(0, func(mdb *metadb.MetaDB) error {
		msr, err := mdb.GetMigrationStatusRecord()
		if err != nil {
			return err
		}
		assert.Nil(t, msr.ExcludedColumns.FromTarget, "failed runs must not record an excluded set")
		return nil
	})
	testutils.FatalIfError(t, err, "failed to read migration status record")

	err = lm.StartExportDataFromTarget(true, map[string]string{"--exclude-column-list": "test_schema.orders:reminder,query"})
	testutils.FatalIfError(t, err, "failed to start export data from target")
	// WaitForCutoverComplete points exportFromTargetCmd at the import data process, which exited
	// above; keep the export data from target this test started instead.
	exportFromTargetCmd := lm.exportFromTargetCmd
	err = lm.WaitForCutoverComplete(0, 120)
	testutils.FatalIfError(t, err, "failed to wait for cutover to target complete")
	lm.exportFromTargetCmd = exportFromTargetCmd

	err = lm.ExecuteTargetDelta()
	testutils.FatalIfError(t, err, "failed to execute target delta")
	err = lm.WaitForFallbackStreamingComplete(map[string]ChangesCount{
		excludeColumnsOrdersTable: {Inserts: 1, Updates: 1, Deletes: 0},
	}, 90, 1)
	testutils.FatalIfError(t, err, "failed to wait for fall-back streaming complete")

	err = lm.WithSourceConn(func(source *sql.DB) error {
		got, err := readExcludeColumnsTestRow(source, 2)
		if err != nil {
			return err
		}
		assert.Equal(t, excludeColumnsTestRow{excludeColumnsTestValue("two +target"), excludeColumnsTestValue("09:00:00+00"), excludeColumnsTestValue("'dog'")}, got)
		count, err := countExcludeColumnsTestRows(source, `note = 'from target' AND reminder IS NULL AND query IS NULL`)
		if err != nil {
			return err
		}
		assert.Equal(t, 1, count, "row inserted on the target must reach the source with NULL in the excluded columns")
		return nil
	})
	testutils.FatalIfError(t, err, "failed to validate source rows")

	err = lm.WithMetaDB(0, func(mdb *metadb.MetaDB) error {
		msr, err := mdb.GetMigrationStatusRecord()
		if err != nil {
			return err
		}
		assert.Equal(t, map[string][]string{excludeColumnsOrdersTable: {"reminder"}}, msr.ExcludedColumns.FromSource)
		assert.Equal(t, map[string][]string{excludeColumnsOrdersTable: {"query", "reminder"}}, msr.ExcludedColumns.FromTarget)
		return nil
	})
	testutils.FatalIfError(t, err, "failed to read migration status record")

	err = lm.StopExportDataFromTarget()
	testutils.FatalIfError(t, err, "failed to stop export data from target")
	err = lm.StartExportDataFromTarget(false, map[string]string{"--exclude-column-list": "test_schema.orders:reminder,query,note"})
	require.Error(t, err, "resuming export data from target with a different set must fail")
	assert.Contains(t, lm.exportFromTargetCmd.Stderr(), "--exclude-column-list cannot change once export data has started.\n"+
		"  started with: '\"test_schema\".\"orders\":query,reminder'\n"+
		"  now:          '\"test_schema\".\"orders\":note,query,reminder'\n"+
		"Re-run with the value it started with.")
	err = lm.StartExportDataFromTarget(true, map[string]string{"--exclude-column-list": "test_schema.orders:query,reminder"})
	testutils.FatalIfError(t, err, "failed to resume export data from target with the same set")

	err = lm.InitiateCutoverToSource(nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to source")
	err = lm.WaitForCutoverSourceComplete(0, 120)
	testutils.FatalIfError(t, err, "failed to wait for cutover to source complete")
}

// TestExcludeColumnListAcrossCutoverIterations pins that the excluded sets recorded in iteration 0
// carry through iterative cutover (cutover to source with a restart, twice):
//   - each iteration's MSR holds both recorded sets;
//   - the exporters voyager spawns stream only because they get those sets on their command lines:
//     export data from source fails on reminder, and export data from target on query, without them;
//   - in a later iteration a restarted exporter, from source or from target, cannot change its set,
//     and the same set resumes;
//   - across all iterations, rows inserted on either side reach the other with NULL in the columns
//     that side's exporter excludes.
func TestExcludeColumnListAcrossCutoverIterations(t *testing.T) {
	t.Parallel()
	const numIterations = 2
	lm := NewLiveMigrationTest(t, newExcludeColumnsTestConfig("test_exclude_column_list_iterations",
		[]string{
			`INSERT INTO test_schema.orders (note, reminder, query) VALUES ('from source', '10:00:00+00', 'fox');`,
			`UPDATE test_schema.orders SET note = note || ' +source', reminder = '10:00:00+00' WHERE id = 1;`,
		},
		[]string{
			`INSERT INTO test_schema.orders (note, reminder, query) VALUES ('from target', '11:00:00+00', 'bird');`,
			`UPDATE test_schema.orders SET note = note || ' +target', query = 'bird' WHERE id = 2;`,
		}))
	defer lm.Cleanup()

	err := lm.SetupContainers(context.Background())
	testutils.FatalIfError(t, err, "failed to setup containers")
	err = lm.SetupSchema()
	testutils.FatalIfError(t, err, "failed to setup schema")

	err = lm.StartExportData(true, map[string]string{"--exclude-column-list": "test_schema.orders:reminder"})
	testutils.FatalIfError(t, err, "failed to start export data")
	err = lm.StartImportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start import data")
	err = lm.WaitForSnapshotComplete(map[string]int64{excludeColumnsOrdersTable: 3}, 60)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		excludeColumnsOrdersTable: {Inserts: 1, Updates: 1, Deletes: 0},
	}, 90, 1)
	testutils.FatalIfError(t, err, "failed to wait for forward streaming complete")

	err = lm.InitiateCutoverToTarget(true, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to target")
	// the first export data from target is started by import data without a flag and fails on query
	require.Eventually(t, func() bool { return lm.GetImportRunner().IsStopped() }, 3*time.Minute, time.Second,
		"export data from target started by import data must fail on the unsupported column")
	err = lm.StartExportDataFromTarget(true, map[string]string{"--exclude-column-list": "test_schema.orders:reminder,query"})
	testutils.FatalIfError(t, err, "failed to start export data from target")
	// WaitForCutoverComplete points exportFromTargetCmd at the import data process, which exited
	// above; keep the export data from target this test started instead.
	exportFromTargetCmd := lm.exportFromTargetCmd
	err = lm.WaitForCutoverComplete(0, 120)
	testutils.FatalIfError(t, err, "failed to wait for cutover to target complete")
	lm.exportFromTargetCmd = exportFromTargetCmd

	err = lm.ExecuteTargetDelta()
	testutils.FatalIfError(t, err, "failed to execute target delta")
	err = lm.WaitForFallbackStreamingComplete(map[string]ChangesCount{
		excludeColumnsOrdersTable: {Inserts: 1, Updates: 1, Deletes: 0},
	}, 90, 1)
	testutils.FatalIfError(t, err, "failed to wait for fall-back streaming complete")

	for iteration := 1; iteration <= numIterations; iteration++ {
		t.Logf("Starting iteration %d", iteration)
		err = lm.InitiateCutoverToSource(map[string]string{"--restart-data-migration-source-target": "true"})
		testutils.FatalIfError(t, err, "failed to initiate cutover to source")
		err = lm.WaitForNextIterationInitialized(iteration-1, 120)
		testutils.FatalIfError(t, err, "failed to wait for next iteration initialized")
		err = lm.WaitForCutoverSourceComplete(iteration-1, 120)
		testutils.FatalIfError(t, err, "failed to wait for cutover to source complete")

		err = lm.WithMetaDB(iteration, func(mdb *metadb.MetaDB) error {
			msr, err := mdb.GetMigrationStatusRecord()
			if err != nil {
				return err
			}
			assert.Equal(t, map[string][]string{excludeColumnsOrdersTable: {"reminder"}}, msr.ExcludedColumns.FromSource, "iteration %d", iteration)
			assert.Equal(t, map[string][]string{excludeColumnsOrdersTable: {"query", "reminder"}}, msr.ExcludedColumns.FromTarget, "iteration %d", iteration)
			return nil
		})
		testutils.FatalIfError(t, err, "failed to read iteration migration status record")

		if iteration == 1 {
			// the spawned export data from source runs changes-only on the parent export dir
			err = lm.StopExportData()
			testutils.FatalIfError(t, err, "failed to stop export data from source")
			err = lm.StartExportDataChangesOnly(false, map[string]string{"--exclude-column-list": "test_schema.orders:reminder,note"})
			require.Error(t, err, "restarting export data from source with a different set must fail")
			assert.Contains(t, lm.GetExportCommandStderr(), "--exclude-column-list cannot change once export data has started.\n"+
				"  started with: '\"test_schema\".\"orders\":reminder'\n"+
				"  now:          '\"test_schema\".\"orders\":note,reminder'\n"+
				"Re-run with the value it started with.")
			err = lm.StartExportDataChangesOnly(true, map[string]string{"--exclude-column-list": "test_schema.orders:reminder"})
			testutils.FatalIfError(t, err, "failed to restart export data from source with the same set")
		}

		// export data from source of this iteration streams only with reminder excluded
		err = lm.ExecuteSourceDelta()
		testutils.FatalIfError(t, err, "failed to execute source delta")
		err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
			excludeColumnsOrdersTable: {Inserts: int64(iteration + 1), Updates: int64(iteration + 1), Deletes: 0},
		}, 120, 1)
		testutils.FatalIfError(t, err, "failed to wait for forward streaming complete")

		// cutover completes only once the spawned export data from target has started, which needs
		// query and reminder excluded
		err = lm.InitiateCutoverToTarget(true, nil)
		testutils.FatalIfError(t, err, "failed to initiate cutover to target")
		err = lm.WaitForCutoverComplete(iteration, 120)
		testutils.FatalIfError(t, err, "failed to wait for cutover to target complete")

		if iteration == 1 {
			err = lm.StopExportDataFromTarget()
			testutils.FatalIfError(t, err, "failed to stop export data from target")
			err = lm.StartExportDataFromTarget(false, map[string]string{"--exclude-column-list": "test_schema.orders:reminder,query,note"})
			require.Error(t, err, "restarting export data from target with a different set must fail")
			assert.Contains(t, lm.exportFromTargetCmd.Stderr(), "--exclude-column-list cannot change once export data has started.\n"+
				"  started with: '\"test_schema\".\"orders\":query,reminder'\n"+
				"  now:          '\"test_schema\".\"orders\":note,query,reminder'\n"+
				"Re-run with the value it started with.")
			err = lm.StartExportDataFromTarget(true, map[string]string{"--exclude-column-list": "test_schema.orders:query,reminder"})
			testutils.FatalIfError(t, err, "failed to restart export data from target with the same set")
		}

		err = lm.ExecuteTargetDelta()
		testutils.FatalIfError(t, err, "failed to execute target delta")
		err = lm.WaitForFallbackStreamingComplete(map[string]ChangesCount{
			excludeColumnsOrdersTable: {Inserts: int64(iteration + 1), Updates: int64(iteration + 1), Deletes: 0},
		}, 120, 1)
		testutils.FatalIfError(t, err, "failed to wait for fall-back streaming complete")
	}

	err = lm.WithSourceTargetConn(func(source, target *sql.DB) error {
		count, err := countExcludeColumnsTestRows(target, `note = 'from source' AND reminder IS NULL`)
		if err != nil {
			return err
		}
		assert.Equal(t, numIterations+1, count, "rows inserted on the source in every iteration must reach the target with NULL reminder")
		count, err = countExcludeColumnsTestRows(source, `note = 'from target' AND reminder IS NULL AND query IS NULL`)
		if err != nil {
			return err
		}
		assert.Equal(t, numIterations+1, count, "rows inserted on the target in every iteration must reach the source with NULL excluded columns")
		return nil
	})
	testutils.FatalIfError(t, err, "failed to validate rows after all iterations")

	err = lm.InitiateCutoverToSource(nil)
	testutils.FatalIfError(t, err, "failed to initiate final cutover to source")
	err = lm.WaitForCutoverSourceComplete(numIterations, 120)
	testutils.FatalIfError(t, err, "failed to wait for final cutover to source complete")
}
