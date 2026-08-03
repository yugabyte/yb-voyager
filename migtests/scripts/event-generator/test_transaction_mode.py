"""
Unit tests for transaction_mode.py (the opt-in, config-gated "transaction
mode" for the event generator) and utils.validate_transaction_mode.

Stdlib unittest, no DB, no network -- everything here uses a fake
cursor/connection double (FakeConnCursor below) or the real PkPool (which
is itself pure in-memory, no DB). Table/column names used in fixtures are
deliberately generic placeholders (hot_a/hot_b/other_x/other_y), not any
real schema.

Covers:
  (a) disabled -> the legacy single-op path is used (tested via the gate
      function is_transaction_mode_enabled; generator.py itself can't be
      imported in a unit test since it opens a real DB connection at
      import time -- see class TestGateMatchesLegacyDispatch docstring).
  (b) enabled -> per-txn statement count within [min, max].
  (c) hot/other split within their configured ranges, H + O == T.
  (d) hot statements only ever hit hot_tables; other statements never do.
  (e) savepoint pair count within range; every SAVEPOINT has a matching
      RELEASE SAVEPOINT, correctly nested inside BEGIN...COMMIT.
  (f) every statement is single-row by default (rows_per_statement absent).
  (g) rows_per_statement (optional): UPDATE/DELETE can batch k rows into one
      statement (pk IN (<k ids>)), events summed as rows not statements,
      DELETE removes all k sampled ids from the pool, INSERT stays
      single-row regardless, and validate_transaction_mode's acceptance/
      rejection of the new config key.
"""

import random
import re
import unittest

from pk_pool import PkPool
from utils import validate_transaction_mode
from transaction_mode import (
    build_transaction_plan,
    choose_savepoint_ranges,
    execute_single_statement,
    is_transaction_mode_enabled,
    resolve_txn_counts,
    run_transaction,
)


# --------------------------------------------------------------------------
# Fixtures
# --------------------------------------------------------------------------

# Generic placeholder schema -- two "hot" tables, two "other" tables. Never
# real table names.
TABLE_SCHEMAS = {
    "hot_a": {
        "columns": {"id": "integer", "val": "text"},
        "primary_key": ["id"],
        "unique_columns": [],
    },
    "hot_b": {
        "columns": {"id": "integer", "val": "text"},
        "primary_key": ["id"],
        "unique_columns": [],
    },
    "other_x": {
        "columns": {"id": "integer", "val": "text"},
        "primary_key": ["id"],
        "unique_columns": [],
    },
    "other_y": {
        "columns": {"id": "integer", "val": "text"},
        "primary_key": ["id"],
        "unique_columns": [],
    },
}

HOT_TABLES = ["hot_a", "hot_b"]
OTHER_TABLE_WEIGHTS = {"other_x": 1, "other_y": 1}


def make_pools():
    """Fresh PkPool per table, pre-seeded so UPDATE/DELETE can target real
    ids via the pool path (not just the scan-based fallback)."""
    pools = {}
    for table in TABLE_SCHEMAS:
        pool = PkPool()
        pool.add_many(range(100))
        pools[table] = pool
    return pools


def make_tm_cfg(**overrides):
    """A representative enabled transaction_mode config. Ranges are chosen
    to be "compatible" (see resolve_txn_counts docstring): statements_per_txn
    fully covers hot+other's combined min/max, so H + O == T always lands
    exactly on the sampled T."""
    cfg = {
        "enabled": True,
        "hot_tables": list(HOT_TABLES),
        "statements_per_txn": {"min": 2, "max": 10},
        "hot_statements_per_txn": {"min": 1, "max": 5},
        "other_statements_per_txn": {"min": 1, "max": 5},
        "savepoint_pairs_per_txn": {"min": 0, "max": 3},
        "hot_op_weights": {"INSERT": 1, "UPDATE": 1, "DELETE": 1},
        "other_op_weights": {"INSERT": 1, "UPDATE": 1, "DELETE": 1},
    }
    cfg.update(overrides)
    return cfg


