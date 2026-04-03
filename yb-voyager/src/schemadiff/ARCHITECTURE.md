# Design & Architecture: Schema Drift Detection

**Author:** Shivansh
**Status:** Draft
**Date:** 2026-03-30

**Related documents:**
- [Functional Spec](FUNCTIONAL_SPEC.md) — command UX, terminal outputs, flags, report formats
- [Library Prototype](DESIGN.md) — original prototype notes (to be superseded by this document)

---

## 1 Component Overview

```
┌─────────────────────────────────────────────────────────┐
│                    yb-voyager CLI                        │
│                                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────┐  │
│  │ export schema│  │  export data │  │  import data  │  │
│  │              │  │              │  │               │  │
│  │  saves S1    │  │  gate: S2    │  │  gate: T2     │  │
│  │              │  │  hook: C1    │  │  hook: C2     │  │
│  │              │  │  exit check  │  │  exit check   │  │
│  └──────┬───────┘  └──────┬───────┘  └───────┬───────┘  │
│         │                 │                  │          │
│  ┌──────────────┐         │                  │          │
│  │import schema │         │                  │          │
│  │              │         │                  │          │
│  │  saves T1    │         │                  │          │
│  │              │         │                  │          │
│  └──────────────┘         │                  │          │
│                           │                  │          │
│  ┌────────────────────────┴──────────────────┴───────┐  │
│  │              yb-voyager schema diff                │  │
│  │                                                   │  │
│  │  Default: C3 + C1 (if snapshots) + C2 (if snaps) │  │
│  │  Explicit: --compare <left>,<right>               │  │
│  └─────────────────────┬─────────────────────────────┘  │
│                        │                                │
└────────────────────────┼────────────────────────────────┘
                         │
          ┌──────────────┴──────────────┐
          │      schemadiff library      │
          │                              │
          │  SnapshotProvider            │
          │  SchemaSnapshot              │
          │  Diff engine                 │
          │  IgnoreRules                 │
          │  ImpactClassifier            │
          │  Report renderer             │
          └──────────────────────────────┘
```

Three layers:

1. **schemadiff library** — standalone Go package. Takes snapshots, diffs them,
   applies ignore rules. No dependency on Voyager internals.
2. **Integration layer** — hooks in existing commands (`export schema`,
   `import schema`, `export data`, `import data`) that call the library.
3. **schema diff command** — new Cobra command that orchestrates all three
   comparisons and generates the report.

---

## 2 Technical Approach: Catalog Queries

The library snapshots schema by querying `pg_catalog` directly on a live
database connection.

### Why catalog queries instead of DDL parsing (`pg_dump` + `pg_query`)

| Consideration | DDL parsing | Catalog queries |
|---------------|-------------|-----------------|
| **Input requirement** | Requires running `pg_dump` and parsing its text output | Direct SQL query on any `database/sql` connection |
| **State consolidation** | `pg_dump` outputs `CREATE TABLE` + separate `ALTER TABLE ADD CONSTRAINT` + separate `CREATE INDEX`; these must be merged to reconstruct the final state | A single catalog query returns the current consolidated state directly |
| **Type representation** | `pg_dump` uses human-readable aliases (`integer`, `character varying`, `bigint`) which vary by PG version; these require normalization | `pg_catalog` stores canonical internal type names (`int4`, `varchar`, `int8`) consistently |
| **Cross-platform** | `pg_dump` must match the server's major version. Voyager installs PG 17; YB is PG 11-compatible. The installed `pg_dump` works for YB too. | Both PG and YB expose identical `pg_catalog` views; same queries work unchanged on both |
| **Runtime metadata** | `pg_dump` emits most DDL details but does not include YB-specific metadata like `yb_table_properties` | Full access to all catalog metadata including YB-specific system functions |
| **Invocation** | Must run as a subprocess, write to disk, then parse the output file | In-process; can be called at any point where a DB connection exists |
| **Multi-database extensibility** | `pg_dump` is PostgreSQL-specific; Oracle/MySQL would need entirely different parsers | Catalog query approach generalizes: Oracle has `ALL_TAB_COLUMNS` / `DBA_*` views, MySQL has `information_schema`. The snapshot interface can be implemented per-database behind a common `SchemaSnapshot` model. |

### Why not WAL-based detection?

PostgreSQL's `pgoutput` (used by Debezium) only captures DML. DDL is not in
the logical replication stream. The WAL contains physical page-level changes
to catalog tables — extracting DDL from those is impractical. Oracle/MySQL logs
do contain DDL, but leveraging that requires Debezium plugin changes — a future
effort. Catalog queries are the right approach: read-only, cross-database,
no source modifications.

---

## 3 Object Types to Compare

**Principle:** We compare only the object types that Voyager exports during
`export schema` and imports during `import schema`. If Voyager does not handle
an object type, there is no point diffing it.

Source: `src/utils/commonVariables.go` — `postgresSchemaObjectList`:

```
SCHEMA, COLLATION, EXTENSION, TYPE, DOMAIN, SEQUENCE, TABLE, INDEX,
FUNCTION, AGGREGATE, PROCEDURE, VIEW, TRIGGER, MVIEW, RULE, COMMENT,
CONVERSION, FOREIGN TABLE, POLICY, OPERATOR
```

**Note on metadata Voyager strips:** Voyager's `pg_dump` uses `--no-owner`,
`--no-privileges`, `--no-tablespaces`, and `--no-comments` (source:
`src/srcdb/data/pg_dump-args.ini`). Since these are intentionally not migrated,
**we will not compare** ownership, GRANT/REVOKE privileges, tablespace
assignments, or comments. This avoids false positives from intentionally
stripped metadata.

### 3.1 Tables

| Property | `pg_catalog` source |
|----------|---------------------|
| Existence (add/drop) | `pg_class` where `relkind IN ('r', 'p')` |
| Schema + name (qualified) | `pg_namespace.nspname`, `pg_class.relname` |
| Is partitioned | `relkind = 'p'` |
| Partition strategy | `pg_partitioned_table.partstrat` — range/list/hash |
| Partition key columns | `pg_partitioned_table.partattrs` + `pg_attribute` |
| Partition children | `pg_inherits` — detects new partitions (e.g. from `pg_partman`) |
| Parent table (for child partitions) | `pg_inherits.inhparent` |
| Replica identity | `pg_class.relreplident` — DEFAULT/NOTHING/FULL/INDEX |

### 3.2 Columns

| Property | `pg_catalog` source |
|----------|---------------------|
| Existence (add/drop) | `pg_attribute` where `attnum > 0 AND NOT attisdropped` |
| Name | `pg_attribute.attname` |
| Data type (canonical) | `pg_type.typname` |
| Type modifiers | `pg_attribute.atttypmod` — varchar(N), numeric(P,S) |
| Nullable | `pg_attribute.attnotnull` |
| Default expression | `pg_get_expr(adbin)` from `pg_attrdef` |
| Is identity | `pg_attribute.attidentity` — ALWAYS / BY DEFAULT |
| Ordinal position | `pg_attribute.attnum` |
| Collation | `pg_attribute.attcollation` |
| Generated expression | `pg_attribute.attgenerated` + `pg_get_expr` |

### 3.3 Constraints

