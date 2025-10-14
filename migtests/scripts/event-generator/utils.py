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
import os
try:
    import yaml  # type: ignore
except Exception:
    yaml = None  # Defer strict error to loader to produce a clearer message

# Module-level Faker instance for reuse; can be overridden via function parameter
_fake = Faker()

# ----- Configuration loading  -----

def load_event_generator_config() -> Dict[str, Any]:
    """
    Load event-generator.yaml and perform basic validation:
    - Ensure PyYAML is available
    - Ensure file exists and top-level is a mapping
    - Ensure required sections and required keys with expected types
    """
    if yaml is None:
        raise RuntimeError(
            "PyYAML is required to load configuration. Install with: pip install PyYAML"
        )

    config_path = os.path.join(os.path.dirname(__file__), "event-generator.yaml")
    if not os.path.exists(config_path):
        raise FileNotFoundError(
            f"Configuration file not found at: {config_path}"
        )

    with open(config_path, "r") as f:
        loaded: Any = yaml.safe_load(f)

    if not isinstance(loaded, dict):
        raise ValueError("Top-level YAML must be a mapping/object")

    # Validate required sections
    for section in ("connection", "generator"):
        if section not in loaded or not isinstance(loaded[section], dict):
            raise ValueError(f"Missing or invalid '{section}' section in config")

    conn = loaded["connection"]
    gen = loaded["generator"]

    # Connection validation
    _require_key_type(conn, "host", str, section_name="connection")
    _require_key_type(conn, "port", int, section_name="connection")
    _require_key_type(conn, "database", str, section_name="connection")
    _require_key_type(conn, "user", str, section_name="connection")
    _require_key_type(conn, "password", str, section_name="connection")

    # Generator validation
    _require_key_type(gen, "schema_name", str)
    _require_key_type(gen, "exclude_table_list", list)
    _require_key_type(gen, "manual_table_list", list)
    _require_key_type(gen, "num_iterations", int)
    _require_key_type(gen, "wait_after_operations", int)
    _require_key_type(gen, "wait_duration_seconds", int)
    _require_key_type(gen, "table_weights", dict)
    _require_key_type(gen, "operations", list)
    _require_key_type(gen, "operation_weights", list)
    _require_key_type(gen, "insert_rows", int)
    _require_key_type(gen, "update_rows", int)
    _require_key_type(gen, "delete_rows", int)
    _require_key_type(gen, "insert_max_retries", int)
    _require_key_type(gen, "update_max_retries", int)

    return loaded  # Strictly the provided values


def _require_key_type(obj: Dict[str, Any], key: str, expected_type: type, *, section_name: str = "generator") -> None:
    if key not in obj:
        raise ValueError(f"Missing key '{key}' in '{section_name}' section")
    if not isinstance(obj[key], expected_type):
        raise ValueError(
            f"Key '{key}' in '{section_name}' must be of type {expected_type.__name__}"
        )


def get_connection_kwargs_from_config(config: Dict[str, Any]) -> Dict[str, Any]:
    """
    Return kwargs to pass to psycopg2.connect, strictly from config.
    """
    conn = config.get("connection", {})
    return {
        "dbname": conn["database"],
        "user": conn["user"],
        "password": conn["password"],
        "host": conn["host"],
        "port": conn["port"],
    }

# ----- Schema discovery/introspection -----

def _qualify_regclass(table_name: str, schema_name: Optional[str]) -> str:
    """Return schema-qualified identifier for regclass resolution when schema is provided."""
    return f"{schema_name}.{table_name}" if schema_name else table_name


def fetch_enum_labels(cursor: Any, table_name: str, column_name: str, schema_name: Optional[str]) -> List[str]:
    """
    Return enum labels for a column that is either a scalar enum or an array of enums.
    Uses pg_type to resolve element type when needed.
    """
    enum_query = """
        SELECT enumlabel
        FROM pg_enum
        WHERE enumtypid = (
            SELECT
                CASE
                    WHEN t.typelem != 0 THEN t.typelem
                    ELSE t.oid
                END
            FROM pg_attribute a
            JOIN pg_type t ON a.atttypid = t.oid
            WHERE a.attrelid = %s::regclass
              AND a.attname = %s
        )
    """
    regclass = _qualify_regclass(table_name, schema_name)
    cursor.execute(enum_query, (regclass, column_name))
    return [row[0] for row in cursor.fetchall()]


def get_array_element_type(cursor: Any, schema_name: Optional[str], table_name: str, column_name: str) -> Optional[str]:
    """
    Return the element type (regtype text) for an ARRAY column, or None if not resolvable.
    Looks up information_schema.columns.udt_name and casts to regtype.
    """
    if schema_name:
        query = """
            SELECT udt_name::regtype
            FROM information_schema.columns
            WHERE table_schema = %s
              AND table_name = %s
              AND column_name = %s
        """
        cursor.execute(query, (schema_name, table_name, column_name))
    else:
        query = """
            SELECT udt_name::regtype
            FROM information_schema.columns
            WHERE table_name = %s
              AND column_name = %s
        """
        cursor.execute(query, (table_name, column_name))
    row = cursor.fetchone()
    return row[0] if row else None


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
            regclass = _qualify_regclass(table_name, schema_name)
            cursor.execute("""
                SELECT (atttypmod - 4) >> 16 AS precision, 
                       (atttypmod - 4) & 65535 AS scale 
                FROM pg_attribute 
                WHERE attrelid = %s::regclass 
                  AND attname = %s
            """, (regclass, column_name))
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
            regclass = _qualify_regclass(table_name, schema_name)
            cursor.execute(
                """
                SELECT atttypmod
                FROM pg_attribute
                WHERE attrelid = %s::regclass
                  AND attname = %s
                """,
                (regclass, column_name),
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
            elem_type = get_array_element_type(cursor, schema_name, table_name, column_name)
            if elem_type:
                array_types[column_name] = elem_type
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
        regclass = _qualify_regclass(table_name, schema_name)
        cursor.execute(primary_key_query, (regclass,))
        result = cursor.fetchone()
        primary_key = result[0] if result else None

    # Determine enum values for USER-DEFINED columns (including arrays of enums)
    user_defined_columns = [
        (column_name, data_type)
        for column_name, data_type in column_info
        if data_type == "USER-DEFINED" or "ARRAY" in data_type.upper()
    ]

    for column_name, data_type in user_defined_columns:
        values = fetch_enum_labels(cursor, table_name, column_name, schema_name)
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


# ----- Data lookup helpers -----

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


def fetch_bit_info_for_column(
    table_schemas: Dict[str, Dict[str, Any]],
    table_name: str,
    column_name: str,
) -> Optional[Dict[str, Any]]:
    """Return bit/varbit metadata for a column if present."""
    if table_name in table_schemas and "bit_info" in table_schemas[table_name]:
        return table_schemas[table_name]["bit_info"].get(column_name)
    return None


# ----- SQL/data generators -----

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


# ----- Execution utility -----

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
