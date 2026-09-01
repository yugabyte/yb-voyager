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
-- Derived from fallback-unique-conflict-test/source_dml.sql, but reshaped for
-- pick_random_custom_key (orchestrator.py): that action randomly selects one
-- (table, columns) pair as this run's `--cdc-partition-key-overrides`, and the
-- importer requires a custom key column to be immutable (never appear in an
-- UPDATE). Since the pick happens at run time, EVERY candidate table's DML
-- must already satisfy that regardless of which one gets picked -- so tables
-- 1-7 and 10/11/13 below keep ONLY DELETE-based free/reuse (free a unique
-- value via DELETE, reuse it via INSERT on a different PK) and drop any
-- UPDATE that would touch the column(s) that might be this run's custom key.
-- Tables 9 and 12 need no changes: their existing DML already never updates
-- check_id/region. Only table 8 (expression-based unique index -- the
-- importer rejects pk/custom routing on these outright, a hard guardrail, not
-- a DML limitation) is excluded from the candidate pool and keeps its
-- original, fuller DML, always PK-routed.

\if :{?cycle}
\else
  \set cycle 0
\endif
\set base (900000000 + :cycle * 100000)

-- ============================================================
-- 1. single_unique_constraint (id PK, email UNIQUE) -- custom-key candidate: (email)
-- ============================================================
BEGIN;
INSERT INTO single_unique_constraint (id, email) VALUES
    (:base + 1, ('suc_user1@conflict.test' || :cycle));

-- DELETE-INSERT: free suc_user1, reuse it on a new PK
DELETE FROM single_unique_constraint WHERE id = :base + 1;
INSERT INTO single_unique_constraint (id, email) VALUES (:base + 101, ('suc_user1@conflict.test' || :cycle));
COMMIT;

-- ============================================================
-- 2. multi_unique_constraint (id PK, UNIQUE(first_name, last_name)) -- custom-key candidate: (first_name, last_name)
-- ============================================================
BEGIN;
INSERT INTO multi_unique_constraint (id, first_name, last_name) VALUES
    (:base + 10001, ('SrcJohn' || :cycle), ('Doe' || :cycle));

-- DELETE-INSERT
DELETE FROM multi_unique_constraint WHERE id = :base + 10001;
INSERT INTO multi_unique_constraint (id, first_name, last_name) VALUES (:base + 10101, ('SrcJohn' || :cycle), ('Doe' || :cycle));
COMMIT;

-- ============================================================
-- 3. same_column_unique_constraint_and_index (id PK, email UNIQUE + UNIQUE INDEX on email) -- custom-key candidate: (email)
-- ============================================================
BEGIN;
INSERT INTO same_column_unique_constraint_and_index (id, email) VALUES
    (:base + 20001, ('scuci_user1@conflict.test' || :cycle));

-- DELETE-INSERT
DELETE FROM same_column_unique_constraint_and_index WHERE id = :base + 20001;
INSERT INTO same_column_unique_constraint_and_index (id, email) VALUES (:base + 20101, ('scuci_user1@conflict.test' || :cycle));
COMMIT;

-- ============================================================
-- 4. single_unique_index (id PK, UNIQUE INDEX on "Ssn" -- case-sensitive column) -- custom-key candidate: ("Ssn")
-- ============================================================
BEGIN;
INSERT INTO single_unique_index (id, "Ssn") VALUES
    (:base + 30001, ('SRC-SSN-1' || :cycle));

-- DELETE-INSERT
DELETE FROM single_unique_index WHERE id = :base + 30001;
INSERT INTO single_unique_index (id, "Ssn") VALUES (:base + 30101, ('SRC-SSN-1' || :cycle));
COMMIT;

-- ============================================================
-- 5. multi_unique_index (id PK, UNIQUE INDEX(first_name, last_name)) -- custom-key candidate: (first_name, last_name)
-- ============================================================
BEGIN;
INSERT INTO multi_unique_index (id, first_name, last_name) VALUES
    (:base + 40001, ('IdxJohn' || :cycle), ('Doe' || :cycle));

-- DELETE-INSERT
DELETE FROM multi_unique_index WHERE id = :base + 40001;
INSERT INTO multi_unique_index (id, first_name, last_name) VALUES (:base + 40101, ('IdxJohn' || :cycle), ('Doe' || :cycle));
COMMIT;

-- ============================================================
-- 6. different_columns_unique_constraint_and_index
--    (id PK, email UNIQUE, UNIQUE INDEX on phone_number) -- two independent unique keys
--    custom-key candidates: (email) or (phone_number)
-- ============================================================
BEGIN;
INSERT INTO different_columns_unique_constraint_and_index (id, email, phone_number) VALUES
    (:base + 50001, ('dcuci_user1@conflict.test' || :cycle), ('dcph-1' || :cycle));

-- DELETE-INSERT (conflict on both email and phone_number)
DELETE FROM different_columns_unique_constraint_and_index WHERE id = :base + 50001;
INSERT INTO different_columns_unique_constraint_and_index (id, email, phone_number) VALUES (:base + 50101, ('dcuci_user1@conflict.test' || :cycle), ('dcph-1' || :cycle));
COMMIT;

