-- smoke DDL/DDL-neutral command
SELECT 1;

-- table for cutover/backlog checks
DROP TABLE IF EXISTS public.cutover_table;
CREATE TABLE public.cutover_table (
	id TEXT PRIMARY KEY
);