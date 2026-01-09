drop table if exists large_col_table;

CREATE TABLE large_col_table (
    id SERIAL PRIMARY KEY, 
    text_col_1 TEXT, 
    text_col_2 TEXT, 
    text_col_3 TEXT, 
    text_col_4 TEXT, 
    text_col_5 TEXT, 
    text_col_6 TEXT, 
    text_col_7 TEXT, 
    text_col_8 TEXT, 
    text_col_9 TEXT, 
    text_col_10 TEXT, 
    text_col_11 TEXT, 
    text_col_12 TEXT, 
    text_col_13 TEXT, 
    text_col_14 TEXT, 
    text_col_15 TEXT, 
    text_col_16 TEXT, 
    text_col_17 TEXT, 
    text_col_18 TEXT, 
    text_col_19 TEXT, 
    text_col_20 TEXT, 
    json_col_1 JSON, 
    json_col_2 JSON, 
    json_col_3 JSON, 
    json_col_4 JSON, 
    json_col_5 JSON, 
    jsonb_col_1 JSONB, 
    jsonb_col_2 JSONB, 
    jsonb_col_3 JSONB, 
    jsonb_col_4 JSONB, 
    jsonb_col_5 JSONB, 
    int_arr_col_1 INTEGER[], 
    int_arr_col_2 INTEGER[], 
    int_arr_col_3 INTEGER[], 
    int_arr_col_4 INTEGER[], 
    int_arr_col_5 INTEGER[], 
    char_arr_col_1 VARCHAR[], 
    char_arr_col_2 VARCHAR[], 
    char_arr_col_3 VARCHAR[], 
    char_arr_col_4 VARCHAR[], 
    char_arr_col_5 VARCHAR[], 
    smallint_col SMALLINT, 
    bigint_col BIGINT, 
    boolean_col BOOLEAN, 
    date_col DATE, 
    time_col TIME, 
    ts_col TIMESTAMP, 
    real_col REAL, 
    double_col DOUBLE PRECISION, 
    numeric_col NUMERIC(10,2), 
    uuid_col UUID
);

\d large_col_table

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