import random
import string
from faker import Faker
import json
import ipaddress
import re
import decimal
import uuid
import psycopg2
from typing import Any, Callable, Dict, List, Optional, Tuple

def get_table_description(cursor: Any, table_name: str, schema_name: Optional[str] = None) -> List[Tuple[str, str]]:
    """Return (column_name, data_type) for a table, expanding numeric/decimal precision/scale."""
    if schema_name:
        cursor.execute(
            """
            SELECT column_name, data_type
            FROM information_schema.columns
            WHERE table_schema = %s AND table_name = %s
            """,
            (schema_name, table_name,),
        )
    else:
        cursor.execute(
            f"SELECT column_name, data_type FROM information_schema.columns WHERE table_name = %s",
            (table_name,),
        )
    column_info = cursor.fetchall()

    for i, (column_name, data_type) in enumerate(column_info):
        if data_type in ('numeric', 'decimal'):
            cursor.execute("""
                SELECT (atttypmod - 4) >> 16 AS precision, 
                       (atttypmod - 4) & 65535 AS scale 
                FROM pg_attribute 
                WHERE attrelid = %s::regclass 
                  AND attname = %s
            """, (table_name, column_name))
            precision, scale = cursor.fetchone()
            column_info[i] = (column_name, f"{data_type}({precision},{scale})")

    return column_info

def convert_pg_table_description(
    cursor: Any,
    column_info: List[Tuple[str, str]],
    table_name: str,
    schema_name: Optional[str] = None,
) -> Dict[str, Dict[str, Any]]:
    """Convert column info into a schema dict (columns, arrays, PK, enums, bit/varbit)."""
    columns = {}
    array_types = {}
    enum_values = {}
    bit_info = {}

    for column_name, data_type in column_info:
        columns[column_name] = data_type

        # Capture bit/varbit length metadata
        if data_type.lower() in ('bit', 'bit varying'):
            cursor.execute(
                """
                SELECT atttypmod
                FROM pg_attribute
                WHERE attrelid = %s::regclass
                  AND attname = %s
                """,
                (table_name, column_name),
            )
            atttypmod_row = cursor.fetchone()
            bit_length = None
            if atttypmod_row and atttypmod_row[0] is not None:
                # For bit/varbit, atttypmod is the declared length; -1 means unlimited (varbit)
                if atttypmod_row[0] > 0:
                    bit_length = atttypmod_row[0]
                else:
                    bit_length = None
            bit_info[column_name] = {
                "varying": data_type.lower() == 'bit varying',
                "length": bit_length,
            }

        # Check for array types
        if 'ARRAY' in data_type.upper():
            # Get the actual array type using the specified query
            if schema_name:
                array_type_query = f"""
                    SELECT udt_name::regtype
                    FROM information_schema.columns 
                    WHERE table_schema = %s
                      AND table_name = %s
                      AND column_name = %s
                """
                cursor.execute(array_type_query, (schema_name, table_name, column_name))
            else:
                array_type_query = f"""
                    SELECT udt_name::regtype
                    FROM information_schema.columns 
                    WHERE table_name = %s
                      AND column_name = %s
                """
                cursor.execute(array_type_query, (table_name, column_name))
            array_type_result = cursor.fetchone()

            if array_type_result:
                array_types[column_name] = array_type_result[0]
            else:
                # Use the original data_type if the query doesn't return a result
                array_types[column_name] = data_type

    # Determine the primary key column
    if schema_name:
        primary_key_query = f"""
            SELECT kcu.column_name
            FROM information_schema.table_constraints tc
            JOIN information_schema.key_column_usage kcu
              ON tc.constraint_name = kcu.constraint_name
             AND tc.table_schema = kcu.table_schema
             AND tc.table_name = kcu.table_name
            WHERE tc.table_schema = %s
              AND tc.table_name = %s
              AND tc.constraint_type = 'PRIMARY KEY'
            ORDER BY kcu.ordinal_position
        """
        cursor.execute(primary_key_query, (schema_name, table_name))
        result = cursor.fetchone()
        primary_key = result[0] if result else None
    else:
        # Fallback using pg_catalog with search_path resolution via regclass
        primary_key_query = f"""
            SELECT a.attname
            FROM pg_index i
            JOIN pg_class c ON c.oid = i.indrelid
            JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = ANY(i.indkey)
            WHERE c.oid = %s::regclass
              AND i.indisprimary
            ORDER BY a.attnum
        """
        cursor.execute(primary_key_query, (table_name,))
        result = cursor.fetchone()
        primary_key = result[0] if result else None

    # Determine enum values for USER-DEFINED columns
    user_defined_columns = [(column_name, data_type) for column_name, data_type in column_info if not data_type.startswith('_')]

    for column_name, _ in user_defined_columns:
        enum_query = f"""
            SELECT enumlabel
            FROM pg_enum
            WHERE enumtypid = (
                SELECT atttypid
                FROM pg_attribute
                WHERE attrelid = %s::regclass
                AND attname = %s
            )
        """
        cursor.execute(enum_query, (table_name, column_name))
        values = [row[0] for row in cursor.fetchall()]
        if values:
            enum_values[column_name] = values

    result = {
        "columns": columns,
        "array_types": array_types,
        "primary_key": primary_key,
        "enum_values": enum_values,
        "bit_info": bit_info,
    }

    return {table_name: result}

