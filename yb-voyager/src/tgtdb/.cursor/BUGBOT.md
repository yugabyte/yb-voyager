# tgtdb Package Review Rules

## Target DB Interface

- Keep the `TargetDB` interface lean. Only add methods that are genuinely needed by multiple target implementations or by the shared import logic layer. YugabyteDB-specific helpers (e.g., `GetYBServers`, partition root resolution) should be accessed via type assertion to `*TargetYugabyteDB`, not added to the interface.
- When naming interface methods, be consistent: either use the `Get` prefix for all getters or omit it for all. Do not mix `GetVersion` with `FetchCores`.

## Import and COPY Logic

- Distinguish between transactional COPY (normal path) and non-transactional COPY (fast path). The fast path (`ROWS_PER_TRANSACTION` not set) is only valid when `on-primary-key-conflict = IGNORE`.
- When COPY returns `rowsAffected + rowsIgnored != batch.RecordCount`, treat it as an error. This assertion catches silent data loss.
- For error classification, check the PG error code class, not just the error message string. Error class `23` (integrity constraint violation), `42` (syntax error or access rule violation), and `22` (data exception) are generally non-retryable.
- When classifying unique constraint violations, distinguish between primary key violations and unique key violations. `on-primary-key-conflict IGNORE` applies only to PK violations, not UK violations.

## Conflict Detection

- The conflict detection cache should only include events where unique key columns have actually *changed*, not every update event. Every update going through conflict detection is a known performance issue.
- When working with unique key columns, handle case-sensitive column names correctly.
- Remove unused code from conflict detection logic promptly. Stale helper functions cause confusion during reviews.

## Parallelism and Core Detection

- When a load balancer is used, `tconfs` has only one entry (the LB URI). Fetching cores from a single LB-routed node underestimates the cluster total. Use the fallback formula (`nodeCount * defaultCoresPerNode`) in this case.
- Move default/max parallelism calculation logic into a separate function for clarity.
- Document the relationship between `tconfs` (filtered by load balancer) and `nodeCount` (total cluster nodes) in comments.

## Partition Handling

- Functions that query tables from `pg_catalog` return both root and leaf partition tables. The caller is responsible for resolving leaf→root mappings using the partition hierarchy if needed. DB-specific functions should not embed command-level handling (e.g., deciding whether to return roots or leaves).
- Always handle the missing-key case when looking up `tableCatalogNameToTuple`. A missing entry should produce an error, not a nil-pointer panic.
- Document whether a function returns only leaf partitions, only roots, or both, as a comment on the function.

## Failpoint Testing

- Failpoint-related code adds noise to production logic. Keep failpoint blocks as small as possible. If the failpoint block requires building an error object, consider wrapping it in a helper.
- Name failpoints descriptively: include whether the failure is retryable or non-retryable (e.g., `importCDCNonRetryableBatchDBError` vs `importCDCRetryableExecuteBatchError`).

## Testing

- Unit tests and integration tests both use testcontainers. Follow the `TestMain` → `m.Run()` pattern for setup/teardown.
- Always include test cases for case-sensitive table names, partitioned tables, and expression indexes.
- Use table-driven tests with `t.Run` to reduce duplication between similar test scenarios (e.g., `TestCollectPgStatStatements_BasicSelect` and `TestCollectPgStatStatements_DML` should share setup logic).
- Define reusable test struct types (e.g., `QueryTest`) instead of anonymous structs repeated across functions.
