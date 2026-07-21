"""
parallel_generator.py -- the reactive worker-pool controller for the event
generator.

See ~/yb-ratetest/dynamic-worker-pool-design.md (sections 4-11) and
IMPLEMENTATION_CONTRACTS.md ("Controller") for the full design and the
exact contract this module implements. Summary:

  1. CALIBRATION: spawn one uncapped worker for `calibration_seconds`,
     measure the aggregate rate it produces (via pg_stat_statements) to
     get `C`, the per-worker throughput ceiling.
  2. Build the SHARED CACHE (schema metadata + PK snapshot, see
     shared_cache.py) before spawning any run worker, so every spawn is a
     fast, cache-backed start; refresh it in the background on a timer,
     atomically flipping to a new version -- a refresh never blocks
     spawning.
  3. A monotonic, controller-owned WORKER-UID allocator hands every spawn
     (including respawns) a fresh id, never reused for the life of the
     run.
  4. The CONTROL LOOP polls `schedule.target_at(now)` (the rate_control
     schedule's baseline/spike target) every `control_interval_seconds`:
       - on a PHASE CHANGE, it jumps the pool straight to the calibrated
         mix (`plan_worker_counts`): `floor(target/C)` uncapped workers
         plus, if a fractional remainder remains and `allow_throttle`, one
         throttled "trimmer" worker carrying it;
       - within a phase, it FINE-TRIMS reactively: add/kill one uncapped
         worker if the achieved rate strays outside a deadband around
         target, damped by a cooldown after any spawn/kill. Scale-down
         always kills the newest uncapped worker (LIFO); the trimmer is
         never touched by fine-trim, only by phase-change replanning
         (respawned with a new throttle when the remainder changes).
  5. Optionally RECALIBRATES `C` in stable windows (rolling EMA, clamped),
     since the per-worker ceiling erodes over a multi-day run.
  6. Stays LEAK-FREE across the ~hundreds of spawn/kill cycles a long run
     produces: dead children are reaped every tick (no zombies), and a
     bounded, reused set of per-slot config files stands in for a
     fresh-tempfile-per-spawn.
  7. Emits a timestamped rate CSV (epoch, t_seconds, target, achieved_evps,
     n_uncapped, n_trimmer, C) so it joins the monitor/Prometheus series by
     absolute time.

The pure decision logic below -- worker-count/trimmer math, the reactive
add/hold/kill decision, the worker_uid allocator, the roster's LIFO
scale-down bookkeeping, the phase-change diff, EMA recalibration with its
clamp, and `Schedule.target_at` -- has no I/O (no DB, no subprocess, no
real clock) and is unit-tested directly in test_parallel_generator.py.
Everything below "I/O helpers" is the impure orchestration that wires
those pieces to real processes/connections and is exercised only in
integration, per the testing rule in IMPLEMENTATION_CONTRACTS.md.
"""

import argparse
import copy
import csv
import math
import os
import shutil
import signal
import subprocess
import sys
import tempfile
import threading
import time

try:
    import yaml  # type: ignore
except Exception:
    yaml = None

try:
    import psycopg2
except ImportError:  # pragma: no cover - required only for a live DB run
    psycopg2 = None

try:
    import utils
except ImportError:  # pragma: no cover - utils.py requires psycopg2 today; see shared_cache.py
    utils = None

import shared_cache
from rate_governor import RateGovernor


# ---------------------------------------------------------------------------
# Config defaults (the `parallel` block -- see IMPLEMENTATION_CONTRACTS.md)
# ---------------------------------------------------------------------------

DEFAULT_PARALLEL_CONFIG = {
    "calibration_seconds": 30,
    "max_workers": 8,
    "control_interval_seconds": 5,
    "deadband_pct": 10,
    "cooldown_seconds": 10,
    "allow_throttle": True,
    "pk_pool_maxsize": 20000,
    "snapshot_refresh_seconds": 10800,
    "recalibrate": True,
    "pk_stride": 100000,
    "run_seconds": 604800,
    # Round-robin workers across all tservers (discovered via yb_servers()).
    # Set false only if the nodes' direct host/port aren't reachable (e.g. a
    # VIP-only deployment) -- then all workers use the single configured host.
    "distribute_across_nodes": True,
    # Backpressure: the reactive "add on low achieved" step may grow the
    # uncapped pool to at most ceil(target/calibration_C) * reactive_margin.
    # Beyond that, a below-target reading is cluster-bound, not worker-bound,
    # so adding more workers only piles load on a struggling cluster.
    "reactive_margin": 1.5,
}


def resolve_parallel_config(parallel_cfg):
    """Merge a (possibly partial) `parallel` config block over
    DEFAULT_PARALLEL_CONFIG. Never mutates the input; unknown keys in
    `parallel_cfg` pass through untouched (forward-compatible)."""
    cfg = dict(DEFAULT_PARALLEL_CONFIG)
    cfg.update(parallel_cfg or {})
    return cfg


# ---------------------------------------------------------------------------
# Pure functions -- no I/O, no subprocess, no DB, no real clock. Unit-tested
# directly in test_parallel_generator.py.
# ---------------------------------------------------------------------------

def peak_target(rate_control):
    """Return the maximum events_per_second across the baseline
    (default_events_per_second) and every schedule entry in a rate_control
    block -- the highest rate the pool must ever be able to serve.
    """
    rates = [rate_control.get("default_events_per_second", 0)]
    for entry in (rate_control.get("schedule") or []):
        rates.append(entry.get("events_per_second", 0))
    return max(rates)