class FakeConnCursor:
    """A single object that plays both "cursor" (execute) and "conn"
    (commit/rollback) roles, appending every call to one shared ordered
    `calls` list -- so SAVEPOINT/RELEASE nesting relative to BEGIN/COMMIT
    can be asserted precisely on one linear trace."""

    def __init__(self, fail_on_call_containing=None):
        self.calls = []
        self.rowcount = 0
        self.commits = 0
        self.rollbacks = 0
        self._fail_on = fail_on_call_containing

    # cursor-like
    def execute(self, sql, params=None):
        if self._fail_on and self._fail_on in sql:
            raise RuntimeError(f"simulated failure on: {sql}")
        self.calls.append(sql.strip())
        upper = sql.strip().upper()
        if upper.startswith("INSERT"):
            self.rowcount = 1
        elif upper.startswith(("UPDATE", "DELETE")):
            # Mirror build_pk_in_condition's "... IN (%s, %s, %s)" shape:
            # rows "affected" equal the number of explicit ids targeted, so
            # a k-row rows_per_statement batch reports rowcount k, same as a
            # real DB would for a pk-list WHERE clause. The scan-based
            # TABLESAMPLE/random() fallback (build_sampling_condition)
            # doesn't produce that pure-%s-list shape, so it falls back to
            # 1 -- no test here asserts an exact rowcount on that path.
            m = re.search(r"IN \(((?:%s,\s*)*%s)\)", sql)
            self.rowcount = m.group(1).count("%s") if m else 1
        else:
            self.rowcount = 0

    # conn-like
    def commit(self):
        self.calls.append("COMMIT")
        self.commits += 1

    def rollback(self):
        self.calls.append("ROLLBACK")
        self.rollbacks += 1


def run_txn_with_fake(plan, pools=None, fail_on_call_containing=None, rows_per_statement=(1, 1)):
    fc = FakeConnCursor(fail_on_call_containing=fail_on_call_containing)
    events = run_transaction(
        fc,
        fc,
        plan,
        TABLE_SCHEMAS,
        pools if pools is not None else make_pools(),
        "POSTGRES",
        {},
        {},
        0,
        pk_value_fn_for_table=lambda t: None,
        unique_value_fns_for_table=lambda t: None,
        rows_per_statement=rows_per_statement,
    )
    return fc, events


# --------------------------------------------------------------------------
# (a) disabled -> legacy path gate
# --------------------------------------------------------------------------

class TestGateMatchesLegacyDispatch(unittest.TestCase):
    """generator.py picks its main-loop branch with:

        TRANSACTION_MODE_ENABLED = is_transaction_mode_enabled(GEN.get("transaction_mode"))
        ...
        if TRANSACTION_MODE_ENABLED: <new txn path> else: <legacy single-op path, untouched>

    generator.py itself can't be imported here (it opens a live psycopg2
    connection at module import time, by design -- see its own module
    docstring/comments), so this test covers the actual decision function
    that gate uses: absent config or enabled=false must route to the
    untouched legacy branch, exactly as it does today.
    """

    def test_absent_block_is_disabled(self):
        self.assertFalse(is_transaction_mode_enabled(None))
        self.assertFalse(is_transaction_mode_enabled({}))

    def test_enabled_false_is_disabled(self):
        self.assertFalse(is_transaction_mode_enabled({"enabled": False, "hot_tables": ["x"]}))

    def test_enabled_true_is_enabled(self):
        self.assertTrue(is_transaction_mode_enabled({"enabled": True, "hot_tables": ["x"]}))

    def test_missing_enabled_key_defaults_disabled(self):
        # A stray transaction_mode block with no 'enabled' key at all must
        # still resolve to the legacy path -- default OFF.
        self.assertFalse(is_transaction_mode_enabled({"hot_tables": ["x"]}))


# --------------------------------------------------------------------------
# (b) + (c) statement counts / hot-other split
# --------------------------------------------------------------------------

