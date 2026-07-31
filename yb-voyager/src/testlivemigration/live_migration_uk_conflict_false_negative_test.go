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

/*
Unique-key conflict-detection FALSE-NEGATIVE reproducers.

Background
----------
During forward live migration Voyager routes CDC events to N parallel workers by a hash of
the primary key (partition-by-pk, the default/"auto" strategy for tables without an
expression unique index). Two events on *different* rows that touch the *same* unique-index
value can therefore land on different workers and race. Voyager's conflict-detection cache is
supposed to notice this and serialize them so the target never sees a transient duplicate.

The detection compares the *raw Debezium string payloads* of the unique-index columns with a
plain Go string `==` (conflictDetectionCache.go: computeConflictBucketKey / uniqueKeyColumnValuesEqual).
Detection is therefore only sound for types where the target index's btree equality equals
byte-equality of the emitted string. It is NOT sound when two values are EQUAL in the unique
index but arrive as DIFFERENT strings, e.g.:

  - numeric scale:      1.0  vs 1.00        (index-equal, string-distinct)
  - jsonb numbers:      {"v":1.0} vs {"v":1.00}
  - interval:           '1 day' vs '24 hours'
  - citext / nondet. collation: 'User@x.com' vs 'user@x.com'

For these, detection compares the freed value's string against the reclaimed value's string,
finds them different, and DOES NOT serialize the events. The reclaiming INSERT can then reach
the target before the freeing DELETE and raise a duplicate-key error (SQLSTATE 23505), which is
a non-retryable, FATAL error for import-data — the whole migration aborts.

There are also STRUCTURAL blind spots where detection never even sees the value:

  - GENERATED ALWAYS ... STORED unique-index columns — the generated column is not published in
    the logical-replication / CDC stream, so detection has no value to compare at all.
  - Deferred unique constraints — the source can legally emit a "claim before release" ordering
    (or a same-transaction swap) that the target, which does not support deferral, cannot accept
    in any per-row apply order.

Each test below drives a REAL end-to-end live migration (PostgreSQL source -> Debezium -> YB
target) and asserts the CORRECT outcome: the migration streams the workload without the target
raising a duplicate-key error. Against current logic these assertions FAIL, demonstrating the
gap. They are expected to PASS once detection is made type-aware (or such tables are forced to
partition-by-table, the way expression unique indexes already are).

These use the `integration_live_migration` tag (NOT a failpoint tag) on purpose: we WANT the
missed conflict to slip through to the database so the real 23505 surfaces. The failpoint-based
conflict tests in live_migration_uk_conflict_test.go instead assert detection PREVENTS the 23505;
they cannot observe a false negative because a missed conflict never fires the failpoint.
*/

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// uniqueViolationSubstrings are the ways a target-side duplicate-key error (SQLSTATE 23505)
// shows up in yb-voyager's import output when a missed conflict reaches the database.
var uniqueViolationSubstrings = []string{
	"violates unique constraint",
	"duplicate key value",
	"23505",
	"Duplicate key",
}

func containsUniqueViolation(s string) bool {
	for _, sub := range uniqueViolationSubstrings {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// waitForImportStop polls the (async) import-data command until it exits or the timeout
// elapses. A missed conflict makes import-data die with a fatal duplicate-key error, so a
// stop here is exactly the failure we are hunting. Returns whether it stopped and the
// combined stderr+stdout captured from the process.
func waitForImportStop(lm *LiveMigrationTest, timeout time.Duration) (stopped bool, output string) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if lm.GetImportRunner() != nil && lm.GetImportRunner().IsStopped() {
			return true, lm.GetImportCommandStderr() + "\n" + lm.GetImportCommandStdout()
		}
		time.Sleep(1 * time.Second)
	}
	return false, lm.GetImportCommandStderr() + "\n" + lm.GetImportCommandStdout()
}

