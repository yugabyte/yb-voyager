# Schema Diff Test: Adding a New Table During Live Migration

## Test Setup
- **Source**: PostgreSQL 17, database `test_schema_diff`, schema `public`
- **Target**: YugabyteDB 2025.2, database `test_schema_diff`, schema `public`
- **Migration type**: Live migration (`snapshot-and-changes`)
- **Initial schema**: Single table `control(id SERIAL PK, name VARCHAR(100), created_at TIMESTAMP)`
- **DDL change**: Adding table `subject(id SERIAL PK, title VARCHAR(200), description TEXT, created_at TIMESTAMP)`

---

## Step-by-Step Observations

### Step 1: Start Migration with Initial Schema
- Exported schema (1 table: `control`) and imported to YugabyteDB
- Started `export data --export-type snapshot-and-changes` and `import data to target`
- Snapshot of 3 initial rows exported and imported successfully
- Verified streaming works: inserted `streaming_test_1` → replicated to target within ~10s
- **Result**: ✅ Migration running correctly

### Step 2: DDL Change — Create `subject` Table on Source
- Created `subject` table with `REPLICA IDENTITY FULL` on source PostgreSQL
- **Result**: ✅ No crash. Both export and import processes continued running without any errors or warnings.

### Step 3: Insert Events into Both Tables
- Inserted 2 rows into `control` and 2 rows into `subject`
- **Observations**:
  - `control` events: ✅ Both exported and imported successfully (total exported events went from 1 → 3)
  - `subject` events: ❌ **Silently dropped** — not exported, not imported, NO error in any log
- **Root cause**: Debezium config (`application.properties`) has `table.include.list=public.control`. The `subject` table was never added to the CDC connector's tracking list.
- The PG publication (`voyager_dbz_publication_...`) also only included `control`.
- **Result**: ⚠️ No crash, but **data loss for the new table is silent** — no warning, no error, no indication that events are being missed.

### Step 4: Stop and Restart Export/Import
- Stopped both processes (Ctrl+C)
- Inserted more data while stopped: 1 row each into `control` and `subject`
- Restarted both processes
- **Observations**:
  - `control` events from the stopped period: ✅ Picked up correctly after restart (replication slot preserved the WAL)
  - `subject` events: ❌ Still ignored (Debezium config unchanged on restart)
  - Restart was clean, no errors
- **Result**: Restart alone does NOT fix the issue. The Debezium config is regenerated from the same metadata.

### Step 5: Workaround Attempt 1 — Only Update meta.db + Debezium Config
- Updated `application.properties`: added `public.subject` to `table.include.list`, `column.include.list`
- Updated `meta.db` SQLite: added `subject` to `TableListExportedFromSource`, `SourceExportedTableListWithLeafPartitions`, `SourceColumnToSequenceMapping`, `tableToCDCPartitioningStrategyMap`
- Added `subject` to the PG publication
- **Result**: ❌ **CRASH** on export restart:
  ```
  error in get_initial_table_list at step 'get_name_tuple_from_qualified_object'
  Error: lookup for table name failed err: {"subject" subject subject}: lookup source table name [.subject]: table name not found: subject
  ```
- **Root cause**: The Go-side name registry (`metainfo/name_registry.json`) was NOT updated. The exporter validates the table list against this JSON registry, and `subject` was not registered there.

### Step 6: Workaround Attempt 2 — Full Workaround (SUCCESSFUL)
All modifications required:

1. **Source PostgreSQL**:
   - `ALTER PUBLICATION voyager_dbz_publication_... ADD TABLE public.subject;`

2. **Target YugabyteDB**:
   - `CREATE TABLE public.subject (...)` — must match source schema

3. **`metainfo/conf/application.properties`**:
   - `table.include.list`: add `public.subject`
   - `column.include.list`: add `public.subject.*`
   - `column_sequence.map`: add `public.subject.id:"public"."subject_id_seq"`
   - `sequence.max.map`: add `"public"."subject_id_seq":3`

4. **`metainfo/name_registry.json`**:
   - `SourceDBTableNames.public`: add `"subject"`
   - `YBTableNames.public`: add `"subject"`
   - `SourceDBSequenceNames.public`: add `"subject_id_seq"`
   - `YBSequenceNames.public`: add `"subject_id_seq"`

5. **`metainfo/meta.db` (SQLite)**:
   - `migration_status.TableListExportedFromSource`: add `"public.subject"`
   - `migration_status.SourceExportedTableListWithLeafPartitions`: add `"public.subject"`
   - `migration_status.SourceColumnToSequenceMapping`: add `public.subject.id` mapping
   - `import_data_status.tableToCDCPartitioningStrategyMap`: add `"public"."subject"` → `"pk"`

6. **Manual data backfill**:
   - Rows inserted into `subject` before the workaround are LOST from CDC. They must be manually copied to the target.

- **Result**: ✅ After all modifications, both export and import restarted successfully. New events for both `control` and `subject` tables are now streamed and imported correctly.

---

## Summary

| Scenario | Crash? | Data Loss? | Details |
|----------|--------|------------|---------|
| Create new table on source during live migration | No | No (for existing tables) | DDL change itself is harmless |
| Insert into new table (no workaround) | No | **YES — SILENT** | Events for untracked table are silently dropped |
| Restart export/import (no config changes) | No | Yes (same as above) | Config is not auto-updated on restart |
| Update only meta.db + Debezium config | **YES — CRASH** | N/A | Name registry validation fails |
| Full workaround (all 5 config files + manual backfill) | No | **Partial** (pre-workaround data must be manually backfilled) | Streaming works for both tables going forward |

## What Crashes and Why

1. **The exporter crashes** if you add a table to `meta.db`/`application.properties` without also updating `name_registry.json`. The crash happens in `get_initial_table_list` → `retrieve_first_run_table_list` because the Go code validates table names against the name registry, which is a separate JSON file.

2. **Nothing crashes** if you just create the table on source without touching any voyager config. However, data for the new table is **silently lost**.

## Workaround Summary

A manual workaround IS possible but requires updating **5 separate configuration/metadata stores**:
1. PostgreSQL publication
2. Debezium `application.properties`
3. Voyager `name_registry.json`
4. Voyager `meta.db` (SQLite, multiple JSON fields)
5. Target database DDL

Plus manual data backfill for any rows inserted before the workaround was applied.

**This workaround is fragile and error-prone** — missing any single component causes either a crash or silent data loss. It is not recommended for production use without thorough testing.

## Recommendations

1. **yb-voyager should warn/error when CDC receives events for untracked tables** instead of silently dropping them.
2. **A built-in `add-table` command** would be valuable to automate the 5-step workaround safely.
3. **Documentation should clearly state** that adding tables during live migration requires manual intervention and has data loss implications.
