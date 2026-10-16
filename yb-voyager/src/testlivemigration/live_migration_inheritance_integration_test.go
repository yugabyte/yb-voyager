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
	"strings"
	"testing"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/ybversion"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// Parent and child tables of an inheritance hierarchy. Both carry their own PRIMARY KEY
// because PostgreSQL does not inherit constraints. The id ranges never overlap (parent
// below 1000, child from 1000) because the importer applies parent UPDATE/DELETE events
// without ONLY, and on a target that has the same hierarchy such a statement also hits a
// child row with the same id.
func getLiveMigrationTestForInheritedTables(t *testing.T, databaseName string) *LiveMigrationTest {
	return NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: databaseName,
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: databaseName,
		},
		SchemaNames: []string{"public"},
		SchemaSQL: []string{
			`CREATE TABLE public.parent_table (
				id INT PRIMARY KEY,
				name TEXT,
				val INT
			);`,
			`CREATE TABLE public.child_table (
				extra TEXT
			) INHERITS (public.parent_table);`,
			`ALTER TABLE public.child_table ADD PRIMARY KEY (id);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE public.parent_table REPLICA IDENTITY FULL;`,
			`ALTER TABLE public.child_table REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO public.parent_table (id, name, val) VALUES (1, 'p1', 10), (2, 'p2', 20), (3, 'p3', 30), (4, 'p4', 40), (5, 'p5', 50);`,
			`INSERT INTO public.child_table (id, name, val, extra) VALUES (1001, 'c1', 10, 'x1'), (1002, 'c2', 20, 'x2'), (1003, 'c3', 30, 'x3'), (1004, 'c4', 40, 'x4'), (1005, 'c5', 50, 'x5');`,
		},
		// ONLY keeps every statement on one table: without it an UPDATE/DELETE on the parent
		// also scans the child, and the per-table event counts below would no longer be exact.
		SourceDeltaSQL: []string{
			`
			DO $$
			BEGIN
				FOR i IN 1..100 LOOP
					INSERT INTO public.parent_table (id, name, val) VALUES (i + 5, 'p' || (i + 5), i);
					INSERT INTO public.child_table (id, name, val, extra) VALUES (i + 1005, 'c' || (i + 1005), i, 'x' || (i + 1005));
					INSERT INTO public.parent_table (id, name, val) VALUES (i + 1005, 'p' || (i + 5), i);

					UPDATE ONLY public.parent_table SET val = val + 1 WHERE id = i + 5;
					UPDATE public.parent_table SET name = name || '-u' WHERE id = i + 1005;
					UPDATE ONLY public.child_table SET val = val + 1, extra = extra || '-u' WHERE id = i + 1005;
					

					IF i % 2 = 0 THEN
						DELETE FROM ONLY public.parent_table WHERE id = i + 5;
						DELETE FROM ONLY public.child_table WHERE id = i + 1005;
					END IF;
				END LOOP;
			END $$;`,
		},
		TargetDeltaSQL: []string{
			`
			DO $$
			BEGIN
				FOR i IN 301..400 LOOP
					INSERT INTO public.parent_table (id, name, val) VALUES (i + 5, 'p' || (i + 5), i);
					INSERT INTO public.child_table (id, name, val, extra) VALUES (i + 1005, 'c' || (i + 1005), i, 'x' || (i + 1005));
					INSERT INTO public.parent_table (id, name, val) VALUES (i + 1005, 'p' || (i + 5), i);


					UPDATE ONLY public.parent_table SET val = val + 1 WHERE id = i + 5;
					UPDATE public.parent_table SET name = name || '-u' WHERE id = i + 1005;
					UPDATE ONLY public.child_table SET val = val + 1, extra = extra || '-u' WHERE id = i + 1005;

					IF i % 2 = 0 THEN
						DELETE FROM ONLY public.parent_table WHERE id = i + 5;
						DELETE FROM ONLY public.child_table WHERE id = i + 1005;
					END IF;
				END LOOP;
			END $$;`,
		},
		CleanupSQL: []string{
			`DROP TABLE IF EXISTS public.child_table CASCADE;`,
			`DROP TABLE IF EXISTS public.parent_table CASCADE;`,
		},
	})
}

