# Schema Drift Detection — Functional Specification

**Author:** Shivansh
**Status:** Draft — pending team review
**Date:** 2026-03-30

---

## 1. Background & Motivation

During live migration, schema changes on the source database cause Voyager
failures that are difficult to diagnose from log files. Customers must manually
reconcile schemas to find the change before they can resume migration.

The suggestion: extract the source schema again, compare it to the target, and
surface the changes with actionable guidance.

---

## 2. Goals

1. Build a **reusable schema diff library** (`schemadiff` package) that can
   snapshot a PostgreSQL-compatible database and diff two snapshots.
2. Detect **source schema drift** — changes made on the source after migration
   started.
3. Compare **source vs target** — identify differences between the PostgreSQL
   source and YugabyteDB target, filtering out expected differences.
4. **Surface actionable information** — when differences are found, tell the user
   what changed, its severity, and what to do.
5. Make the library **pluggable** — usable from a standalone CLI command, within
   live migration hooks, or any future integration point.

## 3. Non-Goals (for V1)

- Automatically resolving schema drift (applying DDL to sync schemas).
- Generating synchronization DDL scripts.
- Supporting Oracle or MySQL as source databases (PostgreSQL only for V1).
- Comparing schema files (DDL text) — we compare live database catalogs.

---

## 4. Technical Approach: Catalog Queries

The library snapshots schema by querying `pg_catalog` directly on a live
database connection.

### Why catalog queries instead of DDL parsing (`pg_dump` + `pg_query`)

| Consideration | DDL parsing | Catalog queries |
|---------------|-------------|-----------------|
| **Input requirement** | Requires running `pg_dump` and parsing its text output | Direct SQL query on any `database/sql` connection |
| **State consolidation** | `pg_dump` outputs `CREATE TABLE` + separate `ALTER TABLE ADD CONSTRAINT` + separate `CREATE INDEX`; these must be merged to reconstruct the final state | A single catalog query returns the current consolidated state directly |
| **Type representation** | `pg_dump` uses human-readable aliases (`integer`, `character varying`, `bigint`) which vary by PG version; these require normalization | `pg_catalog` stores canonical internal type names (`int4`, `varchar`, `int8`) consistently |
| **Cross-platform** | Both PG and YB support `pg_dump`. However, `pg_dump` must match the server's major version (e.g. PG 17 `pg_dump` for PG 17 server). Since Voyager installs PG 17 and YB is PG 11-compatible, the installed `pg_dump` works for YB too. | Both PG and YB expose identical `pg_catalog` views; same queries work unchanged on both |
| **Runtime metadata** | `pg_dump` emits most DDL details (access methods, constraints, etc.) but does not include YB-specific metadata like `yb_table_properties` | Full access to all catalog metadata including YB-specific system functions |
| **Invocation** | Must run as a subprocess, write to disk, then parse the output file | In-process; can be called at any point where a DB connection exists |
| **Multi-database extensibility** | `pg_dump` is PostgreSQL-specific; Oracle/MySQL would need entirely different parsers | Catalog query approach generalizes: Oracle has `ALL_TAB_COLUMNS` / `DBA_*` views, MySQL has `information_schema`. The snapshot interface can be implemented per-database behind a common `SchemaSnapshot` model. |

---

## 5. Scope: Object Types to Compare

**Principle:** We compare only the object types that Voyager exports during
`export schema` and imports during `import schema`. If Voyager does not handle
an object type, there is no point diffing it.

Source: `src/utils/commonVariables.go` — `postgresSchemaObjectList`:

```
SCHEMA, COLLATION, EXTENSION, TYPE, DOMAIN, SEQUENCE, TABLE, INDEX,
FUNCTION, AGGREGATE, PROCEDURE, VIEW, TRIGGER, MVIEW, RULE, COMMENT,
CONVERSION, FOREIGN TABLE, POLICY, OPERATOR
```

The following sections cover each object type and the specific properties we
will compare. Properties are identified by their `pg_catalog` source.

**Note on metadata Voyager strips:** Voyager's `pg_dump` uses `--no-owner`,
`--no-privileges`, `--no-tablespaces`, and `--no-comments` (source:
`src/srcdb/data/pg_dump-args.ini`). Since these are intentionally not migrated,
**we will not compare** ownership, GRANT/REVOKE privileges, tablespace
assignments, or comments. This avoids false positives from intentionally
stripped metadata.

### 5.1 Tables

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

### 5.2 Columns

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

### 5.3 Constraints

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

### 5.4 Indexes

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

### 5.5 Sequences

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

### 5.6 Views

Views (`relkind = 'v'`) are exported by Voyager as the VIEW object type.

| Property | `pg_catalog` source |
|----------|---------------------|
| Existence (add/drop) | `pg_class` where `relkind = 'v'` |
| Definition text | `pg_get_viewdef(oid)` |
| Column list + types | `pg_attribute` |
| Check option | `pg_views.definition` (LOCAL / CASCADED) |

### 5.7 Materialized Views

Materialized views (`relkind = 'm'`) are exported by Voyager as the MVIEW type.

| Property | `pg_catalog` source |
|----------|---------------------|
| Existence (add/drop) | `pg_class` where `relkind = 'm'` |
| Definition text | `pg_get_viewdef(oid)` |
| Column list + types | `pg_attribute` |
| Indexes on MV | `pg_index` (MVs can have indexes) |

