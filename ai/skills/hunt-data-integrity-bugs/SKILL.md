---
name: hunt-data-integrity-bugs
description: Hunt for silent data loss / data corruption in yb-voyager data migration introduced or exposed by recently merged PRs. Turns a data-integrity test plan into real Go tests (container tests against PostgreSQL + YugabyteDB via src/testlivemigration, with crash placement at save points and a type sweep for the type families a PR touches), runs them, classifies every outcome, verifies and minimises each silent divergence or unexpected loud failure, attributes it to the PR(s) under test, and files one deduplicated GitHub issue per distinct bug with a repro test. Unattended by default; skips entirely when no PRs were merged in the window. Accepts a plan file, or a change set (last N hours / commit range / PR numbers) for which it first runs generate-data-integrity-test-plan. Use when asked to "hunt for data-loss bugs", "run the data-integrity hunt", "fuzz voyager for silent corruption", or from a scheduled routine.
---

# Hunt data-integrity bugs

Takes a plan from `generate-data-integrity-test-plan`, writes and runs a test per case, and for every **silent divergence** or **unexpected loud failure** that survives verification files **one GitHub issue per distinct bug**, attributed to the PR(s) under test and carrying a minimal repro test. Issues are deduplicated against every earlier hunt issue, so a bug is filed once no matter how many runs hit it.

## Modes

- **Unattended (default):** no questions, no confirmations. Time-boxed; files issues directly; always ends with a report.
- **`--interactive`:** show the plan summary and ask before running; show each verified finding and ask before filing its issue.

## Inputs and defaults

| Input | Default |
|---|---|
| plan file path, or a change set (`--since 24h`, `a..b`, `--prs 1,2`) | if no plan is given, invoke `generate-data-integrity-test-plan` with the same change set first |
| `--time-budget` | `4h` wall clock for running tests (verification and filing count toward it) |
| `--max-issues` | `5` new issues per run; extra verified findings go in the report |
| `--flows` | all flows in the plan |
| `--target` | the plan's `target_commit` (usually `origin/main`) |

## References

