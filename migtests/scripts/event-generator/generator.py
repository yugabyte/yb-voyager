import random
import itertools
import psycopg2
import re
import threading
from utils import generate_table_schemas
from utils import (
    execute_with_retry,
    build_insert_values,
    get_insert_column_list,
    build_update_values,
    compute_backoff_delay,
    compute_monotonic_pk,
    compute_unique_safe_value,
    is_integer_pk_type,
    get_table_max_pk,
    parse_worker_args,
)
from utils import (
    run_index_operations,
    load_event_generator_config,
    get_connection_kwargs_from_config,
    detect_db_flavor,
    get_estimated_row_count,
    build_sampling_condition,
    build_worker_governor,
    build_pk_in_condition,
    seed_pk_pool,
    read_control_rate,
)
from pk_pool import PkPool
import shared_cache
import time
from utils import set_faker_seed
from rate_governor import NullGovernor
from transaction_mode import (
    is_transaction_mode_enabled,
    build_transaction_plan,
    run_transaction,
)

# ----- CLI arguments -----
# Parser lives in utils.py (build_worker_arg_parser/parse_worker_args) so the
# pure logic below (CLI parsing, monotonic-PK formula, retry classification)
# stays importable/unit-testable without psycopg2 or a DB -- see
# IMPLEMENTATION_CONTRACTS.md "Worker CLI". All args below --config are new
# and optional; absent, this reproduces today's legacy single-process,
# self-seeding, uncapped-or-rate_control-paced behavior.
args = parse_worker_args()

# ----- Config knobs (tuning) from YAML config -----
CONFIG = load_event_generator_config(args.config)
GEN = CONFIG["generator"]
PARALLEL_CONFIG = CONFIG.get("parallel", {}) or {}

SCHEMA_NAME = GEN["schema_name"]

MANUAL_TABLE_LIST = GEN["manual_table_list"]
EXCLUDE_TABLE_LIST = GEN["exclude_table_list"]
TABLE_WEIGHTS = GEN["table_weights"]

NUM_ITERATIONS = GEN["num_iterations"]

# Operation selection via weight map
raw_operation_weights = GEN["operation_weights"]
OPERATIONS = []
OPERATION_WEIGHTS = []
for op_name, weight in raw_operation_weights.items():
    if weight > 0:
        OPERATIONS.append(op_name.upper())
        OPERATION_WEIGHTS.append(float(weight))

# Batch sizes per operation
INSERT_ROWS = GEN["insert_rows"]
UPDATE_ROWS = GEN["update_rows"]
DELETE_ROWS = GEN["delete_rows"]
MIN_COL_SIZE_BYTES = GEN.get("min_col_size_bytes", 0)

# Retries
INSERT_MAX_RETRIES = GEN["insert_max_retries"]
UPDATE_MAX_RETRIES = GEN["update_max_retries"]
# No dedicated config knob for DELETE retries (config's 'generator' block is
# unchanged); reuse UPDATE_MAX_RETRIES as a reasonable bound for the same
# 40001/40P01 conflict retries DELETE now shares with INSERT/UPDATE.
DELETE_MAX_RETRIES = GEN.get("delete_max_retries", UPDATE_MAX_RETRIES)

# Throttling
WAIT_AFTER_OPERATIONS = GEN["wait_after_operations"]
WAIT_DURATION_SECONDS = GEN["wait_duration_seconds"]

# ----- Dynamic worker pool CLI knobs (all optional; absent => legacy) -----
WORKER_UID = args.worker_uid
PK_STRIDE = args.pk_stride
CACHE_DIR = args.cache_dir
# Per-run token (see parallel_generator.run_controller); folded into the
# text/uuid unique-value encodings only (see compute_unique_safe_value) so a
# re-run's restarted worker_uid/counter can't collide with a previous run's
# rows. Absent/empty (legacy standalone invocation) => unchanged encoding.
RUN_ID = args.run_id
PK_POOL_MAXSIZE = PARALLEL_CONFIG.get("pk_pool_maxsize", 20000)

