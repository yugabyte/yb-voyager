# src/ Shared Review Rules

## Package Directory

When adding new code, place it in the appropriate package:

- `srcdb` — Source DB interface and implementations (PostgreSQL, Oracle, MySQL, YugabyteDB-as-source). Any operation that needs to be run against the source DB goes via srcdb.
- `tgtdb` — Target DB interface and implementations (YugabyteDB, PostgreSQL). COPY/import logic, partition resolution, etc. Any operation that needs to be run against the target DB goes via tgtdb
- `metadb` — SQLite metadata DB for migration state (MSR), import file status, and queue segments, etc.
- `namereg` — Name registry mapping table/sequence names across source and target across exporter/importer roles.
- `query/queryissue` — Detects DDL/DML compatibility issues and generates fix recommendations for assessment/analyze reports.
- `query/queryparser` — SQL parsing via pg_query_go and parse-tree traversal helpers.
- `query/sqltransformer` — Transforms exported schema DDL (e.g., merging constraints into CREATE TABLE).
- `migassessment` — Migration assessment: metadata gathering, sizing recommendations, replica discovery, permissions.
- `anon` — SQL anonymizer for diagnostics. Hashes identifiers while preserving SQL structure.
- `callhome` — Sends anonymized diagnostics and migration metrics to Yugabyte.
- `dbzm` — Debezium CDC server lifecycle management, configuration, and YugabyteDB CDC client.
- `importdata` — Import data error handling, error policies, and Prometheus metrics.
- `datafile` — Abstraction for reading CSV, SQL, and text data files.
- `datastore` — Storage abstraction for local filesystem, S3, GCS, and Azure blob.
- `errs` — Structured error types for export and import flows.
- `issue` — Migration issue definitions (type, description, impact, minimum fix versions).
- `utils` — General helpers (paths, exec, regex, struct maps, logging). Sub-packages: `sqlname` (identifier quoting/case), `csv`, `jsonfile`, `s3`, `gcs`, `az`, `httpclient`, `schemareg`.
- `cp` — Control-plane interface for reporting migration events. Implementations: `yugabyted`, `ybaeon`, `noopcp`.
- `adaptiveparallelism` — Adjusts import data parallelism dynamically based on target cluster load.
- `monitor` — Monitors target YugabyteDB health (disk usage, replication lag, node status).
- `compareperf` — Compares source vs target query performance.
- `pgss` — Parsing and merging of `pg_stat_statements` data.
- `errorpolicy` — Policy for handling import errors (abort vs stash-and-continue).
- `lockfile` — File-based locking to prevent concurrent migration runs.
- `config` — Log level and global configuration.
- `constants` — Shared constants (DB types, object types, roles).
- `types` — Shared type definitions used across packages.
- `ybversion` — YugabyteDB version parsing and supported-series checks.
- `reporter/pb` — Progress bar reporters for export phases.
- `reporter/stats` — Streaming import stats and live reporting.
- `sqlldr` — Oracle SQL*Loader control file generation.

## Package Dependencies

- Avoid circular dependencies. If a lower-level package needs something from `cmd`, reconsider the design. Shared types used across packages should live in a leaf package rather than in `cmd`.

## Testing

- Use testcontainers for integration tests that need real database connections.
- Organize `TestMain` to start containers before tests and terminate them after all tests complete (via `m.Run()`). Do not rely on `defer` after `os.Exit`.
- Close database connections promptly after use rather than relying on `defer` at function scope if the connection is only needed for a small portion of the test.
