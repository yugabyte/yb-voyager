## Event Generator (PostgreSQL)

Generates randomized INSERT/UPDATE/DELETE traffic against PostgreSQL tables for testing and migration exercises. Configuration-driven, reproducible when seeded, and type-aware (arrays, enums, bit/varbit, numeric precision/scale, etc.).

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
all workers**, not per-worker):
```yaml
parallel:
  max_workers: 6              # hard cap on worker processes
  calibration_seconds: 30     # how long the one-shot calibration run lasts
  margin: 1.3                 # safety headroom: target workers for 1.3x peak
  run_seconds: 1800           # total wall-clock run time
  monitor_interval_seconds: 5 # how often to print the aggregate rate
```

How it derives the worker count:
1. **Calibrate**: runs a single uncapped worker (no `rate_control`) for
   `calibration_seconds`, measuring events/sec via `pg_stat_statements`
   (`SUM(rows)` over insert/update/delete statements) to get a per-worker
   throughput ceiling.
2. **Derive**: takes the peak target -- the max of
   `rate_control.default_events_per_second` and every `schedule` entry's
   `events_per_second`, so a spike is servable -- and computes
   `workers = ceil(peak * margin / per_worker_ceiling)`, clamped to
   `max_workers`. If the clamp bites, it prints the requested vs. achievable
   rate.
3. **Spawn**: writes `workers` per-worker YAML configs (each with
   `generator.random_seed`/`faker_seed` = base seed + worker index, and
   `rate_control` rates divided by `workers`) to a temp directory and
   launches one `generator.py -c <worker_i.yaml>` subprocess per config.
4. **Monitor**: since `pg_stat_statements` counts DB-wide, the same
   `SUM(rows)` query already aggregates every worker; it's sampled every
   `monitor_interval_seconds` and printed as one combined events/sec figure.
5. **Shutdown**: on `run_seconds` elapsed or Ctrl+C, all worker processes are
   sent SIGTERM (then SIGKILL after a grace period), temp configs are
   cleaned up, and a final summary (total events, mean aggregate ev/s,
   workers used) is printed.

Requires `pg_stat_statements` (`shared_preload_libraries = 'pg_stat_statements'`
plus `CREATE EXTENSION pg_stat_statements;`) -- calibration and monitoring
fail fast with a clear message if it's unavailable.

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
