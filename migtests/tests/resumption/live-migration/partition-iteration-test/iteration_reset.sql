-- Clean slate between iterations.
-- Uses DELETE instead of TRUNCATE because YB's TRUNCATE recreates tablets
-- and can exceed the cluster tablet limit with many partitions.
-- DDL (ALTER SEQUENCE / DROP) is kept separate from DML (DELETE) so that
-- a catalog-version change inside one DO block doesn't invalidate the next
-- DML statement (YB raises MISMATCHED_SCHEMA when DDL+DML are mixed).

-- DML: clear data
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_tables WHERE schemaname='public' AND tablename='events') THEN
        DELETE FROM public.events;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_tables WHERE schemaname='public' AND tablename='sales_region') THEN
        DELETE FROM public.sales_region;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_tables WHERE schemaname='public' AND tablename='emp') THEN
        DELETE FROM public.emp;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_tables WHERE schemaname='public' AND tablename='cutover_table') THEN
        DELETE FROM public.cutover_table;
    END IF;
END $$;

-- DDL: reset sequences (each statement = its own transaction)
ALTER SEQUENCE IF EXISTS public.events_id_seq RESTART WITH 1;
ALTER SEQUENCE IF EXISTS public.sales_region_id_seq RESTART WITH 1;
ALTER SEQUENCE IF EXISTS public.emp_emp_id_seq RESTART WITH 1;

-- DDL: drop validation artifacts
DROP TABLE IF EXISTS public.migration_validate_segments;
DROP FUNCTION IF EXISTS public.compute_schema_segment_hashes(text, text, integer);
DROP FUNCTION IF EXISTS public.compute_table_segment_hashes(text, text, text, integer);
DROP FUNCTION IF EXISTS public.custom_hash_code_from_text(text);