### 5.8 Functions / Procedures

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
signature may be sufficient. Other tools (pgAdmin, PostgresCompare) offer an
"ignore whitespace" toggle for this.

### 5.9 Triggers

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

### 5.10 Types (Enum, Domain, Composite)

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

### 5.11 Extensions

| Property | `pg_catalog` source |
|----------|---------------------|
| Existence (add/drop) | `pg_extension` |
| Version | `pg_extension.extversion` |

### 5.12 Collations

Collations define string sorting and comparison rules. Voyager exports these
as the COLLATION type. User-defined collations only (not system collations).

| Property | `pg_catalog` source |
|----------|---------------------|
| Existence (add/drop) | `pg_collation` |
| Provider | libc / icu / builtin |
| Locale | `collcollate`, `collctype` |
| Deterministic | `collisdeterministic` |

### 5.13 Policies (RLS)

Row-Level Security policies control which rows are visible/modifiable per
database role. Voyager exports these as the POLICY type.

| Property | `pg_catalog` source |
|----------|---------------------|
| Existence (add/drop) | `pg_policy` |
| Table | `pg_policy.polrelid` |
| Permissive/restrictive | `pg_policy.polpermissive` |
| Command (ALL/SELECT/INSERT/UPDATE/DELETE) | `pg_policy.polcmd` |
| USING expression | `pg_get_expr(polqual)` |
| WITH CHECK expression | `pg_get_expr(polwithcheck)` |

### 5.14 Other Voyager-exported types

These types are in `postgresSchemaObjectList` but are **rarely modified during
live migration**. Including them for completeness but they can be deprioritized:

| Type | What it is | Comparison approach |
|------|-----------|---------------------|
| RULE | Query rewrite rules (deprecated feature) | Existence + definition via `pg_get_ruledef` |
| CONVERSION | Encoding conversion functions | Existence only |
| FOREIGN TABLE | Tables backed by foreign data wrappers | Existence + columns (not supported in live migration) |
| OPERATOR | User-defined operators | Existence only |
| AGGREGATE | Aggregate functions | Covered under Functions (§5.8) |
| SCHEMA | Namespace existence | Existence only |

### 5.15 Coverage relative to reference tools

For reference, here is how our scope compares to other tools:

- **Atlas** (open-source, Go) compares ~25 PG object types. It covers everything
  in our scope plus event triggers, casts, foreign servers, user mappings, roles,
  and permissions — none of which Voyager exports (event triggers, casts, foreign
  servers, user mappings are not in `pg_dump` parser; roles/permissions are
  stripped by `--no-owner`/`--no-privileges`). Range types are covered by our
  TYPE bucket. Source: `atlasgo.io/hcl/postgres`.
- **pgAdmin Schema Diff** compares ~10 types: tables, views, MVs, functions,
  procedures, sequences, indexes, triggers, constraints. Both sides must be the
  same PG major version. Source: `pgadmin.org/docs/pgadmin4/latest/schema_diff.html`.
- **PostgresCompare** (commercial) compares 38 types, including less common ones
  like event triggers, foreign data wrappers, publications, subscriptions,
  operator families, text search configurations. It also provides 10 ignore
  toggles and deployment safety classification. Source:
  `postgrescompare.com/docs/guides/comparing-databases/`.

Our scope covers everything pgAdmin compares and aligns with what Atlas compares,
minus types that Voyager does not export or intentionally strips: event triggers,
casts, foreign servers, user mappings (not in Voyager's `pg_dump` parser), and
roles/permissions (stripped by `--no-owner`/`--no-privileges`). Range types are
covered as part of our TYPE bucket.

---

## 6. Expected PG-vs-YB Differences (Ignore Rules)

When comparing a PostgreSQL source against a YugabyteDB target after Voyager has
migrated the schema, many differences are **expected**. The library must suppress
these to avoid false positives.

### 6.1 Differences from Voyager's Schema Transformations

These transformations are applied during `export schema`. Each creates a
predictable difference between the source PG catalog and the target YB catalog.

| # | Transformation | What changes on target vs source | Source file |
|---|----------------|----------------------------------|------------|
| 1 | **Redundant index removal** | Index exists on source but not on target. Voyager identified it as redundant (a stronger index covers the same leading columns) and removed it. Originals saved to `redundant_indexes.sql`. | `src/query/sqltransformer/transformer.go` — `RemoveRedundantIndexes` |
| 2 | **Secondary index ASC reordering** | Index sort direction differs: source has DEFAULT ordering on first column, target has explicit ASC. Voyager adds ASC to range-shard the index on YB. | `transformer.go` — `ModifySecondaryIndexesToRange` |
| 3 | **Colocation clause** | Table/MV on target has `colocation = false` in its storage options (`reloptions`). Voyager adds this for tables recommended as sharded (not colocated). Source does not have this option. | `cmd/exportSchema.go` — `applyShardedTablesRecommendation` |
| 4 | **Partial index: null filtering** | Index on target has a `WHERE <col> IS NOT NULL` clause that source does not have. Voyager added it for columns with high null frequency. | `transformer.go` — `AddPartialClauseForFilteringNULL` |
| 5 | **Partial index: frequent value filtering** | Index on target has a `WHERE <col> <> <value>` clause that source does not have. Voyager added it for columns with a dominant value. | `transformer.go` — `AddPartialClauseForFilteringValue` |

### 6.2 Differences from Inherent PG-vs-YB Behavior

These exist even when identical DDL is applied to both databases.

| # | Difference | What it looks like in a diff |
|---|-----------|------------------------------|
| 1 | **LSM vs btree** | YB stores all btree-equivalent indexes internally as LSM trees. The `pg_am.amname` column reports `lsm` on YB where PG reports `btree`. Every btree index will show this difference. |

### 6.3 Schema properties not migrated by Voyager

Voyager's `pg_dump` uses flags that strip certain metadata (source:
`src/srcdb/data/pg_dump-args.ini`). Since these are intentionally not migrated,
we will **not include these properties in snapshots**:

- Object ownership (`--no-owner`)
- GRANT/REVOKE privileges (`--no-privileges`)
- Tablespace assignments (`--no-tablespaces`)
- Comments on objects (`--no-comments`)

### 6.4 Known YB limitations that may cause expected differences

If the source PG schema uses features that YB does not support, the target will
be missing those objects. Voyager's assessment report (`analyze-schema`) already
flags these before migration. When they appear in a diff, the report should note
them as **expected due to YB limitation**, not as unexpected drift.

**Important: YB version awareness.** Many of these limitations are fixed in
newer YB versions. The issue definitions in `issues_ddl.go` carry a
`MinimumVersionsFixedIn` map that specifies which YB version resolves each
issue (e.g. `STORED_GENERATED_COLUMNS` is fixed in 2.25+/2025.1+/2025.2+;
`UNLOGGED_TABLES` is fixed in 2024.2+). The ignore rules must be
**version-aware**: when comparing against a target YB that is new enough, the
limitation no longer applies and a missing object IS a real difference, not an
expected one. The library should accept the target YB version and consult
`MinimumVersionsFixedIn` to decide whether to suppress each limitation.

The full list is in `src/query/queryissue/issues_ddl.go`. Key categories:

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

**Version-dependent features** (fixed in newer YB — ignore rules must check
target YB version):

| Constant | Feature | Fixed in YB |
|----------|---------|-------------|
| `STORED_GENERATED_COLUMNS` | Stored generated columns | 2.25+ / 2025.1+ |
| `UNLOGGED_TABLES` | UNLOGGED tables | 2024.2+ |
| `SECURITY_INVOKER_VIEWS` | Security invoker views | 2.25+ / 2025.1+ |
| `DETERMINISTIC_OPTION_WITH_COLLATION` | Deterministic attribute in collation | 2.25+ / 2025.1+ |
| `FOREIGN_KEY_REFERENCES_PARTITIONED_TABLE` | FK referencing partitioned table | 2.25+ / 2025.1+ |
| `SQL_BODY_IN_FUNCTION` | SQL-body functions (inline SQL body) | 2.25+ / 2025.1+ |
| `UNIQUE_NULLS_NOT_DISTINCT` | `UNIQUE NULLS NOT DISTINCT` | 2.25+ / 2025.1+ |
| `BEFORE_ROW_TRIGGER_ON_PARTITIONED_TABLE` | BEFORE ROW triggers on partitioned tables | 2.25+ / 2025.1+ |
| `ALTER_TABLE_ADD_PK_ON_PARTITIONED_TABLE` | Adding PK via ALTER to partitioned table | 2024.1+ |
| `UNSUPPORTED_EXTENSION` | Extensions not supported in YB | Version-dependent per extension |
| Multirange types (int4multirange, etc.) | 6 multirange types | 2.25+ / 2025.1+ |

**Unsupported data types:**

Voyager exports the DDL as-is via `pg_dump` — it does not strip unsupported
columns or skip tables. The assessment report (`analyze-schema`) flags these as
issues. During `import schema`, YugabyteDB will return errors for truly
unsupported types. Voyager can continue with `--continue-on-error`.

For **live migration specifically**, Voyager actively **excludes columns** with
unsupported types from the data export (Debezium path). The user is prompted to
confirm. These types include: point, line, lseg, box, path, polygon, circle,
geometry, geography, box2d, box3d, topogeometry, raster, pg_lsn, txid_snapshot,
xml, lo, multirange types, vector, timetz.
(Source: `src/srcdb/postgres.go` — `PostgresUnsupportedDataTypesForDbzm`)

For schema diff purposes: if a column with an unsupported type exists on the
source but was not created on the target (import failed), this will appear as a
column or table difference. The diff should annotate this as a known YB
limitation.

**Policy/role limitations:**

| Constant | Feature |
|----------|---------|
| `POLICY_WITH_ROLES` | Policies referencing specific roles (roles not migrated) |

---

## 7. Architecture

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
│  │  Indexes, Sequences, etc. (all types from §5)             │ │
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
│  │  Transformation rules (§6.1)                              │ │
│  │  Inherent PG-vs-YB rules (§6.2)                          │ │
│  │  YB limitation annotations (§6.4)                         │ │
│  └──────────────────────────────────────────────────────────┘ │
└───────────────────────────────────────────────────────────────┘
          │                              │
          ▼                              ▼
