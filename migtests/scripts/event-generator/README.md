## Event Generator (PostgreSQL / YugabyteDB)

Generates randomized INSERT/UPDATE/DELETE traffic against PostgreSQL **or YugabyteDB** tables for testing and migration exercises — driving load on a source before/during export, or on a target during live-migration cutover/fall-back. Configuration-driven, reproducible when seeded, and type-aware (arrays, enums, bit/varbit, numeric precision/scale, etc.). Runs single-process (`generator.py`) or as a rate-controlled dynamic worker pool (`parallel_generator.py`) that can sustain 10k+ events/sec across a multi-node cluster.

### Prerequisites
- Python 3.8+
- PostgreSQL user with permission to create the `tsm_system_rows` extension (or have it pre-created)
- Python packages:
```bash
pip install psycopg2-binary Faker PyYAML
```

### Files
- `generator.py`: Orchestrates config, DB connection, schema discovery, and event loop.
- `utils.py`: Config loading/validation, schema introspection, value generation, SQL builders, retry logic.
- `event-generator.yaml`: Configuration for connection and generator behavior.

### Configure
Edit `event-generator.yaml`:
```yaml
connection:
  host: localhost
  port: 5432
  database: sakila
  user: postgres
  password: postgres

generator:
  schema_name: public
  manual_table_list: [eg_users, eg_orders]   # empty -> discover all base tables in schema
  exclude_table_list: []                     # applied only if manual_table_list is empty
  num_iterations: 2000                       # -1 for infinite
  wait_after_operations: 0                   # throttle interval; 0 disables
  wait_duration_seconds: 0                   # sleep duration when throttling
  table_weights: { eg_users: 100, eg_orders: 100 }  # default weight=1 if omitted
  operation_weights: { INSERT: 3, UPDATE: 2, DELETE: 1 } # omit or set 0 to disable
  insert_rows: 4 # number of rows to insert per operation
  update_rows: 2 # number of rows to update per operation
  delete_rows: 1 # number of rows to delete per operation
  insert_max_retries: 50                     # retries on unique violations
  update_max_retries: 3                      # per-attempt retries for UPDATE
  random_seed: 12345                         # deterministic table/op choices and numeric data
  faker_seed: 12345                          # deterministic text, uuid, timestamps
```

Notes:
- If `manual_table_list` is empty, all base tables in `schema_name` are targeted (minus `exclude_table_list`).
- Set `num_iterations: -1` to run indefinitely.
- Seeds make runs reproducible. Omit to make runs non-deterministic.

### Rate control
An optional `rate_control` block under `generator:` (commented out by default in `event-generator.yaml`) paces the aggregate **events/second** the generator produces. An event is one changed row: `cursor.rowcount` read after each operation (an INSERT batch counts its rows; UPDATE/DELETE count whatever the sampling actually hit; a failed/rolled-back op counts 0).

```yaml
generator:
  # ...
  rate_control:
    default_events_per_second: 1500   # baseline rate when no spike is active
    report_interval_seconds: 60       # log achieved ev/s every N s (omit/0 = off)
    schedule:                         # optional list of recurring spike windows
      - events_per_second: 10000      # spike target rate
        duration_seconds: 300         #   spike lasts 5 min
        every_seconds: 1800           #   one spike per 30 min (the period)
        offset_seconds: 600           #   first 10 min of each period stay at baseline
        jitter_pct: 10                #   +/-10% randomization of spike start & rate (seeded)
```

Nullability:
- Omitted (default): no pacing — the generator runs as fast as the DB allows, unchanged from today. `wait_after_operations`/`wait_duration_seconds` still apply.
- Present, no `schedule`: steady rate at `default_events_per_second`.
- Present, with `schedule`: baseline plus recurring spike windows; when windows overlap, the max rate wins.

When `rate_control` is present, `wait_after_operations`/`wait_duration_seconds` are ignored (a one-line warning is printed at startup), since they would fight the governor.

