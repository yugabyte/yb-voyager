"""
Unit tests for shared_cache.py.

All pure/no-DB, no network. Two groups of tests:

1. Format/PkBase/reader-API tests -- exercise `current_version`,
   `load_schema`, `table_max_pk`, `open_pk_base`, and `PkBase` directly
   against PRE-WRITTEN FIXTURE FILES (schema.json/max_pk.json/
   pk_index.json/pk/*), never by calling `build_cache`. This keeps them
   independent of `utils.generate_table_schemas` and its catalog queries
   entirely (per the Wave-2 addendum: "don't fake generate_table_schemas'
   catalog queries").

2. `build_cache` integration tests -- exercise the PK-snapshot-building
   half of `build_cache` (versioning, atomic CURRENT flip, int64/json/
   composite PK file formats) via a small `FakeCursor` that only answers
   the two query shapes `build_cache` itself still issues directly
   (MAX(pk) and the PK-values SELECT). The schema-metadata half is
   stubbed by monkeypatching `utils.generate_table_schemas` to return a
   canned dict -- i.e. the whole function is replaced, not its underlying
   catalog queries -- so build_cache's contract ("write generate_table_
   schemas' output verbatim to schema.json") is verified without needing
   a real DB or a fake information_schema/pg_catalog.

`utils` itself is imported guarded: utils.py currently has an
unconditional `import psycopg2` at module scope (a separate,
independently-owned change makes that lazy), and this sandbox has no
psycopg2 installed. If the real import fails, a minimal stub module is
registered in `sys.modules` under the name "utils" *before* `shared_cache`
is imported, so `shared_cache.py`'s own (also-guarded) `import utils`
resolves to it instead of re-attempting -- and failing -- the real one.
"""

import os
import random
import shutil
import struct
import sys
import tempfile
import types
import unittest
from unittest import mock

try:
    import utils
except Exception:
    utils = types.ModuleType("utils")

    def _unset_generate_table_schemas(cursor, schema_name=None, manual_table_list=None,
                                       exclude_table_list=None):
        raise NotImplementedError(
            "stub utils.generate_table_schemas must be patched by the test"
        )

    utils.generate_table_schemas = _unset_generate_table_schemas
    sys.modules["utils"] = utils

import shared_cache
from shared_cache import PkBase, build_cache, current_version, load_schema, \
    open_pk_base, table_max_pk


# --------------------------------------------------------------------------
# Group 1: format/PkBase/reader-API tests against pre-written fixture files.
# --------------------------------------------------------------------------

def _write_fixture_cache(cache_dir, version, schema, max_pk, pk_index, pk_files):
    """Write a complete <cache_dir>/<version>/{schema,max_pk,pk_index}.json
    plus any pk/<file> payloads, and flip CURRENT -- all directly, with no
    call into build_cache/utils. `pk_files` is {relative_path: bytes|list}:
    a list is JSON-dumped, bytes are written raw (for packed int64 arrays).
    """
    import json as _json

    version_dir = os.path.join(cache_dir, version)
    pk_dir = os.path.join(version_dir, "pk")
    os.makedirs(pk_dir, exist_ok=True)

    with open(os.path.join(version_dir, "schema.json"), "w") as f:
        _json.dump(schema, f)
    with open(os.path.join(version_dir, "max_pk.json"), "w") as f:
        _json.dump(max_pk, f)
    with open(os.path.join(version_dir, "pk_index.json"), "w") as f:
        _json.dump(pk_index, f)

    for rel_path, payload in pk_files.items():
        full_path = os.path.join(version_dir, rel_path)
        os.makedirs(os.path.dirname(full_path), exist_ok=True)
        if isinstance(payload, (bytes, bytearray)):
            with open(full_path, "wb") as f:
                f.write(payload)
        else:
            with open(full_path, "w") as f:
                _json.dump(payload, f)

    os.makedirs(cache_dir, exist_ok=True)
    with open(os.path.join(cache_dir, "CURRENT"), "w") as f:
        f.write(version)


def _full_schema_meta(columns, primary_key, array_types=None, enum_values=None, bit_info=None):
    """A schema_meta dict in the FULL shape utils.generate_table_schemas
    returns (columns/primary_key/array_types/enum_values/bit_info) -- what
    a fixture schema.json now looks like end to end."""
    return {
        "columns": columns,
        "primary_key": primary_key,
        "array_types": array_types or {},
        "enum_values": enum_values or {},
        "bit_info": bit_info or {},
    }


