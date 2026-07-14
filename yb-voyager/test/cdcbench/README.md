# cdcbench — CDC ingest benchmark framework

Benchmarks the **live-migration import streaming path** (`streamChangesFromSegment` and
everything under it: NDJSON decode, name-registry lookups, Debezium value conversion,
conflict-detection cache, event channels, batching) against **real Debezium-generated
change events**, with exactly one mock: `TargetDB.ExecuteBatch` succeeds without a
database.

Each workload's events are produced once by a real `yb-voyager export data`
(snapshot-and-changes) run against a testcontainers PostgreSQL source
(`wal_level=logical`), then cached on disk and replayed by the benchmark.

## Running

Requires: Docker (artifact generation only) and an installed `yb-voyager` with
debezium-server (generation only — cached artifacts replay with neither).

```bash
cd yb-voyager

# full suite, 5 runs each (benchstat-ready)
go test -tags cdc_benchmark -run '^$' -bench CDCIngest -benchtime 1x -count 5 ./cmd/

# a single workload
go test -tags cdc_benchmark -run '^$' -bench 'CDCIngest/edge-all-updates' -benchtime 1x ./cmd/

# compare two checkouts
go test -tags cdc_benchmark -run '^$' -bench CDCIngest -benchtime 1x -count 5 ./cmd/ > before.txt
# ...switch branches / apply change...
go test -tags cdc_benchmark -run '^$' -bench CDCIngest -benchtime 1x -count 5 ./cmd/ > after.txt
benchstat before.txt after.txt
```

Reported metrics per workload: `events/s` (ingest throughput), `conflicts/op`,
`batches/op`, `cache-depth-avg` / `cache-depth-max` (conflict-cache occupancy, sampled
every 100ms). Each run also **asserts** its conflict expectation (zero for no-conflict
workloads, non-zero for conflict workloads) and the exact processed-event count.

Output comes in two forms: standard `Benchmark...` lines (machine-readable, what
benchstat consumes) and a human summary table printed after all workloads:

```
--- cdcbench summary ---
WORKLOAD                   RUNS  EVENTS/S  CONFLICTS/RUN  BATCHES/RUN  CACHE DEPTH AVG/MAX  TIME/RUN
inserts-no-uk              1     128,857   0              100          0 / 0                155ms
updates-uk-conflict-pairs  1     30,444    13,961         19,828       1 / 2                657ms
```

Flag semantics worth knowing: `-benchtime 1x` sets iterations *within* one benchmark
execution (`b.N`, one "op" = one full artifact replay); `-count 5` runs each benchmark
5 separate times producing 5 output lines — the independent samples benchstat needs.

## Knobs (env vars)

| Variable | Meaning |
|---|---|
| `CDCBENCH_VOYAGER_BIN` | Path to the `yb-voyager` binary used for artifact generation (default: `PATH` lookup). |
| `CDCBENCH_REGEN=1` | Force artifact regeneration even when cached. |
| `CDCBENCH_ARTIFACT_DIR` | Artifact cache location (default: `test/cdcbench/artifacts/`, gitignored). |
| `CDCBENCH_EXEC_DELAY_MS` | Simulated target batch-commit latency in the `ExecuteBatch` mock (default 0). |
| `CDCBENCH_LOG_DIR` | Persist per-run import logs (Info level) for manual verification of conflict lines; default discards them (the level is Info either way, matching production). |

## Adding a workload

1. Pick a name: `<category>-<description>` where category is one of `oltp-`
   (realistic pattern), `schema-` (shape probe), `edge-` (op-mix corner case),
   `conflict-` (engineered real conflicts). The name IS the testdata directory
   and the benchmark sub-name — one string everywhere.
2. Create `testdata/<name>/{schema,seed,dml}.sql`:
   - `schema.sql` — tables with PKs/unique constraints **and `ALTER TABLE ... REPLICA IDENTITY FULL`** (required for before-images);
   - `seed.sql` — initial rows (exported in the snapshot, not as change events);
   - `dml.sql` — runs while Debezium streams; every row change becomes one event.
3. Register it in `workloads_catalog.go`:

```go
Register(testdataWorkload("oltp-my-workload", []string{"my_table"}, 20_000, false))
```

(arguments: name, exported table list, exact event count of dml.sql, whether
the workload expects conflict detections.)

That's the whole cost: the workload is immediately runnable in isolation via
`-bench 'CDCIngest/oltp-my-workload'`. Artifacts are cached keyed by a content
hash of the workload definition, so editing any SQL auto-invalidates only that
workload's artifact.

Guidance for conflict-free workloads: make every unique-key value globally unique
across seed **and** DML (`nil==nil` also counts as a conflict). For conflict workloads,
engineer collisions where one event's after-image equals another in-flight event's
before-image (see `conflict-update-pairs/dml.sql`).

## Architecture

`test/cdcbench` knows nothing about the `cmd` package. The three pieces that need cmd
internals are injected as closures (`Hooks`) by the shim `cmd/cdc_ingest_bench_test.go`
(build tag `cdc_benchmark`): `Bootstrap` (prepare metaDB/name registry/table list for
an artifact copy and install the mock), `StreamAll` (calls the real `streamChanges` —
the timed region), and `CacheDepth` (conflict-cache occupancy for the sampler).
Everything else — workload registry, artifact generation & caching, the mock (whose
`ExecuteBatch` is a no-op and whose target-side streaming-metadata queries are answered
by an in-memory store with fresh-migration values), conflict counting (logrus hook),
metrics and assertions — lives here.

Generated artifacts are self-contained: the framework patches the YB-side names into
the artifact's name registry (a live `import data` would register them against a real
target). Artifact generation requires a `yb-voyager` binary recent enough to write
multi-column unique-index metadata; artifacts produced by older binaries fail loudly
at conflict-cache initialization and must be regenerated (`CDCBENCH_REGEN=1`).

Known accepted limitation: the depth sampler reads the conflict-cache pointer while
the stream's first event initializes it (a benign, one-time unsynchronized access —
benchmark artifacts are single-exporter-role, so the pointer never changes mid-run).
Expect `go test -race` to flag it.
