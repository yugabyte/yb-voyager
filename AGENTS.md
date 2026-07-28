# INSTRUCTIONS.md

Guidance for AI coding agents (Claude Code, Cursor, etc.) working with this repository.

## Project overview

YugabyteDB Voyager (`yb-voyager`) is a database migration CLI tool. It supports offline and live migrations from PostgreSQL, Oracle, and MySQL to YugabyteDB. See `README.md` for end-user details.

## Repository layout

This repo ships two artifacts that together form YugabyteDB Voyager:

- `yb-voyager/` — the Go CLI module (`module github.com/yugabyte/yb-voyager/yb-voyager`, Go 1.24.2). `main.go` is a thin entrypoint; all command wiring and migration logic lives under `yb-voyager/cmd/` and `yb-voyager/src/`.
- `debezium-server-voyager/` — Java/Maven sources for the Debezium CDC plugin used by live migration. Built and installed separately by the installer.

Supporting trees:
- `installer_scripts/install-yb-voyager` — the canonical build/install entry. Handles Go build, Debezium Maven build, ora2pg setup, and dependency provisioning. Has interactive prompts (ora2pg license, bashrc) — pipe `yes` to accept non-interactively.
- `migtests/` — end-to-end migration tests driven by shell + Python (`run-test.sh`, `live-migration-*-run-test.sh`, scripts under `migtests/scripts/`). Tests live under `migtests/tests/<source>/<scenario>/`.
- `.cursor/BUGBOT.md` — product-level review rules. Treat these as load-bearing; see "Product invariants" below.

## Key environment requirements

- **Go 1.24.2** — required by `go.mod`.
- **JDK 17** (not 21+) — the installer script enforces Java 17–19 for the Debezium build. Set `JAVA_HOME` accordingly.
- **Maven 3.8.4** — auto-installed by the installer.
- **PostgreSQL client 17** — provides `pg_dump`/`pg_restore`.
- **YugabyteDB** — target database.
- **`rsync`** — required by the installer's `-l` (local build) flag.

Set these env vars before building or running:
```
export JAVA_HOME=/path/to/jdk17
export PATH=/path/to/go/bin:$JAVA_HOME/bin:$PATH
```

## Build

The repo is normally built via the installer, not `go build` directly (the installer also wires up the Debezium plugin, ora2pg, and config templates):

```
yes | bash installer_scripts/install-yb-voyager -l -p   # PG-only, local source
yes | bash installer_scripts/install-yb-voyager -l      # full (incl. Oracle/MySQL via ora2pg)
yes | bash installer_scripts/install-yb-voyager -v      # rebuild Go and Java binaries (fast iteration)
```

The `-l` flag builds from local source and requires `rsync`. The installer's Java check caps at version 19 — always use JDK 17.

For pure Go work without touching Debezium/ora2pg, `go build ./...` from `yb-voyager/` works.

## Tests

The Go module uses build tags to separate test tiers. From `yb-voyager/`:

| Tag | Purpose |
| --- | --- |
| `unit` | Pure unit tests. Default for CI's `go.yml`. Run: `go test -tags unit ./...` |
| `issues_integration` | Schema-issue detection tests under `src/query/queryissue/`. |
| `integration` | testcontainers-driven source/target DB tests (`src/srcdb/`, `src/tgtdb/`, etc.). Needs Docker. |
| `integration_voyager_command` | Tests that shell out to a built `yb-voyager` binary. Build the binary first. |
| `integration_live_migration` | Live migration end-to-end with Debezium. |
| `failpoint_export` / `failpoint_import` / `failpoint_cutover` | Failpoint-injected tests. Requires `failpoint-ctl enable` (from `github.com/pingcap/failpoint`) to rewrite source before building. |
| `yb_version_latest_stable` | Version-gated issue tests run against the latest stable YB. |
| `cdc_benchmark` | CDC ingest benchmarks (`test/cdcbench/`, shim in `cmd/`): replay real export-data queue segments through the import streaming path with `ExecuteBatch` mocked. Artifact generation needs Docker + installed `yb-voyager`/Debezium; cached artifacts replay with neither. Run: `go test -tags cdc_benchmark -run '^$' -bench CDCIngest -benchtime 1x -count 5 ./cmd/`. See `test/cdcbench/README.md`. |
| `manual` | Local-only experiments, not run in CI. |

Single test: `go test -tags unit -run TestName ./yb-voyager/cmd/...`

Lint (from `yb-voyager/`): `go vet ./...` and `staticcheck -tags unit ./...`. `staticcheck.conf` disables `S1008`.

End-to-end migtests are invoked outside Go: `bash migtests/scripts/run-test.sh <test-name> [env.sh]`. They build/use the installed `yb-voyager` binary and a real source DB + YugabyteDB target.