class TestResolveTxnCounts(unittest.TestCase):
    def test_counts_within_range_and_sum_matches_total(self):
        cfg = make_tm_cfg()
        for seed in range(300):
            rng = random.Random(seed)
            T, H, O = resolve_txn_counts(cfg, rng)
            self.assertEqual(H + O, T)
            self.assertGreaterEqual(H, cfg["hot_statements_per_txn"]["min"])
            self.assertLessEqual(H, cfg["hot_statements_per_txn"]["max"])
            self.assertGreaterEqual(O, cfg["other_statements_per_txn"]["min"])
            self.assertLessEqual(O, cfg["other_statements_per_txn"]["max"])
            self.assertGreaterEqual(T, cfg["statements_per_txn"]["min"])
            self.assertLessEqual(T, cfg["statements_per_txn"]["max"])

    def test_fixed_ranges_produce_exact_total(self):
        # min == max everywhere: fully deterministic, easiest to hand-verify.
        cfg = make_tm_cfg(
            statements_per_txn={"min": 6, "max": 6},
            hot_statements_per_txn={"min": 2, "max": 2},
            other_statements_per_txn={"min": 4, "max": 4},
        )
        T, H, O = resolve_txn_counts(cfg, random.Random(1))
        self.assertEqual((T, H, O), (6, 2, 4))

    def test_reconciliation_grows_when_hot_and_other_undershoot(self):
        # H, O sampled at their minimums (1+1=2) can't reach T=10 without
        # growing -- O grows first (to its max 5), then H (to its max 5).
        cfg = make_tm_cfg(
            statements_per_txn={"min": 10, "max": 10},
            hot_statements_per_txn={"min": 1, "max": 5},
            other_statements_per_txn={"min": 1, "max": 5},
        )

        class FixedRng:
            """Deterministic stand-in that always returns the range's
            minimum, forcing the undershoot-then-grow branch."""

            def randint(self, a, b):
                return a

        T, H, O = resolve_txn_counts(cfg, FixedRng())
        self.assertEqual(H + O, T)
        self.assertEqual(T, 10)
        self.assertEqual((H, O), (5, 5))

    def test_reconciliation_shrinks_when_hot_and_other_overshoot(self):
        # H, O sampled at their maximums (5+5=10) must shrink to T=2 --
        # O shrinks first (to its min 1), then H (to its min 1).
        cfg = make_tm_cfg(
            statements_per_txn={"min": 2, "max": 2},
            hot_statements_per_txn={"min": 1, "max": 5},
            other_statements_per_txn={"min": 1, "max": 5},
        )

        class FixedRng:
            def randint(self, a, b):
                return b

        T, H, O = resolve_txn_counts(cfg, FixedRng())
        self.assertEqual(H + O, T)
        self.assertEqual(T, 2)
        self.assertEqual((H, O), (1, 1))


class TestBuildTransactionPlanHotOtherSplit(unittest.TestCase):
    def test_statement_count_and_split_within_ranges(self):
        cfg = make_tm_cfg()
        for seed in range(200):
            rng = random.Random(seed)
            plan = build_transaction_plan(cfg, HOT_TABLES, OTHER_TABLE_WEIGHTS, rng)
            statements = plan["statements"]
            hot_count = sum(1 for s in statements if s["hot"])
            other_count = sum(1 for s in statements if not s["hot"])

            self.assertEqual(hot_count + other_count, len(statements))
            self.assertGreaterEqual(len(statements), cfg["statements_per_txn"]["min"])
            self.assertLessEqual(len(statements), cfg["statements_per_txn"]["max"])
            self.assertGreaterEqual(hot_count, cfg["hot_statements_per_txn"]["min"])
            self.assertLessEqual(hot_count, cfg["hot_statements_per_txn"]["max"])
            self.assertGreaterEqual(other_count, cfg["other_statements_per_txn"]["min"])
            self.assertLessEqual(other_count, cfg["other_statements_per_txn"]["max"])

    def test_hot_statements_only_target_hot_tables_and_vice_versa(self):
        cfg = make_tm_cfg()
        hot_set = set(HOT_TABLES)
        for seed in range(200):
            rng = random.Random(seed)
            plan = build_transaction_plan(cfg, HOT_TABLES, OTHER_TABLE_WEIGHTS, rng)
            for stmt in plan["statements"]:
                if stmt["hot"]:
                    self.assertIn(stmt["table"], hot_set)
                else:
                    self.assertNotIn(stmt["table"], hot_set)
                    self.assertIn(stmt["table"], OTHER_TABLE_WEIGHTS)

    def test_operations_come_from_configured_weights(self):
        cfg = make_tm_cfg(
            hot_op_weights={"INSERT": 1, "UPDATE": 0, "DELETE": 0},
            other_op_weights={"INSERT": 0, "UPDATE": 0, "DELETE": 1},
        )
        rng = random.Random(42)
        plan = build_transaction_plan(cfg, HOT_TABLES, OTHER_TABLE_WEIGHTS, rng)
        for stmt in plan["statements"]:
            if stmt["hot"]:
                self.assertEqual(stmt["operation"], "INSERT")
            else:
                self.assertEqual(stmt["operation"], "DELETE")

    def test_empty_other_pool_forces_other_count_to_zero(self):
        # Every known table is 'hot' -- nothing non-hot left to target.
        cfg = make_tm_cfg()
        rng = random.Random(7)
        plan = build_transaction_plan(cfg, HOT_TABLES, {}, rng)
        self.assertTrue(all(s["hot"] for s in plan["statements"]))