| Property | `pg_catalog` source |
|----------|---------------------|
| Primary key: existence + columns + order | `pg_constraint` where `contype = 'p'` |
| Unique constraints: name, columns | `pg_constraint` where `contype = 'u'` |
| Unique: NULLS NOT DISTINCT | `pg_constraint.connoinherit` (PG15+) |
| FK: existence, columns, referenced table/cols | `pg_constraint` where `contype = 'f'`, `confrelid`, `confkey` |
| FK: ON DELETE / ON UPDATE action | `pg_constraint.confdeltype` / `confupdtype` |
| CHECK: existence + expression | `pg_constraint` where `contype = 'c'`, `pg_get_constraintdef(oid)` |
| Exclusion constraint existence | `pg_constraint` where `contype = 'x'` |
| Deferrability | `pg_constraint.condeferrable`, `condeferred` |

### 3.4 Indexes

| Property | `pg_catalog` source |
|----------|---------------------|
| Existence (add/drop) | `pg_index` + `pg_class` |
| Column list + order | `pg_index.indkey` + `pg_attribute` |
| Uniqueness | `pg_index.indisunique` |
| Access method | `pg_am.amname` — btree, hash, gin, gist, brin, spgist, lsm |
| Is partial (has WHERE clause) | `pg_index.indpred IS NOT NULL` |
| WHERE expression text | `pg_get_expr(indpred)` |
| INCLUDE columns | Columns beyond `indnkeyatts` in `indkey` |
| Expression columns | `pg_get_indexdef` for functional indexes |
| Sort direction per column | `pg_index.indoption` flags |
| Nulls first/last | `pg_index.indoption` flags |

### 3.5 Sequences

| Property | `pg_catalog` source |
|----------|---------------------|
| Existence (add/drop) | `pg_sequence` + `pg_class` |
| Data type | `pg_sequence.seqtypid` — int2/int4/int8 |
| Owned-by column | `pg_depend` where `deptype = 'a'` |
| Start value | `pg_sequences.start_value` |
| Increment | `pg_sequences.increment_by` |
| Min/max value | `pg_sequences.min_value`, `max_value` |
| Cycle | `pg_sequences.cycle` |
| Cache | `pg_sequences.cache_size` |

### 3.6 Views

Views (`relkind = 'v'`) are exported by Voyager as the VIEW object type.

| Property | `pg_catalog` source |
|----------|---------------------|
| Existence (add/drop) | `pg_class` where `relkind = 'v'` |
| Definition text | `pg_get_viewdef(oid)` |
| Column list + types | `pg_attribute` |
| Check option | `pg_views.definition` (LOCAL / CASCADED) |

### 3.7 Materialized Views

Materialized views (`relkind = 'm'`) are exported by Voyager as the MVIEW type.

| Property | `pg_catalog` source |
|----------|---------------------|
| Existence (add/drop) | `pg_class` where `relkind = 'm'` |
| Definition text | `pg_get_viewdef(oid)` |
| Column list + types | `pg_attribute` |
| Indexes on MV | `pg_index` (MVs can have indexes) |

### 3.8 Functions / Procedures

Exported by Voyager as FUNCTION, PROCEDURE, and AGGREGATE types.

| Property | `pg_catalog` source |
|----------|---------------------|
| Existence (add/drop) | `pg_proc` |
| Argument signature | `pg_get_function_arguments(oid)` |
| Return type | `pg_proc.prorettype` |
| Language | `pg_proc.prolang` → plpgsql/sql/c |
| Is procedure vs function | `pg_proc.prokind` = 'p' vs 'f' |
| Volatility | `pg_proc.provolatile` — VOLATILE/STABLE/IMMUTABLE |
| Security definer | `pg_proc.prosecdef` |
| Strict | `pg_proc.proisstrict` |
| Parallel safety | `pg_proc.proparallel` — SAFE/UNSAFE/RESTRICTED |

**Open question:** Should we compare function bodies (`prosrc`)? Body
comparison is noisy (whitespace, formatting). For drift detection, existence +
signature may be sufficient.

### 3.9 Triggers

| Property | `pg_catalog` source |
|----------|---------------------|
| Existence (add/drop) | `pg_trigger` |
| Table | `pg_trigger.tgrelid` |
| Timing (BEFORE/AFTER/INSTEAD OF) | `pg_trigger.tgtype` bitmask |
| Event (INSERT/UPDATE/DELETE/TRUNCATE) | `pg_trigger.tgtype` bitmask |
| For each row/statement | `pg_trigger.tgtype` bitmask |
| Function called | `pg_trigger.tgfoid` |
| Enabled/disabled | `pg_trigger.tgenabled` |
| Is constraint trigger | `pg_trigger.tgconstraint != 0` |

### 3.10 Types (Enum, Domain, Composite)

Exported by Voyager as TYPE and DOMAIN.

**Enum types** (`pg_type.typtype = 'e'`):

| Property | `pg_catalog` source |
|----------|---------------------|
| Existence (add/drop) | `pg_type` |
| Enum values (ordered) | `pg_enum` ordered by `enumsortorder` |

**Domains** (`pg_type.typtype = 'd'`):

| Property | `pg_catalog` source |
|----------|---------------------|
| Existence (add/drop) | `pg_type` |
| Base type | `pg_type.typbasetype` |
| Nullable | `pg_type.typnotnull` |
| Default | `pg_type.typdefault` |
| Check constraints | `pg_constraint` on the domain |

**Composite types** (`pg_type.typtype = 'c'`, excluding table row types):

| Property | `pg_catalog` source |
|----------|---------------------|
| Existence (add/drop) | `pg_type` |
| Attributes (name + type) | `pg_attribute` on `typrelid` |

### 3.11 Extensions

| Property | `pg_catalog` source |
|----------|---------------------|
| Existence (add/drop) | `pg_extension` |
| Version | `pg_extension.extversion` |

### 3.12 Collations

User-defined collations only (not system collations).

| Property | `pg_catalog` source |
|----------|---------------------|
| Existence (add/drop) | `pg_collation` |
| Provider | libc / icu / builtin |
| Locale | `collcollate`, `collctype` |
| Deterministic | `collisdeterministic` |

### 3.13 Policies (RLS)

| Property | `pg_catalog` source |
|----------|---------------------|
| Existence (add/drop) | `pg_policy` |
| Table | `pg_policy.polrelid` |
| Permissive/restrictive | `pg_policy.polpermissive` |
| Command (ALL/SELECT/INSERT/UPDATE/DELETE) | `pg_policy.polcmd` |
| USING expression | `pg_get_expr(polqual)` |
| WITH CHECK expression | `pg_get_expr(polwithcheck)` |

### 3.14 Other Voyager-exported types

These are in `postgresSchemaObjectList` but are **rarely modified during
live migration**. Including for completeness but deprioritized:

| Type | Comparison approach |
|------|---------------------|
| RULE | Existence + definition via `pg_get_ruledef` |
| CONVERSION | Existence only |
| FOREIGN TABLE | Existence + columns (not supported in live migration) |
| OPERATOR | Existence only |
| AGGREGATE | Covered under Functions (§3.8) |
| SCHEMA | Existence only |

### 3.15 Coverage relative to reference tools

