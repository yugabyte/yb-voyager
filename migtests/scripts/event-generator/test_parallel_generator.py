"""
Unit tests for the pure functions in parallel_generator.py: worker-count +
trimmer math, the reactive fine-trim decision, the phase-change diff, the
monotonic worker_uid allocator, the LIFO worker roster, EMA recalibration
with its clamp, and Schedule.target_at (reusing rate_governor's schedule
math). All side-effect-free -- no DB, no subprocess, no real clock -- by
design, per IMPLEMENTATION_CONTRACTS.md's testing rule. The impure
orchestration (spawn/kill/calibration/control loop) is deliberately not
exercised here.

`parallel_generator.py` itself imports `psycopg2` and `utils` guarded
(try/except -> None), mirroring migration_monitor.py's pattern, precisely
so this module is importable in an environment without psycopg2
installed (this sandbox has neither psycopg2 nor PyYAML). If a real
`import utils` fails here, a minimal stub is registered in `sys.modules`
before `parallel_generator` is imported, so its own guarded `import utils`
resolves to the stub instead of re-attempting -- and failing -- the real
one. None of the tests below touch `utils` or `psycopg2` functionality
directly; the stub only needs to exist so the module imports cleanly.
"""

import sys
import types
import unittest

try:
    import utils  # noqa: F401
except Exception:
    _stub = types.ModuleType("utils")
    sys.modules["utils"] = _stub

from parallel_generator import (
    CrossNodeProgressTracker,
    Schedule,
    SlotFilePool,
    WorkerRoster,
    WorkerUidAllocator,
    build_worker_argv,
    compute_C_observed,
    decide_adjustment,
    decide_coarse,
    decide_overshoot_shed,
    derive_worker_runtime_config,
    diff_worker_counts,
    fine_trim,
    peak_target,
    plan_worker_counts,
    reactive_worker_cap,
    run_calibration,
    TrailingRateWindow,
    recalibrate_C,
    resolve_parallel_config,
)
import parallel_generator


class TestPeakTarget(unittest.TestCase):
    def test_default_only(self):
        rate_control = {"default_events_per_second": 1500}
        self.assertEqual(peak_target(rate_control), 1500)

    def test_default_plus_spike_schedule(self):
        rate_control = {
            "default_events_per_second": 1500,
            "schedule": [
                {"events_per_second": 10000, "duration_seconds": 300, "every_seconds": 1800},
                {"events_per_second": 4000, "duration_seconds": 60, "every_seconds": 600},
            ],
        }
        self.assertEqual(peak_target(rate_control), 10000)

    def test_empty_schedule_list(self):
        rate_control = {"default_events_per_second": 200, "schedule": []}
        self.assertEqual(peak_target(rate_control), 200)


class TestResolveParallelConfig(unittest.TestCase):
    def test_defaults_applied_when_empty(self):
        cfg = resolve_parallel_config({})
        self.assertEqual(cfg["calibration_seconds"], 30)
        self.assertEqual(cfg["max_workers"], 8)
        self.assertEqual(cfg["control_interval_seconds"], 5)
        self.assertEqual(cfg["deadband_pct"], 10)
        self.assertEqual(cfg["cooldown_seconds"], 10)
        self.assertTrue(cfg["allow_throttle"])
        self.assertEqual(cfg["pk_pool_maxsize"], 20000)
        self.assertEqual(cfg["snapshot_refresh_seconds"], 10800)
        self.assertTrue(cfg["recalibrate"])
        self.assertEqual(cfg["pk_stride"], 100000)
        self.assertEqual(cfg["run_seconds"], 604800)
        # Cascade trimmer controller's fine-knob constants (distinct from
        # the legacy bang-bang `deadband_pct`/`cooldown_seconds` above,
        # which the allow_throttle=False path keeps using unchanged).
        self.assertEqual(cfg["fine_kp"], 0.6)
        self.assertEqual(cfg["fine_deadband_pct"], 2.0)
        self.assertEqual(cfg["fine_slew_pct"], 50.0)
        self.assertEqual(cfg["sat_ticks_needed"], 2)
        self.assertEqual(cfg["high_sat_pct"], 98.0)
        self.assertEqual(cfg["low_sat_pct"], 2.0)

        self.assertEqual(cfg["calibration_warmup_seconds"], 20)

    def test_none_treated_as_empty(self):
        cfg = resolve_parallel_config(None)
        self.assertEqual(cfg["max_workers"], 8)

    def test_partial_override_merges_with_defaults(self):
        cfg = resolve_parallel_config({"max_workers": 4, "allow_throttle": False})
        self.assertEqual(cfg["max_workers"], 4)
        self.assertFalse(cfg["allow_throttle"])
        # Untouched keys keep their defaults.
        self.assertEqual(cfg["calibration_seconds"], 30)

    def test_input_not_mutated(self):
        original = {"max_workers": 4}
        snapshot = dict(original)
        resolve_parallel_config(original)
        self.assertEqual(original, snapshot)


class TestPlanWorkerCounts(unittest.TestCase):
    def test_baseline_below_ceiling_is_pure_trimmer(self):
        # T=1500, C=7000 -> 0 full + trimmer@1500 (design doc's example).
        plan = plan_worker_counts(1500, 7000)
        self.assertEqual(plan["n_full"], 0)
        self.assertAlmostEqual(plan["trimmer_rate"], 1500.0)

    def test_spike_above_ceiling_is_one_full_plus_trimmer(self):
        # T=10000, C=7000 -> 1 full + trimmer@3000 (design doc's example).
        plan = plan_worker_counts(10000, 7000)
        self.assertEqual(plan["n_full"], 1)
        self.assertAlmostEqual(plan["trimmer_rate"], 3000.0)

    def test_exact_multiple_needs_no_trimmer(self):
        plan = plan_worker_counts(14000, 7000)
        self.assertEqual(plan["n_full"], 2)
        self.assertEqual(plan["trimmer_rate"], 0.0)

    def test_zero_target(self):
        plan = plan_worker_counts(0, 7000)
        self.assertEqual(plan["n_full"], 0)
        self.assertEqual(plan["trimmer_rate"], 0.0)

    def test_allow_throttle_false_rounds_to_nearest_worker(self):
        # T=10000, C=7000 -> 10000/7000=1.43 -> rounds to 1.
        plan = plan_worker_counts(10000, 7000, allow_throttle=False)
        self.assertEqual(plan["n_full"], 1)
        self.assertEqual(plan["trimmer_rate"], 0.0)

    def test_allow_throttle_false_rounds_up_when_closer(self):
        # T=12000, C=7000 -> 12000/7000=1.71 -> rounds to 2.
        plan = plan_worker_counts(12000, 7000, allow_throttle=False)
        self.assertEqual(plan["n_full"], 2)

    def test_allow_throttle_false_unreachable_baseline_raises(self):
        # T=1500, C=7000, no throttle -> rounds to 0 -> unreachable, must error.
        with self.assertRaises(ValueError):
            plan_worker_counts(1500, 7000, allow_throttle=False)

    def test_allow_throttle_false_zero_target_is_zero_workers_no_error(self):
        plan = plan_worker_counts(0, 7000, allow_throttle=False)
        self.assertEqual(plan["n_full"], 0)

    def test_invalid_ceiling_raises(self):
        with self.assertRaises(ValueError):
            plan_worker_counts(1000, 0)
        with self.assertRaises(ValueError):
            plan_worker_counts(1000, -5)

    def test_negative_target_raises(self):
        with self.assertRaises(ValueError):
            plan_worker_counts(-1, 7000)


