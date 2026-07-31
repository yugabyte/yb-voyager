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
Conflict-detection false-negative hunting — PG DATA TYPES (Part 1 of
conflict-detection-fn-test-plan.md).

Every case below drives one real PostgreSQL -> Debezium -> YugabyteDB live migration through
the shared runFNCase driver (live_migration_fn_driver_test.go) and classifies the outcome
(SKIP premise / SKIP unreachable-on-YB / FAIL false-negative / FAIL stall / PASS survived).
Nothing is disregarded on documentation: every "unsupported" claim is demonstrated by
attempting the DDL at runtime (dynamic probe -> skip with the live error), and every equality
premise is proven with a preflight SQL check before the hunt.

The last two tests are BESPOKE (they do not use runFNCase):
  - TestFNSafeBatchCanonicalTypes: one migration, many tables, each probing a predicted-safe
    type; per-table dynamic probe + a single verdict loop + a support/verdict matrix.
  - TestFNProbeMatrixUnindexableTypes: NO migration; attempts every "impossible/unsupported"
    DDL on both the PG source and the YB target and logs a support matrix. Documents, never
    asserts (except on container/infrastructure errors).

Conventions (shared with the index-variant file):
  - schema test_schema; every table has PRIMARY KEY id int; unique column named uk.
  - sourceDDL carries the explicit `ALTER TABLE ... REPLICA IDENTITY FULL;` per table.
  - report keys are the quoted `"test_schema"."<table>"` form; seed row id=0.
  - insertCols is "id, uk, payload"; FN-candidate cases run 200 free/reclaim iterations.
  - No t.Parallel(): the suite is run sequentially in isolated processes.
*/

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"
)

// insertCols is the column list used by every free/reclaim delta in this suite.
const insertCols = "id, uk, payload"

// fnTypeIterations is the free/reclaim iteration count for FN-candidate type cases.
const fnTypeIterations = 200

// firstLine returns the first non-empty line of s, used to keep probe-matrix / skip errors
// readable on a single matrix row.
func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return strings.TrimSpace(s[:idx])
	}
	return s
}

// makeSpellingValueFn returns a valueFn that alternates two equal-but-differently-written
// spellings of the same value. For a single-spelling probe pass spellings[0]==spellings[1].
func makeSpellingValueFn(spellings [2]string) func(i int) string {
	return func(i int) string { return spellings[i%2] }
}

// ---------------------------------------------------------------------------
// 3. real/double precision: -0.0 == 0.0 in the btree, distinct as Java strings.
// ---------------------------------------------------------------------------

// TestFNFloatNegativeZero probes signed-zero. In an IEEE-754 btree index -0.0 and 0.0 are the
// SAME key, but Debezium/Java prints them with different signs ("-0.0" vs "0.0"), so detection's
// byte comparison sees two different strings and can miss the conflict. Both float8 and float4
// are exercised in the same migration.
func TestFNFloatNegativeZero(t *testing.T) {
	float8Key := `"test_schema"."float8_uk"`
	float4Key := `"test_schema"."float4_uk"`

	valueFn8 := func(i int) string {
		if i%2 == 0 {
			return `'0.0'::float8`
		}
		return `'-0.0'::float8`
	}
	valueFn4 := func(i int) string {
		if i%2 == 0 {
			return `'0.0'::float4`
		}
		return `'-0.0'::float4`
	}

	delta := buildFreeReclaimDeltaStmts("test_schema.float8_uk", fnTypeIterations, insertCols, valueFn8)
	delta = append(delta, buildFreeReclaimDeltaStmts("test_schema.float4_uk", fnTypeIterations, insertCols, valueFn4)...)

	runFNCase(t, fnCase{
		name:         "float_negzero",
		preflightSQL: []string{`DO $$ BEGIN IF NOT ('-0.0'::float8 = '0.0'::float8) THEN RAISE EXCEPTION 'not equal'; END IF; END $$;`},
		sourceDDL: []string{
			`CREATE TABLE test_schema.float8_uk (id int PRIMARY KEY, uk double precision, payload text, UNIQUE (uk));`,
			`ALTER TABLE test_schema.float8_uk REPLICA IDENTITY FULL;`,
			`CREATE TABLE test_schema.float4_uk (id int PRIMARY KEY, uk real, payload text, UNIQUE (uk));`,
			`ALTER TABLE test_schema.float4_uk REPLICA IDENTITY FULL;`,
		},
		seedSQL: []string{
			`INSERT INTO test_schema.float8_uk (id, uk, payload) VALUES (0, '0.0'::float8, 'seed');`,
			`INSERT INTO test_schema.float4_uk (id, uk, payload) VALUES (0, '0.0'::float4, 'seed');`,
		},
		deltaSQL:             delta,
		expectedSnapshotRows: map[string]int64{float8Key: 1, float4Key: 1},
		expectedChanges: map[string]ChangesCount{
			float8Key: freeReclaimChanges(fnTypeIterations),
			float4Key: freeReclaimChanges(fnTypeIterations),
		},
	})
}

