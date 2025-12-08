# Voyager User Journey Enhancement Proposal

## Executive Summary

This document outlines the proposed enhancements to YB-Voyager's user experience, addressing current challenges in CLI usability, documentation alignment, UI integration, fleet-scale migrations, and Voyager AI workflow. The goal is to transform Voyager from a powerful but complex migration tool into an intuitive, guided experience that scales from single database migrations to fleet-level operations.

---

## Current State Analysis

### Existing Command Structure

```
yb-voyager
├── assess-migration              # Pre-migration assessment
├── assess-migration-bulk         # Fleet-level assessment
├── export
│   ├── schema                    # Export schema from source
│   └── data                      # Export data from source
├── analyze-schema                # Schema compatibility analysis
├── import
│   ├── schema                    # Import schema to target
│   └── data                      # Import data to target
├── export data from target       # Live migration fallback
├── import data to source-replica # Live migration fall-forward
├── initiate cutover to target    # Cutover commands
├── cutover status                # Status check
├── get data-migration-report     # Migration report
├── archive changes               # Archive CDC changes
├── finalize-schema-post-data-import
└── end migration                 # Cleanup
```

### Current State Storage
- **Location**: `export-dir/metainfo/meta.db` (SQLite)
- **Scope**: Local filesystem for all artifacts
- **UI Integration**: Limited YugabyteD control plane integration

### Identified Gaps

| Challenge | Impact | Current Workaround |
|-----------|--------|-------------------|
| No clear migration state visibility | Users unsure of current progress | Manual tracking via documentation |
| Next-step guidance absent | Risk of incorrect command sequence | Heavy documentation dependency |
| Assessment tightly coupled with migration | Cannot assess without export-dir | Must initialize migration project |
| Local filesystem metadata | Cannot scale to fleet operations | Manual coordination across machines |
| Limited UI integration | Voyager page in YBM underutilized | CLI-only workflows |

---

## Proposed User Journey

### Phase 1: Assessment (Standalone Pre-Migration)

**Key Principle**: Assessment should be completely independent of migration execution.

```
┌─────────────────────────────────────────────────────────────────────┐
│                         ASSESSMENT PHASE                            │
│                    (No Target DB Required)                          │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌──────────────────┐    ┌──────────────────┐    ┌───────────────┐ │
│  │ Gather Metadata  │───▶│ Analyze & Report │───▶│ Size Target   │ │
│  │ (SQL Scripts)    │    │                  │    │ (Recommend)   │ │
│  └──────────────────┘    └──────────────────┘    └───────────────┘ │
│           │                       │                      │         │
│           ▼                       ▼                      ▼         │
│   Production-safe         Assessment Report      Cluster sizing    │
│   data collection         (JSON + HTML)          recommendations   │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

#### Proposed New Commands

```bash
# 1. Generate SQL scripts for production DBAs to run
yb-voyager assessment generate-scripts \
  --source-db-type oracle \
  --output-dir /path/to/scripts

# 2. Ingest gathered metadata (collected by scripts)
yb-voyager assessment ingest-metadata \
  --assessment-dir /path/to/assessment \
  --metadata-dir /path/to/collected-data

# 3. Generate assessment report (offline analysis)
yb-voyager assessment generate-report \
  --assessment-dir /path/to/assessment \
  --target-db-version 2024.1.0.0

# 4. Fleet assessment with comparison
yb-voyager assessment fleet \
  --fleet-config fleet.yaml \
  --assessment-dir /path/to/fleet-assessment \
  --compare-mode  # Enable cross-cluster comparison

# 5. Interactive assessment viewer (optional)
yb-voyager assessment view \
  --assessment-dir /path/to/assessment \
  --port 8080  # Local web UI for report exploration
