# Schema Drift Detection — Feature Analysis

This document catalogues every realistic schema drift scenario that can affect a `yb-voyager` live migration, groups them by severity, maps them to detection methods, and identifies workarounds. It is derived from the hands-on test results in `SCHEMA_DRIFT_MANUAL_LOG.md` (scenarios A–S) and from code-level analysis of the exporter/importer pipelines.

---

## 1  Terminology

### 1.1  Migration Phases & Roles

Every `yb-voyager` live migration has an **Export DB** (the database CDC reads from) and an **Import DB** (the database events are applied to). Which physical database fills each role depends on the phase:

| Phase | Export DB | Import DB | Commands |
|-------|----------|-----------|----------|
| **Forward** | Source (PG) | Target (YB) | `export data` + `import data` |
| **Fall-forward (FF)** | Target (YB) | Source-replica | `export data from target` + `import data to source-replica` |
| **Fall-back (FB)** | Target (YB) | Source (PG) | `export data from target` + `import data to source` |

The exporter and importer **share code paths** regardless of phase. This means a scenario described as "add column on the Export DB" produces the same failure whether the Export DB is PostgreSQL (Forward) or YugabyteDB (FF/FB).

**This document describes all scenarios in terms of Export DB and Import DB.** To map to a specific phase, use the table above.

### 1.2  Detection Types

To detect drift, we compare a live database schema against a known reference. There are three useful comparisons:

| Label | What it compares | Phase mapping |
|-------|-----------------|---------------|
| **Exp-Drift** | Live **Export DB** schema vs its stored baseline (snapshot at export start) | Forward → S vs S (live Source vs Source baseline). FF/FB → T vs T (live Target vs Target baseline). |
| **Imp-Drift** | Live **Import DB** schema vs its stored baseline (state after `import schema`) | Forward → T vs T (live Target vs Target baseline). FF/FB → S vs S (live Source vs Source baseline). |
| **Cross-DB** | Live Export DB schema vs Live Import DB schema | S vs T in all phases. |

> **Why role-based labels?** The classic "S vs S / T vs T" labels are tied to physical databases. During Forward, "S vs S" catches export-DB drift. During FF/FB, it is "T vs T" that catches export-DB drift (because the Target is now the export DB). Using **Exp-Drift** and **Imp-Drift** makes the catalog phase-independent — no mental remapping needed.

### 1.3  Impact Levels

| Level | Label | Meaning |
|-------|-------|---------|
| **P0** | **Critical — Restart** | Pipeline cannot self-recover. Internal metadata (`meta.db`, `name_registry.json`, `application.properties`, queue segments) is stale. Fix requires multi-file "mid-migration surgery" that end-users **cannot realistically perform**, or a full migration restart. Possible **silent data loss**. |
| **P1** | **High — Crash → DDL fix + Resume** | Import or export crashes with a clear error (panic, SQLSTATE). Recoverable by running the corresponding DDL on the lagging DB and resuming. **No data loss** if handled promptly — events stay in the queue. |
| **P2** | **Silent Divergence / Pre-Cutover Parity** | No pipeline crash. Source and Import DB silently diverge — either in values flowing through CDC, in ongoing runtime behavior, or in structural schema. The user must align schemas before cutover to avoid post-cutover inconsistency. Also covers purely informational items (e.g. index add/drop) that are detected for completeness. |

---

## 2  Schema Drift Catalog

> **How to read:** "Export DB" = the database being streamed FROM (Source in Forward, Target in FF/FB). "Import DB" = the database events are applied TO (Target in Forward, Source in FF/FB). See §1.1 for the mapping.

### 2.1  P0 — Critical (Restart Required)

These drifts corrupt the pipeline's internal metadata or bypass its capture set. The only realistic user-facing workaround is to **restart the migration** with the new schema in place.