class FixtureCacheTestBase(unittest.TestCase):
    def setUp(self):
        self.cache_dir = tempfile.mkdtemp(prefix="shared_cache_fixture_test_")

    def tearDown(self):
        shutil.rmtree(self.cache_dir, ignore_errors=True)


class TestReaderApiAgainstFixtures(FixtureCacheTestBase):
    """The 'cache serialize/deserialize + mmap load' testing-plan item,
    driven entirely by hand-written fixture files -- no build_cache, no
    utils, no cursor of any kind."""

    def setUp(self):
        super().setUp()
        self.version = "20260720T000000-deadbeef"

        schema = {
            "users": _full_schema_meta(
                columns={"id": "integer", "name": "text", "tags": "ARRAY"},
                primary_key=["id"],
                array_types={"tags": "text"},
            ),
            "sessions": _full_schema_meta(
                columns={"user_id": "integer", "device_id": "integer", "started_at": "timestamp"},
                primary_key=["user_id", "device_id"],
            ),
            "accounts": _full_schema_meta(
                columns={"acct_uuid": "uuid", "balance": "numeric(10,2)", "status": "USER-DEFINED"},
                primary_key=["acct_uuid"],
                enum_values={"status": ["active", "closed"]},
            ),
            "logs": _full_schema_meta(
                columns={"id": "bigint", "msg": "text", "flags": "bit"},
                primary_key=None,
                bit_info={"flags": {"length": 8, "varying": False}},
            ),
        }
        max_pk = {"users": 50, "sessions": None, "accounts": None, "logs": None}
        pk_index = {
            "users": {"format": "int64", "composite": False, "count": 50, "file": "pk/users.i64"},
            "sessions": {"format": "json", "composite": True, "count": 6, "file": "pk/sessions.json"},
            "accounts": {"format": "json", "composite": False, "count": 3, "file": "pk/accounts.json"},
            "logs": {"format": None, "composite": False, "count": 0},
        }
        users_values = list(range(1, 51))
        pk_files = {
            "pk/users.i64": struct.pack("<%dq" % len(users_values), *users_values),
            "pk/sessions.json": [[u, d] for u in range(1, 4) for d in range(1, 3)],
            "pk/accounts.json": ["uuid-1", "uuid-2", "uuid-3"],
        }

        _write_fixture_cache(self.cache_dir, self.version, schema, max_pk, pk_index, pk_files)
        self.schema = schema

    def test_current_version_reads_fixture_pointer(self):
        self.assertEqual(current_version(self.cache_dir), self.version)

    def test_load_schema_returns_full_shape_verbatim(self):
        loaded = load_schema(self.cache_dir, self.version)
        self.assertEqual(loaded, self.schema)
        # Spot-check the fields a reduced/legacy schema.json would have
        # dropped -- array_types/enum_values/bit_info must survive intact.
        self.assertEqual(loaded["users"]["array_types"], {"tags": "text"})
        self.assertEqual(loaded["accounts"]["enum_values"], {"status": ["active", "closed"]})
        self.assertEqual(loaded["logs"]["bit_info"], {"flags": {"length": 8, "varying": False}})
        self.assertIsNone(loaded["logs"]["primary_key"])

    def test_max_pk_only_populated_for_single_col_int_pk(self):
        self.assertEqual(table_max_pk(self.cache_dir, self.version, "users"), 50)
        self.assertIsNone(table_max_pk(self.cache_dir, self.version, "sessions"))
        self.assertIsNone(table_max_pk(self.cache_dir, self.version, "accounts"))
        self.assertIsNone(table_max_pk(self.cache_dir, self.version, "logs"))

    def test_users_base_is_int64_mmap_and_not_composite(self):
        base = open_pk_base(self.cache_dir, self.version, "users")
        try:
            self.assertEqual(len(base), 50)
            self.assertFalse(base.is_composite)
            self.assertIn(1, base)
            self.assertIn(50, base)
            self.assertNotIn(999, base)
            sampled = base.sample(10, random.Random(1))
            self.assertEqual(len(sampled), 10)
            self.assertEqual(len(set(sampled)), 10)
            for pk in sampled:
                self.assertIn(pk, range(1, 51))
        finally:
            base.close()

    def test_users_int64_file_is_sorted_packed_little_endian(self):
        file_path = os.path.join(self.cache_dir, self.version, "pk", "users.i64")
        with open(file_path, "rb") as f:
            data = f.read()
        count = len(data) // 8
        values = struct.unpack("<%dq" % count, data)
        self.assertEqual(list(values), sorted(values))
        self.assertEqual(list(values), list(range(1, 51)))

    def test_sessions_base_is_composite_tuples(self):
        base = open_pk_base(self.cache_dir, self.version, "sessions")
        self.assertTrue(base.is_composite)
        self.assertEqual(len(base), 6)
        self.assertIn((1, 1), base)
        self.assertIn((3, 2), base)
        self.assertNotIn((9, 9), base)
        sampled = base.sample(6, random.Random(2))
        self.assertEqual(len(sampled), 6)
        for pk in sampled:
            self.assertIsInstance(pk, tuple)

    def test_accounts_base_is_scalar_json_not_composite(self):
        base = open_pk_base(self.cache_dir, self.version, "accounts")
        self.assertFalse(base.is_composite)
        self.assertEqual(len(base), 3)
        self.assertIn("uuid-1", base)
        self.assertNotIn("uuid-999", base)

    def test_logs_no_pk_yields_empty_base(self):
        base = open_pk_base(self.cache_dir, self.version, "logs")
        self.assertEqual(len(base), 0)
        self.assertEqual(base.sample(10, random.Random(3)), [])
        self.assertNotIn(1, base)


