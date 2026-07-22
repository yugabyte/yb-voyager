# Cascade trimmer controller — event generator dynamic worker pool

Design date: 2026-07-22. Supersedes the within-phase reactive add/kill logic in
`parallel_generator.py` (`decide_adjustment` bang-bang) with a **cascade
controller**: one persistent throttled "trimmer" worker is a continuous *fine
knob* driven by an integral feedback loop, and the uncapped full workers are a
*coarse base* that changes only when the fine knob saturates.

Motivation: the old within-phase loop killed/added whole uncapped workers on
overshoot/undershoot. When one full worker's throughput straddled the target
deadband (e.g. bisect100: 1 worker ≈ 10.8k, trimmer ≈ 0.6k, target 10k), it
limit-cycled — kill → collapse to trimmer-only (~0.6k) → add → overshoot → kill.
The fix makes the *throttled* worker the control variable and only moves whole
workers when the throttle runs out of range.

## Files touched

- `rate_governor.py` — add `RateGovernor.set_rate(rate)` (pure).
- `utils.py` — add `--control-file` to `build_worker_arg_parser`; add a pure
  `read_control_rate(path, last)` helper.
- `generator.py` — periodically re-read the control file and call
  `GOVERNOR.set_rate`; interpret `rate <= 0` as *pause*.
- `parallel_generator.py` — new pure decision functions + rewired control loop
  (cascade path gated on `allow_throttle=True`; old path kept for `False`).
- test files alongside each.

## Worker side (Option B — runtime-adjustable throttle)

### `RateGovernor.set_rate(rate: float)`
Sets `self.default_events_per_second = float(rate)`. The next `pace()` call sees
`target_rate_at` return the new value; the existing `target != current_target`
branch resets `window_start`/`window_events`, so the new rate takes effect
cleanly without a respawn. Pure; no I/O. `rate` may be 0 (means the worker paces
to ~0 ev/s — see pause below). Do NOT let `set_rate(0)` turn the worker uncapped;
the governor stays engaged. A schedule (spike windows) is never present on a
trimmer, so only `default_events_per_second` matters.

### `utils.read_control_rate(path, last) -> float`
Pure/testable. Reads the single float in `path`. On success returns it. On
missing file, empty content, parse error, or any exception returns `last`
(hold). Never raises. Negative values are returned as-is (caller treats `<=0`
as pause).

### `--control-file` CLI arg + worker loop
- Add `--control-file` (default `None`) to `build_worker_arg_parser`.
- Only the trimmer is launched with `--control-file`; uncapped workers are not.
- The trimmer is ALWAYS launched with an initial `--throttle > 0` so it starts
  in governed mode; the control file then commands it down to 0 (pause) without
  the spawn-time `--throttle=0 == uncapped` footgun ever applying.
- In the worker loop, immediately before `GOVERNOR.pace(events_emitted)`
  (generator.py:582), at most once per ~1s (time-gated, using the same clock):
  `new_rate = utils.read_control_rate(control_file, last_rate)`; if changed,
  `GOVERNOR.set_rate(max(new_rate, 0.0))` and remember `last_rate`.
- **Pause semantics:** if the commanded rate is `<= 0`, the worker must emit
  ~nothing (governor paces to 0 → effectively sleeps). It must NOT go uncapped
  and must NOT exit. Implement by setting the governor rate to a tiny epsilon
  (e.g. 1.0) OR by having the worker skip its write and sleep briefly when the
  commanded rate is `<= 0`. Prefer the epsilon-floor if simpler, but never 0 as
  a literal `--throttle` re-interpretation.

## Controller side — pure decision functions (unit-tested)

Add to `parallel_generator.py`:

### `fine_trim(trimmer_rate, achieved, target, C, kp=0.6, deadband_pct=2.0, slew_pct=50.0, freeze_increase=False) -> float`
The integral fine knob. Returns the new commanded trimmer rate.

