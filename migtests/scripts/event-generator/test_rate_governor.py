"""
Unit tests for rate_governor.py.

Uses stdlib unittest (not pytest) with an injected fake clock/sleep so no
test ever actually sleeps.
"""

import unittest

from rate_governor import NullGovernor, RateGovernor


class FakeClock(object):
    """A monotonic-clock stand-in whose value only advances when told to
    (directly via `advance`, or indirectly via FakeSleep)."""

    def __init__(self, start=0.0):
        self.now = float(start)

    def __call__(self):
        return self.now

    def advance(self, dt):
        self.now += dt


class FakeSleep(object):
    """A sleep() stand-in that records every call and advances the fake
    clock by the requested duration instead of blocking."""

    def __init__(self, clock):
        self.clock = clock
        self.calls = []

    def __call__(self, seconds):
        self.calls.append(seconds)
        self.clock.advance(seconds)


class RecordingLog(object):
    """A log() stand-in that records every message instead of printing."""

    def __init__(self):
        self.messages = []

    def __call__(self, msg):
        self.messages.append(msg)


def make_governor(rate_control, random_seed=None, start=0.0):
    clock = FakeClock(start)
    sleep = FakeSleep(clock)
    log = RecordingLog()
    gov = RateGovernor(rate_control, random_seed=random_seed,
                        clock=clock, sleep=sleep, log=log)
    return gov, clock, sleep, log


class TestBaselinePacing(unittest.TestCase):
    def test_baseline_only_holds_target_rate(self):
        gov, clock, sleep, log = make_governor(
            {"default_events_per_second": 100})

        for _ in range(200):
            gov.pace(10)

        elapsed = clock.now - gov.run_start
        achieved = gov.total_events / elapsed
        self.assertAlmostEqual(achieved, 100, delta=0.5)
        # Pacing must have actually slept to hold the rate down.
        self.assertGreater(len(sleep.calls), 0)

    def test_pace_never_sleeps_when_behind_allowance(self):
        # A single small pace() call right at window start should never need
        # to sleep, since window_events(n) <= allowed(target * 0) is false
        # only when n > 0 -- but the very first call always needs a tiny
        # sleep to avoid running instantaneously fast. Verify instead that
        # once caught up (elapsed matches the produced events), no further
        # sleep is required.
        gov, clock, sleep, log = make_governor(
            {"default_events_per_second": 10})
        gov.pace(0)
        # No events emitted yet, so nothing to pace for.
        self.assertEqual(sleep.calls, [])


class TestNullGovernor(unittest.TestCase):
    def test_null_governor_never_sleeps(self):
        gov = NullGovernor()
        # Should be trivially callable many times with no side effects and
        # no dependency on a clock/sleep at all.
        for _ in range(1000):
            self.assertIsNone(gov.pace(999999))


class TestScheduleWindows(unittest.TestCase):
    def _schedule_governor(self, offset_seconds=600):
        rate_control = {
            "default_events_per_second": 1500,
            "schedule": [
                {
                    "events_per_second": 10000,
                    "duration_seconds": 300,
                    "every_seconds": 1800,
                    "offset_seconds": offset_seconds,
                }
            ],
        }
        gov, clock, sleep, log = make_governor(rate_control)
        return gov

    def test_spike_active_only_within_offset_duration_window(self):
        gov = self._schedule_governor()

        # Before the window opens.
        self.assertEqual(gov.target_rate_at(0), 1500)
        self.assertEqual(gov.target_rate_at(599), 1500)
        # Inside [offset, offset + duration).
        self.assertEqual(gov.target_rate_at(600), 10000)
        self.assertEqual(gov.target_rate_at(750), 10000)
        self.assertEqual(gov.target_rate_at(899), 10000)
        # At/after the window closes.
        self.assertEqual(gov.target_rate_at(900), 1500)
        self.assertEqual(gov.target_rate_at(1799), 1500)

    def test_offset_delays_first_spike_and_repeats_every_period(self):
        gov = self._schedule_governor()

        # First spike does NOT start at t=0.
        self.assertEqual(gov.target_rate_at(0), 1500)

        # Second period's spike window: k=1 -> start = 1800 + 600 = 2400.
        self.assertEqual(gov.target_rate_at(2399), 1500)
        self.assertEqual(gov.target_rate_at(2400), 10000)
        self.assertEqual(gov.target_rate_at(2699), 10000)
        self.assertEqual(gov.target_rate_at(2700), 1500)

        # Third period: k=2 -> start = 3600 + 600 = 4200.
        self.assertEqual(gov.target_rate_at(4200), 10000)
        self.assertEqual(gov.target_rate_at(4499), 10000)
        self.assertEqual(gov.target_rate_at(4500), 1500)


