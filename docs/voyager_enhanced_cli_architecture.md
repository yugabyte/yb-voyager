# Voyager Enhanced CLI Architecture

## Current vs Proposed Command Structure

### Current Command Hierarchy

```
yb-voyager
│
├── assess-migration                    # Requires export-dir
├── assess-migration-bulk               # Fleet assessment (Oracle only)
│
├── export
│   ├── schema
│   └── data
│
├── analyze-schema
│
├── import
│   ├── schema
│   └── data
│       ├── status
│       └── [to target | to source | to source-replica]
│
├── export data
│   ├── status
│   └── from target
│
├── get data-migration-report
│
├── initiate cutover
│   ├── to target
│   ├── to source
│   └── to source-replica
│
├── cutover status
│
├── archive changes
│
├── finalize-schema-post-data-import
│
└── end migration
```

**Problems:**
- Commands are scattered without clear hierarchy
- No status visibility without knowing specific status commands
- No guidance on command sequence
- Assessment tightly coupled with migration workflow

---

### Proposed Command Hierarchy

```
yb-voyager
│
├── assessment                          # STANDALONE - No target DB needed
│   ├── generate-scripts                # Create SQL scripts for DBAs
│   ├── ingest-metadata                 # Import collected metadata
│   ├── run                             # Direct assessment (non-prod)
│   ├── report                          # Generate assessment report
│   ├── fleet                           # Fleet-level assessment
│   │   ├── assess                      # Assess all in fleet config
│   │   ├── compare                     # Cross-cluster comparison
│   │   └── report                      # Consolidated fleet report
│   └── view                            # Interactive report viewer
│
├── migrate                             # MIGRATION - Target DB required
│   ├── init                            # Initialize migration (registers in target DB)
│   ├── status                          # ⭐ Show current state & progress
│   ├── next                            # ⭐ Recommend next action
│   ├── history                         # View migration timeline
│   │
│   ├── export                          
│   │   ├── schema                      # Export schema from source
│   │   └── data                        # Export data from source
│   │       └── status                  
│   │
│   ├── analyze-schema                  # Schema compatibility check
│   │
│   ├── import
│   │   ├── schema                      # Import schema to target
│   │   └── data                        # Import data to target
│   │       └── status
│   │
│   ├── verify                          # Data verification (new)
│   │
│   ├── cutover
│   │   ├── initiate                    # Start cutover
│   │   └── status                      # Cutover progress
│   │
│   ├── finalize                        # Post-migration tasks
│   │
│   └── end                             # Cleanup and finish
│
├── fleet                               # FLEET OPERATIONS
│   ├── init                            # Initialize fleet migration
│   ├── status                          # Fleet-wide status
│   ├── export                          # Coordinate exports
│   ├── import                          # Coordinate imports
│   └── report                          # Consolidated reports
│
├── ai                                  # VOYAGER AI
│   ├── convert                         # Schema conversion help
│   ├── optimize                        # Query optimization suggestions
│   └── troubleshoot                    # Diagnose issues
│
└── version
```

---

## State Machine: Migration Phases

