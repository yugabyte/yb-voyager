# Running data-integrity tests

## Environment preconditions (check all; fail the run with a clear message if any is missing)

| Need | Check | Notes |
|---|---|---|
| Docker | `docker info` | Note `MemTotal`; each container test runs a PG + YB container pair (~1.2–1.5 GB) plus exporter, importer and a Debezium JVM on the host. |
| Go | `go version` matches `yb-voyager/go.mod` | |
| Debezium server + voyager install | `/opt/yb-voyager/debezium-server` exists | Tests exec `yb-voyager`, which starts Debezium from the install. |
| Freshly built `yb-voyager` **first on PATH** | build from the target commit into `$SCRATCH/bin`, `PATH=$SCRATCH/bin:$PATH` | The framework execs `yb-voyager` from PATH. A stale installed binary silently tests old code. Verify with `yb-voyager version` → `GIT_COMMIT_HASH` = target commit. |
| failpoints (only if a case uses them) | `failpoint-ctl enable` in `yb-voyager/` before building/testing | Always `failpoint-ctl disable` afterwards and before any commit; `git status` must show only new test files. |
| Clean env | `unset CONTROL_PLANE_TYPE YUGABYTED_DB_CONN_STRING`; `export YB_VOYAGER_SEND_DIAGNOSTICS=0` | A developer shell with control-plane vars makes **every** export fail at startup (connection refused to yugabyted) — looks like a test failure. |

## Workspace

- Work in a **dedicated git worktree** at the target commit (`git worktree add --detach $SCRATCH/di-wt <sha>`); never in the user's checkout. If the session cannot create worktrees, use a throwaway branch in its own worktree.
- Generated tests go in `yb-voyager/src/testlivemigration/data_integrity_<plan-date>_<case>_test.go` (container) and `yb-voyager/cmd/data_integrity_fuzz_<plan-date>_test.go` (fuzz). Container tests use the existing framework only (`NewLiveMigrationTest`, `TestConfig`, `Start*`/`Wait*`/`Initiate*` methods, `ValidateDataConsistency`, `testutils.CompareTableData`) — do not add shared helper files. Copy the fuzz engine templates into `cmd/` only when fuzz cases exist (engine: `fuzz_engine_test.go.tmpl`, partition model: `fuzz_partitions_test.go.tmpl`; both `//go:build unit`).
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
- Doc comment: the mechanism, the exact sequence that would lose data, and the expected outcome.
- Map plan fields straight onto `TestConfig` (`SchemaSQL`, `SourceSetupSchemaSQL`, `InitialDataSQL`, `SourceDeltaSQL`, `CleanupSQL`, `SourceReplicaDB` for fall-forward). Phases become `lm.ExecuteOnSource(...)` / `ExecuteOnTarget(...)` (DDL drift), `lm.GetImportRunner().Kill()` + removing `<export-dir>/.*Lockfile.lck` + `ResumeImportData`, `StopImportData` / `ResumeImportData`, `InitiateCutoverToTarget` / `InitiateCutoverToSource`, `WaitForForwardStreamingComplete` (only when exact counts are known).
- A refusal shows up as an error from `StartExportData` / `StartImportData*`, or as `WaitForSnapshotComplete` timing out with the process stopped — check stderr (`GetExportCommandStderr` / `GetImportCommandStderr`) to tell which.
- Oracle after the last phase: wait for quiescence (poll `ValidateDataConsistency` until it passes twice ~15s apart, the importer stops, or a timeout), then record `GetImportRunner().IsStopped()`, the `ValidateDataConsistency` error, per-partition counts when relevant, and counts of `ERROR`/`FATAL`/`unexpected rows affected` lines in `<export-dir>/logs/yb-voyager-import-data.log`.
- **In the run**, log one grep-able line per case (`t.Logf("DI-RESULT test=… outcome=… …")`) and never `t.Fatal` on a mismatch — one failing assertion must not hide the rest of the evidence. Log anything you want to inspect later inside the test, because the export dir is deleted when a test passes.
- **In the PR** (see SKILL Step 6) the test asserts the correct behaviour — `require.NoError(t, lm.ValidateDataConsistency(...))` plus `require.False(t, lm.GetImportRunner().IsStopped())`, or an early return on refusal — so it fails until the bug is fixed. See `templates/example_case_test.go.tmpl`.
- Case-sensitive names: quote in SQL (`"Sch"."Tbl"`), pass unquoted schema names in `SchemaNames`, and keep the `snapshot` map keys in `"schema"."table"` form.

## Running

```bash
S=$SCRATCH; cd <worktree>/yb-voyager
PATH=$S/bin:$PATH YB_VOYAGER_SEND_DIAGNOSTICS=0 \
  go test -tags <tag> -count=1 -v -parallel <P> -timeout 60m -run '<regex>' ./src/testlivemigration/ > $S/di-<batch>.log 2>&1
```

- Parallelism `P = clamp(floor((DockerMemGB - 1) / 1.4), 1, 4)`. On a 4 GB Docker VM that's 2; running 3 worked but was tight.
- Run in the background; batches of ≤ P tests; wait for completion notifications rather than polling.
- Tests preserve the export dir **only when they fail** (`t.Fail`). To inspect a passing-but-suspicious run, log what you need (queue lines, log excerpts, row dumps) *inside* the test before it returns.
- Summarise with `grep -a -E 'DI-RESULT|^--- |^(ok|FAIL)'` over the log.

## Classifying outcomes

| Outcome | Meaning | Action |
|---|---|---|
| `CONSISTENT` | target matches source | none |
| `REFUSED` | voyager refused at export/import with a clear message | fine if `expect: refused`; if `expect: consistent`, report as **unexpected refusal** (usability, not data loss) |
| `LOUD` | importer/exporter exited with an error, or ERROR logged | fine if `expect: loud`; otherwise **unexpected loud failure** → report, no PR by default |
| `SILENT` | mismatch while running or exited 0, no ERROR/FATAL (WARN-only counts as silent) | **candidate bug** → verify (below) |
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

`failpoint-ctl disable` if enabled; `docker ps` — remove only containers the run created (`di-probe*`, testcontainers exit on their own); remove the worktree; keep logs and the report in `$SCRATCH/data-integrity/`.
