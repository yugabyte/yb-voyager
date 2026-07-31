# Critical Review — "CDC conflicts, end to end" (High Level Details tab)

Review of the unique-key conflict-detection spec for YugabyteDB Voyager live migration.
Grounded in the code on branch `priyanshi/optimize-conflict-code` (and cross-checked against `main`).

Key source files:
- `yb-voyager/cmd/conflictDetectionCache.go` — the cache + comparison logic
- `yb-voyager/cmd/live_migration.go` — event handling, hashing/routing, batch retry loop
- `yb-voyager/cmd/live_migration_cdc_partition_strategy.go` — partition-strategy resolution
- `yb-voyager/src/tgtdb/event.go` — event model, SQL/prepared-statement construction
- `yb-voyager/src/tgtdb/yugabytedb.go` — `ExecuteBatch`, retryability classification, upsert mode
- `yb-voyager/src/dbzm/config.go`, `debezium-server-voyager/.../DebeziumRecordTransformer.java` — Debezium value emission

---

## TL;DR

The narrative is good and sections 1–3 are largely accurate. But there are:
- **Two real correctness holes in the custom-partition-key proposal** (the same-PK exemption interplay, and what PK reuse actually does under YB upsert mode — silent data loss, not a clean error).
- **One factual error** about failure consequences: a missed conflict is a **fatal abort**, not a retry/stall.
- **Both follow-up notes are genuine problems, confirmed in code**: value comparison is raw string equality with several false-negative classes; deferred unique constraints break the ordering assumption.

Priority edits before circulating: **A1** (same-channel exemption) and **A2** (upsert-mode data loss) — they change the design, not just wording. Then **B1** (fatal-not-retry), then fold in the **C1/C2** answers.

---

## ⚠️ EMPIRICAL RESULTS — end-to-end tests overturned several code-reading hypotheses

I wrote real end-to-end live-migration tests (PostgreSQL source → Debezium → YugabyteDB target, `integration_live_migration` tag) in `yb-voyager/src/testlivemigration/live_migration_uk_conflict_false_negative_test.go` and ran them against an installed `main` build. **What Debezium actually sends, and what YB actually allows, are decisive — and they shrink the false-negative surface dramatically from what the code read suggested.**

