"""
Unit tests for the dynamic-worker-pool additions to utils.py:

  - the guarded `import psycopg2` (utils.py must import fine without it),
  - the monotonic PK formula (compute_monotonic_pk),
  - retry classification + bounded exponential backoff
    (classify_retry / compute_backoff_delay / execute_with_retry),
  - the worker CLI parser (build_worker_arg_parser / parse_worker_args),
  - build_insert_values' new pk_value_fn hook,
  - build_worker_governor's throttle-vs-legacy-rate_control precedence.

Stdlib unittest, no DB, no network. Where a test needs to simulate
psycopg2 being unavailable, it uses the `sys.modules['psycopg2'] = None`
trick (which makes `import psycopg2` raise ModuleNotFoundError) rather
than depending on whatever happens to be installed in this environment,
so the test is meaningful regardless of whether psycopg2 is actually
present here.
"""

import importlib
import sys
import unittest

import utils
from utils import (
    build_insert_values,
    build_worker_arg_parser,
    build_worker_governor,
    classify_retry,
    compute_backoff_delay,
    compute_monotonic_pk,
    execute_with_retry,
    get_table_max_pk,
    is_integer_pk_type,
    parse_worker_args,
)
from rate_governor import NullGovernor, RateGovernor


# --------------------------------------------------------------------------
# psycopg2-absence guard
# --------------------------------------------------------------------------

class TestPsycopg2Guard(unittest.TestCase):
    def test_utils_module_has_psycopg2_attribute(self):
        # Whatever this environment has installed, utils.py must expose a
        # module-level `psycopg2` name (real module, or None) -- never let
        # ImportError propagate out of the module body.
        self.assertTrue(hasattr(utils, "psycopg2"))

    def test_utils_imports_when_psycopg2_is_unavailable(self):
        # Force `import psycopg2` to raise ModuleNotFoundError (the
        # documented sys.modules-set-to-None trick), regardless of whether
        # psycopg2 is actually installed in this environment, then reload
        # utils.py fresh and confirm it imports successfully with
        # utils.psycopg2 set to None.
        saved_psycopg2 = sys.modules.get("psycopg2", "__absent__")
        saved_utils = sys.modules.get("utils", "__absent__")
        sys.modules["psycopg2"] = None
        try:
            if "utils" in sys.modules:
                del sys.modules["utils"]
            reloaded = importlib.import_module("utils")
            self.assertIsNone(reloaded.psycopg2)
            # Pure logic must still work with psycopg2 absent.
            self.assertEqual(reloaded.compute_monotonic_pk(0, 1, 100000, 0), 2)
            parser = reloaded.build_worker_arg_parser()
            args = parser.parse_args([])
            self.assertEqual(args.pk_stride, 100000)
        finally:
            if saved_psycopg2 == "__absent__":
                del sys.modules["psycopg2"]
            else:
                sys.modules["psycopg2"] = saved_psycopg2
            if "utils" in sys.modules:
                del sys.modules["utils"]
            if saved_utils != "__absent__":
                sys.modules["utils"] = saved_utils
            else:
                importlib.import_module("utils")


# --------------------------------------------------------------------------
# Monotonic PK formula
# --------------------------------------------------------------------------

class TestComputeMonotonicPk(unittest.TestCase):
    def test_above_max_pk(self):
        pk = compute_monotonic_pk(max_pk=1000, worker_uid=0, pk_stride=100000, counter=0)
        self.assertEqual(pk, 1001)
        self.assertGreater(pk, 1000)

    def test_monotonic_per_worker_across_counter(self):
        pks = [compute_monotonic_pk(1000, worker_uid=3, pk_stride=100000, counter=c) for c in range(5)]
        self.assertEqual(pks, sorted(pks))
        self.assertEqual(len(pks), len(set(pks)))  # no repeats
        for a, b in zip(pks, pks[1:]):
            self.assertEqual(b - a, 100000)

    def test_disjoint_across_worker_uids(self):
        # Two workers with distinct uids (both < pk_stride) never collide,
        # for any number of rows each has inserted so far.
        stride = 100000
        max_pk = 500
        worker0_pks = {compute_monotonic_pk(max_pk, 0, stride, c) for c in range(50)}
        worker1_pks = {compute_monotonic_pk(max_pk, 1, stride, c) for c in range(50)}
        worker7_pks = {compute_monotonic_pk(max_pk, 7, stride, c) for c in range(50)}
        self.assertEqual(worker0_pks & worker1_pks, set())
        self.assertEqual(worker0_pks & worker7_pks, set())
        self.assertEqual(worker1_pks & worker7_pks, set())

    def test_empty_table_treated_as_max_pk_zero(self):
        # generator.py maps an empty table's max_pk (None) to 0 before
        # calling this -- confirm 0 behaves sanely as a base.
        pk = compute_monotonic_pk(max_pk=0, worker_uid=5, pk_stride=100000, counter=0)
        self.assertEqual(pk, 6)  # 0 + 1 + worker_uid(5) + stride*counter(0)