```
┌────────────────────────────────────────────────────────────────────────────────────┐
│                                 MIGRATION STATE MACHINE                            │
└────────────────────────────────────────────────────────────────────────────────────┘

                              ┌─────────────────┐
                              │   ASSESSMENT    │ (Optional, Standalone)
                              │   NOT_STARTED   │
                              └────────┬────────┘
                                       │ assessment run/report
                                       ▼
                              ┌─────────────────┐
                              │   ASSESSMENT    │
                              │   COMPLETED     │
                              └────────┬────────┘
                                       │
                    ───────────────────┴───────────────────
                   │                                       │
                   │ Start Migration                       │ (Assessment is optional)
                   ▼                                       │
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                              MIGRATION WORKFLOW                                      │
│                         (Requires Target DB Connection)                              │
└─────────────────────────────────────────────────────────────────────────────────────┘
                                       │
                                       ▼
                              ┌─────────────────┐
                              │  INITIALIZED    │ ◀── migrate init
                              │  (State in YB)  │
                              └────────┬────────┘
                                       │
                    ┌──────────────────┼──────────────────┐
                    │                  │                  │
                    ▼                  ▼                  │
           ┌───────────────┐  ┌───────────────┐          │
           │ EXPORT_SCHEMA │  │ EXPORT_SCHEMA │          │
           │ IN_PROGRESS   │  │   COMPLETED   │──────────┤
           └───────┬───────┘  └───────────────┘          │
                   │                                      │
                   ▼                                      │
           ┌───────────────┐  ┌───────────────┐          │
           │ANALYZE_SCHEMA │  │ANALYZE_SCHEMA │          │
           │ IN_PROGRESS   │─▶│   COMPLETED   │──────────┤
           └───────────────┘  └───────────────┘          │
                                      │                  │
                                      ▼                  │
                              ┌───────────────┐          │
                              │ IMPORT_SCHEMA │          │
                              │ IN_PROGRESS   │          │
                              └───────┬───────┘          │
                                      │                  │
                                      ▼                  │
                              ┌───────────────┐          │
                              │ IMPORT_SCHEMA │──────────┤
                              │   COMPLETED   │          │
                              └───────┬───────┘          │
                                      │                  │
         ┌────────────────────────────┼────────────────────────────┐
         │                            │                            │
         ▼                            ▼                            │
┌─────────────────┐          ┌─────────────────┐                   │
│  EXPORT_DATA    │          │  EXPORT_DATA    │                   │
│  IN_PROGRESS    │─────────▶│   COMPLETED     │───────────────────┤
└─────────────────┘          └────────┬────────┘                   │
         │                            │                            │
         │ (Parallel in Live Mode)    │                            │
         ▼                            ▼                            │
┌─────────────────┐          ┌─────────────────┐                   │
│  IMPORT_DATA    │          │  IMPORT_DATA    │                   │
│  IN_PROGRESS    │─────────▶│   COMPLETED     │───────────────────┤
└─────────────────┘          └────────┬────────┘                   │
                                      │                            │
                                      ▼                            │
                              ┌───────────────┐                    │
                              │    VERIFY     │                    │
                              │  IN_PROGRESS  │                    │
                              └───────┬───────┘                    │
                                      │                            │
                                      ▼                            │
                              ┌───────────────┐                    │
                              │    VERIFY     │                    │
                              │   COMPLETED   │────────────────────┤
                              └───────┬───────┘                    │
                                      │                            │
                                      ▼                            │
                              ┌───────────────┐                    │
                              │   CUTOVER     │                    │
                              │   INITIATED   │                    │
                              └───────┬───────┘                    │
                                      │                            │
                                      ▼                            │
                              ┌───────────────┐                    │
                              │   CUTOVER     │                    │
                              │   COMPLETED   │────────────────────┤
                              └───────┬───────┘                    │
                                      │                            │
                                      ▼                            │
                              ┌───────────────┐                    │
                              │   MIGRATION   │ ◀── migrate end    │
                              │    ENDED      │                    │
                              └───────────────┘                    │
```

---

## Data Flow: Assessment Phase (Standalone)

