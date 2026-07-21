-- Schema for the live-migration unique-conflict resumption test.
-- Tables are copied from migtests/tests/pg/unique-key-conflicts-test/snapshot_schema.sql
-- and cover every unique-key shape the streaming-phase conflict-detection cache
-- (yb-voyager/cmd/conflictDetectionCache.go) reasons about.
--
-- REPLICA IDENTITY FULL is set on every table so that Debezium captures the
-- before-image of UPDATE/DELETE rows. The conflict-detection cache compares the
-- before-values of unique-key columns (incl. partial-index predicate columns),
-- so without full before-images the DELETE-*/UPDATE-* conflicts cannot be detected.

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

-- Single-column unique index with NULLS NOT DISTINCT: two NULLs are treated as
-- equal, so a NULL free->reuse across different PKs is a real conflict.
CREATE TABLE single_unique_index_nulls_not_distinct (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255)
);
CREATE UNIQUE INDEX idx_email_nnd ON single_unique_index_nulls_not_distinct (email) NULLS NOT DISTINCT;

-- Multi-column unique index with NULLS NOT DISTINCT.
CREATE TABLE multi_unique_index_nulls_not_distinct (
    id SERIAL PRIMARY KEY,
    first_name VARCHAR(100),
    last_name VARCHAR(100)
);
CREATE UNIQUE INDEX idx_name_nnd ON multi_unique_index_nulls_not_distinct (first_name, last_name) NULLS NOT DISTINCT;

-- Single-column unique index with the default NULLS DISTINCT: NULLs are all
-- distinct, so multiple NULL rows coexist and a NULL free->reuse is NOT a conflict
-- (contrast with single_unique_index_nulls_not_distinct above). Exercises the
-- NULLS DISTINCT branch of the per-index conflict handling.
CREATE TABLE single_unique_index_nulls_distinct (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255)
);
CREATE UNIQUE INDEX idx_email_nd ON single_unique_index_nulls_distinct (email);

-- Partitioned table with a unique index. A unique key on a partitioned table
-- must include the partition-key column, so the key is (email, region).
CREATE TABLE partitioned_unique_conflict (
    id INT,
    region VARCHAR(50),
    email VARCHAR(255),
    PRIMARY KEY (id, region)
) PARTITION BY LIST (region);
CREATE TABLE partitioned_unique_conflict_east PARTITION OF partitioned_unique_conflict FOR VALUES IN ('east');
CREATE TABLE partitioned_unique_conflict_west PARTITION OF partitioned_unique_conflict FOR VALUES IN ('west');
CREATE TABLE partitioned_unique_conflict_default PARTITION OF partitioned_unique_conflict DEFAULT;
CREATE UNIQUE INDEX idx_partitioned_unique_email ON partitioned_unique_conflict (email, region);


-- Full before-images are required for unique-conflict detection during streaming.
ALTER TABLE single_unique_constraint REPLICA IDENTITY FULL;
ALTER TABLE multi_unique_constraint REPLICA IDENTITY FULL;
ALTER TABLE single_unique_index REPLICA IDENTITY FULL;
ALTER TABLE multi_unique_index REPLICA IDENTITY FULL;
ALTER TABLE same_column_unique_constraint_and_index REPLICA IDENTITY FULL;
ALTER TABLE different_columns_unique_constraint_and_index REPLICA IDENTITY FULL;
ALTER TABLE subset_columns_unique_constraint_and_index REPLICA IDENTITY FULL;
ALTER TABLE expression_based_unique_index REPLICA IDENTITY FULL;
ALTER TABLE test_partial_unique_index REPLICA IDENTITY FULL;
ALTER TABLE single_unique_index_nulls_not_distinct REPLICA IDENTITY FULL;
ALTER TABLE multi_unique_index_nulls_not_distinct REPLICA IDENTITY FULL;
ALTER TABLE single_unique_index_nulls_distinct REPLICA IDENTITY FULL;
ALTER TABLE partitioned_unique_conflict_east REPLICA IDENTITY FULL;
ALTER TABLE partitioned_unique_conflict_west REPLICA IDENTITY FULL;
ALTER TABLE partitioned_unique_conflict_default REPLICA IDENTITY FULL;