// ---------------------------------------------------------------------------
// 26. arrays — int[] lower bounds: '[0:1]={1,2}' == '{1,2}', distinct text.
// ---------------------------------------------------------------------------

// TestFNArrayLowerBounds probes array lower-bound decoration. If PG considers an explicit
// lower-bound literal equal to the default one, they are index-equal but arrive as different
// strings; if PG considers them UNEQUAL the premise fails and the driver skips (itself a finding).
func TestFNArrayLowerBounds(t *testing.T) {
	reportKey := `"test_schema"."intarr_uk"`
	valueFn := makeSpellingValueFn([2]string{`'{1,2}'::int[]`, `'[0:1]={1,2}'::int[]`})

	runFNCase(t, fnCase{
		name:         "array_lower_bounds",
		preflightSQL: []string{`DO $$ BEGIN IF NOT ('[0:1]={1,2}'::int[] = '{1,2}'::int[]) THEN RAISE EXCEPTION 'not equal'; END IF; END $$;`},
		sourceDDL: []string{
			`CREATE TABLE test_schema.intarr_uk (id int PRIMARY KEY, uk int[], payload text, UNIQUE (uk));`,
			`ALTER TABLE test_schema.intarr_uk REPLICA IDENTITY FULL;`,
		},
		seedSQL:              []string{`INSERT INTO test_schema.intarr_uk (id, uk, payload) VALUES (0, '{1,2}'::int[], 'seed');`},
		deltaSQL:             buildFreeReclaimDeltaStmts("test_schema.intarr_uk", fnTypeIterations, insertCols, valueFn),
		expectedSnapshotRows: map[string]int64{reportKey: 1},
		expectedChanges:      map[string]ChangesCount{reportKey: freeReclaimChanges(fnTypeIterations)},
	})
}

// ---------------------------------------------------------------------------
// 27. arrays — numeric[] element scale: {1.0} == {1.00}, distinct text.
// ---------------------------------------------------------------------------

// TestFNArrayNumericScale probes whether numeric canonicalization recurses into an array. A plain
// numeric column canonicalizes 1.0/1.00 to "1"; the question is whether the exporter's Decimal
// handling reaches inside ARRAY elements. Literals cap trailing zeros at 8 to stay small while
// staying distinct from the previous iteration.
func TestFNArrayNumericScale(t *testing.T) {
	reportKey := `"test_schema"."numarr_uk"`
	valueFn := func(i int) string { return `'{1.` + strings.Repeat("0", i%8+1) + `}'::numeric[]` }

	runFNCase(t, fnCase{
		name:         "array_numeric_scale",
		preflightSQL: []string{`DO $$ BEGIN IF NOT (ARRAY[1.0::numeric] = ARRAY[1.00::numeric]) THEN RAISE EXCEPTION 'not equal'; END IF; END $$;`},
		sourceDDL: []string{
			`CREATE TABLE test_schema.numarr_uk (id int PRIMARY KEY, uk numeric[], payload text, UNIQUE (uk));`,
			`ALTER TABLE test_schema.numarr_uk REPLICA IDENTITY FULL;`,
		},
		seedSQL:              []string{`INSERT INTO test_schema.numarr_uk (id, uk, payload) VALUES (0, '{1.0}'::numeric[], 'seed');`},
		deltaSQL:             buildFreeReclaimDeltaStmts("test_schema.numarr_uk", fnTypeIterations, insertCols, valueFn),
		expectedSnapshotRows: map[string]int64{reportKey: 1},
		expectedChanges:      map[string]ChangesCount{reportKey: freeReclaimChanges(fnTypeIterations)},
	})
}

// ---------------------------------------------------------------------------
// 30. numrange: '[1.0,2.0]' == '[1.00,2.00]', bounds NOT canonicalized -> distinct text.
// ---------------------------------------------------------------------------

