-- Deterministic unique-conflict DML for the FALLBACK (target -> source) leg.
-- Runs ONCE at the start of the fallback streaming phase on the YugabyteDB
-- target, while the target-side random generator also produces traffic.
--
-- Same four conflict types as the forward leg (DELETE-INSERT, DELETE-UPDATE,
-- UPDATE-INSERT, UPDATE-UPDATE). For an export-from-target source the
-- conflict-detection cache cannot rely on YB-CDC before-images, so it falls
-- back to a coarser rule (cached DELETE => conflict; cached UPDATE touching the
-- same unique-key columns => conflict). These scenarios exercise that path.
--
-- Id / numeric-unique-key base is 950,000,000 so the rows never collide with:
--   * source-origin rows replicated forward (900,000,000 base),
--   * target-side generator rows (random integers in -2e8 .. 2e8).
-- Each table's seed + conflicts run in one transaction to stay isolated from
-- the concurrent generator.

-- ============================================================
-- 1. single_unique_constraint (id PK, email UNIQUE)
-- ============================================================
BEGIN;
INSERT INTO single_unique_constraint (id, email) VALUES
    (950000001, 'tgt_suc_user1@conflict.test'),
    (950000002, 'tgt_suc_user2@conflict.test'),
    (950000003, 'tgt_suc_user3@conflict.test'),
    (950000004, 'tgt_suc_user4@conflict.test'),
    (950000005, 'tgt_suc_user5@conflict.test'),
    (950000006, 'tgt_suc_user6@conflict.test');

-- DELETE-INSERT
DELETE FROM single_unique_constraint WHERE id = 950000001;
INSERT INTO single_unique_constraint (id, email) VALUES (950000101, 'tgt_suc_user1@conflict.test');

-- DELETE-UPDATE
DELETE FROM single_unique_constraint WHERE id = 950000002;
UPDATE single_unique_constraint SET email = 'tgt_suc_user2@conflict.test' WHERE id = 950000003;

-- UPDATE-INSERT
UPDATE single_unique_constraint SET email = 'tgt_suc_user4_moved@conflict.test' WHERE id = 950000004;
INSERT INTO single_unique_constraint (id, email) VALUES (950000102, 'tgt_suc_user4@conflict.test');

-- UPDATE-UPDATE
UPDATE single_unique_constraint SET email = 'tgt_suc_user5_moved@conflict.test' WHERE id = 950000005;
UPDATE single_unique_constraint SET email = 'tgt_suc_user5@conflict.test' WHERE id = 950000006;
COMMIT;

-- ============================================================
-- 2. multi_unique_constraint (id PK, UNIQUE(first_name, last_name))
-- ============================================================
BEGIN;
INSERT INTO multi_unique_constraint (id, first_name, last_name) VALUES
    (950010001, 'TgtJohn',  'Doe'),
    (950010002, 'TgtJane',  'Smith'),
    (950010003, 'TgtBob',   'Jones'),
    (950010004, 'TgtAlice', 'Williams'),
    (950010005, 'TgtTom',   'Clark'),
    (950010006, 'TgtEve',   'Davis');

-- DELETE-INSERT
DELETE FROM multi_unique_constraint WHERE id = 950010001;
INSERT INTO multi_unique_constraint (id, first_name, last_name) VALUES (950010101, 'TgtJohn', 'Doe');

-- DELETE-UPDATE
DELETE FROM multi_unique_constraint WHERE id = 950010002;
UPDATE multi_unique_constraint SET first_name = 'TgtJane', last_name = 'Smith' WHERE id = 950010003;

-- UPDATE-INSERT
UPDATE multi_unique_constraint SET first_name = 'TgtAlice_moved' WHERE id = 950010004;
INSERT INTO multi_unique_constraint (id, first_name, last_name) VALUES (950010102, 'TgtAlice', 'Williams');

-- UPDATE-UPDATE
UPDATE multi_unique_constraint SET first_name = 'TgtTom_moved' WHERE id = 950010005;
UPDATE multi_unique_constraint SET first_name = 'TgtTom', last_name = 'Clark' WHERE id = 950010006;
COMMIT;

