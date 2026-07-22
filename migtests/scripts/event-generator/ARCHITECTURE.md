# Event generator — how it works

A reference for the reactive worker-pool event generator used for
YugabyteDB live-migration spike/soak testing. It drives a target database at a
**scheduled events/sec** (a low baseline with periodic high spikes, e.g. 1.5k
baseline / 10k spikes) and holds that rate as closely as the cluster allows,
adding and removing load-generating workers on the fly.

This document explains the moving parts so you don't have to read all ~1,700
lines of `parallel_generator.py` to reason about a run.

---

## 1. The problem it solves

A single Python process cannot generate tens of thousands of rows/sec against a
distributed DB — it is CPU-bound on row generation (Faker) and serialized on one
connection. So load is produced by **many worker processes**. But the *right*
number of workers is not knowable ahead of time: it depends on the per-worker
throughput ceiling `C`, which varies with the schema, the cluster, cold vs warm
caches, and how loaded the cluster already is.

The generator therefore runs a **controller** that measures `C`, provisions the
worker pool to hit the scheduled target, and continuously corrects the achieved
rate back to target.

---

## 2. Two-process architecture

```
                    ┌─────────────────────────────────────────────┐
                    │  CONTROLLER  (parallel_generator.py)          │
                    │                                               │
   schedule ───────▶│  • measures achieved rate (the "meter")      │
 (rate_control)     │  • decides worker count + trimmer throttle   │
                    │  • spawns / kills workers                     │
                    │  • writes the trimmer's rate to a control     │
                    │    file                                       │
                    └───────┬─────────────────────┬────────────────┘
                            │ spawn/kill           │ writes rate
                            ▼                      ▼
        ┌───────────────────────────┐   ┌─────────────────────────┐
        │  UNCAPPED workers (0..N)  │   │  TRIMMER worker (exactly │
        │  generator.py, run flat   │   │  1)  generator.py, paced │
        │  out (no throttle)        │   │  by a RateGovernor that  │
        └───────────────────────────┘   │  re-reads a control file │
                    │                    └─────────────────────────┘
                    ▼                                │
        ┌───────────────────────────────────────────▼─────────────┐
        │              TARGET DB (YugabyteDB, RF3, N tservers)      │
        └──────────────────────────────────────────────────────────┘

  Shared, read once per worker at spawn:
    • SHARED CACHE  (shared_cache.py): schema metadata + a PK snapshot,
      built once, memory-mapped by every worker → fast spawns.
    • PER-SLOT config files: a small reused pool of temp config files
      (one per worker slot) instead of a fresh tempfile per spawn.
```

- **Controller** (`parallel_generator.py`, function `run_controller`): the brain.
  One process. Never generates load itself.
- **Workers** (`generator.py`): each opens **one** DB connection and issues
  `INSERT`/`UPDATE`/`DELETE` in a mix. Two roles:
  - **Uncapped**: run flat out (no pacing). The coarse knob.
  - **Trimmer**: exactly one persistent worker, paced by a `RateGovernor`
    (`rate_governor.py`) to an exact events/sec. The fine knob. It re-reads its
    **control file** each loop, so the controller can change its rate live
    without respawning it.

Workers distribute across all tservers (round-robin, discovered via
`yb_servers()`), because a per-process driver connection can't load-balance a
single connection (`distribute_across_nodes`, default true).

---

## 3. The cascade controller (coarse + fine)

The target rate is hit with a **two-tier cascade**:

| Tier | Actuator | Granularity | Function |
|------|----------|-------------|----------|
| **Coarse** | whole uncapped workers | ~`C` ev/s per step | `decide_coarse` |
| **Fine**  | the trimmer's throttle | continuous (0..`C`) | `fine_trim` |

- `C` = the **per-worker ceiling**: how many ev/s one uncapped worker sustains.
- To hit `target`: run `floor(target / C)` uncapped workers, and let the
  **trimmer** carry the fractional remainder `target − floor(target/C)·C`.
- The **fine knob** (`fine_trim`) is a proportional integral-style controller
  that nudges the trimmer's rate every tick to erase the residual error
  (`error = target − achieved`), inside a deadband, slew-rate limited.
- The **coarse knob** (`decide_coarse`) only acts when the fine knob **saturates**:
  - trimmer pinned **high** (≥98% of `C`) and still under target for
    `sat_ticks_needed` ticks → **add** a worker.
  - trimmer pinned **low** (≤2% of `C`) and still over target → **remove** a
    worker — but only if `n_full·C ≥ target` (the **anti-hunt guard**: never
    remove into an unreachable gap, or the pool oscillates ±1 forever).
- On a coarse add/remove there is a **bumpless handoff**: the trimmer's rate is
  adjusted by ∓`C` so total commanded load doesn't jump when a whole worker
  appears/disappears.

---

## 4. Calibration (with warm-up)

Before the run, `run_calibration` measures `C`:

