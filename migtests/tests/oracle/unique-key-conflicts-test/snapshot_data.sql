-- Insert data for single_unique_constraint
INSERT INTO single_unique_constraint (email) VALUES ('user1@example.com');
INSERT INTO single_unique_constraint (email) VALUES ('user2@example.com');
INSERT INTO single_unique_constraint (email) VALUES ('user3@example.com');
INSERT INTO single_unique_constraint (email) VALUES ('user4@example.com');
INSERT INTO single_unique_constraint (email) VALUES ('user5@example.com');

-- Insert data for multi_unique_constraint
INSERT INTO multi_unique_constraint (first_name, last_name) VALUES ('John', 'Doe');
INSERT INTO multi_unique_constraint (first_name, last_name) VALUES ('Jane', 'Smith');
INSERT INTO multi_unique_constraint (first_name, last_name) VALUES ('Bob', 'Johnson');
INSERT INTO multi_unique_constraint (first_name, last_name) VALUES ('Alice', 'Williams');
INSERT INTO multi_unique_constraint (first_name, last_name) VALUES ('Tom', 'Clark');

-- Insert data for single_unique_index
INSERT INTO single_unique_index (ssn) VALUES ('SSN1');
INSERT INTO single_unique_index (ssn) VALUES ('SSN2');
INSERT INTO single_unique_index (ssn) VALUES ('SSN3');
INSERT INTO single_unique_index (ssn) VALUES ('SSN4');
INSERT INTO single_unique_index (ssn) VALUES ('SSN5');

-- Insert data for multi_unique_index
INSERT INTO multi_unique_index (first_name, last_name) VALUES ('John', 'Doe');
INSERT INTO multi_unique_index (first_name, last_name) VALUES ('Jane', 'Smith');
INSERT INTO multi_unique_index (first_name, last_name) VALUES ('Bob', 'Johnson');
INSERT INTO multi_unique_index (first_name, last_name) VALUES ('Alice', 'Williams');
INSERT INTO multi_unique_index (first_name, last_name) VALUES ('Tom', 'Clark');

-- Insert data for different_columns_unique_constraint_and_index
INSERT INTO different_columns_unique_constraint_and_index (email, phone_number) VALUES ('user1@example.com', '555-555-5551');
INSERT INTO different_columns_unique_constraint_and_index (email, phone_number) VALUES ('user2@example.com', '555-555-5552');
INSERT INTO different_columns_unique_constraint_and_index (email, phone_number) VALUES ('user3@example.com', '555-555-5553');
INSERT INTO different_columns_unique_constraint_and_index (email, phone_number) VALUES ('user4@example.com', '555-555-5554');
INSERT INTO different_columns_unique_constraint_and_index (email, phone_number) VALUES ('user5@example.com', '555-555-5555');

-- Insert data for subset_columns_unique_constraint_and_index
INSERT INTO subset_columns_unique_constraint_and_index (first_name, last_name, phone_number) VALUES ('John', 'Doe', '123-456-7890');
INSERT INTO subset_columns_unique_constraint_and_index (first_name, last_name, phone_number) VALUES ('Jane', 'Smith', '123-456-7891');
INSERT INTO subset_columns_unique_constraint_and_index (first_name, last_name, phone_number) VALUES ('Bob', 'Johnson', '123-456-7892');
INSERT INTO subset_columns_unique_constraint_and_index (first_name, last_name, phone_number) VALUES ('Alice', 'Williams', '123-456-7893');
INSERT INTO subset_columns_unique_constraint_and_index (first_name, last_name, phone_number) VALUES ('Tom', 'Clark', '123-456-7894');