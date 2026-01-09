drop table if exists large_row_table;

create table large_row_table (
    id SERIAL PRIMARY KEY,
    data TEXT
);

\d large_row_table

set temp_file_limit=2500000000;

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