```
┌────────────────────────────────────────────────────────────────────────────────────┐
│                           ASSESSMENT DATA FLOW                                      │
│                         (No Target DB Required)                                     │
└────────────────────────────────────────────────────────────────────────────────────┘

  OPTION A: Direct Connection (Dev/Staging)
  ═══════════════════════════════════════════

    ┌─────────────┐          ┌─────────────────┐          ┌─────────────────┐
    │   Source    │          │    Voyager      │          │   Assessment    │
    │  Database   │◀────────▶│    Machine      │─────────▶│   Directory     │
    │             │  Query   │                 │  Write   │                 │
    └─────────────┘          └─────────────────┘          └─────────────────┘
                                                                   │
    Command:                                                       │
    yb-voyager assessment run \                                    ▼
      --source-db-host prod.example.com \              ┌─────────────────────┐
      --assessment-dir ./assessment                    │  assessment/        │
                                                       │  ├── metadata/      │
                                                       │  ├── reports/       │
                                                       │  │   ├── *.json     │
                                                       │  │   └── *.html     │
                                                       │  └── sizing/        │
                                                       └─────────────────────┘

  OPTION B: SQL Scripts (Production - DBA Friendly)
  ═══════════════════════════════════════════════════

    Step 1: Generate Scripts                    Step 2: DBA Runs Scripts
    ───────────────────────                     ────────────────────────

    ┌─────────────────┐                        ┌─────────────┐
    │    Voyager      │                        │   Source    │
    │    Machine      │                        │  Database   │
    └────────┬────────┘                        └──────┬──────┘
             │                                        │
             │ generate-scripts                       │ DBA executes
             ▼                                        ▼
    ┌─────────────────┐                        ┌─────────────────┐
    │  scripts/       │  ────── Handoff ─────▶ │  collected/     │
    │  ├── pg_*.sql   │        to DBA          │  ├── *.csv      │
    │  ├── ora_*.sql  │                        │  └── *.json     │
    │  └── README.md  │                        └─────────────────┘
    └─────────────────┘                                │
                                                       │
    Step 3: Ingest Metadata                   Step 4: Generate Report
    ───────────────────────                   ────────────────────────

    ┌─────────────────┐                        ┌─────────────────┐
    │  collected/     │                        │   Assessment    │
    │  ├── *.csv      │                        │   Report        │
    │  └── *.json     │                        │   (JSON + HTML) │
    └────────┬────────┘                        └─────────────────┘
             │                                        ▲
             │ ingest-metadata                        │
             ▼                                        │
    ┌─────────────────┐        generate-report       │
    │  assessment/    │─────────────────────────────▶│
    │  └── metadata/  │
    └─────────────────┘

    Commands:
    1. yb-voyager assessment generate-scripts --source-db-type oracle --output-dir ./scripts
    2. (DBA runs scripts on production, saves to ./collected)
    3. yb-voyager assessment ingest-metadata --metadata-dir ./collected --assessment-dir ./assessment
    4. yb-voyager assessment report --assessment-dir ./assessment

```

---

## Data Flow: Migration Phase (Target DB as Source of Truth)

```
┌────────────────────────────────────────────────────────────────────────────────────┐
│                           MIGRATION DATA FLOW                                       │
│                      (Target DB as Source of Truth)                                 │
└────────────────────────────────────────────────────────────────────────────────────┘

    ┌─────────────────────────────────────────────────────────────────────────────┐
    │                              VOYAGER MACHINE                                 │
    │  ┌──────────────────────────────────────────────────────────────────────┐   │
    │  │                         yb-voyager CLI                                │   │
    │  │                                                                       │   │
    │  │  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────────────┐  │   │
    │  │  │  export   │  │  analyze  │  │  import   │  │      migrate      │  │   │
    │  │  │  schema   │  │  schema   │  │  schema   │  │  status / next    │  │   │
    │  │  └─────┬─────┘  └─────┬─────┘  └─────┬─────┘  └─────────┬─────────┘  │   │
    │  │        │              │              │                  │            │   │
    │  │        ▼              ▼              ▼                  │            │   │
    │  │  ┌─────────────────────────────────────────────────────────────────┐ │   │
    │  │  │                    Migration State Manager                      │ │   │
    │  │  │                                                                 │ │   │
    │  │  │  • Reads state from Target DB                                   │ │   │
    │  │  │  • Writes progress to Target DB                                 │ │   │
    │  │  │  • Stores reports as JSON in Target DB                          │ │   │
    │  │  │  • Maintains local cache for performance                        │ │   │
    │  │  └────────────────────────────────────────────────────────────────────┘ │   │
    │  │                                    │                                │   │
    │  └────────────────────────────────────┼────────────────────────────────┘   │
    │                                       │                                    │
    │  Local Export Directory               │                                    │
    │  (Temporary Storage)                  │                                    │
    │  ┌──────────────────────────────┐     │                                    │
    │  │  export-dir/                 │     │                                    │
    │  │  ├── schema/                 │◀────┤ Schema files                       │
    │  │  │   └── *.sql               │     │                                    │
    │  │  ├── data/                   │◀────┤ Data files                         │
    │  │  │   └── *.sql               │     │                                    │
    │  │  └── reports/                │◀────┤ HTML reports (optional local copy) │
    │  │      └── *.html              │     │                                    │
    │  └──────────────────────────────┘     │                                    │
    │                                       │                                    │
    └───────────────────────────────────────┼────────────────────────────────────┘
                                            │
                    ┌───────────────────────┴───────────────────────┐
                    │                                               │
                    ▼                                               ▼
    ┌───────────────────────────────┐               ┌───────────────────────────────┐
    │        SOURCE DATABASE        │               │    TARGET YUGABYTEDB          │
    │                               │               │                               │
    │  • PostgreSQL / Oracle / MySQL│               │  ┌─────────────────────────┐  │
    │  • Read-only access           │               │  │  ybvoyager schema       │  │
    │  • Schema + Data export       │               │  │                         │  │
    │                               │               │  │  • migrations           │  │
    └───────────────────────────────┘               │  │  • migration_state      │  │
                                                    │  │  • reports (JSONB)      │  │
                                                    │  │  • table_progress       │  │
                                                    │  │  • events               │  │
                                                    │  └─────────────────────────┘  │
                                                    │                               │
                                                    │  ┌─────────────────────────┐  │
                                                    │  │  User Database          │  │
                                                    │  │  (migrated_db)          │  │
                                                    │  │                         │  │
                                                    │  │  • Migrated schema      │  │
                                                    │  │  • Migrated data        │  │
                                                    │  └─────────────────────────┘  │
                                                    │                               │
                                                    └───────────────────────────────┘
                                                                    │
                                                                    │
                    ┌───────────────────────────────────────────────┘
                    │
                    ▼
    ┌───────────────────────────────────────────────────────────────────────────────┐
    │                          YugabyteDB UI / YBM Console                          │
    │                                                                               │
    │  ┌─────────────────────────────────────────────────────────────────────────┐ │
    │  │                        Migrations Dashboard                              │ │
    │  │                                                                          │ │
    │  │   ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌──────────────────┐  │ │
    │  │   │  Active    │  │ Completed  │  │ Assessment │  │  Next Step       │  │ │
    │  │   │ Migrations │  │ Migrations │  │  Reports   │  │  Guidance        │  │ │
    │  │   └────────────┘  └────────────┘  └────────────┘  └──────────────────┘  │ │
    │  │                                                                          │ │
    │  │   All data read directly from ybvoyager schema in Target DB             │ │
    │  │   No separate backend service required                                   │ │
    │  │                                                                          │ │
    │  └─────────────────────────────────────────────────────────────────────────┘ │
    │                                                                               │
    └───────────────────────────────────────────────────────────────────────────────┘
```