┌──────────────────┐        ┌──────────────────────┐
│ CLI command       │        │ Live migration        │
│ `yb-voyager       │        │ drift detection       │
│  schema diff`     │        │ (§8.3)                │
└──────────────────┘        └──────────────────────┘
```

### 7.1 Multi-database extensibility

The architecture uses a `SnapshotProvider` interface to decouple database-specific
catalog introspection from the database-agnostic diff engine:

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

**Implemented providers:**

| Provider | Database | Status | Catalog source |
|----------|----------|--------|----------------|
| `PostgresSnapshotProvider` | PostgreSQL, YugabyteDB | Implemented | `pg_catalog` views |
| `OracleSnapshotProvider` | Oracle | Stub | `ALL_TAB_COLUMNS`, `ALL_CONSTRAINTS`, `ALL_INDEXES`, etc. |
| `MySQLSnapshotProvider` | MySQL | Stub | `information_schema` views |

The `SchemaSnapshot` model, `Diff`, and `FilterDiff` functions are
database-agnostic — they operate on the snapshot struct, not on SQL queries.
The diff engine and ignore rules work unchanged regardless of which provider
populated the snapshot.

A factory function `NewSnapshotProvider(dbType)` returns the appropriate
provider, so callers need not import concrete types directly.

### 7.2 Snapshot serialization

`SchemaSnapshot` must be serializable to JSON so it can be persisted and
compared across different phases of the migration.

Storage location: `export-dir/metainfo/schema_snapshot.json`

---

## 8. Integration Points

### 8.1 Baseline snapshot capture: during `export schema`

**File:** `cmd/exportSchema.go` — function `exportSchema()`

**When:** After `source.DB().ExportSchema(exportDir, schemaDir)` completes
(line ~189) and before `saveSourceDBConfInMSR` (line ~236). At this point the
source connection (`source.DB()`, which is a `srcdb.SourceDB`) is open and
schemas are finalized.

**What:** Call `provider.TakeSnapshot(...)` and serialize to
`export-dir/metainfo/schema/source_baseline.json`.

**Connection wiring:** `SourceDB` interface exposes `Query(string) (*sql.Rows,
error)` and `QueryRow(string) *sql.Row`. Our `TakeSnapshot` currently takes
`*sql.DB`. We need to either:
- (a) Accept a `QueryExecutor` interface (`Query` + `QueryRow`) instead of
  `*sql.DB`, so we can pass `source.DB()` directly, or
- (b) Open a separate `*sql.DB` using the connection URI from the source config.

Option (a) is preferred — it avoids opening a separate connection and works
with any DB type that implements the interface.

**Why:** This is the authoritative baseline — the source schema state that
Voyager designed the migration around.

### 8.2 (Optional) Post-import snapshot capture: end of `import schema`

**File:** `cmd/importSchema.go` — function `importSchema()`

This is **not required** for the core drift detection use case (which compares
against the source). It would only serve as a verification step to confirm that
ignore rules are correctly suppressing expected PG-vs-YB differences after
migration. Can be deferred.

### 8.3 Standalone CLI command: `yb-voyager schema diff`

A new Cobra command. Follows the same UX patterns as `analyze-schema`,
`compare-performance`, and `get data-migration-report`:

- **`--export-dir` is the workspace** — provides the saved baseline snapshot,
  source/target connection details from MSR, and schema list.
- **Source and target connection info come from MetaDB** (saved during
  `export schema` / `import schema`) — the user does not re-specify host, port,
  db-name, etc. Only passwords are needed if not stored.
- **Report is always saved** to `{export-dir}/reports/schema_diff_report.{format}`
  (same pattern as `schema_analysis_report`, `performance_comparison_report`).
- **`--output-format`** controls format, following `analyze-schema` conventions.

#### Primary usage

```
yb-voyager schema diff \
  --export-dir /path/to/export \
  --source-db-password '...' \
  --target-db-password '...'
```

This single command runs **both** comparisons:

1. **Baseline vs live source** — detects source drift (no ignore rules, same
   DB type comparison)
2. **Baseline vs live target** — detects source-target discrepancies (PG-vs-YB
   ignore rules applied)

The report presents both results clearly in separate sections. The user gets a
complete picture without having to choose a mode or run the command twice.

#### How connection info is obtained

Source and target connection details (host, port, user, db name, schemas, SSL
config) are stored in the MetaDB `MigrationStatusRecord` during `export schema`
(`saveSourceDBConfInMSR`) and `import schema` / `import data`
(`updateTargetConfInMigrationStatus`). **Passwords and URIs are explicitly
cleared** before saving — this is a security measure applied across all of
Voyager.

This is the same pattern used by:
- `get data-migration-report` — loads `SourceDBConf` / `TargetDBConf` from
  MSR, gets passwords from flags/env/prompt
  (`getDataMigrationReportCommand.go` lines ~77–91)
- `import data` — loads `tconf` from `msr.TargetDBConf`, gets password from
  flag (`importData.go` lines ~182–190)
- `compare-performance` — loads source metadata from MSR, target from flags
  (`comparePerformanceCommand.go`)

The `schema diff` command follows this exact pattern.

#### Flags

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--export-dir` / `-e` | Yes | — | Migration workspace. Provides baseline snapshot, connection info from MSR, schema list. |
| `--source-db-password` | If source check needed | — | Source DB password. Also via env `SOURCE_DB_PASSWORD` or config file. |
| `--target-db-password` | If target check needed | — | Target DB password. Also via env `TARGET_DB_PASSWORD` or config file. |
| `--output-format` | No | `html,json` | Report format(s): `html`, `json`, `txt`. Comma-separated for multiple. |
| `--check` | No | `both` | Which comparison to run: `source`, `target`, or `both`. |
| `--config-file` / `-c` | No | — | YAML config file (can contain passwords and override any flag). |