// TestFNNumrangeScale probes numeric-range bounds. Unlike int ranges, numrange bounds are not
// canonicalized on input, so a range with 1-digit-scale bounds and one with 2-digit-scale bounds
// are index-equal but retain distinct text.
func TestFNNumrangeScale(t *testing.T) {
	reportKey := `"test_schema"."numrange_uk"`
	valueFn := makeSpellingValueFn([2]string{`'[1.0,2.0]'::numrange`, `'[1.00,2.00]'::numrange`})

	runFNCase(t, fnCase{
		name:         "numrange_scale",
		preflightSQL: []string{`DO $$ BEGIN IF NOT ('[1.0,2.0]'::numrange = '[1.00,2.00]'::numrange) THEN RAISE EXCEPTION 'not equal'; END IF; END $$;`},
		sourceDDL: []string{
			`CREATE TABLE test_schema.numrange_uk (id int PRIMARY KEY, uk numrange, payload text, UNIQUE (uk));`,
			`ALTER TABLE test_schema.numrange_uk REPLICA IDENTITY FULL;`,
		},
		seedSQL:              []string{`INSERT INTO test_schema.numrange_uk (id, uk, payload) VALUES (0, '[1.0,2.0]'::numrange, 'seed');`},
		deltaSQL:             buildFreeReclaimDeltaStmts("test_schema.numrange_uk", fnTypeIterations, insertCols, valueFn),
		expectedSnapshotRows: map[string]int64{reportKey: 1},
		expectedChanges:      map[string]ChangesCount{reportKey: freeReclaimChanges(fnTypeIterations)},
	})
}

// ---------------------------------------------------------------------------
// 32. domain over numeric: does numeric canonicalization apply through a DOMAIN?
// ---------------------------------------------------------------------------

// TestFNDomainNumericScale probes whether the exporter's numeric canonicalization reaches a value
// whose column type is a DOMAIN over numeric. The preflight cannot reference the domain (preflight
// runs BEFORE any DDL, so the domain does not exist yet), so it proves the domain-independent
// premise 1.0::numeric = 1.00::numeric instead.
func TestFNDomainNumericScale(t *testing.T) {
	reportKey := `"test_schema"."domain_uk"`
	valueFn := func(i int) string { return `'1.` + strings.Repeat("0", i%8+1) + `'::numeric` }

	runFNCase(t, fnCase{
		name:         "domain_numeric_scale",
		preflightSQL: []string{`DO $$ BEGIN IF NOT (1.0::numeric = 1.00::numeric) THEN RAISE EXCEPTION 'not equal'; END IF; END $$;`},
		sourceDDL: []string{
			`CREATE DOMAIN test_schema.money_amt AS numeric;`,
			`CREATE TABLE test_schema.domain_uk (id int PRIMARY KEY, uk test_schema.money_amt, payload text, UNIQUE (uk));`,
			`ALTER TABLE test_schema.domain_uk REPLICA IDENTITY FULL;`,
		},
		seedSQL:              []string{`INSERT INTO test_schema.domain_uk (id, uk, payload) VALUES (0, '1.0'::numeric, 'seed');`},
		deltaSQL:             buildFreeReclaimDeltaStmts("test_schema.domain_uk", fnTypeIterations, insertCols, valueFn),
		expectedSnapshotRows: map[string]int64{reportKey: 1},
		expectedChanges:      map[string]ChangesCount{reportKey: freeReclaimChanges(fnTypeIterations)},
	})
}

// ---------------------------------------------------------------------------
// 28. composite type column: (1.0) == (1.00) field-wise, distinct text.
// ---------------------------------------------------------------------------

// TestFNCompositeNumeric probes a user-defined composite type whose only field is numeric. The
// preflight cannot reference the type (it does not exist until the DDL runs), so it proves the
// premise 1.0::numeric = 1.00::numeric instead. If YB rejects a unique index on a UDT column the
// dynamic probe skips the case with the live error — that is the expected finding.
func TestFNCompositeNumeric(t *testing.T) {
	reportKey := `"test_schema"."comp_uk"`
	valueFn := makeSpellingValueFn([2]string{`'(1.0)'::test_schema.pair`, `'(1.00)'::test_schema.pair`})

	runFNCase(t, fnCase{
		name:         "composite_numeric",
		preflightSQL: []string{`DO $$ BEGIN IF NOT (1.0::numeric = 1.00::numeric) THEN RAISE EXCEPTION 'not equal'; END IF; END $$;`},
		sourceDDL: []string{
			`CREATE TYPE test_schema.pair AS (a numeric);`,
			`CREATE TABLE test_schema.comp_uk (id int PRIMARY KEY, uk test_schema.pair, payload text, UNIQUE (uk));`,
			`ALTER TABLE test_schema.comp_uk REPLICA IDENTITY FULL;`,
		},
		seedSQL:              []string{`INSERT INTO test_schema.comp_uk (id, uk, payload) VALUES (0, '(1.0)'::test_schema.pair, 'seed');`},
		deltaSQL:             buildFreeReclaimDeltaStmts("test_schema.comp_uk", fnTypeIterations, insertCols, valueFn),
		expectedSnapshotRows: map[string]int64{reportKey: 1},
		expectedChanges:      map[string]ChangesCount{reportKey: freeReclaimChanges(fnTypeIterations)},
	})
}

