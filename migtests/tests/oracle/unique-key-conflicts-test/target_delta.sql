-- Initial data for `test_schema.SINGLE_UNIQUE_CONSTR`
INSERT INTO test_schema.SINGLE_UNIQUE_CONSTR (ID, EMAIL) VALUES (101, 'target_user101@example.com');
INSERT INTO test_schema.SINGLE_UNIQUE_CONSTR (ID, EMAIL) VALUES (102, 'target_user102@example.com');
INSERT INTO test_schema.SINGLE_UNIQUE_CONSTR (ID, EMAIL) VALUES (103, 'target_user103@example.com');
INSERT INTO test_schema.SINGLE_UNIQUE_CONSTR (ID, EMAIL) VALUES (104, 'target_user104@example.com');
INSERT INTO test_schema.SINGLE_UNIQUE_CONSTR (ID, EMAIL) VALUES (105, 'target_user105@example.com');

-- 1. Conflicts for `test_schema.SINGLE_UNIQUE_CONSTR`
-- DELETE-INSERT conflict
DELETE FROM test_schema.SINGLE_UNIQUE_CONSTR WHERE ID = 101;
INSERT INTO test_schema.SINGLE_UNIQUE_CONSTR (ID, EMAIL) VALUES (106, 'target_user101@example.com');

-- Non-conflict update and delete
UPDATE test_schema.SINGLE_UNIQUE_CONSTR SET EMAIL = 'updated_target_user102@example.com' WHERE ID = 102;
DELETE FROM test_schema.SINGLE_UNIQUE_CONSTR WHERE ID = 103;

-- DELETE-UPDATE conflict
DELETE FROM test_schema.SINGLE_UNIQUE_CONSTR WHERE ID = 104;
UPDATE test_schema.SINGLE_UNIQUE_CONSTR SET EMAIL = 'target_user104@example.com' WHERE ID = 105;

-- UPDATE-INSERT conflict
UPDATE test_schema.SINGLE_UNIQUE_CONSTR SET EMAIL = 'updated_target_user105@example.com' WHERE ID = 105;
INSERT INTO test_schema.SINGLE_UNIQUE_CONSTR (ID, EMAIL) VALUES (107, 'target_user105@example.com');

-- UPDATE-UPDATE conflict
-- Insert initial data
INSERT INTO test_schema.SINGLE_UNIQUE_CONSTR (ID, EMAIL) VALUES (108, 'target_user106@example.com');
INSERT INTO test_schema.SINGLE_UNIQUE_CONSTR (ID, EMAIL) VALUES (109, 'target_user107@example.com');

UPDATE test_schema.SINGLE_UNIQUE_CONSTR SET EMAIL = 'updated_target_user106@example.com' WHERE ID = 108;
UPDATE test_schema.SINGLE_UNIQUE_CONSTR SET EMAIL = 'target_user106@example.com' WHERE ID = 109;



-- Initial data for `test_schema.MULTI_UNIQUE_CONSTR`
INSERT INTO test_schema.MULTI_UNIQUE_CONSTR (ID, FIRST_NAME, LAST_NAME) VALUES (101, 'Target_John', 'Doe');
INSERT INTO test_schema.MULTI_UNIQUE_CONSTR (ID, FIRST_NAME, LAST_NAME) VALUES (102, 'Target_Jane', 'Smith');
INSERT INTO test_schema.MULTI_UNIQUE_CONSTR (ID, FIRST_NAME, LAST_NAME) VALUES (103, 'Target_Tom', 'Clark');
INSERT INTO test_schema.MULTI_UNIQUE_CONSTR (ID, FIRST_NAME, LAST_NAME) VALUES (104, 'Target_Alice', 'Williams');
INSERT INTO test_schema.MULTI_UNIQUE_CONSTR (ID, FIRST_NAME, LAST_NAME) VALUES (105, 'Target_Bob', 'Johnson');

-- 2. Conflicts for `test_schema.MULTI_UNIQUE_CONSTR`
-- DELETE-INSERT conflict
DELETE FROM test_schema.MULTI_UNIQUE_CONSTR WHERE ID = 101;
INSERT INTO test_schema.MULTI_UNIQUE_CONSTR (ID, FIRST_NAME, LAST_NAME) VALUES (106, 'Target_John', 'Doe');

-- Non-conflict update and delete
UPDATE test_schema.MULTI_UNIQUE_CONSTR SET FIRST_NAME = 'Updated_Target_Jane', LAST_NAME = 'Updated_Smith' WHERE ID = 102;
DELETE FROM test_schema.MULTI_UNIQUE_CONSTR WHERE ID = 103;