| Class | Hypothesis (from code read) | **Test result** | Why |
|---|---|---|---|
| **numeric scale** (`1.0` vs `1.00`) | Real false negative | ✅ **Migration survived — NOT an FN** | The yb-voyager Debezium exporter **canonicalizes numeric**: `1.0`, `1.00`, `1.000` all emit as `"1"` in the CDC queue. Equal values → identical strings → detection matches and serializes correctly. |
| **jsonb numeric** (`{"v":1.0}`) | Real false negative | ⛔ **Unreachable on YB** | YB rejects the schema: `INDEX on column of type 'JSONB' not yet supported (SQLSTATE 0A000)`. No index → no FN. |
| **interval** (`1 day` vs `24 hours`) | Real false negative | ⛔ **Unreachable on YB** | YB rejects: `INDEX on column of type 'INTERVAL' not yet supported (0A000)`. |
| **citext / nondet. collation** | Real false negative | ⛔ **Unreachable on YB** | YB rejects: `INDEX on column of type 'user_defined_type' not yet supported (0A000)`; analyze-schema already flags PK/UK-on-citext. Nondeterministic collations are unsupported on YB too. |
| **deferred unique constraint** (same-txn swap) | Structural FN → 23505 | ✅✅ **FALSE NEGATIVE CONFIRMED — real 23505** | Source allows the direct swap only because the constraint is `INITIALLY DEFERRED`; YB silently treats the inline `DEFERRABLE` as immediate (yugabyte-db#32212), so applying either UPDATE row-by-row transiently duplicates: `duplicate key value violates unique constraint "deferred_uk_uk_key" (SQLSTATE 23505)`, on the very first event. Fails **regardless** of whether detection serializes the pair — an immediate constraint simply cannot represent a deferred swap applied one row at a time. |
| **GENERATED ALWAYS … STORED unique column** | Structural FN → 23505 | ✅✅ **FALSE NEGATIVE CONFIRMED — real 23505** | The generated column is **not published in the CDC stream**, so detection has no value to compare. With the target recomputing it (ENABLE ALWAYS trigger — the documented YB workaround, since YB has no stored generated columns), the reclaiming INSERT raced ahead and the target raised `duplicate key value violates unique constraint "gen_uk_g_uidx" (SQLSTATE 23505)`, aborting import. |

**Net correction to sections C1/C3:** the *value-representation* false negatives I flagged do **not** manifest for a **YugabyteDB target** — either the exporter canonicalizes the value (numeric) or YB won't index the type at all (jsonb, interval, citext, nondeterministic collations). The false-negative surface that *does* survive is **structural**, not value-based:
1. **Generated-stored unique columns** (confirmed) — column absent from CDC ⇒ detection blind. Manifests as a 23505 when the target independently recomputes the value (trigger workaround), or as silent wrong/empty data when it doesn't.
2. **Deferred unique constraints** (confirmed) — claim-before-release / swap orderings the immediate YB constraint can't accept. Note this one is not really a *detection* bug: no per-row apply order works, so the fix is policy (declare deferred-UK workloads unsupported for CDC + catch the inline `DEFERRABLE` form that YB silently downgrades, yugabyte-db#32212), not a smarter comparison.

Caveat on scope: these results are for a **PG→YB forward migration**. A value-representation FN could still bite a target that *does* index these types (e.g. YB→PG fall-back with forced PK partitioning, or a future YB that indexes them) — but that is not the default path. The practical takeaway is unchanged and if anything simpler: the fix worth prioritizing is the **structural** blind spots (guard generated-column unique indexes; declare deferred-UK workloads unsupported for CDC), not type-aware value normalization.

### Update — comprehensive type × index-grammar sweep (see conflict-detection-fn-test-plan.md)

A follow-up exhaustive suite (~24 e2e tests over every documented PG type and unique-index
grammar variation, run against a binary built from origin/main @89ca377) upgraded this section:

- **Third confirmed FN: covering indexes — `UNIQUE (uk) INCLUDE (payload)`** (TestFNIncludeColumns,
  real 23505). The discovery query `pgQueryTmplForUniqIndexes` uses `a.attnum = ANY(ix.indkey)`,
  which sweeps INCLUDE columns into the key tuple; conflicts on the real key with differing
  INCLUDE values never match. One-line fix: `array_position(ix.indkey, a.attnum) < ix.indnkeyatts`.
- **Every value-representation candidate resolved SAFE or UNREACHABLE**: float ±0.0, domain-over-
  numeric, TOASTed 8KB unique values (RIF before-images are detoasted), PK-update recycling,
  money/bpchar/"char"/bytea/bool/uuid/enum/timestamptz/date/text_pattern_ops — all survived
  genuine conflict-per-iteration workloads. YB's unique-btree type support is narrow (inet,
  macaddr, varbit, int4range, timetz, tsvector, tsquery, arrays, ranges, UDTs, hstore, jsonb,
  interval, citext all rejected — live-probed each run, never assumed), which closes most exotic
  classes at the schema door. Array lower-bounds premise was disproven in PG itself.
