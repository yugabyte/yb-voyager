# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository layout

This repo ships two artifacts that together form YugabyteDB Voyager:

- `yb-voyager/` — the Go CLI module (`module github.com/yugabyte/yb-voyager/yb-voyager`, Go 1.24.2). `main.go` is a thin entrypoint; all command wiring and migration logic lives under `yb-voyager/cmd/` and `yb-voyager/src/`.
- `debezium-server-voyager/` — Java/Maven sources for the Debezium CDC plugin used by live migration. Built and installed separately by the installer.

Supporting trees:
- `installer_scripts/install-yb-voyager` — the canonical build/install entry. Handles Go build, Debezium Maven build, ora2pg setup, and dependency provisioning. Has interactive prompts (ora2pg license, bashrc) — pipe `yes` to accept non-interactively.
- `migtests/` — end-to-end migration tests driven by shell + Python (`run-test.sh`, `live-migration-*-run-test.sh`, scripts under `migtests/scripts/`). Tests live under `migtests/tests/<source>/<scenario>/`.
- `.cursor/BUGBOT.md` — product-level review rules. Treat these as load-bearing; see "Product invariants" below.
- `AGENTS.md` — Cursor Cloud–specific env requirements (Linux paths). Most of it does not apply to a local macOS checkout; treat as reference only.

## Build

The repo is normally built via the installer, not `go build` directly (the installer also wires up the Debezium plugin, ora2pg, and config templates):

```
yes | bash installer_scripts/install-yb-voyager -l -p   # PG-only, local source
yes | bash installer_scripts/install-yb-voyager -l      # full (incl. Oracle/MySQL via ora2pg)
yes | bash installer_scripts/install-yb-voyager -v      # rebuild Go and java binaries directly modifiable in voyager (preferred - fast iteration when making changes)
```

Required env: `JAVA_HOME` pointing at JDK 17–19 (21+ is rejected by the installer check), Go 1.24.2 on `PATH`. The installer requires `rsync` for `-l`.

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
| `manual` | Local-only experiments, not run in CI. |

Single test: `go test -tags unit -run TestName ./yb-voyager/cmd/...`.

Lint (from `yb-voyager/`): `go vet ./...` and `staticcheck -tags unit ./...`. `staticcheck.conf` disables `S1008`.

End-to-end migtests are invoked outside Go: `bash migtests/scripts/run-test.sh <test-name> [env.sh]`. They build/use the installed `yb-voyager` binary and a real source DB + YugabyteDB target.

## CLI architecture

`cmd/root.go` is the cobra root. Each top-level command (`export schema`, `export data`, `import schema`, `import data`, `assess-migration`, `analyze-schema`, `initiate cutover ...`, `archive changes`, `end migration`, `finalize-schema-post-data-import`, `compare-performance`, etc.) lives in its own file under `cmd/`.

Key cross-cutting concerns wired in `PersistentPreRun`:

- **Config resolution.** Flag precedence is CLI > env var > config file > default. `send-diagnostics` is special-cased because callhome reads it from a `BoolVar` pointer rather than `os.Getenv` at point of use.
- **Lockfile.** Most commands take `<exportDir>/.<commandID>Lockfile.lck`. Stale `.lck` files block re-runs and must be removed if a previous process was killed. `noLockNeededList` in `root.go` lists exceptions.
- **MetaDB init.** `<exportDir>/metainfo/meta.db` is a SQLite store. If it exists, the root preRun loads the `MigrationStatusRecord` (MSR) and checks voyager version compatibility.
- **Iteration resolution.** Live-migration commands (`isLiveMigrationIterationCommand`) redirect `exportDir` to the active iteration sub-directory. Fallback-phase commands may resolve to the *previous* iteration if the latest is still in forward phase — see `resolveToActiveIterationIfRequired` for the precise rule; changes there have subtle effects on cutover-to-source flows.
- **Control plane.** Pluggable: `noopcp` (default), `yugabyted` (writes to a local yugabyted control DB), `ybaeon` (YB Aeon cloud). Selected via `CONTROL_PLANE_TYPE` env var + `*-control-plane.*` config keys.

## Source / target packages

- `src/srcdb/` — source DB drivers (`postgres.go`, `oracle.go`, `mysql.go`, `yugabytedb.go`). Each implements the `SourceDB` interface in `srcdb.go`. `pg_dump.go` and `ora2pg.go` wrap the external dump tools.
- `src/tgtdb/` — target DB drivers (`yugabytedb.go`, `postgres.go`, `oracle.go`) implementing `TargetDB` from `target_db_interface.go`. `conn_pool.go` is the import connection pool; `event.go` is the live-migration event model.
- `src/dbzm/` — Debezium server lifecycle, status, and value conversion for live migration.
- `src/metadb/` — SQLite-backed metadata store. `MigrationStatusRecord` in `migrationStatus.go` is the central serialized state — adding/removing JSON-tagged fields is a backward-compatibility concern (see below).
- `src/namereg/` + `src/utils/sqlname/` — the only correct way to handle DB identifiers. `NameTuple`/`ObjectName` preserve case-sensitive (quoted) PG identifiers. Never concatenate schema/table names manually.
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

Partitioned tables: schema queries return both root and leaf partitions; caller must resolve leaf→root mapping. FK and index semantics on partitioned tables differ from regular tables. Always test with multi-level partition hierarchies.

Sequences vary across paths: pg_dump captures last-values for offline/live; changes-only must fetch them separately before streaming. Every association type (SERIAL, BIGSERIAL, explicit `nextval`, `GENERATED ALWAYS AS IDENTITY`, `ALTER SEQUENCE ... OWNED BY`) must be exercised when changing sequence logic.

Case-sensitive identifiers: route all object-name handling through `sqlname.NameTuple`/`ObjectName`. Test with quoted/case-sensitive names.

## Serialized-state backward compatibility

Users routinely upgrade voyager mid-migration. The following surfaces are load-bearing across versions:

- `MigrationStatusRecord` JSON in `metainfo/meta.db` — removing/renaming `json:"..."` tags can break older binaries reading the same export dir. Prefer additive changes.
- The assessment SQLite DB schema — when adding columns, use `ADD COLUMN IF NOT EXISTS` and write defensive queries; otherwise an older voyager run against the new schema will error.
- Callhome / YugabyteD payload structs — bump the payload version constant when shape changes.
- If a change cannot preserve compat, flag the next release as a breaking release.

## Gotchas

- Interactive installer prompts: pipe `yes` (ora2pg license, bashrc update).
- Installer Java check caps at 19. Use JDK 17.
- Stale `<exportDir>/.*Lockfile.lck` files block re-runs after a killed process; delete them.
- `YB_VOYAGER_SEND_DIAGNOSTICS=0` disables telemetry during development.
- Failpoint tests need `failpoint-ctl enable` to rewrite sources before `go test`; remember `failpoint-ctl disable` afterwards to avoid committing rewritten files.