- `references/environment.md` — probing and provisioning a host (Docker, JDK, Debezium, PG client tools, binary), including restricted-network and Claude Code cloud notes. **Read it first.**
- `references/harness.md` — workspace layout, DDL probing, writing tests from cases, running, outcome classification, verification, cleanup. **Read it before Step 1.**
- `templates/example_case_test.go.tmpl` — a finished repro test written with the existing framework only (the yb-voyager#3834 repro). Container tests follow this shape; no shared helper files.
- `../generate-data-integrity-test-plan/references/` — mechanisms, dimensions, inventory derivation, plan schema.

## Stay in bounds

These hold in every mode, especially unattended:

- **Only PR-attributable work.** Every case, finding and issue ties to one or more PRs in the change set. No PRs merged in the window → no tests at all (Step 0).
- **Change only what the run owns.** Worktrees, scratch dirs, containers and processes the run created. Clean up by name or label (testcontainers label its containers `org.testcontainers=true`; probes use `di-probe*`), never with host-wide commands such as `docker ps -q | xargs docker rm -f` or `rm -rf /tmp/yb-voyager-export*`.
- **System changes only in a throwaway sandbox.** Starting daemons, editing apt sources, installing packages or writing under `/opt` are fine in an ephemeral environment (a cloud session, a CI runner). On a developer machine or shared host, stop and report what is missing instead.
- **Scratch is yours; the repo is not.** Inside the run's scratch worktree you may change anything — add failpoints, log lines or instrumentation, even edit product code — to place a crash or observe a value. Nothing is ever committed, pushed or opened as a PR, and the user's checkout is never touched. Every scratch change a finding depends on is listed in its issue (Step 6). A finding that only reproduces because a scratch edit changed behaviour (rather than only observing or pausing it) is not a bug.
- **Stay on the plan.** Run the planned cases and the verification steps for their candidates. Anything else that looks interesting goes in the report as a lead, not a new investigation.
- **One way out.** The outputs are: new `[data-integrity]` issues (Step 6), the Slack post if a channel was given (Step 7), and the report. The report is the final message plus the files under `$SCRATCH/data-integrity/`; don't publish it as a page, doc or artifact unless the caller asks for one. Don't comment on, edit, close or relabel existing issues or PRs. No push notifications, emails, Jira filing, subscriptions, reactions to CI/review events, or scheduled follow-ups.

## Workflow

```
- [ ] Step 0: Change set gate, preconditions, workspace
- [ ] Step 1: Load and validate the plan
- [ ] Step 2: Type sweep (only for type families a PR touches)
- [ ] Step 3: Container cases (probe DDL, write, run in batches)
- [ ] Step 4: Classify every case
- [ ] Step 5: Verify, attribute, minimise, dedupe each candidate
- [ ] Step 6: One issue per new distinct bug
- [ ] Step 7: Report, Slack, clean up
```

### Step 0: Change set gate, preconditions, workspace

Record `date -u` as the run's start time.

**Gate first, before provisioning anything.** Resolve the change set to its merged PRs (plan skill → Step 0). If **no PRs** were merged in the window, or the plan has **no cases** because no PR touches a data path, stop here: no environment setup, no tests, no issues, no Slack post. The report is one line (`no PRs merged since <time>` or `PRs #a, #b merged; none touch a data path`), then end the session.

Otherwise follow `references/environment.md` (probe, provision, clean env, build the target commit into `$SCRATCH/bin`, pre-pull images, smoke test), then `harness.md` → Environment and Workspace. If a hard requirement fails, stop and report exactly which one (unattended runs must not half-run).

Self-check against the target commit before writing any test:
- If the plan's `inventory` is missing or was built at a different commit, rebuild it (`../generate-data-integrity-test-plan/references/inventory.md`).
- Compile `templates/example_case_test.go.tmpl` (copied into `src/testlivemigration/`, then removed). If it no longer compiles, fix the run's copy, carry on, and report the template drift.
- Grep every anchor in `inventory.anchors`; stale ones go in the drift section.

### Step 1: Load and validate the plan

Validate against `plan-schema.md`; every case must name `linked_prs`. Drop malformed cases with a reason. Sort by priority (P0 first). Estimate duration (~2–5 min per container case at the chosen parallelism, kill/resume and multi-iteration cases ~2×); if the budget can't cover everything, keep all P0, then P1 by mechanism diversity, and list the skipped cases in the report.

### Step 2: Type sweep (only for type families a PR touches)

Runs only for `type_sweep` cases, which the plan adds only for the type families a PR touches. Follow harness → Type sweep tests: resolve the concrete types in those families from the source catalog, run each through offline, live and fall-back with the sweep operations and control columns, and record one cell per type × flow × operation (OK, SILENT, LOUD, STUCK) plus whether voyager warned. Every SILENT, LOUD or STUCK cell is a candidate for Step 5; shrink it to the single value, operation and flow that fails, then group cells by root cause (Step 5) — not by type.

### Step 3: Container cases

1. Probe DDL for `ddl_probe` cases (harness → DDL probe).
2. Write tests from cases (harness → Writing a container test; M11 cases also follow harness → Placing a crash at a save point). Batches from the plan share one test; everything else gets its own test. Risky cases (`expect: refused | loud`) never share a migration with others.
3. Compile: `go vet -tags <tag> ./src/testlivemigration/`.
4. Run in background batches of `P` tests (harness → Running). As each batch finishes, summarise its `DI-RESULT` lines before starting the next, so a harness problem (e.g. every export failing at startup) is caught after one batch, not after the whole budget.

### Step 4: Classify

Classify each case per harness → Classifying outcomes, comparing against the case's `expect`. Fix `TEST_INVALID` tests once and rerun. Record for every case: outcome, expectation, evidence line, duration.

### Step 5: Verify SILENT and unexpected LOUD candidates

For each candidate, run every check in harness → Verifying a candidate: reproduce, real divergence (silent) or stable error (loud), locate, **attribute to the PR(s)**, minimise, signature, and **dedupe against existing issues**. Group candidates by signature — **one bug per signature**, even if several cases hit it. A candidate that fails a check goes in the report with the reason, not in an issue.

### Step 6: One issue per new distinct bug

For each verified signature that dedupe marked **new** or **regression** (up to `--max-issues`; silent before loud, P0 first):

1. Write the minimised repro test (`data_integrity_<slug>_test.go`) with the existing framework, like `templates/example_case_test.go.tmpl`. It **asserts the correct behaviour** (consistent target and importer still running, or a refusal) so it fails today. Normal build tag of the flow. `gofmt` it and run it once in the scratch worktree to confirm it fails for the reported reason.
2. File the issue with `gh issue create --repo yugabyte/yb-voyager --label area/voyager --label status/awaiting-triage --label kind/bug --title … --body-file …`:
   - **Title:** `[data-integrity] <silent symptom in one line>`, or `[data-integrity][loud] <error in one line>` for loud failures; prefix `Regression: ` after the tag when dedupe found a closed-as-fixed match.
   - **Body**, in the shape of `.github/ISSUE_TEMPLATE/voyager.yml`:

     ~~~markdown
     ### Description

     **Attributed to:** #<PR> (<short-sha>) — introduced by it | pre-existing, exposed by testing it (repro at <base-sha> <fails|passes>)
     **Symptom:** <what the target ends up with vs the source, or the error and where the importer stops or hangs>
     **Flow / flags:** <flow>, <non-default flags>
     **Setup and workload:** <synthetic schema and the statements that trigger it>
     **Expected:** <…>   **Actual:** <…>   **Repro rate:** <k/n>
     **Evidence:** <differing rows, queue line, log line — synthetic data only>
     **Suggested fix:** <one or two sentences, if clear>
     **Scratch instrumentation:** none | <each scratch change the repro needs: file:function and what it does, e.g. `cmd/live_migration.go:streamChangesFromSegment` — new failpoint `diCrashAfterCutoverFlag` right after the MSR write>; patch in the second details block
     Regression of #<N>  ← only for regressions

     <details><summary>Repro test — save as yb-voyager/src/testlivemigration/data_integrity_<slug>_test.go and run
     <code>go test -tags integration_live_migration -count=1 -run '^TestName$' ./src/testlivemigration/</code></summary>

     ```go
     <the test file>
     ```
     </details>

     <details><summary>Scratch patch (only if the repro needs one) — apply with <code>git apply</code>, then <code>failpoint-ctl enable</code></summary>

     ```diff
     <git diff of the scratch worktree, excluding the test file>
     ```
     </details>

     `DI-SIG: <signature>`

     ### Issue Type

     kind/bug

     ### Warning: Please confirm that this issue does not contain any sensitive information

     - [x] I confirm this issue does not contain any sensitive information.
     ~~~
   - Synthetic schemas and data only; no customer names, hosts or logs from real systems. Check that before ticking the box.
3. Record the issue URL for the report and Slack.

Verified findings beyond `--max-issues` go in the report only.

### Step 7: Report, Slack, clean up

Write `$SCRATCH/data-integrity/report-<YYYYMMDD>.md`:

- change set (PRs and commits), plan path, target commit, time used / budget (time used = now minus the start time from Step 0; don't estimate it)
- **New issues**: one row per issue filed (link, silent/loud, signature, mechanism, repro rate, attributed PR, introduced vs pre-existing)
- **Already reported**: findings that matched an existing issue (its number and state)
- **Unverified / latent leads**: candidates that failed verification, findings blocked by guardrails, findings over `--max-issues` — with the reason
- **Unexpected refusals** (usability, not data loss)
- **Coverage**: table of every case → outcome vs expectation, with its PR; skipped cases and why; flows not exercised
- **Catalog drift**: uncatalogued/stale flags, stale anchors, guardrails added/removed, templates that needed fixes (see `inventory.md` → Drift report)

If a Slack channel was provided (e.g. by a routine) and the run filed at least one new issue, post exactly this:

- **One top-level message** in the channel: `Data-integrity hunt <YYYY-MM-DD> @ <short-sha> (PRs #a, #b): <N> new issue(s) — <S> silent, <L> loud`.
- **Thread replies** under it, one per new issue: the issue link, `silent` or `loud`, the attributed PR, and a one-line summary.
- If findings also matched already-open issues, **one more thread reply**: `Also reproduced (already open): #x, #y`.
- Nothing else in the channel; post nothing when no new issue was filed.

Then clean up per harness → Cleanup, and **end the session** — no background watchers, subscriptions or scheduled check-ins left behind. Reply with the issue links, the report path, and one line per finding.

## Anti-patterns

- **Testing without a PR to blame.** A case that can't name the PR it attacks doesn't belong in the plan; a finding that can't be attributed doesn't get an issue.
- **Testing a stale binary.** The framework execs `yb-voyager` from PATH; always build the target commit and verify it by path and build time.
- **Batching risky cases.** One crash in a shared migration hides every other table's result.
- **Asserting in the run, not logging.** Fatal assertions during the hunt destroy evidence; assert only in the repro test.
- **One issue per case.** Several cases hitting the same signature are one bug — one issue, with the other cases listed in its body. In a type sweep, many types failing the same way (e.g. every value pasted unquoted into UPDATE) are one issue listing the types.
- **Random kills for narrow windows.** A SIGKILL at a random time almost never lands in a save-point window; place the crash deliberately.
- **Re-filing a known bug.** Always dedupe against every earlier `[data-integrity]` issue and PR, open or closed, before filing.
