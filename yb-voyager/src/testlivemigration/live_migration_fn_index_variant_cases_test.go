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
Conflict-detection false-negative hunting — UNIQUE-INDEX GRAMMAR VARIATIONS (Part 2 of
conflict-detection-fn-test-plan.md).

Each case drives one real PostgreSQL -> Debezium -> YugabyteDB live migration through the
shared runFNCase driver (live_migration_fn_driver_test.go) and classifies the outcome:

  - SKIP  "premise does not hold"     — the equality premise is false in PG.
  - SKIP  "UNREACHABLE on YB target"  — the target rejected the schema (live error, not a guess).
  - FAIL  "FALSE NEGATIVE CONFIRMED"  — import-data died with a duplicate-key error (23505): a
                                        genuine conflict was missed and raced to the target.
  - FAIL  "STALL"                     — events never fully applied and import never exited.
  - PASS  survived                    — all delta events applied, no duplicate-key error.

Conventions (shared with the type-cases file):
  - schema test_schema; every table has PRIMARY KEY id int; unique column named uk.
  - sourceDDL carries the explicit `ALTER TABLE ... REPLICA IDENTITY FULL;` per table.
  - report keys are the quoted `"test_schema"."<table>"` form; seed row id=0.
  - FN-candidate cases run 200 free/reclaim iterations (buildFreeReclaimDeltaStmts) so the
    cross-worker race has ample opportunity to fire.
  - No t.Parallel(): the suite is run sequentially in isolated processes.
*/

import (
	"fmt"
	"testing"
)

// ---------------------------------------------------------------------------
// d. INCLUDE (covering) columns — PREDICTED FALSE NEGATIVE (from code).
// ---------------------------------------------------------------------------

// TestFNIncludeColumns hunts a false negative that is predicted directly from the discovery
// code, not guessed. The unique-index discovery query matches a column into the key with
// `a.attnum = ANY(ix.indkey)` and never slices indkey down to indnkeyatts. pg_index.indkey
// lists BOTH the key columns and the INCLUDE (covering) columns; only the first indnkeyatts
// of them are actually part of the unique key. Without the slice, an INCLUDE column is wrongly
// treated as part of the unique key.
//
// Here the genuine unique key is (uk) only; payload is an INCLUDE column. Every free/reclaim
// iteration reclaims uk=10 (a real conflict on the real key) but with a fresh payload
// ('p1','p2',...). If discovery believes the key is (uk, payload), the freed row
// (uk=10, payload=old) and the reclaiming row (uk=10, payload=new) hash to different conflict
// buckets, detection does NOT serialize them, and the reclaiming INSERT can race ahead of the
// freeing DELETE. The target's real (uk)-only unique index then raises 23505 -> FN CONFIRMED.
func TestFNIncludeColumns(t *testing.T) {
	runFNCase(t, fnCase{
		name: "include_columns",
		sourceDDL: []string{
			`CREATE TABLE test_schema.include_uk (id int PRIMARY KEY, uk int, payload text);`,
			`CREATE UNIQUE INDEX include_uk_idx ON test_schema.include_uk (uk) INCLUDE (payload);`,
			`ALTER TABLE test_schema.include_uk REPLICA IDENTITY FULL;`,
		},
		seedSQL: []string{`INSERT INTO test_schema.include_uk (id, uk, payload) VALUES (0, 10, 'seed');`},
		// uk is ALWAYS 10 (constant -> genuine conflict every iteration); payload varies as
		// 'p<i>' inside the helper (the differing INCLUDE value that the buggy discovery keys on).
		deltaSQL: buildFreeReclaimDeltaStmts("test_schema.include_uk", 200, "id, uk, payload",
			func(i int) string { return "10" }),
		expectedSnapshotRows: map[string]int64{`"test_schema"."include_uk"`: 1},
		expectedChanges:      map[string]ChangesCount{`"test_schema"."include_uk"`: freeReclaimChanges(200)},
	})
}

// ---------------------------------------------------------------------------
// b. Expression unique index — verify the PARTITION_BY_TABLE mitigation END TO END.
// ---------------------------------------------------------------------------