// ---------------------------------------------------------------------------
// 33. hstore key order: 'a=>1,b=>2' == 'b=>2,a=>1' (definitional), distinct written order.
// ---------------------------------------------------------------------------

// TestFNHstoreKeyOrder probes whether the exporter's hstore serialization (hstore.handling.mode
// map -> a Java Map) is key-order stable. hstore storage is order-independent, so the two written
// orders are the same value; if the emitted string order varies, detection's byte comparison can
// miss the conflict. The CREATE EXTENSION is the first statement of the DDL (targetDDL is nil, so
// the same DDL — extension first — runs on both source and target). preflightSQL is deliberately
// nil: the hstore equality premise is definitional, and the extension does not exist at preflight.
func TestFNHstoreKeyOrder(t *testing.T) {
	reportKey := `"test_schema"."hstore_uk"`
	valueFn := makeSpellingValueFn([2]string{`'a=>1,b=>2'::hstore`, `'b=>2,a=>1'::hstore`})

	runFNCase(t, fnCase{
		name: "hstore_key_order",
		sourceDDL: []string{
			`CREATE EXTENSION IF NOT EXISTS hstore;`,
			`CREATE TABLE test_schema.hstore_uk (id int PRIMARY KEY, uk hstore, payload text, UNIQUE (uk));`,
			`ALTER TABLE test_schema.hstore_uk REPLICA IDENTITY FULL;`,
		},
		seedSQL:              []string{`INSERT INTO test_schema.hstore_uk (id, uk, payload) VALUES (0, 'a=>1,b=>2'::hstore, 'seed');`},
		deltaSQL:             buildFreeReclaimDeltaStmts("test_schema.hstore_uk", fnTypeIterations, insertCols, valueFn),
		expectedSnapshotRows: map[string]int64{reportKey: 1},
		expectedChanges:      map[string]ChangesCount{reportKey: freeReclaimChanges(fnTypeIterations)},
	})
}

// ---------------------------------------------------------------------------
// 35. TOAST mechanics: is a DELETE before-image detoasted, or a placeholder?
// ---------------------------------------------------------------------------

// TestFNToastLargeText probes TOAST before-images. The unique value is a deterministic ~8KB text
// that is guaranteed to be TOASTed. Each iteration frees and immediately reclaims the SAME large
// value (a genuine conflict every time). If the DELETE before-image carries the full detoasted
// value (heap RIF flattening), detection can compare it and serialize the pair -> survives. If the
// before-image carries only a TOAST placeholder, detection cannot see the real value, misses the
// conflict, and the reclaiming INSERT races ahead -> 23505.
func TestFNToastLargeText(t *testing.T) {
	const iterations = 50
	reportKey := `"test_schema"."toast_uk"`

	v := func(k int) string { return strings.Repeat(fmt.Sprintf("val-%03d-", k), 1000) } // ~8KB each
	seedVal := v(0)

	// Always reclaim the SAME large value v(0) that the DELETE just freed (no quotes in the value,
	// so single-quoted literals are safe). Written manually rather than via buildFreeReclaimDeltaStmts.
	var delta []string
	for i := 1; i <= iterations; i++ {
		delta = append(delta, fmt.Sprintf(`DELETE FROM test_schema.toast_uk WHERE id = %d;`, i-1))
		delta = append(delta, fmt.Sprintf(`INSERT INTO test_schema.toast_uk (id, uk, payload) VALUES (%d, '%s', 'p');`, i, seedVal))
	}

	runFNCase(t, fnCase{
		name: "toast_large_text",
		sourceDDL: []string{
			`CREATE TABLE test_schema.toast_uk (id int PRIMARY KEY, uk text, payload text, UNIQUE (uk));`,
			`ALTER TABLE test_schema.toast_uk REPLICA IDENTITY FULL;`,
		},
		seedSQL:              []string{fmt.Sprintf(`INSERT INTO test_schema.toast_uk (id, uk, payload) VALUES (0, '%s', 'seed');`, seedVal)},
		deltaSQL:             delta,
		expectedSnapshotRows: map[string]int64{reportKey: 1},
		expectedChanges:      map[string]ChangesCount{reportKey: freeReclaimChanges(iterations)},
	})
}

