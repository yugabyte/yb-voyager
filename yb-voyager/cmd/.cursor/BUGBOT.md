# cmd Package Review Rules

## Cutover Orchestration

Live migration involves multiple concurrent processes (exporter, importer, optional fall-back/fall-forward processes) coordinating via MSR flags:

- Cutover status checks must read the correct iteration's metaDB. The global `metaDB` may point to a different iteration than expected.
- MSR boolean flags must be set only by the process/role they describe. Do not set target-importer flags from a source-importer code path.
- When one process waits for another's flag, add a timeout or at minimum print a message so the user knows something is happening. Infinite silent polls have caused apparent hangs.
- Idempotency checks must happen *before* any side-effecting calls. Do not create iteration directories, update MSR fields, or spawn processes before verifying that the operation has not already been performed.
- When reading migration status records, always nil-check the result. Lookups return `(nil, nil)` when a record is not yet created, which can happen during concurrent iteration transitions.
- When implementing cutover or iteration flows, explicitly document the synchronization contract between exporter and importer processes. Which process waits for which flag, and in what order?

## Error Handling in Commands

- Command-level functions should return errors, not call `utils.ErrExit` directly (unless they are the top-level entry point for a command). This allows callers to handle errors gracefully, run deferred cleanup, and log appropriate context.
- When demoting a hard error to a warning (e.g., for non-critical prompt functions), document *why* the error is non-fatal and ensure all error paths in the called function also behave non-fatally. Watch for transitive `utils.ErrExit` calls in helper functions.

## Process Spawning

- When spawning sub-commands via `exec` for next-iteration workflows, verify:
  1. `--config-file` is passed if a config file is in use.
  2. `--export-dir` is set correctly for the new iteration.
  3. Database credentials (passwords) reference the correct source/target in the current workflow role.
  4. No flag is duplicated (e.g., added both explicitly and via the CLI overrides loop).

## MSR Flag Discipline

- MSR boolean flags (started, requested, processed) must only be set by the process/role they describe. Do not set target-importer flags from a source-importer code path.
- When adding new fields to MSR, consider that the JSON serialized format must remain backward-compatible for users upgrading mid-migration.

## Assessment and Report Generation

- When filtering or transforming assessment issues, prefer declarative helpers (e.g., `lo.Filter`) over manual loop-and-append patterns.
- Validate the entire expected value in tests, not just a substring.

## Testing

- Integration tests using the live-migration testing framework must verify that loop bounds and condition checks are consistent.
- Test helpers that wait on MSR flags must read from the correct iteration's metaDB, not the parent or latest.
- When writing tests for commands, always include cases that exercise idempotent re-runs (calling the same operation twice should not corrupt state).
