-- Schema for the custom-cdc-partition-key live-migration resumption test.
-- Unique-key shapes are copied from fallback-unique-conflict-test/init.sql
-- (originally migtests/tests/pg/unique-key-conflicts-test/snapshot_schema.sql)
-- and cover every unique-key shape the streaming-phase conflict-detection
-- cache (yb-voyager/cmd/conflictDetectionCache.go) reasons about.
--
-- Each table carries 13-14 columns shaped like a production table: an audit
-- backbone (created_at/updated_at NOT NULL, nullable deleted_at) plus a
-- realistic mix of business columns -- varchar/text/timestamp-heavy, with
-- boolean, int, bigint, smallint, date, numeric(38,6), jsonb, uuid, arrays
-- and double precision spread across the tables. NOT NULL columns get a
-- DEFAULT so the deterministic conflict DML (source_dml.sql), which names
-- only the key columns, works unchanged. numeric stays bounded -- unbounded
-- numeric loses trailing zeros through live CDC and breaks row-hash
-- validation.

-- Table with Single Column Unique Constraint
CREATE TABLE single_unique_constraint (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE,
    description text NOT NULL DEFAULT '',
    notes text NOT NULL DEFAULT '',
    failure_reason text NOT NULL DEFAULT '',
    remarks text NOT NULL DEFAULT '',
    retry_count int NOT NULL DEFAULT 0,
    amount numeric(38,6) NOT NULL DEFAULT 0,
    address_line text NOT NULL DEFAULT '',
    due_date date NOT NULL DEFAULT CURRENT_DATE,
    created_at timestamp NOT NULL DEFAULT now(),
    updated_at timestamp NOT NULL DEFAULT now(),
    deleted_at timestamp
);

-- Table with Multiple Column Unique Constraint
CREATE TABLE multi_unique_constraint (
    id SERIAL PRIMARY KEY,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    status varchar NOT NULL DEFAULT '',
    retry_count int NOT NULL DEFAULT 0,
    is_active boolean NOT NULL DEFAULT false,
    metadata jsonb NOT NULL DEFAULT '{}',
    description text NOT NULL DEFAULT '',
    created_at timestamp NOT NULL DEFAULT now(),
    updated_at timestamp NOT NULL DEFAULT now(),
    is_verified boolean,
    notes text,
    deleted_at timestamp,
    CONSTRAINT unique_name UNIQUE (first_name, last_name)
);

-- Table with Single Column Unique Index
CREATE TABLE single_unique_index (
    id SERIAL PRIMARY KEY,
    "Ssn" VARCHAR(100),
    due_date date NOT NULL DEFAULT CURRENT_DATE,
    status varchar NOT NULL DEFAULT '',
    description text NOT NULL DEFAULT '',
    is_active boolean NOT NULL DEFAULT false,
    currency varchar NOT NULL DEFAULT '',
    is_verified boolean NOT NULL DEFAULT false,
    deleted_at timestamp,
    created_at timestamp NOT NULL DEFAULT now(),
    updated_at timestamp NOT NULL DEFAULT now(),
    notes text,
    seq_no bigint
);
CREATE UNIQUE INDEX idx_ssn_unique ON single_unique_index ("Ssn");

-- Table with Multiple Column Unique Index
CREATE TABLE multi_unique_index (
    id SERIAL PRIMARY KEY,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    description text,
    status varchar NOT NULL DEFAULT '',
    currency varchar NOT NULL DEFAULT '',
    metadata jsonb NOT NULL DEFAULT '{}',
    created_at timestamp,
    updated_at timestamp,
    notes text,
    due_date date NOT NULL DEFAULT CURRENT_DATE,
    failure_reason text,
    deleted_at timestamp
);
CREATE UNIQUE INDEX idx_name_unique ON multi_unique_index (first_name, last_name);

