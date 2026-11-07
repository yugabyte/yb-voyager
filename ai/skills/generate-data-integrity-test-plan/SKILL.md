---
name: generate-data-integrity-test-plan
description: Generate an adversarial test plan hunting for silent data loss / data corruption in yb-voyager data migration, targeted at the PRs merged in a window (last N hours/days, commit range, or PR numbers); every case is attributed to a PR, and a window with no data-path PRs yields an empty plan. Maps the changed code to the data paths it touches, then combines schema shapes, workload patterns, flag combinations, run patterns, and migration flows into concrete, runnable test cases with an oracle each. Output is a plan file consumed by hunt-data-integrity-bugs. Use when asked to "generate a data-integrity test plan", "plan data-loss tests for recent changes", or as the first step of a data-integrity hunt.
---

# Generate a data-integrity test plan

Produces a **plan file** of concrete, runnable test cases that try to make voyager lose or corrupt data *silently* — the target ends up different from the source while nothing fails. Every case attacks a specific PR in the change set, so whatever it finds can be attributed. The companion skill `hunt-data-integrity-bugs` turns each case into a Go test, runs it, and files one deduplicated GitHub issue per real bug.

Runs **unattended by default**: never ask questions; make the conservative choice and record it in the plan's `assumptions`. With `--interactive`, show the case list and wait for edits before writing the plan.

## Inputs

| Form | Example | Meaning |
|---|---|---|
| time window (default `24h`) | `--since 3d` | PRs merged to `origin/main` in the window |
| commit range | `a1b2c3..d4e5f6` | the PRs behind exactly these commits |
| PR numbers | `--prs 3814,3820` | these PRs (merge commits on `origin/main`, or head vs base if unmerged) |

No PRs in the change set, or none that maps to a data-path area → write an **empty plan** (`cases: []` plus the reason) and stop. There are no baseline or standing-catalog cases: every case must be attributable to a PR.

Budget flag passed through to the plan: `--max-cases N` (default 30).

## References (read before generating)

