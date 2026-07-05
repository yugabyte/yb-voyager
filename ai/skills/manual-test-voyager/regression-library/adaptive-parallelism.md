# Adaptive parallelism / connection pool

Why it matters: `import data` parallelism is computed from cluster cores and bounded by `--adaptive-parallelism-max`; the pool's invariant is `Parallelism <= MaxParallelism`. A user-supplied max below the auto-computed parallelism used to violate that invariant and **deadlock at pool init** — a silent hang, not an error. This is the anchor bug class for the whole skill: a flag interaction no unit test covered.

Relevant code: `tgtdb/yugabytedb.go` `reconcileAdaptiveParallelism` (caps + warns), `tgtdb/conn_pool.go` (rejects `NumConnections > NumMaxConnections`), `adaptiveparallelism/`.

## Scenarios

### adaptive-parallelism-1: max below auto jobs (the hang)
- Flow: offline (path is shared with live import)
- Origin: production bug — adaptive-parallelism-max < default parallel jobs → import hangs, no data imported
- Setup: base fixture (any multi-table dataset); a YB target where auto jobs > 1
- Command: `import data ... --adaptive-parallelism balanced --adaptive-parallelism-max 1 --disable-pb true`
- Expected oracle: completes; log shows parallelism clamped (e.g. `Using 1-1 parallel jobs (adaptive)`); full row+content parity. **`timeout` exit 124 ⇒ Critical hang.**
- Status: **validated** — passes on this worktree (both fixes present); imported orders=5000/events=4000/customers=1000/widgets=500 with parallelism clamped to 1-1, no hang.

### adaptive-parallelism-2: max = 0 / negative / just-below-auto
- Flow: offline
- Origin: exploratory boundary fuzz around the hang
- Setup: base fixture
- Command: repeat -1 with `--adaptive-parallelism-max` ∈ {0, -1, floor(clusterCores/4)-1}
- Expected oracle: each either caps+warns or fast-errors with a clear message; never hangs, never partial-imports silently.
- Status: seeded

### adaptive-parallelism-3: mutually-exclusive rejection
- Flow: offline
- Origin: guardrail `validateParallelismFlags`
- Setup: any
- Command: `import data ... --parallel-jobs 4 --adaptive-parallelism balanced`
- Expected oracle: fast reject naming both flags (mutually exclusive). Also test `--adaptive-parallelism-max 8 --adaptive-parallelism disabled` → reject.
- Status: seeded

### adaptive-parallelism-4: yugabytedb-amp rejects adaptive
- Flow: offline
- Origin: AMP has no cluster-control API
- Setup: `--target-db-type yugabytedb-amp`
- Command: `import data ... --adaptive-parallelism balanced` (and `... --adaptive-parallelism-max 4`)
- Expected oracle: reject with a message explaining AMP doesn't support adaptive parallelism.
- Status: seeded

### adaptive-parallelism-5: runtime resize under load
- Flow: offline (large fixture) or live
- Origin: exploratory — the adaptive loop steps ±1 every ~10s within `[1, max]`
- Setup: a fixture large enough that import runs > ~30s; optionally set `ADAPTIVE_PARALLELISM_FREQUENCY_SECONDS=2`
- Command: `import data ... --adaptive-parallelism aggressive`
- Expected oracle: pool size stays within `[1, max]`; no connection leak; full parity at the end.
- Status: seeded
