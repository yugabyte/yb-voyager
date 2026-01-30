-- 1. Setup: Define constants for limits
-- Postgres default NAMEDATALEN is 64, meaning names truncate at 63 bytes.
-- Max columns per table is usually 1600.

-- A. Schema with very long name (63 characters)
DROP SCHEMA IF EXISTS "schema_long_name_1234567890123456789012345678901234567890123456";

CREATE SCHEMA "schema_long_name_1234567890123456789012345678901234567890123456";

-- B. Table with very long name inside that schema
DROP TABLE IF EXISTS "schema_long_name_1234567890123456789012345678901234567890123456"."table_long_name_12345678901234567890123456789012345678901234567";

CREATE TABLE "schema_long_name_1234567890123456789012345678901234567890123456"."table_long_name_12345678901234567890123456789012345678901234567" (
    id SERIAL PRIMARY KEY,
    payload TEXT
);

-- set replica identity full for the table
ALTER TABLE "schema_long_name_1234567890123456789012345678901234567890123456"."table_long_name_12345678901234567890123456789012345678901234567" REPLICA IDENTITY FULL;

\d "schema_long_name_1234567890123456789012345678901234567890123456"."table_long_name_12345678901234567890123456789012345678901234567";

-- C. Table with very large column names
-- We create a table where every column name hits the 63-byte limit.
DROP TABLE IF EXISTS public.wide_column_names;

CREATE TABLE public.wide_column_names (
    id SERIAL PRIMARY KEY,
    "col_name_12345678901234567890123456789012345678901234567890123456" INT,
    "col_name_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz1234" INT
);

\d public.wide_column_names;

-- D. Table with very large column count (Max ~1600)
-- We use a DO block to generate this dynamically to avoid a massive text block.
DROP TABLE IF EXISTS public.max_columns_table;

DO $$
DECLARE
    i INTEGER;
    sql_stmt TEXT;
BEGIN
    sql_stmt := 'CREATE TABLE public.max_columns_table (id SERIAL PRIMARY KEY';
    
    -- Add 1500 columns (leaving room for system columns/tuple overhead)
    FOR i IN 1..1500 LOOP
        sql_stmt := sql_stmt || ', col_' || i || ' INTEGER DEFAULT 0';
    END LOOP;
    
    sql_stmt := sql_stmt || ');';
    EXECUTE sql_stmt;
END $$;

\d public.max_columns_table;

-- E. Large number of Indexes in one table
-- Creates a table and adds 100 individual indexes to it.
DROP TABLE IF EXISTS public.heavy_index_table;

CREATE TABLE public.heavy_index_table (
    id SERIAL PRIMARY KEY,
    val1 INT,
    val2 INT,
    payload TEXT
);

\d public.heavy_index_table;

DO $$
DECLARE
    i INTEGER;
BEGIN
    FOR i IN 1..100 LOOP
        EXECUTE format('DROP INDEX IF EXISTS idx_heavy_%s', i);
        EXECUTE format('CREATE INDEX idx_heavy_%s ON public.heavy_index_table (val1, val2)', i);
    END LOOP;
END $$;

-- F. Large number of Unique Indexes in one table
DROP TABLE IF EXISTS public.heavy_unique_index_table;

CREATE TABLE public.heavy_unique_index_table (
    id SERIAL PRIMARY KEY,
    uuid_col UUID DEFAULT gen_random_uuid(),
    email_col TEXT
);

\d public.heavy_unique_index_table;

DO $$
DECLARE
    i INTEGER;
BEGIN
    FOR i IN 1..50 LOOP
        -- Using partial indexes to allow multiple unique constraints on similar data if needed
        -- or just redundant unique indexes to stress metadata parsing.
        EXECUTE format('DROP INDEX IF EXISTS idx_unique_heavy_%s', i);
        EXECUTE format('CREATE UNIQUE INDEX idx_unique_heavy_%s ON public.heavy_unique_index_table (uuid_col)', i);
    END LOOP;
