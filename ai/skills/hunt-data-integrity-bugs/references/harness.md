# Running data-integrity tests

## Environment

Follow `environment.md` first (probe, provision, clean env, build, pre-pull, smoke test). The rest of this page assumes it passed. Keep these exports in `$SCRATCH/env.sh` and source it in every command, because shell state may not persist between commands:

```bash
unset CONTROL_PLANE_TYPE YUGABYTED_DB_CONN_STRING
export YB_VOYAGER_SEND_DIAGNOSTICS=0 LANG=C.UTF-8 LC_ALL=C.UTF-8
export JAVA_HOME=<jdk17>; export PATH=$SCRATCH/bin:$JAVA_HOME/bin:$PATH
```

Failpoints (only if a case needs them): `failpoint-ctl enable` in `yb-voyager/` before building/testing, `failpoint-ctl disable` afterwards and before any commit.

## Workspace

- Work in a **dedicated git worktree** at the target commit (`git worktree add --detach $SCRATCH/di-wt <sha>`); never in the user's checkout. If the session cannot create worktrees, use a throwaway branch in its own worktree.
- Generated tests go in `yb-voyager/src/testlivemigration/data_integrity_<plan-date>_<case>_test.go` . Container tests use the existing framework only (`NewLiveMigrationTest`, `TestConfig`, `Start*`/`Wait*`/`Initiate*` methods, `ValidateDataConsistency`, `testutils.CompareTableData`) — do not add shared helper files.
- Build tag: `integration_live_migration` unless a case needs failpoints; then `integration_live_migration_with_failpoint`.

## DDL probe (for `ddl_probe: true` cases, and whenever unsure)

YB rejects some DDL that PG accepts, and the set changes per YB release. A rejected schema wastes a full container run. Probe first on a scratch container with the image the tests use (read it from a previous test log or `docker images | grep yugabyte`):

```bash
docker run -d --name di-probe yugabytedb/yugabyte:<tag> bin/yugabyted start --background=false --advertise_address=127.0.0.1
# wait until: docker exec di-probe bin/ysqlsh -h 127.0.0.1 -c 'select 1'
docker cp probe.sql di-probe:/tmp/p.sql && docker exec di-probe bin/ysqlsh -h 127.0.0.1 -f /tmp/p.sql 2>&1 | grep -E 'ERROR|DONE'
docker rm -f di-probe
```

Drop or adapt cases whose DDL fails; record them as `TEST_INVALID` with the error.

## Writing a container test from a case

- One `Test` function per case (or per batch of `expect: consistent` cases that share flow and flags). Name: `TestDataIntegrity_<CaseID>_<Slug>`. `t.Parallel()`.
- **Isolate crash-prone cases.** One importer crash stops every table in that migration. Value-fuzz tables (edge values, BC dates, infinities, unusual encodings) crash the importer most often, so give each its own test, never a table inside a shared batch.
- Doc comment: the mechanism, the exact sequence that would lose data, and the expected outcome.
- Map plan fields straight onto `TestConfig` (`SchemaSQL`, `SourceSetupSchemaSQL`, `InitialDataSQL`, `SourceDeltaSQL`, `CleanupSQL`, `SourceReplicaDB` for fall-forward). Phases become source/target SQL (see the next bullet), DDL drift on the target, `lm.GetImportRunner().Kill()` + removing `<export-dir>/.*Lockfile.lck` + `ResumeImportData`, `StopImportData` / `ResumeImportData`, `InitiateCutoverToTarget` / `InitiateCutoverToSource`, `WaitForForwardStreamingComplete` (only when exact counts are known).
- **Run workload SQL through `WithSourceConn` / `WithTargetConn`, not `ExecuteOnSource` / `ExecuteOnTarget`.** The `Execute*` helpers call `utils.ErrExit` on a SQL error, which kills the **whole test binary** — every parallel test in the batch dies with it. Use `Execute*` only for setup statements that must succeed; run delta/phase SQL with `lm.WithSourceConn(func(db *sql.DB) error { _, err := db.Exec(stmt); return err })` and log the error.
- **Refusals with async starts.** Live tests start export and import with `async=true`, and then `Start*` returns nil as soon as the process is running; the exit status only reaches the runner's internal channel. A refusal therefore shows up as the process exiting early — `WaitForSnapshotComplete` fails and `IsStopped()` is true — with the reason in `GetExportCommandStderr()` / `GetImportCommandStderr()`. Handle it as in `templates/example_case_test.go.tmpl`. Only a synchronous start (`async=false`) returns the exit error directly.
- **`IsStopped()` reports an exit once.** It does a non-blocking receive on a one-slot channel that is written exactly once, so only the first call after the process exits returns true. Read it once per decision point, keep the result in a variable (e.g. `stopped := lm.GetImportRunner().IsStopped()`), and reuse that value for logging and classification. Never call it again to "record" a state a poll loop already observed — the second call returns false and turns a LOUD crash into a false SILENT candidate.
- Oracle after the last phase: wait for quiescence (poll `ValidateDataConsistency` until it passes twice ~15s apart, the importer stops, or a timeout), latching `stopped` the first time `IsStopped()` returns true. Then record `stopped`, the `ValidateDataConsistency` error, per-partition counts when relevant, the stop reason (the `error executing batch` / `ERROR ... yugabytedb.go` lines and `GetImportCommandStderr()`), and the count of `unexpected rows affected` WARNs. Don't use the raw count of `ERROR` lines: healthy runs log 10–30 of them (framework polling before a phase starts, expected retries).
- **In the run**, log one grep-able line per case (`t.Logf("DI-RESULT test=… outcome=… …")`) and never `t.Fatal` on a mismatch — one failing assertion must not hide the rest of the evidence. On any outcome other than CONSISTENT, call `t.Fail()` (not `Fatal`) so the framework **keeps the export dir** (it is deleted when a test passes), and log its path. On a mismatch, also dump the differing rows from both sides and the matching queue lines (`<export-dir>/data/queue/*.ndjson`) so the finding can be attributed without a rerun.
- **In the PR** (see SKILL Step 6) the test asserts the correct behaviour — `require.NoError(t, lm.ValidateDataConsistency(...))` plus a single `require.False(t, lm.GetImportRunner().IsStopped())`, or an early return on refusal — so it fails until the bug is fixed. See `templates/example_case_test.go.tmpl`.
- Case-sensitive names: quote in SQL (`"Sch"."Tbl"`), pass unquoted schema names in `SchemaNames`, and keep the `snapshot` map keys in `"schema"."table"` form.