class TestDiffWorkerCounts(unittest.TestCase):
    def test_baseline_to_spike_spawns_full_and_respawns_trimmer(self):
        # baseline: 0 full, trimmer@1500. spike: 1 full, trimmer@3000.
        actions = diff_worker_counts(0, 1500.0, {"n_full": 1, "trimmer_rate": 3000.0})
        self.assertEqual(actions["uncapped_delta"], 1)
        self.assertEqual(actions["trimmer_action"], "respawn")

    def test_spike_to_baseline_kills_full_and_respawns_trimmer(self):
        actions = diff_worker_counts(1, 3000.0, {"n_full": 0, "trimmer_rate": 1500.0})
        self.assertEqual(actions["uncapped_delta"], -1)
        self.assertEqual(actions["trimmer_action"], "respawn")

    def test_no_trimmer_either_side_is_none_action(self):
        actions = diff_worker_counts(2, 0.0, {"n_full": 2, "trimmer_rate": 0.0})
        self.assertEqual(actions["uncapped_delta"], 0)
        self.assertEqual(actions["trimmer_action"], "none")

    def test_same_trimmer_rate_before_and_after_is_none_action(self):
        actions = diff_worker_counts(1, 1500.0, {"n_full": 1, "trimmer_rate": 1500.0})
        self.assertEqual(actions["trimmer_action"], "none")

    def test_first_trimmer_needed_is_spawn(self):
        actions = diff_worker_counts(0, 0.0, {"n_full": 0, "trimmer_rate": 1500.0})
        self.assertEqual(actions["trimmer_action"], "spawn")

    def test_trimmer_no_longer_needed_is_kill(self):
        actions = diff_worker_counts(2, 500.0, {"n_full": 2, "trimmer_rate": 0.0})
        self.assertEqual(actions["trimmer_action"], "kill")

    def test_uncapped_delta_zero_when_already_at_target(self):
        actions = diff_worker_counts(3, 0.0, {"n_full": 3, "trimmer_rate": 0.0})
        self.assertEqual(actions["uncapped_delta"], 0)


class TestDecideAdjustment(unittest.TestCase):
    def test_within_deadband_holds(self):
        # target=1000, deadband_pct=10 -> deadband=100 -> [900, 1100] holds.
        self.assertEqual(decide_adjustment(1000, 1000, 10, None, 10), "hold")
        self.assertEqual(decide_adjustment(950, 1000, 10, None, 10), "hold")
        self.assertEqual(decide_adjustment(1050, 1000, 10, None, 10), "hold")

    def test_below_deadband_adds(self):
        self.assertEqual(decide_adjustment(800, 1000, 10, None, 10), "add")

    def test_above_deadband_kills(self):
        self.assertEqual(decide_adjustment(1200, 1000, 10, None, 10), "kill")

    def test_exactly_at_edge_holds(self):
        # deadband=100 -> target-deadband=900 exactly -> strict '<' means hold.
        self.assertEqual(decide_adjustment(900, 1000, 10, None, 10), "hold")
        self.assertEqual(decide_adjustment(1100, 1000, 10, None, 10), "hold")

    def test_cooldown_suppresses_add(self):
        # Would otherwise "add" (achieved << target), but cooldown blocks it.
        self.assertEqual(decide_adjustment(100, 1000, 10, 3, 10), "hold")

    def test_cooldown_suppresses_kill(self):
        self.assertEqual(decide_adjustment(5000, 1000, 10, 3, 10), "hold")

    def test_cooldown_elapsed_allows_adjustment(self):
        self.assertEqual(decide_adjustment(100, 1000, 10, 10, 10), "add")
        self.assertEqual(decide_adjustment(100, 1000, 10, 15, 10), "add")

    def test_no_prior_adjustment_never_suppressed(self):
        self.assertEqual(decide_adjustment(100, 1000, 10, None, 10), "add")


class TestComputeCObserved(unittest.TestCase):
    def test_subtracts_trimmer_and_divides_by_uncapped(self):
        # achieved=10000, trimmer=3000, n_uncapped=1 -> C_obs=7000.
        self.assertAlmostEqual(compute_C_observed(10000, 3000, 1), 7000.0)

    def test_two_uncapped_workers(self):
        self.assertAlmostEqual(compute_C_observed(15000, 1000, 2), 7000.0)

    def test_no_uncapped_workers_returns_none(self):
        self.assertIsNone(compute_C_observed(1500, 1500, 0))

    def test_zero_trimmer(self):
        self.assertAlmostEqual(compute_C_observed(7000, 0, 1), 7000.0)


class TestRecalibrateC(unittest.TestCase):
    def test_small_increase_within_clamp_applies_ema(self):
        # C=7000, C_obs=7700 (10% high). alpha=0.3 -> ema=7000+0.3*700=7210.
        # delta=210, max_delta=700 (10% of 7000) -> not clamped.
        new_c = recalibrate_C(7000, 7700, alpha=0.3, max_delta_pct=10.0)
        self.assertAlmostEqual(new_c, 7210.0)

    def test_large_jump_clamped_to_max_delta_pct(self):
        # C=7000, C_obs=20000 -> ema pulls far above +10%, clamp caps it at
        # exactly C * 1.10.
        new_c = recalibrate_C(7000, 20000, alpha=0.3, max_delta_pct=10.0)
        self.assertAlmostEqual(new_c, 7700.0)

    def test_large_drop_clamped_to_max_delta_pct(self):
        new_c = recalibrate_C(7000, 100, alpha=0.3, max_delta_pct=10.0)
        self.assertAlmostEqual(new_c, 6300.0)

    def test_c_observed_equal_to_c_is_a_noop(self):
        self.assertAlmostEqual(recalibrate_C(7000, 7000), 7000.0)

    def test_invalid_c_raises(self):
        with self.assertRaises(ValueError):
            recalibrate_C(0, 1000)
        with self.assertRaises(ValueError):
            recalibrate_C(-5, 1000)

    def test_negative_c_observed_raises(self):
        with self.assertRaises(ValueError):
            recalibrate_C(7000, -1)


class TestWorkerUidAllocator(unittest.TestCase):
    def test_monotonic_from_zero_by_default(self):
        allocator = WorkerUidAllocator()
        self.assertEqual(allocator.allocate(), 0)
        self.assertEqual(allocator.allocate(), 1)
        self.assertEqual(allocator.allocate(), 2)

    def test_custom_start(self):
        allocator = WorkerUidAllocator(start=100)
        self.assertEqual(allocator.allocate(), 100)
        self.assertEqual(allocator.allocate(), 101)

    def test_negative_start_raises(self):
        with self.assertRaises(ValueError):
            WorkerUidAllocator(start=-1)

    def test_never_reused_across_simulated_spawn_kill_cycles(self):
        # Simulate ~50 spawn/kill cycles: allocate a uid, "kill" it (i.e.
        # just drop it -- there is deliberately no way to return an id),
        # allocate more. The sequence must stay strictly increasing with
        # no repeats no matter how many are "killed" in between.
        allocator = WorkerUidAllocator()
        seen = set()
        for _ in range(200):
            uid = allocator.allocate()
            self.assertNotIn(uid, seen, "worker_uid was reused after a simulated kill")
            seen.add(uid)
        self.assertEqual(sorted(seen), list(range(200)))

    def test_next_uid_reflects_allocation_count(self):
        allocator = WorkerUidAllocator()
        self.assertEqual(allocator.next_uid, 0)
        allocator.allocate()
        allocator.allocate()
        self.assertEqual(allocator.next_uid, 2)


