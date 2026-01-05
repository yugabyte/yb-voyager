import random
import string
from faker import Faker
import json
import ipaddress
import re
import decimal
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

"""
Declarative configuration schema and helpers for event-generator config loading.
"""

# ---------------------
# Declarative config schema
# ---------------------
CONFIG_SCHEMA: Dict[str, Dict[str, Any]] = {
    "connection": {
        "host": str,
        "port": int,
        "database": str,
        "user": str,
        "password": str,
    },
    "generator": {
        "schema_name": str,
        "exclude_table_list": list,
        "manual_table_list": list,
        "num_iterations": int,
        "wait_after_operations": int,
        "wait_duration_seconds": int,
        "table_weights": dict,
        "operation_weights": dict,
        "insert_rows": int,
        "update_rows": int,
        "delete_rows": int,
        "insert_max_retries": int,
        "update_max_retries": int,
        "min_col_size_bytes": int
    },
}

# ---------------------
# Helper functions
# ---------------------
def load_yaml_file(path: str) -> Dict[str, Any]:
    if yaml is None:
        raise RuntimeError("PyYAML is required to load configuration. Install with: pip install PyYAML")
    if not os.path.exists(path):
        raise FileNotFoundError(f"Configuration file not found at: {path}")

    with open(path, "r") as f:
        content: Any = yaml.safe_load(f)
    if not isinstance(content, dict):
        raise ValueError("Top-level YAML must be a mapping/object")
    return content


def validate_section(section: Dict[str, Any], schema: Dict[str, Any], section_name: str) -> None:
    for key, expected_type in schema.items():
        if key not in section:
            raise ValueError(f"Missing key '{key}' in '{section_name}' section")
        if not isinstance(section[key], expected_type):
            raise ValueError(
                f"Key '{key}' in '{section_name}' must be of type {expected_type.__name__}"
            )


# ---------------------
# Top-level loader
# ---------------------
def load_event_generator_config(path_override: Optional[str] = None) -> Dict[str, Any]:
    """
    Load event-generator.yaml and validate against CONFIG_SCHEMA.
    Returns the loaded config dict as-is.
    """
    if path_override:
        # Support relative paths and '~' expansion
        config_path = os.path.abspath(os.path.expanduser(path_override))
    else:
        config_path = os.path.join(os.path.dirname(__file__), "event-generator.yaml")
    config = load_yaml_file(config_path)

    for section_name, schema in CONFIG_SCHEMA.items():
        section = config.get(section_name)
        if not isinstance(section, dict):
            raise ValueError(f"Missing or invalid '{section_name}' section in config")
        validate_section(section, schema, section_name)

    return config

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


def detect_db_flavor(cursor: Any) -> str:
    """
    Detect the database flavor based on SELECT version().
    Returns:
        "YUGABYTE" when the version string contains "YB" (YugabyteDB),
        otherwise "POSTGRES".
    """
    cursor.execute("SELECT version()")
    row = cursor.fetchone()
    version_str = row[0] if row and row[0] is not None else ""
    if "YB" in version_str.upper():
        flavor = "YUGABYTE"
    else:
        flavor = "POSTGRES"

    print(f"Detected database flavor: {flavor}")
    return flavor


def get_estimated_row_count(
    cursor: Any,
    schema_name: str,
    table_name: str,
) -> Optional[int]:
    """
    Return the estimated row count for a table using pg_class.reltuples.
    """
    cursor.execute(
        """
        SELECT reltuples::bigint
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = %s
          AND c.relname = %s
        """,
        (schema_name, table_name),
    )
    row = cursor.fetchone()
    if not row or row[0] is None or row[0] < 0:
        return None
    return int(row[0])

def set_faker_seed(seed: int) -> None:
    _fake.seed_instance(seed)

# ----- Schema discovery/introspection -----

def _qualify_regclass(table_name: str, schema_name: Optional[str]) -> str:
    """Return schema-qualified identifier for regclass resolution when schema is provided."""
    return f"{schema_name}.{table_name}" if schema_name else table_name


def _schema_filter(schema_name: Optional[str]) -> Tuple[str, Tuple[Any, ...]]:
    """Return a WHERE prefix and params for information_schema queries.

    Example:
        ("table_schema = %s AND ", (schema_name,)) when schema is provided
        ("", ()) when schema is not provided
    """
    if schema_name:
        return "table_schema = %s AND ", (schema_name,)
    return "", ()


