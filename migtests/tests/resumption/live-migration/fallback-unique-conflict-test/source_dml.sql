-- Deterministic unique-conflict DML for the FORWARD (source -> target) leg.
-- Re-applied on a loop throughout streaming by the orchestrator's conflict
-- generator (conflict_generator_source), in parallel with the random event
-- generator, so conflicts are produced continuously rather than once.
--
-- DYNAMIC per cycle: the generator passes `-v cycle=N`, and every id and
-- unique-key value is derived from it -- ids = :base + offset where
-- :base = 900000000 + cycle*100000, and unique-key values are suffixed with
-- :cycle -- so each cycle exercises the conflicts on a FRESH set of rows instead
-- of recycling the same ones. The freeing and reusing events within a cycle
-- still share that cycle's value, so the conflict is preserved every cycle; only
-- the rows differ across cycles. Run standalone (no -v cycle) it defaults to
-- cycle 0. The 100000 stride comfortably fits each cycle's ids, and the
-- 900,000,000 base keeps them outside the random generator's range (-2e8 .. 2e8).
--
-- Exercises every conflict type the streaming-phase conflict-detection cache
-- handles (see yb-voyager/cmd/conflictDetectionCache.go):
--   1. DELETE-INSERT   2. DELETE-UPDATE   3. UPDATE-INSERT   4. UPDATE-UPDATE
-- A conflict is two events with DIFFERENT primary keys but the SAME unique-key
-- value: a "freeing" event (DELETE/UPDATE the holder) and a "reusing" event
-- (INSERT/UPDATE a different PK to that value). They route to different import
-- channels, so the cache must serialize them; otherwise the reusing event hits a
-- unique violation. Every statement is individually valid on the source; the
-- violation would only occur on the target if events were applied out of order.
-- Each table's seed + conflicts run in ONE transaction.

-- ============================================================
-- 1. single_unique_constraint (id PK, email UNIQUE)
-- ============================================================
\if :{?cycle}
\else
  \set cycle 0
\endif
\set base (900000000 + :cycle * 100000)

BEGIN;
INSERT INTO single_unique_constraint (id, email) VALUES
    (:base + 1, ('suc_user1@conflict.test' || :cycle)),
    (:base + 2, ('suc_user2@conflict.test' || :cycle)),
    (:base + 3, ('suc_user3@conflict.test' || :cycle)),
    (:base + 4, ('suc_user4@conflict.test' || :cycle)),
    (:base + 5, ('suc_user5@conflict.test' || :cycle)),
    (:base + 6, ('suc_user6@conflict.test' || :cycle));

-- DELETE-INSERT: free suc_user1, reuse it on a new PK
DELETE FROM single_unique_constraint WHERE id = :base + 1;
INSERT INTO single_unique_constraint (id, email) VALUES (:base + 101, ('suc_user1@conflict.test' || :cycle));

-- DELETE-UPDATE: free suc_user2, reuse it via UPDATE of a different PK
DELETE FROM single_unique_constraint WHERE id = :base + 2;
UPDATE single_unique_constraint SET email = ('suc_user2@conflict.test' || :cycle) WHERE id = :base + 3;

-- UPDATE-INSERT: free suc_user4 by moving it, reuse it on a new PK
UPDATE single_unique_constraint SET email = ('suc_user4_moved@conflict.test' || :cycle) WHERE id = :base + 4;
INSERT INTO single_unique_constraint (id, email) VALUES (:base + 102, ('suc_user4@conflict.test' || :cycle));

-- UPDATE-UPDATE: free suc_user5 by moving it, reuse it via UPDATE of a different PK
UPDATE single_unique_constraint SET email = ('suc_user5_moved@conflict.test' || :cycle) WHERE id = :base + 5;
UPDATE single_unique_constraint SET email = ('suc_user5@conflict.test' || :cycle) WHERE id = :base + 6;
COMMIT;

