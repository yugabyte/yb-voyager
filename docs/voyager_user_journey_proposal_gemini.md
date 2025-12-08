# Voyager User Journey Enhancement Proposal (Gemini Edition)

## Executive Summary

This proposal outlines a strategic evolution of YB-Voyager from a task-based migration tool to a **guided migration platform**. By restructuring the CLI to support standalone assessments and leveraging the Target YugabyteDB as the single source of truth for migration state, we can significantly reduce user friction, enable fleet-scale operations, and unlock powerful UI integration capabilities.

## 1. Current Friction Analysis

Based on the current codebase structure (`yb-voyager/cmd/`), several architectural patterns create user friction:

| Friction Point | Codebase Evidence | User Impact |
| :--- | :--- | :--- |
| **Assessment Coupling** | `assess-migration` requires `export-dir` setup and initializes a project structure. | Users cannot easily "just assess" a DB without committing to a migration workflow. |
| **Hidden State** | State is locked inside `export-dir/metainfo/meta.db` (SQLite). | Users must maintain the exact machine/directory to resume or check status. No visibility for teammates. |
| **Blind Workflow** | No central "status" command. Users must know specific sub-commands (`export data status`, etc.) to check progress. | High reliance on documentation to know "what comes next". |
| **Siloed Reporting** | Reports are local HTML/JSON files. | UI cannot render reports natively; users must manually upload or share files. |

---

## 2. The New User Journey

### Phase 1: Assessment (The "Try Before You Buy" Experience)

**Goal:** Allow users to assess their source database with zero setup friction. No Target DB required.

#### New Workflow
1.  **Generate Collection Scripts**: User generates SQL scripts to run on their production DB (safe, read-only, DBA-approved).
2.  **Run Offline**: User runs scripts on their secured production machine.
3.  **Analyze Anywhere**: User feeds the output to Voyager on a laptop/server for analysis.

#### Proposed CLI Structure
```bash
# 1. Generate lightweight SQL scripts (no connection needed)
yb-voyager assessment generate-scripts --source-db-type oracle --output-dir ./scripts

# 2. [Performed by DBA on Source] -> produces 'metrics.tar.gz'

# 3. Ingest & Analyze (Offline or Online)
yb-voyager assessment analyze \
    --metadata-file ./metrics.tar.gz \
    --output-dir ./assessment-report

# 4. Fleet View (Compare multiple assessments)
yb-voyager assessment fleet-report --dir ./all-assessments
```

### Phase 2: Migration Initialization (The "Commit" Moment)

**Goal:** Formalize the migration by connecting to the Target DB. The Target DB becomes the "Migration Control Plane."

#### Architectural Shift
*   **Old Way:** `yb-voyager export schema` initializes local `meta.db`.
*   **New Way:** `yb-voyager migrate init` connects to Target YB and creates a `ybvoyager_metadata` schema.

#### Proposed CLI Structure
```bash
# Initialize the migration state in the Target DB
yb-voyager migrate init \
    --project-name "crm-migration-v1" \
    --source-db-type oracle \
    --target-db-host yb-tserver-1 \
    --target-db-user ybvoyager_admin

# Response:
# Migration 'crm-migration-v1' initialized.
# State stored in Target DB. 
# Run 'yb-voyager migrate status' to see next steps.
```

### Phase 3: Execution (The "Guided" Experience)

**Goal:** The CLI should tell the user what to do, not the other way around.

#### New Global Commands
*   `yb-voyager migrate status`: Queries Target DB to show overall progress (Schema -> Data -> Verify -> Cutover).
*   `yb-voyager migrate next`: Analyzes current state and outputs the exact command string for the next step.

#### Proposed CLI Experience
```bash
$ yb-voyager migrate status
Project: crm-migration-v1
Phase:   Data Export
Status:  In Progress (45%)
Lag:     125ms (CDC active)

$ yb-voyager migrate next
Suggested Next Step:
Waiting for initial snapshot to complete. 
Once 'status' shows 'Snapshot Complete', you can run:
  yb-voyager import data --project-name "crm-migration-v1"
```

---

## 3. Technical Enhancements

### 3.1 Target DB as Source of Truth
We will transition metadata storage from local SQLite to the Target YugabyteDB.

*   **Schema:** `ybvoyager_metadata` (created on `init`)
*   **Tables:**
    *   `migrations`: Registry of all migration projects.
    *   `state`: Current phase, flags, and configuration.
    *   `reports`: JSON blobs of assessment and schema analysis reports (enables UI rendering).
    *   `events`: Audit log of CLI commands executed against this project.

### 3.2 UI Integration Logic
Since state is now in the Target DB, the YugabyteDB Anywhere (YBA) or YugabyteD UI can simply query the `ybvoyager_metadata` schema to render dashboards.

*   **Real-time Progress:** UI queries row counts/CDC lag directly from metadata tables.
*   **Reports:** UI fetches JSON report blobs and renders them natively (replacing static HTML files).
*   **Multi-User:** Multiple DBAs can check status via UI or CLI from different machines.

---

## 4. Fleet & AI Workflows

### Fleet Migration
With the new architecture, "Fleet" is just a collection of "Projects" stored in the Target DB.

*   **CLI:** `yb-voyager fleet status` queries the Target DB for all active migrations and presents a summary view.
*   **Execution:** A single "Orchestrator" machine can loop through projects and trigger export/import jobs.

### Voyager AI Integration
AI becomes an on-demand assistant invoked via CLI.

```bash
# Schema Error Analysis
$ yb-voyager ai fix-schema --error-file ./failed_ddl.sql
> Analyzing Oracle PL/SQL package 'HR_UTIL'...
> Suggestion: Rewrite lines 45-50 using PostgreSQL DO block.
> Apply fix? [Y/n]

# Performance Tuning
$ yb-voyager ai tune-queries --workload-capture ./pg_stat.csv
> Finding: High contention on sequence 'order_id_seq'.
> Recommendation: Increase CACHE size or use UUIDs.
```

---

## 5. Implementation Roadmap

1.  **Foundation (Q1):**
    *   Build `yb-voyager assessment` sub-command (decoupled from export-dir).
    *   Design `ybvoyager_metadata` schema for Target DB.
2.  **State Migration (Q2):**
    *   Implement `migrate init` to bootstrap Target DB state.
    *   Refactor `export/import` commands to read/write to Target DB instead of local SQLite.
3.  **Experience (Q3):**
    *   Implement global `migrate status` and `migrate next`.
    *   Expose JSON reports in Target DB for UI team.
4.  **Advanced Features (Q4):**
    *   Fleet dashboard commands.
    *   AI assistant sub-commands.

