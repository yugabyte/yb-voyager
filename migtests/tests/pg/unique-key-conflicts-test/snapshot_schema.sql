-- Non-public schema
CREATE SCHEMA non_public;

-- Table in non-public schema with Single Column Unique Constraint
CREATE TABLE non_public.single_unique_constraint (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE
);

-- Table in non-public schema with Multiple Column Unique Constraint
CREATE TABLE non_public.multi_unique_constraint (
    id SERIAL PRIMARY KEY,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    CONSTRAINT unique_name UNIQUE (first_name, last_name)
);

-- Public schema table with Single Column Unique Index
CREATE TABLE public.single_unique_index (
    id SERIAL PRIMARY KEY,
    ssn VARCHAR(20)
);
CREATE UNIQUE INDEX idx_ssn_unique ON public.single_unique_index (ssn);

-- Public schema table with Multiple Column Unique Index
CREATE TABLE public.multi_unique_index (
    id SERIAL PRIMARY KEY,
    first_name VARCHAR(100),
    last_name VARCHAR(100)
);
CREATE UNIQUE INDEX idx_name_unique ON public.multi_unique_index (first_name, last_name);

-- Non-public schema table with Unique Constraint and Unique Index on the Same Column
CREATE TABLE non_public.same_column_unique_constraint_and_index (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE
);
CREATE UNIQUE INDEX idx_email_unique ON non_public.same_column_unique_constraint_and_index (email);

-- Public schema table with Unique Constraint and Unique Index on Different Columns
CREATE TABLE public.different_columns_unique_constraint_and_index (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE,
    phone_number VARCHAR(20)
);
CREATE UNIQUE INDEX idx_phone_unique ON public.different_columns_unique_constraint_and_index (phone_number);

-- Public schema Table with Unique Constraint and Unique Index, having Subset of Columns Overlapping
CREATE TABLE public.subset_columns_unique_constraint_and_index (
    id SERIAL PRIMARY KEY,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    phone_number VARCHAR(20)
);

-- Unique constraint on first_name and last_name
ALTER TABLE public.subset_columns_unique_constraint_and_index ADD CONSTRAINT unique_name_constraint UNIQUE (first_name, last_name);

-- Unique index on first_name, last_name, and phone_number (superset of columns)
CREATE UNIQUE INDEX idx_name_phone_unique ON public.subset_columns_unique_constraint_and_index (first_name, last_name, phone_number);