class TestWorkerRoster(unittest.TestCase):
    def test_add_and_pop_newest_uncapped_is_lifo(self):
        roster = WorkerRoster()
        roster.add_uncapped(1)
        roster.add_uncapped(2)
        roster.add_uncapped(3)
        self.assertEqual(roster.pop_newest_uncapped(), 3)
        self.assertEqual(roster.pop_newest_uncapped(), 2)
        self.assertEqual(roster.pop_newest_uncapped(), 1)
        self.assertIsNone(roster.pop_newest_uncapped())

    def test_trimmer_tracked_separately_and_never_popped(self):
        roster = WorkerRoster()
        roster.set_trimmer(99)
        roster.add_uncapped(1)
        roster.add_uncapped(2)
        # Popping "newest uncapped" twice must exhaust 2 and 1, never touch
        # the trimmer -- this is what "keep the baseline trimmer alive"
        # means at the roster level.
        self.assertEqual(roster.pop_newest_uncapped(), 2)
        self.assertEqual(roster.pop_newest_uncapped(), 1)
        self.assertIsNone(roster.pop_newest_uncapped())
        self.assertEqual(roster.trimmer_uid, 99)

    def test_counts(self):
        roster = WorkerRoster()
        roster.add_uncapped(1)
        roster.add_uncapped(2)
        roster.set_trimmer(3)
        self.assertEqual(roster.n_uncapped(), 2)
        self.assertEqual(roster.n_trimmer(), 1)
        self.assertEqual(sorted(roster.all_uids()), [1, 2, 3])

    def test_no_trimmer_counts_zero(self):
        roster = WorkerRoster()
        self.assertEqual(roster.n_trimmer(), 0)
        self.assertIsNone(roster.trimmer_uid)

    def test_clear_trimmer(self):
        roster = WorkerRoster()
        roster.set_trimmer(5)
        roster.clear_trimmer()
        self.assertIsNone(roster.trimmer_uid)
        self.assertEqual(roster.n_trimmer(), 0)

    def test_remove_uncapped_arbitrary_uid(self):
        # e.g. cleanup after an unexpected crash of a non-newest worker.
        roster = WorkerRoster()
        roster.add_uncapped(1)
        roster.add_uncapped(2)
        roster.add_uncapped(3)
        roster.remove_uncapped(2)
        self.assertEqual(roster.n_uncapped(), 2)
        self.assertEqual(roster.pop_newest_uncapped(), 3)
        self.assertEqual(roster.pop_newest_uncapped(), 1)

    def test_remove_uncapped_missing_uid_is_a_noop(self):
        roster = WorkerRoster()
        roster.add_uncapped(1)
        roster.remove_uncapped(999)
        self.assertEqual(roster.n_uncapped(), 1)


class TestDeriveWorkerRuntimeConfig(unittest.TestCase):
    def _base_config(self):
        return {
            "connection": {"host": "localhost", "port": 5432, "database": "sakila",
                            "user": "postgres", "password": "postgres"},
            "generator": {
                "schema_name": "public",
                "manual_table_list": ["eg_users"],
                "random_seed": 12345,
                "rate_control": {"default_events_per_second": 1500},
            },
            "parallel": {"max_workers": 6, "calibration_seconds": 30},
        }

    def test_parallel_block_removed(self):
        cfg = derive_worker_runtime_config(self._base_config())
        self.assertNotIn("parallel", cfg)

    def test_rate_control_removed(self):
        cfg = derive_worker_runtime_config(self._base_config())
        self.assertNotIn("rate_control", cfg["generator"])

    def test_other_generator_keys_preserved(self):
        cfg = derive_worker_runtime_config(self._base_config())
        self.assertEqual(cfg["generator"]["schema_name"], "public")
        self.assertEqual(cfg["generator"]["manual_table_list"], ["eg_users"])
        self.assertEqual(cfg["generator"]["random_seed"], 12345)

    def test_connection_block_preserved(self):
        cfg = derive_worker_runtime_config(self._base_config())
        self.assertEqual(cfg["connection"]["database"], "sakila")

    def test_base_config_not_mutated(self):
        base = self._base_config()
        derive_worker_runtime_config(base)
        self.assertIn("parallel", base)
        self.assertIn("rate_control", base["generator"])


class TestBuildWorkerArgv(unittest.TestCase):
    def test_uncapped_worker_has_no_throttle_flag(self):
        argv = build_worker_argv(
            "python3", "/path/generator.py", "/tmp/slot_0.yaml", worker_uid=5,
            pk_stride=100000, cache_dir="/tmp/cache", cache_version="v1", throttle=0.0,
        )
        self.assertNotIn("--throttle", argv)
        self.assertIn("--worker-uid", argv)
        self.assertEqual(argv[argv.index("--worker-uid") + 1], "5")
        self.assertEqual(argv[argv.index("--pk-stride") + 1], "100000")
        self.assertEqual(argv[argv.index("--cache-dir") + 1], "/tmp/cache")
        self.assertEqual(argv[argv.index("--cache-version") + 1], "v1")

    def test_trimmer_worker_includes_throttle_flag(self):
        argv = build_worker_argv(
            "python3", "/path/generator.py", "/tmp/slot_1.yaml", worker_uid=7,
            pk_stride=100000, cache_dir="/tmp/cache", cache_version="v1", throttle=1500.0,
        )
        self.assertIn("--throttle", argv)
        self.assertEqual(argv[argv.index("--throttle") + 1], "1500.0")

    def test_config_path_passed_via_dash_c(self):
        argv = build_worker_argv(
            "python3", "/path/generator.py", "/tmp/slot_2.yaml", worker_uid=0,
            pk_stride=100000, cache_dir="/tmp/cache", cache_version="v1",
        )
        self.assertEqual(argv[0], "python3")
        self.assertEqual(argv[1], "/path/generator.py")
        idx = argv.index("-c")
        self.assertEqual(argv[idx + 1], "/tmp/slot_2.yaml")

    def test_uncapped_worker_gets_no_control_file_flag(self):
        # Uncapped workers never get --control-file, even if a path is
        # passed by mistake with throttle=0 -- only a throttled (trimmer)
        # spawn should ever carry it.
        argv = build_worker_argv(
            "python3", "/path/generator.py", "/tmp/slot_3.yaml", worker_uid=1,
            pk_stride=100000, cache_dir="/tmp/cache", cache_version="v1", throttle=0.0,
            control_file=None,
        )
        self.assertNotIn("--control-file", argv)

    def test_trimmer_worker_includes_control_file_flag(self):
        argv = build_worker_argv(
            "python3", "/path/generator.py", "/tmp/slot_4.yaml", worker_uid=2,
            pk_stride=100000, cache_dir="/tmp/cache", cache_version="v1", throttle=1.0,
            control_file="/tmp/parallel_generator_xyz/trimmer_rate.txt",
        )
        self.assertIn("--control-file", argv)
        self.assertEqual(
            argv[argv.index("--control-file") + 1],
            "/tmp/parallel_generator_xyz/trimmer_rate.txt",
        )

    def test_control_file_default_is_absent(self):
        argv = build_worker_argv(
            "python3", "/path/generator.py", "/tmp/slot_5.yaml", worker_uid=3,
            pk_stride=100000, cache_dir="/tmp/cache", cache_version="v1", throttle=1.0,
        )
        self.assertNotIn("--control-file", argv)

    def test_run_id_flag_appended_when_given(self):
        argv = build_worker_argv(
            "python3", "/path/generator.py", "/tmp/slot_6.yaml", worker_uid=4,
            pk_stride=100000, cache_dir="/tmp/cache", cache_version="v1",
            run_id="abc123",
        )
        self.assertIn("--run-id", argv)
        self.assertEqual(argv[argv.index("--run-id") + 1], "abc123")

    def test_run_id_default_is_absent(self):
        argv = build_worker_argv(
            "python3", "/path/generator.py", "/tmp/slot_7.yaml", worker_uid=5,
            pk_stride=100000, cache_dir="/tmp/cache", cache_version="v1",
        )
        self.assertNotIn("--run-id", argv)

    def test_run_id_empty_string_omits_flag(self):
        argv = build_worker_argv(
            "python3", "/path/generator.py", "/tmp/slot_8.yaml", worker_uid=6,
            pk_stride=100000, cache_dir="/tmp/cache", cache_version="v1",
            run_id="",
        )
        self.assertNotIn("--run-id", argv)