class TestIsIntegerPkType(unittest.TestCase):
    def test_integer_types(self):
        for t in ("integer", "bigint", "smallint", "BIGINT", "  int4  "):
            self.assertTrue(is_integer_pk_type(t))

    def test_non_integer_types(self):
        for t in ("uuid", "text", "character varying", None, ""):
            self.assertFalse(is_integer_pk_type(t))


class FakeCursor:
    """Minimal cursor double for get_table_max_pk: records the executed
    SQL and returns a canned row on fetchone()."""

    def __init__(self, row):
        self._row = row
        self.executed = None

    def execute(self, sql, params=None):
        self.executed = sql

    def fetchone(self):
        return self._row


class TestGetTableMaxPk(unittest.TestCase):
    def test_returns_int_when_present(self):
        cur = FakeCursor((42,))
        result = get_table_max_pk(cur, "public", "orders", "id")
        self.assertEqual(result, 42)
        self.assertIn("public.orders", cur.executed)
        self.assertIn("MAX(id)", cur.executed)

    def test_returns_none_for_empty_table(self):
        cur = FakeCursor((None,))
        self.assertIsNone(get_table_max_pk(cur, None, "orders", "id"))

    def test_qualifies_without_schema(self):
        cur = FakeCursor((1,))
        get_table_max_pk(cur, None, "orders", "id")
        self.assertIn("FROM orders", cur.executed)
        self.assertNotIn("None.orders", cur.executed)


# --------------------------------------------------------------------------
# Retry classification + backoff
# --------------------------------------------------------------------------

class FakeDbError(Exception):
    """Stand-in for a psycopg2 error: carries a `.pgcode` attribute without
    requiring psycopg2 to be installed."""

    def __init__(self, pgcode, message="db error"):
        super().__init__(message)
        self.pgcode = pgcode


class TestClassifyRetry(unittest.TestCase):
    def test_unique_violation(self):
        self.assertEqual(classify_retry(FakeDbError("23505")), "unique_violation")

    def test_serialization_failure(self):
        self.assertEqual(classify_retry(FakeDbError("40001")), "conflict")

    def test_deadlock_detected(self):
        self.assertEqual(classify_retry(FakeDbError("40P01")), "conflict")

    def test_fatal_error_not_retryable(self):
        self.assertIsNone(classify_retry(FakeDbError("42601")))  # syntax error

    def test_no_pgcode_not_retryable(self):
        self.assertIsNone(classify_retry(ValueError("not a db error")))
        self.assertIsNone(classify_retry(FakeDbError(None)))


class TestComputeBackoffDelay(unittest.TestCase):
    def test_exponential_growth(self):
        delays = [compute_backoff_delay(a, base=0.05, cap=100.0) for a in (1, 2, 3, 4)]
        self.assertEqual(delays, [0.05, 0.1, 0.2, 0.4])

    def test_capped(self):
        self.assertEqual(compute_backoff_delay(20, base=0.05, cap=2.0), 2.0)

    def test_attempt_below_one_clamped(self):
        self.assertEqual(compute_backoff_delay(0, base=0.05, cap=2.0), 0.05)


