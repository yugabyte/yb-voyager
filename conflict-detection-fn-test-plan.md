# Conflict-Detection False-Negative Test Plan — every PG type × every unique-index variation

Goal: a comprehensive, **empirical** suite of live-migration tests hunting false negatives
(missed conflicts → duplicate-key 23505 abort, or silent corruption) in Voyager's CDC
unique-key conflict detection. Nothing is disregarded on documentation or guesswork: every
"unsupported" claim is demonstrated by attempting the DDL at runtime (dynamic probe → skip
with the live error), and every equality premise is proven with a preflight SQL check before
the hunt.

**Detection model under test** (current `main`): raw Go string `==` of unconverted Debezium
payload values, per unique index discovered from the **target** via `pg_index`
(`a.attnum = ANY(ix.indkey)` — note: no `indnkeyatts` slicing). An FN exists whenever two
values are **equal to the target's btree index** but arrive as **different strings** — or when
detection never sees the value / the index at all (structural).

Empirical results already in hand (previous session):
- numeric scale `1.0` vs `1.00` → **SAFE** (exporter canonicalizes both to `"1"`) — test exists.
- inline `UNIQUE DEFERRABLE` swap → **FN CONFIRMED** (23505) — test exists.
- `GENERATED ALWAYS … STORED` unique col → **FN CONFIRMED** (23505) — test exists.
- jsonb / interval / citext unique index → **YB rejects the index** (0A000) — being converted to dynamic probes.

⚠️ The installed binary was rebuilt from current `origin/main` before any run below
(conflictDetectionCache.go changed ~600 lines since the previous binary).

---

## Part 1 — Every documented PG type (docs ch. 8 + voyager-relevant extensions)

Verdict key: **FN?** = predicted false negative; **SAFE** = predicted caught/impossible;
**PROBE** = expected not-creatable (demonstrated at runtime); **COVERED** = existing tests.

| # | Type | Equality nuance vs emitted string | Prediction | Action |
|---|---|---|---|---|
| 1 | smallint/int/bigint/serial | canonical text | SAFE | COVERED (all existing conflict tests use int keys) |
| 2 | numeric | scale `1.0`=`1.00`; `NaN`=`NaN`; `±Infinity` (PG14+) | scale: **verified SAFE**; NaN/Inf canonical | scale test exists; NaN+Inf → safe-batch |
| 3 | real/double precision | **`-0.0` = `0.0`** in btree; Java prints `-0.0` vs `0.0`. NaN canonical. | **FN?** | `fn_float_negzero` (float8 + float4 tables) w/ preflight `-0.0=0.0` |
| 4 | money | int64 cents, canonical scale-2 | SAFE | safe-batch |
| 5 | text/varchar (deterministic collation) | byte equality ⟺ index equality (incl. NFC/NFD) | SAFE | COVERED |
| 6 | char(n)/bpchar | trailing-space padding canonicalized at storage | SAFE | safe-batch (`'a'` vs `'a  '` inputs) |
| 7 | "char" | single byte | SAFE | safe-batch |
| 8 | citext (ext) | case-insensitive index vs byte compare | FN if creatable; YB rejected before | dynamic probe test (refactor skip) |
| 9 | bytea | canonical hex emission; escape-format input | SAFE | safe-batch |
| 10 | timestamp/timestamptz/date/time | epoch emission; input forms normalized at parse; `infinity` dates | SAFE | safe-batch (tz-offset input forms; date `infinity`) |
| 11 | timetz | equality = same UTC instant **and** same zone (zone is tiebreaker) → equal values byte-identical | SAFE for FN (FP possible if UTC-normalized) | safe-batch |
| 12 | interval | `1 day`=`24 hours` index-equal, distinct text | FN if creatable; YB rejected before | dynamic probe test (refactor skip) |
| 13 | boolean | canonical t/f | SAFE | safe-batch |
| 14 | enum | canonical labels | SAFE | safe-batch |
| 15 | geometric (point/line/lseg/box/path/polygon/circle) | no btree opclass → unique index impossible **in PG** (box `=` is area-equality, gist-only) | PROBE | probe-matrix (expect source-side failure) |
| 16 | inet/cidr | canonical output (`::1` vs long form; default /32) | SAFE | safe-batch (IPv6 abbreviation) |
| 17 | macaddr/macaddr8 | 3+ input formats → canonical | SAFE | safe-batch |
| 18 | bit/bit varying | canonical | SAFE | safe-batch |
| 19 | tsvector | lexemes sorted+dedup'd at parse → canonical storage; PG btree opclass exists | SAFE if creatable; YB support unknown | safe-batch w/ per-table probe |
| 20 | tsquery | whitespace canonicalized, structure preserved | SAFE if creatable | safe-batch w/ per-table probe |
| 21 | uuid | canonical lowercase (uppercase/braces inputs) | SAFE | safe-batch |
| 22 | xml | **no `=` operator** → unique index impossible in PG | PROBE | probe-matrix |
| 23 | json | **no `=` operator** | PROBE | probe-matrix |
| 24 | jsonb | numeric scale preserved inside text (`{"v":1.0}`=`{"v":1.00}`) | FN if creatable; YB rejected before | dynamic probe test (refactor skip) |
| 25 | jsonpath | no btree | PROBE | probe-matrix |
| 26 | arrays — int[] | **lower bounds**: `'[0:1]={1,2}'` vs `'{1,2}'` — equal per array_cmp? emission format? | FN? (uncertain ×2) | `fn_array_lower_bounds` w/ preflight equality check |
| 27 | arrays — numeric[] | element scale `{1.0}` vs `{1.00}`; exporter's Decimal canonicalization may not recurse into ARRAY | **FN?** | `fn_array_numeric_scale` w/ preflight |
| 28 | composite type column | field-wise equality; `(1.0)` vs `(1.00)` equal; Debezium `include.unknown.datatypes` emission | FN if creatable (YB UDT-index likely rejected) | `fn_composite_numeric` dynamic |
| 29 | int4range/daterange | **canonicalized on input** (`[1,3)`=`[1,2]` → same storage) | SAFE | safe-batch |
| 30 | numrange | bounds NOT canonicalized: `[1.0,2.0]`=`[1.00,2.00]` equal, text distinct | **FN?** if creatable | `fn_numrange_scale` dynamic w/ preflight |
| 31 | multirange (PG14+) | as ranges | PROBE | probe-matrix |
| 32 | domain over numeric | does numeric canonicalization apply to domains? | FN? (uncertain) | `fn_domain_numeric_scale` |
| 33 | hstore (ext) | storage canonical (sorted keys) but `hstore.handling.mode=map` → Java Map serialization order may vary | FN? if creatable (YB UDT-index likely rejected) | `fn_hstore_key_order` dynamic |
| 34 | oid/regclass/regtype/pg_lsn | canonical single representation; exotic in unique keys | SAFE/PROBE | probe-matrix only |
| 35 | TOAST mechanics (any long type) | 8KB text unique value: is the DELETE before-image detoasted (RIF flattening) or a placeholder? | SAFE predicted (heapam flattens RIF old tuples) — verify | `fn_toast_large_text` |

