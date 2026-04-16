-- =============================================================================
-- schema_drift baseline reset (public.control_t + public.subject_t)
-- =============================================================================
-- Two layers:
--   1) control_subject_baseline_schema.sql  — run on **source and target** (tables only).
--   2) control_subject_baseline_source_seeds.sql — **source only** (seed INSERTs).
--
-- This file runs (1) then (2) for a full source reset. On target, run **schema only**:
--   \ir control_subject_baseline_schema.sql
--
-- One-time: create empty database on each side:
--   CREATE DATABASE schema_drift;
--
-- From this directory (paths are relative to this file via \ir):
--
--   Source (full baseline):
--     psql -h <src> -p 5432 -U postgres -d postgres -v ON_ERROR_STOP=1 -f control_subject_baseline.sql
--   Target (schema only):
--     ysqlsh -h <yb> -p 5433 -U yugabyte -d yugabyte -v ON_ERROR_STOP=1 -f control_subject_baseline_schema.sql
--
-- If your client does not support \connect inside the included files, connect with
-- `-d schema_drift` and remove the \connect lines from the included SQL.
--
-- Drift runbook: SCHEMA_DRIFT_MANUAL_LOG.md
-- =============================================================================

\ir control_subject_baseline_schema.sql
\ir control_subject_baseline_source_seeds.sql

-- Example drift / ping SQL: SCHEMA_DRIFT_MANUAL_LOG.md