## Migration workflow reference

- **Offline migration steps:** <https://docs.yugabyte.com/stable/yugabyte-voyager/migrate/migrate-steps/>
- **Live migration steps:** <https://docs.yugabyte.com/stable/yugabyte-voyager/migrate/live-migrate/>

## CLI architecture

`cmd/root.go` is the cobra root. Each top-level command (`export schema`, `export data`, `import schema`, `import data`, `assess-migration`, `analyze-schema`, `initiate cutover ...`, `archive changes`, `end migration`, `finalize-schema-post-data-import`, `compare-performance`, etc.) lives in its own file under `cmd/`.

## Source / target packages

- `src/srcdb/` — source DB drivers (`postgres.go`, `oracle.go`, `mysql.go`, `yugabytedb.go`). Each implements the `SourceDB` interface in `srcdb.go`. `pg_dump.go` and `ora2pg.go` wrap the external dump tools.
- `src/tgtdb/` — target DB drivers (`yugabytedb.go`, `postgres.go`, `oracle.go`) implementing `TargetDB` from `target_db_interface.go`. `conn_pool.go` is the import connection pool; `event.go` is the live-migration event model.
- `src/dbzm/` — Debezium server lifecycle, status, and value conversion for live migration.
- `src/metadb/` — SQLite-backed metadata store. `MigrationStatusRecord` in `migrationStatus.go` is the central serialized state — adding/removing JSON-tagged fields is a backward-compatibility concern (see below).
- `src/namereg/` + `src/utils/sqlname/` — the only correct way to handle DB identifiers. `NameTuple`/`ObjectName` preserve case-sensitive (quoted) PG identifiers. Never concatenate schema/table names manually.
- `src/metrics/` — Prometheus metrics surface. `Recorder` interface with a no-op default (`metrics.Get()`, safe when metrics are disabled) and a `PrometheusRecorder` implementation on its own registry (`metrics.NewPrometheusRecorder`). `metrics.NewServer` serves `/metrics` on a dedicated `http.ServeMux`, started via `--metrics-port` on import and export commands (`cmd/importData.go`'s `startMetricsServer`). Decoupled from `--profile` (pprof only); `--prometheus-metrics-port` is a deprecated hidden alias.
- `src/query/queryissue/`, `src/query/queryparser/`, `src/query/sqltransformer/` — SQL parsing and schema-issue detection used by `analyze-schema` and `assess-migration`.
- `src/migassessment/` — `assess-migration` engine: collects DB stats, sizing, replicas, permissions, produces the assessment DB and report. `cmd/templates/` holds the HTML/text report templates.
- `src/callhome/` — anonymous telemetry payloads. Disabled with `YB_VOYAGER_SEND_DIAGNOSTICS=0` or `--send-diagnostics=false`.
- `src/cp/` — control-plane abstractions (`noopcp`, `yugabyted`, `ybaeon`).
- `src/anon/`, `src/errs/`, `src/errorpolicy/`, `src/adaptiveparallelism/`, `src/importdata/`, `src/lockfile/`, `src/datafile/`, `src/datastore/`, `src/reporter/`, `src/monitor/`, `src/version/`, `src/ybversion/` — focused helpers; names are descriptive.

Test infrastructure for Go integration tests lives in `yb-voyager/test/containers/` (testcontainers wrappers for PG/MySQL/Oracle/YB) and `yb-voyager/test/utils/` (failpoint helpers, command runners, schema helpers).

## Product invariants

When changing anything in export/import/cutover or schema handling, evaluate against **all** migration flows — they are not symmetric:

- **Offline:** export-schema → import-schema → export-data (snapshot via `pg_dump`/`ora2pg`) → import-data → post-data import-schema.
- **Live (snapshot + changes):** snapshot + Debezium streaming.
- **Live with fall-back:** after cutover-to-target, data flows back to the original source.
- **Live with fall-forward:** after cutover-to-target, data flows to a standby replica.
- **Changes-only:** no snapshot. Skips `pg_dump`, so sequence values, table-list init, and start-clean semantics differ.
- **Iterative cutover (cutover-to-source with restart):** multiple iterations; each spawns a new metaDB and iteration export-dir. Flag propagation across iterations is fragile.

Source heterogeneity: PG is primary, Oracle/MySQL also supported as sources, YugabyteDB as source for fall-back/fall-forward. Use source-agnostic naming (`GetQueryStats`, not `GetPgStatStatements`). Unsupported-datatype lists, permission checks, and schema-extraction queries are per-source — changing one usually means changing all.

## Serialized-state backward compatibility

Users routinely upgrade voyager mid-migration. The following surfaces are load-bearing across versions:

- `MigrationStatusRecord` JSON in `metainfo/meta.db` — removing/renaming `json:"..."` tags can break older binaries reading the same export dir. Prefer additive changes.
- The assessment SQLite DB schema — when adding columns, use `ADD COLUMN IF NOT EXISTS` and write defensive queries; otherwise an older voyager run against the new schema will error.
- Callhome / YugabyteD payload structs — bump the payload version constant when shape changes.
- If a change cannot preserve compat, flag the next release as a breaking release.

## Metrics

`--metrics-port <port>` (default `0`, disabled) exposes a Prometheus registry at `GET http://<host>:<port>/metrics` on its own `http.ServeMux`, on both import (`import data`, `import data file`, `import data to target/source/source-replica`) and export (`export data from source/target`) commands. `--profile` starts the pprof server and is otherwise independent, but on import commands, if `--profile` is set and no `--metrics-port`/`--prometheus-metrics-port` is given, metrics still start on the role's legacy default port (9101 target/9102 import-file/9103 source-replica/9104 source, `cmd/importData.go`'s `legacyProfileDefaultMetricsPorts`) with a deprecation warning — this preserves pre-`--metrics-port` behavior. Export commands have no legacy default port and stay disabled unless a port is explicitly set. `--prometheus-metrics-port` is a deprecated hidden alias for `--metrics-port` (still works, logs a warning); `--metrics-port` wins if both are set.

