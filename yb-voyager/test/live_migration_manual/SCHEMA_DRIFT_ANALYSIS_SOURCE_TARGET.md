# Schema Drift Detection — Feature Analysis (Source / Target View)

> **Companion doc:** This is the Source/Target-framed version of `SCHEMA_DRIFT_ANALYSIS.md`. Same scenarios, same impact levels, same detection types — but described in terms of **Source (PostgreSQL)** and **Target (YugabyteDB)** rather than abstract Export DB / Import DB roles. Use this version for presenting the Forward live-migration case; use the companion for the role-agnostic view that also covers fall-forward / fall-back.

This document catalogues every realistic schema drift scenario that can affect a `yb-voyager` live migration, groups them by severity, maps them to detection methods, and identifies workarounds. It is derived from the hands-on test results in `SCHEMA_DRIFT_MANUAL_LOG.md` (scenarios A–T) and from code-level analysis of the exporter/importer pipelines.

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
| **P0** | **Critical — Restart required** | The existing migration cannot recover. Voyager's internal capture inventory is broken (stale names, missing partition, no PK for CDC) and there is **no realistic way to fix it short of restarting** the migration with the new schema in place. |
| **P1** | **High — Recoverable without restart** | The existing migration can be brought back to health by a targeted action, **without restarting**. Either: (a) a **separate supplemental migration** captures the new object while the main one keeps running, or (b) a **single DDL on the other side + resume** unblocks a stuck or crashed pipeline. No data loss if handled promptly. |
| **P2** | **Silent divergence / Pre-cutover parity** | No pipeline crash during migration. Source and Target silently diverge — in values flowing through CDC, in runtime behavior, or in structural schema. Must be aligned on the Target **before cutover** to avoid post-cutover inconsistency. Also covers purely informational items (e.g. index add/drop). |

---

## 2  Schema Drift Catalog

### 2.1  P0 — Critical (Restart required)

These four drifts break voyager's capture inventory for the affected object. **Primary** remediation is usually a **full migration restart** with the right table list. **Alternate** paths exist where noted (supplemental migration, exclude table from live list). The DDL is assumed to stand.

> **How to read the columns:**
> - **Consequence** — short summary of what the user sees and **when** something breaks (not every internal query).
> - **Workaround** — primary path first; mid-migration surgery is **not listed** (possible in code but unsupported for customers).

| # | Object | Drift Event | Drift DB | Detection | Consequence | Workaround | Ref |
|---|--------|------------|----------|-----------|-------------|-----------|-----|
| 1 | Table | **Rename table** | Source | S vs S | No immediate crash — existing export keeps streaming **other** tables; replication for the renamed table is already inconsistent with voyager's stored name. **Next time `export data` starts** (after exit, crash, host restart, upgrade, etc.), startup fails with `42P01` — voyager still asks Source for the **old** name. | **Primary:** Restart the **whole** migration with the **new** table name in the initial list. · **Alt:** `TRUNCATE` (or delete all rows from) the corresponding table on **Target**, then run a **separate** `yb-voyager` migration in its own `export-dir` listing **only** `public.<new_name>` — same pattern as a brand-new table. That keeps the renamed table syncing while the main flow is wedged; **any future restart of the main `export data` still hits `42P01`** until the main migration is restarted with corrected metadata. | E |
| 2 | Table | **Drop table** | Source | S vs S | No immediate crash — CDC for **other** tables keeps running; there is nothing left on Source to replicate for the dropped table anyway. **Next time `export data` starts**, startup fails with `42P01` — the stored table list still includes the dropped name. | **Primary:** Restart migration **without** that table in the list. While **both** export and import **never** stop and **never** restart, you may see no error — in practice processes **do** restart (crashes, maintenance, upgrades), which is when this surfaces. | I |
| 3 | Table | **Add partition** (new leaf) | Source | S vs S | **Silent data loss** — new leaf is absent from publication / capture list / `name_registry`, so INSERTs routed only to that leaf are skipped by CDC. **Next `export data` start**, voyager logs *"Detected new partition tables … will not be considered during migration"* and continues **without** adding the leaf. | **Primary:** Restart migration from scratch with the partition present from the start — fresh snapshot + CDC. · **Alt (to validate):** Try a **separate** migration whose table list contains **only** the new leaf (or leaf + root, per testing) — may work if voyager treats the leaf as its own capture object; **not yet confirmed**; watch for overlap with the parent publication, duplicate events, or routing quirks. | P |
| 4 | PK | **Drop primary key** | Source | S vs S | No graceful path — voyager assumes a PK for live CDC. **Next `export data` start**, voyager's no-PK guardrail exits before streaming (`ErrExit`). **While export+import stay up without restarting export**, the **next applied CDC event** for that table can crash **import** with `42601` — apply builds `INSERT … ON CONFLICT ()`, which is invalid SQL when the PK list is empty. | Restart migration with that table **excluded from the live-migration list** (snapshot-only / manual sync for it). | M |

