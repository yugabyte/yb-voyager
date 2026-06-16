# YugabyteDB Voyager — Product-Level Review Rules

## Data Migration Flow Coverage

Every code change involving data migration should be evaluated against **all** data migration flows it could affect:

- **Offline migration**: export-schema → import-schema → export-data (snapshot via pg_dump/ora2pg) → import-data → import-schema (post-data).
- **Live migration**: export-schema → import-schema → export-data (snapshot + streaming via Debezium) → import-data → cutover-to-target → end-migration.
- **Live with fall-back**: export-schema → import-schema → export-data → import-data → export-data-from-target → import-data-to-source. After cutover-to-target, data flows back to the original source so users can roll back.
- **Live with fall-forward**: export-schema → import-schema → export-data → import-data → export-data-from-target → import-data-to-source-replica. After cutover-to-target, data flows to a standby replica.
- **Changes-only mode**: export-schema → import-schema → export-data (changes only — skips snapshot, streams CDC changes) → import-data. Sequence handling, start-clean semantics, and table-list initialization all differ from the snapshot-and-changes path.
- **Iterative cutover (cutover-to-source with restart)**: multiple cutover iterations between source and target. Each iteration creates a new metaDB, spawns new exporter/importer processes, and must propagate all flags correctly.

If a change touches export-data, import-data, or cutover logic, ask: does this work correctly in **each** of the above flows?

## Source Database Types

The primary focus is PostgreSQL, but YugabyteDB Voyager also supports Oracle and MySQL as sources, and YugabyteDB as source (for fall-back/fall-forward). When reviewing:

- Use source-agnostic names for functions and variables. Prefer `GetQueryStats` over `GetPgStatStatements`.
- Unsupported datatype lists, permission checks, and schema extraction queries differ per source. When changing one source implementation, check if the same change is needed in others.
- Features gated by source type (e.g., `changes-only` only for PG/YB, CLOB export only for Oracle) must have explicit validation or guardrails.

## Partitioned Tables

Partitioned tables are a frequent source of missed edge cases:

- Schema queries return both root and leaf partitions. The caller must decide which to use and resolve leaf→root mappings explicitly.
- Issues detected on partitioned tables must consider the full hierarchy: root → intermediate → leaf.
- Foreign keys and indexes on partitioned tables have different semantics than on regular tables.
- Always test with partitioned tables and multi-level partition hierarchies when changing table-handling logic.

## Sequences

Sequence handling varies significantly across code paths:

- Offline: pg_dump captures sequence last-values at snapshot time.
- Live (snapshot + changes): same as offline for initial values; Debezium streams ongoing changes.
- Changes-only: no pg_dump, so sequence values must be fetched separately before streaming begins.
- Sequence association types (SERIAL, BIGSERIAL, explicit `DEFAULT nextval(...)`, `GENERATED ALWAYS AS IDENTITY`, `ALTER SEQUENCE ... OWNED BY`) must all be handled consistently. Changes to sequence queries should be tested against all association types.

## Upgrade and Backward Compatibility

Users may upgrade voyager mid-migration. Serialized state must remain compatible, to the best of your ability.:

- Changing JSON struct tags or removing fields from serialized structs (MSR, assessment report, callhome payloads) can be a breaking change.
- Adding columns to the assessmentDB (SQLite) schema can break older voyager versions that query the new schema. Use defensive queries or `ADD COLUMN IF NOT EXISTS`.
- If it is too complicated to support backward-compatibility, then notify that the subsequent release will need to be marked as a breaking release.
- When changing callhome or YugabyteD payload structs, increment the payload version constant.

## Case-Sensitive Identifiers

PostgreSQL allows case-sensitive (quoted) identifiers. All object name handling must go through the `sqlname` package (`NameTuple` or `ObjectName`), not manual string concatenation. Always test new table/column/schema handling with case-sensitive names.

## Generic Coding Practices

- Keep code simple. Use early returns to reduce nesting. Prefer flat `if err != nil { return err }` over deeply nested success paths.
- Use self-describing variable and function names. Avoid `rec1`/`rec2` — prefer `recCombined`/`recSharded`.
- Remove dead code, unused functions, and leftover debugging artifacts before merging.
- Consolidate duplicate logic into shared helpers rather than copy-pasting across switch cases or source-type implementations.
- Comments should explain *why*, not *what*. Non-obvious decisions, workarounds, and known limitations deserve comments.
