# Status & reporting commands

Why it matters: `export data status`, `import data status`, `cutover status`, and `get data-migration-report` are how users (and automation) observe migration progress. They have flow-specific quirks — e.g. `import data status` returns exit 1 in live mode — and are easy to break without any data-path test noticing. QA "commonly impacted" area.

## Scenarios

### status-1: offline status parity
- Flow: offline
- Origin: QA priority; the report-parity oracle
- Setup: base fixture, completed offline import
- Command: `export data status --output-format json` and `import data status --output-format json`
- Expected oracle: per-table exported == imported counts, and both == actual source/target `count(*)`. Table set complete (no table missing from the report).
- Status: seeded

### status-2: status during an in-progress import
- Flow: offline
- Origin: progress reporting under partial completion
- Setup: large fixture; poll status while import runs
- Command: `import data status` repeatedly during import
- Expected oracle: counts increase monotonically toward the total; no negative/overflow counts; final == total.
- Status: seeded

### status-3: live-mode status quirks
- Flow: live
- Origin: `import data status` exits 1 in live mode; `get data-migration-report` is the live source of truth
- Setup: live migration mid-CDC
- Command: `import data status` (expect non-zero + clear message), `get data-migration-report --output-format json`
- Expected oracle: the exit-1 behavior is documented and the message tells the user to use `get data-migration-report`; the report reconciles snapshot + CDC event counts (inserts/updates/deletes) with injected deltas.
- Status: seeded

### status-4: cutover status transitions
- Flow: live
- Origin: state-transition oracle
- Setup: live migration through cutover
- Command: `cutover status` polled across the cutover
- Expected oracle: progresses to `COMPLETED`; the export/import daemons exit; no orphaned PIDs; status is stable/idempotent after completion.
- Status: seeded

### status-5: status on a fresh / missing / ended export dir
- Flow: any
- Origin: negative/edge
- Setup: (a) export dir with no run yet; (b) after `end migration`
- Command: the status commands against each
- Expected oracle: clear message (no panic / stack trace) for the empty case; sensible output after end migration.
- Status: seeded
