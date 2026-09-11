"""
parallel_generator.py — parallel wrapper/orchestrator around generator.py.

The single-process event generator (generator.py) is left unmodified. This
wrapper:
  1. Loads one base config (a normal generator config plus a top-level
     `parallel` block).
  2. Runs a one-shot CALIBRATION: a single, uncapped generator.py worker,
     measured via pg_stat_statements to find the per-worker throughput
     ceiling.
  3. DERIVES how many worker processes are needed to hit the desired total
     rate (the `rate_control` block in the base config is the DESIRED TOTAL
     across all workers).
  4. SPAWNS N copies of generator.py, each with its own seed and a rate
     scaled down by 1/N, via per-worker YAML config files written to a temp
     directory.
  5. MONITORS the DB-wide aggregate rate (pg_stat_statements already sums
     across all worker connections) and prints it periodically.
  6. SHUTS DOWN all workers cleanly on a time budget or Ctrl+C, cleans up
     temp files, and prints a final summary.

The pure functions below (compute_workers, peak_target, derive_worker_config)
have no side effects and are unit-tested in test_parallel_generator.py.
"""

import argparse
import copy
import math
import os
import shutil
import subprocess
import sys
import tempfile
import time

import psycopg2

try:
    import yaml  # type: ignore
except Exception:
    yaml = None

from utils import load_event_generator_config, get_connection_kwargs_from_config


# ---------------------------------------------------------------------------
# Pure functions (no I/O, no subprocess, no DB) -- unit-tested directly.
# ---------------------------------------------------------------------------

def peak_target(rate_control):
    """Return the maximum events_per_second across the baseline
    (default_events_per_second) and every schedule entry in a rate_control
    block, i.e. the highest rate any worker set must be able to serve (so a
    spike is servable, not just the baseline).
    """
    rates = [rate_control.get("default_events_per_second", 0)]
    for entry in (rate_control.get("schedule") or []):
        rates.append(entry.get("events_per_second", 0))
    return max(rates)


def compute_workers(peak_target_value, per_worker_ceiling, max_workers, margin):
    """Derive how many worker processes are needed to serve `peak_target_value`
    events/sec, given a measured `per_worker_ceiling` (events/sec a single
    uncapped worker can sustain) and a safety `margin` (>1 inflates the
    target so workers aren't run flat-out at their ceiling).

    Returns (workers, reachable):
      - workers: max(1, ceil(peak_target_value * margin / per_worker_ceiling)),
        clamped to max_workers.
      - reachable: whether `workers` copies of a per_worker_ceiling worker can
        serve peak_target_value (workers * per_worker_ceiling >= peak_target_value).
        Always False if clamped by max_workers (unless that clamp happens to
        still meet the raw target).
    """
    if per_worker_ceiling <= 0:
        raise ValueError("per_worker_ceiling must be > 0, got {!r}".format(per_worker_ceiling))
    if max_workers <= 0:
        raise ValueError("max_workers must be > 0, got {!r}".format(max_workers))

    workers = max(1, math.ceil(peak_target_value * margin / per_worker_ceiling))
    reachable = (workers * per_worker_ceiling >= peak_target_value)

    if workers > max_workers:
        workers = max_workers
        reachable = (workers * per_worker_ceiling >= peak_target_value)

    return workers, reachable


def derive_worker_config(base_config, worker_index, workers, base_seed):
    """Build one worker's config from the base config.

    - Deep-copies base_config (never mutates the caller's dict).
    - Removes the top-level `parallel` block (irrelevant to a single worker).
    - Sets generator.random_seed and generator.faker_seed to
      base_seed + worker_index, so each worker is deterministic and workers
      don't reproduce identical event sequences.
    - Divides generator.rate_control.default_events_per_second and every
      schedule entry's events_per_second by `workers` (rounded to the
      nearest int, floored at 1 so no worker is configured with a zero
      rate). Batch sizes, operation weights, durations, offsets, and jitter
      are left untouched -- only the target rate is split.
    """
    cfg = copy.deepcopy(base_config)
    cfg.pop("parallel", None)

    gen = cfg["generator"]
    gen["random_seed"] = base_seed + worker_index
    gen["faker_seed"] = base_seed + worker_index

    rate_control = gen.get("rate_control")
    if rate_control:
        default_rate = rate_control.get("default_events_per_second", 0)
        rate_control["default_events_per_second"] = max(1, int(round(default_rate / workers)))
        for entry in (rate_control.get("schedule") or []):
            entry_rate = entry.get("events_per_second", 0)
            entry["events_per_second"] = max(1, int(round(entry_rate / workers)))

    return cfg