# Rate governor: paces the aggregate events/sec. Two independent knobs can
# engage it:
#   --throttle > 0   : the reactive controller's single-worker "trimmer" cap
#                      (new dynamic worker pool model; takes priority).
#   generator.rate_control (YAML) : legacy schedule-based pacing (unchanged
#                      when --throttle is absent/0).
# Absent both => NullGovernor (today's unpaced default): no per-worker
# sleeping, engaged only as configured.
RATE_CONTROL = GEN.get("rate_control")
GOVERNOR = build_worker_governor(CONFIG, throttle=args.throttle, worker_uid=WORKER_UID)
GOVERNOR_ACTIVE = not isinstance(GOVERNOR, NullGovernor)
if args.throttle and args.throttle > 0:
    print(f"--throttle={args.throttle} ev/s: engaging rate_governor as a single-worker cap "
          f"(overrides generator.rate_control, if configured)")
elif RATE_CONTROL:
    print("rate_control is configured: ignoring wait_after_operations/wait_duration_seconds (legacy throttle)")

# Cascade trimmer controller: runtime-adjustable throttle (Option B). Only
# the persistent "trimmer" worker is launched with --control-file; every
# other worker (uncapped or unset) leaves this None and the block below is
# a no-op. See ARCHITECTURE.md.
#
# Re-read at most once per ~CONTROL_FILE_POLL_SECONDS, immediately before
# GOVERNOR.pace() in the main loop, so a changed commanded rate takes
# effect on the very next pacing decision. `_last_control_rate` seeds from
# the already-engaged --throttle (the trimmer is always spawned with a
# throttle > 0 precisely so this starting point is never 0/uncapped).
CONTROL_FILE = args.control_file
CONTROL_FILE_POLL_SECONDS = 1.0
# A commanded rate <= 0 means PAUSE: the loop skips the DML op entirely and
# idles for CONTROL_FILE_POLL_SECONDS, re-reading each iteration. This replaces
# the old "floor to a 1 ev/s epsilon and keep running full batches" behavior,
# which froze the worker: one 300-row batch at 1 ev/s makes the governor sleep
# ~300s, during which it can never re-read the control file to see the rate rise
# again (observed: trimmer stuck at 0 through an entire baseline recovery).
_last_control_rate = float(args.throttle) if args.throttle and args.throttle > 0 else 0.0
_last_control_read_t = 0.0
# Defense in depth against the same freeze for tiny (but > 0) commanded rates:
# cap any single governor sleep. Never triggers at real trimmer rates
# (hundreds+ ev/s -> sub-second pacing); only bounds pathological low rates.
CONTROL_FILE_MAX_SLEEP_SECONDS = 3.0
if CONTROL_FILE and hasattr(GOVERNOR, "max_single_sleep_seconds"):
    GOVERNOR.max_single_sleep_seconds = CONTROL_FILE_MAX_SLEEP_SECONDS

# Index events flag
ENABLE_INDEX_CREATE_DROP = GEN.get("enable_index_create_drop", False)
INDEX_EVENTS_INTERVAL = GEN.get("index_events_interval", 5)

# Column overrides for partition-aware value generation
COLUMN_OVERRIDES = GEN.get("column_overrides", {})
# ---------------------------------

# Deterministic seeds from YAML, offset per worker so each worker in a pool
# gets distinct (but reproducible) data -- see IMPLEMENTATION_CONTRACTS.md
# "Monotonic PK generation": "RNG/faker seed for a worker = base_seed +
# worker_uid". Never seed random.Random with a tuple (ints only).
SEED = GEN.get("random_seed", GEN.get("seed"))
FAKER_SEED = GEN.get("faker_seed", SEED)

if WORKER_UID is not None:
    if SEED is not None:
        SEED = SEED + WORKER_UID
    if FAKER_SEED is not None:
        FAKER_SEED = FAKER_SEED + WORKER_UID

if SEED is not None:
    random.seed(SEED)

if FAKER_SEED is not None:
    set_faker_seed(FAKER_SEED)


def connect_db(config):
    """Open a fresh psycopg2 connection + cursor from config."""
    new_conn = psycopg2.connect(**get_connection_kwargs_from_config(config))
    return new_conn, new_conn.cursor()


