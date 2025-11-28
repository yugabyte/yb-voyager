-- Insert non-conflict rows for single_unique_constraint
INSERT INTO single_unique_constraint (id, email) VALUES (10, 'non_conflict_user1@example.com');
INSERT INTO single_unique_constraint (id, email) VALUES (11, 'non_conflict_user2@example.com');

-- 1. Conflicts for single_unique_constraint
-- DELETE-INSERT conflict
DELETE FROM single_unique_constraint WHERE id = 1;
INSERT INTO single_unique_constraint (id, email) VALUES (6, 'user1@example.com');

-- Non-conflict update and delete
UPDATE single_unique_constraint SET email = 'updated_non_conflict_user1@example.com' WHERE id = 10;
DELETE FROM single_unique_constraint WHERE id = 11;


-- DELETE-UPDATE conflict
DELETE FROM single_unique_constraint WHERE id = 2;
UPDATE single_unique_constraint SET email = 'user2@example.com' WHERE id = 3;

-- UPDATE-INSERT conflict
UPDATE single_unique_constraint SET email = 'updated_user4@example.com' WHERE id = 4;
INSERT INTO single_unique_constraint (id, email) VALUES (7, 'user4@example.com');

-- UPDATE-UPDATE conflict
UPDATE single_unique_constraint SET email = 'updated_user5@example.com' WHERE id = 5;
UPDATE single_unique_constraint SET email = 'user5@example.com' WHERE id = 6;



-- Insert non-conflict rows for multi_unique_constraint
INSERT INTO multi_unique_constraint (id, first_name, last_name) VALUES (10, 'Non_John', 'Non_Doe');
INSERT INTO multi_unique_constraint (id, first_name, last_name) VALUES (11, 'Non_Jane', 'Non_Smith');

-- 2. Conflicts for multi_unique_constraint
-- DELETE-INSERT conflict
DELETE FROM multi_unique_constraint WHERE id = 1;
INSERT INTO multi_unique_constraint (id, first_name, last_name) VALUES (6, 'John', 'Doe');


-- Non-conflict update and delete
UPDATE multi_unique_constraint SET first_name = 'Updated_Non_John', last_name = 'Updated_Non_Doe' WHERE id = 10;
DELETE FROM multi_unique_constraint WHERE id = 11;


-- DELETE-UPDATE conflict
DELETE FROM multi_unique_constraint WHERE id = 2;
UPDATE multi_unique_constraint SET first_name = 'Jane', last_name = 'Smith' WHERE id = 4;
-- UPDATE-INSERT conflict
UPDATE multi_unique_constraint SET first_name = 'Updated_Tom' WHERE id = 5;
INSERT INTO multi_unique_constraint (id, first_name, last_name) VALUES (7, 'Tom', 'Clark');

-- UPDATE-UPDATE conflict
UPDATE multi_unique_constraint SET first_name = 'Updated_Alice' WHERE id = 4;
UPDATE multi_unique_constraint SET first_name = 'Alice', last_name = 'Williams' WHERE id = 5;



-- Insert non-conflict rows for same_column_unique_constraint_and_index
INSERT INTO same_column_unique_constraint_and_index (id, email) VALUES (10, 'non_conflict_user1@example.com');
INSERT INTO same_column_unique_constraint_and_index (id, email) VALUES (11, 'non_conflict_user2@example.com');

-- 3. Conflicts for same_column_unique_constraint_and_index
-- DELETE-INSERT conflict
DELETE FROM same_column_unique_constraint_and_index WHERE id = 1;
INSERT INTO same_column_unique_constraint_and_index (id, email) VALUES (6, 'user1@example.com');


-- Non-conflict update and delete
UPDATE same_column_unique_constraint_and_index SET email = 'updated_non_conflict_user1@example.com' WHERE id = 10;
DELETE FROM same_column_unique_constraint_and_index WHERE id = 11;