# --------------------------------------------------------------------------
# (e) savepoint ranges
# --------------------------------------------------------------------------

class TestChooseSavepointRanges(unittest.TestCase):
    def test_empty_when_no_statements(self):
        self.assertEqual(choose_savepoint_ranges(0, {"min": 1, "max": 3}, random.Random(1)), [])

    def test_count_within_configured_range(self):
        sp_cfg = {"min": 1, "max": 4}
        for seed in range(300):
            ranges = choose_savepoint_ranges(8, sp_cfg, random.Random(seed))
            self.assertGreaterEqual(len(ranges), sp_cfg["min"])
            self.assertLessEqual(len(ranges), sp_cfg["max"])

    def test_ranges_are_contiguous_non_overlapping_and_in_bounds(self):
        total = 10
        sp_cfg = {"min": 3, "max": 3}
        for seed in range(100):
            ranges = choose_savepoint_ranges(total, sp_cfg, random.Random(seed))
            self.assertEqual(len(ranges), 3)
            ordered = sorted(ranges)
            prev_end = -1
            for start, end in ordered:
                self.assertGreaterEqual(start, 0)
                self.assertLess(end, total)
                self.assertLessEqual(start, end)  # contiguous run, length >= 1
                self.assertGreater(start, prev_end)  # non-overlapping
                prev_end = end

    def test_count_clamped_to_available_statements(self):
        # Only 2 statements available: can't place 5 non-overlapping pairs.
        ranges = choose_savepoint_ranges(2, {"min": 5, "max": 5}, random.Random(3))
        self.assertLessEqual(len(ranges), 2)


# --------------------------------------------------------------------------
# (e) + (f) run_transaction: SQL sequencing, single-row statements
# --------------------------------------------------------------------------