def reconnect_with_backoff(config, max_attempts=10, base=0.5, cap=30.0, sleep_fn=time.sleep):
    """Reconnect to the DB with bounded exponential backoff, then resume.

    No re-seed on reconnect: this worker's PkPool delta/tombstones live in
    process RAM (unaffected by a DB connection drop) and, in cache mode, the
    PK base is a read-only mmap the worker never re-opens. See
    IMPLEMENTATION_CONTRACTS.md / dynamic-worker-pool-design.md 11.5.

    If `max_attempts` is exhausted, re-raises the last connection error so
    the process exits -- letting the controller's liveness poll detect the
    death and respawn a fresh worker (with a fresh seed block) from the
    current cache, per the design.
    """
    attempt = 0
    last_err = None
    while attempt < max_attempts:
        attempt += 1
        try:
            new_conn, new_cursor = connect_db(config)
            print(f"Reconnected to database on attempt {attempt}")
            return new_conn, new_cursor
        except psycopg2.OperationalError as e:
            last_err = e
            delay = compute_backoff_delay(attempt, base, cap)
            print(f"Reconnect attempt {attempt}/{max_attempts} failed ({e}); retrying in {delay:.1f}s")
            sleep_fn(delay)
    print(f"Failed to reconnect after {max_attempts} attempts; exiting so the controller can respawn this worker.")
    raise last_err


# Connect to PostgreSQL using config
conn, cursor = connect_db(CONFIG)

# Detect database flavor (PostgreSQL vs YugabyteDB)
DB_FLAVOR = detect_db_flavor(cursor)

cursor.execute("""
    CREATE EXTENSION IF NOT EXISTS tsm_system_rows;
""")
conn.commit()
print("tsm_system_rows extension is present or created successfully")

# Disabled to allow triggers and constraints to execute
# cursor.execute("SET session_replication_role = 'replica';")

print("Generator starting")
print("Note: No. of iterations may not equal number of events")

# Schema: from the shared cache (fast path, no per-table catalog queries)
# when --cache-dir is given, else the legacy per-table catalog scan.
cache_version = None
if CACHE_DIR:
    cache_version = args.cache_version or shared_cache.current_version(CACHE_DIR)
    print(f"Loading schema from shared cache: {CACHE_DIR} (version={cache_version})")
    table_schemas = shared_cache.load_schema(CACHE_DIR, cache_version)
    print("Schema loaded from cache")
else:
    print("Analysing schema")
    table_schemas = generate_table_schemas(
        cursor,
        schema_name=SCHEMA_NAME,
        manual_table_list=MANUAL_TABLE_LIST,
        exclude_table_list=EXCLUDE_TABLE_LIST,
    )
    print("Schema analysed")

# Precompute estimated row counts once per table for sampling decisions
ROW_ESTIMATES = {}

# Row estimates come from pg_class.reltuples in ONE bulk catalog query --
# instant, and only used to derive the random()<p sampling probability for
# tables without a usable PK pool. We deliberately do NOT run ANALYZE (it can
# fail on a busy cluster) nor count(*) per table: count(*) over hundreds of
# large tables at startup takes minutes-to-hours and strands every worker
# before it reaches the event loop.
_DEFAULT_ROW_ESTIMATE = 100000
try:
    _tnames = list(table_schemas.keys())
    if _tnames:
        cursor.execute(
            """
            SELECT c.relname, c.reltuples::bigint
            FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
            WHERE c.relkind = 'r' AND n.nspname = %s AND c.relname = ANY(%s)
            """,
            (SCHEMA_NAME or "public", _tnames),
        )
        for _relname, _est in cursor.fetchall():
            ROW_ESTIMATES[_relname] = _est if (_est is not None and _est > 0) else _DEFAULT_ROW_ESTIMATE
        conn.commit()
except Exception as e:
    print(f"Row-estimate lookup failed ({e}); using defaults.")
    conn.rollback()
for _t in table_schemas.keys():
    ROW_ESTIMATES.setdefault(_t, _DEFAULT_ROW_ESTIMATE)

print(f"Row estimates ready for {len(ROW_ESTIMATES)} tables (via pg_class.reltuples)")

