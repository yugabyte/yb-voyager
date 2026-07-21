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
    compute_unique_safe_value,
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


# --------------------------------------------------------------------------
# compute_unique_safe_value: unique-safe value generation for every
# single-column unique surface (PK, UNIQUE constraint, standalone unique
# index), any type -- Part 3 of the unique-safe-value-generation design.
# --------------------------------------------------------------------------

class TestComputeUniqueSafeValue(unittest.TestCase):
    def test_integer_type_unique_across_counters(self):
        values = {
            compute_unique_safe_value("integer", worker_uid=3, pk_stride=100, counter=c, max_seed=0)
            for c in range(50)
        }
        self.assertEqual(len(values), 50)

    def test_integer_type_disjoint_across_worker_uids(self):
        # worker_uid < pk_stride keeps per-worker ranges disjoint, exactly
        # like compute_monotonic_pk.
        seen = set()
        for worker_uid in range(10):
            for counter in range(20):
                v = compute_unique_safe_value("bigint", worker_uid, pk_stride=100, counter=counter, max_seed=0)
                self.assertNotIn(v, seen)
                seen.add(v)

    def test_integer_value_always_exceeds_max_seed(self):
        for counter in range(10):
            v = compute_unique_safe_value("integer", worker_uid=2, pk_stride=50, counter=counter, max_seed=1000)
            self.assertGreater(v, 1000)

    def test_numeric_type_uses_same_formula_as_integer(self):
        v_int = compute_unique_safe_value("integer", 5, 100, 3, max_seed=10)
        v_num = compute_unique_safe_value("numeric(10,2)", 5, 100, 3, max_seed=10)
        self.assertEqual(v_int, v_num)

    def test_bigint_overflow_returns_none(self):
        # seed at the true bigint max so +1 overflows -> None (fall back to random)
        v = compute_unique_safe_value(
            "bigint", worker_uid=1, pk_stride=1, counter=1, max_seed=9_223_372_036_854_775_807
        )
        self.assertIsNone(v)

    def test_varchar_no_char_max_returns_plain_worker_counter_encoding(self):
        v = compute_unique_safe_value("text", worker_uid=7, pk_stride=100, counter=3)
        self.assertEqual(v, "u7_3")

    def test_varchar_result_length_within_char_max(self):
        for counter in range(5):
            v = compute_unique_safe_value(
                "character varying(20)", worker_uid=12345, pk_stride=100, counter=counter, char_max=20
            )
            self.assertIsNotNone(v)
            self.assertLessEqual(len(v), 20)

    def test_varchar_falls_back_to_base36_when_plain_encoding_too_long(self):
        # f"u999_123456" (len 12) exceeds char_max=10, forcing the compact
        # base36-encoded path.
        v = compute_unique_safe_value("varchar", worker_uid=999, pk_stride=100, counter=123456, char_max=10)
        self.assertIsNotNone(v)
        self.assertLessEqual(len(v), 10)
        self.assertTrue(v.startswith("u"))

    def test_varchar_unique_across_worker_uids_and_counters_with_tight_char_max(self):
        seen = set()
        for worker_uid in range(5):
            for counter in range(5):
                v = compute_unique_safe_value("varchar", worker_uid, pk_stride=100, counter=counter, char_max=8)
                self.assertIsNotNone(v)
                self.assertNotIn(v, seen)
                seen.add(v)

    def test_varchar_char_max_too_small_returns_none(self):
        v = compute_unique_safe_value("varchar", worker_uid=1, pk_stride=100, counter=1, char_max=1)
        self.assertIsNone(v)

    def test_uuid_deterministic(self):
        v1 = compute_unique_safe_value("uuid", worker_uid=4, pk_stride=100, counter=9)
        v2 = compute_unique_safe_value("uuid", worker_uid=4, pk_stride=100, counter=9)
        self.assertEqual(v1, v2)

    def test_uuid_unique_across_counters_and_worker_uids(self):
        seen = set()
        for worker_uid in range(5):
            for counter in range(5):
                v = compute_unique_safe_value("uuid", worker_uid, pk_stride=100, counter=counter)
                self.assertNotIn(v, seen)
                seen.add(v)

    def test_unsupported_type_returns_none(self):
        self.assertIsNone(compute_unique_safe_value("boolean", worker_uid=1, pk_stride=1, counter=1))
        self.assertIsNone(compute_unique_safe_value("date", worker_uid=1, pk_stride=1, counter=1))
        self.assertIsNone(compute_unique_safe_value(None, worker_uid=1, pk_stride=1, counter=1))


