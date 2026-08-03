import random
import string
from faker import Faker
import json
import ipaddress
import re
import decimal
import uuid
try:
    import psycopg2
except ImportError:  # pragma: no cover - guarded so utils.py (and the
    # pure logic it hosts: monotonic-PK formula, retry/backoff
    # classification, CLI parsing) imports and is unit-testable on a
    # machine without psycopg2 installed. Mirrors migration_monitor.py's
    # guard. Anything that actually needs a live connection (run_index_operations,
    # execute_with_retry's psycopg2-typed error paths) still requires
    # psycopg2 to be installed at call time.
    psycopg2 = None
import time
import argparse
from typing import Any, Callable, Dict, List, Optional, Tuple
import os
import threading
import sys
try:
    import yaml  # type: ignore
except Exception:
    yaml = None  # Defer strict error to loader to produce a clearer message
from rate_governor import RateGovernor, NullGovernor

# Module-level Faker instance for reuse; can be overridden via function parameter
_fake = Faker()

# ----- SQL identifier quoting -----

def quote_ident(name: Any) -> str:
    """Double-quote a SQL identifier (table/column name) for safe embedding
    in a raw SQL string, escaping any embedded double quote by doubling it
    (standard SQL identifier-quoting rule). Use this ONLY at the point a
    name is written into a SQL string -- never on a name still used as a
    Python dict key or passed into a value/column builder, since that would
    make it stop matching table_schemas' unquoted keys.

    Fixes queries against a column/table named with a reserved word (e.g.
    'primary') or containing special characters, which otherwise fail with
    a syntax error when embedded unquoted.
    """
    return '"' + str(name).replace('"', '""') + '"'


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
        "min_col_size_bytes": int,
        "enable_index_create_drop": bool,
        "index_events_interval": int,
        "column_overrides": dict,
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
    # Optional fields that don't need to be present (for backward compatibility)
    optional_fields = {"enable_index_create_drop","index_events_interval","column_overrides","min_col_size_bytes"}
    
    for key, expected_type in schema.items():
        if key not in section:
            if key not in optional_fields:
                raise ValueError(f"Missing key '{key}' in '{section_name}' section")
            continue  # Skip validation for optional fields that are missing
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

    rate_control = config.get("generator", {}).get("rate_control")
    if rate_control is not None:
        validate_rate_control(rate_control)

    transaction_mode = config.get("generator", {}).get("transaction_mode")
    if transaction_mode is not None:
        validate_transaction_mode(transaction_mode)

    return config


# ---------------------
# Rate governor config
# ---------------------

# Known keys for the optional rate_control block and its schedule entries.
# Anything else is a likely typo: warn but don't fail (non-fatal, per spec).
_RATE_CONTROL_KEYS = {"default_events_per_second", "report_interval_seconds", "schedule"}
_RATE_SCHEDULE_ENTRY_KEYS = {
    "events_per_second",
    "duration_seconds",
    "every_seconds",
    "offset_seconds",
    "jitter_pct",
}


def validate_rate_control(rc: Dict[str, Any]) -> None:
    """
    Validate a 'generator.rate_control' block.

    Raises ValueError with a clear message on any violation:
      - default_events_per_second must be present and > 0.
      - Each schedule entry: events_per_second > 0, duration_seconds > 0,
        every_seconds > 0, offset_seconds >= 0, 0 <= jitter_pct <= 50.
      - offset_seconds + duration_seconds <= every_seconds (the spike window
        must fit inside its period).
    Unknown keys print a non-fatal warning (typo guard) rather than raising.
    """
    if not isinstance(rc, dict):
        raise ValueError("rate_control must be a mapping/object")

    for key in rc:
        if key not in _RATE_CONTROL_KEYS:
            print(f"Warning: unknown key '{key}' in 'rate_control' (ignored)")

    if "default_events_per_second" not in rc:
        raise ValueError("rate_control.default_events_per_second is required when 'rate_control' is present")
    default_eps = rc["default_events_per_second"]
    if isinstance(default_eps, bool) or not isinstance(default_eps, (int, float)) or default_eps <= 0:
        raise ValueError(f"rate_control.default_events_per_second must be > 0, got {default_eps!r}")

    report_interval = rc.get("report_interval_seconds", 0)
    if isinstance(report_interval, bool) or not isinstance(report_interval, (int, float)) or report_interval < 0:
        raise ValueError(f"rate_control.report_interval_seconds must be >= 0, got {report_interval!r}")

    schedule = rc.get("schedule")
    if schedule is None:
        schedule = []
    if not isinstance(schedule, list):
        raise ValueError("rate_control.schedule must be a list")

    for idx, entry in enumerate(schedule):
        if not isinstance(entry, dict):
            raise ValueError(f"rate_control.schedule[{idx}] must be a mapping")

        for key in entry:
            if key not in _RATE_SCHEDULE_ENTRY_KEYS:
                print(f"Warning: unknown key '{key}' in 'rate_control.schedule[{idx}]' (ignored)")

        for required_key in ("events_per_second", "duration_seconds", "every_seconds"):
            if required_key not in entry:
                raise ValueError(f"rate_control.schedule[{idx}].{required_key} is required")

        events_per_second = entry["events_per_second"]
        if isinstance(events_per_second, bool) or not isinstance(events_per_second, (int, float)) or events_per_second <= 0:
            raise ValueError(f"rate_control.schedule[{idx}].events_per_second must be > 0, got {events_per_second!r}")

        duration_seconds = entry["duration_seconds"]
        if isinstance(duration_seconds, bool) or not isinstance(duration_seconds, (int, float)) or duration_seconds <= 0:
            raise ValueError(f"rate_control.schedule[{idx}].duration_seconds must be > 0, got {duration_seconds!r}")

        every_seconds = entry["every_seconds"]
        if isinstance(every_seconds, bool) or not isinstance(every_seconds, (int, float)) or every_seconds <= 0:
            raise ValueError(f"rate_control.schedule[{idx}].every_seconds must be > 0, got {every_seconds!r}")

        offset_seconds = entry.get("offset_seconds", 0)
        if isinstance(offset_seconds, bool) or not isinstance(offset_seconds, (int, float)) or offset_seconds < 0:
            raise ValueError(f"rate_control.schedule[{idx}].offset_seconds must be >= 0, got {offset_seconds!r}")

        jitter_pct = entry.get("jitter_pct", 0)
        if isinstance(jitter_pct, bool) or not isinstance(jitter_pct, (int, float)) or not (0 <= jitter_pct <= 50):
            raise ValueError(f"rate_control.schedule[{idx}].jitter_pct must be between 0 and 50, got {jitter_pct!r}")

        if offset_seconds + duration_seconds > every_seconds:
            raise ValueError(
                f"rate_control.schedule[{idx}]: offset_seconds + duration_seconds "
                f"({offset_seconds + duration_seconds}) must be <= every_seconds ({every_seconds})"
            )



# ---------------------
# Transaction mode config (optional, default OFF)
# ---------------------
#
# Known keys for the optional 'generator.transaction_mode' block (see
# transaction_mode.py for the planning/execution logic it feeds). Anything
# else is a likely typo: warn but don't fail, same policy as rate_control.
_TRANSACTION_MODE_KEYS = {
    "enabled",
    "hot_tables",
    "statements_per_txn",
    "hot_statements_per_txn",
    "other_statements_per_txn",
    "savepoint_pairs_per_txn",
    "hot_op_weights",
    "other_op_weights",
    "rows_per_statement",
}
_TRANSACTION_MODE_RANGE_KEYS = (
    "statements_per_txn",
    "hot_statements_per_txn",
    "other_statements_per_txn",
    "savepoint_pairs_per_txn",
)


def validate_transaction_mode(tm: Dict[str, Any]) -> None:
    """
    Validate a 'generator.transaction_mode' block.

    Lives in utils.py (rather than transaction_mode.py, which needs to
    import build_insert_values/build_update_values/etc. from here) so
    load_event_generator_config can call it directly with no import cycle.

    Only enforces the full shape when 'enabled' is true (fails fast at
    startup with a clear message): a minimal or stale block with
    enabled: false (or the key simply absent) is a no-op beyond a type
    check on 'enabled' itself -- the legacy single-op path never reads any
    other key in this block, so it must never be broken by one.

      - hot_tables must be a non-empty list of table name strings.
      - statements_per_txn / hot_statements_per_txn /
        other_statements_per_txn / savepoint_pairs_per_txn must each be a
        mapping with integer 'min'/'max', 0 <= min <= max.
      - hot_op_weights / other_op_weights must be mappings with numeric,
        non-negative INSERT/UPDATE/DELETE weights, at least one > 0.
      - rows_per_statement (OPTIONAL, UPDATE/DELETE only -- INSERT stays
        single-row) must, when present, be a mapping with integer
        'min'/'max', 1 <= min <= max. Absent key defaults to {min: 1, max: 1}
        -- today's single-row-per-statement behavior, byte-for-byte.
    Unknown keys print a non-fatal warning (typo guard) rather than raising.
    """
    if not isinstance(tm, dict):
        raise ValueError("transaction_mode must be a mapping/object")

    for key in tm:
        if key not in _TRANSACTION_MODE_KEYS:
            print(f"Warning: unknown key '{key}' in 'transaction_mode' (ignored)")

    enabled = tm.get("enabled", False)
    if not isinstance(enabled, bool):
        raise ValueError(f"transaction_mode.enabled must be a bool, got {enabled!r}")
    if not enabled:
        return

    hot_tables = tm.get("hot_tables")
    if (
        not isinstance(hot_tables, list)
        or not hot_tables
        or not all(isinstance(t, str) for t in hot_tables)
    ):
        raise ValueError(
            "transaction_mode.hot_tables must be a non-empty list of table names when enabled"
        )

    for key in _TRANSACTION_MODE_RANGE_KEYS:
        _validate_transaction_mode_range(tm, key)

    for key in ("hot_op_weights", "other_op_weights"):
        _validate_transaction_mode_op_weights(tm, key)

    _validate_transaction_mode_rows_per_statement(tm)