**Key observation:** all four P0 scenarios are **Source-side drift** during Forward, and **S vs S** detection catches all of them. After cutover (fall-forward / fall-back), the same DDLs on YugabyteDB produce the same P0 class — see the role-agnostic companion (`SCHEMA_DRIFT_ANALYSIS.md`).

---

### 2.2  P1 — High (Recoverable without restart)

The existing migration can be brought back to health without restarting. The fix is one of two kinds:

- **(a) Silent new-object drift** — no crash, but rows to the new object are being lost. A **separate supplemental migration** captures them while the main migration keeps running.
- **(b) Pipeline crash or stuck** — import fails with a clear SQLSTATE or gets hard-blocked. A **single DDL on the other side + resume** unblocks it; queued events drain successfully, no data loss.

> **"Separate migration"** = launch a second `yb-voyager` flow (its own `export-dir`, its own snapshot + CDC) covering only the affected object, while the main migration keeps running.

#### 2.2a  Silent new-object drift on Source (fix: separate migration)

A new object appeared on Source that voyager's capture set doesn't know about. The main migration keeps running unaffected for everything else; what's needed is a parallel migration for just the new object.

| # | Object | Drift Event | Drift DB | Detection | Consequence | Workaround | Ref |
|---|--------|------------|----------|-----------|-------------|------------|-----|
| 5 | Table | **Create new table** (with PK) | Source | S vs S | **Silent** — new table isn't in Debezium `table.include.list`, publication, or `name_registry`. Every INSERT to it is skipped; no export or import error is ever raised. | Run a **separate migration** for just the new table in its own `export-dir`. The main migration keeps running; no downtime, no data loss. *Caveat: CDC apply disables FK enforcement (`session_replication_role = replica`), so mid-migration inserts are not rejected for FK reasons — but voyager does **not** sync FK DDL mid-run. If the new table references tables already replicated by the main job, **define the matching FK on Target and verify referential parity before cutover** (when normal sessions enforce constraints again).* | C |
| 6 | Table | **Create new table** (no PK) | Source | S vs S | **Silent at runtime** (same as #5) if the heap stays **outside** the live export table list — DDL alone does not register it. The no-PK guardrail (`reportUnsupportedTablesForLiveMigration` in `exportData.go`) intersects `finalTableList` with `GetNonPKTables()` — it does **not** scan every relation on restart. **`ErrExit`** only when this table is **included** in live export (CLI / pattern, MSR / config surgery, etc.) **without** a PK. | Add a **PRIMARY KEY** on Source **before** putting the table on the live migration list or starting a supplemental migration for it; or keep it **excluded** from live CDC and sync out-of-band. | D |

#### 2.2b  Source-side drift crashing / stalling import (fix: DDL on Target + resume)

The Source (PG) schema changed and the Target (YB) no longer accepts the incoming CDC stream. **Export keeps working**; failures land on **import**. Fix is to make Target compatible with the new Source schema, then resume.

| # | Object | Drift Event | Drift DB | Detection | Consequence | Workaround | Ref |
|---|--------|------------|----------|-----------|-------------|------------|-----|
| 7 | Table | **Detach partition** | Source | S vs S | **Silent** while Target still has the partition — `SourceRenameTablesMap` rewrites leaf → root; Target routes correctly. **Silent → stuck** once Target also detaches: import hits `23514` (`no partition of relation "part_t" found for row`) and **resume cannot recover** — the same batch replays forever. | `ALTER TABLE public.part_t ATTACH PARTITION public.part_t_p2 …` on **Target**; resume import — the stuck batch drains on the next retry. Leaves Target structurally re-attached (matching the pipeline's rewrite-to-root assumption); Source can stay detached. *Observed to work in the manual-log Scenario T test.* | T |
| 8 | Column | **Add column** | Source | S vs S, S vs T | **Export OK.** **Import panics** on first event carrying the new column — `QuoteAttributeName` fails because the column is absent from Target's catalog. | `ALTER TABLE … ADD COLUMN` on Target with a compatible type, resume. | A |
| 9 | Column | **Rename column** | Source | S vs S, S vs T | **Export OK.** **Import panics** on first event with the renamed field (not in Target catalog). | `ALTER TABLE … RENAME COLUMN` on Target to match, resume. | F |
| 10 | Column | **Incompatible type change** (e.g. INT→NUMERIC, NUMERIC→TEXT) | Source | S vs S, S vs T | **Export OK.** **Import fails** (`22P02`) when a value that no longer fits Target's old type arrives. | `ALTER TABLE … ALTER COLUMN TYPE` on Target (use `USING` if needed), resume. | G, H |
| 11 | Column | **Drop NOT NULL column** (Target has NOT NULL, no DEFAULT) | Source | S vs T | **Export OK.** **Import fails** (`23502`) — CDC omits the dropped column → NULL inserted into Target's NOT NULL column. | On Target, either drop the column, drop the NOT NULL, or add a DEFAULT; then resume. | L |
| 12 | Enum | **Add enum value** | Source | S vs S, S vs T | **Export OK.** **Import fails** (`22P02`) on first event using the new label — Target's enum type doesn't know it. | `ALTER TYPE … ADD VALUE '<new>'` on Target, resume. | N |
| 13 | PK | **Change PK columns** (e.g. PK(id)→PK(name)) | Source | S vs S, S vs T | **Export OK.** **Import fails** (`42P10`) — voyager issues `ON CONFLICT (name)` but no matching PK/UK exists on Target. | Align PK on Target — drop old PK, add the new one matching Source; resume. | O |
| 14 | Constraint | **Relax NOT NULL** (Target still strict) | Source | S vs T | **Export OK.** **Import fails** (`23502`) when Source sends NULL for a column Target still marks NOT NULL. | Drop NOT NULL on Target (or add a DEFAULT), resume. | S |

#### 2.2c  Target-side drift crashing import (fix: revert DDL on Target + resume)

Uncoordinated DDL ran on the Target (YB) during Forward migration; the incoming CDC stream from Source now violates Target's catalog. **Export keeps working**; failures land on **import**. Target-side DDL during Forward is usually **tampering / operator error** — the fix is to revert it on Target.

> If the Target-side DDL was **intentional** (e.g. the Target should be stricter than Source post-cutover), the correct action is to apply the same DDL on Source too so both sides match. For renames this converts the scenario into a P0 (#1) requiring restart — see the note on row 22.

| # | Object | Drift Event | Drift DB | Detection | Consequence | Workaround | Ref |
|---|--------|------------|----------|-----------|-------------|------------|-----|
| 15 | Constraint | **Add NOT NULL** (no DEFAULT, Source sends NULLs) | Target | T vs T, S vs T | **Export OK.** **Import fails** (`23502`) on first row with NULL for the new NOT NULL column. | Drop NOT NULL on Target (or add DEFAULT), resume. | R |
| 16 | Column | **Add column** (Target-only) | Target | T vs T | **Export OK.** First resume after the ALTER fails — Debezium's cached schema JSON is stale for the Target catalog. | Quit + resume a second time — schema auto-refreshes, no DDL needed. | B |
| 17 | Constraint | **Add CHECK constraint** (Source data can violate) | Target | T vs T, S vs T | **Export OK.** **Import fails** (`23514`) when a Source row that would violate the CHECK arrives. | Drop the CHECK on Target, resume. | Extrap. from R |
| 18 | Constraint | **Add / tighten UNIQUE constraint** | Target | T vs T, S vs T | **Export OK.** **Import fails** — `23505` (duplicate) if a Source row collides, or `42P10` if voyager's `ON CONFLICT` target no longer matches. | Drop or loosen the UK on Target, resume. | Extrap. from O, R |
| 19 | Column | **Drop column** (Source still sends it) | Target | T vs T, S vs T | **Export OK.** **Import fails** — INSERT references a column that no longer exists in Target. | Re-add the column on Target, resume. | Extrap. from A (reversed) |
| 20 | Column | **Incompatible type change** | Target | T vs T, S vs T | **Export OK.** **Import fails** (`22P02`) when a Source value no longer fits the new Target type. | Revert the type on Target, resume. | Extrap. from G (reversed) |
| 21 | Column | **Rename column** (Source still sends old name) | Target | T vs T, S vs T | **Export OK.** **Import fails** — INSERT uses the old column name, absent from Target catalog. | Revert rename on Target, resume. | Extrap. from F (reversed) |
| 22 | Table | **Rename table** | Target | T vs T, S vs T | **Export OK.** **Import fails** (`42P01`) — INSERT targets the old table name. | Revert rename on Target, resume. *Note: if the rename was intentional, do **not** mirror it on Source — that would escalate this into a P0 (#1) requiring restart.* | Extrap. from E (reversed) |
| 23 | Domain | **Domain CHECK tightened** (Source data can violate) | Target | T vs T, S vs T | **Export OK.** **Import fails** — valid Source value rejected by stricter Target domain (check / domain violation). | Revert the domain change on Target, resume. | Extrap. from N, R |

---

### 2.3  P2 — Silent Divergence / Pre-Cutover Parity

No pipeline crash during migration, but Source and Target silently diverge. The fix is always to **align the other side before cutover** — the DDL that was applied on one side is assumed to stand.

| # | Object | Drift Event | Drift DB | Detection | Consequence | Workaround | Ref |
|---|--------|------------|----------|-----------|-------------|------------|-----|
| 24 | Column | **Default value change** | Source | S vs S, S vs T | **No crash** — CDC carries explicit values. **Post-cutover**: INSERTs omitting the column produce different defaults on each side. | Apply the new default on Target before cutover. | Q |
| 25 | Trigger | **Trigger changed / added** | Target | T vs T | **No crash** during migration — triggers disabled by `session_replication_role = replica` at apply time. **Post-cutover**: trigger logic diverges. | Align triggers on both sides before cutover (decide whether the Target trigger should also exist on Source, or be dropped). | Extrap. |
| 26 | Column | **Compatible type widening** (e.g. INT→BIGINT, VARCHAR(50)→VARCHAR(200)) | Source | S vs S, S vs T | **No crash** — values still fit Target's narrower type. **Post-cutover**: column metadata diverges; future writes from the app may exceed Target's narrower type. | Apply the widened type on Target before cutover. | Extrap. from G |
| 27 | Column | **Nullability relaxed on both sides independently** | Both | S vs T | **No crash** — both sides accept NULLs. Hidden semantic divergence if only one side was meant to be nullable. | Verify parity and align nullability on both sides before cutover. | — |
| 28 | Column | **Drop nullable column** (no NOT NULL on Target) | Source | S vs S | **No crash** — CDC omits the dropped column; Target ignores it. **Post-cutover**: Target still has a column Source no longer has. | Drop the column on Target before cutover. | J |
| 29 | Column | **Drop NOT NULL column** (Target has DEFAULT) | Source | S vs S | **No crash** — CDC omits the column; Target fills DEFAULT. **Post-cutover**: same structural drift as #28. | Drop the column on Target before cutover. | K |
| 30 | Constraint | **Foreign key change** | Either | S vs S or T vs T | **No crash** during CDC — FKs are disabled at apply time (`session_replication_role = replica`). Voyager does not sync FK DDL mid-run. **Post-cutover**: stricter-side FK will reject valid writes from the other side. | Align FK definitions on both sides before re-enabling FKs post-cutover. | — |
| 31 | Sequence | **Sequence properties changed** (increment, min, max, cycle) | Either | S vs S or T vs T | **No crash** — voyager syncs sequence **values** at end of migration, **not** DDL properties. **Post-cutover**: divergent INCREMENT / CYCLE → divergent generated IDs. | Apply the sequence DDL change on the other side before cutover. | — |
| 32 | View | **View definition changed** | Either | S vs S or T vs T | **No crash** — CDC covers base tables only, not views. **Post-cutover**: any query through the view diverges between the two sides. | Re-create the view on the other side to match before cutover. | — |
| 33 | MView | **Materialized view changed** | Either | S vs S or T vs T | **No crash** — MVs aren't part of CDC. **Post-cutover**: stale or missing MV on the other side. | Align MV DDL on the other side and `REFRESH` before cutover. | — |
| 34 | Function | **Function / procedure changed** | Either | S vs S or T vs T | **No crash** — function bodies aren't part of CDC. **Post-cutover**: if the function is used in triggers / defaults / views, behavior diverges. | Apply the same function definition on the other side before cutover. | — |
| 35 | Policy | **RLS policy changed** | Either | S vs S or T vs T | **No crash** — policies aren't part of CDC. **Post-cutover**: access control diverges between sides. | Apply the policy change on the other side before cutover. | — |
| 36 | Index | **Index added / dropped** | Target | T vs T | **No correctness impact.** May affect import throughput (dropping non-critical indexes can speed imports). | Advisory only — no action required during migration; ensure required indexes exist on Target before cutover for query performance. | — |

---

## 3  Detection Coverage Matrix

| Detection Type | P0 (#1–4) | P1 §2.2a (#5, #6) | P1 §2.2b (#7–14) | P1 §2.2c (#15–23) | P2 (#24–36) |
|---------------|-----------|-------------------|-------------------|-------------------|-------------|
| **S vs S** (Source: Live vs Stored) | **All 4** | **Both** | #7–14 | — | #24, #26, #28–31, #32–35 |
| **T vs T** (Target: Live vs Stored) | — | — | — | **All 9** | #25, #30–36 |
| **S vs T** (Parity) | — | — | #8–14 (not #7) | #15, #17–23 | #24, #26, #27 |

**Key takeaways:**

- **S vs S catches every P0 and every P1 Source-side scenario** (#1–14). This is the highest-value single detection.
- **T vs T catches all Target-side drift** (#15–23) — useful during Forward, and critical during fall-forward / fall-back (see §4.2).
- **S vs T** catches most P1 scenarios that show up as Source/Target mismatches, but **cannot** catch P0 (those are voyager-internal-metadata issues, not just schema mismatches) nor the silent new-object §2.2a cases (Target simply doesn't have the new object yet, which on its own isn't an error). Its value is as the most actionable diagnostic: messages like *"Source column X is NUMERIC but Target column X is INTEGER."*
- Implementing **S vs S + T vs T** covers all detectable scenarios. Adding **S vs T** provides defence-in-depth and the best user-facing messages.

---

## 4  When Detection Matters Most

### 4.1  Forward migration (the main case)

Developers are actively working on the **Source (PostgreSQL)**. This is the most likely source of drift during the live migration window.

| Realistic threat | What drifts | Detection that catches it |
|-----------------|-----------|--------------------------|
| Developers change Source schema mid-migration | All P0 (#1–4) and P1 Source-side (#5–14) | **S vs S** |
| Someone runs unplanned DDL on Target | P1 Target-side (#15–23) | **T vs T** |
| Post-cutover divergence risk | P2 (#24–36) | **S vs T** (pre-cutover gate) |

### 4.2  Fall-forward / Fall-back phase

After cutover, the application is live on **YugabyteDB (Target)**. Developers are now actively working on the Target. During fall-forward (stream YB → Source-replica) or fall-back (stream YB → Source), the Target is now the export side. This means:

- Every P0 scenario (rename table, drop table, add partition, drop PK) can now happen **on the Target**.
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
3. **No guidance** — for P0 scenarios (rename table, drop table, add partition, drop PK), the user is stuck; the only way forward is to restart the migration, but the error message doesn't say that. For P1 silent drift (new table, new table without PK), the user doesn't even know anything went wrong. For P1 crashes (column added on Source, new enum value, Target-side tampering), the right fix is a single DDL on the other side — but again, the error message doesn't tell the user that.
4. **No early warning** — the drift may have happened hours ago, but the user only discovers it when a problematic row finally arrives. For P2 scenarios (default changes, compatible type widening), the user may never discover the divergence until after cutover.

### 5.2  What the feature does

Schema drift detection **proactively compares live database schemas against known baselines** and reports categorized findings to the user **before** (or immediately when) something breaks. It answers three questions:

- **What changed?** — e.g., "Column `score` on Source `subject_t` changed from `INTEGER` to `NUMERIC(10,2)` since export started."
- **How bad is it?** — P0 (restart required), P1 (recoverable without restart — separate migration or DDL + resume), or P2 (align before cutover).
- **What should I do?** — Concrete remediation: "Run `ALTER TABLE public.subject_t ALTER COLUMN score TYPE NUMERIC(10,2) …` on the Target, then resume." Or: "Run a separate `yb-voyager` migration for the new table `public.reward_t` in its own `export-dir`." Or: "Restart the migration with the renamed table `public.subject_v2` in the initial table list."

### 5.3  What the user sees

A categorized report, something like:

```
Schema Drift Report (Forward migration, Source: PostgreSQL, Target: YugabyteDB)
Generated: 2026-04-16 14:32:00

CRITICAL (restart required):
  [P0] Source table "public.part_t" has a new partition "part_t_p3"
       not in the export capture set. Rows to this partition are NOT
       being exported. Fix: restart migration with the partition present
       from the start.

HIGH (recoverable without restart):
  [P1] Source has a new table "public.reward_t" (PK: id) not in the
       export capture set. INSERTs to this table are being silently skipped.
       Fix: run a separate yb-voyager migration for this table in its own
            export-dir (main migration keeps running).

  [P1] Source "public.subject_t.score" type changed: INTEGER → NUMERIC(10,2)
       Target still has INTEGER. Import will fail on non-integer values.
       Fix: ALTER TABLE public.subject_t ALTER COLUMN score TYPE NUMERIC(10,2)
            USING score::numeric;   (run on Target, then resume)

  [P1] Source "public.subject_phase_t" has new enum value "n_gamma"
       not present on Target.
       Fix: ALTER TYPE public.subject_phase_t ADD VALUE 'n_gamma';
            (run on Target, then resume)

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

1. **S vs S is the highest-value detection** — it catches all P0 restart-required scenarios and all Source-side P1 scenarios (silent new-object drift plus import-crashing Source changes). T vs T catches Target-side drift during Forward and becomes equally critical during fall-forward / fall-back. Both are essential for full-lifecycle coverage.

2. **The baselines already exist** — `name_registry.json` stores table names, `data/schemas/*_schema.json` stores column names and types, `meta.db` stores table lists and unique-key columns. The feature needs to query the live DB catalog and diff against these artifacts.

3. **Role-agnostic implementation** — during Forward, Source is the export side. During FF/FB, Target is. The detection code should be written once and parameterized rather than hard-coded to a particular physical DB.

4. **Actionable output over raw diffs** — the value is not in listing every schema difference. It is in classifying the impact (P0/P1/P2) and giving the user the exact remediation — DDL to run, separate migration to launch, or restart to plan. The catalog in §2 provides the classification rules.

5. **P0 detection should be blocking** — if we detect a P0 (rename table, drop table, add partition, drop PK), we should surface it as an error the user cannot ignore; the pipeline is in a state only a restart can fix.

6. **P1 detection should be actionable, not blocking** — P1 scenarios are recoverable with a targeted action while the migration is running. Surface them clearly with the remediation command, but don't force a halt — especially for silent §2.2a cases (new table) where catching the drift early lets the user start a separate migration before too many events are lost.