| # | Object | Drift Event | Drift DB | Detection | Consequence | Realistic Workaround | Ref |
|---|--------|------------|----------|-----------|-------------|---------------------|-----|
| 1 | Table | **Create new table** (with PK) | Export DB | Exp-Drift | Table not in Debezium capture set or `name_registry`. Rows **never exported**. | Restart migration with new table included | C |
| 2 | Table | **Create new table** (no PK) | Export DB | Exp-Drift | Same as #1, plus `export data` guardrail rejects tables without a PK for live CDC. | Add PK first, then restart | D |
| 3 | Table | **Rename table** | Export DB | Exp-Drift | Export fails (`42P01` — old name gone). MSR, descriptors, Debezium config all reference old name. | Restart migration | E |
| 4 | Table | **Drop table** | Export DB | Exp-Drift | Export fails (`42P01` on `reltuples`). Stale refs in MSR, Debezium config, schema files, queue segments, `postdata.sql`. | Restart migration (without the table) | I |
| 5 | Table | **Add partition** (new leaf) | Export DB | Exp-Drift | Exporter detects and **ignores** new partition. Rows routed there are **silently lost**. Publication not updated. | Restart migration with partition present | P |
| 6 | PK | **Drop primary key** | Export DB | Exp-Drift | Export: no-PK guardrail → `ErrExit`. Import: `42601` (`ON CONFLICT ()` is invalid SQL). | Re-add PK on export DB, or restart | M |
| 7 | Table | **Detach partition** | Export DB | Exp-Drift | **Silent divergence** while Import DB keeps the partition (no warning — `SourceRenameTablesMap` still rewrites leaf → root, Import DB routes the row correctly). If Import DB **also** detaches, import hits **`23514`** (`no partition of relation "part_t" found for row`) and the pipeline is **stuck** — resume replays the same batch. | **Re-attach partition on Import DB** (observed to unblock import); else restart migration, or mid-migration surgery (publication + MSR + `name_registry`) | T |

**Key observation:** every P0 scenario involves the **Export DB**. They all break the exporter's internal table/capture inventory. **Exp-Drift** detection catches **all** of them.

- During **Forward**: this means Source drift (developers changing PG while migration runs).
- During **FF/FB**: this means Target drift (developers changing YB post-cutover while fallback/fallforward streaming runs).

---

### 2.2  P1 — High (Crash → DDL Fix + Resume)

Import (or rarely export) crashes with a clear error. Fix the lagging DB's schema, resume, and the queued events drain successfully. No data loss.

#### 2.2a  Export-DB drift

The Export DB schema changed, causing a mismatch with the Import DB. The fix is to apply the same (or compatible) DDL on the Import DB, then resume.

- **Forward:** developers change Source → align Target.
- **FF/FB:** developers change Target → align Source / Source-replica.

| # | Object | Drift Event | Drift DB | Detection | Consequence | Workaround | Ref |
|---|--------|------------|----------|-----------|-------------|------------|-----|
| 8 | Column | **Add column** | Export DB | Exp-Drift, Cross-DB | Import **panic**: new column not in import DB catalog (`QuoteAttributeName`) | Add column on import DB, resume | A |
| 9 | Column | **Rename column** | Export DB | Exp-Drift, Cross-DB | Import **panic**: renamed field not in import DB catalog | Rename column on import DB, resume | F |
| 10 | Column | **Incompatible type change** (e.g. INT→NUMERIC, NUMERIC→TEXT) | Export DB | Exp-Drift, Cross-DB | Import `22P02`: value does not fit import DB type | Alter type on import DB, resume | G, H |
| 11 | Column | **Drop NOT NULL column** (import DB has NOT NULL, no DEFAULT) | Export DB | Cross-DB | Import `23502`: CDC omits dropped column → NULL into NOT NULL import DB column | Drop column (or add DEFAULT) on import DB, resume | L |
| 12 | Enum | **Add enum value** | Export DB | Exp-Drift, Cross-DB | Import `22P02`: unknown label on import DB enum type | `ALTER TYPE … ADD VALUE` on import DB, resume | N |
| 13 | PK | **Change PK columns** (e.g. PK(id)→PK(name)) | Export DB | Exp-Drift, Cross-DB | Import `42P10`: `ON CONFLICT (name)` — no matching constraint on import DB | Align PK on import DB, resume | O |
| 14 | Constraint | **Relax NOT NULL** (import DB still strict) | Export DB | Cross-DB | Import `23502`: NULLs from export DB rejected by import DB NOT NULL | Drop NOT NULL (or add DEFAULT) on import DB, resume | S |

#### 2.2b  Import-DB drift

The Import DB schema changed (uncoordinated DDL or "tampering"), making it incompatible with the incoming CDC stream. The fix is to revert the change on the Import DB, then resume.

- **Forward:** someone changes Target → revert on Target.
- **FF/FB:** someone changes Source / Source-replica → revert there.

