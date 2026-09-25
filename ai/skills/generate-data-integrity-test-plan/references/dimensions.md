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
- unique index created on the target only / after import start (known limitation — loud)

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
unconstrained `numeric` ★, `numeric(p,s)`, `float4/8` (NaN, ±Infinity, -0), `money`, `timestamp` vs `timestamptz` (time zones, infinity), `date` (BC, infinity), `interval`, `time`/`timetz`, `bytea`, `json` vs `jsonb` (key order, whitespace, duplicate keys), arrays (NULL elements, multi-dim), `hstore` (NULL values), enums, domains, ranges, `uuid`, `inet/cidr`, `bit/varbit`, `char(n)` padding, text with NUL-adjacent/unicode/emoji/very long values, TOASTed values unchanged in an UPDATE.

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
- bulk `COPY` into the source during streaming; `TRUNCATE` (known limitation)
- writes during export start (snapshot/CDC boundary, M7); writes during cutover
- writes on the target after cutover (fall-back/fall-forward flows)

## Flags and settings
- `--cdc-partition-key auto | pk | table`
- `--cdc-partition-key-overrides`: custom single column, composite, **= partition column**, ≠ partition column, nullable column, case-sensitive column, a table-level override on some tables only
- `--use-partition-root true | false` ★ (with root PK vs leaf-only PK)
- `--table-list` / `--exclude-table-list`: root only, some leaves only, glob patterns, case-sensitive names
- `--source-db-schema` lists that omit a leaf's schema ★
- `--start-clean` on re-run; `--parallel-jobs`; `--on-primary-key-conflict` (ERROR / IGNORE)
- `--disable-sequential-scan-on-update-deletes`; `--max-retries-streaming`
- export `--export-type snapshot-and-changes | changes-only`
- env (internal/testing only): `NUM_EVENT_CHANNELS`, `MAX_EVENTS_PER_BATCH`, `MAX_INTERVAL_BETWEEN_BATCHES` — small channel counts raise collision rates
- flag changes between runs (guardrails should refuse; a silently accepted change is M10)

## Run patterns
- straight through to cutover
- SIGKILL the importer (and separately the exporter) mid-stream, then resume; twice ★ (E, K7)
- graceful stop + resume
- resume with changed flags; resume with `--start-clean`
- crash on a loud error, then resume (does it recover or stay stuck? J2b)
- cutover during a write burst
- iterative cutover (cutover to source and back, 2+ iterations)
- archive changes enabled / segment cleanup
- mid-stream DDL (known limitations — expect loud or documented; silent is still a bug report candidate if undocumented)
- failpoint-injected errors (retryable, retryable-after-commit, non-retryable) from `cmd/failpoints.go` / `src/tgtdb/failpoints.go`

## Flows
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
- `lm.GetImportRunner().IsStopped()` and the process exit code — distinguishes silent from loud.
- Import/export logs: `ERROR`/`FATAL` lines (loud), `unexpected rows affected` WARNs (M3 signal), `conflict detected` counts, `duplicate key value` lines.
- After cutover: sequence `last_value` / next generated id on the new primary vs source max.
- Raw queue events (`<export-dir>/data/queue/*.ndjson`) to attribute a mismatch to capture (M4/M5) vs apply (M1–M3).
- **Silent** = mismatch while the process is running or exited 0 and no ERROR/FATAL was logged (WARN-only counts as silent).
