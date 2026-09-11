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

### Forcing unique-key conflicts (CDC conflict detection testing)
Set `force_conflicts.enabled: true` to add a `FORCE_CONFLICT` operation alongside INSERT/UPDATE/DELETE. On a table with a plain single-column `UNIQUE` index (not the primary key), it:
1. Picks an existing row and its current value for that unique column.
2. Frees that value on the source row via `DELETE` or `UPDATE` (per `free_via`).
3. Immediately reuses the same value on a *different* row via `INSERT` or `UPDATE` (per `reuse_via`), regenerating the rest of that row deterministically from the value (`faker_for_key` in `utils.py`) so it's reproducible.

Because the two rows have different primary keys, the resulting CDC events can land on different parallel apply channels during live migration and race - this is exactly what yb-voyager's `ConflictDetectionCache` (`yb-voyager/cmd/conflictDetectionCache.go`) is meant to detect and serialize. Each forced conflict is printed as `[FORCE_CONFLICT] table=... column=... value=... FREE_OP -> REUSE_OP`; correlate that with `"conflict detected for table ..."` log lines in the target import logs to confirm detection actually fired.

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
