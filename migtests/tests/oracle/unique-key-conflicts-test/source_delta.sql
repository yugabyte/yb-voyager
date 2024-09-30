-- Insert non-conflict rows for SINGLE_UNIQUE_CONSTR
INSERT INTO SINGLE_UNIQUE_CONSTR (ID, EMAIL) VALUES (10, 'non_conflict_user1@example.com');
INSERT INTO SINGLE_UNIQUE_CONSTR (ID, EMAIL) VALUES (11, 'non_conflict_user2@example.com');

-- 1. Conflicts for SINGLE_UNIQUE_CONSTR
-- DELETE-INSERT conflict
DELETE FROM SINGLE_UNIQUE_CONSTR WHERE id = 1;
INSERT INTO SINGLE_UNIQUE_CONSTR (id, email) VALUES (6, 'user1@example.com');

-- Non-conflict update and delete
UPDATE SINGLE_UNIQUE_CONSTR SET EMAIL = 'updated_non_conflict_user1@example.com' WHERE ID = 10;
DELETE FROM SINGLE_UNIQUE_CONSTR WHERE ID = 11;

-- DELETE-UPDATE conflict
DELETE FROM SINGLE_UNIQUE_CONSTR WHERE id = 2;
UPDATE SINGLE_UNIQUE_CONSTR SET email = 'user2@example.com' WHERE id = 3;

-- UPDATE-INSERT conflict
UPDATE SINGLE_UNIQUE_CONSTR SET email = 'updated_user4@example.com' WHERE id = 4;
INSERT INTO SINGLE_UNIQUE_CONSTR (id, email) VALUES (7, 'user4@example.com');

-- UPDATE-UPDATE conflict
UPDATE SINGLE_UNIQUE_CONSTR SET email = 'updated_user5@example.com' WHERE id = 5;
UPDATE SINGLE_UNIQUE_CONSTR SET email = 'user5@example.com' WHERE id = 6;
COMMIT;



-- Insert non-conflict rows for MULTI_UNIQUE_CONSTR
INSERT INTO MULTI_UNIQUE_CONSTR (ID, FIRST_NAME, LAST_NAME) VALUES (10, 'Non_John', 'Non_Doe');
INSERT INTO MULTI_UNIQUE_CONSTR (ID, FIRST_NAME, LAST_NAME) VALUES (11, 'Non_Jane', 'Non_Smith');

-- 2. Conflicts for MULTI_UNIQUE_CONSTR
-- DELETE-INSERT conflict
DELETE FROM MULTI_UNIQUE_CONSTR WHERE id = 1;
INSERT INTO MULTI_UNIQUE_CONSTR (id, first_name, last_name) VALUES (6, 'John', 'Doe');

-- Non-conflict update and delete
UPDATE MULTI_UNIQUE_CONSTR SET FIRST_NAME = 'Updated_Non_John', LAST_NAME = 'Updated_Non_Doe' WHERE ID = 10;
DELETE FROM MULTI_UNIQUE_CONSTR WHERE ID = 11;

-- DELETE-UPDATE conflict
DELETE FROM MULTI_UNIQUE_CONSTR WHERE id = 2;
UPDATE MULTI_UNIQUE_CONSTR SET first_name = 'Jane', last_name = 'Smith' WHERE id = 4;

-- UPDATE-INSERT conflict
UPDATE MULTI_UNIQUE_CONSTR SET first_name = 'Updated_Tom' WHERE id = 5;
INSERT INTO MULTI_UNIQUE_CONSTR (id, first_name, last_name) VALUES (7, 'Tom', 'Clark');

-- UPDATE-UPDATE conflict
UPDATE MULTI_UNIQUE_CONSTR SET first_name = 'Updated_Alice' WHERE id = 4;
UPDATE MULTI_UNIQUE_CONSTR SET first_name = 'Alice', last_name = 'Williams' WHERE id = 5;
COMMIT;