// Table inheritance is GA on YugabyteDB only from 2026.1; the Tech Preview releases need a
// gflag the shared test container does not set, so CREATE TABLE ... INHERITS fails there.
func requireInheritanceOnTarget(t *testing.T, lm *LiveMigrationTest) {
	dbVersion := strings.Split(lm.GetTargetContainer().GetConfig().DBVersion, "-")[0]
	v, err := ybversion.NewYBVersion(dbVersion)
	testutils.FatalIfError(t, err, "failed to parse target yugabytedb version")
	if !v.GreaterThanOrEqual(ybversion.V2026_1_0_0) {
		t.Skipf("table inheritance is not GA on YugabyteDB %s, needs 2026.1.0.0 or later", dbVersion)
	}
}

// TestLiveMigrationWithFallbackForInheritedTables pins the invariant that an inheritance
// hierarchy is migrated as independent tables in every phase of a live migration with
// fall-back: snapshot, forward streaming, and streaming back from the target. The parent and
// the child each get their own rows on the other side and nothing else. The parent's rows must
// never be duplicated into the child and the child's rows must never be folded into the
// parent, which is the failure mode when an inheritance child is mistaken for a partition.
//
// Schema:
//   - parent_table: id PRIMARY KEY, rows with id < 1000
//   - child_table INHERITS (parent_table) with its own PRIMARY KEY, rows with id >= 1000
//
// Per table the delta is 100 INSERTs, 100 UPDATEs and 50 DELETEs on the source, and again on
// the target after cutover. Row comparisons use ONLY on the parent so they count the parent's
// own rows and not the child's, on both source and target.
func TestLiveMigrationWithFallbackForInheritedTables(t *testing.T) {
	t.Parallel()

	lm := getLiveMigrationTestForInheritedTables(t, "test_live_fb_inherited_tables")
	defer lm.Cleanup()

	err := lm.SetupContainers(context.Background())
	testutils.FatalIfError(t, err, "failed to setup containers")

	requireInheritanceOnTarget(t, lm)

	err = lm.SetupSchema()
	testutils.FatalIfError(t, err, "failed to setup schema")

	parentOwnRows := `ONLY "public"."parent_table"`
	childRows := `"public"."child_table"`
	tablesToCompare := []string{parentOwnRows, childRows}

	err = lm.StartExportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start export data")

	err = lm.StartImportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start import data")

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"public"."parent_table"`: 5,
		`"public"."child_table"`:  5,
	}, 60)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	err = lm.ValidateRowCount(tablesToCompare)
	testutils.FatalIfError(t, err, "failed to validate row count after snapshot")

	err = lm.ValidateDataConsistency(tablesToCompare, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency after snapshot")

	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"public"."parent_table"`: {Inserts: 200, Updates: 200, Deletes: 50},
		`"public"."child_table"`:  {Inserts: 100, Updates: 200, Deletes: 50},
	}, 60, 1)
	testutils.FatalIfError(t, err, "failed to wait for forward streaming complete")

	err = lm.ValidateRowCount(tablesToCompare)
	testutils.FatalIfError(t, err, "failed to validate row count after forward streaming")

	err = lm.ValidateDataConsistency(tablesToCompare, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency after forward streaming")

	err = lm.InitiateCutoverToTarget(true, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to target")

	err = lm.WaitForCutoverComplete(0, 60)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

	err = lm.ExecuteTargetDelta()
	testutils.FatalIfError(t, err, "failed to execute target delta")

	err = lm.WaitForFallbackStreamingComplete(map[string]ChangesCount{
		`"public"."parent_table"`: {Inserts: 200, Updates: 200, Deletes: 50},
		`"public"."child_table"`:  {Inserts: 100, Updates: 200, Deletes: 50},
	}, 60, 1)
	testutils.FatalIfError(t, err, "failed to wait for fallback streaming complete")

	err = lm.ValidateRowCount(tablesToCompare)
	testutils.FatalIfError(t, err, "failed to validate row count after fallback streaming")

	err = lm.ValidateDataConsistency(tablesToCompare, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency after fallback streaming")

	err = lm.InitiateCutoverToSource(nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to source")

	err = lm.WaitForCutoverSourceComplete(0, 160)
	testutils.FatalIfError(t, err, "failed to wait for cutover to source complete")

	err = lm.ValidateDataConsistency(tablesToCompare, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency after cutover to source")
}
