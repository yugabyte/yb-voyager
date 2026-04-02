-- Table with Single Column Unique Constraint
CREATE TABLE single_unique_constraint (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE
);

-- Table with Multiple Column Unique Constraint
CREATE TABLE multi_unique_constraint (
    id SERIAL PRIMARY KEY,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    CONSTRAINT unique_name UNIQUE (first_name, last_name)
);

-- Table with Single Column Unique Index
CREATE TABLE single_unique_index (
    id SERIAL PRIMARY KEY,
    "Ssn" VARCHAR(100)
);
CREATE UNIQUE INDEX idx_ssn_unique ON single_unique_index ("Ssn");

-- Table with Multiple Column Unique Index
CREATE TABLE multi_unique_index (
    id SERIAL PRIMARY KEY,
    first_name VARCHAR(100),
    last_name VARCHAR(100)
);
CREATE UNIQUE INDEX idx_name_unique ON multi_unique_index (first_name, last_name);

-- Table with Unique Constraint and Unique Index on the Same Column
CREATE TABLE same_column_unique_constraint_and_index (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE
);
CREATE UNIQUE INDEX idx_email_unique ON same_column_unique_constraint_and_index (email);

-- Table with Unique Constraint and Unique Index on Different Columns
CREATE TABLE different_columns_unique_constraint_and_index (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE,
    phone_number VARCHAR(20)
);
CREATE UNIQUE INDEX idx_phone_unique ON different_columns_unique_constraint_and_index (phone_number);

-- Table with Unique Constraint and Unique Index, having Subset of Columns Overlapping
CREATE TABLE subset_columns_unique_constraint_and_index (
    id SERIAL PRIMARY KEY,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    phone_number VARCHAR(20)
);

-- Unique constraint on first_name and last_name
ALTER TABLE subset_columns_unique_constraint_and_index ADD CONSTRAINT unique_name_constraint UNIQUE (first_name, last_name);

-- Unique index on first_name, last_name, and phone_number (superset of columns)
CREATE UNIQUE INDEX idx_name_phone_unique ON subset_columns_unique_constraint_and_index (first_name, last_name, phone_number);


CREATE TABLE expression_based_unique_index (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255)
);
CREATE UNIQUE INDEX idx_email_unique_expression ON expression_based_unique_index (LOWER(email));

CREATE TABLE test_partial_unique_index (
    id SERIAL PRIMARY KEY,
    check_id int,
    most_recent boolean
);

CREATE UNIQUE INDEX idx_test_partial_unique_index ON test_partial_unique_index (check_id) WHERE most_recent;
