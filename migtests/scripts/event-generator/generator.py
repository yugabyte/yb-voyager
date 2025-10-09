import random
import psycopg2
from utils import generate_table_schemas
from utils import (
    build_bit_cast_expr,
    generate_random_data,
    fetch_enum_values_for_column,
    fetch_array_types_for_column,
    execute_with_retry,
    build_insert_values,
)
import time


# ----- Config knobs (tuning) -----

#
# Either we can provide our own table list
# Or we can specify the schema from where all tables will be picked up
#

SCHEMA_NAME = "public"
EXCLUDE_TABLE_LIST = ['eg_arr','eg_bits','eg_bytea','eg_full','eg_meta_bits','eg_num','eg_pk','eg_retry','eg_tsv','x','y']

# Tables to target explicitly (leave empty to use schema scan)
MANUAL_TABLE_LIST = [
    "eg_users",
    "eg_orders",
]

NUM_ITERATIONS = 200

# Throttling
WAIT_AFTER_OPERATIONS = 200000
WAIT_DURATION_SECONDS = 0

# Table selection (override weights per table; unspecified default to 1)
TABLE_WEIGHTS = {
    "eg_users": 100,
    "eg_orders": 100,
}

# Operation selection
OPERATIONS = ["INSERT", "UPDATE", "DELETE"]
OPERATION_WEIGHTS = [3, 2, 1]

# Batch sizes per operation
INSERT_ROWS = 4
UPDATE_ROWS = 2
DELETE_ROWS = 1

# Retries
INSERT_MAX_RETRIES = 50
UPDATE_MAX_RETRIES = 3
# ---------------------------------

# Connect to PostgreSQL

conn = psycopg2.connect(dbname="test", user="postgres", password="postgres", host="localhost", port="5432")
cursor = conn.cursor()
cursor.execute("""
    CREATE EXTENSION IF NOT EXISTS tsm_system_rows;
""")
print("tsm_system_rows extension is present or created successfully")

# Disabled to allow triggers and constraints to execute
# cursor.execute("SET session_replication_role = 'replica';")

print("Generator starting")
print("Note: No. of iterations may not equal number of events")
print("Analysing schema")


# Manual list

# manual_table_list = ["v","v1","v2"]
# table_schemas = generate_table_schemas(cursor, manual_table_list=manual_table_list)
# print(table_schemas)

# Schema based or manual list
table_schemas = generate_table_schemas(
    cursor,
    schema_name=SCHEMA_NAME,
    manual_table_list=MANUAL_TABLE_LIST,
    exclude_table_list=EXCLUDE_TABLE_LIST,
)
# print(table_schemas)

print("Schema analysed")


num_iterations = NUM_ITERATIONS # Specify the number of iterations
wait_after_operations = WAIT_AFTER_OPERATIONS  # Adjust this value to set the desired wait interval
wait_duration_seconds = WAIT_DURATION_SECONDS  # Adjust this value to set the duration of the wait in seconds


