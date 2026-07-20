"""
shared_cache: a read-only, versioned, mmap-backed cache of schema metadata
and live primary-key snapshots, built once by the controller and shared
(read-only) by every worker process.

See ~/yb-ratetest/dynamic-worker-pool-design.md section 7 ("Shared cache")
and IMPLEMENTATION_CONTRACTS.md for the design and exact API this module
implements.

Layout on disk:

    <cache_dir>/
        CURRENT                     # text file: the current version string
        <version>/
            schema.json             # {table: schema_meta}, schema_meta is the FULL
                                     #   per-table shape returned by
                                     #   utils.generate_table_schemas -- columns,
                                     #   primary_key, array_types, enum_values, bit_info.
                                     #   Written verbatim; load_schema returns it as-is.
            max_pk.json             # {table: max_pk_int | None}
            pk_index.json           # {table: {"format": "int64"|"json"|None,
                                     #          "composite": bool, "count": int,
                                     #          "file": "pk/<table>.<ext>"}}
            pk/
                <table>.i64         # single-col integer PK: sorted, packed
                                     #   little-endian int64 array (mmap'd)
                <table>.json        # composite PK: JSON array of tuples;
                                     #   single-col non-integer PK: JSON
                                     #   array of scalars (loaded, not mmap'd)

`build_cache` is the *only* writer, and never mutates an existing version:
each call creates a brand-new `<version>/` directory and then atomically
flips `CURRENT` (via a temp-file + `os.replace`) to point at it. Workers
only ever read via `current_version` / `load_schema` / `table_max_pk` /
`open_pk_base`.

Only `build_cache` touches a database (via a duck-typed `cursor` --
`cursor.execute(sql, params)` / `cursor.fetchall()` / `cursor.fetchone()`,
matching the psycopg2 cursor protocol). It delegates the schema-metadata
half of the build to `utils.generate_table_schemas` (same call
`generator.py` makes today), so `schema.json` carries the FULL per-table
shape (columns/primary_key/array_types/enum_values/bit_info), not a
reduced one -- `load_schema` hands that back to workers verbatim, a
drop-in replacement for a per-worker `generate_table_schemas` call.
Everything else -- `PkBase`, `current_version`, `load_schema`,
`table_max_pk`, `open_pk_base`, and `build_cache`'s own PK-snapshot
file-format/query logic -- is pure stdlib (json/os/struct/mmap/bisect/
collections/tempfile) and importable/testable without a database.

`utils` is imported guarded (try/except), mirroring the same pattern
`migration_monitor.py` uses for `psycopg2`: `utils.py` itself currently has
an unconditional `import psycopg2` at module scope (a separate,
independently-owned change makes that lazy), so importing it can fail in
an environment without psycopg2 installed. When that happens, `utils` is
left `None` and `build_cache` raises a clear `RuntimeError` if actually
called -- every other function in this module (`PkBase`, `load_schema`,
`open_pk_base`, ...) has no dependency on `utils`/psycopg2 at all and
keeps working regardless.
"""

import bisect
import json
import os
import struct
import tempfile
import time
import uuid

try:
    import mmap
except ImportError:  # pragma: no cover - mmap is stdlib on all supported platforms
    mmap = None

try:
    import utils
except ImportError:  # pragma: no cover - utils.py requires psycopg2 today; see module docstring
    utils = None