- **Atlas** (open-source, Go) compares ~25 PG object types. It covers everything
  in our scope plus event triggers, casts, foreign servers, user mappings, roles,
  and permissions — none of which Voyager exports.
- **pgAdmin Schema Diff** compares ~10 types: tables, views, MVs, functions,
  procedures, sequences, indexes, triggers, constraints.
- **PostgresCompare** (commercial) compares 38 types, including less common ones
  like event triggers, foreign data wrappers, publications, subscriptions.

Our scope covers everything pgAdmin compares and aligns with Atlas, minus types
that Voyager does not export or intentionally strips.

---

## 4 Library Architecture

### 4.1 Package layout

```
schemadiff/
├── schema.go           # SchemaSnapshot model, SnapshotProvider interface, QueryExecutor
├── postgres.go         # PostgresSnapshotProvider (PG + YB catalog queries)
├── oracle.go           # OracleSnapshotProvider (stub)
├── mysql.go            # MySQLSnapshotProvider (stub)
├── diff.go             # Diff(a, b *SchemaSnapshot) → []DiffEntry
├── ignore.go           # IgnoreRule struct, predefined PG-vs-YB rules, pending-object rules
├── severity.go         # Impact level classification per diff type
├── report.go           # Report struct, renderers (html, json, txt)
├── schema_test.go      # Unit tests for snapshot model
├── diff_test.go        # Unit tests for diff engine
└── ignore_test.go      # Unit tests for ignore rules
```

### 4.2 Processing pipeline

```
SchemaSnapshot          # Tables, columns, constraints, indexes, etc.
    ↓ captured by
SnapshotProvider        # Interface: TakeSnapshot(db, schemas) → SchemaSnapshot
    ↓ serialized to
JSON baseline           # {export-dir}/metainfo/schema/{source,target}_baseline.json
    ↓ compared by
Diff()                  # Produces []DiffEntry
    ↓ filtered by
IgnoreRule              # Match func + Reason string → splits into (real, suppressed)
    ↓ classified by
ImpactClassifier        # DiffEntry type → LEVEL_1 / LEVEL_2 / LEVEL_3
    ↓ rendered by
ReportRenderer          # Formats report as HTML / JSON / TXT
```

### 4.3 Package diagram

```
┌───────────────────────────────────────────────────────────────┐
│                      schemadiff package                        │
│                                                                │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │               SnapshotProvider (interface)                │ │
│  │  TakeSnapshot(db, schemas) → SchemaSnapshot              │ │
│  │  DatabaseType() → string                                  │ │
│  ├──────────┬──────────────┬─────────────┬─────────────────┤ │
│  │ Postgres │ YugabyteDB   │ Oracle      │ MySQL           │ │
│  │ Provider │ (same as PG) │ Provider    │ Provider        │ │
│  │ pg_catalog│              │ ALL_TAB_*   │ info_schema     │ │
│  │ ✅ impl  │ ✅ impl      │ stub        │ stub            │ │
│  └──────────┴──────────────┴─────────────┴─────────────────┘ │
│                         │                                      │
│                         ▼                                      │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │             SchemaSnapshot (model)                        │ │
│  │  Database-agnostic: Tables, Columns, Constraints,         │ │
│  │  Indexes, Sequences, etc. (all types from §3)             │ │
│  └──────────────────────────────────────────────────────────┘ │
│            │                              │                    │
│            ▼                              ▼                    │
│  ┌────────────────┐            ┌──────────────────┐          │
│  │      Diff      │            │   FilterDiff     │          │
│  │  (a, b) →      │            │  (rules) →       │          │
│  │  []DiffEntry   │────────────│  real / ignored   │          │
│  └────────────────┘            └──────────────────┘          │
│                                         │                     │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │               Ignore Rules                                │ │
│  │  Voyager transformation rules (§5.1)                      │ │
│  │  Inherent PG-vs-YB rules (§5.2)                          │ │
│  │  YB limitation rules (§5.4) — version-aware               │ │
│  │  Pending post-data-import rules (§5.5)                    │ │
│  └──────────────────────────────────────────────────────────┘ │
│                                         │                     │
│  ┌────────────────┐            ┌──────────────────┐          │
│  │ ImpactClassifier│           │  ReportRenderer   │          │
│  │ DiffType →     │           │  HTML / JSON / TXT │          │
│  │ LEVEL_1/2/3    │           │                    │          │
│  └────────────────┘            └──────────────────┘          │
└───────────────────────────────────────────────────────────────┘
```

### 4.4 Core interfaces and types

```go
type QueryExecutor interface {
    Query(query string) (*sql.Rows, error)
    QueryRow(query string) *sql.Row
}

type SnapshotProvider interface {
    TakeSnapshot(db QueryExecutor, schemas []string) (*SchemaSnapshot, error)
    DatabaseType() string
}
```

`QueryExecutor` is a minimal interface satisfied by `*sql.DB`, Voyager's
`srcdb.SourceDB`, and `tgtdb.TargetYugabyteDB`. This lets us pass existing
Voyager connections directly to `TakeSnapshot` without opening separate
connections or exposing private `*sql.DB` fields.

| Provider | Database | Status | Catalog source |
|----------|----------|--------|----------------|
| `PostgresSnapshotProvider` | PostgreSQL, YugabyteDB | Implemented | `pg_catalog` views |
| `OracleSnapshotProvider` | Oracle | Stub | `ALL_TAB_COLUMNS`, `ALL_CONSTRAINTS`, `ALL_INDEXES`, etc. |
| `MySQLSnapshotProvider` | MySQL | Stub | `information_schema` views |

A factory function `NewSnapshotProvider(dbType)` returns the appropriate
provider, so callers need not import concrete types.

The `SchemaSnapshot` model, `Diff`, and `FilterDiff` functions are
database-agnostic — they operate on the snapshot struct, not on SQL queries.

### 4.5 Snapshot serialization

`SchemaSnapshot` is serializable to JSON for persistence. Snapshots are
accumulated in a timestamped directory:

```
{export-dir}/metainfo/schema/snapshots/
├── source_export_schema_<timestamp>.json
├── source_export_data_start_<timestamp>.json
├── source_export_data_start_<timestamp>.json   (resume)
├── target_import_schema_<timestamp>.json
├── target_import_data_start_<timestamp>.json
└── target_import_data_start_<timestamp>.json   (resume)
```

A helper function `LatestSnapshot(exportDir, side)` returns the most recently
saved snapshot for the given side ("source" or "target") by sorting files by
timestamp. This is used by both periodic drift checks and the default
`schema diff` command.

### 4.6 IgnoreRule struct

```go
type IgnoreRule struct {
    Name   string                      // Human-readable rule name
    Match  func(diff DiffEntry) bool   // Returns true if this diff should be suppressed
    Reason string                      // Static reason shown in the SUPPRESSED block
}
```

Rules are composable. Different comparisons use different rule sets:

| Comparison | Rules applied |
|-----------|---------------|
| C1 (latest source snapshot vs live source) | None — any difference is real drift |
| C2 (latest target snapshot vs live target) | None — any difference is unexpected |
| C3 (live source vs live target) | PG-vs-YB rules + pending-object rules |
| Explicit (snapshot vs snapshot, same DB type) | None |
| Explicit (PG snapshot vs YB snapshot/live) | PG-vs-YB rules + pending-object rules |

