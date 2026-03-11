# cmd Package Review Rules

## Cutover and Iteration Logic

- Idempotency checks must happen *before* any side-effecting calls. Do not create iteration directories, update MSR fields, or spawn processes before verifying that the operation has not already been performed.
- When reading `MigrationStatusRecord`, always nil-check the result. `GetMigrationStatusRecord` returns `(nil, nil)` when a record is not yet created, which can happen during concurrent iteration transitions.
- The cutover status functions (`getCutoverToSourceStatus`, `getCutoverToTargetStatus`) operate on the global `metaDB`. Verify that `metaDB` points to the correct iteration's metadata before calling them.
- When implementing cutover or iteration flows, explicitly document the synchronization contract between exporter and importer processes. Which process waits for which flag, and in what order?

## Error Handling in Commands

- Command-level functions should return errors, not call `utils.ErrExit` directly (unless they are the top-level entry point for a command). This allows callers to handle errors gracefully, run deferred cleanup, and log appropriate context.
- When demoting a hard error to a warning (e.g., for non-critical prompt functions), document *why* the error is non-fatal and ensure all error paths in the called function also behave non-fatally. Watch for transitive `utils.ErrExit` calls in helper functions.

## Process Spawning

- When spawning sub-commands via `exec` for next-iteration workflows, verify:
  1. `--config-file` is passed if a config file is in use.
  2. `--export-dir` is set correctly for the new iteration.
  3. Database credentials (passwords) reference the correct source/target in the current workflow role.
  4. No flag is duplicated (e.g., `--config-file` added both explicitly and via the CLI overrides loop).

## MigrationStatusRecord Flags

- MSR boolean flags like `ImportDataToTargetStarted`, `ExportDataFromSourceStarted` must only be set by the process/role they describe. Do not set target-importer flags from a source-importer code path.
- When adding new fields to MSR, consider that the JSON serialized format must remain backward-compatible for users upgrading mid-migration.

## Assessment and Report Generation

- When filtering or transforming assessment issues, use `lo.Filter` over manual loop-and-append patterns.
- Validate the entire expected value in tests, not just a substring. Use `assert.Equal` over `assert.Contains` when the full value is known.

## Testing

- Integration tests using the live-migration testing framework must verify that loop bounds and condition checks are consistent (e.g., if iterating `for i := 1; i <= 5`, do not check `if i == 10`).
- Test helpers like `WaitForNextIterationInitialized` must read the flag from the correct metaDB (the current iteration, not the latest).
- When writing tests for commands, always include cases that exercise idempotent re-runs (calling the same operation twice should not corrupt state).