Validation runs at startup and fails fast with a clear message: `default_events_per_second > 0`; per schedule entry `events_per_second > 0`, `duration_seconds > 0`, `every_seconds > 0`, `offset_seconds >= 0`, `0 <= jitter_pct <= 50`; and `offset_seconds + duration_seconds <= every_seconds` so a spike window fits inside its period. Unknown keys print a non-fatal warning (typo guard) rather than failing.

Example — the 24h fall-back (YB→PG) throughput test: a ~1.5k ev/s baseline with a 10k ev/s spike for 5 min every 30 min, a 10-min baseline lead-in each period, and ±10% jitter:
```yaml
rate_control:
  default_events_per_second: 1500
  report_interval_seconds: 60
  schedule:
    - events_per_second: 10000
      duration_seconds: 300
      every_seconds: 1800
      offset_seconds: 600
      jitter_pct: 10
```
Run with `num_iterations: -1` and e.g. `timeout 24h python3 generator.py -c event-generator.yaml`.

### Parallel runs

`generator.py` is single-process. For rates beyond what one process can push
against the DB, `parallel_generator.py` wraps it: it launches N unmodified
copies of `generator.py`, each pacing a slice of the total rate, and reports
the combined throughput. `generator.py` itself is never changed.

Add a top-level `parallel` block to a normal config (connection + generator,
including a `rate_control` block whose rates are the **desired TOTAL across
all workers**, not per-worker). A fully-commented template of every field is
in `event-generator.yaml`; the most common ones:
```yaml
parallel:
  run_seconds: 1800            # total run duration after calibration (Ctrl+C/SIGTERM also stop)
  max_workers: 8               # hard cap on worker processes (shortfall is logged if the peak
                               #   target needs more)
  calibration_seconds: 30      # length of the per-worker-ceiling (C) measurement window
  calibration_warmup_seconds: 20  # warm the probe worker before measuring C (avoids a cold,
                               #   ~3-4x-too-low C that causes spike overshoot)
  allow_throttle: true         # cascade controller: whole-worker coarse knob + one fractional
                               #   trimmer worker for fine adjustment
  distribute_across_nodes: true   # round-robin workers across all tservers (via yb_servers())
```

How the reactive controller works (unlike the older fixed-count model, worker
count is **not** computed up front — it adapts during the run):
1. **Calibrate**: warms one probe worker for `calibration_warmup_seconds`, then
   measures its steady-state throughput over `calibration_seconds` to get the
   per-worker ceiling **C** (events/sec), via `pg_stat_statements`
   (`SUM(rows)` over insert/update/delete). Warm-up matters: a cold C reads
   3-4x low and makes the feed-forward step over-provision, overshooting a spike.
2. **Feed-forward**: on each target change (baseline↔spike) it jumps the worker
   count to `ceil(target / C)` so it reaches the new rate fast instead of
   ramping one worker at a time.
3. **React** (every `control_interval_seconds`): compares the achieved rate
   (a `meter_window_seconds` trailing average) to target and adjusts — a
   **coarse** knob adds/removes whole workers, and (when `allow_throttle`) a
   **fine** trimmer throttles a single worker for sub-worker precision. A
   backpressure cap (`reactive_margin`) stops it piling workers on a
   cluster-bound target. C is re-estimated over the run (`recalibrate`).
4. **Distribute**: with `distribute_across_nodes`, each worker is pinned to a
   specific tserver round-robin (see "Multi-node clusters" below).
5. **Shutdown**: on `run_seconds` elapsed or Ctrl+C/SIGTERM, all workers are
   reaped (SIGTERM then SIGKILL; workers also die automatically if the
   controller is killed), and a `--rate-csv` time series is written if given.

