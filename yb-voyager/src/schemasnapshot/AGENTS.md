# schemasnapshot Package — Engineering Standards

> **Scope:** files under `yb-voyager/src/schemasnapshot/`, the capture policy in `src/export/schema_snapshot_capture.go`, the capture call sites in `cmd/` (`exportSchema.go`, `exportData.go`, `export.go`), and the `schema_snapshots` table in `src/metadb/schemaSnapshot.go`. These are standards for **writing** code here, not only for reviewing it.
> Repo-wide standards live in `AGENTS.md` at each parent directory up to the repo root — read those as well. Design: `yb-voyager/design-specs/schema-detect-drift.md` (§5.9 for keys and indexes, §6 for flows). Consumers: `src/schemadiff/AGENTS.md`, `src/schema/schemadrift/AGENTS.md`.

## Layering

- `schemasnapshot` reads the catalog and stores/loads snapshots. It holds no policy. When and whether to capture lives in `src/export.SchemaSnapshotCapture`; the exporter-role gate lives in `cmd` (`sourceCapture()`). Keep each layer to its job.
- It never compares snapshots and never filters them. Capture every table in the given schemas; never apply `--table-list` at capture time — filtering at capture loses tables added or renamed later. Table filtering is `schemadiff.Scope`'s job.
- PostgreSQL only. Engine-specific code stays in its provider file; cross-engine concerns (type normalization for Oracle/MySQL) are not this package's work.

## Capture must never fail a migration

- Capture is on by default and best effort. A failure is logged (`Warnf`) and the export carries on; never return it as an export error or reach `utils.ErrExit` from it. The default was turned on only after checking that no capture path can fail an export — keep it that way.
- Every attempt — start, periodic, exit, and the fallback placeholder — is bounded by the single `CaptureTimeout`. Do not add another timeout.
- The placeholder write after a failed capture uses a fresh context, never the capture's: a timed-out capture context made the placeholder fail too.
- Every failed capture writes a placeholder (periodic ones included) so the gap shows on the drift timeline.
- The capture function returns errors and callers decide. Do not add a `...BestEffort` wrapper. Callers pass the `ctx`; the capture does not own one.
- Periodic capture runs for the whole export, snapshot and streaming phases alike. Do not gate it on `MODE_SNAPSHOT`, and do not dedup unchanged ticks — a capture per tick is what tells the user which hour the drift happened in.
- The exit capture runs on every exit, error exits included (a failure caused by drift must record the drifted schema). One `defer` plus an atexit hook, guarded by a `CompareAndSwap` claim; stop the periodic ticker first.
- Known gap, documented in spec §6: iterative cutover does not forward the CLI capture flags to the next iteration's exporter. Do not "fix" it silently.

## Capture consistency

- All loader queries run inside `Capture`'s read-only `REPEATABLE READ` transaction, through the `QueryExecutor` they are given. Never take a `*sql.DB` in a loader.
- The provider clears `search_path` (`set_config('search_path','',true)`) before the first query so every rendered text (types, defaults, constraint and index definitions, `::regclass`) is schema-qualified. New queries run after it and never set it.
- Placeholders are `$N`, built once by `buildInPlaceholders` and passed to every loader. `?` fails on pgx and sqlmock does not catch it.
- Version-gate any catalog column newer than the supported floor (PG 9 offline, PG 10 live). Example: the `conparentid` filter runs only on PG 11+.
- `CaptureParams.Schemas` are raw unquoted names matched as data against `nspname`. Callers use `GetSchemaListUnquoted`; quoted names capture zero tables and read as "no drift".
- Providers return content and server version only; `Capture` stamps `Version`, `DatabaseType`, `DBMetadata`.

## Snapshot model

- `SchemaSnapshot{Header, Content}`: the header goes to metaDB columns, the content to the JSON blob. A fact lives in exactly one of them (`Side` is header-only). Do not mirror a field into both, and do not tag duplicates `json:"-"`.
- `IsPlaceholder` is decided when the header is built (`newHeader(..., isPlaceholder)`) and stored in its own column. Do not derive it from `snapshot_json IS NULL`. `Name()` is derived from label + second-precision time and never stored.
- Identity: `ObjectRef` for schema-level objects, `TableScopedObjectRef{Table, Name}` for objects under a table. Keep those names. Render with `ForDisplay`, key maps with `ForKey` (unquoted, dot-joined `sqlname.Key()`); its dot-collision limitation is accepted — do not swap in a quoted key.
- Every object carries a catalog `ID` that survives rename but not drop+recreate (`pg_class.oid`, `pg_constraint.oid`, `"{tableOID}:{attnum}"`). Columns nest in `Table.Columns`.
- `Constraint` and `Index` are separate structs holding only `ID`, `Definition`, `FromParent`. Do not merge them, and do not add fields nothing reads yet (`ColumnAttnums`, `Deferrable`, expression trees were all cut). Kind is the `Table` field holding the object; no kind field.
- `Definition` is display-only. Never compare it.
- Copies inherited from a parent are captured with `FromParent` set. Deliberate exclusions, each kept as a `-- comment` in the SQL: invalid indexes, indexes behind PK/unique/exclusion constraints, exclusion constraints (YB does not support them), per-partition clones of an FK to a partitioned table, PG 18 NOT NULL rows.
- Fail on catalog surprises instead of skipping or guessing: a constraint or index whose table is not in the snapshot, two primary keys, an unknown `contype`, an index definition without the expected prefix.

## Stored-state compatibility

Snapshots in `meta.db` are read by later binaries after a mid-migration upgrade.

- JSON changes are additive. Never rename or remove a tag, a label, or a reason string.
- When older snapshots cannot answer a new question, bump `currentSnapshotVersion` with a named constant (`VersionWithKeysAndIndexes`) and make `schemadiff` compare the new data only when both sides are at that version. Do not add a separate "captured" flag — it stores the same fact twice. Do not reject older versions.
- `DecodeSnapshot` wraps `ErrSnapshotVersionUnsupported` for version 0 and newer-than-binary. Callers treat it as an error, not a gap.
- The table is created lazily on first write, not in `initMetaDB`. Schemas are stored as JSON, not comma-joined. Two captures in the same second collide on `UNIQUE(name)`; tests sleep one second rather than the product changing.

## Labels and reasons

Add a label or reason to `labelReasons`, then add a case in `schemadrift`'s `phaseFor`, `timingFor`, `isBeforeExportData` and `deriveEvent`. `LabelSourceLive` is in-memory only; never persist it.

## Code style here

- Leaf errors use `goerrors.Errorf`; `fmt.Errorf` only with `%w`, on one line (CI greps for it).
- Use `constants.POSTGRESQL`, `SideSource`, `Label*`, `Reason*` — no string literals, tests included.
- Request structs over positional args plus a bare bool (`CaptureRequest`).

## Testing

- Unit (`unit` tag): pure helpers (`linkTableHierarchy`, `attach*`, `stripIndexDefPrefix`, `parseMajorVersion`) and persistence round trips. Integration (`integration` tag, testcontainers): `postgres_integration_test.go` for catalog SQL; `src/export` for the capture policy; `src/testschemadrift` for capture hooks through the built binary.
- A new captured object needs integration cases with case-sensitive names, a partitioned table (root and leaf), an inheritance child, and its `FromParent` copy.
- Each test must exercise a distinct path; do not add timing or tautological tests.