```

#### Assessment Report Storage
```
assessment-dir/
├── metadata/
│   ├── schema-objects.csv
│   ├── table-statistics.csv
│   ├── index-info.csv
│   └── iops-metrics.csv
├── reports/
│   ├── assessment_report.json    # Machine-readable
│   └── assessment_report.html    # Human-readable
├── sizing/
│   ├── cluster_recommendations.json
│   └── sizing_details.json
└── fleet/                         # For bulk assessments
    ├── comparison_matrix.json
    └── consolidated_report.html
```

---

### Phase 2: Migration (Target DB Required)

**Key Principle**: Target DB is the source of truth for all migration state.

```
┌─────────────────────────────────────────────────────────────────────┐
│                         MIGRATION PHASE                             │
│                    (Target DB Required)                             │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌────────────┐   ┌────────────┐   ┌────────────┐   ┌───────────┐  │
│  │ Initialize │──▶│ Export     │──▶│ Import     │──▶│ Finalize  │  │
│  │ Migration  │   │ (Schema+   │   │ (Schema+   │   │ & Cutover │  │
│  │            │   │  Data)     │   │  Data)     │   │           │  │
│  └────────────┘   └────────────┘   └────────────┘   └───────────┘  │
│        │                │                │                │        │
│        ▼                ▼                ▼                ▼        │
│   State stored     Progress in      State synced      Migration    │
│   in Target DB     Target DB        continuously      completed    │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

#### Proposed New Commands

```bash
# 1. Initialize migration (requires target DB)
yb-voyager migrate init \
  --migration-name "prod-to-yb-2024" \
  --source-db-type postgresql \
  --target-db-host yb-cluster.example.com \
  --target-db-user admin \
  --target-db-name migrated_db

# 2. Check migration status (reads from target DB)
yb-voyager migrate status \
  --target-db-host yb-cluster.example.com \
  --migration-uuid <uuid>  # or --migration-name

# 3. Get next recommended step
yb-voyager migrate next \
  --target-db-host yb-cluster.example.com \
  --migration-uuid <uuid>

# 4. View migration history/timeline
yb-voyager migrate history \
  --target-db-host yb-cluster.example.com \
  --migration-uuid <uuid>
```

#### Target DB Metadata Schema

```sql
-- Migration registry
CREATE TABLE ybvoyager.migrations (
    migration_uuid UUID PRIMARY KEY,
    migration_name TEXT,
    source_db_type TEXT,
    source_db_version TEXT,
    source_schemas TEXT[],
    target_db_name TEXT,
    created_at TIMESTAMPTZ,
    current_phase TEXT,
    current_status TEXT,
    voyager_version TEXT
);

-- Migration state machine
CREATE TABLE ybvoyager.migration_state (
    migration_uuid UUID REFERENCES ybvoyager.migrations,
    phase TEXT,
    step TEXT,
    status TEXT,  -- NOT_STARTED, IN_PROGRESS, COMPLETED, FAILED
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    metadata JSONB,
    PRIMARY KEY (migration_uuid, phase, step)
);

-- Migration reports (JSON for UI rendering)
CREATE TABLE ybvoyager.migration_reports (
    migration_uuid UUID REFERENCES ybvoyager.migrations,
    report_type TEXT,  -- assessment, schema_analysis, data_migration
    report_data JSONB,
    generated_at TIMESTAMPTZ,
    PRIMARY KEY (migration_uuid, report_type)
);

-- Table-level progress
CREATE TABLE ybvoyager.table_progress (
    migration_uuid UUID,
    table_name TEXT,
    schema_name TEXT,
    phase TEXT,
    total_rows BIGINT,
    completed_rows BIGINT,
    errored_rows BIGINT,
    status TEXT,
    updated_at TIMESTAMPTZ,
    PRIMARY KEY (migration_uuid, schema_name, table_name, phase)
);
```

---

### Phase 3: CLI Enhancement Details

#### 3.1 Status Command Enhancement

