# Schema drift detection: `schema detect-drift`

|  |  |
| :---- | :---- |
| **Status** | Draft |
| **Author** | Shivansh Gahlot |
| **Tracking** | [\#3617](https://github.com/yugabyte/yb-voyager/issues/3617) · [DB-21962](https://yugabyte.atlassian.net/browse/DB-21962) · keys and indexes: [\#3885](https://github.com/yugabyte/yb-voyager/issues/3885) · [DB-24176](https://yugabyte.atlassian.net/browse/DB-24176) |
| **Implementation** | [\#3811](https://github.com/yugabyte/yb-voyager/pull/3811) → [\#3812](https://github.com/yugabyte/yb-voyager/pull/3812) → [\#3813](https://github.com/yugabyte/yb-voyager/pull/3813) → [\#3814](https://github.com/yugabyte/yb-voyager/pull/3814) → [\#3817](https://github.com/yugabyte/yb-voyager/pull/3817) → [\#3827](https://github.com/yugabyte/yb-voyager/pull/3827) → [\#3873](https://github.com/yugabyte/yb-voyager/pull/3873) → [\#3877](https://github.com/yugabyte/yb-voyager/pull/3877) → [\#3878](https://github.com/yugabyte/yb-voyager/pull/3878) |
| **Contractual** | §3 public surface, §4 data model, §5 rules, §6 flow matrix. Everything else is advisory. |

## 1\. Context

Voyager already records point-in-time snapshots of the PostgreSQL source schema at migration milestones (`schemasnapshot`, [\#3614](https://github.com/yugabyte/yb-voyager/pull/3614), [\#3655](https://github.com/yugabyte/yb-voyager/pull/3655)) and can compute the table- and column-level differences between any two snapshots (`schemadiff`, [\#3648](https://github.com/yugabyte/yb-voyager/pull/3648)). Nothing yet turns that history into something a user can act on. When `import data` fails on a column the target does not have, or when a table added mid-migration silently never arrives, the user has no way to learn that the source schema moved, when it moved, or what to do about it.

This design adds the layer above the diff engine that owns *drift*: which snapshot pairs are comparable, what a change means for the migration in flight, and a command that writes the answer as a report.

**Requirements this design depends on**

- A user can run one read-only command against an export directory and get a report of every change to a table, column, key, check or index on the source since `export schema`, including changes since the last stored snapshot.  
- Every reported change carries a severity that answers "what does the migration do about this", and a corrective action.  
- Each change is attributed to the interval between two captures and labelled with what the migration was doing then.  
- The report is available as HTML for reading and JSON for tooling. The JSON shape is stable across releases.  
- Scoping mirrors the export commands: `--table-list` / `--exclude-table-list` and `--object-type-list` / `--exclude-object-type-list`.

**Non-goals for this version**

- Objects other than tables, columns, primary keys, unique constraints, unique indexes, checks, foreign keys and indexes. Coverage matches what `schemasnapshot` captures.  
- Sources other than PostgreSQL.  
- Detecting drift on the target, or on the source after cutover-to-target.  
- Applying, suggesting, or generating DDL. The report says what to do; the user does it.  
- Preventing drift (event triggers, locks). Tracked separately in [\#3681](https://github.com/yugabyte/yb-voyager/issues/3681).

## 2\. Where it sits

```
cmd/schema.go, cmd/detectDrift.go            flags, source connection, table universe,
        │                                      exit codes, report files
        │  ListSnapshots / LoadSnapshotByName / Capture(live)
        ├────────────────────────────────► src/schemasnapshot      (given)
        │                                    │  SnapshotHeader, SnapshotContent, labels,
        │                                    │  metadb table schema_snapshots
        │  BuildReport / RenderJSON / RenderHTML
        ▼
src/schema/schemadrift                        timeline, intervals, phase, severity,
        │                                     Impact & action, report model, renderers
        │  NewDiffer(Config{Scope}).Diff(prev, next)
        ▼
src/schemadiff                                Difference, DiffType, Scope   (given; Scope
                                              collapsed in #3811)
```

Each layer speaks its own vocabulary, and the boundary is where the reframing happens:

| Layer | Talks about | Does not know about |
| :---- | :---- | :---- |
| `cmd` | flags, the live source connection, the table universe, exit codes, file paths | intervals, severity, phases |
| `schemadrift` | captures on a timeline, comparable pairs, intervals, phase, severity, Impact & action | how snapshots are stored, how a diff is computed, flags |
| `schemadiff` | two `SnapshotContent` values and the `Difference` list between them | time, labels, the migration |
| `schemasnapshot` | capturing and persisting one snapshot | comparison |

`schemadrift` does no I/O beyond its embedded HTML template. Reading snapshots and writing report files belong to `cmd`.

**What the given layers provide.** `schemasnapshot` captures a `SchemaSnapshot` (a `SnapshotHeader` stored in metaDB columns plus a `SnapshotContent` blob) under a label from a fixed vocabulary: `export_schema`, `export_data_from_source_start`, `export_data_from_source_periodic`, `export_data_from_source_exit`. A capture that fails writes a placeholder header with no content so the moment still appears on the timeline. `schemadiff.Diff(a, b)` returns a sorted `[]Difference` over tables, columns, keys, checks and indexes, each tagged with a `DiffType` such as `COLUMN_TYPE_CHANGED`; `Differ` applies a `Scope` after diffing.

## 3\. Public surface

### 3.1 `schemadiff.Scope` (changed in \#3811)

```go
type Scope struct {
	Schemas     []string                   // the exact set to keep; matched against EITHER side's schema
	Tables      []schemasnapshot.ObjectRef // the exact set to keep; matched against the finding's anchor table
	ObjectTypes []ObjectType               // the exact set to keep; matched against the finding's ObjectType
}
```

One positive allow-list per dimension, each holding the **exact** set to keep. There is no exclude counterpart: an include and an exclude list together would leave "empty" ambiguous and admit combinations with no defined meaning. Resolving a user's `--exclude-*` flag into a keep-set is the caller's job because only the caller knows the full universe.

**Empty means empty, not "all".** An unfiltered run passes the whole universe explicitly. The alternative -- reading an empty list as "keep everything" -- makes empty carry two meanings, "the user did not filter" and "the user excluded everything", and the second is a legitimate request the caller would then have to intercept before the engine inverted it.

**`Schemas` matches EITHER side of the finding.** A table that moves between schemas has a different schema on each side, so a move out of the requested set must still be reported once -- the user needs to know a table left their scope. Matching only one side would either hide that move or invent a drop.

The schema filter is applied AFTER diffing, never by narrowing the snapshot content before it. Projecting each side down to the requested schemas first turns `public.orders` -> `sales.orders` into a `TABLE_DROPPED`, because side B no longer holds the table at all. The diff engine has to see both schemas to recognise the move; only then can the finding be judged in or out of scope.

**A finding with no anchor table passes the `Tables` filter.** A top-level object -- a view, a function, a sequence -- has no host table, so `--table-list` has nothing to say about it; `--object-type-list` is the dimension that selects object kinds. Dropping such findings because a table list was given would make drift disappear from the report silently, which is the worse failure for a tool whose job is to report it.

### 3.2 `schemasnapshot.LabelSourceLive` (added in \#3812)

```go
const LabelSourceLive = "source_live" // accepts no reason; never persisted
```

`Capture` validates its label against the known vocabulary, so the live read taken by `detect-drift` needs one. It is named for what the snapshot **is** -- a live read of the source -- rather than for the command that takes it, which is what makes it usable as the timeline identity directly: every point on the timeline is identified by its `Header.Label`, and nothing has to carry a second identity alongside it.

### 3.3 `schemadrift` inputs

`BuildReport` takes `schemasnapshot.SchemaSnapshot` values directly -- a header plus content, where `Content` is nil for a failed capture. There is no wrapper type: the timeline identity of each point is its `Header.Label`, so nothing needs adding to what the snapshot already carries.

```go
type DetectionConfig struct {
	Source        Source
	Snapshots     []schemasnapshot.SchemaSnapshot // oldest first; the live read, if any, is last
	Scope         schemadiff.Scope                // the exact sets compared; Comparing is rendered from it
	MigrationInfo MigrationInfo                   // what the migration status record says about the flow
}

type MigrationInfo struct {
	ExportType         string    // snapshot-only, snapshot-and-changes, changes-only; "" before export data records it
	CutoverRequestedAt time.Time // zero until cutover to target is requested
	FallForwardEnabled bool      // set once import data to source-replica has run
}
```

The complete input to `BuildReport`. Plain data, no connections or handles, so the assembler is testable with fixtures.

`MigrationInfo` is what the report needs from the migration status record to place each change in a timing (§5.3). `cmd` fills it from `ExportTypeFromSource`, `CutoverTimings.ToTargetRequestedAt` and `FallForwardEnabled` of the record in the metaDB the snapshots come from. `ExportType` is empty until `export data` records it, so an empty value means "unknown flow", not "offline".

`Comparing.Tables` and `Comparing.ObjectTypes` are rendered from `Scope`, which holds the exact sets compared, so they are not passed separately. The report does not say whether a list flag narrowed those sets: `Scope` cannot carry that, and the listed sets already tell a reader what "no drift" covers.

```go
func BuildReport(p DetectionConfig) (Report, error)
```

Walks `p.Snapshots` oldest-first, diffs each comparable pair, and assembles the report. Rules in §5.2. `Report.GeneratedAt` is stamped here from the wall clock rather than passed in: it describes the act of building the report, not the data being reported on. It returns an error for a finding whose identity is neither a table nor a table-scoped object: that is a new engine object kind the report does not know how to place, and emitting it with an empty object would read as a real finding on nothing. It also returns the error from `getDriftInfo` (§3.4) for a `DiffType` with no guidance.

### 3.4 `schemadrift` classification

```go
type Severity string

const (
	SeverityAdvisory            Severity = "advisory"
	SeverityPotentialImpact     Severity = "potential_impact"
	SeverityBreaksRecoverable   Severity = "breaks_migration_recoverable"
	SeverityBreaksUnrecoverable Severity = "breaks_migration_unrecoverable"
)
```

Severity answers "what does the migration do about this change", not "how alarming is the DDL". Vocabulary in §5.4.

```go
type DriftInfo struct {
	Severity Severity `json:"severity"`
	Impact   string   `json:"impact,omitempty"`
	Action   string   `json:"action,omitempty"`
}

type Timing string

const (
	TimingBeforeExportData            Timing = "before_export_data"
	TimingOfflineAfterExportStart     Timing = "offline_after_export_start"
	TimingLiveAfterExportStart        Timing = "live_after_export_start"
	TimingChangesOnlyAfterExportStart Timing = "changes_only_after_export_start"
	TimingAfterCutover                Timing = "after_cutover"
)

var infoByDiffType map[schemadiff.DiffType]map[Timing]DriftInfo
var fallBackRiskByDiffType map[schemadiff.DiffType]string // appended to Impact while a cutover with fall-back is still possible
var fallForwardActionByDiffType map[schemadiff.DiffType]string // appended to Action while fall-forward is in use
var afterCutoverInfo DriftInfo

type situation struct {
	Timing Timing
	// A cutover with fall-back can still happen: live or changes-only, before cutover,
	// and fall-forward not in use, since voyager refuses to enable both.
	// Adds the type's fall-back sentence to Impact.
	FallBackPossible bool
	// Fall-forward is in use and cutover has not happened. Adds the type's
	// fall-forward sentence to Action.
	FallForwardPossible bool
}

func getDriftInfo(t schemadiff.DiffType, s situation) (DriftInfo, error)
```

Severity and its explanatory note are one value, and each type's values for a timing are one `DriftInfo` in one map. Parallel maps keyed by `DiffType` would let them disagree silently, and an entry with a severity but no note would render a finding the report cannot explain.

The Impact and Action depend on the change type AND on when the change happened (§5.3), so `infoByDiffType` is keyed by both. Every type lists `before_export_data`, `offline_after_export_start` and `live_after_export_start` explicitly; `changes_only_after_export_start` is listed only where its text differs from live. Text shared across timings is one named `DriftInfo` referenced from each timing, never copied. `getDriftInfo` applies four rules:

1. `TimingAfterCutover`: `afterCutoverInfo`, the same advisory for every type: "This change was made after cutover to target. Data already migrated to the target is not affected." It has no Action and no fall-back or fall-forward sentence: after cutover the source is the fall-back side, whose drift is not tracked (§1).
2. Otherwise the type's entry for the timing. `TimingChangesOnlyAfterExportStart` with no entry of its own uses the `TimingLiveAfterExportStart` entry. No entry is an error naming the type and the timing: a new type must come with its guidance, and reporting it as advisory with no note would hide a change the report cannot explain.
3. When `situation.FallBackPossible`, the type's sentence in `fallBackRiskByDiffType`, if any, is appended to the Impact.
4. When `situation.FallForwardPossible`, the type's sentence in `fallForwardActionByDiffType`, if any, is appended to the Action. Every type but `TABLE_PARTITION_CHILDREN_CHANGED` has one: make on the source-replica any change the Action asks for on the target, and restart `import data to source-replica` whenever the Action restarts `import data`. It is worded that way so it also reads correctly where the Action is only a restart, a fix on the source, or a table exclusion.

Each type is one `DiffType`, so the type alone says what happened. Changes whose two directions need different guidance are separate types (`COLUMN_NOT_NULL_SET` / `COLUMN_NOT_NULL_DROPPED`, §5.4), not one type plus a direction read from the old and new values.

`DriftInfo` is what enriches a raw `schemadiff.Difference` into drift: what the change means for the migration in flight. It is exported and embedded in `DriftEntry` rather than copied field by field.

### 3.5 `schemadrift` renderers (\#3813)

```go
func RenderJSON(r Report) ([]byte, error)
func RenderHTML(r Report) ([]byte, error)
```

JSON is the indented marshalling of `Report`. HTML is a single self-contained page from an embedded template with no external assets and no JavaScript. Identifiers render with minimal quoting so a printed object path is valid, copy-pasteable SQL: `sales."MixedCase"` when needed, `public.orders` otherwise. `RenderHTML` returns an error rather than rendering a report it cannot display faithfully: an interval that no capture opens or closes, or an operation, severity, value type or object identity outside the vocabulary the renderer knows. An unknown capture label is the one exception: it only loses its timeline marker, as it only loses its phase in §5.3.

### 3.6 Command (\#3814)

```
yb-voyager schema detect-drift --export-dir <dir> \
    --source-db-user <u> --source-db-name <db> --source-db-schema <s>[,...] \
    [--source-db-host --source-db-port --source-db-password ...] \
    [--output-format html|json] \
    [--table-list <globs> | --exclude-table-list <globs>] \
    [--object-type-list TABLE,COLUMN,CONSTRAINT,INDEX | --exclude-object-type-list ...]
```

|  |  |
| :---- | :---- |
| Parent | new `schema` command for standalone schema tooling outside the export/import workflow |
| Maturity | Tech Preview: `[TECH PREVIEW]` leads the command help and the command's section in the four migration config templates |
| Source type | PostgreSQL only; any other value is an error, exit 1 |
| Output | `<export-dir>/reports/drift_analysis_report.html` and `.json`, overwritten on each run. `--output-format html` or `json` writes only that one; unset writes both |
| Exit codes | `0` the report was written, whether or not it found drift · `1` error (flags, connection, unreadable snapshot). A script reads drift from `summary.change_count` in the JSON report. |
| Config file | section `schema-detect-drift` with keys `log-level`, `output-format`, the four list flags; commented-out in the four migration templates |
| Table scope | an unqualified `--table-list` entry needs a default schema, so `--source-db-schema` without `public` must qualify every entry as `schema.table` |
| Schema list | `--source-db-schema` entries are trimmed and matched as `export schema` matches them |
| Export dir | must already hold a migration project; the command never creates one |
| State | read-only; writes only under `reports/` |
| Lock | takes its own per-command lock on the export dir, so two `detect-drift` runs cannot overwrite each other's report; export and import are not blocked |
| Telemetry | `schema-detect-drift` phase payload, `SchemaDriftPhasePayload` v1.1 (v1.0 before the direction-split types and timing-based severities; the key and index types and the `CONSTRAINT` and `INDEX` object types only add keys and values, so they keep v1.1): the counts, the number of tables and the object types compared (sorted, without duplicates), and the drift-type and severity histograms. Carries the anonymized `SourceDBDetails` too, which already lists the schemas, so the payload does not count them again. Migration type is not reported: the command runs before `export data` records it and after a start-clean clears it, so there is no point at which it is reliably known. No schema, table, column, constraint or index names, and no definitions — identifiers do not go into this payload. `invoked_by` is `detect-drift` for the command, and `export-data` or `cutover-to-target` for the in-process check (§3.7). Governed by `--send-diagnostics` as everywhere else. |
| Telemetry on failure | A run that fails before it builds a report still reports, as `ERROR` with the counts left zero: that is the population which could not use the feature at all. `error` carries `msg: "schema drift"` and the step that failed (`setup`, `connect_to_source`, `resolve_schemas`, `load_snapshots`, `capture_live_schema`, `resolve_scope`, `build_report`, `nothing_compared`, `write_reports`), never the error text, which can carry table names, patterns and paths. It follows `errs.ImportBatchError`: the run tags each failure with an `errs.SchemaDriftError` step, and callhome adds the `step` key. Callhome sets `msg` itself instead of keeping the text before the first `:`, so the console shows the cause unchanged and no wrapper can push a name into `msg`. The stack trace and any SQLSTATE still go with it. A failure before the export dir's metaDB is opened sends nothing, because the anonymizer is not set up yet. A failure after that, including one in the root pre-run, reports like any other phase, with a nil migration UUID if the UUID was never read. |

### 3.7 In-process check

```go
func checkSchemaDrift(in driftCheckInput) (*schemadrift.Report, []string, error)

type driftCheckInput struct {
	Schemas  []string          // unquoted, already resolved
	Filters  driftScopeFilters // zero value: every table, every object type
	LiveRead bool
	Formats  []string
	Invoker  driftInvoker
}
```

`schema detect-drift` and the in-process callers in `export data` and `initiate cutover to target` share this function. It runs from the snapshot load through the report write. The caller owns the source connection, and `checkSchemaDrift` never connects or disconnects. It takes no lock: a standalone run at the same moment only reads metaDB and writes its own files. It reads the migration status record with `metaDB.GetMigrationStatusRecord` and passes `driftMigrationInfoFrom(msr)` to `BuildReport` (§3.3), so all three callers place changes in a timing the same way; a nil record is an error. The returned report is nil when the run failed before one was built.

| Invoker | Report files under `reports/` | Notes (snapshot load, report overwrite) |
| :---- | :---- | :---- |
| `schema detect-drift` | `drift_analysis_report.html`, `.json` | console |
| `export data` | `drift_analysis_report_export_data.html`, `.json` | log |
| `initiate cutover to target` | `drift_analysis_report_cutover_to_target.html`, `.json` | log |

Each writer overwrites only its own files, so a failure or a cutover never overwrites the report the user is reading. Every caller names its invoker; an unset one is an error, so a caller cannot fall into the command's files, console output or callhome guard by omission.

## 4\. Data model

### 4.1 Report

The JSON report is an output file, not migration state, so upgrades never read an old report with a new binary. The compatibility concern is the other way: users and tooling parse the JSON. Field names and tags are contractual from the first release that ships the command; until then the shape is still being settled and `version` stays 1. After that, changes are additive and any non-additive change bumps `version`.

```go
type Report struct {
	Report        string            `json:"report"`   // always "schema_drift"
	Version       int               `json:"version"`  // 1
	GeneratedAt   time.Time         `json:"generated_at"`
	Source        Source            `json:"source"`
	Window        Window            `json:"window"`   // first capture to last capture
	Comparing     Comparing         `json:"comparing"`
	Summary       Summary           `json:"summary"`
	Drifts        []DriftEntry      `json:"drifts"`
	CapturePoints []CapturePoint    `json:"capture_points"` // every point on the timeline, placeholders included
}

type Source struct {
	DatabaseType    string `json:"database_type"`
	Host            string `json:"host"`
	Port            int    `json:"port"`
	Database        string `json:"database"`
	DatabaseVersion string `json:"database_version"`
}

type Window struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

type Comparing struct {
	Schemas     []string `json:"schemas"`
	Tables      []string `json:"tables"` // what was compared, not what was typed
	ObjectTypes []string `json:"object_types"`
}

type Summary struct {
	ChangeCount           int  `json:"change_count"`
	ComparedIntervalCount int  `json:"compared_interval_count"` // intervals actually diffed
	StoredCaptureCount    int  `json:"stored_capture_count"`    // placeholders included; live read excluded
	LiveCompared          bool `json:"live_compared"`
}

type DriftEntry struct {
	Diff // what changed, on what, to what -- flattened by encoding/json

	// When it was detected, and what the migration was doing then.
	Window Window `json:"window"`          // the interval it was detected in
	Phase  string `json:"phase,omitempty"` // §5.3

	// What it means: Severity, Impact, Action -- flattened by encoding/json.
	DriftInfo
}

type Diff struct {
	// What changed, as the diff engine classified it.
	Type       schemadiff.DiffType   `json:"type"`
	Operation  schemadiff.Operation  `json:"operation"`           // ADDED | DROPPED | CHANGED
	ObjectType schemadiff.ObjectType `json:"object_type"`         // TABLE | COLUMN | CONSTRAINT | INDEX
	Attribute  schemadiff.Attribute  `json:"attribute,omitempty"` // "" for ADDED/DROPPED

	// What it changed on, and to what.
	Object    schemasnapshot.ObjectRef `json:"object"`               // the table
	SubObject string                   `json:"sub_object,omitempty"` // the column, constraint or index; empty for TABLE
	OldValue  any                      `json:"old_value,omitempty"`
	NewValue  any                      `json:"new_value,omitempty"`
}

type CapturePoint struct {
	Label      string    `json:"label"`
	Reason     string    `json:"reason,omitempty"`
	CapturedAt time.Time `json:"captured_at"`
	Excluded   string    `json:"excluded,omitempty"` // why it was bridged; empty when used
}
```

`Comparing` states what was compared, not what the user typed. Unfiltered, `Tables` is the whole universe; filtered, it is the resolved keep-set. Every identifier in the report -- `Comparing.Schemas`, `Comparing.Tables`, and the schema names in `CapturePoint.Excluded` -- is minimally quoted for the source engine, so a mixed-case schema reads `"Sales"` everywhere it appears. Matching itself uses the raw catalog names.

`CapturePoints` is every point on the timeline, not only the ones holding schema: a stored capture, a stored placeholder (the capture failed, so nothing is behind it), and the live read (never persisted). `StoredCaptureCount` counts the first two -- a placeholder is a persisted row -- so it is a count of stored records, not of usable snapshots.

`CapturePoint.Excluded` exists because a point the assembler bridged is otherwise indistinguishable from one that contributed nothing. It is `omitempty` because a normal run bridges nothing. The exclusion is recorded on the point, not on an interval: a bridged capture does not end an interval, so there is no un-compared span to list.

`Diff` holds the `schemadiff.Difference` fields a report needs, flattened rather than embedding `Difference` itself. Its `ObjectA`/`ObjectB` are `ObjectIdent` interface values, which marshal but cannot be unmarshalled, and they hold a different shape per finding (a column's identity nests its table; a table's does not), so one JSON key would carry two schemas. `Difference` also carries no JSON tags, so embedding would publish Go field names into this contract and make every field later added to the diff engine part of it. Flattening also does once what every consumer would otherwise repeat: choosing the display side, and splitting a column into its table and its own name.

`Object` and `SubObject` always identify the display side. For a change, that is the new identity; for a drop, the old one. A renamed column therefore appears under its new name with the old name in `OldValue`.

### 4.2 Persisted state

The `schema_snapshots` metaDB table is unchanged. The `SnapshotContent` blob goes from version 1 to version 2, which adds keys, checks and indexes under each table (§5.9). The change is additive: an older voyager rejects a version 2 blob with `ErrSnapshotVersionUnsupported` rather than misreading it, and this voyager still reads version 1 blobs. The live read is held in memory and never saved.

## 5\. Behaviour

### 5.1 End-to-end flow

One invocation of `schema detect-drift`, as the sequence of calls that carry data across the layers in §2. Types are the ones declared in §3 and §4. Unexported names inside a package are the author's current intent; the crossings between packages and the types passed across them are the contract.

```
cmd.detectDrift()
 │
 ├─ metaDB.GetMigrationStatusRecord(), cmd.driftMigrationInfoFrom(msr) → MigrationInfo; a nil record is an error
 ├─ schemasnapshot.ListSnapshots(metaDB)                      → []SnapshotHeader         cmd → schemasnapshot
 ├─ per header: schemasnapshot.LoadSnapshotByName(metaDB, name) → *SnapshotContent | nil  cmd → schemasnapshot
 │      each becomes a schemasnapshot.SchemaSnapshot{Header, Content}
 │
 ├─ cmd.captureLiveSnapshotForDrift(schemas)                  → *SchemaSnapshot, error                        §5.6
 │      └─ schemasnapshot.Capture(ctx, db, CaptureParams{Label: LabelSourceLive})        cmd → schemasnapshot
 │         its Header.Label IS the timeline identity; appended as the last input
 │
 ├─ cmd.driftTableUniverse(contents, live)                    → table universe                                §5.5
 ├─ namereg.NewInMemorySourceNameRegistry(universe)           → []NameTuple, cmd.extractTableListFromString,
 │      cmd.expandDriftPartitions / complementDriftTableRefs  → []ObjectRef, then schemadiff.Scope            §5.5
 │
 ├─ schemadrift.BuildReport(DetectionConfig)                  → Report, error            cmd → schemadrift
 │      ├─ schemadiff.NewDiffer(Config{Scope})
 │      ├─ per comparable pair (§5.2): differ.Diff(prev, next) → []Difference            schemadrift → schemadiff
 │      ├─ phaseFor(prevCapture, nextCapture)                 → phase string                                   §5.3
 │      ├─ situationFor(captures, prevIdx, nextIdx, MigrationInfo) → situation                                §5.3
 │      └─ per Difference: getDriftInfo(d.Type, situation)    → DriftInfo, embedded in DriftEntry, or error   §5.4
 │
 ├─ if Summary.ComparedIntervalCount == 0: cmd.nothingComparedError(report) → error, exit 1; no file is written   §7
 │
 ├─ cmd.writeDriftReports(report, formats)
 │      ├─ schemadrift.RenderJSON(Report)                     → []byte                   cmd → schemadrift
 │      └─ schemadrift.RenderHTML(Report)                     → []byte                   cmd → schemadrift
 │         written to <export-dir>/reports/drift_analysis_report.{json,html}
 │
 └─ cmd.printDriftSummary(report, paths); exit 0
```

Everything above `BuildReport` is `cmd` assembling plain data; everything inside it is pure computation over that data; everything after it is serialisation. The four decision points that follow are the places where two implementations of this flow could diverge on the same input.

### 5.2 Which pairs are compared

**Where:** `schemadrift.BuildReport`, the walk over `DetectionConfig.Snapshots`. **In:** `[]schemasnapshot.SchemaSnapshot`, oldest first. **Out:** the `(prev, next)` pairs handed to `Differ.Diff`, plus `Report.CapturePoints`. **Decides:** which two snapshots form an interval, and what to do with inputs that cannot be an interval's side.

The walk keeps a *baseline*: the most recent snapshot eligible to be the older side of a comparison.

| Current input | Baseline | Action |
| :---- | :---- | :---- |
| `Content == nil` (failed capture) | any | Set `CapturePoint.Excluded`. Leave the baseline alone, so the next usable snapshot compares back across the gap. |
| `Header.Schemas` does not cover `Scope.Schemas` | any | Set `CapturePoint.Excluded` naming what it did and did not cover. Leave the baseline alone -- bridged, exactly like a failed capture. |
| covers, no baseline yet | none | Becomes the baseline. Nothing to compare. |
| covers | set | Diff baseline → current through `Differ` with `p.Scope`. Each `Difference` becomes a `DriftEntry` carrying the interval's `Window` and `Phase`. Current becomes the baseline. |

A failed capture is *bridged*, not a boundary. The drift that happened around it is still real; what is lost is only the ability to say which side of the failed capture it fell on, and the wider `Window` on the entry says so.

A capture that does not COVER the requested schemas is bridged for the same reason a failed one is: a requested table missing from it is not evidence of a drop, only evidence that nobody looked. Covering MORE than was requested is not a mismatch -- those captures are compared normally, and the extra schemas' findings are removed by `Scope.Schemas` (§3.1) rather than by declining the comparison. Coverage ignores order and duplicates.

The test is coverage, not equality. A run narrowed at detect-drift time has a live read carrying exactly `--source-db-schema` while history carries whatever export used, so requiring equal schema sets would reject every interval and report no drift at all.

`Report.Window` spans the first to the last `CapturePoint`, placeholders and the live read included. `Summary.StoredCaptureCount` counts the stored captures, placeholders included -- a placeholder is a persisted row; only the live read is excluded, and it is reported through `LiveCompared`.

`Summary.ComparedIntervalCount` is how many intervals were actually diffed. It is the honest counterpart to `ChangeCount`: zero changes over zero intervals is not a clean report, it is a report that examined nothing, and without this field a reader has to scan `capture_points[].excluded` to notice. `StoredCaptureCount` cannot stand in -- placeholders count toward it, so two failed captures read as two snapshots examined.

`Summary.LiveCompared` means the live read was actually DIFFED against a baseline, not merely that one was taken. A live read that is bridged, or that is the first usable snapshot and so has nothing behind it, reports false -- otherwise the summary claims the source was checked against history when it was not.

### 5.3 Phase

**Where:** `schemadrift.phaseFor(prev, next CapturePoint) string`, called once per interval from `BuildReport`. **In:** the two `CapturePoint.Label` values bracketing an interval. **Out:** `DriftEntry.Phase`. **Decides:** what the migration was doing while the interval's drift appeared.

Unlisted pairs get `""` and the renderer shows the window alone.

| Earlier Label | Later Label | Phase |
| :---- | :---- | :---- |
| `export_schema` | `export_data_from_source_start` | `export data: pending` |
| `…_start` or `…_periodic` | `…_periodic` or `…_exit` | `export data: running` |
| `export_data_from_source_exit` | `export_data_from_source_start` | `export data: paused` |
| any | `source_live` | `since last capture` |
| anything else |  | `""` |

The exit capture's `Reason` (`cutover`, `complete`, `interrupt`, `error`) says how a run ended and is shown on the timeline; the phase says what was happening while the drift appeared. A start→exit pair is therefore "running", whatever the exit reason.

**Timing.** `schemadrift.situationFor(points []CapturePoint, prevIdx, nextIdx int, m MigrationInfo) (situation, error)`, next to `phaseFor` and called once per interval, places the interval's changes in a `Timing` for the guidance lookup (§5.4). The phase is a display label; the timing also needs the flow and the cutover, so the two are computed separately. Rules in order, first match wins:

| Timing | When |
| :---- | :---- |
| `after_cutover` | the earlier capture is an `export_data_from_source_exit` with reason `cutover`, or it was taken at or after `CutoverRequestedAt` |
| `before_export_data` | export data has not recorded an export type yet (`ExportType` is empty: `export data` saves it before its start capture), whatever the captures are; or the first export data capture (start, periodic or exit) at or after the later capture is an `export_data_from_source_start` with reason `initial` or `clean_restart`, whatever the earlier capture is. So two `export schema` runs before `export data`, or an `export schema` re-run before a clean restart, are before export data. A `resume` start means data was already exported, even after an `export schema --start-clean`. With no export data capture at or after the later capture, the export type decides |
| `offline_after_export_start` | `ExportType` is `snapshot-only` |
| `changes_only_after_export_start` | `ExportType` is `changes-only` |
| `live_after_export_start` | `ExportType` is `snapshot-and-changes` |

Any other `ExportType` is an error: guessing a timing would print advice for the wrong flow.

`situation.FallBackPossible` is true unless the timing is `after_cutover` or `ExportType` is `snapshot-only`: a live flow, or one whose export type is not recorded yet, can still cut over with fall-back. An interval that spans the cutover request is not `after_cutover`, because the change may have come before it.

Known gap: `export data --start-clean` can change the export type (for example `snapshot-only` to `snapshot-and-changes`), while the snapshots from the earlier run stay in metaDB. `situationFor` applies the current `ExportType` to every interval, so intervals from before the restart get the new run's timing and text. This is accepted because such a restart is rare and only affects history from before it.

### 5.4 Severity

**Where:** `schemadrift.getDriftInfo(t schemadiff.DiffType, s situation) (DriftInfo, error)`, backed by `infoByDiffType`, `fallBackRiskByDiffType` and `afterCutoverInfo`, called once per `Difference` from `BuildReport`. **In:** a `DiffType` and the interval's `situation` (§5.3). **Out:** the `DriftInfo` embedded in each `DriftEntry` -- `Severity`, `Impact`, `Action` -- or an error for a type and timing with no entry. **Decides:** what the migration does about a change, and the note that explains it.

The guidance is a matrix: one entry per change type and timing. The resolution order is in §3.4: after cutover, every type gets the same advisory; changes-only uses the live text when a type has none of its own; an unknown type is an error; and the fall-back sentence is appended while a cutover with fall-back is still possible.

The change types, by object:

| Object | DiffTypes |
| :---- | :---- |
| Table | `TABLE_ADDED`, `TABLE_DROPPED`, `TABLE_NAME_CHANGED`, `TABLE_SCHEMA_CHANGED`, `TABLE_KIND_CHANGED` |
| Partition (on the partition) | `TABLE_PARTITION_ATTACHED`, `TABLE_PARTITION_DETACHED`; a move between parents is one of each |
| Partition (on the parent) | `TABLE_PARTITION_CHILDREN_CHANGED` |
| Inheritance (on the child) | `TABLE_INHERIT_ADDED`, `TABLE_INHERIT_REMOVED`: at most one of each, listing the parents added or removed |
| Inheritance (on the parent) | `TABLE_INHERITED_BY_ADDED`, `TABLE_INHERITED_BY_REMOVED`: at most one of each, listing the children added or removed |
| Column | `COLUMN_ADDED`, `COLUMN_DROPPED`, `COLUMN_NAME_CHANGED`, `COLUMN_TYPE_CHANGED`, `COLUMN_NOT_NULL_SET`, `COLUMN_NOT_NULL_DROPPED`, `COLUMN_DEFAULT_CHANGED` |
| Constraint | `PRIMARY_KEY_`, `UNIQUE_`, `CHECK_`, `FOREIGN_KEY_` each with `ADDED`, `DROPPED`, `NAME_CHANGED` |
| Index | `UNIQUE_INDEX_`, `INDEX_` each with `ADDED`, `DROPPED`, `NAME_CHANGED` |

Key and index types have object type `CONSTRAINT` or `INDEX`, and `SubObject` is the constraint or index name. `*_NAME_CHANGED` uses attribute `NAME`; `ADDED` and `DROPPED` carry the definition as the new or old value, which the report shows for an addition and omits for a drop. How they are found is in §5.9.

Partition and inheritance links name the other table, but are compared by its OID when tables are matched by OID: renaming or moving a parent or child is reported only on that table, not as a detach and attach on every linked table.

The split partition, inheritance and NOT NULL types keep `Operation` `CHANGED` and their attribute (`PARTITION_PARENT`, `INHERITS`, `INHERITED_BY`, `NULLABILITY`), so they render as before. A partition finding carries the one parent it is about, and an inheritance finding the list of parents or children added or removed: the old value for a detach or removal, the new value for an attach or addition, the other side empty. A NOT NULL finding carries the old and new nullability.

Severity by change and timing (changes-only has the live severity; after cutover every type is `advisory`). The exact Impact and Action text is in `infoByDiffType` in `schemadrift`, and in the Addendum B design doc (DB-23869).

| DiffType | Before export data | Offline, after export data started | Live, after export data started |
| :---- | :---- | :---- | :---- |
| `TABLE_DROPPED` | `advisory` | `advisory` | `breaks_migration_unrecoverable` |
| `TABLE_NAME_CHANGED`, `TABLE_SCHEMA_CHANGED` | `breaks_migration_recoverable` | `potential_impact` | `breaks_migration_unrecoverable` |
| `TABLE_ADDED` | `breaks_migration_recoverable` | `advisory` | `advisory` |
| `COLUMN_ADDED` | `breaks_migration_recoverable` | `potential_impact` | `breaks_migration_recoverable` |
| `COLUMN_DROPPED`, `COLUMN_TYPE_CHANGED`, `COLUMN_DEFAULT_CHANGED`, `TABLE_KIND_CHANGED` | `potential_impact` | `potential_impact` | `potential_impact` |
| `COLUMN_NAME_CHANGED` | `breaks_migration_recoverable` | `potential_impact` | `breaks_migration_unrecoverable` |
| `COLUMN_NOT_NULL_DROPPED` | `breaks_migration_recoverable` | `potential_impact` | `breaks_migration_recoverable` |
| `COLUMN_NOT_NULL_SET`, `TABLE_PARTITION_CHILDREN_CHANGED`, `TABLE_INHERIT_REMOVED`, `TABLE_INHERITED_BY_REMOVED` | `advisory` | `advisory` | `advisory` |
| `TABLE_PARTITION_ATTACHED` | `breaks_migration_recoverable` | `potential_impact` | `potential_impact` |
| `TABLE_PARTITION_DETACHED` | `potential_impact` | `potential_impact` | `breaks_migration_unrecoverable` |
| `TABLE_INHERIT_ADDED`, `TABLE_INHERITED_BY_ADDED` | `breaks_migration_recoverable` | `potential_impact` | `breaks_migration_unrecoverable` |
| `PRIMARY_KEY_ADDED` | `breaks_migration_recoverable` | `advisory` | `advisory` |
| `PRIMARY_KEY_DROPPED` | `breaks_migration_recoverable` | `potential_impact` | `breaks_migration_unrecoverable` |
| `UNIQUE_DROPPED`, `UNIQUE_INDEX_DROPPED`, `CHECK_DROPPED` | `breaks_migration_recoverable` | `potential_impact` | `breaks_migration_recoverable` |
| `FOREIGN_KEY_DROPPED` | `potential_impact` | `potential_impact` | `potential_impact` |
| `UNIQUE_INDEX_ADDED` | `advisory` | `advisory` | `potential_impact` |
| `UNIQUE_ADDED`, `CHECK_ADDED`, `FOREIGN_KEY_ADDED`, `INDEX_ADDED`, `INDEX_DROPPED`, every key and index `*_NAME_CHANGED` | `advisory` | `advisory` | `advisory` |

Recoverable means the target can be fixed within this migration and `import data` run again. Unrecoverable means `--start-clean`, a new migration, or a separate migration for the affected rows is needed; this includes rows that are silently not migrated. `TABLE_KIND_CHANGED` never fires on PostgreSQL (making a table partitioned is a drop plus an add), so its text is not part of the verified set. The key and index severities come from runs on a PostgreSQL 17 source and a YugabyteDB 2026.1 target, recorded in the Addendum C design doc (DB-24176). `PRIMARY_KEY_DROPPED` is reported for a changed key too (§5.9), so a key changed in one transaction, which is recoverable, reads as unrecoverable when live. Key changes are rare, and overstating is the safe direction. For the same reason `PRIMARY_KEY_ADDED` before export data takes the live severity: whether the migration will be live or offline is not known until export data starts, and its Impact says an offline migration is not affected. `UNIQUE_INDEX_ADDED` is `potential_impact` when live because an index on an expression can't be added on the target until cutover: import data's conflict check reads only key columns, and the tables it applies in order are fixed on its first run, so adding the index mid-stream can stop import data on a unique violation that a restart does not prevent. Dropping the index on the target and running import data again recovers.

`UNIQUE_ADDED`, `UNIQUE_INDEX_ADDED`, `CHECK_ADDED` and `FOREIGN_KEY_ADDED` have a fall-back sentence for when the target does not get the change. `PRIMARY_KEY_ADDED` has none: before export data, import data refuses the table until the target has the key, and after export data starts the table either was not in the live migration or its partitions already had the same keys on both sides. For all but the foreign key, import data to source stops at the first YugabyteDB row that breaks it: `session_replication_role = replica` leaves unique and check constraints enforced. A foreign key is never enforced there: import data to source runs with foreign-key checks off, or refuses to start until they are turned off, so rows that break the key load and the source's key no longer holds for them.

Severity is about what the migration does, not how alarming the DDL is. An added column is more severe than a dropped one because incoming rows carry a value the target cannot store, while a dropped column only leaves the target with an extra column.

Every entry in `infoByDiffType` carries a non-empty Severity, Impact and Action. Backticks in the text become inline code in HTML and stay literal in JSON.

### 5.5 Table universe and scope resolution

**Where:** `cmd.driftTableUniverse`, `namereg.NewInMemorySourceNameRegistry`, export data's `cmd.extractTableListFromString`, `cmd.expandDriftPartitions`, `cmd.complementDriftTableRefs`, and the object-type equivalents, before `BuildReport`. **In:** every loaded `SnapshotContent`, the live read, and the four list flags. **Out:** `schemadiff.Scope` for `DetectionConfig.Scope`. **Decides:** what a `--table-list` pattern can name, and how an exclude list becomes the positive allow-list `Scope` expects.

The set of tables a pattern can match is the union of every loadable stored snapshot and the live read. The live read already holds every table in the catalog, so the catalog is not listed separately. A table dropped from the source but present in history is therefore still addressable, which is the case where the user most needs the report. Failing the live read is an error (exit 1).

Names resolve the way export data resolves them. The universe is loaded into an in-memory name registry, which turns it into `NameTuple`s. The flags then go through export data's `extractTableListFromString`, so the glob matching and the unknown-table error are export's own. The registry is never written: the migration's `name_registry.json` lists only the tables present at export data's first run, so it cannot name a table created since.

A partitioned table matched by either list brings every partition beneath it, at every level, as in export data. A partition matched on its own brings only itself. The hierarchy comes from `PartitionChildren` in the snapshots and the live read, so a partition dropped since is still expanded and its drop is still reported.

| Flag | Resolution |
| :---- | :---- |
| neither list flag | `Scope.Tables` \= the whole universe, passed explicitly; `Comparing.Tables` is that same set |
| `--table-list` | patterns resolved against the universe by export data's matcher, then each matched partitioned table expanded to its partitions; unknown pattern is an error, exit 1 |
| `--exclude-table-list` | resolved and expanded the same way, then complemented against the universe; an empty result is an error, exit 1 (the report would be empty, so the command says so rather than emitting one) |
| both | error, exit 1 |

`--object-type-list` and `--exclude-object-type-list` follow the same shape over `{TABLE, COLUMN, CONSTRAINT, INDEX}`. `CONSTRAINT` covers primary keys, unique constraints, checks and foreign keys; `INDEX` covers indexes and standalone unique indexes.

All four list flags are normalised before use: a value that is empty once trimmed (`"  "`, `","`) counts as unset. Treated as set, it would resolve to an empty keep-set, and since `Scope` keeps nothing for an empty dimension the run would drop every finding and read as "no drift".

### 5.6 Live read

**Where:** `cmd.captureLiveSnapshotForDrift(schemas) (*schemasnapshot.SchemaSnapshot, error)`, after history is loaded and before the universe is built. **In:** the open source connection and the resolved schema list. **Out:** the last snapshot, labelled `source_live`, so the report always ends at now.

The command captures the current source schema in memory under `LabelSourceLive` and appends it as the last input. It is captured with exactly `--source-db-schema`, so it always covers the request; history may cover more, and the extra schemas' findings are filtered rather than the comparison declined. If the live capture fails, the run fails with exit 1 and writes no report. The live read is the only comparison that covers drift since the last stored snapshot, so a report without it can show no drift while the source has drifted.

The in-process check in `export data` runs without a live read: the exit capture stored the end state moments earlier. Such a report ends at the last stored capture. Its `live_compared` is false, and the terminal summary and the HTML footer leave out the live read.

### 5.7 Check on export failure

The source exporter checks for drift in one case: Debezium failed after its status file recorded streaming. Drift can only break the export there, for example with an added column the connector cannot handle. The check runs right after "Export of data failed!", through `checkSchemaDrift` (§3.7), and never changes the exit code.

| How the export ends | Check |
| :---- | :---- |
| Debezium fails after its status file records streaming | yes. For PostgreSQL that is from startup: `pg_dump` takes the snapshot, so Debezium runs with `snapshot.mode=never` and records streaming before it connects. A startup failure Voyager does not catch first, such as a dropped publication, is checked too and reports no drift. |
| Debezium fails before it records streaming | no. For PostgreSQL that is never the case. A missing replication slot falls under the `ErrExit` row: Voyager's own check exits before Debezium starts. |
| `pg_dump` fails | no: it holds `ACCESS SHARE` locks, so DDL waits and an added column is simply dumped |
| Any other error, or an `ErrExit` | no |
| Ctrl-C, SIGTERM, an end-migration stop | no |
| Success or cutover | no |

Whether Debezium had switched to streaming is read from its status file once it has exited, not only from the exporter's last poll of it, so a crash moments after the switch still counts as a streaming failure.

Ctrl-C reaches Voyager and Debezium together, since they share a process group, so Debezium's error alone does not mean it failed. Two guards keep an interrupt out: Debezium's exit code 130 or 143, which Java exits with on SIGINT or SIGTERM, is not a failure; and the check returns at once when Voyager is shutting down.

The check compares every table in the export's schemas, ignoring `--table-list` and `--exclude-table-list`, so a new table still shows. It takes no live read and does not reconnect: the exit capture stored the end state moments earlier. When that capture failed, the report ends at the last stored one. It follows `--disable-schema-snapshot-capture`, because without captures there is nothing to compare.

```
Export of data failed! Check <export-dir>/logs for more details.

Checking the source schema for drift...
No schema drift found on the source. Report: <export-dir>/reports/drift_analysis_report_export_data.html
```

With drift it prints the summary of §5.1 instead. If the check fails, it prints `Could not check the source schema for drift: <reason>`.

### 5.8 Pre-check before cutover to target

`initiate cutover to target` checks for drift just before its confirmation prompt, so the user reads the result before answering. The check includes the live read: the exporter is still running, so the live read is the only view of drift since the last periodic capture.

The check covers only the tables the migration exports: the stored export table list (`TableListExportedFromSource`), passed as its table list, so each partitioned root brings in all its partitions, including leaves added mid-migration. DDL on other tables in the migrated schemas does not count, since those tables are not on the target. Known gap: the list holds a table's name at export time, so after a rename only the rename itself is reported, not later changes under the new name.

```
yb-voyager initiate cutover to target --export-dir <dir> --prepare-for-fall-back <yes|no> \
    [--source-db-password <p>] [--skip-pre-checks schema_drift]
```

| Flag | Config key | Purpose |
| :---- | :---- | :---- |
| `--source-db-password` | `source.db-password`, or env `SOURCE_DB_PASSWORD` | Source password for the pre-check. Prompted for when unset. Host, port, user, database, schemas and SSL come from the migration status record. |
| `--skip-pre-checks <list>` | `initiate-cutover-to-target.skip-pre-checks` | Comma-separated cutover pre-checks to skip. Today the only name is `schema_drift`. An unknown name is an error. |

One list flag, not one boolean per check, because more cutover pre-checks are planned.

| Situation | Behaviour |
| :---- | :---- |
| `--skip-pre-checks` includes `schema_drift`, or the export dir holds no captures | skipped, with one line |
| every stored capture failed (only placeholders) | skipped, with a warning: there is nothing to compare against, and the export data log says why the captures failed |
| a non-PostgreSQL source | skipped, logged only: the check can never run there, so the console stays as it was |
| cutover to target already requested | skipped silently; the existing "already initiated" message follows |
| no drift | one line, then the prompt |
| drift found, at a terminal | the summary of §5.1, then "Have you applied these changes to the target?". `y` goes on to the cutover prompt; anything else stops cutover with exit 1 and names `--skip-pre-checks schema_drift` |
| drift found under `--yes`, or with no terminal on stdin (`echo y \|`, a here-string, Ansible `stdin:`) | the summary, then cutover fails with exit 1 before the MSR update: nobody can confirm, and a piped answer would confirm a summary nobody read. The message does not name `--yes`, which is hidden. A pseudo-terminal (`expect`, Ansible's `expect` module) looks like a person and gets the prompt |
| the check cannot run: no password, an unreachable source, a failed live read | cutover fails with exit 1 and names `--skip-pre-checks schema_drift`; a connect or password failure also points at the source connection and `--source-db-password` |

Any change counts, whatever its severity. A pre-check that cannot run fails cutover, because silently passing would read as "no drift". The password prompt follows the "Checking the source schema for drift..." line, which says why cutover asks for it. Without a terminal the prompt cannot read, so a script must pass `--source-db-password` or set `SOURCE_DB_PASSWORD`, and the failure says so. The connect and the live read are each bounded by the 10-second capture budget, so a wedged source cannot hold cutover. The check runs in the `Run` of `cutoverToTargetCmd`, before `InitiateCutover`, so cutover to source and to source-replica are unaffected.

The SSL certificate and key paths in the migration status record are host paths. In the Docker install, cutover passes no SSL flags of its own, so `docker/yb-voyager-docker` must mount the paths that earlier commands recorded in `metainfo/docker-metadata-*`, as it already does for `end migration`. Without that, the check cannot read the CA file and fails cutover for every SSL source.

### 5.9 Keys, checks and indexes

**Where:** `schemasnapshot` (capture), `schemadiff.compareMatchedTables` (matching). **In:** two snapshots of one matched table. **Out:** the key and index findings of §5.4. **Decides:** what is captured, and when a change to a key or index is reported.

**Captured.** Each `Table` holds `PrimaryKey *Constraint`, `UniqueKeys`, `Checks` and `ForeignKeys` (`[]Constraint`), and `UniqueIndexes` and `Indexes` (`[]Index`). Both structs embed `TableScopedObjectRef` and carry `ID` (the OID), `Definition` (what PostgreSQL prints) and `FromParent`. The kind is the field that holds the object, so there is no kind field to disagree with it.

| Kind | Catalog | Left out |
| :---- | :---- | :---- |
| Primary key, unique, check, foreign key | `pg_constraint`, `contype` `p`, `u`, `c`, `f` | NOT NULL rows (PostgreSQL 18 stores them here; column nullability covers them), exclusion constraints (YugabyteDB does not support them, so there is nothing to change on the target), trigger constraints, domain constraints, the per-partition clones PostgreSQL adds for a foreign key to a partitioned table (the foreign key itself is captured) |
| Unique index | `pg_index`, `indisunique`, no constraint behind it | invalid indexes |
| Index | `pg_index`, not unique | indexes behind a primary key, unique or exclusion constraint (the constraint reports them, or is left out), invalid indexes |

The PostgreSQL provider empties `search_path` for the capture transaction before its first query, so every captured text is schema-qualified and never depends on the session: column types and defaults, a foreign key's `REFERENCES`, and index definitions. Copies made for partitions and inheritance are captured with `FromParent` set, so the diff can tell them from the table's own objects. A `TABLE_ADDED` finding's definition lists the table's own keys, checks and indexes, because its Action is to create the table on the target. For a partition or inheritance child it also names `PARTITION OF` or `INHERITS`, since creating the table that way brings the copies the definition leaves out.

**Matching, by OID only.**

| On the two sides | Reported |
| :---- | :---- |
| Same OID, a copy on either side | nothing: a parent rename renames its copies, and attach, detach and inheritance have table-level findings |
| Same OID, name differs | `*_NAME_CHANGED` |
| Only in the earlier snapshot, not a copy | `*_DROPPED` |
| Only in the later snapshot, not a copy | `*_ADDED` |
| Only on one side, a copy | nothing: the parent's finding covers it |

The primary key is one slot per table: none to some is `PRIMARY_KEY_ADDED`, some to none is `PRIMARY_KEY_DROPPED`, and a new OID is both.

`Definition` is never compared. PostgreSQL prints it with current names, so renaming a column, a referenced table or a schema changes the text of objects that did not change. In place, PostgreSQL can change only an object's name and flags; a new expression or column list needs a drop and re-create, which gives a new OID.

**Snapshots before the upgrade.** A version 1 snapshot holds no keys or indexes. A pair with a version 1 side compares tables and columns only, so a migration upgraded midway keeps working: key and index changes made in the one interval that spans the upgrade are not reported, and every later interval reports them. A version 1 snapshot also printed column types and defaults with the session's `search_path`, so that interval reports `COLUMN_TYPE_CHANGED` or `COLUMN_DEFAULT_CHANGED` for columns whose type or sequence is in a schema on that path, typically `public`. This is accepted: it happens once, and the finding shows only the added schema name.

**Accepted limitations.** Each overstates rather than hides a change:

- Anything that gives an object a new OID reads as a drop plus an add, whether or not anything changed: a re-create in one transaction, `REINDEX CONCURRENTLY` on a standalone index, `ALTER COLUMN TYPE` on a column a key, check or index uses, and `ADD … USING INDEX`. The drop's severity and Action apply. The unique drops' Impact says that, next to an added key or index on the same columns, the source still enforces uniqueness and the target should keep it.
- Keys and indexes dropped together with their column each get a plain `*_DROPPED` next to `COLUMN_DROPPED`.
- `DEFERRABLE`, `VALIDATE`, and PostgreSQL 18 `NOT ENFORCED` are not reported. A primary key or unique constraint can only become deferrable by being re-created, which is reported as a drop plus an add.

Pairing leftovers by name, which removes the first two, is designed in the Addendum C design doc and left for later.

## 6\. Migration-flow matrix

Capture happens in `export schema` and, when the exporter role is the source exporter, at `export data` start, every `--schema-snapshot-capture-interval` minutes (default 60), and at exit. It runs by default on a PostgreSQL source and is a no-op on any other; `--disable-schema-snapshot-capture` turns it off. A capture failure is logged and never fails the export. `detect-drift` reads whatever `<export-dir>/metainfo/meta.db` holds.

| Flow | Captures | detect-drift | Timing after `export data` starts (§5.3) | Notes |
| :---- | :---- | :---- | :---- | :---- |
| Offline | export schema, export data start / periodic / exit(complete) | covered | `offline_after_export_start` |  |
| Live, snapshot \+ changes | as offline; periodic continues through streaming; exit reason `cutover` | covered | `live_after_export_start`; `after_cutover` from the exit(cutover) capture or the cutover request | Cutover to target runs the pre-check of §5.8 before its prompt. |
| Live with fall-back | source side as above; `export data from target` takes no captures | covered for the source, up to cutover | as live; a type's fall-back sentence is appended before cutover | Source-side DDL after cutover-to-target is only visible through the live read. Target-side drift is a non-goal (§1). |
| Live with fall-forward | same as fall-back | same | as live; once import data to source-replica has run, a type's fall-forward sentence is appended to Action before cutover, and no fall-back sentence is added, since fall-back cannot also be enabled |  |
| Changes-only | export schema, export data start / periodic / exit; no `pg_dump`, but capture is gated on role, not on export type | covered | `changes_only_after_export_start`, which uses the live text where a type has none of its own; `after_cutover` as live |  |
| Iterative cutover | each iteration's source exporter captures into that iteration's own metaDB | per iteration only | from the migration status record of the metaDB the snapshots come from | `--export-dir` pointed at the main dir sees the main metaDB; pointed at an iteration dir sees only that iteration. No cross-iteration timeline. Open question §9. Cutover to target's pre-check (§5.8) also runs per iteration, against that iteration's metaDB: DDL applied before the iteration's first capture is not reported. The next iteration's exporter is started without the CLI's `--disable-schema-snapshot-capture` and `--schema-snapshot-capture-interval`, so it captures at the defaults unless the config file sets them. |
| Non-PostgreSQL source | none (capture is a no-op) | error, exit 1 | n/a | Oracle and MySQL are non-goals (§1). |

## 7\. Failure modes

| Situation | Behaviour | Why this and not the alternative |
| :---- | :---- | :---- |
| Export dir holds no migration project | error, exit 1, as for every command that needs a started migration; no project is created, only the run's log | The command reads an existing migration. Creating a project there would leave a directory that looks like a migration that never ran. |
| A capture fails during export | placeholder header written, export unaffected, warning logged | Capture is best effort and off the data path. It must never fail a migration. |
| Placeholder in history | bridged (§5.2); appears on the timeline as a failed marker | Dropping it would hide that a capture was attempted; making it a boundary would hide real drift. |
| Snapshot blob has an unsupported `Version` | error, exit 1 | A newer voyager wrote it. Silently skipping would produce a report that looks complete. |
| A version 1 snapshot is paired with a version 2 one | tables and columns only for that interval (§5.9) | Rejecting it would fail detect-drift and the cutover pre-check for every migration upgraded midway. Comparing keys would report every key and index as added. |
| Snapshot header exists but blob cannot be loaded for any other reason | warning, treated like a placeholder | The moment is still on the timeline; the report bridges across it. |
| A capture did not cover the requested schemas | bridged, and recorded on its `CapturePoint` (§5.2) | A requested table missing from it means nobody looked, not that it was dropped. Treating it as a boundary lost every interval around it. |
| Live capture fails or the source is unreachable | error, exit 1; no report is written | The live read covers drift since the last stored snapshot. A report without it can show no drift while the source has drifted, and a script reading `summary.change_count` would not notice. |
| One stored snapshot, and the live read makes it a pair | warning; report covers that single interval | Not an error: one stored capture plus the live read is a real interval. Zero stored snapshots gets no warning, because it can never form an interval and always lands on the row below. |
| No comparable pair at all (`ComparedIntervalCount == 0`) | error, exit 1; the message is derived from what the assembler recorded, and names only the case that actually occurred | An empty report reads as "no drift". The three causes — no captures stored, none usable, only one usable — need different advice, so the message must not assert a cause it did not observe. Capture cannot be enabled retroactively, so "re-run the export" is never the remedy for the run in hand. |
| `DiffType` with no guidance entry | error, exit 1 (§3.4) | Reporting it as advisory with no note would show a change the report cannot explain. A test fails when a `DiffType` has no entry, so only a coding error reaches this. |
| The migration status record is missing | error, exit 1 | Without it the report cannot place a change in a timing, and guessing one would print advice for the wrong flow. |
| The cutover pre-check cannot run (§5.8) | cutover fails, exit 1, and names `--skip-pre-checks schema_drift` | A pre-check that silently passes reads as "no drift" just before the user commits to the target. |
| The check on export failure fails (§5.7) | one line, export exit code unchanged | The export's own error is the failure. The check is best effort, like capture. |
| The HTML renderer meets a state it cannot display (§3.5) | error, exit 1 | Rendering past it drops or misprints a finding in a report that still looks complete. |
| Report file already exists | overwritten with a notice | Reports are regenerated, not versioned. |

## 8\. Hot-path statement

None. `detect-drift` runs once per invocation over a handful of snapshots. Capture during export runs once per interval with a 10-second budget and is independent of the data path. Nothing here runs per row or per event.

## 9\. Alternatives considered

| Decision | Chosen | Rejected | Why |
| :---- | :---- | :---- | :---- |
| Scope shape | one allow-list per dimension | include \+ exclude lists per dimension | Empty was ambiguous; callers could pass undefined combinations. Only the CLI knows the universe needed to complement an exclude list. |
| Failed capture | bridge across it | treat as an interval boundary; drop it from the timeline | Drift around it is real. The wider window is the honest answer. |
| Capture does not cover the requested schemas | bridge it, record on the point | skip the interval; compare anyway; error out | Bridging keeps the intervals either side, which skipping lost. Comparing anyway reports tables nobody looked for as dropped. Erroring blocks a report that is still useful. |
| Narrowing to a subset of the captured schemas | filter findings after diffing | project each snapshot's content first | Projection turns a cross-schema move into a false `TABLE_DROPPED` (§3.1). |
| Severity and note | one struct in one map | parallel maps keyed by `DiffType` | Parallel maps drift silently. A test asserts every mapped type has a note. |
| Severity semantics | what the migration does | how alarming the DDL is | The user's question is "is my migration broken", not "was this a big change". |
| Guidance key | change type and timing | change type alone | The right advice depends on when the change happened: a table dropped before `export data` is not part of the migration, one dropped during a live export makes a restart fail. |
| Direction of a change | one `DiffType` per direction (`COLUMN_NOT_NULL_SET` / `_DROPPED`, attached / detached, added / removed) | one type, with the direction read from the old and new values | The two directions need different guidance, and the type alone then says what happened, in the report and in callhome's `drifts_by_type`. |
| Matching keys and indexes | by OID only; a new OID is a drop plus an add | also pairing leftovers by name, to report re-creates and definition changes | Pairing needs more captured fields and about 20 more types. OID-only overstates severity in rare cases (§5.9) and never hides a change. |
| Key and index definitions | captured for display, never compared | compare the text on the same OID | Renaming a column, a referenced table or a schema changes the printed text of objects that did not change, which would give false findings. |
| Live read identity | new `LabelSourceLive`, never persisted; it IS the timeline identity | reuse an existing label; carry a second identity beside the label | `Capture` validates labels, and a persisted label would file the live read as history. Naming the label for what the snapshot is, not for the command that takes it, removes the need for a parallel identity field. |
| Live capture failure | error, exit 1 | warn and report history alone | Connecting and listing the source's tables both precede the capture, so a source that is down already fails the run. What is left is a capture that failed on a reachable source, and a report missing the newest interval reads as "no drift". |
| Report formats | `--output-format` takes one value; unset writes both | a comma-separated list | Matches `analyze-schema`. With both as the default, a list only lets a user spell out the default. |
| Where the report lives | files under `reports/` | metaDB | It is output, not state. No upgrade concern, and users can share it. |
| Table universe | union of history and live read | live catalog only | A dropped table is the case the report exists for. |
| Table-name resolution | in-memory name registry over the universe, then export data's matcher | the migration's `name_registry.json`; a matcher of its own | The stored registry cannot name a table created after export data's first run. A matcher of its own could drift from what the same flag means in export. |
| Partitioned table in a table list | expands to every partition beneath it | matches only that table | The flag means the same as in export data. Matching only the parent would drop drift on its partitions, and a dropped finding reads as no drift. |
| HTML | embedded template, view model built in Go, no JavaScript | client-side rendering of the JSON | Opens anywhere, including air-gapped hosts. Grouping logic stays testable in Go. |
| Exit codes | `0` report written · `1` error | `0` no drift · `1` drift found · `2` error | Every voyager command exits 1 on error, and so does every shared helper that fails through `utils.ErrExit`. Putting drift on 1 would make a failed run indistinguishable from a successful one that found drift. A script reads drift from `summary.change_count` in the JSON report. |
| Export-dir lock | its own per-command lock | no lock | The report files are shared state between two runs on one export dir. The lock file is named per command, so it never blocks export or import. |
| Command placement | new `schema` parent | top-level `detect-drift` | Leaves room for sibling schema tools without crowding the root. |

## 10\. Open questions

1. **Renamed tables under `--table-list`.** Alias handling in `schemadiff` is disabled, so filtering by a renamed table's name returns only the findings anchored to that name, not its full history. Unfiltered runs report both. A fix needs a cross-window alias map or anchors keyed by table OID.  
2. **Iterative cutover.** Each iteration captures into its own metaDB. Should `detect-drift` on the main export dir walk the iteration dirs and present one timeline?  
3. **Target-side and post-cutover drift.** `SnapshotHeader.Side` exists for this. Is capture from the target exporter roles wanted for fall-back and fall-forward?  
4. **Default for capture.** It is off. What evidence flips it on by default, and does the 60-minute interval stay?  
5. **Object coverage.** Sequences, views and functions are outside `schemasnapshot` today. Which come next, and does the severity vocabulary hold for them?  
6. **ID-based matching across instances.** [\#3672](https://github.com/yugabyte/yb-voyager/issues/3672): OID-based table matching assumes the same PostgreSQL instance across snapshots.