-- Insert non-conflict rows for SINGLE_UNIQUE_IDX
INSERT INTO SINGLE_UNIQUE_IDX (ID, SSN) VALUES (10, 'non_conflict_SSN1');
INSERT INTO SINGLE_UNIQUE_IDX (ID, SSN) VALUES (11, 'non_conflict_SSN2');

-- 3. Conflicts for SINGLE_UNIQUE_IDX
-- DELETE-INSERT conflict
DELETE FROM SINGLE_UNIQUE_IDX WHERE id = 1;
INSERT INTO SINGLE_UNIQUE_IDX (id, ssn) VALUES (6, 'SSN1');

-- Non-conflict update and delete
UPDATE SINGLE_UNIQUE_IDX SET SSN = 'updated_non_conflict_SSN' WHERE ID = 10;
DELETE FROM SINGLE_UNIQUE_IDX WHERE ID = 11;

-- DELETE-UPDATE conflict
DELETE FROM SINGLE_UNIQUE_IDX WHERE id = 2;
UPDATE SINGLE_UNIQUE_IDX SET ssn = 'SSN2' WHERE id = 3;

-- UPDATE-INSERT conflict
UPDATE SINGLE_UNIQUE_IDX SET ssn = 'updated_SSN4' WHERE id = 4;
INSERT INTO SINGLE_UNIQUE_IDX (id, ssn) VALUES (7, 'SSN4');

-- UPDATE-UPDATE conflict
UPDATE SINGLE_UNIQUE_IDX SET ssn = 'updated_SSN5' WHERE id = 5;
UPDATE SINGLE_UNIQUE_IDX SET ssn = 'SSN5' WHERE id = 6;
COMMIT;



-- Insert non-conflict rows for MULTI_UNIQUE_IDX
INSERT INTO MULTI_UNIQUE_IDX (ID, FIRST_NAME, LAST_NAME) VALUES (10, 'Non_John', 'Non_Doe');
INSERT INTO MULTI_UNIQUE_IDX (ID, FIRST_NAME, LAST_NAME) VALUES (11, 'Non_Jane', 'Non_Smith');

-- 4. Conflicts for MULTI_UNIQUE_IDX
-- DELETE-INSERT conflict
DELETE FROM MULTI_UNIQUE_IDX WHERE id = 1;
INSERT INTO MULTI_UNIQUE_IDX (id, first_name, last_name) VALUES (6, 'John', 'Doe');

-- Non-conflict update and delete
UPDATE MULTI_UNIQUE_IDX SET FIRST_NAME = 'Updated_Non_John', LAST_NAME = 'Updated_Non_Doe' WHERE ID = 10;
DELETE FROM MULTI_UNIQUE_IDX WHERE ID = 11;

-- DELETE-UPDATE conflict
DELETE FROM MULTI_UNIQUE_IDX WHERE id = 2;
UPDATE MULTI_UNIQUE_IDX SET first_name = 'Jane', last_name = 'Smith' WHERE id = 3;

-- UPDATE-INSERT conflict
UPDATE MULTI_UNIQUE_IDX SET first_name = 'Updated_Tom' WHERE id = 5;
INSERT INTO MULTI_UNIQUE_IDX (id, first_name, last_name) VALUES (7, 'Tom', 'Clark');

-- UPDATE-UPDATE conflict
UPDATE MULTI_UNIQUE_IDX SET first_name = 'Updated_Alice' WHERE id = 4;
UPDATE MULTI_UNIQUE_IDX SET first_name = 'Alice', last_name = 'Williams' WHERE id = 6;
COMMIT;



-- Insert non-conflict rows for DIFF_COLUMNS_CONSTR_IDX
INSERT INTO DIFF_COLUMNS_CONSTR_IDX (id, email, phone_number) VALUES (10, 'non_conflict_user1@example.com', '555-555-5566');
INSERT INTO DIFF_COLUMNS_CONSTR_IDX (id, email, phone_number) VALUES (11, 'non_conflict_user2@example.com', '555-555-5567');