```bash
$ yb-voyager migrate status --target-db-host yb.example.com

╔══════════════════════════════════════════════════════════════════════════════╗
║                        MIGRATION STATUS                                       ║
║  Migration: prod-to-yb-2024                                                   ║
║  UUID: 550e8400-e29b-41d4-a716-446655440000                                  ║
╠══════════════════════════════════════════════════════════════════════════════╣
║                                                                               ║
║  Phase                    Status          Progress        Duration            ║
║  ─────────────────────────────────────────────────────────────────────────── ║
║  ✓ Assessment             COMPLETED       100%            2h 15m              ║
║  ✓ Export Schema          COMPLETED       100%            5m                  ║
║  ✓ Analyze Schema         COMPLETED       100%            1m                  ║
║  ✓ Import Schema          COMPLETED       100%            3m                  ║
║  ● Export Data            IN_PROGRESS     67%             4h 30m              ║
║  ○ Import Data            NOT_STARTED     -               -                   ║
║  ○ Verify Data            NOT_STARTED     -               -                   ║
║  ○ Cutover                NOT_STARTED     -               -                   ║
║                                                                               ║
╠══════════════════════════════════════════════════════════════════════════════╣
║  Current Activity:                                                            ║
║  Exporting table: public.orders (145,000 / 217,000 rows)                     ║
║                                                                               ║
║  ▶ NEXT STEP: Wait for export data to complete, then run:                    ║
║    yb-voyager import data --target-db-host yb.example.com \                  ║
║      --migration-uuid 550e8400-e29b-41d4-a716-446655440000                   ║
╚══════════════════════════════════════════════════════════════════════════════╝
```

#### 3.2 Help Output Enhancement

```bash
$ yb-voyager migrate --help

Migrate data from source database to YugabyteDB.

WORKFLOW OVERVIEW:
  A typical migration follows these steps:
  
  1. ASSESS    → Analyze source DB for compatibility (optional but recommended)
  2. INIT      → Initialize migration, register with target DB
  3. EXPORT    → Export schema and data from source
  4. IMPORT    → Import schema and data to target  
  5. VERIFY    → Verify data integrity
  6. CUTOVER   → Switch application to target DB

COMMANDS:
  init          Initialize a new migration project
  status        Show current migration status and progress
  next          Show the recommended next command to run
  history       View migration event timeline
  
  export        Export schema/data from source database
  import        Import schema/data to target database
  verify        Verify migration integrity
  cutover       Initiate cutover to target database
  
  end           End migration and cleanup

EXAMPLES:
  # Start a new migration
  yb-voyager migrate init --source-db-type postgresql --target-db-host yb.example.com

  # Check status
  yb-voyager migrate status --target-db-host yb.example.com

  # See what to do next
  yb-voyager migrate next --target-db-host yb.example.com

Use "yb-voyager migrate [command] --help" for more information about a command.
```

---

### Phase 4: UI Integration

#### 4.1 YugabyteDB UI Migration Page

The UI reads directly from the `ybvoyager` schema in the target database:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  YugabyteDB Anywhere / YugabyteD UI                                         │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  Cluster: prod-yb-cluster                                                   │
│  ├── Overview                                                               │
│  ├── Tables                                                                 │
│  ├── Metrics                                                                │
│  └── ✨ Migrations  ◀─── New dedicated section                              │
│       │                                                                     │
│       ├── Active Migrations                                                 │
│       │   └── prod-to-yb-2024 [IN_PROGRESS: Export Data 67%]               │
│       │                                                                     │
│       ├── Completed Migrations                                              │
│       │   └── staging-to-yb-2024 [COMPLETED: 2024-01-15]                   │
│       │                                                                     │
│       └── Assessment Reports                                                │
│           ├── oracle-prod-assessment [READY]                                │
│           └── pg-analytics-assessment [READY]                               │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

