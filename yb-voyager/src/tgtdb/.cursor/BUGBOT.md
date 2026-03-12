# tgtdb Package Review Rules

## Target DB Interface

- Keep the target DB interface lean. Only add methods that are genuinely needed by multiple target implementations or by the shared import logic layer. DB-specific helpers should be accessed via type assertion, not added to the interface.
- When naming interface methods, be consistent: either use the `Get` prefix for all getters or omit it for all.

## Import and COPY Logic


- After COPY, assert that rows-affected plus rows-ignored equals the batch record count. A mismatch indicates silent data loss.
- For error classification, check the PG error code class, not just the error message string. Integrity constraint violations, syntax errors, and data exceptions are generally non-retryable.
- When classifying unique constraint violations, distinguish between primary key violations and unique key violations. The primary-key-conflict IGNORE flag applies only to PK violations, not UK violations.


## Partition Handling

- Functions that query tables from system catalogs return both root and leaf partition tables. The caller is responsible for resolving leaf→root mappings. DB-specific functions should not embed command-level handling (e.g., deciding whether to return roots or leaves).
- Always handle the missing-key case when looking up table name mappings. A missing entry should produce an error, not a nil-pointer panic.
- Document whether a function returns only leaf partitions, only roots, or both, as a comment on the function.

## Failpoint Testing

- Failpoint-related code adds noise to production logic. Keep failpoint blocks as small as possible.
- Name failpoints descriptively: include whether the failure is retryable or non-retryable.

## Testing

- Follow the `TestMain` → `m.Run()` pattern for setup/teardown with testcontainers.
- Include test cases for case-sensitive table names, partitioned tables, and expression indexes, wherever applicable
- Use table-driven tests with `t.Run` to reduce duplication between similar test scenarios.
- Define reusable test struct types instead of repeating anonymous structs across functions.
