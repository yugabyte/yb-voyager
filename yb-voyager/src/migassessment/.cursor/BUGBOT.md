# migassessment Package Review Rules

## Naming

- Use source-agnostic names throughout. `GetQueryStats` not `GetPgStatStatements`, `HasQueryStatsData` not `HasPGSSData`, `DB_QUERY_STATS` not `DB_QUERIES_SUMMARY`.
- When naming struct fields for metrics, make it clear what they measure: `Scans` or `TupleReads`, not ambiguous names like `Count`.
- Table and struct names that are used across source types should not contain PG-specific references.

## AssessmentDB

- The assessmentDB is a SQLite database. When loading data, go through `BulkInsert` for CSV→SQLite unless special handling is needed (e.g., PGSS where CSV columns vary by PG version). Document the reason for any special handling with a comment.
- `ON CONFLICT` clauses in INSERT statements are unnecessary when `start-clean` is required for re-runs. Do not add them speculatively.
- When a query fails on the assessmentDB, return the error. Do not `log.Warn` and continue — this hides corruption and makes debugging harder.

## Sizing Recommendations

- Group operations by strategy: complete all steps for `colocatedAndShardedCombined` before starting `allSharded`, rather than interleaving them.
- When intermediate variables like `reasoning` are used only for debugging, log them (`log.Infof`) rather than storing them in unused variables.
- Safety checks for empty/nil data: check that `indexImpacts` is non-empty and that map lookups succeed before proceeding with calculations.

## Multi-Node Assessment

- When gathering metadata from multiple nodes (primary + replicas), report errors per node with enough context (hostname, error details) to be actionable. Do not summarize multiple failures as "metadata collection failed on one or more nodes."
- `displayName` is a display concern for `ProgressTracker`. Do not pass it through the entire gathering pipeline. Only `nodeName`/`nodeId` should flow through the gather logic.
- When there are no replicas, follow the single-node code path. Do not display "primary"/"replica" terminology to users who have no replicas.

## Upgrade Safety

- Adding a column to the assessmentDB SQLite schema is a breaking change if older versions of voyager try to query the new schema. Guard new column access with version checks or `ALTER TABLE ... ADD COLUMN IF NOT EXISTS`.
- When changing the structure of assessment report payloads (structs used in `SizingRecommendation`, `AssessmentIssue`, etc.), increment both `ASSESS_MIGRATION_PAYLOAD_VERSION` and `ASSESS_MIGRATION_YBD_PAYLOAD_VERSION` in `cmd/common.go`.

## Testing

- Test the upgrade path: verify that an assessmentDB created by an older voyager version still works with new code.
- When asserting sizing results, verify that one specific recommendation is returned rather than accepting any of multiple valid options.