class Schedule(object):
    """Thin, pure wrapper around rate_governor.RateGovernor's piecewise
    schedule + jitter math, exposing `target_at(now)` in absolute-clock
    terms (`now` is the same clock the caller's `run_start` was taken
    from). Decoupled from RateGovernor's pacing/sleep behavior -- this
    never sleeps and never mutates pacing state, so it's safe to poll from
    the control loop every tick.

    `target_at` is a pure function of (rate_control, random_seed, run_start,
    now): reused verbatim from RateGovernor.target_rate_at, which is itself
    already a pure function of elapsed time -- this wrapper only adds the
    absolute-vs-elapsed bookkeeping the controller wants.
    """

    def __init__(self, rate_control, random_seed, run_start=0.0):
        # clock/sleep are irrelevant here (target_rate_at never calls
        # either), but RateGovernor.__init__ requires clock to compute
        # run_start/window_start book-keeping we simply never read.
        self._governor = RateGovernor(
            rate_control, random_seed=random_seed,
            clock=lambda: 0.0, sleep=lambda seconds: None,
        )
        self.run_start = run_start

    def target_at(self, now):
        """Effective target rate (events/sec) at absolute time `now`."""
        return self._governor.target_rate_at(now - self.run_start)


def plan_worker_counts(target, C, allow_throttle=True):
    """Return {"n_full": int, "trimmer_rate": float} for the worker mix
    needed to hit `target` events/sec given a per-worker ceiling `C`.

    allow_throttle=True (default): n_full = floor(target / C) uncapped
    workers; trimmer_rate = target - n_full*C is the fractional remainder
    a single throttled "trimmer" worker should be paced to (0.0 when
    target is an exact multiple of C -- no trimmer needed).

    allow_throttle=False: round to the nearest whole worker
    (n_full = round(target / C)); trimmer_rate is always 0.0. Raises
    ValueError if that rounds to 0 workers for a target > 0 (unreachable
    with a single whole worker and throttling disabled -- see
    IMPLEMENTATION_CONTRACTS.md's error-handling guardrails).
    """
    if C <= 0:
        raise ValueError("C (per-worker ceiling) must be > 0, got {!r}".format(C))
    if target < 0:
        raise ValueError("target must be >= 0, got {!r}".format(target))

    if not allow_throttle:
        if target <= 0:
            return {"n_full": 0, "trimmer_rate": 0.0}
        n_full = int(round(target / C))
        if n_full <= 0:
            raise ValueError(
                "target {:.1f} ev/s is unreachable with a single whole worker "
                "(per-worker ceiling {:.1f} ev/s) and allow_throttle=False".format(target, C)
            )
        return {"n_full": n_full, "trimmer_rate": 0.0}

    n_full = int(math.floor(target / C))
    trimmer_rate = target - n_full * C
    if trimmer_rate < 1e-9:
        trimmer_rate = 0.0
    return {"n_full": n_full, "trimmer_rate": trimmer_rate}


def diff_worker_counts(current_n_full, current_trimmer_rate, plan):
    """Given the currently-running uncapped worker count and trimmer rate
    (0.0 if no trimmer is running) and a target `plan` (from
    plan_worker_counts), return the actions needed to converge:

      - "uncapped_delta": positive => spawn this many uncapped workers;
        negative => kill this many (LIFO, newest first -- the caller's
        WorkerRoster.pop_newest_uncapped enforces the "newest first"
        part).
      - "trimmer_action": one of
          "none"    -- leave the trimmer as-is (including "no trimmer
                       either way"),
          "spawn"   -- none running, one is now needed,
          "kill"    -- one is running, no longer needed,
          "respawn" -- one is running but at the wrong rate. Throttle is
                       fixed at spawn time, so a rate change can only be
                       applied by killing and replacing it.

    This is what makes "keep the baseline trimmer alive across phases"
    concrete: a phase change that happens to want the same trimmer rate as
    before leaves it running untouched; the general case (rate differs)
    respawns it. Either way the trimmer is never selected by the reactive
    fine-trim's scale-down (that only ever pops uncapped workers).
    """
    target_n_full = plan["n_full"]
    target_trimmer_rate = plan["trimmer_rate"]
    uncapped_delta = target_n_full - current_n_full

    have_trimmer = current_trimmer_rate > 0
    want_trimmer = target_trimmer_rate > 0

    if not have_trimmer and not want_trimmer:
        trimmer_action = "none"
    elif not have_trimmer and want_trimmer:
        trimmer_action = "spawn"
    elif have_trimmer and not want_trimmer:
        trimmer_action = "kill"
    elif abs(current_trimmer_rate - target_trimmer_rate) < 1e-6:
        trimmer_action = "none"
    else:
        trimmer_action = "respawn"

    return {"uncapped_delta": uncapped_delta, "trimmer_action": trimmer_action}


def decide_adjustment(achieved, target, deadband_pct, seconds_since_last_adjust, cooldown_seconds):
    """Return "add" | "kill" | "hold" for the within-phase reactive
    fine-trim step.

    Never adjusts within `cooldown_seconds` of the last spawn/kill
    (`seconds_since_last_adjust` is None when there has been none yet, in
    which case cooldown never suppresses). `deadband` is `deadband_pct`%
    of `target` on both sides: achieved below target-deadband -> "add";
    above target+deadband -> "kill"; otherwise -> "hold". Exactly-at-the-
    edge (achieved == target +/- deadband) holds (strict comparisons),
    matching the design's "correct for C being wrong or eroding" intent
    without hair-triggering on rounding noise.
    """
    if seconds_since_last_adjust is not None and seconds_since_last_adjust < cooldown_seconds:
        return "hold"

    deadband = target * (deadband_pct / 100.0)
    if achieved < target - deadband:
        return "add"
    if achieved > target + deadband:
        return "kill"
    return "hold"


