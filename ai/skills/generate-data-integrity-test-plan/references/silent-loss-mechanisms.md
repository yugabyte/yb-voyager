# How voyager can lose or corrupt data silently

Every case in a plan attacks one of these mechanisms. Each entry: what goes wrong, why nothing fails, the areas whose changes can trigger it, and the workload shape that exposes it. IDs are stable — plans reference them.

Background that most mechanisms rely on:
- CDC events are routed to `NUM_EVENT_CHANNELS` (default 100) channels by a **partition key** (PK values, table name, or custom key columns). Order is guaranteed only **within** a channel.
- Streamed INSERT is `INSERT … ON CONFLICT (<event.Key>) DO NOTHING`. UPDATE/DELETE are `… WHERE <event.Key>`; **rows affected ≠ 1 is only a WARN** (`yugabytedb.go` "unexpected rows affected").
- Conflict detection (`conflictDetectionCache.go`) serialises cross-channel events that share a unique-index value (plus the PK for custom-key tables). It is skipped for `PARTITION_BY_TABLE`.
- Unique violations (`23505`) are non-retryable → the importer exits. That is **loud**, not silent.

## M1. Same-row events split across channels, INSERT dropped by DO NOTHING
DELETE(row) and INSERT(same PK) land on different channels and apply in the wrong order: the INSERT hits the still-present row, is skipped by `ON CONFLICT DO NOTHING`, then the DELETE removes it. Target ends up missing the row; nothing errors.
- Triggers: routing key differs between the two events (custom key changed or reused, value spelled differently in before/after images, PK guard columns ≠ the PK used by ON CONFLICT, partitions with different PKs).
- Areas: `cdc-routing`, `conflict-detection`, `partitions`, `value-encoding`.
- Workload: delete + re-insert the same PK under a different routing value; row movement across partitions; PK change.
- Found: fuzzer P10 (PK guard from a random leaf) — latent behind the export guardrail.

## M2. Missed conflict on a unique index (usually loud, sometimes silent)
Conflict detection fails to serialise a free-then-reuse of a unique value. Normally the target raises `23505` (loud). It becomes **silent** when the colliding constraint is the one `ON CONFLICT` targets (then the row is dropped — see M1), or when the violation is absorbed (IGNORE policies, retries that "succeed" later).
- Triggers: unique index unknown to the importer (added later, target-only, on a leaf only), value normalisation mismatch, NULL semantics, partial-index predicates, subset-of-composite updates, changed-columns-only update images.
- Areas: `conflict-detection`, `value-encoding`, `partitions`.

## M3. Statement matches the wrong set of rows
UPDATE/DELETE `WHERE <key>` where the key doesn't identify one row on the target table the statement runs against: extra rows are modified/deleted, or zero rows are touched. Only a WARN.
- Triggers: statement runs against a partitioned root while the key is a leaf-local PK (same id in several leaves); PK on target differs from source; key columns missing/renamed; case-folding of identifiers.
- Areas: `apply-sql`, `partitions`, `identifiers`, `guardrails`.
- Found: **yb-voyager#3834** (root without PK + `--use-partition-root true`, K4b).

## M4. Events never captured
Rows written to objects that aren't in the publication / table list / schema list never reach the queue. Snapshot and CDC can disagree about what's included.
- Triggers: partitions in schemas outside `--source-db-schema`; partitions created mid-migration; `--table-list` naming only some partitions or only the root; exclude lists; name-registry misses; publication `publish_via_partition_root` choices.
- Areas: `table-selection`, `partitions`, `snapshot`.
- Found: K10 (leaf in an unlisted schema, silent), J1 (new partition mid-stream, documented limitation but silent).

## M5. Value transformation
The value stored on the target differs from the source (lost scale, truncated precision, time-zone shifts, encoding, array/JSON formatting, NaN/-0, TOAST placeholders).
- Areas: `value-encoding`, `snapshot` (live snapshot goes through Debezium, offline through `pg_dump`).
- Found: unconstrained `numeric` loses trailing zeros in live migration (`4.000` → `4`); `numeric(p,s)` unaffected.

## M6. Resume / restart replays or skips work
After a crash or restart, per-channel `lastAppliedVsn` differs across channels and the conflict cache starts empty. Events can be skipped (VSN ≤ last applied on a channel whose batch actually failed), re-applied (batch committed but reported failed), or reordered against a peer channel.
- Triggers: SIGKILL mid-batch, retryable-after-commit errors, `--start-clean` on a non-empty target, flag changes between runs, archive/cleanup of segments.
- Areas: `resume-restart`, `cutover-iteration`.

## M7. Snapshot / CDC boundary
Rows changed between the snapshot point and the start of streaming are applied twice, not at all, or out of order with snapshot rows (e.g. an UPDATE for a row whose snapshot copy is imported later).
- Areas: `snapshot`, `cdc-routing`.
- Workload: continuous writes while export data starts; large tables so snapshot import overlaps streaming.

## M8. Cutover / reverse-direction differences
After cutover, fall-back (YB → PG) and fall-forward (YB → replica) use different exporters (YB CDC, weaker before-images), forced `PARTITION_BY_TABLE`, and iteration state. Data written on YB after cutover, sequences, and multi-iteration hand-offs can diverge.
- Areas: `cutover-iteration`, `sequences`, `value-encoding`.

## M9. Sequences
Sequence last values not restored (or restored low) after cutover → later inserts on the new primary collide or reuse ids; changes-only flow never snapshots sequence values.
- Areas: `sequences`, `cutover-iteration`, `snapshot`.

## M10. Accepted-but-unsafe configuration
A flag or schema combination that voyager accepts without a guardrail, where one of M1–M9 then happens deterministically. Guardrails that exist today: DEFERRABLE unique/PK refused at export; tables without PK refused; replica identity checks; mismatched leaf PKs refused at export; expression UK / UK on STORED generated column force `table` routing and reject `pk`/custom.
- Areas: `guardrails` + whatever the combination touches.
- Workload: combine flags pairwise with schema shapes; any mismatch is a bug, and the fix is usually a new guardrail.