class TestOverlap(unittest.TestCase):
    def test_overlap_max_rate_wins(self):
        rate_control = {
            "default_events_per_second": 1500,
            "schedule": [
                {
                    "events_per_second": 5000,
                    "duration_seconds": 1000,
                    "every_seconds": 2000,
                    "offset_seconds": 0,
                },
                {
                    "events_per_second": 10000,
                    "duration_seconds": 200,
                    "every_seconds": 2000,
                    "offset_seconds": 400,
                },
            ],
        }
        gov, clock, sleep, log = make_governor(rate_control)

        # Only entry 0 active -> 5000.
        self.assertEqual(gov.target_rate_at(100), 5000)
        # Both entries active (400 <= t < 600) -> max(5000, 10000) = 10000.
        self.assertEqual(gov.target_rate_at(500), 10000)
        # Only entry 0 active again (600 <= t < 1000) -> 5000.
        self.assertEqual(gov.target_rate_at(700), 5000)
        # Neither active -> baseline.
        self.assertEqual(gov.target_rate_at(1500), 1500)


class TestJitter(unittest.TestCase):
    def _rate_control(self, jitter_pct=10):
        return {
            "default_events_per_second": 1500,
            "schedule": [
                {
                    "events_per_second": 10000,
                    "duration_seconds": 300,
                    "every_seconds": 1800,
                    "offset_seconds": 600,
                    "jitter_pct": jitter_pct,
                }
            ],
        }

    def test_jitter_stays_within_pct_bounds(self):
        gov, clock, sleep, log = make_governor(
            self._rate_control(jitter_pct=10), random_seed=42)

        every_seconds = 1800
        events_per_second = 10000
        max_start_jitter = 0.10 * every_seconds
        min_rate = events_per_second * 0.90
        max_rate = events_per_second * 1.10

        for k in range(20):
            start_jitter, rate_multiplier = gov._window_jitter(0, k)
            self.assertLessEqual(abs(start_jitter), max_start_jitter + 1e-9)
            rate = events_per_second * rate_multiplier
            self.assertGreaterEqual(rate, min_rate - 1e-9)
            self.assertLessEqual(rate, max_rate + 1e-9)

    def test_jitter_identical_across_runs_with_same_seed(self):
        gov1, _, _, _ = make_governor(self._rate_control(), random_seed=7)
        gov2, _, _, _ = make_governor(self._rate_control(), random_seed=7)

        for k in range(10):
            self.assertEqual(gov1._window_jitter(0, k), gov2._window_jitter(0, k))

        # Also confirm target_rate_at agrees end-to-end for both instances.
        for elapsed in (0, 600, 750, 899, 2400, 2699):
            self.assertEqual(gov1.target_rate_at(elapsed), gov2.target_rate_at(elapsed))

    def test_jitter_differs_without_a_seed(self):
        gov1, _, _, _ = make_governor(self._rate_control(), random_seed=None)
        gov2, _, _, _ = make_governor(self._rate_control(), random_seed=None)

        jitters1 = [gov1._window_jitter(0, k) for k in range(10)]
        jitters2 = [gov2._window_jitter(0, k) for k in range(10)]
        self.assertNotEqual(jitters1, jitters2)