-- ============================================================
-- 2. multi_unique_constraint (id PK, UNIQUE(first_name, last_name))
-- ============================================================
BEGIN;
INSERT INTO multi_unique_constraint (id, first_name, last_name) VALUES
    (:base + 10001, ('SrcJohn' || :cycle),  ('Doe' || :cycle)),
    (:base + 10002, ('SrcJane' || :cycle),  ('Smith' || :cycle)),
    (:base + 10003, ('SrcBob' || :cycle),   ('Jones' || :cycle)),
    (:base + 10004, ('SrcAlice' || :cycle), ('Williams' || :cycle)),
    (:base + 10005, ('SrcTom' || :cycle),   ('Clark' || :cycle)),
    (:base + 10006, ('SrcEve' || :cycle),   ('Davis' || :cycle));

-- DELETE-INSERT
DELETE FROM multi_unique_constraint WHERE id = :base + 10001;
INSERT INTO multi_unique_constraint (id, first_name, last_name) VALUES (:base + 10101, ('SrcJohn' || :cycle), ('Doe' || :cycle));

-- DELETE-UPDATE
DELETE FROM multi_unique_constraint WHERE id = :base + 10002;
UPDATE multi_unique_constraint SET first_name = ('SrcJane' || :cycle), last_name = ('Smith' || :cycle) WHERE id = :base + 10003;

-- UPDATE-INSERT
UPDATE multi_unique_constraint SET first_name = ('SrcAlice_moved' || :cycle) WHERE id = :base + 10004;
INSERT INTO multi_unique_constraint (id, first_name, last_name) VALUES (:base + 10102, ('SrcAlice' || :cycle), ('Williams' || :cycle));

-- UPDATE-UPDATE
UPDATE multi_unique_constraint SET first_name = ('SrcTom_moved' || :cycle) WHERE id = :base + 10005;
UPDATE multi_unique_constraint SET first_name = ('SrcTom' || :cycle), last_name = ('Clark' || :cycle) WHERE id = :base + 10006;
COMMIT;

-- ============================================================
-- 3. same_column_unique_constraint_and_index (id PK, email UNIQUE + UNIQUE INDEX on email)
-- ============================================================
BEGIN;
INSERT INTO same_column_unique_constraint_and_index (id, email) VALUES
    (:base + 20001, ('scuci_user1@conflict.test' || :cycle)),
    (:base + 20002, ('scuci_user2@conflict.test' || :cycle)),
    (:base + 20003, ('scuci_user3@conflict.test' || :cycle)),
    (:base + 20004, ('scuci_user4@conflict.test' || :cycle)),
    (:base + 20005, ('scuci_user5@conflict.test' || :cycle)),
    (:base + 20006, ('scuci_user6@conflict.test' || :cycle));

-- DELETE-INSERT
DELETE FROM same_column_unique_constraint_and_index WHERE id = :base + 20001;
INSERT INTO same_column_unique_constraint_and_index (id, email) VALUES (:base + 20101, ('scuci_user1@conflict.test' || :cycle));

-- DELETE-UPDATE
DELETE FROM same_column_unique_constraint_and_index WHERE id = :base + 20002;
UPDATE same_column_unique_constraint_and_index SET email = ('scuci_user2@conflict.test' || :cycle) WHERE id = :base + 20003;

-- UPDATE-INSERT
UPDATE same_column_unique_constraint_and_index SET email = ('scuci_user4_moved@conflict.test' || :cycle) WHERE id = :base + 20004;
INSERT INTO same_column_unique_constraint_and_index (id, email) VALUES (:base + 20102, ('scuci_user4@conflict.test' || :cycle));

-- UPDATE-UPDATE
UPDATE same_column_unique_constraint_and_index SET email = ('scuci_user5_moved@conflict.test' || :cycle) WHERE id = :base + 20005;
UPDATE same_column_unique_constraint_and_index SET email = ('scuci_user5@conflict.test' || :cycle) WHERE id = :base + 20006;
COMMIT;

