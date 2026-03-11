# srcdb Package Review Rules

## Source DB Abstraction

- The `SourceDB` interface methods should return source-agnostic types. DB-specific internals (e.g., `pg_catalog` column names, Oracle `ISEQ$$` naming) should be handled inside the implementation, not leaked to callers.
- When implementing a method for multiple source types (PostgreSQL, Oracle, MySQL, YugabyteDB-as-source), keep the logic parallel across implementations. If one implementation handles edge cases (e.g., PG version differences), the others should at minimum document that the edge case does not apply.

## Schema and Object Queries

- Use `sqlname.ObjectName` for table and sequence names returned to callers. Do not return raw `string` tuples.
- Use `table.AsQualifiedCatalogName()` when building SQL queries against `pg_catalog` or `information_schema`. Do not manually concatenate `schema.table` strings.
- When SQL queries differ between PG versions (e.g., `pg_sequences` vs `information_schema.sequences`), add inline `-- comments` explaining what changed between versions and why the fallback exists.
- Add `WHERE` clauses to filter by schema/owner. Queries against `information_schema` or Oracle catalogs without schema filtering will return objects from unrelated schemas.

## Unsupported Datatype Handling

- Maintain clear lists of unsupported datatypes per connector type (logical replication, gRPC, ora2pg). When adding or removing a type, update all relevant lists.
- When checking if a column type is unsupported, handle both qualified (`public.hstore`) and unqualified (`hstore`) type names. Extension-defined types may appear qualified in catalog queries but unqualified in the unsupported types list.
- Use `strings.EqualFold` for case-insensitive datatype comparisons.

## Column Order and Metadata

- Column lists returned by `GetAllTableColumnsInfo` must preserve the `attnum` ordering from the catalog. This ordering is relied upon by Debezium and pg_dump-based export paths. Do not use a `map` for column metadata if ordering matters.
- When storing column metadata, distinguish between the parser/internal type name (e.g., `int4`) and the user-facing SQL type name (e.g., `integer`). Detection logic should use internal names; display/report logic should use SQL names.

## Permissions and Guardrails

- Permission check functions should return errors, not call `utils.ErrExit`. The caller decides whether a missing permission is fatal.
- Guardrails (permission warnings, PGSS extension checks) are informational prompts, not hard failures. If a guardrail check itself fails (e.g., the permission query errors), log a warning and continue rather than terminating.

## Replication and Sequences

- When querying replication slots and streaming replicas, handle the case where `active_pid` is NULL (slot exists but no active consumer).
- Sequence queries must handle all association types: SERIAL/BIGSERIAL, explicit `CREATE SEQUENCE` + `DEFAULT nextval(...)`, `ALTER SEQUENCE ... OWNED BY`, and `GENERATED ALWAYS AS IDENTITY`.

## Testing

- Integration tests must use testcontainers. Each test should set up its own schema, run queries, assert results, and clean up.
- Always include test cases with case-sensitive table, column, and schema names.
- When testing with multiple scenarios in one test function, use `t.Run` sub-tests with descriptive names so failures are easy to locate.
- When large SQL queries differ only in a few clauses, add comments highlighting the differences.