1. `reset_fn()` clears `pg_stat_statements` counters.
2. Spawn **one** uncapped worker.
3. **Warm-up** (`calibration_warmup_seconds`, default 20s): let that worker run
   *unmeasured* first, so JIT-less Python import, the psycopg2 connection, Faker
   lazy init, the memory-mapped shared cache, and the DB-side caches all reach
   steady state. **Without this, `C` is measured cold and comes out ~3–4× too
   low** (e.g. ~450 instead of ~1,800 ev/s), which makes the phase-change jump
   (§5) massively over-provision workers and overshoot the target.
4. Measure the aggregate rate over `calibration_seconds` → `C`.
5. Kill the calibration worker.

`calib_C` (the calibration value) is kept as a **stable** ceiling used only for
the backpressure cap (§6); the live `C` may drift via recalibration but
`calib_C` never does.

---

## 5. The control loop (per tick)

Every `control_interval_seconds` the controller reads the meter and the current
target, then:

**On a phase change** (target changed, e.g. baseline→spike):
- **Feed-forward jump**: immediately set the pool to `floor(target/C)` uncapped
  workers (reserving one slot for the persistent trimmer), rather than ramping
  up one worker per tick. Set the trimmer's rate to the remainder.

**Within a phase** (target unchanged), in this order:
1. **(0) Overshoot fast-shed** (`decide_overshoot_shed`, safety net): if the
   achieved rate exceeds target by >25% for ≥2 consecutive ticks, the pool was
   over-provisioned (usually a cold-`C` jump that then warmed up). Shed a
   bounded number of workers (≤ half the pool per tick) **and snap `C` upward**
   to the observed per-worker rate. Only ever reduces workers and only ever
   raises `C` — it cannot cause a downward `C` spiral. When it fires it skips
   steps 2–3 this tick.
2. **(a) Recalibrate `C`** (optional, `recalibrate`): EMA-nudge `C` toward the
   observed `(achieved − trimmer_rate)/n_uncapped`, clamped to ≤10%/tick, so the
   ceiling tracks slow drift over a long run without thrashing.
3. **(b) Coarse**: `decide_coarse` may add/remove one worker (§3).
4. **(c) Fine**: `fine_trim` adjusts the trimmer to clean up the residual.

**Anti-windup (`freeze_increase`)**: right after any spawn (phase jump or coarse
add) the new worker is still ramping, so `achieved` reads transiently low.
During the post-spawn cooldown the fine knob may only *decrease*, never
increase — otherwise it winds up and overshoots the instant the worker reaches
full. The overshoot-shed also sets this for the tick it fires.

---

## 6. Key mechanisms & safety rails

- **The meter** (`achieved`): cluster-wide DML rows/sec from
  `pg_stat_statements`. When load is spread across tservers a single connection
  under-counts, so a `CrossNodeEventMeter` sums all nodes. The raw reading is
  jumpy (rows arrive in clumps at a ~5s poll), so it is smoothed by a
  **`TrailingRateWindow`** (`meter_window_seconds`, default 15s) — the controller
  acts on the moving average, not the single-interval delta.
  > Note: on YugabyteDB, `pg_stat_database.tup_*` counters are always 0 — do NOT
  > use them as a meter. Use `pg_stat_statements` (this meter) or `yb_stats`.
- **Backpressure cap** (`reactive_worker_cap`): the reactive add step will not
  grow the pool beyond `ceil(target/calib_C)·reactive_margin`. If achieved is
  below target even with the workers the target *should* need, the bottleneck is
  the **cluster**, not the worker count — adding more only piles load on a
  struggling cluster (the anti-spiral rule). Uses the stable `calib_C`, never the
  eroding live `C`.
- **Cluster-distress guard**: if any worker crashed this tick
  (`crashed_this_interval > 0`), the loop **holds** the pool (respawns to keep it
  level) but never grows it — a dying tserver must not be handed more load.
- **Leak-free churn**: dead workers are reaped every tick and respawned
  (preserving role) so a lost worker never silently shrinks the pool; a bounded
  reused set of per-slot config files avoids a tempfile-per-spawn leak over the
  hundreds of spawn/kill cycles a long run produces.
- **Orphan-proofing** (`PR_SET_PDEATHSIG`, Linux): each worker asks the kernel to
  SIGKILL it if the controller dies, so a killed controller never leaves workers
  hammering the DB. SIGTERM to the controller is converted to KeyboardInterrupt
  so its `finally` block reaps all children.

---

## 7. The worker (`generator.py`) and rate governor

Each worker:
- Loads schema + PK pool from the **shared cache** (memory-mapped, no re-query).
- Loops issuing operations in the configured mix
  (`operation_weights`, default INSERT:UPDATE:DELETE = 6:3:1), with configurable
  rows-per-statement (`insert_rows`/`update_rows`/`delete_rows`) and PK selection
  from a `PkPool`.
