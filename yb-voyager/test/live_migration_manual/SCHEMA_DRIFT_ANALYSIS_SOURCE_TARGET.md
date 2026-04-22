# Schema Drift Detection — Feature Analysis (Source / Target View)

> **Companion doc:** This is the Source/Target-framed version of `SCHEMA_DRIFT_ANALYSIS.md`. Same scenarios, same impact levels, same detection types — but described in terms of **Source (PostgreSQL)** and **Target (YugabyteDB)** rather than abstract Export DB / Import DB roles. Use this version for presenting the Forward live-migration case; use the companion for the role-agnostic view that also covers fall-forward / fall-back.

This document catalogues every realistic schema drift scenario that can affect a `yb-voyager` live migration, groups them by severity, maps them to detection methods, and identifies workarounds. It is derived from the hands-on test results in `SCHEMA_DRIFT_MANUAL_LOG.md` (scenarios A–S) and from code-level analysis of the exporter/importer pipelines.

---

## 1  Terminology

### 1.1  Migration Setup

Live migration with `yb-voyager`:

- **Source**: PostgreSQL. Debezium runs logical replication against it. `yb-voyager export data` reads CDC events from here.
- **Target**: YugabyteDB. `yb-voyager import data` applies CDC events here.

During the live phase, developers may still be making schema changes on **either** side:

- **Source-side drift** — developers continue to work on PostgreSQL while the migration runs. This is the most common drift source.
- **Target-side drift** — somebody runs unplanned DDL on YugabyteDB. Less common during Forward migration, but it does happen (and becomes the primary concern post-cutover during fall-forward / fall-back — see §4).

### 1.2  Detection Types

To detect drift, we compare a live database schema against a known reference. There are three useful comparisons:

| Label | What it compares | What it catches |
|-------|-----------------|-----------------|
| **S vs S** | Live **Source** (PG) schema vs its stored baseline (snapshot at export start) | Source-side drift — the schema developers are changing while the migration runs |
| **T vs T** | Live **Target** (YB) schema vs its stored baseline (state after `import schema`) | Target-side drift — unplanned or uncoordinated DDL on YugabyteDB |
| **S vs T** | Live Source (PG) schema vs Live Target (YB) schema | Parity mismatch — any column / type / constraint difference between the two, regardless of who changed it |

### 1.3  Impact Levels

| Level | Label | Meaning |
|-------|-------|---------|
| **P0** | **Critical — Restart** | Pipeline cannot self-recover. Internal metadata (`meta.db`, `name_registry.json`, `application.properties`, queue segments) is stale. Fix requires multi-file "mid-migration surgery" that end-users **cannot realistically perform**, or a full migration restart. Possible **silent data loss**. |
| **P1** | **High — Crash → DDL fix + Resume** | Import or export crashes with a clear error (panic, SQLSTATE). Recoverable by running the corresponding DDL on the lagging DB and resuming. **No data loss** if handled promptly — events stay in the queue. |
| **P2** | **Silent Divergence / Pre-Cutover Parity** | No pipeline crash. Source and Target silently diverge — either in values flowing through CDC, in ongoing runtime behavior, or in structural schema. The user must align schemas before cutover to avoid post-cutover inconsistency. Also covers purely informational items (e.g. index add/drop) that are detected for completeness. |

---

## 2  Schema Drift Catalog

### 2.1  P0 — Critical (Restart Required)

These drifts corrupt the pipeline's internal metadata or bypass its capture set. The only realistic user-facing workaround is to **restart the migration** with the new schema in place.