-- DELETE-UPDATE conflict
DELETE FROM same_column_unique_constraint_and_index WHERE id = 2;
UPDATE same_column_unique_constraint_and_index SET email = 'user2@example.com' WHERE id = 3;
-- UPDATE-INSERT conflict
UPDATE same_column_unique_constraint_and_index SET email = 'updated_user4@example.com' WHERE id = 4;
INSERT INTO same_column_unique_constraint_and_index (id, email) VALUES (7, 'user4@example.com');

-- UPDATE-UPDATE conflict
UPDATE same_column_unique_constraint_and_index SET email = 'updated_user5@example.com' WHERE id = 5;
UPDATE same_column_unique_constraint_and_index SET email = 'user5@example.com' WHERE id = 6;


-- Insert non-conflict rows for single_unique_index
INSERT INTO single_unique_index (id, ssn) VALUES (10, 'non_conflict_ssn1');
INSERT INTO single_unique_index (id, ssn) VALUES (11, 'non_conflict_ssn2');

-- 4. Conflicts for single_unique_index
-- DELETE-INSERT conflict
DELETE FROM single_unique_index WHERE id = 1;
INSERT INTO single_unique_index (id, ssn) VALUES (6, 'SSN1');


-- Non-conflict update and delete
UPDATE single_unique_index SET ssn = 'updated_non_conflict_ssn1' WHERE id = 10;
DELETE FROM single_unique_index WHERE id = 11;


-- DELETE-UPDATE conflict
DELETE FROM single_unique_index WHERE id = 2;
UPDATE single_unique_index SET ssn = 'SSN2' WHERE id = 3;
-- UPDATE-INSERT conflict
UPDATE single_unique_index SET ssn = 'updated_SSN4' WHERE id = 4;
INSERT INTO single_unique_index (id, ssn) VALUES (7, 'SSN4');

-- UPDATE-UPDATE conflict
UPDATE single_unique_index SET ssn = 'updated_SSN5' WHERE id = 5;
UPDATE single_unique_index SET ssn = 'SSN5' WHERE id = 6;


-- Insert non-conflict rows for multi_unique_index
INSERT INTO multi_unique_index (id, first_name, last_name) VALUES (10, 'Non_John', 'Non_Doe');
INSERT INTO multi_unique_index (id, first_name, last_name) VALUES (11, 'Non_Jane', 'Non_Smith');

-- 5. Conflicts for multi_unique_index
-- DELETE-INSERT conflict
DELETE FROM multi_unique_index WHERE id = 1;
INSERT INTO multi_unique_index (id, first_name, last_name) VALUES (6, 'John', 'Doe');


-- Non-conflict update and delete
UPDATE multi_unique_index SET first_name = 'Updated_Non_John', last_name = 'Updated_Non_Doe' WHERE id = 10;
DELETE FROM multi_unique_index WHERE id = 11;


-- DELETE-UPDATE conflict
DELETE FROM multi_unique_index WHERE id = 2;
UPDATE multi_unique_index SET first_name = 'Jane', last_name = 'Smith' WHERE id = 3;
-- UPDATE-INSERT conflict
UPDATE multi_unique_index SET first_name = 'Updated_Tom' WHERE id = 5;
INSERT INTO multi_unique_index (id, first_name, last_name) VALUES (7, 'Tom', 'Clark');

-- UPDATE-UPDATE conflict
UPDATE multi_unique_index SET first_name = 'Updated_Alice' WHERE id = 4;
UPDATE multi_unique_index SET first_name = 'Alice', last_name = 'Williams' WHERE id = 6;


-- Insert non-conflict rows for different_columns_unique_constraint_and_index
INSERT INTO different_columns_unique_constraint_and_index (id, email, phone_number) VALUES (10, 'non_conflict_user1@example.com', '555-555-5566');
INSERT INTO different_columns_unique_constraint_and_index (id, email, phone_number) VALUES (11, 'non_conflict_user2@example.com', '555-555-5567');

