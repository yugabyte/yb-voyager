# migtests (Migration Integration Tests) Review Rules

## Test Structure

- Each test directory should contain: `env.sh` (config), `init-db` (setup), `cleanup-db` (teardown), `validate` (assertions), and relevant SQL/schema files.
- Tests must be self-contained. Do not rely on state left by a previous test run. Use `start-clean` where applicable.
- Remove unnecessary artifacts from test files: no `\d` meta-commands, no leftover debugging comments, no commented-out code.

## Validation Scripts

- Validate actual data values, not just row counts. Row count checks alone miss data corruption.
- Fix shared library functions (`yb.py`, `functions.sh`) rather than patching individual test validation scripts. If `get_column_to_data_type_mapping` or `fetch_all_procedures` has a bug, fix it in `lib/yb.py` rather than adding workarounds in each test's `validate` script.
- When comparing data across source and target, cast values to TEXT for comparison to handle type serialization differences.
- Reusable validation patterns (e.g., fetching column values by type-casting to text) should be library functions in `lib/yb.py`, not copy-pasted across test validate scripts.

## Expected Files

- When updating `expectedAssessmentReport.json`, `expected_failed*.sql`, or `expected_callhome_payloads/*.json`, verify that the changes match the actual Go constants and code changes. Mismatches between expected JSON and Go constants cause byte-for-byte comparison failures in `cmp -s`.
- Use the correct expected file for the YugabyteDB version being tested (e.g., `expected_failed_2025.2.sql` vs `expected_failed_2024.2.sql`). Do not create redundant version-specific files if the content is identical.
- Make the minimum necessary change to expected files. Do not regenerate entire files when only one field changed.

## Shell Scripts (`scripts/`)

- In `functions.sh`, use existing helper functions (`tail_log_file`, `create_source_db`) instead of reimplementing their logic inline.
- Quote `PGPASSWORD` values to handle special characters in passwords: `PGPASSWORD='p@ssword' psql ...`.
- Common setup steps (e.g., `CREATE EXTENSION pg_stat_statements`, `ALTER REPLICA`) should live in shared functions, not duplicated in each test's scripts.
- When removing or changing shared functions, verify that all tests that call them still work.

## Event Generator (`scripts/event-generator/`)

- Index names created by the event generator must be prefixed to avoid collision with application indexes (e.g., `evt_gen_idx_`). Keep names short to stay within PostgreSQL's 63-character identifier limit.
- Index operation intervals should be configurable via YAML (`index_events_interval`), not hard-coded.
- Use `random.choice` for action selection rather than pre-computing action lists.
- Track only indexes created by the generator for cleanup. Do not drop indexes that existed before the test.

## Live Migration Tests

- Resumption tests must validate data integrity after resume, not just that the process completed. Use segment-hash validation or equivalent row-level comparison.
- YAML scenario files: use `scenario.local.yaml` for local development, `scenario.yaml.template` for Jenkins pipelines. The template uses environment variable substitution for target DB parameters.
- Wait intervals for streaming phases should be long enough to allow meaningful data flow. Document how Jenkins overrides these via template files.

## YAML and Config

- Do not include unnecessary fields like `version` or `run_id` if they are not used by the test framework.
- Config field names should be descriptive: `enable_index_create_drop` not `index_operations`, `stop_index_thread` not `stop_thread`.
- Retry counts for operations with non-deterministic failure rates (e.g., DDL on active tables) should be based on observed behavior, not arbitrary guesses.

## Test Data Hygiene

- GRANT scripts should handle replica setup. Do not duplicate `ALTER REPLICA` calls both in `functions.sh` and in per-test grant scripts.
- `pgcrypto` and other extensions should be created before any tables that depend on them.
- Common SQL schemas shared across tests should live in `common-sql/` and be referenced by path in scenario files, not baked into the framework.