| # | Object | Drift Event | Drift DB | Detection | Consequence | Realistic Workaround | Ref |
|---|--------|------------|----------|-----------|-------------|---------------------|-----|
| 1 | Table | **Create new table** (with PK) | Source | S vs S | Table not in Debezium capture set or `name_registry`. Rows **never exported**. | Restart migration with new table included | C |
| 2 | Table | **Create new table** (no PK) | Source | S vs S | Same as #1, plus `export data` guardrail rejects tables without a PK for live CDC. | Add PK first, then restart | D |
| 3 | Table | **Rename table** | Source | S vs S | Export fails (`42P01` — old name gone). MSR, descriptors, Debezium config all reference old name. | Restart migration | E |
| 4 | Table | **Drop table** | Source | S vs S | Export fails (`42P01` on `reltuples`). Stale refs in MSR, Debezium config, schema files, queue segments, `postdata.sql`. | Restart migration (without the table) | I |
| 5 | Table | **Add partition** (new leaf) | Source | S vs S | Exporter detects and **ignores** new partition. Rows routed there are **silently lost**. Publication not updated. | Restart migration with partition present | P |
| 6 | PK | **Drop primary key** | Source | S vs S | Export: no-PK guardrail → `ErrExit`. Import: `42601` (`ON CONFLICT ()` is invalid SQL). | Re-add PK on Source, or restart | M |
| 7 | Table | **Detach partition** | Source | S vs S | **Silent divergence** while Target keeps the partition (no warning — `SourceRenameTablesMap` still rewrites leaf → root, Target routes the row correctly). If Target **also** detaches, import hits **`23514`** (`no partition of relation "part_t" found for row`) and the pipeline is **stuck** — resume replays the same batch. | **Re-attach partition on Target** (observed to unblock import); else restart migration, or mid-migration surgery (publication + MSR + `name_registry`) | T |

**Key observation:** every P0 scenario is **Source-side drift**. They all break the exporter's internal table/capture inventory. **S vs S** detection catches **all** of them.

---

### 2.2  P1 — High (Crash → DDL Fix + Resume)

Import (or rarely export) crashes with a clear error. Fix the lagging DB's schema, resume, and the queued events drain successfully. No data loss.

#### 2.2a  Source-side drift

The Source (PG) schema changed, causing a mismatch with the Target (YB). The fix is to apply the same (or compatible) DDL on the Target, then resume.

| # | Object | Drift Event | Drift DB | Detection | Consequence | Workaround | Ref |
|---|--------|------------|----------|-----------|-------------|------------|-----|
| 8 | Column | **Add column** | Source | S vs S, S vs T | Import **panic**: new column not in Target catalog (`QuoteAttributeName`) | Add column on Target, resume | A |
| 9 | Column | **Rename column** | Source | S vs S, S vs T | Import **panic**: renamed field not in Target catalog | Rename column on Target, resume | F |
| 10 | Column | **Incompatible type change** (e.g. INT→NUMERIC, NUMERIC→TEXT) | Source | S vs S, S vs T | Import `22P02`: value does not fit Target type | Alter type on Target, resume | G, H |
| 11 | Column | **Drop NOT NULL column** (Target has NOT NULL, no DEFAULT) | Source | S vs T | Import `23502`: CDC omits dropped column → NULL into NOT NULL Target column | Drop column (or add DEFAULT) on Target, resume | L |
| 12 | Enum | **Add enum value** | Source | S vs S, S vs T | Import `22P02`: unknown label on Target enum type | `ALTER TYPE … ADD VALUE` on Target, resume | N |
| 13 | PK | **Change PK columns** (e.g. PK(id)→PK(name)) | Source | S vs S, S vs T | Import `42P10`: `ON CONFLICT (name)` — no matching constraint on Target | Align PK on Target, resume | O |
| 14 | Constraint | **Relax NOT NULL** (Target still strict) | Source | S vs T | Import `23502`: NULLs from Source rejected by Target NOT NULL | Drop NOT NULL (or add DEFAULT) on Target, resume | S |

#### 2.2b  Target-side drift

The Target (YB) schema changed (uncoordinated DDL or "tampering"), making it incompatible with the incoming CDC stream. The fix is to revert the change on the Target, then resume.