Metric names follow the `yb_voyager_<import|export>_data_<snapshot|cdc>_*` scheme; direction is encoded in the name, so import metrics carry `importer_role` and export metrics carry `exporter_role` (no generic `role` label). `migration_uuid, session_id` are on every metric.

Metric catalogue:
- `yb_voyager_import_data_snapshot_rows_total`, `yb_voyager_import_data_snapshot_bytes_total` (counters) — rows/bytes imported during snapshot. Labels: `+ importer_role, table_name, schema_name`. Seeded from the persisted per-table imported row/byte counts on resume, so they never reset to 0 mid-migration.
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

Dropped (see PR review): `yb_voyager_import_snapshot_batches_in_flight` (derive via PromQL), `yb_voyager_cdc_import_rate_events_per_second` (use `rate()`), `yb_voyager_export_errors_total`, `yb_voyager_import_pool_pending_close_connections`, `yb_voyager_cdc_debezium_up` (deferred to a future PR exposing Debezium's own metrics).

## Gotchas

- Interactive installer prompts: pipe `yes` (ora2pg license, bashrc update).
- Installer Java check caps at version 19. Always use JDK 17.
- The `-l` (local build) flag requires `rsync` to be installed.
- Stale `<exportDir>/.*Lockfile.lck` files block re-runs after a killed process; delete them.
- `YB_VOYAGER_SEND_DIAGNOSTICS=0` disables telemetry during development.
- Failpoint tests need `failpoint-ctl enable` to rewrite sources before `go test`; remember `failpoint-ctl disable` afterwards to avoid committing rewritten files.

## Cursor Cloud environment

The following is specific to the Cursor Cloud (Ubuntu Linux) sandbox. Local macOS / dev-machine setups should ignore the paths and user names below.

### Pre-installed paths

- Go: `/usr/local/go/bin/go`
- JDK 17: `/usr/lib/jvm/java-17-openjdk-amd64`
- Maven: `/opt/yb-voyager/yb-debezium-maven-3.8.4` (auto-installed by the installer)
- YugabyteDB: `/opt/yugabyte-2025.2.1.0`

```
export JAVA_HOME=/usr/lib/jvm/java-17-openjdk-amd64
export PATH=/usr/local/go/bin:$JAVA_HOME/bin:$PATH
```

### Running local databases

**PostgreSQL** (source):
- Already running via systemd on port 5432. User `postgres`, password `postgres`.
- Start if stopped: `sudo pg_ctlcluster 17 main start`

**YugabyteDB** (target):
- Start: `/opt/yugabyte-2025.2.1.0/bin/yugabyted start --advertise_address 127.0.0.1`
- Status: `/opt/yugabyte-2025.2.1.0/bin/yugabyted status`
- Stop: `/opt/yugabyte-2025.2.1.0/bin/yugabyted stop`
- YSQL on port 5433, user `yugabyte`, password `yugabyte`.
- ysqlsh: `/opt/yugabyte-2025.2.1.0/bin/ysqlsh -U yugabyte -d <dbname> -h 127.0.0.1`
- The YugabyteDB data directory must be owned by `ubuntu` (not root). If you get FIPS/permission errors on start, run `sudo chown -R ubuntu:ubuntu /opt/yugabyte-2025.2.1.0`.