// ---------------------------------------------------------------------------
// 9 (bespoke). Safe-batch: many predicted-SAFE canonical types in one migration.
// ---------------------------------------------------------------------------

// sbEntry is one predicted-safe type probed by the safe-batch test.
type sbEntry struct {
	table     string    // table name under test_schema (also drives the report key)
	preDDL    []string  // optional CREATE TYPE etc. run before the table (on both source and target)
	colDef    string    // SQL type of the uk column
	indexDDL  string    // optional custom unique index; if set, no inline UNIQUE(uk) is emitted
	spellings [2]string // two equal-value spellings (pass the same string twice for a single spelling)
}

// tableDDL returns the DDL (preDDL + CREATE TABLE + optional custom index) for the entry.
func (e sbEntry) tableDDL() []string {
	out := append([]string{}, e.preDDL...)
	uniqueClause := ", UNIQUE (uk)"
	if e.indexDDL != "" {
		uniqueClause = ""
	}
	out = append(out, fmt.Sprintf(`CREATE TABLE test_schema.%s (id int PRIMARY KEY, uk %s, payload text%s);`, e.table, e.colDef, uniqueClause))
	if e.indexDDL != "" {
		out = append(out, e.indexDDL)
	}
	return out
}

// TestFNSafeBatchCanonicalTypes drives ONE migration with many tables, each probing a type whose
// btree equality is predicted to equal byte-equality of the emitted Debezium string (so detection
// is sound). Each entry is dynamically probed on the target first, then the source; unreachable
// entries are logged and excluded. Surviving entries run a short free/reclaim workload; a single
// verdict loop then requires that NO predicted-safe type raced to a duplicate-key error. On a
// 23505 the constraint name in the import output tail attributes the offending type. A support/
// verdict MATRIX is logged at the end.
func TestFNSafeBatchCanonicalTypes(t *testing.T) {
	dbName := "fn_safebatch"
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:    ContainerConfig{Type: "postgresql", ForLive: true, DatabaseName: dbName},
		TargetDB:    ContainerConfig{Type: "yugabytedb", DatabaseName: dbName},
		SchemaNames: []string{"test_schema"},
		SchemaSQL:   []string{`CREATE SCHEMA IF NOT EXISTS test_schema;`},
		CleanupSQL:  []string{`DROP SCHEMA IF EXISTS test_schema CASCADE;`},
	})
	defer lm.Cleanup()

	if err := lm.SetupContainers(context.Background()); err != nil {
		t.Fatalf("failed to setup containers: %v", err)
	}
	if err := lm.SetupSchema(); err != nil {
		t.Fatalf("failed to setup schema: %v", err)
	}

	entries := []sbEntry{
		{table: "sb_money", colDef: "money", spellings: [2]string{`'1.00'::money`, `'$1.00'::money`}},
		{table: "sb_bpchar", colDef: "char(4)", spellings: [2]string{`'ab'`, `'ab  '`}},
		{table: "sb_char", colDef: `"char"`, spellings: [2]string{`'x'`, `'x'`}},
		{table: "sb_bytea", colDef: "bytea", spellings: [2]string{`'\x616263'::bytea`, `'abc'::bytea`}},
		{table: "sb_bool", colDef: "boolean", spellings: [2]string{`true`, `'1'::boolean`}},
		{table: "sb_uuid", colDef: "uuid", spellings: [2]string{`'A0EEBC99-9C0B-4EF8-BB6D-6BB9BD380A11'::uuid`, `'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11'::uuid`}},
		{table: "sb_inet", colDef: "inet", spellings: [2]string{`'::1'::inet`, `'0:0:0:0:0:0:0:1'::inet`}},
		{table: "sb_macaddr", colDef: "macaddr", spellings: [2]string{`'08:00:2b:01:02:03'::macaddr`, `'08002b010203'::macaddr`}},
		{table: "sb_varbit", colDef: "bit varying", spellings: [2]string{`B'101'`, `B'101'`}},
		{table: "sb_enum", preDDL: []string{`CREATE TYPE test_schema.mood AS ENUM ('sad','ok','happy');`}, colDef: "test_schema.mood", spellings: [2]string{`'ok'`, `'ok'`}},
		{table: "sb_timestamptz", colDef: "timestamptz", spellings: [2]string{`'2024-06-01 10:00:00+02'::timestamptz`, `'2024-06-01 08:00:00+00'::timestamptz`}},
		// NOTE: 'infinity'::date was originally used here and uncovered a standalone data-shipping
		// bug: Debezium encodes dates as int32 days-since-1970 and PG's date-infinity (INT32_MAX
		// days-since-2000) overflows the conversion (+10957) to -2147472692, which the import then
		// renders as an unparseable literal -> non-retryable apply error -> migration aborts.
		// Tracked as a separate finding; a normal date keeps this batch about conflict detection.
		{table: "sb_date", colDef: "date", spellings: [2]string{`'2024-06-01'::date`, `'2024-06-01'::date`}},
		{table: "sb_int4range", colDef: "int4range", spellings: [2]string{`'[1,3)'::int4range`, `'[1,2]'::int4range`}},
		{table: "sb_timetz", colDef: "timetz", spellings: [2]string{`'10:00:00+02'::timetz`, `'10:00:00+02'::timetz`}},
		{table: "sb_tsvector", colDef: "tsvector", spellings: [2]string{`'b a'::tsvector`, `'a b'::tsvector`}},
		{table: "sb_tsquery", colDef: "tsquery", spellings: [2]string{`'a & b'::tsquery`, `'a&b'::tsquery`}},
		{table: "sb_textpat", colDef: "text", indexDDL: `CREATE UNIQUE INDEX sb_textpat_uk_idx ON test_schema.sb_textpat (uk text_pattern_ops);`, spellings: [2]string{`'abc'`, `'abc'`}},
	}

	type includedEntry struct {
		idx       int
		e         sbEntry
		reportKey string
	}
	var included []includedEntry
	status := make([]string, len(entries))

	for i, e := range entries {
		ddl := e.tableDDL()
		if stmt, err := execStmtsOn(lm.WithTargetConn, ddl); err != nil {
			status[i] = "SKIPPED (YB: " + firstLine(err.Error()) + ")"
			t.Logf("SKIP table %s: unreachable on YB (stmt %q): %v", e.table, stmt, err)
			continue
		}
		srcDDL := append(e.tableDDL(), fmt.Sprintf(`ALTER TABLE test_schema.%s REPLICA IDENTITY FULL;`, e.table))
		if stmt, err := execStmtsOn(lm.WithSourceConn, srcDDL); err != nil {
			status[i] = "SKIPPED (PG: " + firstLine(err.Error()) + ")"
			t.Logf("SKIP table %s: unreachable on source PG (stmt %q): %v", e.table, stmt, err)
			continue
		}
		seed := fmt.Sprintf(`INSERT INTO test_schema.%s (id, uk, payload) VALUES (0, %s, 'seed');`, e.table, e.spellings[0])
		if stmt, err := execStmtsOn(lm.WithSourceConn, []string{seed}); err != nil {
			status[i] = "SKIPPED (seed failed: " + firstLine(err.Error()) + ")"
			t.Logf("SKIP table %s: seed failed (stmt %q): %v", e.table, stmt, err)
			continue
		}
		included = append(included, includedEntry{idx: i, e: e, reportKey: fmt.Sprintf(`"test_schema"."%s"`, e.table)})
	}

	if len(included) == 0 {
		t.Skip("no safe-batch types were creatable on both source and target")
	}

	if err := lm.StartExportData(true, nil); err != nil {
		t.Fatalf("failed to start export data: %v", err)
	}
	if err := lm.StartImportData(true, nil); err != nil {
		t.Fatalf("failed to start import data: %v", err)
	}

	snapshot := map[string]int64{}
	for _, ie := range included {
		snapshot[ie.reportKey] = 1
	}
	if err := lm.WaitForSnapshotComplete(snapshot, 120); err != nil {
		t.Fatalf("snapshot did not complete: %v\nimport output tail:\n%s", err,
			tail(lm.GetImportCommandStderr()+lm.GetImportCommandStdout(), 25))
	}

	const iters = 12
	var delta []string
	expected := map[string]ChangesCount{}
	for _, ie := range included {
		delta = append(delta, buildFreeReclaimDeltaStmts("test_schema."+ie.e.table, iters, insertCols, makeSpellingValueFn(ie.e.spellings))...)
		expected[ie.reportKey] = freeReclaimChanges(iters)
	}
	if stmt, err := execStmtsOn(lm.WithSourceConn, delta); err != nil {
		t.Fatalf("delta failed (test bug) on %q: %v", stmt, err)
	}

	// Verdict loop (copied from runFNCase): duplicate-key error -> false negative; all expected
	// events applied -> survived; timeout with import still running -> stall.
	timeout := 300 * time.Second
	start := time.Now()
	survived := false
	for !survived {
		if lm.GetImportRunner() != nil && lm.GetImportRunner().IsStopped() {
			out := lm.GetImportCommandStderr() + "\n" + lm.GetImportCommandStdout()
			if containsUniqueViolation(out) {
				t.Fatalf("FALSE NEGATIVE in safe-batch — a predicted-safe type raced to a duplicate-key error.\n"+
					"the failing constraint in the output tail attributes the offending type.\n"+
					"---- import-data output (tail) ----\n%s", tail(out, 30))
			}
			t.Fatalf("import-data stopped unexpectedly (not a unique violation):\n%s", tail(out, 30))
		}
		done, err := lm.streamingPhaseCompleted(expected, "source", "target")
		if err == nil && done {
			survived = true
			break
		}
		if time.Since(start) > timeout {
			t.Fatalf("STALL: import still running but delta events not fully applied after %v.\nimport output tail:\n%s",
				timeout, tail(lm.GetImportCommandStderr()+lm.GetImportCommandStdout(), 25))
		}
		time.Sleep(3 * time.Second)
	}

	for _, ie := range included {
		status[ie.idx] = "INCLUDED — SURVIVED (no duplicate-key error)"
	}
	var b strings.Builder
	b.WriteString("SAFE-BATCH MATRIX (predicted-safe canonical types):\n")
	for i, e := range entries {
		b.WriteString(fmt.Sprintf("  %-16s : %s\n", e.table, status[i]))
	}
	t.Log(b.String())
}