// TestFNExpressionIndexMitigation verifies the existing mitigation empirically rather than
// trusting it. A table whose unique index is an EXPRESSION index (here lower(uk)) must be
// auto-forced to PARTITION_BY_TABLE routing, so all of its events go to a single worker in
// commit order and a cross-worker conflict is structurally impossible.
//
// The workload alternates 'alice' / 'ALICE': lower() makes them equal every iteration, so this
// is a genuine conflict on lower(uk) on EVERY free/reclaim. With by-table routing the freeing
// DELETE always applies before the reclaiming INSERT, so the migration must SURVIVE.
//
// If instead it dies with a duplicate-key error, the expression-index PARTITION_BY_TABLE
// mitigation is BROKEN on main: the table was routed partition-by-pk and the case raced to a
// 23505. (The shared driver's failure message reports it as a confirmed false negative; for an
// expression-index table that specifically means the by-table routing failed to engage.)
func TestFNExpressionIndexMitigation(t *testing.T) {
	runFNCase(t, fnCase{
		name: "expression_mitigation",
		sourceDDL: []string{
			`CREATE TABLE test_schema.expr_uk (id int PRIMARY KEY, uk text, payload text);`,
			`CREATE UNIQUE INDEX expr_uk_idx ON test_schema.expr_uk (lower(uk));`,
			`ALTER TABLE test_schema.expr_uk REPLICA IDENTITY FULL;`,
		},
		seedSQL: []string{`INSERT INTO test_schema.expr_uk (id, uk, payload) VALUES (0, 'Alice', 'seed');`},
		// lower('alice') == lower('ALICE') every iteration -> genuine conflict every time.
		deltaSQL: buildFreeReclaimDeltaStmts("test_schema.expr_uk", 200, "id, uk, payload",
			func(i int) string {
				if i%2 == 0 {
					return "'alice'"
				}
				return "'ALICE'"
			}),
		expectedSnapshotRows: map[string]int64{`"test_schema"."expr_uk"`: 1},
		expectedChanges:      map[string]ChangesCount{`"test_schema"."expr_uk"`: freeReclaimChanges(200)},
	})
}

// ---------------------------------------------------------------------------
// n. Target-only unique index (schema drift) — PREDICTED SAFE (target-based discovery).
// ---------------------------------------------------------------------------

// TestFNTargetOnlyUniqueIndex exercises schema drift: the unique index exists ONLY on the YB
// target, not on the PG source. Current main reads unique indexes from the TARGET during
// initializeConflictDetectionCache (tdb.GetTableToUniqueIndexesMap), so detection should still
// know the (uk) index and serialize the free/reclaim of uk=10 -> the migration SURVIVES.
//
// The source has NO unique index, so its free/reclaim of uk=10 across new PKs is perfectly
// valid SQL and never conflicts on the source side.
//
// PREDICTION: survives (detection is target-based). If it instead dies with 23505, discovery is
// source-based, not target-based — which would falsify the "unique indexes are read from the
// target" claim this test validates.
func TestFNTargetOnlyUniqueIndex(t *testing.T) {
	runFNCase(t, fnCase{
		name: "target_only_index",
		// Source: table + RIF, but NO unique index.
		sourceDDL: []string{
			`CREATE TABLE test_schema.target_only_uk (id int PRIMARY KEY, uk int, payload text);`,
			`ALTER TABLE test_schema.target_only_uk REPLICA IDENTITY FULL;`,
		},
		// Target: same table PLUS the unique index that only the target has.
		targetDDL: []string{
			`CREATE TABLE test_schema.target_only_uk (id int PRIMARY KEY, uk int, payload text);`,
			`CREATE UNIQUE INDEX target_only_uk_idx ON test_schema.target_only_uk (uk);`,
		},
		seedSQL: []string{`INSERT INTO test_schema.target_only_uk (id, uk, payload) VALUES (0, 10, 'seed');`},
		deltaSQL: buildFreeReclaimDeltaStmts("test_schema.target_only_uk", 200, "id, uk, payload",
			func(i int) string { return "10" }),
		expectedSnapshotRows: map[string]int64{`"test_schema"."target_only_uk"`: 1},
		expectedChanges:      map[string]ChangesCount{`"test_schema"."target_only_uk"`: freeReclaimChanges(200)},
	})
}

// ---------------------------------------------------------------------------
// o. Mid-stream CREATE UNIQUE INDEX — PREDICTED FALSE NEGATIVE.
// ---------------------------------------------------------------------------

