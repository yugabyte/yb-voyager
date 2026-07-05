# Partitions

Why it matters: partitioned tables have many moving parts across a migration — parent vs child DDL, `ATTACH PARTITION`, index attachment, data routing into the right partition, and table-list semantics (does listing the parent include children?). A routing bug shows as correct parent totals but wrong per-partition counts. QA "commonly impacted" area.

## Scenarios

### partitions-1: range-partitioned round-trip
- Flow: offline
- Origin: QA priority + validated in the skill's smoke fixture
- Setup: range-partitioned `events` (2+ partitions), rows spread across all partitions
- Command: full offline pipeline (export schema → export data → import schema → import data)
- Expected oracle: parent total AND each partition's `count(*)` match source; partitions attached (`\d+ events` shows children on target); content hash per partition matches.
- Status: validated (parent+children imported correctly in the smoke run: events=4000 across events_2023=2000/events_2024=2000)

### partitions-2: list + hash partitioning
- Flow: offline
- Origin: coverage gap beyond range
- Setup: one LIST-partitioned and one HASH-partitioned table
- Command: full offline pipeline
- Expected oracle: same per-partition parity; correct partition strategy preserved on target.
- Status: seeded

### partitions-3: table-list with a partitioned table
- Flow: offline
- Origin: table-list × partition interaction (classic edge)
- Setup: partitioned table + other tables
- Command: `export data --table-list public.events ...` (parent only) and separately listing a single child
- Expected oracle: documented, consistent behavior — listing the parent migrates all partitions' data; listing a child behaves per docs. No silent data loss of unlisted partitions.
- Status: seeded

### partitions-4: partition added between export and import
- Flow: live
- Origin: schema-evolution edge during live migration
- Setup: live migration; add a new partition on the source during CDC
- Command: live pipeline with a delta that inserts into a newly-added partition
- Expected oracle: rows route correctly or a clear, documented limitation is surfaced — not silent loss.
- Status: seeded