### 4.7 Impact classification

Static mapping from diff type to impact level:

| Diff type | Impact level | Reasoning |
|-----------|-------------|-----------|
| Table dropped | LEVEL_3 | All data for this table will fail to import |
| Column dropped | LEVEL_3 | CDC events reference a column that doesn't exist on target |
| Column type changed | LEVEL_3 | Type mismatch between CDC events and target schema |
| PK changed (columns/add/drop) | LEVEL_3 | Voyager relies on PK for CDC ordering and conflict detection |
| Table added on source | LEVEL_2 | New table not in Voyager's table list; data will be missed |
| Column added on source | LEVEL_2 | New column data won't be captured by existing CDC configuration |
| Partition added/dropped | LEVEL_2 | New partition's data won't be migrated / CDC events may fail |
| UK changed (columns/add/drop) | LEVEL_2 | May affect conflict detection on target |
| Trigger changed | LEVEL_2 | May affect data behavior on target during import |
| Enum values changed | LEVEL_2 | CDC events may contain values not in target's enum definition |
| Column nullable changed | LEVEL_2 | May cause constraint violations during import |
| Default value changed | LEVEL_1 | Defaults applied at write time; usually cosmetic for migration |
| FK changed | LEVEL_1 | FKs are deferred during import; rarely breaks migration |
| CHECK constraint changed | LEVEL_1 | May cause import failures if new check rejects existing data |
| Index added/dropped/changed | LEVEL_1 | Indexes don't affect data correctness |
| Sequence properties changed | LEVEL_1 | Sequence values are restored post-migration |
| View/MV definition changed | LEVEL_1 | Views are not part of data migration pipeline |
| Function/procedure changed | LEVEL_1 | Functions are imported from DDL files, not live-synced |

---

## 5 Ignore Rules: PG-vs-YB Expected Differences

When comparing a PostgreSQL source against a YugabyteDB target after Voyager has
migrated the schema, many differences are **expected**. The library must suppress
these to avoid false positives in C3.

### 5.1 Differences from Voyager's schema transformations

These are applied during `export schema`. Each creates a predictable difference
between the source PG catalog and the target YB catalog.

| # | Transformation | What changes on target vs source | Source file |
|---|----------------|----------------------------------|------------|
| 1 | **Redundant index removal** | Index exists on source but not on target. Voyager identified it as redundant (a stronger index covers the same leading columns) and removed it. Originals saved to `redundant_indexes.sql`. | `src/query/sqltransformer/transformer.go` — `RemoveRedundantIndexes` |
| 2 | **Secondary index ASC reordering** | Index sort direction differs: source has DEFAULT ordering on first column, target has explicit ASC. Voyager adds ASC to range-shard the index on YB. | `transformer.go` — `ModifySecondaryIndexesToRange` |
| 3 | **Colocation clause** | Table/MV on target has `colocation = false` in its storage options (`reloptions`). Voyager adds this for tables recommended as sharded (not colocated). Source does not have this option. | `cmd/exportSchema.go` — `applyShardedTablesRecommendation` |
| 4 | **Partial index: null filtering** | Index on target has a `WHERE <col> IS NOT NULL` clause that source does not have. Voyager added it for columns with high null frequency. | `transformer.go` — `AddPartialClauseForFilteringNULL` |
| 5 | **Partial index: frequent value filtering** | Index on target has a `WHERE <col> <> <value>` clause that source does not have. Voyager added it for columns with a dominant value. | `transformer.go` — `AddPartialClauseForFilteringValue` |

**IgnoreRule examples for §5.1:**

```go
IgnoreRule{
    Name:   "redundant-index-removed",
    Match:  func(d DiffEntry) bool { /* index exists on source, missing on target, listed in redundant_indexes.sql */ },
    Reason: "Removed by Voyager as redundant (covered by a stronger index). Original saved in redundant_indexes.sql.",
}
IgnoreRule{
    Name:   "secondary-index-asc-reorder",
    Match:  func(d DiffEntry) bool { /* index sort direction differs only in explicit ASC on first column */ },
    Reason: "Voyager adds explicit ASC for range-sharding on YugabyteDB.",
}
```

### 5.2 Differences from inherent PG-vs-YB behavior

These exist even when identical DDL is applied to both databases.

| # | Difference | What it looks like in a diff |
|---|-----------|------------------------------|
| 1 | **LSM vs btree** | YB stores all btree-equivalent indexes internally as LSM trees. `pg_am.amname` reports `lsm` on YB where PG reports `btree`. Every btree index will show this difference. |

```go
IgnoreRule{
    Name:   "btree-to-lsm",
    Match:  func(d DiffEntry) bool { /* index method changed from btree to lsm */ },
    Reason: "YugabyteDB uses LSM storage for all btree indexes.",
}
```

### 5.3 Schema properties not compared

Voyager's `pg_dump` uses flags that strip certain metadata (source:
`src/srcdb/data/pg_dump-args.ini`). Since these are intentionally not migrated,
we **exclude them from snapshots entirely** rather than suppressing them as diffs:

- Object ownership (`--no-owner`)
- GRANT/REVOKE privileges (`--no-privileges`)
- Tablespace assignments (`--no-tablespaces`)
- Comments on objects (`--no-comments`)

### 5.4 Known YB limitations (version-aware rules)

If the source PG schema uses features that YB does not support, the target will
be missing those objects. The diff should annotate these as expected due to YB
limitation, not as drift.

**YB version awareness:** Many limitations are fixed in newer YB versions. The
issue definitions in `issues_ddl.go` carry a `MinimumVersionsFixedIn` map. The
ignore rules must check the target YB version: if the version is new enough, the
limitation no longer applies and a missing object IS a real difference.

The library accepts the target YB version and consults `MinimumVersionsFixedIn`
to decide whether to suppress each limitation.

**Unsupported index access methods** (indexes on source, absent on target):

| Constant | Feature |
|----------|---------|
| `UNSUPPORTED_GIST_INDEX_METHOD` | GiST indexes |
| `UNSUPPORTED_BRIN_INDEX_METHOD` | BRIN indexes |
| `UNSUPPORTED_SPGIST_INDEX_METHOD` | SP-GiST indexes |
| `MULTI_COLUMN_GIN_INDEX` | Multi-column GIN indexes |
| `ORDERED_GIN_INDEX` | GIN indexes with ASC/DESC/HASH ordering |

**Permanently unsupported features** (no `MinimumVersionsFixedIn`):

| Constant | Feature |
|----------|---------|
| `EXCLUSION_CONSTRAINTS` | Exclusion constraints |
| `DEFERRABLE_CONSTRAINTS` | Deferrable constraints (non-FK) |
| `INHERITANCE` | Table inheritance (`INHERITS`) |
| `STORAGE_PARAMETERS` | Storage parameters on tables/indexes/constraints |
| `CONSTRAINT_TRIGGER` | Constraint triggers |
| `REFERENCING_CLAUSE_IN_TRIGGER` | `REFERENCING` clause (transition tables) |
| `EXPRESSION_PARTITION_WITH_PK_UK` | Expression-partitioned tables with PK/UK |
| `MULTI_COLUMN_LIST_PARTITION` | Multi-column `PARTITION BY LIST` |
| `INSUFFICIENT_COLUMNS_IN_PK_FOR_PARTITION` | Partition key columns not in PK |
| `NON_DETERMINISTIC_COLLATION` | Non-deterministic collations |

