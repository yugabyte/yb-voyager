-- Clean slate between iterations.
-- Uses DELETE instead of TRUNCATE because YB's TRUNCATE recreates tablets
-- and can exceed the cluster tablet limit with many partitions.
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

    IF EXISTS (SELECT 1 FROM pg_sequences WHERE schemaname='public' AND sequencename='events_id_seq') THEN
        ALTER SEQUENCE public.events_id_seq RESTART WITH 1;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_sequences WHERE schemaname='public' AND sequencename='sales_region_id_seq') THEN
        ALTER SEQUENCE public.sales_region_id_seq RESTART WITH 1;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_sequences WHERE schemaname='public' AND sequencename='emp_emp_id_seq') THEN
        ALTER SEQUENCE public.emp_emp_id_seq RESTART WITH 1;
    END IF;
END $$;

DROP TABLE IF EXISTS public.migration_validate_segments;
DROP FUNCTION IF EXISTS public.compute_schema_segment_hashes(text, text, integer);
DROP FUNCTION IF EXISTS public.compute_table_segment_hashes(text, text, text, integer);
DROP FUNCTION IF EXISTS public.custom_hash_code_from_text(text);
