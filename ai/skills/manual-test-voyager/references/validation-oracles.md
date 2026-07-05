# Validation oracles

How to decide, for each scenario, whether the observed behavior is a PASS, a real FAIL (bug), or an EXPECTED-FAIL (graceful negative). "Import data complete" printed to stdout is **not** an oracle — it has been printed while silently losing rows. Always assert against an independent oracle.

Every scenario in the plan carries an **expected-outcome oracle**. Match the observed result to it. A PASS must cite which oracle it satisfied.

## 1. Hang detection

The highest-value oracle for this skill (it's what the anchor bug is).

- Wrap **every** `yb-voyager` command in `timeout <N>` (macOS: `timeout` / `gtimeout` from coreutils; Linux: `timeout`). Exit code **124** = timed out = candidate hang. Set N generously above the expected runtime for the fixture size; a hang is usually an infinite block (pool init, lock wait), not a slow import, so it will blow past any reasonable N.
- For long imports where a global timeout is coarse, add a **log-inactivity watchdog**: if `logs/yb-voyager-import-data.log` has no new lines for `M` seconds while the process is alive, treat as a stall and capture a goroutine dump (`SIGQUIT` the process) before killing.
- When a hang is found, before killing, capture: the last 50 log lines, the process's goroutine stack (`kill -QUIT <pid>` → stderr), and the exact flags. That stack is the bug report.

Anchor repro (must never hang):
```
timeout 240 "$BIN" import data --export-dir "$EXPDIR" $TGT \
  --adaptive-parallelism balanced --adaptive-parallelism-max 1 --disable-pb true --yes
```
Expected oracle: completes; log shows parallelism clamped to the max (e.g. `Using 1-1 parallel jobs (adaptive)`); all rows imported. Exit 124 ⇒ **Critical hang**.

## 2. Crash / error-code oracle

- Capture the exit code and full stderr of each command.
- For **positive** scenarios: non-zero exit ⇒ FAIL. Also scan the log for `panic:`, `goroutine`, `nil pointer`, `runtime error` even on exit 0 (a swallowed panic in a worker).
- For **negative** scenarios: a non-zero exit is *expected* — the test is on the **message quality** (oracle 5), not the failure itself. A negative scenario that *succeeds* (exit 0 when it should have rejected) is a FAIL.

Known guardrails a negative test should trip (from `flag-surface.md`): `--parallel-jobs` + adaptive enabled; `--adaptive-parallelism-max` with adaptive disabled; `--truncate-tables` without `--start-clean`; batch size above the target default; yugabytedb-amp + any adaptive flag. Each must `ErrExit` with a clear message — verify both the rejection and the wording.

## 3. Data-parity oracle (silent data loss / corruption)

Because the fixture source is under your control, query it live and diff against the target — stronger than the migtests hard-coded constants and needs no golden file.

**Row counts, per table** (fast first pass):
```
# same query against source (5490/mtv_src) and target (5491/mtv_tgt)
SELECT relname, n_live_tup FROM pg_stat_user_tables ORDER BY 1;   -- approximate; prefer exact:
SELECT count(*) FROM <table>;                                     -- exact, per table
```
Mismatch ⇒ FAIL (data loss or duplication). For **partitioned** tables, count both the parent and each partition — a partition-routing bug shows as parent-total right but per-partition wrong.

**Content equivalence** (catches wrong values with right counts): per table, a hash of the rows **ordered by primary key**:
```
-- per table, both DBs; compare the single hash value
SELECT md5(string_agg(t::text, ',' ORDER BY <pk_columns>)) FROM <table> t;
```
> **Collation trap (verified false-positive):** do NOT order by the row text (`ORDER BY t::text`). Text sort order is **collation-dependent** and PG (en_US) vs YugabyteDB (often C) sort differently, so `ORDER BY t::text` produces a different concatenation order — hence a different md5 — for *identical* data. Order by the PK (integer/stable) instead. Also prefer canonical numeric formatting (`to_char(n,'FM…0.00')`) over the raw `::text` whole-row cast to avoid representational nuances. When counts + column sums match but a row-text hash differs, suspect this before suspecting data loss.

For big tables, hash a column aggregate instead (cheap proxy): `SELECT count(*), sum(<numeric_col>), min(<col>), max(<col>) FROM <table>`. The migtests `yb.py` uses row counts + column SUM + `Counter` equality for the same purpose — reuse `get_sum_of_column_of_table` / `assert_all_values_of_col` when driving that library.

**Constraints** (behavioral): attempt a violating INSERT on the target and assert the SQLSTATE (NOT NULL 23502, UNIQUE 23505, CHECK 23514, FK 23503). `yb.py:run_query_and_chk_error` does exactly this.

## 4. Sequence / identity advance oracle

A migration must set each sequence/identity past the max migrated value, or the first app insert collides. This is a recurring bug class and a QA-priority area.

```
-- on the TARGET, after import: a fresh insert must get an id beyond the migrated range
INSERT INTO orders (customer, amount) VALUES ('post_migration', 1) RETURNING id;   -- must be > max(migrated id)
```
Validated-good example: migrated 5000 orders → fresh insert returned `5001`; explicit sequence `widget_seq` (start 100, 500 rows → last 599) → fresh insert returned `600`. An id ≤ the migrated max ⇒ FAIL.

## 5. Report-parity oracle

- `export data status` vs `import data status` (both `--output-format json`): per-table exported vs imported counts must match. This is the pipeline's own claim of parity — diff it *and* cross-check against oracle 3 (the tool can be self-consistently wrong).
- For live: `get data-migration-report --output-format json` — snapshot + CDC event counts (`exported/imported inserts/updates/deletes`) must reconcile with the deltas you injected.
- For assess/analyze: `reports/schema_analysis_report.json` and `assessment/reports/migration_assessment_report.json`. Diff against a golden with volatile fields blanked (mirror `functions.sh:normalize_json` — blank `VoyagerVersion`, `TargetDBVersion`, `SizeInBytes`, `RowCount`, etc., `jq --sort-keys`, sort arrays) so only meaningful changes surface.

## 6. Usability oracle

Per the QA guidance, error messages / recommendations / UX are in scope, not a nicety.

For each negative or edge scenario, judge the captured stderr:
- Does it **name the offending flag/value** and say what's wrong?
- Does it **suggest the fix** ("use --start-clean true with --truncate-tables")?
- Is it a clean `ERROR:` line, or a raw Go stack / panic leaking to the user? A stack trace to the user is itself a FAIL for usability even if the underlying rejection is correct.
- Recommendations (assess-migration sizing, schema-analysis suggestions): are they present, plausible, and consistent with the schema?

## 7. State-transition oracle (end-to-end)

Validate the *workflow*, not just each command:
- **Resumability**: kill a daemon mid-run, relaunch identical command → resumes from checkpoint, final data parity holds (oracle 3), no duplicate rows.
- **Cutover**: `cutover status` reaches `COMPLETED`; the export/import daemons exit cleanly (no orphaned PIDs).
- **End migration**: `end migration` completes, backups exist under `--backup-dir`, and re-running commands against the ended migration behaves sanely.
- **failed.sql**: `schema/failed.sql` must not exist after a clean import (its presence = unimported schema objects).

## Classification

| Result | Meaning |
| --- | --- |
| **PASS** | Observed behavior matched the scenario's expected oracle. Cite the oracle. |
| **FAIL** | Real bug: hang, crash/panic, data mismatch, sequence collision, wrong/missing report, negative case that succeeded, or user-facing stack trace. Capture exact repro. |
| **EXPECTED-FAIL** | Negative case failed *gracefully* exactly as designed (right exit + actionable message). This is a PASS of the negative test. |
| **BLOCKED** | Could not run (env/setup problem). Not a verdict on the code — fix and rerun. |

Every FAIL in the report must include: scenario name, exact commands, the oracle it violated, observed vs expected, severity, and (for hangs) the goroutine stack.