// TestFNMidStreamCreatedIndex documents the failure mode when a unique index is created on the
// target AFTER streaming has started. The conflict-detection cache is initialized exactly once
// when the streaming phase begins and is never refreshed, so an index created afterwards is
// invisible to detection.
//
// NOTE: modifying the schema during an in-flight live migration is documented-UNSUPPORTED. This
// test therefore documents the failure mode rather than asserting a supported path.
//
// Neither sourceDDL nor targetDDL creates the unique index; afterSnapshotTargetSQL creates it on
// the target only after the driver has observed the streaming phase (and the cache is built).
// The source never has a unique index, so its free/reclaim of uk=10 is valid SQL throughout.
//
// PREDICTION: FALSE NEGATIVE (23505) — the reclaiming INSERT of uk=10 races ahead of the freeing
// DELETE and hits the freshly-created, cache-invisible target index.
func TestFNMidStreamCreatedIndex(t *testing.T) {
	runFNCase(t, fnCase{
		name: "midstream_index",
		sourceDDL: []string{
			`CREATE TABLE test_schema.midstream_uk (id int PRIMARY KEY, uk int, payload text);`,
			`ALTER TABLE test_schema.midstream_uk REPLICA IDENTITY FULL;`,
		},
		// Target starts WITHOUT the unique index; it is added mid-stream below.
		targetDDL: []string{
			`CREATE TABLE test_schema.midstream_uk (id int PRIMARY KEY, uk int, payload text);`,
		},
		seedSQL: []string{`INSERT INTO test_schema.midstream_uk (id, uk, payload) VALUES (0, 10, 'seed');`},
		// Created on the target only after streaming has started (cache already initialized).
		afterSnapshotTargetSQL: []string{`CREATE UNIQUE INDEX midstream_uk_idx ON test_schema.midstream_uk (uk);`},
		deltaSQL: buildFreeReclaimDeltaStmts("test_schema.midstream_uk", 200, "id, uk, payload",
			func(i int) string { return "10" }),
		expectedSnapshotRows: map[string]int64{`"test_schema"."midstream_uk"`: 1},
		expectedChanges:      map[string]ChangesCount{`"test_schema"."midstream_uk"`: freeReclaimChanges(200)},
	})
}

// ---------------------------------------------------------------------------
// e. NULLS NOT DISTINCT (PG15) — PREDICTED SAFE (detection handles the flag).
// ---------------------------------------------------------------------------

// TestFNNullsNotDistinct exercises a NULLS NOT DISTINCT unique index (PG15+). Under this flag
// NULL conflicts with NULL, so a free/reclaim where the unique value is ALWAYS NULL is a genuine
// conflict on every iteration. Detection is expected to honour the flag (its bothNil branch
// treats two NULLs as a conflict) and serialize the pair -> the migration SURVIVES.
//
// If YB rejects a NULLS NOT DISTINCT index the dynamic target probe skips the case with the live
// error (itself a finding). The source seed is written with an explicit NULL.
//
// PREDICTION: survives (bothNil -> conflict). A 23505 here would mean detection ignores the
// NULLS NOT DISTINCT semantics and lets two NULL rows race.
func TestFNNullsNotDistinct(t *testing.T) {
	runFNCase(t, fnCase{
		name: "nulls_not_distinct",
		sourceDDL: []string{
			`CREATE TABLE test_schema.nnd_uk (id int PRIMARY KEY, uk int, payload text);`,
			`CREATE UNIQUE INDEX nnd_uk_idx ON test_schema.nnd_uk (uk) NULLS NOT DISTINCT;`,
			`ALTER TABLE test_schema.nnd_uk REPLICA IDENTITY FULL;`,
		},
		// Seed with an explicit NULL unique value.
		seedSQL: []string{`INSERT INTO test_schema.nnd_uk (id, uk, payload) VALUES (0, NULL, 'seed');`},
		// uk is ALWAYS NULL -> under NULLS NOT DISTINCT this is a genuine conflict every iteration.
		deltaSQL: buildFreeReclaimDeltaStmts("test_schema.nnd_uk", 200, "id, uk, payload",
			func(i int) string { return "NULL" }),
		expectedSnapshotRows: map[string]int64{`"test_schema"."nnd_uk"`: 1},
		expectedChanges:      map[string]ChangesCount{`"test_schema"."nnd_uk"`: freeReclaimChanges(200)},
	})
}

// ---------------------------------------------------------------------------
// h. Nondeterministic collation unique index — PREDICTED FALSE NEGATIVE (if creatable).
// ---------------------------------------------------------------------------

