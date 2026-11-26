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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/cmd"
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

	fromSource, _ := readExcludedColumns(t, lm, 0)
	assert.Nil(t, fromSource, "failed runs must not record an excluded set")

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

	fromSource, _ = readExcludedColumns(t, lm, 0)
	assert.Equal(t, map[string][]string{excludeColumnsOrdersTable: {"reminder"}}, fromSource)

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

	_, fromTarget := readExcludedColumns(t, lm, 0)
	assert.Nil(t, fromTarget, "failed runs must not record an excluded set")

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

	fromSource, fromTarget := readExcludedColumns(t, lm, 0)
	assert.Equal(t, map[string][]string{excludeColumnsOrdersTable: {"reminder"}}, fromSource)
	assert.Equal(t, map[string][]string{excludeColumnsOrdersTable: {"query", "reminder"}}, fromTarget)

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

		fromSource, fromTarget := readExcludedColumns(t, lm, iteration)
		assert.Equal(t, map[string][]string{excludeColumnsOrdersTable: {"reminder"}}, fromSource, "iteration %d", iteration)
		assert.Equal(t, map[string][]string{excludeColumnsOrdersTable: {"query", "reminder"}}, fromTarget, "iteration %d", iteration)

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

// readExcludedColumns returns the excluded columns recorded by the source and the target exporter
// in the given iteration's metaDB; nil means that exporter has not recorded a set.
func readExcludedColumns(t *testing.T, lm *LiveMigrationTest, iteration int) (fromSource map[string][]string, fromTarget map[string][]string) {
	err := lm.WithMetaDB(iteration, func(mdb *metadb.MetaDB) error {
		sourceRecord, err := mdb.GetExportDataSourceDBExporterStatusRecord()
		if err != nil {
			return err
		}
		if sourceRecord != nil {
			fromSource = sourceRecord.ExcludedColumns
		}
		targetRecord, err := mdb.GetExportDataTargetDBExporterStatusRecord()
		if err != nil {
			return err
		}
		if targetRecord != nil {
			fromTarget = targetRecord.ExcludedColumns
		}
		return nil
	})
	testutils.FatalIfError(t, err, "failed to read excluded columns from metaDB")
	return fromSource, fromTarget
}

// cdcStrategiesByTable returns the persisted per-table CDC partitioning strategy, keyed as stored.
func cdcStrategiesByTable(t *testing.T, lm *LiveMigrationTest) map[string]string {
	t.Helper()
	require.NoError(t, lm.InitMetaDB(), "failed to initialize meta db")
	importDataStatus, err := lm.GetMetaDB().GetImportDataStatusRecord()
	require.NoError(t, err, "failed to get import data status record")
	strategies := map[string]string{}
	for table, partitionKey := range importDataStatus.TableToCDCPartitionKey {
		strategies[table] = partitionKey.Strategy
	}
	return strategies
}

// validateColumnsConsistency compares only the given columns of table between source and target.
// Excluded columns differ by design: rows inserted during streaming get NULL in them on the target.
func validateColumnsConsistency(lm *LiveMigrationTest, table string, columns string) error {
	return lm.ValidateDataConsistency([]string{fmt.Sprintf("(SELECT %s FROM %s) AS compared", columns, table)}, "id")
}