-- Table with Unique Constraint and Unique Index on the Same Column
CREATE TABLE same_column_unique_constraint_and_index (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE,
    status varchar NOT NULL DEFAULT '',
    external_id uuid NOT NULL DEFAULT gen_random_uuid(),
    currency varchar NOT NULL DEFAULT '',
    retry_count int NOT NULL DEFAULT 0,
    attempt_count int NOT NULL DEFAULT 0,
    tags varchar[] NOT NULL DEFAULT '{}',
    processed_at timestamp,
    deleted_at timestamp,
    created_at timestamp NOT NULL DEFAULT now(),
    updated_at timestamp NOT NULL DEFAULT now(),
    reference_code varchar NOT NULL DEFAULT '',
    org_unit varchar NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX idx_email_unique ON same_column_unique_constraint_and_index (email);

-- Table with Unique Constraint and Unique Index on Different Columns
CREATE TABLE different_columns_unique_constraint_and_index (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE,
    phone_number VARCHAR(20),
    status varchar NOT NULL DEFAULT '',
    retry_count int NOT NULL DEFAULT 0,
    is_active boolean NOT NULL DEFAULT false,
    metadata jsonb NOT NULL DEFAULT '{}',
    description text NOT NULL DEFAULT '',
    notes text,
    failure_reason text,
    created_at timestamp NOT NULL DEFAULT now(),
    updated_at timestamp NOT NULL DEFAULT now(),
    deleted_at timestamp
);
CREATE UNIQUE INDEX idx_phone_unique ON different_columns_unique_constraint_and_index (phone_number);

-- Table with Unique Constraint and Unique Index, having Subset of Columns Overlapping
CREATE TABLE subset_columns_unique_constraint_and_index (
    id SERIAL PRIMARY KEY,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    phone_number VARCHAR(20),
    status varchar NOT NULL DEFAULT '',
    currency varchar NOT NULL DEFAULT '',
    priority smallint NOT NULL DEFAULT 0,
    processed_at timestamp,
    deleted_at timestamp,
    created_at timestamp NOT NULL DEFAULT now(),
    updated_at timestamp NOT NULL DEFAULT now(),
    is_active boolean NOT NULL DEFAULT false,
    retry_count int NOT NULL DEFAULT 0,
    attempt_count int NOT NULL DEFAULT 0
);

-- Unique constraint on first_name and last_name
ALTER TABLE subset_columns_unique_constraint_and_index ADD CONSTRAINT unique_name_constraint UNIQUE (first_name, last_name);

-- Unique index on first_name, last_name, and phone_number (superset of columns)
CREATE UNIQUE INDEX idx_name_phone_unique ON subset_columns_unique_constraint_and_index (first_name, last_name, phone_number);


CREATE TABLE expression_based_unique_index (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255),
    description text NOT NULL DEFAULT '',
    due_date date NOT NULL DEFAULT CURRENT_DATE,
    created_at timestamp NOT NULL DEFAULT now(),
    updated_at timestamp NOT NULL DEFAULT now(),
    status varchar NOT NULL DEFAULT '',
    is_active boolean NOT NULL DEFAULT false,
    deleted_at timestamp,
    amount numeric(38,6),
    notes text,
    failure_reason text,
    remarks text,
    address_line text
);
CREATE UNIQUE INDEX idx_email_unique_expression ON expression_based_unique_index (LOWER(email));

CREATE TABLE test_partial_unique_index (
    id SERIAL PRIMARY KEY,
    check_id int,
    most_recent boolean,
    status varchar NOT NULL DEFAULT '',
    metadata jsonb,
    retry_count int NOT NULL DEFAULT 0,
    is_active boolean NOT NULL DEFAULT false,
    currency varchar NOT NULL DEFAULT '',
    reference_code varchar NOT NULL DEFAULT '',
    org_unit varchar,
    created_at timestamp NOT NULL DEFAULT now(),
    updated_at timestamp NOT NULL DEFAULT now(),
    deleted_at timestamp
);

CREATE UNIQUE INDEX idx_test_partial_unique_index ON test_partial_unique_index (check_id) WHERE most_recent;

