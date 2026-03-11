# anon (SQL Anonymizer) Package Review Rules

## Parse Tree Processing

- Each DDL object type (CREATE TABLE, CREATE VIEW, CREATE FUNCTION, etc.) should have a dedicated handler function whose name matches the node type.
- When switch cases differ only in the prefix applied, factor out the common anonymization logic and resolve the prefix in each case. Do not duplicate the full anonymization block across cases.
- Handle nil pointers before accessing proto node fields. Use early nil checks at the top of handler functions.
- Reuse existing helper functions for traversing and anonymizing string nodes, range variables, column references, etc., instead of manually duplicating that logic.

## Identifier Hashing

- The hash function already handles empty strings by returning empty. Do not add redundant empty-string checks before every call.
- Use consistent prefixes for anonymized identifiers: `table_`, `column_`, `index_`, `type_`, `const_string_`, `const_int_`, etc. The prefix should convey the object type to aid analysis of anonymized schemas.
- For type names, distinguish between built-in PG types (which should NOT be anonymized) and user-defined types (which should). Be aware that users can create types with the same name as built-in types in custom schemas.

## Built-in Functions and Types

- When checking if a function is built-in, handle both qualified (`pg_catalog.nextval`) and unqualified (`nextval`) forms.
- Sequence functions (`nextval`, `setval`, `currval`) need special handling: the sequence name argument must be anonymized as a sequence identifier, not as a generic constant.
- Do not anonymize boolean literal values — they carry no sensitive information and obscure the SQL structure.
- For generic parameters like length, delimiter, or precision, consider whether anonymization is necessary. These are structural, not user-specific.

## Code Style

- Use early returns to avoid deep nesting. Handlers in this package tend to accumulate 5-6 levels of `if` nesting; flatten them.
- Add SQL examples in comments for each handler function showing the DDL pattern being processed.
- When adding support for a new DDL type, add corresponding unit tests covering all variants listed in the PostgreSQL docs.

## Scope of Anonymization

- Views, materialized views, and triggers are not anonymized as top-level DDL objects. However, if they appear inside other objects' DDL (e.g., a view referenced in a function body), their identifiers within that context should be anonymized.
- It is acceptable for the anonymizer to handle DDL types that the underlying dump tool (pg_dump, ora2pg) may not currently export, as long as the handling is correct. Do not remove support for a DDL type just because it is not currently dumped.

## Testing

- Each DDL object type must have test cases.
- Test cases should cover all documented variants of a DDL type (e.g., for `CREATE TYPE`: composite, enum, range, base, shell).
- Include test cases for DDL types that are not currently exported but may be in the future, marked with comments indicating they are forward-looking.
