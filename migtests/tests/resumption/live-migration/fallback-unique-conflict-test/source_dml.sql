-- Deterministic unique-conflict DML for the FORWARD (source -> target) leg.
-- Run REPEATEDLY on a loop throughout the streaming phase by the orchestrator's
-- conflict generator (conflict_generator_source), in parallel with the random
-- event generator, so the conflicts are produced continuously rather than once.
-- LOOP-SAFE: each block first deletes its own high-range rows (id >= 900000000)
-- so every cycle re-seeds from a clean slate.
--
-- Goal: deterministically exercise every conflict type the streaming-phase
-- conflict-detection cache handles (see yb-voyager/cmd/conflictDetectionCache.go):
--   1. DELETE-INSERT   2. DELETE-UPDATE   3. UPDATE-INSERT   4. UPDATE-UPDATE
-- A conflict is two events with DIFFERENT primary keys but the SAME unique-key
-- value: the "freeing" event (DELETE/UPDATE the holder) and the "reusing" event
-- (INSERT/UPDATE a different PK to that value). Because the two rows have
-- different PKs they can be routed to different import channels, so the cache
-- must serialize them; otherwise the reusing event hits a unique violation.
--
-- Design notes:
--  * Each table's seed + conflicts run in ONE transaction so the concurrent
--    random generator cannot delete a seed row between the seed and the
--    conflicting op (which would silently skip the conflict).
--  * All primary keys / numeric unique keys use values > 200,000,000, which is
--    outside the generator's random integer range (-2e8 .. 2e8). This guarantees
--    the deterministic rows never collide with generator-inserted rows.
--  * On the source side every statement is individually valid (no unique
--    violation); the violation only *would* occur on the target if the events
--    were applied out of order, which is exactly what the cache prevents.

-- ============================================================
-- 1. single_unique_constraint (id PK, email UNIQUE)
-- ============================================================
BEGIN;
DELETE FROM single_unique_constraint WHERE id >= 900000000;
INSERT INTO single_unique_constraint (id, email) VALUES
    (900000001, 'suc_user1@conflict.test'),
    (900000002, 'suc_user2@conflict.test'),
    (900000003, 'suc_user3@conflict.test'),
    (900000004, 'suc_user4@conflict.test'),
    (900000005, 'suc_user5@conflict.test'),
    (900000006, 'suc_user6@conflict.test');

-- DELETE-INSERT: free suc_user1, reuse it on a new PK
DELETE FROM single_unique_constraint WHERE id = 900000001;
INSERT INTO single_unique_constraint (id, email) VALUES (900000101, 'suc_user1@conflict.test');

-- DELETE-UPDATE: free suc_user2, reuse it via UPDATE of a different PK
DELETE FROM single_unique_constraint WHERE id = 900000002;
UPDATE single_unique_constraint SET email = 'suc_user2@conflict.test' WHERE id = 900000003;

-- UPDATE-INSERT: free suc_user4 by moving it, reuse it on a new PK
UPDATE single_unique_constraint SET email = 'suc_user4_moved@conflict.test' WHERE id = 900000004;
INSERT INTO single_unique_constraint (id, email) VALUES (900000102, 'suc_user4@conflict.test');

-- UPDATE-UPDATE: free suc_user5 by moving it, reuse it via UPDATE of a different PK
UPDATE single_unique_constraint SET email = 'suc_user5_moved@conflict.test' WHERE id = 900000005;
UPDATE single_unique_constraint SET email = 'suc_user5@conflict.test' WHERE id = 900000006;
COMMIT;

-- ============================================================
-- 2. multi_unique_constraint (id PK, UNIQUE(first_name, last_name))
-- ============================================================
BEGIN;
DELETE FROM multi_unique_constraint WHERE id >= 900000000;
INSERT INTO multi_unique_constraint (id, first_name, last_name) VALUES
    (900010001, 'SrcJohn',  'Doe'),
    (900010002, 'SrcJane',  'Smith'),
    (900010003, 'SrcBob',   'Jones'),
    (900010004, 'SrcAlice', 'Williams'),
    (900010005, 'SrcTom',   'Clark'),
    (900010006, 'SrcEve',   'Davis');

-- DELETE-INSERT
DELETE FROM multi_unique_constraint WHERE id = 900010001;
INSERT INTO multi_unique_constraint (id, first_name, last_name) VALUES (900010101, 'SrcJohn', 'Doe');

-- DELETE-UPDATE
DELETE FROM multi_unique_constraint WHERE id = 900010002;
UPDATE multi_unique_constraint SET first_name = 'SrcJane', last_name = 'Smith' WHERE id = 900010003;