- **Positive verifications**: expression-index mitigation works end-to-end; target-only unique
  indexes ARE detected (discovery reads the target — the doc's "target-only → No" claim is stale);
  NULLS NOT DISTINCT handled correctly.
- **Standalone data-shipping bug found**: `'infinity'::date` overflows Debezium's int32
  days-since-epoch conversion (INT32_MAX + 10957 wraps to −2147472692) → unparseable literal →
  non-retryable apply error → migration aborts. Snapshot path unaffected. Needs its own ticket;
  audit timestamp/timestamptz infinity too.

---

## A. Correctness holes in the custom-partition-key design (Section 4)

### A1. The same-PK exemption silently breaks under a custom key — not mentioned in the doc

Today conflict detection **never treats two events with equal PK values as a conflict** — `sameTableEventsHaveSamePK` (`conflictDetectionCache.go:842-858`) skips them in both lookup paths, rationale: "same PK ⇒ same channel ⇒ applied in order." A custom key invalidates that inference in **both** directions:

- **Defeats the proposed PK-conflict check.** The doc's own PK-reuse example (`DELETE id=5002` with routing key K1 / `INSERT id=5002` with routing key K2) is a pair with *equal PK values* — exactly what the exemption filters out before any comparison. Unless the exemption is reworked, the new PK check is bypassed for the very case it exists to catch.
- **Breaks the feature's headline claim.** Figure 10 says "detection has nothing left to catch in steady state." Not with current code: the state-machine table's UPDATE+INSERT pair has *different* PKs and matching `(parent_id, is_current)` unique-key values, so `findValueConflictLocked` still flags it even though the custom key routes both to the same worker — every transition still pays wait + flush-all-workers. The feature would achieve nothing.

**Fix (must be stated in the spec):** the exemption must change from "same PK" to **"routes to the same channel"** (same routing-key value / same hash). This is the load-bearing design change, not a detail.

### A2. PK reuse under a custom key causes *silent data loss*, not a "duplicate primary key" error

The doc says "If E2 lands first: duplicate primary key." Wrong for a YugabyteDB target. CDC INSERT events run with `yb_enable_upsert_mode=true` (`src/tgtdb/yugabytedb.go:1336-1339`), so an insert on an existing PK does **not** error — it **overwrites the row**:

1. E2 `INSERT id=5002` (routing key K2) applies first → silently overwrites the live K1 row.
2. E1 `DELETE id=5002` (routing key K1) applies → the K2 row that should survive is **gone**.
3. No error, no log, wrong data at cutover.

(Upsert + secondary unique indexes is independently hazardous — see [yugabyte-db#13687](https://github.com/yugabyte/yugabyte-db/issues/13687), where an upsert-mode insert can silently delete *other* rows colliding on a unique index.)

This makes the PK-conflict check a **hard data-integrity requirement**, not an error-avoidance nicety — and raises the stakes on A1.

### A3. The immutability guardrail can't run only "at import start"

`assert before == after on the key per event` is inherently a **per-event, mid-stream** check. The spec must answer: what happens when it fires at hour 40 of a 3-day migration? Key is fixed for the migration, so the only recovery is a fresh start. State it, and argue for the strongest possible upfront screen. Mirror image worth noting: the same PK-update hazard exists **today** under partition-by-PK (Debezium turns PK updates into DELETE+INSERT keyed differently) — so "the property the PK gives for free" is only mostly free.

### A4. Smaller unspecified points in Section 4

- **NULL routing-key values**: `hashEvent` (`live_migration.go:409-441`) dereferences `*e.Key[k]` unconditionally; a NULL key needs a sentinel or it panics. Perf guidance ("warn, don't refuse") is right, but the impl detail is missing.
- **Per-op key source**: INSERT → after-image, UPDATE/DELETE → before-image. Edge: an UPDATE's after-image carries Debezium's **unavailable-value placeholder** for unchanged TOASTed columns; if the routing key or a UK column is large/TOASTable, routing/detection compare placeholders. A short id-style key is safe; the general spec isn't.
- **Composite keys**: the config sketch (`schema.table:routing_column`) allows one column. Is `(tenant_id, entity_id)` in scope? Say so.
- **Expression unique indexes**: `cdc-partition-key: pk` is already *rejected* for expr-UK tables (`live_migration_cdc_partition_strategy.go:123-182`). A custom key must be rejected/special-cased too — it can't co-locate conflicts on `lower(email)`, and the "coverage" guardrail can't even evaluate coverage (no column list). Unaddressed.
- **Persistence**: be explicit the key is stored in metaDB (like `TableToCDCPartitioningStrategyMap`) and validated on resume, so an edited config fails loudly.
- **Partitioned tables**: events arrive against the root table (leaf indexes merged to root, `src/tgtdb/postgres.go:382-427`); config must be keyed by root table.

---

## B. Factual errors / inaccuracies (Sections 1–3)

1. **"Retrying doesn't help … migration can stall on a permanent error" (Fig 5) — wrong.** A 23505 is classified non-retryable (`IsPgErrorCodeNonRetryable`, `src/tgtdb/yugabytedb.go:408-442`), the retry loop breaks on the first attempt, and `processEvents` calls `utils.ErrExit` (`live_migration.go:488-528`). A missed conflict is an **immediate fatal abort**. Worse, per A2, on YB some missed conflicts don't error at all → silent wrong data. The doc understates false-negative severity.
2. **Note "Hash Key = table-name+primary-key" is under Strategy 2 but describes Strategy 1.** By-table hashes table name only; by-PK hashes table name + sorted PK values (`live_migration.go:409-441`). Misplaced.
3. **Oracle missing from defaults table** (dangling "PostgreSQL / →"). Oracle sources funnel to PARTITION_BY_TABLE → detection effectively unused for Oracle. Tab-1 claims Oracle gets "full value-based detection." Reconcile.
4. **Unique indexes are read live from the target DB, not source-export metadata** — `initializeConflictDetectionCache` → `tdb.GetTableToUniqueIndexesMap` (`live_migration.go:545-554`, query `src/tgtdb/postgres.go:435-476`). The remaining-work item "read from target instead of source" and the tab-1 "target-only unique index → No" both look **stale on this branch**. Verify + update.
5. **"plus old-vs-old for partial indexes"** — code currently runs before-vs-before for **every** unique index (that's the open "skip before-vs-before when no partial index" item, DB-22047). Overstates how targeted the waste is.
6. **"A true conflict on every write"** — the first transition of each payment is a lone INSERT with nothing to conflict with. Use "every transition after the first."
7. **Section 5: "The first six levers… The last one changes the routing"** — table has 6 rows, custom key is row 3, and the actual last row is a cost lever. Sentence contradicts its table.

---

## C. Follow-up notes — researched

### C1. "Normalized or raw values?" → **Raw. Go string `==` on unconverted Debezium payloads.**

Both the bucket-key fast path (`computeConflictBucketKey`, `conflictDetectionCache.go:433-462`) and the direct comparator (`uniqueKeyColumnValuesEqual`, `:789-799`, literally `*left == *right`). Detection runs **before** the value converter (`handleEvent`, `live_migration.go:360-403`), and cached copies keep unconverted values — so both sides use the *same* representation. That kills formatting-drift between the two events, but not source-level equality mismatches.

**Governing principle:** detection is sound only for types where the target index's btree `=` equals **byte equality of the emitted Debezium string**. Violators (⚠️ note: the numeric row below was **disproven by an end-to-end test** — see the "EMPIRICAL UPDATE" box):

| Sub-item | Verdict |
|---|---|
| **Float NaN** | OK — every NaN bit pattern canonicalizes to `"NaN"` on both Java and PG sides. **But** `-0.0` vs `0.0` are index-equal, string-unequal → missed conflict (untested; low value — floats in unique keys are rare). |
| **Numeric precision** | ~~Real false negative~~ → **DISPROVEN by e2e test.** The yb-voyager Debezium exporter **canonicalizes numeric** to a scale-normalized string: `1.0`, `1.00`, `1.000` all emit as `"1"`. Numerically-equal values therefore produce *identical* strings, detection matches them, and the conflict is serialized correctly. Bare `numeric`/`numeric(p,s)` columns are **safe**. (The code-reading hypothesis — that `BigDecimal.toString()` preserves scale — did not hold empirically.) |
| **Collations / citext** | **Real false negative (test-confirmed — see box).** `citext` and nondeterministic collations compare case/accent-insensitively in the index but byte-wise in detection. Partly mitigated by analyze-schema flagging PK/UK-on-citext + nondeterministic collations — but if the user proceeds, detection is blind. |
| **PG vs YB comparators** | The comparison that matters is *target index equality* vs detection string equality — the divergences are the rows above. PG-vs-YB drift is secondary (YB follows PG for these types). |

### C2. "Deferred unique constraints" → **Yes, they break the assumption.**

PG logical decoding emits changes in intra-transaction statement order; a deferred constraint lets the source legally emit **claim-before-release** (insert taking value A before the delete/update freeing it) or a **swap** (`U1: A→B; U2: B→A`). Detection assumes release precedes claim, and **inserts are never cached** (`Put` rejects op `c`, `conflictDetectionCache.go:164-167`) — so claim-first orderings are undetectable by construction. Three scenarios:

1. **Out-of-line `UNIQUE … DEFERRABLE`** — YB rejects it at import-schema ([#1709](https://github.com/yugabyte/yugabyte-db/issues/1709)); user drops it; detection reads target indexes so nothing is tracked → benign, but constraint unenforced.
2. **Inline column-level `UNIQUE DEFERRABLE`** — YB **silently ignores** the deferrable clause and creates a plain unique index ([#32212](https://github.com/yugabyte/yugabyte-db/issues/32212)). Now a legitimate swap/claim-first workload produces streams the plain target index can't accept → fatal 23505 on correct source data. **The dangerous case.**
3. **Fall-back to PG** (deferrable constraint retained): target batches don't preserve source txn boundaries (batch = per-channel count/time window, `live_migration.go:447-486`; one target txn per batch, `yugabytedb.go:1200-1329`), so a swap split across two batches fails at the first commit even on a deferrable target. Partition-by-table does **not** save you.

**Mitigation is policy, not a detection fix:** analyze-schema already flags deferrable non-FK constraints (`queryissue/detectors_ddl.go:144-151`, IMPACT_LEVEL_3). The live-migration spec should (a) declare deferrable-UK workloads unsupported for CDC, and (b) verify the parser catches the *inline* deferrable UK form (#32212 hole — YB won't catch it for you).

Exclusion constraints (name-dropped in Fig 4): not fetched (`indisunique = TRUE` filter), invisible to detection — but YB doesn't support them (import-schema blocker), so moot for forward migration. One clarifying sentence.

---

## C3. Additional false negatives found (beyond numeric/collation)

**Family 1 — more value-equality misses (btree `=` coarser than byte-equality of emitted string):**

| Type | Why it misses | Example (index-equal, string-unequal) |
|---|---|---|
| `interval` | PG flattens 1 mon = 30 d, 1 d = 24 h; `interval.handling.mode=string` (`dbzm/config.go`) emits distinct ISO strings | `'1 day'`→`P1D` vs `'24 hours'`→`PT24H` |
| bare `NUMERIC` (no scale) | `VariableScaleDecimal.toString()` preserves each value's own scale | `1.0` vs `1.00` |
| jsonb w/ numbers | jsonb `=` compares numbers numerically; text preserves written scale | `{"a":1.0}` vs `{"a":1.00}` |
| numeric in containers | scale issue propagates through `numeric[]`, `numrange`, composites, domains | `numrange('[1.0,2.0]')` vs `'[1.00,2.00]'` |
| float signed zero | `-0.0 = 0.0` in index; Java prints `-0.0` vs `0.0` | `-0.0` vs `0.0` |
| hstore | `hstore.handling.mode=map` — non-deterministic key order can emit same value two ways (verify) | `"a"=>"1","b"=>"2"` vs `"b"=>"2","a"=>"1"` |

**Safe types:** integers, text/varchar under deterministic collation (byte-wise, so NFC/NFD agree with detection), uuid, date/timestamp/timestamptz (epoch numbers pre-conversion), bytea, boolean, enum, char(n) (pre-padded), inet, discrete ranges (int4range/daterange — canonicalized on input).

**Family 2 — structural blind spots (worse; detection never sees the values):**

1. **Unique index on `GENERATED ALWAYS AS … STORED` column** — pgoutput doesn't publish generated columns (pre-PG18) → column absent from every event → `computeConflictBucketKey` returns not-indexable → **zero conflicts ever detected**, while target index enforces it → fatal 23505 / upsert corruption. **Not** caught by force-by-table (index looks plain; `indexprs` is NULL). Tempered by analyze-schema flagging stored generated cols as unsupported (`issues_ddl.go:29-46`), but the "convert to trigger" workaround changes the risk profile. Cheap guardrail: at import start, verify every unique-index column is present in event payloads; force by-table otherwise.
2. **Unique index created after import-data starts** — cache loads index list once at startup (`live_migration.go:545-554`), never refreshes → mid-stream DDL invisible to detection while fully enforced by target.
3. **Deferred unique constraints** — see C2.
4. **YB-as-source before-images** — already in the doc.

**Adjacent false *positive* (add to follow-ups):** Debezium's unchanged-TOAST placeholder (`__debezium_unavailable_value`) is treated as an ordinary string (no reference anywhere in the Go path). Two updates on different rows both leaving a TOASTed unique column unchanged would both "claim" the placeholder → spurious conflict. Can't cause a false negative (unchanged column claims nothing; a real value never equals the placeholder).

**Distilled recommendation:** the two mitigations are already in the doc's vocabulary — (1) type-aware normalization at comparison time for the numeric family (at minimum strip trailing fractional zeros); (2) force partition-by-table for tables whose unique keys detection can't faithfully compare (citext, nondeterministic collation, generated columns) — exactly how expression indexes are handled today.

---

## D. What the doc gets right (verified)

Head-of-line-blocking (detection pre-dispatch on the single stream thread); the four-step response (deletes/updates cached, inserts wait-only, flush-all on conflict, wake-and-recheck); NULL / NULLS-NOT-DISTINCT handling via `pg_index.indnullsnotdistinct`; expr-UK tables forced to by-table with explicit `pk` rejected; REPLICA IDENTITY FULL enforced as an export-side check; the email/phone "no perfect key" example.