Passwords can be supplied three ways (standard Voyager precedence):
1. **CLI flags** (`--source-db-password`, `--target-db-password`)
2. **Environment variables** (`SOURCE_DB_PASSWORD`, `TARGET_DB_PASSWORD`)
3. **Config file** (YAML, via `--config-file`)
4. **Interactive prompt** (if none of the above provided and terminal is
   interactive)

The `--check` flag exists for cases where only one side is accessible (e.g.
during export-data, only the source connection is available). By default it
runs both comparisons.

#### Prerequisites and guardrails

Source and target config are saved to MSR at different stages:

| Stage completed | MSR has source config | MSR has target config |
|-----------------|----------------------|----------------------|
| `export schema` | Yes | No |
| `import schema` | Yes | Yes |

The command must validate prerequisites based on `--check`:

- **`--check source`**: requires `export schema` done (source config in MSR).
  Error if not: `"Run 'export schema' before 'schema diff --check source'."`
- **`--check target`**: requires `import schema` or `import data` done (target
  config in MSR). Error if not: `"Run 'import schema' before 'schema diff
  --check target'. Target connection details are not yet available."`
- **`--check both`** (default): requires both. If only source config exists
  (export schema done, import schema not yet), **auto-downgrades** to
  `--check source` with a message: `"Target connection not yet available
  (import schema not run). Running source drift check only."`

This follows the pattern of `analyze-schema` which checks `schemaIsExported()`
before proceeding, and `compare-performance` which requires `assess-migration`
to have been run first.

**Baseline snapshot:** If `export schema` was run before this feature was
added, no baseline snapshot exists. The command takes a fresh source snapshot,
saves it as the baseline for future runs, and only runs the **target parity
check**. The source drift check is skipped because comparing a just-taken
snapshot against the same live source would always produce zero differences.

#### Report output

Reports are saved to:
- `{export-dir}/reports/schema_diff_report.html`
- `{export-dir}/reports/schema_diff_report.json`

The command prints the path:
```
-- find schema diff report at: /path/to/export/reports/schema_diff_report.html
```

A summary is also printed to the terminal:
```
Schema Diff Summary
===================
Source drift (baseline vs live source):  2 differences (1 CRITICAL, 1 WARNING)
Target parity (baseline vs live target): 1 difference (0 CRITICAL, 1 WARNING)
                                         3 expected PG-vs-YB differences suppressed

See full report: /path/to/export/reports/schema_diff_report.html
```

#### What the drift goroutine tells users

The live migration drift warning (§8.4) directs users to this simple command:

```
⚠ SCHEMA DRIFT DETECTED on source (checked 2026-03-30 14:23:05):
  CRITICAL: [COLUMN MODIFIED] public.orders.amount — type changed
  Run 'yb-voyager schema diff --export-dir <dir>' for full report.
```

No mode selection, no re-specifying connection details. Just the export dir.

### 8.4 Live migration drift detection

During live migration, `export data` and `import data` are **separate
long-running OS processes** coordinated via MetaDB (`meta.db`). Each has its
own DB connection that stays open for the entire streaming duration. We run
periodic drift detection in both independently via dedicated goroutines.

#### 8.4.1 Approach: dedicated goroutine (not inline hooks)

Rather than hooking into existing loops (Debezium polling, segment processing),
we spawn a **dedicated goroutine** for drift detection. Reasons:

- **Decoupled from hot path** — drift checks are expensive (multiple catalog
  queries). They should not add latency to event processing or status polling.
- **Independent interval** — we want to check every N minutes (e.g. 10 min),
  not every 500ms or 2s like the existing loops.
- **Consistent pattern** — both export and import already spawn goroutines for
  progress reporting (`go reportStreamingProgress(ctx)`,
  `go statsReporter.ReportStats(ctx)`). A drift detection goroutine follows
  the same pattern.
- **Clean lifecycle** — goroutine takes a `context.Context` and stops when the
  parent context is cancelled (same as progress goroutines).

#### 8.4.2 Source-side detection (in `export data`)

**File:** `cmd/exportDataDebezium.go`

**Where to start goroutine:** In `checkAndHandleSnapshotComplete()` (lines
~582–588), alongside the existing progress goroutines:

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

**Connection available:** `source.DB()` (global `srcdb.SourceDB` in
`cmd/export.go` line 36) — connected at `exportData()` line 488, disconnected
on return. Stays open for the entire streaming duration.

**What the goroutine does (pseudocode):**

```go
func monitorSourceSchemaDrift(ctx context.Context, db QueryExecutor,
    schemas []string, exportDir string) {
    baseline := loadBaselineSnapshot(exportDir)
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
            diff := schemadiff.Diff(baseline, current)
            // No ignore rules — same database type comparison
            if len(diff.Changes) > 0 {
                reportDrift(diff)
            }
        }
    }
}
```

**What to compare:** Source-now vs source-baseline. This is a **same-database
comparison** so no PG-vs-YB ignore rules apply. Almost all differences are real
drift.

