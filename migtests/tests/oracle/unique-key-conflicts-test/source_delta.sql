-- Conflicts for single_unique_constraint
DELETE FROM single_unique_constraint WHERE id = 1;
INSERT INTO single_unique_constraint (id, email) VALUES (6, 'user1@example.com');  -- DELETE-INSERT conflict

DELETE FROM single_unique_constraint WHERE id = 2;
UPDATE single_unique_constraint SET email = 'user2@example.com' WHERE id = 3;  -- DELETE-UPDATE conflict

UPDATE single_unique_constraint SET email = 'updated_user4@example.com' WHERE id = 4;
INSERT INTO single_unique_constraint (id, email) VALUES (7, 'user4@example.com');  -- UPDATE-INSERT conflict

UPDATE single_unique_constraint SET email = 'updated_user5@example.com' WHERE id = 5;
UPDATE single_unique_constraint SET email = 'user5@example.com' WHERE id = 6;  -- UPDATE-UPDATE conflict

-- Conflicts for multi_unique_constraint
DELETE FROM multi_unique_constraint WHERE id = 1;
INSERT INTO multi_unique_constraint (id, first_name, last_name) VALUES (6, 'John', 'Doe');  -- DELETE-INSERT conflict

DELETE FROM multi_unique_constraint WHERE id = 2;
UPDATE multi_unique_constraint SET first_name = 'Jane', last_name = 'Smith' WHERE id = 4;  -- DELETE-UPDATE conflict

UPDATE multi_unique_constraint SET first_name = 'Updated_Tom' WHERE id = 5;
INSERT INTO multi_unique_constraint (id, first_name, last_name) VALUES (7, 'Tom', 'Clark');  -- UPDATE-INSERT conflict

UPDATE multi_unique_constraint SET first_name = 'Updated_Alice' WHERE id = 4;
UPDATE multi_unique_constraint SET first_name = 'Alice', last_name = 'Williams' WHERE id = 5;  -- UPDATE-UPDATE conflict

-- Conflicts for single_unique_index
DELETE FROM single_unique_index WHERE id = 1;
INSERT INTO single_unique_index (id, ssn) VALUES (6, 'SSN1');  -- DELETE-INSERT conflict

DELETE FROM single_unique_index WHERE id = 2;
UPDATE single_unique_index SET ssn = 'SSN2' WHERE id = 3;  -- DELETE-UPDATE conflict

UPDATE single_unique_index SET ssn = 'updated_SSN4' WHERE id = 4;
INSERT INTO single_unique_index (id, ssn) VALUES (7, 'SSN4');  -- UPDATE-INSERT conflict

UPDATE single_unique_index SET ssn = 'updated_SSN5' WHERE id = 5;
UPDATE single_unique_index SET ssn = 'SSN5' WHERE id = 6;  -- UPDATE-UPDATE conflict

-- Conflicts for multi_unique_index
DELETE FROM multi_unique_index WHERE id = 1;
INSERT INTO multi_unique_index (id, first_name, last_name) VALUES (6, 'John', 'Doe');  -- DELETE-INSERT conflict

DELETE FROM multi_unique_index WHERE id = 2;
UPDATE multi_unique_index SET first_name = 'Jane', last_name = 'Smith' WHERE id = 3;  -- DELETE-UPDATE conflict

UPDATE multi_unique_index SET first_name = 'Updated_Tom' WHERE id = 5;
INSERT INTO multi_unique_index (id, first_name, last_name) VALUES (7, 'Tom', 'Clark');  -- UPDATE-INSERT conflict

UPDATE multi_unique_index SET first_name = 'Updated_Alice' WHERE id = 4;
UPDATE multi_unique_index SET first_name = 'Alice', last_name = 'Williams' WHERE id = 6;  -- UPDATE-UPDATE conflict

-- Conflicts for different_columns_unique_constraint_and_index
DELETE FROM different_columns_unique_constraint_and_index WHERE id = 1;
INSERT INTO different_columns_unique_constraint_and_index (id, email, phone_number) VALUES (6, 'user1@example.com', '555-555-5560');  -- DELETE-INSERT conflict on email

DELETE FROM different_columns_unique_constraint_and_index WHERE id = 2;
UPDATE different_columns_unique_constraint_and_index SET phone_number = '555-555-5552' WHERE id = 3;  -- DELETE-UPDATE conflict on phone_number

UPDATE different_columns_unique_constraint_and_index SET email = 'updated_user4@example.com' WHERE id = 4;
INSERT INTO different_columns_unique_constraint_and_index (id, email, phone_number) VALUES (7, 'user4@example.com', '555-555-5561');  -- UPDATE-INSERT conflict on email

UPDATE different_columns_unique_constraint_and_index SET phone_number = '555-555-5558' WHERE id = 5;
UPDATE different_columns_unique_constraint_and_index SET phone_number = '555-555-5555' WHERE id = 6;  -- UPDATE-UPDATE conflict on phone_number

-- Conflicts for subset_columns_unique_constraint_and_index
DELETE FROM subset_columns_unique_constraint_and_index WHERE id = 1;
INSERT INTO subset_columns_unique_constraint_and_index (id, first_name, last_name, phone_number) VALUES (6, 'John', 'Doe', '123-456-7890');  -- DELETE-INSERT conflict

DELETE FROM subset_columns_unique_constraint_and_index WHERE id = 2;
UPDATE subset_columns_unique_constraint_and_index SET first_name = 'Jane', last_name = 'Smith' WHERE id = 3;  -- DELETE-UPDATE conflict

UPDATE subset_columns_unique_constraint_and_index SET first_name = 'Updated_Bob' WHERE id = 3;
INSERT INTO subset_columns_unique_constraint_and_index (id, first_name, last_name, phone_number) VALUES (7, 'Bob', 'Johnson', '123-456-7892');  -- UPDATE-INSERT conflict

UPDATE subset_columns_unique_constraint_and_index SET first_name = 'Updated_Alice' WHERE id = 4;
UPDATE subset_columns_unique_constraint_and_index SET first_name = 'Alice', last_name = 'Williams', phone_number = '123-456-7893' WHERE id = 5;  -- UPDATE-UPDATE conflict