# yugabytedb control plane

Reports a migration into the `voyager_core` tables of a YugabyteDB/PostgreSQL
database. The tables and what Voyager writes to them are in
[`voyager_core_schema.md`](voyager_core_schema.md). Voyager creates none of
them; `Init` fails if one is missing or not writable.

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
- **`SCHEMA_ANALYSIS_PAYLOAD_VERSION` is bumped by hand** when
  `utils.SchemaReport` changes shape; nothing checks it.
