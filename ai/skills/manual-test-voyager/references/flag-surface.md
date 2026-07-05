# Flag surface & interaction traps

The knobs most likely to expose bugs, and the flag *interactions* that scripted tests under-cover. When a diff touches any of these, the plan must include the boundary and negative cases here. Flag defaults/behavior drift — re-read `yb-voyager/cmd/import.go`, `export.go`, `importDataFileCommand.go` when in doubt.

## import data — parallelism / batching (highest-value)

| Flag | Default | Notes / interactions |
| --- | --- | --- |
| `--parallel-jobs` | `0` → auto (≈ clusterCores/4; falls back to nodes×16 if detect fails) | Import parallelism = conn-pool size. **Mutually exclusive with adaptive-parallelism enabled** (errors). |
| `--adaptive-parallelism` | `balanced` (YB) / `disabled` (PG, yugabytedb-amp) | `disabled`/`balanced`/`aggressive`. Adjusts pool every ~10s off cluster CPU/mem (balanced threshold 80%, aggressive 95%). |
| `--adaptive-parallelism-max` | `0` → parallel-jobs×4 (≈ clusterCores) | Ceiling when adaptive enabled. **Only valid when adaptive enabled** (else errors). **The anchor-bug knob.** |
| `--batch-size` | `0` → YB 20000 / PG 100000 / Oracle 10000000 | Rows per batch. Rejected if **greater than the target default**. |

**Interaction traps (each is a required test case):**
- `--adaptive-parallelism balanced --adaptive-parallelism-max 1` (and `= 0`, `= negative`, `= just below clusterCores/4`) → must **cap+warn or fast-error, never hang**. Historically deadlocked at pool init because `NumConnections > NumMaxConnections` drained a channel sized only `NumMaxConnections`. Both fixes now exist: reconcile caps `Parallelism = MaxParallelism` with a warning (`tgtdb/yugabytedb.go` `reconcileAdaptiveParallelism`), and the pool rejects `NumConnections > NumMaxConnections` (`tgtdb/conn_pool.go`).
- `--parallel-jobs 4 --adaptive-parallelism balanced` → must reject (mutually exclusive).
- `--adaptive-parallelism-max 8 --adaptive-parallelism disabled` → must reject.
- `--target-db-type yugabytedb-amp` with any enabled `--adaptive-parallelism` or any `--adaptive-parallelism-max` → must reject (AMP has no cluster-control API).
- `--batch-size` above the target default → must reject with the limit named.
- Runtime resize: adaptive loop steps ±1 within `[1, NumMaxConnections]` every `ADAPTIVE_PARALLELISM_FREQUENCY_SECONDS` (env, default 10). Env knobs also worth fuzzing: `MAX_CPU_THRESHOLD`, `MIN_AVAILABLE_MEMORY_THRESHOLD`.

The same parallelism flags exist for `import data to source` / `import data to source-replica` (fall-back / fall-forward) with different auto defaults (PG N/2, Oracle 16) — cover them when live scope is in play.

## import data — error handling / data safety

| Flag | Default | Notes |
| --- | --- | --- |
| `--error-policy-snapshot` | `abort` | `abort` vs `stash-and-continue` on row read/transform/ingest error. `import data file` uses `--error-policy` (same semantics). |
| `--on-primary-key-conflict` | `ERROR-POLICY` | vs `IGNORE` (skip existing-PK rows). |
| `--enable-upsert` | `false` | **Corrupts secondary indexes if present** — high-value bug target; test with a table that has secondary indexes. |
| `--start-clean` | `false` | Fresh import; on non-empty tables prompts, and if continued imports in upsert mode (duplicate-row risk without PK). |
| `--truncate-tables` | `false` | **Only valid with `--start-clean true`** (else errors). |
| `--skip-replication-checks` | `false` | Skips the guardrail that aborts import when replication/xCluster is detected. |
| `--disable-transactional-writes` (hidden) | `false` | Non-transactional COPY. |

**Traps:** `--truncate-tables true` alone (no `--start-clean`) → reject; `--enable-upsert` on a table with secondary indexes → verify index integrity after (oracle 3 content hash on the indexed column); `--start-clean` re-run on populated tables → no duplicates.

## export data / import data file

| Flag | Default | Notes |
| --- | --- | --- |
| `--export-type` (export data) | `snapshot-only` | `snapshot-and-changes` switches on live mode (no separate live flag). |
| `--parallel-jobs` (export data) | `4` | Source-side export parallelism. |
| `--table-list` / `--exclude-table-list` (+ `-file-path`) | `""` | Glob `?`/`*`; case-sensitivity via quoting. **Partitioned tables + table-list is a classic edge case** — does listing the parent include partitions? |
| import data file: `--file-table-map`, `--escape-char`, `--quote-char`, `--null-string`, `--file-opts` (deprecated) | — | CSV parse knobs; malformed CSV / wrong quote char → error-policy behavior. |

## Negative cases beyond flags (QA "negative & edge" focus)

- **Unreachable host** — wrong `--source-db-host` / `--target-db-host` or a down port: must fail fast with a clear connection error, not hang or emit a stack trace.
- **Wrong credentials / missing DB** — clear auth/does-not-exist error.
- **Wrong sub-command / unknown flag** — cobra usage error naming the mistake.
- **Missing / stale export dir** — running `import data` before `export data`, or a `.*Lockfile.lck` left by a killed run (delete `<exportDir>/*.lck` to recover — a documented gotcha).
- **Interrupted run then resume** — see the state-transition oracle.
- **Multi-schema** — source with 2+ schemas: object routing, name collisions across schemas, and `--source-db-schema a,b` handling.

## Commonly impacted areas (always spot-check, even if the diff looks unrelated)

Per QA guidance: **partitions, sequences, status commands, multi-schema**. Fold at least one scenario per relevant area into every plan. See the matching entries under `regression-library/`.
