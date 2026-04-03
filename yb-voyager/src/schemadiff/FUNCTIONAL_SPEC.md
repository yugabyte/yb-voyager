# Functional Spec: Schema Drift Detection

**Author:** Shivansh
**Status:** Draft — pending team review
**Date:** 2026-03-30

---

## 1 Introduction

During live migration, schema changes on the source database cause Voyager
failures that are difficult to diagnose. A customer might add a column, change a
data type, or have `pg_partman` create a new partition — and Voyager's CDC
pipeline breaks with cryptic errors. Tracing the root cause through log files
requires manually comparing schemas, which is slow and error-prone.

**Schema drift detection** gives users a single command to identify exactly what
changed, where, and what to do about it. It also runs automatically during live
migration to warn about drift before it causes failures.

The feature has two parts:

1. **`yb-voyager schema diff`** — a new CLI command that compares source and
   target schemas and produces a diagnostic report.
2. **Proactive drift detection** — a background check during live migration
   (`export data` / `import data`) that warns the user in the terminal when
   schema drift is detected.

**How it works:** Schema comparison is performed by querying `pg_catalog` views
directly on live database connections. This approach works identically on
PostgreSQL and YugabyteDB, requires no external tools (no `pg_dump`), and can
run in-process wherever a DB connection exists. See the
[Design & Architecture](ARCHITECTURE.md) spec for full details on catalog
queries, object coverage, and ignore rules.

---

## 2 Requirements

1. Detect source drift, target drift, and source-vs-target discrepancies.
2. Suppress expected differences (PG-vs-YB behavior, version-aware YB
   limitations, objects deferred until `finalize-schema-post-data-import`).
3. Classify each difference by impact level (LEVEL_3 / LEVEL_2 / LEVEL_1).
4. Proactively warn during live migration when schema drift is detected.
5. Provide an on-demand `schema diff` command with a saved report.
6. Support PostgreSQL as source. Oracle and MySQL are deferred.

---

## 3 Comparisons

Three comparisons power the entire feature. Each has a specific purpose.

### 3.1 C1 — Latest source snapshot vs live source

| | |
|---|---|
| **Compares** | Latest saved source snapshot vs current live source |
| **DB types** | PG vs PG (same type) |
| **Ignore rules** | None — any difference is real drift |
| **Purpose** | "Did the source change since our last checkpoint?" |
| **Used in** | `export data` pre-start gate, periodic hooks, `schema diff` default mode (if snapshots exist) |

### 3.2 C2 — Latest target snapshot vs live target

| | |
|---|---|
| **Compares** | Latest saved target snapshot vs current live target |
| **DB types** | YB vs YB (same type) |
| **Ignore rules** | None — any difference is unexpected |
| **Purpose** | "Did the target change since our last checkpoint?" |
| **Used in** | `import data` pre-start gate, periodic hooks, `schema diff` default mode (if snapshots exist) |

The target schema does not change during `import data`. Indexes and constraints
are created later by `finalize-schema-post-data-import`, which runs after
cutover. So any difference detected during `import data` is genuinely unexpected
— someone or something outside Voyager modified the target.

### 3.3 C3 — Live source vs live target

| | |
|---|---|
| **Compares** | Current live source vs current live target |
| **DB types** | PG vs YB (cross-database) |
| **Ignore rules** | Yes — PG-vs-YB expected differences suppressed |
| **Purpose** | "What's different between my two databases right now?" |
| **Used in** | `schema diff` command (always — primary section) |

This is the most directly useful comparison. It works without any baseline — a user who upgrades Voyager
mid-migration can immediately run this.

### 3.4 Summary

```
HOOKS (proactive, automatic during live migration):
  export data  →  C1: latest source snapshot vs live source  (PG vs PG, no ignore rules)
  import data  →  C2: latest target snapshot vs live target  (YB vs YB, no ignore rules)

COMMAND — default (no --compare flag):
  schema diff  →  C3: live source vs live target             (PG vs YB, ignore rules)
                  C1: latest source snapshot vs live source   (if snapshots exist)
                  C2: latest target snapshot vs live target   (if snapshots exist)

COMMAND — explicit (--compare <left>,<right>):
  schema diff  →  Runs only the requested comparison.
                  Ignore rules applied when comparing PG vs YB.
```

