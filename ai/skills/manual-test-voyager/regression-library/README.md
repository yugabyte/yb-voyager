# Regression library

A growing store of adversarial scenarios that Voyager must keep passing. Each entry is a QA-priority area or a real bug's repro, distilled to something this skill can run. Phase 1 pulls in every entry matching the change's affected surface; Phase 4 appends new entries for every gap found.

This is the durable memory of "things that broke or almost broke." Never let a discovered failure mode leave without an entry here.

## How to use
- **During planning (Phase 1):** for each affected area (see the diff's affected-surface table), include every scenario from the matching entry as a mandatory plan row.
- **After a run (Phase 4):** if you found a bug or a should-have-existed scenario, add it to the relevant entry (or create a new one) using the format below.

## Entry format
```
### <area>-<n>: <short name>
- Flow: offline | live | both
- Origin: <PR #, bug link, QA guidance, review finding, or "exploratory">
- Setup: <fixture requirements — e.g. table with secondary index>
- Command: <the exact yb-voyager invocation, with flags>
- Expected oracle: <PASS condition — the single source of truth>
- Status: validated (seen to pass on a good build) | seeded (proposed, not yet run) | regressed (currently failing — link)
```

## Index
- `adaptive-parallelism.md` — parallelism/pool boundary cases (the anchor hang). **validated**
- `partitions.md` — range/list/hash partition routing, table-list interaction.
- `sequences.md` — sequence/identity advance after migration.
- `status-commands.md` — export/import data status, cutover status, data-migration-report.
- `multi-schema.md` — 2+ source schemas: routing, collisions, `--source-db-schema` handling.

## Seeding priorities (QA north star)
Partitions, sequences, status commands, and multi-schema are the "commonly impacted" areas — keep these entries the richest, and add negative/edge cases (unreachable host, invalid flags, wrong commands) across all of them.
