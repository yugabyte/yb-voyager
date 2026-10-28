# schemadrift Package — Engineering Standards

> **Scope:** files under `yb-voyager/src/schema/schemadrift/` (including `templates/drift_report.html`), and its callers: `cmd/detectDrift.go` (the `schema detect-drift` command, `checkSchemaDrift`, `checkSchemaDriftOnExportFailure`), `checkSchemaDriftBeforeCutover` in `cmd/cutoverToTargetCmd.go`, `src/errs/schemadrift.go`, and the drift callhome payload. These are standards for **writing** code here, not only for reviewing it.
> Repo-wide standards live in `AGENTS.md` at each parent directory up to the repo root — read those as well. Design: `yb-voyager/design-specs/schema-detect-drift.md`. Lower layers: `src/schemasnapshot/AGENTS.md`, `src/schemadiff/AGENTS.md`.

## Before you change anything

- Edit the design spec first, then the code. The spec states current behaviour only — never "previously", "no longer", or "replaces an earlier rule". The same goes for code comments: no PR numbers, review thread references, or mockup names.
- Every guidance claim about what voyager does (what fails, what a restart needs) must be checked on a real PG/YB run before you write it. Several plausible claims were wrong.

## Layering

- `BuildReport` takes plain data (`DetectionConfig`: `Source`, oldest-first `[]SchemaSnapshot` with the live read last as `LabelSourceLive`, `Scope`, `MigrationInfo`) and does no I/O. It sets `GeneratedAt` itself. Do not add flags the report can derive (`TablesFiltered`), a wrapper around snapshots, or a separate live-read input.
- `cmd` reads snapshots and the MSR, takes the live read, resolves scope, and writes files. Shared drift code lives in `cmd/detectDrift.go` under its section banners; do not add new `cmd/` files for drift.
- Callers run the check in process through `checkSchemaDrift(driftCheckInput{...})`. Never spawn `yb-voyager schema detect-drift` as a subprocess.

## Report contract

- The JSON is read by scripts (`summary.change_count`). Never rename or remove a tag; bump `Report.Version` on a shape change.
- `DriftEntry` embeds `Diff` and `DriftInfo`. Do not embed `schemadiff.Difference`: it has interface identities that cannot be decoded and no JSON tags, so its field set would become the published contract.
- Names were settled in review; do not reintroduce rejected ones: `DetectionConfig` (not `DetectionInput`/`BuildParams`), `DriftInfo` (not `DiffTypeInfo`), `Severity` (not `Status`), `getDriftInfo` (not `classify`), `CapturePoints` (not `Captures`/`Timeline`), `infoByDiffType` (no stutter). A name must not mean two things in one scope.
- `ComparedIntervalCount == 0` is not a clean run. Do not infer "nothing compared" from `StoredCaptureCount` (it counts failed captures) or from empty `Drifts`.
- Mention the live read only when there is one (`Report.HasLiveRead()`). `detect-drift` and the cutover pre-check take one; the export-failure check does not.

## Which snapshots are compared

- Compare each usable snapshot with the nearest earlier usable one. Bridge two kinds: a failed capture (nil `Content`), and one missing any requested schema (`missingSchemas`) — a table absent from it was never looked at. Record the reason on `CapturePoint.Excluded`. Never drop a bridged point and never make it a boundary.
- A capture with extra schemas is compared; `Scope` removes the extras.

## Guidance: severity, Impact, Action

- Severity answers "what does the migration do", not "how scary is the DDL". An added column is recoverable (import data fails on it); a dropped one is potential impact.
- Keep the table flat: `infoByDiffType map[DiffType]map[Timing]DriftInfo`, plus `fallBackRiskByDiffType` and `fallForwardActionByDiffType`. Text shared across timings goes in named vars in one block. Do not add a wrapper struct, default entries, or helpers like `withChangesOnlyAsLive`. No golden testdata file for the text.
- Every type lists `TimingBeforeExportData`, `TimingOfflineAfterExportStart`, `TimingLiveAfterExportStart`. Add `TimingChangesOnlyAfterExportStart` only when it differs from live — it does wherever live says to restart with `--start-clean`, which changes-only rejects, so it says to start a new migration in a new export directory. `TimingAfterCutover` is one shared `afterCutoverInfo` with no Action.
- A missing entry is an error in `getDriftInfo`, never a fallback to advisory.
- Pick guidance by the timing of the interval the change happened in, not when the command runs. Before export data there is one severity, since offline vs live is not known yet.
- Writing rules: assume the workload already follows the new schema; never tell users to undo their source change unless voyager needs it; offline Actions say "before the application moves to the target"; state failures plainly, without softening; name commands and flags in backticks (they become `<code>` in HTML).
- The fall-back sentence goes on Impact while a fall-back cutover is still possible; the fall-forward sentence goes on every Action except `TABLE_PARTITION_CHILDREN_CHANGED` while fall-forward is in use before cutover. Never both.

