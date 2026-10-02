# Dimensions to combine

Pick the slices relevant to the mechanisms selected for a plan, then combine pairwise. Values marked ★ have found or nearly found bugs before — include them whenever their area is in scope.

## Schema shapes

**Primary keys**
- single int / bigint; composite (2–3 cols, including ones whose string concatenation collides: `(1,23)` vs `(12,3)`)
- text / uuid / timestamptz / numeric ★ (spelling: `1.0` vs `1.00`) / float (`0` vs `-0`)
- case-sensitive PK column names
- PK changed by UPDATE (arrives as DELETE + INSERT)

**Unique indexes**
- single; composite; nullable (NULLS DISTINCT default); `NULLS NOT DISTINCT`
- partial (`WHERE flag`, soft-delete `WHERE deleted_at IS NULL`, statesman `(payment_id, most_recent) WHERE most_recent`) ★
- several UKs on one table (no single key covers all)
- `INCLUDE (...)` columns; expression UK (forces `table` routing); UK on STORED generated column (forces `table`)
- UK on a TOASTed large text value
- unique index created on the target only / after import start

**Partitioned tables** ★
- LIST / RANGE / HASH; DEFAULT partition; multi-level (LIST → RANGE → HASH)
- leaves in several schemas, including a case-sensitive schema; a leaf in a schema **not** in the schema list ★ (K10)
- PK on the root vs **only on leaves** ★ (#3834); leaves with *different* PKs (refused at export — keep a regression case)
- unique index on the root; on every leaf; on **only some** leaves; different UKs on different leaves
- row movement (partition-column UPDATE → DELETE + INSERT across leaves)
- same PK value present in several leaves (legal with leaf-only PKs)

**Other**
- same table name in two schemas; case-sensitive schema/table/column names; very long identifiers
- sequences: serial, bigserial, identity (ALWAYS / BY DEFAULT), `DEFAULT nextval`, `OWNED BY`, a sequence shared by two tables, non-integer columns
- generated columns; foreign keys (not enforced on target: `session_replication_role=replica`); triggers on source
- tables with many columns; wide rows; empty tables at snapshot

## Value types (M5)

Seed list for hand-written M5 cases. For a type sweep, use the families below.

unconstrained `numeric` ★, `numeric(p,s)`, `float4/8` (NaN, ±Infinity, -0), `money`, `timestamp` vs `timestamptz` (time zones, infinity), `date` (BC, infinity), `interval`, `time`/`timetz`, `bytea`, `json` vs `jsonb` (key order, whitespace, duplicate keys), arrays (NULL elements, multi-dim), `hstore` (NULL values), enums, domains, ranges, `uuid`, `inet/cidr`, `bit/varbit`, `char(n)` padding, text with NUL-adjacent/unicode/emoji/very long values, TOASTed values unchanged in an UPDATE.

## Type sweep (M5, only for families a PR touches)

Families, each with its edge values (the concrete types come from the source catalog at run time, including arrays and domains over them and extension types that are installed):

| Family | Types (examples) | Edge values |
|---|---|---|
| integer / float | int2/4/8, float4/8 | min/max, `NaN`, `±Infinity`, `-0` |
| numeric / money | numeric, numeric(p,s), money | unconstrained scale, trailing zeros, 100+ digits, `NaN`/`±Infinity` |
| date / time | date, time, timetz, timestamp, timestamptz | BC dates, `±infinity`, `24:00:00`, year > 294276, odd time zones |
| interval | interval | extreme fields, mixed signs |
| text / char | text, varchar(n), char(n), name, "char", citext | unicode, emoji, quotes, backslashes, control bytes, padding, very long |
| binary / bit | bytea, bit(n), varbit | empty, all-zero, > 64 bits |
| json | json, jsonb, jsonpath | key order, whitespace, duplicate keys, nested arrays |
| identifiers | uuid, inet, cidr, macaddr, macaddr8 | v4/v6, masks |
| enum / composite / domain | user-defined | domain typmod, nested composites, NULL fields |
| array | any `_type` | NULL elements, empty, multi-dim, special characters |
| range / multirange | int4range … tstzrange, multiranges, custom ranges | empty, unbounded, inclusive/exclusive bounds |
| geometric / text search | point, box, polygon, tsvector, tsquery | precision, weights |
| system / catalog | oid, xid, xid8, tid, cid, reg*, pg_lsn, pg_snapshot, refcursor, int2vector | any non-trivial value |
| extension types | hstore, ltree, … (whatever is installed) | NULL values, special characters |

Each type runs through **offline, live and fall-back** and these operations: snapshot row, CDC INSERT, CDC UPDATE setting only that column, CDC UPDATE of another column while a large (TOASTed) value stays unchanged, CDC DELETE. Every migration carries **control columns** (`int`, `text`); a type's result counts only if the controls passed in the same run. Start from the existing `src/testlivemigration/live_migration_datatype_edge_cases_test.go` (`getDatatypeEdgeCasesTestConfig`, forward and fall-back tests) and extend its shape to the swept families.

## Workloads
- insert-only; update-only (non-key columns only; UK columns; **subset of a composite UK**; partial-index predicate only); delete-only ★ (updates/deletes without inserts never trip ON CONFLICT errors — K4b)
- delete + re-insert same PK; same PK re-inserted under a different custom-key value ★
- free-then-reuse a unique value across different PKs (and across leaves)
- swaps and rotations of unique values through a temp value (A→tmp, B→A, A→B)
- statesman transitions (demote old current row, insert new current row) ★
- soft delete + re-create
- NULL transitions (NULL → value → NULL) on UK and custom-key columns
- row movement across partitions; move out and back quickly with the same id
- PK change followed by reuse of the old PK
- no-op updates (`SET x = x`); repeated updates of one row in one transaction
- large single transactions vs many tiny transactions; concurrent sessions interleaving
- bulk `COPY` into the source during streaming; `TRUNCATE`
- writes during export start (snapshot/CDC boundary, M7); writes during cutover
- writes on the target after cutover (fall-back/fall-forward flows)

## Flags and settings

Seed list only — the authoritative set is `inventory.flags` (Step 0.5). Flags there but not here are uncatalogued and get cases when the change touches them.

- `--cdc-partition-key auto | pk | table`
- `--cdc-partition-key-overrides`: custom single column, composite, **= partition column**, ≠ partition column, nullable column, case-sensitive column, a table-level override on some tables only
- `--use-partition-root true | false` ★ (with root PK vs leaf-only PK)
- `--table-list` / `--exclude-table-list`: root only, some leaves only, glob patterns, case-sensitive names
- `--source-db-schema` lists that omit a leaf's schema ★
- `--table-list-file-path` / `--exclude-table-list-file-path` (same semantics as the inline lists)
- `--start-clean` on re-run, with and without `--truncate-tables`; `--parallel-jobs`; `--on-primary-key-conflict` (`ERROR-POLICY` / `IGNORE`)
- `--batch-size`; `--adaptive-parallelism` / `--adaptive-parallelism-max` (snapshot import batching and concurrency)
- `--enable-upsert` (UPSERT on the target; unsafe with secondary indexes, per its help text); `--error-policy-snapshot` (`abort` / `stash-and-continue` — stashed rows are loud only if reported)
- `--transaction-ordering`
- `--target-endpoints` / `--use-public-ip` (which YB nodes receive the import)
- `--skip-replication-checks`, `--run-guardrails-checks false` (skip pre-flight checks; a silent loss behind a skipped check is still worth reporting)
- `--disable-sequential-scan-on-update-deletes` (hidden, default true — absent from `--help`, not stale); `--max-retries-streaming`
- export `--use-yb-grpc-connector` (YB-as-source: fall-back / fall-forward export from target)
- export `--disable-schema-snapshot-capture` / `--schema-snapshot-capture-interval` (schema snapshots for `schema detect-drift`)
- `--prepare-for-fall-back` / `--restart-data-migration-source-target` (cutover commands)
- export `--export-type snapshot-and-changes | changes-only`
- host locale for the voyager/Debezium processes (`LANG`/`LC_ALL=C` vs `C.UTF-8`) — the JVM's default charset follows it; run the default suite under UTF-8 and cover `C` explicitly with non-ASCII text
- env (internal/testing only): `NUM_EVENT_CHANNELS`, `MAX_EVENTS_PER_BATCH`, `MAX_INTERVAL_BETWEEN_BATCHES` — small channel counts raise collision rates
- flag changes between runs (guardrails should refuse; a silently accepted change is M10)

**Not data-path** (never reported as uncatalogued): connection and credentials (`--source-db-*`, `--source-replica-db-*`, `--target-db-*` except `--target-db-schema`, `--*-ssl-*`, `--oracle-*`), logging and UX (`--log-*`, `--disable-pb`, `--yes`, `--help`, `--send-diagnostics`, `--metrics-port`), paths and housekeeping (`--export-dir`, `--config-file`, `--backup-*`, `--archive-dir`, `--fs-utilization-threshold`, `--policy`, `--save-migration-reports`, `--exclude-file-list`), and Oracle-only export (`--allow-oracle-clob-data-export`).

## Run patterns
- straight through to cutover
- SIGKILL the importer (and separately the exporter) mid-stream, then resume; twice ★ (E, K7)
- graceful stop + resume
- resume with changed flags; resume with `--start-clean`
- crash on a loud error, then resume (does it recover or stay stuck? J2b)
- cutover during a write burst
- iterative cutover (cutover to source and back, 2+ iterations)
- archive changes enabled / segment cleanup
- mid-stream DDL (new partitions, new unique indexes, added columns) — a clear failure is acceptable; silent divergence is a finding
- failpoint-injected errors (retryable, retryable-after-commit, non-retryable) from `cmd/failpoints.go` / `src/tgtdb/failpoints.go`
- **crash at a save point** (M11): crash exactly between a saved claim and the work it covers, with the window held open (backlog ahead of the trigger, target row lock), then resume ★ (#3854)
- writes committed just before cutover is initiated (separate from cutover during a burst): are rows committed before `initiate cutover` always migrated?

## Flows

Entry points below are hints; `inventory.framework` lists what exists at the target commit.

| Flow | Framework entry points |
|---|---|
| live snapshot + changes | `StartExportData`, `StartImportData[WithEnv]`, `InitiateCutoverToTarget(false, …)` |
| live fall-back | `InitiateCutoverToTarget(true, …)`, `StartExportDataFromTarget`, `StartImportDataToSource`, `InitiateCutoverToSource` |
| live fall-forward | `SourceReplicaDB` container, `StartImportDataToSourceReplica`, `WaitForFallForwardEnabled` |
| changes-only | `StartExportDataChangesOnly` |
| iterative cutover | `InitiateCutoverToSource`, `WaitForNextIterationInitialized`, `WaitForCutoverSourceComplete` |
| offline | `testutils.VoyagerCommandRunner` driving `export data` / `import data` (see `cmd/*_test.go` with `integration_voyager_command`) |

## Oracles
- **Primary:** after the stream quiesces, full-row `testutils.CompareTableData(source, target, table, orderBy)` for every table, ordered by the full PK (include the partition column for leaf-local PKs).
- Row counts per partition (catches M3/M4 where totals happen to match).
- `lm.GetImportRunner().IsStopped()` — distinguishes silent from loud. It returns true only on the first call after the process exits, so read it once and keep the value (see `hunt-data-integrity-bugs/references/harness.md`).
- Import/export logs: `ERROR`/`FATAL` lines (loud), `unexpected rows affected` WARNs (M3 signal), `conflict detected` counts, `duplicate key value` lines.
- After cutover: sequence `last_value` / next generated id on the new primary vs source max.
- Raw queue events (`<export-dir>/data/queue/*.ndjson`) to attribute a mismatch to capture (M4/M5) vs apply (M1–M3).
- **Saved state after a crash** (M11): read the MSR and metaDB with `lm.WithMetaDB` right after the crash and before resuming; every saved claim must be true of the target at that moment (e.g. a segment marked imported has all its events on the target).
- **Resume completes**: the resumed run reaches the next phase within a timeout; hanging, waiting forever or crash-looping is STUCK.
- **Warned** (type sweeps): for each type and flow, whether assess-migration, analyze-schema or export warned (`inventory.type_warnings`). Silent loss with no warning is the worst cell; a warned type that works is a report-only note.
- **Silent** = mismatch while the process is running or exited 0 and no ERROR/FATAL was logged (WARN-only counts as silent).