<!-- ### 3.5 Why not WAL-based detection?

PostgreSQL's logical decoding (`pgoutput`) only captures DML (INSERT, UPDATE,
DELETE). DDL commands are not part of the WAL logical replication stream. The
WAL contains physical page-level changes to catalog tables, but extracting DDL
from those is impractical. Oracle and MySQL logs do contain DDL, but leveraging
that would require changes to Voyager's Debezium plugin — a larger future
effort. Catalog queries are the right approach: they work across all source
databases, require no source modifications, and are read-only. -->

---

## 4 Surface Area & Usage

### 4.1 New command: `yb-voyager schema diff`

#### When to use

- **Migration broke** — investigate which schema differences caused the failure.
- **After a drift warning** — the live migration hook told you drift was
  detected; this command shows the full details.
- **Before cutover** — final sanity check that source and target are in sync.
- **After `export schema` + `import schema`** — verify the target was set up
  correctly.
- **Compare two saved snapshots** — inspect what changed between two points in
  the migration timeline.

#### Syntax

```
yb-voyager schema diff --export-dir <dir> [flags]
```

#### Flags

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--export-dir` / `-e` | Yes | — | Migration workspace directory. Provides saved snapshots, report output location, and deferred-object metadata. |
| `--source-db-host` | Conditional | — | Source database host. Required when comparison involves `live-source`. |
| `--source-db-port` | Conditional | — | Source database port. |
| `--source-db-user` | Conditional | — | Source database user. |
| `--source-db-name` | Conditional | — | Source database name. |
| `--source-db-password` | Conditional | — | Source database password. |
| `--source-db-schema` | Conditional | — | Comma-separated schema list. |
| `--target-db-host` | Conditional | — | Target database host. Required when comparison involves `live-target`. |
| `--target-db-port` | Conditional | — | Target database port. |
| `--target-db-user` | Conditional | — | Target database user. |
| `--target-db-name` | Conditional | — | Target database name. |
| `--target-db-password` | Conditional | — | Target database password. |
| `--config-file` / `-c` | No | — | Path to YAML config file. Contains all connection details and flags. |
| `--compare` | No | — | Explicit comparison: `<left>,<right>`. See below. |
| `--list-snapshots` | No | — | List available saved snapshots and exit. |
| `--output-format` | No | `html,json` | Report format(s): `html`, `json`, `txt`. Comma-separated. |

Connection details are provided via explicit flags or a config file — the same
config file used for other Voyager commands (`export schema`, `import data`,
etc.). Since the config file already contains host, port, user, password, and
schema information for both source and target, the typical invocation is just:

```
yb-voyager schema diff --export-dir /path/to/export --config-file voyager-config.yaml
```

If any required connection parameter is missing for a comparison that needs it,
the command errors out with a clear message indicating which parameter is
missing.

#### `--compare` flag

Controls which comparison to run:

| `--compare` value | What runs | Connections needed |
|-------------------|-----------|-------------------|
| *(not provided)* | **Full diagnostic:** C3 + C1 (if source snapshots exist) + C2 (if target snapshots exist) | Both source + target |
| `live-source,live-target` | C3 only: live source vs live target | Both source + target |
| `live-source,<snapshot>` | Live source vs a saved snapshot | Source only |
| `<snapshot>,live-target` | Saved snapshot vs live target | Target only |
| `<snapshot1>,<snapshot2>` | Two saved snapshots (fully offline) | Neither |

When `--compare` is not provided (the default), the command produces the full
three-section report: C3 (live source vs live target), plus C1 and C2 using the
latest available snapshots for each side. This is the most useful diagnostic
mode.

When `--compare` is explicitly provided — even if the value is
`live-source,live-target` — only that single comparison runs.

Ignore rules are applied automatically when one side is PG and the other is YB.
Same-DB comparisons (PG vs PG, YB vs YB) apply no ignore rules.

#### `--list-snapshots` flag

Lists all saved snapshots in the export directory and exits:

```
$ yb-voyager schema diff --export-dir /path/to/export --list-snapshots