Requires `pg_stat_statements` (`shared_preload_libraries = 'pg_stat_statements'`
plus `CREATE EXTENSION pg_stat_statements;`) -- calibration and monitoring
fail fast with a clear message if it's unavailable. Full controller internals
(the cascade math, recalibration, overshoot-shed) are in `ARCHITECTURE.md`.

Write a per-second throughput CSV with `--rate-csv <path>` (columns:
epoch, t_seconds, target, achieved, n_uncapped_workers, trimmer, C, trimmer_rate).

#### Multi-node clusters (node distribution)

By default (`parallel.distribute_across_nodes: true`) the controller discovers
every tserver via `yb_servers()` and **assigns each worker a specific node,
round-robin**, overriding host/port in that worker's config so it connects
directly to its node. This spreads write/coordination load evenly across the
cluster instead of piling it onto the single configured host (which overloads
that one node — a tserver heartbeat timeout — while the rest sit idle).

This is done explicitly rather than via a driver-level connection load
balancer (e.g. the YugabyteDB smart driver) on purpose: each worker is a
**separate process opening a single connection**, so a per-process balancer has
nothing to balance and every process independently lands on the same seed node.
Explicit round-robin from the controller is the only thing that actually
distributes a one-connection-per-process fleet.

Because writes are then spread across nodes, the controller also sums
`pg_stat_statements` across all nodes for its rate measurement. Set
`distribute_across_nodes: false` only if the nodes' direct host/port aren't
reachable (e.g. a VIP-only deployment) — then all workers use the one
configured host.

Run with:
```bash
python3 parallel_generator.py -c event-generator.yaml
```

### Run
From the folder:
```bash
# Default: uses event-generator.yaml in this folder
python3 generator.py

# Provide a custom config path
python3 generator.py -c /path/to/event-generator.yaml
python3 generator.py --config ./configs/dev.yaml
```
- When not provided, the script loads `event-generator.yaml` that sits next to `generator.py`.
- The script ensures `tsm_system_rows` exists:
  - Requires `CREATE EXTENSION tsm_system_rows;` privileges (superuser or granted).
- Stop anytime with Ctrl+C. The connection is closed cleanly.

### How it works (summary)
- Discovers target tables and their schemas (columns, primary key, array element types, enum labels, bit/varbit metadata, numeric precision/scale).
- Chooses a table and operation per configured weights each iteration.
- INSERT: builds type-compatible VALUES for a batch; retries on unique violations by regenerating data.
- UPDATE: picks 1..N non-PK columns, generates a type-aware SET clause, and targets rows using `TABLESAMPLE SYSTEM_ROWS(update_rows)`.
- DELETE: deletes rows sampled via `TABLESAMPLE SYSTEM_ROWS(delete_rows)`.
- Optional throttling after a configured number of operations.

### Unique-value generation and its caveats (parallel/dynamic-worker mode)