# Build an in-memory PK pool per table that has a primary key, prefilled
# with existing ids. This lets UPDATE/DELETE target explicit rows via
# `WHERE pk IN (...)` (an indexed point lookup, fast at any table size)
# instead of falling back to build_sampling_condition's full-table scan.
#
# Cache mode (--cache-dir): every table with a primary key (single-column
# or composite) gets a pool backed by the shared, mmap'd, read-only PkBase
# -- no per-table seed queries (build_pk_in_condition already supports
# composite keys).
# Legacy mode (no --cache-dir): only single-column PK tables get a pool,
# self-seeded via seed_pk_pool (unchanged); composite/missing-PK tables
# keep using the scan-based fallback, exactly as before.
#
# MAX_PK / PK_COUNTERS drive monotonic PK generation (--worker-uid): only
# for single-column integer PK tables, where a usable max(pk) ceiling
# exists (see compute_monotonic_pk). Tables without one keep legacy random
# PK generation for their PK column.
POOLS = {}
MAX_PK = {}
PK_COUNTERS = {}
for table in table_schemas.keys():
    primary_key = table_schemas[table]["primary_key"]
    if not primary_key:
        continue

    if CACHE_DIR:
        pk_base = shared_cache.open_pk_base(CACHE_DIR, cache_version, table)
        POOLS[table] = PkPool(base=pk_base, maxsize=PK_POOL_MAXSIZE)
    elif len(primary_key) == 1:
        pool = PkPool(maxsize=PK_POOL_MAXSIZE)
        seed_pk_pool(cursor, SCHEMA_NAME, table, primary_key, pool)
        POOLS[table] = pool

    if WORKER_UID is not None and len(primary_key) == 1:
        pk_col = primary_key[0]
        data_type = table_schemas[table]["columns"].get(pk_col)
        if is_integer_pk_type(data_type):
            if CACHE_DIR:
                max_pk = shared_cache.table_max_pk(CACHE_DIR, cache_version, table)
            else:
                max_pk = get_table_max_pk(cursor, SCHEMA_NAME, table, pk_col)
            # None means an empty table (no PK/composite/non-integer tables
            # never reach this branch) -- start the monotonic stride from 0.
            MAX_PK[table] = max_pk if max_pk is not None else 0
            PK_COUNTERS[table] = 0

print(f"PK pools ready for tables: {list(POOLS.keys())}")
if WORKER_UID is not None:
    print(f"Monotonic PK generation enabled for tables: {list(MAX_PK.keys())} "
          f"(worker_uid={WORKER_UID}, pk_stride={PK_STRIDE})")

# UNIQUE_VALUE_FNS[table] = {col: zero-arg closure} covering every
# single-column unique surface (PK, UNIQUE constraint, standalone unique
# index) named in that table's "unique_columns" (see
# utils.generate_table_schemas_bulk / compute_unique_safe_value) -- the
# generalization of the monotonic-PK scheme above to every type and every
# unique surface, not just single-column integer PKs. Only built in
# parallel mode (--worker-uid); absent, build_insert_values falls back to
# its legacy path (pk_value_fn for a single-column integer PK, else plain
# random generation for that column) exactly as before. Each closure keeps
# its own per-(table, column) counter, so the PK column -- naturally a
# unique surface too -- gets a fn here that takes priority over
# make_pk_value_fn's for the same column (build_insert_values' priority
# order); make_pk_value_fn / MAX_PK / PK_COUNTERS are left in place
# unchanged as the fallback for tables/columns unique_value_fns doesn't
# cover, and are simply never invoked when both would apply to the same
# column -- no double-generation.
UNIQUE_VALUE_FNS = {}
UNIQUE_VALUE_COUNTERS = {}
_unique_surface_count = 0
if WORKER_UID is not None:
    for table in table_schemas.keys():
        unique_columns = table_schemas[table].get("unique_columns") or []
        if not unique_columns:
            continue
        columns_info = table_schemas[table]["columns"]
        cached_max_seeds = table_schemas[table].get("unique_max_seeds") or {} if CACHE_DIR else {}

        table_fns = {}
        for col in unique_columns:
            data_type = columns_info.get(col)
            if not data_type:
                continue

            is_orderable = is_integer_pk_type(data_type) or data_type.strip().lower().startswith(
                ("numeric", "decimal")
            )
            if not is_orderable:
                max_seed = 0
            elif CACHE_DIR:
                max_seed = cached_max_seeds.get(col, 0)
            else:
                max_pk_val = get_table_max_pk(cursor, SCHEMA_NAME, table, col)
                max_seed = max_pk_val if max_pk_val is not None else 0

            char_max = None
            length_match = re.search(r"\((\d+)\)", data_type)
            if length_match:
                char_max = int(length_match.group(1))

            UNIQUE_VALUE_COUNTERS[(table, col)] = 0

            def _make_unique_value_fn(table=table, col=col, data_type=data_type,
                                       max_seed=max_seed, char_max=char_max):
                def _next_unique_value():
                    counter = UNIQUE_VALUE_COUNTERS[(table, col)]
                    UNIQUE_VALUE_COUNTERS[(table, col)] = counter + 1
                    return compute_unique_safe_value(
                        data_type, WORKER_UID, PK_STRIDE, counter,
                        max_seed=max_seed, char_max=char_max, run_id=RUN_ID,
                    )
                return _next_unique_value

            table_fns[col] = _make_unique_value_fn()
            _unique_surface_count += 1

        if table_fns:
            UNIQUE_VALUE_FNS[table] = table_fns

