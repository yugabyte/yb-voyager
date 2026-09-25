# Known limitations and already-reported findings

Cases that only reproduce an entry here are `expect: known` — run them only when the change touches that area, and report a result only if the **behaviour changed** (e.g. a loud failure became silent, or a refusal disappeared). Update this file when a hunt files a new PR or an issue is fixed.

## Documented limitations (live migration docs)
- Schema changes on the source are not recognised during live migration (new columns, tables, indexes). A unique index added mid-stream → importer crashes with `23505`; restart re-reads indexes and recovers.
- Adding or deleting partitions during live migration is not supported. **Observed silent:** rows in a partition created mid-migration are never captured (publication lists leaves; `publish_via_partition_root=false`); no warning.
- Tables without a primary key are not supported (refused at export).
- `TRUNCATE` on the source is not replicated.
- Unsupported types for live migration: POINT, LINE, LSEG, BOX, PATH, POLYGON, CIRCLE, GEOMETRY, GEOGRAPHY, BOX2D, BOX3D, TOPOGEOMETRY, RASTER, PG_LSN, TXID_SNAPSHOT, LO, multiranges, VECTOR, XML, TIMETZ (varies by target version).
- Case-sensitive names are partially supported.
- Sequences not attached to a column, or on non-integer columns, are not resumed automatically.

## YugabyteDB DDL that cannot be created on the target (don't generate these as target schema)
- indexes / PKs on `interval`, `citext` (user-defined types)
- `DEFERRABLE` unique / PK constraints (voyager export refuses them up front)
- exclusion constraints

## Guardrails that refuse configurations (regression cases: expect `refused`)
- DEFERRABLE unique/PK constraint → export refuses.
- table with a unique index but REPLICA IDENTITY not FULL / missing permissions → export refuses.
- leaf partitions with different PK columns → export refuses ("all leaf partitions … share the same primary key columns").
- a leaf partition without a PK → export refuses.
- expression unique index (root or any leaf) or UK on a STORED generated column with `pk`/custom routing → import refuses before snapshot.
- changing `--cdc-partition-key` / overrides / import table list between runs → import refuses.

## Reported findings (don't open another PR for the same signature)
| Signature | Status | Reference |
|---|---|---|
| Root without own PK (leaf-only PKs) + `--use-partition-root true` → UPDATE/DELETE hit the same key in every leaf (M3) | filed | yb-voyager#3834 / DB-23802 |
| Leaf partition in a schema not in the migration's schema list → its rows silently missing (M4) | found 2026-09-24, not filed (by choice) | K10 in the first hunt |
| Unconstrained `numeric` loses trailing zeros in live migration (`4.000` → `4`) (M5) | found 2026-09-24, not filed (by choice) | test I in the first hunt |
| PK guard for a PK-less root copies a random leaf's PK (map iteration) (M1, latent behind the mixed-leaf-PK guardrail) | found 2026-09-24, not filed | fuzzer P10 |
| Cross-leaf false positives when leaf-local unique indexes are merged onto the root | known false positive (slow, not wrong) | #3820 review |