END $$;

-- G. Large number of indexes across tables
-- Creates 50 tables, each with 5 indexes.
DO $$
DECLARE
    t INTEGER;
    idx INTEGER;
BEGIN
    FOR t IN 1..50 LOOP
        EXECUTE format('DROP TABLE IF EXISTS public.multi_table_%s', t);
        EXECUTE format('CREATE TABLE public.multi_table_%s (id serial primary key, val int)', t);
        FOR idx IN 1..5 LOOP
            EXECUTE format('DROP INDEX IF EXISTS idx_multi_table_%s_%s', t, idx);
            EXECUTE format('CREATE INDEX idx_multi_table_%s_%s ON public.multi_table_%s (val)', t, idx, t);
        END LOOP;
    END LOOP;
END $$;

-- 0. Prerequisite: Ensure pgcrypto is available for UUID generation (if using Postgres < 13)
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- H. Composite Primary Key (Standard Types)
-- CDC Challenge: The tool must handle updates by referencing multiple columns to identify the row.
DROP TABLE IF EXISTS public.composite_pk_simple;

CREATE TABLE public.composite_pk_simple (
    region_id INT,
    branch_code TEXT,
    account_id INT,
    balance DECIMAL(10,2),
    PRIMARY KEY (region_id, branch_code, account_id)
);

\d public.composite_pk_simple;

-- I. Composite Primary Key (Mixed/Complex Types)
-- CDC Challenge: Handling timestamp formatting inside a composite key string.
DROP TABLE IF EXISTS public.composite_pk_complex;

CREATE TABLE public.composite_pk_complex (
    transaction_id UUID DEFAULT gen_random_uuid(),
    event_time TIMESTAMP DEFAULT now(),
    category_code VARCHAR(10),
    payload JSONB,
    PRIMARY KEY (transaction_id, event_time)
);

\d public.composite_pk_complex;

-- J. UUID Primary Key
-- CDC Challenge: Some older tools treat UUIDs as strings, others as 128-bit integers.
DROP TABLE IF EXISTS public.uuid_pk_table;

CREATE TABLE public.uuid_pk_table (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    user_data TEXT
);

\d public.uuid_pk_table;

-- K. Binary (BYTEA) Primary Key
-- CDC Challenge: This is a "breaker." JSON cannot store raw binary.
-- The CDC tool MUST encode this (usually Base64 or Hex) before emitting the event.
-- If it tries to emit raw bytes, the JSON serializer will crash.
DROP TABLE IF EXISTS public.binary_pk_table;

CREATE TABLE public.binary_pk_table (
    id BYTEA PRIMARY KEY,
    file_name TEXT,
    mime_type TEXT
);

\d public.binary_pk_table;

-- L. "The Kitchen Sink" PK (Composite including UUID and Enum)
-- First, create a custom enum type
DROP TYPE IF EXISTS task_status;

CREATE TYPE task_status AS ENUM ('queued', 'running', 'done');

DROP TABLE IF EXISTS public.kitchen_sink_pk;

CREATE TABLE public.kitchen_sink_pk (
    job_id UUID DEFAULT gen_random_uuid(),
    status task_status,
    shard_id INT,
    result_data TEXT,
    PRIMARY KEY (job_id, status, shard_id)
);

\d public.kitchen_sink_pk;

-- table for cutover/backlog checks
DROP TABLE IF EXISTS public.cutover_table;
CREATE TABLE public.cutover_table (
	id SERIAL PRIMARY KEY,
    status TEXT
);

-- set replica identity full for all tables
DO $CUSTOM$ 
    DECLARE
		r record;
    BEGIN
        FOR r IN (SELECT table_schema,table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE') 
        LOOP
            EXECUTE 'ALTER TABLE ' || r.table_schema || '."' || r.table_name || '" REPLICA IDENTITY FULL';
        END LOOP;
    END $CUSTOM$;