`freeze_increase` (anti-windup during actuator ramp — added after review
validation): when True, the knob may DECREASE but never INCREASE. The loop sets
it during the post-spawn cooldown (right after a phase-change feed-forward jump
or a coarse add/remove), because a just-spawned uncapped worker is still ramping;
raising the trimmer while `achieved` is transiently low would wind up and
overshoot when the worker reaches full. Closed-loop simulation showed a ~35%
one-tick overshoot on every spike onset without this guard, and 0% with it.
```
error = target - achieved
if abs(error) <= target * deadband_pct/100:  return trimmer_rate   # hold, no twitch
step = kp * error
max_step = C * slew_pct/100
step = clamp(step, -max_step, +max_step)      # slew limit
new = trimmer_rate + step
return clamp(new, 0.0, C)                      # anti-windup
```
Stability: closed loop `error_next = (1-kp)*error` for the unsaturated region →
monotone decay for `0 < kp <= 1`. Keep kp <= 1.

### `decide_coarse(n_full, trimmer_rate, achieved, target, C, cap, sat_high_ticks, sat_low_ticks, sat_ticks_needed=2) -> "add"|"remove"|"hold"`
The coarse base knob. `sat_high_ticks`/`sat_low_ticks` are consecutive-tick
counters the caller maintains.
```
high_sat = trimmer_rate >= C * 0.98
low_sat  = trimmer_rate <= C * 0.02
under = achieved < target
over  = achieved > target

# ADD: fine knob pinned high and still under target, room under cap.
if high_sat and under and sat_high_ticks >= sat_ticks_needed and n_full < cap:
    return "add"

# REMOVE: fine knob pinned low and still over target, AND removing keeps the
# target reachable: after removal, max achievable = (n_full-1)*C + C = n_full*C.
if low_sat and over and sat_low_ticks >= sat_ticks_needed and n_full > 0 \
   and (n_full * C) >= target:
    return "remove"

return "hold"
```
The `(n_full * C) >= target` guard is the ANTI-HUNT guard: it refuses to remove
a worker when the target would land in the unreachable gap between
"trimmer-only max C" and "one more full worker". In that case the loop holds at
a bounded overshoot and relies on recalibration to raise C.

Feed-forward on the coarse transition (bumpless handoff), applied by the caller:
- on "add": after spawning, `trimmer_rate := clamp(trimmer_rate - C, 0, C)`.
- on "remove": after killing newest uncapped, `trimmer_rate := clamp(trimmer_rate + C, 0, C)`.
Then the integral loop cleans up the residual over the next ticks.

## Controller loop rewiring (cascade path, `allow_throttle=True`)

Keep: reap/respawn, `reactive_worker_cap`, crash backpressure
(`crashed_this_interval`), `recalibrate_C`, CSV/monitor logging, calibration.

Per-tick order (this ORDER matters — recalibrate before coarse):
1. Reap dead workers, respawn preserving role/throttle (trimmer keeps its
   control-file path so it resumes at the current commanded rate).
2. Measure `achieved` (unchanged meter).
3. **Phase change** (`target != current_target`): feed-forward jump — set
   `n_full_target = floor(target/C)` (spawn/kill uncapped to match, respecting
   max_workers/cap), ensure the persistent trimmer exists, and set
   `trimmer_rate := target - n_full_target*C` (clamped `[0,C]`). Reset saturation
   counters and cooldown.
4. **Within phase** (`target == current_target`):
   a. Recalibrate C if `n_uncapped >= 1` and stable (existing gate), BEFORE (b).
   b. Coarse: update `sat_high_ticks`/`sat_low_ticks` from current
      `trimmer_rate` vs C; if past cooldown, call `decide_coarse`. On "add"
      (respect crash-backpressure + `reactive_worker_cap` + max_workers) spawn an
      uncapped worker and apply the −C feed-forward; on "remove" kill newest
      uncapped and apply the +C feed-forward; reset counters + cooldown on any
      change.
   c. Fine: `trimmer_rate = fine_trim(trimmer_rate, achieved, target, C_live, ...)`.
5. Write `trimmer_rate` to the trimmer's control file **atomically** (write a
   temp file in the same dir, `os.replace`). Do this whenever it changed.
6. Log/CSV: add a `trimmer_rate` column (commanded ev/s) alongside existing.

Trimmer lifecycle: spawned once (persistent), never killed except on crash
(reap respawns it) or a phase change that legitimately wants 0 full + 0 trimmer
(target 0). It uses one stable control-file path (e.g.
`<tmp_dir>/trimmer_rate.txt`); uncapped workers never receive `--control-file`.