-- UPDATE-INSERT
UPDATE multi_unique_constraint SET first_name = 'SrcAlice_moved' WHERE id = 900010004;
INSERT INTO multi_unique_constraint (id, first_name, last_name) VALUES (900010102, 'SrcAlice', 'Williams');

-- UPDATE-UPDATE
UPDATE multi_unique_constraint SET first_name = 'SrcTom_moved' WHERE id = 900010005;
UPDATE multi_unique_constraint SET first_name = 'SrcTom', last_name = 'Clark' WHERE id = 900010006;
COMMIT;

-- ============================================================
-- 3. same_column_unique_constraint_and_index (id PK, email UNIQUE + UNIQUE INDEX on email)
-- ============================================================
BEGIN;
DELETE FROM same_column_unique_constraint_and_index WHERE id >= 900000000;
INSERT INTO same_column_unique_constraint_and_index (id, email) VALUES
    (900020001, 'scuci_user1@conflict.test'),
    (900020002, 'scuci_user2@conflict.test'),
    (900020003, 'scuci_user3@conflict.test'),
    (900020004, 'scuci_user4@conflict.test'),
    (900020005, 'scuci_user5@conflict.test'),
    (900020006, 'scuci_user6@conflict.test');

-- DELETE-INSERT
DELETE FROM same_column_unique_constraint_and_index WHERE id = 900020001;
INSERT INTO same_column_unique_constraint_and_index (id, email) VALUES (900020101, 'scuci_user1@conflict.test');

-- DELETE-UPDATE
DELETE FROM same_column_unique_constraint_and_index WHERE id = 900020002;
UPDATE same_column_unique_constraint_and_index SET email = 'scuci_user2@conflict.test' WHERE id = 900020003;

-- UPDATE-INSERT
UPDATE same_column_unique_constraint_and_index SET email = 'scuci_user4_moved@conflict.test' WHERE id = 900020004;
INSERT INTO same_column_unique_constraint_and_index (id, email) VALUES (900020102, 'scuci_user4@conflict.test');

-- UPDATE-UPDATE
UPDATE same_column_unique_constraint_and_index SET email = 'scuci_user5_moved@conflict.test' WHERE id = 900020005;
UPDATE same_column_unique_constraint_and_index SET email = 'scuci_user5@conflict.test' WHERE id = 900020006;
COMMIT;

-- ============================================================
-- 4. single_unique_index (id PK, UNIQUE INDEX on "Ssn" -- case-sensitive column)
-- ============================================================
BEGIN;
DELETE FROM single_unique_index WHERE id >= 900000000;
INSERT INTO single_unique_index (id, "Ssn") VALUES
    (900030001, 'SRC-SSN-1'),
    (900030002, 'SRC-SSN-2'),
    (900030003, 'SRC-SSN-3'),
    (900030004, 'SRC-SSN-4'),
    (900030005, 'SRC-SSN-5'),
    (900030006, 'SRC-SSN-6');

-- DELETE-INSERT
DELETE FROM single_unique_index WHERE id = 900030001;
INSERT INTO single_unique_index (id, "Ssn") VALUES (900030101, 'SRC-SSN-1');

-- DELETE-UPDATE
DELETE FROM single_unique_index WHERE id = 900030002;
UPDATE single_unique_index SET "Ssn" = 'SRC-SSN-2' WHERE id = 900030003;

-- UPDATE-INSERT
UPDATE single_unique_index SET "Ssn" = 'SRC-SSN-4-moved' WHERE id = 900030004;
INSERT INTO single_unique_index (id, "Ssn") VALUES (900030102, 'SRC-SSN-4');

-- UPDATE-UPDATE
UPDATE single_unique_index SET "Ssn" = 'SRC-SSN-5-moved' WHERE id = 900030005;
UPDATE single_unique_index SET "Ssn" = 'SRC-SSN-5' WHERE id = 900030006;
COMMIT;

-- ============================================================
-- 5. multi_unique_index (id PK, UNIQUE INDEX(first_name, last_name))
-- ============================================================
BEGIN;
DELETE FROM multi_unique_index WHERE id >= 900000000;
INSERT INTO multi_unique_index (id, first_name, last_name) VALUES
    (900040001, 'IdxJohn',  'Doe'),
    (900040002, 'IdxJane',  'Smith'),
    (900040003, 'IdxBob',   'Jones'),
    (900040004, 'IdxAlice', 'Williams'),
    (900040005, 'IdxTom',   'Clark'),
    (900040006, 'IdxEve',   'Davis');