#### 4.2 Migration Detail View (UI)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  Migration: prod-to-yb-2024                                                 │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  [Progress Timeline - Interactive Visualization]                            │
│                                                                             │
│  ●────●────●────●────◐────○────○────○                                       │
│  Init  Export Analyze Import Export Import Verify Cutover                   │
│        Schema Schema  Schema Data   Data                                    │
│                                                                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  📊 Current Phase: Export Data                                              │
│                                                                             │
│  Table Progress:                                                            │
│  ┌────────────────────┬──────────┬───────────┬──────────┬─────────┐        │
│  │ Table              │ Status   │ Exported  │ Total    │ Rate    │        │
│  ├────────────────────┼──────────┼───────────┼──────────┼─────────┤        │
│  │ public.users       │ DONE     │ 50,000    │ 50,000   │ -       │        │
│  │ public.orders      │ RUNNING  │ 145,000   │ 217,000  │ 1.2K/s  │        │
│  │ public.products    │ PENDING  │ 0         │ 89,000   │ -       │        │
│  └────────────────────┴──────────┴───────────┴──────────┴─────────┘        │
│                                                                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  🔔 Recommended Next Action:                                                │
│                                                                             │
│  Once export completes, run the following command on your Voyager machine:  │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │ yb-voyager import data --target-db-host yb.example.com \            │   │
│  │   --migration-uuid 550e8400-e29b-41d4-a716-446655440000             │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                 [📋 Copy]   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

### Phase 5: Fleet Migration Workflow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         FLEET MIGRATION WORKFLOW                            │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   Fleet Config (fleet.yaml)                                                 │
│   ┌───────────────────────────────────────────────────────────────────┐    │
│   │ clusters:                                                          │    │
│   │   - name: oracle-prod-east                                         │    │
│   │     source-db-type: oracle                                         │    │
│   │     schemas: [HR, FINANCE, SALES]                                  │    │
│   │   - name: oracle-prod-west                                         │    │
│   │     source-db-type: oracle                                         │    │
│   │     schemas: [HR, INVENTORY]                                       │    │
│   │   - name: pg-analytics                                             │    │
│   │     source-db-type: postgresql                                     │    │
│   │     schemas: [analytics, reporting]                                │    │
│   └───────────────────────────────────────────────────────────────────┘    │
│                                                                             │
│   Fleet Commands:                                                           │
│   ┌───────────────────────────────────────────────────────────────────┐    │
│   │ # Assess all clusters                                              │    │
│   │ yb-voyager fleet assess --config fleet.yaml                        │    │
│   │                                                                    │    │
│   │ # View consolidated report                                         │    │
│   │ yb-voyager fleet report --config fleet.yaml                        │    │
│   │                                                                    │    │
│   │ # Initialize all migrations                                        │    │
│   │ yb-voyager fleet init --config fleet.yaml --target-db-host yb.com  │    │
│   │                                                                    │    │
│   │ # Check fleet status                                               │    │
│   │ yb-voyager fleet status --config fleet.yaml                        │    │
│   └───────────────────────────────────────────────────────────────────┘    │
│                                                                             │
│   Fleet Dashboard (UI):                                                     │
│   ┌───────────────────────────────────────────────────────────────────┐    │
│   │  Cluster          │ Schemas │ Status      │ Progress │ ETA       │    │
│   ├───────────────────┼─────────┼─────────────┼──────────┼───────────│    │
│   │  oracle-prod-east │ 3       │ Migrating   │ 45%      │ 6h        │    │
│   │  oracle-prod-west │ 2       │ Queued      │ 0%       │ 4h        │    │
│   │  pg-analytics     │ 2       │ Assessed    │ -        │ 2h        │    │
│   └───────────────────────────────────────────────────────────────────┘    │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

