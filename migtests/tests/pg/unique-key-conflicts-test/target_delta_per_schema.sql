
-- Initial data for `single_unique_constraint`
INSERT INTO single_unique_constraint (id, email) VALUES (101, 'target_user101@example.com');
INSERT INTO single_unique_constraint (id, email) VALUES (102, 'target_user102@example.com');
INSERT INTO single_unique_constraint (id, email) VALUES (103, 'target_user103@example.com');
INSERT INTO single_unique_constraint (id, email) VALUES (104, 'target_user104@example.com');
INSERT INTO single_unique_constraint (id, email) VALUES (105, 'target_user105@example.com');

-- 1. Conflicts for `single_unique_constraint`
-- DELETE-INSERT conflict
DELETE FROM single_unique_constraint WHERE id = 101;
INSERT INTO single_unique_constraint (id, email) VALUES (106, 'target_user101@example.com');

-- Non-conflict update and delete
UPDATE single_unique_constraint SET email = 'updated_target_user102@example.com' WHERE id = 102;
DELETE FROM single_unique_constraint WHERE id = 103;

-- DELETE-UPDATE conflict
DELETE FROM single_unique_constraint WHERE id = 104;
UPDATE single_unique_constraint SET email = 'target_user104@example.com' WHERE id = 105;

-- UPDATE-INSERT conflict
UPDATE single_unique_constraint SET email = 'updated_target_user105@example.com' WHERE id = 105;
INSERT INTO single_unique_constraint (id, email) VALUES (107, 'target_user105@example.com');

-- UPDATE-UPDATE conflict
-- Insert initial data
INSERT INTO single_unique_constraint (id, email) VALUES (108, 'target_user106@example.com');
INSERT INTO single_unique_constraint (id, email) VALUES (109, 'target_user107@example.com');

UPDATE single_unique_constraint SET email = 'updated_target_user106@example.com' WHERE id = 108;
UPDATE single_unique_constraint SET email = 'target_user106@example.com' WHERE id = 109;








-- Insert initial data for `multi_unique_constraint`
INSERT INTO multi_unique_constraint (id, first_name, last_name) VALUES (101, 'Target_John', 'Doe');
INSERT INTO multi_unique_constraint (id, first_name, last_name) VALUES (102, 'Target_Jane', 'Smith');
INSERT INTO multi_unique_constraint (id, first_name, last_name) VALUES (103, 'Target_Tom', 'Clark');
INSERT INTO multi_unique_constraint (id, first_name, last_name) VALUES (104, 'Target_Alice', 'Williams');
INSERT INTO multi_unique_constraint (id, first_name, last_name) VALUES (105, 'Target_Bob', 'Johnson');

-- 2. Conflicts for `multi_unique_constraint`
-- DELETE-INSERT conflict
DELETE FROM multi_unique_constraint WHERE id = 101;
INSERT INTO multi_unique_constraint (id, first_name, last_name) VALUES (106, 'Target_John', 'Doe');

-- Non-conflict update and delete
UPDATE multi_unique_constraint SET first_name = 'Updated_Target_Jane', last_name = 'Updated_Smith' WHERE id = 102;
DELETE FROM multi_unique_constraint WHERE id = 103;

-- DELETE-UPDATE conflict
DELETE FROM multi_unique_constraint WHERE id = 104;
UPDATE multi_unique_constraint SET first_name = 'Target_Alice', last_name = 'Williams' WHERE id = 105;

-- UPDATE-INSERT conflict
UPDATE multi_unique_constraint SET first_name = 'Updated_Target_Bob' WHERE id = 105;
INSERT INTO multi_unique_constraint (id, first_name, last_name) VALUES (107, 'Target_Bob', 'Johnson');

-- UPDATE-UPDATE conflict
-- Insert initial data
INSERT INTO multi_unique_constraint (id, first_name, last_name) VALUES (108, 'Target_Dave', 'Wilson');
INSERT INTO multi_unique_constraint (id, first_name, last_name) VALUES (109, 'Target_Eve', 'Davis');

UPDATE multi_unique_constraint SET first_name = 'Updated_Target_Dave' WHERE id = 108;
UPDATE multi_unique_constraint SET first_name = 'Target_Dave', last_name = 'Wilson' WHERE id = 109;