-- DELETE-INSERT
DELETE FROM multi_unique_index WHERE id = 900040001;
INSERT INTO multi_unique_index (id, first_name, last_name) VALUES (900040101, 'IdxJohn', 'Doe');

-- DELETE-UPDATE
DELETE FROM multi_unique_index WHERE id = 900040002;
UPDATE multi_unique_index SET first_name = 'IdxJane', last_name = 'Smith' WHERE id = 900040003;

-- UPDATE-INSERT
UPDATE multi_unique_index SET first_name = 'IdxAlice_moved' WHERE id = 900040004;
INSERT INTO multi_unique_index (id, first_name, last_name) VALUES (900040102, 'IdxAlice', 'Williams');

-- UPDATE-UPDATE
UPDATE multi_unique_index SET first_name = 'IdxTom_moved' WHERE id = 900040005;
UPDATE multi_unique_index SET first_name = 'IdxTom', last_name = 'Clark' WHERE id = 900040006;
COMMIT;

-- ============================================================
-- 6. different_columns_unique_constraint_and_index
--    (id PK, email UNIQUE, UNIQUE INDEX on phone_number) -- two independent unique keys
-- ============================================================
BEGIN;
DELETE FROM different_columns_unique_constraint_and_index WHERE id >= 900000000;
INSERT INTO different_columns_unique_constraint_and_index (id, email, phone_number) VALUES
    (900050001, 'dcuci_user1@conflict.test', 'dcph-1'),
    (900050002, 'dcuci_user2@conflict.test', 'dcph-2'),
    (900050003, 'dcuci_user3@conflict.test', 'dcph-3'),
    (900050004, 'dcuci_user4@conflict.test', 'dcph-4'),
    (900050005, 'dcuci_user5@conflict.test', 'dcph-5'),
    (900050006, 'dcuci_user6@conflict.test', 'dcph-6');

-- DELETE-INSERT (conflict on both email and phone_number)
DELETE FROM different_columns_unique_constraint_and_index WHERE id = 900050001;
INSERT INTO different_columns_unique_constraint_and_index (id, email, phone_number) VALUES (900050101, 'dcuci_user1@conflict.test', 'dcph-1');

-- DELETE-UPDATE (conflict on phone_number unique index)
DELETE FROM different_columns_unique_constraint_and_index WHERE id = 900050002;
UPDATE different_columns_unique_constraint_and_index SET phone_number = 'dcph-2' WHERE id = 900050003;

-- UPDATE-INSERT (conflict on email unique constraint)
UPDATE different_columns_unique_constraint_and_index SET email = 'dcuci_user4_moved@conflict.test' WHERE id = 900050004;
INSERT INTO different_columns_unique_constraint_and_index (id, email, phone_number) VALUES (900050102, 'dcuci_user4@conflict.test', 'dcph-104');

-- UPDATE-UPDATE (conflict on phone_number unique index)
UPDATE different_columns_unique_constraint_and_index SET phone_number = 'dcph-5-moved' WHERE id = 900050005;
UPDATE different_columns_unique_constraint_and_index SET phone_number = 'dcph-5' WHERE id = 900050006;
COMMIT;

-- ============================================================
-- 7. subset_columns_unique_constraint_and_index
--    (id PK, UNIQUE(first_name,last_name), UNIQUE INDEX(first_name,last_name,phone_number))
-- ============================================================
BEGIN;
DELETE FROM subset_columns_unique_constraint_and_index WHERE id >= 900000000;
INSERT INTO subset_columns_unique_constraint_and_index (id, first_name, last_name, phone_number) VALUES
    (900060001, 'SubJohn',  'Doe',      'subph-1'),
    (900060002, 'SubJane',  'Smith',    'subph-2'),
    (900060003, 'SubBob',   'Jones',    'subph-3'),
    (900060004, 'SubAlice', 'Williams', 'subph-4'),
    (900060005, 'SubTom',   'Clark',    'subph-5'),
    (900060006, 'SubEve',   'Davis',    'subph-6');

-- DELETE-INSERT
DELETE FROM subset_columns_unique_constraint_and_index WHERE id = 900060001;
INSERT INTO subset_columns_unique_constraint_and_index (id, first_name, last_name, phone_number) VALUES (900060101, 'SubJohn', 'Doe', 'subph-101');

-- DELETE-UPDATE
DELETE FROM subset_columns_unique_constraint_and_index WHERE id = 900060002;
UPDATE subset_columns_unique_constraint_and_index SET first_name = 'SubJane', last_name = 'Smith' WHERE id = 900060003;