**Version-dependent features** (fixed in newer YB):

| Constant | Feature | Fixed in YB |
|----------|---------|-------------|
| `STORED_GENERATED_COLUMNS` | Stored generated columns | 2.25+ / 2025.1+ |
| `UNLOGGED_TABLES` | UNLOGGED tables | 2024.2+ |
| `SECURITY_INVOKER_VIEWS` | Security invoker views | 2.25+ / 2025.1+ |
| `DETERMINISTIC_OPTION_WITH_COLLATION` | Deterministic attribute in collation | 2.25+ / 2025.1+ |
| `FOREIGN_KEY_REFERENCES_PARTITIONED_TABLE` | FK referencing partitioned table | 2.25+ / 2025.1+ |
| `SQL_BODY_IN_FUNCTION` | SQL-body functions | 2.25+ / 2025.1+ |
| `UNIQUE_NULLS_NOT_DISTINCT` | `UNIQUE NULLS NOT DISTINCT` | 2.25+ / 2025.1+ |
| `BEFORE_ROW_TRIGGER_ON_PARTITIONED_TABLE` | BEFORE ROW triggers on partitioned tables | 2.25+ / 2025.1+ |
| `ALTER_TABLE_ADD_PK_ON_PARTITIONED_TABLE` | Adding PK via ALTER to partitioned table | 2024.1+ |
| `UNSUPPORTED_EXTENSION` | Extensions not supported in YB | Per-extension |
| Multirange types | 6 multirange types | 2.25+ / 2025.1+ |

```go
IgnoreRule{
    Name:   "yb-unsupported-gist",
    Match:  func(d DiffEntry) bool { /* GiST index on source, missing on target */ },
    Reason: "GiST indexes are not supported on YugabyteDB.",
}
IgnoreRule{
    Name:   "yb-stored-generated-columns",
    Match:  func(d DiffEntry) bool { /* stored generated column, AND target YB < 2.25 */ },
    Reason: "Stored generated columns not supported on YugabyteDB < 2.25.",
}
```

**Unsupported data types:**

Voyager's assessment report flags unsupported types. During live migration,
Voyager actively excludes columns with unsupported types from Debezium export.
These include: point, line, lseg, box, path, polygon, circle, geometry,
geography, box2d, box3d, topogeometry, raster, pg_lsn, txid_snapshot, xml, lo,
multirange types, vector, timetz.
(Source: `src/srcdb/postgres.go` — `PostgresUnsupportedDataTypesForDbzm`)

If a column with an unsupported type exists on the source but was not created on
the target, the diff annotates this as a known YB limitation.

**Policy/role limitations:**

| Constant | Feature |
|----------|---------|
| `POLICY_WITH_ROLES` | Policies referencing specific roles (roles not migrated) |

### 5.5 Objects pending `finalize-schema-post-data-import`

During live migration, Voyager defers creating certain objects on the target
until after data import completes:

- All secondary indexes
- NOT VALID constraints (validated later)
- Unique indexes

If `finalize-schema-post-data-import` has NOT been run, these objects exist on
the source but are intentionally missing on the target. C3 suppresses them.

Once `finalize-schema-post-data-import` has been run (tracked in MetaDB), this
suppression is disabled — any missing object is a real difference.

**How we know what's deferred:** The exported SQL files in
`{export-dir}/schema/` contain the full list of indexes and constraints that
Voyager will create during `finalize-schema-post-data-import`.

```go
IgnoreRule{
    Name:   "pending-post-data-import",
    Match:  func(d DiffEntry) bool { /* index/constraint missing on target AND listed in deferred SQL files AND finalize not yet run */ },
    Reason: "Deferred until 'finalize-schema-post-data-import'. Will be created when you run: yb-voyager finalize-schema-post-data-import --export-dir ...",
}
```

---

## 6 Data Flow: Snapshot Lifecycle

Snapshots are captured at multiple points in the migration workflow. Each is
saved with a descriptive timestamped name so the full history is preserved.

### 6.1 S1 — Source snapshot during `export schema`

```
export schema
    │
    ├── 1. Connect to source DB (source.DB())
    ├── 2. Export schema via pg_dump (existing logic)
    ├── 3. ── NEW ── Take source snapshot
    │       │
    │       ├── provider = schemadiff.NewSnapshotProvider("postgresql")
    │       ├── snapshot = provider.TakeSnapshot(db, schemas)
    │       └── serialize snapshot to JSON
    │
    ├── 4. ── NEW ── Save to {export-dir}/metainfo/schema/snapshots/source_export_schema_<ts>.json
    ├── 5. ── NEW ── Log: "Schema snapshot taken: source_export_schema_<ts>"
    ├── 6. Save source config to MSR (existing: saveSourceDBConfInMSR)
    └── 7. Set ExportSchemaDone in MSR (existing: setSchemaIsExported)
```

**Where in code:** `cmd/exportSchema.go`, after `printSchemaFilesPaths()` and
before `saveSourceDBConfInMSR()`.

### 6.2 T1 — Target snapshot during `import schema`

```
import schema
    │
    ├── 1. Connect to target DB (existing logic)
    ├── 2. Import schema DDL (existing logic)
    ├── 3. ── NEW ── Take target snapshot
    │       │
    │       ├── provider = schemadiff.NewSnapshotProvider("postgresql")
    │       │   (YugabyteDB uses same pg_catalog views)
    │       ├── snapshot = provider.TakeSnapshot(targetConn, schemas)
    │       └── serialize snapshot to JSON
    │
    ├── 4. ── NEW ── Save to {export-dir}/metainfo/schema/snapshots/target_import_schema_<ts>.json
    ├── 5. ── NEW ── Log: "Schema snapshot taken: target_import_schema_<ts>"
    └── 6. ── NEW ── Save target config to MSR (currently only in import data)
```

**Where in code:** `cmd/importSchema.go`, at the end of `importSchema()` after
all DDL is applied. Also requires adding `updateTargetConfInMigrationStatus()`
call which currently only exists in `cmd/importData.go`.

### 6.3 S2 — Source snapshot at `export data` start

```
export data (before streaming begins)
    │
    ├── 1. ── NEW ── Take source snapshot → source_export_data_start_<ts>.json
    ├── 2. ── NEW ── Log: "Schema snapshot taken: source_export_data_start_<ts>"
    ├── 3. ── NEW ── Compare against latest previous source snapshot (S1)
    │       │
    │       ├── diffs found → print summary, prompt "Continue with export data? [y/N]"
    │       │     ├── user says N → exit
    │       │     └── user says y → proceed
    │       └── no diffs → log "No source schema changes detected since last snapshot."
    │
    └── 4. Proceed to Debezium start (existing logic)
```

On resume (S3, S4, ...): same flow, but compares against the latest snapshot
(S2 on first resume, S3 on second, etc.). `start clean` follows the same
resume path.

### 6.4 T2 — Target snapshot at `import data` start

