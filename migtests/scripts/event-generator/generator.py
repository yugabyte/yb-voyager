import random
import itertools
import psycopg2
import threading
from utils import generate_table_schemas
from utils import (
    execute_with_retry,
    build_insert_values,
    build_update_values,
)
from utils import (
    run_index_operations,
    load_event_generator_config,
    get_connection_kwargs_from_config,
    detect_db_flavor,
    get_estimated_row_count,
    build_sampling_condition,
    build_rate_governor,
    build_pk_in_condition,
    seed_pk_pool,
)
from pk_pool import PkPool
import time
from utils import set_faker_seed
import argparse

# ----- CLI arguments -----
parser = argparse.ArgumentParser(description="Event Generator for PostgreSQL")
parser.add_argument(
    "-c",
    "--config",
    default=None,
    help="Path to event-generator YAML config (defaults to event-generator.yaml in this folder)",
)
args = parser.parse_args()

# ----- Config knobs (tuning) from YAML config -----
CONFIG = load_event_generator_config(args.config)
GEN = CONFIG["generator"]

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

# Throttling
WAIT_AFTER_OPERATIONS = GEN["wait_after_operations"]
WAIT_DURATION_SECONDS = GEN["wait_duration_seconds"]

# Rate governor: paces the aggregate events/sec (event = one changed row, i.e.
# cursor.rowcount after each operation) when generator.rate_control is
# configured; otherwise a no-op NullGovernor preserves today's unpaced
# behavior. When active, it takes over pacing and the legacy
# wait_after_operations/wait_duration_seconds knobs above are ignored.
RATE_CONTROL = GEN.get("rate_control")
GOVERNOR = build_rate_governor(CONFIG)
if RATE_CONTROL:
    print("rate_control is configured: ignoring wait_after_operations/wait_duration_seconds (legacy throttle)")

# Index events flag
ENABLE_INDEX_CREATE_DROP = GEN.get("enable_index_create_drop", False)
INDEX_EVENTS_INTERVAL = GEN.get("index_events_interval", 5)

# Column overrides for partition-aware value generation
COLUMN_OVERRIDES = GEN.get("column_overrides", {})
# ---------------------------------

# Deterministic seeds from YAML
SEED = GEN.get("random_seed", GEN.get("seed"))
FAKER_SEED = GEN.get("faker_seed", SEED)

if SEED is not None:
    random.seed(SEED)

if FAKER_SEED is not None:
    set_faker_seed(FAKER_SEED)

# Connect to PostgreSQL using config
conn = psycopg2.connect(**get_connection_kwargs_from_config(CONFIG))
cursor = conn.cursor()

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
print("Analysing schema")

# Schema based or manual list
table_schemas = generate_table_schemas(
    cursor,
    schema_name=SCHEMA_NAME,
    manual_table_list=MANUAL_TABLE_LIST,
    exclude_table_list=EXCLUDE_TABLE_LIST,
)
print("Schema analysed")

# Precompute estimated row counts once per table for sampling decisions
ROW_ESTIMATES = {}

try:
    # Refresh planner statistics up front for better row estimates
    cursor.execute("ANALYZE;")
    conn.commit()
    for table in table_schemas.keys():
        ROW_ESTIMATES[table] = get_estimated_row_count(cursor, SCHEMA_NAME, table)
except Exception as e:
    print(f"Error refreshing planner statistics using ANALYZE: {e}. Getting row estimates using count(*).")
    # Rollback the failed transaction before proceeding
    conn.rollback()
    # Using count(*) to get row estimates
    for table in table_schemas.keys():
        cursor.execute(f"SELECT COUNT(*) FROM {SCHEMA_NAME}.{table};")
        ROW_ESTIMATES[table] = cursor.fetchone()[0]
        conn.commit()

print("Row estimates: ", ROW_ESTIMATES)

# Build an in-memory PK pool per table that has a single-column primary key,
# prefilled with existing ids. This lets UPDATE/DELETE target explicit rows
# via `WHERE pk IN (...)` (an indexed point lookup, fast at any table size)
# instead of falling back to build_sampling_condition's full-table scan.
# Tables with a composite or missing primary key get no pool and keep using
# the scan-based fallback unchanged.
POOLS = {}
for table in table_schemas.keys():
    primary_key = table_schemas[table]["primary_key"]
    if primary_key and len(primary_key) == 1:
        pool = PkPool()
        seed_pk_pool(cursor, SCHEMA_NAME, table, primary_key, pool)
        POOLS[table] = pool