-- Single-column unique index with NULLS NOT DISTINCT: two NULLs are treated as
-- equal, so a NULL free->reuse across different PKs is a real conflict.
CREATE TABLE single_unique_index_nulls_not_distinct (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255),
    status varchar NOT NULL DEFAULT '',
    currency varchar NOT NULL DEFAULT '',
    due_date date NOT NULL DEFAULT CURRENT_DATE,
    reference_code varchar NOT NULL DEFAULT '',
    org_unit varchar NOT NULL DEFAULT '',
    seq_no bigint NOT NULL DEFAULT 0,
    amount numeric(38,6),
    is_active boolean NOT NULL DEFAULT false,
    deleted_at timestamp,
    created_at timestamp NOT NULL DEFAULT now(),
    updated_at timestamp NOT NULL DEFAULT now(),
    channel varchar
);
CREATE UNIQUE INDEX idx_email_nnd ON single_unique_index_nulls_not_distinct (email) NULLS NOT DISTINCT;

-- Multi-column unique index with NULLS NOT DISTINCT.
CREATE TABLE multi_unique_index_nulls_not_distinct (
    id SERIAL PRIMARY KEY,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    description text NOT NULL DEFAULT '',
    notes text NOT NULL DEFAULT '',
    failure_reason text,
    amount numeric(38,6) NOT NULL DEFAULT 0,
    status varchar NOT NULL DEFAULT '',
    created_at timestamp NOT NULL DEFAULT now(),
    updated_at timestamp NOT NULL DEFAULT now(),
    remarks text,
    deleted_at timestamp,
    is_active boolean,
    address_line text
);
CREATE UNIQUE INDEX idx_name_nnd ON multi_unique_index_nulls_not_distinct (first_name, last_name) NULLS NOT DISTINCT;

-- Single-column unique index with the default NULLS DISTINCT: NULLs are all
-- distinct, so multiple NULL rows coexist and a NULL free->reuse is NOT a conflict
-- (contrast with single_unique_index_nulls_not_distinct above). Exercises the
-- NULLS DISTINCT branch of the per-index conflict handling.
CREATE TABLE single_unique_index_nulls_distinct (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255),
    status varchar NOT NULL DEFAULT '',
    currency varchar NOT NULL DEFAULT '',
    processed_at timestamp,
    expires_at timestamp,
    description text,
    external_id uuid NOT NULL DEFAULT gen_random_uuid(),
    reference_code varchar,
    org_unit varchar NOT NULL DEFAULT '',
    deleted_at timestamp,
    created_at timestamp NOT NULL DEFAULT now(),
    updated_at timestamp NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_email_nd ON single_unique_index_nulls_distinct (email);

-- Partitioned table with a unique index. A unique key on a partitioned table
-- must include the partition-key column, so the key is (email, region).
CREATE TABLE partitioned_unique_conflict (
    id INT,
    region VARCHAR(50),
    email VARCHAR(255),
    description text NOT NULL DEFAULT '',
    status varchar NOT NULL DEFAULT '',
    retry_count int NOT NULL DEFAULT 0,
    currency varchar,
    reference_code varchar,
    attempt_count int NOT NULL DEFAULT 0,
    score double precision,
    created_at timestamp NOT NULL DEFAULT now(),
    updated_at timestamp NOT NULL DEFAULT now(),
    due_date date,
    deleted_at timestamp,
    PRIMARY KEY (id, region)
) PARTITION BY LIST (region);
CREATE TABLE partitioned_unique_conflict_east PARTITION OF partitioned_unique_conflict FOR VALUES IN ('east');
CREATE TABLE partitioned_unique_conflict_west PARTITION OF partitioned_unique_conflict FOR VALUES IN ('west');
CREATE TABLE partitioned_unique_conflict_default PARTITION OF partitioned_unique_conflict DEFAULT;
CREATE UNIQUE INDEX idx_partitioned_unique_email ON partitioned_unique_conflict (email, region);
