# Event Generator — Rate Governor Design

**Date:** 2026-07-13
**Component:** `migtests/scripts/event-generator/`
**Status:** Approved design, ready for implementation

## Problem

The event generator (`generator.py`) runs its iteration loop as fast as the
database accepts statements. The only throttle is open-loop
(`wait_after_operations` + `wait_duration_seconds`), which cannot hold a target
rate because it ignores how many events were emitted and how much time elapsed.
As a result the generator cannot produce a known, controllable CDC
**events/second** rate.

We need this for a live-migration fall-back (YB→PG) throughput test: a 24-hour
run at a ~1.5k events/sec baseline with periodic 10k events/sec spikes, to
observe how the CDC pipeline absorbs and recovers from load. The generator must
be able to hold a chosen baseline rate and inject recurring spike windows.

## Goals

- Hold a configurable steady **events/second** rate (events = rows changed).
- Support recurring spike windows layered on the baseline.
- Be fully optional and backward compatible: omitting the config preserves
  today's behavior exactly.
- Be reusable/holistic: a general schedule model, not hard-coded to one spike.
- Be simple to read and configure, with in-config documentation.

## Non-goals

- No duration-based stop knob (`max_runtime_seconds`) for now — a 24h run uses
  `num_iterations: -1` plus an external `timeout 24h ...` or Ctrl+C. Easy to add
  later.
- No spike ramp (gradual ease-in/out) for now — spikes are step changes.
- No per-table rate distribution — the governor paces the aggregate event rate;
  table selection remains weight-driven as today.

## Config schema

A single optional `rate_control` block under `generator:` in
`event-generator.yaml`.

```yaml
rate_control:
  default_events_per_second: 1500   # baseline rate when no spike is active
  report_interval_seconds: 60       # log achieved ev/s every N s (omit/0 = off)
  schedule:                         # optional list of recurring spike windows
    - events_per_second: 10000      # spike target rate
      duration_seconds: 300         #   spike lasts 5 min
      every_seconds: 1800           #   one spike per 30 min (the period)
      offset_seconds: 600           #   first 10 min of each period stay at baseline
      jitter_pct: 10                #   ±10% randomization of spike start & rate (seeded)
```

### Nullability rules

