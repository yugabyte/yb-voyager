"""
rate_governor.py — pacing engine for the event generator.

Pure stdlib (time, random) — no psycopg2/Faker/yaml. This module is imported
and unit-tested standalone, without a DB connection.

See docs/superpowers/specs/2026-07-13-event-generator-rate-governor-design.md
for the full design: config schema, semantics, and the pacing algorithm this
implements verbatim.
"""

import random
import time


class NullGovernor(object):
    """No-op governor used when `rate_control` is absent from config.

    Preserves today's behavior exactly: the generator runs as fast as the
    DB allows, with no pacing and no sleeping.
    """

    def pace(self, events_emitted):
        pass


class RateGovernor(object):
    """Piecewise-constant deadline pacing with a reset at each target
    transition.

    Holds a configured baseline `events_per_second` and layers recurring
    spike windows (the `schedule`) on top, with max-wins semantics when
    windows overlap. Jitter (start time and rate) is a deterministic pure
    function of (random_seed, entry_index, window_index).

    clock/sleep/log are injected so tests can use a fake clock and never
    actually sleep.
    """

    def __init__(self, rate_control, *, random_seed=None,
                 clock=time.monotonic, sleep=time.sleep, log=print,
                 wall_clock=time.time, max_single_sleep_seconds=None):
        # Cap on any single pace() sleep. Defense against a very low commanded
        # rate combined with a large per-operation batch (e.g. a 300-row INSERT
        # at 1 ev/s would otherwise sleep ~300s) freezing the worker so it can
        # never re-read its control file to see the rate go back up. None = no
        # cap (unit tests / legacy). The dynamic-worker trimmer sets a few
        # seconds. A capped sleep may over-produce at sub-cap rates, but the
        # controller measures that and steers the command down (to pause) --
        # far better than a multi-minute freeze.
        self.max_single_sleep_seconds = max_single_sleep_seconds
        self.default_events_per_second = rate_control["default_events_per_second"]
        self.report_interval = rate_control.get("report_interval_seconds", 0) or 0
        self.schedule = rate_control.get("schedule") or []
        self.random_seed = random_seed
        # Effective seed used for the deterministic jitter formula below. When
        # the caller supplies random_seed, this makes jitter a pure function
        # of (random_seed, entry_index, k), so two governors built with the
        # same random_seed reproduce identical jitter. When no random_seed is
        # given, draw one fresh value from the (OS-seeded) global `random`
        # module here, once, so jitter stays stable/pure for the lifetime of
        # this instance but differs across separate unseeded runs.
        self._effective_seed = random_seed if random_seed is not None else random.random()

        self._clock = clock
        self._sleep = sleep
        self._log = log
        # Wall-clock (real epoch) source, separate from the pacing `clock`
        # (monotonic). Only used to stamp report lines with an absolute time so
        # they can be joined against other timestamped series (monitor CSV,
        # Prometheus cdcsdk_flush_lag, ...) after the run. Injected for tests.
        self._wall_clock = wall_clock

        # Pacing state.
        self.run_start = self._clock()
        self.window_start = self.run_start
        self.window_events = 0
        self.current_target = None

        # Reporting state.
        self.total_events = 0
        self.last_report = self.run_start
        self.report_events = 0

    # ----- schedule / jitter helpers -----

    def _window_jitter(self, entry_index, k):
        """Return (start_jitter_seconds, rate_multiplier) for schedule entry
        `entry_index`'s window index `k`.

        A pure function of (random_seed, entry_index, k): two RateGovernor
        instances built with the same random_seed produce identical jitter
        for the same (entry_index, k), independent of call order or of any
        other entry/window.
        """
        entry = self.schedule[entry_index]
        jitter_pct = entry.get("jitter_pct", 0) or 0
        if jitter_pct == 0:
            return 0.0, 1.0

        # Independent RNG seeded per (effective_seed, entry_index, k) so the
        # jitter for one window never perturbs another's draw sequence.
        rng = random.Random((self._effective_seed, entry_index, k))
        start_u = rng.uniform(-1, 1)
        rate_u = rng.uniform(-1, 1)

        every_seconds = entry["every_seconds"]
        start_jitter = start_u * (jitter_pct / 100.0) * every_seconds
        rate_multiplier = 1 + rate_u * (jitter_pct / 100.0)
        return start_jitter, rate_multiplier

    def _entry_active_rate(self, entry_index, elapsed):
        """Return the jittered events_per_second for schedule entry
        `entry_index` if it is active at `elapsed`, else None.

        Checks window indices k-1, k, k+1 around the nominal
        k = floor((elapsed - offset) / every), since start jitter can shift
        a window's boundaries across the nominal index.
        """
        entry = self.schedule[entry_index]
        every_seconds = entry["every_seconds"]
        offset_seconds = entry.get("offset_seconds", 0) or 0
        duration_seconds = entry["duration_seconds"]
        events_per_second = entry["events_per_second"]

        nominal_k = int((elapsed - offset_seconds) // every_seconds)

        for k in (nominal_k - 1, nominal_k, nominal_k + 1):
            if k < 0:
                continue
            start_jitter, rate_multiplier = self._window_jitter(entry_index, k)
            window_begin = k * every_seconds + offset_seconds + start_jitter
            window_end = window_begin + duration_seconds
            if window_begin <= elapsed < window_end:
                return events_per_second * rate_multiplier

        return None

    def target_rate_at(self, elapsed):
        """Effective target rate at elapsed seconds since run_start: the
        max events_per_second among all active schedule entries, or
        default_events_per_second if none are active.
        """
        active_rates = []
        for entry_index in range(len(self.schedule)):
            rate = self._entry_active_rate(entry_index, elapsed)
            if rate is not None:
                active_rates.append(rate)

        if active_rates:
            return max(active_rates)
        return self.default_events_per_second

    # ----- runtime-adjustable throttle (cascade trimmer controller) -----

    def set_rate(self, rate):
        """Set the governor's baseline target to `rate` (events/sec).

        Pure; no I/O, no clock access. Used by the cascade trimmer
        controller's persistent "trimmer" worker (Option B: runtime-
        adjustable throttle via a control file re-read in the worker loop --
        see ARCHITECTURE.md)
        to change the commanded rate without a respawn: the next `pace()`
        call sees `target_rate_at` return this new value, which differs
        from `self.current_target`, so the existing reset branch clears
        `window_start`/`window_events` and the new rate takes effect
        cleanly. A trimmer never carries a schedule, so only
        `default_events_per_second` matters here.

        `rate` may be 0. This does NOT turn the governor uncapped/disengaged
        -- it stays a RateGovernor, just targeting 0. Callers that want
        "pause" semantics (emit ~nothing, but never uncapped, never exit)
        must floor the commanded rate to a small positive epsilon before
        calling this -- see generator.py's control-file wiring -- since
        `pace()`'s own `target > 0` guard skips rate enforcement entirely
        when the target is exactly 0.
        """
        self.default_events_per_second = float(rate)

    # ----- pacing -----

    def pace(self, events_emitted):
        now = self._clock()
        elapsed = now - self.run_start
        target = self.target_rate_at(elapsed)

        if target != self.current_target:
            self.window_start = now
            self.window_events = 0
            self.current_target = target

        self.window_events += events_emitted
        self.total_events += events_emitted
        self.report_events += events_emitted

        if target > 0:
            window_elapsed = now - self.window_start
            allowed = target * window_elapsed
            if self.window_events > allowed:
                sleep_needed = self.window_events / target - window_elapsed
                if self.max_single_sleep_seconds is not None:
                    sleep_needed = min(sleep_needed, self.max_single_sleep_seconds)
                if sleep_needed > 0:
                    self._sleep(sleep_needed)

        # Refresh the clock after any pacing sleep so the reported rate divides by
        # the true elapsed wall-clock (including the sleep), not the pre-sleep time.
        now = self._clock()

        if self.report_interval > 0 and now - self.last_report >= self.report_interval:
            elapsed_since_report = now - self.last_report
            achieved = self.report_events / elapsed_since_report if elapsed_since_report > 0 else 0.0
            wall = self._wall_clock()
            self._log(
                "[rate_governor] ts={:.3f} ({}) achieved={:.1f} ev/s target={} total_events={}".format(
                    wall, time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(wall)),
                    achieved, self.current_target, self.total_events
                )
            )
            self.report_events = 0
            self.last_report = now
