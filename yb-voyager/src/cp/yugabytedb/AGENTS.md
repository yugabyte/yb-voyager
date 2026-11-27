# yugabytedb control plane

Reports a migration into the `voyager_core` tables of a YugabyteDB/PostgreSQL
database. The tables and what Voyager writes to them are in
[`voyager_core_schema.md`](voyager_core_schema.md). Voyager creates none of
them; `Init` fails if one is missing or not writable.

When `VOYAGER_OPERATION_ID` is set, a command also records its run in the
platform's `step_execution` table; see [Step claiming](#step-claiming).

The type is internal: it is not in the config templates or in the
`control-plane-type` error message.

## Step claiming

A platform that launches Voyager sets `VOYAGER_OPERATION_ID` (and optionally
`EXECUTOR_REF`). Each command then owns one `step_execution` row for its run.
Full design: `design-specs/cp-step-claiming.md`.

- **Voyager touches only its own row.** It never expires, closes or otherwise
  modifies another process's row. Cancelling, expiring and rerunning a step
  belong to the platform's controller.
- **A claim is an insert, nothing more.** If the step already has a `running`
  row, live or stale, the claim fails with "already running". The controller
  resolves it.
- **The row stays open for the whole command.** It is closed once, at command
  end: `succeeded` from `PersistentPostRun`, `failed` with the error from the
  exit handler. Phase done events never close it.
- **A done event that fails to write fails the step.** The write error is kept,
  and the end-of-command close records `failed` with it.
- **Every row written while claiming is on is stamped** with the step
  execution id. A write with no active step execution is an error, not an
  unstamped row.
- **The heartbeat signals liveness only.** It runs every 10 s. A failed
  heartbeat is logged and retried and never stops the command. A heartbeat
  that updates 0 rows means the controller closed the row (a cancel), and the
  command stops.
- **Fencing is the controller's job.** A process cut off from the control
  plane keeps running, so before rerunning a step whose row is still `running`
  the controller must stop the old process (for example, delete its Job).
- **cmd talks only to the `cp.ControlPlane` interface.** Step claiming methods
  are on the interface; control planes without step claiming implement them as
  no-ops. cmd never type-asserts a concrete control plane.
- **Child Voyager processes never claim.** They get their environment from the
  control plane's child-env method, with `VOYAGER_OPERATION_ID` removed. A
  child that works for the parent's step (the assessment `export schema` runs)
  also gets `VOYAGER_STEP_EXECUTION_ID` and joins: it stamps rows with that id
  and never claims, heartbeats or closes. The variable is internal: only
  Voyager sets it, for its own children. Setting both variables is an `Init`
  error.
- **`Init` fails fast** when claiming is on and a table or column it uses is
  missing. With both variables unset, no query or statement differs from a run
  without claiming.
- **Offline commands only.** Live, fall-back and fall-forward commands do not
  claim.

## Known gaps

- **Partitioned tables (PostgreSQL source).** Leaf partitions' data files are
  renamed to the root table, so several import tasks share one
  `table_data_progress` key. A batch holding the same key twice fails with
  `ON CONFLICT DO UPDATE command cannot affect row a second time` (21000), so
  the initial `not_started` batch of such an import is dropped. Later updates
  each carry one leaf's counts and overwrite the root's row.
- **Import resume resets progress.** On resume, `import data` first reports
  every pending table as `not_started` with 0 rows, and the upsert overwrites
  the row. It is correct again after the table's next progress update.
- **Live migration is not covered.** Only offline `export data` writes the
  `data.export_*` entries and the `source`/`export` rows. `data.imported` is
  written when `import data` ends, so a live import reads as started
  throughout. `target`/`import` is the only other stream written; the
  fall-back and fall-forward legs report nothing, and there is no iteration
  column.
- **Every failed write is retried.** `exec` makes 5 attempts for any error,
  including ones the server will reject each time, reconnecting between
  attempts, with no connect or statement timeout. `Finalize` waits for every
  queued event, so an unreachable control plane delays the command's exit.
- **A panic in a publisher hangs `Finalize`.** The publisher goroutine exits
  without marking its event done, and later events queue with no reader.
- **`snapshot_rows_estimated` is never NULL for "unknown".** On export it is
  0 until the source's estimate is read, and that estimate can itself be -1
  (PostgreSQL 14+ `reltuples` for a table never vacuumed or analyzed). 0
  therefore also reads as an empty table, and -1 makes a progress percentage
  meaningless.
- **`done` and `completed` are both written as table statuses.** `done` is
  export only and transient: the table's data is dumped but its final row
  count is not in yet. `completed` is the terminal status for both
  directions. The schema doc lists both without saying so.
- **`SCHEMA_ANALYSIS_PAYLOAD_VERSION` is bumped by hand** when
  `utils.SchemaReport` changes shape; nothing checks it.
- **Step claiming: a killed process leaves its row `running`.** Nothing in
  Voyager closes it, so a rerun fails with "already running" until the
  controller closes it. Exit paths that bypass `atexit` (`os.Exit` in a
  signal-free goroutine) are the same.
- **Step claiming: a failed done-event write exits 0.** The step execution is
  closed `failed`, but the command's exit status is unchanged, since the
  write is async.