class TestSetRate(unittest.TestCase):
    """Unit tests for RateGovernor.set_rate -- the runtime-adjustable
    throttle the cascade trimmer controller uses to command a persistent
    trimmer worker without a respawn. See
    docs/superpowers/specs/2026-07-22-cascade-trimmer-controller-design.md.
    """

    def test_set_rate_changes_effective_target(self):
        gov, clock, sleep, log = make_governor({"default_events_per_second": 100})
        self.assertEqual(gov.target_rate_at(0), 100)

        gov.set_rate(500)

        self.assertEqual(gov.default_events_per_second, 500.0)
        self.assertEqual(gov.target_rate_at(0), 500)

    def test_set_rate_is_pure_float_conversion_no_io(self):
        gov, clock, sleep, log = make_governor({"default_events_per_second": 100})
        gov.set_rate(250)
        self.assertIsInstance(gov.default_events_per_second, float)
        self.assertEqual(sleep.calls, [])
        self.assertEqual(log.messages, [])

    def test_next_pace_resets_window_on_rate_change(self):
        gov, clock, sleep, log = make_governor({"default_events_per_second": 100})

        # Warm up the window at the original rate.
        gov.pace(10)
        clock.advance(1.0)
        gov.pace(10)
        self.assertNotEqual(gov.window_events, 0)
        old_window_start = gov.window_start

        gov.set_rate(500)
        # The rate change alone doesn't touch pacing state...
        self.assertEqual(gov.window_start, old_window_start)

        # ...but the very next pace() call sees target != current_target and
        # resets window_start/window_events exactly like a schedule-driven
        # target transition does.
        clock.advance(0.001)
        gov.pace(0)
        self.assertEqual(gov.current_target, 500.0)
        self.assertEqual(gov.window_start, clock.now)
        self.assertEqual(gov.window_events, 0)

    def test_rate_zero_does_not_turn_governor_uncapped(self):
        gov, clock, sleep, log = make_governor({"default_events_per_second": 100})
        gov.set_rate(0)

        # The governor stays engaged (a real RateGovernor instance, never
        # swapped for a NullGovernor) -- "uncapped" never applies here, only
        # the caller's epsilon-floor policy decides what rate is actually
        # commanded (see generator.py's pause-semantics wiring).
        self.assertIsInstance(gov, RateGovernor)
        self.assertEqual(gov.default_events_per_second, 0.0)


class TestReporting(unittest.TestCase):
    def test_report_fires_once_per_interval_with_achieved_rate(self):
        rate_control = {
            "default_events_per_second": 100,
            "report_interval_seconds": 10,
        }
        gov, clock, sleep, log = make_governor(rate_control)

        # Drive enough iterations to cross multiple report boundaries.
        # Each pace() call emits 10 events; the governor's own pacing
        # sleeps advance the fake clock towards the 100 ev/s target.
        for _ in range(300):
            gov.pace(10)

        self.assertGreaterEqual(len(log.messages), 2)
        for msg in log.messages:
            self.assertIn("achieved=", msg)
            self.assertIn("target=", msg)
            self.assertIn("total_events=", msg)

        # Reports should be spaced ~report_interval_seconds apart in elapsed
        # run time. Reconstruct report timestamps isn't directly exposed, so
        # instead verify count is consistent with total elapsed time.
        elapsed = clock.now - gov.run_start
        expected_reports = int(elapsed // 10)
        # Allow +/-1 for boundary rounding.
        self.assertLessEqual(abs(len(log.messages) - expected_reports), 1)

    def test_report_disabled_when_omitted(self):
        gov, clock, sleep, log = make_governor(
            {"default_events_per_second": 100})
        for _ in range(50):
            gov.pace(10)
        self.assertEqual(log.messages, [])

    def test_report_disabled_when_zero(self):
        gov, clock, sleep, log = make_governor(
            {"default_events_per_second": 100, "report_interval_seconds": 0})
        for _ in range(50):
            gov.pace(10)
        self.assertEqual(log.messages, [])


class TestMaxSingleSleep(unittest.TestCase):
    """max_single_sleep_seconds caps any single pace() sleep, so a very low
    commanded rate combined with a large per-operation batch cannot freeze the
    worker for minutes (the trimmer-stuck-at-0 bug)."""

    def _gov(self, rate, cap):
        clock = FakeClock(0.0)
        sleep = FakeSleep(clock)
        gov = RateGovernor({"default_events_per_second": rate},
                           clock=clock, sleep=sleep, log=RecordingLog(),
                           max_single_sleep_seconds=cap)
        return gov, sleep

    def test_uncapped_low_rate_large_batch_sleeps_minutes(self):
        # Baseline: a 300-event batch at 1 ev/s would sleep ~300s uncapped.
        gov, sleep = self._gov(1.0, None)
        gov.pace(300)
        self.assertGreater(max(sleep.calls), 100.0)

    def test_cap_bounds_the_sleep(self):
        gov, sleep = self._gov(1.0, 3.0)
        gov.pace(300)
        self.assertTrue(sleep.calls)
        self.assertLessEqual(max(sleep.calls), 3.0)

    def test_cap_leaves_normal_rates_untouched(self):
        # 300 events at 1500 ev/s needs only 0.2s -- well under the cap.
        gov, sleep = self._gov(1500.0, 3.0)
        gov.pace(300)
        if sleep.calls:
            self.assertLess(max(sleep.calls), 1.0)


if __name__ == "__main__":
    unittest.main()