def reactive_worker_cap(target, C, margin=1.5):
    """Max uncapped workers the within-phase reactive loop may grow to.

    The reactive "add on low achieved" step must not spiral: if achieved is
    below target even with the workers the target theoretically needs, the
    bottleneck is the cluster (or the measurement), not the worker count --
    adding more only piles load on a struggling cluster. (Observed: a 10k
    target with C~3300 ramped to 15 workers = ~5x the intended load, tipping
    tservers into heartbeat timeout.) Cap growth at ceil(target/C) * margin.

    `C` MUST be the stable calibration ceiling, never the live-recalibrated
    C: a struggling cluster erodes the live C, which would inflate
    ceil(target/C) and defeat the cap. Returns at least 1.
    """
    if C <= 0:
        return 1
    need = math.ceil(target / C)
    return max(1, int(math.ceil(need * margin)))


def compute_C_observed(achieved, trimmer_rate, n_uncapped):
    """Return the observed per-worker ceiling implied by the current
    achieved rate: (achieved - trimmer_rate) / n_uncapped, subtracting the
    trimmer's own throttled contribution so the estimate reflects only the
    uncapped workers. Returns None when there are no uncapped workers to
    measure (nothing to observe -- skip recalibration at pure baseline).
    """
    if n_uncapped <= 0:
        return None
    return (achieved - trimmer_rate) / n_uncapped


def recalibrate_C(C, C_observed, alpha=0.3, max_delta_pct=10.0):
    """EMA-update C toward C_observed (`C_new = alpha*C_observed +
    (1-alpha)*C`), then clamp the resulting change to at most
    `max_delta_pct`% of C (up or down) so a transient reading can't thrash
    the worker-count math across a multi-day run.
    """
    if C <= 0:
        raise ValueError("C must be > 0, got {!r}".format(C))
    if C_observed < 0:
        raise ValueError("C_observed must be >= 0, got {!r}".format(C_observed))

    ema = alpha * C_observed + (1 - alpha) * C
    delta = ema - C
    max_delta = C * (max_delta_pct / 100.0)
    if delta > max_delta:
        delta = max_delta
    elif delta < -max_delta:
        delta = -max_delta
    return C + delta


class WorkerUidAllocator(object):
    """Monotonic, controller-owned worker_uid allocator: every call to
    `allocate()` returns a fresh id, one higher than the last, for the
    life of the run -- including across respawns after a kill or crash.
    Ids are never reused; there is deliberately no "give an id back"
    operation, since that's precisely the respawn seed-reuse collision
    this allocator exists to prevent (see IMPLEMENTATION_CONTRACTS.md,
    "Monotonic PK generation").
    """

    def __init__(self, start=0):
        if start < 0:
            raise ValueError("start must be >= 0, got {!r}".format(start))
        self._next = start

    def allocate(self):
        uid = self._next
        self._next += 1
        return uid

    @property
    def next_uid(self):
        return self._next


class WorkerRoster(object):
    """Pure bookkeeping of currently-running worker uids: an ordered list
    of uncapped workers (append = spawn order, so the list's tail is
    always the newest) plus at most one trimmer uid tracked separately.

    `pop_newest_uncapped` is the "kill newest uncapped workers first
    (LIFO)" scale-down policy from the design; it can never return the
    trimmer, which is exactly what keeps the trimmer alive across
    fine-trim scale-downs -- only phase-change replanning (via
    diff_worker_counts) ever kills/respawns it.
    """

    def __init__(self):
        self._uncapped = []
        self.trimmer_uid = None

    def add_uncapped(self, uid):
        self._uncapped.append(uid)

    def pop_newest_uncapped(self):
        if not self._uncapped:
            return None
        return self._uncapped.pop()

    def remove_uncapped(self, uid):
        """Remove a specific uid wherever it is in the list (e.g. cleanup
        after an unexpected worker crash, not a scale-down decision)."""
        try:
            self._uncapped.remove(uid)
        except ValueError:
            pass

    def set_trimmer(self, uid):
        self.trimmer_uid = uid

    def clear_trimmer(self):
        self.trimmer_uid = None

    def n_uncapped(self):
        return len(self._uncapped)

    def n_trimmer(self):
        return 1 if self.trimmer_uid is not None else 0

    def all_uids(self):
        uids = list(self._uncapped)
        if self.trimmer_uid is not None:
            uids.append(self.trimmer_uid)
        return uids


def derive_worker_runtime_config(base_config):
    """Build the config file every spawned worker runs with: a deep copy
    of `base_config` with the `parallel` block removed (irrelevant to a
    single worker) and `generator.rate_control` removed (superseded by the
    controller's per-worker `--throttle` CLI flag -- the aggregate rate is
    now set by worker-count modulation, not per-worker rate_control
    pacing). Never mutates `base_config`. Every worker gets the identical
    result; only `--worker-uid`/`--throttle`/cache flags differ per spawn,
    which is what lets a bounded, reused set of slot files stand in for a
    fresh temp file per spawn (see SlotFilePool).
    """
    cfg = copy.deepcopy(base_config)
    cfg.pop("parallel", None)
    gen = cfg.get("generator")
    if gen is not None:
        gen.pop("rate_control", None)
    return cfg


