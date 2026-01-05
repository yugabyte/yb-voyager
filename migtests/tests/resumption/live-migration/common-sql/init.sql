DROP TABLE IF EXISTS public.test_table;

CREATE TABLE public.test_table (
	id int PRIMARY KEY,
	name varchar(255)
);

ALTER TABLE public.test_table REPLICA IDENTITY FULL;

-- datatypes 

drop table if exists num_types;

create table num_types(id serial primary key, v1 smallint, v2 integer,v3 bigint,v4 decimal(6,3), v6 money);

\d num_types

drop table if exists decimal_types;

create table decimal_types(id int PRIMARY KEY, n1 numeric(108,9), n2 numeric(19,2));

drop type if exists week cascade;

CREATE TYPE week AS ENUM ('Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun');

drop table if exists datatypes1;

create table datatypes1(id serial primary key, bool_type boolean,char_type1 CHAR (1),varchar_type VARCHAR(100),byte_type bytea, enum_type week);

\d datatypes1

drop table if exists datetime_type;

create table datetime_type(id serial primary key, v1 date, v2 time, v3 timestamp,v4 TIMESTAMP without TIME ZONE default CURRENT_TIMESTAMP(0));

\d datetime_type

drop table if exists datetime_type2;

create table datetime_type2(id serial primary key, v1 timestamp);

\d datetime_type2

drop table if exists datatypes2;

create table datatypes2(id serial primary key, v1 json, v2 BIT(10), v3 int ARRAY[4], v5 BIT VARYING);

\d datatypes2

drop table if exists null_and_default;
create table null_and_default(id int PRIMARY KEY, b boolean default false, i int default 10, val varchar default 'testdefault');

drop table if exists tsvector_table cascade;

CREATE TABLE tsvector_table (
    id SERIAL PRIMARY KEY,
    title TEXT,
    content TEXT,
    title_tsv TSVECTOR,
    content_tsv TSVECTOR
);

drop table if exists large_row_table;

create table large_row_table (
    id SERIAL PRIMARY KEY,
    data TEXT
);

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