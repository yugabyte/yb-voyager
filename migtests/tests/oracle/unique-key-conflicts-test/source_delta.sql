-- Conflicts for single_unique_constr
-- DELETE-INSERT conflict
DELETE FROM single_unique_constr WHERE id = 1;
INSERT INTO single_unique_constr (id, email) VALUES (6, 'user1@example.com');
COMMIT;

-- DELETE-UPDATE conflict
DELETE FROM single_unique_constr WHERE id = 2;
UPDATE single_unique_constr SET email = 'user2@example.com' WHERE id = 3;
COMMIT;

-- UPDATE-INSERT conflict
UPDATE single_unique_constr SET email = 'updated_user4@example.com' WHERE id = 4;
INSERT INTO single_unique_constr (id, email) VALUES (7, 'user4@example.com');
COMMIT;

-- UPDATE-UPDATE conflict
UPDATE single_unique_constr SET email = 'updated_user5@example.com' WHERE id = 5;
UPDATE single_unique_constr SET email = 'user5@example.com' WHERE id = 6;
COMMIT;

-- Conflicts for multi_unique_constr
-- DELETE-INSERT conflict
DELETE FROM multi_unique_constr WHERE id = 1;
INSERT INTO multi_unique_constr (id, first_name, last_name) VALUES (6, 'John', 'Doe');
COMMIT;

-- DELETE-UPDATE conflict
DELETE FROM multi_unique_constr WHERE id = 2;
UPDATE multi_unique_constr SET first_name = 'Jane', last_name = 'Smith' WHERE id = 4;
COMMIT;

-- UPDATE-INSERT conflict
UPDATE multi_unique_constr SET first_name = 'Updated_Tom' WHERE id = 5;
INSERT INTO multi_unique_constr (id, first_name, last_name) VALUES (7, 'Tom', 'Clark');
COMMIT;

-- UPDATE-UPDATE conflict
UPDATE multi_unique_constr SET first_name = 'Updated_Alice' WHERE id = 4;
UPDATE multi_unique_constr SET first_name = 'Alice', last_name = 'Williams' WHERE id = 5;
COMMIT;

-- Conflicts for single_unique_idx
-- DELETE-INSERT conflict
DELETE FROM single_unique_idx WHERE id = 1;
INSERT INTO single_unique_idx (id, ssn) VALUES (6, 'SSN1');
COMMIT;

-- DELETE-UPDATE conflict
DELETE FROM single_unique_idx WHERE id = 2;
UPDATE single_unique_idx SET ssn = 'SSN2' WHERE id = 3;
COMMIT;

-- UPDATE-INSERT conflict
UPDATE single_unique_idx SET ssn = 'updated_SSN4' WHERE id = 4;
INSERT INTO single_unique_idx (id, ssn) VALUES (7, 'SSN4');
COMMIT;

-- UPDATE-UPDATE conflict
UPDATE single_unique_idx SET ssn = 'updated_SSN5' WHERE id = 5;
UPDATE single_unique_idx SET ssn = 'SSN5' WHERE id = 6;
COMMIT;

-- Conflicts for multi_unique_idx
-- DELETE-INSERT conflict
DELETE FROM multi_unique_idx WHERE id = 1;
INSERT INTO multi_unique_idx (id, first_name, last_name) VALUES (6, 'John', 'Doe');
COMMIT;

-- DELETE-UPDATE conflict
DELETE FROM multi_unique_idx WHERE id = 2;
UPDATE multi_unique_idx SET first_name = 'Jane', last_name = 'Smith' WHERE id = 3;
COMMIT;

-- UPDATE-INSERT conflict
UPDATE multi_unique_idx SET first_name = 'Updated_Tom' WHERE id = 5;
INSERT INTO multi_unique_idx (id, first_name, last_name) VALUES (7, 'Tom', 'Clark');
COMMIT;

-- UPDATE-UPDATE conflict
UPDATE multi_unique_idx SET first_name = 'Updated_Alice' WHERE id = 4;
UPDATE multi_unique_idx SET first_name = 'Alice', last_name = 'Williams' WHERE id = 6;
COMMIT;

-- Conflicts for diff_columns_constr_idx
-- DELETE-INSERT conflict
DELETE FROM diff_columns_constr_idx WHERE id = 1;
INSERT INTO diff_columns_constr_idx (id, email, phone_number) VALUES (6, 'user1@example.com', '555-555-5560');
COMMIT; on email

-- DELETE-UPDATE conflict
DELETE FROM diff_columns_constr_idx WHERE id = 2;
UPDATE diff_columns_constr_idx SET phone_number = '555-555-5552' WHERE id = 3;
COMMIT; on phone_number

-- UPDATE-INSERT conflict
UPDATE diff_columns_constr_idx SET email = 'updated_user4@example.com' WHERE id = 4;
INSERT INTO diff_columns_constr_idx (id, email, phone_number) VALUES (7, 'user4@example.com', '555-555-5561');
COMMIT; on email

-- UPDATE-UPDATE conflict
UPDATE diff_columns_constr_idx SET phone_number = '555-555-5558' WHERE id = 5;
UPDATE diff_columns_constr_idx SET phone_number = '555-555-5555' WHERE id = 6;
COMMIT; on phone_number

-- Conflicts for subset_columns_constr_idx
-- DELETE-INSERT conflict
DELETE FROM subset_columns_constr_idx WHERE id = 1;
INSERT INTO subset_columns_constr_idx (id, first_name, last_name, phone_number) VALUES (6, 'John', 'Doe', '123-456-7890');
COMMIT;

-- DELETE-UPDATE conflict
DELETE FROM subset_columns_constr_idx WHERE id = 2;
UPDATE subset_columns_constr_idx SET first_name = 'Jane', last_name = 'Smith' WHERE id = 3;
COMMIT;

-- UPDATE-INSERT conflict
UPDATE subset_columns_constr_idx SET first_name = 'Updated_Bob' WHERE id = 3;
INSERT INTO subset_columns_constr_idx (id, first_name, last_name, phone_number) VALUES (7, 'Bob', 'Johnson', '123-456-7892');
COMMIT;

-- UPDATE-UPDATE conflict
UPDATE subset_columns_constr_idx SET first_name = 'Updated_Alice' WHERE id = 4;
UPDATE subset_columns_constr_idx SET first_name = 'Alice', last_name = 'Williams', phone_number = '123-456-7893' WHERE id = 5;
COMMIT;