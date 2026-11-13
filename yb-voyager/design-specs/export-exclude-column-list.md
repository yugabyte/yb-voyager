# Excluding columns from live-migration streaming: `--exclude-column-list`

|  |  |
| :---- | :---- |
| **Status** | Draft |
| **Author** | Priyanshi Gupta |
| **Issue** | [\#3911](https://github.com/yugabyte/yb-voyager/issues/3911) |
| **Implementation** | Branch `priyanshi/exclude-column-list` |
| **Contractual** | §3 user-facing surface, §5 behaviour, §6 shared guardrails, §8 flow matrix, §9 serialized state. Everything else is advisory. |

## 1\. Context

Before this change, when `export data` found columns whose datatype the migration flow cannot stream, it printed them and asked whether to continue. On "yes" it silently dropped those columns from the Debezium column list. That has three problems:

- A prompt is a poor contract for an automated migration. Scripts pipe `yes` past it, so columns are dropped without anyone deciding to drop them.
- The decision is not recorded. A resumed or restarted export recomputes it, and nothing stops two runs from streaming different column sets.
- Some unsupported columns cannot be dropped at all. Excluding a primary-key column crashes Debezium's record transformer (the event key still carries the column but the value schema does not), and the importer cannot apply updates or deletes without the key.

This design replaces the prompt with an explicit flag. For PostgreSQL and YugabyteDB sources in live migration, `export data` fails while any unsupported column is left and tells the user exactly what to pass:

```
--exclude-column-list '<table>:<col1>,<col2>;<table2>:<col1>,<col2>'
```

**Requirements**

- For PG/YB live migration, a column with an unsupported datatype either appears in `--exclude-column-list` or fails the export. Nothing is dropped implicitly.
- Every entry the user passes is checked against the tables actually being exported, before Debezium starts: the table exists and is exported, each column exists, and no column is in the primary key.
- All problems in one value are reported together, not one per run.
- The error suggests a value that can be pasted as is: the user's own value plus the columns still missing.
- A table whose unsupported column is in the primary key can only be excluded as a whole, and the error says so.
- The column set is fixed for the whole migration. A resumed or restarted exporter that passes a different set fails.
- `export data from target` (fall-back, fall-forward) gets its own flag, because the YugabyteDB connectors support different datatypes from the PostgreSQL one.
- Case-sensitive table and column names work, and partitioned tables are handled through their root.

**Non-goals**

- **Excluding columns from the snapshot.** The PostgreSQL snapshot is taken with `pg_dump`, which copies every column. The flag changes only what Debezium streams (§5.1).
- **Oracle and MySQL.** Their behaviour, including the prompt, is unchanged. The flag is rejected for them (§5.2).
- **Offline migration.** Offline export has no streaming, so the flag is rejected there.
- **Changing which types are unsupported.** The per-connector lists and the type-tree walk are covered by [unsupported-datatype-reporting.md](unsupported-datatype-reporting.md).
- **Import changes.** Excluded columns are absent from the Debezium events and the importer needs no change.

## 2\. Where it sits

```
 export data / export data from target
   PreRun ── validateExcludeColumnListFlag ─────────────── rejects Oracle/MySQL, offline, bad syntax;
   │                                                        parseTableColumnsFlag keeps the parsed map
   │
   finalizeTableAndColumnList (cmd/exportData.go)
     1. FilterEmptyTables            (offline only)
     2. FilterUnsupportedTables
     3. GetColumnsWithSupportedTypes ── srcdb: supported / unsupported columns per table
        │
        ├── PG or YB source, live ──► finalizeColumnListForLiveMigration          (new path)
        │                               srcdb GetTableToColumns, GetPrimaryKeyColumns
        │                               addPrimaryKeyToRootTableIfRequired
        │                               validateExcludedColumns ─┬─ resolveExcludeColumnList ─┐
        │                                                        └─ resolveTableColumns ──────┴─ shared guardrails (§6)
        │                               handleExcludedColumnForTargetExporter ── MSR (target exporter only)
        │                               applyExcludedColumns
        │                               buildUnsupportedColumnsError      (on leftovers)
        │                               recordExcludedColumns ── MSR
        │
        └── Oracle, MySQL, offline PG ─► handleUnsupportedColumnsInExportData   (old prompt, unchanged)
   │
   tablesColumnList ──► prepareDebeziumConfig ──► debezium.source.column.include.list
                    └─► pg_dump snapshot (ignores the list)

 cutover / import data
   setUpNextIterationMSR ── copies ExcludedColumns to the next iteration
   startExportDataFromSourceOnNextIteration ── passes --exclude-column-list <FromSource>
```

All new export-side code lives in a separate section at the bottom of `cmd/exportData.go`. The parsing and lookup helpers it shares with import live in `cmd/exportImportCommonGuardrails.go`, and column-name matching in `src/utils/sqlname/column_match.go`.

## 3\. User-facing surface

### 3.1 Flag

| Command | Flag | Config key |
| :---- | :---- | :---- |
| `export data` (`export data from source`) | `--exclude-column-list` | `export-data` / `export-data-from-source`: `exclude-column-list` |
| `export data from target` | `--exclude-column-list` | `export-data-from-target`: `exclude-column-list` |

The flag is registered once in `registerExportDataFlags` (`cmd/export.go`), which both commands use, and the key is allowed in `allowedExportDataConfigKeys` and `allowedExportDataFromTargetConfigKeys` (`cmd/config.go`). It is documented in the three live config templates (`live-migration.yaml`, `live-migration-with-fall-back.yaml`, `live-migration-with-fall-forward.yaml`); the fall-back and fall-forward templates also document it under `export-data-from-target`. It is not in the offline template.

### 3.2 Value syntax

```
<entry>[;<entry>...]
<entry>  = <table>:<column>[,<column>...]
<table>  = [<schema>.]<table name>
```

- Entries are separated by `;`, the table from its columns by `:`, and columns by `,`. None of these split inside double quotes, so a quoted name may contain them.
- On the command line the value needs single quotes because of `;`.
- **Tables** resolve through the name registry, as `--table-list` does. A bare name resolves in the default schema. Case-sensitive table names must be double-quoted.
- **Columns** are matched against the table's real columns ignoring case (§5.5). A column name needs double quotes only when the table has columns that differ only in case.
- **Partitioned tables** are named by their root table. The exclusion applies to every partition.

Example:

```
--exclude-column-list 'public.orders:note,"Meta";"Sales"."Geo":geom'
```

### 3.3 Help text

> semicolon-separated list of `<table>:<column1>,<column2>,...` entries naming the columns to leave out of the changes streamed in live migration. The snapshot still copies these columns. Only for PostgreSQL source databases and 'export data from target'. For partitioned tables, specify the root table; the columns are excluded from all its partitions. Primary key columns cannot be excluded. Column names are matched ignoring case; double-quote a column name only if the table has columns differing just in case. In case the table names are case sensitive, double-quote them.

## 4\. What happens to an excluded column

| Phase | Source → target (`export data`) | Target → source/replica (`export data from target`) |
| :---- | :---- | :---- |
| Snapshot | Copied by `pg_dump`. The target has the snapshot value. | n/a |
| UPDATE during streaming | Not streamed. The target keeps the snapshot value. | Not streamed. The source/replica keeps its pre-cutover value. |
| INSERT during streaming | Streamed without the column. The target row has NULL, or the column default if the target applies one. | Streamed without the column. NULL on the source/replica. |
| NOT NULL on the receiving side | Inserts fail. The user must drop the constraint; every message about excluded columns says so. | Same, on the source/replica. |

## 5\. Behaviour

### 5.1 Scope: streaming only

The finished column list feeds Debezium's `column.include.list` through `prepareDebeziumConfig`. The PostgreSQL snapshot is a `pg_dump`, which does not take a column list. Excluding columns from the snapshot as well would need a separate mechanism (§13) and is out of scope.

### 5.2 Flag validation (`validateExcludeColumnListFlag`)

Runs in the command's PreRun, before any table or database work.

| Condition | Result |
| :---- | :---- |
| Flag empty | Nothing to do. |
| Source is Oracle or MySQL | Fail: `--exclude-column-list is supported only for PostgreSQL source databases`. |
| `--export-type snapshot-only` | Fail: `--exclude-column-list is supported only for live migration (--export-type snapshot-and-changes or changes-only)`. |
| Syntax error | Fail with the parse error (§5.3). Otherwise the parsed value is kept for `finalizeColumnListForLiveMigration`. |

YugabyteDB is accepted because `export data from target` runs with YB as its source. YugabyteDB is never exported offline.

### 5.3 Parsing (`parseTableColumnsFlag`)

`parseTableColumnsFlag` is shared with import (§6) and built on `parsePerTableFlag` and `parseColumnNameList`. It needs no database, so it runs in PreRun. It rejects:

- unbalanced double quotes
- an entry with no `:`, or with a second unquoted `:`
- an empty table name or an empty column list
- the same table name written twice
- an empty column name, or the same column name twice in one entry
- a non-empty value with no entries, such as `;`

Trailing `;` and whitespace around names are ignored. Parsing produces a `map[string][]string` from each table, as typed, to its columns, as typed. The order of the entries is not kept: every list printed later is sorted by table name.

### 5.4 Two paths in `finalizeTableAndColumnList`

Step 3 of `finalizeTableAndColumnList` used to call `handleUnsupportedColumnsInExportData` for every source. It now branches:

| Source and mode | Path |
| :---- | :---- |
| PostgreSQL or YugabyteDB, live (`snapshot-and-changes`, `changes-only`) | `finalizeColumnListForLiveMigration` (new). Fails on any unsupported column left. |
| Oracle, MySQL, or offline PostgreSQL | `handleUnsupportedColumnsInExportData` (prompt and skip) and `filterTableWithEmptySupportedColumnList`, unchanged. |

The new path runs after the table-level filters, so it works on exactly the tables that will be exported.

`GetTableToColumns` is on the `SourceDB` interface and returns each table's column names in definition order. PostgreSQL and YugabyteDB implement it with the same catalog query as `GetColumnsWithSupportedTypes`. Oracle and MySQL `panic("not implemented")`; they never reach it because the flag is rejected for them and the new path runs only for PG/YB.

### 5.5 Checking the entries (`validateExcludedColumns`)

`validateExcludedColumns` takes the parsed map and a table-lookup function, resolves the tables (`resolveExcludeColumnList`), and matches each table's columns (`resolveTableColumns`). It returns the resolved `NameTuple` → matched columns and every problem found. All problems are reported in one error:

```
invalid --exclude-column-list:
  public.missing: table not found
  public.orders: table public.orders is already listed as orders
  public.sales_q1: is a partition of public.sales; specify the root table public.sales
  public.audit: table is not part of this export; check --table-list and --exclude-table-list
  public.orders: column "foo" does not exist
  public.orders: column "note" is ambiguous; double-quote the exact column name
  public.orders: column "doc" is listed more than once
  public.orders: column "id" is part of the primary key and cannot be excluded
```

| Check | Rule |
| :---- | :---- |
| Table lookup | The lookup passed in; `finalizeColumnListForLiveMigration` passes `LookupTableNameAndIgnoreIfTargetNotFoundBasedOnRole`. Tables are looked up in sorted order. A table that is not found, or that resolves to a table already listed under another spelling, is a problem. |
| Partition root | `GetRootTableOfPartition`. Naming a leaf partition is a problem that names the root to use instead. On the target a partitioned table is one table, so excluding a column on only some partitions would leave it partly NULL. |
| Exported | The table must be in the finalized table list. A table removed by `--table-list` / `--exclude-table-list`, or skipped by voyager, is a problem (see §14 on skipped tables). |
| Column match | `resolveTableColumns`, which calls `sqlname.FindBestMatchingColumnName` against the table's columns: an exact match, then a single case-insensitive match, then the all-lowercase column if several differ only in case. Otherwise "does not exist" or "ambiguous". |
| Duplicate after matching | Two spellings of the same column are a problem. |
| Primary key | `GetPrimaryKeyColumns`. A key column cannot be excluded. If its datatype is also unsupported, the message adds that the whole table must be excluded with `--exclude-table-list` (on the source exporter) or had to be (on the target exporter). |

Columns are checked once, on the root table. PostgreSQL requires every partition to have its root's columns. After the checks the matched catalog names replace what the user typed, so everything later, including what is saved in the MSR, uses real column names.

Tables without a primary key are already rejected for live migration by `reportUnsupportedTablesForLiveMigration`, with one exception: a partitioned root with no key of its own is accepted when every partition has one, and `reportLeafPartitionsWithMismatchedPrimaryKeys` ensures the partitions' keys are identical. Such a root takes its partitions' key for these checks (`addPrimaryKeyToRootTableIfRequired`), so `--exclude-column-list` rejects that key's columns, and an unsupported column in it is reported as a primary-key column.

### 5.6 Applying the exclusions (`handleExcludedColumnForTargetExporter`, `applyExcludedColumns`)

1. **Effective set.**
   - **Source exporter:** the user's list.
   - **Target exporter:** `handleExcludedColumnForTargetExporter`, called only for the target exporter, takes for each root table the union of `ExcludedColumns.FromSource` (saved by the source exporter) and its own list. Columns excluded on the source export are never kept up to date on the target, so streaming them back would overwrite the source or replica's current values with stale snapshot values or NULL on every UPDATE, because events carry the full row.
2. **Per exported table**, roots and leaves alike, take the exclusions of the table's root and remove them from the table's supported column list.
   - A table with exclusions always gets an explicit column list. If its supported list was `*`, it is expanded to all columns first, since `*` would let Debezium stream the excluded ones.
   - A table with no exclusions keeps its list unchanged, including `*`.
   - With `--table-list` selecting only some leaves, only those leaves are exported and get the root's exclusions.
   - The copy to leaves is redone on every run, so partitions attached later are covered.
3. **Leftovers.** For each table, the unsupported columns not excluded are collected, once per root:
   - `tablesWithNoColumns`: every column was excluded.
   - `primaryKeyTables`: an unsupported column is in the root's primary key.
   - `otherTables`: everything else.

   `primaryKeyTables` and `otherTables` map each root table (as `ForOutput`) to its columns.

### 5.7 Unsupported columns left (`buildUnsupportedColumnsError`)

If any leftovers exist, export fails. Each section is printed only when it has entries; a table in the primary-key group is not repeated in the other group. Suggested values name root tables and are merged with what the user already passed, so they can be pasted as is. Tables are listed sorted by name, in the column lists and in the suggested values; each table's columns keep their order.

**Source exporter:**

```
Every column of the following tables is excluded by --exclude-column-list:
  public.audit
Exclude these tables with --exclude-table-list instead.

The following primary key columns have datatypes not supported for this migration:
  public.test_pk_un: id
A table cannot be migrated without its primary key. Exclude these tables with --exclude-table-list:
  --exclude-table-list 'public.test_pk_un'

The following columns have datatypes not supported for this migration:
  public.t1: geom, rng
To proceed, do one of the following:
  1. Exclude these columns with --exclude-column-list:
       --exclude-column-list 'public.orders:note;public.t1:geom,rng'
     Changes to excluded columns are not streamed: the target keeps their snapshot values, and rows
     inserted during streaming get NULL in them. If any of them has a NOT NULL constraint in the
     target database, remove it.
  2. Exclude these tables with --exclude-table-list:
       --exclude-table-list 'public.t1'
```

**Target exporter.** `export data from target` cannot take a table list, so:

- A table with an unsupported key column cannot be fixed at this stage. The message says it had to be excluded with `--exclude-table-list` on `export data from source`, which means starting the migration again.
- For other columns, only `--exclude-column-list` is suggested ("Restart 'export data from target' with these columns excluded"). The note says that the source or replica keeps their values from before cutover, and that rows inserted after cutover get NULL.

The messages name the columns but not their datatypes.

### 5.8 Success output

```
Excluding the following columns from the changes streamed in live migration:
  public.orders: doc, note
  public.sales: "Geo"
Changes to excluded columns are not streamed: ...
```

The list covers the effective set (§5.6), so on the target exporter it includes the columns inherited from the source export. The note matches the exporter's role.

### 5.9 Datatype differences between connectors

`export data from target` streams from YugabyteDB, whose connectors do not support the same types as the PostgreSQL source connector:

| Datatype | PG source (`PostgresUnsupportedDataTypesForDbzm`) | YB logical (`YugabyteUnsupportedDataTypesForDbzmLogical`) | YB gRPC (`YugabyteUnsupportedDataTypesForDbzmGrpc`) |
| :---- | :---- | :---- | :---- |
| `TSQUERY` | supported | **unsupported** | **unsupported** |
| `TSVECTOR`, `HSTORE`, `CITEXT`, `LTREE`, composite types | supported | supported | **unsupported** |
| `BOX2D`, `BOX3D`, `TOPOGEOMETRY`, `LO` | unsupported | not in the list | not in the list |

A column that streamed fine forwards can therefore fail in reverse. That is why the target exporter has its own flag and its own saved set.

### 5.10 Keeping the set fixed (`recordExcludedColumns`)

The column set Debezium streams must not change during a migration, otherwise some events would carry a column and others would not.

| Situation | Behaviour |
| :---- | :---- |
| First run of the exporter | Saves its own resolved list (not the merged set) in the MSR: `ExcludedColumns.FromSource` or `ExcludedColumns.FromTarget`. An empty map is saved when no flag is passed. |
| Resume with the same set | Continues. The comparison is on resolved table keys and sorted column names, so a different spelling of the same set passes. |
| Resume with a different set | Fails and prints both values in flag syntax, so the user can re-run with the original: `--exclude-column-list cannot change once export data has started. started with: '...' now: '...' Re-run with the value it started with.` |
| `export data --start-clean` | `clearMigrationStateIfRequired` resets `ExcludedColumns`. |
| Next cutover iteration | `setUpNextIterationMSR` copies `ExcludedColumns` into the new iteration's MSR. `startExportDataFromSourceOnNextIteration` passes `--exclude-column-list` rendered from `FromSource` by `formatExcludeColumnList` (sorted tables, min-quoted columns), so the spawned exporter passes the resume check. |

### 5.11 How `export data from target` gets its exclusions

Source and target have the same tables and columns, so most columns that need excluding in reverse were already excluded on the source export. Those reach the target exporter without being passed again. Only the columns the YB connector cannot stream on top of that need the target exporter's own flag.

| Columns | How the target exporter gets them | User action |
| :---- | :---- | :---- |
| Excluded on the source export (`FromSource`) | Read from the MSR in the same export directory by `handleExcludedColumnForTargetExporter` and merged per root table (§5.6). They are removed before the datatype check, so they never cause an error on the target. | None. |
| Unsupported only by the YB connector and not excluded on the source (`TSQUERY`; with gRPC also `HSTORE`, `CITEXT`, `LTREE`, `TSVECTOR`, composites, see §5.9) | Only from the target exporter's own `--exclude-column-list`. Without it, export fails at startup with the exact value to use (§5.7). | Add them to `export-data-from-target: exclude-column-list`, or pass the flag. |

**The spawned command does not carry `FromSource`.** `import data` starts `export data from target` itself (`generateExportDataFromTargetCommand`), and it deliberately does not pass the source's list as `--exclude-column-list`:

- **One copy of the fact.** The MSR already holds the list; a command-line copy would be a second one.
- **It would block the user's own flag.** The target exporter saves whatever its flag holds as `FromTarget` and refuses to resume with a different list (§5.10). If the spawned command passed the source columns, a later restart with "source columns plus the extra ones" would count as a changed list.

**How the extra columns reach the spawned exporter today:**

| Mode | Behaviour |
| :---- | :---- |
| Config file | The spawned command gets `--config-file` through the global arguments, so `export-data-from-target: exclude-column-list` is read. Works without user action at cutover. |
| CLI | The spawned command gets only global flags. If extra unsupported columns exist, the target exporter fails at startup; the user restarts `export data from target` by hand with the suggested flag value. Recoverable, but it always fails once, and automated runs (the gRPC `pg/datatypes` migtests) have no one to restart it. |

**Proposed: check at cutover time.** `initiate cutover to target` with `--prepare-for-fall-back` or fall-forward knows which connector the target exporter will use. It can run the YB connector's datatype check on the exported tables, leave out everything in `FromSource`, and fail with the suggested `export-data-from-target: exclude-column-list` value before anything is spawned. The same point could take a value for CLI users (for example `--exclude-column-list-for-target`), save it in the MSR, and have `generateExportDataFromTargetCommand` pass it on. Tracked in §14.

## 6\. Guardrails shared with `--cdc-partition-key-overrides`

`import data`'s `--cdc-partition-key-overrides` uses the same `<table>:<value>;...` shape (`public.orders:table;sales.events:pk;public.payments:(customer_id,region)`). The shape and lookup checks are shared, and each flag keeps the rules for what its value means.

| Helper | Location | Used by export | Used by import |
| :---- | :---- | :---- | :---- |
| `parseTableColumnsFlag` | `cmd/exportImportCommonGuardrails.go` | `validateExcludeColumnListFlag` | — (its values are `pk`, `table` or `(cols)`) |
| `resolveTableColumns` | same | `validateExcludedColumns` | `validateAndFinalizeCDCPartitionKeysPerTable` (custom key columns, matched against the target with `tconf.TargetDBType`) |
| `parsePerTableFlag` | same | `parseTableColumnsFlag` | `parseCdcPartitionKeyOverrides` |
| `parseColumnNameList` | same | `parseTableColumnsFlag` | `parseCdcPartitionKeyOverrideValue` (custom key) |
| `splitOutsideQuotes` | same | via the two above | via the two above |
| `lookupPerTableFlagTables` | same | `resolveExcludeColumnList` | `resolveCdcPartitionKeyOverrides` |
| `perTableFlagProblemsError` | same | `finalizeColumnListForLiveMigration` | `resolveCdcPartitionKeyOverrides`, `validateAndFinalizeCDCPartitionKeysPerTable` |
| `sqlname.FindBestMatchingColumnName` | `src/utils/sqlname/column_match.go` | via `resolveTableColumns` | via `resolveTableColumns`; also `tdb.FindBestMatchingTargetColumnName` elsewhere in import |

**Kept per flag:**

- **Exclude list:** partition roots, export membership, primary keys and unsupported datatypes.
- **Overrides:** the `pk` / `table` / `(cols)` values, and the import-table-list check.

**Effect on `--cdc-partition-key-overrides`:**

- **Quoted names:** a double-quoted name can now contain `;`, `:` or `,`.
- **Rejected values:** unbalanced quotes and separator-only values such as `" ; ;"` now fail.
- **Table problems reported together:** all of them, sorted, instead of only the first in map order.
- **Error format:** messages now take the form `invalid --cdc-partition-key-overrides:\n  <table>: <problem>`.
- **Custom key columns:** a missing or ambiguous column now reads `<table>: column "x" does not exist` (or `is ambiguous`), with all tables reported together, sorted. The old message's `(available columns: ...)` list is gone, and a column listed twice in `(…)` is now reported.

`sqlname.FindBestMatchingColumnName` is the logic that used to be `AttributeNameRegistry.FindBestMatchingColumnName`. It depended only on the database type. The error types `ErrAmbiguousColumnName` and `ErrColumnNameNotFound` moved with it; `tgtdb` keeps type aliases so existing `errors.As` checks are unchanged.

## 7\. Data model

### 7.1 Persisted (`metadb.MigrationStatusRecord`)

```go
ExcludedColumns ExcludedColumnsRecord `json:"ExcludedColumns"`

type ExcludedColumnsRecord struct {
	FromSource map[string][]string `json:"FromSource"` // set only by the source exporter
	FromTarget map[string][]string `json:"FromTarget"` // set only by the target exporter; its own flag only
}
```

- Keys are root tables as `NameTuple.ForKey()`; values are catalog column names, sorted.
- A nil map means that exporter has not recorded a set (not started, or started by a voyager version without the flag). An empty map means it ran with no exclusions.
- `FromTarget` does not repeat `FromSource`. The effective target-side set is their union, recomputed each run (§5.6), so each fact is stored once.

### 7.2 In memory (`cmd/exportData.go`)

| Type | Holds |
| :---- | :---- |
| `map[string][]string` | A table, written as in a flag value, to its columns: the parsed flag, the suggested values, and report rows. |
| `exportTableColumnMetadata` | The exported tables (roots and leaves), the leaf-to-root map, each table's column names, primary-key columns, and the supported and unsupported columns. `rootOf` looks up a table's root; `addPrimaryKeyToRootTableIfRequired` gives a keyless partitioned root its partitions' key (§5.5). |
| `unsupportedColumnsReport` | Leftovers from §5.6: `primaryKeyTables` and `otherTables` (root table → columns), `tablesWithNoColumns`. |

## 8\. Migration-flow matrix

| Flow | Source exporter | Target exporter |
| :---- | :---- | :---- |
| Offline | Flag rejected. Unsupported columns follow the old path. | n/a |
| Live (snapshot + changes) | Flag applies to streaming. Snapshot copies every column. | n/a |
| Changes-only | Same as live; there is no snapshot, so excluded columns are never copied. | n/a |
| Live with fall-back | As live. | Inherits `FromSource` from the MSR; own flag only for YB-connector-only columns. Effective set is `FromSource` ∪ own (§5.6, §5.11). Checked against the YB connector in use. |
| Live with fall-forward | As live. | Same as fall-back, towards the replica. |
| Iterative cutover | Each iteration's source exporter gets `FromSource` through the spawned command line and the copied MSR. | `FromTarget` is copied with the MSR. The spawned target exporter does not get a flag on the command line in CLI mode (§14). |

Oracle and MySQL sources: the flag is rejected and every flow behaves as before.

## 9\. Serialized state and compatibility

| Surface | Change | Compatibility |
| :---- | :---- | :---- |
| MSR JSON | New `ExcludedColumns` sub-struct. | Additive. Older binaries ignore it. A record written by an older binary reads as nil maps, so the first run of the new binary records its set. |
| Config file | New `exclude-column-list` key under `export-data`, `export-data-from-source` and `export-data-from-target`. | Additive. |
| Debezium config | `column.include.list` may now list columns explicitly for a table that used to get `*`. | Same property, different value. |
| Assessment, callhome | None. | Unchanged. |

**Behaviour change for existing users (release note).** For PostgreSQL sources and for `export data from target`:

- Unsupported-datatype columns are no longer skipped after a prompt. Scripts that answered the prompt with `yes |` fail until they pass the flag.
- A table with an unsupported primary-key column must be in `--exclude-table-list`.
- With fall-back or fall-forward, `export data from target` fails on columns the YB connector cannot stream (for example `TSQUERY`) until they are in its own `--exclude-column-list`.

Oracle and MySQL are unchanged.

## 10\. Failure modes

| Situation | Behaviour |
| :---- | :---- |
| Excluded column has NOT NULL on the receiving side | Rows inserted during streaming fail on import. Not detected by export; every exclusion message tells the user to drop the constraint. |
| Primary-key column passed in the flag | Rejected by the check (§5.5) instead of crashing Debezium's transformer. |
| Catalog or primary-key query fails | Export fails with `get columns of tables: ...` or `get primary key columns of tables: ...`. |
| MSR missing when recording or reading the source set | Error. It should exist by this point in export; continuing would lose the resume guard. |
| Partition attached after the first run | Gets its root's exclusions on the next run, because leaves are recomputed every run. |
| Column renamed or dropped on the source between runs | The saved name no longer matches; the check reports "does not exist". The user must restart with `--start-clean`. |
| Column name containing `"` (written `"x""y"`) | Not supported. The matcher strips the outer quotes but does not un-escape `""`, so the name is reported as not existing. Live migration already fails on such columns: the importer quotes column names with Go's `%q` (`AttributeNameRegistry.QuoteAttributeName`), producing `"x\"y"`, which PostgreSQL rejects. |

## 11\. Performance

Not on a hot path. The new path runs once per `export data` start: one catalog query for columns, one for primary keys, and in-memory checks over the table list. Nothing runs per row or per change event. Debezium's own column filtering is unchanged.

## 12\. Testing

| Layer | Test | Covers |
| :---- | :---- | :---- |
| Parsing | `TestParseExcludeColumnList` (`cmd`, unit) | Qualified and bare tables; quoted names containing `;`, `:` and `,`, kept as typed; missing or extra `:`; empty table, column or list; duplicate columns; unbalanced quotes; separator-only values. |
| Round trip | `TestExcludeColumnListFlagValueRoundTrips` (`cmd`, unit) | A recorded map renders to flag syntax and parses back to the same entries. |
| Resolution | `TestResolveExcludeColumnList` (`cmd`, unit) | Missing table; same table under two spellings. |
| Checks | `TestValidateExcludedColumns` (`cmd`, unit, fake table lookup) | Missing table, missing column, leaf partition named, table not exported, primary key with and without unsupported type, case-insensitive matching, lowercase preference, ambiguous name, duplicate after matching. |
| Apply | `TestApplyExcludedColumns` (`cmd`, unit) | Excluded unsupported columns pass; root exclusions reach leaves; `*` becomes an explicit list; all columns excluded; key-column and other leftovers. |
| Error text | `TestBuildUnsupportedColumnsError` (`cmd`, unit) | Exact messages for source and target roles, merged suggestions. |
| Resume guard | `TestRecordExcludedColumns` (`cmd`, unit, temp metaDB) | First run saves the set sorted; same set in another order passes; a different set fails with the exact message and leaves the record unchanged; a run without the flag saves an empty set; the target exporter saves `FromTarget` without touching `FromSource`. |
| Keyless partitioned root | `TestExcludeColumnsOnPartitionedRootWithoutPrimaryKey` (`cmd`, unit) | The root takes its partitions' key; excluding a column of that key is rejected; an unsupported column in it is reported as a primary-key table. |
| Target merge | `TestHandleExcludedColumnForTargetExporter` (`cmd`, unit, temp metaDB) | `FromSource` is added per root table and leaves are skipped; after the merge only a `tsquery` column the source did not exclude is left as unsupported. |
| Matcher | `TestFindBestMatchingColumnName` (`sqlname`, unit) | Exact, quoted, case-insensitive, lowercase (PG/YB) and uppercase (Oracle) preference, ambiguous, not found. |
| Shared guardrails | `TestParseCdcPartitionKeyOverrides`, `TestResolveCdcPartitionKeyOverrides` (`cmd`, unit) | Shared parser and lookup through the import flag, including quoted separators and aggregated table problems. |
| Catalog | `TestPostgresGetTableColumns` (`srcdb`, integration) | Column names in definition order for a case-sensitive schema and table, with a dropped column, a reserved-word column, a domain and an array column; partition root and leaf. |
| End to end | migtest `pg/datatypes` | `EXPORT_EX_COLUMN_LIST` lists every unsupported column of the test schema; passed by `export_data` in `migtests/scripts/functions.sh` and by the live config templates. |

**Gaps:**

- **No YugabyteDB integration test** for `GetTableToColumns`.
- **No unit test that the source exporter skips the `FromSource` merge.** That role check is in `finalizeColumnListForLiveMigration`, which needs a database.
- **No end-to-end test for `export data from target`:**
  - `TSQUERY` failing in reverse and passing once listed
  - a source-excluded column not being NULLed on the source after an UPDATE on the target
  - the resume guard on the target exporter
- **No migtest asserts the streaming outcome:** excluded columns keeping their snapshot values, and NULL on inserted rows.
- **gRPC-connector migtests:** the `pg/datatypes` fall-back and fall-forward jobs run in CLI mode with `USE_YB_GRPC_CONNECTOR=true`. They need a target-side exclusion that cannot be passed today (§14).

## 13\. Alternatives considered

- **`schema.table.column` list format.** Repeats the table for every column and is ambiguous with quoted names that contain dots. Rejected in favour of `<table>:<columns>;...`, which matches `--cdc-partition-key-overrides`.
- **Keep the prompt and add the flag as an override.** Leaves the silent `yes |` path in place. Rejected.
- **Make `pg_dump` honour the column list.** Would also keep excluded columns out of the snapshot, but `pg_dump` has no column filter and the snapshot path would need its own mechanism. Out of scope for now.
- **Ignore entries for tables voyager skipped (empty, unsupported).** Proposed in the plan so that a config does not fail depending on data. Not implemented; such entries are currently errors (§14).
- **PG identifier rules for columns (unquoted lowercased).** Replaced by best-match matching shared with import, so users do not need to know a column's exact case unless two columns differ only in case.
- **Reach `GetTableToColumns` by type assertion instead of the `SourceDB` interface.** Avoids `panic` stubs on Oracle and MySQL, but adds a one-method interface in `cmd`. The interface method was chosen.
- **Keep the user's entry order (a slice of table/columns pairs).** Would let the suggested value follow the order the user typed. A `map[string][]string` was chosen: the same shape as the import flag's parsed value, with problems and suggestions sorted by table name.
- **Show each unsupported column's datatype in the error.** Needed a `format_type` column in the catalog query and a typed column list. Dropped to keep one plain column list.
- **Two top-level MSR fields.** Replaced by one `ExcludedColumns` sub-struct, per the MSR grouping convention.
- **Pass `FromSource` to the spawned `export data from target` as `--exclude-column-list`.** Duplicates the MSR and would make the target exporter save the source's columns as its own, so a later restart that adds the extra YB-only columns would fail the resume check. Rejected; the target exporter reads `FromSource` from the MSR instead (§5.11).

## 14\. Open questions

1. **Extra YB-only columns for the spawned target exporter (§5.11).** Columns excluded on the source are already inherited; only the YB-connector-only ones are at issue. Options:
   - **Cutover-time check (proposed).** `initiate cutover to target` with fall-back or fall-forward fails early with the suggested target value, and can take a value for CLI users to save in the MSR and pass to the spawned command.
   - **Keep the manual restart in CLI mode.** The target exporter fails at startup with the exact value, and the user restarts it with the flag.
   - **For CI now:** run the gRPC `pg/datatypes` fall-back and fall-forward migtests in config-file mode with `EXPORT_FROM_TARGET_EX_COLUMN_LIST` set. This needs no product change.
2. **Column names containing `"`.** Treated as unsupported (§10). Fixing it needs both the matcher to un-escape `""` and the importer's `%q` quoting in `QuoteAttributeName` to double the quote instead; the latter is a separate, pre-existing import bug.
3. **Entries for tables voyager skipped.** Should they be ignored with a message (the plan) rather than fail as "not part of this export"?
4. **Status output.** Show the excluded columns in `export data status`.
5. **Catch it earlier.** `assess-migration` already reports `unsupportedDataTypesForLiveMigrationWithFForFB`. Should it also print suggested `exclude-column-list` values for both exporters?
6. **Import-time check.** Fail `import data` early when an excluded column is NOT NULL without a default on the receiving side?
7. **File variant.** Is an `--exclude-column-list-file-path` needed? Proposed: no, for now.
8. **Issue text.** Issue \#3911 still describes `|` separators and automatic skipping; update it to match this spec.