if WORKER_UID is not None:
    print(f"Unique-safe value generation enabled for {_unique_surface_count} (table,column) "
          f"surfaces across {len(UNIQUE_VALUE_FNS)} tables "
          f"(worker_uid={WORKER_UID}, pk_stride={PK_STRIDE})")


def make_pk_value_fn(table_name):
    """Return a zero-arg callable producing this worker's next monotonic PK
    for `table_name` (see compute_monotonic_pk), or None if the table isn't
    eligible (no --worker-uid, composite/missing/non-integer PK) -- in
    which case build_insert_values falls back to its normal random
    generation for the PK column, unchanged.
    """
    if table_name not in MAX_PK:
        return None

    def _next_pk():
        counter = PK_COUNTERS[table_name]
        PK_COUNTERS[table_name] = counter + 1
        return compute_monotonic_pk(MAX_PK[table_name], WORKER_UID, PK_STRIDE, counter)

    return _next_pk


# Precompute table selection weights once: default weight 1 for unspecified tables
RESOLVED_TABLE_WEIGHTS = dict(TABLE_WEIGHTS)
for table in table_schemas.keys():
    RESOLVED_TABLE_WEIGHTS.setdefault(table, 1)

# ----- Transaction mode (optional, default OFF; see transaction_mode.py) -----
# Absent 'generator.transaction_mode' block, or 'enabled: false', means the
# main loop below runs its legacy single-op path exactly as before --
# HOT_TABLES/OTHER_TABLE_WEIGHTS are simply never read in that case.
TRANSACTION_MODE_CFG = GEN.get("transaction_mode")
TRANSACTION_MODE_ENABLED = is_transaction_mode_enabled(TRANSACTION_MODE_CFG)
HOT_TABLES: list = []
OTHER_TABLE_WEIGHTS: dict = {}
if TRANSACTION_MODE_ENABLED:
    HOT_TABLES = list(TRANSACTION_MODE_CFG["hot_tables"])
    _hot_set = set(HOT_TABLES)
    OTHER_TABLE_WEIGHTS = {t: w for t, w in RESOLVED_TABLE_WEIGHTS.items() if t not in _hot_set}
    print(
        f"Transaction mode enabled: {len(HOT_TABLES)} hot table(s), "
        f"{len(OTHER_TABLE_WEIGHTS)} other table(s) eligible"
    )

# Start index operations thread if enabled
stop_index_thread = None
index_thread = None
if ENABLE_INDEX_CREATE_DROP:
    stop_index_thread = threading.Event()
    index_thread = threading.Thread(
        target=run_index_operations,
        args=(stop_index_thread, CONFIG, SCHEMA_NAME, table_schemas, INDEX_EVENTS_INTERVAL),
        daemon=True
    )
    index_thread.start()
    print("Index events enabled - running concurrently with IUD operations")

