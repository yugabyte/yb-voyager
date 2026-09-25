---
name: hunt-data-integrity-bugs
description: Hunt for silent data loss / data corruption in yb-voyager data migration by turning a data-integrity test plan into real Go tests (container tests against PostgreSQL + YugabyteDB via src/testlivemigration, plus unit schedule fuzzers over the real CDC routing and conflict-detection code), running them, classifying every outcome, verifying and minimising each silent-divergence finding, and opening one draft PR per distinct bug containing a failing test. Unattended by default. Accepts a plan file, or a change set (commit range / last N hours / PR numbers) for which it first runs generate-data-integrity-test-plan. Use when asked to "hunt for data-loss bugs", "run the data-integrity hunt", "fuzz voyager for silent corruption", or from a scheduled routine.
---

# Hunt data-integrity bugs

Takes a plan from `generate-data-integrity-test-plan`, writes and runs a test per case, and for every **silent** divergence that survives verification opens **one draft PR per distinct bug** with a single failing test. CI on those PRs fails by design (the tests use the normal integration build tags) — the red test is the signal.

## Modes

- **Unattended (default):** no questions, no confirmations. Time-boxed; opens PRs directly; always ends with a report.
- **`--interactive`:** show the plan summary and ask before running; show each verified finding and ask before opening its PR.

## Inputs and defaults

| Input | Default |
|---|---|
| plan file path, or a change set (`--since 24h`, `a..b`, `--prs 1,2`) | if no plan is given, invoke `generate-data-integrity-test-plan` with the same change set first |
| `--time-budget` | `4h` wall clock for running tests (verification and PRs count toward it) |
| `--max-prs` | `5` per run; extra verified findings go in the report |
| `--flows` | all flows in the plan |
| `--target` | the plan's `target_commit` (usually `origin/main`) |

## References