def get_table_list(cursor: Any, schema_name: Optional[str] = None, exclude_table_list: Optional[List[str]] = None) -> List[str]:
    """List base tables in a schema (or all schemas), excluding given names."""
    if schema_name:
        cursor.execute("""
            SELECT table_name 
            FROM information_schema.tables 
            WHERE table_schema = %s AND table_type = 'BASE TABLE'
        """, (schema_name,))
    else:
        cursor.execute("""
            SELECT table_name 
            FROM information_schema.tables 
            WHERE table_type = 'BASE TABLE'
        """)

    # Always return a flat list of table names
    tables = [row[0] for row in cursor.fetchall()]
    if exclude_table_list:
        tables = [t for t in tables if t not in exclude_table_list]

    return tables

def execute_with_retry(
    run_once_fn: Callable[[], None],
    rebuild_fn: Callable[[], None],
    rollback_fn: Callable[[], None],
    *,
    max_retries: int = 50,
) -> bool:
    """Execute write, retrying on UniqueViolation with regenerated values; return success."""
    retry_count = 0
    while retry_count <= max_retries:
        try:
            run_once_fn()
            return True
        except psycopg2.errors.UniqueViolation as e:
            rollback_fn()
            retry_count += 1
            print(f"Retrying operation after UniqueViolation (attempt {retry_count} of {max_retries})")
            print(f"Error details: {e}")
            rebuild_fn()
        except Exception:
            # For non-unique errors, propagate after rollback
            rollback_fn()
            raise
    print("Reached maximum retry attempts. Skipping...")
    return False

def generate_table_schemas(
    cursor: Any,
    schema_name: Optional[str] = None,
    manual_table_list: Optional[List[str]] = None,
    exclude_table_list: Optional[List[str]] = None,
) -> Dict[str, Dict[str, Any]]:
    """Build generator schemas from information_schema and pg_catalog."""
    if manual_table_list:
        table_list = manual_table_list
    else:
        table_list = get_table_list(cursor, schema_name, exclude_table_list)

    table_schemas = {}
    for table_name in table_list:
        column_info = get_table_description(cursor, table_name, schema_name)

        if column_info:
            result = convert_pg_table_description(cursor, column_info, table_name, schema_name)
            table_schemas.update(result)
        else:
            print(f"Table '{table_name}' not found.")

    return table_schemas

# Module-level Faker instance for reuse; can be overridden via function parameter
_fake = Faker()

def fetch_bit_info_for_column(
    table_schemas: Dict[str, Dict[str, Any]],
    table_name: str,
    column_name: str,
) -> Optional[Dict[str, Any]]:
    """Return bit/varbit metadata for a column if present."""
    if table_name in table_schemas and "bit_info" in table_schemas[table_name]:
        return table_schemas[table_name]["bit_info"].get(column_name)
    return None

def build_bit_cast_expr(
    table_schemas: Dict[str, Dict[str, Any]],
    table_name: str,
    column_name: str,
) -> str:
    """Construct a CAST expression for a valid bit/varbit literal for the column."""
    info = fetch_bit_info_for_column(table_schemas, table_name, column_name)
    # Default safe lengths if metadata missing
    is_varying = False
    max_len = None
    if info:
        is_varying = bool(info.get("varying"))
        max_len = info.get("length")
    # Determine length to generate
    if is_varying:
        # Choose length within max if specified, else up to 64
        chosen_len = random.randint(1, max_len if isinstance(max_len, int) and max_len > 0 else 64)
        bit_str = ''.join(random.choice(['0', '1']) for _ in range(chosen_len))
        if isinstance(max_len, int) and max_len > 0:
            return f"CAST('{bit_str}' AS varbit({max_len}))"
        else:
            return f"CAST('{bit_str}' AS varbit)"
    else:
        # Fixed bit(n); if no length, default to 8
        fixed_len = max_len if isinstance(max_len, int) and max_len > 0 else 8
        bit_str = ''.join(random.choice(['0', '1']) for _ in range(fixed_len))
        return f"CAST('{bit_str}' AS bit({fixed_len}))"

def build_insert_values(
    table_schemas: Dict[str, Dict[str, Any]],
    table_name: str,
    number_of_rows_to_insert: int,
) -> str:
    """Build VALUES list like (v1, v2), (v1, v2) for INSERT ... VALUES ..."""
    rows = []
    for _ in range(number_of_rows_to_insert):
        values = []
        for column_name, data_type in table_schemas[table_name]["columns"].items():
            if "bit" in data_type.lower():
                values.append(build_bit_cast_expr(table_schemas, table_name, column_name))
            elif data_type != "USER-DEFINED" and data_type != "ARRAY":
                values.append(f"'{generate_random_data(data_type, table_name, None, None)}'")
            else:
                enum_values = fetch_enum_values_for_column(table_schemas, table_name, column_name)
                array_types = fetch_array_types_for_column(table_schemas, table_name, column_name)
                value = generate_random_data(data_type, table_name, enum_values, array_types)
                values.append(f"'{value}'" if value is not None else "NULL")
        rows.append(f"({', '.join(values)})")
    return ", ".join(rows)