```
import data (before streaming begins)
    │
    ├── 1. ── NEW ── Take target snapshot → target_import_data_start_<ts>.json
    ├── 2. ── NEW ── Log: "Schema snapshot taken: target_import_data_start_<ts>"
    ├── 3. ── NEW ── Compare against latest previous target snapshot (T1)
    │       │
    │       ├── diffs found → print summary, prompt "Continue with import data? [y/N]"
    │       └── no diffs → log "No target schema changes detected since last snapshot."
    │
    └── 4. Proceed to import (existing logic)
```

On resume: same logic, comparing against latest available target snapshot.

---

## 7 Data Flow: Live Migration Hooks

### 7.1 Export data — periodic C1 (latest source snapshot vs live source)

```
export data (streaming phase)
    │
    ├── Pre-start gate completed (§6.3)
    ├── Start Debezium (existing)
    ├── ── NEW ── Launch drift detection goroutine
    │       │
    │       ├── reference = LatestSnapshot(exportDir, "source")
    │       ├── Every N minutes:
    │       │     │
    │       │     ├── provider.TakeSnapshot(sourceDB, schemas)
    │       │     ├── diffs = schemadiff.Diff(reference, liveSnapshot)
    │       │     │
    │       │     ├── if len(diffs) > 0:
    │       │     │     ├── Format one-line summary per diff
    │       │     │     └── Send warning to display channel
    │       │     │
    │       │     └── if len(diffs) == 0:
    │       │           └── (silent, no output)
    │       │
    │       └── Stop when: context cancelled (export data ends)
    │
    ├── Monitor Debezium progress (existing loop)
    │     │
    │     └── Display progress table + drift warnings (if any)
    │
    └── Exit handler (§7.3)
```

**Where in code:** `cmd/exportDataDebezium.go`. The goroutine is launched
after Debezium starts, in `checkAndHandleSnapshotComplete()` alongside the
existing progress goroutines:

```go
utils.PrintAndLogfInfo("streaming changes to a local queue file...")
if !disablePb || callhome.SendDiagnostics {
    go calculateStreamingProgress(ctx)
}
if !disablePb {
    go reportStreamingProgress(ctx)
}
// NEW: start drift detection goroutine
go monitorSourceSchemaDrift(ctx, source.DB(), schemas, exportDir)
```

**Connection:** Uses the existing source DB connection (`source.DB()` — global
`srcdb.SourceDB` in `cmd/export.go`). Stays open for the entire streaming
duration.

### 7.2 Import data — periodic C2 (latest target snapshot vs live target)

```
import data (streaming phase)
    │
    ├── Pre-start gate completed (§6.4)
    ├── Connect to target DB (existing)
    ├── ── NEW ── Launch drift detection goroutine
    │       │
    │       ├── reference = LatestSnapshot(exportDir, "target")
    │       ├── Every N minutes:
    │       │     │
    │       │     ├── provider.TakeSnapshot(targetDB, schemas)
    │       │     ├── diffs = schemadiff.Diff(reference, liveSnapshot)
    │       │     │
    │       │     ├── if len(diffs) > 0:
    │       │     │     └── Send warning to display channel
    │       │     │
    │       │     └── if len(diffs) == 0:
    │       │           └── (silent)
    │       │
    │       └── Stop when: context cancelled (import data ends)
    │
    ├── Stream events / import snapshot (existing loop)
    │     │
    │     └── Display progress table + drift warnings (if any)
    │
    └── Exit handler (§7.3)
```

**Where in code:** `cmd/live_migration.go` / `cmd/importData.go`, in
`streamChanges()` alongside the existing stats reporter goroutine:

```go
if !disablePb {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    go statsReporter.ReportStats(ctx)
    defer statsReporter.Finalize()
}
// NEW: start drift detection goroutine
go monitorTargetSchemaDrift(ctx, tdb, schemas, exportDir)
```

**Connection:** Uses the existing target DB connection (`tdb` — global
`tgtdb.TargetDB` in `cmd/import.go`). The `QueryExecutor` interface lets us
pass `tdb` without exposing the private `*sql.DB` field.

### 7.3 Exit drift check (best-effort)

When `export data` or `import data` exits — error, crash, or SIGTERM — a
deferred function runs a final quick drift check.

```
exit / crash / SIGTERM
    │
    ├── ── NEW ── Best-effort drift check (deferred)
    │       │
    │       ├── reference = LatestSnapshot(exportDir, side)
    │       ├── current = provider.TakeSnapshot(db, schemas)
    │       │     └── if error → silently skip, process is already exiting
    │       ├── diffs = schemadiff.Diff(reference, current)
    │       │
    │       ├── if diffs found → print summary to terminal
    │       └── if no diffs → print "Exit drift check: no schema changes detected."
    │
    └── Process terminates
```

This is a terminal-only signal. Nothing is persisted to disk. If the database
is unreachable, the check is silently skipped.

---

## 8 Goroutine Design for Live Migration Hooks

The drift detection goroutine runs independently of the main data pipeline.
It must not block event processing or affect throughput.

### 8.1 Concurrency model

```
                    Main goroutine                 Drift goroutine
                    ──────────────                 ───────────────
                         │                              │
                    Start export data              Launch with context
                         │                              │
                    ┌────┴────┐                    ┌────┴────┐
                    │ Process │                    │  Sleep  │
                    │ events  │                    │ N mins  │
                    │         │                    │         │
                    │         │                    ├─────────┤
                    │         │                    │  Take   │
                    │         │                    │snapshot │
                    │         │                    │  + diff │
                    │         │                    │         │
                    │         │◄──── warning ──────┤ if diffs│
                    │         │     (channel)      │  found  │
                    │ Display │                    │         │
                    │ warning │                    ├─────────┤
                    │ in UI   │                    │  Sleep  │
                    │         │                    │ N mins  │
                    └────┬────┘                    └────┬────┘
                         │                              │
                    Context cancel ──────────────► Goroutine exits
```

### 8.2 Pseudocode

```go
func monitorSourceSchemaDrift(ctx context.Context, db QueryExecutor,
    schemas []string, exportDir string) {
    reference := schemadiff.LatestSnapshot(exportDir, "source")
    if reference == nil {
        log.Infof("Source schema snapshots not found. Drift detection disabled.")
        return
    }
    provider := schemadiff.NewSnapshotProvider("postgresql")
    ticker := time.NewTicker(10 * time.Minute)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            current, err := provider.TakeSnapshot(db, schemas)
            if err != nil {
                log.Warnf("schema drift check failed: %v", err)
                continue
            }
            diffs := schemadiff.Diff(reference, current)
            if len(diffs.Changes) > 0 {
                warnings <- formatDriftWarning("source", diffs)
            }
        }
    }
}
```

### 8.3 Design properties

**Communication:** Goroutine sends warnings to the main goroutine via a
buffered channel. The main goroutine reads during its display update cycle and
renders warnings below the progress table.

**Error handling:** If the snapshot query fails (connection lost, timeout), the
goroutine logs the error and retries on the next interval. It never crashes
the main pipeline.

**Resource usage:** One snapshot is a handful of `pg_catalog` SELECT queries.
Negligible load on the database.

**Display integration:** Both export and import streaming use `uilive`
(`github.com/gosuri/uilive`) to render a refreshing metrics table. Import side
has `DisplayInformation(info string)` method that appends text to the table
as a red row. Export side needs a similar method added. When `--disable-pb` is
set, warnings go through `utils.PrintAndLog()` directly.

