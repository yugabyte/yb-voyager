---
name: Export-Data Refactoring Analysis
overview: Incremental refactoring plan to extract export-data logic from cmd into a new exportdata package with clean interfaces, no globals, and a feature flag to switch between old and new code paths.
todos:
  - id: phase-1
    content: "Phase 1: Package skeleton - interfaces, ExportContext struct, orchestrator shell, feature flag wiring in cmd"
  - id: phase-2
    content: "Phase 2: PGDumpExporter - first real implementation (PG/YB + snapshot-only + pg_dump)"
  - id: phase-3
    content: "Phase 3: Ora2pgExporter (Oracle/MySQL + snapshot-only + ora2pg)"
  - id: phase-4
    content: "Phase 4: DebeziumEngine + DebeziumSnapshotExporter (BETA_FAST snapshot-only)"
  - id: phase-5
    content: "Phase 5: DebeziumChangesOnlyExporter (all roles - SOURCE/TARGET_FB/TARGET_FF)"
  - id: phase-6
    content: "Phase 6: PGDumpDebeziumExporter (PG live migration - pg_dump + Debezium streaming)"
  - id: phase-7
    content: "Phase 7: DebeziumSnapshotAndChangesExporter (Oracle/MySQL/YB live migration)"
isProject: false
---

# Export-Data Refactoring Plan

## Strategy

Build a new `src/exportdata/` package alongside the existing `cmd/` code. No globals in the new package -- all state passed via structs. A feature flag in `cmd/exportData.go` routes to new or old code. Each phase adds one exporter implementation; the flag falls back to old code for unsupported combinations.

## Interfaces

```go
type SnapshotOnlyExporter interface {
    Setup() error
    ExportSnapshot(ctx context.Context) error
    PostProcessing() error
}

type SnapshotAndChangesExporter interface {
    Setup() error
    ExportSnapshot(ctx context.Context) error
    StartExportingChanges(ctx context.Context) error
    PostProcessing() error
    SupportsConcurrentSnapshotAndChanges() bool
}

type ChangesOnlyExporter interface {
    Setup() error
    StartExportingChanges(ctx context.Context) error
    PostProcessing() error
}
```

## ExportContext -- replaces globals

Everything an exporter needs is passed through this struct (constructed in `cmd`, passed to `exportdata`):

```go
type ExportContext struct {
    // Core config (immutable after construction)
    ExportDir     string
    ExportType    string
    ExporterRole  string
    MigrationUUID uuid.UUID
    StartClean    bool
    DisablePb     bool
    UseDebezium   bool

    // Source DB
    Source   *srcdb.Source
    SourceDB srcdb.SourceDB

    // Dependencies (injected)
    MetaDB       *metadb.MetaDB
    ControlPlane cp.ControlPlane

    // State (populated by orchestrator during shared setup, before exporter methods run)
    FinalTableList      []sqlname.NameTuple
    TablesColumnList    *utils.StructMap[sqlname.NameTuple, []string]
    PartitionsToRootMap map[string]string
    LeafPartitions      *utils.StructMap[sqlname.NameTuple, []sqlname.NameTuple]
}
```

Fields like `exportPhase`, `runId`, `tablesProgressMetadata`, `ybCDCClient`, and the streaming metrics counters become local state inside the exporter implementations that need them -- not on the context.

## Feature flag and routing

In `cmd/exportData.go`, within `exportDataCommandFn` (or `exportData()`), add a switch:

```go
if useNewExportDataPackage() {
    ectx := exportdata.ExportContext{
        ExportDir:     exportDir,
        ExportType:    exportType,
        ExporterRole:  exporterRole,
        MigrationUUID: migrationUUID,
        StartClean:    bool(startClean),
        DisablePb:     bool(disablePb),
        UseDebezium:   useDebezium,
        Source:        &source,
        SourceDB:      source.DB(),
        MetaDB:        metaDB,
        ControlPlane:  controlPlane,
    }
    return exportdata.Run(&ectx)
}
// ... existing code unchanged ...
```

`useNewExportDataPackage()` checks an env var (e.g., `YB_VOYAGER_REFACTORED_EXPORT=1`). During development, this is opt-in. Once all exporters are implemented and tested, it becomes the default and the old code is removed.

For **incremental rollout**, the new package can also report which combinations it supports:

```go
func IsSupported(exportType, dbType, role string, useDebezium bool) bool
```

