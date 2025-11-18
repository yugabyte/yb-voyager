import random
import itertools
import psycopg2
from utils import generate_table_schemas
from utils import (
    build_bit_cast_expr,
    generate_random_data,
    fetch_enum_values_for_column,
    fetch_array_types_for_column,
    execute_with_retry,
    build_insert_values,
    build_update_values,
)
from utils import load_event_generator_config, get_connection_kwargs_from_config
import time
from utils import set_faker_seed
import argparse
import os

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

# Operation selection
OPERATIONS = GEN["operations"]
OPERATION_WEIGHTS = GEN["operation_weights"]

# Batch sizes per operation
INSERT_ROWS = GEN["insert_rows"]
UPDATE_ROWS = GEN["update_rows"]
DELETE_ROWS = GEN["delete_rows"]

# Retries
INSERT_MAX_RETRIES = GEN["insert_max_retries"]
UPDATE_MAX_RETRIES = GEN["update_max_retries"]

# Throttling
WAIT_AFTER_OPERATIONS = GEN["wait_after_operations"]
WAIT_DURATION_SECONDS = GEN["wait_duration_seconds"]

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
# print(table_schemas)

print("Schema analysed")

# Precompute table selection weights once: default weight 1 for unspecified tables
RESOLVED_TABLE_WEIGHTS = dict(TABLE_WEIGHTS)
for table in table_schemas.keys():
    RESOLVED_TABLE_WEIGHTS.setdefault(table, 1)

iteration_iter = itertools.count(1) if NUM_ITERATIONS == -1 else range(1, NUM_ITERATIONS + 1)

try:
    for i in iteration_iter:
        # Choose a random table
        table_name = random.choices(
            list(RESOLVED_TABLE_WEIGHTS.keys()),
            weights=list(RESOLVED_TABLE_WEIGHTS.values()),
        )[0]

        #print(table_name)

        # Generate a random operation
        operation = random.choices(OPERATIONS, weights=OPERATION_WEIGHTS)[0]

        # print(operation)

        try:
            if operation == "INSERT":
                # Generate random data and execute INSERT statement
                columns = ", ".join(table_schemas[table_name]["columns"].keys())
                values_holder = {"values_list": build_insert_values(table_schemas, table_name, INSERT_ROWS)}

                # Prepare callbacks for retryable execution
                def run_once():
                    query_to_run = f"INSERT INTO {table_name} ({columns}) VALUES {values_holder['values_list']}"
                    cursor.execute(query_to_run)

                def rebuild():
                    values_holder["values_list"] = build_insert_values(table_schemas, table_name, INSERT_ROWS)

                success = execute_with_retry(run_once, rebuild, conn.rollback, max_retries=INSERT_MAX_RETRIES)
                if success:
                    pass
            
            elif operation == "UPDATE":
                for _ in range(UPDATE_MAX_RETRIES):
                    columns = table_schemas[table_name]["columns"]
                    primary_key = table_schemas[table_name]["primary_key"]

                    if len(columns) == 1:
                        #print(f"Table {table_name} has only one column. Skipping update for this table.")
                        break  # Skip the entire update operation for tables with only one column
                
                    updateable_columns = [col for col in columns if col != primary_key]

                    if not updateable_columns:
                        print(f"No updateable columns found for table {table_name}. Retrying...")
                        continue

                    num_columns_to_update = random.randint(1, len(updateable_columns))

                    # Randomly choose the columns to update
                    columns_to_update = random.sample(updateable_columns, num_columns_to_update)

                    set_clause, params = build_update_values(table_schemas, table_name, columns_to_update)
                    where_clause = f"{primary_key} IN (SELECT {primary_key} FROM {table_name} TABLESAMPLE SYSTEM_ROWS(%s))"
                    # where_clause = f"{primary_key} IN (SELECT {primary_key} FROM {table_name} ORDER BY RANDOM() LIMIT %s)"
                    # where_clause = f"{primary_key} IN (SELECT {primary_key} FROM {table_name} TABLESAMPLE SYSTEM (0.00007))"
                    query_to_run = f"UPDATE {table_name} SET {set_clause} WHERE {where_clause}"
                    params = params + [UPDATE_ROWS]
                    
                    # print(query_to_run, params)

                    try:
                        cursor.execute(query_to_run, params)
                        conn.commit()
                        break  # Break out of the loop if the update is successful
                    except Exception as e:
                        #print(f"Error executing query: {query_to_run} with params: {params}")
                        #print(f"Error details: {e}")
                        conn.rollback()

            elif operation == "DELETE":
                primary_key = table_schemas[table_name]["primary_key"]
                query_to_run = f"DELETE FROM {table_name} WHERE {primary_key} IN (SELECT {primary_key} FROM {table_name} TABLESAMPLE SYSTEM_ROWS(%s))"
                # query_to_run = f"DELETE FROM {table_name} WHERE {primary_key} IN (SELECT {primary_key} FROM {table_name} ORDER BY RANDOM() LIMIT %s)"
                # query_to_run = f"DELETE FROM {table_name} WHERE {primary_key} IN (SELECT {primary_key} FROM {table_name} LIMIT %s)"
                params = (DELETE_ROWS,)
                #print(query_to_run, params)
                cursor.execute(query_to_run, params)

                conn.commit()
                # successful_operations += 1 # This line is removed

            if WAIT_AFTER_OPERATIONS and i % WAIT_AFTER_OPERATIONS == 0 and i != 0:
                if WAIT_DURATION_SECONDS > 0:
                    print("-" * 50)
                    print(f"Waiting for {WAIT_DURATION_SECONDS} seconds after {i} operations...")
                    print("-" * 50)
                    time.sleep(WAIT_DURATION_SECONDS)
                conn.commit()

            conn.commit()

        except psycopg2.Error as e:
            print(f"An error occurred: {e}")
            if "current transaction is aborted" in str(e):
                print("Transaction aborted. Commands ignored until the end of the transaction block.")
            # Print the SQL statement causing the error if available
            #if hasattr(e, 'cursor') and e.cursor is not None and hasattr(e.cursor, 'query'):
                #print(f"SQL statement: {e.cursor.query.decode('utf-8')}")
            #print("-" * 50)
            # Rollback the transaction to avoid leaving it in an inconsistent state
            conn.rollback()

except KeyboardInterrupt:
    print("Received KeyboardInterrupt. Stopping generator...")
finally:
    # Commit changes outside the loop for UPDATE and DELETE operations
    try:
        conn.commit()
    finally:
        # Close the connection
        conn.close()
        print("Program Complete")