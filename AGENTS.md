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

## Engineering standards (per-directory `AGENTS.md`)

Standards live in an `AGENTS.md` file in the directory they govern, from this file down to individual packages. They are **authoring** standards first and review criteria second — read the ones covering the code you are about to change, before you change it.

Each such directory carries three entries for the same content: `AGENTS.md` (the real file), `CLAUDE.md` (symlink — Claude Code reads `CLAUDE.md`, not `AGENTS.md`), and `.cursor/BUGBOT.md` (symlink — Cursor Bugbot reads only this name). Edit `AGENTS.md`; never edit a symlink. When adding standards for a new subtree, create all three.

Current scopes: repo root (this file), `debezium-server-voyager/`, `migtests/`, `yb-voyager/`, `yb-voyager/cmd/`, `yb-voyager/src/`, and the `src/` packages `anon`, `callhome`, `errs`, `metadb`, `metrics`, `migassessment`, `query/queryissue`, `srcdb`, `testlivemigration`, `tgtdb`, `utils`.

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

Lint (from `yb-voyager/`), matching CI (`go.yml`): `go vet ./...`, `go vet -tags unit ./...`, `staticcheck ./...`, and `staticcheck -tags unit ./...` (pin the staticcheck version from `versions/ci-config.json`). `staticcheck.conf` disables `S1008` (explicit boolean returns) and `ST1005` (user-facing error strings are deliberately capitalized/punctuated) — when editing it, keep `"inherit"` as the first entry or every check is silently disabled. Also keep the tree `gofmt`-clean.

End-to-end migtests are invoked outside Go: `bash migtests/scripts/run-test.sh <test-name> [env.sh]`. They build/use the installed `yb-voyager` binary and a real source DB + YugabyteDB target.

## Migration workflow reference

- **Offline migration steps:** <https://docs.yugabyte.com/stable/yugabyte-voyager/migrate/migrate-steps/>
- **Live migration steps:** <https://docs.yugabyte.com/stable/yugabyte-voyager/migrate/live-migrate/>

## CLI architecture

`cmd/root.go` is the cobra root. Each top-level command (`export schema`, `export data`, `import schema`, `import data`, `assess-migration`, `analyze-schema`, `initiate cutover ...`, `archive changes`, `end migration`, `finalize-schema-post-data-import`, `compare-performance`, etc.) lives in its own file under `cmd/`.

## Source / target packages

`yb-voyager/src/AGENTS.md` holds the full package directory and the rule for where new code belongs. Only the entry points and cross-package invariants are repeated here:

- `src/srcdb/` — each driver implements the `SourceDB` interface in `srcdb.go`. `pg_dump.go` and `ora2pg.go` wrap the external dump tools.
- `src/tgtdb/` — each driver implements `TargetDB` from `target_db_interface.go`. `conn_pool.go` is the import connection pool; `event.go` is the live-migration event model.
- `src/metadb/` — `MigrationStatusRecord` in `migrationStatus.go` is the central serialized state; adding/removing JSON-tagged fields is a backward-compatibility concern (see below).
- `src/namereg/` + `src/utils/sqlname/` — the only correct way to handle DB identifiers. `NameTuple`/`ObjectName` preserve case-sensitive (quoted) PG identifiers. Never concatenate schema/table names manually.
- `src/metrics/` — Prometheus metrics surface. `Recorder` interface with a no-op default (`metrics.Get()`, safe when metrics are disabled) and a `PrometheusRecorder` on its own registry; `metrics.NewServer` serves `/metrics` on a dedicated `http.ServeMux`, started from `cmd/importData.go`'s `startMetricsServer`. See `src/metrics/AGENTS.md` for the flags, naming scheme, and catalogue.

Test infrastructure for Go integration tests lives in `yb-voyager/test/containers/` (testcontainers wrappers for PG/MySQL/Oracle/YB) and `yb-voyager/test/utils/` (failpoint helpers, command runners, schema helpers).

## Product invariants

When changing anything in export/import/cutover or schema handling, evaluate against **all** migration flows — they are not symmetric:

- **Offline:** export-schema → import-schema → export-data (snapshot via `pg_dump`/`ora2pg`) → import-data → post-data import-schema.
- **Live (snapshot + changes):** export-schema → import-schema → export-data (snapshot + streaming via Debezium) → import-data → cutover-to-target → end-migration.
- **Live with fall-back:** as above, then export-data-from-target → import-data-to-source. After cutover-to-target, data flows back to the original source so users can roll back.
- **Live with fall-forward:** as above, then export-data-from-target → import-data-to-source-replica. After cutover-to-target, data flows to a standby replica.
- **Changes-only:** no snapshot — streams CDC changes only. Skips `pg_dump`, so sequence handling, start-clean semantics, and table-list initialization all differ from the snapshot-and-changes path.
- **Iterative cutover (cutover-to-source with restart):** multiple cutover iterations between source and target. Each iteration creates a new metaDB and iteration export-dir, spawns new exporter/importer processes, and must propagate all flags correctly.

If a change touches export-data, import-data, or cutover logic, ask: does this work correctly in **each** of the above flows?

### Source database types

PG is primary, but Oracle and MySQL are also supported as sources, and YugabyteDB as source for fall-back/fall-forward.

- Use source-agnostic names for functions and variables. Prefer `GetQueryStats` over `GetPgStatStatements`.
- Unsupported-datatype lists, permission checks, and schema-extraction queries differ per source. When changing one source implementation, check whether the same change is needed in the others.
- Features gated by source type (e.g. `changes-only` only for PG/YB, CLOB export only for Oracle) must have explicit validation or guardrails.

### Partitioned tables

A frequent source of missed edge cases:

- Schema queries return both root and leaf partitions. The caller must decide which to use and resolve leaf→root mappings explicitly.
- Issues detected on partitioned tables must consider the full hierarchy: root → intermediate → leaf.
- Foreign keys and indexes on partitioned tables have different semantics than on regular tables.
- Always test with partitioned tables and multi-level partition hierarchies when changing table-handling logic.

### Sequences

Sequence handling varies significantly across code paths:

- Offline: `pg_dump` captures sequence last-values at snapshot time.
- Live (snapshot + changes): same as offline for initial values; Debezium streams ongoing changes.
- Changes-only: no `pg_dump`, so sequence values must be fetched separately before streaming begins.
- Sequence association types (SERIAL, BIGSERIAL, explicit `DEFAULT nextval(...)`, `GENERATED ALWAYS AS IDENTITY`, `ALTER SEQUENCE ... OWNED BY`) must all be handled consistently. Changes to sequence queries should be tested against all association types.

### Case-sensitive identifiers

PostgreSQL allows case-sensitive (quoted) identifiers. All object-name handling must go through the `sqlname` package (`NameTuple` or `ObjectName`), never manual string concatenation. Always test new table/column/schema handling with case-sensitive names.

## Serialized-state backward compatibility

Users routinely upgrade voyager mid-migration, so serialized state must remain compatible to the best of your ability:

- `MigrationStatusRecord` JSON in `metainfo/meta.db` — removing/renaming `json:"..."` tags, or removing fields, can break older binaries reading the same export dir. Prefer additive changes. The same applies to the assessment report and callhome payload structs.
- The assessment SQLite DB schema — when adding columns, use `ADD COLUMN IF NOT EXISTS` and write defensive queries; otherwise an older voyager run against the new schema will error.
- Callhome / YugabyteD payload structs — bump the payload version constant when the shape changes.
- If preserving backward compatibility is too complicated, say so explicitly: the next release then has to be marked a breaking release.

## Performance-critical (hot) paths

Some code runs once per migration; some runs once per row or per change event and dominates throughput. Treat the latter as hot paths and hold them to a higher standard:

- **Per-event / CDC processing during live migration** — event decoding, conflict detection (`conflictDetectionCache`), and everything on the import-data event loop run for every streamed change. This path has regressed before; a small per-event allocation multiplies by millions of events.
- **The per-row import-data loop and per-tuple value conversion** — batching, type conversion, and name lookups here run per row.

On a hot path:

- Avoid per-iteration heap allocations. Build strings with a single `strings.Builder`, writing each component directly (`b.WriteString(col); b.WriteString("=<missing>")`) — do **not** construct temporary strings with `+` and then `WriteString` them.
- Hoist invariant work out of the loop: pre-compile regexes, reuse buffers, compute lookups once.
- Avoid re-doing map/JSON/marshal work per event when it can be cached or done once.
- For non-trivial changes to a hot path, add or update a benchmark and report before/after numbers rather than eyeballing it.

If you are unsure whether a path is hot, ask; do not assume it is cold.

## Design and abstraction

Applies especially to new packages, interfaces, and abstractions:

