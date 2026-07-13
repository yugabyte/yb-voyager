"""
Unit tests for the pure functions in parallel_generator.py.

Only compute_workers, peak_target, and derive_worker_config are covered
here -- these are side-effect-free (no subprocess, no DB, no file I/O), by
design, so they can be tested in isolation. Runtime orchestration (spawn,
monitor, calibration) is deliberately not exercised, since this environment
has no live DB and generator.py must not be run against one.
"""

import copy
import unittest

from parallel_generator import compute_workers, derive_worker_config, peak_target


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

    def test_default_higher_than_all_spikes(self):
        rate_control = {
            "default_events_per_second": 5000,
            "schedule": [
                {"events_per_second": 100, "duration_seconds": 10, "every_seconds": 60},
            ],
        }
        self.assertEqual(peak_target(rate_control), 5000)

    def test_empty_schedule_list(self):
        rate_control = {"default_events_per_second": 200, "schedule": []}
        self.assertEqual(peak_target(rate_control), 200)


class TestComputeWorkers(unittest.TestCase):
    def test_exact_target_no_margin(self):
        # peak=100, ceiling=50, margin=1.0 -> exactly 2 workers, reachable.
        workers, reachable = compute_workers(
            peak_target_value=100, per_worker_ceiling=50, max_workers=10, margin=1.0
        )
        self.assertEqual(workers, 2)
        self.assertTrue(reachable)

    def test_needs_rounding_up(self):
        # peak=101, ceiling=50, margin=1.0 -> 101/50 = 2.02 -> ceil to 3.
        workers, reachable = compute_workers(
            peak_target_value=101, per_worker_ceiling=50, max_workers=10, margin=1.0
        )
        self.assertEqual(workers, 3)
        self.assertTrue(reachable)

    def test_clamps_at_max_workers_and_marks_unreachable(self):
        # peak=10000, ceiling=50, margin=1.0 -> would need 200 workers, but
        # max_workers=4 clamps it; 4*50=200 < 10000, so unreachable.
        workers, reachable = compute_workers(
            peak_target_value=10000, per_worker_ceiling=50, max_workers=4, margin=1.0
        )
        self.assertEqual(workers, 4)
        self.assertFalse(reachable)

    def test_single_worker_case(self):
        # peak=10, ceiling=1000, margin=1.3 -> ceil(13/1000)=1 -> single worker.
        workers, reachable = compute_workers(
            peak_target_value=10, per_worker_ceiling=1000, max_workers=6, margin=1.3
        )
        self.assertEqual(workers, 1)
        self.assertTrue(reachable)

    def test_margin_applied_can_push_an_extra_worker(self):
        # peak=100, ceiling=100: without margin this is exactly 1 worker,
        # but margin=1.3 inflates the target to 130, requiring 2 workers.
        workers_no_margin, _ = compute_workers(
            peak_target_value=100, per_worker_ceiling=100, max_workers=10, margin=1.0
        )
        workers_with_margin, reachable = compute_workers(
            peak_target_value=100, per_worker_ceiling=100, max_workers=10, margin=1.3
        )
        self.assertEqual(workers_no_margin, 1)
        self.assertEqual(workers_with_margin, 2)
        self.assertTrue(reachable)

    def test_clamp_still_reachable_is_marked_true(self):
        # peak=100, ceiling=60, margin=1.0 -> ceil(100/60)=2, clamp at
        # max_workers=1 -> but 1*60=60 < 100, so still unreachable.
        # This case exists to make sure the reachable flag is recomputed
        # against the clamped worker count, not left stale from before the
        # clamp when that clamp happens to already meet the raw target.
        workers, reachable = compute_workers(
            peak_target_value=50, per_worker_ceiling=60, max_workers=1, margin=1.3
        )
        self.assertEqual(workers, 1)
        # 1*60=60 >= 50 -> reachable even though clamped.
        self.assertTrue(reachable)

    def test_invalid_ceiling_raises(self):
        with self.assertRaises(ValueError):
            compute_workers(peak_target_value=100, per_worker_ceiling=0, max_workers=6, margin=1.3)

    def test_invalid_max_workers_raises(self):
        with self.assertRaises(ValueError):
            compute_workers(peak_target_value=100, per_worker_ceiling=50, max_workers=0, margin=1.3)