-- Initial data for `same_column_unique_constraint_and_index`
INSERT INTO same_column_unique_constraint_and_index (id, email) VALUES (101, 'target_user101@example.com');
INSERT INTO same_column_unique_constraint_and_index (id, email) VALUES (102, 'target_user102@example.com');
INSERT INTO same_column_unique_constraint_and_index (id, email) VALUES (103, 'target_user103@example.com');
INSERT INTO same_column_unique_constraint_and_index (id, email) VALUES (104, 'target_user104@example.com');
INSERT INTO same_column_unique_constraint_and_index (id, email) VALUES (105, 'target_user105@example.com');

-- 3. Conflicts for `same_column_unique_constraint_and_index`
-- DELETE-INSERT conflict
DELETE FROM same_column_unique_constraint_and_index WHERE id = 101;
INSERT INTO same_column_unique_constraint_and_index (id, email) VALUES (106, 'target_user101@example.com');

-- Non-conflict update and delete
UPDATE same_column_unique_constraint_and_index SET email = 'updated_target_user102@example.com' WHERE id = 102;
DELETE FROM same_column_unique_constraint_and_index WHERE id = 103;

-- DELETE-UPDATE conflict
DELETE FROM same_column_unique_constraint_and_index WHERE id = 104;
UPDATE same_column_unique_constraint_and_index SET email = 'target_user104@example.com' WHERE id = 105;

-- UPDATE-INSERT conflict
UPDATE same_column_unique_constraint_and_index SET email = 'updated_target_user105@example.com' WHERE id = 105;
INSERT INTO same_column_unique_constraint_and_index (id, email) VALUES (107, 'target_user105@example.com');

-- UPDATE-UPDATE conflict
-- Insert initial data
INSERT INTO same_column_unique_constraint_and_index (id, email) VALUES (108, 'target_user106@example.com');
INSERT INTO same_column_unique_constraint_and_index (id, email) VALUES (109, 'target_user107@example.com');

UPDATE same_column_unique_constraint_and_index SET email = 'updated_target_user106@example.com' WHERE id = 108;
UPDATE same_column_unique_constraint_and_index SET email = 'target_user106@example.com' WHERE id = 109;







-- Initial data for `single_unique_index`
INSERT INTO single_unique_index (id, "Ssn") VALUES (101, 'target_ssn101');
INSERT INTO single_unique_index (id, "Ssn") VALUES (102, 'target_ssn102');
INSERT INTO single_unique_index (id, "Ssn") VALUES (103, 'target_ssn103');
INSERT INTO single_unique_index (id, "Ssn") VALUES (104, 'target_ssn104');
INSERT INTO single_unique_index (id, "Ssn") VALUES (105, 'target_ssn105');

-- 4. Conflicts for `single_unique_index`
-- DELETE-INSERT conflict
DELETE FROM single_unique_index WHERE id = 101;
INSERT INTO single_unique_index (id, "Ssn") VALUES (106, 'target_ssn101');

-- Non-conflict update and delete
UPDATE single_unique_index SET "Ssn" = 'updated_target_ssn102' WHERE id = 102;
DELETE FROM single_unique_index WHERE id = 103;

-- DELETE-UPDATE conflict
DELETE FROM single_unique_index WHERE id = 104;
UPDATE single_unique_index SET "Ssn" = 'target_ssn104' WHERE id = 105;

-- UPDATE-INSERT conflict
UPDATE single_unique_index SET "Ssn" = 'updated_target_ssn105' WHERE id = 105;
INSERT INTO single_unique_index (id, "Ssn") VALUES (107, 'target_ssn105');

-- UPDATE-UPDATE conflict
-- Insert initial data
INSERT INTO single_unique_index (id, "Ssn") VALUES (108, 'target_ssn106');
INSERT INTO single_unique_index (id, "Ssn") VALUES (109, 'target_ssn107');

UPDATE single_unique_index SET "Ssn" = 'updated_target_ssn106' WHERE id = 108;
UPDATE single_unique_index SET "Ssn" = 'target_ssn106' WHERE id = 109;






