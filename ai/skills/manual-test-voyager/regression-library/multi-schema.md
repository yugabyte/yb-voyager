# Multi-schema

Why it matters: a source with more than one schema stresses object routing, name collisions across schemas, and `--source-db-schema` handling (single, list, all). PG-source migrations flatten differently than Oracle/MySQL. Bugs here silently drop or misroute objects. QA "commonly impacted" area.

## Scenarios

### multi-schema-1: two schemas, same table name
- Flow: offline
- Origin: QA priority; name-collision edge
- Setup: source with schemas `s1` and `s2`, each containing a table `t` with different data
- Command: `export schema/data --source-db-schema s1,s2 ...` → import → validate
- Expected oracle: both `s1.t` and `s2.t` exist on target with correct, non-swapped data; row+content parity per schema. (For PG source, confirm how schemas map onto the target — no silent merge.)
- Status: seeded

### multi-schema-2: single schema selected from many
- Flow: offline
- Origin: `--source-db-schema` selectivity
- Setup: source with 3 schemas
- Command: `--source-db-schema s2` only
- Expected oracle: only `s2` objects/data migrated; `s1`/`s3` absent; no error about the unselected schemas.
- Status: seeded

### multi-schema-3: cross-schema dependencies
- Flow: offline
- Origin: FK / view / function referencing another schema
- Setup: `s2.orders` FK → `s1.customers`; a view in `s2` selecting from `s1`
- Command: full offline pipeline with both schemas
- Expected oracle: dependencies resolve; FK enforced on target; view valid; no `failed.sql`.
- Status: seeded

### multi-schema-4: schema with special / quoted identifiers
- Flow: offline
- Origin: case-sensitivity / reserved words (namereg / NameTuple handling)
- Setup: schema and table names needing quoting (mixed case, reserved word)
- Command: full offline pipeline
- Expected oracle: identifiers preserved exactly (case-sensitive) on target; queries against them work.
- Status: seeded