| # | Object | Drift Event | Drift DB | Detection | Consequence | Workaround | Ref |
|---|--------|------------|----------|-----------|-------------|------------|-----|
| 15 | Constraint | **Add NOT NULL** (no DEFAULT, Source sends NULLs) | Target | T vs T, S vs T | Import `23502`: rows with NULL rejected | Drop NOT NULL (or add DEFAULT) on Target, resume | R |
| 16 | Column | **Add column** (Target-only) | Target | T vs T | Stale Debezium schema JSON → import error on first resume | Quit + resume again (schema auto-refreshes) | B |
| 17 | Constraint | **Add CHECK constraint** (Source data can violate) | Target | T vs T, S vs T | Import `23514`: check violation on valid Source data | Drop CHECK on Target, resume | Extrap. from R |
| 18 | Constraint | **Add / tighten UNIQUE constraint** | Target | T vs T, S vs T | Import `23505` or `42P10`: duplicate or conflict mismatch | Drop / adjust UK on Target, resume | Extrap. from O, R |
| 19 | Column | **Drop column** (Source still sends it) | Target | T vs T, S vs T | Import error: INSERT references non-existent Target column | Re-add column on Target, resume | Extrap. from A (reversed) |
| 20 | Column | **Incompatible type change** | Target | T vs T, S vs T | Import type mismatch error (e.g. `22P02`) | Revert type on Target, resume | Extrap. from G (reversed) |
| 21 | Column | **Rename column** (Source still sends old name) | Target | T vs T, S vs T | Import error: INSERT references old column name not in Target catalog | Revert rename on Target, resume | Extrap. from F (reversed) |
| 22 | Table | **Rename table** | Target | T vs T, S vs T | Import error: INSERT into old table name → `42P01` | Revert rename on Target, resume | Extrap. from E (reversed) |
| 23 | Domain | **Domain CHECK tightened** (Source data can violate) | Target | T vs T, S vs T | Import check/domain violation: valid Source value rejected by stricter domain | Revert domain change on Target, resume | Extrap. from N, R |

---

### 2.3  P2 — Silent Divergence / Pre-Cutover Parity

No pipeline crash during migration, but the Source and Target silently diverge. The user must align schemas before cutover to avoid post-cutover inconsistency. This level also covers purely informational items (e.g. index add/drop) detected for completeness.

| # | Object | Drift Event | Drift DB | Detection | Consequence | Workaround | Ref |
|---|--------|------------|----------|-----------|-------------|------------|-----|
| 24 | Column | **Default value change** | Source | S vs S, S vs T | CDC carries explicit values → no crash. Post-cutover, inserts omitting the column will produce different defaults on each side. | Align default on Target before cutover | Q |
| 25 | Trigger | **Trigger changed / added** | Target | T vs T | `session_replication_role = replica` disables triggers during import → no crash. Post-cutover, trigger logic diverges. | Align triggers before cutover | Extrap. |
| 26 | Column | **Compatible type widening** (e.g. INT→BIGINT, VARCHAR(50)→VARCHAR(200)) | Source | S vs S, S vs T | Values still fit Target type → no crash. Schema metadata drifts silently. | Align type on Target before cutover | Extrap. from G |
| 27 | Column | **Nullability relaxed on both sides independently** | Both | S vs T | No crash (both accept NULLs). Hidden divergence if only one side was intentional. | Verify parity | — |
| 28 | Column | **Drop nullable column** (no NOT NULL on Target) | Source | S vs S | CDC omits column; Target column fills with NULL or is ignored. No crash. Post-cutover: Target still has a column the Source dropped. | Align (drop column on Target) before cutover | J |
| 29 | Column | **Drop NOT NULL column** (Target has DEFAULT) | Source | S vs S | CDC omits column; Target fills DEFAULT. No crash. Post-cutover schema divergence same as #28. | Align before cutover | K |
| 30 | Constraint | **Foreign key change** | Either | S vs S or T vs T | FKs disabled during import (`session_replication_role = replica`) → no CDC impact. Voyager does **not** sync FK DDL changes mid-run. Post-cutover: stricter-side FK can reject valid-on-other-side writes. | Align FK before cutover | — |
| 31 | Sequence | **Sequence properties changed** (increment, min, max, cycle) | Either | S vs S or T vs T | Voyager syncs sequence *values* at end-migration, **not** DDL properties. Post-cutover: divergent INCREMENT / CYCLE produces divergent generated IDs. | Align sequence DDL before cutover | — |
| 32 | View | **View definition changed** | Either | S vs S or T vs T | CDC does not capture view changes (only base tables). Post-cutover query divergence for anything using the view. | Align before cutover | — |
| 33 | MView | **Materialized view changed** | Either | S vs S or T vs T | Not part of CDC. Post-cutover stale or missing MView. | Align and refresh before cutover | — |
| 34 | Function | **Function / procedure changed** | Either | S vs S or T vs T | Not part of CDC. If used in triggers or column defaults, behavior diverges post-cutover. | Align before cutover | — |
| 35 | Policy | **RLS policy changed** | Either | S vs S or T vs T | Not part of CDC. Post-cutover access control divergence. | Align before cutover | — |
| 36 | Index | **Index added / dropped** | Target | T vs T | No data correctness impact. May affect import throughput (dropping an index can speed imports). | Advisory only — no action required | — |