`allow_throttle=False`: leave the existing `decide_adjustment` add/kill path
exactly as-is (no trimmer, no fine knob).

## Edge cases (all evaluated — must be covered or provably safe)

1. `target == 0`: command trimmer to pause (`<=0`); do not turn it uncapped; do
   not exit. `n_full` → 0.
2. `target < C`: 0 full, trimmer ≈ target via integral. No coarse action.
3. **C underestimated (bisect100)**: recalibrate-before-coarse raises C to real
   per-worker; if it can't in one step, the `n_full*C >= target` guard prevents
   hunting — the loop parks at a bounded overshoot until C catches up.
4. Cluster-bound (achieved stuck low): trimmer winds to C, coarse-adds up to
   `reactive_worker_cap`, then holds. Reported honestly, no spiral.
5. Meter noise / one-tick spike: deadband + slew clamp bound the reaction to the
   trimmer only; self-corrects next tick.
6. pg_stat_statements reset (`delta<0` → achieved≈0 one tick): slew clamp bounds
   the false +error kick; recovers next tick.
7. Worker crash/respawn: reap respawns trimmer with same control-file path →
   resumes at commanded rate. Crash-backpressure blocks coarse-add during
   distress.
8. Phase changes (baseline↔spike): feed-forward jump gives an immediate mix, no
   slow integral ramp.
9. Coarse add and remove states are mutually exclusive (trimmer can't be pinned
   high and low at once) → no flapping. Saturation counters + cooldown prevent
   premature coarse action during integral settling.

## Tests (TDD — write first, then implement)

- `fine_trim`: hold within deadband; proportional step; slew clamp both signs;
  anti-windup clamp to [0,C]; converges monotonically over several ticks
  (simulate closed loop with a fake plant achieved = base + trimmer).
- `decide_coarse`: add when high-sat+under+room; remove when low-sat+over+guard
  passes; NO remove when `n_full*C < target` (anti-hunt); hold otherwise;
  respects sat_ticks_needed and cap; n_full>0 for remove.
- `set_rate`: changes effective target; next pace resets window; rate 0 does not
  make it uncapped.
- `read_control_rate`: valid float; missing file → last; empty → last; garbage →
  last; negative returned as-is.
- Integration-style (pure, fake plant): full bisect100-like scenario
  (C_est=9400, real per-worker=10400, target 10000) converges to a stable state
  with NO sustained oscillation once recalibration is allowed.

## Post-validation fixes (2026-07-22, from the first live run + yb_stats ground truth)

The first validation run confirmed the coarse knob no longer hunts (workers held
steady through every spike; yb_stats confirmed all three schemas reach ~10k). It
also exposed two real defects, both fixed:

1. **Meter noise → C erosion → overshoot.** The raw pg_stat_statements rate at a
   5s interval swings +/-50% even when true throughput is steady (yb_stats). Fed
   to the fine knob it thrashes; with recalibrate on, the low readings dragged C
   down (bisect100: 8469 -> 4801), so the coarse knob over-added a worker and
   real throughput hit ~2x target. Fix: `TrailingRateWindow` -- the controller
   now acts on a ~`meter_window_seconds` (default 15s) moving average of the
   monotonic cumulative counter, feeding fine_trim, decide_coarse saturation, and
   recalibrate. This denoises without banking error (input counter is monotonic).

2. **Trimmer freeze at pause.** Commanding the trimmer <= 0 floored it to a 1 ev/s
   epsilon and kept running full 300-row batches; one batch at 1 ev/s makes
   RateGovernor.pace() sleep ~300s, during which the worker cannot re-read its
   control file to see the rate rise again (observed: trimmer stuck at 0 through
   a whole baseline recovery). Fix: (a) worker treats commanded rate <= 0 as
   PAUSE -- skip the DML op and idle CONTROL_FILE_POLL_SECONDS, re-reading each
   pass; (b) `RateGovernor.max_single_sleep_seconds` caps any single pace() sleep
   (trimmer sets 3s) as defense for tiny positive rates. Never triggers at real
   trimmer rates (hundreds+ ev/s -> sub-second pacing).

## Non-goals
- No change to calibration, the underlying meter tracker
  (`CrossNodeProgressTracker`), PK/seed logic, or the `allow_throttle=False` path.