class TestSlotFilePool(unittest.TestCase):
    def test_acquire_returns_distinct_paths_up_to_size(self):
        pool = SlotFilePool("/tmp/whatever", size=3)
        idx0, path0 = pool.acquire()
        idx1, path1 = pool.acquire()
        idx2, path2 = pool.acquire()
        self.assertEqual({idx0, idx1, idx2}, {0, 1, 2})
        self.assertEqual(len({path0, path1, path2}), 3)

    def test_exhausted_pool_raises(self):
        pool = SlotFilePool("/tmp/whatever", size=2)
        pool.acquire()
        pool.acquire()
        with self.assertRaises(RuntimeError):
            pool.acquire()

    def test_release_makes_slot_reusable(self):
        pool = SlotFilePool("/tmp/whatever", size=1)
        idx, path = pool.acquire()
        with self.assertRaises(RuntimeError):
            pool.acquire()
        pool.release(idx)
        idx2, path2 = pool.acquire()
        self.assertEqual(idx2, idx)
        self.assertEqual(path2, path)

    def test_in_use_count(self):
        pool = SlotFilePool("/tmp/whatever", size=4)
        self.assertEqual(pool.in_use_count(), 0)
        idx0, _ = pool.acquire()
        idx1, _ = pool.acquire()
        self.assertEqual(pool.in_use_count(), 2)
        pool.release(idx0)
        self.assertEqual(pool.in_use_count(), 1)
        pool.release(idx1)
        self.assertEqual(pool.in_use_count(), 0)

    def test_never_exceeds_size_paths(self):
        pool = SlotFilePool("/tmp/whatever", size=5)
        self.assertEqual(len(pool._paths), 5)

    def test_size_must_be_positive(self):
        with self.assertRaises(ValueError):
            SlotFilePool("/tmp/whatever", size=0)


class TestSchedule(unittest.TestCase):
    def _rate_control(self):
        return {
            "default_events_per_second": 1500,
            "schedule": [
                {
                    "events_per_second": 10000,
                    "duration_seconds": 300,
                    "every_seconds": 1800,
                    "offset_seconds": 600,
                    "jitter_pct": 0,
                }
            ],
        }

    def test_baseline_before_first_spike_window(self):
        schedule = Schedule(self._rate_control(), random_seed=42, run_start=1000.0)
        # elapsed=100 (< offset_seconds=600) -> baseline.
        self.assertEqual(schedule.target_at(1100.0), 1500)

    def test_spike_window_active(self):
        schedule = Schedule(self._rate_control(), random_seed=42, run_start=1000.0)
        # elapsed=600 (window start, inclusive) -> spike active.
        self.assertEqual(schedule.target_at(1600.0), 10000)
        # elapsed=899 (still within the 300s window) -> spike active.
        self.assertEqual(schedule.target_at(1899.0), 10000)

    def test_spike_window_boundary_end_is_exclusive(self):
        schedule = Schedule(self._rate_control(), random_seed=42, run_start=1000.0)
        # window_end = 600+300 = 900 -> at exactly elapsed=900, spike has ended.
        self.assertEqual(schedule.target_at(1900.0), 1500)

    def test_recurs_every_period(self):
        schedule = Schedule(self._rate_control(), random_seed=42, run_start=0.0)
        # Second period: offset + every_seconds = 600 + 1800 = 2400.
        self.assertEqual(schedule.target_at(2400.0), 10000)
        self.assertEqual(schedule.target_at(2400.0 + 300.0), 1500)

    def test_deterministic_for_same_random_seed(self):
        rate_control_with_jitter = {
            "default_events_per_second": 1500,
            "schedule": [
                {"events_per_second": 10000, "duration_seconds": 300, "every_seconds": 1800,
                 "offset_seconds": 600, "jitter_pct": 10},
            ],
        }
        s1 = Schedule(rate_control_with_jitter, random_seed=7, run_start=0.0)
        s2 = Schedule(rate_control_with_jitter, random_seed=7, run_start=0.0)
        for now in (100.0, 650.0, 900.0, 2500.0):
            self.assertEqual(s1.target_at(now), s2.target_at(now))

    def test_run_start_offsets_absolute_time_correctly(self):
        schedule_a = Schedule(self._rate_control(), random_seed=1, run_start=0.0)
        schedule_b = Schedule(self._rate_control(), random_seed=1, run_start=5000.0)
        # Same *elapsed* time (700s into the run) must give the same target
        # regardless of where run_start sits on the absolute clock.
        self.assertEqual(schedule_a.target_at(700.0), schedule_b.target_at(5700.0))


class TestReactiveWorkerCap(unittest.TestCase):
    """Backpressure cap: reactive scale-up may not exceed
    ceil(target/C) * margin, keyed on the STABLE calibration C."""

    def test_caps_reactive_growth(self):
        # The real incident: target 10k, calibration C~3279 -> need 4,
        # margin 1.5 -> cap 6 (NOT the 15 the old loop ramped to).
        self.assertEqual(reactive_worker_cap(10000, 3279, 1.5), 6)

    def test_default_margin(self):
        # ceil(10000/5000)=2, *1.5=3
        self.assertEqual(reactive_worker_cap(10000, 5000), 3)

    def test_margin_one_is_exact_need(self):
        self.assertEqual(reactive_worker_cap(10000, 2500, 1.0), 4)

    def test_small_target_still_allows_headroom(self):
        # tiny target vs huge C: need=ceil(1500/5000)=1, *1.5 -> ceil=2.
        self.assertEqual(reactive_worker_cap(1500, 5000, 1.5), 2)
        # margin 1.0 gives the exact floor of 1 for a sub-C target.
        self.assertEqual(reactive_worker_cap(1500, 5000, 1.0), 1)

    def test_nonpositive_C_returns_one(self):
        self.assertEqual(reactive_worker_cap(10000, 0, 1.5), 1)
        self.assertEqual(reactive_worker_cap(10000, -5, 1.5), 1)

    def test_cap_does_not_grow_as_live_C_would_erode(self):
        # Keyed on the stable calibration C, a low (eroded) C is never passed;
        # but even if target rises the cap stays proportional to calib C.
        cap_healthy = reactive_worker_cap(10000, 5000, 1.5)   # 3
        cap_more_load = reactive_worker_cap(10000, 5000, 1.5)  # same C -> same
        self.assertEqual(cap_healthy, cap_more_load)


# ---------------------------------------------------------------------------
# Cascade trimmer controller -- pure decision functions.
# See ARCHITECTURE.md.
# ---------------------------------------------------------------------------