## Part 2 — Unique-index grammar variations (CREATE INDEX / constraint syntax)

| # | Variation | Mechanism | Prediction | Action |
|---|---|---|---|---|
| a | UNIQUE constraint (inline/out-of-line), CREATE UNIQUE INDEX | plain | SAFE | COVERED |
| b | Expression index — `lower(col)`, `((col))` parenthesized, mixed col+expr | forced PARTITION_BY_TABLE (main has it) | SAFE — **verify the mitigation empirically** | `fn_expression_mitigation` (case-swap workload; expect survive) |
| c | Partial (WHERE) | predicate ignored → over-detection (FP), no FN vector | SAFE | COVERED (failpoint suite) |
| d | **INCLUDE (covering)** | discovery query uses `ANY(ix.indkey)` without `indnkeyatts` slice → INCLUDE cols treated as key cols → conflict on real key w/ differing INCLUDE value missed | **FN — predicted from code** | `fn_include_columns` |
| e | NULLS NOT DISTINCT (PG15) | detection handles flag (bothNil→conflict) | SAFE if creatable on YB | `fn_nulls_not_distinct` |
| f | DEFERRABLE inline | YB silently downgrades (#32212); swap unreppresentable row-at-a-time | **FN CONFIRMED** | exists |
| g | DEFERRABLE out-of-line | YB rejects at CREATE | PROBE (constraint absent → unenforced) | probe-matrix |
| h | Nondeterministic collation (ICU, `deterministic=false`) unique index | case-insensitive equality vs byte compare | FN if creatable on YB (CREATE COLLATION support unknown) | `fn_nondeterministic_collation` dynamic |
| i | Opclass variants (text_pattern_ops) | `=` operator unchanged | SAFE | safe-batch |
| j | EXCLUDE (col WITH =) | not `indisunique` → invisible to detection; YB rejects EXCLUDE | PROBE (FN hazard only for PG-target fallback) | probe-matrix |
| k | ASC/DESC, NULLS FIRST/LAST, fillfactor, tablespace, CONCURRENTLY, USING INDEX | no equality impact | SAFE | note only |
| l | Unique index on partitioned tables | leaf→root merge | — | COVERED (existing partition tests) |
| m | Unique index on GENERATED STORED column | column absent from CDC | **FN CONFIRMED** | exists |
| n | **Target-only unique index** (schema drift) | discovery reads target on main | SAFE predicted — verifies stale doc claim | `fn_target_only_index` |
| o | **Mid-stream CREATE UNIQUE INDEX** (after import starts) | cache built once at streaming init, never refreshed | **FN — predicted** (op is documented-unsupported; verify anyway) | `fn_midstream_index` |
| p | PK-update recycling (Debezium delete+create pair) | insert should conflict with cached delete before-image | SAFE predicted — verify | `fn_pk_update_recycle` |
| q | Quoted/case-sensitive columns; multiple unique indexes per table | — | SAFE | COVERED |

## Part 3 — Test architecture

All tests: `//go:build integration_live_migration`, package `testlivemigration`, real
PG→Debezium→YB migration via `LiveMigrationTest`. **Run isolated/sequential** (no
`t.Parallel()`): the container helper `ExecuteSqlsOnDB` calls `os.Exit` on failure and the
report poller is racy under parallel load.

**Shared driver** `runFNCase(t, fnCase)` in `live_migration_fn_type_cases_test.go`:

1. `TestConfig` with `SchemaSQL: CREATE SCHEMA only`; unique DB name `fn_<name>`.
2. **Preflight** (optional SQL on source via `WithSourceConn`): proves the equality premise,
   e.g. `DO $$ ... IF NOT ('-0.0'::float8 = '0.0'::float8) THEN RAISE ... $$`. Failure ⇒
   `t.Skipf("premise does not hold: ...")`.
3. **Dynamic probes**: target DDL first via `WithTargetConn` + `db.Exec` (graceful errors) —
   failure ⇒ `t.Skipf("UNREACHABLE on YB: %v", err)` with the live error. Then source DDL via
   `WithSourceConn` — failure ⇒ skip "unreachable on source PG". RIF statements included in
   source DDL explicitly. NEVER `ExecuteSqlsOnDB` for fallible DDL (os.Exit).
4. Seed → export/import async → `WaitForSnapshotComplete` (report keys are the
   `"test_schema"."tbl"` quoted form).
5. Optional `afterSnapshotTargetSQL` hook (mid-stream index test).
6. Delta via `WithSourceConn` Exec loop (~200 free/reclaim iterations for race-based cases).
7. **Verdict loop** (up to 240s): import stopped + output contains 23505/unique-violation ⇒
   `t.Fatalf("FALSE NEGATIVE CONFIRMED …")`; stopped otherwise ⇒ fatal (unexpected);
   `streamingPhaseCompleted(expectedChanges)` true ⇒ survived (fast exit); timeout with
   import running ⇒ fatal "stall — possible conflict-wait deadlock". `expectedChanges == nil`
   ⇒ fixed 120s observation window instead (for cases with uncertain event counts).

**Safe-batch test** (one migration, many tables): per-table dynamic target probe (skip +
log the table on failure), ~12 free/reclaim iterations per surviving table, one verdict
loop. On 23505 the constraint name in stderr attributes the offending type.

**Probe-matrix test** (no migration): attempts every "impossible/unsupported" DDL on both
the PG source container and YB target container and logs a support matrix
(`PG: OK|error — YB: OK|error`). Documents rather than asserts; never assumed from docs.

## Part 4 — Case inventory to implement

File `live_migration_fn_type_cases_test.go` (Agent A):
driver + `fn_float_negzero`, `fn_array_lower_bounds`, `fn_array_numeric_scale`,
`fn_numrange_scale`, `fn_domain_numeric_scale`, `fn_composite_numeric`, `fn_hstore_key_order`,
`fn_toast_large_text`, safe-batch, probe-matrix; refactor the 3 hardcoded skips
(jsonb/interval/citext) into dynamic driver cases.

File `live_migration_fn_index_variant_cases_test.go` (Agent B):
`fn_include_columns`, `fn_expression_mitigation`, `fn_target_only_index`,
`fn_midstream_index`, `fn_nulls_not_distinct`, `fn_nondeterministic_collation`,
`fn_pk_update_recycle`.

## Part 5 — Results (all runs on a binary built from origin/main @ 89ca37722, 2026-07-30)

### Confirmed FALSE NEGATIVES (real 23505 → import-data aborts)

| Case | Test | Evidence |
|---|---|---|
| **Covering index `UNIQUE (uk) INCLUDE (payload)`** — NEW this round, predicted from the discovery query | TestFNIncludeColumns | `duplicate key value violates unique constraint "include_uk_idx"` @83s. Root cause: `pgQueryTmplForUniqIndexes` uses `a.attnum = ANY(ix.indkey)` — INCLUDE columns land in the key tuple, so a conflict on uk with differing payload never matches. **Fix: filter `array_position(ix.indkey, a.attnum) < ix.indnkeyatts`.** |
| **GENERATED ALWAYS … STORED unique column** | TestLiveMigrationFalseNegativeGeneratedStoredUnique | Re-confirmed on new binary (`gen_uk_g_uidx`, 76s). Column absent from CDC stream → detection blind. |
| **Inline `UNIQUE DEFERRABLE` swap** | TestLiveMigrationFalseNegativeDeferredUniqueConstraint | Re-confirmed on new binary (`deferred_uk_uk_key`, 50s). YB silently downgrades inline DEFERRABLE (#32212); no row-at-a-time order is valid. |

### Verified SAFE (survived a genuine conflict-per-iteration workload, 200 iters)

| Case | Test | Note |
|---|---|---|
| numeric scale `1.0`/`1.00` | …NumericScale | exporter canonicalizes both to `"1"` (queue-verified) |
| float4/float8 `-0.0`/`0.0` | TestFNFloatNegativeZero | sign of zero canonicalized in emission |
| domain over numeric | TestFNDomainNumericScale | canonicalization applies through domains; YB indexes domains fine |
| TOAST ~8KB unique text, delete→reinsert | TestFNToastLargeText | RIF delete before-images are fully detoasted (heapam flattens) |
| PK-update recycling (delete+create pairs) | TestFNPkUpdateRecycle | 100 PK bumps + 50 free/reclaims, no error |
| expression index `lower(uk)` | TestFNExpressionIndexMitigation | mitigation verified: auto partition-by-table absorbs 200 real conflicts |
| target-only unique index | TestFNTargetOnlyUniqueIndex | discovery reads the TARGET on main → drift covered; design-doc claim stale |

### Premise disproven (no FN class exists)

| Case | Why |
|---|---|
| int[] lower bounds `'[0:1]={1,2}'` vs `'{1,2}'` | preflight proved they are NOT equal in PG — lower bounds are significant for array equality |

### Unreachable — empirically demonstrated, never assumed (live errors captured per run)

- **No btree opclass in PG itself** (unique index impossible even at the source): point, box, line, lseg, path, polygon, circle, xml, json, jsonpath.
- **PG allows, YB target rejects the index/constraint** (SQLSTATE 0A000/42704/42P16): tsvector, tsquery, nummultirange, composite type, hstore, int[], numeric[], numrange, interval, jsonb, citext, **inet, macaddr, bit varying, int4range, timetz** (from the batch probes), EXCLUDE (uk WITH =), out-of-line DEFERRABLE UNIQUE, nondeterministic collation (`CREATE COLLATION … deterministic = false` itself rejected).
- **Both OK, canonical representation** → safe: oid, regclass, pg_lsn.

### Inconclusive / caveats

| Case | Status |
|---|---|
| Mid-stream `CREATE UNIQUE INDEX` on target | SURVIVED, but with **zero** conflict-detection activity in the logs — the cache was demonstrably blind; the likely masker is YB's online index build overlapping the churn. A variant that waits for the index to be valid before the delta would sharpen this. The operation is documented-unsupported during live migration anyway. |

### Retry results (final)

| Case | Result |
|---|---|
| NULLS NOT DISTINCT | **SURVIVED** — YB accepts the index; detection treats NULL=NULL correctly under the flag through 200 genuine conflicts. (First run was a harness flake: two `get data-migration-report` processes raced on the report JSON.) |
| Safe-batch canonical types | **ALL INCLUDED TYPES SURVIVED**: money, char(n), "char", bytea, boolean, uuid, enum, **timestamptz** (offset-form inputs), date, text_pattern_ops opclass. Skipped by live YB probe: inet, macaddr, varbit, int4range, timetz, tsvector, tsquery (`INDEX on column of type '…' not yet supported`). timestamptz surviving confirms the first-run abort was purely the date-infinity bug. |

### Standalone finding beyond conflict detection — date `'infinity'` breaks CDC apply

Seeding `'infinity'::date` produced queue value `uk = -2147472692`: PG stores date-infinity as
INT32_MAX (days since 2000-01-01); Debezium's conversion to days-since-1970 adds 10957 and
**overflows int32** (2147483647 + 10957 − 2³² = −2147472692). The import renders an unparseable
literal → `ERROR: time zone displacement out of range` → non-retryable → **migration aborts**.
Snapshot is unaffected (pg_dump text path). Any table with infinity dates hitting the CDC path
kills live migration. Timestamp/timestamptz infinity likely needs the same audit.