def build_worker_argv(python_exe, generator_path, config_path, worker_uid, pk_stride,
                       cache_dir, cache_version, throttle=0.0):
    """Build the argv for one worker spawn, per IMPLEMENTATION_CONTRACTS.md's
    "Worker CLI" contract: `-c`, `--worker-uid`, `--pk-stride`,
    `--cache-dir`, `--cache-version`, and `--throttle` only when > 0
    (0/absent means uncapped -- the worker must not engage rate_governor
    at all in that case).
    """
    argv = [
        python_exe, generator_path,
        "-c", config_path,
        "--worker-uid", str(worker_uid),
        "--pk-stride", str(pk_stride),
        "--cache-dir", cache_dir,
        "--cache-version", cache_version,
    ]
    if throttle and throttle > 0:
        argv += ["--throttle", str(throttle)]
    return argv


class SlotFilePool(object):
    """A bounded, reused set of `size` config-file paths under `tmp_dir`.
    Spawning hands out a free slot (`acquire`); the worker's exit returns
    it (`release`). This is what keeps "write a new YAML per spawn" from
    growing without bound across the ~hundreds of spawn/kill cycles a
    multi-day run produces -- at most `size` files ever exist, and they're
    simply overwritten in place on reuse.

    Pure bookkeeping only -- no file I/O happens here; the caller writes
    to (and the OS creates) the paths this hands out.
    """

    def __init__(self, tmp_dir, size):
        if size <= 0:
            raise ValueError("size must be > 0, got {!r}".format(size))
        self._paths = [os.path.join(tmp_dir, "slot_{}.yaml".format(i)) for i in range(size)]
        self._free = list(range(size))
        self.size = size

    def acquire(self):
        if not self._free:
            raise RuntimeError(
                "no free config-file slots available (all {} in use); "
                "increase max_workers headroom".format(self.size)
            )
        idx = self._free.pop()
        return idx, self._paths[idx]

    def release(self, idx):
        if idx not in self._free:
            self._free.append(idx)

    def in_use_count(self):
        return self.size - len(self._free)


# ---------------------------------------------------------------------------
# I/O helpers
# ---------------------------------------------------------------------------

GENERATOR_PATH = os.path.join(os.path.dirname(os.path.abspath(__file__)), "generator.py")


def write_yaml_config(data, path):
    if yaml is None:
        raise RuntimeError("PyYAML is required to write worker configs. Install with: pip install PyYAML")
    with open(path, "w") as f:
        yaml.safe_dump(data, f, default_flow_style=False, sort_keys=False)


def query_events_sum(cursor):
    """DB-wide count of rows touched by INSERT/UPDATE/DELETE statements,
    per pg_stat_statements, since the last pg_stat_statements_reset(). This
    already sums across every backend/connection, so it aggregates all
    worker processes without any inter-process bookkeeping. Per
    IMPLEMENTATION_CONTRACTS.md's error handling: a negative delta (e.g.
    pg_stat_statements was reset by something else mid-run) is the
    caller's job to clamp to 0, not this function's -- it only ever
    returns the current cumulative sum.
    """
    cursor.execute(
        "SELECT COALESCE(SUM(rows), 0) FROM pg_stat_statements "
        "WHERE query ~* '^(insert|update|delete)'"
    )
    return float(cursor.fetchone()[0])


def discover_nodes(cursor):
    """Return [(host, port), ...] for every live tserver via yb_servers().
    Empty list if the query fails or returns nothing (caller falls back to
    single-host mode). Used both to round-robin workers across nodes and to
    build the cross-node rate meter.
    """
    try:
        cursor.execute("SELECT host, port FROM yb_servers()")
        return [(h, int(p)) for h, p in cursor.fetchall()]
    except psycopg2.Error as e:
        print("[dist] WARNING: yb_servers() failed ({}); using single host".format(e))
        return []


class CrossNodeEventMeter:
    """Sum pg_stat_statements DML rows across ALL cluster nodes.

    pg_stat_statements is per-node: each tserver's postgres counts only the
    DML it coordinated. Once workers are spread across nodes (the controller
    round-robins them), a single monitor connection sees only its own node's
    slice -- which makes calibration and the rate loop read near-zero even
    while the cluster is writing hundreds of thousands of rows. This meter
    opens one connection per node (from `nodes`) and sums each node's
    pg_stat_statements every poll.

    Robust to this cluster's intermittent per-node heartbeat blips: a node
    that fails to answer a poll contributes its last known value (not 0), so
    the cluster-wide delta the controller computes never spikes or dips from
    a transient node hiccup.
    """

    def __init__(self, conn_kwargs, nodes):
        self.conns = []
        self.hosts = []
        self.last = {}
        for host, port in nodes:
            try:
                kw = dict(conn_kwargs)
                kw["host"] = host
                kw["port"] = int(port)
                c = psycopg2.connect(**kw)
                c.autocommit = True
                self.conns.append(c)
                self.hosts.append(host)
                self.last[host] = 0.0
            except psycopg2.Error as e:
                print("[meter] WARNING: no measurement connection to {} ({}); "
                      "excluding it from rate totals".format(host, e))

    def events_sum(self):
        total = 0.0
        for host, c in zip(self.hosts, self.conns):
            try:
                cur = c.cursor()
                v = query_events_sum(cur)
                cur.close()
                self.last[host] = v
            except psycopg2.Error:
                v = self.last.get(host, 0.0)  # transient node hiccup: reuse last
            total += v
        return total

    def reset(self):
        for c in self.conns:
            try:
                cur = c.cursor()
                cur.execute("SELECT pg_stat_statements_reset()")
                cur.close()
            except psycopg2.Error:
                pass
        for h in self.last:
            self.last[h] = 0.0

    def node_count(self):
        return len(self.conns)

    def close(self):
        for c in self.conns:
            try:
                c.close()
            except Exception:
                pass