Available snapshots:
  source_export_schema_2026-03-25T10:00:00      (export schema)
  source_export_data_start_2026-03-26T14:00:00   (export data start)
  source_export_data_start_2026-03-27T09:00:00   (export data resume)
  target_import_schema_2026-03-25T12:00:00      (import schema)
  target_import_data_start_2026-03-26T15:00:00   (import data start)
```

These names can be used in `--compare`:

```
yb-voyager schema diff --export-dir /path/to/export \
  --compare source_export_data_start_2026-03-26T14:00:00,source_export_data_start_2026-03-27T09:00:00
```

#### Examples

Full diagnostic (default — most common usage):

```
yb-voyager schema diff \
  --export-dir /path/to/export \
  --config-file voyager-config.yaml
```

Full diagnostic with explicit flags:

```
yb-voyager schema diff \
  --export-dir /path/to/export \
  --source-db-host pg-source --source-db-port 5432 --source-db-user postgres \
  --source-db-name mydb --source-db-password '***' --source-db-schema public \
  --target-db-host yb-target --target-db-port 5433 --target-db-user yugabyte \
  --target-db-name mydb --target-db-password '***'
```

Compare two saved snapshots (offline, no DB connections needed):

```
yb-voyager schema diff \
  --export-dir /path/to/export \
  --compare source_export_data_start_2026-03-26T14:00:00,source_export_data_start_2026-03-27T09:00:00
```

Compare live source against a saved snapshot:

```
yb-voyager schema diff \
  --export-dir /path/to/export \
  --config-file voyager-config.yaml \
  --compare live-source,target_import_schema_2026-03-25T12:00:00
```

#### Fallback: missing snapshots

When running the default full diagnostic (no `--compare`), if snapshots are
missing for one side, the corresponding baseline comparison is skipped. C3
always runs.

| Missing snapshot | Effect |
|-----------------|--------|
| No source snapshots | C1 skipped. Log: `"Source baseline not found, skipping source change tracking."` |
| No target snapshots | C2 skipped. Log: `"Target baseline not found, skipping target change tracking."` |

---

### 4.2 Command output: terminal summary

The command always prints a concise summary to the terminal. The summary tells
the user what was checked, what was found, and where to look.

#### Case 1: Full diagnostic — differences found

```
$ yb-voyager schema diff \
    --export-dir /path/to/export \
    --config-file voyager-config.yaml \
    --output-format html,json

Schema Diff Report
==================

