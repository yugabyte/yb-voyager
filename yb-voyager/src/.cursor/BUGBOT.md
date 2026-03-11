# src/ Shared Review Rules

These rules apply to all packages under `yb-voyager/src/`.

## Package Dependencies

- Avoid circular dependencies. If a function in a lower-level package (e.g., `utils`, `errs`) needs something from `cmd`, reconsider the design. Shared types used across packages should live in a leaf package (e.g., `types`, `utils/commonVariables.go`) rather than in `cmd`.
- The `errs` package should remain lightweight — it defines error types, not error-handling logic. DB-specific error construction (e.g., `NewImportBatchErrorForPgErr`) belongs in the relevant DB package (`tgtdb`, `srcdb`).

## metadb Package

- `MigrationStatusRecord` is a large, growing struct. When adding new fields, group related fields into sub-structs (e.g., `CutoverFlags`, `ExportFlags`) instead of adding flat fields. This is a known tech debt tracked in issue #2409.
- JSON tags on MSR fields must remain stable for backward compatibility. Renaming a JSON tag is a breaking change for users upgrading mid-migration.
- Field ordering convention: `Requested` → `Detected` → `Processed` for cutover-related flags.
- Add descriptive comments to MSR fields explaining their semantics and which process/role sets them.

## utils Package

- Pre-compile regexes at package level. Do not call `regexp.MustCompile` inside functions that are called in loops.
- Use existing utility functions before adding new ones. Check for `FileOrFolderExists`, `ConnectToSQLiteDatabase` (note: SQLite, not Sqlite), `lo.Keys`, `mapset` helpers, `jsonfile.Load`, etc.
- Keep test utilities in the separate `test/utils` (testutils) package — not in `src/utils` — so they are not linked into production binaries.
- When adding utility functions, give them clear names that convey the input/output types: `SnakeCaseToTitleCase`, `BuildObjectName`, `WaitForLineInLogFile`.

## errs Package

- Error types should implement the standard `error` interface.
- Stack traces / execution history should be multi-line and formatted for readability, not single-line strings.
- DB-specific error extraction logic (`pgErr.Code`, `pgErr.Where`, etc.) should live in the DB-specific packages, not in `errs`.

## callhome Package

- Never include sensitive information in callhome payloads: no column names, no data values, no user-identifying information.
- When changing structs that are serialized into callhome or YugabyteD payloads, increment the corresponding payload version constant (`ASSESS_MIGRATION_PAYLOAD_VERSION`, `ASSESS_MIGRATION_YBD_PAYLOAD_VERSION`, etc.) in `cmd/common.go`.
- Sanitize payloads using `sanitizePayload`. When adding new string fields to payload structs, verify they are handled by the sanitizer.

## Testing

- Use testcontainers for integration tests that need real database connections.
- Organize `TestMain` to start containers before tests and terminate them after all tests complete (via `m.Run()`). Do not rely on `defer` after `os.Exit`.
- Close database connections promptly after use rather than relying on `defer` at function scope if the connection is only needed for a small portion of the test.