-- ============================================================
-- 4. single_unique_index (id PK, UNIQUE INDEX on "Ssn" -- case-sensitive column)
-- ============================================================
BEGIN;
INSERT INTO single_unique_index (id, "Ssn") VALUES
    (:base + 30001, ('SRC-SSN-1' || :cycle)),
    (:base + 30002, ('SRC-SSN-2' || :cycle)),
    (:base + 30003, ('SRC-SSN-3' || :cycle)),
    (:base + 30004, ('SRC-SSN-4' || :cycle)),
    (:base + 30005, ('SRC-SSN-5' || :cycle)),
    (:base + 30006, ('SRC-SSN-6' || :cycle));

-- DELETE-INSERT
DELETE FROM single_unique_index WHERE id = :base + 30001;
INSERT INTO single_unique_index (id, "Ssn") VALUES (:base + 30101, ('SRC-SSN-1' || :cycle));

-- DELETE-UPDATE
DELETE FROM single_unique_index WHERE id = :base + 30002;
UPDATE single_unique_index SET "Ssn" = ('SRC-SSN-2' || :cycle) WHERE id = :base + 30003;

-- UPDATE-INSERT
UPDATE single_unique_index SET "Ssn" = ('SRC-SSN-4-moved' || :cycle) WHERE id = :base + 30004;
INSERT INTO single_unique_index (id, "Ssn") VALUES (:base + 30102, ('SRC-SSN-4' || :cycle));

-- UPDATE-UPDATE
UPDATE single_unique_index SET "Ssn" = ('SRC-SSN-5-moved' || :cycle) WHERE id = :base + 30005;
UPDATE single_unique_index SET "Ssn" = ('SRC-SSN-5' || :cycle) WHERE id = :base + 30006;
COMMIT;

-- ============================================================
-- 5. multi_unique_index (id PK, UNIQUE INDEX(first_name, last_name))
-- ============================================================
BEGIN;
INSERT INTO multi_unique_index (id, first_name, last_name) VALUES
    (:base + 40001, ('IdxJohn' || :cycle),  ('Doe' || :cycle)),
    (:base + 40002, ('IdxJane' || :cycle),  ('Smith' || :cycle)),
    (:base + 40003, ('IdxBob' || :cycle),   ('Jones' || :cycle)),
    (:base + 40004, ('IdxAlice' || :cycle), ('Williams' || :cycle)),
    (:base + 40005, ('IdxTom' || :cycle),   ('Clark' || :cycle)),
    (:base + 40006, ('IdxEve' || :cycle),   ('Davis' || :cycle));

-- DELETE-INSERT
DELETE FROM multi_unique_index WHERE id = :base + 40001;
INSERT INTO multi_unique_index (id, first_name, last_name) VALUES (:base + 40101, ('IdxJohn' || :cycle), ('Doe' || :cycle));

-- DELETE-UPDATE
DELETE FROM multi_unique_index WHERE id = :base + 40002;
UPDATE multi_unique_index SET first_name = ('IdxJane' || :cycle), last_name = ('Smith' || :cycle) WHERE id = :base + 40003;

-- UPDATE-INSERT
UPDATE multi_unique_index SET first_name = ('IdxAlice_moved' || :cycle) WHERE id = :base + 40004;
INSERT INTO multi_unique_index (id, first_name, last_name) VALUES (:base + 40102, ('IdxAlice' || :cycle), ('Williams' || :cycle));

-- UPDATE-UPDATE
UPDATE multi_unique_index SET first_name = ('IdxTom_moved' || :cycle) WHERE id = :base + 40005;
UPDATE multi_unique_index SET first_name = ('IdxTom' || :cycle), last_name = ('Clark' || :cycle) WHERE id = :base + 40006;
COMMIT;

