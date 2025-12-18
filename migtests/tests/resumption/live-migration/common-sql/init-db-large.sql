-- init-db-large.sql
-- Creates a large schema footprint for scale testing:
--   - 1000 tables across datatype templates (by default)
--   - 1 cutover table (public.cutover_table)
--   - REPLICA IDENTITY FULL on all created tables + cutover
--
-- Usage (psql):
--   \i init-db-large.sql
--
-- You can override defaults by calling:
--   SELECT public.init_db_large('lt', 1000);

CREATE OR REPLACE FUNCTION public.drop_large_tables(p_prefix text)
RETURNS void
LANGUAGE plpgsql
AS $fn$
DECLARE
  r record;
BEGIN
  -- Drop any prior large tables that match "<prefix>_...." naming.
  FOR r IN (
    SELECT tablename
    FROM pg_catalog.pg_tables
    WHERE schemaname = 'public'
      AND tablename LIKE (p_prefix || E'\\_%') ESCAPE E'\\'
  )
  LOOP
    EXECUTE format('DROP TABLE IF EXISTS public.%I CASCADE', r.tablename);
  END LOOP;

  -- Drop the enum type used by large tables (if present).
  EXECUTE 'DROP TYPE IF EXISTS public.week_large CASCADE';
END;
$fn$;

CREATE OR REPLACE FUNCTION public.create_large_tables(p_prefix text, p_total int)
RETURNS void
LANGUAGE plpgsql
AS $fn$
DECLARE
  i int;
  tname text;
  template_idx int;
BEGIN
  IF p_total IS NULL OR p_total <= 0 THEN
    RAISE EXCEPTION 'p_total must be > 0 (got %)', p_total;
  END IF;

  -- Shared enum type for one of the templates.
  BEGIN
    EXECUTE 'CREATE TYPE public.week_large AS ENUM (''Mon'', ''Tue'', ''Wed'', ''Thu'', ''Fri'', ''Sat'', ''Sun'')';
  EXCEPTION
    WHEN duplicate_object THEN
      NULL;
  END;

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
  END LOOP;
END;
$fn$;

CREATE OR REPLACE FUNCTION public.create_cutover_table()
RETURNS void
LANGUAGE plpgsql
AS $fn$
BEGIN
  EXECUTE 'DROP TABLE IF EXISTS public.cutover_table';
  EXECUTE 'CREATE TABLE public.cutover_table (id serial PRIMARY KEY, status text)';
END;
$fn$;

CREATE OR REPLACE FUNCTION public.set_replica_identity_full_for_large_tables(p_prefix text)
RETURNS void
LANGUAGE plpgsql
AS $fn$
DECLARE
  r record;
BEGIN
  FOR r IN (
    SELECT table_name
    FROM information_schema.tables
    WHERE table_schema = 'public'
      AND table_type = 'BASE TABLE'
      AND (
        table_name = 'cutover_table'
        OR table_name LIKE (p_prefix || E'\\_%') ESCAPE E'\\'
      )
  )
  LOOP
    EXECUTE format('ALTER TABLE public.%I REPLICA IDENTITY FULL', r.table_name);
  END LOOP;
END;
$fn$;

CREATE OR REPLACE FUNCTION public.init_db_large(p_prefix text DEFAULT 'lt', p_total int DEFAULT 1000)
RETURNS void
LANGUAGE plpgsql
AS $fn$
BEGIN
  PERFORM public.drop_large_tables(p_prefix);
  PERFORM public.create_large_tables(p_prefix, p_total);
  PERFORM public.create_cutover_table();
  PERFORM public.set_replica_identity_full_for_large_tables(p_prefix);
END;
$fn$;

-- Default behavior when this file is executed.
SELECT public.init_db_large('lt', 5000);