class TestExecuteWithRetry(unittest.TestCase):
    def test_unique_violation_retries_without_sleep(self):
        attempts = {"n": 0}
        sleeps = []

        def run_once():
            attempts["n"] += 1
            if attempts["n"] < 3:
                raise FakeDbError("23505")

        rebuilds = {"n": 0}
        success = execute_with_retry(
            run_once,
            lambda: rebuilds.__setitem__("n", rebuilds["n"] + 1),
            lambda: None,
            max_retries=5,
            sleep_fn=lambda d: sleeps.append(d),
        )
        self.assertTrue(success)
        self.assertEqual(attempts["n"], 3)
        self.assertEqual(rebuilds["n"], 2)
        self.assertEqual(sleeps, [])  # unique-violation retry never sleeps

    def test_conflict_retries_with_backoff_sleep(self):
        attempts = {"n": 0}
        sleeps = []

        def run_once():
            attempts["n"] += 1
            if attempts["n"] < 3:
                raise FakeDbError("40001")

        success = execute_with_retry(
            run_once,
            lambda: None,
            lambda: None,
            max_retries=5,
            backoff_base=0.01,
            backoff_cap=1.0,
            sleep_fn=lambda d: sleeps.append(d),
        )
        self.assertTrue(success)
        self.assertEqual(attempts["n"], 3)
        self.assertEqual(sleeps, [0.01, 0.02])

    def test_deadlock_also_retried(self):
        calls = {"n": 0}

        def run_once():
            calls["n"] += 1
            if calls["n"] == 1:
                raise FakeDbError("40P01")

        success = execute_with_retry(
            run_once, lambda: None, lambda: None, max_retries=3, sleep_fn=lambda d: None
        )
        self.assertTrue(success)
        self.assertEqual(calls["n"], 2)

    def test_fatal_error_propagates_after_rollback(self):
        rollback_calls = {"n": 0}

        def run_once():
            raise FakeDbError("42601")

        with self.assertRaises(FakeDbError):
            execute_with_retry(
                run_once,
                lambda: None,
                lambda: rollback_calls.__setitem__("n", rollback_calls["n"] + 1),
                max_retries=3,
            )
        self.assertEqual(rollback_calls["n"], 1)

    def test_exhausts_max_retries_returns_false(self):
        def run_once():
            raise FakeDbError("40001")

        success = execute_with_retry(
            run_once, lambda: None, lambda: None, max_retries=2, sleep_fn=lambda d: None
        )
        self.assertFalse(success)


# --------------------------------------------------------------------------
# Worker CLI parsing
# --------------------------------------------------------------------------

class TestWorkerArgParser(unittest.TestCase):
    def test_defaults_reproduce_legacy_behavior(self):
        args = parse_worker_args([])
        self.assertIsNone(args.config)
        self.assertIsNone(args.cache_dir)
        self.assertIsNone(args.cache_version)
        self.assertIsNone(args.worker_uid)
        self.assertEqual(args.pk_stride, 100000)
        self.assertEqual(args.throttle, 0.0)

    def test_overrides(self):
        args = parse_worker_args([
            "-c", "/tmp/my.yaml",
            "--cache-dir", "/tmp/cache",
            "--cache-version", "v3",
            "--worker-uid", "7",
            "--pk-stride", "50000",
            "--throttle", "1500.5",
        ])
        self.assertEqual(args.config, "/tmp/my.yaml")
        self.assertEqual(args.cache_dir, "/tmp/cache")
        self.assertEqual(args.cache_version, "v3")
        self.assertEqual(args.worker_uid, 7)
        self.assertEqual(args.pk_stride, 50000)
        self.assertEqual(args.throttle, 1500.5)

    def test_worker_uid_and_pk_stride_are_ints(self):
        args = parse_worker_args(["--worker-uid", "3", "--pk-stride", "9"])
        self.assertIsInstance(args.worker_uid, int)
        self.assertIsInstance(args.pk_stride, int)

    def test_parser_has_expected_flags(self):
        parser = build_worker_arg_parser()
        flags = set()
        for action in parser._actions:
            flags.update(action.option_strings)
        for expected in ("--cache-dir", "--cache-version", "--worker-uid", "--pk-stride", "--throttle"):
            self.assertIn(expected, flags)


# --------------------------------------------------------------------------
# build_insert_values with pk_value_fn
# --------------------------------------------------------------------------

class TestBuildInsertValuesMonotonicPk(unittest.TestCase):
    def _schema(self):
        return {
            "orders": {
                "columns": {"id": "integer", "amount": "integer"},
                "primary_key": ["id"],
                "array_types": {},
                "enum_values": {},
                "bit_info": {},
            }
        }

    def test_pk_column_uses_pk_value_fn(self):
        schema = self._schema()
        counter = {"n": 0}

        def pk_value_fn():
            counter["n"] += 1
            return 1000 + counter["n"]

        values_str, pk_values = build_insert_values(schema, "orders", 3, pk_value_fn=pk_value_fn)
        self.assertEqual(pk_values, [1001, 1002, 1003])
        # Each row's literal PK value should appear in the VALUES fragment.
        for expected_pk in (1001, 1002, 1003):
            self.assertIn(f"'{expected_pk}'", values_str)

    def test_no_pk_value_fn_falls_back_to_random(self):
        schema = self._schema()
        values_str, pk_values = build_insert_values(schema, "orders", 2, pk_value_fn=None)
        self.assertEqual(len(pk_values), 2)
        # Random ints from generate_random_data's "integer" branch -- just
        # confirm the legacy path still produces a value per row.
        for v in pk_values:
            self.assertIsInstance(v, int)

    def test_pk_value_fn_ignored_for_composite_pk(self):
        schema = {
            "line_items": {
                "columns": {"order_id": "integer", "line_no": "integer"},
                "primary_key": ["order_id", "line_no"],
                "array_types": {},
                "enum_values": {},
                "bit_info": {},
            }
        }
        called = {"n": 0}

        def pk_value_fn():
            called["n"] += 1
            return 999999

        _, pk_values = build_insert_values(schema, "line_items", 2, pk_value_fn=pk_value_fn)
        self.assertEqual(called["n"], 0)  # never invoked for composite PKs
        self.assertEqual(len(pk_values), 2)
        for v in pk_values:
            self.assertIsInstance(v, tuple)