class TestPkBaseDirectFileFormat(unittest.TestCase):
    """Exercises the int64 mmap format directly (no fixtures/cache_dir at
    all), i.e. the 'cache serialize/deserialize + mmap load' testing-plan
    item at the PkBase level."""

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp(prefix="pkbase_test_")

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_mmap_backed_base_reads_back_correctly(self):
        values = sorted([10, 5, 7, 1, 999, 42])
        path = os.path.join(self.tmpdir, "t.i64")
        with open(path, "wb") as f:
            f.write(struct.pack("<%dq" % len(values), *values))

        base = PkBase.from_int64_file(path, len(values))
        try:
            self.assertEqual(len(base), len(values))
            for v in values:
                self.assertIn(v, base)
            self.assertNotIn(123456, base)
            sampled = base.sample(len(values), random.Random(4))
            self.assertEqual(sorted(sampled), values)
        finally:
            base.close()

    def test_empty_int64_base(self):
        path = os.path.join(self.tmpdir, "empty.i64")
        with open(path, "wb"):
            pass
        base = PkBase.from_int64_file(path, 0)
        self.assertEqual(len(base), 0)
        self.assertEqual(base.sample(5, random.Random(5)), [])

    def test_from_values_composite(self):
        base = PkBase.from_values([(1, "a"), (2, "b"), (3, "c")], composite=True)
        self.assertTrue(base.is_composite)
        self.assertIn((2, "b"), base)
        self.assertNotIn((9, "z"), base)
        self.assertEqual(len(base.sample(100, random.Random(6))), 3)

    def test_from_values_scalar(self):
        base = PkBase.from_values(["a", "b", "c"], composite=False)
        self.assertFalse(base.is_composite)
        self.assertIn("a", base)
        self.assertNotIn("z", base)

    def test_pk_base_empty_classmethod(self):
        base = PkBase.empty()
        self.assertEqual(len(base), 0)
        self.assertFalse(base.is_composite)
        self.assertNotIn(1, base)
        self.assertEqual(base.sample(10, random.Random(7)), [])


class TestIsIntegerType(unittest.TestCase):
    def test_recognizes_integer_types(self):
        for t in ("integer", "bigint", "smallint", "INTEGER", " bigint "):
            self.assertTrue(shared_cache._is_integer_type(t))

    def test_rejects_non_integer_types(self):
        for t in ("text", "uuid", "numeric(10,2)", "character varying(50)", None, ""):
            self.assertFalse(shared_cache._is_integer_type(t))


# --------------------------------------------------------------------------
# Group 2: build_cache tests. utils.generate_table_schemas is monkeypatched
# wholesale (never its underlying catalog queries); a FakeCursor answers
# only the PK-values/MAX(pk) queries build_cache still issues itself.
# --------------------------------------------------------------------------