#### 8.4.3 Target-side detection (in `import data`)

**File:** `cmd/live_migration.go`

**Where to start goroutine:** In `streamChanges()` (lines ~129–134), alongside
the existing stats reporter goroutine:

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

**Connection available:** `tdb` (global `tgtdb.TargetDB` in `cmd/import.go`
line 45). `TargetYugabyteDB` holds `db *sql.DB` (line 69 of
`tgtdb/yugabytedb.go`). The `QueryExecutor` interface approach from §8.1 lets
us pass `tdb` without exposing the private `*sql.DB` field.

**What to compare:** Target-now vs source-baseline. This is a **cross-database
comparison** so PG-vs-YB ignore rules apply.

#### 8.4.4 Display integration: how drift warnings appear

Both export and import streaming use `uilive` (`github.com/gosuri/uilive`) to
render a refreshing metrics table in the terminal. Existing patterns for
injecting messages into this display:

- **Import side:** `StreamImportStatsReporter` has a `DisplayInformation(info
  string)` method (in `src/reporter/stats/streamImportReporter.go` line 133)
  that appends text to a `displayExtraInfo` field. On the next refresh cycle,
  this text appears as a **red row** at the bottom of the uilive table.
- **Export side:** No equivalent `DisplayInformation` method exists today. We
  would add one to the export progress display.
- **When `--disable-pb` is set:** Warnings go through
  `utils.PrintAndLog(info)` directly (see `displayMonitoringInformationOnTheConsole`
  in `cmd/importData.go` lines ~1636–1650).

**Example: what the user sees during `export data` streaming when drift is detected:**

```
| Metric                                   |                        Value |
| ---------------------------------------- | ---------------------------- |
| Total Exported Events                    |                       45,231 |
| Export Rate (last 3 min)                 |                  1,204 rows/s |
| Total Exported Events (this run)         |                       12,405 |
| Export Elapsed Time (this run)           |                   00:34:12   |
| ---------------------------------------- | ---------------------------- |

⚠ SCHEMA DRIFT DETECTED on source (checked 2026-03-30 14:23:05):
  CRITICAL: [COLUMN MODIFIED] public.orders.amount — type: numeric(10,2) -> numeric(12,4)
  WARNING:  [PARTITION ADDED] public.events -> public.events_2026_04
  Run 'yb-voyager schema diff --export-dir <dir>' for full report.
```

**Example: what the user sees during `import data` streaming:**

```
| Metric                                   |                        Value |
| ---------------------------------------- | ---------------------------- |
| Total Imported Events                    |                       38,917 |
| Import Rate (last 3 min)                |                    982 rows/s |
| Remaining Events                         |                        6,314 |
| ---------------------------------------- | ---------------------------- |

⚠ SCHEMA DRIFT DETECTED between source baseline and target (checked 2026-03-30 14:23:05):
  CRITICAL: [COLUMN MODIFIED] public.orders.amount — type: numeric(10,2) -> numeric(12,4)
  Run 'yb-voyager schema diff --export-dir <dir>' for full report.

Suppressed: 3 expected PG-vs-YB differences (use --verbose for details)
```

**When `--disable-pb` is set** (no uilive table):

```
2026/03/30 14:23:05 [WARN] Schema drift detected on source:
  CRITICAL: [COLUMN MODIFIED] public.orders.amount — type: numeric(10,2) -> numeric(12,4)
  WARNING:  [PARTITION ADDED] public.events -> public.events_2026_04
  Run 'yb-voyager schema diff --export-dir <dir>' for full report.
```

#### 8.4.5 Process coordination (not needed)

Export and import are separate OS processes. Each runs drift detection
independently using only its own DB connection. No cross-process RPC, shared
memory, or MetaDB coordination is needed for drift detection itself.

If we want export-side drift results visible to the import side (or vice
versa), we could write results to a file in the export directory that the
other process reads — but this is not required for V1.

### 8.5 Integration flow summary

```
export schema                 import schema
     │                              │
     ├─ source.DB().Connect()       ├─ pgx.Connect(target)
     ├─ ExportSchema(...)           ├─ importSchemaInternal(...)
     ├─ ★ TakeSnapshot(source)      │   (optional: snapshot target)
     │   → source_baseline.json     │
     ├─ Disconnect()                ├─ Close()
     │                              │
     ▼                              ▼
export data (live)            import data (live)
     │                              │
     ├─ source.DB().Connect()       ├─ tdb.InitConnPool()
     ├─ startDebezium()             ├─ streamChanges()
     │                              │
     │  ┌────────────────────┐      │  ┌────────────────────┐
     │  │ go reportProgress  │      │  │ go statsReporter    │
     │  │ go calcProgress    │      │  │   .ReportStats      │
     │  │ ★ go monitorDrift  │      │  │ ★ go monitorDrift   │
     │  └────────────────────┘      │  └────────────────────┘
     │                              │
     │  monitorDrift goroutine:     │  monitorDrift goroutine:
     │  every N min:                │  every N min:
     │    snapshot source           │    snapshot target
     │    diff vs baseline          │    diff vs baseline
     │    (no ignore rules)         │    (with YB ignore rules)
     │    → uilive warning row      │    → uilive warning row
     │      or log.Warn if no pb   │      or log.Warn if no pb
     │                              │
     ├─ Disconnect()                ├─ Finalize()
     ▼                              ▼

Standalone: yb-voyager schema diff --export-dir <dir>
     │
     ├─ Load baseline from export-dir
     ├─ Load source/target conn info from MSR
     ├─ Connect(source) + Connect(target)
     ├─ TakeSnapshot(source) + TakeSnapshot(target)
     ├─ Diff(baseline, source) — source drift
     ├─ Diff(baseline, target) + FilterDiff(rules) — target parity
     ├─ Save report to {export-dir}/reports/schema_diff_report.{html,json}
     ├─ Print summary to terminal
     └─ Disconnect both
```