- **Favor simplicity over abstraction.** Introduce layers, interfaces, factories, registries, or other indirection only when they provide a clear benefit or there is strong evidence that upcoming scale or feature growth will require them.
- **YAGNI.** Do not add fields, parameters, or extension seams that nothing consumes yet. Pull unused surface out until the consumer lands.
- **Respect layering.** Each layer does only its job — for example, populate domain data before entering the persistence layer.
- **Name to avoid collisions and ambiguity.** A field named `Role` next to Postgres roles, or two fields that mean the same thing, will confuse readers. Prefer qualified, single-purpose names.
- **One source of truth.** Do not persist the same fact two ways.

## Fail loudly on unexpected state

- When code reaches a state that "shouldn't happen", return an error — do not log a warning and continue, and do not silently skip the work.
- Do not silently fall back to a weaker mechanism (e.g. matching by name because an ID is missing, or using a default because a lookup failed). Either the fallback is a designed, documented behavior, or the missing input is an error.
- Avoid in-band sentinel values: an empty string, zero, or nil must not carry two different meanings (e.g. "not set" vs "legitimately empty"). Use an explicit flag or a distinct representation.
- For each new defensive branch, ask: when is this reachable, and is tolerating it correct? If the answer is "never", error out.

## Generic coding practices

- Keep code simple. Use early returns to reduce nesting. Prefer flat `if err != nil { return err }` over deeply nested success paths.
- Prefer letting the database do set-oriented work (ordering, aggregation, grouping, deduplication) in SQL rather than post-processing rows in Go, when it does not hurt readability.
- Use self-describing variable and function names. Avoid `rec1`/`rec2` — prefer `recCombined`/`recSharded`.
- Remove dead code, unused functions, and leftover debugging artifacts before merging.
- Consolidate duplicate logic into shared helpers rather than copy-pasting across switch cases or source-type implementations.
- Anything fed into a hash, fingerprint, or serialized key must be built in a deterministic order (sort map keys and column lists first).

## Comments

**Default: no comments.** Not "a few short ones" — none. The code, the function and variable names, the log messages, and the error strings already say *what* the code does; a comment that repeats them is noise, and it rots at the next edit while looking authoritative.

Everything below is the narrow exception. Before adding any comment or doc-comment line, write it, delete it, and add it back only if you can finish this sentence with something concrete:

> "Without this line, a reader would wrongly conclude ______."

If you cannot name the specific wrong conclusion, it is narration — leave it out. "It documents intent", "it adds useful context", "it helps explain" do not pass. When in doubt, it fails.

What passes is always a non-obvious **why**, never a **what**:

- An invariant or ordering the code cannot show — which process/role sets an MSR flag, that a value only takes effect after a restart, why a global is set and restored.
- Behaviour that differs across migration flows, where the code path looks flow-agnostic but isn't.
- A workaround for a specific product bug or an enforced PG/YB rule — name it, with the issue number.
- Why a branch exists at all, when the reason is outside the function — e.g. why detection skips partitioned tables.
- A non-obvious clause, join, or filter condition inside a multi-line SQL string.
- Anything a narrower `AGENTS.md` explicitly requires. Several packages mandate specific comments — MSR field semantics and which role sets them, callhome payload version history, the DDL pattern each `anon` handler processes. The inner scope wins; do not delete those in the name of this default.

Never write these — delete on sight, no judgement call:

- A line that paraphrases the code it sits on ("read from config", "group by table", "create user on target").
- A restatement of a function, field, or constant name, or of an error message.
- Change context — `// fix for #1234`, `// added after the failed run`, `// from triage`. That belongs in the commit message or PR body.

When a refactor renames or restructures something, sweep the surrounding comments and error messages for the old names. A stale comment is worse than no comment: it reads as authoritative while describing code that no longer exists.

A surviving comment is one or two lines. If it needs a paragraph, the explanation is wrong or the code is: shorten it, restructure the code, or move the prose to the PR description. This does not override doc comments on exported Go identifiers, which follow normal Go convention.

## Metrics

`--metrics-port <port>` (default `0`, disabled) exposes a Prometheus registry at `GET http://<host>:<port>/metrics` on the import and export data commands; `--profile` (pprof) is independent of it. Full flag semantics including the legacy `--profile` default ports, the metric naming scheme and label conventions, the metric catalogue, and the list of deliberately dropped metrics live in `yb-voyager/src/metrics/AGENTS.md`.

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