---

## CLI Output Examples

### Example 1: `migrate status`

```
$ yb-voyager migrate status --target-db-host yb.example.com --migration-name prod-migration

┌──────────────────────────────────────────────────────────────────────────────────────┐
│  MIGRATION STATUS                                                                    │
│                                                                                      │
│  Name: prod-migration                                                                │
│  UUID: 550e8400-e29b-41d4-a716-446655440000                                         │
│  Source: postgresql://source.example.com:5432/prod_db                               │
│  Target: yugabytedb://yb.example.com:5433/migrated_db                               │
├──────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                      │
│  PHASE PROGRESS                                                                      │
│                                                                                      │
│  ✓ Initialize         COMPLETED     ████████████████████  100%   0h 01m             │
│  ✓ Export Schema      COMPLETED     ████████████████████  100%   0h 05m             │
│  ✓ Analyze Schema     COMPLETED     ████████████████████  100%   0h 02m             │
│  ✓ Import Schema      COMPLETED     ████████████████████  100%   0h 08m             │
│  ● Export Data        IN_PROGRESS   ████████████░░░░░░░░   62%   2h 15m (est: 1h)   │
│  ○ Import Data        NOT_STARTED   ░░░░░░░░░░░░░░░░░░░░    0%   -                  │
│  ○ Verify             NOT_STARTED   ░░░░░░░░░░░░░░░░░░░░    0%   -                  │
│  ○ Cutover            NOT_STARTED   ░░░░░░░░░░░░░░░░░░░░    0%   -                  │
│                                                                                      │
├──────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                      │
│  CURRENT ACTIVITY: Export Data                                                       │
│                                                                                      │
│  Table                    │ Status      │ Exported       │ Total          │ Rate    │
│  ─────────────────────────┼─────────────┼────────────────┼────────────────┼─────────│
│  public.customers         │ DONE        │ 1,250,000      │ 1,250,000      │ -       │
│  public.orders            │ EXPORTING   │ 3,456,789      │ 5,500,000      │ 12K/s   │
│  public.order_items       │ QUEUED      │ 0              │ 15,000,000     │ -       │
│  public.products          │ QUEUED      │ 0              │ 85,000         │ -       │
│                                                                                      │
├──────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                      │
│  ▶ NEXT STEP:                                                                        │
│                                                                                      │
│  Wait for export to complete, then run:                                              │
│                                                                                      │
│    yb-voyager migrate import data \                                                  │
│      --target-db-host yb.example.com \                                               │
│      --migration-uuid 550e8400-e29b-41d4-a716-446655440000                          │
│                                                                                      │
└──────────────────────────────────────────────────────────────────────────────────────┘
```

