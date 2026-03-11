# callhome Package Review Rules

## Payload Safety

- Never include sensitive user data in callhome payloads: no column names, no row values, no table contents, no file paths that expose directory structure, no database passwords or connection strings.
- When adding new string fields to payload structs, verify they are processed by the payload sanitizer. The sanitizer recursively walks structs and strips anchor tags / sensitive patterns from string fields.
- Descriptions that include format specifiers (`%s`, `%v`) must have those specifiers replaced in the sanitized payload to avoid leaking specific object names.

## Payload Versioning

- When changing any struct serialized into callhome or YugabyteD payloads, increment the relevant payload version constant. There are separate versions for the general assessment payload and the YugabyteD payload; both may need incrementing when shared structs change.
- Document version history as comments near the version constants so reviewers can trace what changed in each version.

## Testing

- Verify expected callhome JSON payloads in migtests. When changing payload fields, update these expected files and confirm they still match actual outputs.
- Do not rely solely on field counts for assertion. Validate the full structure of new or changed payload fields.