-- Initial data for `multi_unique_index`
INSERT INTO multi_unique_index (id, first_name, last_name) VALUES (101, 'Target_Jane', 'Smith');
INSERT INTO multi_unique_index (id, first_name, last_name) VALUES (102, 'Target_Tom', 'Clark');
INSERT INTO multi_unique_index (id, first_name, last_name) VALUES (103, 'Target_Alice', 'Williams');
INSERT INTO multi_unique_index (id, first_name, last_name) VALUES (104, 'Target_Bob', 'Johnson');
INSERT INTO multi_unique_index (id, first_name, last_name) VALUES (105, 'Target_Carol', 'Brown');

-- 5. Conflicts for `multi_unique_index`
-- DELETE-INSERT conflict
DELETE FROM multi_unique_index WHERE id = 101;
INSERT INTO multi_unique_index (id, first_name, last_name) VALUES (106, 'Target_Jane', 'Smith');  -- Reusing deleted names

-- Non-conflict update and delete
UPDATE multi_unique_index SET first_name = 'Updated_Target_Tom', last_name = 'Updated_Clark' WHERE id = 102;  -- Non-conflict update
DELETE FROM multi_unique_index WHERE id = 103;  -- Non-conflict delete

-- DELETE-UPDATE conflict
DELETE FROM multi_unique_index WHERE id = 104;
UPDATE multi_unique_index SET first_name = 'Target_Bob', last_name = 'Johnson' WHERE id = 105;  -- Using deleted names

-- UPDATE-INSERT conflict
UPDATE multi_unique_index SET first_name = 'Updated_Target_Carol' WHERE id = 105;
INSERT INTO multi_unique_index (id, first_name, last_name) VALUES (107, 'Target_Carol', 'Brown');  -- Reusing old names

-- UPDATE-UPDATE conflict
-- Insert initial data
INSERT INTO multi_unique_index (id, first_name, last_name) VALUES (108, 'Target_Dave', 'Wilson');
INSERT INTO multi_unique_index (id, first_name, last_name) VALUES (109, 'Target_Eve', 'Davis');

UPDATE multi_unique_index SET first_name = 'Updated_Target_Dave' WHERE id = 108;
UPDATE multi_unique_index SET first_name = 'Target_Dave', last_name = 'Wilson' WHERE id = 109;  -- Using old names



-- Initial data for `different_columns_unique_constraint_and_index`
INSERT INTO different_columns_unique_constraint_and_index (id, email, phone_number) VALUES (101, 'target_user101@example.com', '555-555-5510');
INSERT INTO different_columns_unique_constraint_and_index (id, email, phone_number) VALUES (102, 'target_user102@example.com', '555-555-5511');
INSERT INTO different_columns_unique_constraint_and_index (id, email, phone_number) VALUES (103, 'target_user103@example.com', '555-555-5512');
INSERT INTO different_columns_unique_constraint_and_index (id, email, phone_number) VALUES (104, 'target_user104@example.com', '555-555-5513');
INSERT INTO different_columns_unique_constraint_and_index (id, email, phone_number) VALUES (105, 'target_user105@example.com', '555-555-5514');

-- 6. Conflicts for `different_columns_unique_constraint_and_index`
-- DELETE-INSERT conflict
DELETE FROM different_columns_unique_constraint_and_index WHERE id = 101;
INSERT INTO different_columns_unique_constraint_and_index (id, email, phone_number) VALUES (106, 'target_user101@example.com', '555-555-5510');  -- Reusing deleted email and phone_number

-- Non-conflict update and delete
UPDATE different_columns_unique_constraint_and_index SET email = 'updated_target_user102@example.com' WHERE id = 102;  -- Non-conflict update on email
DELETE FROM different_columns_unique_constraint_and_index WHERE id = 103;  -- Non-conflict delete

-- DELETE-UPDATE conflict
DELETE FROM different_columns_unique_constraint_and_index WHERE id = 104;
UPDATE different_columns_unique_constraint_and_index SET phone_number = '555-555-5513' WHERE id = 105;  -- Using deleted phone_number

-- UPDATE-INSERT conflict
UPDATE different_columns_unique_constraint_and_index SET email = 'updated_target_user105@example.com' WHERE id = 105;
INSERT INTO different_columns_unique_constraint_and_index (id, email, phone_number) VALUES (107, 'target_user105@example.com', '555-555-5515');  -- Reusing old email with new phone_number