-- ============================================================
-- 6. different_columns_unique_constraint_and_index
--    (id PK, email UNIQUE, UNIQUE INDEX on phone_number) -- two independent unique keys
-- ============================================================
BEGIN;
INSERT INTO different_columns_unique_constraint_and_index (id, email, phone_number) VALUES
    (:base + 50001, ('dcuci_user1@conflict.test' || :cycle), ('dcph-1' || :cycle)),
    (:base + 50002, ('dcuci_user2@conflict.test' || :cycle), ('dcph-2' || :cycle)),
    (:base + 50003, ('dcuci_user3@conflict.test' || :cycle), ('dcph-3' || :cycle)),
    (:base + 50004, ('dcuci_user4@conflict.test' || :cycle), ('dcph-4' || :cycle)),
    (:base + 50005, ('dcuci_user5@conflict.test' || :cycle), ('dcph-5' || :cycle)),
    (:base + 50006, ('dcuci_user6@conflict.test' || :cycle), ('dcph-6' || :cycle));

-- DELETE-INSERT (conflict on both email and phone_number)
DELETE FROM different_columns_unique_constraint_and_index WHERE id = :base + 50001;
INSERT INTO different_columns_unique_constraint_and_index (id, email, phone_number) VALUES (:base + 50101, ('dcuci_user1@conflict.test' || :cycle), ('dcph-1' || :cycle));

-- DELETE-UPDATE (conflict on phone_number unique index)
DELETE FROM different_columns_unique_constraint_and_index WHERE id = :base + 50002;
UPDATE different_columns_unique_constraint_and_index SET phone_number = ('dcph-2' || :cycle) WHERE id = :base + 50003;

-- UPDATE-INSERT (conflict on email unique constraint)
UPDATE different_columns_unique_constraint_and_index SET email = ('dcuci_user4_moved@conflict.test' || :cycle) WHERE id = :base + 50004;
INSERT INTO different_columns_unique_constraint_and_index (id, email, phone_number) VALUES (:base + 50102, ('dcuci_user4@conflict.test' || :cycle), ('dcph-104' || :cycle));

-- UPDATE-UPDATE (conflict on phone_number unique index)
UPDATE different_columns_unique_constraint_and_index SET phone_number = ('dcph-5-moved' || :cycle) WHERE id = :base + 50005;
UPDATE different_columns_unique_constraint_and_index SET phone_number = ('dcph-5' || :cycle) WHERE id = :base + 50006;
COMMIT;

-- ============================================================
-- 7. subset_columns_unique_constraint_and_index
--    (id PK, UNIQUE(first_name,last_name), UNIQUE INDEX(first_name,last_name,phone_number))
-- ============================================================
BEGIN;
INSERT INTO subset_columns_unique_constraint_and_index (id, first_name, last_name, phone_number) VALUES
    (:base + 60001, ('SubJohn' || :cycle),  ('Doe' || :cycle),      ('subph-1' || :cycle)),
    (:base + 60002, ('SubJane' || :cycle),  ('Smith' || :cycle),    ('subph-2' || :cycle)),
    (:base + 60003, ('SubBob' || :cycle),   ('Jones' || :cycle),    ('subph-3' || :cycle)),
    (:base + 60004, ('SubAlice' || :cycle), ('Williams' || :cycle), ('subph-4' || :cycle)),
    (:base + 60005, ('SubTom' || :cycle),   ('Clark' || :cycle),    ('subph-5' || :cycle)),
    (:base + 60006, ('SubEve' || :cycle),   ('Davis' || :cycle),    ('subph-6' || :cycle));

-- DELETE-INSERT
DELETE FROM subset_columns_unique_constraint_and_index WHERE id = :base + 60001;
INSERT INTO subset_columns_unique_constraint_and_index (id, first_name, last_name, phone_number) VALUES (:base + 60101, ('SubJohn' || :cycle), ('Doe' || :cycle), ('subph-101' || :cycle));

