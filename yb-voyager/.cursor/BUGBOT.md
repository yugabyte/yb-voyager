# Go Code Review Rules (yb-voyager)

## Interface Design

- Keep interfaces lean. Only add methods to `TargetDB` / `SourceDB` interfaces if they are needed by multiple implementations or callers. DB-specific helpers should be type-asserted, not added to the shared interface.
- When interface methods return only one of their return values in practice, reconsider whether the extra return values are needed.
- Include named parameters in interface method signatures for clarity (e.g., `ImportBatch(table string, args ImportBatchArgs) error` instead of bare types).

## Global State

- The global `metaDB` variable is a known tech debt. Any function that temporarily replaces it must restore the original value (typically via `defer`). Add a comment explaining why the swap is necessary.
- Avoid introducing new global variables. Prefer passing dependencies explicitly through function arguments or struct fields.

## Object Names

- Use the `sqlname` package (`sqlname.ObjectName`, `sqlname.NewObjectNameWithQualifiedName`, `AsQualifiedCatalogName()`) for all object name handling. Do not construct qualified names with `fmt.Sprintf("%s.%s", schema, table)`.
- When passing schema names to `sqlname` constructors, pass the default schema name (e.g., `"public"`), not the database name.

## Flag and Config Handling

- When spawning sub-processes for the next iteration or fall-back/fall-forward workflows, verify that all required flags (`--config-file`, `--export-dir`, `--table-list`) are passed. Missing flags have caused production bugs.
- When adding a new flag to a command, check whether it needs to be propagated to related commands (e.g., `export-data` flags may need to reach `import-data-to-source`).

## Idempotency

- State-modifying operations (cutover initiation, iteration creation, MSR updates) must be idempotent. Always check whether an operation was already performed before executing side effects.
- Consider crash recovery: if the process dies midway through a multi-step state change, will a re-run produce correct behavior?

## Logging

- Use appropriate log levels: `log.Infof` for normal progress, `log.Warnf` for recoverable issues the user should know about, `log.Errorf` for failures. Do not log errors at Info level or progress at Warn level.
- Include the table name, file path, or operation context in log messages. Bare "failed to do X" messages without context are not actionable.

## Testing

- For test files in Go packages, use the `unit` build tag (`//go:build unit`).
- Use `testify/assert` assertions consistently. Avoid mixing `t.Errorf` with `assert.Equal` in the same test file.
- Use `testify/require` for setup steps that must succeed for the test to be meaningful.
- When testing error paths, verify the specific error type or message — not just that an error occurred.