-- 5. Conflicts for DIFF_COLUMNS_CONSTR_IDX
-- DELETE-INSERT conflict
DELETE FROM DIFF_COLUMNS_CONSTR_IDX WHERE id = 1;
INSERT INTO DIFF_COLUMNS_CONSTR_IDX (id, email, phone_number) VALUES (6, 'user1@example.com', '555-555-5560');

-- Non-conflict update and delete
UPDATE DIFF_COLUMNS_CONSTR_IDX SET email = 'non_conflict_updated_user1@example.com' WHERE id = 10;
DELETE FROM DIFF_COLUMNS_CONSTR_IDX WHERE id = 11;

-- DELETE-UPDATE conflict
DELETE FROM DIFF_COLUMNS_CONSTR_IDX WHERE id = 2;
UPDATE DIFF_COLUMNS_CONSTR_IDX SET phone_number = '555-555-5552' WHERE id = 3;

-- UPDATE-INSERT conflict
UPDATE DIFF_COLUMNS_CONSTR_IDX SET email = 'updated_user4@example.com' WHERE id = 4;
INSERT INTO DIFF_COLUMNS_CONSTR_IDX (id, email, phone_number) VALUES (7, 'user4@example.com', '555-555-5561');

-- UPDATE-UPDATE conflict
UPDATE DIFF_COLUMNS_CONSTR_IDX SET phone_number = '555-555-5558' WHERE id = 5;
UPDATE DIFF_COLUMNS_CONSTR_IDX SET phone_number = '555-555-5555' WHERE id = 6;
COMMIT;



-- Insert non-conflict rows for SUBSET_COLUMNS_CONSTR_IDX
INSERT INTO SUBSET_COLUMNS_CONSTR_IDX (id, first_name, last_name, phone_number) VALUES (10, 'Non_John', 'Non_Doe', '123-456-7890');
INSERT INTO SUBSET_COLUMNS_CONSTR_IDX (id, first_name, last_name, phone_number) VALUES (11, 'Non_Jane', 'Non_Smith', '123-456-7891');

-- 6. Conflicts for SUBSET_COLUMNS_CONSTR_IDX
-- DELETE-INSERT conflict
DELETE FROM SUBSET_COLUMNS_CONSTR_IDX WHERE id = 1;
INSERT INTO SUBSET_COLUMNS_CONSTR_IDX (id, first_name, last_name, phone_number) VALUES (6, 'John', 'Doe', '123-456-7890');

-- Non-conflict update and delete
UPDATE SUBSET_COLUMNS_CONSTR_IDX SET first_name = 'Updated_Non_John', last_name = 'Updated_Non_Doe', phone_number = '123-456-7890' WHERE id = 10;
DELETE FROM SUBSET_COLUMNS_CONSTR_IDX WHERE id = 11;

-- DELETE-UPDATE conflict
DELETE FROM SUBSET_COLUMNS_CONSTR_IDX WHERE id = 2;
UPDATE SUBSET_COLUMNS_CONSTR_IDX SET first_name = 'Jane', last_name = 'Smith' WHERE id = 3;

-- UPDATE-INSERT conflict
UPDATE SUBSET_COLUMNS_CONSTR_IDX SET first_name = 'Updated_Bob' WHERE id = 3;
INSERT INTO SUBSET_COLUMNS_CONSTR_IDX (id, first_name, last_name, phone_number) VALUES (7, 'Bob', 'Johnson', '123-456-7892');

-- UPDATE-UPDATE conflict
UPDATE SUBSET_COLUMNS_CONSTR_IDX SET first_name = 'Updated_Alice' WHERE id = 4;
UPDATE SUBSET_COLUMNS_CONSTR_IDX SET first_name = 'Alice', last_name = 'Williams', phone_number = '123-456-7893' WHERE id = 5;
COMMIT;