-- 6. Conflicts for different_columns_unique_constraint_and_index
-- TODO: one event with multiple conflicts, what if both Uniq Const and Uniq Index are changed in one event
-- DELETE-INSERT conflict
DELETE FROM different_columns_unique_constraint_and_index WHERE id = 1;
INSERT INTO different_columns_unique_constraint_and_index (id, email, phone_number) VALUES (6, 'user1@example.com', '555-555-5560');


-- Non-conflict update and delete
UPDATE different_columns_unique_constraint_and_index SET email = 'non_conflict_updated_user1@example.com' WHERE id = 10;
DELETE FROM different_columns_unique_constraint_and_index WHERE id = 11;


-- DELETE-UPDATE conflict
DELETE FROM different_columns_unique_constraint_and_index WHERE id = 2;
UPDATE different_columns_unique_constraint_and_index SET phone_number = '555-555-5552' WHERE id = 3;
-- UPDATE-INSERT conflict
UPDATE different_columns_unique_constraint_and_index SET email = 'updated_user4@example.com' WHERE id = 4;
INSERT INTO different_columns_unique_constraint_and_index (id, email, phone_number) VALUES (7, 'user4@example.com', '555-555-5561');

-- UPDATE-UPDATE conflict
UPDATE different_columns_unique_constraint_and_index SET phone_number = '555-555-5558' WHERE id = 5;
UPDATE different_columns_unique_constraint_and_index SET phone_number = '555-555-5555' WHERE id = 6;

-- Insert non-conflict rows for subset_columns_unique_constraint_and_index
INSERT INTO subset_columns_unique_constraint_and_index (id, first_name, last_name, phone_number) VALUES (10, 'Non_John', 'Non_Doe', '123-456-7890');
INSERT INTO subset_columns_unique_constraint_and_index (id, first_name, last_name, phone_number) VALUES (11, 'Non_Jane', 'Non_Smith', '123-456-7891');

-- 7. Conflicts for subset_columns_unique_constraint_and_index
-- PG allows unique constraint and unique index on same column(s)
-- DELETE-INSERT conflict
DELETE FROM subset_columns_unique_constraint_and_index WHERE id = 1;
INSERT INTO subset_columns_unique_constraint_and_index (id, first_name, last_name, phone_number) VALUES (6, 'John', 'Doe', '123-456-7890');


-- Non-conflict update and delete
UPDATE subset_columns_unique_constraint_and_index SET first_name = 'Updated_Non_John', last_name = 'Updated_Non_Doe', phone_number = '123-456-7890' WHERE id = 10;
DELETE FROM subset_columns_unique_constraint_and_index WHERE id = 11;


-- DELETE-UPDATE conflict
DELETE FROM subset_columns_unique_constraint_and_index WHERE id = 2;
UPDATE subset_columns_unique_constraint_and_index SET first_name = 'Jane', last_name = 'Smith' WHERE id = 3;
-- UPDATE-INSERT conflict
UPDATE subset_columns_unique_constraint_and_index SET first_name = 'Updated_Bob' WHERE id = 3;
INSERT INTO subset_columns_unique_constraint_and_index (id, first_name, last_name, phone_number) VALUES (7, 'Bob', 'Johnson', '123-456-7892');

-- UPDATE-UPDATE conflict
UPDATE subset_columns_unique_constraint_and_index SET first_name = 'Updated_Alice' WHERE id = 4;
UPDATE subset_columns_unique_constraint_and_index SET first_name = 'Alice', last_name = 'Williams', phone_number = '123-456-7893' WHERE id = 5;

UPDATE expression_based_unique_index SET email = 'updated_user4@example.com' WHERE id = 4;
INSERT INTO expression_based_unique_index (email) VALUES ('user4@example.com'); --conflict lower case email 

-- UPDATE-UPDATE conflict
UPDATE expression_based_unique_index SET email = 'updated_user5@example.com' WHERE id = 5;
UPDATE expression_based_unique_index SET email = 'user5@example.com' WHERE id = 6;