---
name: generate-data-integrity-test-plan
description: Generate an adversarial test plan hunting for silent data loss / data corruption in yb-voyager data migration, targeted at a set of recent changes (commit range, last N hours/days, or PR numbers). Maps the changed code to the data paths it touches, then combines schema shapes, workload patterns, flag combinations, run patterns, and migration flows into concrete, runnable test cases with an oracle each. Output is a plan file consumed by hunt-data-integrity-bugs. Use when asked to "generate a data-integrity test plan", "plan data-loss tests for recent changes", or as the first step of a data-integrity hunt.
---

# Generate a data-integrity test plan

Produces a **plan file** of concrete, runnable test cases that try to make voyager lose or corrupt data *silently* — the target ends up different from the source while nothing fails. The companion skill `hunt-data-integrity-bugs` turns each case into a Go test, runs it, and opens a PR per real bug.

Runs **unattended by default**: never ask questions; make the conservative choice and record it in the plan's `assumptions`. With `--interactive`, show the case list and wait for edits before writing the plan.

## Inputs

| Form | Example | Meaning |
|---|---|---|
| time window (default `24h`) | `--since 3d` | commits on `origin/main` in the window |
| commit range | `a1b2c3..d4e5f6` | exactly these commits |
| PR numbers | `--prs 3814,3820` | the PRs' head commits vs their base |
| none of the above has data-path changes | — | emit a **baseline-only** plan: only the Step 4 baseline cases |

Budget flag passed through to the plan: `--max-cases N` (default 30).

## References (read before generating)