-- DELETE-UPDATE
DELETE FROM subset_columns_unique_constraint_and_index WHERE id = :base + 60002;
UPDATE subset_columns_unique_constraint_and_index SET first_name = ('SubJane' || :cycle), last_name = ('Smith' || :cycle) WHERE id = :base + 60003;

-- UPDATE-INSERT
UPDATE subset_columns_unique_constraint_and_index SET first_name = ('SubAlice_moved' || :cycle) WHERE id = :base + 60004;
INSERT INTO subset_columns_unique_constraint_and_index (id, first_name, last_name, phone_number) VALUES (:base + 60102, ('SubAlice' || :cycle), ('Williams' || :cycle), ('subph-102' || :cycle));

-- UPDATE-UPDATE
UPDATE subset_columns_unique_constraint_and_index SET first_name = ('SubTom_moved' || :cycle) WHERE id = :base + 60005;
UPDATE subset_columns_unique_constraint_and_index SET first_name = ('SubTom' || :cycle), last_name = ('Clark' || :cycle) WHERE id = :base + 60006;
COMMIT;

-- ============================================================
-- 8. expression_based_unique_index (id PK, UNIQUE INDEX on LOWER(email))
--    Conflicts are produced via different letter-casing that collapses to the
--    same LOWER(email) value.
-- ============================================================
BEGIN;
INSERT INTO expression_based_unique_index (id, email) VALUES
    (:base + 70001, ('Expr_User1@conflict.test' || :cycle)),
    (:base + 70002, ('Expr_User2@conflict.test' || :cycle)),
    (:base + 70003, ('Expr_User3@conflict.test' || :cycle)),
    (:base + 70004, ('Expr_User4@conflict.test' || :cycle)),
    (:base + 70005, ('Expr_User5@conflict.test' || :cycle)),
    (:base + 70006, ('Expr_User6@conflict.test' || :cycle));

-- DELETE-INSERT (LOWER(email) collision)
DELETE FROM expression_based_unique_index WHERE id = :base + 70001;
INSERT INTO expression_based_unique_index (id, email) VALUES (:base + 70101, ('EXPR_USER1@conflict.test' || :cycle));

-- DELETE-UPDATE
DELETE FROM expression_based_unique_index WHERE id = :base + 70002;
UPDATE expression_based_unique_index SET email = ('EXPR_USER2@conflict.test' || :cycle) WHERE id = :base + 70003;

-- UPDATE-INSERT
UPDATE expression_based_unique_index SET email = ('Expr_User4_moved@conflict.test' || :cycle) WHERE id = :base + 70004;
INSERT INTO expression_based_unique_index (id, email) VALUES (:base + 70102, ('expr_user4@conflict.test' || :cycle));

-- UPDATE-UPDATE
UPDATE expression_based_unique_index SET email = ('Expr_User5_moved@conflict.test' || :cycle) WHERE id = :base + 70005;
UPDATE expression_based_unique_index SET email = ('EXPR_user5@conflict.test' || :cycle) WHERE id = :base + 70006;
COMMIT;

-- ============================================================
-- 9. test_partial_unique_index (id PK, UNIQUE INDEX(check_id) WHERE most_recent)
--    Only rows with most_recent = true participate in the unique index, so the
--    conflicts here exercise the partial-predicate before/after logic.
--    check_id values are > 2e8 so they cannot collide with generator rows.
-- ============================================================
BEGIN;
INSERT INTO test_partial_unique_index (id, check_id, most_recent) VALUES
    (:base + 80001, :base + 91, true),    -- UPDATE-INSERT: active holder of check_id 900000091
    (:base + 80002, :base + 92, true),    -- DELETE-INSERT: active holder of check_id 900000092
    (:base + 80003, :base + 93, true),    -- DELETE-UPDATE: active holder of check_id 900000093 (to be deleted)
    (:base + 80004, :base + 93, false),   -- DELETE-UPDATE: inactive partner, flipped to active
    (:base + 80005, :base + 94, true),    -- UPDATE-UPDATE: active holder of check_id 900000094 (to be deactivated)
    (:base + 80006, :base + 94, false);   -- UPDATE-UPDATE: inactive partner, flipped to active