class FakeCursor(object):
    """Duck-typed stand-in for a psycopg2 cursor. Answers only the two SQL
    shapes build_cache issues directly today: `SELECT MAX(col) FROM t` and
    `SELECT c1, c2 FROM t LIMIT %s`. Backed by {table: {"rows": [...]}}."""

    def __init__(self, catalog):
        self.catalog = catalog
        self._result = None

    def execute(self, sql, params=None):
        params = params or ()
        stripped = sql.strip()

        if stripped.upper().startswith("SELECT MAX("):
            col = stripped.split("MAX(", 1)[1].split(")", 1)[0]
            table_name = stripped.split("FROM", 1)[1].strip().split(".")[-1].strip()
            rows = self.catalog[table_name]["rows"]
            pk_cols = self.catalog[table_name]["pk_cols"]
            col_idx = pk_cols.index(col)
            values = [r[col_idx] for r in rows]
            self._result = [(max(values) if values else None,)]
            return

        # PK-values seeding query: "SELECT c1, c2 FROM t LIMIT %s"
        table_name = stripped.split("FROM", 1)[1].split("LIMIT")[0].strip().split(".")[-1].strip()
        limit = params[0]
        rows = self.catalog[table_name]["rows"]
        self._result = rows[:limit]

    def fetchall(self):
        return self._result

    def fetchone(self):
        return self._result[0] if self._result else None


def _make_schemas_and_catalog():
    """Return (schemas, catalog): `schemas` is the canned return value for
    the monkeypatched utils.generate_table_schemas (full shape, including
    non-empty array_types/enum_values so we can assert they survive
    verbatim); `catalog` backs FakeCursor's PK-value/MAX queries only."""
    schemas = {
        "users": _full_schema_meta(
            columns={"id": "integer", "name": "text"},
            primary_key=["id"],
            array_types={"tags": "text"},
        ),
        "sessions": _full_schema_meta(
            columns={"user_id": "integer", "device_id": "integer", "started_at": "timestamp"},
            primary_key=["user_id", "device_id"],
        ),
        "accounts": _full_schema_meta(
            columns={"acct_uuid": "uuid", "balance": "numeric(10,2)"},
            primary_key=["acct_uuid"],
            enum_values={"status": ["active", "closed"]},
        ),
        "logs": _full_schema_meta(
            columns={"id": "bigint", "msg": "text"},
            primary_key=None,
        ),
    }
    catalog = {
        "users": {"pk_cols": ["id"], "rows": [(i,) for i in range(1, 51)]},
        "sessions": {"pk_cols": ["user_id", "device_id"], "rows": [(u, d) for u in range(1, 4) for d in range(1, 3)]},
        "accounts": {"pk_cols": ["acct_uuid"], "rows": [("uuid-1",), ("uuid-2",), ("uuid-3",)]},
        "logs": {"pk_cols": [], "rows": []},
    }
    return schemas, catalog


class BuildCacheTestBase(unittest.TestCase):
    def setUp(self):
        self.cache_dir = tempfile.mkdtemp(prefix="shared_cache_buildcache_test_")

    def tearDown(self):
        shutil.rmtree(self.cache_dir, ignore_errors=True)


class TestBuildCacheCallsGenerateTableSchemas(BuildCacheTestBase):
    def test_calls_generate_table_schemas_with_expected_args(self):
        schemas, catalog = _make_schemas_and_catalog()
        cursor = FakeCursor(catalog)
        table_list = ["users", "sessions", "accounts", "logs"]

        with mock.patch.object(utils, "generate_table_schemas", return_value=schemas) as m:
            build_cache(cursor, "public", table_list, 1000, self.cache_dir)

        m.assert_called_once_with(cursor, schema_name="public", manual_table_list=table_list)

    def test_schema_json_is_generate_table_schemas_output_verbatim(self):
        schemas, catalog = _make_schemas_and_catalog()
        cursor = FakeCursor(catalog)
        table_list = list(schemas.keys())

        with mock.patch.object(utils, "generate_table_schemas", return_value=schemas):
            version = build_cache(cursor, None, table_list, 1000, self.cache_dir)

        loaded = load_schema(self.cache_dir, version)
        self.assertEqual(loaded, schemas)
        self.assertEqual(loaded["users"]["array_types"], {"tags": "text"})
        self.assertEqual(loaded["accounts"]["enum_values"], {"status": ["active", "closed"]})

    def test_raises_clear_error_when_utils_unavailable(self):
        schemas, catalog = _make_schemas_and_catalog()
        cursor = FakeCursor(catalog)
        with mock.patch.object(shared_cache, "utils", None):
            with self.assertRaises(RuntimeError):
                build_cache(cursor, None, list(schemas.keys()), 1000, self.cache_dir)