- `references/silent-loss-mechanisms.md` — **how** voyager loses data silently; every case must name the mechanism it attacks.
- `references/dimensions.md` — the catalog of schema shapes, workloads, flags, run patterns, flows, and value types to combine.
- `references/inventory.md` — how to derive flags, config keys, env knobs, guardrails, target-DDL support, framework entry points and code anchors from the target commit on every run. Nothing that changes with a release is hardcoded; the catalogs below are seeds.
- `references/plan-schema.md` — the plan file format, with a worked example (the K4b case that found yb-voyager#3834).

## Workflow

```
- [ ] Step 0: Resolve the change set
- [ ] Step 0.5: Build the live inventory
- [ ] Step 1: Map changed code to data-path areas
- [ ] Step 2: Pick mechanisms and dimension slices per area
- [ ] Step 3: Generate cases (pairwise, adversarial, with oracles)
- [ ] Step 4: Add baseline cases
- [ ] Step 5: Rank, cap, validate, write the plan
```

### Step 0: Resolve the change set

```bash
git fetch origin main
# window:
git log origin/main --since="24 hours ago" --format='%H %s'
# or range / PRs:
gh pr view <N> --json headRefOid,baseRefOid,title,body
git diff --stat <base> <head>
```

Record `base`, `head`, commit list, and PR bodies (intent). The plan's target commit is `head` (default: `origin/main` tip).

### Step 0.5: Build the live inventory

Follow `references/inventory.md` against the worktree at `head` and a `yb-voyager` binary built from it. Store the result in the plan's `inventory`. From here on, use the inventory — not the seed lists in `dimensions.md` — for which flags, guardrails and framework methods exist. Uncatalogued flags that the diff touches get cases; all drift goes into the plan summary.

### Step 1: Map changed code to areas

Use the diff (full files for changed functions, not just hunks). Map each changed path/function to one or more **areas**:

| Area | Typical paths |
|---|---|
| `cdc-routing` | `cmd/live_migration.go` (`hashEvent`, `GetEventPartitionKey`, `handleEvent`), `cmd/live_migration_cdc_partition_strategy.go`, `cmd/import.go` (cdc flags) |
| `conflict-detection` | `cmd/conflictDetectionCache.go`, `addPrimaryKeyToConflictSetForCustomTables`, unique-index discovery in `src/tgtdb/*` |
| `apply-sql` | `src/tgtdb/event.go` (stmt builders), `src/tgtdb/yugabytedb.go` / `postgres.go` `ExecuteBatch`, `processEvents` |
| `value-encoding` | `src/dbzm/*`, `debezium-server-voyager/**`, value converters, datatype mapping |
| `table-selection` | table-list / exclude flags, name registry, publication / replication-slot setup, schema lists |
| `partitions` | partition→root mapping, `--use-partition-root`, leaf/root PK and index resolution |
| `snapshot` | export data snapshot, import data file tasks, snapshot/CDC boundary |
| `resume-restart` | channel `lastAppliedVsn`, `--start-clean`, lockfiles, idempotency of batches |
| `cutover-iteration` | cutover, fall-back / fall-forward importers, iterative cutover, `end migration` |
| `sequences` | sequence capture / restore, identity / serial handling |
| `identifiers` | `sqlname`, `namereg`, case-sensitive names |
| `guardrails` | pre-flight validations that allow/refuse configurations |

If a commit only touches tests, docs, assessment, callhome, or schema-only paths, it maps to nothing; note it.

### Step 2: Mechanisms and dimension slices

For each area, take the mechanisms in `silent-loss-mechanisms.md` that list that area, and the dimension slices in `dimensions.md` relevant to them. Read the changed code and write down, per mechanism, **the specific way the change could trigger it** (e.g. "new key derivation for partitioned roots → M3 statements hit rows in other leaves"). Mechanisms with no plausible link to the change are dropped for this plan.

### Step 3: Generate cases

Each case = one **mechanism** × a concrete **schema** × **workload** × **flags** × **run pattern** × **flow**, plus an **oracle** and an **expectation**. Rules:

- **Pairwise, not Cartesian.** Cover every pair of relevant dimension values at least once; do not enumerate the full product.
- **Adversarial by construction.** The workload must create the condition the mechanism needs (same key in two leaves, a value freed and reused across channels, an update that touches only some columns of a composite key, a PK reused under a different custom key, …). A case whose data can't trigger its mechanism is useless — state in `why_it_can_fail` what has to go wrong for the case to fail.
- **Include a control** when cheap: the same case with the triggering condition removed (e.g. distinct ids in each partition), so a failure can be attributed to that condition.
- **Valid on both databases.** Source SQL must succeed on PostgreSQL; schema must be creatable on YugabyteDB, whose DDL support changes per release. Mark every case with non-trivial DDL `ddl_probe: true` so the hunt probes it first (`inventory.md` → Target DDL support).
- **Workloads obey constraints.** Every source statement must succeed; a case whose delta errors on the source proves nothing.
- **Every case has an oracle** (`dimensions.md` → Oracles): full-row source-vs-target comparison after quiescence, plus any case-specific check (per-partition counts, sequence values after cutover, rows-affected warnings).
- **Expectation** is one of `consistent` (should migrate cleanly), `refused` (a guardrail in `inventory.guardrails` should reject it up front), `loud` (should fail with a clear error). A silent mismatch is a bug under every expectation.
- **Value fuzzing:** when the change maps to `value-encoding` (data types, converters, Debezium config), add value-fuzz cases (`value_fuzz` in the plan). Value fuzzing applies **only when the change touches data types or value encoding** (area `value-encoding`: datatype mapping, value converters, Debezium config or plugin, snapshot/CDC value formatting). It is still a container test: create one column per affected type, then insert and update rows with randomized and edge values for each type (NULL, empty, min/max, precision and scale extremes, NaN/±Infinity/-0, time zones and infinities, unicode and very long strings, NULL array elements, JSON key order and duplicates, TOASTed sizes), through both the snapshot and the change stream, and compare source and target row by row. Use a fixed seed and log it so a failure reproduces.
- **Adversarial variants of new tests.** If the change adds tests, add cases that break their assumptions (the gaps a reviewer would flag: one-sided assertions, avoided edge values, only-forward flow).

### Step 4: Baseline cases

Add **at most 3 baseline cases** per plan: cases from the standing catalog that are *not* tied to the change set, rotating through mechanisms (pick by `hash(date) mod N` so consecutive runs differ). They catch older bugs, but they must stay a small, clearly labelled slice of the run:

- Set `scope: "baseline"` and `linked_change: "baseline"` on each; every other case is `scope: "targeted"` and names the commit it attacks.
- Baseline cases follow the same rules as targeted ones — in particular, **no value fuzzing** unless the change set itself maps to `value-encoding`.
- If Step 1 mapped nothing, the plan is baseline-only: the same ≤ 3 cases, nothing else.
- A baseline case is investigated only as far as the hunt's verification steps require; exploration beyond the planned case goes in the report as a lead.

### Step 5: Rank, cap, validate, write

Priority:
- **P0** — attacks a changed code path via a mechanism that yields silent loss, or a flag combination the change newly allows.
- **P1** — changed area, mechanism yields loud failure or needs an unusual config.
- **P2** — baseline / regression coverage.

Cap to `--max-cases` (drop lowest priority first, keep at least one case per mechanism selected in Step 2). Validate each case against `plan-schema.md` (required fields, SQL non-empty, oracle present). Group container cases into **batches** of cases that can share one migration (same flow and flags, all `expect: consistent`) — a crash in one table would hide the rest, so never batch `refused`/`loud`/risky cases with others.

Write:
- `<scratch>/data-integrity/plan-<head-short>-<YYYYMMDD>.json` — the plan.
- `<scratch>/data-integrity/plan-<head-short>-<YYYYMMDD>.md` — a one-screen summary: change set, areas, mechanisms, and a table of cases (id, priority, mechanism, flow, one-line setup, expectation).

Reply with the two paths and the P0 case titles. When chained from the hunt skill, return the plan path only.

## Anti-patterns

- **Cases without a mechanism.** "Test partitions" is not a case; "custom key ≠ partition column, same id recycled inside a leaf under a new key value, `--use-partition-root false` → M1 drop-by-DO-NOTHING if the PK guard misses it" is.
- **Happy-path workloads.** Inserting distinct rows and checking counts finds nothing; each workload must set up the race, reuse, or ambiguity its mechanism needs.
- **Cartesian explosions.** 5 dimensions × 6 values each is 7,776 cases. Pairwise plus mechanism targeting keeps it to tens.
- **Hardcoded inventories.** Flags, guardrails, and framework methods come from Step 0.5, not from memory or the seed lists.
- **Untestable SQL.** Unsupported-on-YB DDL or source statements that violate constraints waste a whole container run.
