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
	Tables      []schemasnapshot.ObjectRef // empty = all; matched against the finding's anchor table
	ObjectTypes []ObjectType               // empty = all; matched against the finding's ObjectType
}
```

One positive allow-list per dimension. The previous shape carried an include and an exclude list per dimension; "empty" was ambiguous and callers could pass combinations with no defined meaning. Resolving a user's `--exclude-*` flag into a keep-set is the caller's job because only the caller knows the full universe.

### 3.2 `schemasnapshot.LabelDetectDrift` (added in \#3814)

```go
const LabelDetectDrift = "detect_drift" // accepts no reason; never persisted
```

`Capture` validates its label against the known vocabulary. The live read taken by `detect-drift` needs a label to pass that validation, and must not collide with a persisted label.

### 3.3 `schemadrift` inputs

```go
const SeriesSourceLive = "source_live"
```

The `Series` value of the live read. Stored snapshots use their capture label as Series.

```go
type SnapshotInput struct {
	Header  schemasnapshot.SnapshotHeader
	Content *schemasnapshot.SnapshotContent // nil for a failed capture
	Series  string                          // Header.Label for stored snapshots, SeriesSourceLive for the live read
}
```

One point in the chronological sequence handed to `BuildReport`.

```go
type BuildParams struct {
	Source              Source
	Schemas             []string        // display only
	Snapshots           []SnapshotInput // oldest first; the live read, if any, is last
	Scope               schemadiff.Scope
	Tables              []string        // what was compared, resolved; see Comparing
	TablesFiltered      bool
	ObjectTypes         []string
	ObjectTypesFiltered bool
	GeneratedAt         time.Time
}
```

The complete input to `BuildReport`. Plain data, no connections or handles, so the assembler is testable with fixtures.

```go
func BuildReport(p BuildParams) Report
```

Walks `p.Snapshots` oldest-first, diffs each comparable pair, and assembles the report. Rules in §5.2.

### 3.4 `schemadrift` classification

```go
type Status string

const (
	StatusAdvisory            Status = "advisory"
	StatusPotentialImpact     Status = "potential_impact"
	StatusBreaksRecoverable   Status = "breaks_migration_recoverable"
	StatusBreaksUnrecoverable Status = "breaks_migration_unrecoverable"
)
```

Severity answers "what does the migration do about this change", not "how alarming is the DDL". Vocabulary in §5.4.

```go
type classification struct {
	Status Status
	Impact string
	Action string
}

var classificationByDiffType map[schemadiff.DiffType]classification

func classify(t schemadiff.DiffType) classification
```

Severity and its explanatory note are one value in one map. Two parallel maps keyed by `DiffType` would let them disagree silently, and an entry with a severity but no note would render a finding the report cannot explain. `classify` returns `StatusAdvisory` with empty Impact and Action for a `DiffType` the map does not know.

### 3.5 `schemadrift` renderers (\#3813)

```go
func RenderJSON(r Report) ([]byte, error)
func RenderHTML(r Report) ([]byte, error)
```

JSON is the indented marshalling of `Report`. HTML is a single self-contained page from an embedded template with no external assets and no JavaScript. Identifiers render with minimal quoting so a printed object path is valid, copy-pasteable SQL: `sales."MixedCase"` when needed, `public.orders` otherwise.

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
| Exit codes | `0` no drift · `1` drift found · `2` operational error (flags, connection, unreadable snapshot) |
| Config file | section `schema-detect-drift` with keys `log-level`, `output-format`, the four list flags |
| State | read-only; writes only under `reports/` |

## 4\. Data model

### 4.1 Report

The JSON report is an output file, not migration state, so upgrades never read an old report with a new binary. The compatibility concern is the other way: users and tooling parse the JSON. Field names and tags are contractual; changes are additive and any non-additive change bumps `version`.

```go
type Report struct {
	Report      string            `json:"report"`   // always "schema_drift"
	Version     int               `json:"version"`  // 1
	GeneratedAt time.Time         `json:"generated_at"`
	Source      Source            `json:"source"`
	Window      Window            `json:"window"`   // first capture to last capture
	Comparing   Comparing         `json:"comparing"`
	Summary     Summary           `json:"summary"`
	Diffs       []DiffEntry       `json:"diffs"`
	Captures    []Capture         `json:"captures"` // every point on the timeline, placeholders included
	Skipped     []SkippedInterval `json:"skipped,omitempty"`
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
	Schemas             []string `json:"schemas"`
	Tables              []string `json:"tables"`       // what was compared, not what was typed
	TablesFiltered      bool     `json:"tables_filtered"`
	ObjectTypes         []string `json:"object_types"`
	ObjectTypesFiltered bool     `json:"object_types_filtered"`
}

