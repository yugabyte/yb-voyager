# Go Code Review Rules (yb-voyager)

## Package Layering

- The `cmd/` package should ideally contain only command-handling logic: Cobra command definitions, CLI flag parsing, config-file resolution, invoking other sub-commands, and top-level orchestration. Core business logic (data import/export algorithms, schema analysis, conflict detection, assessment calculations, etc.) should reside in dedicated packages under `src/`. This separation is not consistently followed in the existing codebase, but new code should strive for it as much as possible. When adding significant new logic, prefer creating or extending a package under `src/` and calling it from `cmd/`, rather than embedding it directly in `cmd/`.

## Error Handling

- Never silently swallow errors. If a function returns an error, either handle it, return it, or log it with sufficient context. Do not `log.Warnf` and continue when the error indicates a real failure.
- Do not call `utils.ErrExit` inside functions that are expected to return errors to their callers. `ErrExit` terminates the process and bypasses deferred cleanup, error wrapping, and caller-level recovery.
- When wrapping errors, include enough context to trace the source: table name, file path, operation attempted, etc.

## Nil and Boundary Checks

- Always nil-check pointers before dereferencing. In particular, metaDB record lookups can return `(nil, nil)` when a record is not found.
- When accessing map entries, use the two-value form (`val, ok := m[key]`) and handle the missing-key case explicitly.
- Check slice bounds before indexing.

## Constants and Magic Values

- Use named constants for string literals, magic numbers, SQL error codes, and repeated query fragments.
- Prefer `const` over `var` for values that never change.
- Pre-compile regexes at package level (`var fooRegex = regexp.MustCompile(...)`) instead of inside functions that may be called repeatedly.

## Interface Design

- Keep interfaces lean. Only add methods to shared interfaces (source DB, target DB) if they are needed by multiple implementations or callers. DB-specific helpers should be accessed via type assertion (if they are applicable only for that DB), and not added to the shared interface.
- Include named parameters in interface method signatures for clarity.

## Global State

- Avoid introducing new global variables. Prefer passing dependencies explicitly through function arguments or struct fields.


## Object Names

- Use the `sqlname` package for all object name handling(table names especially). Do not construct qualified names via manual string concatenation.


## Flag and Config Handling

- When adding a new flag to a command, check whether it needs to be propagated to related commands (e.g., `export-data` flags may need to reach `import-data-to-source`).
- When adding a new flag, ensure that it is supported by config-file, CLI both, and added to the config file templates. (yb-voyager/config-templates)

## Idempotency

- State-modifying operations (cutover initiation, iteration creation, MSR updates) must be idempotent. Always check whether an operation was already performed before executing side effects.
- Consider crash recovery: if the process dies midway through a multi-step state change, will a re-run produce correct behavior? This is especially true for data migration where the commands are resumable.

## Concurrency

- Shared mutable state must be protected by a mutex or channel. Document the synchronization strategy when multiple goroutines access the same data.
- When using global variables, add comments explaining why they are set/restored and what the invariant is.

## Logging

- Use appropriate log levels: `log.Infof` for normal progress, `log.Warnf` for recoverable issues the user should know about, `log.Errorf` for failures. Do not log errors at Info level or progress at Warn level.
- Include the table name, file path, or operation context in log messages.

## SQL Queries

- Add inline comments (`-- comment`) in multi-line SQL strings to explain non-obvious clauses, joins, or filter conditions.
- When the same SQL pattern is used for both PostgreSQL and YugabyteDB, extract it into a shared constant and document any version-specific differences.
- Use parameterized queries or prepared statements rather than `fmt.Sprintf` with user-supplied values.

## Code Organization

- Use the `lo` library helpers (`lo.Filter`, `lo.Keys`, `lo.Map`, `lo.Some`, `lo.Ternary`, `lo.Without`) instead of manual loops where they improve clarity.
- Place struct methods immediately below the struct definition for readability.

## Testing

- Use the `unit` build tag for unit tests (`//go:build unit`). Run with `go test -tags unit ./...`.
- Use `assert.Equal(t, expected, actual)` with the expected value first. Swapping expected/actual produces confusing failure messages.
- Use `assert.ElementsMatch` for unordered comparisons instead of manually sorting.
- Prefer table-driven tests with `t.Run(name, func(t *testing.T) { ... })` for multiple scenarios.
- Use `testify/require` for setup steps that must succeed for the test to be meaningful.
- Each test should be self-contained: set up its schema objects, run assertions, and clean up.
- Integration tests that use testcontainers should clean up their own resources.
- Always include test cases for case-sensitive table and column names, wherever applicable.
- When testing error paths, verify the specific error type or message — not just that an error occurred.
