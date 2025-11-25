DROP TABLE IF EXISTS public.test_table;

CREATE TABLE public.test_table (
	id int PRIMARY KEY,
	name varchar(255)
);

ALTER TABLE public.test_table REPLICA IDENTITY FULL;

-- table for cutover/backlog checks
DROP TABLE IF EXISTS public.cutover_table;
CREATE TABLE public.cutover_table (
	id TEXT PRIMARY KEY
);

-- smoke DDL/DDL-neutral command
SELECT 1;