# --------------------------------------------------------------------------
# build_insert_values' new unique_value_fns hook -- Part 4 of the
# unique-safe-value-generation design.
# --------------------------------------------------------------------------

class TestBuildInsertValuesUniqueValueFns(unittest.TestCase):
    def _schema(self):
        return {
            "widgets": {
                "columns": {"id": "integer", "code": "character varying(20)", "amount": "integer"},
                "primary_key": ["id"],
                "array_types": {},
                "enum_values": {},
                "bit_info": {},
            }
        }

    def test_unique_value_fn_used_for_configured_column(self):
        schema = self._schema()
        calls = {"n": 0}

        def code_fn():
            calls["n"] += 1
            return f"code-{calls['n']}"

        values_str, _ = build_insert_values(schema, "widgets", 2, unique_value_fns={"code": code_fn})
        self.assertEqual(calls["n"], 2)
        self.assertIn("'code-1'", values_str)
        self.assertIn("'code-2'", values_str)

    def test_column_override_takes_priority_over_unique_value_fn(self):
        schema = self._schema()
        called = {"n": 0}

        def code_fn():
            called["n"] += 1
            return "should-not-be-used"

        overrides = {"widgets": {"code": {"type": "choice", "values": ["fixed-value"]}}}
        values_str, _ = build_insert_values(
            schema, "widgets", 2, column_overrides=overrides, unique_value_fns={"code": code_fn},
        )
        self.assertEqual(called["n"], 0)
        self.assertEqual(values_str.count("'fixed-value'"), 2)

    def test_unique_value_fn_returning_none_falls_back_to_random(self):
        schema = self._schema()

        def code_fn():
            return None

        values_str, _ = build_insert_values(schema, "widgets", 1, unique_value_fns={"code": code_fn})
        # Falls through to normal random varchar generation -- just confirm
        # a value was produced for every column (no crash, no stray NULLs
        # for a NOT-NULL-shaped varchar column).
        self.assertEqual(values_str.count("'"), 6)  # 3 columns * 2 quotes each

    def test_unique_value_fn_takes_priority_over_pk_value_fn_for_same_column(self):
        schema = self._schema()
        pk_calls = {"n": 0}

        def pk_fn():
            pk_calls["n"] += 1
            return 999999

        def id_unique_fn():
            return 42

        _, pk_values = build_insert_values(
            schema, "widgets", 1, pk_value_fn=pk_fn, unique_value_fns={"id": id_unique_fn},
        )
        self.assertEqual(pk_calls["n"], 0)  # unique_value_fns wins; pk_value_fn never invoked
        self.assertEqual(pk_values, [42])

    def test_pk_value_fn_still_works_when_unique_value_fns_is_none(self):
        schema = self._schema()
        counter = {"n": 0}

        def pk_fn():
            counter["n"] += 1
            return 500 + counter["n"]

        _, pk_values = build_insert_values(schema, "widgets", 2, pk_value_fn=pk_fn, unique_value_fns=None)
        self.assertEqual(pk_values, [501, 502])


class TestUniqueSafeIntegerWidthOverflow(unittest.TestCase):
    """compute_unique_safe_value must fall back (None) once the monotonic
    value would exceed the column type's own integer ceiling, not just bigint's."""

    def test_smallint_falls_back_past_int2_max(self):
        # small counter fits; a large one blows past 32767 -> None
        self.assertIsInstance(
            utils.compute_unique_safe_value("smallint", 0, 100000, 0, max_seed=0), int)
        self.assertIsNone(
            utils.compute_unique_safe_value("smallint", 0, 100000, 1, max_seed=0))

    def test_integer_falls_back_past_int4_max(self):
        # 100000 * counter crosses 2_147_483_647 around counter ~21475
        self.assertIsInstance(
            utils.compute_unique_safe_value("integer", 0, 100000, 100, max_seed=0), int)
        self.assertIsNone(
            utils.compute_unique_safe_value("integer", 0, 100000, 30000, max_seed=0))

    def test_bigint_still_has_wide_headroom(self):
        # a value that overflows int4 is fine for bigint
        v = utils.compute_unique_safe_value("bigint", 0, 100000, 30000, max_seed=0)
        self.assertIsInstance(v, int)
        self.assertGreater(v, 2147483647)

    def test_returned_int_values_never_exceed_type_max(self):
        for dtype, ceiling in (("smallint", 32767), ("integer", 2147483647)):
            for counter in range(0, 40000, 137):
                v = utils.compute_unique_safe_value(dtype, 3, 100000, counter, max_seed=5)
                if v is not None:
                    self.assertLessEqual(v, ceiling, (dtype, counter))


if __name__ == "__main__":
    unittest.main()