# --------------------------------------------------------------------------
# build_worker_governor: throttle vs legacy rate_control precedence
# --------------------------------------------------------------------------

class TestBuildWorkerGovernor(unittest.TestCase):
    def _config(self, rate_control=None):
        gen = {}
        if rate_control is not None:
            gen["rate_control"] = rate_control
        return {"generator": gen}

    def test_throttle_engages_flat_rate_governor(self):
        governor = build_worker_governor(self._config(), throttle=1500.0)
        self.assertIsInstance(governor, RateGovernor)
        self.assertEqual(governor.default_events_per_second, 1500.0)
        self.assertEqual(governor.schedule, [])

    def test_throttle_overrides_configured_rate_control(self):
        rc = {"default_events_per_second": 999, "schedule": [
            {"events_per_second": 5000, "duration_seconds": 60, "every_seconds": 300}
        ]}
        governor = build_worker_governor(self._config(rate_control=rc), throttle=42.0)
        self.assertIsInstance(governor, RateGovernor)
        self.assertEqual(governor.default_events_per_second, 42.0)
        self.assertEqual(governor.schedule, [])  # throttle bypasses the schedule too

    def test_no_throttle_no_rate_control_is_null_governor(self):
        governor = build_worker_governor(self._config(), throttle=0.0)
        self.assertIsInstance(governor, NullGovernor)

    def test_no_throttle_falls_back_to_configured_rate_control(self):
        rc = {"default_events_per_second": 777}
        governor = build_worker_governor(self._config(rate_control=rc), throttle=0.0)
        self.assertIsInstance(governor, RateGovernor)
        self.assertEqual(governor.default_events_per_second, 777)


if __name__ == "__main__":
    unittest.main()


class TestSmartDriverLoadBalance(unittest.TestCase):
    """connection.load_balance -> YB smart-driver kwargs (is_load_balance_enabled,
    _load_balance_value, get_connection_kwargs_from_config)."""

    def _cfg(self, **conn_extra):
        conn = {"database": "d", "user": "u", "password": "p",
                "host": "h", "port": 5433}
        conn.update(conn_extra)
        return {"connection": conn}

    def test_disabled_by_default(self):
        self.assertFalse(utils.is_load_balance_enabled(self._cfg()["connection"]))
        kw = utils.get_connection_kwargs_from_config(self._cfg())
        self.assertNotIn("load_balance", kw)
        self.assertNotIn("topology_keys", kw)
        self.assertEqual(kw["host"], "h")

    def test_falsey_strings_disabled(self):
        for v in ("false", "0", "no", "off", ""):
            self.assertFalse(utils.is_load_balance_enabled({"load_balance": v}), v)

    def test_bool_true_becomes_string_true(self):
        kw = utils.get_connection_kwargs_from_config(self._cfg(load_balance=True))
        self.assertEqual(kw["load_balance"], "true")

    def test_string_value_passes_through(self):
        kw = utils.get_connection_kwargs_from_config(self._cfg(load_balance="only-primary"))
        self.assertEqual(kw["load_balance"], "only-primary")

    def test_topology_keys_included_only_when_lb_on(self):
        kw = utils.get_connection_kwargs_from_config(
            self._cfg(load_balance=True, topology_keys="aws.us-west-2.us-west-2a"))
        self.assertEqual(kw["topology_keys"], "aws.us-west-2.us-west-2a")
        # topology_keys without load_balance is ignored
        kw2 = utils.get_connection_kwargs_from_config(
            self._cfg(topology_keys="aws.us-west-2.us-west-2a"))
        self.assertNotIn("topology_keys", kw2)


if __name__ == "__main__":
    unittest.main()
