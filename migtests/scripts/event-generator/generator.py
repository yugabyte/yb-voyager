import random
import psycopg2
from utils import generate_table_schemas
from utils import (
    build_bit_cast_expr,
    generate_random_data,
    fetch_enum_values_for_column,
    fetch_array_types_for_column,
)
import time


# Connect to PostgreSQL

conn = psycopg2.connect(dbname="test", user="postgres", password="postgres", host="localhost", port="5432")
cursor = conn.cursor()
cursor.execute("""
    CREATE EXTENSION IF NOT EXISTS tsm_system_rows;
""")
print("tsm_system_rows extension is present or created successfully")

# Disabled to allow triggers and constraints to execute
# cursor.execute("SET session_replication_role = 'replica';")

# Initialize Faker for generating random data
# fake = Faker()

print("Generator starting")
print("Note: No. of iterations may not equal number of events")
print("Analysing schema")
#
# Either we can provide our own table list
# Or we can specify the schema from where all tables will be picked up
#

# Manual list

# manual_table_list = ["v","v1","v2"]
# table_schemas = generate_table_schemas(cursor, manual_table_list=manual_table_list)
# print(table_schemas)

# Schema based
exclude_table_list = ['table_to_be_excluded',]
schema_name = "public"
table_schemas = generate_table_schemas(cursor, schema_name=schema_name, exclude_table_list=exclude_table_list)
# print(table_schemas)

print("Schema analysed")

def execute_with_retry(run_once_fn, rebuild_fn, *, max_retries=50):
    retry_count = 0
    while retry_count <= max_retries:
        try:
            run_once_fn()
            return True
        except psycopg2.errors.UniqueViolation as e:
            conn.rollback()
            retry_count += 1
            print(f"Retrying operation after UniqueViolation (attempt {retry_count} of {max_retries})")
            print(f"Error details: {e}")
            rebuild_fn()
        except Exception:
            # For non-unique errors, just propagate after rollback
            conn.rollback()
            raise
    print("Reached maximum retry attempts. Skipping...")
    return False


num_iterations = 20000 # Specify the number of iterations
wait_after_operations = 200000  # Adjust this value to set the desired wait interval
wait_duration_seconds = 0  # Adjust this value to set the duration of the wait in seconds


for i in range(num_iterations):
    # Choose a random table
    table_weights = {} #specify weights for tables you want to prioritise
    # table_weights = {"table1": 5, "table2": 4}
    for table in table_schemas.keys():
        table_weights.setdefault(table, 1)

    table_name = random.choices(list(table_weights.keys()), weights=list(table_weights.values()))[0]

    #print(table_name)
    #table_name = random.choice(list(table_schemas.keys()))

    # Generate a random operation
    # operations = ["INSERT"]
    # weights = [1]
    operations = ["INSERT", "UPDATE", "DELETE"]
    weights = [3,2,1]  # Mention the weight of the operations

    operation = random.choices(operations, weights=weights)[0]

    # print(operation)

    try:
        if operation == "INSERT":
    # Generate random data and execute INSERT statement
            columns = ", ".join(table_schemas[table_name]["columns"].keys())
            values_list = []
            number_of_rows_to_insert = 40 # Adjust as needed
            for _ in range(number_of_rows_to_insert):
                values = []
                for column_name, data_type in table_schemas[table_name]["columns"].items():
                    if "bit" in data_type.lower():
                        values.append(build_bit_cast_expr(table_schemas, table_name, column_name, data_type))
                    elif data_type != "USER-DEFINED" and data_type != "ARRAY":
                        values.append(f"'{generate_random_data(data_type, table_name, None, None)}'")
                    else:
                        enum_values = fetch_enum_values_for_column(table_schemas, table_name, column_name)
                        array_types = fetch_array_types_for_column(table_schemas, table_name, column_name)
                        value = generate_random_data(data_type, table_name, enum_values, array_types)
                        values.append(f"'{value}'" if value is not None else "NULL")
                values_list.append(f"({', '.join(values)})")

            values_list = ", ".join(values_list)
            values_holder = {"values_list": values_list}

            # Prepare callbacks for retryable execution
            def run_once():
                query_to_run = f"INSERT INTO {table_name} ({columns}) VALUES {values_holder['values_list']}"
                cursor.execute(query_to_run)

            def rebuild():
                regenerated_rows = []
                for _ in range(number_of_rows_to_insert):
                    vals = []
                    for column_name, data_type in table_schemas[table_name]["columns"].items():
                        if "bit" in data_type.lower():
                            vals.append(build_bit_cast_expr(table_schemas, table_name, column_name, data_type))
                        elif data_type != "USER-DEFINED" and data_type != "ARRAY":
                            vals.append(f"'{generate_random_data(data_type, table_name, None, None)}'")
                        else:
                            enum_values = fetch_enum_values_for_column(table_schemas, table_name, column_name)
                            array_types = fetch_array_types_for_column(table_schemas, table_name, column_name)
                            v = generate_random_data(data_type, table_name, enum_values, array_types)
                            vals.append(f"'{v}'" if v is not None else "NULL")
                    regenerated_rows.append(f"({', '.join(vals)})")
                values_holder["values_list"] = ", ".join(regenerated_rows)

            success = execute_with_retry(run_once, rebuild, max_retries=50)
            if success:
                pass # No successful_operations += 1 here as it's removed
        
        elif operation == "UPDATE":
            num_rows_to_update = 20 # Set the number of rows to update in 1 operation
            max_retries = 3  # Set a maximum number of retries to avoid infinite loops

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
                        expr = build_bit_cast_expr(table_schemas, table_name, col, data_type)
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
            num_rows_to_delete = 10 # Set the number of rows to delete in 1 operation

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