for i in range(num_iterations):
    # Choose a random table
    table_weights = dict(TABLE_WEIGHTS) #specify weights for tables you want to prioritise
    for table in table_schemas.keys():
        table_weights.setdefault(table, 1)

    table_name = random.choices(list(table_weights.keys()), weights=list(table_weights.values()))[0]

    #print(table_name)

    # Generate a random operation
    operations = OPERATIONS
    weights = OPERATION_WEIGHTS  # Mention the weight of the operations

    operation = random.choices(operations, weights=weights)[0]

    # print(operation)

    try:
        if operation == "INSERT":
    # Generate random data and execute INSERT statement
            columns = ", ".join(table_schemas[table_name]["columns"].keys())
            number_of_rows_to_insert = INSERT_ROWS # Adjust as needed
            values_holder = {"values_list": build_insert_values(table_schemas, table_name, number_of_rows_to_insert)}

            # Prepare callbacks for retryable execution
            def run_once():
                query_to_run = f"INSERT INTO {table_name} ({columns}) VALUES {values_holder['values_list']}"
                cursor.execute(query_to_run)

            def rebuild():
                values_holder["values_list"] = build_insert_values(table_schemas, table_name, number_of_rows_to_insert)

            success = execute_with_retry(run_once, rebuild, conn.rollback, max_retries=INSERT_MAX_RETRIES)
            if success:
                pass # No successful_operations += 1 here as it's removed
        
        elif operation == "UPDATE":
            num_rows_to_update = UPDATE_ROWS # Set the number of rows to update in 1 operation
            max_retries = UPDATE_MAX_RETRIES  # Set a maximum number of retries to avoid infinite loops

            for _ in range(max_retries):
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
                # Build SET clause with special handling for bit/varbit
                set_parts = []
                params = []
                for col in columns_to_update:
                    data_type = columns[col]
                    if "bit" in data_type.lower():
                        expr = build_bit_cast_expr(table_schemas, table_name, col)
                        set_parts.append(f"{col} = {expr}")
                    else:
                        if data_type == "USER-DEFINED":
                            enum_values = fetch_enum_values_for_column(table_schemas, table_name, col)
                            value = generate_random_data(data_type, table_name, enum_values, None)
                        else:
                            array_types = fetch_array_types_for_column(table_schemas, table_name, col)
                            value = generate_random_data(data_type, table_name, None, array_types)
                        if value is None:
                            set_parts.append(f"{col} = NULL")
                        else:
                            set_parts.append(f"{col} = %s")
                            params.append(value)
                set_clause = ", ".join(set_parts)
                where_clause = f"{primary_key} IN (SELECT {primary_key} FROM {table_name} TABLESAMPLE SYSTEM_ROWS(%s))"
                # where_clause = f"{primary_key} IN (SELECT {primary_key} FROM {table_name} ORDER BY RANDOM() LIMIT %s)"
                # where_clause = f"{primary_key} IN (SELECT {primary_key} FROM {table_name} TABLESAMPLE SYSTEM (0.00007))"
                query_to_run = f"UPDATE {table_name} SET {set_clause} WHERE {where_clause}"
                params = params + [num_rows_to_update]
                
                # print(query_to_run, params)

                try:
                    cursor.execute(query_to_run, params)
                    conn.commit()
                    # successful_operations += 1 # This line is removed
                    break  # Break out of the loop if the update is successful
                except Exception as e:
                    #print(f"Error executing query: {query_to_run} with params: {params}")
                    #print(f"Error details: {e}")
                    conn.rollback()

        elif operation == "DELETE":
            num_rows_to_delete = DELETE_ROWS # Set the number of rows to delete in 1 operation

            primary_key = table_schemas[table_name]["primary_key"]
            query_to_run = f"DELETE FROM {table_name} WHERE {primary_key} IN (SELECT {primary_key} FROM {table_name} TABLESAMPLE SYSTEM_ROWS(%s))"
            # query_to_run = f"DELETE FROM {table_name} WHERE {primary_key} IN (SELECT {primary_key} FROM {table_name} ORDER BY RANDOM() LIMIT %s)"
            # query_to_run = f"DELETE FROM {table_name} WHERE {primary_key} IN (SELECT {primary_key} FROM {table_name} LIMIT %s)"
            params = (num_rows_to_delete,)
            #print(query_to_run, params)
            cursor.execute(query_to_run, params)

            conn.commit()
            # successful_operations += 1 # This line is removed

        if i % wait_after_operations == 0 and i != 0:
            print("-" * 50)
            print(f"Waiting for {wait_duration_seconds} seconds after {i} operations...")
            print("-" * 50)
            time.sleep(wait_duration_seconds)
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


# Commit changes outside the loop for UPDATE and DELETE operations
conn.commit()

# Close the connection
conn.close()

print("Program Complete")