class TestFineTrim(unittest.TestCase):
    """fine_trim: the integral fine knob driving the persistent trimmer's
    commanded rate. error = target - achieved; hold inside the deadband;
    otherwise a kp-scaled, slew-clamped step; anti-windup clamps the result
    to [0, C]."""

    def test_hold_within_deadband(self):
        # target=10000, deadband_pct=2 -> deadband=200 -> [9800,10200] holds.
        self.assertEqual(fine_trim(600.0, 9900.0, 10000.0, 7000.0), 600.0)
        self.assertEqual(fine_trim(600.0, 10100.0, 10000.0, 7000.0), 600.0)
        self.assertEqual(fine_trim(600.0, 10000.0, 10000.0, 7000.0), 600.0)

    def test_exactly_at_deadband_edge_holds(self):
        # Spec uses <=, so exactly at the edge still holds.
        self.assertEqual(fine_trim(600.0, 9800.0, 10000.0, 7000.0), 600.0)
        self.assertEqual(fine_trim(600.0, 10200.0, 10000.0, 7000.0), 600.0)

    def test_proportional_step_below_target(self):
        # error = 10000-9000 = 1000, outside deadband(200). step = kp*error
        # = 0.6*1000 = 600. max_step = C*0.5 = 3500 -> not clamped.
        # new = 600 (current) + 600 = 1200, within [0, 7000].
        result = fine_trim(600.0, 9000.0, 10000.0, 7000.0)
        self.assertAlmostEqual(result, 1200.0)

    def test_proportional_step_above_target(self):
        # error = 10000-11000 = -1000. step = 0.6*-1000 = -600.
        # new = 600 - 600 = 0.
        result = fine_trim(600.0, 11000.0, 10000.0, 7000.0)
        self.assertAlmostEqual(result, 0.0)

    def test_slew_clamp_positive_side(self):
        # Huge undershoot: error=10000-100=9900. step=0.6*9900=5940 >
        # max_step=C*0.5=3500 -> clamped to +3500. new=600+3500=4100.
        result = fine_trim(600.0, 100.0, 10000.0, 7000.0)
        self.assertAlmostEqual(result, 4100.0)

    def test_slew_clamp_negative_side(self):
        # Huge overshoot: error=10000-30000=-20000. step=0.6*-20000=-12000 <
        # -max_step=-3500 -> clamped to -3500. new=4000-3500=500.
        result = fine_trim(4000.0, 30000.0, 10000.0, 7000.0)
        self.assertAlmostEqual(result, 500.0)

    def test_anti_windup_clamps_to_C_upper_bound(self):
        # trimmer already near C; a further undershoot step must not push
        # it above C.
        result = fine_trim(6900.0, 100.0, 10000.0, 7000.0, slew_pct=200.0)
        self.assertAlmostEqual(result, 7000.0)

    def test_anti_windup_clamps_to_zero_lower_bound(self):
        # trimmer already near 0; a further overshoot step must not push it
        # negative.
        result = fine_trim(100.0, 30000.0, 10000.0, 7000.0, slew_pct=200.0)
        self.assertAlmostEqual(result, 0.0)

    def test_zero_target_drives_toward_zero(self):
        # target=0 -> deadband=0; any positive achieved is "over" -> steps
        # trimmer down, clamped at 0.
        result = fine_trim(500.0, 400.0, 0.0, 7000.0)
        self.assertLess(result, 500.0)
        self.assertGreaterEqual(result, 0.0)

    def test_converges_monotonically_over_several_ticks(self):
        # Closed-loop simulation: achieved = base + trimmer (fake plant),
        # base fixed (e.g. contribution from full workers), only the
        # trimmer is adjusted. With deadband effectively disabled and no
        # slew/anti-windup engaged, error should decay geometrically by
        # (1 - kp) every tick -- monotone decay for 0 < kp <= 1.
        kp = 0.5
        C = 10000.0
        target = 1000.0
        base = 0.0
        trimmer_rate = 0.0

        prev_abs_error = None
        for _ in range(8):
            achieved = base + trimmer_rate
            error = target - achieved
            if prev_abs_error is not None:
                # Strictly decreasing in magnitude (until it gets very
                # close to target, at which point later assertions below
                # confirm convergence).
                self.assertLessEqual(abs(error), prev_abs_error + 1e-9)
            prev_abs_error = abs(error)
            trimmer_rate = fine_trim(
                trimmer_rate, achieved, target, C, kp=kp, deadband_pct=0.0, slew_pct=200.0
            )

        final_achieved = base + trimmer_rate
        self.assertAlmostEqual(final_achieved, target, delta=target * 0.05)

    def test_never_returns_negative_or_above_C(self):
        for achieved in (0.0, 5000.0, 50000.0):
            for trimmer_rate in (0.0, 3500.0, 7000.0):
                result = fine_trim(trimmer_rate, achieved, 10000.0, 7000.0)
                self.assertGreaterEqual(result, 0.0)
                self.assertLessEqual(result, 7000.0)

    def test_freeze_increase_blocks_upward_step(self):
        # Under target (would normally step UP), but freeze_increase=True ->
        # the knob must NOT rise. This is the anti-windup guard the loop
        # applies during the post-spawn cooldown while a worker is ramping.
        without = fine_trim(3000.0, 3000.0, 10000.0, 7000.0)
        self.assertGreater(without, 3000.0)  # sanity: normally rises
        with_freeze = fine_trim(3000.0, 3000.0, 10000.0, 7000.0, freeze_increase=True)
        self.assertEqual(with_freeze, 3000.0)

    def test_freeze_increase_still_allows_decrease(self):
        # Over target: the knob must still be free to step DOWN even with
        # freeze_increase=True (correcting an overshoot is always safe).
        result = fine_trim(4000.0, 12000.0, 10000.0, 7000.0, freeze_increase=True)
        self.assertLess(result, 4000.0)

    def test_freeze_increase_holds_within_deadband(self):
        # Inside the deadband it holds regardless of freeze_increase.
        self.assertEqual(
            fine_trim(600.0, 10000.0, 10000.0, 7000.0, freeze_increase=True), 600.0
        )

    def test_freeze_increase_prevents_spike_onset_overshoot(self):
        # Regression: phase-change baseline->spike with a 1-tick worker-ramp
        # lag. Without the guard the trimmer winds up while the new uncapped
        # worker ramps, then both land -> a large one-tick overshoot. With
        # freeze_increase during the ramp window, peak achieved stays at or
        # below target.
        PW = 7000.0        # per-uncapped-worker real rate
        C = 7000.0
        target = 10000.0

        def run(guard):
            n_full = 1
            trimmer = 3000.0          # phase-change FF jump remainder
            ramp_left = 1             # the freshly-spawned worker ramps 1 tick
            peak = 0.0
            for _ in range(6):
                eff = n_full - (1 if ramp_left > 0 else 0)
                achieved = eff * PW + min(trimmer, PW)
                peak = max(peak, achieved)
                in_cooldown = ramp_left > 0
                if ramp_left > 0:
                    ramp_left -= 1
                trimmer = fine_trim(
                    trimmer, achieved, target, C,
                    freeze_increase=(guard and in_cooldown),
                )
            return peak

        self.assertGreater(run(guard=False), target * 1.10)   # >10% overshoot unguarded
        self.assertLessEqual(run(guard=True), target + 1e-6)  # no overshoot guarded