-- UPDATE-INSERT: deactivate active holder (frees the partial-index key), insert a new active row with same check_id
UPDATE test_partial_unique_index SET most_recent = false WHERE id = :base + 80001;
INSERT INTO test_partial_unique_index (id, check_id, most_recent) VALUES (:base + 80101, :base + 91, true);

-- DELETE-INSERT: delete active holder, insert a new active row with same check_id
DELETE FROM test_partial_unique_index WHERE id = :base + 80002;
INSERT INTO test_partial_unique_index (id, check_id, most_recent) VALUES (:base + 80102, :base + 92, true);

-- DELETE-UPDATE: delete active holder, flip the inactive partner to active (same check_id)
DELETE FROM test_partial_unique_index WHERE id = :base + 80003;
UPDATE test_partial_unique_index SET most_recent = true WHERE id = :base + 80004;

-- UPDATE-UPDATE: deactivate active holder, activate the inactive partner (same check_id)
UPDATE test_partial_unique_index SET most_recent = false WHERE id = :base + 80005;
UPDATE test_partial_unique_index SET most_recent = true WHERE id = :base + 80006;
COMMIT;

-- ============================================================
-- 10. single_unique_index_nulls_not_distinct (id PK, UNIQUE INDEX(email) NULLS NOT DISTINCT)
-- Under NULLS NOT DISTINCT two NULLs are equal, so a NULL free->reuse across PKs is a
-- real conflict. Only one NULL can exist at a time, so the NULL is moved off at the end
-- of the block, leaving none behind for the next cycle.
-- ============================================================
BEGIN;
-- UPDATE-INSERT (non-null value)
INSERT INTO single_unique_index_nulls_not_distinct (id, email) VALUES (:base + 85001, ('nnd1_user1@conflict.test' || :cycle));
UPDATE single_unique_index_nulls_not_distinct SET email = ('nnd1_user1_moved@conflict.test' || :cycle) WHERE id = :base + 85001;
INSERT INTO single_unique_index_nulls_not_distinct (id, email) VALUES (:base + 85101, ('nnd1_user1@conflict.test' || :cycle));

-- NULL free->reuse: free the NULL by moving base+85010 off it, reuse NULL on base+85011
-- (conflict under NULLS NOT DISTINCT), then move base+85011 off NULL to leave none behind.
INSERT INTO single_unique_index_nulls_not_distinct (id, email) VALUES (:base + 85010, NULL);
UPDATE single_unique_index_nulls_not_distinct SET email = ('nnd1_wasnull_a@conflict.test' || :cycle) WHERE id = :base + 85010;
INSERT INTO single_unique_index_nulls_not_distinct (id, email) VALUES (:base + 85011, NULL);
UPDATE single_unique_index_nulls_not_distinct SET email = ('nnd1_wasnull_b@conflict.test' || :cycle) WHERE id = :base + 85011;
COMMIT;

-- ============================================================
-- 11. multi_unique_index_nulls_not_distinct (id PK, UNIQUE INDEX(first_name, last_name) NULLS NOT DISTINCT)
-- ============================================================
BEGIN;
-- UPDATE-INSERT (non-null values)
INSERT INTO multi_unique_index_nulls_not_distinct (id, first_name, last_name) VALUES (:base + 86001, ('nnd2First' || :cycle), ('nnd2Last' || :cycle));
UPDATE multi_unique_index_nulls_not_distinct SET first_name = ('nnd2First_moved' || :cycle) WHERE id = :base + 86001;
INSERT INTO multi_unique_index_nulls_not_distinct (id, first_name, last_name) VALUES (:base + 86101, ('nnd2First' || :cycle), ('nnd2Last' || :cycle));

