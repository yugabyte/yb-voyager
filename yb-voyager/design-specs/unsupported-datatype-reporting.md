# Unsupported datatype reporting across analyze-schema, assess-migration and export data

|  |  |
| :---- | :---- |
| **Status** | Draft |
| **Author** | Priyanshi Gupta |
| **Implementation** | [\#3897](https://github.com/yugabyte/yb-voyager/pull/3897) (export data) → [\#3898](https://github.com/yugabyte/yb-voyager/pull/3898) (analyze-schema, assess-migration) |
| **Contractual** | §3 type resolution, §4 unsupported type sets, §5 per-phase behaviour, §6 consistency contract, §7 flow matrix. Everything else is advisory. |

## 1\. Context

A PostgreSQL column is unsupported in a migration flow when its type, or any type it is built from, cannot be migrated in that flow. "Built from" matters because users rarely use the problem type directly. They wrap it:

```sql
CREATE DOMAIN lsn_dom AS pg_lsn;                 -- domain over an unsupported type
CREATE DOMAIN lsn_arr_dom AS pg_lsn[];           -- domain over an array of one
CREATE TYPE address AS (street text, loc point); -- composite with an unsupported field
CREATE TABLE t (a lsn_dom, b lsn_arr_dom[], c address);
```

Voyager reports these columns in three places, at three different points of the migration:

| Phase | Question it answers | Input | Output |
| :---- | :---- | :---- | :---- |
| `analyze-schema` | Which columns in the exported schema will not migrate? | Exported DDL files | Schema analysis report issues |
| `assess-migration` | Same question, before anything is exported | Schema dumped during assessment, then analyzed | Assessment report issues and callhome |
| `export data` | Which columns must be left out of this export? | Live source catalog | Column include list and a confirmation prompt |

Before this work each phase decided "unsupported" differently. Only the declared type was checked, so all three columns in the example above were missed. Assessment for PG also had a third path that read column types from a catalog CSV (`table-columns-data-types.psql`) and reported them a second time.

**Requirements**

- A column is unsupported in a flow if any type in its type tree is unsupported in that flow. The tree follows domains to their base type, arrays to their element type and composites to their fields, recursively.
- `analyze-schema` and `assess-migration` agree exactly for PostgreSQL: assessment shows what analyze reports and nothing else.
- `export data` skips exactly the columns that the earlier phases warned about for that flow, with the exceptions listed in §6.2.
- Each column is reported on the table that declares it, once for every distinct unsupported type it is built from.
- Case-sensitive schema, table, column and type names work in every phase.

**Non-goals**

- Changing which types are unsupported. The per-flow lists in `srcdb` stay as they are.
- Oracle and MySQL catalog paths. Oracle keeps its own catalog path in assessment and export (§5.2.3). The analyze-schema detectors are source-agnostic, though, so Oracle and MySQL analyze-schema also pick up the new walk (§5.1.4).
- Import-side value conversion. §6.2 lists where it constrains export, but this design does not change it.
- The deprecated `UnsupportedDataTypes` and datatype `MigrationCaveats` fields of the assessment report. For PG they are no longer filled (§8).

## 2\. Where it sits

```
                   srcdb lists (§4): PostgresUnsupportedDataTypes,
                   PostgresUnsupportedDataTypesForDbzm, Yugabyte...ForDbzm{Logical,Grpc}
                        │                                   │
     ┌──────────────────┴─────────────┐          ┌──────────┴──────────────────────┐
     │ src/query/queryissue           │          │ src/srcdb                       │
     │  column_datatypes.go           │          │  pg_column_types.go             │
     │   GetUnderlyingTypes  (parser) │          │   columnUnderlyingTypesQuery    │
     │   GetColumnDatatypeSupport     │          │   (recursive catalog walk)      │
     │  detectors_ddl.go              │          │  postgres.go / yugabytedb.go    │
     │   TableIssueDetector           │          │   GetColumnsWithSupportedTypes  │
     │   ForeignTableIssueDetector    │          └──────────┬──────────────────────┘
     └──────────────┬─────────────────┘                     │
                    │ QueryIssue                            │ supported / unsupported
                    ▼                                       │ columns per table
     cmd/analyzeSchema.go                                   ▼
      convertIssueInstanceToAnalyzeIssue          cmd/exportData.go
                    │ utils.AnalyzeSchemaIssue     handleUnsupportedColumnsInExportData
                    ▼
     cmd/assessMigrationCommand.go
      addPGUnsupportedDatatypeIssuesFromSchemaAnalysisReport
                    │ AssessmentIssue
                    ▼
     assessment report (JSON/HTML), callhome_payload_builder.go
```

There are two type walks. They exist for different reasons and are not merged:

| Walk | Used by | Sees | Why it must be this one |
| :---- | :---- | :---- | :---- |
| Parser (`GetUnderlyingTypes`) | analyze-schema, assess-migration | `CREATE DOMAIN` / `CREATE TYPE` in the exported schema | analyze-schema runs offline on files and has no source connection. |
| Catalog (`columnUnderlyingTypesQuery`) | export data | `pg_type` / `pg_attribute` of the live source or YB target | Export must match the database as it is now, including types outside the exported schemas and drift since `export schema`. |

The type **lists** are shared. Only the way a column's type tree is found differs. §6 states where the two walks are expected to agree and where they are known not to.

## 3\. Type resolution

### 3.1 Type tree

A column's type tree is its declared type plus every type it is built from:

| Node | Children |
| :---- | :---- |
| Domain | its base type |
| Array (`T[]`, or a domain over one) | its element type `T` |
| Composite | the type of each field |
| Enum | none |
| Base, extension or range type | none |

The walk is depth-first and keeps a visited set, so a type reached twice is listed once, and a cycle through composites terminates. The declared type comes first. A flow reports every distinct type in the tree that it does not support (§4). A domain over `pg_lsn` is reported as `pg_lsn`, not as the domain. A composite with an `xml` and a `pg_lsn` field gets both issues, so one run shows every blocker.

### 3.2 Parser walk (`queryissue.GetUnderlyingTypes`)

```go
type TypeKind int // TypeKindOther, TypeKindDomain, TypeKindComposite, TypeKindEnum, TypeKindRange

type UnderlyingType struct {
    Name string   // qualified for types defined in the exported schema
    Kind TypeKind
}

func (p *ParserIssueDetector) GetUnderlyingTypes(typeName string) []UnderlyingType
func (t UnderlyingType) BaseName() string // unqualified, no "[]", canonical spelling
```

- **Inputs.** `ParseAndProcessDDL` records every `CREATE DOMAIN` and `CREATE TYPE` before any `CREATE TABLE` is checked. The object-type order of the first pass guarantees this: SCHEMA, COLLATION, EXTENSION, TYPE, DOMAIN, SEQUENCE, TABLE, ....
- **Name lookup.** A name is looked up exactly as written and is never qualified with a schema. pg_dump sets `search_path` to empty and qualifies every user type, and PG searches `pg_catalog` first. So an unqualified name is a built-in, even when the table's schema defines a domain or type of that name. For example, `x xml` next to `CREATE DOMAIN public.xml` is the built-in XML.
- **Types not in the schema.** Built-ins, extension types (`geometry`, `lo`, `hstore`) and types from schemas that were not exported come out as `TypeKindOther`, named as written. They are never walked further.
- **List matching.** Only `TypeKindOther` types are matched against the lists, by `BaseName()`. A domain, composite or enum defined in the schema never matches by its own name, so `public.point` or `public.xml` is judged by what it is built from.

### 3.3 Catalog walk (`srcdb.columnUnderlyingTypesQuery`)

One recursive CTE per export, over every table in the export list:

- Domain: `typtype = 'd'` → `typbasetype`.
- Array: `typtype = 'b' AND typcategory = 'A'` → `typelem`. It checks `typtype = 'b'` because a domain over an array copies the array's `typcategory`.
- Composite: `typtype = 'c'` → `atttypid` of each live field of `typrelid`.
- A `visited` array stops cycles. `MIN(depth)` per type removes duplicates.

Each row carries `typname` and `typtype`. Composite and range names are schema-qualified, so those user types cannot collide with a built-in name. Domains, enums and user base types come back as a bare `typname` and can collide (§6.2 D5). A column is unsupported if any row matches the name list (case-insensitively) or the flow's `typtype` list (§4.2).

## 4\. Unsupported type sets

### 4.1 Lists

All lists live in `src/srcdb` and are the single source of truth for both walks.

| Flow | List | Notes |
| :---- | :---- | :---- |
| Offline | `PostgresUnsupportedDataTypes` | Types YugabyteDB cannot store, e.g. `pg_lsn`, `xml`, `geometry`, `lo`, multiranges. |
| Live (PG source, Debezium) | `PostgresUnsupportedDataTypesForDbzm` (`GetPGLiveMigrationUnsupportedDatatypes`) | Superset of offline. Adds geometric types, `vector`, `timetz`, `cid`, `refcursor` and the built-in ranges (`int4range`, `int8range`, `numrange`, `tsrange`, `tstzrange`, `daterange`). |
| Live with fall-forward / fall-back | `GetPGLiveMigrationWithFFOrFBUnsupportedDatatypes` | `YugabyteUnsupportedDataTypesForDbzmLogical` minus the live list, i.e. today `tsquery`. |
| Export from YB (logical connector) | `YugabyteUnsupportedDataTypesForDbzmLogical` | Used by the FF/FB exporter. |
| Export from YB (gRPC connector) | `YugabyteUnsupportedDataTypesForDbzmGrpc` | Adds `hstore`, `citext`, `ltree`, `tsvector`. |

### 4.2 Kind-based rules

Some types are unsupported by kind rather than by name:

| Kind | analyze / assess | export data |
| :---- | :---- | :---- |
| Composite | Never unsupported by itself: only its fields are checked (D2). | Skipped only by the YB gRPC exporter (`typtype = 'c'`). |
| Range (`typtype = 'r'`) | Live issue `UNSUPPORTED_DATATYPE_LIVE_MIGRATION_RANGE`. Built-in ranges match by name; a user-defined range is `TypeKindRange` (D1). | Skipped by every Debezium export (PG and YB): the importer's value converter cannot handle ranges. |
| Multirange (`typtype = 'm'`) | Built-in multiranges by name, with their own issues. The multirange created with a user-defined range is `TypeKindRange` and gets the range issue. | Skipped by `typtype`, which also covers user-defined multiranges. |
| Enum | Leaf. Never unsupported by itself. | Same. gRPC: tables with arrays of enums or UDTs are dropped whole by `FilterUnsupportedTables`. |

The remaining differences are listed in §6.2.

The parser learns user-defined ranges from `CREATE TYPE ... AS RANGE`. It also records the multirange type PostgreSQL creates with each one: the `multirange_type_name` option if given (pg_dump always writes it), else the name PostgreSQL derives by inserting `multi` before the first `range`, or appending `_multirange`.

## 5\. Behaviour by phase

### 5.1 `analyze-schema`

#### 5.1.1 Detection

`TableIssueDetector.DetectIssues` checks each column of a `CREATE TABLE`. For a typed table (`CREATE TABLE t OF type`), the columns are the fields of that composite:

```go
support := d.GetColumnDatatypeSupport(col.GetFullTypeName(), table.SchemaName)
// support.UnsupportedInOffline        → ReportUnsupportedDatatypes(...)
// support.UnsupportedInLive
//      Kind == Range                  → NewRangeLiveMigrationDatatypeIssue(..., type name, column)
//      otherwise                      → ReportUnsupportedDatatypesInLive(...)
// else support.UnsupportedInLiveWithFFOrFB → ReportUnsupportedDatatypesInLiveWithFFOrFB(...)
```

`GetColumnDatatypeSupport` returns, for each flow, every distinct type in the tree that the flow does not support. Types not defined in the exported schema are deduplicated by base name, so `xml` and `pg_catalog.xml` in one tree raise one issue. Under FF/FB only the outermost composite is reported: a composite nested in it is part of the same value. The FF/FB issue is raised only when the column is supported in plain live migration, because an unsupported live column already blocks FF/FB.

`ForeignTableIssueDetector` checks offline support only, since live migration does not stream foreign tables.

`ReportUnsupportedDatatypes*` map a base name to its issue constructor. A name present in a list but missing from the switch exits with `Unrecognized unsupported data type`. Adding a type to a list therefore requires adding its issue in the same change.

#### 5.1.2 What is reported where

| Object | Reported? | Reason |
| :---- | :---- | :---- |
| Regular table | Yes, per column. | |
| Partitioned table (root) | Yes, per column. | |
| Intermediate or leaf partition, as pg_dump writes it | Yes, per column. | pg_dump writes `CREATE TABLE` with the full column list, then `ALTER TABLE ONLY root ATTACH PARTITION`. The issue repeats on every partition. |
| Partition written as `PARTITION OF` | No. | No column list in the DDL. Only seen in hand-written schemas, not in pg_dump output. |
| Typed table (`CREATE TABLE t OF type`) | Yes, per field of the composite. | pg_dump keeps the `OF` form, whose DDL lists only column options. A composite outside the exported schema reports nothing (D4). |
| Inheritance parent | Yes, per column. | |
| Inheritance child | Only for columns it declares itself. | Inherited columns are reported on the parent. The table also gets the separate `INHERITANCE` issue. |
| Foreign table | Offline issues only. | §5.1.1. |
| View, materialized view | No. | A view stores no data. Its source tables carry the issue. |
| Function arguments and return types, `ALTER TABLE ... ADD COLUMN` | No. | Out of scope, unchanged by this design. |

Each issue is a **TABLE**-level issue. `ObjectName` is the qualified table name and `SqlStatement` is the `CREATE TABLE`. The offending type is in the issue type, name and description. The column is in the description and in `Details.ColumnName`, which every datatype issue constructor (offline, live and FF/FB) sets.

#### 5.1.3 Categories and filtering

`convertIssueInstanceToAnalyzeIssue` sets the category:

| Issue types | Category |
| :---- | :---- |
| Offline datatype issues (`UNSUPPORTED_DATATYPE_*`) | `UNSUPPORTED_DATATYPES_CATEGORY` |
| `queryissue.UnsupportedDatatypesInLiveMigrationIssues` | `MIGRATION_CAVEATS_CATEGORY` |
| `queryissue.UnsupportedDatatypesInLiveMigrationIssuesWithFForFBIssues` | `MIGRATION_CAVEATS_CATEGORY` |

`GetDDLIssues` then removes issues fixed in the target YB version, and `finalizeIssues` drops a live issue when the offline issue for the same type is present on the same object. A `pg_lsn` column therefore shows one issue, not two.

#### 5.1.4 Oracle and MySQL sources

`TableIssueDetector`, `ForeignTableIssueDetector`, the `CREATE DOMAIN` parsing and `CompositeFields` do not check the source type. analyze-schema on the PG DDL that ora2pg produces for Oracle and MySQL therefore also walks domains, arrays and composite fields, and reads the column type with `GetFullTypeName()` instead of `TypeName`. For Oracle this matters because ora2pg writes object types as composites, whose fields are now checked. The lists matched are still the PG lists, as before. There is no Oracle or MySQL analyze test for this yet (§11).

### 5.2 `assess-migration`

#### 5.2.1 PostgreSQL

Assessment dumps the schema and runs analyze-schema on it. Datatype issues are taken straight from that report:

```go
func addPGUnsupportedDatatypeIssuesFromSchemaAnalysisReport(schemaAnalysisReport utils.SchemaReport) {
    for _, analyzeIssue := range schemaAnalysisReport.Issues {
        if analyzeIssue.IssueType == UNSUPPORTED_DATATYPES_CATEGORY ||
            slices.Contains(queryissue.UnsupportedDatatypesInLiveMigrationIssues, analyzeIssue.Type) ||
            slices.Contains(queryissue.UnsupportedDatatypesInLiveMigrationIssuesWithFForFBIssues, analyzeIssue.Type) {
            assessmentReport.AppendIssues(convertAnalyzeSchemaIssueToAssessmentIssue(analyzeIssue))
        }
    }
}
```

There is no PG catalog query for column types. `table-columns-data-types.psql` is deleted. The assessment CSV loader globs for files, so an export dir from an older version that still has the CSV keeps working, and a new export dir without it is fine.

**Invariant:** for PostgreSQL, the set of datatype issues in the assessment report equals the set of datatype issues in the analyze-schema report of the same schema.

#### 5.2.2 Report and callhome

- Issues keep their analyze category. Offline issues are under "Unsupported datatypes", and live and FF/FB issues are under "Migration caveats".
- `ObjectType` is `TABLE` and `ObjectName` is `schema.table`. The column name is in the description and in `Details.ColumnName`. The HTML report shows it as a "More Information" row.
- Callhome anonymises `object_name` by object type: `AnonymizeQualifiedTableName` for PG (TABLE), `AnonymizeQualifiedColumnName` for Oracle (COLUMN). PG rows also carry the anonymised `CREATE TABLE` in `sql_statement`, once per affected column. Callhome does not send `Details`, and `ColumnName` is a sensitive key in any case.

#### 5.2.3 Oracle

Unchanged. `fetchOracleColumnsWithUnsupportedDataTypes` reads the gathered catalog CSV, fills the deprecated `UnsupportedDataTypes` field, and `addOracleAssessmentIssuesForUnsupportedDatatypes` raises COLUMN-level issues against `OracleUnsupportedDataTypes`. There is no ora2pg-parsed DDL to walk at that point, so a parser path is not an option.

### 5.3 `export data`

#### 5.3.1 When it runs

`getFinalTableColumnList` calls `GetColumnsWithSupportedTypes(finalTableList, useDebezium, changeStreamingIsEnabled(exportType))` after table filtering.

- **Offline with `pg_dump`.** Returns empty maps, so every column is exported. Offline-unsupported types are a target-side problem: analyze and assess report them, and `import schema` or `import data` fails on them.
- **Debezium (live, or offline with Debezium).** Runs the catalog walk (§3.3) against the database the exporter is connected to.

#### 5.3.2 Lists by exporter role

| Exporter | Name list | `typtype` list |
| :---- | :---- | :---- |
| Source (PG) | `PostgresUnsupportedDataTypesForDbzm` | `r`, `m` |
| Target for FF/FB, logical connector | `YugabyteUnsupportedDataTypesForDbzmLogical` | `r`, `m` |
| Target for FF/FB, gRPC connector | `YugabyteUnsupportedDataTypesForDbzmGrpc` | `r`, `m`, `c` |

#### 5.3.3 Output

- Each table maps to its supported columns, or `["*"]` when every column is supported.
- If any column is unsupported, `handleUnsupportedColumnsInExportData` lists them per table and asks the user to confirm. On yes, the export continues without them and warns the user to drop NOT NULL on those columns in the importing database. On no, it exits and points to `--exclude-table-list`.
- A table with no supported columns left is removed from the export.

#### 5.3.4 Tables in the list

The export table list is what Debezium streams, so for partitioned tables it holds the leaf partitions. The catalog walk works on leaves directly, because every partition has its own `pg_attribute` rows with the root's types. Case-sensitive names go through `NameTuple.ForCatalogQuery()` and come back through a lookup keyed by `AsQualifiedCatalogName()`. A row for a table that was not requested is an error.

## 6\. Consistency contract

### 6.1 Must agree

For a schema where every type a column uses is defined in the exported schemas, and for the PG source exporter:

> A column appears as a live-migration datatype issue in analyze/assess **if and only if** export data from the source skips it.

Both sides use the same list, both walk domains, arrays and composites, and both stop at enums. Tests that change either walk must keep this true for:

- a domain over an unsupported type, and a domain over a domain over one
- an array of an unsupported type, a domain over that array, and an array of that domain
- a composite with an unsupported field, and a composite nesting that composite
- a built-in range, a user-defined range, its multirange, and a domain or composite over a range
- each of the above with case-sensitive schema, type, table and column names
- a partitioned root with a multi-level hierarchy, where analyze reports it on the root and on every partition, and export skips it on every leaf

### 6.2 Known divergences

| \# | Case | analyze / assess | export data | Proposed resolution |
| :---- | :---- | :---- | :---- | :---- |
| D1 | Built-in or user range column (`int4range`, `tsrange`, a `CREATE TYPE ... AS RANGE`) | Was not reported. | Skipped in every Debezium export. | **Fixed.** Ranges and user-defined multiranges are reported as `UNSUPPORTED_DATATYPE_LIVE_MIGRATION_RANGE`. |
| D2 | Composite column, FF/FB with logical connector | Was reported as user-defined type under FF/FB. | Not skipped. | **Fixed.** The logical connector is the default and streams composites. `UNSUPPORTED_DATATYPE_LIVE_MIGRATION_WITH_FF_FB_USER_DEFINED` is retired. A composite is reported only when the walk finds an unsupported type among its fields, as the field type's issue. |
| D3 | `hstore`, `citext`, `ltree`, `tsvector` with the gRPC connector | Not reported: FF/FB issues assume the logical connector. | Skipped only when the FF/FB target exporter uses the gRPC connector. The PG source exporter and the logical connector stream them. | Acceptable while logical is the default. Document it. |
| D4 | Domain or composite defined in a schema that was not exported, including the type of a typed table | Missed: the parser cannot walk it and sees only the outer name. | Skipped: the catalog sees every schema. | Accept. Export is the safety net. |
| D5 | User type named like a list entry, e.g. `CREATE TYPE public.point AS (...)` or `CREATE DOMAIN public.xml AS text` | Not matched: only types outside the exported schema are matched by name (§3.2). | Composite or range: not matched, because the name is qualified. Domain, enum or user base type: matched, because the catalog returns a bare `typname`. So a `public.xml` domain over `text` is skipped by export but not reported. | Fix in the catalog walk: match names only for `pg_catalog` and extension-owned types, as `typeOidsQuery` already does, or qualify every user type. Follow-up on #3897. |
| D6 | Schema drift after `export schema` | Reflects the exported files. | Reflects the live catalog. | By design. Drift is covered by `schema detect-drift`. |


## 7\. Migration-flow matrix

| Flow | Offline issue | Live issue | FF/FB issue | export data skips |
| :---- | :---- | :---- | :---- | :---- |
| Offline (`pg_dump`) | Blocks: import fails on the type. | n/a | n/a | Nothing. |
| Live (snapshot + changes) | Shown. Column can't be stored on YB. | Shown. Column is skipped. | n/a | PG Dbzm list, `r`, `m`. |
| Changes-only | Shown. | Shown. | n/a | Same as live. The skip list does not depend on the snapshot. |
| Live with fall-forward | Shown. | Shown. | Shown. | Source: PG Dbzm list. Target exporter: YB list for its connector. |
| Live with fall-back | Shown. | Shown. | Shown. | Same as fall-forward. |
| Iterative cutover | Shown. | Shown. | Shown. | Re-computed per iteration from the live catalog of the database being exported. |

analyze-schema and assess-migration cannot know which flow the user will pick, so they report every flow's issues. Only export data knows the flow, and it is the only phase that acts on it.

## 8\. Serialized state and compatibility

| Surface | Change | Compatibility |
| :---- | :---- | :---- |
| Assessment report JSON, `Issues[]` for PG datatypes | `ObjectType` `COLUMN` → `TABLE`. `ObjectName` `schema.table.column` → `schema.table`. `Details` now holds `{"ColumnName": ...}`. Rows removed: view columns, inherited columns (the parent still reports them), and live and FF/FB rows on foreign tables (§5.1.1). Rows added: a column built from several unsupported types gets one row per type, and each pg_dump-written partition repeats its root's rows. | No field added or removed; `Details` was already on the struct. Consumers that grouped by column must read it from `Details.ColumnName`. |
| Analyze-schema report JSON and HTML | Same `Details.ColumnName` on datatype issues. The HTML shows a "More Information" row. | Additive. |
| `UnsupportedDataTypes` (deprecated) | Empty for PG. Still filled for Oracle. | The field and its JSON tag stay. |
| `utils.TableColumnsDataTypes` | Loses fields only the PG CSV filled. | Serialised into the yugabyted payload (`AssessmentReportYugabyteD.UnsupportedDataTypes`). The removed fields were all `json:"-"`, so the wire shape is unchanged. |
| Assessment SQLite | PG no longer loads `table_columns_data_types`. | Old DBs with the table are read without error. |
| Callhome `assess_migration` payload | For PG datatype issues, `object_type` is `TABLE` instead of `COLUMN`, for the live and FF/FB rows too. `object_name` is an anonymised table name; it was empty before. `sql_statement` is now filled with the anonymised `CREATE TABLE`, repeated once per affected column. | Keys unchanged, values changed. Whether this needs a bump of `ASSESS_MIGRATION_CALLHOME_PAYLOAD_VERSION` (2.0) is open (§13). |
| Export data | Unchanged surface. | Column include lists are re-computed every run. |

## 9\. Failure modes

| Situation | Behaviour |
| :---- | :---- |
| Base name in a list but missing from the `ReportUnsupportedDatatypes*` switch | `utils.ErrExit("Unrecognized unsupported data type ...")`. A unit test should iterate every list and assert each name has a constructor, so this fails in CI rather than at a customer. That test does not exist yet (§11). |
| Catalog walk query fails | `export data` exits with `get columns with supported types: ...`. |
| Catalog returns a table that was not requested | Error. It means the `NameTuple` round-trip is broken, and guessing would skip the wrong columns. |
| Domain or type referenced but not defined in the exported schema | Treated as `TypeKindOther`. Not an error, because extension and cross-schema types are normal (D4). |
| Cyclic or self-referencing composites | Visited set in both walks. Each type is reported once. |

## 10\. Performance

None of this is on a hot path. The parser walk runs once per column during analyze, bounded by the depth of the type tree. The catalog walk is one query per `export data` start, over the whole table list. Neither runs per row or per change event.

## 11\. Testing

| Layer | Test | Covers |
| :---- | :---- | :---- |
| Type walk | `TestGetUnderlyingTypes` (`queryissue`) | Domains, stacked domains, arrays, nested composites, enums, ranges and their multiranges, default-schema resolution, case-sensitive names. No cycle case yet. |
| Detection | `TestDatatypeIssuesInPartitionAndInheritanceHierarchy` (`queryissue`) | Case-sensitive partitioned root with a cross-schema domain and composite. A pg_dump-style partition (`CREATE TABLE` plus `ATTACH PARTITION`) is reported. A `PARTITION OF` partition is not. Inheritance parent and children. |
| Detection | `TestDomainOverUnsupportedDatatypeThroughGetDDLIssues` (`queryissue`) | Unqualified `point`, `xml` and `lo` next to user types of those names get the built-in issues. Typed tables: case-sensitive names with column options, a nested composite, and a composite outside the exported schema. A column built from several unsupported types, including `xml` and `pg_catalog.xml`. A nested composite under FF/FB. |
| Index on user-defined types | `TestIndexOnUserDefinedTypes` (`queryissue`) | A user-defined range or multirange counts as a user-defined type for index and PK/UK issues, like a composite, since YugabyteDB rejects both as `user_defined_type`. An enum does not. |
| Parser | `TestProcessRangeType` (`queryparser`) | Range types and the derived or explicit multirange name. |
| Detection | Existing `parser_issue_detector_test.go` datatype cases | Per-type issue constructors, offline/live/FF-FB split, live issue suppressed by offline. |
| Detection | Unit test that every name in each list has an issue constructor (§9) | **Missing on the branch; to add.** |
| Detection | Oracle analyze-schema test with an ora2pg object type (§5.1.4) | **Missing on the branch; to add.** |
| Assessment | `cmd` unit test for `addPGUnsupportedDatatypeIssuesFromSchemaAnalysisReport` | Selects exactly the three issue groups. Keeps category, object and `Details`. **Missing on the branch; to add.** |
| Export | `srcdb` integration tests for `GetColumnsWithSupportedTypes` (PG and YB) | Domains over arrays, arrays of domains, user-defined ranges, cross-schema domains, case-sensitive names. |
| End to end | migtests `assessment-report-test`, `assessment-report-test-with-tdb`, `basic-assessment-report-test`, `osm`, `omnibus`, `adventureworks`, `pgtbrus`, datatypes callhome | Expected reports regenerated for TABLE-level PG issues with `Details.ColumnName`, including the basic HTML report. |

**Gaps:**

- No migtest runs export data against a partitioned table that has an unsupported column.
- No test asserts the §6.1 invariant across both walks. A table-driven test that feeds the same DDL to the parser and to a PG container, and compares the live issues with the skipped columns, would catch D1-type drift as soon as it is introduced.

## 12\. Alternatives considered

- **Keep the PG catalog CSV in assessment and add the parser on top.** Two sources of truth, double reporting, and analyze and assess disagreeing. Rejected.
- **Carry internal per-column details on assessment issues so assessment could rebuild the old per-column lists.** Adds a field only assessment reads, and it feeds the deprecated sections. Rejected.
- **Use the catalog walk for analyze too.** analyze-schema runs on files with no source connection. Rejected.
- **Use the parser walk for export data.** The exported schema can be stale or partial (D4, D6), and skipping the wrong columns loses data. Rejected.

## 13\. Open questions

1. Does changing what `object_name` holds for PG datatype issues require a callhome payload version bump?
2. Should `GetUnderlyingTypes` and `UnderlyingType` stay exported? Today only `queryissue` uses them.
3. Should views get an informational issue that points at the source table's column? Today they get nothing.