// TestLiveMigrationExcludedUniqueIndexColumnForcesTablePartitioning pins the import guardrail for a
// unique index containing an --exclude-column-list column (#3962). Debezium streams events without
// the excluded column, and conflict detection, which runs for pk/custom partitioning, needs every
// unique-index column in the event, so it would exit with "column ... is missing from fields".
//   - auto resolves such a table to table partitioning, which skips conflict detection; a table
//     whose unique index has no excluded column stays on pk;
//   - an explicit pk or custom strategy for it is rejected before the snapshot and on resume;
//   - the delta deletes a row, inserts one, and changes the non-excluded column of the composite
//     index (code, note): each of these reached conflict detection before the fix.
func TestLiveMigrationExcludedUniqueIndexColumnForcesTablePartitioning(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "cdc_excluded_unique_index_column",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "cdc_excluded_unique_index_column",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.accounts (
				id int PRIMARY KEY,
				email text UNIQUE,
				code int,
				note text,
				val int,
				UNIQUE (code, note)
			);
			CREATE TABLE test_schema.plain_accounts (
				id int PRIMARY KEY,
				email text UNIQUE,
				val int
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.accounts REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.plain_accounts REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.accounts SELECT i, 'a' || i || '@example.com', i, 'n' || i, 0 FROM generate_series(1, 10) i;`,
			`INSERT INTO test_schema.plain_accounts SELECT i, 'p' || i || '@example.com', 0 FROM generate_series(1, 10) i;`,
		},
		SourceDeltaSQL: []string{
			`DELETE FROM test_schema.accounts WHERE id = 3;`,
			`INSERT INTO test_schema.accounts VALUES (11, 'a11@example.com', 11, 'n11', 0);`,
			`UPDATE test_schema.accounts SET code = code + 100 WHERE id = 1;`,
			`UPDATE test_schema.accounts SET val = 5 WHERE id = 2;`,
			`DELETE FROM test_schema.plain_accounts WHERE id = 3;`,
			`INSERT INTO test_schema.plain_accounts VALUES (11, 'p11@example.com', 0);`,
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

	err = lm.StartExportData(true, map[string]string{"--exclude-column-list": "test_schema.accounts:email,note"})
	testutils.FatalIfError(t, err, "failed to start export data")

	const excludedUniqueColumnRejection = "is not allowed for table test_schema.accounts because its unique index(es) " +
		"accounts_code_note_key [note], accounts_email_key [email] contain column(s) excluded via --exclude-column-list; " +
		"use table (via --cdc-partition-key or --cdc-partition-key-overrides)"

	err = lm.StartImportData(false, map[string]string{"--cdc-partition-key": "pk"})
	require.Error(t, err, "import with pk on a table with an excluded unique-index column must fail")
	output := lm.GetImportCommandStderr() + lm.GetImportCommandStdout()
	require.Contains(t, output, "cdc-partition-key pk "+excludedUniqueColumnRejection)

	err = lm.StartImportData(false, map[string]string{"--cdc-partition-key-overrides": "test_schema.accounts:(val)"})
	require.Error(t, err, "import with a custom key on a table with an excluded unique-index column must fail")
	output = lm.GetImportCommandStderr() + lm.GetImportCommandStdout()
	require.Contains(t, output, "cdc-partition-key custom "+excludedUniqueColumnRejection)

	err = lm.StartImportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start import data")

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"test_schema"."accounts"`:       10,
		`"test_schema"."plain_accounts"`: 10,
	}, 120)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	require.Equal(t, map[string]string{
		`"test_schema"."accounts"`:       cmd.PARTITION_BY_TABLE,
		`"test_schema"."plain_accounts"`: cmd.PARTITION_BY_PK,
	}, cdcStrategiesByTable(t, lm))

	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"test_schema"."accounts"`:       {Inserts: 1, Updates: 2, Deletes: 1},
		`"test_schema"."plain_accounts"`: {Inserts: 1, Updates: 0, Deletes: 1},
	}, 120, 5)
	testutils.FatalIfError(t, err, "failed to wait for forward streaming complete")
	require.False(t, lm.GetImportRunner().IsStopped(), "import data must keep streaming")

	err = validateColumnsConsistency(lm, "test_schema.accounts", "id, code, val")
	testutils.FatalIfError(t, err, "accounts diverged on the streamed columns")
	err = lm.ValidateDataConsistency([]string{`"test_schema"."plain_accounts"`}, "id")
	testutils.FatalIfError(t, err, "plain_accounts diverged")

	err = lm.WithTargetConn(func(target *sql.DB) error {
		for id, expected := range map[int][2]sql.NullString{
			1:  {{String: "a1@example.com", Valid: true}, {String: "n1", Valid: true}},
			11: {{}, {}},
		} {
			var got [2]sql.NullString
			if err := target.QueryRow(`SELECT email, note FROM test_schema.accounts WHERE id = $1`, id).Scan(&got[0], &got[1]); err != nil {
				return err
			}
			assert.Equal(t, expected, got, "excluded columns of target row id=%d", id)
		}
		return nil
	})
	testutils.FatalIfError(t, err, "failed to read excluded columns on the target")

	err = lm.StopImportData()
	testutils.FatalIfError(t, err, "failed to stop import data")
	// StartImportData builds a fresh command, so the rejected override does not leak into the
	// resume below the way ResumeImportData's accumulated args would.
	err = lm.StartImportData(false, map[string]string{"--cdc-partition-key-overrides": "test_schema.accounts:pk"})
	require.Error(t, err, "resuming with pk on the table with an excluded unique-index column must fail")
	output = lm.GetImportCommandStderr() + lm.GetImportCommandStdout()
	require.Contains(t, output, "cdc-partition-key pk "+excludedUniqueColumnRejection)

	err = lm.StartImportData(true, nil)
	testutils.FatalIfError(t, err, "failed to resume import data")

	err = lm.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")
	err = lm.WaitForCutoverComplete(0, 60)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")
}

// TestLiveMigrationExcludedUniqueIndexColumnOnPartitionedTables pins the #3962 guardrail for
// partitioned tables. Excluded columns are recorded per root table, and leaf unique indexes reach
// the guardrail only after the target merges them into their root, so both sides must line up:
//   - root_uk_events has a unique index on the root containing the excluded email;
//   - leaf_uk_events has unique indexes only on its leaves, on the excluded code;
//   - plain_partitioned_events excludes val, which no unique index contains, so it stays on pk.
//
// Updates stay within a partition, since a cross-partition update streams as a delete and an insert.
func TestLiveMigrationExcludedUniqueIndexColumnOnPartitionedTables(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "cdc_excluded_unique_column_partitioned",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "cdc_excluded_unique_column_partitioned",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.root_uk_events (
				id int,
				region text,
				email text,
				val int,
				PRIMARY KEY (id, region),
				UNIQUE (email, region)
			) PARTITION BY LIST (region);
			CREATE TABLE test_schema.root_uk_events_east PARTITION OF test_schema.root_uk_events FOR VALUES IN ('east');
			CREATE TABLE test_schema.root_uk_events_west PARTITION OF test_schema.root_uk_events FOR VALUES IN ('west');

			CREATE TABLE test_schema.leaf_uk_events (
				id int,
				region text,
				code text,
				val int,
				PRIMARY KEY (id, region)
			) PARTITION BY LIST (region);
			CREATE TABLE test_schema.leaf_uk_events_east PARTITION OF test_schema.leaf_uk_events FOR VALUES IN ('east');
			CREATE TABLE test_schema.leaf_uk_events_west PARTITION OF test_schema.leaf_uk_events FOR VALUES IN ('west');
			CREATE UNIQUE INDEX leaf_uk_events_east_code ON test_schema.leaf_uk_events_east (code);
			CREATE UNIQUE INDEX leaf_uk_events_west_code ON test_schema.leaf_uk_events_west (code);

			CREATE TABLE test_schema.plain_partitioned_events (
				id int,
				region text,
				email text,
				val int,
				PRIMARY KEY (id, region),
				UNIQUE (email, region)
			) PARTITION BY LIST (region);
			CREATE TABLE test_schema.plain_partitioned_events_east PARTITION OF test_schema.plain_partitioned_events FOR VALUES IN ('east');
			CREATE TABLE test_schema.plain_partitioned_events_west PARTITION OF test_schema.plain_partitioned_events FOR VALUES IN ('west');`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.root_uk_events REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.root_uk_events_east REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.root_uk_events_west REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.leaf_uk_events REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.leaf_uk_events_east REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.leaf_uk_events_west REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.plain_partitioned_events REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.plain_partitioned_events_east REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.plain_partitioned_events_west REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.root_uk_events SELECT i, CASE WHEN i % 2 = 1 THEN 'east' ELSE 'west' END, 'r' || i || '@example.com', 0 FROM generate_series(1, 10) i;`,
			`INSERT INTO test_schema.leaf_uk_events SELECT i, CASE WHEN i % 2 = 1 THEN 'east' ELSE 'west' END, 'c' || i, 0 FROM generate_series(1, 10) i;`,
			`INSERT INTO test_schema.plain_partitioned_events SELECT i, CASE WHEN i % 2 = 1 THEN 'east' ELSE 'west' END, 'p' || i || '@example.com', 0 FROM generate_series(1, 10) i;`,
		},
		SourceDeltaSQL: []string{
			`DELETE FROM test_schema.root_uk_events WHERE id = 3;`,
			`INSERT INTO test_schema.root_uk_events VALUES (11, 'east', 'r11@example.com', 0);`,
			`UPDATE test_schema.root_uk_events SET val = 5 WHERE id IN (1, 2);`,
			`DELETE FROM test_schema.leaf_uk_events WHERE id = 3;`,
			`INSERT INTO test_schema.leaf_uk_events VALUES (11, 'east', 'c11', 0);`,
			`UPDATE test_schema.leaf_uk_events SET val = 5 WHERE id IN (1, 2);`,
			`DELETE FROM test_schema.plain_partitioned_events WHERE id = 3;`,
			`INSERT INTO test_schema.plain_partitioned_events VALUES (11, 'east', 'p11@example.com', 0);`,
			`UPDATE test_schema.plain_partitioned_events SET email = 'p' || id || '-new@example.com' WHERE id IN (1, 2);`,
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

	err = lm.StartExportData(true, map[string]string{
		"--exclude-column-list": "test_schema.root_uk_events:email;test_schema.leaf_uk_events:code;test_schema.plain_partitioned_events:val",
	})
	testutils.FatalIfError(t, err, "failed to start export data")

	// The index name is not asserted: the error may name the root's index or any leaf's copy of it.
	for table, excludedColumn := range map[string]string{
		"test_schema.root_uk_events": "email",
		"test_schema.leaf_uk_events": "code",
	} {
		err = lm.StartImportData(false, map[string]string{"--cdc-partition-key-overrides": table + ":pk"})
		require.Error(t, err, "import with pk on %s must fail", table)
		output := lm.GetImportCommandStderr() + lm.GetImportCommandStdout()
		require.Contains(t, output, "cdc-partition-key pk is not allowed for table "+table+" because its unique index(es) ")
		require.Contains(t, output, " ["+excludedColumn+"] contain column(s) excluded via --exclude-column-list; "+
			"use table (via --cdc-partition-key or --cdc-partition-key-overrides)")
	}

	err = lm.StartImportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start import data")

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"test_schema"."root_uk_events"`:           10,
		`"test_schema"."leaf_uk_events"`:           10,
		`"test_schema"."plain_partitioned_events"`: 10,
	}, 120)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	require.Equal(t, map[string]string{
		`"test_schema"."root_uk_events"`:           cmd.PARTITION_BY_TABLE,
		`"test_schema"."leaf_uk_events"`:           cmd.PARTITION_BY_TABLE,
		`"test_schema"."plain_partitioned_events"`: cmd.PARTITION_BY_PK,
	}, cdcStrategiesByTable(t, lm))

	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"test_schema"."root_uk_events"`:           {Inserts: 1, Updates: 2, Deletes: 1},
		`"test_schema"."leaf_uk_events"`:           {Inserts: 1, Updates: 2, Deletes: 1},
		`"test_schema"."plain_partitioned_events"`: {Inserts: 1, Updates: 2, Deletes: 1},
	}, 120, 5)
	testutils.FatalIfError(t, err, "failed to wait for forward streaming complete")
	require.False(t, lm.GetImportRunner().IsStopped(), "import data must keep streaming")

	err = validateColumnsConsistency(lm, "test_schema.root_uk_events", "id, region, val")
	testutils.FatalIfError(t, err, "root_uk_events diverged on the streamed columns")
	err = validateColumnsConsistency(lm, "test_schema.leaf_uk_events", "id, region, val")
	testutils.FatalIfError(t, err, "leaf_uk_events diverged on the streamed columns")
	err = validateColumnsConsistency(lm, "test_schema.plain_partitioned_events", "id, region, email")
	testutils.FatalIfError(t, err, "plain_partitioned_events diverged on the streamed columns")

	err = lm.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")
	err = lm.WaitForCutoverComplete(0, 60)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")
}

// TestLiveMigrationCdcPartitionKeyRejectsCustomKeyOnExcludedColumn pins the import guardrail for a
// custom partition key naming an --exclude-column-list column (#3955). Debezium streams events
// without it, so routing by it would fail with "custom partition key column ... not found in event"
// on the first event. The excluded column is case-sensitive ("Region", excluded as region) to
// check that the key and the recorded set are compared by their resolved names. A custom key on
// the remaining columns is accepted and streams.
func TestLiveMigrationCdcPartitionKeyRejectsCustomKeyOnExcludedColumn(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "cdc_custom_key_on_excluded_column",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "cdc_custom_key_on_excluded_column",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.orders (
				id int PRIMARY KEY,
				customer_id int,
				"Region" text,
				amount int
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.orders REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.orders SELECT i, i % 3, 'r' || i, i * 10 FROM generate_series(1, 10) i;`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema.orders SELECT i, i % 3, 'r' || i, i * 10 FROM generate_series(11, 15) i;`,
			`UPDATE test_schema.orders SET amount = amount + 1 WHERE id = 1;`,
			`DELETE FROM test_schema.orders WHERE id = 2;`,
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

	err = lm.StartExportData(true, map[string]string{"--exclude-column-list": "test_schema.orders:region"})
	testutils.FatalIfError(t, err, "failed to start export data")

	err = lm.StartImportData(false, map[string]string{"--cdc-partition-key-overrides": `test_schema.orders:(customer_id,"Region")`})
	require.Error(t, err, "import with a custom key on an excluded column must fail")
	output := lm.GetImportCommandStderr() + lm.GetImportCommandStdout()
	require.Contains(t, output, "cdc-partition-key custom is not allowed for table test_schema.orders because custom key column(s) - [Region] are excluded columns; "+
		"use table (via --cdc-partition-key or --cdc-partition-key-overrides)")

	err = lm.StartImportData(true, map[string]string{"--cdc-partition-key-overrides": "test_schema.orders:(customer_id)"})
	testutils.FatalIfError(t, err, "failed to start import data")

	err = lm.WaitForSnapshotComplete(map[string]int64{`"test_schema"."orders"`: 10}, 120)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")
	requireCustomCdcPartitionKeyPersisted(t, lm, `"test_schema"."orders"`, []string{"customer_id"})

	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"test_schema"."orders"`: {Inserts: 5, Updates: 1, Deletes: 1},
	}, 120, 5)
	testutils.FatalIfError(t, err, "failed to wait for forward streaming complete")

	err = validateColumnsConsistency(lm, "test_schema.orders", "id, customer_id, amount")
	testutils.FatalIfError(t, err, "orders diverged on the streamed columns")

	err = lm.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")
	err = lm.WaitForCutoverComplete(0, 60)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")
}