// tail returns the last n lines of s, so failure messages stay readable.
func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) <= n {
		return s
	}
	return "...\n" + strings.Join(lines[len(lines)-n:], "\n")
}

// assertMigrationSurvives is the shared assertion for the value-equality false negatives. It
// waits after the conflict-generating delta and requires that import-data did NOT die with a
// duplicate-key error. Against current (string-equality) detection this FAILS.
func assertMigrationSurvives(t *testing.T, lm *LiveMigrationTest, className string) {
	t.Helper()
	stopped, out := waitForImportStop(lm, 180*time.Second)
	if stopped && containsUniqueViolation(out) {
		t.Fatalf("FALSE NEGATIVE CONFIRMED (%s):\n"+
			"conflict detection compared raw Debezium strings, missed a genuine unique-key conflict,\n"+
			"and the reclaiming event raced ahead of the freeing event -> the target raised a\n"+
			"duplicate-key error and import-data aborted. A correct (type-aware) comparison, or\n"+
			"forcing this table to partition-by-table, would have serialized the pair.\n"+
			"---- import-data output (tail) ----\n%s", className, tail(out, 25))
	}
	if stopped {
		t.Fatalf("import-data stopped unexpectedly (not a unique violation) for %s:\n%s", className, tail(out, 25))
	}
	// Still running: the missed conflict did not manifest as a crash this run. That is the
	// correct behavior; nothing to assert further here.
	t.Logf("%s: import-data still streaming after delta (no duplicate-key error observed)", className)
}

// buildFreeReclaimDelta builds a delta that repeatedly frees a unique value from one row and
// reclaims it on a brand-new row (different PK -> different worker under partition-by-pk). The
// freed and reclaimed values are EQUAL in the unique index but arrive as DIFFERENT strings
// (valueFn(i) must return an index-equal but string-distinct literal per i). Each DELETE/INSERT
// is a separate statement so they become separate CDC events in commit order.
func buildFreeReclaimDelta(schemaTable string, n int, valueFn func(i int) string) []string {
	out := make([]string, 0, 2*n)
	for i := 1; i <= n; i++ {
		out = append(out, fmt.Sprintf(`DELETE FROM %s WHERE id = %d;`, schemaTable, i-1))
		out = append(out, fmt.Sprintf(`INSERT INTO %s (id, uk, payload) VALUES (%d, %s, 'p%d');`,
			schemaTable, i, valueFn(i), i))
	}
	return out
}

const fnConflictIterations = 200

// ---------------------------------------------------------------------------
// 1. numeric scale: 1.0 == 1.00 in the index, distinct as Debezium strings
// ---------------------------------------------------------------------------

