-- Insert data for single_unique_constraint
INSERT INTO single_unique_constraint (email) 
VALUES 
    ('user1@example.com'), 
    ('user2@example.com'), 
    ('user3@example.com'), 
    ('user4@example.com'), 
    ('user5@example.com');

-- Insert data for multi_unique_constraint
INSERT INTO multi_unique_constraint (first_name, last_name) 
VALUES 
    ('John', 'Doe'), 
    ('Jane', 'Smith'), 
    ('Bob', 'Johnson'), 
    ('Alice', 'Williams'), 
    ('Tom', 'Clark');

-- Insert data for same_column_unique_constraint_and_index
INSERT INTO same_column_unique_constraint_and_index (email) 
VALUES
    ('user1@example.com'),
    ('user2@example.com'),
    ('user3@example.com'),
    ('user4@example.com'),
    ('user5@example.com');

-- Insert data for single_unique_index
INSERT INTO single_unique_index ("Ssn") 
VALUES 
    ('SSN1'), 
    ('SSN2'), 
    ('SSN3'), 
    ('SSN4'), 
    ('SSN5');

-- Insert data for multi_unique_index
INSERT INTO multi_unique_index (first_name, last_name) 
VALUES 
    ('John', 'Doe'), 
    ('Jane', 'Smith'), 
    ('Bob', 'Johnson'), 
    ('Alice', 'Williams'), 
    ('Tom', 'Clark');

-- Insert data for different_columns_unique_constraint_and_index
INSERT INTO different_columns_unique_constraint_and_index (email, phone_number) 
VALUES 
    ('user1@example.com', '555-555-5551'), 
    ('user2@example.com', '555-555-5552'), 
    ('user3@example.com', '555-555-5553'), 
    ('user4@example.com', '555-555-5554'), 
    ('user5@example.com', '555-555-5555');

-- Insert data for subset_columns_unique_constraint_and_index
INSERT INTO subset_columns_unique_constraint_and_index (first_name, last_name, phone_number) 
VALUES 
    ('John', 'Doe', '123-456-7890'), 
    ('Jane', 'Smith', '123-456-7891'), 
    ('Bob', 'Johnson', '123-456-7892'), 
    ('Alice', 'Williams', '123-456-7893'), 
    ('Tom', 'Clark', '123-456-7894');

-- Insert data for expression_based_unique_index
INSERT INTO expression_based_unique_index (email) 
VALUES 
    ('user1@example.com'),
    ('usER2@example.com'),
    ('usER3@example.com'),
    ('USER4@example.com'),
    ('USer5@example.com');
    
INSERT INTO test_partial_unique_index (check_id, most_recent) VALUES
    (1, true), --1
    (2, true), --2
    (3, true), --3
    (4, true), --4
    (4, false), --5
    (5, true), --6
    (6, true), --7
    (6, false), --8
    (7, true), --9    
    (8, true); --10