def get_array_element_type(cursor: Any, schema_name: Optional[str], table_name: str, column_name: str) -> Optional[str]:
    """
    Return the element type (regtype text) for an ARRAY column, or None if not resolvable.
    Looks up information_schema.columns.udt_name and casts to regtype.
    """
    where_prefix, where_params = _schema_filter(schema_name)
    query = f"""
            SELECT udt_name::regtype
            FROM information_schema.columns
            WHERE {where_prefix} table_name = %s
              AND column_name = %s
        """
    cursor.execute(query, where_params + (table_name, column_name))
    row = cursor.fetchone()
    return row[0] if row else None


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

def get_table_list(cursor: Any, schema_name: Optional[str] = None, exclude_table_list: Optional[List[str]] = None) -> List[str]:
    """List base tables in a schema (or all schemas), excluding given names."""
    where_prefix, where_params = _schema_filter(schema_name)
    cursor.execute(
        f"""
            SELECT table_name 
            FROM information_schema.tables 
            WHERE {where_prefix} table_type = 'BASE TABLE'
        """,
        where_params,
    )

    # Always return a flat list of table names
    tables = [row[0] for row in cursor.fetchall()]
    if exclude_table_list:
        tables = [t for t in tables if t not in exclude_table_list]

    return tables


def get_table_description(cursor: Any, table_name: str, schema_name: Optional[str] = None) -> List[Tuple[str, str]]:
    """Return (column_name, data_type) for a table, expanding numeric/decimal precision/scale."""
    where_prefix, where_params = _schema_filter(schema_name)
    cursor.execute(
        f"""
            SELECT column_name, data_type
            FROM information_schema.columns
            WHERE {where_prefix} table_name = %s
            """,
        where_params + (table_name,),
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

def _build_columns_dict(column_info: List[Tuple[str, str]]) -> Dict[str, str]:
    """Return a mapping of column_name -> data_type from information_schema results."""
    return {column_name: data_type for column_name, data_type in column_info}


def _build_bit_info(
    cursor: Any,
    table_name: str,
    schema_name: Optional[str],
    columns: Dict[str, str],
) -> Dict[str, Dict[str, Any]]:
    """Return bit/varbit metadata dict for columns that are declared as bit/varbit."""
    bit_info: Dict[str, Dict[str, Any]] = {}
    regclass = _qualify_regclass(table_name, schema_name)
    for column_name, data_type in columns.items():
        if data_type.lower() in ('bit', 'bit varying'):
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
    return bit_info


def _build_array_types(
    cursor: Any,
    schema_name: Optional[str],
    table_name: str,
    columns: Dict[str, str],
) -> Dict[str, str]:
    """Return mapping of ARRAY columns to their element type (or original data_type as fallback)."""
    array_types: Dict[str, str] = {}
    for column_name, data_type in columns.items():
        if 'ARRAY' in data_type.upper():
            elem_type = get_array_element_type(cursor, schema_name, table_name, column_name)
            if elem_type:
                array_types[column_name] = elem_type
            else:
                array_types[column_name] = data_type
    return array_types


def _find_primary_key(
    cursor: Any,
    table_name: str,
    schema_name: Optional[str],
) -> Optional[str]:
    """Return the name of the primary key column or None if not found."""
    regclass = _qualify_regclass(table_name, schema_name)
    cursor.execute(
        """
            SELECT a.attname
            FROM pg_index i
            JOIN pg_class c ON c.oid = i.indrelid
            JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = ANY(i.indkey)
            WHERE c.oid = %s::regclass
              AND i.indisprimary
            ORDER BY a.attnum
        """,
        (regclass,),
    )
    result = cursor.fetchone()
    return result[0] if result else None


def _build_enum_values(
    cursor: Any,
    table_name: str,
    schema_name: Optional[str],
    column_info: List[Tuple[str, str]],
) -> Dict[str, List[str]]:
    """Return mapping of column_name -> enum labels for USER-DEFINED columns (incl. arrays)."""
    enum_values: Dict[str, List[str]] = {}
    user_defined_columns = [
        (column_name, data_type)
        for column_name, data_type in column_info
        if data_type == "USER-DEFINED" or "ARRAY" in data_type.upper()
    ]
    for column_name, data_type in user_defined_columns:
        values = fetch_enum_labels(cursor, table_name, column_name, schema_name)
        if values:
            enum_values[column_name] = values
    return enum_values


def convert_pg_table_description(
    cursor: Any,
    column_info: List[Tuple[str, str]],
    table_name: str,
    schema_name: Optional[str] = None,
) -> Dict[str, Dict[str, Any]]:
    """Convert column info into a schema dict (columns, arrays, PK, enums, bit/varbit)."""
    columns = _build_columns_dict(column_info)
    bit_info = _build_bit_info(cursor, table_name, schema_name, columns)
    array_types = _build_array_types(cursor, schema_name, table_name, columns)
    primary_key = _find_primary_key(cursor, table_name, schema_name)
    enum_values = _build_enum_values(cursor, table_name, schema_name, column_info)

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
        if column_name in columns_info:
            data_type = columns_info[column_name]
            if data_type == "USER-DEFINED" and "enum_values" in table_schemas[table_name]:
                # Fetch enum values based on the column name
                enum_values = table_schemas[table_name]["enum_values"].get(column_name, [])
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
    min_col_size_bytes: int = 0,
) -> Any:
    """Generate random data compatible with a Postgres column type."""
    fake = faker_instance or _fake

    if "text" in data_type or "bytea" in data_type and min_col_size_bytes > 0:
        parts = []
        batch_size = max(1, min_col_size_bytes // 10)
        while True:
            parts.append(fake.pystr(min_chars=batch_size, max_chars=batch_size))
            value = "".join(parts)
            if len(value.encode("utf-8")) >= min_col_size_bytes:
                return value

    elif "json" in data_type or "jsonb" in data_type:
        obj = {}
        while len(json.dumps(obj).encode("utf-8")) < min_col_size_bytes:
            chunk_size = max(1000, min_col_size_bytes // 10)  # Generate in chunks
            text_value = fake.text(max_nb_chars=chunk_size)
            obj[fake.word()] = text_value
        return json.dumps(obj)

    elif "tsvector" in data_type and min_col_size_bytes > 0:
        words = []
        while True:
            chunk_size = max(1000, min_col_size_bytes // 10)  # Generate in chunks
            text_chunk = fake.text(max_nb_chars=chunk_size)
            words.append(text_chunk)
            value = ' '.join(words)
            if len(value.encode("utf-8")) >= min_col_size_bytes:
                return value

    elif "ARRAY" in data_type and min_col_size_bytes > 0 and array_types:
        elements = []

        def gen_elem():
            if "int" in array_types:
                return str(random.randint(0, 1_000_000))
            elif "bool" in array_types:
                return random.choice(["true", "false"])
            elif "uuid" in array_types:
                return f'"{fake.uuid4()}"'
            else:
                return f'"{fake.word()}"'

        BATCH_SIZE = max(10, min_col_size_bytes // 10)

        while True:
            # grow in batches
            elements.extend(gen_elem() for _ in range(BATCH_SIZE))

            value = "{" + ",".join(elements) + "}"

            if len(value.encode("utf-8")) >= min_col_size_bytes:
                return value

    elif "varchar" in data_type or "text" in data_type or "character varying" in data_type or "bytea" in data_type:
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
    elif "integer" in data_type or "real" in data_type:
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
            # Produce a deterministic, ordered array literal (not a Python set)
            vals = [str(random.randint(-100000, 100000)) for _ in range(3)]
            return '{' + ', '.join(vals) + '}'
        # Add more cases for other ARRAY data types as needed

    elif "uuid" in data_type:
        # Use Faker's uuid4 which is seeded via set_faker_seed for determinism
        return _fake.uuid4()
    
    elif "tsvector" in data_type:
        words = [fake.word() for _ in range(5)]
        return ' '.join(words)

    elif "tsquery" in data_type:
        words = [fake.word() for _ in range(5)]
        return ' & '.join(words)
    
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
    min_col_size_bytes: int = 0,
) -> str:
    """Build VALUES list like (v1, v2), (v1, v2) for INSERT ... VALUES ..."""
    rows = []
    for _ in range(number_of_rows_to_insert):
        values = []

        for column_name, data_type in table_schemas[table_name]["columns"].items():
            if "bit" in data_type.lower():
                values.append(build_bit_cast_expr(table_schemas, table_name, column_name))
            elif data_type != "USER-DEFINED" and data_type != "ARRAY":
                value = generate_random_data(data_type, table_name, None, None, None, min_col_size_bytes)
                if "bytea" in data_type and isinstance(value, bytes):
                    hex_value = value.hex()
                    values.append(f"'\\\\x{hex_value}'")
                else:
                    if isinstance(value, str):
                        escaped_value = value.replace("'", "''")
                        values.append(f"'{escaped_value}'")
                    else:
                        values.append(f"'{value}'" if value is not None else "NULL")
            else:
                enum_values = fetch_enum_values_for_column(table_schemas, table_name, column_name)
                array_types = fetch_array_types_for_column(table_schemas, table_name, column_name)
                value = generate_random_data(data_type, table_name, enum_values, array_types, None, min_col_size_bytes)
                if isinstance(value, str):
                    escaped_value = value.replace("'", "''")
                    values.append(f"'{escaped_value}'" if value is not None else "NULL")
                else:
                    values.append(f"'{value}'" if value is not None else "NULL")
        rows.append(f"({', '.join(values)})")
    return ", ".join(rows)


# ----- UPDATE helpers -----

def build_update_values(
    table_schemas: Dict[str, Dict[str, Any]],
    table_name: str,
    columns_to_update: List[str],
    min_col_size_bytes: int = 0,
) -> Tuple[str, List[Any]]:
    """Build a SET clause and params for UPDATE with type-aware handling.

    Returns a tuple of (set_clause, params), where set_clause is a comma-joined
    list of column assignments and params are the corresponding values for
    non-bit columns.
    """
    columns = table_schemas[table_name]["columns"]
    set_parts: List[str] = []
    params: List[Any] = []

    for col in columns_to_update:
        data_type = columns[col]
        if "bit" in data_type.lower():
            expr = build_bit_cast_expr(table_schemas, table_name, col)
            set_parts.append(f"{col} = {expr}")
        else:
            if data_type == "USER-DEFINED":
                enum_values = fetch_enum_values_for_column(table_schemas, table_name, col)
                value = generate_random_data(data_type, table_name, enum_values, None, None, min_col_size_bytes)
            else:
                array_types = fetch_array_types_for_column(table_schemas, table_name, col)
                value = generate_random_data(data_type, table_name, None, array_types, None, min_col_size_bytes)
            if value is None:
                set_parts.append(f"{col} = NULL")
            else:
                set_parts.append(f"{col} = %s")
                params.append(value)

    set_clause = ", ".join(set_parts)
    return set_clause, params

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


# ----- Sampling helpers -----

DEFAULT_ROW_ESTIMATE = 1000

def build_sampling_condition(
    db_flavor: str,
    table_name: str,
    primary_key: str,
    target_row_count: int,
    estimated_row_count: Optional[int],
) -> Tuple[str, List[Any]]:
    """
    Build a WHERE condition fragment and parameters for sampling rows
    for UPDATE/DELETE operations.

    For PostgreSQL, this uses TABLESAMPLE SYSTEM_ROWS(target_row_count).
    For YugabyteDB, it uses a probabilistic filter WHERE random() < p,
    where p is derived from target_row_count and an estimated row count.
    """
    if db_flavor == "POSTGRES":
        where_clause = (
            f"{primary_key} IN ("
            f"SELECT {primary_key} FROM {table_name} TABLESAMPLE SYSTEM_ROWS(%s))"
        )
        return where_clause, [target_row_count]

    # YugabyteDB path: derive p from estimated row count
    est = estimated_row_count if estimated_row_count and estimated_row_count > 0 else DEFAULT_ROW_ESTIMATE

    # Derive sampling probability p from desired rows and estimated row count.
    p = min(1.0, float(target_row_count) / float(est))

    where_clause = (
        f"{primary_key} IN ("
        f"SELECT {primary_key} FROM {table_name} WHERE random() < %s)"
    )
    return where_clause, [p]