| # | Object | Drift Event | Drift DB | Detection | Consequence | Workaround | Ref |
|---|--------|------------|----------|-----------|-------------|------------|-----|
| 15 | Constraint | **Add NOT NULL** (no DEFAULT, export DB sends NULLs) | Import DB | Imp-Drift, Cross-DB | Import `23502`: rows with NULL rejected | Drop NOT NULL (or add DEFAULT) on import DB, resume | R |
| 16 | Column | **Add column** (import-DB-only) | Import DB | Imp-Drift | Stale Debezium schema JSON → import error on first resume | Quit + resume again (schema auto-refreshes) | B |
| 17 | Constraint | **Add CHECK constraint** (export DB data can violate) | Import DB | Imp-Drift, Cross-DB | Import `23514`: check violation on valid export DB data | Drop CHECK on import DB, resume | Extrap. from R |
| 18 | Constraint | **Add / tighten UNIQUE constraint** | Import DB | Imp-Drift, Cross-DB | Import `23505` or `42P10`: duplicate or conflict mismatch | Drop / adjust UK on import DB, resume | Extrap. from O, R |
| 19 | Column | **Drop column** (export DB still sends it) | Import DB | Imp-Drift, Cross-DB | Import error: INSERT references non-existent import DB column | Re-add column on import DB, resume | Extrap. from A (reversed) |
| 20 | Column | **Incompatible type change** | Import DB | Imp-Drift, Cross-DB | Import type mismatch error (e.g. `22P02`) | Revert type on import DB, resume | Extrap. from G (reversed) |
| 21 | Column | **Rename column** (export DB still sends old name) | Import DB | Imp-Drift, Cross-DB | Import error: INSERT references old column name not in import DB catalog | Revert rename on import DB, resume | Extrap. from F (reversed) |
| 22 | Table | **Rename table** | Import DB | Imp-Drift, Cross-DB | Import error: INSERT into old table name → `42P01` | Revert rename on import DB, resume | Extrap. from E (reversed) |
| 23 | Domain | **Domain CHECK tightened** (export DB data can violate) | Import DB | Imp-Drift, Cross-DB | Import check/domain violation: valid export DB value rejected by stricter domain | Revert domain change on import DB, resume | Extrap. from N, R |

---

### 2.3  P2 — Silent Divergence / Pre-Cutover Parity

No pipeline crash during migration, but the Export DB and Import DB silently diverge. The user must align schemas before cutover to avoid post-cutover inconsistency. This level also covers purely informational items (e.g. index add/drop) detected for completeness.

| # | Object | Drift Event | Drift DB | Detection | Consequence | Workaround | Ref |
|---|--------|------------|----------|-----------|-------------|------------|-----|
| 24 | Column | **Default value change** | Export DB | Exp-Drift, Cross-DB | CDC carries explicit values → no crash. Post-cutover, inserts omitting the column will produce different defaults on each side. | Align default on import DB before cutover | Q |
| 25 | Trigger | **Trigger changed / added** | Import DB | Imp-Drift | `session_replication_role = replica` disables triggers during import → no crash. Post-cutover, trigger logic diverges. | Align triggers before cutover | Extrap. |
| 26 | Column | **Compatible type widening** (e.g. INT→BIGINT, VARCHAR(50)→VARCHAR(200)) | Export DB | Exp-Drift, Cross-DB | Values still fit import DB type → no crash. Schema metadata drifts silently. | Align type on import DB before cutover | Extrap. from G |
| 27 | Column | **Nullability relaxed on both sides independently** | Both | Cross-DB | No crash (both accept NULLs). Hidden divergence if only one side was intentional. | Verify parity | — |
| 28 | Column | **Drop nullable column** (no NOT NULL on import DB) | Export DB | Exp-Drift | CDC omits column; import DB column fills with NULL or is ignored. No crash. Post-cutover: import DB still has a column the export DB dropped. | Align (drop column on import DB) before cutover | J |
| 29 | Column | **Drop NOT NULL column** (import DB has DEFAULT) | Export DB | Exp-Drift | CDC omits column; import DB fills DEFAULT. No crash. Post-cutover schema divergence same as #28. | Align before cutover | K |
| 30 | Constraint | **Foreign key change** | Either | Exp-Drift or Imp-Drift | FKs disabled during import (`session_replication_role = replica`) → no CDC impact. Voyager does **not** sync FK DDL changes mid-run. Post-cutover: stricter-side FK can reject valid-on-other-side writes. | Align FK before cutover | — |
| 31 | Sequence | **Sequence properties changed** (increment, min, max, cycle) | Either | Exp-Drift or Imp-Drift | Voyager syncs sequence *values* at end-migration, **not** DDL properties. Post-cutover: divergent INCREMENT / CYCLE produces divergent generated IDs. | Align sequence DDL before cutover | — |
| 32 | View | **View definition changed** | Either | Exp-Drift or Imp-Drift | CDC does not capture view changes (only base tables). Post-cutover query divergence for anything using the view. | Align before cutover | — |
| 33 | MView | **Materialized view changed** | Either | Exp-Drift or Imp-Drift | Not part of CDC. Post-cutover stale or missing MView. | Align and refresh before cutover | — |
| 34 | Function | **Function / procedure changed** | Either | Exp-Drift or Imp-Drift | Not part of CDC. If used in triggers or column defaults, behavior diverges post-cutover. | Align before cutover | — |
| 35 | Policy | **RLS policy changed** | Either | Exp-Drift or Imp-Drift | Not part of CDC. Post-cutover access control divergence. | Align before cutover | — |
| 36 | Index | **Index added / dropped** | Import DB | Imp-Drift | No data correctness impact. May affect import throughput (dropping an index can speed imports). | Advisory only — no action required | — |