# ---------------------------------------------------------------------------
# I/O helpers
# ---------------------------------------------------------------------------

def write_yaml_config(data, path):
    if yaml is None:
        raise RuntimeError("PyYAML is required to write worker configs. Install with: pip install PyYAML")
    with open(path, "w") as f:
        yaml.safe_dump(data, f, default_flow_style=False, sort_keys=False)


def query_events_sum(cursor):
    """DB-wide count of rows touched by INSERT/UPDATE/DELETE statements,
    per pg_stat_statements, since the last pg_stat_statements_reset(). This
    already sums across every backend/connection, so it aggregates all
    worker processes without any inter-process bookkeeping.
    """
    cursor.execute(
        "SELECT COALESCE(SUM(rows), 0) FROM pg_stat_statements "
        "WHERE query ~* '^(insert|update|delete)'"
    )
    return float(cursor.fetchone()[0])


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
    that are still alive after `grace_seconds`. Safe to call with already-
    dead processes or an empty/None-containing list.
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


# ---------------------------------------------------------------------------
# Orchestration
# ---------------------------------------------------------------------------

GENERATOR_PATH = os.path.join(os.path.dirname(os.path.abspath(__file__)), "generator.py")

# The calibration worker commits rows while probing; its seed must not coincide
# with any run worker's seed (base_seed + i), or that worker would replay the
# same id stream and collide on every INSERT. This offset keeps it clear.
CALIBRATION_SEED_OFFSET = 1000000


def run_calibration(base_config, tmp_dir, calibration_seconds, cursor):
    """Run one uncapped generator.py worker for `calibration_seconds`,
    measure the aggregate events/sec it produces via pg_stat_statements, and
    return per_worker_ceiling (events/sec a single unpaced worker can
    sustain against this DB/schema/config).
    """
    calib_config = copy.deepcopy(base_config)
    calib_config.pop("parallel", None)
    calib_config["generator"].pop("rate_control", None)
    calib_config["generator"]["num_iterations"] = -1
    # Use a seed far outside the run-worker range (base_seed + i). The calibration
    # worker commits rows during its probe; if it shared a seed with a run worker,
    # that worker would replay the same id stream and collide on every INSERT.
    _base_seed = calib_config["generator"].get(
        "random_seed", calib_config["generator"].get("seed", 0)) or 0
    calib_config["generator"]["random_seed"] = _base_seed + CALIBRATION_SEED_OFFSET
    calib_config["generator"]["faker_seed"] = _base_seed + CALIBRATION_SEED_OFFSET

    calib_path = os.path.join(tmp_dir, "tmp_calib.yaml")
    write_yaml_config(calib_config, calib_path)

    reset_pg_stat_statements(cursor)

    print("[calibration] starting 1 uncapped worker for {}s...".format(calibration_seconds))
    calib_proc = subprocess.Popen([sys.executable, GENERATOR_PATH, "-c", calib_path])
    try:
        start_sum = query_events_sum(cursor)
        start_time = time.monotonic()
        time.sleep(calibration_seconds)
        end_sum = query_events_sum(cursor)
        elapsed = time.monotonic() - start_time

        if calib_proc.poll() is not None:
            print(
                "[calibration] WARNING: calibration worker exited early with code {} "
                "during calibration".format(calib_proc.returncode)
            )
    finally:
        terminate_processes([calib_proc])

    if elapsed <= 0:
        raise RuntimeError("calibration elapsed time was non-positive; cannot compute a ceiling")

    per_worker_ceiling = (end_sum - start_sum) / elapsed
    print(
        "[calibration] measured per_worker_ceiling = {:.1f} ev/s "
        "(delta={} rows over {:.1f}s)".format(per_worker_ceiling, end_sum - start_sum, elapsed)
    )
    return per_worker_ceiling


def spawn_workers(base_config, workers, base_seed, tmp_dir):
    worker_procs = []
    for i in range(workers):
        worker_cfg = derive_worker_config(base_config, i, workers, base_seed)
        worker_path = os.path.join(tmp_dir, "worker_{}.yaml".format(i))
        write_yaml_config(worker_cfg, worker_path)
        proc = subprocess.Popen([sys.executable, GENERATOR_PATH, "-c", worker_path])
        worker_procs.append(proc)
    print("[spawn] launched {} worker process(es).".format(workers))
    return worker_procs