type Summary struct {
	ChangeCount  int  `json:"change_count"`
	CaptureCount int  `json:"capture_count"` // stored snapshots only, live read excluded
	LiveCompared bool `json:"live_compared"`
}

type DiffEntry struct {
	Seq        int                      `json:"seq"`                 // 1-based, continuous across intervals
	Type       string                   `json:"type"`                // schemadiff.DiffType
	Operation  string                   `json:"operation"`           // ADDED | DROPPED | CHANGED
	ObjectType string                   `json:"object_type"`         // TABLE | COLUMN
	Attribute  string                   `json:"attribute,omitempty"` // "" for ADDED/DROPPED
	Object     schemasnapshot.ObjectRef `json:"object"`              // the table
	SubObject  string                   `json:"sub_object,omitempty"` // the column, when ObjectType is COLUMN
	Status     string                   `json:"status"`              // Status
	OldValue   any                      `json:"old_value,omitempty"`
	NewValue   any                      `json:"new_value,omitempty"`
	Window     Window                   `json:"window"`              // the interval it was detected in
	Phase      string                   `json:"phase,omitempty"`     // §5.3
	Impact     string                   `json:"impact,omitempty"`
	Action     string                   `json:"action,omitempty"`
}

type SkippedInterval struct {
	From   string `json:"from"`   // Series of the earlier capture
	To     string `json:"to"`
	Window Window `json:"window"`
	Reason string `json:"reason"`
}

type Capture struct {
	Series     string    `json:"series"`
	Reason     string    `json:"reason,omitempty"`
	CapturedAt time.Time `json:"captured_at"`
}
```

`Comparing` states what was compared, not what the user typed. Unfiltered, `Tables` is the whole universe; filtered, it is the resolved keep-set. The `*Filtered` flags tell the two apart so an empty list never has to mean two things.

`Skipped` exists because an interval the assembler declined to compare is otherwise indistinguishable from one that had no changes. It is `omitempty` because a normal run has none.

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
 │      each becomes schemadrift.SnapshotInput{Header, Content, Series: Header.Label}
 │
 ├─ cmd.captureLiveSnapshotForDrift(schemas)                  → *SnapshotInput | nil                          §5.6
 │      └─ schemasnapshot.Capture(ctx, db, CaptureParams{Label: LabelDetectDrift})       cmd → schemasnapshot
 │         result carries Series: SeriesSourceLive; appended as the last input
 │
 ├─ cmd.buildDriftTableCandidates(contents, live)             → table universe                                §5.5
 ├─ cmd.resolveDriftTableRefs / complementDriftTableRefs      → []ObjectRef, then schemadiff.Scope            §5.5
 │
 ├─ schemadrift.BuildReport(BuildParams)                      → Report                   cmd → schemadrift
 │      ├─ schemadiff.NewDiffer(Config{Scope})
 │      ├─ per comparable pair (§5.2): differ.Diff(prev, next) → []Difference            schemadrift → schemadiff
 │      ├─ phaseFor(prevCapture, nextCapture)                 → phase string                                   §5.3
 │      └─ per Difference: classify(d.Type)                   → classification, folded into DiffEntry        §5.4
 │
 ├─ cmd.writeDriftReports(report, formats)
 │      ├─ schemadrift.RenderJSON(Report)                     → []byte                   cmd → schemadrift
 │      └─ schemadrift.RenderHTML(Report)                     → []byte                   cmd → schemadrift
 │         written to <export-dir>/reports/drift_analysis_report.{json,html}
 │
 └─ cmd.printDriftSummary(report, paths); exit 0 or 1 by Summary.ChangeCount
```