---

## 3  Detection Coverage Matrix

| Detection Type | P0 (#1–7) | P1 (#8–23) | P2 (#24–36) |
|---------------|-----------|-----------|-------------|
| **S vs S** (Source: Live vs Stored) | **All 7** | #8–14 | #24, #26, #28–31, #32–35 |
| **T vs T** (Target: Live vs Stored) | — | #15–23 | #25, #30–36 |
| **S vs T** (Parity) | — | #8–17, #19–23 | #24, #26, #27 |

**Key takeaways:**

- **S vs S alone catches every P0 scenario.** This is the highest-value single detection.
- **T vs T catches all Target-side drift** (P1 §2.2b) — useful during Forward, and critical during fall-forward / fall-back (see §4.2).
- **S vs T** catches most P1 scenarios from either side but **cannot** catch P0 (those are internal-metadata issues, not just schema mismatches). It is, however, the most actionable diagnostic: it produces messages like *"Source column X is NUMERIC but Target column X is INTEGER."*
- Implementing **S vs S + T vs T** covers all detectable scenarios. Adding **S vs T** provides defence-in-depth and the best user-facing messages.

---

## 4  When Detection Matters Most

### 4.1  Forward migration (the main case)

Developers are actively working on the **Source (PostgreSQL)**. This is the most likely source of drift during the live migration window.

| Realistic threat | What drifts | Detection that catches it |
|-----------------|-----------|--------------------------|
| Developers change Source schema mid-migration | All P0 (#1–7) and P1 Source-side (#8–14) scenarios | **S vs S** |
| Someone runs unplanned DDL on Target | P1 Target-side (#15–23) scenarios | **T vs T** |
| Post-cutover divergence risk | P2 (#24–36) scenarios | **S vs T** (pre-cutover gate) |

### 4.2  Fall-forward / Fall-back phase

After cutover, the application is live on **YugabyteDB (Target)**. Developers are now actively working on the Target. During fall-forward (stream YB → Source-replica) or fall-back (stream YB → Source), the Target is now the export side. This means:

- Every P0 scenario (new table, drop table, add partition, drop PK, …) can now happen **on the Target**.
- **T vs T** becomes critical during FF/FB — it is the detection that catches P0-level drift in that phase.

The detection code should be written to work against either database. The role-agnostic version of this document (`SCHEMA_DRIFT_ANALYSIS.md`) describes it in "Export DB / Import DB" terms precisely to avoid confusion here. For a Forward-only presentation, the Source/Target framing used in this document is sufficient.

### 4.3  Trigger points

| Trigger Point | What to check | Why |
|--------------|--------------|-----|
| **Periodic (during `export data`)** | S vs S | Catch Source drift before events reach the queue |
| **Periodic (during `import data`)** | T vs T | Catch Target drift before batch apply |
| **On import error** | S vs T | Diagnose the root cause and suggest the fix DDL |
| **Before cutover** | S vs T | Final parity gate — ensure schemas match before switching traffic |
| **On `export data` resume** | S vs S | Detect drift that happened while export was stopped |
| **On `import data` resume** | T vs T, S vs T | Detect drift that happened while import was stopped |

---

## 5  How Schema Drift Detection Helps the User

### 5.1  The problem without this feature

Today, when a schema drift happens during a live migration, the user's experience is:

1. **Something breaks** — the importer crashes with a SQLSTATE error, or the exporter exits with a guardrail message, or (worst case) rows are silently lost and nobody notices.
2. **No diagnosis** — the error message says things like `column "from_source" not found` or `ON CONFLICT () syntax error`, but it does not tell the user *"someone ran ALTER TABLE on the Source and you need to run the same ALTER on the Target."*
3. **No guidance** — for P0 scenarios (dropped table, new partition, dropped PK), the user is stuck. The pipeline is broken in a way that cannot be fixed by DDL alone. The only real path is to restart the entire migration, but the error message does not say that either.
4. **No early warning** — the drift may have happened hours ago, but the user only discovers it when a problematic row finally arrives. For P2 scenarios (default changes, compatible type widening), the user may never discover the divergence until after cutover.

### 5.2  What the feature does

Schema drift detection **proactively compares live database schemas against known baselines** and reports categorized findings to the user **before** (or immediately when) something breaks. It answers three questions:

- **What changed?** — e.g., "Column `score` on Source `subject_t` changed from `INTEGER` to `NUMERIC(10,2)` since export started."
- **How bad is it?** — P0 (restart required), P1 (DDL fix + resume), or P2 (align before cutover).
- **What should I do?** — Concrete remediation: "Run `ALTER TABLE public.subject_t ALTER COLUMN score TYPE NUMERIC(10,2) …` on the Target, then resume." Or for P0: "This drift requires restarting the migration with the new schema included."

### 5.3  What the user sees

A categorized report, something like:

```
Schema Drift Report (Forward migration, Source: PostgreSQL, Target: YugabyteDB)
Generated: 2026-04-16 14:32:00

CRITICAL (restart required):
  [P0] Source table "public.part_t" has a new partition "part_t_p3"
       not in the export capture set. Rows to this partition are NOT
       being exported. Restart migration to include it.

HIGH (DDL fix + resume):
  [P1] Source "public.subject_t.score" type changed: INTEGER → NUMERIC(10,2)
       Target still has INTEGER. Import will fail on non-integer values.
       Fix: ALTER TABLE public.subject_t ALTER COLUMN score TYPE NUMERIC(10,2)
            USING score::numeric;   (run on Target)

  [P1] Source "public.subject_phase_t" has new enum value "n_gamma"
       not present on Target.
       Fix: ALTER TYPE public.subject_phase_t ADD VALUE 'n_gamma';
            (run on Target)

ADVISORY (align before cutover):
  [P2] Source "public.subject_t.flag" DEFAULT changed: 'q_old' → 'q_new'
       Target still defaults to 'q_old'. No import failure, but post-cutover
       inserts omitting "flag" will produce different values.
       Fix: ALTER TABLE public.subject_t ALTER COLUMN flag SET DEFAULT 'q_new';
            (run on Target)

  [P2] Target index "idx_subject_t_note" was dropped. Performance-only impact.
       No action required.

Drift-free objects: public.control_t (OK), public.side_t (OK)
```

### 5.4  Design implications

1. **S vs S is the highest-value detection** — it catches every P0 (the scenarios that require a restart and can cause silent data loss). T vs T catches Target-side drift during Forward and becomes equally critical during fall-forward / fall-back. Both are essential for full-lifecycle coverage.

2. **The baselines already exist** — `name_registry.json` stores table names, `data/schemas/*_schema.json` stores column names and types, `meta.db` stores table lists and unique-key columns. The feature needs to query the live DB catalog and diff against these artifacts.

3. **Role-agnostic implementation** — during Forward, Source is the export side. During FF/FB, Target is. The detection code should be written once and parameterized rather than hard-coded to a particular physical DB.

4. **Actionable output over raw diffs** — the value is not in listing every schema difference. It is in classifying the impact (P0/P1/P2) and giving the user the exact DDL to run. The catalog in §2 provides the classification rules.

5. **P0 detection should be blocking** — if we detect a P0 drift (new table, dropped PK, added partition), we should surface it as an error/warning that the user cannot ignore, because continuing the migration will cause silent data loss or a crash that cannot be fixed without a restart.