class TestRunTransactionSequencing(unittest.TestCase):
    def test_begin_first_commit_last_and_savepoints_nested_inside(self):
        plan = {
            "statements": [
                {"table": "hot_a", "operation": "INSERT", "hot": True},
                {"table": "other_x", "operation": "UPDATE", "hot": False},
                {"table": "hot_b", "operation": "DELETE", "hot": True},
            ],
            "savepoint_ranges": [(0, 1), (2, 2)],
        }
        fc, committed = run_txn_with_fake(plan)

        self.assertEqual(committed, 3)
        self.assertEqual(fc.calls[0], "BEGIN")
        self.assertEqual(fc.calls[-1], "COMMIT")
        self.assertEqual(fc.commits, 1)
        self.assertEqual(fc.rollbacks, 0)

        sp1_open = fc.calls.index("SAVEPOINT sp_1")
        sp1_close = fc.calls.index("RELEASE SAVEPOINT sp_1")
        sp2_open = fc.calls.index("SAVEPOINT sp_2")
        sp2_close = fc.calls.index("RELEASE SAVEPOINT sp_2")

        begin_idx = 0
        commit_idx = len(fc.calls) - 1
        for idx in (sp1_open, sp1_close, sp2_open, sp2_close):
            self.assertGreater(idx, begin_idx)
            self.assertLess(idx, commit_idx)

        self.assertLess(sp1_open, sp1_close)
        self.assertLess(sp2_open, sp2_close)
        # sp_1 wraps statements 0-1 (INSERT then UPDATE), so its RELEASE
        # comes after the UPDATE and before sp_2 even opens.
        self.assertLess(sp1_close, sp2_open)

    def test_every_savepoint_has_a_matching_release(self):
        cfg = make_tm_cfg()
        for seed in range(50):
            rng = random.Random(seed)
            plan = build_transaction_plan(cfg, HOT_TABLES, OTHER_TABLE_WEIGHTS, rng)
            fc, _ = run_txn_with_fake(plan)

            opens = [c for c in fc.calls if c.startswith("SAVEPOINT ")]
            closes = [c for c in fc.calls if c.startswith("RELEASE SAVEPOINT ")]
            self.assertEqual(len(opens), len(plan["savepoint_ranges"]))
            open_names = {c.split(" ", 1)[1] for c in opens}
            close_names = {c.split(" ", 2)[2] for c in closes}
            self.assertEqual(open_names, close_names)
            for name in open_names:
                self.assertLess(
                    fc.calls.index(f"SAVEPOINT {name}"),
                    fc.calls.index(f"RELEASE SAVEPOINT {name}"),
                )

    def test_no_savepoints_still_wraps_begin_commit(self):
        plan = {
            "statements": [{"table": "hot_a", "operation": "INSERT", "hot": True}],
            "savepoint_ranges": [],
        }
        fc, committed = run_txn_with_fake(plan)
        self.assertEqual(committed, 1)
        self.assertEqual(fc.calls, ["BEGIN", fc.calls[1], "COMMIT"])
        self.assertTrue(fc.calls[1].upper().startswith("INSERT"))

    def test_insert_statements_are_single_row(self):
        plan = {
            "statements": [{"table": "hot_a", "operation": "INSERT", "hot": True}],
            "savepoint_ranges": [],
        }
        fc, _ = run_txn_with_fake(plan)
        insert_sql = next(c for c in fc.calls if c.upper().startswith("INSERT"))
        # A single-row VALUES list has exactly one "(...)" group -- no
        # "), (" row separator.
        self.assertNotIn("), (", insert_sql)

    def test_update_and_delete_target_exactly_one_pk(self):
        plan = {
            "statements": [
                {"table": "other_x", "operation": "UPDATE", "hot": False},
                {"table": "other_y", "operation": "DELETE", "hot": False},
            ],
            "savepoint_ranges": [],
        }
        fc, _ = run_txn_with_fake(plan)
        update_sql = next(c for c in fc.calls if c.upper().startswith("UPDATE"))
        delete_sql = next(c for c in fc.calls if c.upper().startswith("DELETE"))
        # PkPool-backed targeting produces "id IN (%s)" for exactly 1 id --
        # never "IN (%s, %s...)".
        self.assertIn("IN (%s)", update_sql)
        self.assertIn("IN (%s)", delete_sql)

    def test_mid_transaction_error_never_commits(self):
        plan = {
            "statements": [
                {"table": "hot_a", "operation": "INSERT", "hot": True},
                {"table": "hot_b", "operation": "INSERT", "hot": True},
            ],
            "savepoint_ranges": [],
        }
        with self.assertRaises(RuntimeError):
            run_txn_with_fake(plan, fail_on_call_containing='INSERT INTO "hot_b"')
        # No leaked FakeConnCursor to inspect after the raise (it's local to
        # the helper), so re-run manually to inspect calls up to the error.
        fc = FakeConnCursor(fail_on_call_containing='INSERT INTO "hot_b"')
        with self.assertRaises(RuntimeError):
            run_transaction(
                fc, fc, plan, TABLE_SCHEMAS, make_pools(), "POSTGRES", {}, {}, 0,
                pk_value_fn_for_table=lambda t: None,
                unique_value_fns_for_table=lambda t: None,
            )
        self.assertNotIn("COMMIT", fc.calls)
        self.assertEqual(fc.commits, 0)