- **Uncapped**: no pacing — runs as fast as its connection/CPU allow.
- **Trimmer**: wraps the loop in a `RateGovernor` (`rate_governor.py`) that paces
  to an exact events/sec using piecewise-constant deadline pacing. Each loop it
  re-reads its **control file**; when the controller writes a new rate,
  `GOVERNOR.set_rate(...)` picks it up live (no respawn). A `NullGovernor` is used
  when no pacing is configured.

Per-worker throughput is dominated by **Python row generation (Faker) + CPU**,
not DB latency — so `C` scales with the generator box's CPU, and the target DB is
usually *not* the bottleneck for reaching a given rate (see §9).

---

## 8. Configuration reference (`parallel` block)

Defaults live in `DEFAULT_PARALLEL_CONFIG`; a run's YAML `parallel:` block
overrides them. Most-used knobs:

| Key | Default | Meaning |
|-----|---------|---------|
| `max_workers` | 8 | Hard cap on total workers. Set ≈ `cores − 4` on the generator box. |
| `calibration_seconds` | 30 | Measurement window for `C`. |
| `calibration_warmup_seconds` | 20 | **Warm-up before measuring `C`** (§4). 0 = legacy cold-start. |
| `control_interval_seconds` | 5 | Control loop period. |
| `run_seconds` | 604800 | Total run length. |
| `allow_throttle` | true | Use the cascade (coarse+fine) controller. False = legacy bang-bang path. |
| `recalibrate` | true | Let `C` drift via clamped EMA in stable windows. |
| `reactive_margin` | 1.5 | Backpressure cap = `ceil(target/calib_C) × this`. |
| `distribute_across_nodes` | true | Round-robin workers across tservers. |
| `meter_window_seconds` | 15 | Trailing-window smoothing of the meter. |
| `fine_kp` / `fine_deadband_pct` / `fine_slew_pct` | 0.6 / 2 / 50 | Fine-knob gain / hold-band / max step (%C). |
| `sat_ticks_needed` / `high_sat_pct` / `low_sat_pct` | 2 / 98 / 2 | Coarse-knob saturation trigger. |
| `pk_pool_maxsize` / `pk_stride` | 20000 / 100000 | PK pool sizing/striding. |

The **schedule** lives in `generator.rate_control` (a baseline
`default_events_per_second` plus a list of recurring spike windows with
`events_per_second` / `duration_seconds` / `every_seconds` / `offset_seconds`).

---

## 9. Operational notes, gotchas, known limits

- **The generator box is the usual bottleneck, not the cluster.** Per-worker
  throughput is CPU-bound on Faker row generation; measured DB statement latency
  stays ~flat (≈22ms) from baseline to a 10k spike. To reach a higher sustained
  rate, add generator CPU (bigger/more boxes) — not just more workers on an
  already-saturated box (that just drives `C` down as workers fight for cores).
- **Per-spike cold-start ramp (known limitation).** The pool is torn down at
  baseline and respawned cold at each spike onset. Those fresh workers cold-start
  slowly under mutual CPU contention (`C` transiently collapses), so a short
  (~300s) spike can spend its first ~2–3 min ramping before it reaches target.
  The `calibration_warmup_seconds` fix makes the *initial provisioning* correct
  (right worker count, no giant overshoot), but does not warm the per-spike
  workers. Holding target for the *whole* spike would require keeping a warm pool
  of workers alive across baseline (persistent throttled workers) rather than
  respawning them — a candidate future change.
- **Overshoot vs undershoot.** With a warm `C`, the feed-forward jump lands near
  the right worker count; the overshoot-shed (§5.0) is the guard for the residual
  case where workers warm past expectation. With a cold `C`, the jump
  over-provisions and the shed / recalibration claw it back (slowly on a short
  spike).
- **Stale lockfiles.** A killed run can leave `<exportDir>/.*Lockfile.lck`; delete
  before re-running.
- **Meter caveat.** `pg_stat_statements` must be enabled; `pg_stat_database.tup_*`
  is unusable on YB (always 0).

---

## 10. Output

A timestamped rate CSV (`--rate-csv`): columns
`epoch, t_seconds, target, achieved_evps, n_uncapped, n_trimmer, C, trimmer_rate`
— joinable by absolute time with monitor/Prometheus/`yb_stats` series for
post-run charting.

---

## 11. Where the logic is tested

The **pure decision logic** — `plan_worker_counts`, `decide_coarse`,
`fine_trim`, `decide_overshoot_shed`, `recalibrate_C`, `compute_C_observed`,
`reactive_worker_cap`, `TrailingRateWindow`, `Schedule.target_at`, the roster's
LIFO bookkeeping, and `run_calibration`'s warm-up ordering — has no I/O and is
unit-tested in `test_parallel_generator.py`. The impure orchestration
(subprocess spawn/kill, DB connections) is exercised only in integration.