def monitor(cursor, worker_procs, run_seconds, monitor_interval_seconds):
    """Poll the DB-wide aggregate rate every monitor_interval_seconds until
    run_seconds elapses or the caller is interrupted (Ctrl+C). Returns
    (total_events, mean_rate, interrupted).
    """
    reset_pg_stat_statements(cursor)

    run_start = time.monotonic()
    last_sample_time = run_start
    last_sum = query_events_sum(cursor)
    samples = []
    interrupted = False
    dead_workers_reported = set()

    try:
        while True:
            elapsed_total = time.monotonic() - run_start
            if elapsed_total >= run_seconds:
                break

            sleep_for = min(monitor_interval_seconds, run_seconds - elapsed_total)
            if sleep_for > 0:
                time.sleep(sleep_for)

            now = time.monotonic()
            cur_sum = query_events_sum(cursor)
            interval = now - last_sample_time
            rate = (cur_sum - last_sum) / interval if interval > 0 else 0.0
            samples.append(rate)
            print(
                "[monitor] aggregate rate = {:.1f} ev/s (t={:.0f}s, total events so far: {})".format(
                    rate, now - run_start, cur_sum
                )
            )
            last_sample_time = now
            last_sum = cur_sum

            for idx, p in enumerate(worker_procs):
                if idx in dead_workers_reported:
                    continue
                if p.poll() is not None:
                    print(
                        "[monitor] WARNING: worker {} exited early with code {}".format(
                            idx, p.returncode
                        )
                    )
                    dead_workers_reported.add(idx)
    except KeyboardInterrupt:
        print("[monitor] received Ctrl+C, stopping all workers...")
        interrupted = True

    total_events = last_sum
    mean_rate = sum(samples) / len(samples) if samples else 0.0
    return total_events, mean_rate, interrupted


def main():
    parser = argparse.ArgumentParser(
        description="Parallel wrapper/orchestrator for the event generator"
    )
    parser.add_argument(
        "-c",
        "--config",
        default=None,
        help="Path to base event-generator YAML config (must include a top-level 'parallel' block)",
    )
    args = parser.parse_args()

    base_config = load_event_generator_config(args.config)
    gen = base_config["generator"]

    base_seed = gen.get("random_seed", gen.get("seed"))
    if base_seed is None:
        base_seed = 0

    rate_control = gen.get("rate_control")
    if not rate_control:
        print(
            "ERROR: base config must have a generator.rate_control block. "
            "Its rates are treated as the DESIRED TOTAL across all workers."
        )
        sys.exit(1)

    parallel_cfg = base_config.get("parallel") or {}
    max_workers = parallel_cfg.get("max_workers", 6)
    calibration_seconds = parallel_cfg.get("calibration_seconds", 30)
    margin = parallel_cfg.get("margin", 1.3)
    run_seconds = parallel_cfg.get("run_seconds", 1800)
    monitor_interval_seconds = parallel_cfg.get("monitor_interval_seconds", 5)

    tmp_dir = tempfile.mkdtemp(prefix="parallel_generator_")
    conn = None
    worker_procs = []

    try:
        conn = psycopg2.connect(**get_connection_kwargs_from_config(base_config))
        conn.autocommit = True
        cursor = conn.cursor()

        try:
            per_worker_ceiling = run_calibration(base_config, tmp_dir, calibration_seconds, cursor)
        except RuntimeError as e:
            print("ERROR: {}".format(e))
            sys.exit(1)

        target_peak = peak_target(rate_control)
        workers, reachable = compute_workers(target_peak, per_worker_ceiling, max_workers, margin)
        per_worker_target = rate_control.get("default_events_per_second", 0) / workers

        print(
            "[plan] per_worker_ceiling={:.1f} ev/s, workers={}, "
            "per-worker baseline target~{:.1f} ev/s".format(
                per_worker_ceiling, workers, per_worker_target
            )
        )
        if not reachable:
            achievable = workers * per_worker_ceiling
            print(
                "[plan] WARNING: requested peak {:.1f} ev/s not reachable with "
                "max_workers={}; achievable ~{:.1f} ev/s with {} workers".format(
                    target_peak, max_workers, achievable, workers
                )
            )

        worker_procs = spawn_workers(base_config, workers, base_seed, tmp_dir)

        total_events, mean_rate, interrupted = monitor(
            cursor, worker_procs, run_seconds, monitor_interval_seconds
        )

        print(
            "[summary] total_events={} mean_aggregate_ev_s={:.1f} workers_used={}{}".format(
                total_events, mean_rate, workers, " (interrupted)" if interrupted else ""
            )
        )
    finally:
        terminate_processes(worker_procs)
        if conn is not None:
            try:
                conn.close()
            except Exception:
                pass
        shutil.rmtree(tmp_dir, ignore_errors=True)


if __name__ == "__main__":
    main()
