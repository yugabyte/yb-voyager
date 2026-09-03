# metrics Package — Engineering Standards

> **Scope:** files under `yb-voyager/src/metrics/`, plus the metrics wiring in `cmd/` and the recording call sites in `src/importdata/` and the export reporters. These are standards for **writing** code here, not only for reviewing it.
> Repo-wide standards live in `AGENTS.md` at each parent directory up to the repo root — read those as well.

## Surface and CLI wiring

`--metrics-port <port>` (default `0`, disabled) exposes a Prometheus registry at `GET http://<host>:<port>/metrics` on its own `http.ServeMux`, on both import (`import data`, `import data file`, `import data to target/source/source-replica`) and export (`export data from source/target`) commands. `--profile` starts the pprof server and is otherwise independent, but on import commands, if `--profile` is set and no `--metrics-port`/`--prometheus-metrics-port` is given, metrics still start on the role's legacy default port (9101 target/9102 import-file/9103 source-replica/9104 source, `cmd/importData.go`'s `legacyProfileDefaultMetricsPorts`) with a deprecation warning — this preserves pre-`--metrics-port` behavior. Export commands have no legacy default port and stay disabled unless a port is explicitly set. `--prometheus-metrics-port` is a deprecated hidden alias for `--metrics-port` (still works, logs a warning); `--metrics-port` wins if both are set.

## Naming and labels

Metric names follow the `yb_voyager_<import|export>_data_<snapshot|cdc>_*` scheme; direction is encoded in the name, so import metrics carry `importer_role` and export metrics carry `exporter_role` (no generic `role` label). `migration_uuid, session_id` are on every metric.

## Metric catalogue

Metric catalogue:
- `yb_voyager_import_data_snapshot_rows_total`, `yb_voyager_import_data_snapshot_bytes_total` (counters) — rows/bytes imported during snapshot. Labels: `+ importer_role, table_name, schema_name`. Pre-registered at 0 per table so panels aren't empty before the first batch; not seeded from persisted per-table totals, so they reset to 0 on every process restart (use `rate()`/`increase()` for a per-run view). A gauge-based cross-resume cumulative view is planned for a future PR.
- `yb_voyager_import_data_snapshot_batch_{created,submitted,ingested}_total` (counters) — batch lifecycle. Same labels as above. (The in-flight gauge derived from submitted-minus-ingested was dropped; compute it in PromQL instead.)
- `yb_voyager_import_data_snapshot_batch_size_{rows,bytes}` (histograms) — per-batch size distribution. Same labels.
- `yb_voyager_import_data_snapshot_table_last_batch_ingested_timestamp_seconds` (gauge) — Unix timestamp of the most recent ingest per table; used to detect stalls. Same labels.
- `yb_voyager_import_data_snapshot_table_expected_rows` (gauge) — expected total rows for the table (the denominator); stays a gauge (it's a target, not a cumulative count — distinct from `..._rows_total`). Same labels.
- `yb_voyager_import_data_snapshot_tables_total` (gauge), `yb_voyager_export_data_snapshot_tables_total` (gauge) — number of tables in scope for the snapshot phase; set once from all tasks (not just pending ones), so it stays accurate across resume. Labels: `+ importer_role` / `+ exporter_role` respectively.
- `yb_voyager_import_data_errors_total`, `yb_voyager_import_data_error_bytes_total` (counters) — import errors by `error_kind` (`row_processing`, `batch_ingestion`). Same labels `+ error_kind`.
- `yb_voyager_import_data_cdc_events_total` (counter) — CDC events imported by `event_type` (`insert`/`update`/`delete`). Labels: `+ importer_role, event_type`. (Use `rate()` for an events/sec view; the separate rate gauge was dropped.)
- `yb_voyager_import_data_cdc_events_pending`, `yb_voyager_import_data_cdc_estimated_seconds_to_catch_up` (gauges) — CDC lag and ETA. Labels: `+ importer_role`.
- `yb_voyager_export_data_snapshot_rows_total` (counter) — exported snapshot rows per table; a delta-tracking counter fed by the export status reporter's cumulative counts (was a gauge). Labels: `+ exporter_role, table_name, schema_name`.
- `yb_voyager_export_data_snapshot_table_expected_rows` (gauge) — expected total rows for the table during snapshot export. Same labels.
- `yb_voyager_export_data_cdc_events_total` (counter) — total CDC events exported. Labels: `+ exporter_role`.
- `yb_voyager_import_data_parallelism` (gauge) — current import parallelism (adaptive level, or the fixed `--parallel-jobs` value emitted once when adaptive parallelism is disabled). Labels: `+ importer_role`.
- `yb_voyager_export_data_parallelism` (gauge) — configured export parallelism. Labels: `+ exporter_role`.
- `yb_voyager_cluster_node_cpu_percent` (gauge) — per-node target cluster CPU usage. Labels: `+ node`.

## Dropped metrics

Do not reintroduce these without a reason that survives the one they were dropped for.

Dropped (see PR review): `yb_voyager_import_snapshot_batches_in_flight` (derive via PromQL), `yb_voyager_cdc_import_rate_events_per_second` (use `rate()`), `yb_voyager_export_errors_total`, `yb_voyager_import_pool_pending_close_connections`, `yb_voyager_cdc_debezium_up` (deferred to a future PR exposing Debezium's own metrics).