def _validate_transaction_mode_range(tm: Dict[str, Any], key: str) -> None:
    rng = tm.get(key)
    if not isinstance(rng, dict) or "min" not in rng or "max" not in rng:
        raise ValueError(f"transaction_mode.{key} is required (with 'min'/'max') when enabled")
    lo, hi = rng["min"], rng["max"]
    if isinstance(lo, bool) or isinstance(hi, bool) or not isinstance(lo, int) or not isinstance(hi, int):
        raise ValueError(f"transaction_mode.{key}.min/max must be integers, got min={lo!r}, max={hi!r}")
    if lo < 0 or hi < lo:
        raise ValueError(f"transaction_mode.{key} must satisfy 0 <= min <= max, got min={lo}, max={hi}")


def _validate_transaction_mode_op_weights(tm: Dict[str, Any], key: str) -> None:
    weights = tm.get(key)
    if not isinstance(weights, dict):
        raise ValueError(f"transaction_mode.{key} is required (a mapping) when enabled")
    total = 0.0
    for op in ("INSERT", "UPDATE", "DELETE"):
        w = weights.get(op, 0)
        if isinstance(w, bool) or not isinstance(w, (int, float)) or w < 0:
            raise ValueError(f"transaction_mode.{key}.{op} must be a number >= 0, got {w!r}")
        total += w
    if total <= 0:
        raise ValueError(f"transaction_mode.{key} must have at least one operation with weight > 0")


def _validate_transaction_mode_rows_per_statement(tm: Dict[str, Any]) -> None:
    """Validate the OPTIONAL 'rows_per_statement' key: a {min, max} range
    bounding how many rows a single UPDATE/DELETE statement affects (INSERT
    is never affected -- it stays single-row unconditionally).

    Absent key is valid -- callers treat it as {min: 1, max: 1}, i.e. today's
    single-row-per-statement behavior, unchanged. When present, same
    {min,max} shape as the other ranges above, except min must be >= 1 (a
    statement targeting 0 rows makes no sense here, unlike e.g.
    savepoint_pairs_per_txn where 0 is a valid pair count).
    """
    if "rows_per_statement" not in tm:
        return
    rng = tm["rows_per_statement"]
    if not isinstance(rng, dict) or "min" not in rng or "max" not in rng:
        raise ValueError(
            "transaction_mode.rows_per_statement must be a mapping with 'min'/'max' when present"
        )
    lo, hi = rng["min"], rng["max"]
    if isinstance(lo, bool) or isinstance(hi, bool) or not isinstance(lo, int) or not isinstance(hi, int):
        raise ValueError(
            f"transaction_mode.rows_per_statement.min/max must be integers, got min={lo!r}, max={hi!r}"
        )
    if lo < 1 or hi < lo:
        raise ValueError(
            f"transaction_mode.rows_per_statement must satisfy 1 <= min <= max, got min={lo}, max={hi}"
        )


def build_rate_governor(config: Dict[str, Any], **injectables) -> Any:
    """
    Build the rate governor used to pace the generator's event loop.

    Returns a NullGovernor() (no pacing; today's unpaced behavior) when
    'generator.rate_control' is absent. Otherwise returns a RateGovernor
    configured from the rate_control block, seeded from 'generator.random_seed'
    (falling back to 'generator.seed') for deterministic jitter.
    """
    gen = config["generator"]
    rate_control = gen.get("rate_control")
    if not rate_control:
        return NullGovernor()
    random_seed = gen.get("random_seed", gen.get("seed"))
    return RateGovernor(rate_control, random_seed=random_seed, **injectables)


def build_worker_governor(
    config: Dict[str, Any],
    throttle: float = 0.0,
    worker_uid: Optional[int] = None,
    **injectables,
) -> Any:
    """
    Build the rate governor for a worker process launched via the dynamic
    worker pool's CLI (see IMPLEMENTATION_CONTRACTS.md "Worker CLI").

    - throttle > 0: engage a flat, single-worker cap of `throttle` events/sec
      -- the reactive controller's "trimmer" role (worker-count modulation
      does the coarse rate shaping; this is the only sleeping that
      survives). This takes priority over any configured
      'generator.rate_control', which the new controller model supersedes
      for CLI-launched workers.
    - throttle <= 0 (default/absent): legacy behavior -- delegates to
      build_rate_governor(config), so an uncapped worker with no
      'generator.rate_control' configured gets a NullGovernor (today's
      unpaced default), while one with 'generator.rate_control' configured
      keeps using it unchanged.
    """
    if throttle and throttle > 0:
        rate_control = {"default_events_per_second": float(throttle)}
        return RateGovernor(rate_control, random_seed=worker_uid, **injectables)
    return build_rate_governor(config, **injectables)


def build_worker_arg_parser() -> "argparse.ArgumentParser":
    """
    Build generator.py's CLI argument parser.

    Lives in utils.py (psycopg2-optional, guarded above) rather than
    generator.py so it -- along with the other pure logic below
    (compute_monotonic_pk, classify_retry, compute_backoff_delay) -- stays
    importable and unit-testable without psycopg2 or a DB connection.
    generator.py itself always requires a live psycopg2 connection to run,
    but never needs to be imported by a test for these pieces to be
    covered.

    All worker-pool args are optional; when absent, generator.py reproduces
    today's legacy single-process, self-seeding, unpaced (or
    rate_control-paced) behavior. See IMPLEMENTATION_CONTRACTS.md
    "Worker CLI".
    """
    parser = argparse.ArgumentParser(description="Event Generator for PostgreSQL")
    parser.add_argument(
        "-c",
        "--config",
        default=None,
        help="Path to event-generator YAML config (defaults to event-generator.yaml in this folder)",
    )
    parser.add_argument(
        "--cache-dir",
        default=None,
        help="Shared cache directory (schema + PK snapshot) built by the controller "
        "(shared_cache.build_cache). When given, schema + PK pools load from the cache "
        "instead of per-table catalog/seed queries. Absent => legacy self-seeding.",
    )
    parser.add_argument(
        "--cache-version",
        default=None,
        help="Cache version to load (see shared_cache.py). Defaults to the cache's current version.",
    )
    parser.add_argument(
        "--worker-uid",
        type=int,
        default=None,
        help="Unique worker id, monotonic and never reused across a run. Drives the "
        "monotonic PK stride formula and the RNG/faker seed offset (base_seed + worker_uid). "
        "Absent => legacy random PK generation.",
    )
    parser.add_argument(
        "--pk-stride",
        type=int,
        default=100000,
        help="Stride between worker_uids in the monotonic PK formula (default: %(default)s). "
        "Must exceed the total number of workers ever spawned across the whole run.",
    )
    parser.add_argument(
        "--throttle",
        type=float,
        default=0.0,
        help="Single-worker events/sec cap (the reactive controller's 'trimmer' role). "
        "0 or absent => uncapped; rate_governor is not engaged for this cap "
        "(a configured generator.rate_control, if any, still applies).",
    )
    parser.add_argument(
        "--control-file",
        default=None,
        help="Path to a control file the cascade trimmer controller periodically rewrites "
        "with a single float events/sec rate. Only the persistent 'trimmer' worker gets "
        "this flag; when set, the worker loop re-reads it (at most once per ~1s) and calls "
        "rate_governor.set_rate on any change -- see "
        "ARCHITECTURE.md. "
        "Absent (default) => no runtime rate re-reading (uncapped workers never get this).",
    )
    parser.add_argument(
        "--run-id",
        default="",
        help="Per-run token (short string, timestamp-derived, not persisted) folded into "
        "the text/uuid unique-value encodings (see compute_unique_safe_value) so a re-run "
        "-- whose worker_uid/counter both restart at 0 -- can't regenerate a value that "
        "collides with a previous run's rows. Absent/empty (default) => legacy encoding, "
        "unchanged. Never applied to the integer branch (worker_uid feeds an arithmetic "
        "formula there; a run_id would risk overflowing narrow int columns).",
    )
    return parser


def parse_worker_args(argv: Optional[List[str]] = None) -> "argparse.Namespace":
    """Parse generator.py's CLI args. See build_worker_arg_parser."""
    return build_worker_arg_parser().parse_args(argv)


