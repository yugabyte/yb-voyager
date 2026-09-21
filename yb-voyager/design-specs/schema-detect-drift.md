# Schema drift detection: `schema detect-drift`

|  |  |
| :---- | :---- |
| **Status** | Draft |
| **Author** | Shivansh Gahlot |
| **Tracking** | [\#3617](https://github.com/yugabyte/yb-voyager/issues/3617) · [DB-21962](https://yugabyte.atlassian.net/browse/DB-21962) |
| **Implementation** | [\#3811](https://github.com/yugabyte/yb-voyager/pull/3811) → [\#3812](https://github.com/yugabyte/yb-voyager/pull/3812) → [\#3813](https://github.com/yugabyte/yb-voyager/pull/3813) → [\#3814](https://github.com/yugabyte/yb-voyager/pull/3814) → [\#3815](https://github.com/yugabyte/yb-voyager/pull/3815) → [\#3817](https://github.com/yugabyte/yb-voyager/pull/3817) |
| **Contractual** | §3 public surface, §4 data model, §5 rules, §6 flow matrix. Everything else is advisory. |

## 1\. Context

Voyager already records point-in-time snapshots of the PostgreSQL source schema at migration milestones (`schemasnapshot`, [\#3614](https://github.com/yugabyte/yb-voyager/pull/3614), [\#3655](https://github.com/yugabyte/yb-voyager/pull/3655)) and can compute the table- and column-level differences between any two snapshots (`schemadiff`, [\#3648](https://github.com/yugabyte/yb-voyager/pull/3648)). Nothing yet turns that history into something a user can act on. When `import data` fails on a column the target does not have, or when a table added mid-migration silently never arrives, the user has no way to learn that the source schema moved, when it moved, or what to do about it.

This design adds the layer above the diff engine that owns *drift*: which snapshot pairs are comparable, what a change means for the migration in flight, and a command that writes the answer as a report.

**Requirements this design depends on**

- A user can run one read-only command against an export directory and get a report of every table or column change on the source since `export schema`, including changes since the last stored snapshot.  
- Every reported change carries a severity that answers "what does the migration do about this", and a corrective action.  
- Each change is attributed to the interval between two captures and labelled with what the migration was doing then.  
- The report is available as HTML for reading and JSON for tooling. The JSON shape is stable across releases.  
- Scoping mirrors the export commands: `--table-list` / `--exclude-table-list` and `--object-type-list` / `--exclude-object-type-list`.

**Non-goals for this version**

- Objects other than tables and columns. Coverage matches what `schemasnapshot` captures.  
- Sources other than PostgreSQL.  
- Detecting drift on the target, or on the source after cutover-to-target.  
- Applying, suggesting, or generating DDL. The report says what to do; the user does it.  
- Preventing drift (event triggers, locks). Tracked separately in [\#3681](https://github.com/yugabyte/yb-voyager/issues/3681).  
- Turning capture on by default. It stays behind `--disable-schema-snapshot-capture=false`.

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

**What the given layers provide.** `schemasnapshot` captures a `SchemaSnapshot` (a `SnapshotHeader` stored in metaDB columns plus a `SnapshotContent` blob) under a label from a fixed vocabulary: `export_schema`, `export_data_from_source_start`, `export_data_from_source_periodic`, `export_data_from_source_exit`. A capture that fails writes a placeholder header with no content so the moment still appears on the timeline. `schemadiff.Diff(a, b)` returns a sorted `[]Difference` over tables and columns, each tagged with a `DiffType` such as `COLUMN_TYPE_CHANGED`; `Differ` applies a `Scope` after diffing.

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
	Source    Source
	Snapshots []schemasnapshot.SchemaSnapshot // oldest first; the live read, if any, is last
	Scope     schemadiff.Scope                // the exact sets compared; Comparing is rendered from it
}
```

The complete input to `BuildReport`. Plain data, no connections or handles, so the assembler is testable with fixtures.

`Comparing.Tables` and `Comparing.ObjectTypes` are rendered from `Scope`, which holds the exact sets compared, so they are not passed separately. The report does not say whether a list flag narrowed those sets: `Scope` cannot carry that, and the listed sets already tell a reader what "no drift" covers.

```go
func BuildReport(p DetectionConfig) (Report, error)
```

Walks `p.Snapshots` oldest-first, diffs each comparable pair, and assembles the report. Rules in §5.2. `Report.GeneratedAt` is stamped here from the wall clock rather than passed in: it describes the act of building the report, not the data being reported on. It returns an error for a finding whose identity is neither a table nor a table-scoped object: that is a new engine object kind the report does not know how to place, and emitting it with an empty object would read as a real finding on nothing.

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

var infoByDiffType map[schemadiff.DiffType]DriftInfo

func getDriftInfo(t schemadiff.DiffType) DriftInfo
```

Severity and its explanatory note are one value in one map. Two parallel maps keyed by `DiffType` would let them disagree silently, and an entry with a severity but no note would render a finding the report cannot explain. `getDriftInfo` returns `SeverityAdvisory` with empty Impact and Action for a `DiffType` the map does not know.

`DriftInfo` is what enriches a raw `schemadiff.Difference` into drift: what the change means for the migration in flight. It is exported and embedded in `DriftEntry` rather than copied field by field. Today `getDriftInfo` keys on the DiffType alone, so every entry of a given type carries the same three values; deriving them per finding (from the old/new values) would not change the shape.

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
    [--output-format html,json] \
    [--table-list <globs> | --exclude-table-list <globs>] \
    [--object-type-list TABLE,COLUMN | --exclude-object-type-list ...]
```

|  |  |
| :---- | :---- |
| Parent | new `schema` command for standalone schema tooling outside the export/import workflow |
| Source type | PostgreSQL only; any other value is an operational error |
| Output | `<export-dir>/reports/drift_analysis_report.html` and `.json`, overwritten on each run |
| Exit codes | `0` the report was written, whether or not it found drift · `1` error (flags, connection, unreadable snapshot). A script reads drift from `summary.change_count` in the JSON report. |
| Config file | section `schema-detect-drift` with keys `log-level`, `output-format`, the four list flags; commented-out in the four migration templates |
| Table scope | an unqualified `--table-list` entry needs a default schema, so `--source-db-schema` without `public` must qualify every entry as `schema.table` |
| State | read-only; writes only under `reports/` |

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
	ObjectType schemadiff.ObjectType `json:"object_type"`         // TABLE | COLUMN
	Attribute  schemadiff.Attribute  `json:"attribute,omitempty"` // "" for ADDED/DROPPED

	// What it changed on, and to what.
	Object    schemasnapshot.ObjectRef `json:"object"`               // the table
	SubObject string                   `json:"sub_object,omitempty"` // the column, when ObjectType is COLUMN
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

None added. The `schema_snapshots` metaDB table and the `SnapshotContent` blob format are owned by `schemasnapshot` and unchanged. The live read is held in memory and never saved.

## 5\. Behaviour

### 5.1 End-to-end flow

One invocation of `schema detect-drift`, as the sequence of calls that carry data across the layers in §2. Types are the ones declared in §3 and §4. Unexported names inside a package are the author's current intent; the crossings between packages and the types passed across them are the contract.

```
cmd.detectDrift()
 │
 ├─ schemasnapshot.ListSnapshots(metaDB)                      → []SnapshotHeader         cmd → schemasnapshot
 ├─ per header: schemasnapshot.LoadSnapshotByName(metaDB, name) → *SnapshotContent | nil  cmd → schemasnapshot
 │      each becomes a schemasnapshot.SchemaSnapshot{Header, Content}
 │
 ├─ cmd.captureLiveSnapshotForDrift(schemas)                  → *SchemaSnapshot | nil                         §5.6
 │      └─ schemasnapshot.Capture(ctx, db, CaptureParams{Label: LabelSourceLive})        cmd → schemasnapshot
 │         its Header.Label IS the timeline identity; appended as the last input
 │
 ├─ cmd.buildDriftTableCandidates(contents, live)             → table universe                                §5.5
 ├─ cmd.resolveDriftTableRefs / complementDriftTableRefs      → []ObjectRef, then schemadiff.Scope            §5.5
 │
 ├─ schemadrift.BuildReport(DetectionConfig)                  → Report, error            cmd → schemadrift
 │      ├─ schemadiff.NewDiffer(Config{Scope})
 │      ├─ per comparable pair (§5.2): differ.Diff(prev, next) → []Difference            schemadrift → schemadiff
 │      ├─ phaseFor(prevCapture, nextCapture)                 → phase string                                   §5.3
 │      └─ per Difference: getDriftInfo(d.Type)               → DriftInfo, embedded in DriftEntry             §5.4
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

### 5.4 Severity

**Where:** `schemadrift.getDriftInfo(t schemadiff.DiffType) DriftInfo`, backed by `infoByDiffType`, called once per `Difference` from `BuildReport`. **In:** a `DiffType`. **Out:** the `DriftInfo` embedded in each `DriftEntry` -- `Severity`, `Impact`, `Action`. **Decides:** what the migration does about a change, and the note that explains it.

| Severity | Meaning | DiffTypes |
| :---- | :---- | :---- |
| `breaks_migration_unrecoverable` | `export data` keeps running but cannot be restarted; the migration must restart from scratch | `TABLE_DROPPED`, `TABLE_NAME_CHANGED`, `TABLE_SCHEMA_CHANGED` |
| `breaks_migration_recoverable` | `import data` can fail until the same DDL is applied on the target, then resumes | `COLUMN_ADDED`, `COLUMN_NAME_CHANGED`, `COLUMN_TYPE_CHANGED`, `COLUMN_NULLABILITY_CHANGED` |
| `potential_impact` | the migration is unaffected but source and target now diverge; reconcile before cutover | `TABLE_ADDED`, `COLUMN_DROPPED`, `COLUMN_DEFAULT_CHANGED`, `TABLE_KIND_CHANGED`, `TABLE_PARTITION_PARENT_CHANGED`, `TABLE_PARTITION_CHILDREN_CHANGED` |
| `advisory` | informational; inheritance changes and any `DiffType` the map does not know | `TABLE_INHERITS_CHANGED`, `TABLE_INHERITED_BY_CHANGED`, unknown |

The ordering reads backwards from a DDL point of view and that is deliberate. An added column is more severe than a dropped one because incoming rows carry a value the target cannot store, while a dropped column only leaves the target with an extra column. A dropped table is the worst case because the stored table list still expects it, so the next `export data` restart fails.

Every entry in the map carries a non-empty Impact and Action. Backticks in the text become inline code in HTML and stay literal in JSON.

### 5.5 Table universe and scope resolution

**Where:** `cmd.buildDriftTableCandidates`, `cmd.resolveDriftTableRefs`, `cmd.complementDriftTableRefs`, and the object-type equivalents, before `BuildReport`. **In:** the live catalog, every loaded `SnapshotContent`, the live read, and the four list flags. **Out:** `schemadiff.Scope` for `DetectionConfig.Scope`. **Decides:** what a `--table-list` pattern can name, and how an exclude list becomes the positive allow-list `Scope` expects.

The set of tables a pattern can match is the union of three sources: the live catalog, every loadable stored snapshot, and the live read. A table dropped from the source but present in history is therefore still addressable, which is the case where the user most needs the report.

| Flag | Resolution |
| :---- | :---- |
| neither list flag | `Scope.Tables` \= the whole universe, passed explicitly; `Comparing.Tables` is that same set |
| `--table-list` | patterns resolved against the universe with the same glob matcher as export; unknown pattern is an operational error |
| `--exclude-table-list` | resolved the same way, then complemented against the universe; an empty result is an operational error (the report would be empty, so the command says so rather than emitting one) |
| both | operational error |

`--object-type-list` and `--exclude-object-type-list` follow the same shape over `{TABLE, COLUMN}`.

All four list flags are normalised before use: a value that is empty once trimmed (`"  "`, `","`) counts as unset, so it cannot report itself as a filter that narrowed nothing.

### 5.6 Live read

**Where:** `cmd.captureLiveSnapshotForDrift(schemas) *schemasnapshot.SchemaSnapshot`, after history is loaded and before the universe is built. **In:** the open source connection and the resolved schema list. **Out:** the last snapshot, labelled `source_live`, or nil. **Decides:** whether the report ends at the last stored snapshot or at now.

The command captures the current source schema in memory under `LabelSourceLive` and appends it as the last input. It is captured with exactly `--source-db-schema`, so it always covers the request; history may cover more, and the extra schemas' findings are filtered rather than the comparison declined. If the live capture fails, the report is built from history alone and `LiveCompared` says so.

## 6\. Migration-flow matrix

Capture happens in `export schema` and, when the exporter role is the source exporter, at `export data` start, every `--schema-snapshot-capture-interval` minutes (default 60), and at exit. It is a no-op unless `--disable-schema-snapshot-capture=false` and the source is PostgreSQL. `detect-drift` reads whatever `<export-dir>/metainfo/meta.db` holds.

| Flow | Captures | detect-drift | Notes |
| :---- | :---- | :---- | :---- |
| Offline | export schema, export data start / periodic / exit(complete) | covered |  |
| Live, snapshot \+ changes | as offline; periodic continues through streaming; exit reason `cutover` | covered | The cutover footer (\#3815) nudges the user to run it before confirming. |
| Live with fall-back | source side as above; `export data from target` takes no captures | covered for the source, up to cutover | Source-side DDL after cutover-to-target is only visible through the live read. Target-side drift is a non-goal (§1). |
| Live with fall-forward | same as fall-back | same |  |
| Changes-only | export schema, export data start / periodic / exit; no `pg_dump`, but capture is gated on role, not on export type | covered |  |
| Iterative cutover | each iteration's source exporter captures into that iteration's own metaDB | per iteration only | `--export-dir` pointed at the main dir sees the main metaDB; pointed at an iteration dir sees only that iteration. No cross-iteration timeline. Open question §9. |
| Non-PostgreSQL source | none (capture is a no-op) | error, exit 1 | Oracle and MySQL are non-goals (§1). |

## 7\. Failure modes

| Situation | Behaviour | Why this and not the alternative |
| :---- | :---- | :---- |
| A capture fails during export | placeholder header written, export unaffected, warning logged | Capture is best effort and off the data path. It must never fail a migration. |
| Placeholder in history | bridged (§5.2); appears on the timeline as a failed marker | Dropping it would hide that a capture was attempted; making it a boundary would hide real drift. |
| Snapshot blob has an unsupported `Version` | error, exit 1 | A newer voyager wrote it. Silently skipping would produce a report that looks complete. |
| Snapshot header exists but blob cannot be loaded for any other reason | warning, treated like a placeholder | The moment is still on the timeline; the report bridges across it. |
| A capture did not cover the requested schemas | bridged, and recorded on its `CapturePoint` (§5.2) | A requested table missing from it means nobody looked, not that it was dropped. Treating it as a boundary lost every interval around it. |
| Live capture fails or the source is unreachable | connection failure is an error, exit 1; capture failure after connecting warns and continues history-only | The user asked for the live comparison, but history alone is still a useful report. |
| One stored snapshot, and the live read makes it a pair | warning; report covers that single interval | Not an error: one stored capture plus the live read is a real interval. Zero stored snapshots gets no warning, because it can never form an interval and always lands on the row below. |
| No comparable pair at all (`ComparedIntervalCount == 0`) | error, exit 1; the message is derived from what the assembler recorded, and names only the case that actually occurred | An empty report reads as "no drift". The three causes — no captures stored, none usable, only one usable — need different advice, so the message must not assert a cause it did not observe. Capture cannot be enabled retroactively, so "re-run the export" is never the remedy for the run in hand. |
| `DiffType` not in the classification map | `advisory`, no Impact or Action, note omitted in the render | Dropping the change would hide it. |
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
| Live read identity | new `LabelSourceLive`, never persisted; it IS the timeline identity | reuse an existing label; carry a second identity beside the label | `Capture` validates labels, and a persisted label would file the live read as history. Naming the label for what the snapshot is, not for the command that takes it, removes the need for a parallel identity field. |
| Where the report lives | files under `reports/` | metaDB | It is output, not state. No upgrade concern, and users can share it. |
| Table universe | union of live catalog, history, live read | live catalog only | A dropped table is the case the report exists for. |
| HTML | embedded template, view model built in Go, no JavaScript | client-side rendering of the JSON | Opens anywhere, including air-gapped hosts. Grouping logic stays testable in Go. |
| Exit codes | `0` report written · `1` error | `0` no drift · `1` drift found · `2` error | Every voyager command exits 1 on error, and so does every shared helper that fails through `utils.ErrExit`. Putting drift on 1 would make a failed run indistinguishable from a successful one that found drift. A script reads drift from `summary.change_count` in the JSON report. |
| Command placement | new `schema` parent | top-level `detect-drift` | Leaves room for sibling schema tools without crowding the root. |

## 10\. Open questions

1. **Renamed tables under `--table-list`.** Alias handling in `schemadiff` is disabled, so filtering by a renamed table's name returns only the findings anchored to that name, not its full history. Unfiltered runs report both. A fix needs a cross-window alias map or anchors keyed by table OID.  
2. **Iterative cutover.** Each iteration captures into its own metaDB. Should `detect-drift` on the main export dir walk the iteration dirs and present one timeline?  
3. **Target-side and post-cutover drift.** `SnapshotHeader.Side` exists for this. Is capture from the target exporter roles wanted for fall-back and fall-forward?  
4. **Default for capture.** It is off. What evidence flips it on by default, and does the 60-minute interval stay?  
5. **Object coverage.** Indexes, constraints, sequences, and functions are outside `schemasnapshot` today. Which come next, and does the severity vocabulary hold for them?  
6. **ID-based matching across instances.** [\#3672](https://github.com/yugabyte/yb-voyager/issues/3672): OID-based table matching assumes the same PostgreSQL instance across snapshots.

