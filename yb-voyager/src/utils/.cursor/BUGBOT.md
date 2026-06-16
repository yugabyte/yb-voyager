# utils Package Review Rules

- Pre-compile regexes at package level. Do not call `regexp.MustCompile` inside functions that are called in loops.
- Use existing utility functions before adding new ones. Check what already exists in the package to avoid duplication.
- Keep test utilities in the separate `test/utils` (testutils) package — not in `src/utils` — so they are not linked into production binaries.
- When adding utility functions, give them clear names that convey the input/output types.