-- (NULL, NULL) free->reuse: two all-NULL rows conflict under NULLS NOT DISTINCT.
INSERT INTO multi_unique_index_nulls_not_distinct (id, first_name, last_name) VALUES (:base + 86010, NULL, NULL);
UPDATE multi_unique_index_nulls_not_distinct SET first_name = ('nnd2wasnull_a' || :cycle) WHERE id = :base + 86010;
INSERT INTO multi_unique_index_nulls_not_distinct (id, first_name, last_name) VALUES (:base + 86011, NULL, NULL);
UPDATE multi_unique_index_nulls_not_distinct SET first_name = ('nnd2wasnull_b' || :cycle) WHERE id = :base + 86011;
COMMIT;

-- ============================================================
-- 12. partitioned_unique_conflict (PK(id, region), UNIQUE INDEX(email, region), PARTITION BY LIST(region))
-- The unique key includes the partition-key column; conflicts are exercised within a partition.
-- ============================================================
BEGIN;
INSERT INTO partitioned_unique_conflict (id, region, email) VALUES
    (:base + 87001, 'east', ('puc_user1@conflict.test' || :cycle)),
    (:base + 87002, 'east', ('puc_user2@conflict.test' || :cycle)),
    (:base + 87003, 'west', ('puc_user3@conflict.test' || :cycle)),
    (:base + 87004, 'west', ('puc_user4@conflict.test' || :cycle));

-- DELETE-INSERT within the 'east' partition: free (puc_user1, east), reuse on a new PK
DELETE FROM partitioned_unique_conflict WHERE id = :base + 87001 AND region = 'east';
INSERT INTO partitioned_unique_conflict (id, region, email) VALUES (:base + 87101, 'east', ('puc_user1@conflict.test' || :cycle));

-- UPDATE-INSERT within the 'west' partition: free (puc_user3, west) by moving it, reuse on a new PK
UPDATE partitioned_unique_conflict SET email = ('puc_user3_moved@conflict.test' || :cycle) WHERE id = :base + 87003 AND region = 'west';
INSERT INTO partitioned_unique_conflict (id, region, email) VALUES (:base + 87103, 'west', ('puc_user3@conflict.test' || :cycle));
COMMIT;

-- ============================================================
-- 13. single_unique_index_nulls_distinct (id PK, UNIQUE INDEX(email) -- default NULLS DISTINCT)
-- Under NULLS DISTINCT multiple NULLs coexist and a NULL free->reuse is NOT a conflict,
-- so these NULL rows must import without being (wrongly) serialized. A non-null value
-- conflict is included to confirm real conflicts still fire on this table.
-- ============================================================
BEGIN;
-- Multiple NULL rows coexist under NULLS DISTINCT (no conflict, no violation)
INSERT INTO single_unique_index_nulls_distinct (id, email) VALUES
    (:base + 89001, NULL),
    (:base + 89002, NULL),
    (:base + 89003, NULL);

-- NULL free->reuse: delete a NULL holder, insert another NULL. Under NULLS DISTINCT
-- these NULLs do NOT conflict, so the cache must not serialize them.
DELETE FROM single_unique_index_nulls_distinct WHERE id = :base + 89001;
INSERT INTO single_unique_index_nulls_distinct (id, email) VALUES (:base + 89101, NULL);

-- Non-null value free->reuse: a real conflict that must still be detected here.
INSERT INTO single_unique_index_nulls_distinct (id, email) VALUES (:base + 89010, ('nd_user1@conflict.test' || :cycle));
UPDATE single_unique_index_nulls_distinct SET email = ('nd_user1_moved@conflict.test' || :cycle) WHERE id = :base + 89010;
INSERT INTO single_unique_index_nulls_distinct (id, email) VALUES (:base + 89110, ('nd_user1@conflict.test' || :cycle));
COMMIT;