**Process coordination:** Export and import are separate OS processes. Each runs
drift detection independently using only its own DB connection. No cross-process
coordination is needed.

---

## 9 Data Flow: `schema diff` Command

The command has two modes based on whether `--compare` is provided.

### 9.1 Default mode (no `--compare` flag) — full diagnostic

```
yb-voyager schema diff --export-dir <dir> --config-file ... [flags]
    │
    ├── 1. Parse connection details from flags / config file
    │       │
    │       ├── Resolve source: --source-db-host, port, user, name, password, schema
    │       ├── Resolve target: --target-db-host, port, user, name, password
    │       └── Error if any required parameter is missing
    │
    ├── 2. Build connections
    │       │
    │       ├── sourceConn = connect(sourceConf)
    │       └── targetConn = connect(targetConf)
    │
    ├── 3. Take live snapshots (can be parallelized)
    │       │
    │       ├── sourceSnapshot = provider.TakeSnapshot(sourceConn, schemas)
    │       └── targetSnapshot = provider.TakeSnapshot(targetConn, schemas)
    │
    ├── 4. Run C3: live source vs live target
    │       │
    │       ├── rawDiffs = schemadiff.Diff(sourceSnapshot, targetSnapshot)
    │       ├── Apply PG-vs-YB ignore rules → split into (realDiffs, suppressed)
    │       ├── Apply pending-post-data-import rules → split into (realDiffs, pending)
    │       │     └── Check export-dir: has finalize-schema-post-data-import been run?
    │       │         If not, check export-dir SQL files for deferred objects.
    │       └── Classify realDiffs by impact level (LEVEL_1/2/3)
    │
    ├── 5. Run C1: latest source snapshot vs live source (if snapshots exist)
    │       │
    │       ├── reference = LatestSnapshot(exportDir, "source")
    │       ├── diffs = schemadiff.Diff(reference, sourceSnapshot)
    │       └── Classify by impact level (no ignore rules)
    │
    ├── 6. Run C2: latest target snapshot vs live target (if snapshots exist)
    │       │
    │       ├── reference = LatestSnapshot(exportDir, "target")
    │       ├── diffs = schemadiff.Diff(reference, targetSnapshot)
    │       └── Classify by impact level (no ignore rules)
    │
    ├── 7. Generate report
    │       │
    │       ├── Build report struct with all three sections
    │       ├── Render to requested formats (html, json, txt)
    │       └── Save to {export-dir}/reports/schema_diff_report.{format}
    │
    └── 8. Print terminal summary
```

### 9.2 Explicit mode (`--compare <left>,<right>`) — single comparison

```
yb-voyager schema diff --export-dir <dir> --compare <left>,<right> [flags]
    │
    ├── 1. Parse --compare value
    │       │
    │       ├── left = "live-source" | "live-target" | <snapshot-name>
    │       └── right = "live-source" | "live-target" | <snapshot-name>
    │
    ├── 2. Resolve each side
    │       │
    │       ├── "live-source" → connect to source DB, take snapshot
    │       │     └── requires source connection flags
    │       ├── "live-target" → connect to target DB, take snapshot
    │       │     └── requires target connection flags
    │       └── <snapshot-name> → load from {export-dir}/metainfo/schema/snapshots/
    │             └── no connection needed
    │
    ├── 3. Run single comparison
    │       │
    │       ├── diffs = schemadiff.Diff(leftSnapshot, rightSnapshot)
    │       ├── Determine if ignore rules apply:
    │       │     └── PG vs YB → yes; PG vs PG or YB vs YB → no
    │       └── Classify by impact level
    │
    ├── 4. Generate report (single section)
    │
    └── 5. Print terminal summary
```

### 9.3 `--list-snapshots` mode

```
yb-voyager schema diff --export-dir <dir> --list-snapshots
    │
    ├── 1. Scan {export-dir}/metainfo/schema/snapshots/
    ├── 2. List files sorted by timestamp
    └── 3. Print and exit
```

---

## 10 Integration Points

### 10.1 Files touched (existing)

| File | Change |
|------|--------|
| `cmd/exportSchema.go` | Add: take S1 snapshot after pg_dump completes |
| `cmd/importSchema.go` | Add: take T1 snapshot, save target config to MSR |
| `cmd/exportDataDebezium.go` | Add: pre-start gate (S2 snapshot + diff + prompt), launch C1 goroutine, exit drift check |
| `cmd/importData.go` / `cmd/live_migration.go` | Add: pre-start gate (T2 snapshot + diff + prompt), launch C2 goroutine, exit drift check |

### 10.2 Files created (new)

| File | Purpose |
|------|---------|
| `cmd/schemaDiffCommand.go` | New Cobra command: `yb-voyager schema diff` with `--compare`, `--list-snapshots`, connection flags |
| `src/schemadiff/diff.go` | Diff engine: `Diff(a, b *SchemaSnapshot) → DiffResult` |
| `src/schemadiff/snapshot.go` | `SaveSnapshot`, `LatestSnapshot`, `ListSnapshots` — snapshot persistence and retrieval |
| `src/schemadiff/ignore.go` | IgnoreRule struct, predefined rules, `FilterDiff()` |
| `src/schemadiff/severity.go` | Impact level classification per diff type |
| `src/schemadiff/report.go` | Report struct + renderers (HTML, JSON, TXT) |
| `cmd/templates/schema_diff_report.template` | HTML report template (follows existing `migration_assessment_report.template` pattern) |

### 10.3 MetaDB changes

| Field | Where | Purpose |
|-------|-------|---------|
| `TargetDBConf` in MSR | `cmd/importSchema.go` (new) | Save target connection details during import schema (currently only saved during import data). Small change: call `updateTargetConfInMigrationStatus()` at end of `importSchema()`. |

### 10.4 Export directory layout

```
{export-dir}/
├── metainfo/
│   └── schema/
│       └── snapshots/                                          ← NEW: accumulated snapshots
│           ├── source_export_schema_2026-03-25T10:00:00.json
│           ├── source_export_data_start_2026-03-26T14:00:00.json
│           ├── target_import_schema_2026-03-25T12:00:00.json
│           └── target_import_data_start_2026-03-26T15:00:00.json
├── reports/
│   ├── schema_diff_report.html     ← NEW: diff report
│   ├── schema_diff_report.json     ← NEW: diff report
│   └── schema_diff_report.txt      ← NEW: diff report
└── schema/
    └── redundant_indexes.sql       ← EXISTING: referenced by ignore rules
```

---

## 11 Comparison Matrix

| | C1 | C2 | C3 |
|---|---|---|---|
| **Left side** | Latest source snapshot | Latest target snapshot | Live source |
| **Right side** | Live source | Live target | Live target |
| **DB types** | PG vs PG | YB vs YB | PG vs YB |
| **Ignore rules** | None | None | PG-vs-YB + pending objects |
| **Needs snapshots** | Yes | Yes | No |
| **Used in pre-start gate** | export data start/resume | import data start/resume | — |
| **Used in periodic hooks** | export data | import data | — |
| **Used in exit check** | export data | import data | — |
| **Used in command (default)** | If snapshots exist | If snapshots exist | Always |
| **Detects** | Source drift | Target tampering | All current differences |