-- UPDATE-INSERT
UPDATE subset_columns_unique_constraint_and_index SET first_name = 'SubAlice_moved' WHERE id = 900060004;
INSERT INTO subset_columns_unique_constraint_and_index (id, first_name, last_name, phone_number) VALUES (900060102, 'SubAlice', 'Williams', 'subph-102');

-- UPDATE-UPDATE
UPDATE subset_columns_unique_constraint_and_index SET first_name = 'SubTom_moved' WHERE id = 900060005;
UPDATE subset_columns_unique_constraint_and_index SET first_name = 'SubTom', last_name = 'Clark' WHERE id = 900060006;
COMMIT;

-- ============================================================
-- 8. expression_based_unique_index (id PK, UNIQUE INDEX on LOWER(email))
--    Conflicts are produced via different letter-casing that collapses to the
--    same LOWER(email) value.
-- ============================================================
BEGIN;
DELETE FROM expression_based_unique_index WHERE id >= 900000000;
INSERT INTO expression_based_unique_index (id, email) VALUES
    (900070001, 'Expr_User1@conflict.test'),
    (900070002, 'Expr_User2@conflict.test'),
    (900070003, 'Expr_User3@conflict.test'),
    (900070004, 'Expr_User4@conflict.test'),
    (900070005, 'Expr_User5@conflict.test'),
    (900070006, 'Expr_User6@conflict.test');

-- DELETE-INSERT (LOWER(email) collision)
DELETE FROM expression_based_unique_index WHERE id = 900070001;
INSERT INTO expression_based_unique_index (id, email) VALUES (900070101, 'EXPR_USER1@conflict.test');

-- DELETE-UPDATE
DELETE FROM expression_based_unique_index WHERE id = 900070002;
UPDATE expression_based_unique_index SET email = 'EXPR_USER2@conflict.test' WHERE id = 900070003;

-- UPDATE-INSERT
UPDATE expression_based_unique_index SET email = 'Expr_User4_moved@conflict.test' WHERE id = 900070004;
INSERT INTO expression_based_unique_index (id, email) VALUES (900070102, 'expr_user4@conflict.test');

-- UPDATE-UPDATE
UPDATE expression_based_unique_index SET email = 'Expr_User5_moved@conflict.test' WHERE id = 900070005;
UPDATE expression_based_unique_index SET email = 'EXPR_user5@conflict.test' WHERE id = 900070006;
COMMIT;

-- ============================================================
-- 9. test_partial_unique_index (id PK, UNIQUE INDEX(check_id) WHERE most_recent)
--    Only rows with most_recent = true participate in the unique index, so the
--    conflicts here exercise the partial-predicate before/after logic.
--    check_id values are > 2e8 so they cannot collide with generator rows.
-- ============================================================
BEGIN;
DELETE FROM test_partial_unique_index WHERE id >= 900000000;
INSERT INTO test_partial_unique_index (id, check_id, most_recent) VALUES
    (900080001, 900000091, true),    -- UPDATE-INSERT: active holder of check_id 900000091
    (900080002, 900000092, true),    -- DELETE-INSERT: active holder of check_id 900000092
    (900080003, 900000093, true),    -- DELETE-UPDATE: active holder of check_id 900000093 (to be deleted)
    (900080004, 900000093, false),   -- DELETE-UPDATE: inactive partner, flipped to active
    (900080005, 900000094, true),    -- UPDATE-UPDATE: active holder of check_id 900000094 (to be deactivated)
    (900080006, 900000094, false);   -- UPDATE-UPDATE: inactive partner, flipped to active

-- UPDATE-INSERT: deactivate active holder (frees the partial-index key), insert a new active row with same check_id
UPDATE test_partial_unique_index SET most_recent = false WHERE id = 900080001;
INSERT INTO test_partial_unique_index (id, check_id, most_recent) VALUES (900080101, 900000091, true);

-- DELETE-INSERT: delete active holder, insert a new active row with same check_id
DELETE FROM test_partial_unique_index WHERE id = 900080002;
INSERT INTO test_partial_unique_index (id, check_id, most_recent) VALUES (900080102, 900000092, true);

-- DELETE-UPDATE: delete active holder, flip the inactive partner to active (same check_id)
DELETE FROM test_partial_unique_index WHERE id = 900080003;
UPDATE test_partial_unique_index SET most_recent = true WHERE id = 900080004;

-- UPDATE-UPDATE: deactivate active holder, activate the inactive partner (same check_id)
UPDATE test_partial_unique_index SET most_recent = false WHERE id = 900080005;
UPDATE test_partial_unique_index SET most_recent = true WHERE id = 900080006;
COMMIT;