Comparing source (postgresql://source-host:5432/mydb)
      vs target (yugabytedb://target-host:5433/mydb)
Schemas: public

Section 1 — Source vs Target: 3 differences found
  1 LEVEL_3, 1 LEVEL_2, 1 LEVEL_1
  (3 expected PG-vs-YB differences suppressed)
  (12 objects pending 'finalize-schema-post-data-import')

Section 2 — Source changes since last snapshot: 2 changes detected
  1 LEVEL_3, 1 LEVEL_2

Section 3 — Target changes since last snapshot: no unexpected changes

Report saved to:
  /path/to/export/reports/schema_diff_report.html
  /path/to/export/reports/schema_diff_report.json
```

#### Case 2: Full diagnostic — everything in sync

```
$ yb-voyager schema diff \
    --export-dir /path/to/export \
    --config-file voyager-config.yaml \
    --output-format html,json

Schema Diff Report
==================

Comparing source (postgresql://source-host:5432/mydb)
      vs target (yugabytedb://target-host:5433/mydb)
Schemas: public

Section 1 — Source vs Target: no differences
  (3 expected PG-vs-YB differences suppressed)
  (no pending objects — 'finalize-schema-post-data-import' completed)

Section 2 — Source changes since last snapshot: no changes

Section 3 — Target changes since last snapshot: no changes

All schemas are in sync.

Report saved to:
  /path/to/export/reports/schema_diff_report.html
  /path/to/export/reports/schema_diff_report.json
```

#### Case 3: Full diagnostic — no snapshots (upgraded mid-migration)

```
$ yb-voyager schema diff \
    --export-dir /path/to/export \
    --config-file voyager-config.yaml \
    --output-format html,json

Schema Diff Report
==================

Comparing source (postgresql://source-host:5432/mydb)
      vs target (yugabytedb://target-host:5433/mydb)
Schemas: public

Section 1 — Source vs Target: 2 differences found
  1 LEVEL_3, 1 LEVEL_2
  (3 expected PG-vs-YB differences suppressed)

Note: Source snapshots not available — skipping source change tracking.
Note: Target snapshots not available — skipping target change tracking.

Report saved to:
  /path/to/export/reports/schema_diff_report.html
  /path/to/export/reports/schema_diff_report.json
```

#### Case 4: Explicit comparison (single pair)

```
$ yb-voyager schema diff \
    --export-dir /path/to/export \
    --compare source_export_data_start_2026-03-26T14:00:00,source_export_data_start_2026-03-27T09:00:00

Schema Diff Report
==================

Comparing source_export_data_start_2026-03-26T14:00:00
      vs source_export_data_start_2026-03-27T09:00:00

1 change detected:
  1 LEVEL_2

Report saved to:
  /path/to/export/reports/schema_diff_report.html
  /path/to/export/reports/schema_diff_report.json
```

---

### 4.3 Command output: full report

The full report is saved to `{export-dir}/reports/schema_diff_report.{format}`.
It contains up to three sections depending on what baselines are available.

#### Section 1 — Source vs Target

This section always runs. It connects to both databases, takes a live snapshot
of each, and compares them. Expected PG-vs-YB differences are suppressed.

```
═══════════════════════════════════════════════════════════════
  SECTION 1: SOURCE vs TARGET
  Comparing live source against live target
═══════════════════════════════════════════════════════════════
  Source: postgresql://source-host:5432/mydb
  Target: yugabytedb://target-host:5433/mydb
  Schemas: public
  Checked at: 2026-03-30 14:23:05 UTC
═══════════════════════════════════════════════════════════════

LEVEL_3 — 1 issue:

  ● [COLUMN TYPE CHANGED] public.orders.amount
    Source: numeric(12,4)
    Target: numeric(10,2)

LEVEL_2 — 1 issue:

  ● [TABLE MISSING ON TARGET] public.events_2026_04

LEVEL_1 — 1 issue:

  ● [DEFAULT VALUE CHANGED] public.users.status
    Source: 'active'
    Target: 'pending'

─────────────────────────────────────────────────────────────
SUPPRESSED — 3 expected PG-vs-YB differences (not issues):

  ○ [INDEX METHOD] public.orders_pkey
    Source: btree | Target: lsm
    Reason: YugabyteDB uses LSM storage for all btree indexes.

  ○ [INDEX METHOD] public.idx_orders_date
    Source: btree | Target: lsm
    Reason: YugabyteDB uses LSM storage for all btree indexes.

  ○ [INDEX DROPPED] public.idx_orders_redundant
    Reason: Removed by Voyager as redundant (covered by a stronger index).
            Original saved in redundant_indexes.sql.

─────────────────────────────────────────────────────────────
PENDING — 12 objects deferred until 'finalize-schema-post-data-import':

  ○ [INDEX MISSING ON TARGET] public.idx_orders_customer_id
  ○ [INDEX MISSING ON TARGET] public.idx_orders_created_at
  ○ [INDEX MISSING ON TARGET] public.idx_users_email
  ... (9 more)

  These will be created when you run:
    yb-voyager finalize-schema-post-data-import --export-dir /path/to/export
```

Note: The PENDING block only appears if `finalize-schema-post-data-import` has
not yet been run. After it completes, any missing index is reported as a real
difference.

#### Section 2 — Source changes since last snapshot

This section only appears in the default full diagnostic (no `--compare` flag)
and only if source snapshots exist. It compares the latest saved source snapshot
against the live source. Same-DB comparison (PG vs PG) — no ignore rules. Any
difference means the source was modified since the last checkpoint.

```
═══════════════════════════════════════════════════════════════
  SECTION 2: SOURCE CHANGES
  What changed on the source since last snapshot
═══════════════════════════════════════════════════════════════
  Snapshot: source_export_data_start_2026-03-26T14:00:00
  Live source checked: 2026-03-30 14:23:05 UTC
═══════════════════════════════════════════════════════════════

2 changes detected:

  LEVEL_3:
  ● [COLUMN TYPE CHANGED] public.orders.amount
    Was:  numeric(10,2)
    Now:  numeric(12,4)

  LEVEL_2:
  ● [TABLE ADDED] public.events_2026_04
```

#### Section 3 — Target changes since last snapshot

This section only appears in the default full diagnostic and only if target
snapshots exist. It compares the latest saved target snapshot against the live
target. Same-DB comparison (YB vs YB) — no ignore rules. Any difference means
the target was modified unexpectedly.

When no changes are found:

```
═══════════════════════════════════════════════════════════════
  SECTION 3: TARGET CHANGES
  What changed on the target since last snapshot
═══════════════════════════════════════════════════════════════
  Snapshot: target_import_data_start_2026-03-26T15:00:00
  Live target checked: 2026-03-30 14:23:05 UTC
═══════════════════════════════════════════════════════════════

No unexpected changes. The target schema matches the snapshot.
```

When changes are found:

```
═══════════════════════════════════════════════════════════════
  SECTION 3: TARGET CHANGES
  What changed on the target since last snapshot
═══════════════════════════════════════════════════════════════
  Snapshot: target_import_data_start_2026-03-26T15:00:00
  Live target checked: 2026-03-30 14:23:05 UTC
═══════════════════════════════════════════════════════════════

1 change detected:

  LEVEL_2:
  ● [INDEX DROPPED] public.idx_users_email
```

---

### 4.4 Proactive drift detection during live migration

During live migration, Voyager detects schema drift at three points: before a
phase starts (or resumes), periodically during the phase, and as a best-effort
check on exit/crash.

#### 4.4.1 Pre-start gate — `export data`

When `export data` starts (fresh start or resume after failure or `start clean`),
Voyager:

1. Takes a new source snapshot (S2 on first start; S3, S4, ... on subsequent
   resumes).
2. Prints: `Schema snapshot taken: source_export_data_start_<timestamp>`
3. Compares the new snapshot against the latest **previous** source snapshot
   (S1 on first start; S2 on first resume; etc.).
4. If differences are found, prints a summary and prompts:

```
⚠ SOURCE SCHEMA CHANGED since last snapshot (source_export_schema_2026-03-25T10:00:00)

  LEVEL_3: public.orders.amount — column type changed (numeric(10,2) → numeric(12,4))
  LEVEL_2: public.events_2026_04 — new table/partition added

  These changes may cause export failures or data inconsistencies.

  To see the full diff between source and target, run:
    yb-voyager schema diff --export-dir /path/to/export

  Continue with export data? [y/N]
```

5. If the user selects `N`, the command exits. If `y`, export proceeds with the
   new snapshot as the reference for periodic checks.
6. If no differences are found, export proceeds normally. A log line confirms:
   `No source schema changes detected since last snapshot.`

#### 4.4.2 Pre-start gate — `import data`

Same logic as §4.4.1 but for the target side:

1. Takes a new target snapshot (T2 on first start; T3, T4, ... on resumes).
2. Prints: `Schema snapshot taken: target_import_data_start_<timestamp>`
3. Compares against the latest previous target snapshot.
4. If differences are found:

```
⚠ TARGET SCHEMA CHANGED since last snapshot (target_import_schema_2026-03-25T12:00:00)
  This was not done by Voyager and may cause import failures.

  LEVEL_2: public.idx_users_email — index dropped

  To see the full diff between source and target, run:
    yb-voyager schema diff --export-dir /path/to/export

  Continue with import data? [y/N]
```

5. `N` exits. `y` proceeds with the new snapshot as the periodic check
   reference.

#### 4.4.3 Periodic drift checks during `export data` (C1)

Once `export data` is running, a background goroutine takes a live source
snapshot every N minutes (default: 10) and compares it against the **latest
saved source snapshot** (S2, S3, etc.).

**When drift is detected:**

```
| Metric                                   |                        Value |
| ---------------------------------------- | ---------------------------- |
| Total Exported Events                    |                       45,231 |
| Export Rate (last 3 min)                 |                  1,204 rows/s |
| Total Exported Events (this run)         |                       12,405 |
| Export Elapsed Time (this run)           |                   00:34:12   |
| ---------------------------------------- | ---------------------------- |

⚠ SOURCE SCHEMA CHANGED (detected 2026-03-30 14:23:05)
  The source database schema has changed since last snapshot.
  This may cause export failures or data inconsistencies.

  LEVEL_3: public.orders.amount — column type changed (numeric(10,2) → numeric(12,4))
  LEVEL_2: public.events_2026_04 — new table/partition added

  To see the full diff between source and target, run:
    yb-voyager schema diff --export-dir /path/to/export
```

**When no drift is detected:** Nothing is printed. The check runs silently.

#### 4.4.4 Periodic drift checks during `import data` (C2)

Same structure as §4.4.3, comparing live target against the latest saved target
snapshot.

**When drift is detected:**

```
| Metric                                   |                        Value |
| ---------------------------------------- | ---------------------------- |
| Total Imported Events                    |                       38,917 |
| Import Rate (last 3 min)                |                    982 rows/s |
| Remaining Events                         |                        6,314 |
| ---------------------------------------- | ---------------------------- |

⚠ TARGET SCHEMA CHANGED (detected 2026-03-30 14:23:05)
  The target database schema has been modified since last snapshot.
  This was not done by Voyager and may cause import failures.

  LEVEL_2: public.idx_users_email — index dropped

  To see the full diff between source and target, run:
    yb-voyager schema diff --export-dir /path/to/export
```

#### 4.4.5 Exit drift check (best-effort)

When `export data` or `import data` exits — whether from a crash, error, or
SIGTERM — Voyager runs a final, best-effort schema drift check before the
process terminates. The result is printed to the terminal so the user can see
it before the output scrolls away or the terminal closes.

This is a quick "did schema drift cause this?" signal. It does not block
shutdown or persist anything to disk.

```
──────────────────────────────────────────────────────
Exit drift check (best-effort):

  Source schema: 2 changes since last snapshot
    LEVEL_3: public.orders.amount — column type changed
    LEVEL_2: public.events_2026_04 — new table/partition added

  For full details, run:
    yb-voyager schema diff --export-dir /path/to/export
──────────────────────────────────────────────────────
```

If no drift is detected, prints: `Exit drift check: no schema changes detected.`

If the check itself fails (e.g., database unreachable), it is silently skipped —
the process is already exiting, so we do not add noise.

#### 4.4.6 When `--disable-pb` is set

If the progress display is disabled, warnings appear as standard log messages:

```
2026/03/30 14:23:05 [WARNING] Source schema changed since last snapshot:
  LEVEL_3: public.orders.amount — column type changed
  LEVEL_2: public.events_2026_04 — new table/partition added
  Run 'yb-voyager schema diff --export-dir /path/to/export' to see the full diff between source and target.
```

```
2026/03/30 14:23:05 [WARNING] Target schema changed since last snapshot (not by Voyager):
  LEVEL_2: public.idx_users_email — index dropped
  Run 'yb-voyager schema diff --export-dir /path/to/export' to see the full diff between source and target.
```

#### 4.4.7 Check frequency

The background check runs every 10 minutes by default.

**Open question:** Should this be configurable via flag or environment variable?

#### 4.4.8 When no snapshots exist

If `export schema` was run before this feature existed, no source snapshots are
available. The `export data` hook skips the pre-start gate and periodic drift
checks, logging once at startup:

```
2026/03/30 10:00:00 [INFO] Source schema snapshots not found. Schema drift
detection during export is disabled. Run 'yb-voyager schema diff' to compare
source and target schemas on-demand.
```

Similarly, if no target snapshots exist, the `import data` hook logs:

```
2026/03/30 10:00:00 [INFO] Target schema snapshots not found. Schema drift
detection during import is disabled. Run 'yb-voyager schema diff' to compare
source and target schemas on-demand.
```

---

### 4.5 Snapshots

Schema snapshots are captured at multiple points in the migration workflow.
Each snapshot is timestamped and stored in the export directory. Voyager always
logs when a snapshot is taken.

#### 4.5.1 Snapshot lifecycle

| When | Snapshot | Purpose |
|------|----------|---------|
| `export schema` completes | Source baseline (S1) | "What did the source look like when we designed the migration?" |
| `import schema` completes | Target baseline (T1) | "What did the target look like after import?" |
| `export data` starts | Source pre-export (S2) | Compared against S1 as a pre-start gate (see §4.4.1). Periodic drift checks compare against this. |
| `export data` resumes after failure / `start clean` | Source re-baseline (S3, S4, ...) | Compared against previous snapshot as a gate. Periodic drift checks now compare against this new snapshot. |
| `import data` starts | Target pre-import (T2) | Compared against T1 as a pre-start gate (see §4.4.2). Periodic drift checks compare against this. |
| `import data` resumes after failure / `start clean` | Target re-baseline (T3, T4, ...) | Compared against previous snapshot as a gate. Periodic drift checks now compare against this new snapshot. |

#### 4.5.2 Storage

Snapshots are accumulated in a dedicated directory:

```
{export-dir}/metainfo/schema/snapshots/
├── source_export_schema_2026-03-25T10:00:00.json
├── source_export_data_start_2026-03-26T14:00:00.json
├── source_export_data_start_2026-03-27T09:00:00.json   (after resume)
├── target_import_schema_2026-03-25T12:00:00.json
├── target_import_data_start_2026-03-26T15:00:00.json
└── target_import_data_start_2026-03-27T10:00:00.json   (after resume)
```

The "latest" snapshot for each side (source / target) is what periodic drift
checks and the default `schema diff` command compare against.

Users do not interact with these files directly under normal usage. Power users
can reference them via `--list-snapshots` and `--compare` (see §4.1).

---

### 4.6 Impact levels

Uses the same impact levels as the migration assessment report for consistency.

| Level | Meaning | Examples |
|-------|---------|---------|
| **LEVEL_3** | Significant impact — migration will fail or produce incorrect data. No simple workaround. | Column dropped, column type changed, table dropped, PK changed |
| **LEVEL_2** | Moderate impact — may cause subtle issues or missed data. | New table/column on source, partition added/dropped, UK changed, trigger changed, enum values changed |
| **LEVEL_1** | Minimal impact — low-risk cosmetic or schema-only difference. | Default value changed, FK changed, index added/dropped, sequence properties changed, view/function definition changed |

### 4.7 Suppressed differences (C3 ignore rules)

When comparing PostgreSQL source against YugabyteDB target (C3), some
differences are expected and not a sign of drift. These are automatically
suppressed and listed in a separate "Suppressed" block for transparency.

There are two categories of suppressed differences:

#### 4.7.1 PG-vs-YB expected differences (always suppressed in C3)

| Difference | Why it's expected |
|-----------|------------------|
| Index storage method (btree → lsm) | YugabyteDB uses LSM trees for all btree indexes. |
| Redundant index missing on target | Voyager removes indexes covered by a stronger index. Originals saved in `redundant_indexes.sql`. |
| Index sort direction (explicit ASC) | Voyager may add explicit ASC for range-shard indexes. |
| Colocation settings | Voyager may add `colocation = false` to sharded tables. |
| Unsupported features missing | GiST indexes, exclusion constraints, etc. that YB does not support. **Version-aware:** many YB limitations are fixed in newer versions (e.g. stored generated columns in 2.25+, unlogged tables in 2024.2+). A missing object is only suppressed if the target YB version does not support it. If the target is new enough, the same missing object is reported as a real difference. |

#### 4.7.2 Objects pending `finalize-schema-post-data-import`

During live migration, Voyager defers creating certain objects on the target
until after data import completes, for performance:

- **Indexes** (all types)
- **NOT VALID constraints** (validated later)
- **Unique indexes**

If `finalize-schema-post-data-import` has NOT been run yet, these objects exist
on the source but are intentionally missing on the target. The C3 comparison
suppresses them and lists them in their own block:

```
PENDING — 24 objects deferred until 'finalize-schema-post-data-import':

  ○ [INDEX MISSING ON TARGET] public.idx_orders_date
  ○ [INDEX MISSING ON TARGET] public.idx_users_email
  ○ [CONSTRAINT MISSING ON TARGET] public.chk_orders_amount (NOT VALID)
  ... (21 more)

  These will be created when you run:
    yb-voyager finalize-schema-post-data-import --export-dir /path/to/export
```

Once `finalize-schema-post-data-import` has been run, this suppression is
disabled. Any missing index or constraint after that point is reported as a real
difference.

**How we know what's deferred:** The migration status in MetaDB tracks whether
`finalize-schema-post-data-import` has been run. The exported SQL files in
`{export-dir}/schema/` contain the full list of indexes and constraints that
Voyager will create during that phase.

**This does NOT affect C2 (target baseline vs live target):** The target
baseline is captured after `import schema`, which also doesn't have these
deferred objects. Both baseline and live target are missing the same objects, so
no diff appears.

#### 4.7.3 Display rules

Suppressed differences are NOT counted in the summary severity totals. They
appear in their own clearly labeled blocks at the end of Section 1, so the user
can verify them if needed but is not overwhelmed by expected differences.

---

## 5 Compatibility

### Installation and packaging

| | Yes / No / N/A | Notes |
|---|---|---|
| Docker | Yes | No new dependencies; uses existing DB drivers. |
| Nightly / Dev builds | Yes | Part of standard `yb-voyager` binary. |
| Airgapped installation | Yes | No external network calls. |
| Yum / APT / Brew | Yes | Part of existing `yb-voyager` package. |

### Performance

| | Yes / No / N/A | Notes |
|---|---|---|
| New perf testing planned | No | Snapshot is a set of `pg_catalog` queries — lightweight. Background drift check uses a separate goroutine and does not block event processing. |

### Testing

| | Yes / No / N/A | Notes |
|---|---|---|
| Offline migration | N/A | `schema diff` command works anytime. Hooks are live-migration only. |
| Live migration | Yes | Background drift detection during `export data` and `import data`. |
| SSL | Yes | Inherits SSL config from connection flags / config file. |
| New tests planned | Yes | Integration tests using testcontainers (PG + YB). Unit tests for diff engine, ignore rules, severity classification. |
| Jenkins pipeline | Yes | For integration test suite. |

### Supportability

| | Yes / No / N/A | Notes |
|---|---|---|
| Assess / Analyze | N/A | Not affected. |
| Command line output | Yes | New `schema diff` command. Background drift warnings in live migration. |

---

## 6 Caveats and unsupported features

- **Report shows facts only.** Each diff shows the object, property, and
  old/new values. Context-aware impact explanations and corrective DDL
  suggestions are not generated.
- **PostgreSQL source only.** Oracle and MySQL are deferred.
- **Does not auto-fix drift.** The feature tells you what changed — you fix it.
- **Background check interval is not configurable.** Fixed at 10 minutes.
- **Exit drift check is best-effort.** If the database is unreachable at exit,
  the check is silently skipped.

---

## 7 References

| Document | Description |
|----------|-------------|
| [Design & Architecture](ARCHITECTURE.md) | System architecture, data flows, catalog queries, object coverage, ignore rules, integration code paths. |
| QA Test Spec | TBD |
