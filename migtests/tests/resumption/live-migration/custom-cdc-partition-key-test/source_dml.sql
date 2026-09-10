-- Deterministic unique-conflict DML for the FORWARD (source -> target) leg.
-- Re-applied on a loop throughout streaming by the orchestrator's conflict
-- generator, in parallel with the random event generator, so conflicts are
-- produced continuously rather than once.
--
-- DYNAMIC per cycle: the generator passes `-v cycle=N`, and every id and
-- unique-key value is derived from it -- ids = :base + offset where
-- :base = 900000000 + cycle*100000, and unique-key values are suffixed with
-- :cycle -- so each cycle exercises the conflicts on a FRESH set of rows instead
-- of recycling the same ones. Run standalone (no -v cycle) it defaults to
-- cycle 0.
--
-- EVERY column is given an explicit value, and the two rows of a free/reuse
-- pair differ in every column except the unique key they deliberately collide
-- on. pick_random_custom_key samples the custom key from a table's non-key
-- columns, so a column left at its DEFAULT (or NULL) would carry the SAME
-- value on both rows of the pair, route them to the SAME channel, and quietly
-- stop that pair from exercising conflict detection at all. Only the key
-- columns carry :cycle; the rest are plain literals keyed off the row index,
-- since all they have to do is differ within the pair.
--
-- Derived from fallback-unique-conflict-test/source_dml.sql, but reshaped for
-- pick_random_custom_key (orchestrator.py): that action randomly selects ONE
-- (table, columns) candidate as this run's `--cdc-partition-key-overrides`,
-- and the importer requires a custom key column to be immutable (never appear
-- in an UPDATE's SET list). Since the pick happens at run time, EVERY candidate
-- table's DML must already satisfy that regardless of which one gets picked --
-- so tables 2-7 and 10/11/13 below keep ONLY DELETE-based free/reuse (free a
-- unique value via DELETE, reuse it via INSERT on a different PK) and drop
-- any UPDATE that would touch the column(s) that might be this run's custom
-- key. Tables 9 and 12 need no changes: their existing DML already never
-- updates check_id/region.
--
-- Table 1 is the deliberate exception among the candidates: instead of a
-- same-value free/reuse (a no-false-negative case, like every other
-- candidate), it reuses the SAME PK with a DIFFERENT custom-key value. When
-- it is picked, that exercises the synthetic-PK-as-unique-index guard added
-- for custom-key tables (PR #3746) and conflicts MUST still be detected
-- (expect_conflicts: true in the scenario); when not picked, the same PK
-- always routes to the same channel under pk routing, so the pattern is
-- simply inert. Table 8 (expression-based unique index -- the importer
-- rejects pk/custom routing on these outright, a hard guardrail, not a DML
-- limitation) is never a candidate and keeps its original, fuller DML,
-- always PK-routed.

\if :{?cycle}
\else
  \set cycle 0
\endif
\set base (900000000 + :cycle * 100000)

-- ============================================================
-- 1. single_unique_constraint (id PK, email UNIQUE) -- custom-key candidate: (email).
-- PK-RECYCLE case, not a no-false-negative case like every other candidate:
-- when picked, it exercises the synthetic-PK-as-unique-index guard added for
-- custom-key tables (PR #3746, live_migration.go
-- addPrimaryKeyToConflictSetForCustomTables). DELETE frees id=:base+1
-- (email=A); a NEW row then reuses the SAME id with a DIFFERENT email (B).
-- Because A != B, these two events route to DIFFERENT channels under
-- custom-key routing even though they share a PK -- without the synthetic
-- PK-index guard they could apply out of order on the target and violate the
-- primary key. So when this table is picked, conflicts MUST still be
-- detected (expect_conflicts: true in the scenario); when not picked, the
-- same PK always routes to the same channel under pk routing and the
-- pattern is simply inert.
-- ============================================================
BEGIN;

INSERT INTO single_unique_constraint
    (id, email, status, description, amount, due_date, seq_no, is_active, metadata, priority, created_at, updated_at, deleted_at)
VALUES
    ((:base + 1), ('suc_pkrecycle_a@conflict.test' || :cycle), 'st_suc_a', 'desc_suc_a', 0.250000, DATE '2026-01-01', (:base + 0), true, '{"row": "suc_a"}'::jsonb, 0, TIMESTAMP '2026-01-01 00:00:00', TIMESTAMP '2026-03-01 00:00:00', TIMESTAMP '2026-06-01 00:00:00');

DELETE FROM single_unique_constraint WHERE id = :base + 1;

INSERT INTO single_unique_constraint
    (id, email, status, description, amount, due_date, seq_no, is_active, metadata, priority, created_at, updated_at, deleted_at)
VALUES
    ((:base + 1), ('suc_pkrecycle_b@conflict.test' || :cycle), 'st_suc_b', 'desc_suc_b', 1.250000, DATE '2026-01-02', (:base + 1), false, '{"row": "suc_b"}'::jsonb, 1, TIMESTAMP '2026-01-01 00:00:01', TIMESTAMP '2026-03-01 00:00:01', TIMESTAMP '2026-06-01 00:00:01');

COMMIT;

-- ============================================================
-- 2. multi_unique_constraint (id PK, UNIQUE(first_name, last_name)) -- custom-key candidate: (first_name, last_name)
-- ============================================================
BEGIN;

INSERT INTO multi_unique_constraint
    (id, first_name, last_name, status, metadata, amount, due_date, seq_no, is_active, priority, created_at, updated_at, deleted_at)
VALUES
    ((:base + 10001), ('SrcJohn' || :cycle), ('Doe' || :cycle), 'st_muc_a', '{"row": "muc_a"}'::jsonb, 0.250000, DATE '2026-01-01', (:base + 0), true, 0, TIMESTAMP '2026-01-01 00:00:00', TIMESTAMP '2026-03-01 00:00:00', TIMESTAMP '2026-06-01 00:00:00');

-- DELETE-INSERT
DELETE FROM multi_unique_constraint WHERE id = :base + 10001;

INSERT INTO multi_unique_constraint
    (id, first_name, last_name, status, metadata, amount, due_date, seq_no, is_active, priority, created_at, updated_at, deleted_at)
VALUES
    ((:base + 10101), ('SrcJohn' || :cycle), ('Doe' || :cycle), 'st_muc_b', '{"row": "muc_b"}'::jsonb, 1.250000, DATE '2026-01-02', (:base + 1), false, 1, TIMESTAMP '2026-01-01 00:00:01', TIMESTAMP '2026-03-01 00:00:01', TIMESTAMP '2026-06-01 00:00:01');

COMMIT;

-- ============================================================
-- 3. same_column_unique_constraint_and_index (id PK, email UNIQUE + UNIQUE INDEX on email) -- custom-key candidate: (email)
-- ============================================================
BEGIN;

INSERT INTO same_column_unique_constraint_and_index
    (id, email, status, description, amount, due_date, seq_no, is_active, tags, retry_count, metadata, created_at, updated_at, deleted_at)
VALUES
    ((:base + 20001), ('scuci_user1@conflict.test' || :cycle), 'st_scuci_a', 'desc_scuci_a', 0.250000, DATE '2026-01-01', (:base + 0), true, '{tag_scuci_a}'::varchar[], 0, '{"row": "scuci_a"}'::jsonb, TIMESTAMP '2026-01-01 00:00:00', TIMESTAMP '2026-03-01 00:00:00', TIMESTAMP '2026-06-01 00:00:00');

-- DELETE-INSERT
DELETE FROM same_column_unique_constraint_and_index WHERE id = :base + 20001;

INSERT INTO same_column_unique_constraint_and_index
    (id, email, status, description, amount, due_date, seq_no, is_active, tags, retry_count, metadata, created_at, updated_at, deleted_at)
VALUES
    ((:base + 20101), ('scuci_user1@conflict.test' || :cycle), 'st_scuci_b', 'desc_scuci_b', 1.250000, DATE '2026-01-02', (:base + 1), false, '{tag_scuci_b}'::varchar[], 1, '{"row": "scuci_b"}'::jsonb, TIMESTAMP '2026-01-01 00:00:01', TIMESTAMP '2026-03-01 00:00:01', TIMESTAMP '2026-06-01 00:00:01');

COMMIT;

-- ============================================================
-- 4. single_unique_index (id PK, UNIQUE INDEX on "Ssn" -- case-sensitive column) -- custom-key candidate: ("Ssn")
-- ============================================================
BEGIN;

INSERT INTO single_unique_index
    (id, "Ssn", status, description, amount, due_date, seq_no, is_active, score, external_ref, created_at, updated_at, deleted_at)
VALUES
    ((:base + 30001), ('SRC-SSN-1' || :cycle), 'st_sui_a', 'desc_sui_a', 0.250000, DATE '2026-01-01', (:base + 0), true, 0.5, md5('sui_a')::uuid, TIMESTAMP '2026-01-01 00:00:00', TIMESTAMP '2026-03-01 00:00:00', TIMESTAMP '2026-06-01 00:00:00');

-- DELETE-INSERT
DELETE FROM single_unique_index WHERE id = :base + 30001;

INSERT INTO single_unique_index
    (id, "Ssn", status, description, amount, due_date, seq_no, is_active, score, external_ref, created_at, updated_at, deleted_at)
VALUES
    ((:base + 30101), ('SRC-SSN-1' || :cycle), 'st_sui_b', 'desc_sui_b', 1.250000, DATE '2026-01-02', (:base + 1), false, 1.5, md5('sui_b')::uuid, TIMESTAMP '2026-01-01 00:00:01', TIMESTAMP '2026-03-01 00:00:01', TIMESTAMP '2026-06-01 00:00:01');

COMMIT;

-- ============================================================
-- 5. multi_unique_index (id PK, UNIQUE INDEX(first_name, last_name)) -- custom-key candidate: (first_name, last_name)
-- ============================================================
BEGIN;

INSERT INTO multi_unique_index
    (id, first_name, last_name, status, description, amount, due_date, seq_no, is_active, external_ref, created_at, updated_at, deleted_at)
VALUES
    ((:base + 40001), ('IdxJohn' || :cycle), ('Doe' || :cycle), 'st_mui_a', 'desc_mui_a', 0.250000, DATE '2026-01-01', (:base + 0), true, md5('mui_a')::uuid, TIMESTAMP '2026-01-01 00:00:00', TIMESTAMP '2026-03-01 00:00:00', TIMESTAMP '2026-06-01 00:00:00');

-- DELETE-INSERT
DELETE FROM multi_unique_index WHERE id = :base + 40001;

INSERT INTO multi_unique_index
    (id, first_name, last_name, status, description, amount, due_date, seq_no, is_active, external_ref, created_at, updated_at, deleted_at)
VALUES
    ((:base + 40101), ('IdxJohn' || :cycle), ('Doe' || :cycle), 'st_mui_b', 'desc_mui_b', 1.250000, DATE '2026-01-02', (:base + 1), false, md5('mui_b')::uuid, TIMESTAMP '2026-01-01 00:00:01', TIMESTAMP '2026-03-01 00:00:01', TIMESTAMP '2026-06-01 00:00:01');

COMMIT;

-- ============================================================
-- 6. different_columns_unique_constraint_and_index
--    (id PK, email UNIQUE, UNIQUE INDEX on phone_number) -- two independent unique keys
--    custom-key candidates: (email) or (phone_number)
-- ============================================================
BEGIN;

INSERT INTO different_columns_unique_constraint_and_index
    (id, email, phone_number, status, metadata, amount, due_date, seq_no, is_active, retry_count, created_at, updated_at, deleted_at)
VALUES
    ((:base + 50001), ('dcuci_user1@conflict.test' || :cycle), ('dcph-1' || :cycle), 'st_dcuci_a', '{"row": "dcuci_a"}'::jsonb, 0.250000, DATE '2026-01-01', (:base + 0), true, 0, TIMESTAMP '2026-01-01 00:00:00', TIMESTAMP '2026-03-01 00:00:00', TIMESTAMP '2026-06-01 00:00:00');

-- DELETE-INSERT (conflict on both email and phone_number)
DELETE FROM different_columns_unique_constraint_and_index WHERE id = :base + 50001;

INSERT INTO different_columns_unique_constraint_and_index
    (id, email, phone_number, status, metadata, amount, due_date, seq_no, is_active, retry_count, created_at, updated_at, deleted_at)
VALUES
    ((:base + 50101), ('dcuci_user1@conflict.test' || :cycle), ('dcph-1' || :cycle), 'st_dcuci_b', '{"row": "dcuci_b"}'::jsonb, 1.250000, DATE '2026-01-02', (:base + 1), false, 1, TIMESTAMP '2026-01-01 00:00:01', TIMESTAMP '2026-03-01 00:00:01', TIMESTAMP '2026-06-01 00:00:01');

COMMIT;

-- ============================================================
-- 7. subset_columns_unique_constraint_and_index
--    (id PK, UNIQUE(first_name,last_name), UNIQUE INDEX(first_name,last_name,phone_number))
--    custom-key candidate: (first_name, last_name)
-- ============================================================
BEGIN;

INSERT INTO subset_columns_unique_constraint_and_index
    (id, first_name, last_name, phone_number, status, description, amount, due_date, seq_no, is_active, metadata, created_at, updated_at, deleted_at)
VALUES
    ((:base + 60001), ('SubJohn' || :cycle), ('Doe' || :cycle), ('subph-1' || :cycle), 'st_scui_a', 'desc_scui_a', 0.250000, DATE '2026-01-01', (:base + 0), true, '{"row": "scui_a"}'::jsonb, TIMESTAMP '2026-01-01 00:00:00', TIMESTAMP '2026-03-01 00:00:00', TIMESTAMP '2026-06-01 00:00:00');

-- DELETE-INSERT
DELETE FROM subset_columns_unique_constraint_and_index WHERE id = :base + 60001;

INSERT INTO subset_columns_unique_constraint_and_index
    (id, first_name, last_name, phone_number, status, description, amount, due_date, seq_no, is_active, metadata, created_at, updated_at, deleted_at)
VALUES
    ((:base + 60101), ('SubJohn' || :cycle), ('Doe' || :cycle), ('subph-101' || :cycle), 'st_scui_b', 'desc_scui_b', 1.250000, DATE '2026-01-02', (:base + 1), false, '{"row": "scui_b"}'::jsonb, TIMESTAMP '2026-01-01 00:00:01', TIMESTAMP '2026-03-01 00:00:01', TIMESTAMP '2026-06-01 00:00:01');

COMMIT;

-- ============================================================
-- 8. expression_based_unique_index (id PK, UNIQUE INDEX on LOWER(email))
--    NOT a custom-key candidate: the importer rejects pk/custom routing for
--    tables with an expression-based unique index. Kept PK-routed with its
--    full original conflict coverage.
--    Conflicts are produced via different letter-casing that collapses to the
--    same LOWER(email) value.
-- ============================================================
BEGIN;

INSERT INTO expression_based_unique_index
    (id, email, status, description, amount, due_date, seq_no, is_active, notes, metadata, priority, created_at, updated_at, deleted_at)
VALUES
    ((:base + 70001), ('Expr_User1@conflict.test' || :cycle), 'st_ebui_1', 'desc_ebui_1', 0.250000, DATE '2026-01-01', (:base + 0), true, 'notes_ebui_1', '{"row": "ebui_1"}'::jsonb, 0, TIMESTAMP '2026-01-01 00:00:00', TIMESTAMP '2026-03-01 00:00:00', TIMESTAMP '2026-06-01 00:00:00'),
    ((:base + 70002), ('Expr_User2@conflict.test' || :cycle), 'st_ebui_2', 'desc_ebui_2', 1.250000, DATE '2026-01-02', (:base + 1), false, 'notes_ebui_2', '{"row": "ebui_2"}'::jsonb, 1, TIMESTAMP '2026-01-01 00:00:01', TIMESTAMP '2026-03-01 00:00:01', TIMESTAMP '2026-06-01 00:00:01'),
    ((:base + 70003), ('Expr_User3@conflict.test' || :cycle), 'st_ebui_3', 'desc_ebui_3', 2.250000, DATE '2026-01-03', (:base + 2), true, 'notes_ebui_3', '{"row": "ebui_3"}'::jsonb, 2, TIMESTAMP '2026-01-01 00:00:02', TIMESTAMP '2026-03-01 00:00:02', TIMESTAMP '2026-06-01 00:00:02'),
    ((:base + 70004), ('Expr_User4@conflict.test' || :cycle), 'st_ebui_4', 'desc_ebui_4', 3.250000, DATE '2026-01-04', (:base + 3), false, 'notes_ebui_4', '{"row": "ebui_4"}'::jsonb, 3, TIMESTAMP '2026-01-01 00:00:03', TIMESTAMP '2026-03-01 00:00:03', TIMESTAMP '2026-06-01 00:00:03'),
    ((:base + 70005), ('Expr_User5@conflict.test' || :cycle), 'st_ebui_5', 'desc_ebui_5', 4.250000, DATE '2026-01-05', (:base + 4), true, 'notes_ebui_5', '{"row": "ebui_5"}'::jsonb, 4, TIMESTAMP '2026-01-01 00:00:04', TIMESTAMP '2026-03-01 00:00:04', TIMESTAMP '2026-06-01 00:00:04'),
    ((:base + 70006), ('Expr_User6@conflict.test' || :cycle), 'st_ebui_6', 'desc_ebui_6', 5.250000, DATE '2026-01-06', (:base + 5), false, 'notes_ebui_6', '{"row": "ebui_6"}'::jsonb, 5, TIMESTAMP '2026-01-01 00:00:05', TIMESTAMP '2026-03-01 00:00:05', TIMESTAMP '2026-06-01 00:00:05');

-- DELETE-INSERT (LOWER(email) collision)
DELETE FROM expression_based_unique_index WHERE id = :base + 70001;

INSERT INTO expression_based_unique_index
    (id, email, status, description, amount, due_date, seq_no, is_active, notes, metadata, priority, created_at, updated_at, deleted_at)
VALUES
    ((:base + 70101), ('EXPR_USER1@conflict.test' || :cycle), 'st_ebui_101', 'desc_ebui_101', 7.250000, DATE '2026-01-08', (:base + 7), false, 'notes_ebui_101', '{"row": "ebui_101"}'::jsonb, 7, TIMESTAMP '2026-01-01 00:00:07', TIMESTAMP '2026-03-01 00:00:07', TIMESTAMP '2026-06-01 00:00:07');

-- DELETE-UPDATE
DELETE FROM expression_based_unique_index WHERE id = :base + 70002;
UPDATE expression_based_unique_index SET email = ('EXPR_USER2@conflict.test' || :cycle) WHERE id = :base + 70003;

-- UPDATE-INSERT
UPDATE expression_based_unique_index SET email = ('Expr_User4_moved@conflict.test' || :cycle) WHERE id = :base + 70004;

INSERT INTO expression_based_unique_index
    (id, email, status, description, amount, due_date, seq_no, is_active, notes, metadata, priority, created_at, updated_at, deleted_at)
VALUES
    ((:base + 70102), ('expr_user4@conflict.test' || :cycle), 'st_ebui_102', 'desc_ebui_102', 8.250000, DATE '2026-01-09', (:base + 8), true, 'notes_ebui_102', '{"row": "ebui_102"}'::jsonb, 8, TIMESTAMP '2026-01-01 00:00:08', TIMESTAMP '2026-03-01 00:00:08', TIMESTAMP '2026-06-01 00:00:08');

-- UPDATE-UPDATE
UPDATE expression_based_unique_index SET email = ('Expr_User5_moved@conflict.test' || :cycle) WHERE id = :base + 70005;
UPDATE expression_based_unique_index SET email = ('EXPR_user5@conflict.test' || :cycle) WHERE id = :base + 70006;
COMMIT;

-- ============================================================
-- 9. test_partial_unique_index (id PK, UNIQUE INDEX(check_id) WHERE most_recent)
--    custom-key candidate: (check_id) -- already immutable as written: only
--    `most_recent` is ever updated, check_id itself is only ever inserted.
--    Only rows with most_recent = true participate in the unique index, so the
--    conflicts here exercise the partial-predicate before/after logic.
--    check_id values are > 2e8 so they cannot collide with generator rows.
-- ============================================================
BEGIN;

INSERT INTO test_partial_unique_index
    (id, check_id, most_recent, status, description, amount, due_date, seq_no, is_active, metadata, created_at, updated_at, deleted_at)
VALUES
    ((:base + 80001), (:base + 91), true, 'st_tpui_1', 'desc_tpui_1', 0.250000, DATE '2026-01-01', (:base + 0), true, '{"row": "tpui_1"}'::jsonb, TIMESTAMP '2026-01-01 00:00:00', TIMESTAMP '2026-03-01 00:00:00', TIMESTAMP '2026-06-01 00:00:00'),
    ((:base + 80002), (:base + 92), true, 'st_tpui_2', 'desc_tpui_2', 2.250000, DATE '2026-01-03', (:base + 2), true, '{"row": "tpui_2"}'::jsonb, TIMESTAMP '2026-01-01 00:00:02', TIMESTAMP '2026-03-01 00:00:02', TIMESTAMP '2026-06-01 00:00:02'),
    ((:base + 80003), (:base + 93), true, 'st_tpui_3', 'desc_tpui_3', 4.250000, DATE '2026-01-05', (:base + 4), true, '{"row": "tpui_3"}'::jsonb, TIMESTAMP '2026-01-01 00:00:04', TIMESTAMP '2026-03-01 00:00:04', TIMESTAMP '2026-06-01 00:00:04'),
    ((:base + 80004), (:base + 93), false, 'st_tpui_4', 'desc_tpui_4', 5.250000, DATE '2026-01-06', (:base + 5), false, '{"row": "tpui_4"}'::jsonb, TIMESTAMP '2026-01-01 00:00:05', TIMESTAMP '2026-03-01 00:00:05', TIMESTAMP '2026-06-01 00:00:05'),
    ((:base + 80005), (:base + 94), true, 'st_tpui_5', 'desc_tpui_5', 6.250000, DATE '2026-01-07', (:base + 6), true, '{"row": "tpui_5"}'::jsonb, TIMESTAMP '2026-01-01 00:00:06', TIMESTAMP '2026-03-01 00:00:06', TIMESTAMP '2026-06-01 00:00:06'),
    ((:base + 80006), (:base + 94), false, 'st_tpui_6', 'desc_tpui_6', 7.250000, DATE '2026-01-08', (:base + 7), false, '{"row": "tpui_6"}'::jsonb, TIMESTAMP '2026-01-01 00:00:07', TIMESTAMP '2026-03-01 00:00:07', TIMESTAMP '2026-06-01 00:00:07');

-- UPDATE-INSERT: deactivate active holder (frees the partial-index key), insert a new active row with same check_id
UPDATE test_partial_unique_index SET most_recent = false WHERE id = :base + 80001;

INSERT INTO test_partial_unique_index
    (id, check_id, most_recent, status, description, amount, due_date, seq_no, is_active, metadata, created_at, updated_at, deleted_at)
VALUES
    ((:base + 80101), (:base + 91), true, 'st_tpui_101', 'desc_tpui_101', 1.250000, DATE '2026-01-02', (:base + 1), false, '{"row": "tpui_101"}'::jsonb, TIMESTAMP '2026-01-01 00:00:01', TIMESTAMP '2026-03-01 00:00:01', TIMESTAMP '2026-06-01 00:00:01');

-- DELETE-INSERT: delete active holder, insert a new active row with same check_id
DELETE FROM test_partial_unique_index WHERE id = :base + 80002;

INSERT INTO test_partial_unique_index
    (id, check_id, most_recent, status, description, amount, due_date, seq_no, is_active, metadata, created_at, updated_at, deleted_at)
VALUES
    ((:base + 80102), (:base + 92), true, 'st_tpui_102', 'desc_tpui_102', 3.250000, DATE '2026-01-04', (:base + 3), false, '{"row": "tpui_102"}'::jsonb, TIMESTAMP '2026-01-01 00:00:03', TIMESTAMP '2026-03-01 00:00:03', TIMESTAMP '2026-06-01 00:00:03');

-- DELETE-UPDATE: delete active holder, flip the inactive partner to active (same check_id)
DELETE FROM test_partial_unique_index WHERE id = :base + 80003;
UPDATE test_partial_unique_index SET most_recent = true WHERE id = :base + 80004;

-- UPDATE-UPDATE: deactivate active holder, activate the inactive partner (same check_id)
UPDATE test_partial_unique_index SET most_recent = false WHERE id = :base + 80005;
UPDATE test_partial_unique_index SET most_recent = true WHERE id = :base + 80006;
COMMIT;

-- ============================================================
-- 10. single_unique_index_nulls_not_distinct (id PK, UNIQUE INDEX(email) NULLS NOT DISTINCT)
--     custom-key candidate: (email) -- free/reuse rewritten to DELETE-INSERT
--     (never UPDATE) so email is immutability-safe if picked as the custom key.
-- Under NULLS NOT DISTINCT two NULLs are equal, so a NULL free->reuse across PKs is a
-- real conflict. Only one NULL can exist at a time, so the NULL is cleared via DELETE
-- at the end of the block, leaving none behind for the next cycle.
-- ============================================================
BEGIN;
-- DELETE-INSERT (non-null value)

INSERT INTO single_unique_index_nulls_not_distinct
    (id, email, status, description, amount, due_date, seq_no, is_active, priority, score, external_ref, created_at, updated_at, deleted_at)
VALUES
    ((:base + 85001), ('nnd1_user1@conflict.test' || :cycle), 'st_nnd1_a', 'desc_nnd1_a', 0.250000, DATE '2026-01-01', (:base + 0), true, 0, 0.5, md5('nnd1_a')::uuid, TIMESTAMP '2026-01-01 00:00:00', TIMESTAMP '2026-03-01 00:00:00', TIMESTAMP '2026-06-01 00:00:00');

DELETE FROM single_unique_index_nulls_not_distinct WHERE id = :base + 85001;

INSERT INTO single_unique_index_nulls_not_distinct
    (id, email, status, description, amount, due_date, seq_no, is_active, priority, score, external_ref, created_at, updated_at, deleted_at)
VALUES
    ((:base + 85101), ('nnd1_user1@conflict.test' || :cycle), 'st_nnd1_b', 'desc_nnd1_b', 1.250000, DATE '2026-01-02', (:base + 1), false, 1, 1.5, md5('nnd1_b')::uuid, TIMESTAMP '2026-01-01 00:00:01', TIMESTAMP '2026-03-01 00:00:01', TIMESTAMP '2026-06-01 00:00:01');

-- NULL free->reuse: free the NULL by deleting base+85010, reuse NULL on base+85011
-- (conflict under NULLS NOT DISTINCT), then delete base+85011 to leave none behind.

INSERT INTO single_unique_index_nulls_not_distinct
    (id, email, status, description, amount, due_date, seq_no, is_active, priority, score, external_ref, created_at, updated_at, deleted_at)
VALUES
    ((:base + 85010), NULL, 'st_nnd1_n1', 'desc_nnd1_n1', 2.250000, DATE '2026-01-03', (:base + 2), true, 2, 2.5, md5('nnd1_n1')::uuid, TIMESTAMP '2026-01-01 00:00:02', TIMESTAMP '2026-03-01 00:00:02', TIMESTAMP '2026-06-01 00:00:02');

DELETE FROM single_unique_index_nulls_not_distinct WHERE id = :base + 85010;

INSERT INTO single_unique_index_nulls_not_distinct
    (id, email, status, description, amount, due_date, seq_no, is_active, priority, score, external_ref, created_at, updated_at, deleted_at)
VALUES
    ((:base + 85011), NULL, 'st_nnd1_n2', 'desc_nnd1_n2', 3.250000, DATE '2026-01-04', (:base + 3), false, 3, 3.5, md5('nnd1_n2')::uuid, TIMESTAMP '2026-01-01 00:00:03', TIMESTAMP '2026-03-01 00:00:03', TIMESTAMP '2026-06-01 00:00:03');

DELETE FROM single_unique_index_nulls_not_distinct WHERE id = :base + 85011;
COMMIT;

-- ============================================================
-- 11. multi_unique_index_nulls_not_distinct (id PK, UNIQUE INDEX(first_name, last_name) NULLS NOT DISTINCT)
--     custom-key candidate: (first_name, last_name) -- free/reuse rewritten to
--     DELETE-INSERT (never UPDATE), same reasoning as table 10.
-- ============================================================
BEGIN;
-- DELETE-INSERT (non-null values)

INSERT INTO multi_unique_index_nulls_not_distinct
    (id, first_name, last_name, status, description, amount, due_date, seq_no, is_active, score, external_ref, created_at, updated_at, deleted_at)
VALUES
    ((:base + 86001), ('nnd2First' || :cycle), ('nnd2Last' || :cycle), 'st_nnd2_a', 'desc_nnd2_a', 0.250000, DATE '2026-01-01', (:base + 0), true, 0.5, md5('nnd2_a')::uuid, TIMESTAMP '2026-01-01 00:00:00', TIMESTAMP '2026-03-01 00:00:00', TIMESTAMP '2026-06-01 00:00:00');

DELETE FROM multi_unique_index_nulls_not_distinct WHERE id = :base + 86001;

INSERT INTO multi_unique_index_nulls_not_distinct
    (id, first_name, last_name, status, description, amount, due_date, seq_no, is_active, score, external_ref, created_at, updated_at, deleted_at)
VALUES
    ((:base + 86101), ('nnd2First' || :cycle), ('nnd2Last' || :cycle), 'st_nnd2_b', 'desc_nnd2_b', 1.250000, DATE '2026-01-02', (:base + 1), false, 1.5, md5('nnd2_b')::uuid, TIMESTAMP '2026-01-01 00:00:01', TIMESTAMP '2026-03-01 00:00:01', TIMESTAMP '2026-06-01 00:00:01');

-- (NULL, NULL) free->reuse: two all-NULL rows conflict under NULLS NOT DISTINCT.
-- Cleared via DELETE at the end so none is left behind for the next cycle.

INSERT INTO multi_unique_index_nulls_not_distinct
    (id, first_name, last_name, status, description, amount, due_date, seq_no, is_active, score, external_ref, created_at, updated_at, deleted_at)
VALUES
    ((:base + 86010), NULL, NULL, 'st_nnd2_n1', 'desc_nnd2_n1', 2.250000, DATE '2026-01-03', (:base + 2), true, 2.5, md5('nnd2_n1')::uuid, TIMESTAMP '2026-01-01 00:00:02', TIMESTAMP '2026-03-01 00:00:02', TIMESTAMP '2026-06-01 00:00:02');

DELETE FROM multi_unique_index_nulls_not_distinct WHERE id = :base + 86010;

INSERT INTO multi_unique_index_nulls_not_distinct
    (id, first_name, last_name, status, description, amount, due_date, seq_no, is_active, score, external_ref, created_at, updated_at, deleted_at)
VALUES
    ((:base + 86011), NULL, NULL, 'st_nnd2_n2', 'desc_nnd2_n2', 3.250000, DATE '2026-01-04', (:base + 3), false, 3.5, md5('nnd2_n2')::uuid, TIMESTAMP '2026-01-01 00:00:03', TIMESTAMP '2026-03-01 00:00:03', TIMESTAMP '2026-06-01 00:00:03');

DELETE FROM multi_unique_index_nulls_not_distinct WHERE id = :base + 86011;
COMMIT;

-- ============================================================
-- 12. partitioned_unique_conflict (PK(id, region), UNIQUE INDEX(email, region), PARTITION BY LIST(region))
--     custom-key candidate: (region) -- already immutable as written: only
--     `email` is ever updated, region is fixed per row.
-- The unique key includes the partition-key column; conflicts are exercised within a partition.
-- ============================================================
BEGIN;

INSERT INTO partitioned_unique_conflict
    (id, region, email, status, description, amount, due_date, seq_no, is_active, tags, retry_count, created_at, updated_at, deleted_at)
VALUES
    ((:base + 87001), 'east', ('puc_user1@conflict.test' || :cycle), 'st_puc_1', 'desc_puc_1', 0.250000, DATE '2026-01-01', (:base + 0), true, '{tag_puc_1}'::varchar[], 0, TIMESTAMP '2026-01-01 00:00:00', TIMESTAMP '2026-03-01 00:00:00', TIMESTAMP '2026-06-01 00:00:00'),
    ((:base + 87002), 'east', ('puc_user2@conflict.test' || :cycle), 'st_puc_2', 'desc_puc_2', 2.250000, DATE '2026-01-03', (:base + 2), true, '{tag_puc_2}'::varchar[], 2, TIMESTAMP '2026-01-01 00:00:02', TIMESTAMP '2026-03-01 00:00:02', TIMESTAMP '2026-06-01 00:00:02'),
    ((:base + 87003), 'west', ('puc_user3@conflict.test' || :cycle), 'st_puc_3', 'desc_puc_3', 4.250000, DATE '2026-01-05', (:base + 4), true, '{tag_puc_3}'::varchar[], 4, TIMESTAMP '2026-01-01 00:00:04', TIMESTAMP '2026-03-01 00:00:04', TIMESTAMP '2026-06-01 00:00:04'),
    ((:base + 87004), 'west', ('puc_user4@conflict.test' || :cycle), 'st_puc_4', 'desc_puc_4', 6.250000, DATE '2026-01-07', (:base + 6), true, '{tag_puc_4}'::varchar[], 6, TIMESTAMP '2026-01-01 00:00:06', TIMESTAMP '2026-03-01 00:00:06', TIMESTAMP '2026-06-01 00:00:06');

-- DELETE-INSERT within the 'east' partition: free (puc_user1, east), reuse on a new PK
DELETE FROM partitioned_unique_conflict WHERE id = :base + 87001 AND region = 'east';

INSERT INTO partitioned_unique_conflict
    (id, region, email, status, description, amount, due_date, seq_no, is_active, tags, retry_count, created_at, updated_at, deleted_at)
VALUES
    ((:base + 87101), 'east', ('puc_user1@conflict.test' || :cycle), 'st_puc_101', 'desc_puc_101', 1.250000, DATE '2026-01-02', (:base + 1), false, '{tag_puc_101}'::varchar[], 1, TIMESTAMP '2026-01-01 00:00:01', TIMESTAMP '2026-03-01 00:00:01', TIMESTAMP '2026-06-01 00:00:01');

-- UPDATE-INSERT within the 'west' partition: free (puc_user3, west) by moving it, reuse on a new PK
UPDATE partitioned_unique_conflict SET email = ('puc_user3_moved@conflict.test' || :cycle) WHERE id = :base + 87003 AND region = 'west';

INSERT INTO partitioned_unique_conflict
    (id, region, email, status, description, amount, due_date, seq_no, is_active, tags, retry_count, created_at, updated_at, deleted_at)
VALUES
    ((:base + 87103), 'west', ('puc_user3@conflict.test' || :cycle), 'st_puc_103', 'desc_puc_103', 5.250000, DATE '2026-01-06', (:base + 5), false, '{tag_puc_103}'::varchar[], 5, TIMESTAMP '2026-01-01 00:00:05', TIMESTAMP '2026-03-01 00:00:05', TIMESTAMP '2026-06-01 00:00:05');

COMMIT;

-- ============================================================
-- 13. single_unique_index_nulls_distinct (id PK, UNIQUE INDEX(email) -- default NULLS DISTINCT)
--     custom-key candidate: (email) -- the non-null free->reuse step is rewritten
--     to DELETE-INSERT (never UPDATE) so email is immutability-safe if picked.
-- Under NULLS DISTINCT multiple NULLs coexist and a NULL free->reuse is NOT a conflict,
-- so these NULL rows must import without being (wrongly) serialized. A non-null value
-- conflict is included to confirm real conflicts still fire on this table.
-- ============================================================
BEGIN;
-- Multiple NULL rows coexist under NULLS DISTINCT (no conflict, no violation)

INSERT INTO single_unique_index_nulls_distinct
    (id, email, status, description, amount, due_date, seq_no, is_active, external_ref, tags, created_at, updated_at, deleted_at)
VALUES
    ((:base + 89001), NULL, 'st_nd_n1', 'desc_nd_n1', 0.250000, DATE '2026-01-01', (:base + 0), true, md5('nd_n1')::uuid, '{tag_nd_n1}'::varchar[], TIMESTAMP '2026-01-01 00:00:00', TIMESTAMP '2026-03-01 00:00:00', TIMESTAMP '2026-06-01 00:00:00'),
    ((:base + 89002), NULL, 'st_nd_n2', 'desc_nd_n2', 2.250000, DATE '2026-01-03', (:base + 2), true, md5('nd_n2')::uuid, '{tag_nd_n2}'::varchar[], TIMESTAMP '2026-01-01 00:00:02', TIMESTAMP '2026-03-01 00:00:02', TIMESTAMP '2026-06-01 00:00:02'),
    ((:base + 89003), NULL, 'st_nd_n3', 'desc_nd_n3', 4.250000, DATE '2026-01-05', (:base + 4), true, md5('nd_n3')::uuid, '{tag_nd_n3}'::varchar[], TIMESTAMP '2026-01-01 00:00:04', TIMESTAMP '2026-03-01 00:00:04', TIMESTAMP '2026-06-01 00:00:04');

-- NULL free->reuse: delete a NULL holder, insert another NULL. Under NULLS DISTINCT
-- these NULLs do NOT conflict, so the cache must not serialize them.
DELETE FROM single_unique_index_nulls_distinct WHERE id = :base + 89001;

INSERT INTO single_unique_index_nulls_distinct
    (id, email, status, description, amount, due_date, seq_no, is_active, external_ref, tags, created_at, updated_at, deleted_at)
VALUES
    ((:base + 89101), NULL, 'st_nd_n101', 'desc_nd_n101', 1.250000, DATE '2026-01-02', (:base + 1), false, md5('nd_n101')::uuid, '{tag_nd_n101}'::varchar[], TIMESTAMP '2026-01-01 00:00:01', TIMESTAMP '2026-03-01 00:00:01', TIMESTAMP '2026-06-01 00:00:01');

-- Non-null value free->reuse (DELETE-INSERT): a real conflict that must still be detected here.

INSERT INTO single_unique_index_nulls_distinct
    (id, email, status, description, amount, due_date, seq_no, is_active, external_ref, tags, created_at, updated_at, deleted_at)
VALUES
    ((:base + 89010), ('nd_user1@conflict.test' || :cycle), 'st_nd_a', 'desc_nd_a', 6.250000, DATE '2026-01-07', (:base + 6), true, md5('nd_a')::uuid, '{tag_nd_a}'::varchar[], TIMESTAMP '2026-01-01 00:00:06', TIMESTAMP '2026-03-01 00:00:06', TIMESTAMP '2026-06-01 00:00:06');

DELETE FROM single_unique_index_nulls_distinct WHERE id = :base + 89010;

INSERT INTO single_unique_index_nulls_distinct
    (id, email, status, description, amount, due_date, seq_no, is_active, external_ref, tags, created_at, updated_at, deleted_at)
VALUES
    ((:base + 89110), ('nd_user1@conflict.test' || :cycle), 'st_nd_b', 'desc_nd_b', 7.250000, DATE '2026-01-08', (:base + 7), false, md5('nd_b')::uuid, '{tag_nd_b}'::varchar[], TIMESTAMP '2026-01-01 00:00:07', TIMESTAMP '2026-03-01 00:00:07', TIMESTAMP '2026-06-01 00:00:07');

COMMIT;