-- UPDATE-UPDATE conflict
-- Insert initial data
INSERT INTO different_columns_unique_constraint_and_index (id, email, phone_number) VALUES (108, 'target_user106@example.com', '555-555-5516');
INSERT INTO different_columns_unique_constraint_and_index (id, email, phone_number) VALUES (109, 'target_user107@example.com', '555-555-5517');

UPDATE different_columns_unique_constraint_and_index SET phone_number = '555-555-5518' WHERE id = 108;
UPDATE different_columns_unique_constraint_and_index SET phone_number = '555-555-5516' WHERE id = 109;  -- Using old phone_number





-- Initial data for `subset_columns_unique_constraint_and_index`
INSERT INTO subset_columns_unique_constraint_and_index (id, first_name, last_name, phone_number) VALUES (101, 'Target_John', 'Doe', '123-456-7810');
INSERT INTO subset_columns_unique_constraint_and_index (id, first_name, last_name, phone_number) VALUES (102, 'Target_Jane', 'Smith', '123-456-7811');
INSERT INTO subset_columns_unique_constraint_and_index (id, first_name, last_name, phone_number) VALUES (103, 'Target_Bob', 'Johnson', '123-456-7812');
INSERT INTO subset_columns_unique_constraint_and_index (id, first_name, last_name, phone_number) VALUES (104, 'Target_Alice', 'Williams', '123-456-7813');
INSERT INTO subset_columns_unique_constraint_and_index (id, first_name, last_name, phone_number) VALUES (105, 'Target_Carol', 'Brown', '123-456-7814');

-- 7. Conflicts for `subset_columns_unique_constraint_and_index`
-- DELETE-INSERT conflict
DELETE FROM subset_columns_unique_constraint_and_index WHERE id = 101;
INSERT INTO subset_columns_unique_constraint_and_index (id, first_name, last_name, phone_number) VALUES (106, 'Target_John', 'Doe', '123-456-7810');  -- Reusing deleted values

-- Non-conflict update and delete
UPDATE subset_columns_unique_constraint_and_index SET first_name = 'Updated_Target_Jane', last_name = 'Updated_Smith' WHERE id = 102;  -- Non-conflict update
DELETE FROM subset_columns_unique_constraint_and_index WHERE id = 103;  -- Non-conflict delete

-- DELETE-UPDATE conflict
DELETE FROM subset_columns_unique_constraint_and_index WHERE id = 104;
UPDATE subset_columns_unique_constraint_and_index SET first_name = 'Target_Alice', last_name = 'Williams' WHERE id = 105;  -- Using deleted first_name and last_name

-- UPDATE-INSERT conflict
UPDATE subset_columns_unique_constraint_and_index SET phone_number = '123-456-7855' WHERE id = 105;
INSERT INTO subset_columns_unique_constraint_and_index (id, first_name, last_name, phone_number) VALUES (107, 'Target_Dave', 'Wilson', '123-456-7815');  -- Reusing old phone_number

-- UPDATE-UPDATE conflict
-- Insert initial data
INSERT INTO subset_columns_unique_constraint_and_index (id, first_name, last_name, phone_number) VALUES (108, 'Target_Eve', 'Davis', '123-456-7816');
INSERT INTO subset_columns_unique_constraint_and_index (id, first_name, last_name, phone_number) VALUES (109, 'Target_Frank', 'Miller', '123-456-7817');

UPDATE subset_columns_unique_constraint_and_index SET phone_number = '123-456-7818' WHERE id = 108;
UPDATE subset_columns_unique_constraint_and_index SET phone_number = '123-456-7816' WHERE id = 109;  -- Using old phone_number


-- events for test_partial_unique_index table
-- will uncomment in another PR as part of cdc partitioning strategy changes
-- UPDATE test_partial_unique_index SET most_recent = false WHERE check_id = 1;
-- INSERT INTO test_partial_unique_index (check_id, most_recent) VALUES (1, true);

-- UPDATE test_partial_unique_index SET most_recent = false WHERE check_id = 2;
-- INSERT INTO test_partial_unique_index (check_id, most_recent) VALUES (2, true);