// ---------------------------------------------------------------------------
// 10 (bespoke). Probe matrix: which "impossible/unsupported" DDL is creatable where?
// ---------------------------------------------------------------------------

// probeEntry is one unique-index DDL attempted on both the PG source and the YB target.
type probeEntry struct {
	name  string   // human-readable label for the matrix
	table string   // table name (dropped after each attempt)
	ddl   []string // statements to attempt (CREATE EXTENSION/TYPE/COLLATION + CREATE TABLE + index)
}

// TestFNProbeMatrixUnindexableTypes drives NO migration. For each "documented impossible /
// unsupported" unique-index construction it attempts the DDL on the PG source and on the YB
// target, records OK or the first line of the live error, drops whatever succeeded, and logs a
// support matrix. It documents rather than asserts (geometric types are expected to fail on PG
// because they have no btree opclass; xml/json have no equality operator; etc.). It fails only on
// unexpected infrastructure errors (container setup).
func TestFNProbeMatrixUnindexableTypes(t *testing.T) {
	dbName := "fn_probematrix"
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:    ContainerConfig{Type: "postgresql", ForLive: true, DatabaseName: dbName},
		TargetDB:    ContainerConfig{Type: "yugabytedb", DatabaseName: dbName},
		SchemaNames: []string{"test_schema"},
		SchemaSQL:   []string{`CREATE SCHEMA IF NOT EXISTS test_schema;`},
		CleanupSQL:  []string{`DROP SCHEMA IF EXISTS test_schema CASCADE;`},
	})
	defer lm.Cleanup()

	if err := lm.SetupContainers(context.Background()); err != nil {
		t.Fatalf("failed to setup containers: %v", err)
	}
	if err := lm.SetupSchema(); err != nil {
		t.Fatalf("failed to setup schema: %v", err)
	}

	// uniqueTable builds a plain "table with an inline UNIQUE(uk) of the given type" probe.
	uniqueTable := func(table, coltype string) []string {
		return []string{fmt.Sprintf(`CREATE TABLE test_schema.%s (id int PRIMARY KEY, uk %s, UNIQUE (uk));`, table, coltype)}
	}

	entries := []probeEntry{
		// Geometric types: no btree opclass -> unique index impossible in PG (demonstrated).
		{name: "point", table: "probe_1", ddl: uniqueTable("probe_1", "point")},
		{name: "box", table: "probe_2", ddl: uniqueTable("probe_2", "box")},
		{name: "line", table: "probe_3", ddl: uniqueTable("probe_3", "line")},
		{name: "lseg", table: "probe_4", ddl: uniqueTable("probe_4", "lseg")},
		{name: "path", table: "probe_5", ddl: uniqueTable("probe_5", "path")},
		{name: "polygon", table: "probe_6", ddl: uniqueTable("probe_6", "polygon")},
		{name: "circle", table: "probe_7", ddl: uniqueTable("probe_7", "circle")},
		// No equality operator / no btree.
		{name: "xml", table: "probe_8", ddl: uniqueTable("probe_8", "xml")},
		{name: "json", table: "probe_9", ddl: uniqueTable("probe_9", "json")},
		{name: "jsonpath", table: "probe_10", ddl: uniqueTable("probe_10", "jsonpath")},
		// Full-text and other exotic-but-maybe-indexable types.
		{name: "tsvector", table: "probe_11", ddl: uniqueTable("probe_11", "tsvector")},
		{name: "tsquery", table: "probe_12", ddl: uniqueTable("probe_12", "tsquery")},
		{name: "nummultirange", table: "probe_13", ddl: uniqueTable("probe_13", "nummultirange")},
		{name: "oid", table: "probe_14", ddl: uniqueTable("probe_14", "oid")},
		{name: "regclass", table: "probe_15", ddl: uniqueTable("probe_15", "regclass")},
		{name: "pg_lsn", table: "probe_16", ddl: uniqueTable("probe_16", "pg_lsn")},
		// Composite type column.
		{name: "composite type", table: "probe_17", ddl: []string{
			`CREATE TYPE test_schema.probe_ct AS (a int);`,
			`CREATE TABLE test_schema.probe_17 (id int PRIMARY KEY, uk test_schema.probe_ct, UNIQUE (uk));`,
		}},
		// Extension types.
		{name: "hstore", table: "probe_18", ddl: []string{
			`CREATE EXTENSION IF NOT EXISTS hstore;`,
			`CREATE TABLE test_schema.probe_18 (id int PRIMARY KEY, uk hstore, UNIQUE (uk));`,
		}},
		// Arrays, ranges, interval, jsonb.
		{name: "int[]", table: "probe_19", ddl: uniqueTable("probe_19", "int[]")},
		{name: "numeric[]", table: "probe_20", ddl: uniqueTable("probe_20", "numeric[]")},
		{name: "numrange", table: "probe_21", ddl: uniqueTable("probe_21", "numrange")},
		{name: "interval", table: "probe_22", ddl: uniqueTable("probe_22", "interval")},
		{name: "jsonb", table: "probe_23", ddl: uniqueTable("probe_23", "jsonb")},
		{name: "citext", table: "probe_24", ddl: []string{
			`CREATE EXTENSION IF NOT EXISTS citext;`,
			`CREATE TABLE test_schema.probe_24 (id int PRIMARY KEY, uk citext, UNIQUE (uk));`,
		}},
		// EXCLUDE constraint with = (not indisunique -> invisible to detection; YB rejects EXCLUDE).
		{name: "EXCLUDE (uk WITH =)", table: "probe_25", ddl: []string{
			`CREATE TABLE test_schema.probe_25 (id int PRIMARY KEY, uk int, EXCLUDE (uk WITH =));`,
		}},
		// Out-of-line deferrable unique constraint.
		{name: "deferrable unique (out-of-line)", table: "probe_26", ddl: []string{
			`CREATE TABLE test_schema.probe_26 (id int PRIMARY KEY, uk int);`,
			`ALTER TABLE test_schema.probe_26 ADD CONSTRAINT probe_26_uk_key UNIQUE (uk) DEFERRABLE INITIALLY DEFERRED;`,
		}},
		// Nondeterministic (case-insensitive) ICU collation unique index.
		{name: "nondeterministic collation", table: "probe_27", ddl: []string{
			`CREATE COLLATION test_schema.probe_ci (provider = icu, locale = 'und-u-ks-level2', deterministic = false);`,
			`CREATE TABLE test_schema.probe_27 (id int PRIMARY KEY, uk text);`,
			`CREATE UNIQUE INDEX probe_27_uk_idx ON test_schema.probe_27 (uk COLLATE test_schema.probe_ci);`,
		}},
	}

	// runProbe attempts ddl on one connection, always drops the probe table afterwards, and
	// returns "OK" or "error: <first line>". It never fails the test.
	runProbe := func(withConn func(func(*sql.DB) error) error, e probeEntry) string {
		_, err := execStmtsOn(withConn, e.ddl)
		_, _ = execStmtsOn(withConn, []string{fmt.Sprintf(`DROP TABLE IF EXISTS test_schema.%s CASCADE;`, e.table)})
		if err != nil {
			return "error: " + firstLine(err.Error())
		}
		return "OK"
	}

	var b strings.Builder
	b.WriteString("UNINDEXABLE-TYPE PROBE MATRIX (unique index attempted on each side):\n")
	for _, e := range entries {
		pg := runProbe(lm.WithSourceConn, e)
		yb := runProbe(lm.WithTargetConn, e)
		line := fmt.Sprintf("  %-32s | PG: %-45s | YB: %s", e.name, pg, yb)
		t.Log(line)
		b.WriteString(line + "\n")
	}
	t.Log(b.String())
}