_INTEGER_DATA_TYPES = frozenset(
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


def _is_integer_type(data_type):
    if not data_type:
        return False
    return data_type.strip().lower() in _INTEGER_DATA_TYPES


# --------------------------------------------------------------------------
# Atomic file helpers
# --------------------------------------------------------------------------

def _write_json_atomic(path, obj):
    directory = os.path.dirname(path) or "."
    os.makedirs(directory, exist_ok=True)
    fd, tmp_path = tempfile.mkstemp(dir=directory, prefix=".tmp-", suffix=".json")
    try:
        with os.fdopen(fd, "w") as f:
            # default=str stringifies PK values that aren't JSON-native
            # (datetime/date/time, Decimal, etc.) for timestamp/numeric PK
            # columns; the worker passes them back as IN-clause params and YB
            # casts. int PKs use the packed-int64 path, not this.
            json.dump(obj, f, default=str)
        os.replace(tmp_path, path)
    except Exception:
        try:
            os.remove(tmp_path)
        except OSError:
            pass
        raise


def _write_int64_array_atomic(path, values):
    directory = os.path.dirname(path) or "."
    os.makedirs(directory, exist_ok=True)
    fd, tmp_path = tempfile.mkstemp(dir=directory, prefix=".tmp-", suffix=".i64")
    try:
        with os.fdopen(fd, "wb") as f:
            if values:
                f.write(struct.pack("<%dq" % len(values), *values))
        os.replace(tmp_path, path)
    except Exception:
        try:
            os.remove(tmp_path)
        except OSError:
            pass
        raise


def _set_current(cache_dir, version):
    os.makedirs(cache_dir, exist_ok=True)
    fd, tmp_path = tempfile.mkstemp(dir=cache_dir, prefix=".tmp-CURRENT-")
    try:
        with os.fdopen(fd, "w") as f:
            f.write(version)
        os.replace(tmp_path, os.path.join(cache_dir, "CURRENT"))
    except Exception:
        try:
            os.remove(tmp_path)
        except OSError:
            pass
        raise


def _new_version_id():
    return "%s-%s" % (time.strftime("%Y%m%dT%H%M%S"), uuid.uuid4().hex[:8])


# --------------------------------------------------------------------------
# PK-snapshot query logic (self-contained: duck-typed cursor only). Schema
# metadata itself now comes from utils.generate_table_schemas (see
# build_cache below) -- only the PK-values/max(pk) queries stay local here.
# --------------------------------------------------------------------------

def _qualify(table_name, schema_name):
    return "%s.%s" % (schema_name, table_name) if schema_name else table_name


def _query_max_pk(cursor, table_name, schema_name, pk_col):
    qualified = _qualify(table_name, schema_name)
    cursor.execute("SELECT MAX(%s) FROM %s" % (pk_col, qualified))
    row = cursor.fetchone()
    if not row or row[0] is None:
        return None
    return int(row[0])


def _query_pk_values(cursor, table_name, schema_name, pk_cols, limit):
    qualified = _qualify(table_name, schema_name)
    cols_sql = ", ".join(pk_cols)
    cursor.execute(
        "SELECT %s FROM %s LIMIT %%s" % (cols_sql, qualified),
        (limit,),
    )
    return cursor.fetchall()


def build_cache(cursor, schema_name, table_list, pk_pool_maxsize, cache_dir):
    """Build a brand-new, immutable, versioned cache: FULL schema metadata
    for every table in `table_list` (via `utils.generate_table_schemas` --
    the same call `generator.py` makes today, so `schema.json` ends up with
    columns/primary_key/array_types/enum_values/bit_info, not a reduced
    shape), plus up to `pk_pool_maxsize` live PK values (and `max(pk)`) per
    table. Atomically flips `CURRENT` to the new version and returns the
    version string. Never mutates an existing version -- only ever called
    by the controller.
    """
    if utils is None:
        raise RuntimeError(
            "the utils module (utils.generate_table_schemas) is required to build the "
            "shared cache but could not be imported -- see shared_cache.py's module "
            "docstring (utils.py currently requires psycopg2 at import time)"
        )

    version = _new_version_id()
    version_dir = os.path.join(cache_dir, version)
    pk_dir = os.path.join(version_dir, "pk")

    # The one and only schema-metadata query path: identical to what a
    # standalone generator.py worker does today, so schema.json is a
    # verbatim, complete capture -- workers reading it back via
    # load_schema() get a drop-in replacement for their own
    # generate_table_schemas() call, no per-worker catalog queries needed.
    # Batched (schema-wide) introspection -- per-table/per-column
    # generate_table_schemas is fatally slow at hundreds-of-tables scale on YB.
    schemas = utils.generate_table_schemas_bulk(
        cursor, schema_name=schema_name, manual_table_list=table_list
    )

    max_pk = {}
    pk_index = {}

    for table_name in table_list:
        table_schema = schemas.get(table_name)
        if table_schema is None:
            # utils.generate_table_schemas already printed "Table '...' not
            # found." for this one; nothing to snapshot.
            continue

        columns = table_schema.get("columns") or {}
        primary_key = table_schema.get("primary_key")

        if not primary_key:
            pk_index[table_name] = {"format": None, "composite": False, "count": 0}
            max_pk[table_name] = None
            continue

        composite = len(primary_key) > 1
        rows = _query_pk_values(cursor, table_name, schema_name, primary_key, pk_pool_maxsize)

        if not composite:
            pk_col = primary_key[0]
            is_int = _is_integer_type(columns.get(pk_col))
            if is_int:
                values = sorted(int(row[0]) for row in rows)
                file_name = "%s.i64" % table_name
                _write_int64_array_atomic(os.path.join(pk_dir, file_name), values)
                pk_index[table_name] = {
                    "format": "int64",
                    "composite": False,
                    "count": len(values),
                    "file": os.path.join("pk", file_name),
                }
                max_pk[table_name] = _query_max_pk(cursor, table_name, schema_name, pk_col)
                continue

            # Single-column, non-integer PK (e.g. uuid/text): can't be
            # packed as int64; store as a loaded (not mmap'd) JSON array of
            # scalars. Still usable for fast IN (...) seeding -- just not
            # sharable via zero-copy mmap.
            values = [row[0] for row in rows]
            file_name = "%s.json" % table_name
            _write_json_atomic(os.path.join(pk_dir, file_name), values)
            pk_index[table_name] = {
                "format": "json",
                "composite": False,
                "count": len(values),
                "file": os.path.join("pk", file_name),
            }
            max_pk[table_name] = None
            continue

        # Composite PK: store as JSON tuples (loaded, not mmap'd). This is
        # the path that lets the 12 composite-PK tables use the pool too,
        # via build_pk_in_condition-compatible tuples.
        values = [list(row) for row in rows]
        file_name = "%s.json" % table_name
        _write_json_atomic(os.path.join(pk_dir, file_name), values)
        pk_index[table_name] = {
            "format": "json",
            "composite": True,
            "count": len(values),
            "file": os.path.join("pk", file_name),
        }
        max_pk[table_name] = None

    _write_json_atomic(os.path.join(version_dir, "schema.json"), schemas)
    _write_json_atomic(os.path.join(version_dir, "max_pk.json"), max_pk)
    _write_json_atomic(os.path.join(version_dir, "pk_index.json"), pk_index)

    # Only after every file for this version is durably written do we flip
    # the pointer -- a reader following CURRENT can never observe a
    # partially-written version.
    _set_current(cache_dir, version)
    return version


# --------------------------------------------------------------------------
# Reader API
# --------------------------------------------------------------------------

def current_version(cache_dir):
    """Return the current version string."""
    with open(os.path.join(cache_dir, "CURRENT"), "r") as f:
        return f.read().strip()


def load_schema(cache_dir, version):
    """Return {table: schema_meta} for the given version."""
    path = os.path.join(cache_dir, version, "schema.json")
    with open(path, "r") as f:
        return json.load(f)


def table_max_pk(cache_dir, version, table):
    """Return max(pk) for `table` in this version, or None (no PK, a
    composite PK, or a non-integer single-column PK -- none of which
    produce a usable integer ceiling for monotonic PK generation)."""
    path = os.path.join(cache_dir, version, "max_pk.json")
    with open(path, "r") as f:
        data = json.load(f)
    return data.get(table)


def open_pk_base(cache_dir, version, table):
    """Return a read-only `PkBase` for `table` in this version.

    If the table has no seedable PK, returns an empty `PkBase` (len 0)
    rather than raising -- callers fall back to random()-based sampling
    for that table (logged elsewhere), per the design's error handling.
    """
    version_dir = os.path.join(cache_dir, version)
    index_path = os.path.join(version_dir, "pk_index.json")
    with open(index_path, "r") as f:
        pk_index = json.load(f)

    entry = pk_index.get(table)
    if not entry or entry.get("format") is None:
        return PkBase.empty()

    file_path = os.path.join(version_dir, entry["file"])
    if entry["format"] == "int64":
        return PkBase.from_int64_file(file_path, entry["count"])

    with open(file_path, "r") as f:
        values = json.load(f)
    composite = bool(entry.get("composite"))
    if composite:
        values = [tuple(v) for v in values]
    return PkBase.from_values(values, composite=composite)


class PkBase(object):
    """Read-only, shared base snapshot of primary-key values.

    Two backing formats:
      - int64: a sorted, mmap'd little-endian int64 array (single-column
        integer PKs). Zero-copy via `memoryview(...).cast('q')` over the
        mmap, so every worker process shares the same physical pages --
        one copy in RAM regardless of worker count. Membership is a
        binary search (the array is sorted at build time).
      - json: a fully loaded, in-memory Python list (composite PKs, as
        tuples; or single-column non-integer PKs, as scalars). Since it's
        already fully materialized, membership uses a plain set.

    Built only by `build_cache` (via `open_pk_base`); a `PkPool` only ever
    reads from this (`__len__`, `__contains__`, `sample`) and never
    mutates it.
    """

    def __init__(self):
        self.is_composite = False
        self._count = 0
        self._mv = None
        self._mmap = None
        self._file = None
        self._values = None
        self._member_set = None

    @classmethod
    def empty(cls):
        base = cls()
        base._values = []
        base._member_set = frozenset()
        return base

    @classmethod
    def from_int64_file(cls, file_path, count):
        base = cls()
        base.is_composite = False
        base._count = count
        if count == 0:
            base._values = []
            base._member_set = frozenset()
            return base
        f = open(file_path, "rb")
        mm = mmap.mmap(f.fileno(), 0, access=mmap.ACCESS_READ)
        base._file = f
        base._mmap = mm
        base._mv = memoryview(mm).cast("q")
        return base

    @classmethod
    def from_values(cls, values, composite):
        base = cls()
        base.is_composite = composite
        base._values = list(values)
        base._count = len(base._values)
        if composite:
            base._member_set = frozenset(tuple(v) for v in base._values)
        else:
            base._member_set = frozenset(base._values)
        return base

    def __len__(self):
        return self._count

    def __contains__(self, pk):
        if self._mv is not None:
            idx = bisect.bisect_left(self._mv, pk)
            return idx < len(self._mv) and self._mv[idx] == pk
        return pk in self._member_set

    def sample(self, n, rng):
        """Return up to `n` members, chosen at random via `rng`."""
        if n <= 0 or self._count == 0:
            return []
        if self._mv is not None:
            if n >= self._count:
                idxs = list(range(self._count))
                rng.shuffle(idxs)
                return [self._mv[i] for i in idxs]
            idxs = rng.sample(range(self._count), n)
            return [self._mv[i] for i in idxs]

        if n >= self._count:
            values = list(self._values)
            rng.shuffle(values)
            return values
        return rng.sample(self._values, n)

    def close(self):
        """Release the mmap/file handle (int64-backed instances only)."""
        if self._mmap is not None:
            self._mv = None
            self._mmap.close()
            self._mmap = None
        if self._file is not None:
            self._file.close()
            self._file = None