class TestExecuteSingleStatementIsSingleRow(unittest.TestCase):
    def test_insert_produces_exactly_one_row(self):
        pools = make_pools()
        fc = FakeConnCursor()
        rowcount, (add_ids, remove_ids) = execute_single_statement(
            fc, TABLE_SCHEMAS, pools, "POSTGRES", {}, {}, 0,
            lambda t: None, lambda t: None,
            "hot_a", "INSERT",
        )
        self.assertEqual(len(add_ids), 1)
        self.assertEqual(remove_ids, [])
        insert_sql = fc.calls[-1]
        self.assertNotIn("), (", insert_sql)

    def test_delete_removes_exactly_one_id(self):
        pools = make_pools()
        fc = FakeConnCursor()
        rowcount, (add_ids, remove_ids) = execute_single_statement(
            fc, TABLE_SCHEMAS, pools, "POSTGRES", {}, {}, 0,
            lambda t: None, lambda t: None,
            "other_x", "DELETE",
        )
        self.assertEqual(add_ids, [])
        self.assertEqual(len(remove_ids), 1)
        self.assertIn("IN (%s)", fc.calls[-1])

    def test_update_skips_pk_less_table(self):
        schemas = {"no_pk": {"columns": {"a": "text"}, "primary_key": None, "unique_columns": []}}
        fc = FakeConnCursor()
        rowcount, (add_ids, remove_ids) = execute_single_statement(
            fc, schemas, {}, "POSTGRES", {}, {}, 0,
            lambda t: None, lambda t: None,
            "no_pk", "UPDATE",
        )
        self.assertEqual((rowcount, add_ids, remove_ids), (0, [], []))
        self.assertEqual(fc.calls, [])  # nothing executed


# --------------------------------------------------------------------------
# (g) rows_per_statement: optional k-row batching for UPDATE/DELETE
# --------------------------------------------------------------------------

class TestRowsPerStatementBatching(unittest.TestCase):
    """rows_per_statement (see event-generator.yaml's transaction_mode
    template) lets a single UPDATE/DELETE statement affect k rows instead of
    1, amortizing the round trip. INSERT is never affected. Absent config
    (the default (1, 1) execute_single_statement/run_transaction fall back
    to) must reproduce today's single-row behavior byte-for-byte."""

    def test_absent_rows_per_statement_is_single_row(self):
        # No rows_per_statement arg at all -- exercises the (1, 1) default.
        pools = make_pools()
        fc = FakeConnCursor()
        rowcount, (add_ids, remove_ids) = execute_single_statement(
            fc, TABLE_SCHEMAS, pools, "POSTGRES", {}, {}, 0,
            lambda t: None, lambda t: None,
            "other_x", "UPDATE",
        )
        self.assertEqual(rowcount, 1)
        self.assertIn("IN (%s)", fc.calls[-1])

    def test_update_batches_to_k_ids_when_configured(self):
        pools = make_pools()
        fc = FakeConnCursor()
        rowcount, (add_ids, remove_ids) = execute_single_statement(
            fc, TABLE_SCHEMAS, pools, "POSTGRES", {}, {}, 0,
            lambda t: None, lambda t: None,
            "other_x", "UPDATE", (3, 3),
        )
        update_sql = fc.calls[-1]
        self.assertIn("IN (%s, %s, %s)", update_sql)
        self.assertEqual(rowcount, 3)
        # UPDATE never removes ids from the pool -- rows stay live.
        self.assertEqual((add_ids, remove_ids), ([], []))

    def test_delete_batches_to_k_ids_and_removes_all_k_from_pool(self):
        pools = make_pools()
        pool = pools["other_y"]
        before_len = len(pool)

        fc = FakeConnCursor()
        rowcount, (add_ids, remove_ids) = execute_single_statement(
            fc, TABLE_SCHEMAS, pools, "POSTGRES", {}, {}, 0,
            lambda t: None, lambda t: None,
            "other_y", "DELETE", (3, 3),
        )
        delete_sql = fc.calls[-1]
        self.assertIn("IN (%s, %s, %s)", delete_sql)
        self.assertEqual(rowcount, 3)
        self.assertEqual(len(remove_ids), 3)
        self.assertEqual(add_ids, [])
        # execute_single_statement itself never touches the pool (bookkeeping
        # is deferred to run_transaction, after commit) -- drive it through
        # run_transaction to confirm ALL k sampled ids actually disappear.
        plan = {
            "statements": [{"table": "other_y", "operation": "DELETE", "hot": False}],
            "savepoint_ranges": [],
        }
        _fc2, events = run_txn_with_fake(plan, pools=pools, rows_per_statement=(3, 3))
        self.assertEqual(events, 3)
        self.assertEqual(len(pool), before_len - 3)

    def test_insert_stays_single_row_regardless_of_rows_per_statement(self):
        pools = make_pools()
        fc = FakeConnCursor()
        rowcount, (add_ids, remove_ids) = execute_single_statement(
            fc, TABLE_SCHEMAS, pools, "POSTGRES", {}, {}, 0,
            lambda t: None, lambda t: None,
            "hot_a", "INSERT", (5, 5),
        )
        self.assertEqual(rowcount, 1)
        self.assertEqual(len(add_ids), 1)
        self.assertEqual(remove_ids, [])
        insert_sql = fc.calls[-1]
        self.assertNotIn("), (", insert_sql)  # still exactly one VALUES row

    def test_run_transaction_events_are_summed_rowcounts_not_statement_count(self):
        plan = {
            "statements": [
                {"table": "hot_a", "operation": "INSERT", "hot": True},
                {"table": "other_x", "operation": "UPDATE", "hot": False},
                {"table": "other_y", "operation": "DELETE", "hot": False},
            ],
            "savepoint_ranges": [],
        }
        _fc, events = run_txn_with_fake(plan, rows_per_statement=(4, 4))
        # INSERT always contributes 1; UPDATE and DELETE each contribute 4.
        self.assertEqual(events, 1 + 4 + 4)

    def test_default_rows_per_statement_events_equal_statement_count(self):
        # Back-compat: absent/default rows_per_statement means events ==
        # statement count exactly, as before this feature existed.
        plan = {
            "statements": [
                {"table": "hot_a", "operation": "INSERT", "hot": True},
                {"table": "other_x", "operation": "UPDATE", "hot": False},
                {"table": "other_y", "operation": "DELETE", "hot": False},
            ],
            "savepoint_ranges": [],
        }
        fc, events = run_txn_with_fake(plan)
        self.assertEqual(events, len(plan["statements"]))
        for sql in fc.calls:
            if sql.upper().startswith(("UPDATE", "DELETE")):
                self.assertIn("IN (%s)", sql)