-- ============================================================
-- 3. same_column_unique_constraint_and_index (id PK, email UNIQUE + UNIQUE INDEX on email)
-- ============================================================
BEGIN;
INSERT INTO same_column_unique_constraint_and_index (id, email) VALUES
    (950020001, 'tgt_scuci_user1@conflict.test'),
    (950020002, 'tgt_scuci_user2@conflict.test'),
    (950020003, 'tgt_scuci_user3@conflict.test'),
    (950020004, 'tgt_scuci_user4@conflict.test'),
    (950020005, 'tgt_scuci_user5@conflict.test'),
    (950020006, 'tgt_scuci_user6@conflict.test');

-- DELETE-INSERT
DELETE FROM same_column_unique_constraint_and_index WHERE id = 950020001;
INSERT INTO same_column_unique_constraint_and_index (id, email) VALUES (950020101, 'tgt_scuci_user1@conflict.test');

-- DELETE-UPDATE
DELETE FROM same_column_unique_constraint_and_index WHERE id = 950020002;
UPDATE same_column_unique_constraint_and_index SET email = 'tgt_scuci_user2@conflict.test' WHERE id = 950020003;

-- UPDATE-INSERT
UPDATE same_column_unique_constraint_and_index SET email = 'tgt_scuci_user4_moved@conflict.test' WHERE id = 950020004;
INSERT INTO same_column_unique_constraint_and_index (id, email) VALUES (950020102, 'tgt_scuci_user4@conflict.test');

-- UPDATE-UPDATE
UPDATE same_column_unique_constraint_and_index SET email = 'tgt_scuci_user5_moved@conflict.test' WHERE id = 950020005;
UPDATE same_column_unique_constraint_and_index SET email = 'tgt_scuci_user5@conflict.test' WHERE id = 950020006;
COMMIT;

-- ============================================================
-- 4. single_unique_index (id PK, UNIQUE INDEX on "Ssn" -- case-sensitive column)
-- ============================================================
BEGIN;
INSERT INTO single_unique_index (id, "Ssn") VALUES
    (950030001, 'TGT-SSN-1'),
    (950030002, 'TGT-SSN-2'),
    (950030003, 'TGT-SSN-3'),
    (950030004, 'TGT-SSN-4'),
    (950030005, 'TGT-SSN-5'),
    (950030006, 'TGT-SSN-6');

-- DELETE-INSERT
DELETE FROM single_unique_index WHERE id = 950030001;
INSERT INTO single_unique_index (id, "Ssn") VALUES (950030101, 'TGT-SSN-1');

-- DELETE-UPDATE
DELETE FROM single_unique_index WHERE id = 950030002;
UPDATE single_unique_index SET "Ssn" = 'TGT-SSN-2' WHERE id = 950030003;

-- UPDATE-INSERT
UPDATE single_unique_index SET "Ssn" = 'TGT-SSN-4-moved' WHERE id = 950030004;
INSERT INTO single_unique_index (id, "Ssn") VALUES (950030102, 'TGT-SSN-4');

-- UPDATE-UPDATE
UPDATE single_unique_index SET "Ssn" = 'TGT-SSN-5-moved' WHERE id = 950030005;
UPDATE single_unique_index SET "Ssn" = 'TGT-SSN-5' WHERE id = 950030006;
COMMIT;

-- ============================================================
-- 5. multi_unique_index (id PK, UNIQUE INDEX(first_name, last_name))
-- ============================================================
BEGIN;
INSERT INTO multi_unique_index (id, first_name, last_name) VALUES
    (950040001, 'TgtIdxJohn',  'Doe'),
    (950040002, 'TgtIdxJane',  'Smith'),
    (950040003, 'TgtIdxBob',   'Jones'),
    (950040004, 'TgtIdxAlice', 'Williams'),
    (950040005, 'TgtIdxTom',   'Clark'),
    (950040006, 'TgtIdxEve',   'Davis');

-- DELETE-INSERT
DELETE FROM multi_unique_index WHERE id = 950040001;
INSERT INTO multi_unique_index (id, first_name, last_name) VALUES (950040101, 'TgtIdxJohn', 'Doe');