### Phase 6: Voyager AI Integration Points

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         VOYAGER AI INTEGRATION                              │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  1. Schema Conversion Assistance                                            │
│     ┌─────────────────────────────────────────────────────────────────┐    │
│     │ $ yb-voyager ai convert --schema-file failed.sql                 │    │
│     │                                                                  │    │
│     │ AI Analysis: Found 3 unsupported constructs                      │    │
│     │                                                                  │    │
│     │ Issue 1: BRIN Index (line 45)                                    │    │
│     │ ├── Original: CREATE INDEX idx_ts ON events USING BRIN(ts);      │    │
│     │ ├── Suggested: CREATE INDEX idx_ts ON events(ts ASC);            │    │
│     │ └── Reason: BRIN not supported; range-sharded index recommended  │    │
│     │                                                                  │    │
│     │ Apply suggestions? [y/n/review]:                                 │    │
│     └─────────────────────────────────────────────────────────────────┘    │
│                                                                             │
│  2. Query Optimization Suggestions                                          │
│     ┌─────────────────────────────────────────────────────────────────┐    │
│     │ $ yb-voyager ai analyze-queries --assessment-dir ./assessment    │    │
│     │                                                                  │    │
│     │ Analyzed 1,247 queries from pg_stat_statements                   │    │
│     │                                                                  │    │
│     │ High Impact Recommendations:                                     │    │
│     │ 1. Query pattern: SELECT * FROM orders WHERE customer_id = ?     │    │
│     │    Frequency: 50,000/hour                                        │    │
│     │    Suggestion: Add covering index on (customer_id, order_date)   │    │
│     │                                                                  │    │
│     │ 2. Query pattern: JOIN across users, orders, products            │    │
│     │    Frequency: 1,000/hour                                         │    │
│     │    Suggestion: Consider colocation for these tables              │    │
│     └─────────────────────────────────────────────────────────────────┘    │
│                                                                             │
│  3. Migration Troubleshooting                                               │
│     ┌─────────────────────────────────────────────────────────────────┐    │
│     │ $ yb-voyager ai troubleshoot                                     │    │
│     │                                                                  │    │
│     │ Detected Issue: Import data slower than expected                 │    │
│     │                                                                  │    │
│     │ Analysis:                                                        │    │
│     │ • Target cluster CPU: 85% (high)                                 │    │
│     │ • Parallel jobs: 16 (too many for 8-core nodes)                  │    │
│     │ • Batch size: 20,000 (optimal)                                   │    │
│     │                                                                  │    │
│     │ Recommendations:                                                 │    │
│     │ 1. Reduce parallel-jobs to 8                                     │    │
│     │ 2. Enable adaptive parallelism                                   │    │
│     │                                                                  │    │
│     │ Apply? [y/n]:                                                    │    │
│     └─────────────────────────────────────────────────────────────────┘    │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Implementation Roadmap

### Phase 1: Foundation (Q1)

| Item | Description | Effort |
|------|-------------|--------|
| **1.1** | Refactor assessment to be standalone | 2 weeks |
| **1.2** | Create assessment SQL script generator | 1 week |
| **1.3** | Implement `migrate status` command | 1 week |
| **1.4** | Implement `migrate next` command | 1 week |
| **1.5** | Enhanced `--help` output with workflow guidance | 1 week |

### Phase 2: Target DB as Source of Truth (Q2)

| Item | Description | Effort |
|------|-------------|--------|
| **2.1** | Design and implement target DB metadata schema | 2 weeks |
| **2.2** | Migrate local metadb to target DB | 3 weeks |
| **2.3** | Implement state synchronization | 2 weeks |
| **2.4** | Store reports as JSON in target DB | 1 week |
| **2.5** | Backward compatibility layer | 1 week |

### Phase 3: UI Integration (Q2-Q3)

| Item | Description | Effort |
|------|-------------|--------|
| **3.1** | Define UI API contracts (read from target DB) | 1 week |
| **3.2** | Migration dashboard component | 3 weeks |
| **3.3** | Assessment report viewer | 2 weeks |
| **3.4** | Real-time progress updates | 2 weeks |
| **3.5** | Next-step guidance in UI | 1 week |