# --------------------------------------------------------------------------
# Pool bookkeeping is deferred until after commit
# --------------------------------------------------------------------------

class TestPoolBookkeepingAfterCommit(unittest.TestCase):
    def test_insert_adds_to_pool_only_after_commit(self):
        pools = make_pools()
        plan = {
            "statements": [{"table": "hot_a", "operation": "INSERT", "hot": True}],
            "savepoint_ranges": [],
        }
        before_len = len(pools["hot_a"])
        run_txn_with_fake(plan, pools=pools)
        self.assertEqual(len(pools["hot_a"]), before_len + 1)

    def test_failed_transaction_never_touches_pool(self):
        pools = make_pools()
        plan = {
            "statements": [{"table": "hot_a", "operation": "INSERT", "hot": True}],
            "savepoint_ranges": [],
        }
        before_len = len(pools["hot_a"])
        fc = FakeConnCursor(fail_on_call_containing='INSERT INTO "hot_a"')
        with self.assertRaises(RuntimeError):
            run_transaction(
                fc, fc, plan, TABLE_SCHEMAS, pools, "POSTGRES", {}, {}, 0,
                pk_value_fn_for_table=lambda t: None,
                unique_value_fns_for_table=lambda t: None,
            )
        self.assertEqual(len(pools["hot_a"]), before_len)


# --------------------------------------------------------------------------
# validate_transaction_mode
# --------------------------------------------------------------------------

class TestReservedWordColumnIsQuoted(unittest.TestCase):
    """A column literally named 'primary' (a SQL reserved word) used to
    trigger 'syntax error at or near "primary"' because
    execute_single_statement built raw, unquoted SQL. quote_ident (see
    utils.py) fixes this by double-quoting every identifier at the point
    it's written into a SQL string -- never on a name still used as a dict
    key or builder input. Here 'primary' is also the table's PK, so it
    shows up in INSERT's column list AND in UPDATE/DELETE's WHERE clause
    (via build_pk_in_condition)."""

    SCHEMA = {
        "widgets": {
            "columns": {"primary": "integer", "val": "text"},
            "primary_key": ["primary"],
            "unique_columns": [],
        }
    }

    def _pools(self):
        pool = PkPool()
        pool.add_many(range(100))
        return {"widgets": pool}

    def _assert_quoted_not_bare(self, sql):
        self.assertIn('"primary"', sql)
        self.assertIn('"widgets"', sql)
        # No occurrence of the bare (unquoted) identifier once every quoted
        # occurrence is stripped out.
        unquoted = sql.replace('"primary"', "")
        self.assertNotIn("primary", unquoted)

    def test_insert_quotes_reserved_word_column(self):
        fc = FakeConnCursor()
        execute_single_statement(
            fc, self.SCHEMA, self._pools(), "POSTGRES", {}, {}, 0,
            lambda t: None, lambda t: None,
            "widgets", "INSERT",
        )
        sql = fc.calls[-1]
        self.assertTrue(sql.upper().startswith("INSERT"))
        self._assert_quoted_not_bare(sql)

    def test_update_quotes_reserved_word_pk_in_where(self):
        fc = FakeConnCursor()
        execute_single_statement(
            fc, self.SCHEMA, self._pools(), "POSTGRES", {}, {}, 0,
            lambda t: None, lambda t: None,
            "widgets", "UPDATE",
        )
        sql = fc.calls[-1]
        self.assertTrue(sql.upper().startswith("UPDATE"))
        self._assert_quoted_not_bare(sql)

    def test_delete_quotes_reserved_word_pk_in_where(self):
        fc = FakeConnCursor()
        execute_single_statement(
            fc, self.SCHEMA, self._pools(), "POSTGRES", {}, {}, 0,
            lambda t: None, lambda t: None,
            "widgets", "DELETE",
        )
        sql = fc.calls[-1]
        self.assertTrue(sql.upper().startswith("DELETE"))
        self._assert_quoted_not_bare(sql)