-- DELETE-UPDATE
DELETE FROM multi_unique_index WHERE id = 950040002;
UPDATE multi_unique_index SET first_name = 'TgtIdxJane', last_name = 'Smith' WHERE id = 950040003;

-- UPDATE-INSERT
UPDATE multi_unique_index SET first_name = 'TgtIdxAlice_moved' WHERE id = 950040004;
INSERT INTO multi_unique_index (id, first_name, last_name) VALUES (950040102, 'TgtIdxAlice', 'Williams');

-- UPDATE-UPDATE
UPDATE multi_unique_index SET first_name = 'TgtIdxTom_moved' WHERE id = 950040005;
UPDATE multi_unique_index SET first_name = 'TgtIdxTom', last_name = 'Clark' WHERE id = 950040006;
COMMIT;

-- ============================================================
-- 6. different_columns_unique_constraint_and_index
--    (id PK, email UNIQUE, UNIQUE INDEX on phone_number)
-- ============================================================
BEGIN;
INSERT INTO different_columns_unique_constraint_and_index (id, email, phone_number) VALUES
    (950050001, 'tgt_dcuci_user1@conflict.test', 'tgtdcph-1'),
    (950050002, 'tgt_dcuci_user2@conflict.test', 'tgtdcph-2'),
    (950050003, 'tgt_dcuci_user3@conflict.test', 'tgtdcph-3'),
    (950050004, 'tgt_dcuci_user4@conflict.test', 'tgtdcph-4'),
    (950050005, 'tgt_dcuci_user5@conflict.test', 'tgtdcph-5'),
    (950050006, 'tgt_dcuci_user6@conflict.test', 'tgtdcph-6');

-- DELETE-INSERT (conflict on both email and phone_number)
DELETE FROM different_columns_unique_constraint_and_index WHERE id = 950050001;
INSERT INTO different_columns_unique_constraint_and_index (id, email, phone_number) VALUES (950050101, 'tgt_dcuci_user1@conflict.test', 'tgtdcph-1');

-- DELETE-UPDATE (conflict on phone_number unique index)
DELETE FROM different_columns_unique_constraint_and_index WHERE id = 950050002;
UPDATE different_columns_unique_constraint_and_index SET phone_number = 'tgtdcph-2' WHERE id = 950050003;

-- UPDATE-INSERT (conflict on email unique constraint)
UPDATE different_columns_unique_constraint_and_index SET email = 'tgt_dcuci_user4_moved@conflict.test' WHERE id = 950050004;
INSERT INTO different_columns_unique_constraint_and_index (id, email, phone_number) VALUES (950050102, 'tgt_dcuci_user4@conflict.test', 'tgtdcph-104');

-- UPDATE-UPDATE (conflict on phone_number unique index)
UPDATE different_columns_unique_constraint_and_index SET phone_number = 'tgtdcph-5-moved' WHERE id = 950050005;
UPDATE different_columns_unique_constraint_and_index SET phone_number = 'tgtdcph-5' WHERE id = 950050006;
COMMIT;

-- ============================================================
-- 7. subset_columns_unique_constraint_and_index
--    (id PK, UNIQUE(first_name,last_name), UNIQUE INDEX(first_name,last_name,phone_number))
-- ============================================================
BEGIN;
INSERT INTO subset_columns_unique_constraint_and_index (id, first_name, last_name, phone_number) VALUES
    (950060001, 'TgtSubJohn',  'Doe',      'tgtsubph-1'),
    (950060002, 'TgtSubJane',  'Smith',    'tgtsubph-2'),
    (950060003, 'TgtSubBob',   'Jones',    'tgtsubph-3'),
    (950060004, 'TgtSubAlice', 'Williams', 'tgtsubph-4'),
    (950060005, 'TgtSubTom',   'Clark',    'tgtsubph-5'),
    (950060006, 'TgtSubEve',   'Davis',    'tgtsubph-6');

-- DELETE-INSERT
DELETE FROM subset_columns_unique_constraint_and_index WHERE id = 950060001;
INSERT INTO subset_columns_unique_constraint_and_index (id, first_name, last_name, phone_number) VALUES (950060101, 'TgtSubJohn', 'Doe', 'tgtsubph-101');

