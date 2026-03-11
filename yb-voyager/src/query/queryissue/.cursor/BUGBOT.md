# queryissue Package Review Rules

## Parse Tree Handling

- Always clone a parse tree before mutating it. When multiple issues share a workaround tree, mutating in-place corrupts subsequent recommendations.
- When a fix generator fails, do not overwrite the shared tree variable with nil. Assign the return value to a temporary variable and only commit on success.
- After deparsing a workaround tree, check that the result is non-empty before using it as a recommendation. An empty recommendation is worse than no recommendation.

## Table Metadata

- All per-table information (columns, constraints, indexes, partition hierarchy, usage stats) should be consolidated in the central table metadata structure. Do not create ad-hoc maps for table properties that the metadata struct already tracks.
- Use get-or-create helpers rather than manual check-then-create patterns.
- For partitioned tables, resolve the full partition hierarchy (root → intermediate → leaf) using topological sort to ensure parent metadata is populated before child metadata.

## Issue Definition

- Issue names should describe the *problem*, not the recommendation. Example: "Missing Primary Key for Table" not "Add Primary Key Recommendation."
- Issue descriptions should be source-database-agnostic. Do not reference PostgreSQL-specific storage details unless the issue is PostgreSQL-specific.
- Documentation link patterns must match the actual docs anchor format. Verify link patterns with the docs team when adding new issues.
- String constants for descriptions and names must not contain stray trailing characters (e.g., an unintended trailing `"` inside a backtick-delimited raw string).

## Details vs Internal Details

- The `Details` map on issues is displayed in the HTML assessment report UI. Do not put internal metadata (used only for detection logic) into `Details` — use an internal-only field instead.
- Sensitive information (column names with user data implications) should not be included in details if they are sent to callhome.

## Detection Logic

- When checking index coverage for foreign keys, use set equality on column name prefixes, not permutation generation.
- For expression columns in indexes, immediately disqualify the index from coverage checks rather than trying to match expression text.
- When skipping partitioned tables in detection (e.g., because issues will be reported on leaf partitions), add an explicit comment explaining why.

## Testing

- Each new issue type must have unit tests verifying the issue is detected and the recommended SQL (if any) is correct.
- Include test cases for:
  - Case-sensitive table and column names.
  - Partitioned tables (root, intermediate, leaf).
  - Foreign keys spanning across schemas.
  - Indexes with expression columns.
- Verify that expected assessment report JSON files in migtests match the actual constants and transformations. A mismatch between Go constants and expected JSON will cause integration test failures.
