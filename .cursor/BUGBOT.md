# YugabyteDB Voyager — Project-Wide Review Rules

## Error Handling

- Never silently swallow errors. If a function returns an error, either handle it, return it, or log it with sufficient context. Do not use `log.Warnf` and continue when the error indicates a real failure.
- Do not call `utils.ErrExit` inside functions that are expected to return errors to their callers. `ErrExit` terminates the process and bypasses deferred cleanup, error wrapping, and caller-level recovery.
- When wrapping errors, include enough context to trace the source: table name, file path, operation attempted, etc.

## Naming

- Use descriptive, source-database-agnostic names for functions and variables. Prefer `GetQueryStats` over `GetPgStatStatements`. The tool supports PostgreSQL, Oracle, and MySQL sources.
- Follow Go naming conventions: `CamelCase` for exported identifiers, `camelCase` for unexported. Avoid `this` as a receiver name — use short, descriptive names consistent with Go idiom.
- Issue names (in assessment reports) should describe the problem, not the recommendation. E.g., "Missing Primary Key for Table" not "Add Primary Key Recommendation."
- Variable names like `rec1`/`rec2` are unclear; prefer `recCombined`/`recSharded` or other self-describing names.

## Constants and Magic Values

- Use named constants for string literals, magic numbers, SQL error codes, and repeated query fragments. Do not scatter raw literals across multiple files.
- Prefer `const` over `var` for values that never change.
- Pre-compile regexes at package level (`var fooRegex = regexp.MustCompile(...)`) instead of inside functions that may be called repeatedly.

## Nil and Boundary Checks

- Always nil-check pointers before dereferencing, especially for `MigrationStatusRecord` lookups which can return `(nil, nil)` when a record is not found.
- When accessing map entries, use the two-value form (`val, ok := m[key]`) and handle the missing-key case explicitly. This avoids panics and silent zero-value bugs.
- Check slice bounds before indexing (e.g., `indexParams[0]`).

## Backward Compatibility and Upgrades

- Changing JSON struct field names or removing fields from serialized structs (MSR, assessment report, callhome payloads) is a breaking change for users who upgrade mid-migration. JSON tags must remain stable; add new fields rather than rename existing ones.
- When adding fields to callhome or YugabyteD payloads, increment the corresponding payload version constant.

## Code Organization

- Remove dead code, unused functions, and leftover debugging artifacts before merging.
- Consolidate duplicate logic into shared helper functions rather than copy-pasting across cases.
- Use early returns to reduce nesting depth; avoid 5+ levels of `if` indentation.
- Place struct methods immediately below the struct definition for readability.
- Use the `lo` library helpers (`lo.Filter`, `lo.Keys`, `lo.Map`, `lo.Some`, `lo.Ternary`, `lo.Without`) instead of manual loops where they improve clarity.

## Concurrency

- Shared mutable state must be protected by a mutex or channel. Document the synchronization strategy when multiple goroutines access the same data.
- When using global variables (e.g., the global `metaDB`), add comments explaining why they are set/restored and what the invariant is.

## SQL Queries

- Add inline comments (`-- comment`) in multi-line SQL strings to explain non-obvious clauses, joins, or filter conditions.
- When the same SQL pattern is used for both PostgreSQL and YugabyteDB, extract it into a shared constant and document any version-specific differences.
- Use parameterized queries or prepared statements rather than `fmt.Sprintf` with user-supplied values.

## Case Sensitivity

- Always consider case-sensitive identifiers (table names, column names, schema names). YugabyteDB Voyager must handle quoted identifiers correctly through the `sqlname` package. Any new code dealing with object names should use `sqlname.ObjectName` or equivalent, not raw strings.

## Testing Rules

- Use the `unit` build tag for unit tests. Run with `go test -tags unit ./...`.
- Use `assert.Equal(t, expected, actual)` with the expected value first. Swapping expected/actual produces confusing failure messages.
- Use `assert.ElementsMatch` for unordered comparisons instead of manually sorting.
- Prefer table-driven tests with `t.Run(name, func(t *testing.T) { ... })` for multiple scenarios. Each sub-test should have a descriptive name.
- Each test should be self-contained: set up its schema objects, run assertions, and clean up. Do not rely on shared mutable state across tests.
- Integration tests that use testcontainers should clean up their own resources (schemas, tables, sequences).
- Always include test cases for case-sensitive table and column names.