### Example 2: `migrate next`

```
$ yb-voyager migrate next --target-db-host yb.example.com --migration-name prod-migration

┌──────────────────────────────────────────────────────────────────────────────────────┐
│  RECOMMENDED NEXT ACTION                                                             │
│                                                                                      │
│  Migration: prod-migration                                                           │
│  Current Phase: Export Data (IN_PROGRESS - 62%)                                      │
├──────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                      │
│  ⏳ ACTION: Wait for export to complete                                               │
│                                                                                      │
│  The export is currently in progress. Once completed, run:                           │
│                                                                                      │
│  ┌────────────────────────────────────────────────────────────────────────────────┐ │
│  │ yb-voyager migrate import data \                                               │ │
│  │   --target-db-host yb.example.com \                                            │ │
│  │   --target-db-user admin \                                                     │ │
│  │   --migration-uuid 550e8400-e29b-41d4-a716-446655440000                       │ │
│  └────────────────────────────────────────────────────────────────────────────────┘ │
│                                                                      [Copy] [Run]    │
│                                                                                      │
│  TIPS:                                                                               │
│  • You can start import data while export is still running for live migrations       │
│  • Use --parallel-jobs to control parallelism (default: auto-detected)               │
│  • Monitor progress with: yb-voyager migrate status                                  │
│                                                                                      │
└──────────────────────────────────────────────────────────────────────────────────────┘
```

### Example 3: `assessment run`

```
$ yb-voyager assessment run --source-db-type postgresql --source-db-host prod.example.com

┌──────────────────────────────────────────────────────────────────────────────────────┐
│  ASSESSMENT IN PROGRESS                                                              │
│                                                                                      │
│  Source: postgresql://prod.example.com:5432/prod_db                                  │
│  Target Version: 2024.1.0.0 (latest stable)                                          │
├──────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                      │
│  [■■■■■■■■■■■■■■■■■░░░] 85%                                                          │
│                                                                                      │
│  ✓ Connected to source database                                                      │
│  ✓ Gathered schema metadata (247 objects)                                            │
│  ✓ Collected table statistics                                                        │
│  ✓ Analyzed index usage                                                              │
│  ● Capturing IOPS metrics (2m remaining)                                             │
│  ○ Generating report                                                                 │
│                                                                                      │
├──────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                      │
│  PRELIMINARY FINDINGS:                                                               │
│                                                                                      │
│  Migration Complexity: MEDIUM                                                        │
│                                                                                      │
│  • 3 unsupported features detected                                                   │
│  • 12 performance optimization opportunities                                         │
│  • Estimated data size: 125 GB                                                       │
│  • Recommended cluster: 3 nodes x 8 vCPU, 32GB RAM                                   │
│                                                                                      │
└──────────────────────────────────────────────────────────────────────────────────────┘
```

---

## Benefits Summary

| Benefit | Impact |
|---------|--------|
| **Standalone Assessment** | DBAs can evaluate migrations without committing to target DB |
| **SQL Script Generation** | Production-safe assessment via DBA-executed scripts |
| **Target DB as Source of Truth** | Single source of migration state, enables UI integration |
| **Guided Workflow** | `status` and `next` commands eliminate documentation dependency |
| **Fleet Operations** | Coordinated multi-database migrations at scale |
| **UI Integration** | Real-time dashboards without additional backend services |
| **AI Integration** | Clear touchpoints for schema conversion and optimization |

---

## Migration Path

### Backward Compatibility

1. Existing `export-dir` based migrations will continue to work
2. New migrations can opt-in to target DB storage
3. Gradual migration of existing state to target DB
4. Legacy commands remain available with deprecation warnings

### Upgrade Path

```bash
# For existing migrations
yb-voyager migrate upgrade \
  --export-dir /path/to/export \
  --target-db-host yb.example.com

# This will:
# 1. Read existing state from local metadb
# 2. Register migration in target DB
# 3. Sync all state to target DB
# 4. Enable new status/next commands
```