-- DELETE-UPDATE conflict
DELETE FROM test_schema.MULTI_UNIQUE_CONSTR WHERE ID = 104;
UPDATE test_schema.MULTI_UNIQUE_CONSTR SET FIRST_NAME = 'Target_Alice', LAST_NAME = 'Williams' WHERE ID = 105;

-- UPDATE-INSERT conflict
UPDATE test_schema.MULTI_UNIQUE_CONSTR SET FIRST_NAME = 'Updated_Target_Bob' WHERE ID = 105;
INSERT INTO test_schema.MULTI_UNIQUE_CONSTR (ID, FIRST_NAME, LAST_NAME) VALUES (107, 'Target_Bob', 'Johnson');

-- UPDATE-UPDATE conflict
-- Insert initial data
INSERT INTO test_schema.MULTI_UNIQUE_CONSTR (ID, FIRST_NAME, LAST_NAME) VALUES (108, 'Target_Dave', 'Wilson');
INSERT INTO test_schema.MULTI_UNIQUE_CONSTR (ID, FIRST_NAME, LAST_NAME) VALUES (109, 'Target_Eve', 'Davis');

UPDATE test_schema.MULTI_UNIQUE_CONSTR SET FIRST_NAME = 'Updated_Target_Dave' WHERE ID = 108;
UPDATE test_schema.MULTI_UNIQUE_CONSTR SET FIRST_NAME = 'Target_Dave', LAST_NAME = 'Wilson' WHERE ID = 109;



-- Initial data for `test_schema.SINGLE_UNIQUE_IDX`
INSERT INTO test_schema.SINGLE_UNIQUE_IDX (ID, SSN) VALUES (101, 'target_ssn101');
INSERT INTO test_schema.SINGLE_UNIQUE_IDX (ID, SSN) VALUES (102, 'target_ssn102');
INSERT INTO test_schema.SINGLE_UNIQUE_IDX (ID, SSN) VALUES (103, 'target_ssn103');
INSERT INTO test_schema.SINGLE_UNIQUE_IDX (ID, SSN) VALUES (104, 'target_ssn104');
INSERT INTO test_schema.SINGLE_UNIQUE_IDX (ID, SSN) VALUES (105, 'target_ssn105');

-- 3. Conflicts for `test_schema.SINGLE_UNIQUE_IDX`
-- DELETE-INSERT conflict
DELETE FROM test_schema.SINGLE_UNIQUE_IDX WHERE ID = 101;
INSERT INTO test_schema.SINGLE_UNIQUE_IDX (ID, SSN) VALUES (106, 'target_ssn101');

-- Non-conflict update and delete
UPDATE test_schema.SINGLE_UNIQUE_IDX SET SSN = 'updated_target_ssn102' WHERE ID = 102;
DELETE FROM test_schema.SINGLE_UNIQUE_IDX WHERE ID = 103;

-- DELETE-UPDATE conflict
DELETE FROM test_schema.SINGLE_UNIQUE_IDX WHERE ID = 104;
UPDATE test_schema.SINGLE_UNIQUE_IDX SET SSN = 'target_ssn104' WHERE ID = 105;

-- UPDATE-INSERT conflict
UPDATE test_schema.SINGLE_UNIQUE_IDX SET SSN = 'updated_target_ssn105' WHERE ID = 105;
INSERT INTO test_schema.SINGLE_UNIQUE_IDX (ID, SSN) VALUES (107, 'target_ssn105');

-- UPDATE-UPDATE conflict
-- Insert initial data
INSERT INTO test_schema.SINGLE_UNIQUE_IDX (ID, SSN) VALUES (108, 'target_ssn106');
INSERT INTO test_schema.SINGLE_UNIQUE_IDX (ID, SSN) VALUES (109, 'target_ssn107');

UPDATE test_schema.SINGLE_UNIQUE_IDX SET SSN = 'updated_target_ssn106' WHERE ID = 108;
UPDATE test_schema.SINGLE_UNIQUE_IDX SET SSN = 'target_ssn106' WHERE ID = 109;



-- Initial data for `test_schema.MULTI_UNIQUE_IDX`
INSERT INTO test_schema.MULTI_UNIQUE_IDX (ID, FIRST_NAME, LAST_NAME) VALUES (101, 'Target_Jane', 'Smith');
INSERT INTO test_schema.MULTI_UNIQUE_IDX (ID, FIRST_NAME, LAST_NAME) VALUES (102, 'Target_Tom', 'Clark');
INSERT INTO test_schema.MULTI_UNIQUE_IDX (ID, FIRST_NAME, LAST_NAME) VALUES (103, 'Target_Alice', 'Williams');
INSERT INTO test_schema.MULTI_UNIQUE_IDX (ID, FIRST_NAME, LAST_NAME) VALUES (104, 'Target_Bob', 'Johnson');
INSERT INTO test_schema.MULTI_UNIQUE_IDX (ID, FIRST_NAME, LAST_NAME) VALUES (105, 'Target_Carol', 'Brown');