def generate_random_data(
    data_type: str,
    table_name: str,
    enum_values: Optional[List[str]] = None,
    array_types: Optional[str] = None,
    faker_instance: Optional[Faker] = None,
) -> Any:
    """Generate random data compatible with a Postgres column type."""
    fake = faker_instance or _fake
    if "varchar" in data_type or "text" in data_type or "character varying" in data_type or "bytea" in data_type:
        value = ' '.join([fake.word() for _ in range(3)])
        return value # Change 3 to the desired number of words
    elif "boolean" in data_type:
        return random.choice(["true", "false"])
    elif "char" in data_type:
        return fake.word()[:1]
    elif "USER-DEFINED" in data_type and enum_values:
        val = random.choice(enum_values)
        return val
    elif "USER-DEFINED" in data_type and not enum_values:
        print(f"Inserting NULL since User-Defined type unknown for table: {table_name}")
        return None  # Return None for USER-DEFINED types without enum_values
    elif "timestamp" in data_type:
        return fake.iso8601(tzinfo=None)
    elif "numeric" in data_type or "double precision" in data_type:
        match = re.search(r"\((\d+),(\d+)\)", data_type)
        if match:
            precision, scale = map(int, match.groups())
        else:
            precision, scale = 7, 2

        # max_value = 10 ** (precision - scale) - 10 ** -scale
        # return round(random.uniform(-max_value, max_value), scale)

        floatStr=""
        for i in range(precision-scale):
            floatStr+=random.choice(string.digits)

        decimalStr=""
        for i in range(scale):
            decimalStr+=random.choice(string.digits)

        num = decimal.Decimal(f"{floatStr}.{decimalStr}")
        return num

    elif "smallint" in data_type:
        return random.randint(-1000, 1000)
    elif "integer" in data_type:
        return random.randint(-200000000, 200000000)
    elif "bigint" in data_type:
        return random.randint(-9223372000000000000, 9223372000000000000)
    elif "date" in data_type:
        return fake.date()
    elif "time" in data_type:
        return fake.time()
    elif "json" in data_type or "jsonb" in data_type:
        # Generate a random JSON object (customize based on your requirements)
        json_data = {fake.word(): fake.word(), fake.word(): random.randint(-10000, 10000), fake.word(): fake.date()}
        return json.dumps(json_data)
    elif "inet" in data_type:
        # Generate a random IP address
        return str(ipaddress.IPv4Address(random.randint(2**24, 2**32 - 1)))
    elif "money" in data_type:
        precision, scale = 5, 2  # Adjust precision and scale as needed
        max_value = 10 ** (precision - scale)
        money_value = random.randint(0, max_value * 100) / 100
        return money_value

    elif "ARRAY" in data_type and array_types:
        # Handle ARRAY data type based on the specific array element type
        if "varchar" in array_types or "text" in array_types:
            result = [f'"{fake.word()}"' for _ in range(3)] # Change 3 to the desired number of words
            return '{' + ', '.join(result) + '}'
        elif "integer" in array_types:
            return {random.randint(-100000, 100000) for _ in range(3)}  # Change 3 to the desired number of elements
        # Add more cases for other ARRAY data types as needed

    elif "uuid" in data_type:
        return str(uuid.uuid4())
    
    elif "tsvector" in data_type:
        words = [fake.word() for _ in range(5)]
        return ' '.join(words)
    
    # -- START: BIT TYPES --
    elif "bit" in data_type.lower():
        # We now generate bit strings with correct widths during INSERT construction
        # Return a placeholder; actual value and CAST are handled by caller
        return None
    # -- END: BIT TYPES --
        
    else:
        print(f"No handling for data type: {data_type}")
        return None

def fetch_enum_values_for_column(
    table_schemas: Dict[str, Dict[str, Any]],
    table_name: str,
    column_name: str,
) -> List[str]:
    """Return enum labels for a USER-DEFINED enum column, else empty list."""
    enum_values = []

    # Check if the table and column exist in the schemas
    if table_name in table_schemas and "columns" in table_schemas[table_name]:
        columns_info = table_schemas[table_name]["columns"]
        #print(columns_info)
        if column_name in columns_info:
            data_type = columns_info[column_name]
            if data_type == "USER-DEFINED" and "enum_values" in table_schemas[table_name]:
                # Fetch enum values based on the column name
                enum_values = table_schemas[table_name]["enum_values"].get(column_name, [])
    #print(enum_values)
    return enum_values

def fetch_array_types_for_column(
    table_schemas: Dict[str, Dict[str, Any]],
    table_name: str,
    column_name: str,
) -> Optional[str]:
    """Return element type for an ARRAY column (e.g., 'integer'), if known."""
    array_types = {}

    # Check if the table and column exist in the schemas
    if table_name in table_schemas and "array_types" in table_schemas[table_name]:
        array_types = table_schemas[table_name]["array_types"]

    return array_types.get(column_name, None)