Everything above `BuildReport` is `cmd` assembling plain data; everything inside it is pure computation over that data; everything after it is serialisation. The four decision points that follow are the places where two implementations of this flow could diverge on the same input.

### 5.2 Which pairs are compared

**Where:** `schemadrift.BuildReport`, the walk over `BuildParams.Snapshots`. **In:** `[]SnapshotInput`, oldest first. **Out:** the `(prev, next)` pairs handed to `Differ.Diff`, plus `Report.Skipped` and `Report.Captures`. **Decides:** which two snapshots form an interval, and what to do with inputs that cannot be an interval's side.

The walk keeps a *baseline*: the most recent snapshot eligible to be the older side of a comparison.

| Current input | Baseline | Action |
| :---- | :---- | :---- |
| `Content == nil` (failed capture) | any | Record as a `Capture`. Leave the baseline alone, so the next real snapshot compares back across the gap. |
| has content | none yet | Becomes the baseline. Nothing to compare. |
| has content, `Header.Schemas` set differs from baseline's | set | Record a `SkippedInterval` with the reason. Current becomes the baseline. |
| has content, same schema set | set | Diff baseline → current through `Differ` with `p.Scope`. Each `Difference` becomes a `DiffEntry` carrying the interval's `Window` and `Phase`. Current becomes the baseline. |

A failed capture is *bridged*, not a boundary. The drift that happened around it is still real; what is lost is only the ability to say which side of the failed capture it fell on, and the wider `Window` on the entry says so.

A schema-set mismatch is *skipped*, not compared. Two snapshots covering different schemas would report every table in the extra schema as added or dropped. Set comparison ignores order and duplicates.

`Seq` increments across the whole report, including across intervals that produced no entries, so a reader can refer to "finding 7" unambiguously.

`Report.Window` spans the first to the last `Capture`, placeholders and the live read included. `Summary.CaptureCount` counts stored snapshots only; the live read is reported through `LiveCompared`.

### 5.3 Phase

**Where:** `schemadrift.phaseFor(prev, next Capture) string`, called once per interval from `BuildReport`. **In:** the two `Capture.Series` values bracketing an interval. **Out:** `DiffEntry.Phase`. **Decides:** what the migration was doing while the interval's drift appeared.

Unlisted pairs get `""` and the renderer shows the window alone.

| Earlier Series | Later Series | Phase |
| :---- | :---- | :---- |
| `export_schema` | `export_data_from_source_start` | `export data: pending` |
| `…_start` or `…_periodic` | `…_periodic` or `…_exit` | `export data: running` |
| `export_data_from_source_exit` | `export_data_from_source_start` | `export data: paused` |
| any | `source_live` | `since last capture` |
| anything else |  | `""` |

The exit capture's `Reason` (`cutover`, `complete`, `interrupt`, `error`) says how a run ended and is shown on the timeline; the phase says what was happening while the drift appeared. A start→exit pair is therefore "running", whatever the exit reason.

### 5.4 Severity

**Where:** `schemadrift.classify(t schemadiff.DiffType) classification`, backed by `classificationByDiffType`, called once per `Difference` from `BuildReport`. **In:** a `DiffType`. **Out:** `DiffEntry.Status`, `Impact`, `Action`. **Decides:** what the migration does about a change, and the note that explains it.