class TestBuildCachePkSnapshotLogic(BuildCacheTestBase):
    def setUp(self):
        super().setUp()
        self.schemas, self.catalog = _make_schemas_and_catalog()
        cursor = FakeCursor(self.catalog)
        with mock.patch.object(utils, "generate_table_schemas", return_value=self.schemas):
            self.version = build_cache(
                cursor, None, ["users", "sessions", "accounts", "logs"], 1000, self.cache_dir
            )

    def test_current_version_points_at_new_version(self):
        self.assertEqual(current_version(self.cache_dir), self.version)

    def test_max_pk_only_populated_for_single_col_int_pk(self):
        self.assertEqual(table_max_pk(self.cache_dir, self.version, "users"), 50)
        self.assertIsNone(table_max_pk(self.cache_dir, self.version, "sessions"))
        self.assertIsNone(table_max_pk(self.cache_dir, self.version, "accounts"))
        self.assertIsNone(table_max_pk(self.cache_dir, self.version, "logs"))

    def test_users_base_is_int64_and_not_composite(self):
        base = open_pk_base(self.cache_dir, self.version, "users")
        try:
            self.assertEqual(len(base), 50)
            self.assertFalse(base.is_composite)
            self.assertIn(1, base)
            self.assertNotIn(999, base)
        finally:
            base.close()

    def test_users_int64_file_is_sorted_packed_little_endian(self):
        file_path = os.path.join(self.cache_dir, self.version, "pk", "users.i64")
        with open(file_path, "rb") as f:
            data = f.read()
        count = len(data) // 8
        values = struct.unpack("<%dq" % count, data)
        self.assertEqual(list(values), sorted(values))
        self.assertEqual(list(values), list(range(1, 51)))

    def test_sessions_base_is_composite_tuples(self):
        base = open_pk_base(self.cache_dir, self.version, "sessions")
        self.assertTrue(base.is_composite)
        self.assertEqual(len(base), 6)
        self.assertIn((1, 1), base)
        self.assertNotIn((9, 9), base)

    def test_accounts_base_is_scalar_json_not_composite(self):
        base = open_pk_base(self.cache_dir, self.version, "accounts")
        self.assertFalse(base.is_composite)
        self.assertEqual(len(base), 3)
        self.assertIn("uuid-1", base)

    def test_logs_no_pk_yields_empty_base(self):
        base = open_pk_base(self.cache_dir, self.version, "logs")
        self.assertEqual(len(base), 0)

    def test_pk_pool_maxsize_bounds_seeded_snapshot(self):
        cursor = FakeCursor(self.catalog)
        with mock.patch.object(utils, "generate_table_schemas", return_value=self.schemas):
            version = build_cache(cursor, None, ["users"], 10, self.cache_dir)
        base = open_pk_base(self.cache_dir, version, "users")
        self.assertEqual(len(base), 10)
        base.close()


class TestVersioningAndAtomicFlip(BuildCacheTestBase):
    def test_two_builds_produce_two_versions_and_current_tracks_latest(self):
        schemas, catalog = _make_schemas_and_catalog()
        cursor1 = FakeCursor(catalog)
        with mock.patch.object(utils, "generate_table_schemas", return_value=schemas):
            v1 = build_cache(cursor1, None, ["users"], 1000, self.cache_dir)
        self.assertEqual(current_version(self.cache_dir), v1)

        # Simulate a refresh: table now has more rows.
        catalog2 = _make_schemas_and_catalog()[1]
        catalog2["users"]["rows"] = [(i,) for i in range(1, 101)]
        cursor2 = FakeCursor(catalog2)
        with mock.patch.object(utils, "generate_table_schemas", return_value=schemas):
            v2 = build_cache(cursor2, None, ["users"], 1000, self.cache_dir)

        self.assertNotEqual(v1, v2)
        self.assertEqual(current_version(self.cache_dir), v2)

        # v1's files are untouched by the second build.
        base_v1 = open_pk_base(self.cache_dir, v1, "users")
        base_v2 = open_pk_base(self.cache_dir, v2, "users")
        try:
            self.assertEqual(len(base_v1), 50)
            self.assertEqual(len(base_v2), 100)
        finally:
            base_v1.close()
            base_v2.close()

    def test_current_file_contains_only_the_version_string(self):
        schemas, catalog = _make_schemas_and_catalog()
        cursor = FakeCursor(catalog)
        with mock.patch.object(utils, "generate_table_schemas", return_value=schemas):
            version = build_cache(cursor, None, ["users"], 1000, self.cache_dir)
        with open(os.path.join(self.cache_dir, "CURRENT")) as f:
            self.assertEqual(f.read().strip(), version)


if __name__ == "__main__":
    unittest.main()
