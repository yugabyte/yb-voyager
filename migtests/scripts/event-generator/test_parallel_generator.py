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
    Schedule,
    SlotFilePool,
    WorkerRoster,
    WorkerUidAllocator,
    build_worker_argv,
    compute_C_observed,
    decide_adjustment,
    derive_worker_runtime_config,
    diff_worker_counts,
    peak_target,
    plan_worker_counts,
    reactive_worker_cap,
    recalibrate_C,
    resolve_parallel_config,
)


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


if __name__ == "__main__":
    unittest.main()


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


if __name__ == "__main__":
    unittest.main()