---

## 9. Diff Severity Classification

Each `DiffEntry` carries a severity level. The classifications below are
**proposed** and need team validation.

| Severity | Meaning |
|----------|---------|
| **CRITICAL** | Will break live migration or cause silent data loss. User must act. |
| **WARNING** | May cause incorrect behavior or missed data. User should investigate. |
| **INFO** | Low-impact difference. Informational; no action usually required. |

### Proposed classification per change type

**Open question for team:** Please review each row. These are based on our
understanding of how each change affects live migration. We need confirmation.

| Change | Proposed Severity | Reasoning |
|--------|-------------------|-----------|
| **Table dropped** | CRITICAL | All data for this table will fail to import |
| **Table added on source** | WARNING | New table not in Voyager's table list; data will be missed |
| **Column dropped** | CRITICAL | CDC events will reference a column that doesn't exist on target |
| **Column added on source** | WARNING | New column data won't be captured by existing CDC configuration |
| **Column type changed** | CRITICAL | Type mismatch between CDC events and target schema |
| **Column nullable changed** | WARNING | May cause constraint violations during import |
| **Column default changed** | INFO | Defaults are applied at write time; usually cosmetic for migration |
| **PK columns changed** | CRITICAL | Voyager relies on PK for CDC ordering and conflict detection |
| **PK added/dropped** | CRITICAL | Same as above |
| **UK columns changed** | WARNING | May affect conflict detection on target |
| **UK added/dropped** | WARNING | Same as above |
| **FK added/dropped/changed** | INFO | FKs are deferred during import; changes rarely break migration |
| **CHECK constraint changed** | INFO | May cause import failures if new check rejects existing data |
| **Partition added** | WARNING | New partition's data won't be migrated (not in table list) |
| **Partition dropped** | WARNING | CDC events for dropped partition may cause errors |
| **Index added/dropped** | INFO | Indexes don't affect data correctness |
| **Index columns/method changed** | INFO | Same reasoning |
| **Sequence properties changed** | INFO | Sequence values are restored post-migration |
| **View/MV definition changed** | INFO | Views are not part of data migration pipeline |
| **Function/procedure changed** | INFO | Functions are imported from DDL files, not live-synced |
| **Trigger added/dropped/changed** | WARNING | May affect data behavior on target during import |
| **Enum values changed** | WARNING | CDC events may contain values not in target's enum definition |

---

## 10. User Experience: What We Report

### 10.1 Report file

Report is saved to `{export-dir}/reports/schema_diff_report.{format}`,
following the same convention as `schema_analysis_report` and
`performance_comparison_report`.

Supported formats via `--output-format`:

| Format | File | Use case |
|--------|------|----------|
| **HTML** | `schema_diff_report.html` | Human-readable, shareable with support/team |
| **JSON** | `schema_diff_report.json` | Machine-readable, automation-friendly |
| **TXT** | `schema_diff_report.txt` | Plain text for terminal/log review |

Default: generates both HTML and JSON (same as `analyze-schema`).

The HTML report will use a `text/template` in `cmd/templates/` following the
existing pattern for `migration_assessment_report.template` and
`schema_analysis_report.html`.

### 10.2 Full report structure (HTML/TXT)

The report has two sections — one per comparison:

```
Schema Diff Report
==================
Generated: 2026-03-30 14:25:00
Export dir: /path/to/export
Baseline:  source snapshot from export schema (2026-03-28 10:00:00)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Section 1: Source Drift (baseline vs live source)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Source: postgresql://source-host:5432/mydb (schemas: public)

CRITICAL (1 issue):
  [COLUMN MODIFIED] public.orders.amount
    type: numeric(10,2) -> numeric(12,4)
    Action: ALTER the column type on target to match, then restart import.

WARNING (1 issue):
  [PARTITION ADDED] public.events -> public.events_2026_04
    new partition on source
    Action: Create this partition on target and add to table list.

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Section 2: Target Parity (baseline vs live target)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Target: yugabytedb://target-host:5433/mydb (schemas: public)

No unexpected differences found.

Suppressed (3 expected PG-vs-YB differences):
  [INDEX MODIFIED] public.orders_pkey
    method: btree -> lsm (expected: YB uses LSM for btree indexes)
  [INDEX MODIFIED] public.idx_orders_date
    method: btree -> lsm (expected: YB uses LSM for btree indexes)
  [INDEX DROPPED] public.idx_orders_redundant
    (expected: removed as redundant by Voyager)
```

### 10.3 Terminal summary (always printed)

After saving the report, the command prints a brief summary:

```
Schema Diff Summary
===================
Source drift (baseline vs live source):  2 differences (1 CRITICAL, 1 WARNING)
Target parity (baseline vs live target): 0 real differences
                                         3 expected PG-vs-YB differences suppressed

-- find schema diff report at: /path/to/export/reports/schema_diff_report.html
```

### 10.4 Behavior during live migration

The drift detection goroutine (§8.4) uses a **condensed format** suitable for
the uilive progress table. Only CRITICAL and WARNING are shown inline; INFO
diffs are logged only:

**On CRITICAL diff detected:**
- Append warning row to uilive table (red text via `DisplayInformation`)
- Log the full diff to Voyager's log file
- Include the `yb-voyager schema diff --export-dir` command for full report

**On WARNING diff detected:**
- Append warning row to uilive table
- Log the full diff

**On INFO diff detected:**
- Log only (not displayed in terminal)

**Open question for team:** Should detection of CRITICAL diffs affect the
migration process in any way (e.g. set an error state, require acknowledgment),
or should it be purely informational with the user deciding what to do?

---

## 11. Open Questions for Team Review

1. **Severity classification (§9):** Review the proposed severity for each
   change type. Are we over- or under-classifying anything?

2. **Live migration detection approach (§8.3):** Start with standalone CLI
   only, or also add proactive detection in export/import data from the start?
   If proactive, what frequency?

3. **Function body comparison (§5.8):** Compare bodies or only existence +
   signature?

4. **V1 scope:** Which of the object types in §5 should be in V1 vs deferred?

5. **CRITICAL diff behavior (§10.3):** Purely informational, or should it
   affect migration state?

6. **PK change handling:** Block migration entirely, or allow with user
   confirmation? (Pending input from Priyanshi)

7. **Complete ignore rule validation:** The ignore rules in §6.1 need to be
   tested against actual PG-vs-YB catalog output to confirm they are exhaustive.
   Are there other Voyager transformations we are missing?

---

## 12. Known Limitations & Design Considerations

These are architectural constraints and trade-offs we are aware of. Some may be
addressed in future iterations; others are inherent to the approach.

### 12.1 Snapshot consistency (transaction isolation)

`TakeSnapshot` issues multiple sequential queries (`loadTables`, `loadColumns`,
`loadConstraints`, etc.) against the database. If the schema is being modified
concurrently (e.g. a DDL statement runs between our `loadTables` and
`loadColumns` queries), the snapshot may be internally inconsistent — a table
could appear in `Tables` but have no columns, or a constraint could reference a
column that wasn't captured.

**Mitigation options (to be decided):**
- Wrap all catalog queries in a single `REPEATABLE READ` or `SERIALIZABLE`
  transaction, which guarantees a consistent view across all queries.
- Accept the risk for now, since schema DDL during a snapshot is rare and
  the snapshot is a diagnostic tool, not a data-integrity mechanism.

### 12.2 DiffEntry detail is a free-form string

Currently `DiffEntry.Details` is a human-readable string (e.g.
`"type: int4 -> int8; nullable: true -> false"`). This works for display but
makes it difficult for code to programmatically inspect what changed — for
example, determining whether a column type change is a widening (safe) vs
narrowing (dangerous) conversion.

**Future consideration:** Add structured fields to `DiffEntry` (e.g.
`OldValue`, `NewValue`, `Property`) alongside the human-readable `Details`,
so that severity classification and automated actions can inspect changes
programmatically rather than parsing strings.

### 12.3 Ignore rule composability

Different comparison scenarios need different ignore rule sets:

| Scenario | Rules needed |
|----------|-------------|
| Source vs source (drift detection) | Minimal — almost all differences are real drift |
| Source PG vs target YB | Full set: Voyager transformations (§6.1) + inherent PG-vs-YB (§6.2) + YB limitations (§6.4) |
| Target YB vs saved baseline | Subset of §6.2 + §6.4 only (no Voyager transformation rules) |

The current `DefaultYBIgnoreRules` function returns a fixed set. The design
should support composing rule sets based on the comparison context, so callers
can select only the rules relevant to their scenario.

### 12.4 Baseline snapshot validity over time

The baseline snapshot captured during `export schema` (§8.1) represents the
source at a specific point in time. Over a long-running migration (days/weeks),
both source and target may evolve. The baseline may become stale — not because
of drift, but because the migration has progressed through multiple phases.

Considerations:
- Should the baseline be refreshed at certain milestones (e.g. after
  `import schema` completes)?
- Should we store multiple snapshots (source-at-export, target-after-import)
  for more precise comparisons?

### 12.5 Performance at scale

For databases with thousands of tables, tens of thousands of columns, and many
indexes, the catalog queries and in-memory snapshot may become slow or
memory-intensive. The current prototype loads everything into memory at once.

For V1 this is acceptable since Voyager already handles large schemas. If it
becomes a bottleneck, we could add schema-level parallelism or incremental
snapshot loading.

### 12.6 Type normalization across databases

When comparing across different database types (PG vs Oracle, PG vs MySQL),
the same logical type has different names (`int4` vs `NUMBER(10)` vs `INT`).
The `SchemaSnapshot` stores types as reported by each database's catalog.
Cross-database comparison will require a type normalization layer that maps
database-specific type names to a common canonical form. This is not needed
for V1 (PostgreSQL only) but will be required when Oracle/MySQL providers
are implemented.
