-- A. Schema with very long name (63 characters)
DROP SCHEMA IF EXISTS "schema_long_name_1234567890123456789012345678901234567890123456";

CREATE SCHEMA "schema_long_name_1234567890123456789012345678901234567890123456";

-- B. Table with very long name inside that schema
DROP TABLE IF EXISTS "schema_long_name_1234567890123456789012345678901234567890123456"."table_long_name_12345678901234567890123456789012345678901234567";

CREATE TABLE "schema_long_name_1234567890123456789012345678901234567890123456"."table_long_name_12345678901234567890123456789012345678901234567" (
    id SERIAL PRIMARY KEY,
    payload TEXT
);

-- C. Table with very large column names
DROP TABLE IF EXISTS public.wide_column_names;

CREATE TABLE public.wide_column_names (
    id SERIAL PRIMARY KEY,
    "col_name_12345678901234567890123456789012345678901234567890123456" INT,
    "col_name_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz1234" INT
);

-- D. Table with very large column count (Max ~1600)
DROP TABLE IF EXISTS public.max_columns_table;

DO $$
DECLARE
    i INTEGER;
    sql_stmt TEXT;
BEGIN
    sql_stmt := 'CREATE TABLE public.max_columns_table (id SERIAL PRIMARY KEY';
    
    FOR i IN 1..1500 LOOP
        sql_stmt := sql_stmt || ', col_' || i || ' INTEGER DEFAULT 0';
    END LOOP;
    
    sql_stmt := sql_stmt || ');';
    EXECUTE sql_stmt;
END $$;

-- E. Large number of Indexes in one table
DROP TABLE IF EXISTS public.heavy_index_table;

CREATE TABLE public.heavy_index_table (
    id SERIAL PRIMARY KEY,
    val1 INT,
    val2 INT,
    payload TEXT
);

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

DO $$
DECLARE
    i INTEGER;
BEGIN
    FOR i IN 1..50 LOOP
        EXECUTE format('DROP INDEX IF EXISTS idx_unique_heavy_%s', i);
        EXECUTE format('CREATE UNIQUE INDEX idx_unique_heavy_%s ON public.heavy_unique_index_table (uuid_col)', i);
    END LOOP;
END $$;

-- G. Large number of indexes across tables
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

-- H. Composite Primary Key (Standard Types)
DROP TABLE IF EXISTS public.composite_pk_simple;

CREATE TABLE public.composite_pk_simple (
    region_id INT,
    branch_code TEXT,
    account_id INT,
    balance DECIMAL(10,2),
    PRIMARY KEY (region_id, branch_code, account_id)
);

-- I. Composite Primary Key (Mixed/Complex Types)
DROP TABLE IF EXISTS public.composite_pk_complex;

CREATE TABLE public.composite_pk_complex (
    transaction_id UUID DEFAULT gen_random_uuid(),
    event_time TIMESTAMP DEFAULT now(),
    category_code VARCHAR(10),
    payload JSONB,
    PRIMARY KEY (transaction_id, event_time)
);

-- J. UUID Primary Key
DROP TABLE IF EXISTS public.uuid_pk_table;

CREATE TABLE public.uuid_pk_table (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    user_data TEXT
);

-- K. Binary (BYTEA) Primary Key
DROP TABLE IF EXISTS public.binary_pk_table;

CREATE TABLE public.binary_pk_table (
    id BYTEA PRIMARY KEY,
    file_name TEXT,
    mime_type TEXT
);

-- L. "The Kitchen Sink" PK (Composite including UUID and Enum)
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