// TestFNNondeterministicCollation exercises a unique index on a text column with a
// nondeterministic ICU collation (case-insensitive at level 2). Premise (asserted here in prose
// because it cannot be preflighted — the collation does not exist until the DDL below runs):
// under provider=icu, locale='und-u-ks-level2', deterministic=false, 'Alice' = 'alice' = 'ALICE'.
//
// A plain (non-expression) unique index on a collated column is NOT an expression index, so the
// PARTITION_BY_TABLE mitigation does NOT engage; the table is routed partition-by-pk and events
// on the same collated value can land on different workers. Detection compares the raw Debezium
// strings with a byte ==: 'alice' and 'ALICE' are byte-distinct but index-equal, so the conflict
// is missed and the reclaiming INSERT can race ahead -> 23505.
//
// If YB rejects CREATE COLLATION (deterministic=false) or the collated unique index, the dynamic
// probe skips the case with the live error (a finding). preflightSQL is deliberately nil.
//
// PREDICTION: FALSE NEGATIVE if the schema is creatable on YB (byte comparison in detection vs a
// case-insensitive btree index).
func TestFNNondeterministicCollation(t *testing.T) {
	runFNCase(t, fnCase{
		name: "nondeterministic_collation",
		sourceDDL: []string{
			`CREATE COLLATION test_schema.ci (provider = icu, locale = 'und-u-ks-level2', deterministic = false);`,
			`CREATE TABLE test_schema.ndcoll_uk (id int PRIMARY KEY, uk text COLLATE test_schema.ci, payload text, UNIQUE (uk));`,
			`ALTER TABLE test_schema.ndcoll_uk REPLICA IDENTITY FULL;`,
		},
		seedSQL: []string{`INSERT INTO test_schema.ndcoll_uk (id, uk, payload) VALUES (0, 'Alice', 'seed');`},
		// 'alice' == 'ALICE' under the level-2 ICU collation every iteration -> genuine conflict.
		deltaSQL: buildFreeReclaimDeltaStmts("test_schema.ndcoll_uk", 200, "id, uk, payload",
			func(i int) string {
				if i%2 == 0 {
					return "'alice'"
				}
				return "'ALICE'"
			}),
		expectedSnapshotRows: map[string]int64{`"test_schema"."ndcoll_uk"`: 1},
		expectedChanges:      map[string]ChangesCount{`"test_schema"."ndcoll_uk"`: freeReclaimChanges(200)},
	})
}

// ---------------------------------------------------------------------------
// p. PK-update recycling — PREDICTED SAFE (verify Debezium delete+create ordering).
// ---------------------------------------------------------------------------

// TestFNPkUpdateRecycle verifies that per-row and unique-key ordering hold when a unique value is
// recycled through primary-key churn. Debezium emits a primary-key UPDATE as a DELETE(old pk) +
// INSERT(new pk) pair (or, depending on config, as an UPDATE), so the emitted event shape and
// counts are uncertain — hence expectedChanges is nil and the driver observes for a fixed window.
//
// Phase 1: 100 successive PK updates walk the single row's PK 0 -> 100 while uk stays 10 the whole
// time (no free/reclaim yet). Phase 2: 50 free/reclaim iterations of uk=10 with fresh PKs that
// continue the same PK sequence (100 -> 150).
//
// PREDICTION: survives — the reclaiming INSERT should conflict with the cached delete before-image
// and be serialized. If a 23505 appears, PK-update handling has an ordering hole (an INSERT
// reordered ahead of the DELETE that freed its PK / unique value).
func TestFNPkUpdateRecycle(t *testing.T) {
	var delta []string
	// Phase 1: 100 PK updates; the row keeps uk=10 and its PK walks 0 -> 100.
	for i := 1; i <= 100; i++ {
		delta = append(delta, fmt.Sprintf(`UPDATE test_schema.pkupd_uk SET id = id + 1 WHERE id = %d;`, i-1))
	}
	// Phase 2: 50 free/reclaim of uk=10 with fresh PKs continuing the sequence (101 -> 150).
	for j := 0; j < 50; j++ {
		cur := 100 + j // 100, 101, ..., 149
		delta = append(delta, fmt.Sprintf(`DELETE FROM test_schema.pkupd_uk WHERE id = %d;`, cur))
		delta = append(delta, fmt.Sprintf(`INSERT INTO test_schema.pkupd_uk (id, uk, payload) VALUES (%d, 10, 'p');`, cur+1))
	}

	runFNCase(t, fnCase{
		name: "pk_update_recycle",
		sourceDDL: []string{
			`CREATE TABLE test_schema.pkupd_uk (id int PRIMARY KEY, uk int, payload text, UNIQUE (uk));`,
			`ALTER TABLE test_schema.pkupd_uk REPLICA IDENTITY FULL;`,
		},
		seedSQL:              []string{`INSERT INTO test_schema.pkupd_uk (id, uk, payload) VALUES (0, 10, 'seed');`},
		deltaSQL:             delta,
		expectedSnapshotRows: map[string]int64{`"test_schema"."pkupd_uk"`: 1},
		// PK updates may be emitted as UPDATE or as DELETE+INSERT pairs -> counts uncertain;
		// observe for a fixed window instead of waiting for exact event counts.
		expectedChanges:    nil,
		observationSeconds: 180,
	})
}