def reset_pg_stat_statements(cursor):
    """Reset pg_stat_statements counters. Raises RuntimeError with a clear,
    actionable message if the extension is not installed/enabled.
    """
    try:
        cursor.execute("SELECT pg_stat_statements_reset()")
    except psycopg2.Error as e:
        raise RuntimeError(
            "pg_stat_statements is required for calibration and monitoring "
            "but is not available ({}). Enable it via "
            "shared_preload_libraries = 'pg_stat_statements' in postgresql.conf "
            "(requires a restart) and then run "
            "CREATE EXTENSION IF NOT EXISTS pg_stat_statements;".format(e)
        )


def terminate_processes(procs, grace_seconds=5):
    """Terminate a list of subprocess.Popen (SIGTERM), then SIGKILL any
    that are still alive after `grace_seconds`, waiting on (reaping) every
    one of them. Safe to call with already-dead processes or an empty/
    None-containing list.
    """
    procs = [p for p in procs if p is not None]

    for p in procs:
        if p.poll() is None:
            try:
                p.terminate()
            except OSError:
                pass

    deadline = time.time() + grace_seconds
    for p in procs:
        remaining = deadline - time.time()
        if remaining <= 0:
            break
        try:
            p.wait(timeout=remaining)
        except subprocess.TimeoutExpired:
            pass

    for p in procs:
        if p.poll() is None:
            try:
                p.kill()
            except OSError:
                pass
        try:
            p.wait(timeout=5)
        except Exception:
            pass


def reap_dead(worker_procs):
    """Poll every tracked worker; return [(uid, returncode), ...] for any
    whose process has already exited. `Popen.poll()` itself performs the
    non-blocking `waitpid(WNOHANG)` reap, so calling this every control-
    loop tick is what prevents zombie accumulation across the ~hundreds of
    spawn/kill cycles a multi-day run produces -- no separate SIGCHLD
    handler is needed.
    """
    dead = []
    for uid, entry in list(worker_procs.items()):
        proc = entry["proc"]
        if proc.poll() is not None:
            dead.append((uid, proc.returncode))
    return dead


def resolve_table_list(cursor, gen):
    """Return the concrete table list to build the shared cache for:
    `generator.manual_table_list` if non-empty, else a schema scan via
    utils.get_table_list (the same fallback generator.py itself uses).
    """
    manual = gen.get("manual_table_list")
    if manual:
        return list(manual)
    if utils is None:
        raise RuntimeError(
            "the utils module is required to discover the table list (no "
            "manual_table_list given) but could not be imported"
        )
    return utils.get_table_list(cursor, gen.get("schema_name"), gen.get("exclude_table_list"))


def start_snapshot_refresh_thread(connection_kwargs, schema_name, table_list, pk_pool_maxsize,
                                   cache_dir, snapshot_refresh_seconds, stop_event):
    """Start a background thread that rebuilds the shared cache into a new
    version every `snapshot_refresh_seconds`, atomically flipping CURRENT
    (see shared_cache.build_cache) -- multi-day PK-snapshot freshness
    (design doc section 11.2). Uses its own DB connection, entirely
    separate from any worker's or the controller's own main cursor, and
    never touches a running worker's delta/tombstones. A refresh failure
    is logged and retried on the next tick; it never raises into the
    control loop and never blocks spawning (spawns always just read
    whatever CURRENT currently points at).
    """
    def loop():
        while not stop_event.wait(snapshot_refresh_seconds):
            conn = None
            try:
                conn = psycopg2.connect(**connection_kwargs)
                conn.autocommit = True
                cur = conn.cursor()
                version = shared_cache.build_cache(
                    cur, schema_name, table_list, pk_pool_maxsize, cache_dir
                )
                print("[cache] refreshed snapshot -> version {}".format(version))
            except Exception as e:
                print("[cache] WARNING: snapshot refresh failed: {}".format(e))
            finally:
                if conn is not None:
                    try:
                        conn.close()
                    except Exception:
                        pass

    thread = threading.Thread(target=loop, name="snapshot-refresh", daemon=True)
    thread.start()
    return thread


# ---------------------------------------------------------------------------
# Orchestration (impure: subprocess, DB connections, real clock/sleep).
# Exercised only in integration -- see IMPLEMENTATION_CONTRACTS.md's
# testing rule; not run by this slice's unit tests.
# ---------------------------------------------------------------------------

def run_calibration(spawn_worker, kill_worker, measure_fn, calibration_seconds, reset_fn=None):
    """Run one uncapped worker (spawned via the same `spawn_worker` the
    main loop uses, so it goes through the identical shared-cache-backed
    fast-start path) for `calibration_seconds`, measure the aggregate rate
    it produces via `measure_fn` (cluster-wide pg_stat_statements DML rows;
    cross-node when load balancing is on), kill it, and return the measured
    per-worker ceiling C.
    """
    if reset_fn is not None:
        reset_fn()
    print("[calibration] starting 1 uncapped worker for {}s...".format(calibration_seconds))
    uid, proc = spawn_worker(throttle=0.0)
    try:
        start_sum = measure_fn()
        start_t = time.monotonic()
        time.sleep(calibration_seconds)
        end_sum = measure_fn()
        elapsed = time.monotonic() - start_t
        if proc.poll() is not None:
            print(
                "[calibration] WARNING: worker exited early (code={}) during "
                "calibration".format(proc.returncode)
            )
    finally:
        kill_worker(uid)

    if elapsed <= 0:
        raise RuntimeError("calibration elapsed time was non-positive; cannot compute a ceiling")

    C = (end_sum - start_sum) / elapsed
    print(
        "[calibration] measured C (per-worker ceiling) = {:.1f} ev/s "
        "(delta={} rows over {:.1f}s)".format(C, end_sum - start_sum, elapsed)
    )
    return C