def get_connection_kwargs_from_config(config: Dict[str, Any]) -> Dict[str, Any]:
    """
    Return kwargs to pass to psycopg2.connect, strictly from config.

    Connections go to exactly the configured host. Spreading load across a
    multi-node cluster is done by the controller assigning each worker a
    specific node (round-robin over yb_servers()) and overriding host/port in
    that worker's config -- NOT by a driver-level load balancer, which cannot
    help a fleet of single-connection worker processes (each process has its
    own empty balancer and they all pick the same node). See the controller's
    node-distribution logic.
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


# ----- Monotonic PK generation (dynamic worker pool: --worker-uid) -----

# Data types the monotonic PK formula can drive (matches
# shared_cache._is_integer_type -- duplicated here rather than imported so
# utils.py has no dependency on shared_cache.py).
_INTEGER_PK_DATA_TYPES = frozenset(
    [
        "smallint",
        "integer",
        "bigint",
        "int",
        "int2",
        "int4",
        "int8",
        "smallserial",
        "serial",
        "bigserial",
    ]
)


def is_integer_pk_type(data_type: Optional[str]) -> bool:
    """Return True if `data_type` is a single-column integer PK type the
    monotonic PK formula can drive (see compute_monotonic_pk). Composite
    keys, and non-integer single-column keys (uuid/text/...), fall back to
    legacy random PK generation for that table -- there is no usable
    integer ceiling to build a monotonic stride from.
    """
    if not data_type:
        return False
    return data_type.strip().lower() in _INTEGER_PK_DATA_TYPES


def get_table_max_pk(
    cursor: Any,
    schema_name: Optional[str],
    table_name: str,
    pk_col: str,
) -> Optional[int]:
    """Return MAX(pk_col) for table_name, or None if the table is empty.

    Used to seed monotonic PK generation (--worker-uid) in legacy/no-cache
    mode, where shared_cache.table_max_pk() isn't available (that path is
    the controller-built cache's job; this is the direct-query fallback for
    a standalone worker with no --cache-dir).
    """
    qualified_table = f"{quote_ident(schema_name)}.{quote_ident(table_name)}" if schema_name else quote_ident(table_name)
    cursor.execute(f"SELECT MAX({quote_ident(pk_col)}) FROM {qualified_table}")
    row = cursor.fetchone()
    if not row or row[0] is None:
        return None
    return int(row[0])


def compute_monotonic_pk(max_pk: int, worker_uid: int, pk_stride: int, counter: int) -> int:
    """Compute the deterministic INSERT primary-key value for a worker.

    pk = max_pk + 1 + worker_uid + pk_stride * counter

    where `counter` increments once per row this worker has inserted into
    this table (starting at 0). Disjoint across worker_uids (each spawned
    worker's range never overlaps another's, as long as worker_uid <
    pk_stride for every worker -- the controller's global monotonic
    worker_uid allocator plus a sufficiently large pk_stride guarantee
    this), strictly increasing per worker, and always above the table's
    recorded max(pk) at seed time. See IMPLEMENTATION_CONTRACTS.md
    "Monotonic PK generation".
    """
    return max_pk + 1 + worker_uid + pk_stride * counter


# ----- Unique-safe value generation (any single-column unique surface) -----
#
# Today only single-column integer PKs get collision-free values (via
# compute_monotonic_pk above); every other unique surface -- secondary
# UNIQUE constraints/indexes, and non-integer PKs (varchar/uuid/...) --
# falls back to plain random generation and collides heavily as tables
# fill. compute_unique_safe_value generalizes the same idea (namespace by
# worker_uid, advance a strictly-increasing per-(table,column) counter) to
# every type generate_table_schemas_bulk's new "unique_columns" can name.

# Per-width integer ceilings. compute_unique_safe_value returns None (caller
# falls back to random generation) once the monotonic value would exceed the
# column's own type max -- otherwise a plain `integer` column overflows int32
# (SQLSTATE 22003 "value out of range") long before bigint's ceiling, which
# hard-fails the whole INSERT batch instead of just retrying.
_INT2_MAX = 32767
_INT4_MAX = 2147483647
_INT8_MAX = 9223372036854775807
_UNIQUE_SAFE_INT_OVERFLOW_THRESHOLD = _INT8_MAX  # bigint/numeric ceiling


def _integer_type_ceiling(dtype: str) -> int:
    """Max value the monotonic scheme may emit for an integer/numeric column
    of SQL type `dtype` (already lowercased). Narrow ints get their real
    ceiling so we fall back to random before overflowing the column."""
    if dtype in ("smallint", "int2", "smallserial"):
        return _INT2_MAX
    if dtype in ("integer", "int", "int4", "serial"):
        return _INT4_MAX
    return _INT8_MAX  # bigint/int8/bigserial, numeric/decimal

_BASE36_ALPHABET = "0123456789abcdefghijklmnopqrstuvwxyz"

# Stride used only for the compact base36 text-encoding fallback below --
# deliberately distinct from pk_stride (which governs the integer monotonic
# formula this function also drives). Large enough that no single worker
# will ever generate anywhere near this many values for one text column, so
# each worker_uid's segment of the encoded space never collides with
# another worker's.
_TEXT_UNIQUE_STRIDE = 10 ** 9


def _to_base36(n: int) -> str:
    """Encode a non-negative int as a lowercase base36 string (no leading
    zero-padding), for the compact unique text-column encoding below."""
    if n == 0:
        return "0"
    digits = []
    while n > 0:
        n, rem = divmod(n, 36)
        digits.append(_BASE36_ALPHABET[rem])
    return "".join(reversed(digits))


def compute_unique_safe_value(
    data_type: Optional[str],
    worker_uid: int,
    pk_stride: int,
    counter: int,
    *,
    max_seed: int = 0,
    char_max: Optional[int] = None,
    run_id: str = "",
) -> Any:
    """Compute a guaranteed-unique value for one single-column unique
    surface (PK, UNIQUE constraint, or standalone unique index), for this
    worker/counter. Returns None when `data_type` isn't one this function
    can drive deterministically, or the deterministic value it would
    produce is unusable (bigint overflow; a compact text encoding that
    still doesn't fit `char_max`) -- callers fall back to the normal
    type-aware random-generation path for that column, unchanged (the
    existing unique-violation retry safety net still covers it).

    - integer family / numeric: `max_seed + 1 + worker_uid + pk_stride *
      counter` -- same shape as compute_monotonic_pk, but seeded from a
      plain `MAX(col)` (see generate_table_schemas_bulk's
      "unique_max_seeds") instead of a primary key's max. None once the value
      would exceed the column type's own max (int2/int4/int8), so a narrow
      integer column falls back to random rather than overflowing. `run_id`
      is ignored here -- worker_uid feeds this arithmetic formula directly,
      and folding in a run-scoped token would risk overflowing narrow int
      columns. This branch is already re-run-safe: it seeds from MAX(col),
      which only grows across runs.
    - text/varchar/char: `f"u{worker_uid}_{counter}"` -- namespaced by
      worker, strictly increasing per worker, so it never repeats across
      any (worker_uid, counter) pair. If `char_max` is given and that
      string is too long, falls back to a compact base36 encoding of
      `worker_uid * _TEXT_UNIQUE_STRIDE + counter` (still unique for the
      same reason: disjoint worker_uid segments, strictly-increasing
      counter within each). None if even that can't fit `char_max`. When
      `run_id` is truthy it's folded into both forms (`f"u{run_id}_{worker_uid}_{counter}"`
      / `"u" + run_id + _to_base36(...)`) so a re-run (which restarts
      worker_uid/counter from scratch) can't regenerate a value that
      collides with a previous run's rows. Falsy `run_id` (default) is
      byte-identical to the legacy encoding.
    - uuid: `uuid.uuid5(uuid.NAMESPACE_OID, f"{worker_uid}:{counter}")` --
      deterministic and unique for the same (worker_uid, counter)
      uniqueness argument. When `run_id` is truthy, it's folded into the
      uuid5 name (`f"{run_id}:{worker_uid}:{counter}"`) for the same
      cross-run anti-collision reason as the text branch; falsy `run_id`
      (default) is byte-identical to the legacy encoding.
    - anything else: None.
    """
    if not data_type:
        return None
    dtype = data_type.strip().lower()

    if is_integer_pk_type(dtype) or dtype.startswith("numeric") or dtype.startswith("decimal"):
        value = max_seed + 1 + worker_uid + pk_stride * counter
        # Fall back to random once we'd exceed the column type's own max, not
        # just bigint's -- an int4/int2 column overflows far sooner.
        if value > _integer_type_ceiling(dtype):
            return None
        return value

    if "uuid" in dtype:
        if run_id:
            return uuid.uuid5(uuid.NAMESPACE_OID, f"{run_id}:{worker_uid}:{counter}")
        return uuid.uuid5(uuid.NAMESPACE_OID, f"{worker_uid}:{counter}")

    if any(token in dtype for token in ("varchar", "character varying", "text", "char", "character")):
        full = f"u{run_id}_{worker_uid}_{counter}" if run_id else f"u{worker_uid}_{counter}"
        if char_max is None or len(full) <= char_max:
            return full
        compact = (
            "u" + run_id + _to_base36(worker_uid * _TEXT_UNIQUE_STRIDE + counter)
            if run_id
            else "u" + _to_base36(worker_uid * _TEXT_UNIQUE_STRIDE + counter)
        )
        if len(compact) <= char_max:
            return compact
        return None

    return None


# ----- Column override helpers -----

def generate_override_value(override_spec: Dict[str, Any]) -> Any:
    """Generate a value based on a column_overrides spec entry.

    Supported override types:
      - choice: pick randomly from a list of values
      - timestamp_range: random timestamp between min and max (ISO format strings)
      - date_range: random date between min and max (YYYY-MM-DD strings)
      - int_range: random integer between min and max
    """
    from datetime import datetime, timedelta, date as date_type

    override_type = override_spec.get("type")

    if override_type == "choice":
        return random.choice(override_spec["values"])

    elif override_type == "timestamp_range":
        min_ts = datetime.fromisoformat(override_spec["min"])
        max_ts = datetime.fromisoformat(override_spec["max"])
        delta = (max_ts - min_ts).total_seconds()
        random_seconds = random.uniform(0, delta)
        result = min_ts + timedelta(seconds=random_seconds)
        return result.strftime("%Y-%m-%d %H:%M:%S")

    elif override_type == "date_range":
        min_d = date_type.fromisoformat(override_spec["min"])
        max_d = date_type.fromisoformat(override_spec["max"])
        delta_days = (max_d - min_d).days
        random_days = random.randint(0, delta_days)
        result = min_d + timedelta(days=random_days)
        return result.isoformat()

    elif override_type == "int_range":
        return random.randint(override_spec["min"], override_spec["max"])

    else:
        raise ValueError(f"Unknown column_overrides type: {override_type}")


def get_column_override(
    column_overrides: Dict[str, Any],
    table_name: str,
    column_name: str,
) -> Optional[Dict[str, Any]]:
    """Look up an override spec for a given table.column, or None."""
    table_overrides = column_overrides.get(table_name)
    if not table_overrides:
        return None
    return table_overrides.get(column_name)


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
        elif data_type == 'character varying' or data_type == 'varchar' or data_type == 'char' or data_type == 'character':
            where_prefix, where_params = _schema_filter(schema_name)
            cursor.execute(
                f"""
                SELECT character_maximum_length
                FROM information_schema.columns
                WHERE {where_prefix} column_name = %s AND table_name = %s
                """,
                where_params + (column_name, table_name),
            )
            character_maximum_length = cursor.fetchone()[0]
            if character_maximum_length is not None and character_maximum_length > 0:
                column_info[i] = (column_name, f"{data_type}({character_maximum_length})")
    return column_info

def _build_columns_dict(column_info: List[Tuple[str, str]]) -> Dict[str, str]:
    """Return a mapping of column_name -> data_type from information_schema results."""
    return {column_name: data_type for column_name, data_type in column_info}


def _is_hstore_udt(udt_name: Optional[str]) -> bool:
    """True if `udt_name` (information_schema.columns.udt_name, or a
    udt_name::regtype::text) names the hstore extension type. Handles a
    possible schema-qualified/quoted form (e.g. 'public.hstore') by
    checking the unqualified suffix rather than requiring an exact match.
    """
    if not udt_name:
        return False
    unqualified = udt_name.strip().strip('"').lower().rsplit(".", 1)[-1]
    return unqualified == "hstore"


def _resolve_hstore_columns(
    cursor: Any,
    table_name: str,
    schema_name: Optional[str],
    columns: Dict[str, str],
) -> None:
    """Relabel USER-DEFINED columns backed by the hstore extension type from
    'USER-DEFINED' to the literal type string 'hstore' in `columns` (in
    place), so generate_random_data can synthesize a real hstore literal
    for them instead of falling into the generic "unknown USER-DEFINED
    type" NULL/omit path. hstore is reported by information_schema.columns
    as data_type='USER-DEFINED' with udt_name='hstore' -- indistinguishable
    from any other extension scalar type (or a genuinely unresolvable one)
    without this extra per-column udt_name lookup. Per-table path
    (convert_pg_table_description) only -- generate_table_schemas_bulk does
    the equivalent inline using its already-fetched udt_regtype map, no
    extra queries needed there.
    """
    where_prefix, where_params = _schema_filter(schema_name)
    for column_name, data_type in list(columns.items()):
        if data_type != "USER-DEFINED":
            continue
        cursor.execute(
            f"""
            SELECT udt_name
            FROM information_schema.columns
            WHERE {where_prefix} table_name = %s AND column_name = %s
            """,
            where_params + (table_name, column_name),
        )
        row = cursor.fetchone()
        if row and _is_hstore_udt(row[0]):
            columns[column_name] = "hstore"


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
) -> Optional[List[str]]:
    """Return the primary key column(s) as a list, or None if not found.

    For partitioned tables where the root has no PK, falls back to querying
    the first child partition's PK.
    """
    regclass = _qualify_regclass(table_name, schema_name)
    pk_query = """
        SELECT a.attname
        FROM pg_index i
        JOIN pg_class c ON c.oid = i.indrelid
        JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = ANY(i.indkey)
        WHERE c.oid = %s::regclass
          AND i.indisprimary
        ORDER BY a.attnum
    """
    cursor.execute(pk_query, (regclass,))
    rows = cursor.fetchall()
    if rows:
        return [r[0] for r in rows]

    # Root partitioned tables often have no PK; walk down through children
    # until we find a leaf partition that has a PK (handles multilevel partitioning).
    children_query = """
        SELECT c.relname
        FROM pg_inherits i
        JOIN pg_class c ON c.oid = i.inhrelid
        JOIN pg_class p ON p.oid = i.inhparent
        JOIN pg_namespace n ON n.oid = p.relnamespace
        WHERE p.relname = %s AND n.nspname = %s
        ORDER BY c.relname
        LIMIT 1
    """
    current = table_name
    visited = set()
    while current not in visited:
        visited.add(current)
        cursor.execute(children_query, (current, schema_name or 'public'))
        child = cursor.fetchone()
        if not child:
            break
        child_regclass = _qualify_regclass(child[0], schema_name)
        cursor.execute(pk_query, (child_regclass,))
        rows = cursor.fetchall()
        if rows:
            pk_cols = [r[0] for r in rows]
            print(f"PK for '{table_name}' resolved from child '{child[0]}': {pk_cols}")
            return pk_cols
        current = child[0]
    return None


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


def _find_unique_columns(
    cursor: Any,
    table_name: str,
    schema_name: Optional[str],
    columns: Dict[str, str],
) -> List[str]:
    """Return one representative column per unique surface (PK, UNIQUE
    constraint, standalone unique index, OR partial unique index) on
    `table_name` -- every single-column one, plus, for each multi-column
    one, an integer-typed column if present, else the lowest-attnum column
    (making one component of a unique tuple unique makes the whole tuple
    unique). Per-table mirror of the schema-wide query in
    generate_table_schemas_bulk, for the per-table fallback path
    (generate_table_schemas / convert_pg_table_description).

    Expression-index members (attnum 0 in indkey) have no matching
    pg_attribute row and are silently dropped by the join. Partial unique
    indexes (e.g. `... WHERE col IS NOT NULL`) are INCLUDED: making the
    representative column globally unique makes the (partial) unique index
    unviolatable too, since global uniqueness implies uniqueness over any
    subset of rows -- so routing it through the same unique-value machinery
    is always safe, even though the index itself only constrains a subset.
    """
    regclass = _qualify_regclass(table_name, schema_name)
    cursor.execute(
        """
        SELECT i.indexrelid, a.attname
        FROM pg_index i
        JOIN pg_class c ON c.oid = i.indrelid
        JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = ANY(i.indkey)
        WHERE c.oid = %s::regclass
          AND i.indisunique
        ORDER BY i.indexrelid, a.attnum
        """,
        (regclass,),
    )
    idx_cols_by_index: Dict[Any, List[str]] = {}
    for indexrelid, attname in cursor.fetchall():
        idx_cols_by_index.setdefault(indexrelid, []).append(attname)

    seen = set()
    unique_columns: List[str] = []
    for idx_cols in idx_cols_by_index.values():
        if len(idx_cols) == 1:
            candidate = idx_cols[0]
        else:
            candidate = next(
                (c for c in idx_cols if is_integer_pk_type(columns.get(c))),
                idx_cols[0],
            )
        if candidate not in seen:
            seen.add(candidate)
            unique_columns.append(candidate)
    return unique_columns


def convert_pg_table_description(
    cursor: Any,
    column_info: List[Tuple[str, str]],
    table_name: str,
    schema_name: Optional[str] = None,
) -> Dict[str, Dict[str, Any]]:
    """Convert column info into a schema dict (columns, arrays, PK, enums, bit/varbit)."""
    columns = _build_columns_dict(column_info)
    _resolve_hstore_columns(cursor, table_name, schema_name, columns)
    bit_info = _build_bit_info(cursor, table_name, schema_name, columns)
    array_types = _build_array_types(cursor, schema_name, table_name, columns)
    primary_key = _find_primary_key(cursor, table_name, schema_name)
    enum_values = _build_enum_values(cursor, table_name, schema_name, column_info)
    unique_columns = _find_unique_columns(cursor, table_name, schema_name, columns)

    result = {
        "columns": columns,
        "array_types": array_types,
        "primary_key": primary_key,
        "enum_values": enum_values,
        "bit_info": bit_info,
        "unique_columns": unique_columns,
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


def generate_table_schemas_bulk(
    cursor: Any,
    schema_name: Optional[str] = None,
    manual_table_list: Optional[List[str]] = None,
    exclude_table_list: Optional[List[str]] = None,
) -> Dict[str, Dict[str, Any]]:
    """Same output as generate_table_schemas, but built with a handful of
    schema-wide catalog queries instead of per-table/per-column queries.

    The per-column information_schema queries in generate_table_schemas are
    fatally slow at hundreds-of-tables scale on YugabyteDB (each query pays the
    information_schema view-evaluation cost). This issues ~4 batched queries and
    groups the results in Python. Output shape is identical:
    {table: {columns, array_types, primary_key, enum_values, bit_info}}.
    """
    if manual_table_list:
        table_list = list(manual_table_list)
    else:
        table_list = get_table_list(cursor, schema_name, exclude_table_list)
    if not table_list:
        return {}
    table_set = set(table_list)
    nsp = schema_name or "public"
    where_prefix, where_params = _schema_filter(schema_name)

    # 1) all columns (+ char length, numeric precision/scale, udt regtype) in one query
    cols_by_table: Dict[str, List[Tuple[str, str]]] = {}
    udt_regtype: Dict[Tuple[str, str], Optional[str]] = {}
    cursor.execute(
        f"""
        SELECT table_name, column_name, data_type, character_maximum_length,
               numeric_precision, numeric_scale, udt_name::regtype::text
        FROM information_schema.columns
        WHERE {where_prefix} table_name = ANY(%s)
        ORDER BY table_name, ordinal_position
        """,
        where_params + (table_list,),
    )
    for tname, col, dtype, charmax, nprec, nscale, udt_rt in cursor.fetchall():
        if tname not in table_set:
            continue
        dstr = dtype
        if dtype in ("numeric", "decimal") and nprec is not None:
            dstr = f"{dtype}({nprec},{nscale or 0})"
        elif dtype in ("character varying", "varchar", "char", "character") and charmax:
            dstr = f"{dtype}({charmax})"
        cols_by_table.setdefault(tname, []).append((col, dstr))
        udt_regtype[(tname, col)] = udt_rt

    # 2) all primary keys in one query (attnum order, matching _find_primary_key)
    pk_by_table: Dict[str, List[str]] = {}
    cursor.execute(
        """
        SELECT c.relname, a.attname
        FROM pg_index i
        JOIN pg_class c ON c.oid = i.indrelid
        JOIN pg_namespace n ON n.oid = c.relnamespace
        JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = ANY(i.indkey)
        WHERE i.indisprimary AND n.nspname = %s AND c.relname = ANY(%s)
        ORDER BY c.relname, a.attnum
        """,
        (nsp, table_list),
    )
    for tname, attname in cursor.fetchall():
        pk_by_table.setdefault(tname, []).append(attname)

    # 3) all enum / array-of-enum labels in one query
    enum_by_table: Dict[str, Dict[str, List[str]]] = {}
    cursor.execute(
        """
        SELECT c.relname, a.attname, e.enumlabel
        FROM pg_attribute a
        JOIN pg_class c ON c.oid = a.attrelid
        JOIN pg_namespace n ON n.oid = c.relnamespace
        JOIN pg_type t ON t.oid = a.atttypid
        JOIN pg_enum e ON e.enumtypid = CASE WHEN t.typelem <> 0 THEN t.typelem ELSE t.oid END
        WHERE n.nspname = %s AND c.relname = ANY(%s) AND a.attnum > 0 AND NOT a.attisdropped
        ORDER BY c.relname, a.attnum, e.enumsortorder
        """,
        (nsp, table_list),
    )
    for tname, attname, label in cursor.fetchall():
        enum_by_table.setdefault(tname, {}).setdefault(attname, []).append(label)

    # 4) all bit/varbit metadata in one query
    bit_by_table: Dict[str, Dict[str, Dict[str, Any]]] = {}
    cursor.execute(
        """
        SELECT c.relname, a.attname, a.atttypmod, format_type(a.atttypid, NULL) AS ftype
        FROM pg_attribute a
        JOIN pg_class c ON c.oid = a.attrelid
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = %s AND c.relname = ANY(%s) AND a.attnum > 0 AND NOT a.attisdropped
          AND format_type(a.atttypid, NULL) IN ('bit', 'bit varying')
        """,
        (nsp, table_list),
    )
    for tname, attname, atttypmod, ftype in cursor.fetchall():
        length = atttypmod if (atttypmod is not None and atttypmod > 0) else None
        bit_by_table.setdefault(tname, {})[attname] = {
            "varying": ftype == "bit varying",
            "length": length,
        }

    # 5) every unique surface (PK, UNIQUE constraint, standalone unique
    # index, OR partial unique index) in one schema-wide query, resolved to
    # column names via pg_attribute. An expression-index member (attnum 0
    # in indkey) has no matching pg_attribute row and is silently dropped
    # by the join. Partial unique indexes (indpred IS NOT NULL) are
    # INCLUDED: making the representative column globally unique makes the
    # (partial) unique index unviolatable too, since global uniqueness
    # implies uniqueness over any subset of rows. Rows are grouped below by
    # (table, indexrelid) so a multi-column unique index contributes
    # exactly ONE representative column (see Part 1 of the
    # unique-safe-value-generation design: making one component of a
    # unique tuple unique makes inserts collision-free for the whole row).
    unique_idx_cols: Dict[str, Dict[Any, List[str]]] = {}
    cursor.execute(
        """
        SELECT c.relname, i.indexrelid, a.attname
        FROM pg_index i
        JOIN pg_class c ON c.oid = i.indrelid
        JOIN pg_namespace n ON n.oid = c.relnamespace
        JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = ANY(i.indkey)
        WHERE i.indisunique
          AND n.nspname = %s AND c.relname = ANY(%s)
        ORDER BY c.relname, i.indexrelid, a.attnum
        """,
        (nsp, table_list),
    )
    for tname, indexrelid, attname in cursor.fetchall():
        unique_idx_cols.setdefault(tname, {}).setdefault(indexrelid, []).append(attname)

    table_schemas: Dict[str, Dict[str, Any]] = {}
    for tname in table_list:
        col_info = cols_by_table.get(tname)
        if not col_info:
            print(f"Table '{tname}' not found.")
            continue
        columns = {c: d for c, d in col_info}
        # Relabel USER-DEFINED hstore columns to the literal type string
        # 'hstore' (see _resolve_hstore_columns / _is_hstore_udt -- same
        # detection, reusing udt_regtype instead of an extra per-column
        # query since it's already fetched for every column above).
        for c, d in col_info:
            if d == "USER-DEFINED" and _is_hstore_udt(udt_regtype.get((tname, c))):
                columns[c] = "hstore"
        array_types: Dict[str, str] = {}
        for c, d in col_info:
            if "ARRAY" in d.upper():
                rt = udt_regtype.get((tname, c))
                array_types[c] = rt if rt else d
        primary_key = pk_by_table.get(tname)
        if primary_key is None:
            # rare: partitioned root with PK only on children -> per-table fallback
            primary_key = _find_primary_key(cursor, tname, schema_name)

        unique_columns: List[str] = []
        seen_unique_cols = set()
        for idx_cols in unique_idx_cols.get(tname, {}).values():
            if len(idx_cols) == 1:
                candidate = idx_cols[0]
            else:
                # multi-column unique index: prefer an integer-typed
                # column, else the lowest-attnum (first) column
                candidate = next(
                    (c for c in idx_cols if is_integer_pk_type(columns.get(c))),
                    idx_cols[0],
                )
            if candidate not in seen_unique_cols:
                seen_unique_cols.add(candidate)
                unique_columns.append(candidate)

        # unique_max_seeds: for orderable (integer-family/numeric) unique
        # columns only -- generated values there must exceed the current
        # max already in the table. Text/uuid/other unique columns need no
        # seed (they're namespaced by worker_uid+counter instead) and are
        # omitted. These columns are indexed, so MAX is a cheap index scan.
        unique_max_seeds: Dict[str, int] = {}
        for col in unique_columns:
            col_dtype = columns.get(col)
            is_orderable = is_integer_pk_type(col_dtype) or (
                (col_dtype or "").strip().lower().startswith(("numeric", "decimal"))
            )
            if not is_orderable:
                continue
            qualified = _qualify_regclass(tname, schema_name)
            cursor.execute(f"SELECT MAX({quote_ident(col)}) FROM {qualified}")
            row = cursor.fetchone()
            unique_max_seeds[col] = int(row[0]) if row and row[0] is not None else 0

        table_schemas[tname] = {
            "columns": columns,
            "array_types": array_types,
            "primary_key": primary_key,
            "enum_values": enum_by_table.get(tname, {}),
            "bit_info": bit_by_table.get(tname, {}),
            "unique_columns": unique_columns,
            "unique_max_seeds": unique_max_seeds,
        }
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

# ----- Index events helpers -----

def run_index_operations(stop_index_thread: threading.Event, config: Dict[str, Any], schema_name: str, table_schemas: Dict[str, Dict[str, Any]], index_events_interval: int):
    """Run index create/drop operations in a separate thread with its own connection."""
    # Create a separate connection for index operations
    index_conn = psycopg2.connect(**get_connection_kwargs_from_config(config))
    index_conn.autocommit = True
    index_cur = index_conn.cursor()
    db_flavor = detect_db_flavor(index_cur)

    # Unsupported by B-tree indexable data types for YugabyteDB and PostgreSQL
    unsupported_indexable_data_types = ["citext", "tsvector", "tsquery", "inet", "bit varying", "bit", "json", "jsonb", "xml", "point", "line", "lseg", "box", "path", "polygon", "circle", "ARRAY"] if db_flavor == "YUGABYTE" else ["json", "jsonb"]

    indexable_columns = []
    for table_name, table_info in table_schemas.items():
        columns = table_info.get("columns", {})
        for column_name, data_type in columns.items():
            if data_type not in unsupported_indexable_data_types:
                indexable_columns.append((table_name, column_name))
    MAX_RETRIES = 5
    
    print("Index operations thread started")
    
    try:
        while not stop_index_thread.is_set():
            action = random.choice(["create", "drop"])
            retry_count = 0
            sql = ""
            idx_name = ""
            
            try:
                if action == "create":
                    # Pick any random column that can be indexed
                    index_col = random.choice(indexable_columns)
                    if index_col:
                        table, col = index_col
                        idx_name = f"event_gen_idx_{table}_{col}_{random.randint(1000,9999)}"
                        sql = f'CREATE INDEX CONCURRENTLY "{idx_name}" ON "{schema_name}"."{table}" ("{col}");'
                    else:
                        print("No columns found to index.")

                else:
                    # Pick a random droppable index
                    index_cur.execute("""
                        SELECT n.nspname AS schema_name,
                               ic.relname AS index_name
                        FROM pg_class ic
                        JOIN pg_namespace n ON n.oid = ic.relnamespace
                        JOIN pg_index i ON i.indexrelid = ic.oid
                        LEFT JOIN pg_constraint c ON c.conindid = ic.oid
                        WHERE n.nspname = %s
                          AND ic.relkind = 'i'
                          AND c.oid IS NULL
                          AND ic.relname LIKE 'event_gen_idx_%%'
                        ORDER BY random()
                        LIMIT 1
                    """, (schema_name,))
                    row = index_cur.fetchone()
                    
                    if row:
                        schema_name_val, idx_name = row
                        sql = f'DROP INDEX IF EXISTS "{schema_name_val}"."{idx_name}";'
                    else:
                        print("No indexes found to drop.")
                
                if sql:
                    while retry_count < MAX_RETRIES:
                        try:
                            index_cur.execute(sql)
                            print(f"Successful operation on index: {idx_name}")
                            time.sleep(index_events_interval)
                            break
                        except psycopg2.Error as e:
                            print(f"Index operation error on {idx_name}: {e}")
                            time.sleep(1)
                            retry_count += 1
                            if retry_count < MAX_RETRIES:
                                print(f"Retrying operation on index {idx_name} (attempt {retry_count} of {MAX_RETRIES})")
                    else:
                        print(f"[INDEX] GIVING UP on {action.upper()} {idx_name} after {MAX_RETRIES} attempts")

            except Exception as e:
                print(f"Unexpected error in index operations: {e}")
                time.sleep(1)
    
    except Exception as e:
        print(f"Fatal error in index operations thread: {e}")
    finally:
        index_cur.close()
        index_conn.close()
        print("Index operations thread stopped")

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


def generate_hstore_value(
    faker_instance: Optional[Faker] = None,
    min_pairs: int = 1,
    max_pairs: int = 3,
) -> str:
    """Generate a valid hstore text literal, e.g. 'k_0=>v1,k_1=>v2' -- a few
    random key=>value pairs. Faker's word() never contains '=>', ',', or
    whitespace, so neither side needs quoting. An empty hstore ('') is also
    valid Postgres input, but we always emit at least one pair for a
    livelier data distribution. Keys are suffixed with their index so two
    pairs never collapse into one via a duplicate key.
    """
    fake = faker_instance or _fake
    num_pairs = random.randint(min_pairs, max_pairs)
    return ",".join(f"{fake.word()}_{i}=>{fake.word()}" for i in range(num_pairs))


# Element types generate_array_literal knows how to produce a valid, safely
# quoted/unquoted literal for. Anything else (date/timestamp/json/geometric/
# composite/enum elements, ...) is deliberately left unhandled -- guessing a
# literal for a type we don't recognize risks an invalid-syntax error, worse
# than the omit-for-default fallback this enables (see
# get_insert_column_list).
_ARRAY_NUMERIC_ELEMENT_TYPES = frozenset(
    ["smallint", "integer", "bigint", "numeric", "decimal", "real", "double precision", "money"]
)
_ARRAY_STRING_ELEMENT_PREFIXES = ("character", "varchar", "text", "citext", "bpchar")


def _classify_array_element(element_type: Optional[str]) -> Optional[str]:
    """Classify an ARRAY column's element type into 'bool' / 'uuid' /
    'numeric' / 'string' for generate_array_literal, or None if it isn't
    one recognized.

    `element_type` is the regtype text Postgres reports for the column's
    OWN array type (get_array_element_type / generate_table_schemas_bulk's
    udt_regtype both resolve `udt_name::regtype`, where udt_name for an
    ARRAY column is the array type's own name, e.g. '_int4' or
    '_varchar') -- regtype's output function special-cases arrays and
    renders that as "<element type>[]" (e.g. 'integer[]', 'character
    varying[]', 'boolean[]'), NOT the bare element type. A trailing '[]' is
    stripped before classifying so exact-name checks (the numeric set
    below) match regardless -- confirmed against a live Postgres
    (varchar[]/int4[]/bool[] columns) during manual verification of this
    fix; the bool/uuid/string checks below happen to tolerate the
    unstripped suffix too (startswith), but stripping it up front is the
    correct, non-coincidental fix.
    """
    et = (element_type or "").strip().lower()
    if et.endswith("[]"):
        et = et[:-2].strip()
    if et.startswith("bool"):
        return "bool"
    if et.startswith("uuid"):
        return "uuid"
    if et in _ARRAY_NUMERIC_ELEMENT_TYPES:
        return "numeric"
    if et.startswith(_ARRAY_STRING_ELEMENT_PREFIXES):
        return "string"
    return None


def generate_array_literal(
    element_type: Optional[str],
    faker_instance: Optional[Faker] = None,
    min_elements: int = 1,
    max_elements: int = 3,
) -> Optional[str]:
    """Build a valid PostgreSQL array literal, e.g. '{"a","b"}' or
    '{1,2,3}', for an ARRAY column whose element type is `element_type`.
    String-like elements are double-quoted (safe for any word Faker
    produces, or a uuid); numeric/bool elements are emitted bare/unquoted.

    Returns None if `element_type` isn't recognized (see
    _classify_array_element) -- callers (generate_random_data,
    get_insert_column_list) treat that exactly like any other
    unsynthesizable type: omit the column from the INSERT rather than risk
    an invalid literal or an explicit NULL that could violate a NOT NULL
    constraint.
    """
    fake = faker_instance or _fake
    kind = _classify_array_element(element_type)
    if kind is None:
        return None

    if kind == "bool":
        gen_elem = lambda: random.choice(["true", "false"])
    elif kind == "uuid":
        gen_elem = lambda: f'"{fake.uuid4()}"'
    elif kind == "numeric":
        gen_elem = lambda: str(random.randint(-100000, 100000))
    else:  # "string"
        gen_elem = lambda: f'"{fake.word()}"'

    num_elements = random.randint(min_elements, max_elements)
    return "{" + ",".join(gen_elem() for _ in range(num_elements)) + "}"


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

    if ("text" in data_type or "bytea" in data_type or (("character varying" in data_type or "varchar" in data_type) and re.search(r"\((\d+)\)", data_type) is None)) and min_col_size_bytes > 0:
        parts = []
        batch_size = max(1, min_col_size_bytes // 10)
        while True:
            parts.append(fake.pystr(min_chars=batch_size, max_chars=batch_size))
            value = "".join(parts)
            if len(value.encode("utf-8")) >= min_col_size_bytes:
                return value

    elif ("json" in data_type or "jsonb" in data_type) and min_col_size_bytes > 0:
        obj = {}
        while len(json.dumps(obj).encode("utf-8")) < min_col_size_bytes:
            chunk_size = max(1000, min_col_size_bytes // 10)
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

    elif "varchar" in data_type or "text" in data_type or "character varying" in data_type or "bytea" in data_type or "character" in data_type or "char" in data_type:
        match = re.search(r"\((\d+)\)", data_type)
        if match:
            character_maximum_length = int(match.groups()[0])
            value = fake.pystr(max_chars=character_maximum_length)
        else:
            value = ' '.join([fake.word() for _ in range(3)])
        return value
    elif "boolean" in data_type:
        return random.choice(["true", "false"])
    elif data_type == "hstore":
        # hstore columns are reported by information_schema as
        # data_type='USER-DEFINED'; the schema builders (see
        # _resolve_hstore_columns / generate_table_schemas_bulk) relabel
        # them to the literal string 'hstore' so they land here instead of
        # the generic "unknown USER-DEFINED type" NULL path below.
        return generate_hstore_value(fake)
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
        # Delegates to generate_array_literal, which covers string/numeric/
        # bool/uuid element types generically -- including 'character
        # varying' (the regtype text for varchar[]/character varying[]
        # columns), which the old "varchar"/"text" substring checks here
        # missed (neither is a substring of "character varying"), silently
        # falling through with no return -- i.e. an implicit NULL -- and
        # violating NOT NULL constraints on columns like a NOT NULL
        # varchar[] with a DEFAULT. Returns None for element types it
        # doesn't recognize (date/timestamp/json/geometric/...), same as
        # any other unsynthesizable type -- see get_insert_column_list's
        # omit-for-default fallback.
        return generate_array_literal(array_types, fake)

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


def _column_value_is_synthesizable(
    data_type: str,
    enum_values: Optional[List[str]],
    array_types: Optional[str],
) -> bool:
    """Return False for exactly the (data_type, enum_values, array_types)
    combinations generate_random_data is documented to return None for --
    a USER-DEFINED type with no known enum labels (hstore is relabeled to
    the literal type string 'hstore' by the schema builders, so a real
    hstore column never reaches this check as 'USER-DEFINED'), or an ARRAY
    column whose element type _classify_array_element doesn't recognize.
    Used by get_insert_column_list (and, through it, build_insert_values)
    to decide -- once per column, before generating any rows -- whether to
    omit that column from the INSERT's column list entirely rather than
    ever attempt to generate (and fall back to an explicit SQL NULL for) a
    value it cannot produce for that type.
    """
    if data_type == "USER-DEFINED":
        return bool(enum_values)
    if "ARRAY" in data_type:
        return _classify_array_element(array_types) is not None
    return True


def get_insert_column_list(
    table_schemas: Dict[str, Dict[str, Any]],
    table_name: str,
    column_overrides: Optional[Dict[str, Any]] = None,
    unique_value_fns: Optional[Dict[str, Callable[[], Any]]] = None,
    pk_value_fn: Optional[Callable[[], Any]] = None,
) -> List[str]:
    """Return the ordered column-name list build_insert_values will actually
    populate for `table_name`'s INSERT -- every column except ones that are
    BOTH (a) not covered by column_overrides / unique_value_fns /
    pk_value_fn (those priorities always produce a value, or an explicit
    NULL the caller configured -- left exactly as before), AND (b) a type
    generate_random_data cannot synthesize a value for (see
    _column_value_is_synthesizable).

    Omitted columns let the table's own DEFAULT apply at INSERT time,
    instead of an explicit SQL NULL -- which overrides a NOT NULL column's
    DEFAULT and raises a not-null violation (the bug this fixes: e.g. a NOT
    NULL hstore/array column with a DEFAULT). If such a column also has no
    DEFAULT, the INSERT still fails on the NOT NULL constraint -- expected
    and acceptable (see build_insert_values docstring, part 2).

    Callers MUST build their INSERT statement's column list from this --
    NOT directly from table_schemas[table]['columns'].keys() -- so the
    column list matches exactly what build_insert_values emits per row.
    See generator.py / transaction_mode.py's INSERT branches.
    """
    primary_key = table_schemas[table_name].get("primary_key")
    if isinstance(primary_key, str):
        pk_cols = [primary_key]
    elif isinstance(primary_key, list):
        pk_cols = primary_key
    else:
        pk_cols = []

    result: List[str] = []
    for column_name, data_type in table_schemas[table_name]["columns"].items():
        if get_column_override(column_overrides or {}, table_name, column_name):
            result.append(column_name)
            continue
        if (unique_value_fns or {}).get(column_name) is not None:
            result.append(column_name)
            continue
        if pk_value_fn is not None and len(pk_cols) == 1 and column_name == pk_cols[0]:
            result.append(column_name)
            continue
        if "bit" in data_type.lower():
            result.append(column_name)
            continue

        enum_values = fetch_enum_values_for_column(table_schemas, table_name, column_name)
        array_types = fetch_array_types_for_column(table_schemas, table_name, column_name)
        if _column_value_is_synthesizable(data_type, enum_values, array_types):
            result.append(column_name)
        # else: omitted -- see docstring above.
    return result


def build_insert_values(
    table_schemas: Dict[str, Dict[str, Any]],
    table_name: str,
    number_of_rows_to_insert: int,
    min_col_size_bytes: int = 0,
    column_overrides: Optional[Dict[str, Any]] = None,
    pk_value_fn: Optional[Callable[[], Any]] = None,
    unique_value_fns: Optional[Dict[str, Callable[[], Any]]] = None,
) -> Tuple[str, List[Any]]:
    """Build VALUES list like (v1, v2), (v1, v2) for INSERT ... VALUES ...

    Returns a tuple (values_string, pk_values):
      - values_string: the SQL VALUES fragment, e.g. "(1, 'a'), (2, 'b')".
      - pk_values: a list with one entry per inserted row, capturing the RAW
        (pre-SQL-quoting) generated primary-key value(s) for that row --
        a scalar for a single-column PK, a tuple for a composite PK, or None
        if the table has no primary key. Callers use this to refresh an
        in-memory PkPool after a successful INSERT, so later UPDATE/DELETE
        operations can target rows directly instead of scanning the table.

    Per column, the value is picked with this priority:
      1. `column_overrides`, if configured for that table.column.
      2. `unique_value_fns[column_name]`, if given and it returns a
         non-None value (see compute_unique_safe_value) -- guaranteed-unique
         generation for any single-column unique surface (PK, UNIQUE
         constraint, standalone unique index), any type. If the fn returns
         None (type it can't drive, or an unusable encoding), falls through
         to the next priority instead of using None as the value.
      3. `pk_value_fn`, if given -- the dynamic worker pool's monotonic PK
         scheme (see compute_monotonic_pk). Only applies to the table's
         single PK column; composite keys and PK-less tables ignore it.
         Kept for back-compat / callers that don't populate
         `unique_value_fns` for the PK column.
      4. normal type-aware random generation, unchanged.

    Part 2 -- omit-for-default fallback: any column NOT covered by
    priorities 1-3, whose type generate_random_data still cannot produce a
    value for (see get_insert_column_list), is skipped entirely here --
    no entry is added to this row's value tuple for it. Callers must build
    their INSERT's column list via get_insert_column_list (not
    table_schemas[table]['columns'].keys()) so the two stay in sync. This
    replaces the old behavior of emitting an explicit SQL NULL for such a
    column, which overrides a NOT-NULL-with-DEFAULT column's DEFAULT and
    raises a not-null violation that used to kill the whole worker process.
    """
    primary_key = table_schemas[table_name].get("primary_key")
    if isinstance(primary_key, str):
        pk_cols = [primary_key]
    elif isinstance(primary_key, list):
        pk_cols = primary_key
    else:
        pk_cols = []

    insertable_columns = set(
        get_insert_column_list(table_schemas, table_name, column_overrides, unique_value_fns, pk_value_fn)
    )

    rows = []
    pk_values: List[Any] = []
    for _ in range(number_of_rows_to_insert):
        values = []
        row_pk_captured: Dict[str, Any] = {}

        for column_name, data_type in table_schemas[table_name]["columns"].items():
            if column_name not in insertable_columns:
                # Type generate_random_data can't synthesize a value for,
                # and not covered by an override/unique/monotonic-pk hook --
                # omit it from this row entirely (see get_insert_column_list
                # and this function's docstring, part 2).
                continue

            override_spec = get_column_override(column_overrides or {}, table_name, column_name)
            is_monotonic_pk_col = (
                pk_value_fn is not None
                and len(pk_cols) == 1
                and column_name == pk_cols[0]
            )

            unique_fn = (unique_value_fns or {}).get(column_name)
            unique_value = unique_fn() if (not override_spec and unique_fn is not None) else None
            used_unique_value = unique_value is not None

            if override_spec:
                value = generate_override_value(override_spec)
                values.append(f"'{value}'" if value is not None else "NULL")
            elif used_unique_value:
                value = unique_value
                if isinstance(value, str):
                    escaped_value = value.replace("'", "''")
                    values.append(f"'{escaped_value}'")
                else:
                    values.append(f"'{value}'")
            elif is_monotonic_pk_col:
                value = pk_value_fn()
                values.append(f"'{value}'" if value is not None else "NULL")
            elif "bit" in data_type.lower():
                values.append(build_bit_cast_expr(table_schemas, table_name, column_name))
                value = None
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

            if column_name in pk_cols:
                row_pk_captured[column_name] = value

        rows.append(f"({', '.join(values)})")

        if not pk_cols:
            pk_values.append(None)
        elif len(pk_cols) == 1:
            pk_values.append(row_pk_captured.get(pk_cols[0]))
        else:
            pk_values.append(tuple(row_pk_captured.get(col) for col in pk_cols))

    return ", ".join(rows), pk_values


# ----- UPDATE helpers -----

def build_update_values(
    table_schemas: Dict[str, Dict[str, Any]],
    table_name: str,
    columns_to_update: List[str],
    min_col_size_bytes: int = 0,
    column_overrides: Optional[Dict[str, Any]] = None,
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
        quoted_col = quote_ident(col)
        override_spec = get_column_override(column_overrides or {}, table_name, col)
        if override_spec:
            value = generate_override_value(override_spec)
            if value is None:
                set_parts.append(f"{quoted_col} = NULL")
            else:
                set_parts.append(f"{quoted_col} = %s")
                params.append(value)
        elif "bit" in data_type.lower():
            expr = build_bit_cast_expr(table_schemas, table_name, col)
            set_parts.append(f"{quoted_col} = {expr}")
        else:
            if data_type == "USER-DEFINED":
                enum_values = fetch_enum_values_for_column(table_schemas, table_name, col)
                value = generate_random_data(data_type, table_name, enum_values, None, None, min_col_size_bytes)
            else:
                array_types = fetch_array_types_for_column(table_schemas, table_name, col)
                value = generate_random_data(data_type, table_name, None, array_types, None, min_col_size_bytes)
            if value is None:
                set_parts.append(f"{quoted_col} = NULL")
            else:
                set_parts.append(f"{quoted_col} = %s")
                params.append(value)

    set_clause = ", ".join(set_parts)
    return set_clause, params

# ----- Execution utility -----

# SQLSTATEs this module treats as retryable (see classify_retry). Values are
# the PostgreSQL/YugabyteDB error codes, not psycopg2 exception class names,
# so classification works on anything exposing a `.pgcode` attribute -- a
# real psycopg2 error, or a plain fake in unit tests -- without requiring
# psycopg2 to be installed.
SQLSTATE_UNIQUE_VIOLATION = "23505"
SQLSTATE_SERIALIZATION_FAILURE = "40001"  # serialization failure / read-restart
SQLSTATE_DEADLOCK_DETECTED = "40P01"


def classify_retry(exc: BaseException) -> Optional[str]:
    """Classify `exc` for retry purposes by SQLSTATE (`.pgcode`).

    Returns:
      - "unique_violation" for SQLSTATE 23505 (retried immediately, with
        regenerated values -- e.g. a fresh monotonic/random PK -- and no
        backoff sleep; unchanged from before this existed).
      - "conflict" for SQLSTATE 40001 (serialization failure / read-restart
        -- YB throws this even for a single session reading recently-written
        data across tablets) or 40P01 (deadlock detected); retried with
        bounded exponential backoff (see compute_backoff_delay).
      - None for anything else (including exceptions with no `.pgcode`,
        e.g. a plain psycopg2.OperationalError from a dropped connection,
        or a non-DB exception) -- not retryable here; the caller propagates.
    """
    pgcode = getattr(exc, "pgcode", None)
    if pgcode == SQLSTATE_UNIQUE_VIOLATION:
        return "unique_violation"
    if pgcode in (SQLSTATE_SERIALIZATION_FAILURE, SQLSTATE_DEADLOCK_DETECTED):
        return "conflict"
    return None


def compute_backoff_delay(attempt: int, base: float = 0.05, cap: float = 2.0) -> float:
    """Bounded exponential backoff delay (seconds) for retry `attempt`
    (1-indexed): base * 2**(attempt-1), capped at `cap`. Pure and
    deterministic (no jitter, no clock access), so directly unit-testable.
    """
    if attempt < 1:
        attempt = 1
    return min(cap, base * (2 ** (attempt - 1)))


def read_control_rate(path: Optional[str], last: float) -> float:
    """Read the single float control rate from `path` for the cascade
    trimmer controller's runtime-adjustable throttle (Option B -- see
    ARCHITECTURE.md).

    On success, returns the parsed float. On `path` being None/empty,
    missing file, empty/whitespace-only content, a parse error, or any
    other exception (e.g. a transient read racing the controller's atomic
    replace), returns `last` (hold the previous commanded rate) -- this
    function never raises. Negative values are returned as-is; interpreting
    a rate <= 0 as "pause" is the caller's (generator.py's) job, not this
    function's.
    """
    if not path:
        return last
    try:
        with open(path, "r") as f:
            content = f.read().strip()
        if not content:
            return last
        return float(content)
    except Exception:
        return last


def execute_with_retry(
    run_once_fn: Callable[[], None],
    rebuild_fn: Callable[[], None],
    rollback_fn: Callable[[], None],
    *,
    max_retries: int = 50,
    backoff_base: float = 0.05,
    backoff_cap: float = 2.0,
    sleep_fn: Callable[[float], None] = time.sleep,
) -> bool:
    """Execute a write, retrying on:
      - UniqueViolation (23505): immediate retry with regenerated values
        (via `rebuild_fn`), no backoff sleep -- unchanged from before.
      - Serialization/read-restart (40001) or deadlock (40P01): retried with
        bounded exponential backoff (`compute_backoff_delay`), also calling
        `rebuild_fn` (a safe no-op for callers whose values don't need
        regenerating on a plain conflict retry).

    Any other exception propagates after `rollback_fn()`, unchanged from
    before. Returns True on success, False if max_retries is exhausted.
    """
    retry_count = 0
    while retry_count <= max_retries:
        try:
            run_once_fn()
            return True
        except Exception as e:
            kind = classify_retry(e)
            if kind is None:
                rollback_fn()
                raise
            rollback_fn()
            retry_count += 1
            if kind == "unique_violation":
                print(f"Retrying operation after UniqueViolation (attempt {retry_count} of {max_retries})")
                print(f"Error details: {e}")
            else:
                delay = compute_backoff_delay(retry_count, backoff_base, backoff_cap)
                print(
                    f"Retrying operation after {kind} ({getattr(e, 'pgcode', '?')}) "
                    f"(attempt {retry_count} of {max_retries}), backing off {delay:.3f}s"
                )
                print(f"Error details: {e}")
                sleep_fn(delay)
            rebuild_fn()
    print("Reached maximum retry attempts. Skipping...")
    return False


# ----- Sampling helpers -----

DEFAULT_ROW_ESTIMATE = 1000

def build_sampling_condition(
    db_flavor: str,
    table_name: str,
    primary_key: "str | List[str]",
    target_row_count: int,
    estimated_row_count: Optional[int],
) -> Tuple[str, List[Any]]:
    """
    Build a WHERE condition fragment and parameters for sampling rows
    for UPDATE/DELETE operations.

    primary_key can be a single column name (str) or a list of column names.
    For composite PKs, uses row-value syntax: (col1, col2) IN (SELECT col1, col2 ...).

    For PostgreSQL, this uses TABLESAMPLE SYSTEM_ROWS(target_row_count).
    For YugabyteDB, it uses a probabilistic filter WHERE random() < p,
    where p is derived from target_row_count and an estimated row count.
    """
    if isinstance(primary_key, str):
        pk_cols = [primary_key]
    else:
        pk_cols = primary_key

    quoted_table = quote_ident(table_name)
    if len(pk_cols) == 1:
        pk_select = quote_ident(pk_cols[0])
        pk_where = quote_ident(pk_cols[0])
    else:
        pk_select = ", ".join(quote_ident(c) for c in pk_cols)
        pk_where = f"({pk_select})"

    if db_flavor == "POSTGRES":
        where_clause = (
            f"{pk_where} IN ("
            f"SELECT {pk_select} FROM {quoted_table} TABLESAMPLE SYSTEM_ROWS(%s))"
        )
        return where_clause, [target_row_count]

    # YugabyteDB path: derive p from estimated row count
    est = estimated_row_count if estimated_row_count and estimated_row_count > 0 else DEFAULT_ROW_ESTIMATE

    p = min(1.0, float(target_row_count) / float(est))

    where_clause = (
        f"{pk_where} IN ("
        f"SELECT {pk_select} FROM {quoted_table} WHERE random() < %s)"
    )
    return where_clause, [p]


def build_pk_in_condition(
    primary_key: "str | List[str]",
    ids: List[Any],
) -> Tuple[str, List[Any]]:
    """
    Build a WHERE clause fragment and parameters that target explicit
    primary-key values via IN, for use with an in-memory PkPool instead of
    scanning the table (see build_sampling_condition for the scan-based
    fallback).

    primary_key can be a single column name (str) or a list of column names,
    mirroring the pk formatting used in build_sampling_condition.

    For a single-column PK, `ids` is a list of scalar values and this
    returns row-value form, e.g. ("id IN (%s, %s, %s)", ids).

    For a composite PK, `ids` is a list of tuples (one per PK column, in
    primary_key order) and this returns row-value form, e.g.
    ("(c1, c2) IN ((%s, %s), (%s, %s))", flat_params).
    """
    if isinstance(primary_key, str):
        pk_cols = [primary_key]
    else:
        pk_cols = primary_key

    if not ids:
        # No ids to target; caller should not normally reach here (the
        # generator only calls this when a non-empty sample was drawn from
        # the pool), but guard against building invalid SQL.
        return "1=0", []

    if len(pk_cols) == 1:
        placeholders = ", ".join(["%s"] * len(ids))
        where_clause = f"{quote_ident(pk_cols[0])} IN ({placeholders})"
        return where_clause, list(ids)

    pk_where = "(" + ", ".join(quote_ident(c) for c in pk_cols) + ")"
    row_placeholder = "(" + ", ".join(["%s"] * len(pk_cols)) + ")"
    placeholders = ", ".join([row_placeholder] * len(ids))
    where_clause = f"{pk_where} IN ({placeholders})"
    flat_params: List[Any] = []
    for row in ids:
        flat_params.extend(row)
    return where_clause, flat_params


def seed_pk_pool(
    cursor: Any,
    schema_name: Optional[str],
    table_name: str,
    primary_key: "str | List[str]",
    pool: Any,
    limit: int = 200000,
) -> None:
    """
    Prefill an in-memory PkPool with primary-key values that already exist
    in a table, so UPDATE/DELETE can target rows directly from the start.

    Only supported for a single-column primary key: runs
    `SELECT <pk> FROM <schema>.<table> LIMIT <limit>` and adds the results to
    `pool`. For a composite (or missing) primary key, this is a no-op and
    `pool` is left empty -- callers should fall back to
    build_sampling_condition in that case.
    """
    if isinstance(primary_key, str):
        pk_col = primary_key
    elif isinstance(primary_key, list) and len(primary_key) == 1:
        pk_col = primary_key[0]
    else:
        return

    qualified_table = f"{quote_ident(schema_name)}.{quote_ident(table_name)}" if schema_name else quote_ident(table_name)
    cursor.execute(f"SELECT {quote_ident(pk_col)} FROM {qualified_table} LIMIT %s", (limit,))
    pool.add_many([row[0] for row in cursor.fetchall()])