class TestValidateTransactionMode(unittest.TestCase):
    def test_disabled_minimal_block_is_valid(self):
        validate_transaction_mode({"enabled": False})  # must not raise
        validate_transaction_mode({})  # not even 'enabled' present

    def test_enabled_valid_block_passes(self):
        validate_transaction_mode(make_tm_cfg())  # must not raise

    def test_enabled_requires_non_empty_hot_tables(self):
        with self.assertRaises(ValueError):
            validate_transaction_mode(make_tm_cfg(hot_tables=[]))
        with self.assertRaises(ValueError):
            validate_transaction_mode(make_tm_cfg(hot_tables="not_a_list"))

    def test_enabled_requires_valid_ranges(self):
        with self.assertRaises(ValueError):
            validate_transaction_mode(make_tm_cfg(statements_per_txn={"min": 5, "max": 2}))
        with self.assertRaises(ValueError):
            validate_transaction_mode(make_tm_cfg(savepoint_pairs_per_txn={"min": -1, "max": 2}))

    def test_enabled_requires_at_least_one_positive_op_weight(self):
        with self.assertRaises(ValueError):
            validate_transaction_mode(
                make_tm_cfg(hot_op_weights={"INSERT": 0, "UPDATE": 0, "DELETE": 0})
            )

    def test_not_a_mapping_raises(self):
        with self.assertRaises(ValueError):
            validate_transaction_mode([])

    def test_rows_per_statement_absent_is_valid(self):
        cfg = make_tm_cfg()
        self.assertNotIn("rows_per_statement", cfg)
        validate_transaction_mode(cfg)  # must not raise

    def test_rows_per_statement_valid_ranges_accepted(self):
        validate_transaction_mode(make_tm_cfg(rows_per_statement={"min": 1, "max": 5}))
        validate_transaction_mode(make_tm_cfg(rows_per_statement={"min": 3, "max": 3}))

    def test_rows_per_statement_rejects_min_below_one(self):
        with self.assertRaises(ValueError):
            validate_transaction_mode(make_tm_cfg(rows_per_statement={"min": 0, "max": 5}))

    def test_rows_per_statement_rejects_max_below_min(self):
        with self.assertRaises(ValueError):
            validate_transaction_mode(make_tm_cfg(rows_per_statement={"min": 5, "max": 2}))

    def test_rows_per_statement_rejects_non_int(self):
        with self.assertRaises(ValueError):
            validate_transaction_mode(make_tm_cfg(rows_per_statement={"min": 1.5, "max": 5}))
        with self.assertRaises(ValueError):
            validate_transaction_mode(make_tm_cfg(rows_per_statement={"min": True, "max": 5}))

    def test_rows_per_statement_rejects_missing_min_or_max(self):
        with self.assertRaises(ValueError):
            validate_transaction_mode(make_tm_cfg(rows_per_statement={"min": 1}))
        with self.assertRaises(ValueError):
            validate_transaction_mode(make_tm_cfg(rows_per_statement={"max": 5}))


if __name__ == "__main__":
    unittest.main()