-- 4. Conflicts for `test_schema.MULTI_UNIQUE_IDX`
-- DELETE-INSERT conflict
DELETE FROM test_schema.MULTI_UNIQUE_IDX WHERE ID = 101;
INSERT INTO test_schema.MULTI_UNIQUE_IDX (ID, FIRST_NAME, LAST_NAME) VALUES (106, 'Target_Jane', 'Smith');

-- Non-conflict update and delete
UPDATE test_schema.MULTI_UNIQUE_IDX SET FIRST_NAME = 'Updated_Target_Tom', LAST_NAME = 'Updated_Clark' WHERE ID = 102;
DELETE FROM test_schema.MULTI_UNIQUE_IDX WHERE ID = 103;

-- DELETE-UPDATE conflict
DELETE FROM test_schema.MULTI_UNIQUE_IDX WHERE ID = 104;
UPDATE test_schema.MULTI_UNIQUE_IDX SET FIRST_NAME = 'Target_Bob', LAST_NAME = 'Johnson' WHERE ID = 105;

-- UPDATE-INSERT conflict
UPDATE test_schema.MULTI_UNIQUE_IDX SET FIRST_NAME = 'Updated_Target_Carol' WHERE ID = 105;
INSERT INTO test_schema.MULTI_UNIQUE_IDX (ID, FIRST_NAME, LAST_NAME) VALUES (107, 'Target_Carol', 'Brown');

-- UPDATE-UPDATE conflict
-- Insert initial data
INSERT INTO test_schema.MULTI_UNIQUE_IDX (ID, FIRST_NAME, LAST_NAME) VALUES (108, 'Target_Dave', 'Wilson');
INSERT INTO test_schema.MULTI_UNIQUE_IDX (ID, FIRST_NAME, LAST_NAME) VALUES (109, 'Target_Eve', 'Davis');

UPDATE test_schema.MULTI_UNIQUE_IDX SET FIRST_NAME = 'Updated_Target_Dave' WHERE ID = 108;
UPDATE test_schema.MULTI_UNIQUE_IDX SET FIRST_NAME = 'Target_Dave', LAST_NAME = 'Wilson' WHERE ID = 109;



-- Initial data for `test_schema.DIFF_COLUMNS_CONSTR_IDX`
INSERT INTO test_schema.DIFF_COLUMNS_CONSTR_IDX (ID, EMAIL, PHONE_NUMBER) VALUES (101, 'target_user101@example.com', '555-555-5510');
INSERT INTO test_schema.DIFF_COLUMNS_CONSTR_IDX (ID, EMAIL, PHONE_NUMBER) VALUES (102, 'target_user102@example.com', '555-555-5511');
INSERT INTO test_schema.DIFF_COLUMNS_CONSTR_IDX (ID, EMAIL, PHONE_NUMBER) VALUES (103, 'target_user103@example.com', '555-555-5512');
INSERT INTO test_schema.DIFF_COLUMNS_CONSTR_IDX (ID, EMAIL, PHONE_NUMBER) VALUES (104, 'target_user104@example.com', '555-555-5513');
INSERT INTO test_schema.DIFF_COLUMNS_CONSTR_IDX (ID, EMAIL, PHONE_NUMBER) VALUES (105, 'target_user105@example.com', '555-555-5514');

-- 5. Conflicts for `test_schema.DIFF_COLUMNS_CONSTR_IDX`
-- DELETE-INSERT conflict
DELETE FROM test_schema.DIFF_COLUMNS_CONSTR_IDX WHERE ID = 101;
INSERT INTO test_schema.DIFF_COLUMNS_CONSTR_IDX (ID, EMAIL, PHONE_NUMBER) VALUES (106, 'target_user101@example.com', '555-555-5510');

-- Non-conflict update and delete
UPDATE test_schema.DIFF_COLUMNS_CONSTR_IDX SET EMAIL = 'updated_target_user102@example.com' WHERE ID = 102;
DELETE FROM test_schema.DIFF_COLUMNS_CONSTR_IDX WHERE ID = 103;

-- DELETE-UPDATE conflict
DELETE FROM test_schema.DIFF_COLUMNS_CONSTR_IDX WHERE ID = 104;
UPDATE test_schema.DIFF_COLUMNS_CONSTR_IDX SET PHONE_NUMBER = '555-555-5513' WHERE ID = 105;

-- UPDATE-INSERT conflict
UPDATE test_schema.DIFF_COLUMNS_CONSTR_IDX SET EMAIL = 'updated_target_user105@example.com' WHERE ID = 105;
INSERT INTO test_schema.DIFF_COLUMNS_CONSTR_IDX (ID, EMAIL, PHONE_NUMBER) VALUES (107, 'target_user105@example.com', '555-555-5515');