The cmd layer calls `exportdata.IsSupported(...)` -- if true and the env flag is set, route to new code; otherwise, fall back to old code. This lets you ship one exporter at a time with confidence.

## Orchestrator in the new package

The orchestrator lives in `src/exportdata/orchestrator.go`. It handles the shared pre-export setup (currently lines 475-629 of `exportData()`) and then delegates to the appropriate exporter:

```go
func Run(ctx *ExportContext) error {
    // 1. shared setup: connect, schemas, table list, column list
    if err := commonSetup(ctx); err != nil {
        return err
    }
    // 2. shared validation
    if err := validate(ctx); err != nil {
        return err
    }
    // 3. dispatch to exporter
    switch ctx.ExportType {
    case SNAPSHOT_ONLY:
        e := newSnapshotOnlyExporter(ctx)
        if err := e.Setup(); err != nil { return err }
        if err := e.ExportSnapshot(goCtx); err != nil { return err }
        return e.PostProcessing()
    case SNAPSHOT_AND_CHANGES:
        e := newSnapshotAndChangesExporter(ctx)
        if err := e.Setup(); err != nil { return err }
        if err := e.ExportSnapshot(goCtx); err != nil { return err }
        if err := e.StartExportingChanges(goCtx); err != nil { return err }
        return e.PostProcessing()
    case CHANGES_ONLY:
        e := newChangesOnlyExporter(ctx)
        if err := e.Setup(); err != nil { return err }
        if err := e.StartExportingChanges(goCtx); err != nil { return err }
        return e.PostProcessing()
    }
}
```

`commonSetup()` handles: connect to source, get schemas, name registry init, get table list, finalize table/column lists, display table list. This is the code currently at lines 475-629 of `cmd/exportData.go`.

`validate()` handles shared checks: `CheckSourceDBVersion`, schema usage permissions.

## Design decisions

- **Cutover cleanup** goes in `PostProcessing()` (delete replication slots/publications/CDC streams, mark cutover processed, `updateCutoverDetectedFlag`). No polling loop in Go -- Debezium/Java handles detection.
- **Validation split**: Shared checks in `validate()` (version, schema permissions). Exporter-specific checks (tool dependencies, replication permissions) in each `Setup()`.
- **DebeziumEngine**: Shared struct composed into the 5 Debezium-based exporters. Encapsulates config prep, start, progress tracking, snapshot completion detection. Introduced in Phase 4.

## Incremental phases

### Phase 1: Package skeleton

**New files:**
- `src/exportdata/interfaces.go` -- the three interfaces
- `src/exportdata/context.go` -- `ExportContext` struct
- `src/exportdata/orchestrator.go` -- `Run()`, `commonSetup()`, `validate()`, factory functions
- `src/exportdata/factory.go` -- `IsSupported()`, `newSnapshotOnlyExporter()`, etc.

**Change in cmd (minimal):**
- [`cmd/exportData.go`](yb-voyager/cmd/exportData.go): Add the feature flag check + `ExportContext` construction at the top of `exportData()`. ~15 lines. Old code untouched below the `if`.

**Result:** Compiles, but `IsSupported()` returns false for everything. No behavioral change.

### Phase 2: PGDumpExporter

The simplest path: PG or YB + snapshot-only + no Debezium.

**New files:**
- `src/exportdata/pgdump_exporter.go` -- implements `SnapshotOnlyExporter`

**What gets ported:**
- `Setup()`: dependency check for pg_dump/strings, store table list in MSR, initialize table metadata, set `SnapshotMechanism = "pg_dump"`
- `ExportSnapshot()`: launch `SourceDB.ExportData()`, progress tracking, wait for completion
- `PostProcessing()`: `ExportDataPostProcessing()`, `renameDatafileDescriptor` (PG only), display row counts, control-plane events

**Key porting challenge:** `exportDataOffline()` currently uses globals (`source`, `exportDir`, `metaDB`, `controlPlane`, `exporterRole`, `tablesProgressMetadata`, `disablePb`). The new code reads all of these from `ExportContext`. `tablesProgressMetadata` becomes a field on `PGDumpExporter`.

**`IsSupported()` now returns true for:** `(SNAPSHOT_ONLY, POSTGRESQL, SOURCE, !useDebezium)` and `(SNAPSHOT_ONLY, YUGABYTEDB, SOURCE, !useDebezium)`.

