# migassessment Package Review Rules

## Naming

- Use source-agnostic names throughout. Functions, table names, and struct fields that work across source types should not contain PG-specific references.
- When naming struct fields for metrics, make it clear what they measure (e.g., `Scans` not `Count`).

## AssessmentDB

- The assessmentDB is a SQLite database. When loading data, go through the bulk-insert path for CSV→SQLite unless special handling is needed (e.g., when CSV columns vary by source DB version). Document the reason for any special handling with a comment.
- `ON CONFLICT` clauses in INSERT statements are unnecessary when `start-clean` is required for re-runs. Do not add them speculatively.

## Sizing Recommendations

- Group operations by strategy: complete all steps for one sizing strategy before starting the next, rather than interleaving them.
- Safety checks for empty/nil data: verify that lookup maps contain the expected keys before proceeding with calculations.

## Multi-Node Assessment

- When gathering metadata from multiple nodes (primary + replicas), report errors per node with enough context (hostname, error details) to be actionable. Do not summarize multiple failures into a single vague message.
- When there are no replicas, follow the single-node code path. Do not display "primary"/"replica" terminology to users who have no replicas.

## Upgrade Safety

- Adding a column to the assessmentDB SQLite schema is a breaking change if older versions of voyager try to query the new schema. Guard new column access with version checks or `ADD COLUMN IF NOT EXISTS`.
- When changing the structure of assessment report or callhome payload structs, increment the relevant payload version constants.

## Testing

- Test the upgrade path: verify that an assessmentDB created by an older voyager version still works with new code.