---

## 3  Detection Coverage Matrix

| Detection Type | P0 (#1–7) | P1 (#8–23) | P2 (#24–36) |
|---------------|-----------|-----------|-------------|
| **Exp-Drift** (Export DB: Live vs Stored) | **All 7** | #8–14 | #24, #26, #28–31, #32–35 |
| **Imp-Drift** (Import DB: Live vs Stored) | — | #15–23 | #25, #30–36 |
| **Cross-DB** (Export DB vs Import DB) | — | #8–17, #19–23 | #24, #26, #27 |

**Key takeaways:**

- **Exp-Drift alone catches every P0 scenario.** This is the highest-value single detection.
- **Imp-Drift catches all import-DB drift** (P1 §2.2b). During FF/FB, when the Target is the active export DB, Imp-Drift also becomes critical for the Source/Source-replica side.
- **Cross-DB** catches most P1 scenarios from either side but **cannot** catch P0 (those are internal-metadata issues, not just schema mismatches).
- Implementing **Exp-Drift + Imp-Drift** covers all detectable scenarios. Adding **Cross-DB** provides defence-in-depth and the most actionable diagnostic output.

---

## 4  When Detection Matters Most — By Phase

### 4.1  Forward migration

Developers are actively working on the **Source** (the Export DB). This is the most likely source of drift.

| Realistic threat | What drifts | Detection that catches it |
|-----------------|-----------|--------------------------|
| Developers change Source schema mid-migration | All P0 (#1–7) and P1 export-side (#8–14) scenarios | **Exp-Drift** (= S vs S during Forward) |
| Someone runs unplanned DDL on Target | P1 import-side (#15–23) scenarios | **Imp-Drift** (= T vs T during Forward) |

### 4.2  Fall-forward / Fall-back

After cutover, the application is live on **YugabyteDB (Target)**. Developers are now actively working on the Target — it is their production database. FF/FB streaming is exporting changes FROM the Target.

| Realistic threat | What drifts | Detection that catches it |
|-----------------|-----------|--------------------------|
| Developers change Target schema post-cutover | **All P0 (#1–7)** and P1 export-side (#8–14) scenarios — on the **Target** now | **Exp-Drift** (= T vs T during FF/FB) |
| Someone changes Source / Source-replica | P1 import-side (#15–23) scenarios | **Imp-Drift** (= S vs S during FF/FB) |

**The critical insight:** P0 scenarios (the worst ones — restart required, silent data loss) can happen on **whichever database is the Export DB**. During Forward that's the Source. During FF/FB that's the Target. Exp-Drift detection must run against the Export DB in every phase.

### 4.3  Trigger points

| Trigger Point | What to check | Why |
|--------------|--------------|-----|
| **Periodic (during export)** | Exp-Drift | Catch export-DB drift before events reach the queue |
| **Periodic (during import)** | Imp-Drift | Catch import-DB drift before batch apply |
| **On import error** | Cross-DB | Diagnose the root cause and suggest the fix DDL |
| **Before cutover** | Cross-DB | Final parity gate — ensure schemas match before switching traffic |
| **On export resume** | Exp-Drift | Detect drift that happened while export was stopped |
| **On import resume** | Imp-Drift, Cross-DB | Detect drift that happened while import was stopped |

### 4.4  Implementation note

The detection code should be parameterized by **role** (Export DB / Import DB), not hard-coded to Source or Target:

| Detection | Forward: runs against | FF/FB: runs against |
|-----------|---------------------|-------------------|
| **Exp-Drift** | Source (PG) | Target (YB) |
| **Imp-Drift** | Target (YB) | Source / Source-replica (PG) |
| **Cross-DB** | Source ↔ Target | Target ↔ Source / Source-replica |

---

## 5  How Schema Drift Detection Helps the User

### 5.1  The problem without this feature

Today, when a schema drift happens during a live migration, the user's experience is:

1. **Something breaks** — the importer crashes with a SQLSTATE error, or the exporter exits with a guardrail message, or (worst case) rows are silently lost and nobody notices.
2. **No diagnosis** — the error message says things like `column "from_source" not found` or `ON CONFLICT () syntax error`, but it does not tell the user *"someone ran ALTER TABLE on the export DB and you need to run the same ALTER on the import DB."*
3. **No guidance** — for P0 scenarios (dropped table, new partition, dropped PK), the user is stuck. The pipeline is broken in a way that cannot be fixed by DDL alone. The only real path is to restart the entire migration, but the error message does not say that either.
4. **No early warning** — the drift may have happened hours ago, but the user only discovers it when a problematic row finally arrives. For P2 scenarios (default changes, compatible type widening), the user may never discover the divergence until after cutover.

### 5.2  What the feature does

Schema drift detection **proactively compares live database schemas against known baselines** and reports categorized findings to the user **before** (or immediately when) something breaks. It answers three questions:

- **What changed?** — e.g., "Column `score` on `subject_t` in the export DB changed from `INTEGER` to `NUMERIC(10,2)` since export started."
- **How bad is it?** — P0 (restart required), P1 (DDL fix + resume), or P2 (align before cutover).
- **What should I do?** — Concrete remediation: "Run `ALTER TABLE public.subject_t ALTER COLUMN score TYPE NUMERIC(10,2) …` on the import DB, then resume." Or for P0: "This drift requires restarting the migration with the new schema included."

### 5.3  What the user sees

A categorized report, something like:

```
Schema Drift Report (Forward migration, export DB: PostgreSQL, import DB: YugabyteDB)
Generated: 2026-04-16 14:32:00

CRITICAL (restart required):
  [P0] Export DB table "public.part_t" has a new partition "part_t_p3"
       not in the export capture set. Rows to this partition are NOT
       being exported. Restart migration to include it.

HIGH (DDL fix + resume):
  [P1] Export DB "public.subject_t.score" type changed: INTEGER → NUMERIC(10,2)
       Import DB still has INTEGER. Import will fail on non-integer values.
       Fix: ALTER TABLE public.subject_t ALTER COLUMN score TYPE NUMERIC(10,2)
            USING score::numeric;   (run on import DB)

  [P1] Export DB "public.subject_phase_t" has new enum value "n_gamma"
       not present on import DB.
       Fix: ALTER TYPE public.subject_phase_t ADD VALUE 'n_gamma';
            (run on import DB)

ADVISORY (align before cutover):
  [P2] Export DB "public.subject_t.flag" DEFAULT changed: 'q_old' → 'q_new'
       Import DB still defaults to 'q_old'. No import failure, but post-cutover
       inserts omitting "flag" will produce different values.
       Fix: ALTER TABLE public.subject_t ALTER COLUMN flag SET DEFAULT 'q_new';
            (run on import DB)

  [P2] Import DB index "idx_subject_t_note" was dropped. Performance-only impact.
       No action required.

Drift-free objects: public.control_t (OK), public.side_t (OK)
```

### 5.4  Design implications

1. **Exp-Drift is the highest-value detection** — it catches every P0 (the scenarios that require a restart and can cause silent data loss). Imp-Drift catches the same P0 patterns when the other database becomes the export DB in a different phase. Both are essential across the full lifecycle.

2. **The baselines already exist** — `name_registry.json` stores table names, `data/schemas/*_schema.json` stores column names and types, `meta.db` stores table lists and unique-key columns. The feature needs to query the live DB catalog and diff against these artifacts.

3. **Phase-agnostic implementation** — the detection code should be parameterized by "export DB" and "import DB" roles, not hard-coded to Source or Target. During Forward, Source is the export DB; during FF/FB, Target is. The same diff logic works for both.

4. **Actionable output over raw diffs** — the value is not in listing every schema difference. It is in classifying the impact (P0/P1/P2) and giving the user the exact DDL to run. The catalog in §2 provides the classification rules.

5. **P0 detection should be blocking** — if we detect a P0 drift (new table, dropped PK, added partition), we should surface it as an error/warning that the user cannot ignore, because continuing the migration will cause silent data loss or a crash that cannot be fixed without a restart.