def run_controller(base_config, rate_csv_path=None):
    if psycopg2 is None:
        print("ERROR: psycopg2 is required to run the controller against a live DB.")
        sys.exit(1)

    # Convert SIGTERM into KeyboardInterrupt so a plain `kill`/`pkill` (or a job
    # manager terminating the controller) still runs the finally block that reaps
    # child workers. Without this, SIGTERM ends the process immediately and the
    # workers are orphaned -- they keep writing to the target DB indefinitely.
    def _sigterm_to_interrupt(signum, frame):
        raise KeyboardInterrupt()
    try:
        signal.signal(signal.SIGTERM, _sigterm_to_interrupt)
    except (ValueError, OSError):
        # Not on the main thread (e.g. under some test harnesses); skip.
        pass
    if utils is None:
        print("ERROR: the utils module could not be imported (required for config/DB helpers).")
        sys.exit(1)

    gen = base_config["generator"]
    rate_control = gen.get("rate_control")
    if not rate_control:
        print(
            "ERROR: base config must have a generator.rate_control block. "
            "Its rates are the schedule the controller's worker-count modulation targets."
        )
        sys.exit(1)

    parallel_cfg = resolve_parallel_config(base_config.get("parallel"))
    max_workers = parallel_cfg["max_workers"]
    calibration_seconds = parallel_cfg["calibration_seconds"]
    control_interval = parallel_cfg["control_interval_seconds"]
    deadband_pct = parallel_cfg["deadband_pct"]
    cooldown_seconds = parallel_cfg["cooldown_seconds"]
    allow_throttle = parallel_cfg["allow_throttle"]
    pk_pool_maxsize = parallel_cfg["pk_pool_maxsize"]
    snapshot_refresh_seconds = parallel_cfg["snapshot_refresh_seconds"]
    recalibrate = parallel_cfg["recalibrate"]
    pk_stride = parallel_cfg["pk_stride"]
    run_seconds = parallel_cfg["run_seconds"]
    reactive_margin = parallel_cfg.get("reactive_margin", 1.5)

    base_seed = gen.get("random_seed", gen.get("seed"))
    if base_seed is None:
        base_seed = 0

    schema_name = gen["schema_name"]
    conn_kwargs = utils.get_connection_kwargs_from_config(base_config)
    conn = psycopg2.connect(**conn_kwargs)
    conn.autocommit = True
    cursor = conn.cursor()

    # Distribute workers across the cluster's tservers by assigning each worker
    # a specific node (round-robin), overriding host/port in that worker's
    # config. A driver-level connection load balancer cannot do this for us:
    # each worker is a separate process opening a single connection, so every
    # process starts with an empty balancer and they all pick the same node.
    # nodes = [] means single-host mode (all workers use the configured host).
    distribute = parallel_cfg.get("distribute_across_nodes", True)
    nodes = discover_nodes(cursor) if distribute else []
    if len(nodes) > 1:
        print("[dist] distributing workers across {} tservers: {}".format(
            len(nodes), ", ".join(h for h, _ in nodes)))

    # Rate measurement. pg_stat_statements is per-node; once writes are spread
    # across nodes a single connection under-counts wildly, so sum across all
    # nodes via a CrossNodeEventMeter. With a single node the plain cursor is
    # equivalent and cheaper.
    meter = None
    if len(nodes) > 1:
        meter = CrossNodeEventMeter(conn_kwargs, nodes)
        print("[meter] cross-node rate measurement over {} node(s)".format(meter.node_count()))
        measure_fn = meter.events_sum
        reset_fn = meter.reset
    else:
        measure_fn = lambda: query_events_sum(cursor)
        reset_fn = lambda: reset_pg_stat_statements(cursor)

    table_list = resolve_table_list(cursor, gen)

    cache_dir = tempfile.mkdtemp(prefix="event_generator_cache_")
    tmp_dir = tempfile.mkdtemp(prefix="parallel_generator_")
    slot_pool = SlotFilePool(tmp_dir, max_workers + 1)  # +1 headroom for the trimmer
    uid_allocator = WorkerUidAllocator()
    worker_procs = {}  # worker_uid -> {"proc": Popen, "slot": int, "throttle": float}
    roster = WorkerRoster()
    runtime_cfg = derive_worker_runtime_config(base_config)
    stop_refresh = threading.Event()
    refresh_thread = None
    csv_file = None
    interrupted = False

    def spawn_worker(throttle=0.0):
        uid = uid_allocator.allocate()
        slot_idx, slot_path = slot_pool.acquire()
        # Assign this worker a specific node (round-robin) so load spreads
        # evenly across the cluster; each worker connects directly to its node.
        cfg = runtime_cfg
        if nodes:
            node_host, node_port = nodes[uid % len(nodes)]
            cfg = copy.deepcopy(runtime_cfg)
            cfg.setdefault("connection", {})
            cfg["connection"]["host"] = node_host
            cfg["connection"]["port"] = node_port
        write_yaml_config(cfg, slot_path)
        cur_version = shared_cache.current_version(cache_dir)
        argv = build_worker_argv(
            sys.executable, GENERATOR_PATH, slot_path, uid, pk_stride,
            cache_dir, cur_version, throttle,
        )
        proc = subprocess.Popen(argv)
        worker_procs[uid] = {"proc": proc, "slot": slot_idx, "throttle": throttle}
        return uid, proc

    def kill_worker(uid):
        entry = worker_procs.pop(uid, None)
        if entry is None:
            return
        terminate_processes([entry["proc"]])
        slot_pool.release(entry["slot"])

    def apply_phase_plan(target, plan):
        n_full = plan["n_full"]
        trimmer_rate = plan["trimmer_rate"]

        total_wanted = n_full + (1 if trimmer_rate > 0 else 0)
        if total_wanted > max_workers:
            allowed_full = max(0, max_workers - (1 if trimmer_rate > 0 else 0))
            achievable = allowed_full * C_state["C"] + (trimmer_rate if trimmer_rate > 0 else 0.0)
            print(
                "[plan] WARNING: target {:.1f} ev/s needs {} workers > max_workers={}; "
                "capping to {} full + trimmer, achievable ~{:.1f} ev/s".format(
                    target, total_wanted, max_workers, allowed_full, achievable
                )
            )
            n_full = allowed_full

        current_trimmer_rate = 0.0
        if roster.trimmer_uid is not None:
            current_trimmer_rate = worker_procs.get(roster.trimmer_uid, {}).get("throttle", 0.0)

        actions = diff_worker_counts(
            roster.n_uncapped(), current_trimmer_rate, {"n_full": n_full, "trimmer_rate": trimmer_rate}
        )

        delta = actions["uncapped_delta"]
        if delta > 0:
            for _ in range(delta):
                if len(worker_procs) >= max_workers:
                    print("[plan] WARNING: at max_workers={}, cannot add more uncapped workers".format(max_workers))
                    break
                uid, _ = spawn_worker(throttle=0.0)
                roster.add_uncapped(uid)
        elif delta < 0:
            for _ in range(-delta):
                victim = roster.pop_newest_uncapped()
                if victim is None:
                    break
                kill_worker(victim)

        if actions["trimmer_action"] == "spawn":
            uid, _ = spawn_worker(throttle=trimmer_rate)
            roster.set_trimmer(uid)
        elif actions["trimmer_action"] == "kill":
            if roster.trimmer_uid is not None:
                kill_worker(roster.trimmer_uid)
                roster.clear_trimmer()
        elif actions["trimmer_action"] == "respawn":
            if roster.trimmer_uid is not None:
                kill_worker(roster.trimmer_uid)
            uid, _ = spawn_worker(throttle=trimmer_rate)
            roster.set_trimmer(uid)

    try:
        print("[cache] building initial shared cache for {} table(s)...".format(len(table_list)))
        shared_cache.build_cache(cursor, schema_name, table_list, pk_pool_maxsize, cache_dir)
        print("[cache] built version {}".format(shared_cache.current_version(cache_dir)))

        if snapshot_refresh_seconds and snapshot_refresh_seconds > 0:
            refresh_thread = start_snapshot_refresh_thread(
                conn_kwargs, schema_name, table_list, pk_pool_maxsize, cache_dir,
                snapshot_refresh_seconds, stop_refresh,
            )

        C = run_calibration(spawn_worker, kill_worker, measure_fn, calibration_seconds, reset_fn=reset_fn)
        C_state = {"C": C}
        # Stable calibration ceiling for the reactive backpressure cap. Never
        # replaced by the live-recalibrated C (which erodes under cluster
        # distress and would inflate the cap -- exactly the spiral we prevent).
        calib_C = C

        peak = peak_target(rate_control)
        if peak > max_workers * C:
            achievable = max_workers * C
            print(
                "[plan] WARNING: peak target {:.1f} ev/s exceeds max_workers*C={:.1f}; "
                "will cap at {} workers (~{:.1f} ev/s achievable)".format(
                    peak, achievable, max_workers, achievable
                )
            )

        schedule = Schedule(rate_control, random_seed=base_seed, run_start=0.0)
        run_start = time.monotonic()
        schedule.run_start = run_start

        rate_csv_path = rate_csv_path or "parallel_generator_rates_{}.csv".format(int(time.time()))
        csv_file = open(rate_csv_path, "w", newline="")
        writer = csv.writer(csv_file)
        writer.writerow(["epoch", "t_seconds", "target", "achieved_evps", "n_uncapped", "n_trimmer", "C"])
        print("[rates] writing timestamped rate CSV to {}".format(rate_csv_path))

        reset_fn()
        last_sample_t = time.monotonic()
        last_sum = measure_fn()
        current_target = None
        last_adjust_t = None
        end_time = run_start + run_seconds

        while True:
            now = time.monotonic()
            if now >= end_time:
                break
            sleep_for = min(control_interval, end_time - now)
            if sleep_for > 0:
                time.sleep(sleep_for)

            now = time.monotonic()

            # Leak-free churn: reap anything that exited (crash, OOM, etc.)
            # and respawn it (fresh worker_uid, current cache) preserving
            # its role, so a lost worker doesn't silently shrink the pool.
            # crashed_this_interval is a distress signal: while workers are
            # dying (e.g. a tserver heartbeat-timing-out), the reactive loop
            # must NOT add more workers -- that piles load on a struggling
            # cluster. Respawning to hold the pool is fine; growing it is not.
            crashed_this_interval = 0
            for uid, code in reap_dead(worker_procs):
                crashed_this_interval += 1
                entry = worker_procs.get(uid) or {}
                throttle = entry.get("throttle", 0.0)
                was_trimmer = (roster.trimmer_uid == uid)
                print(
                    "[monitor] WARNING: worker uid={} exited unexpectedly (code={}); "
                    "respawning".format(uid, code)
                )
                dead_entry = worker_procs.pop(uid, None)
                if dead_entry is not None:
                    slot_pool.release(dead_entry["slot"])
                if was_trimmer:
                    roster.clear_trimmer()
                else:
                    roster.remove_uncapped(uid)
                new_uid, _ = spawn_worker(throttle=throttle)
                if was_trimmer:
                    roster.set_trimmer(new_uid)
                else:
                    roster.add_uncapped(new_uid)

            cur_sum = measure_fn()
            interval = now - last_sample_t
            delta = cur_sum - last_sum
            if delta < 0:
                # pg_stat_statements was reset out from under us; treat as
                # a fresh baseline rather than reporting a negative rate.
                delta = 0.0
            achieved = delta / interval if interval > 0 else 0.0
            last_sample_t, last_sum = now, cur_sum

            target = schedule.target_at(now)

            if current_target is None or target != current_target:
                plan = plan_worker_counts(target, C_state["C"], allow_throttle)
                apply_phase_plan(target, plan)
                current_target = target
                last_adjust_t = now
            else:
                seconds_since_last_adjust = (now - last_adjust_t) if last_adjust_t is not None else None
                decision = decide_adjustment(
                    achieved, target, deadband_pct, seconds_since_last_adjust, cooldown_seconds
                )
                if decision == "add":
                    cap = reactive_worker_cap(target, calib_C, reactive_margin)
                    if crashed_this_interval > 0:
                        # Cluster distress: workers are dying. Adding more would
                        # amplify the overload. Hold (respawns already keep the
                        # pool level); let it recover.
                        print("[backpressure] {} worker(s) crashed this interval; "
                              "not adding (cluster distress)".format(crashed_this_interval))
                    elif roster.n_uncapped() >= cap:
                        # Below target despite having the workers the target
                        # needs -> cluster-bound, not worker-bound. Adding more
                        # only hurts. This is the anti-spiral cap.
                        print("[backpressure] holding at n_uncapped={} (reactive cap {} "
                              "for target={:.0f}, calibC={:.0f}); shortfall is cluster-bound"
                              .format(roster.n_uncapped(), cap, target, calib_C))
                    elif len(worker_procs) < max_workers:
                        uid, _ = spawn_worker(throttle=0.0)
                        roster.add_uncapped(uid)
                        last_adjust_t = now
                    else:
                        print("[control] want to add a worker but at max_workers={}".format(max_workers))
                elif decision == "kill":
                    victim = roster.pop_newest_uncapped()
                    if victim is not None:
                        kill_worker(victim)
                        last_adjust_t = now

                if recalibrate:
                    n_unc = roster.n_uncapped()
                    stable = (last_adjust_t is None) or ((now - last_adjust_t) >= cooldown_seconds)
                    if stable and n_unc >= 1:
                        trimmer_rate = 0.0
                        if roster.trimmer_uid is not None:
                            trimmer_rate = worker_procs.get(roster.trimmer_uid, {}).get("throttle", 0.0)
                        c_obs = compute_C_observed(achieved, trimmer_rate, n_unc)
                        if c_obs is not None and c_obs > 0:
                            C_state["C"] = recalibrate_C(C_state["C"], c_obs)

            wall = time.time()
            t_seconds = now - run_start
            writer.writerow([
                "{:.3f}".format(wall), "{:.1f}".format(t_seconds), "{:.1f}".format(target),
                "{:.1f}".format(achieved), roster.n_uncapped(), roster.n_trimmer(),
                "{:.1f}".format(C_state["C"]),
            ])
            csv_file.flush()
            print(
                "[monitor] ts={:.3f} t={:.0f}s target={:.1f} achieved={:.1f} "
                "n_uncapped={} n_trimmer={} C={:.1f}".format(
                    wall, t_seconds, target, achieved, roster.n_uncapped(), roster.n_trimmer(), C_state["C"]
                )
            )

    except KeyboardInterrupt:
        print("[monitor] received Ctrl+C, stopping all workers...")
        interrupted = True
    finally:
        stop_refresh.set()
        if refresh_thread is not None:
            refresh_thread.join(timeout=5)
        terminate_processes([entry["proc"] for entry in worker_procs.values()])
        if csv_file is not None:
            try:
                csv_file.close()
            except Exception:
                pass
        if meter is not None:
            meter.close()
        try:
            conn.close()
        except Exception:
            pass
        shutil.rmtree(tmp_dir, ignore_errors=True)
        shutil.rmtree(cache_dir, ignore_errors=True)

    print("[summary] run {}".format("interrupted" if interrupted else "completed"))


def main():
    parser = argparse.ArgumentParser(
        description="Reactive worker-pool controller for the event generator"
    )
    parser.add_argument(
        "-c", "--config", default=None,
        help="Path to base event-generator YAML config (must include a top-level 'parallel' block)",
    )
    parser.add_argument(
        "--rate-csv", default=None,
        help="Path to write the timestamped rate CSV (default: parallel_generator_rates_<epoch>.csv)",
    )
    args = parser.parse_args()

    base_config = utils.load_event_generator_config(args.config) if utils is not None \
        else _fail_no_utils()
    run_controller(base_config, rate_csv_path=args.rate_csv)


def _fail_no_utils():
    print("ERROR: the utils module could not be imported (required to load config).")
    sys.exit(1)


if __name__ == "__main__":
    main()
