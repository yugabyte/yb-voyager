# debezium-server-voyager Review Rules

Java/Maven sources for the Debezium CDC plugin used by live migration. Changes here run on the per-event export hot path.

## Cross-Language Test Coverage

- A change to value conversion, record transformation, or type handling must come with (or explicitly reference) corresponding Go-side live-migration tests — including the fall-back/fall-forward direction where applicable. Java-side unit tests alone do not exercise the shipped pipeline.

## Data Handling

- Handle SQL NULL and missing fields explicitly for every type a converter touches; a null-path NPE crash-loops the streaming phase.
- Value conversion runs per event: avoid per-record allocations, regex compilation, and lookups that can be hoisted or cached.
- New or changed datatype handling must state which datatypes are affected and be checked against both directions (source→target and target→source for fall-back).

## Compatibility

- The plugin is built with JDK 17 and deployed alongside a specific Debezium version; do not introduce APIs beyond those versions.
- Changes to the on-disk queue segment format or metadata must remain readable by the Go importer across a mid-migration upgrade.