class TestDecideCoarse(unittest.TestCase):
    """decide_coarse: the coarse base-knob decision (add/remove/hold a
    whole uncapped worker) driven by the fine knob's saturation state, with
    the anti-hunt guard on removal."""

    def test_add_when_high_sat_under_and_room(self):
        # trimmer pinned high (>=98% of C), achieved under target, enough
        # consecutive saturated ticks, and room under cap.
        decision = decide_coarse(
            n_full=1, trimmer_rate=6900.0, achieved=9000.0, target=10000.0, C=7000.0,
            cap=5, sat_high_ticks=2, sat_low_ticks=0, sat_ticks_needed=2,
        )
        self.assertEqual(decision, "add")

    def test_no_add_when_not_enough_saturated_ticks(self):
        decision = decide_coarse(
            n_full=1, trimmer_rate=6900.0, achieved=9000.0, target=10000.0, C=7000.0,
            cap=5, sat_high_ticks=1, sat_low_ticks=0, sat_ticks_needed=2,
        )
        self.assertEqual(decision, "hold")

    def test_no_add_when_at_cap(self):
        decision = decide_coarse(
            n_full=5, trimmer_rate=6900.0, achieved=9000.0, target=10000.0, C=7000.0,
            cap=5, sat_high_ticks=2, sat_low_ticks=0, sat_ticks_needed=2,
        )
        self.assertEqual(decision, "hold")

    def test_no_add_when_achieved_not_under_target(self):
        decision = decide_coarse(
            n_full=1, trimmer_rate=6900.0, achieved=10500.0, target=10000.0, C=7000.0,
            cap=5, sat_high_ticks=2, sat_low_ticks=0, sat_ticks_needed=2,
        )
        self.assertEqual(decision, "hold")

    def test_remove_when_low_sat_over_and_guard_passes(self):
        # n_full=2, C=5000 -> n_full*C=10000 >= target=9000, so removing
        # still leaves the target reachable.
        decision = decide_coarse(
            n_full=2, trimmer_rate=50.0, achieved=9500.0, target=9000.0, C=5000.0,
            cap=5, sat_high_ticks=0, sat_low_ticks=2, sat_ticks_needed=2,
        )
        self.assertEqual(decision, "remove")

    def test_anti_hunt_no_remove_when_n_full_times_C_below_target(self):
        # The critical anti-hunt case from the spec: n_full=1, C=5000 ->
        # n_full*C=5000 < target=9000 -- removing would collapse to a
        # trimmer-only max of C, well under target. Must hold even though
        # low_sat + over + enough ticks are all true.
        decision = decide_coarse(
            n_full=1, trimmer_rate=50.0, achieved=9500.0, target=9000.0, C=5000.0,
            cap=5, sat_high_ticks=0, sat_low_ticks=2, sat_ticks_needed=2,
        )
        self.assertEqual(decision, "hold")

    def test_no_remove_when_n_full_is_zero(self):
        # Can't remove below 0 full workers.
        decision = decide_coarse(
            n_full=0, trimmer_rate=50.0, achieved=9500.0, target=9000.0, C=5000.0,
            cap=5, sat_high_ticks=0, sat_low_ticks=2, sat_ticks_needed=2,
        )
        self.assertEqual(decision, "hold")

    def test_no_remove_when_not_enough_saturated_ticks(self):
        decision = decide_coarse(
            n_full=2, trimmer_rate=50.0, achieved=9500.0, target=9000.0, C=5000.0,
            cap=5, sat_high_ticks=0, sat_low_ticks=1, sat_ticks_needed=2,
        )
        self.assertEqual(decision, "hold")

    def test_no_remove_when_achieved_not_over_target(self):
        decision = decide_coarse(
            n_full=2, trimmer_rate=50.0, achieved=8500.0, target=9000.0, C=5000.0,
            cap=5, sat_high_ticks=0, sat_low_ticks=2, sat_ticks_needed=2,
        )
        self.assertEqual(decision, "hold")

    def test_hold_when_neither_saturated(self):
        # trimmer sitting mid-range -- not pinned high or low.
        decision = decide_coarse(
            n_full=1, trimmer_rate=3500.0, achieved=8000.0, target=10000.0, C=7000.0,
            cap=5, sat_high_ticks=5, sat_low_ticks=5, sat_ticks_needed=2,
        )
        self.assertEqual(decision, "hold")

    def test_high_and_low_sat_mutually_exclusive(self):
        # For any C > 0, a single trimmer_rate can never satisfy both
        # >= 0.98*C and <= 0.02*C simultaneously -- no flapping between
        # add/remove within one call.
        for trimmer_rate in (0.0, 50.0, 3500.0, 6900.0, 7000.0):
            high_sat = trimmer_rate >= 7000.0 * 0.98
            low_sat = trimmer_rate <= 7000.0 * 0.02
            self.assertFalse(high_sat and low_sat)


