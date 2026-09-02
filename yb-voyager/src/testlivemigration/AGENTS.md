# testlivemigration Package — Engineering Standards

> **Scope:** files under `yb-voyager/src/testlivemigration/`. These are standards for **writing** code here, not only for reviewing it.
> Repo-wide standards live in `AGENTS.md` at each parent directory up to the repo root — read those as well.

End-to-end live-migration tests drive real voyager processes against containers. They are the slowest and flakiest tier, so review them for determinism first.

## Async Process Discipline

- Distinguish sync vs async command runs. After an async start, do not read the process's output buffers or shared files until the run completes or a synchronized signal fires — concurrent reads race with the writer.
- Never use a fixed `time.Sleep` to wait for a migration phase; use the framework's polling helpers with an explicit timeout.

## Assertions

- Conflict/event/row counts from streaming are timing-dependent: assert a justified range (both lower and upper bound), derived from the workload and explained in a comment. `>= 0`-style bounds are vacuous.
- When the test depends on a specific configuration being in effect (partitioning strategy, replica identity, flag value), assert that state explicitly rather than assuming it — otherwise a behavior change upstream silently changes what the test exercises.
- Prefer asserting whole expected values (full map/slice) over substring matching.

## Test Design

- Each test states in a doc comment which invariant it pins and why the chosen data pattern triggers it.
- Follow the file's `t.Parallel()` convention; deviating deserves a comment.
- Place new tests in the file matching their subject area (conflict-detection tests with conflict tests, etc.).