iteration_iter = itertools.count(1) if NUM_ITERATIONS == -1 else range(1, NUM_ITERATIONS + 1)

try:
    for i in iteration_iter:
        # Cascade trimmer control (trimmer worker only): re-read the commanded
        # rate at the TOP of the loop, time-gated. A changed rate > 0 is pushed
        # to the governor for this tick's pacing; a rate <= 0 means PAUSE -- skip
        # this iteration's DML entirely and idle briefly, re-reading next pass.
        if CONTROL_FILE:
            _now = time.monotonic()
            if _now - _last_control_read_t >= CONTROL_FILE_POLL_SECONDS:
                _last_control_read_t = _now
                _r = read_control_rate(CONTROL_FILE, _last_control_rate)
                if _r != _last_control_rate:
                    _last_control_rate = _r
                    if _r > 0:
                        GOVERNOR.set_rate(_r)
            if _last_control_rate <= 0:
                time.sleep(CONTROL_FILE_POLL_SECONDS)
                continue

        try:
            # Rows actually changed this iteration. Set only when a statement really
            # executes; stays 0 if the op is skipped (no PK, no updateable columns,
            # retry budget exhausted) so we never re-count a stale cursor.rowcount.
            events_emitted = 0
            if TRANSACTION_MODE_ENABLED:
                # One multi-statement transaction (BEGIN ... [SAVEPOINT ...
                # RELEASE SAVEPOINT ...] ... COMMIT) instead of one op -- see
                # transaction_mode.py. Each committed STATEMENT counts as one
                # event (rows are always 1 here), matching GOVERNOR.pace below.
                plan = build_transaction_plan(
                    TRANSACTION_MODE_CFG, HOT_TABLES, OTHER_TABLE_WEIGHTS, random,
                )
                events_emitted = run_transaction(
                    conn, cursor, plan, table_schemas, POOLS, DB_FLAVOR, ROW_ESTIMATES,
                    COLUMN_OVERRIDES, MIN_COL_SIZE_BYTES,
                    pk_value_fn_for_table=make_pk_value_fn,
                    unique_value_fns_for_table=UNIQUE_VALUE_FNS.get,
                )
            else:
                # Choose a random table
                table_name = random.choices(
                    list(RESOLVED_TABLE_WEIGHTS.keys()),
                    weights=list(RESOLVED_TABLE_WEIGHTS.values()),
                )[0]
                # Generate a random operation
                operation = random.choices(OPERATIONS, weights=OPERATION_WEIGHTS)[0]

                if operation == "INSERT":
                    # Generate random data and execute INSERT statement. Every
                    # single-column unique surface (PK, UNIQUE constraint,
                    # standalone unique index) named in unique_columns uses the
                    # unique-safe scheme (UNIQUE_VALUE_FNS) when eligible; the
                    # PK column also keeps make_pk_value_fn as a fallback for
                    # when it isn't (unique_value_fns takes priority for the
                    # same column -- see build_insert_values). Anything neither
                    # covers falls back to normal type-aware random generation,
                    # unchanged. Any column still unsynthesizable (e.g. an
                    # exotic type) is omitted from the column list entirely
                    # (see get_insert_column_list) so its DEFAULT applies,
                    # instead of an explicit NULL that would violate a NOT
                    # NULL constraint.
                    pk_value_fn = make_pk_value_fn(table_name)
                    unique_value_fns = UNIQUE_VALUE_FNS.get(table_name)
                    columns = ", ".join(get_insert_column_list(
                        table_schemas, table_name, COLUMN_OVERRIDES, unique_value_fns, pk_value_fn,
                    ))
                    init_values_list, init_pk_values = build_insert_values(
                        table_schemas, table_name, INSERT_ROWS, MIN_COL_SIZE_BYTES,
                        COLUMN_OVERRIDES, pk_value_fn=pk_value_fn, unique_value_fns=unique_value_fns,
                    )
                    values_holder = {"values_list": init_values_list, "pk_values": init_pk_values}

                    # Prepare callbacks for retryable execution
                    def run_once():
                        query_to_run = f"INSERT INTO {table_name} ({columns}) VALUES {values_holder['values_list']}"
                        cursor.execute(query_to_run)

                    def rebuild():
                        # Reuses the same pk_value_fn/unique_value_fns closures,
                        # so a retry (unique violation safety net, or a
                        # 40001/40P01 conflict) simply continues each per-table
                        # counter -- it can never repeat or collide with a
                        # value already attempted.
                        new_values_list, new_pk_values = build_insert_values(
                            table_schemas, table_name, INSERT_ROWS, MIN_COL_SIZE_BYTES,
                            COLUMN_OVERRIDES, pk_value_fn=pk_value_fn, unique_value_fns=unique_value_fns,
                        )
                        values_holder["values_list"] = new_values_list
                        values_holder["pk_values"] = new_pk_values

                    success = execute_with_retry(run_once, rebuild, conn.rollback, max_retries=INSERT_MAX_RETRIES)
                    if success:
                        conn.commit()
                        events_emitted = max(cursor.rowcount or 0, 0)
                        # Refresh the PK pool (if any) with the ids we just inserted,
                        # so subsequent UPDATE/DELETE can target them directly.
                        pool = POOLS.get(table_name)
                        if pool is not None:
                            pool.add_many([pk for pk in values_holder["pk_values"] if pk is not None])

                elif operation == "UPDATE":
                    primary_key = table_schemas[table_name]["primary_key"]
                    if not primary_key:
                        print(f"Skipping UPDATE on '{table_name}': no primary key found")
                        continue

                    pk_set = set(primary_key) if isinstance(primary_key, list) else {primary_key}
                    columns = table_schemas[table_name]["columns"]

                    # Never UPDATE a unique-constrained column (PK or a secondary
                    # unique index) to a fresh random value -- that reintroduces
                    # the collision storm the unique-safe INSERT path removes, on
                    # columns whose values must stay unique. Exclude them from the
                    # SET list; the non-unique columns still get updated.
                    no_update = pk_set | set(table_schemas[table_name].get("unique_columns", []))

                    if len(columns) <= len(no_update):
                        continue

                    updateable_columns = [col for col in columns if col not in no_update]
                    if not updateable_columns:
                        print(f"No updateable columns found for table {table_name}. Skipping.")
                        continue

                    pool = POOLS.get(table_name)
                    query_holder = {}

                    def build_update_query_and_params():
                        num_columns_to_update = random.randint(1, len(updateable_columns))
                        columns_to_update = random.sample(updateable_columns, num_columns_to_update)
                        set_clause, params = build_update_values(table_schemas, table_name, columns_to_update, MIN_COL_SIZE_BYTES, COLUMN_OVERRIDES)

                        # Prefer targeting explicit ids from the in-memory PK pool
                        # (indexed point lookup) over the full-table-scan fallback.
                        # UPDATE never removes ids from the pool -- rows stay live.
                        pool_ids = pool.sample(UPDATE_ROWS) if pool is not None and len(pool) > 0 else []
                        if pool_ids:
                            where_clause, sampling_params = build_pk_in_condition(primary_key, pool_ids)
                        else:
                            where_clause, sampling_params = build_sampling_condition(
                                db_flavor=DB_FLAVOR,
                                table_name=table_name,
                                primary_key=primary_key,
                                target_row_count=UPDATE_ROWS,
                                estimated_row_count=ROW_ESTIMATES.get(table_name),
                            )
                        query = f"UPDATE {table_name} SET {set_clause} WHERE {where_clause}"
                        return query, params + sampling_params

                    query_holder["query"], query_holder["params"] = build_update_query_and_params()

                    def run_once():
                        cursor.execute(query_holder["query"], query_holder["params"])

                    def rebuild():
                        query_holder["query"], query_holder["params"] = build_update_query_and_params()

                    success = execute_with_retry(run_once, rebuild, conn.rollback, max_retries=UPDATE_MAX_RETRIES)
                    if success:
                        conn.commit()
                        events_emitted = max(cursor.rowcount or 0, 0)

                elif operation == "DELETE":
                    primary_key = table_schemas[table_name]["primary_key"]
                    if not primary_key:
                        print(f"Skipping DELETE on '{table_name}': no primary key found")
                        continue

                    # Prefer targeting explicit ids from the in-memory PK pool
                    # (indexed point lookup) over the full-table-scan fallback.
                    pool = POOLS.get(table_name)
                    query_holder = {}

                    def build_delete_query_and_params():
                        pool_ids = pool.sample(DELETE_ROWS) if pool is not None and len(pool) > 0 else []
                        if pool_ids:
                            where_clause, sampling_params = build_pk_in_condition(primary_key, pool_ids)
                        else:
                            where_clause, sampling_params = build_sampling_condition(
                                db_flavor=DB_FLAVOR,
                                table_name=table_name,
                                primary_key=primary_key,
                                target_row_count=DELETE_ROWS,
                                estimated_row_count=ROW_ESTIMATES.get(table_name),
                            )
                        query = f"DELETE FROM {table_name} WHERE {where_clause}"
                        return query, sampling_params, pool_ids

                    query_holder["query"], query_holder["params"], query_holder["pool_ids"] = build_delete_query_and_params()

                    def run_once():
                        cursor.execute(query_holder["query"], query_holder["params"])

                    def rebuild():
                        query_holder["query"], query_holder["params"], query_holder["pool_ids"] = build_delete_query_and_params()

                    success = execute_with_retry(run_once, rebuild, conn.rollback, max_retries=DELETE_MAX_RETRIES)
                    if success:
                        conn.commit()
                        events_emitted = max(cursor.rowcount or 0, 0)

                        # Delete succeeded -- these ids are no longer live.
                        pool_ids = query_holder["pool_ids"]
                        if pool_ids and pool is not None:
                            pool.remove_many(pool_ids)

            if not GOVERNOR_ACTIVE and WAIT_AFTER_OPERATIONS and i % WAIT_AFTER_OPERATIONS == 0 and i != 0:
                if WAIT_DURATION_SECONDS > 0:
                    print("-" * 50)
                    print(f"Waiting for {WAIT_DURATION_SECONDS} seconds after {i} operations...")
                    print("-" * 50)
                    time.sleep(WAIT_DURATION_SECONDS)
                conn.commit()

            conn.commit()

            # events_emitted was captured right after each successful statement above
            # (0 if this iteration executed nothing), so we never re-count a stale rowcount.
            GOVERNOR.pace(events_emitted)

        except psycopg2.Error as e:
            print(f"An error occurred: {e}")
            if "current transaction is aborted" in str(e):
                print("Transaction aborted. Commands ignored until the end of the transaction block.")

            # Connection loss (OperationalError, admin shutdown, closed
            # socket) needs a reconnect, not just a rollback -- the session
            # underlying `conn`/`cursor` is gone. Reconnect-and-resume: no
            # re-seed (this worker's PkPool delta/tombstones are in-process
            # RAM; the cache-mode base is a read-only mmap untouched by a
            # connection drop). See IMPLEMENTATION_CONTRACTS.md /
            # dynamic-worker-pool-design.md 11.5.
            connection_lost = isinstance(e, psycopg2.OperationalError) or bool(getattr(conn, "closed", 0))
            if connection_lost:
                print("Connection lost; reconnecting and resuming (no re-seed)...")
                conn, cursor = reconnect_with_backoff(CONFIG)
            else:
                try:
                    conn.rollback()
                except psycopg2.Error as rollback_err:
                    print(f"Rollback failed too ({rollback_err}); reconnecting...")
                    conn, cursor = reconnect_with_backoff(CONFIG)

except KeyboardInterrupt:
    print("Received KeyboardInterrupt. Stopping generator...")
finally:
    # Stop index operations thread if it's running
    if ENABLE_INDEX_CREATE_DROP and stop_index_thread is not None and index_thread is not None:
        print("Stopping index operations thread...")
        stop_index_thread.set()
        index_thread.join(timeout=5)

    # Commit changes outside the loop for UPDATE and DELETE operations
    try:
        conn.commit()
    except psycopg2.Error:
        pass
    finally:
        # Close the connection
        try:
            conn.close()
        except psycopg2.Error:
            pass
        print("Program Complete")
