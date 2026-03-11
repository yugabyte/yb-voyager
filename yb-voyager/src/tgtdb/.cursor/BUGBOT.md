# tgtdb Package Review Rules

## Target DB Interface

- Keep the target DB interface lean. Only add methods that are genuinely needed by multiple target implementations or by the shared import logic layer. DB-specific helpers should be accessed via type assertion, not added to the interface.
- When naming interface methods, be consistent: either use the `Get` prefix for all getters or omit it for all.

## Import and COPY Logic

- Distinguish between transactional COPY (normal path) and non-transactional COPY (fast path). The fast path is only valid when primary-key-conflict mode is set to IGNORE.
- After COPY, assert that rows-affected plus rows-ignored equals the batch record count. A mismatch indicates silent data loss.
- For error classification, check the PG error code class, not just the error message string. Integrity constraint violations, syntax errors, and data exceptions are generally non-retryable.
- When classifying unique constraint violations, distinguish between primary key violations and unique key violations. The primary-key-conflict IGNORE flag applies only to PK violations, not UK violations.

## Conflict Detection

- The conflict detection cache should only include events where unique key columns have actually *changed*, not every update event. Caching every update is a known performance issue.
- When working with unique key columns, handle case-sensitive column names correctly.
- Remove unused code from conflict detection logic promptly. Stale helper functions cause confusion during reviews.

## Parallelism and Core Detection

- When a load balancer is in front of the target cluster, fetching cores from a single LB-routed node underestimates the cluster total. Use the fallback formula (node count × default cores per node) in this case.
- Move default/max parallelism calculation logic into a separate function for clarity.
- Document the relationship between connection configs filtered by load balancer and total cluster node count in comments.

## Partition Handling

- Functions that query tables from system catalogs return both root and leaf partition tables. The caller is responsible for resolving leaf→root mappings. DB-specific functions should not embed command-level handling (e.g., deciding whether to return roots or leaves).
- Always handle the missing-key case when looking up table name mappings. A missing entry should produce an error, not a nil-pointer panic.
- Document whether a function returns only leaf partitions, only roots, or both, as a comment on the function.

## Failpoint Testing

- Failpoint-related code adds noise to production logic. Keep failpoint blocks as small as possible.
- Name failpoints descriptively: include whether the failure is retryable or non-retryable.

## Testing

- Follow the `TestMain` → `m.Run()` pattern for setup/teardown with testcontainers.
- Always include test cases for case-sensitive table names, partitioned tables, and expression indexes.
- Use table-driven tests with `t.Run` to reduce duplication between similar test scenarios.
- Define reusable test struct types instead of repeating anonymous structs across functions.
