# src/ Shared Review Rules

## Package Dependencies

- Avoid circular dependencies. If a lower-level package needs something from `cmd`, reconsider the design. Shared types used across packages should live in a leaf package rather than in `cmd`.

## Testing

- Use testcontainers for integration tests that need real database connections.
- Organize `TestMain` to start containers before tests and terminate them after all tests complete (via `m.Run()`). Do not rely on `defer` after `os.Exit`.
- Close database connections promptly after use rather than relying on `defer` at function scope if the connection is only needed for a small portion of the test.
