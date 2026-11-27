# yugabytedb control plane

Reports a migration into the `voyager_core` tables of a YugabyteDB/PostgreSQL
database. The tables and what Voyager writes to them are in
[`voyager_core_schema.md`](voyager_core_schema.md). Voyager creates none of
them; `Init` fails if one is missing or not writable.

When `VOYAGER_OPERATION_ID` is set, a command also claims its step in the
platform's `step_execution` table, heartbeats it and closes it (see the schema
doc and `design-specs/cp-step-claiming.md`). `Init` then also fails if those
tables or the `step_execution_id` columns are missing. With it unset, no query
or statement differs from before.

`VOYAGER_STEP_EXECUTION_ID` is the internal join mode: a child process (the
assessment `export schema` runs) stamps rows with its parent's step execution
and never claims, heartbeats or closes it. Voyager sets it only for its own
child; setting both vars is an `Init` error.

The type is internal: it is not in the config templates or in the
`control-plane-type` error message.

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
- **Step claiming: a killed process leaves its attempt open.** Nothing closes
  it until the next claimer finds its heartbeat older than 2 minutes. Exit
  paths that bypass `atexit` (`os.Exit` in a signal-free goroutine) are the same.
- **Step claiming: a failed done-transaction exits 0.** The attempt is closed
  `failed`, but the command's exit status is unchanged, since the write is async.
- **Step claiming covers offline commands only.** Live, fall-back and
  fall-forward commands do not claim.