-- UPDATE-UPDATE conflict
-- Insert initial data
INSERT INTO test_schema.DIFF_COLUMNS_CONSTR_IDX (ID, EMAIL, PHONE_NUMBER) VALUES (108, 'target_user106@example.com', '555-555-5516');
INSERT INTO test_schema.DIFF_COLUMNS_CONSTR_IDX (ID, EMAIL, PHONE_NUMBER) VALUES (109, 'target_user107@example.com', '555-555-5517');

UPDATE test_schema.DIFF_COLUMNS_CONSTR_IDX SET PHONE_NUMBER = '555-555-5518' WHERE ID = 108;
UPDATE test_schema.DIFF_COLUMNS_CONSTR_IDX SET PHONE_NUMBER = '555-555-5516' WHERE ID = 109;



-- Initial data for `test_schema.SUBSET_COLUMNS_CONSTR_IDX`
INSERT INTO test_schema.SUBSET_COLUMNS_CONSTR_IDX (ID, FIRST_NAME, LAST_NAME, PHONE_NUMBER) VALUES (101, 'Target_John', 'Doe', '123-456-7810');
INSERT INTO test_schema.SUBSET_COLUMNS_CONSTR_IDX (ID, FIRST_NAME, LAST_NAME, PHONE_NUMBER) VALUES (102, 'Target_Jane', 'Smith', '123-456-7811');
INSERT INTO test_schema.SUBSET_COLUMNS_CONSTR_IDX (ID, FIRST_NAME, LAST_NAME, PHONE_NUMBER) VALUES (103, 'Target_Bob', 'Johnson', '123-456-7812');
INSERT INTO test_schema.SUBSET_COLUMNS_CONSTR_IDX (ID, FIRST_NAME, LAST_NAME, PHONE_NUMBER) VALUES (104, 'Target_Alice', 'Williams', '123-456-7813');
INSERT INTO test_schema.SUBSET_COLUMNS_CONSTR_IDX (ID, FIRST_NAME, LAST_NAME, PHONE_NUMBER) VALUES (105, 'Target_Carol', 'Brown', '123-456-7814');

-- 6. Conflicts for `test_schema.SUBSET_COLUMNS_CONSTR_IDX`
-- DELETE-INSERT conflict
DELETE FROM test_schema.SUBSET_COLUMNS_CONSTR_IDX WHERE ID = 101;
INSERT INTO test_schema.SUBSET_COLUMNS_CONSTR_IDX (ID, FIRST_NAME, LAST_NAME, PHONE_NUMBER) VALUES (106, 'Target_John', 'Doe', '123-456-7810');

-- Non-conflict update and delete
UPDATE test_schema.SUBSET_COLUMNS_CONSTR_IDX SET FIRST_NAME = 'Updated_Target_Jane', LAST_NAME = 'Updated_Smith' WHERE ID = 102;
DELETE FROM test_schema.SUBSET_COLUMNS_CONSTR_IDX WHERE ID = 103;

-- DELETE-UPDATE conflict
DELETE FROM test_schema.SUBSET_COLUMNS_CONSTR_IDX WHERE ID = 104;
UPDATE test_schema.SUBSET_COLUMNS_CONSTR_IDX SET FIRST_NAME = 'Target_Alice', LAST_NAME = 'Williams' WHERE ID = 105;

-- UPDATE-INSERT conflict
UPDATE test_schema.SUBSET_COLUMNS_CONSTR_IDX SET PHONE_NUMBER = '123-456-7855' WHERE ID = 105;
INSERT INTO test_schema.SUBSET_COLUMNS_CONSTR_IDX (ID, FIRST_NAME, LAST_NAME, PHONE_NUMBER) VALUES (107, 'Target_Dave', 'Wilson', '123-456-7815');

-- UPDATE-UPDATE conflict
-- Insert initial data
INSERT INTO test_schema.SUBSET_COLUMNS_CONSTR_IDX (ID, FIRST_NAME, LAST_NAME, PHONE_NUMBER) VALUES (108, 'Target_Eve', 'Davis', '123-456-7816');
INSERT INTO test_schema.SUBSET_COLUMNS_CONSTR_IDX (ID, FIRST_NAME, LAST_NAME, PHONE_NUMBER) VALUES (109, 'Target_Frank', 'Miller', '123-456-7817');

UPDATE test_schema.SUBSET_COLUMNS_CONSTR_IDX SET PHONE_NUMBER = '123-456-7818' WHERE ID = 108;
UPDATE test_schema.SUBSET_COLUMNS_CONSTR_IDX SET PHONE_NUMBER = '123-456-7816' WHERE ID = 109;