## Timing and phase

- An unknown export type is an error, not a default.
- Before export data: the export type is empty, or the next export-data capture after the interval is a start with reason `initial` or `clean_restart`.
- After cutover only when the interval starts at or after the cutover request, or at an exit capture with reason `cutover`.
- A new `schemasnapshot` label or reason needs a case in `phaseFor`, `timingFor`, `isBeforeExportData` and `deriveEvent`.

## Rendering

- No panics anywhere. Unexpected states return errors up through `BuildReport` and `RenderHTML` (unknown operation, severity or value type, empty name, an interval with no capture point). The one tolerance: `deriveEvent` ignores an unknown label, which a newer voyager may have written.
- The HTML is self-contained: inline CSS, no external assets, no JavaScript. Build display strings in Go so they are unit-tested. Escape first, then add markup (`codeSpans`).
- Render names with `ForDisplay`; schema lists with `displaySchemas` so they quote like table names. Adjacent timeline markers with nothing between them mean "no change"; do not add filler.

## Command and in-process checks

- Exit codes: 0 when the report is written, drift or not; 1 on any error, through `utils.ErrExit`. No exit 2, no raw `os.Exit`.
- Zero compared intervals is an error. Its message names the case that happened, and never tells the user to re-run export — capture cannot be added after the fact.
- A live-read failure is an error. Validators do not mutate flags (`resolveDetectDriftFlagDefaults` sets, `validateDetectDriftFlags` checks).
- Resolve `--table-list` with export's machinery: an in-memory name registry over stored snapshots plus the live read (so dropped tables stay nameable), `extractTableListFromString`, partitioned tables expanded to their descendants.
- `checkSchemaDrift` reads the MSR fresh: initiate cutover writes `ToTargetRequestedAt` while export data runs.
- Export data runs the check only after Debezium fails once streaming has started (`errs.DebeziumStreamingError`). Not on Ctrl-C, guardrails, `ErrExit` paths, or non-source roles. No live read, no exit-code change, and it runs after "Export of data failed!". Do not add drift hints to atexit handlers or generic failure paths.
- The cutover pre-check is skipped with `--skip-pre-checks schema_drift`. A non-PG source skips with a log line. Without a terminal it refuses on drift. Never mention the hidden `--yes`. On "no", print exactly: "Cutover not started. To cut over without the drift check, pass --skip-pre-checks schema_drift." Scope it with `msr.TableListExportedFromSource`; add no MSR field.
- Errors use `errs.NewSchemaDriftError(errs.SCHEMA_DRIFT_STEP_*, err)` at the call site. Do not add a home-grown wrapper.
- Callhome sends counts only: no error text, names, paths, or values. Bump `SCHEMA_DRIFT_CALLHOME_PAYLOAD_VERSION` for a structural or semantic change to the payload, not for a new enum value.
- Do not add guards for inputs no caller can produce.

## Testing

- `unit` tests here build `SchemaSnapshot`/`Report` literals. Unit tests of the `cmd` drift helpers (`checkSchemaDrift`, the cutover pre-check, the callhome payload) stay in `cmd/detectDrift_test.go`. End-to-end tests that run the built binary go in `src/testschemadrift` (`integration_voyager_command`); add no new `cmd` test files.
- New guidance: extend `TestGetDriftInfo_SeverityPerTiming` and the completeness tests.
- Pair selection: cover failed, schema-subset and superset captures, a final live read, and fewer than two usable captures. Rendering: test case-sensitive names (`"Sales"."Orders"`) and each error path.
- Keep CI lean: one test per rule, no JSON round trips, no per-kind copies of the same render test, and no tautological checks.