-- ============================================================
-- 7. subset_columns_unique_constraint_and_index
--    (id PK, UNIQUE(first_name,last_name), UNIQUE INDEX(first_name,last_name,phone_number))
--    custom-key candidate: (first_name, last_name)
-- ============================================================
BEGIN;
INSERT INTO subset_columns_unique_constraint_and_index (id, first_name, last_name, phone_number) VALUES
    (:base + 60001, ('SubJohn' || :cycle), ('Doe' || :cycle), ('subph-1' || :cycle));

-- DELETE-INSERT
DELETE FROM subset_columns_unique_constraint_and_index WHERE id = :base + 60001;
INSERT INTO subset_columns_unique_constraint_and_index (id, first_name, last_name, phone_number) VALUES (:base + 60101, ('SubJohn' || :cycle), ('Doe' || :cycle), ('subph-101' || :cycle));
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
--    custom-key candidate: (check_id) -- already immutable as written: only
--    `most_recent` is ever updated, check_id itself is only ever inserted.
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
--     custom-key candidate: (email) -- free/reuse rewritten to DELETE-INSERT
--     (never UPDATE) so email is immutability-safe if picked as the custom key.
-- Under NULLS NOT DISTINCT two NULLs are equal, so a NULL free->reuse across PKs is a
-- real conflict. Only one NULL can exist at a time, so the NULL is cleared via DELETE
-- at the end of the block, leaving none behind for the next cycle.
-- ============================================================
BEGIN;
-- DELETE-INSERT (non-null value)
INSERT INTO single_unique_index_nulls_not_distinct (id, email) VALUES (:base + 85001, ('nnd1_user1@conflict.test' || :cycle));
DELETE FROM single_unique_index_nulls_not_distinct WHERE id = :base + 85001;
INSERT INTO single_unique_index_nulls_not_distinct (id, email) VALUES (:base + 85101, ('nnd1_user1@conflict.test' || :cycle));

-- NULL free->reuse: free the NULL by deleting base+85010, reuse NULL on base+85011
-- (conflict under NULLS NOT DISTINCT), then delete base+85011 to leave none behind.
INSERT INTO single_unique_index_nulls_not_distinct (id, email) VALUES (:base + 85010, NULL);
DELETE FROM single_unique_index_nulls_not_distinct WHERE id = :base + 85010;
INSERT INTO single_unique_index_nulls_not_distinct (id, email) VALUES (:base + 85011, NULL);
DELETE FROM single_unique_index_nulls_not_distinct WHERE id = :base + 85011;
COMMIT;

-- ============================================================
-- 11. multi_unique_index_nulls_not_distinct (id PK, UNIQUE INDEX(first_name, last_name) NULLS NOT DISTINCT)
--     custom-key candidate: (first_name, last_name) -- free/reuse rewritten to
--     DELETE-INSERT (never UPDATE), same reasoning as table 10.
-- ============================================================
BEGIN;
-- DELETE-INSERT (non-null values)
INSERT INTO multi_unique_index_nulls_not_distinct (id, first_name, last_name) VALUES (:base + 86001, ('nnd2First' || :cycle), ('nnd2Last' || :cycle));
DELETE FROM multi_unique_index_nulls_not_distinct WHERE id = :base + 86001;
INSERT INTO multi_unique_index_nulls_not_distinct (id, first_name, last_name) VALUES (:base + 86101, ('nnd2First' || :cycle), ('nnd2Last' || :cycle));

-- (NULL, NULL) free->reuse: two all-NULL rows conflict under NULLS NOT DISTINCT.
-- Cleared via DELETE at the end so none is left behind for the next cycle.
INSERT INTO multi_unique_index_nulls_not_distinct (id, first_name, last_name) VALUES (:base + 86010, NULL, NULL);
DELETE FROM multi_unique_index_nulls_not_distinct WHERE id = :base + 86010;
INSERT INTO multi_unique_index_nulls_not_distinct (id, first_name, last_name) VALUES (:base + 86011, NULL, NULL);
DELETE FROM multi_unique_index_nulls_not_distinct WHERE id = :base + 86011;
COMMIT;

-- ============================================================
-- 12. partitioned_unique_conflict (PK(id, region), UNIQUE INDEX(email, region), PARTITION BY LIST(region))
--     custom-key candidate: (region) -- already immutable as written: only
--     `email` is ever updated, region is fixed per row.
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
--     custom-key candidate: (email) -- the non-null free->reuse step is rewritten
--     to DELETE-INSERT (never UPDATE) so email is immutability-safe if picked.
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

-- Non-null value free->reuse (DELETE-INSERT): a real conflict that must still be detected here.
INSERT INTO single_unique_index_nulls_distinct (id, email) VALUES (:base + 89010, ('nd_user1@conflict.test' || :cycle));
DELETE FROM single_unique_index_nulls_distinct WHERE id = :base + 89010;
INSERT INTO single_unique_index_nulls_distinct (id, email) VALUES (:base + 89110, ('nd_user1@conflict.test' || :cycle));
COMMIT;
