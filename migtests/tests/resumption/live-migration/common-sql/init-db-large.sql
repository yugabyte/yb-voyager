-- init-db-large.sql
-- Creates a large schema footprint for scale testing:
--   - 1000 tables across datatype templates (by default)
--   - 1 cutover table (public.cutover_table)
--   - REPLICA IDENTITY FULL on all created tables + cutover
--
-- Usage (psql):
--   \i init-db-large.sql
--
-- IMPORTANT:
--   This script uses a stored PROCEDURE and does periodic COMMITs to avoid
--   exhausting shared memory / max_locks_per_transaction when creating thousands
--   of tables. Do NOT run it inside an explicit BEGIN/COMMIT transaction block.
--
-- You can override defaults by calling:
--   CALL public.init_db_large('lt', 5000, 200);

CREATE OR REPLACE PROCEDURE public.init_db_large(
  p_prefix text DEFAULT 'lt',
  p_total int DEFAULT 1000,
  p_batch int DEFAULT 200
)
LANGUAGE plpgsql
AS $proc$
DECLARE
  r record;
  i int;
  tname text;
  template_idx int;
  op_count int := 0;
BEGIN
  IF p_total IS NULL OR p_total <= 0 THEN
    RAISE EXCEPTION 'p_total must be > 0 (got %)', p_total;
  END IF;
  IF p_batch IS NULL OR p_batch <= 0 THEN
    RAISE EXCEPTION 'p_batch must be > 0 (got %)', p_batch;
  END IF;

  -- Drop any prior large tables that match "<prefix>_...." naming.
  FOR r IN (
    SELECT tablename
    FROM pg_catalog.pg_tables
    WHERE schemaname = 'public'
      AND tablename LIKE (p_prefix || E'\\_%') ESCAPE E'\\'
    ORDER BY tablename
  )
  LOOP
    EXECUTE format('DROP TABLE IF EXISTS public.%I CASCADE', r.tablename);
    op_count := op_count + 1;
    IF (op_count % p_batch) = 0 THEN
      COMMIT;
    END IF;
  END LOOP;

  -- Drop the enum type used by the template (if present).
  -- (No CASCADE: we expect all dependent large tables already dropped.)
  BEGIN
    EXECUTE 'DROP TYPE IF EXISTS public.week_large';
  EXCEPTION
    WHEN dependent_objects_still_exist THEN
      -- If anything still depends on it, keep going; the create step will reuse it.
      NULL;
  END;

  -- Shared enum type for one of the templates.
  BEGIN
    EXECUTE 'CREATE TYPE public.week_large AS ENUM (''Mon'', ''Tue'', ''Wed'', ''Thu'', ''Fri'', ''Sat'', ''Sun'')';
  EXCEPTION
    WHEN duplicate_object THEN
      NULL;
  END;

  op_count := 0;
  FOR i IN 1..p_total LOOP
    -- Table name: <prefix>_<4-digit>
    tname := p_prefix || '_' || lpad(i::text, 4, '0');

    -- Round-robin templates across datatypes.
    template_idx := (i - 1) % 5;

    IF template_idx = 0 THEN
      -- Numeric-ish
      EXECUTE format(
        'CREATE TABLE public.%I (id serial PRIMARY KEY, v1 smallint, v2 integer, v3 bigint, v4 decimal(6,3), v6 money)',
        tname
      );
    ELSIF template_idx = 1 THEN
      -- High-precision numerics
      EXECUTE format(
        'CREATE TABLE public.%I (id int PRIMARY KEY, n1 numeric(108,9), n2 numeric(19,2))',
        tname
      );
    ELSIF template_idx = 2 THEN
      -- Basic types + enum
      EXECUTE format(
        'CREATE TABLE public.%I (id serial PRIMARY KEY, bool_type boolean, char_type1 char(1), varchar_type varchar(100), byte_type bytea, enum_type public.week_large)',
        tname
      );
    ELSIF template_idx = 3 THEN
      -- Date/time
      EXECUTE format(
        'CREATE TABLE public.%I (id serial PRIMARY KEY, v1 date, v2 time, v3 timestamp, v4 timestamp without time zone DEFAULT CURRENT_TIMESTAMP(0))',
        tname
      );
    ELSE
      -- JSON/bit/array
      EXECUTE format(
        'CREATE TABLE public.%I (id serial PRIMARY KEY, v1 json, v2 bit(10), v3 int[] , v5 bit varying)',
        tname
      );
    END IF;

    -- Set replica identity immediately (keeps work bounded per batch).
    EXECUTE format('ALTER TABLE public.%I REPLICA IDENTITY FULL', tname);

    op_count := op_count + 1;
    IF (op_count % p_batch) = 0 THEN
      COMMIT;
    END IF;
  END LOOP;

  -- Cutover table (recreated each run).
  EXECUTE 'DROP TABLE IF EXISTS public.cutover_table';
  EXECUTE 'CREATE TABLE public.cutover_table (id serial PRIMARY KEY, status text)';
  EXECUTE 'ALTER TABLE public.cutover_table REPLICA IDENTITY FULL';

  COMMIT;
END;
$proc$;

-- Default behavior when this file is executed.
CALL public.init_db_large('lt', 850, 200);