In parallel mode (workers launched with `--worker-uid`, i.e. the dynamic worker
pool), the generator produces **collision-free values for every single-column
unique surface** — primary keys *and* secondary UNIQUE constraints/indexes —
instead of generating them randomly and colliding as tables fill. This removes
the retry storm that otherwise amplifies cluster load far beyond the achieved
insert rate (a full node's worth of wasted execute/rollback churn). Each worker
occupies a disjoint value range (`worker_uid` + `pk_stride`), so no two workers
ever produce the same value.

Coverage by column type (single-column unique surfaces):

| Type | How a unique value is produced | Guaranteed unique? |
| --- | --- | --- |
| `integer` / `bigint` / `numeric` | monotonic: `max(col) + 1 + worker_uid + pk_stride*counter` | yes, until the column type's own max (then random fallback) |
| `text` / `varchar` / `char` | worker-namespaced string `u<uid>_<counter>` (compact base36 form to fit small `char_max`) | yes |
| `uuid` | `uuid5(NAMESPACE_OID, "<uid>:<counter>")` | yes |
| anything else | falls back to normal random generation | best-effort (unique-violation retry still applies) |

For a **composite** unique constraint, one representative column (preferring an
integer-typed one) is made unique-safe, which guarantees the whole tuple is
unique.

#### Caveats (read before a long soak)

1. **Narrow-integer unique columns are best-effort.** Past the column type's max
   the scheme falls back to random. A UNIQUE `smallint` (max 32,767) can hold at
   most ~65k distinct values and cannot grow beyond that regardless — it will
   collide constantly on the random fallback. A UNIQUE `integer` degrades only at
   hundreds of millions of rows; `bigint` has effectively unlimited headroom.
   *Action:* scan a new source schema for **single-column** unique `smallint`
   columns on tables meant to grow large, and exclude/prep those tables.
2. **UPDATE never modifies unique columns.** To avoid re-introducing collisions,
   unique columns (PK + secondary) are excluded from UPDATE `SET` lists. This is
   realistic (apps rarely rewrite unique keys) but means CDC UPDATE events won't
   carry changes to those columns, and a table whose only non-PK columns are all
   unique is skipped for UPDATEs (INSERT/DELETE still run). **Open question:**
   revisit whether some update-to-unique coverage is wanted (would require
   generating unique-safe values in the UPDATE path too).
3. **Composite uniques rely on one representative column.** If that column is a
   narrow integer that falls back to random, the tuple's guarantee weakens to
   best-effort. A composite made entirely of unhandled/exotic types has no
   deterministic representative and is best-effort.
4. **Exotic unique types fall back to random** (e.g. unique `timestamp`, `inet`,
   `bytea`, `money`, arrays). Collision-prone but retry-covered, never fatal.
5. **Tiny `char_max`** (roughly `varchar(<4)`) unique columns may not fit even the
   compact encoding and fall back to random.
6. **Seeds are point-in-time.** `MAX(col)` seeds are captured at cache-build time;
   external writers or restarts with pre-existing higher values can make an
   integer seed stale (retry-covered). Disjointness assumes `worker_uid <
   pk_stride` (default 100,000) — safe unless a single run spawns ~100k workers.

All fallback cases degrade *gracefully* to random-plus-retry (they cannot crash a
worker or a node) — they just insert slightly slower on those specific columns.

#### Reference: real anonymized real-world schema (341 tables) snapshot

Unique-surface columns by type, as measured on the test schema — re-run this
check when pointing at a new source to re-validate the caveats above:

| Type | single-column unique | in composite unique |
| --- | --- | --- |
| varchar | 374 | 176 |
| text | 42 | 104 |
| integer | 1 | 42 |
| bigint | 1 | 1 |
| uuid | 1 | 0 |
| smallint | **0** | 1 |
| boolean / date / timestamp | 0 | 58 |

Takeaway for this schema: **no single-column unique `smallint`** (caveat 1 does
not bite), and every single-column unique surface is a fully-handled type. The
lone smallint and the boolean/date/timestamp uniques appear only inside composite
constraints, covered by their representative column.

### Type handling (high level)
- Textual (`text`, `varchar`, `character varying`, `bytea`), booleans, integers, numeric/decimal (respects precision/scale), date/time/timestamp, `json/jsonb`, `inet`, `uuid`, `tsvector`.
- Arrays: text and integer arrays supported out-of-the-box.
- Enums: scalar enum columns supported via discovered labels.
- Bit/varbit: generates correctly cast literals based on column metadata.

Limitations:
- Array generation is implemented for text and integer; other array element types may default to NULL.
- Enum arrays aren’t explicitly generated as arrays of labels.
- UPDATE/DELETE assume a primary key; tables without PK may be skipped or error depending on setup.

### Tuning tips
- Use `table_weights` and `operation_weights` to bias traffic.
- Increase `insert_rows`/`update_rows`/`delete_rows` for heavier batches.
- Throttle with `wait_after_operations` and `wait_duration_seconds` to reduce load.
- Use seeds to debug or record deterministic sequences.