### Phase 3: Ora2pgExporter

Oracle/MySQL + snapshot-only.

**New files:**
- `src/exportdata/ora2pg_exporter.go` -- implements `SnapshotOnlyExporter`

Very similar to PGDumpExporter. Main differences: dependency checks (ora2pg, sqlplus), `SnapshotMechanism = "ora2pg"`, no `renameDatafileDescriptor`.

**`IsSupported()` adds:** `(SNAPSHOT_ONLY, ORACLE, SOURCE, !useDebezium)`, `(SNAPSHOT_ONLY, MYSQL, SOURCE, !useDebezium)`.

### Phase 4: DebeziumEngine + DebeziumSnapshotExporter

Introduces shared Debezium infrastructure. Beta snapshot-only path.

**New files:**
- `src/exportdata/debezium_engine.go` -- `DebeziumEngine` struct (config prep, start, progress tracking, snapshot completion)
- `src/exportdata/debezium_snapshot_exporter.go` -- implements `SnapshotOnlyExporter` using `DebeziumEngine`

**What gets ported into DebeziumEngine:**
- `prepareDebeziumConfig()` (~250 lines from `cmd/exportDataDebezium.go:50-198`)
- `debeziumExportData()` start + snapshot progress loop (~70 lines from `cmd/exportDataDebezium.go:391-458`)
- `checkAndHandleSnapshotComplete()` snapshot portion (lines 521-534)
- SSL param preparation (`prepareSSLParamsForDebezium`)
- `writeDataFileDescriptor()`

**`IsSupported()` adds:** `(SNAPSHOT_ONLY, *, SOURCE, useDebezium)`.

### Phase 5: DebeziumChangesOnlyExporter

First streaming exporter. Handles all three roles.

**New files:**
- `src/exportdata/debezium_changes_only_exporter.go` -- implements `ChangesOnlyExporter`

**What gets ported:**
- `Setup()`: replication slot/publication creation (PG source) or YB replication slot/CDC stream (target). Sequence initial values.
- `StartExportingChanges()`: start Debezium via `DebeziumEngine`, streaming progress reporting
- `PostProcessing()`: delete replication slot/publication/CDC stream, mark cutover processed, `updateCutoverDetectedFlag`

The `ybCDCClient` becomes a field on the exporter, not a global. Streaming metrics (`totalEventCount` etc.) become local fields.

**`IsSupported()` adds:** `(CHANGES_ONLY, POSTGRESQL, SOURCE)`, `(CHANGES_ONLY, YUGABYTEDB, TARGET_FB)`, `(CHANGES_ONLY, YUGABYTEDB, TARGET_FF)`.

### Phase 6: PGDumpDebeziumExporter

PG live migration (most complex single path).

**New files:**
- `src/exportdata/pgdump_debezium_exporter.go` -- implements `SnapshotAndChangesExporter`

**What gets ported:**
- `Setup()`: create PG replication slot + publication, prepare Debezium config
- `ExportSnapshot()`: pg_dump with snapshot name (reuses PGDump logic from Phase 2)
- `StartExportingChanges()`: configure Debezium with `snapshotMode=never` + replication slot, start via `DebeziumEngine`
- `PostProcessing()`: delete PG replication slot + publication, mark cutover, fallback setup
- `SupportsConcurrentSnapshotAndChanges()` returns `true`

**`IsSupported()` adds:** `(SNAPSHOT_AND_CHANGES, POSTGRESQL, SOURCE)`.

### Phase 7: DebeziumSnapshotAndChangesExporter

Oracle/MySQL/YB live migration.

**New files:**
- `src/exportdata/debezium_snapshot_and_changes_exporter.go` -- implements `SnapshotAndChangesExporter`

**What gets ported:**
- `Setup()`: prepare Debezium config with `snapshotMode=initial`
- `ExportSnapshot()`: start Debezium via `DebeziumEngine`, wait for snapshot completion
- `StartExportingChanges()`: Debezium already running; monitor streaming progress
- `PostProcessing()`: mark cutover processed
- `SupportsConcurrentSnapshotAndChanges()` returns `false`

**`IsSupported()` adds:** `(SNAPSHOT_AND_CHANGES, ORACLE|MYSQL|YUGABYTEDB, SOURCE)`.

After this phase, `IsSupported()` returns true for all 12 combinations. The env flag can be flipped to default-on, and the old code can eventually be removed.