### Phase 4: Fleet Operations (Q3)

| Item | Description | Effort |
|------|-------------|--------|
| **4.1** | Fleet configuration format design | 1 week |
| **4.2** | Fleet assessment commands | 2 weeks |
| **4.3** | Fleet migration orchestration | 3 weeks |
| **4.4** | Consolidated fleet reporting | 2 weeks |
| **4.5** | Fleet dashboard in UI | 2 weeks |

### Phase 5: AI Integration (Q4)

| Item | Description | Effort |
|------|-------------|--------|
| **5.1** | AI schema conversion assistance | 3 weeks |
| **5.2** | AI query optimization suggestions | 3 weeks |
| **5.3** | AI troubleshooting assistant | 2 weeks |
| **5.4** | Integration with Voyager AI service | 2 weeks |

---

## Documentation Restructure

### Proposed Documentation Structure

```
docs/
├── getting-started/
│   ├── quickstart.md
│   ├── installation.md
│   └── first-migration.md
│
├── assessment/                    # Standalone assessment docs
│   ├── overview.md
│   ├── gather-metadata.md
│   ├── generate-report.md
│   ├── fleet-assessment.md
│   └── sizing-recommendations.md
│
├── migration/                     # Migration workflow docs
│   ├── overview.md
│   ├── 1-initialize.md
│   ├── 2-export-schema.md
│   ├── 3-analyze-schema.md
│   ├── 4-import-schema.md
│   ├── 5-export-data.md
│   ├── 6-import-data.md
│   ├── 7-verify.md
│   ├── 8-cutover.md
│   └── 9-end-migration.md
│
├── live-migration/
│   ├── overview.md
│   ├── fall-forward.md
│   └── fall-back.md
│
├── fleet-migration/
│   ├── overview.md
│   ├── configuration.md
│   └── orchestration.md
│
├── ui-guide/
│   ├── migrations-dashboard.md
│   └── assessment-viewer.md
│
├── voyager-ai/
│   ├── overview.md
│   ├── schema-conversion.md
│   └── query-optimization.md
│
└── reference/
    ├── commands.md
    └── configuration.md
```

---

## Success Metrics

| Metric | Current | Target |
|--------|---------|--------|
| Time to first successful migration | ~2 days | < 4 hours |
| Documentation lookups per migration | ~50 | < 10 |
| Support tickets for "what to do next" | High | Near zero |
| Fleet assessment completion rate | N/A | > 95% |
| UI adoption for migration monitoring | < 5% | > 60% |

---

## Appendix A: Comparison with Current Flow

### Current Assessment Flow

```bash
# Current: Assessment requires export-dir setup
mkdir /path/to/export-dir
yb-voyager assess-migration \
  --source-db-type oracle \
  --source-db-host source.example.com \
  --source-db-user admin \
  --source-db-schema HR \
  --export-dir /path/to/export-dir  # Required even for just assessment
```

### Proposed Assessment Flow

```bash
# Proposed: Assessment is standalone
# Option 1: Direct connection (for non-production)
yb-voyager assessment run \
  --source-db-type oracle \
  --source-db-host source.example.com \
  --assessment-dir /path/to/assessment

# Option 2: SQL scripts for production (DBA-friendly)
yb-voyager assessment generate-scripts --source-db-type oracle --output-dir ./scripts
# DBA runs scripts on production, saves output
yb-voyager assessment ingest-metadata --assessment-dir ./assessment --metadata-dir ./collected
yb-voyager assessment generate-report --assessment-dir ./assessment
```

### Current Migration Flow

```bash
# Current: No guidance on what to do next
yb-voyager export schema --export-dir ./export --source-db-type postgresql ...
# User must know to run analyze-schema next
yb-voyager analyze-schema --export-dir ./export
# User must know to run import schema next
yb-voyager import schema --export-dir ./export --target-db-host yb.example.com ...
# ... and so on
```