class TestCascadeClosedLoopConvergence(unittest.TestCase):
    """Pure, fake-plant integration test wiring fine_trim + decide_coarse +
    compute_C_observed + recalibrate_C together exactly as the cascade
    controller's per-tick order prescribes (recalibrate BEFORE coarse),
    with NO real clock, DB, or subprocess -- just the pure functions.
    """

    def _simulate(self, real_per_worker, target, C_initial, cap, ticks=40,
                  sat_ticks_needed=2, cluster_ceiling=None):
        """Drive the cascade loop against a fake plant:
        achieved = n_full * real_per_worker + trimmer_rate (optionally
        capped at `cluster_ceiling`, modeling a cluster-bound bottleneck
        that doesn't scale with worker count).

        Mirrors run_controller's per-tick order: reap (n/a, pure sim),
        measure achieved, recalibrate (if n_full>=1), coarse (update sat
        counters, decide, apply feed-forward), fine (fine_trim). Per the
        spec, a phase change's initial n_full_target also respects the
        reactive cap, so the simulated starting point is capped too.
        Returns the list of achieved values, one per tick, for the caller
        to assert convergence/no-oscillation on.
        """
        n_full = min(int(target // C_initial), cap)
        trimmer_rate = max(0.0, min(C_initial, target - n_full * C_initial))
        C = C_initial
        sat_high_ticks = 0
        sat_low_ticks = 0
        achieved_history = []
        decisions_history = []

        for _ in range(ticks):
            achieved = n_full * real_per_worker + trimmer_rate
            if cluster_ceiling is not None:
                achieved = min(achieved, cluster_ceiling)

            # (a) Recalibrate BEFORE coarse, only when there's something to
            # observe.
            if n_full >= 1:
                c_obs = compute_C_observed(achieved, trimmer_rate, n_full)
                if c_obs is not None and c_obs > 0:
                    C = recalibrate_C(C, c_obs)

            # (b) Coarse.
            high_sat = trimmer_rate >= C * 0.98
            low_sat = trimmer_rate <= C * 0.02
            sat_high_ticks = sat_high_ticks + 1 if high_sat else 0
            sat_low_ticks = sat_low_ticks + 1 if low_sat else 0

            decision = decide_coarse(
                n_full, trimmer_rate, achieved, target, C, cap,
                sat_high_ticks, sat_low_ticks, sat_ticks_needed=sat_ticks_needed,
            )
            decisions_history.append(decision)
            if decision == "add":
                n_full += 1
                trimmer_rate = max(0.0, min(C, trimmer_rate - C))
                sat_high_ticks = 0
                sat_low_ticks = 0
            elif decision == "remove":
                n_full -= 1
                trimmer_rate = max(0.0, min(C, trimmer_rate + C))
                sat_high_ticks = 0
                sat_low_ticks = 0

            # (c) Fine.
            trimmer_rate = fine_trim(trimmer_rate, achieved, target, C)

            achieved_history.append(achieved)

        return achieved_history, decisions_history, n_full, trimmer_rate, C

    def test_bisect100_scenario_converges_without_sustained_oscillation(self):
        # The motivating incident: C is underestimated (9400) vs the real
        # per-worker throughput (10400), target 10000. The old bang-bang
        # loop limit-cycled (kill -> collapse to trimmer-only -> add ->
        # overshoot -> kill). The cascade loop must settle down.
        achieved_history, decisions_history, n_full, trimmer_rate, C = self._simulate(
            real_per_worker=10400.0, target=10000.0, C_initial=9400.0, cap=5,
        )

        final_achieved = achieved_history[-1]
        self.assertLess(abs(final_achieved - 10000.0), 10000.0 * 0.05)

        # No sustained oscillation: the tail of the run should be a tight
        # band, not swinging between overshoot/undershoot extremes.
        tail = achieved_history[-8:]
        self.assertLess(max(tail) - min(tail), 10000.0 * 0.05)

        # No repeated add/remove flapping in the tail (a coarse action is
        # fine once while settling, but not a sustained back-and-forth).
        tail_decisions = decisions_history[-8:]
        flips = sum(
            1 for a, b in zip(tail_decisions, tail_decisions[1:])
            if {a, b} == {"add", "remove"}
        )
        self.assertEqual(flips, 0)

    def test_cluster_bound_holds_at_cap_without_spiral(self):
        # achieved permanently stuck low (cluster-bound, not worker-bound):
        # model a real cluster ceiling that doesn't scale with worker count
        # (the real incident: target 10k, calibration C~3279, cluster
        # tips over well below what the math says n_full should achieve).
        # The coarse knob should wind up to the reactive cap and then HOLD
        # -- never spiral past it -- exactly the anti-spiral property
        # reactive_worker_cap exists for.
        achieved_history, decisions_history, n_full, trimmer_rate, C = self._simulate(
            real_per_worker=3279.0, target=10000.0, C_initial=3279.0, cap=6,
            cluster_ceiling=3300.0, ticks=60,
        )
        # Bounded: never exceeds the cap.
        self.assertLessEqual(n_full, 6)
        # Once at the cap, no further coarse action fires (reported
        # honestly at a bounded shortfall rather than spiraling).
        self.assertEqual(n_full, 6)
        self.assertEqual(decisions_history[-5:], ["hold"] * 5)
        # No add/remove flapping anywhere in the run -- only "add"s (winding
        # up to the cap) ever fire, never a "remove" chasing it back down.
        self.assertNotIn("remove", decisions_history)

    def test_target_below_C_uses_pure_trimmer_no_coarse_action(self):
        # target < C: 0 full workers, trimmer converges toward target via
        # the integral loop alone; no coarse action should ever fire.
        achieved_history, decisions_history, n_full, trimmer_rate, C = self._simulate(
            real_per_worker=7000.0, target=1500.0, C_initial=7000.0, cap=5, ticks=15,
        )
        self.assertEqual(n_full, 0)
        self.assertTrue(all(d == "hold" for d in decisions_history))
        self.assertLess(abs(achieved_history[-1] - 1500.0), 1500.0 * 0.05)


class TestCrossNodeProgressTracker(unittest.TestCase):
    """Forward-progress accounting: counts only positive per-node deltas, so a
    decrease (restart OR pg_stat_statements eviction) never inflates the total.
    """

    # The first reading of a host is a BASELINE (counts 0); subsequent
    # readings count only their positive delta. pg_stat_statements is reset to
    # ~0 at start, so counted() tracks real rows written since the run began.

    def test_forward_progress_accumulates(self):
        t = CrossNodeProgressTracker()
        t.observe("a", 100.0)          # baseline
        self.assertEqual(t.counted(), 0.0)
        t.observe("a", 250.0)          # +150
        self.assertEqual(t.counted(), 150.0)
        t.observe("a", 300.0)          # +50
        self.assertEqual(t.counted(), 200.0)

    def test_eviction_partial_drop_does_not_inflate(self):
        # THE bug: a partial drop (LRU eviction, not a reset) must NOT be
        # banked. Old design turned 300 -> 295 into +295; forward-only adds 0.
        t = CrossNodeProgressTracker()
        t.observe("a", 100.0)          # baseline
        t.observe("a", 300.0)          # +200
        t.observe("a", 295.0)          # eviction drop -> +0
        self.assertEqual(t.counted(), 200.0)
        t.observe("a", 350.0)          # 295 -> 350 = +55
        self.assertEqual(t.counted(), 255.0)   # NOT inflated by the dip

    def test_repeated_evictions_do_not_runaway(self):
        # The exact heavy-run failure mode: many small drops. Forward-only
        # total stays bounded; old banking would explode into the millions.
        t = CrossNodeProgressTracker()
        t.observe("a", 50.0)           # baseline
        t.observe("a", 100.0)          # +50
        t.observe("a", 90.0)           # drop, +0 (re-base to 90)
        t.observe("a", 95.0)           # +5
        t.observe("a", 85.0)           # drop, +0 (re-base to 85)
        t.observe("a", 120.0)          # 120-85 = +35
        self.assertEqual(t.counted(), 90.0)   # bounded; old banking -> ~235+

    def test_restart_to_zero_counts_only_new_progress(self):
        t = CrossNodeProgressTracker()
        t.observe("a", 100.0)          # baseline
        t.observe("a", 300.0)          # +200
        t.observe("a", 0.0)            # restart -> +0
        self.assertEqual(t.counted(), 200.0)
        t.observe("a", 50.0)           # new post-restart rows: +50
        self.assertEqual(t.counted(), 250.0)

    def test_one_node_reset_does_not_zero_healthy_nodes(self):
        # The original goal, preserved: node a bounces while b keeps writing;
        # the cluster total must still rise from b's progress, never collapse.
        t = CrossNodeProgressTracker()
        t.observe("a", 100.0)          # baselines
        t.observe("b", 100.0)
        t.observe("a", 300.0)          # +200
        t.observe("b", 200.0)          # +100
        self.assertEqual(t.counted(), 300.0)
        t.observe("a", 0.0)            # a restarted: +0
        t.observe("b", 260.0)          # b grew: +60
        self.assertEqual(t.counted(), 360.0)   # rose, not zeroed

    def test_failed_poll_skipped_then_catches_up(self):
        # A failed poll simply isn't observed; the next reading counts the real
        # delta across the gap (no loss, no double-count).
        t = CrossNodeProgressTracker()
        t.observe("a", 100.0)          # baseline
        t.observe("a", 150.0)          # +50
        # (poll fails -> caller does not call observe)
        t.observe("a", 230.0)          # +80 across the gap
        self.assertEqual(t.counted(), 130.0)

    def test_reset_clears_all_state(self):
        t = CrossNodeProgressTracker()
        t.observe("a", 100.0)
        t.observe("a", 300.0)          # counted 200
        t.reset()
        t.observe("a", 50.0)           # fresh baseline
        t.observe("a", 70.0)           # +20
        self.assertEqual(t.counted(), 20.0)


class TestTrailingRateWindow(unittest.TestCase):
    """TrailingRateWindow: moving-average rate over a trailing window of a
    monotonic cumulative counter -- denoises the meter before it drives the
    controller."""

    def test_first_sample_returns_zero(self):
        w = TrailingRateWindow(15.0)
        self.assertEqual(w.update(0.0, 1000.0), 0.0)

    def test_steady_rate(self):
        # 1000 ev per 5s = 200 ev/s, constant.
        w = TrailingRateWindow(15.0)
        w.update(0.0, 0.0)
        self.assertAlmostEqual(w.update(5.0, 1000.0), 200.0)
        self.assertAlmostEqual(w.update(10.0, 2000.0), 200.0)
        self.assertAlmostEqual(w.update(15.0, 3000.0), 200.0)

    def test_smooths_a_spike_in_one_interval(self):
        # A single jumpy interval is averaged over the window rather than
        # reported at its raw instantaneous value.
        w = TrailingRateWindow(15.0)
        w.update(0.0, 0.0)
        w.update(5.0, 1000.0)      # 200/s
        w.update(10.0, 2000.0)     # 200/s
        # A big jump this interval: +4000 in 5s = 4000/s instantaneous...
        smoothed = w.update(15.0, 6000.0)
        # ...but windowed over 15s it is 6000/15 = 400/s, far below 4000.
        self.assertAlmostEqual(smoothed, 400.0)
        self.assertLess(smoothed, 4000.0)

    def test_window_drops_old_samples(self):
        # After enough time, only ~window_seconds of history is used.
        w = TrailingRateWindow(15.0)
        for i in range(0, 40, 5):
            w.update(float(i), float(i) * 200.0)  # steady 200/s
        # Steady input -> steady output regardless of how many samples elapsed.
        self.assertAlmostEqual(w.update(40.0, 8000.0), 200.0)
        # Oldest retained sample should be within ~one interval of the window.
        self.assertLessEqual(40.0 - w._samples[0][0], 15.0 + 5.0)

    def test_never_negative_on_flat_counter(self):
        w = TrailingRateWindow(15.0)
        w.update(0.0, 5000.0)
        self.assertEqual(w.update(5.0, 5000.0), 0.0)  # no progress -> 0, not negative

    def test_warmup_averages_available_history(self):
        # Before a full window exists, it averages over what it has.
        w = TrailingRateWindow(60.0)
        w.update(0.0, 0.0)
        self.assertAlmostEqual(w.update(5.0, 500.0), 100.0)  # 500/5s

    def test_rejects_nonpositive_window(self):
        with self.assertRaises(ValueError):
            TrailingRateWindow(0)
        with self.assertRaises(ValueError):
            TrailingRateWindow(-5)


class TestDecideOvershootShed(unittest.TestCase):
    """decide_overshoot_shed: the cautious fast-shed safety net for a large,
    sustained overshoot (e.g. after a cold-C phase-change over-provision)."""

    def test_no_fire_when_not_enough_overshoot_ticks(self):
        n_shed, c_snap = decide_overshoot_shed(
            n_uncapped=22, achieved=24000.0, trimmer_rate=0.0, target=10000.0,
            overshoot_ticks=1, overshoot_ticks_needed=2,
        )
        self.assertEqual(n_shed, 0)
        self.assertIsNone(c_snap)

    def test_no_fire_when_within_overshoot_threshold(self):
        # achieved = 1.1 * target -- under the default 25% threshold.
        n_shed, c_snap = decide_overshoot_shed(
            n_uncapped=22, achieved=11000.0, trimmer_rate=0.0, target=10000.0,
            overshoot_ticks=5, overshoot_ticks_needed=2,
        )
        self.assertEqual(n_shed, 0)
        self.assertIsNone(c_snap)

    def test_no_fire_when_fewer_than_two_uncapped(self):
        n_shed, c_snap = decide_overshoot_shed(
            n_uncapped=1, achieved=24000.0, trimmer_rate=0.0, target=10000.0,
            overshoot_ticks=5, overshoot_ticks_needed=2,
        )
        self.assertEqual(n_shed, 0)
        self.assertIsNone(c_snap)

    def test_fires_on_sustained_large_overshoot_bounded_by_max_shed_frac(self):
        # n_uncapped=10, C_obs = 24000/10 = 2400; desired_n = floor(10000/2400)
        # = 4; raw_shed = 6; max_shed = floor(10*0.5) = 5 -> bounded to 5.
        n_shed, c_snap = decide_overshoot_shed(
            n_uncapped=10, achieved=24000.0, trimmer_rate=0.0, target=10000.0,
            overshoot_ticks=2, overshoot_ticks_needed=2,
        )
        self.assertEqual(n_shed, 5)
        self.assertAlmostEqual(c_snap, 2400.0)

    def test_incident_realistic_case(self):
        # Mirrors the incident: cold C over-provisioned 22 workers for a
        # 10k target; achieved overshoots to 24000.
        n_shed, c_snap = decide_overshoot_shed(
            n_uncapped=22, achieved=24000.0, trimmer_rate=0.0, target=10000.0,
            overshoot_ticks=2, overshoot_ticks_needed=2,
        )
        self.assertGreater(n_shed, 0)
        self.assertLessEqual(n_shed, 11)  # floor(22 * 0.5)
        self.assertAlmostEqual(c_snap, 24000.0 / 22.0)

    def test_division_by_zero_guard(self):
        # achieved - trimmer_rate <= 0 (trimmer alone accounts for all of it,
        # or more) -- must not raise, must return (0, None).
        n_shed, c_snap = decide_overshoot_shed(
            n_uncapped=5, achieved=13000.0, trimmer_rate=13000.0, target=10000.0,
            overshoot_ticks=5, overshoot_ticks_needed=2,
        )
        self.assertEqual(n_shed, 0)
        self.assertIsNone(c_snap)

        n_shed2, c_snap2 = decide_overshoot_shed(
            n_uncapped=5, achieved=13000.0, trimmer_rate=14000.0, target=10000.0,
            overshoot_ticks=5, overshoot_ticks_needed=2,
        )
        self.assertEqual(n_shed2, 0)
        self.assertIsNone(c_snap2)


class _FakeProc(object):
    """Minimal Popen stand-in: never exits during calibration."""

    def poll(self):
        return None


class TestRunCalibrationWarmup(unittest.TestCase):
    """run_calibration: the warmup-before-measuring behavior.
    `parallel_generator.time.sleep`/`time.monotonic` are monkeypatched to
    fake, controlled values so the test doesn't actually sleep and the
    ordering of warmup-sleep vs. the measurement window can be asserted
    directly, per IMPLEMENTATION_CONTRACTS.md's testing rule (no real clock).
    """

    def setUp(self):
        self._orig_sleep = parallel_generator.time.sleep
        self._orig_monotonic = parallel_generator.time.monotonic
        self.calls = []

        def fake_sleep(seconds):
            self.calls.append(("sleep", seconds))

        parallel_generator.time.sleep = fake_sleep
        self._monotonic_values = iter([100.0, 130.0])
        parallel_generator.time.monotonic = lambda: next(self._monotonic_values)

    def tearDown(self):
        parallel_generator.time.sleep = self._orig_sleep
        parallel_generator.time.monotonic = self._orig_monotonic

    def _make_spawn_kill(self):
        spawned = []
        killed = []

        def spawn_worker(throttle=0.0):
            uid = 1
            spawned.append(uid)
            return uid, _FakeProc()

        def kill_worker(uid):
            killed.append(uid)

        return spawn_worker, kill_worker, spawned, killed

    def test_warmup_zero_is_legacy_behavior(self):
        spawn_worker, kill_worker, spawned, killed = self._make_spawn_kill()
        measure_values = iter([1000.0, 2500.0])

        def measure_fn():
            v = next(measure_values)
            self.calls.append(("measure", v))
            return v

        C = run_calibration(
            spawn_worker, kill_worker, measure_fn, calibration_seconds=30,
            warmup_seconds=0,
        )

        # (2500 - 1000) / (130 - 100) = 50.0
        self.assertAlmostEqual(C, 50.0)
        # Exactly one sleep call, for calibration_seconds -- no warmup sleep.
        self.assertEqual(self.calls, [
            ("measure", 1000.0),
            ("sleep", 30),
            ("measure", 2500.0),
        ])
        self.assertEqual(spawned, [1])
        self.assertEqual(killed, [1])

    def test_warmup_positive_sleeps_before_measuring(self):
        spawn_worker, kill_worker, spawned, killed = self._make_spawn_kill()
        # Post-warmup samples only: the pre-warmup rows the cold worker
        # produced are never observed by measure_fn at all.
        measure_values = iter([5000.0, 6500.0])

        def measure_fn():
            v = next(measure_values)
            self.calls.append(("measure", v))
            return v

        C = run_calibration(
            spawn_worker, kill_worker, measure_fn, calibration_seconds=30,
            warmup_seconds=20,
        )

        self.assertAlmostEqual(C, 50.0)
        # Warmup sleep happens first, THEN start_sum is measured, THEN the
        # calibration sleep, THEN end_sum -- proving the measurement window
        # starts only after the worker has warmed up.
        self.assertEqual(self.calls, [
            ("sleep", 20),
            ("measure", 5000.0),
            ("sleep", 30),
            ("measure", 6500.0),
        ])
        self.assertEqual(spawned, [1])
        self.assertEqual(killed, [1])


if __name__ == "__main__":
    unittest.main()