-- DELETE-UPDATE
DELETE FROM subset_columns_unique_constraint_and_index WHERE id = 950060002;
UPDATE subset_columns_unique_constraint_and_index SET first_name = 'TgtSubJane', last_name = 'Smith' WHERE id = 950060003;

-- UPDATE-INSERT
UPDATE subset_columns_unique_constraint_and_index SET first_name = 'TgtSubAlice_moved' WHERE id = 950060004;
INSERT INTO subset_columns_unique_constraint_and_index (id, first_name, last_name, phone_number) VALUES (950060102, 'TgtSubAlice', 'Williams', 'tgtsubph-102');

-- UPDATE-UPDATE
UPDATE subset_columns_unique_constraint_and_index SET first_name = 'TgtSubTom_moved' WHERE id = 950060005;
UPDATE subset_columns_unique_constraint_and_index SET first_name = 'TgtSubTom', last_name = 'Clark' WHERE id = 950060006;
COMMIT;

-- ============================================================
-- 8. expression_based_unique_index (id PK, UNIQUE INDEX on LOWER(email))
-- ============================================================
BEGIN;
INSERT INTO expression_based_unique_index (id, email) VALUES
    (950070001, 'Tgt_Expr_User1@conflict.test'),
    (950070002, 'Tgt_Expr_User2@conflict.test'),
    (950070003, 'Tgt_Expr_User3@conflict.test'),
    (950070004, 'Tgt_Expr_User4@conflict.test'),
    (950070005, 'Tgt_Expr_User5@conflict.test'),
    (950070006, 'Tgt_Expr_User6@conflict.test');

-- DELETE-INSERT (LOWER(email) collision)
DELETE FROM expression_based_unique_index WHERE id = 950070001;
INSERT INTO expression_based_unique_index (id, email) VALUES (950070101, 'TGT_EXPR_USER1@conflict.test');

-- DELETE-UPDATE
DELETE FROM expression_based_unique_index WHERE id = 950070002;
UPDATE expression_based_unique_index SET email = 'TGT_EXPR_USER2@conflict.test' WHERE id = 950070003;

-- UPDATE-INSERT
UPDATE expression_based_unique_index SET email = 'Tgt_Expr_User4_moved@conflict.test' WHERE id = 950070004;
INSERT INTO expression_based_unique_index (id, email) VALUES (950070102, 'tgt_expr_user4@conflict.test');

-- UPDATE-UPDATE
UPDATE expression_based_unique_index SET email = 'Tgt_Expr_User5_moved@conflict.test' WHERE id = 950070005;
UPDATE expression_based_unique_index SET email = 'TGT_EXPR_user5@conflict.test' WHERE id = 950070006;
COMMIT;

-- ============================================================
-- 9. test_partial_unique_index (id PK, UNIQUE INDEX(check_id) WHERE most_recent)
-- ============================================================
BEGIN;
INSERT INTO test_partial_unique_index (id, check_id, most_recent) VALUES
    (950080001, 950000091, true),
    (950080002, 950000092, true),
    (950080003, 950000093, true),
    (950080004, 950000093, false),
    (950080005, 950000094, true),
    (950080006, 950000094, false);

-- UPDATE-INSERT
UPDATE test_partial_unique_index SET most_recent = false WHERE id = 950080001;
INSERT INTO test_partial_unique_index (id, check_id, most_recent) VALUES (950080101, 950000091, true);

-- DELETE-INSERT
DELETE FROM test_partial_unique_index WHERE id = 950080002;
INSERT INTO test_partial_unique_index (id, check_id, most_recent) VALUES (950080102, 950000092, true);

-- DELETE-UPDATE
DELETE FROM test_partial_unique_index WHERE id = 950080003;
UPDATE test_partial_unique_index SET most_recent = true WHERE id = 950080004;

-- UPDATE-UPDATE
UPDATE test_partial_unique_index SET most_recent = false WHERE id = 950080005;
UPDATE test_partial_unique_index SET most_recent = true WHERE id = 950080006;
COMMIT;
