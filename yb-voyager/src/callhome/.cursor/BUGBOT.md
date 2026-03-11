# callhome Package Review Rules

## Payload Safety

- Never include sensitive user data in callhome payloads: no column names, no row values, no table contents, no file paths that expose directory structure, no database passwords or connection strings.
- When adding new fields to payload structs, verify they are processed by `sanitizePayload`. The sanitizer recursively walks structs and strips anchor tags / sensitive patterns from string fields.
- When changing any struct used in callhome payloads (`SizingRecommendation`, `AssessmentIssue`, callhome-specific structs), increment the relevant payload version constant.
- Descriptions that include user-facing format specifiers (`%s`, `%v`) must have those specifiers replaced with `XXX` in the sanitized payload to avoid leaking specific object names.

## Payload Versioning

- Payload version constants live in `cmd/common.go`. There are separate versions for the general assessment payload and the YugabyteD payload. Both may need incrementing when shared structs change.
- Document version history as comments near the version constants so reviewers can trace what changed in each version.

## Testing

- Verify expected callhome JSON payloads in migtests (`expected_callhome_payloads/`). When changing payload fields, update these expected files and confirm they still match actual outputs.
- Do not rely solely on field counts for assertion. Validate the full structure of new or changed payload fields.