- `references/harness.md` — environment preconditions, workspace layout, DDL probing, writing tests from cases, running, outcome classification, verification, cleanup. **Read it before Step 1.**
- `templates/example_case_test.go.tmpl` — a finished PR test written with the existing framework only (the yb-voyager#3834 repro). Container tests follow this shape; no shared helper files.
- `templates/fuzz_engine_test.go.tmpl`, `templates/fuzz_partitions_test.go.tmpl` — unit schedule fuzzers (real `hashEvent` + `ConflictDetectionCache` + a model target with voyager's apply semantics, random interleavings, detection-off mutant). Extend them with the plan's `fuzz` scenarios.
- `../generate-data-integrity-test-plan/references/` — mechanisms, dimensions, inventory derivation, plan schema.

## Workflow

```
- [ ] Step 0: Preconditions and workspace
- [ ] Step 1: Load and validate the plan
- [ ] Step 2: Fuzz cases
- [ ] Step 3: Container cases (probe DDL, write, run in batches)
- [ ] Step 4: Classify every case
- [ ] Step 5: Verify, minimise, dedupe each SILENT candidate
- [ ] Step 6: One draft PR per distinct bug
- [ ] Step 7: Report and clean up
```

### Step 0: Preconditions and workspace

Follow `harness.md` → Environment preconditions and Workspace. Create the worktree at the target commit, build `yb-voyager` into `$SCRATCH/bin`, put it first on PATH, and verify `GIT_COMMIT_HASH`. If a precondition fails, stop and report exactly which (unattended runs must not half-run).

Self-check against the target commit before writing any test:
- If the plan's `inventory` is missing or was built at a different commit, rebuild it (`../generate-data-integrity-test-plan/references/inventory.md`).
- Compile `templates/example_case_test.go.tmpl` (copied into `src/testlivemigration/`, then removed) and, if fuzz cases exist, the fuzz templates. If a template no longer compiles, fix the run's copy, carry on, and report the template drift. If the fuzz engines can't be repaired quickly, skip fuzz cases and say so.
- Grep every anchor in `inventory.anchors`; stale ones go in the drift section.

### Step 1: Load and validate the plan

Validate against `plan-schema.md`. Drop malformed cases with a reason. Sort by priority (P0 first). Estimate duration (~2–5 min per container case at the chosen parallelism, kill/resume and multi-iteration cases ~2×); if the budget can't cover everything, keep all P0, then P1 by mechanism diversity, and list the skipped cases in the report.

### Step 2: Fuzz cases

Copy the fuzz templates into `yb-voyager/cmd/`, add one scenario per plan `fuzz` case (and a detection-off mutant for each, to prove the scenario can see races), and run `go test -tags unit -run TestDataIntegrityFuzz ./cmd/`. Fuzzers run in seconds — do them first.

A fuzz failure is a **lead, not a finding**: the model can be wrong about real encodings or guardrails (spellings that Debezium normalises, schemas that export refuses). Every fuzz failure must be turned into a container case and reproduced end-to-end before it can become a PR. If the container case shows a guardrail blocks it, report it as *latent* (no PR).

### Step 3: Container cases

1. Probe DDL for `ddl_probe` cases (harness → DDL probe).
2. Write tests from cases (harness → Writing a container test). Batches from the plan share one test; everything else gets its own test. Risky cases (`expect: refused | loud`) never share a migration with others.
3. Compile: `go vet -tags <tag> ./src/testlivemigration/`.
4. Run in background batches of `P` tests (harness → Running). As each batch finishes, summarise its `DI-RESULT` lines before starting the next, so a harness problem (e.g. every export failing at startup) is caught after one batch, not after the whole budget.

### Step 4: Classify

Classify each case per harness → Classifying outcomes, comparing against the case's `expect`. Fix `TEST_INVALID` tests once and rerun. Record for every case: outcome, expectation, evidence line, duration.

### Step 5: Verify SILENT candidates

For each candidate, run all six checks in harness → Verifying a SILENT candidate (reproduce 2/3, dump real divergence, attribute, minimise, not already reported, signature). Group candidates by signature — **one bug per signature**, even if several cases hit it. A candidate that fails any check goes in the report with the reason, not in a PR.

### Step 6: One draft PR per distinct bug

For each verified signature (up to `--max-prs`, P0 first):

1. Branch from the target commit: `data-integrity/<YYYYMMDD>-<short-slug>`.
2. Add **only** the minimised test file (`src/testlivemigration/data_integrity_<slug>_test.go`), written with the existing framework like `templates/example_case_test.go.tmpl`. The test **asserts the correct behaviour** (consistent target, or a refusal) so it fails today. Build tag: the normal tag of the flow (`integration_live_migration`, or `..._with_failpoint` only if needed). `gofmt` the files. `git status` must show nothing else (no failpoint-rewritten files, no plan files).
3. Run the new test once on the branch to confirm it fails for the reported reason.
4. Commit (follow the repo's attribution rules), push, and open a **draft** PR:
   - title `[data-integrity] <symptom in one line>`
   - body from the repo's `.github/PULL_REQUEST_TEMPLATE` via the `pr-description` skill: setup, workload, expected vs actual, repro rate, how to run, suggested fix. Note that CI fails by design, and end with the signature line for dedupe.
   - no customer names or data anywhere (synthetic schemas only).

Do not file Jira tickets; the user can run `create-voyager-issue` on a PR they want tracked.

### Step 7: Report and clean up

Write `$SCRATCH/data-integrity/report-<YYYYMMDD>.md`:

- change set, plan path, target commit, time used / budget
- **Findings**: one row per PR (link, signature, mechanism, repro rate)
- **Unverified / latent leads**: fuzz-only failures, candidates that failed verification, findings blocked by guardrails — with the reason
- **Unexpected loud failures and refusals** (not data loss, but worth a look)
- **Coverage**: table of every case → outcome vs expectation; skipped cases and why; flows not exercised
- **Catalog drift**: uncatalogued/stale flags, stale anchors, guardrails added/removed, templates that needed fixes (see `inventory.md` → Drift report)
- **Suggested follow-ups**: guardrails to consider, catalog additions

Then clean up per harness → Cleanup. Reply with the PR links, the report path, and one line per finding. If an Artifact tool is available, publish the report as a page and include the link.

## Anti-patterns

- **Testing a stale binary.** The framework execs `yb-voyager` from PATH; always build the target commit and verify the hash.
- **Trusting the fuzzer alone.** Two of the first hunt's fuzz "failures" (value spellings) were impossible end-to-end; one (random PK guard) was blocked by an export guardrail. Only container repros become PRs.
- **Batching risky cases.** One crash in a shared migration hides every other table's result.
- **Asserting in the run, not logging.** Fatal assertions during the hunt destroy evidence; assert only in the PR version.
- **One PR per case.** Several cases hitting the same signature are one bug — one PR, with the others mentioned in its body.
- **Committing noise.** PR branches carry the one failing test file only — never failpoint rewrites, plans, logs, or fuzz scaffolding.