class TestDeriveWorkerConfig(unittest.TestCase):
    def _base_config(self):
        return {
            "connection": {
                "host": "localhost",
                "port": 5432,
                "database": "sakila",
                "user": "postgres",
                "password": "postgres",
            },
            "generator": {
                "schema_name": "public",
                "manual_table_list": ["eg_users"],
                "exclude_table_list": [],
                "num_iterations": -1,
                "wait_after_operations": 0,
                "wait_duration_seconds": 0,
                "table_weights": {"eg_users": 100},
                "operation_weights": {"INSERT": 3, "UPDATE": 2, "DELETE": 1},
                "insert_rows": 4,
                "update_rows": 2,
                "delete_rows": 1,
                "insert_max_retries": 50,
                "update_max_retries": 3,
                "random_seed": 12345,
                "faker_seed": 12345,
                "rate_control": {
                    "default_events_per_second": 1500,
                    "report_interval_seconds": 60,
                    "schedule": [
                        {
                            "events_per_second": 10000,
                            "duration_seconds": 300,
                            "every_seconds": 1800,
                            "offset_seconds": 600,
                            "jitter_pct": 10,
                        }
                    ],
                },
            },
            "parallel": {
                "max_workers": 6,
                "calibration_seconds": 30,
                "margin": 1.3,
                "run_seconds": 1800,
                "monitor_interval_seconds": 5,
            },
        }

    def test_seeds_are_base_plus_index(self):
        base = self._base_config()
        cfg0 = derive_worker_config(base, worker_index=0, workers=3, base_seed=100)
        cfg1 = derive_worker_config(base, worker_index=1, workers=3, base_seed=100)
        cfg2 = derive_worker_config(base, worker_index=2, workers=3, base_seed=100)

        self.assertEqual(cfg0["generator"]["random_seed"], 100)
        self.assertEqual(cfg0["generator"]["faker_seed"], 100)
        self.assertEqual(cfg1["generator"]["random_seed"], 101)
        self.assertEqual(cfg1["generator"]["faker_seed"], 101)
        self.assertEqual(cfg2["generator"]["random_seed"], 102)
        self.assertEqual(cfg2["generator"]["faker_seed"], 102)

    def test_rates_divided_by_worker_count(self):
        base = self._base_config()
        cfg = derive_worker_config(base, worker_index=0, workers=3, base_seed=0)

        rc = cfg["generator"]["rate_control"]
        # 1500 / 3 = 500 exactly.
        self.assertEqual(rc["default_events_per_second"], 500)
        # 10000 / 3 = 3333.33... -> rounds to 3333.
        self.assertEqual(rc["schedule"][0]["events_per_second"], 3333)

    def test_rate_floored_to_minimum_one(self):
        base = self._base_config()
        base["generator"]["rate_control"]["default_events_per_second"] = 2
        base["generator"]["rate_control"]["schedule"][0]["events_per_second"] = 1
        cfg = derive_worker_config(base, worker_index=0, workers=5, base_seed=0)

        rc = cfg["generator"]["rate_control"]
        self.assertEqual(rc["default_events_per_second"], 1)
        self.assertEqual(rc["schedule"][0]["events_per_second"], 1)

    def test_batch_sizes_and_weights_untouched(self):
        base = self._base_config()
        cfg = derive_worker_config(base, worker_index=1, workers=4, base_seed=0)

        gen = cfg["generator"]
        self.assertEqual(gen["insert_rows"], 4)
        self.assertEqual(gen["update_rows"], 2)
        self.assertEqual(gen["delete_rows"], 1)
        self.assertEqual(gen["operation_weights"], {"INSERT": 3, "UPDATE": 2, "DELETE": 1})
        self.assertEqual(gen["table_weights"], {"eg_users": 100})
        # Schedule durations/offsets/jitter must be untouched -- only the
        # rate itself is split.
        entry = gen["rate_control"]["schedule"][0]
        self.assertEqual(entry["duration_seconds"], 300)
        self.assertEqual(entry["every_seconds"], 1800)
        self.assertEqual(entry["offset_seconds"], 600)
        self.assertEqual(entry["jitter_pct"], 10)

    def test_parallel_block_removed(self):
        base = self._base_config()
        cfg = derive_worker_config(base, worker_index=0, workers=2, base_seed=0)
        self.assertNotIn("parallel", cfg)

    def test_base_config_not_mutated(self):
        base = self._base_config()
        original = copy.deepcopy(base)

        derive_worker_config(base, worker_index=0, workers=3, base_seed=0)

        self.assertEqual(base, original)

    def test_multiple_calls_do_not_interfere(self):
        # Deriving worker 1's config must not affect a previously-derived
        # worker 0's config (i.e. no shared mutable state / aliasing).
        base = self._base_config()
        cfg0 = derive_worker_config(base, worker_index=0, workers=2, base_seed=0)
        cfg1 = derive_worker_config(base, worker_index=1, workers=2, base_seed=0)

        self.assertEqual(cfg0["generator"]["random_seed"], 0)
        self.assertEqual(cfg1["generator"]["random_seed"], 1)
        self.assertEqual(cfg0["generator"]["rate_control"]["default_events_per_second"], 750)
        self.assertEqual(cfg1["generator"]["rate_control"]["default_events_per_second"], 750)


if __name__ == "__main__":
    unittest.main()