print(f"PK pools seeded for tables: {list(POOLS.keys())}")

# Precompute table selection weights once: default weight 1 for unspecified tables
RESOLVED_TABLE_WEIGHTS = dict(TABLE_WEIGHTS)
for table in table_schemas.keys():
    RESOLVED_TABLE_WEIGHTS.setdefault(table, 1)

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
        # Choose a random table
        table_name = random.choices(
            list(RESOLVED_TABLE_WEIGHTS.keys()),
            weights=list(RESOLVED_TABLE_WEIGHTS.values()),
        )[0]
        # Generate a random operation
        operation = random.choices(OPERATIONS, weights=OPERATION_WEIGHTS)[0]

        try:
            if operation == "INSERT":
                # Generate random data and execute INSERT statement
                columns = ", ".join(table_schemas[table_name]["columns"].keys())
                init_values_list, init_pk_values = build_insert_values(table_schemas, table_name, INSERT_ROWS, MIN_COL_SIZE_BYTES, COLUMN_OVERRIDES)
                values_holder = {"values_list": init_values_list, "pk_values": init_pk_values}

                # Prepare callbacks for retryable execution
                def run_once():
                    query_to_run = f"INSERT INTO {table_name} ({columns}) VALUES {values_holder['values_list']}"
                    cursor.execute(query_to_run)

                def rebuild():
                    new_values_list, new_pk_values = build_insert_values(table_schemas, table_name, INSERT_ROWS, MIN_COL_SIZE_BYTES, COLUMN_OVERRIDES)
                    values_holder["values_list"] = new_values_list
                    values_holder["pk_values"] = new_pk_values

                success = execute_with_retry(run_once, rebuild, conn.rollback, max_retries=INSERT_MAX_RETRIES)
                if success:
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
                pool = POOLS.get(table_name)

                for _ in range(UPDATE_MAX_RETRIES):
                    columns = table_schemas[table_name]["columns"]

                    if len(columns) <= len(pk_set):
                        break

                    updateable_columns = [col for col in columns if col not in pk_set]

                    if not updateable_columns:
                        print(f"No updateable columns found for table {table_name}. Retrying...")
                        continue

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
                    query_to_run = f"UPDATE {table_name} SET {set_clause} WHERE {where_clause}"
                    full_params = params + sampling_params

                    try:
                        cursor.execute(query_to_run, full_params)
                        conn.commit()
                        break
                    except Exception as e:
                        print(f"UPDATE failed on '{table_name}': {e}")
                        conn.rollback()

            elif operation == "DELETE":
                primary_key = table_schemas[table_name]["primary_key"]
                if not primary_key:
                    print(f"Skipping DELETE on '{table_name}': no primary key found")
                    continue

                # Prefer targeting explicit ids from the in-memory PK pool
                # (indexed point lookup) over the full-table-scan fallback.
                pool = POOLS.get(table_name)
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
                query_to_run = f"DELETE FROM {table_name} WHERE {where_clause}"
                cursor.execute(query_to_run, sampling_params)

                conn.commit()

                # Delete succeeded (no exception raised above) -- these ids
                # are no longer live.
                if pool_ids and pool is not None:
                    pool.remove_many(pool_ids)

            if not RATE_CONTROL and WAIT_AFTER_OPERATIONS and i % WAIT_AFTER_OPERATIONS == 0 and i != 0:
                if WAIT_DURATION_SECONDS > 0:
                    print("-" * 50)
                    print(f"Waiting for {WAIT_DURATION_SECONDS} seconds after {i} operations...")
                    print("-" * 50)
                    time.sleep(WAIT_DURATION_SECONDS)
                conn.commit()

            conn.commit()

            # event = one changed row (cursor.rowcount); 0 on failure/rollback/unknown
            rowcount = cursor.rowcount
            events_emitted = rowcount if rowcount is not None and rowcount > 0 else 0
            GOVERNOR.pace(events_emitted)

        except psycopg2.Error as e:
            print(f"An error occurred: {e}")
            if "current transaction is aborted" in str(e):
                print("Transaction aborted. Commands ignored until the end of the transaction block.")
            # Rollback the transaction to avoid leaving it in an inconsistent state
            conn.rollback()

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
    finally:
        # Close the connection
        conn.close()
        print("Program Complete")