- `references/silent-loss-mechanisms.md` — **how** voyager loses data silently; every case must name the mechanism it attacks.
- `references/dimensions.md` — the catalog of schema shapes, workloads, flags, run patterns, flows, and value types to combine.
- `references/inventory.md` — how to derive flags, config keys, env knobs, guardrails, target-DDL support, framework entry points and code anchors from the target commit on every run. Nothing that changes with a release is hardcoded; the catalogs below are seeds.
- `references/plan-schema.md` — the plan file format, with a worked example (the K4b case that found yb-voyager#3834).

## Workflow

```
- [ ] Step 0: Resolve the change set to PRs (stop if none)
- [ ] Step 0.5: Build the live inventory
- [ ] Step 1: Map each PR's changed code to data-path areas (stop if none)
- [ ] Step 2: Pick mechanisms and dimension slices per area
- [ ] Step 3: Generate cases (pairwise, adversarial, with oracles), each linked to its PR(s)
- [ ] Step 4: Rank, cap, validate, write the plan
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

Map every commit to its PR: the `(#NNNN)` suffix of a squash-merge subject, else `gh api repos/yugabyte/yb-voyager/commits/<sha>/pulls -q '.[].number'`. Record `base`, `head`, the commit list, and each PR's number, title, body (intent) and merge commit. The plan's target commit is `head` (default: `origin/main` tip).

**Gate.** If the change set has no PRs, write the empty plan and stop. Then do a quick Step 1 pass over the PR diffs before building the inventory: if no PR maps to a data-path area, write the empty plan (listing each PR and why it maps to nothing) and stop.

### Step 0.5: Build the live inventory

Follow `references/inventory.md` against the worktree at `head` and a `yb-voyager` binary built from it. Store the result in the plan's `inventory`. From here on, use the inventory — not the seed lists in `dimensions.md` — for which flags, guardrails and framework methods exist. Uncatalogued flags that the diff touches get cases; all drift goes into the plan summary.

### Step 1: Map changed code to areas

Use each PR's diff (full files for changed functions, not just hunks). Map each changed path/function to one or more **areas**, keeping track of which PR it came from:

| Area | Typical paths |
|---|---|
| `cdc-routing` | `cmd/live_migration.go` (`hashEvent`, `GetEventPartitionKey`, `handleEvent`), `cmd/live_migration_cdc_partition_strategy.go`, `cmd/import.go` (cdc flags) |
| `conflict-detection` | `cmd/conflictDetectionCache.go`, `addPrimaryKeyToConflictSetForCustomTables`, unique-index discovery in `src/tgtdb/*` |
| `apply-sql` | `src/tgtdb/event.go` (stmt builders), `src/tgtdb/yugabytedb.go` / `postgres.go` `ExecuteBatch`, `processEvents` |
| `value-encoding` | `src/dbzm/*`, `debezium-server-voyager/**`, value converters, datatype mapping, unsupported-type lists (`GetPGLiveMigrationUnsupportedDatatypes`, `ReportUnsupportedDatatypes*`, `fetchColumnsWithUnsupportedDataTypes`) |
| `persistence` | MSR writes (`UpdateMigrationStatusRecord`, fields in `src/metadb/migrationStatus.go`), metaDB marks (`MarkEventQueueSegmentAsProcessed`, `src/metadb/*`), status/state JSON files, queue segment rotation and archive, failpoints (`cmd/failpoints.go`, `cmd/export_failpoints.go`, `src/tgtdb/failpoints.go`) |
| `table-selection` | table-list / exclude flags, name registry, publication / replication-slot setup, schema lists |
| `partitions` | partition→root mapping, `--use-partition-root`, leaf/root PK and index resolution |
| `snapshot` | export data snapshot, import data file tasks, snapshot/CDC boundary |
| `resume-restart` | channel `lastAppliedVsn`, `--start-clean`, lockfiles, idempotency of batches |
| `cutover-iteration` | cutover, fall-back / fall-forward importers, iterative cutover, `end migration` |
| `sequences` | sequence capture / restore, identity / serial handling |
| `identifiers` | `sqlname`, `namereg`, case-sensitive names |
| `guardrails` | pre-flight validations that allow/refuse configurations |

If a PR only touches docs, assessment, callhome, or schema-only paths, it maps to nothing; note it. A PR that only adds tests maps to the areas those tests exercise (its cases attack the gaps in the new tests).

### Step 2: Mechanisms and dimension slices

For each area, take the mechanisms in `silent-loss-mechanisms.md` that list that area, and the dimension slices in `dimensions.md` relevant to them. Read the changed code and write down, per mechanism, **the specific way the change could trigger it** (e.g. "new key derivation for partitioned roots → M3 statements hit rows in other leaves"). Mechanisms with no plausible link to the change are dropped for this plan.

**Save-point map** (when a PR maps to `persistence`, `resume-restart` or `cutover-iteration`). For each changed function and its callers, list every durable write, what it claims (e.g. "segment N fully applied by the target importer"), and the work that claim covers. Flag (a) a claim written before its work is durable, and (b) two writes that must hold together but are committed separately. Each flagged point becomes an M11 case that crashes exactly there (`crash_at`), plus a control that crashes just after the work. Record the map in the plan's `save_points`; also check save points the PR *moved*, since a reordering can open a window it didn't have before.

**Touched type families** (when a PR maps to `value-encoding`). From the converter, mapping or list the PR changes, name the type families it affects (`dimensions.md` → Type sweep). Only those families are swept; a PR that touches no family gets no sweep.

### Step 3: Generate cases

Each case = one **mechanism** × a concrete **schema** × **workload** × **flags** × **run pattern** × **flow**, plus an **oracle**, an **expectation** and the **PR(s) it attacks** (`linked_prs`). Rules:

- **Attributable.** A case exists only because a specific PR changed a specific path; name both (`linked_prs`, `linked_change`). If you can't say which PR a case tests, drop it.

- **Pairwise, not Cartesian.** Cover every pair of relevant dimension values at least once; do not enumerate the full product.
- **Adversarial by construction.** The workload must create the condition the mechanism needs (same key in two leaves, a value freed and reused across channels, an update that touches only some columns of a composite key, a PK reused under a different custom key, …). A case whose data can't trigger its mechanism is useless — state in `why_it_can_fail` what has to go wrong for the case to fail.
- **Include a control** when cheap: the same case with the triggering condition removed (e.g. distinct ids in each partition), so a failure can be attributed to that condition.
- **Valid on both databases.** Source SQL must succeed on PostgreSQL; schema must be creatable on YugabyteDB, whose DDL support changes per release. Mark every case with non-trivial DDL `ddl_probe: true` so the hunt probes it first (`inventory.md` → Target DDL support).
- **Workloads obey constraints.** Every source statement must succeed; a case whose delta errors on the source proves nothing.
- **Every case has an oracle** (`dimensions.md` → Oracles): full-row source-vs-target comparison after quiescence, plus any case-specific check (per-partition counts, sequence values after cutover, rows-affected warnings).
- **Expectation** is one of `consistent` (should migrate cleanly), `refused` (a guardrail in `inventory.guardrails` should reject it up front), `loud` (should fail with a clear error). A silent mismatch is a bug under every expectation.
- **Type sweep:** when a PR maps to `value-encoding`, add one `type_sweep` case for the touched families only (`dimensions.md` → Type sweep): every type in those families × offline, live and fall-back × the sweep operations, with control columns and a warnings check. The hunt resolves the concrete types from the source catalog at run time.
- **Crash placement:** an M11 case says where to crash (`crash_at`): an existing failpoint at the save point if there is one, else a log line that marks the window plus a way to hold it open; a scratch-only failpoint is the last resort. Random SIGKILL is not a substitute.
- **Adversarial variants of new tests.** If the change adds tests, add cases that break their assumptions (the gaps a reviewer would flag: one-sided assertions, avoided edge values, only-forward flow).

### Step 4: Rank, cap, validate, write

Priority:
- **P0** — attacks a changed code path via a mechanism that yields silent loss, or a flag combination the change newly allows.
- **P1** — changed area, mechanism yields loud failure or needs an unusual config.
- **P2** — lower-likelihood variants and controls for P0/P1 cases.

Cap to `--max-cases` (drop lowest priority first, keep at least one case per mechanism selected in Step 2). Validate each case against `plan-schema.md` (required fields, SQL non-empty, oracle present). Group container cases into **batches** of cases that can share one migration (same flow and flags, all `expect: consistent`) — a crash in one table would hide the rest, so never batch `refused`/`loud`/risky cases with others.

Write:
- `<scratch>/data-integrity/plan-<head-short>-<YYYYMMDD>.json` — the plan.
- `<scratch>/data-integrity/plan-<head-short>-<YYYYMMDD>.md` — a one-screen summary: PRs, areas, mechanisms, and a table of cases (id, PR, priority, mechanism, flow, one-line setup, expectation).

Reply with the two paths and the P0 case titles (or the reason the plan is empty). When chained from the hunt skill, return the plan path only.

## Anti-patterns

- **Cases without a mechanism.** "Test partitions" is not a case; "custom key ≠ partition column, same id recycled inside a leaf under a new key value, `--use-partition-root false` → M1 drop-by-DO-NOTHING if the PK guard misses it" is.
- **Happy-path workloads.** Inserting distinct rows and checking counts finds nothing; each workload must set up the race, reuse, or ambiguity its mechanism needs.
- **Cartesian explosions.** 5 dimensions × 6 values each is 7,776 cases. Pairwise plus mechanism targeting keeps it to tens.
- **Hardcoded inventories.** Flags, guardrails, and framework methods come from Step 0.5, not from memory or the seed lists.
- **Untestable SQL.** Unsupported-on-YB DDL or source statements that violate constraints waste a whole container run.