| Config state | Behavior |
| --- | --- |
| `rate_control` omitted | No pacing. Runs as fast as the DB allows (today's behavior). `wait_after_operations`/`wait_duration_seconds` still apply. |
| `rate_control` present, no `schedule` | Steady rate at `default_events_per_second`. |
| `rate_control` present, with `schedule` | Baseline plus recurring spike windows. |

When `rate_control` is present, the legacy `wait_after_operations` /
`wait_duration_seconds` knobs are ignored (a one-line warning is printed at
startup), since they would fight the governor.

### Field reference

- `default_events_per_second` (required when block present): baseline target rate
  when no schedule entry is active. Must be > 0.
- `report_interval_seconds` (optional): if > 0, the governor logs the achieved
  rate, current target, and cumulative event count every N seconds. Omit or 0
  disables reporting.
- `schedule` (optional): list of recurring spike windows. Each entry:
  - `events_per_second` (required): spike target rate. Must be > 0.
  - `duration_seconds` (required): length of each active spike window. Must be > 0.
  - `every_seconds` (required): the period; one spike window per period. Must be > 0.
  - `offset_seconds` (optional, default 0): how far into each period the spike
    window starts. Sets the phase / first-spike delay.
  - `jitter_pct` (optional, default 0): randomizes each spike window's start time
    and rate by up to ±this percent, drawn deterministically from `random_seed`.

### Validation (fail fast at startup, clear message)

- `default_events_per_second` > 0.
- For each schedule entry: `events_per_second` > 0, `duration_seconds` > 0,
  `every_seconds` > 0, `offset_seconds` >= 0, `0 <= jitter_pct <= 50`.
- `offset_seconds + duration_seconds <= every_seconds` — so a spike window fits
  inside its period and does not bleed into the next.
- Unknown keys inside `rate_control` / schedule entries raise a warning (typo
  guard) but are non-fatal.

## Semantics

**Event = one changed row.** After each operation the generator reads
`cursor.rowcount` (rows actually inserted/updated/deleted) and reports that count
to the governor. This makes the target honest: an INSERT of `insert_rows` counts
its batch; UPDATE/DELETE count whatever `TABLESAMPLE` actually hit; a
failed/rolled-back op counts 0.

**Effective target at time *t*** = the **maximum** `events_per_second` among all
schedule entries active at *t*; if none are active, `default_events_per_second`.
Max-wins is the rule when windows overlap — the more aggressive load takes
precedence.

**A schedule entry is active** during
`[k·every + offset + startJitter(k),  … + duration)` for k = 0, 1, 2, …

**Jitter** is a pure, deterministic function of `(random_seed, entry_index,
window_index k)`:
- start jitter: `uniform(-1, 1) · (jitter_pct/100) · every_seconds` added to the
  window's start time.
- rate jitter: `events_per_second · (1 + uniform(-1, 1) · jitter_pct/100)`.

Because jitter can shift a window's start, `target_rate_at(elapsed)` checks
window indices `k-1, k, k+1` around `k = floor((elapsed - offset)/every)`.

### Diagram (goes verbatim into `event-generator.yaml` comments)

```
Anatomy of one period (every_seconds):

  |<--------------------- every_seconds (30 min) --------------------->|
  |<--- offset --->|<-- duration -->|<--------- recovery tail -------->|
  |    (10 min)    |    (5 min)     |            (15 min)              |
  +================+################+==================================+
       baseline          SPIKE                   baseline
  ^period start                                      ^next period start

Repeating over the run   (░ = baseline, █ = spike):

  t(min): 0        10   15            30   40   45            60
          ░░░░░░░░░░█████░░░░░░░░░░░░░░░░░░░░█████░░░░░░░░░░░░░░░  ...
          └─ period 1 (30m) ──────────────┘└─ period 2 (30m) ──┘

  Gap between consecutive spikes = every_seconds − duration_seconds
                                 = 1800 − 300 = 1500 s (25 min),
  independent of offset (offset only sets the phase / first-spike delay).
```

## Pacing algorithm

Piecewise-constant deadline pacing with a **reset at each target transition**.
This holds the rate accurately within a constant-rate window and makes a spike
take effect immediately, with no integral math.

State: `run_start` (monotonic), `window_start`, `window_events`,
`current_target`, `total_events`, `last_report`, `report_events`.

On `pace(n)`:
1. `now = clock()`, `elapsed = now - run_start`.
2. `target = target_rate_at(elapsed)`.
3. If `target != current_target`: reset the window
   (`window_start = now`, `window_events = 0`, `current_target = target`).
4. `window_events += n`; `total_events += n`; `report_events += n`.
5. If `target > 0`:
   - `window_elapsed = now - window_start`
   - `allowed = target · window_elapsed`
   - if `window_events > allowed`:
     `sleep_needed = window_events / target - window_elapsed`;
     if `sleep_needed > 0`, `sleep(sleep_needed)`.
6. Reporting: if `report_interval > 0` and `now - last_report >= report_interval`:
   log `achieved = report_events / (now - last_report)`, `current_target`,
   `total_events`; reset `report_events = 0`, `last_report = now`.

Drift does not accumulate: each call recomputes `allowed` from the true window
elapsed, so small per-iteration errors self-correct.

## Architecture

Three focused pieces; the governor is isolated and unit-testable without a DB.

### `rate_governor.py` (new file, pure stdlib — no psycopg2/Faker/yaml)

```python
class RateGovernor:
    def __init__(self, rate_control: dict, *, random_seed=None,
                 clock=time.monotonic, sleep=time.sleep, log=print): ...
    def pace(self, events_emitted: int) -> None: ...
    # internal: target_rate_at(elapsed), _window_jitter(entry_index, k)

class NullGovernor:
    def pace(self, events_emitted: int) -> None:  # no-op
        pass
```

- Clock, sleep, and log are injected so tests use a fake clock and never sleep.
- Pure stdlib so its unit tests run anywhere (no DB, no third-party deps).

### `utils.py`

- `validate_rate_control(rc: dict) -> None` — raises `ValueError` with a clear
  message on any violation above. Called from `load_event_generator_config`.
- `build_rate_governor(config, **injectables) -> RateGovernor | NullGovernor` —
  returns `NullGovernor` when `rate_control` is absent, else a configured
  `RateGovernor` (passing `random_seed` from config for deterministic jitter).

### `generator.py`

- Construct the governor once (via `build_rate_governor`) after config load.
- If `rate_control` present, skip the legacy `wait_after_operations` block and
  print a one-line warning that it is ignored.
- After each operation, capture `n = cursor.rowcount` (0 on failure/rollback) and
  call `governor.pace(n)` once per iteration.

## Data flow

```
event-generator.yaml
      │  load_event_generator_config (+ validate_rate_control)
      ▼
   CONFIG ──► build_rate_governor ──► RateGovernor | NullGovernor
                                            ▲
loop: pick table+op ─► execute ─► n = cursor.rowcount ─► governor.pace(n)
                                                           (sleeps to hold target,
                                                            logs periodic report)
```

## Error handling

- Monotonic clock — immune to wall-clock changes over a 24h run.
- Failed op → `cursor.rowcount` may be 0/-1 → treated as 0 events; pacing
  continues.
- `sleep_needed` is only applied when positive.
- Invalid `rate_control` is rejected before any DB connection/work.

## Testing (TDD, `rate_governor` unit tests with a fake clock)

- Baseline only: with a fake clock advancing in fixed steps, `pace()` sleeps to
  hold `default_events_per_second`; achieved rate ≈ target.
- No `rate_control` → `NullGovernor.pace()` never sleeps.
- Spike window: `target_rate_at` returns baseline outside the window and the
  spike rate inside `[offset, offset+duration)` of each period.
- Offset: first spike starts at `offset`, not at t=0; spikes repeat every
  `every_seconds`.
- Overlap: two active entries → max rate wins.
- Jitter: start/rate stay within ±`jitter_pct` bounds and are identical across
  two runs with the same `random_seed` (determinism), and differ without a seed.
- Reporting: a report is emitted once per `report_interval_seconds` with the
  correct achieved rate; disabled when omitted/0.
- Validation: each invalid field and the `offset + duration > every` case raise
  `ValueError` with a clear message.

## Backward compatibility

- `rate_control` is purely additive and optional. Existing configs and migtests
  are unaffected.
- No change to `MigrationStatusRecord` or any serialized Voyager state — this is
  a standalone test-harness script.

## Example: the 24h fall-back throughput test

```yaml
generator:
  # ... existing connection/table/operation config ...
  num_iterations: -1                # run until stopped
  rate_control:
    default_events_per_second: 1500
    report_interval_seconds: 60
    schedule:
      - events_per_second: 10000
        duration_seconds: 300       # 5-min spike
        every_seconds: 1800         # every 30 min
        offset_seconds: 600         # 10-min baseline lead-in each period
        jitter_pct: 10
```

Run against the YB source (fall-back source) with e.g.
`timeout 24h python3 generator.py -c event-generator.yaml`.
Average produced rate = (300·10000 + 1500·1500)/1800 ≈ 2917 ev/s, below the
connector's ~4–5k drain, so each spike's ~1.8M-event backlog clears within the
25-min gap before the next spike.
