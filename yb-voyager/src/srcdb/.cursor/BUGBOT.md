# srcdb Package Review Rules

## Source DB Abstraction

- Interface methods should return source-agnostic types. DB-specific internals (catalog column names, internal sequence naming) should be handled inside the implementation, not leaked to callers.
- When implementing a method for multiple source types (PostgreSQL, Oracle, MySQL, YugabyteDB-as-source), keep the logic parallel across implementations. If one implementation handles edge cases (e.g., PG version differences), the others should at minimum document that the edge case does not apply.

## Schema and Object Queries

- Use the `sqlname` package for table and sequence names returned to callers or in input args. Do not use raw strings.
- Use qualified catalog name helpers of NameTuple when building SQL queries against system catalogs. Do not manually concatenate `schema.table` strings.
- When SQL queries differ between DB versions, add inline `-- comments` explaining what changed and why the fallback exists.
- Add `WHERE` clauses to filter by schema/owner. Queries against system catalogs without schema filtering will return objects from unrelated schemas.

## Unsupported Datatype Handling

- Maintain clear lists of unsupported datatypes per connector type (logical replication, gRPC, ora2pg). When adding or removing a type, update all relevant lists.
- When checking if a column type is unsupported, handle both qualified (`public.hstore`) and unqualified (`hstore`) type names. Extension-defined types may appear qualified in catalog queries but unqualified in the unsupported types list.
- Use case-insensitive comparison for datatype names.

## Column Order and Metadata

- Column lists must preserve attribute-number ordering from the catalog. This ordering is relied upon by Debezium and pg_dump-based export paths. Do not use an unordered map for column metadata if ordering matters.
- When storing column metadata, distinguish between the internal type name (e.g., `int4`) and the user-facing SQL type name (e.g., `integer`). Detection logic should use internal names; display/report logic should use SQL names.

## Permissions and Guardrails

- Permission check functions and guradrails should return errors, not terminate the process. The caller decides whether a missing permission is fatal or can be logged and continued. 

## Replication and Sequences

- When querying replication slots and streaming replicas, handle the case where active PID is NULL (slot exists but no active consumer).
- Sequence queries must handle all association types: SERIAL/BIGSERIAL, explicit `CREATE SEQUENCE` + `DEFAULT nextval(...)`, `ALTER SEQUENCE ... OWNED BY`, and `GENERATED ALWAYS AS IDENTITY`.

## Testing

- Integration tests must use testcontainers. Each test should set up its own schema, run queries, assert results, and clean up.
- Always include test cases with case-sensitive table, column, and schema names.
- When testing with multiple scenarios in one test function, use `t.Run` sub-tests with descriptive names so failures are easy to locate.
- When large SQL queries differ only in a few clauses, add comments highlighting the differences.
