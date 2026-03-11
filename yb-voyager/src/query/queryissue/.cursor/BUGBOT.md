# queryissue Package Review Rules

## Parse Tree Handling

- Always clone a parse tree before mutating it. When multiple issues share a `workaroundParseTree`, mutating in-place corrupts subsequent recommendations. Use `queryparser.CloneParseTree` and assign the result to a new variable.
- When a generator function fails, do not overwrite the shared tree variable with nil. Assign the return value to a temporary variable and only commit on success:
  ```
  result, err := generator.GenerateRecommendedSql(clone)
  if err != nil {
      log.Warnf(...)
      continue
  }
  workaroundParseTree = result
  ```
- After deparsing a workaround tree, check that the result is non-empty before assigning it to `RecommendedSQL`. An empty recommendation is worse than no recommendation.

## TableMetadata

- All per-table information (columns, constraints, indexes, partition hierarchy, usage stats) should be consolidated in the `TableMetadata` struct. Do not create ad-hoc `map[string]...` structures for table properties that `TableMetadata` already tracks.
- Use `getOrCreateTableMetadata` rather than a manual check-then-create pattern.
- Populate metadata in the `FinalizeTablesMetadata` step for DDLs processed from schema dumps. For DDLs found inside PL/pgSQL blocks, metadata may need to be populated on demand.
- For partitioned tables, resolve the full partition hierarchy (root → intermediate → leaf) using topological sort to ensure parent metadata is populated before child metadata.

## Issue Definition

- Issue names should describe the *problem*, not the recommendation. Example: "Missing Primary Key for Table" not "Add Primary Key Recommendation."
- Issue descriptions should be source-database-agnostic. Do not reference PostgreSQL-specific storage details unless the issue is PostgreSQL-specific.
- Documentation link patterns must match the actual docs anchor format. Verify link patterns with the docs team when adding new issues.
- String constants for descriptions and names must not contain stray trailing characters (e.g., an unintended trailing `"` inside a backtick-delimited raw string).

## Details and InternalDetails

- The `Details` map on `QueryIssue` is displayed in the HTML assessment report UI. Do not put internal metadata (like `ColumnType` used for detection logic) into `Details` — use `InternalDetails` instead.
- Sensitive information (column names with user data implications) should not be included in `Details` if the details are sent to callhome.

## Detection Logic

- When checking index coverage for foreign keys, use set equality on column name prefixes, not permutation generation.
- For expression columns in indexes, immediately disqualify the index from coverage checks rather than trying to match expression text.
- When skipping partitioned tables in detection (e.g., because issues will be reported on leaf partitions), add an explicit comment explaining why.

## Testing

- Each new issue type must have at least one unit test in `parser_issue_detector_test.go` that verifies the issue is detected and the `RecommendedSQL` (if any) is correct.
- Include test cases for:
  - Case-sensitive table and column names.
  - Partitioned tables (root, intermediate, leaf).
  - Foreign keys spanning across schemas.
  - Indexes with expression columns.
- Verify that the `expectedAssessmentReport.json` in migtests matches the actual constants and struct transformations. A mismatch between Go constants and expected JSON will cause integration test failures.