func TestLiveMigrationFalseNegativeNumericScale(t *testing.T) {
	t.Parallel()
	tbl := "test_schema.num_uk"
	reportKey := `"test_schema"."num_uk"`

	// value(i) = 1 with i trailing fractional zeros: "1.0", "1.00", ... all numerically 1,
	// every string distinct, so detection never finds a string match (guaranteed miss).
	valueFn := func(i int) string { return "1." + strings.Repeat("0", i) }

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:    ContainerConfig{Type: "postgresql", ForLive: true, DatabaseName: "fn_numeric"},
		TargetDB:    ContainerConfig{Type: "yugabytedb", DatabaseName: "fn_numeric"},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			 CREATE TABLE test_schema.num_uk (
				 id      int PRIMARY KEY,
				 uk      numeric,          -- unconstrained numeric preserves scale
				 payload text,
				 UNIQUE (uk)
			 );`,
		},
		SourceSetupSchemaSQL: []string{`ALTER TABLE test_schema.num_uk REPLICA IDENTITY FULL;`},
		InitialDataSQL:       []string{`INSERT INTO test_schema.num_uk (id, uk, payload) VALUES (0, 1.0, 'seed');`},
		SourceDeltaSQL:       buildFreeReclaimDelta(tbl, fnConflictIterations, valueFn),
		CleanupSQL:           []string{`DROP SCHEMA IF EXISTS test_schema CASCADE;`},
	})
	defer lm.Cleanup()

	require.NoError(t, lm.SetupContainers(context.Background()))
	require.NoError(t, lm.SetupSchema())
	require.NoError(t, lm.StartExportData(true, nil))
	require.NoError(t, lm.StartImportData(true, nil))
	require.NoError(t, lm.WaitForSnapshotComplete(map[string]int64{reportKey: 1}, 60))
	require.NoError(t, lm.ExecuteSourceDelta())
	assertMigrationSurvives(t, lm, "numeric scale 1.0 vs 1.00")
}

// ---------------------------------------------------------------------------
// 2. jsonb numbers: {"v":1.0} == {"v":1.00}, distinct as Debezium JSON strings
// ---------------------------------------------------------------------------

func TestLiveMigrationFalseNegativeJsonbNumericScale(t *testing.T) {
	// FINDING (previously verified end-to-end): YugabyteDB rejected a unique index on a JSONB
	// column with ERROR: INDEX on column of type 'JSONB' not yet supported (SQLSTATE 0A000), so
	// the schema was rejected at import-schema time and this false-negative class could not arise
	// for a YB target. The skip is now DYNAMIC: the driver attempts the DDL on the live target
	// and skips with whatever error the running YB returns — so if a future YB supports a jsonb
	// unique index, the FN hunt proceeds automatically.
	//
	// The hazard, if reachable: PostgreSQL jsonb preserves the numeric scale a value was written
	// with — '{"v":1.0}' and '{"v":1.00}' are distinct text but equal jsonb documents — and
	// Debezium emits the raw jsonb text, so detection's string compare would miss the conflict.
	// The numeric-column canonicalization that saves a plain `numeric` column does NOT reach
	// inside jsonb text.
	valueFn := func(i int) string {
		if i%2 == 0 {
			return `'{"v":1.0}'::jsonb`
		}
		return `'{"v":1.00}'::jsonb`
	}
	runFNCase(t, fnCase{
		name:         "jsonb",
		preflightSQL: []string{`DO $$ BEGIN IF NOT ('{"v":1.0}'::jsonb = '{"v":1.00}'::jsonb) THEN RAISE EXCEPTION 'not equal'; END IF; END $$;`},
		sourceDDL: []string{
			`CREATE TABLE test_schema.jsonb_uk (id int PRIMARY KEY, uk jsonb, payload text, UNIQUE (uk));`,
			`ALTER TABLE test_schema.jsonb_uk REPLICA IDENTITY FULL;`,
		},
		seedSQL:              []string{`INSERT INTO test_schema.jsonb_uk (id, uk, payload) VALUES (0, '{"v":1.0}'::jsonb, 'seed');`},
		deltaSQL:             buildFreeReclaimDeltaStmts("test_schema.jsonb_uk", fnConflictIterations, "id, uk, payload", valueFn),
		expectedSnapshotRows: map[string]int64{`"test_schema"."jsonb_uk"`: 1},
		expectedChanges:      map[string]ChangesCount{`"test_schema"."jsonb_uk"`: freeReclaimChanges(fnConflictIterations)},
	})
}

// ---------------------------------------------------------------------------
// 3. interval: '1 day' == '24 hours' in the index, distinct textual forms.
//    (Whether Debezium normalizes these to the same string is exactly what this test probes;
//     if it does, there is no false negative and the migration will survive.)
// ---------------------------------------------------------------------------

func TestLiveMigrationFalseNegativeInterval(t *testing.T) {
	// FINDING (previously verified end-to-end): YugabyteDB rejected a unique index on an INTERVAL
	// column with ERROR: INDEX on column of type 'INTERVAL' not yet supported (SQLSTATE 0A000), so
	// this false-negative class was unreachable on a YB target. The skip is now DYNAMIC: the driver
	// attempts the DDL on the live target and skips with whatever error the running YB returns — so
	// if a future YB supports an interval unique index, the FN hunt proceeds automatically.
	//
	// The hazard, if reachable: '1 day' == '24 hours' == '1440 minutes' == '86400 seconds' in the
	// interval btree, but they are distinct textual forms. This probes whether Debezium normalizes
	// them to the same string (no FN) or preserves the textual form (detection's string compare
	// would miss the conflict).
	forms := []string{`'1 day'::interval`, `'24 hours'::interval`, `'1440 minutes'::interval`, `'86400 seconds'::interval`}
	valueFn := func(i int) string { return forms[i%len(forms)] }
	runFNCase(t, fnCase{
		name:         "interval",
		preflightSQL: []string{`DO $$ BEGIN IF NOT ('1 day'::interval = '24 hours'::interval) THEN RAISE EXCEPTION 'not equal'; END IF; END $$;`},
		sourceDDL: []string{
			`CREATE TABLE test_schema.interval_uk (id int PRIMARY KEY, uk interval, payload text, UNIQUE (uk));`,
			`ALTER TABLE test_schema.interval_uk REPLICA IDENTITY FULL;`,
		},
		seedSQL:              []string{`INSERT INTO test_schema.interval_uk (id, uk, payload) VALUES (0, '1 day'::interval, 'seed');`},
		deltaSQL:             buildFreeReclaimDeltaStmts("test_schema.interval_uk", fnConflictIterations, "id, uk, payload", valueFn),
		expectedSnapshotRows: map[string]int64{`"test_schema"."interval_uk"`: 1},
		expectedChanges:      map[string]ChangesCount{`"test_schema"."interval_uk"`: freeReclaimChanges(fnConflictIterations)},
	})
}

// ---------------------------------------------------------------------------
// 4. citext (case-insensitive): 'User@x.com' == 'user@x.com' in the index, distinct strings.
// ---------------------------------------------------------------------------

func TestLiveMigrationFalseNegativeCitextCollation(t *testing.T) {
	// FINDING (previously verified end-to-end): YugabyteDB rejected a unique index on a citext
	// column with ERROR: INDEX on column of type 'user_defined_type' not yet supported
	// (SQLSTATE 0A000) — citext is an extension user-defined type, and analyze-schema already flags
	// PK/UK-on-citext as unsupported. The skip is now DYNAMIC: the driver attempts the DDL on the
	// live target and skips with whatever error the running YB returns — so if a future YB supports
	// a citext unique index, the FN hunt proceeds automatically.
	//
	// The hazard, if reachable: citext compares case-insensitively in the index but Debezium emits
	// the raw text, so 'user@example.com' and 'USER@EXAMPLE.COM' — index-equal, string-distinct —
	// would slip past detection's byte comparison. (The same reasoning applies to text columns
	// under a nondeterministic collation; see TestFNNondeterministicCollation.)
	//
	// The CREATE EXTENSION is the first statement of BOTH targetDDL and sourceDDL (the source
	// variant additionally sets REPLICA IDENTITY FULL); preflightSQL is nil because the extension
	// does not exist until the DDL below runs.
	valueFn := func(i int) string {
		if i%2 == 0 {
			return `'user@example.com'::citext`
		}
		return `'USER@EXAMPLE.COM'::citext`
	}
	runFNCase(t, fnCase{
		name: "citext",
		targetDDL: []string{
			`CREATE EXTENSION IF NOT EXISTS citext;`,
			`CREATE TABLE test_schema.citext_uk (id int PRIMARY KEY, uk citext, payload text, UNIQUE (uk));`,
		},
		sourceDDL: []string{
			`CREATE EXTENSION IF NOT EXISTS citext;`,
			`CREATE TABLE test_schema.citext_uk (id int PRIMARY KEY, uk citext, payload text, UNIQUE (uk));`,
			`ALTER TABLE test_schema.citext_uk REPLICA IDENTITY FULL;`,
		},
		seedSQL:              []string{`INSERT INTO test_schema.citext_uk (id, uk, payload) VALUES (0, 'user@example.com'::citext, 'seed');`},
		deltaSQL:             buildFreeReclaimDeltaStmts("test_schema.citext_uk", fnConflictIterations, "id, uk, payload", valueFn),
		expectedSnapshotRows: map[string]int64{`"test_schema"."citext_uk"`: 1},
		expectedChanges:      map[string]ChangesCount{`"test_schema"."citext_uk"`: freeReclaimChanges(fnConflictIterations)},
	})
}

// ---------------------------------------------------------------------------
// 5. deferred unique constraint: a same-transaction swap is legal on the source (checked at
//    commit) but the YB target treats the inline UNIQUE ... DEFERRABLE as an ordinary immediate
//    unique index (yugabyte-db#32212), so neither per-row apply order is acceptable -> 23505.
//    This one is deterministic: it does not depend on the cross-worker race.
// ---------------------------------------------------------------------------

func TestLiveMigrationFalseNegativeDeferredUniqueConstraint(t *testing.T) {
	t.Parallel()
	reportKey := `"test_schema"."deferred_uk"`

	// Build several DIRECT same-transaction swaps (no temp/parking value). Each swap exchanges
	// the uk values of two rows: id=a (uk=a) <-> id=b (uk=b). This is legal on the source ONLY
	// because the constraint is DEFERRABLE INITIALLY DEFERRED (checked at commit). After the
	// first UPDATE alone, two rows transiently share a uk value — which an *immediate* constraint
	// rejects. On the YB target the inline DEFERRABLE is silently ignored (yugabyte-db#32212), so
	// the constraint is immediate and applying either UPDATE on its own is a duplicate-key error,
	// regardless of ordering. (A temp/parking value would make the sequence conflict-free and
	// would NOT exercise deferral — so we deliberately do not park.)
	var delta []string
	for k := 0; k < 10; k++ {
		a, b := 2*k+1, 2*k+2
		delta = append(delta, fmt.Sprintf(
			`DO $$ BEGIN
				UPDATE test_schema.deferred_uk SET uk = %d WHERE id = %d;  -- a takes b's value
				UPDATE test_schema.deferred_uk SET uk = %d WHERE id = %d;  -- b takes a's value
			END $$;`, b, a, a, b))
	}

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:    ContainerConfig{Type: "postgresql", ForLive: true, DatabaseName: "fn_deferred"},
		TargetDB:    ContainerConfig{Type: "yugabytedb", DatabaseName: "fn_deferred"},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			 CREATE TABLE test_schema.deferred_uk (
				 id      int PRIMARY KEY,
				 uk      int UNIQUE DEFERRABLE INITIALLY DEFERRED,
				 payload text
			 );`,
		},
		SourceSetupSchemaSQL: []string{`ALTER TABLE test_schema.deferred_uk REPLICA IDENTITY FULL;`},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.deferred_uk (id, uk, payload)
			 SELECT i, i, 'seed' FROM generate_series(1, 20) AS i;`,
		},
		SourceDeltaSQL: delta,
		CleanupSQL:     []string{`DROP SCHEMA IF EXISTS test_schema CASCADE;`},
	})
	defer lm.Cleanup()

	require.NoError(t, lm.SetupContainers(context.Background()))
	require.NoError(t, lm.SetupSchema())
	require.NoError(t, lm.StartExportData(true, nil))
	require.NoError(t, lm.StartImportData(true, nil))
	require.NoError(t, lm.WaitForSnapshotComplete(map[string]int64{reportKey: 20}, 60))
	require.NoError(t, lm.ExecuteSourceDelta())
	assertMigrationSurvives(t, lm, "deferred unique constraint same-transaction swap")
}

// ---------------------------------------------------------------------------
// 6. GENERATED ALWAYS ... STORED unique column: the generated column is not published in the
//    CDC stream, so detection has no value to compare. The source generates the value; the
//    target recomputes it with an ENABLE ALWAYS trigger (fires even under
//    session_replication_role=replica, which import-data uses). A missed conflict on the
//    generated column then hits the target's UNIQUE(g) index.
// ---------------------------------------------------------------------------

func TestLiveMigrationFalseNegativeGeneratedStoredUnique(t *testing.T) {
	t.Parallel()
	reportKey := `"test_schema"."gen_uk"`

	// Free/reclaim the same generated value g=10 across new PKs. base is constant so g is
	// constant; the generated column never appears in the CDC events.
	var delta []string
	for i := 1; i <= fnConflictIterations; i++ {
		delta = append(delta, fmt.Sprintf(`DELETE FROM test_schema.gen_uk WHERE id = %d;`, i-1))
		delta = append(delta, fmt.Sprintf(`INSERT INTO test_schema.gen_uk (id, base, payload) VALUES (%d, 1, 'p%d');`, i, i))
	}

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:    ContainerConfig{Type: "postgresql", ForLive: true, DatabaseName: "fn_generated"},
		TargetDB:    ContainerConfig{Type: "yugabytedb", DatabaseName: "fn_generated"},
		SchemaNames: []string{"test_schema"},
		// Only create the schema on both sides here; the table shape differs per side (source
		// has a STORED generated column, unsupported on YB), so it is created per side below.
		SchemaSQL: []string{`CREATE SCHEMA IF NOT EXISTS test_schema;`},
		SourceSetupSchemaSQL: []string{
			`CREATE TABLE test_schema.gen_uk (
				 id      int PRIMARY KEY,
				 base    int NOT NULL,
				 g       int GENERATED ALWAYS AS (base * 10) STORED,
				 payload text,
				 UNIQUE (g)
			 );`,
			`ALTER TABLE test_schema.gen_uk REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{`INSERT INTO test_schema.gen_uk (id, base, payload) VALUES (0, 1, 'seed');`},
		SourceDeltaSQL: delta,
		CleanupSQL:     []string{`DROP SCHEMA IF EXISTS test_schema CASCADE;`},
	})
	defer lm.Cleanup()

	require.NoError(t, lm.SetupContainers(context.Background()))
	require.NoError(t, lm.SetupSchema())

	// Target table: plain g column + ENABLE ALWAYS trigger recomputing it + UNIQUE(g).
	require.NoError(t, lm.ExecuteOnTarget(
		`CREATE TABLE test_schema.gen_uk (
			 id      int PRIMARY KEY,
			 base    int NOT NULL,
			 g       int,
			 payload text
		 );`,
		`CREATE OR REPLACE FUNCTION test_schema.gen_uk_fill() RETURNS trigger AS $$
		 BEGIN NEW.g := NEW.base * 10; RETURN NEW; END; $$ LANGUAGE plpgsql;`,
		`CREATE TRIGGER gen_uk_fill_trg BEFORE INSERT OR UPDATE ON test_schema.gen_uk
			 FOR EACH ROW EXECUTE FUNCTION test_schema.gen_uk_fill();`,
		// ENABLE ALWAYS so the trigger fires during import-data (session_replication_role=replica).
		`ALTER TABLE test_schema.gen_uk ENABLE ALWAYS TRIGGER gen_uk_fill_trg;`,
		`CREATE UNIQUE INDEX gen_uk_g_uidx ON test_schema.gen_uk (g);`,
	))

	require.NoError(t, lm.StartExportData(true, nil))
	require.NoError(t, lm.StartImportData(true, nil))
	require.NoError(t, lm.WaitForSnapshotComplete(map[string]int64{reportKey: 1}, 60))

	require.NoError(t, lm.ExecuteSourceDelta())
	assertMigrationSurvives(t, lm, "GENERATED ALWAYS STORED unique column (g absent from CDC)")
}
