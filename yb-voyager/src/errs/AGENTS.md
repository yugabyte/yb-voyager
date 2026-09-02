# errs Package — Engineering Standards

> **Scope:** files under `yb-voyager/src/errs/`. These are standards for **writing** code here, not only for reviewing it.
> Repo-wide standards live in `AGENTS.md` at each parent directory up to the repo root — read those as well.

- This package should remain lightweight — it defines error types, not error-handling logic.
- Error types should implement the standard `error` interface.
- Stack traces / execution history should be multi-line and formatted for readability, not single-line strings.
- DB-specific error extraction logic should live in the relevant DB package (`tgtdb`, `srcdb`), not here. This avoids circular dependencies and keeps each DB's error handling self-contained.