| Status | Meaning | DiffTypes |
| :---- | :---- | :---- |
| `breaks_migration_unrecoverable` | `export data` keeps running but cannot be restarted; the migration must restart from scratch | `TABLE_DROPPED`, `TABLE_NAME_CHANGED`, `TABLE_SCHEMA_CHANGED` |
| `breaks_migration_recoverable` | `import data` can fail until the same DDL is applied on the target, then resumes | `COLUMN_ADDED`, `COLUMN_NAME_CHANGED`, `COLUMN_TYPE_CHANGED`, `COLUMN_NULLABILITY_CHANGED` |
| `potential_impact` | the migration is unaffected but source and target now diverge; reconcile before cutover | `TABLE_ADDED`, `COLUMN_DROPPED`, `COLUMN_DEFAULT_CHANGED`, `TABLE_KIND_CHANGED`, `TABLE_PARTITION_PARENT_CHANGED`, `TABLE_PARTITION_CHILDREN_CHANGED` |
| `advisory` | informational; inheritance changes and any `DiffType` the map does not know | `TABLE_INHERITS_CHANGED`, `TABLE_INHERITED_BY_CHANGED`, unknown |

The ordering reads backwards from a DDL point of view and that is deliberate. An added column is more severe than a dropped one because incoming rows carry a value the target cannot store, while a dropped column only leaves the target with an extra column. A dropped table is the worst case because the stored table list still expects it, so the next `export data` restart fails.

Every entry in the map carries a non-empty Impact and Action. Backticks in the text become inline code in HTML and stay literal in JSON.

### 5.5 Table universe and scope resolution

**Where:** `cmd.buildDriftTableCandidates`, `cmd.resolveDriftTableRefs`, `cmd.complementDriftTableRefs`, and the object-type equivalents, before `BuildReport`. **In:** the live catalog, every loaded `SnapshotContent`, the live read, and the four list flags. **Out:** `schemadiff.Scope` for `BuildParams.Scope`, and the resolved lists and flags for `Comparing`. **Decides:** what a `--table-list` pattern can name, and how an exclude list becomes the positive allow-list `Scope` expects.

The set of tables a pattern can match is the union of three sources: the live catalog, every loadable stored snapshot, and the live read. A table dropped from the source but present in history is therefore still addressable, which is the case where the user most needs the report.

| Flag | Resolution |
| :---- | :---- |
| neither list flag | `Scope.Tables` empty (all); `Comparing.Tables` \= the universe, `TablesFiltered=false` |
| `--table-list` | patterns resolved against the universe with the same glob matcher as export; unknown pattern is an operational error |
| `--exclude-table-list` | resolved the same way, then complemented against the universe; an empty result is an operational error |
| both | operational error |

`--object-type-list` and `--exclude-object-type-list` follow the same shape over `{TABLE, COLUMN}`.

### 5.6 Live read

**Where:** `cmd.captureLiveSnapshotForDrift(schemas) *schemadrift.SnapshotInput`, after history is loaded and before the universe is built. **In:** the open source connection and the resolved schema list. **Out:** the last `SnapshotInput`, with Series `source_live`, or nil. **Decides:** whether the report ends at the last stored snapshot or at now.

The command captures the current source schema in memory under `LabelDetectDrift` and appends it as the last input. It uses the same schema list as the stored snapshots so the pair is comparable. If the live capture fails, the report is built from history alone and the summary says so.

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
| Non-PostgreSQL source | none (capture is a no-op) | exits 2 | Oracle and MySQL are non-goals (§1). |

## 7\. Failure modes