## Running

```bash
source $SCRATCH/env.sh; cd <worktree>/yb-voyager
go test -tags <tag> -count=1 -v -parallel <P> -timeout 60m -run '<regex>' ./src/testlivemigration/ > $S/di-<batch>.log 2>&1
```

- Parallelism `P = clamp(floor((DockerMemGB - 1) / 1.4), 1, 4)`. On a 4 GB Docker VM that's 2; running 3 worked but was tight.
- Run in the background; batches of ≤ P tests; wait for completion notifications rather than polling.
- **Fail fast.** About two minutes after a batch starts, check that every test's export actually started: `grep -a -E 'Missing dependencies|Failed to start application|Export of data failed' <log>` must be empty. Otherwise stop the batch and fix the environment — a broken environment otherwise shows up only as a snapshot timeout minutes later.
- **Filter log watchers.** Before a phase starts, the framework polls `get data-migration-report` every couple of seconds and logs its failures, which floods any watcher. Match only what matters: `grep -a --line-buffered -E 'DI-RESULT|^--- |^(ok|FAIL)|panic:|Missing dependencies|Export of data failed|error executing batch'`.
- **Stopping tests.** Kill only the test binary: `pgrep -f 'testlivemigration\.test' | xargs -r kill`. Never `pkill -f` a pattern that also appears in your own command line (it kills your shell). Then remove leftover containers the run created.
- **Repeat race-prone cases.** Ordering races are probabilistic: run kill/resume, recycle and swap cases at least twice (`-count=2`), and more when the budget allows.
- Tests preserve the export dir **only when they fail** (`t.Fail`) — see Writing a container test.
- Summarise with `grep -a -E 'DI-RESULT|^--- |^(ok|FAIL)'` over the log.

## Classifying outcomes

| Outcome | Meaning | Action |
|---|---|---|
| `CONSISTENT` | target matches source | none |
| `REFUSED` | voyager refused at export/import with a clear message | fine if `expect: refused`; if `expect: consistent`, report as **unexpected refusal** (usability, not data loss) |
| `LOUD` | importer/exporter exited with an error (latched `stopped` is true, or a stop reason is logged) | fine if `expect: loud`; otherwise **unexpected loud failure** → report, no PR by default |
| `SILENT` | mismatch while the importer is still running after quiescence, or it exited 0 (WARN-only counts as silent) | **candidate bug** → verify (below) |
| `SNAPSHOT_INCOMPLETE` | snapshot never reached the expected counts | log counts; if source > target with no error, treat as `SILENT` (M4); if an error, `LOUD` |
| `INCONCLUSIVE` | mismatch plus timeout, unclear | rerun once with a longer timeout; else report |
| `TEST_INVALID` | source SQL failed, DDL unsupported, wrong expected counts | fix the test and rerun once; else drop with the reason |

## Verifying a SILENT candidate (all must hold before a PR)

1. **Reproduces**: rerun the case alone twice more; at least 2 of 3 runs `SILENT` (races may be probabilistic — record the rate).
2. **Real divergence**: dump the differing rows from both sides (not just the first mismatch) and confirm the source rows are what the workload should produce.
3. **Attributed**: say where it happens — never captured (queue lacks the events: M4), transformed (queue has a different value: M5), or applied wrongly (queue correct, target wrong: M1–M3/M6). Quote the evidence (queue line, log WARN, SQL shape).
4. **Minimal**: shrink rows, statements, tables and flags while it still fails; the PR test must be the smallest repro.
5. **Not already reported**: search open PRs and issues:
   `gh pr list --repo yugabyte/yb-voyager --state open --search '"[data-integrity]" in:title'` and `gh issue list --search '<key terms>'`. Same signature → add a comment with the new evidence instead of a new PR.
6. **Signature**: `<mechanism>|<schema shape>|<flags>|<workload>` in one line (e.g. `M3|partitioned root w/o PK, leaf PK(id), same id in 2 leaves|use-partition-root=true|update/delete only`). Used for dedupe.

## Cleanup (always, even on failure)

`failpoint-ctl disable` if enabled; stop leftover test binaries (see Running); `docker ps` — remove only containers the run created (`di-probe*`, testcontainers exit on their own); remove every worktree the run added; keep logs and the report in `$SCRATCH/data-integrity/`.
