DROP TABLE IF EXISTS public.cutover_table;
DROP TABLE IF EXISTS public.migration_validate_segments;

-- Drop validation helper functions if present
DROP FUNCTION IF EXISTS public.compute_schema_segment_hashes(text, text, integer);
DROP FUNCTION IF EXISTS public.compute_table_segment_hashes(text, text, text, integer);
DROP FUNCTION IF EXISTS public.custom_hash_code_from_text(text);