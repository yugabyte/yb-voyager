TRUNCATE TABLE public.events;
TRUNCATE TABLE public.cutover_table;
ALTER SEQUENCE public.events_id_seq RESTART WITH 1;
DROP TABLE IF EXISTS public.migration_validate_segments;
DROP FUNCTION IF EXISTS public.compute_schema_segment_hashes(text, text, integer);
DROP FUNCTION IF EXISTS public.compute_table_segment_hashes(text, text, text, integer);
DROP FUNCTION IF EXISTS public.custom_hash_code_from_text(text);