### Proposed Migration Flow

```bash
# Proposed: Guided workflow with status and next-step recommendations
yb-voyager migrate init --source-db-type postgresql --target-db-host yb.example.com ...
yb-voyager migrate status  # Shows: "Next: export schema"
yb-voyager migrate export schema ...
yb-voyager migrate status  # Shows: "Export schema DONE. Next: analyze schema"
yb-voyager migrate next    # Outputs the exact command to run
```

---

## Appendix B: Target DB Schema Details

```sql
-- Create dedicated schema for voyager metadata
CREATE SCHEMA IF NOT EXISTS ybvoyager;

-- Main migration registry
CREATE TABLE ybvoyager.migrations (
    migration_uuid UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    migration_name TEXT NOT NULL,
    source_db_type TEXT NOT NULL,
    source_db_host TEXT,
    source_db_version TEXT,
    source_db_identifier TEXT,  -- System identifier for deduplication
    source_schemas TEXT[],
    target_db_name TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    current_phase TEXT,
    current_status TEXT,
    voyager_version TEXT,
    voyager_host TEXT,
    UNIQUE(source_db_identifier, source_schemas, target_db_name)
);

-- Migration phases and steps state
CREATE TABLE ybvoyager.migration_state (
    migration_uuid UUID REFERENCES ybvoyager.migrations(migration_uuid),
    phase TEXT NOT NULL,  -- assessment, export_schema, analyze_schema, import_schema, export_data, import_data, verify, cutover
    step TEXT,            -- Sub-step within phase
    status TEXT NOT NULL, -- NOT_STARTED, IN_PROGRESS, COMPLETED, FAILED
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    error_message TEXT,
    metadata JSONB,       -- Phase-specific metadata
    PRIMARY KEY (migration_uuid, phase, COALESCE(step, ''))
);

-- Reports storage (JSON for UI rendering)
CREATE TABLE ybvoyager.reports (
    migration_uuid UUID REFERENCES ybvoyager.migrations(migration_uuid),
    report_type TEXT NOT NULL,   -- assessment, schema_analysis, data_migration, etc.
    report_version INT DEFAULT 1,
    report_data JSONB NOT NULL,  -- Full report in JSON for UI rendering
    generated_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (migration_uuid, report_type, report_version)
);

-- Table-level progress tracking
CREATE TABLE ybvoyager.table_progress (
    migration_uuid UUID REFERENCES ybvoyager.migrations(migration_uuid),
    schema_name TEXT NOT NULL,
    table_name TEXT NOT NULL,
    phase TEXT NOT NULL,  -- export_data, import_data, verify
    total_rows BIGINT,
    completed_rows BIGINT DEFAULT 0,
    errored_rows BIGINT DEFAULT 0,
    status TEXT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    rate_per_second FLOAT,
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (migration_uuid, schema_name, table_name, phase)
);

-- Event log for audit and debugging
CREATE TABLE ybvoyager.events (
    event_id BIGSERIAL PRIMARY KEY,
    migration_uuid UUID REFERENCES ybvoyager.migrations(migration_uuid),
    event_type TEXT NOT NULL,
    event_data JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Index for efficient queries
CREATE INDEX idx_migrations_name ON ybvoyager.migrations(migration_name);
CREATE INDEX idx_events_migration ON ybvoyager.events(migration_uuid, created_at);
```

---

## Conclusion

This proposal addresses the five key challenges identified:

1. **CLI Usability**: `migrate status` and `migrate next` commands provide clear guidance
2. **Documentation Alignment**: Documentation restructured to mirror CLI workflow
3. **UI Integration**: Target DB as source of truth enables real-time UI updates
4. **Fleet Migration**: Dedicated fleet commands and consolidated reporting
5. **Voyager AI**: Clear integration points for AI-assisted migration

The phased implementation approach allows for incremental delivery while maintaining backward compatibility with existing migrations.