| Situation | Behaviour | Why this and not the alternative |
| :---- | :---- | :---- |
| A capture fails during export | placeholder header written, export unaffected, warning logged | Capture is best effort and off the data path. It must never fail a migration. |
| Placeholder in history | bridged (§5.2); appears on the timeline as a failed marker | Dropping it would hide that a capture was attempted; making it a boundary would hide real drift. |
| Snapshot blob has an unsupported `Version` | operational error, exit 2 | A newer voyager wrote it. Silently skipping would produce a report that looks complete. |
| Snapshot header exists but blob cannot be loaded for any other reason | warning, treated like a placeholder | The moment is still on the timeline; the report bridges across it. |
| Schema sets differ between consecutive snapshots | interval skipped and recorded (§5.2) | Comparing would report every table in the differing schema; dropping the interval silently would look like "no changes". |
| Live capture fails or the source is unreachable | connection failure is exit 2; capture failure after connecting warns and continues history-only | The user asked for the live comparison, but history alone is still a useful report. |
| No or one stored snapshot | warning; report reflects only the live read or the single interval | Not an error: the user may simply not have enabled capture. The `--help` text names the prerequisite. |
| `DiffType` not in the classification map | `advisory`, no Impact or Action, note omitted in the render | Dropping the change would hide it. |
| Report file already exists | overwritten with a notice | Reports are regenerated, not versioned. |

## 8\. Hot-path statement

None. `detect-drift` runs once per invocation over a handful of snapshots. Capture during export runs once per interval with a 10-second budget and is independent of the data path. Nothing here runs per row or per event.

## 9\. Alternatives considered

| Decision | Chosen | Rejected | Why |
| :---- | :---- | :---- | :---- |
| Scope shape | one allow-list per dimension | include \+ exclude lists per dimension | Empty was ambiguous; callers could pass undefined combinations. Only the CLI knows the universe needed to complement an exclude list. |
| Failed capture | bridge across it | treat as an interval boundary; drop it from the timeline | Drift around it is real. The wider window is the honest answer. |
| Schema-set mismatch | skip and record | compare anyway; error out | Comparing produces noise; erroring blocks the whole report for one bad pair. |
| Severity and note | one struct in one map | parallel maps keyed by `DiffType` | Parallel maps drift silently. A test asserts every mapped type has a note. |
| Severity semantics | what the migration does | how alarming the DDL is | The user's question is "is my migration broken", not "was this a big change". |
| Live read identity | new `LabelDetectDrift`, never persisted; timeline identity is a separate `Series` | reuse an existing label; add a label that is also persisted | `Capture` validates labels and a persisted label would file the live read as history. |
| Where the report lives | files under `reports/` | metaDB | It is output, not state. No upgrade concern, and users can share it. |
| Table universe | union of live catalog, history, live read | live catalog only | A dropped table is the case the report exists for. |
| HTML | embedded template, view model built in Go, no JavaScript | client-side rendering of the JSON | Opens anywhere, including air-gapped hosts. Grouping logic stays testable in Go. |
| Exit codes | 0 / 1 / 2 | always 0; 0 / non-zero | Lets a script gate cutover on "no drift" without parsing the report. |
| Command placement | new `schema` parent | top-level `detect-drift` | Leaves room for sibling schema tools without crowding the root. |

## 10\. Open questions

1. **Renamed tables under `--table-list`.** Alias handling in `schemadiff` is disabled, so filtering by a renamed table's name returns only the findings anchored to that name, not its full history. Unfiltered runs report both. A fix needs a cross-window alias map or anchors keyed by table OID.  
2. **Iterative cutover.** Each iteration captures into its own metaDB. Should `detect-drift` on the main export dir walk the iteration dirs and present one timeline?  
3. **Target-side and post-cutover drift.** `SnapshotHeader.Side` exists for this. Is capture from the target exporter roles wanted for fall-back and fall-forward?  
4. **Default for capture.** It is off. What evidence flips it on by default, and does the 60-minute interval stay?  
5. **Object coverage.** Indexes, constraints, sequences, and functions are outside `schemasnapshot` today. Which come next, and does the severity vocabulary hold for them?  
6. **ID-based matching across instances.** [\#3672](https://github.com/yugabyte/yb-voyager/issues/3672): OID-based table matching assumes the same PostgreSQL instance across snapshots.