---

## 12 Sequence Diagram: Full Migration with Drift Detection

```
User          export schema    import schema    export data       import data       schema diff
 │                 │                │                │                 │                │
 │──run───────────►│                │                │                 │                │
 │                 │                │                │                 │                │
 │                 ├─pg_dump        │                │                 │                │
 │                 ├─take S1        │                │                 │                │
 │                 ├─save config    │                │                 │                │
 │◄────done────────┤                │                │                 │                │
 │                 │                │                │                 │                │
 │──run────────────────────────────►│                │                 │                │
 │                 │                │                │                 │                │
 │                 │                ├─import DDL     │                 │                │
 │                 │                ├─take T1        │                 │                │
 │                 │                ├─save config    │                 │                │
 │◄────done─────────────────────────┤                │                 │                │
 │                 │                │                │                 │                │
 │──run─────────────────────────────────────────────►│                 │                │
 │                 │                │                │                 │                │
 │                 │                │                ├─take S2         │                │
 │                 │                │                ├─diff S1 vs S2   │                │
 │◄────"drift detected, continue?" ─────────────────┤ (gate)          │                │
 │──y──────────────────────────────────────────────►│                 │                │
 │                 │                │                │                 │                │
 │                 │                │                ├─start Debezium  │                │
 │                 │                │                ├─launch C1       │                │
 │                 │                │                │ goroutine       │                │
 │                 │                │                │    │            │                │
 │                 │                │                │    ├─snapshot   │                │
 │                 │                │                │    ├─diff       │                │
 │◄─── ⚠ drift warning ────────────────────────────────  │(if diffs)  │                │
 │                 │                │                │    │            │                │
 │                 │                │                │    :            │                │
 │                 │                │                │  (export data   │                │
 │                 │                │                │   crashes)      │                │
 │                 │                │                ├─exit drift check│                │
 │◄─── "2 changes since last snapshot" ─────────────┤                 │                │
 │                 │                │                │                 │                │
 │──resume──────────────────────────────────────────►│                 │                │
 │                 │                │                │                 │                │
 │                 │                │                ├─take S3         │                │
 │                 │                │                ├─diff S2 vs S3   │                │
 │◄────"no changes, proceeding" ────────────────────┤                 │                │
 │                 │                │                ├─resume Debezium │                │
 │                 │                │                │                 │                │
 │──run──────────────────────────────────────────────────────────────►│                │
 │                 │                │                │                 │                │
 │                 │                │                │                 ├─take T2        │
 │                 │                │                │                 ├─diff T1 vs T2  │
 │◄────"no changes, proceeding" ─────────────────────────────────────┤                │
 │                 │                │                │                 │                │
 │                 │                │                │                 ├─start import   │
 │                 │                │                │                 ├─launch C2      │
 │                 │                │                │                 │ goroutine      │
 │                 │                │                │                 │    │           │
 │                 │                │                │                 │    ├─(silent)  │
 │                 │                │                │                 │               │
 │──run (on-demand)────────────────────────────────────────────────────────────────────►│
 │                 │                │                │                 │                │
 │                 │                │                │                 │                ├─connect
 │                 │                │                │                 │                │ source+target
 │                 │                │                │                 │                ├─snapshot both
 │                 │                │                │                 │                ├─C3: diff
 │                 │                │                │                 │                ├─C1: diff
 │                 │                │                │                 │                ├─C2: diff
 │                 │                │                │                 │                ├─generate
 │                 │                │                │                 │                │ report
 │◄────report──────────────────────────────────────────────────────────────────────────┤
 │                 │                │                │                 │                │
```

---

## 13 Known Limitations & Design Considerations

### 13.1 Snapshot consistency (transaction isolation)

`TakeSnapshot` issues multiple sequential queries (`loadTables`, `loadColumns`,
`loadConstraints`, etc.) against the database. If the schema is being modified
concurrently, the snapshot may be internally inconsistent — a table could appear
in `Tables` but have no columns.

**Mitigation options:**
- Wrap all catalog queries in a single `REPEATABLE READ` transaction for a
  consistent view across all queries.
- Accept the risk: schema DDL during a snapshot is rare, and this is a
  diagnostic tool, not a data-integrity mechanism.

### 13.2 DiffEntry detail structure

Currently `DiffEntry.Details` is a human-readable string (e.g.
`"type: int4 -> int8; nullable: true -> false"`). This works for display but
makes programmatic inspection difficult.

**Future:** Add structured fields (`OldValue`, `NewValue`, `Property`) alongside
the human-readable `Details`, so severity classification and automated actions
can inspect changes programmatically.

### 13.3 Ignore rule composability

Different comparisons need different rule sets:

| Scenario | Rules needed |
|----------|-------------|
| C1: Source snapshot vs live source | None — all differences are real |
| C2: Target snapshot vs live target | None — all differences are real |
| C3: Live source PG vs live target YB | Full set: transformations (§5.1) + inherent PG-vs-YB (§5.2) + YB limitations (§5.4) + pending objects (§5.5) |
| Explicit: same DB type | None — all differences are real |
| Explicit: PG vs YB | Full set (same as C3) |

The design supports composing rule sets based on comparison context via a
`[]IgnoreRule` slice passed to `FilterDiff()`. The `--compare` flag determines
the DB types of each side, which in turn determines the rule set.

### 13.4 Snapshot validity over time

Each snapshot represents the schema at a specific point in time. Over a
long-running migration (days/weeks), the earliest snapshot (S1 from
`export schema`) may become stale. This is by design — drift FROM the migration
start is exactly what we want to detect. The multiple-snapshot approach (S1,
S2, S3, ...) provides a timeline: S1→S2 shows pre-data-export changes,
S2→S3 shows changes during a failure/resume cycle, and the latest snapshot
serves as the reference for periodic drift checks.

### 13.5 Performance at scale

For databases with thousands of tables and tens of thousands of columns, the
catalog queries and in-memory snapshot may become slow. The current prototype
loads everything into memory. This is acceptable since Voyager already
handles large schemas.

If it becomes a bottleneck: schema-level parallelism or incremental snapshot
loading can be added.

### 13.6 Type normalization across databases

When comparing across database types (PG vs Oracle, PG vs MySQL), the same
logical type has different names (`int4` vs `NUMBER(10)` vs `INT`). The
`SchemaSnapshot` stores types as reported by each database's catalog.
Cross-database comparison will require a type normalization layer. Not needed
initially (PostgreSQL only).

---

## 14 Open Questions

1. **Function body comparison (§3.8):** Compare bodies or only existence +
   signature? Body comparison is noisy due to whitespace/formatting.

2. **Object scope:** Which of the types in §3 should be included vs deferred?
   The core set (tables, columns, constraints, indexes, sequences, views) covers
   the most common drift scenarios.

3. **CRITICAL diff behavior:** Purely informational, or should detecting LEVEL_3
   drift during live migration affect migration state (e.g. set an error flag)?

4. **Snapshot transaction isolation (§13.1):** Use `REPEATABLE READ` from the
   start, or accept the risk initially?

5. **Complete ignore rule validation:** The rules in §5.1 need testing against
   actual PG-vs-YB catalog output to confirm they are exhaustive.
