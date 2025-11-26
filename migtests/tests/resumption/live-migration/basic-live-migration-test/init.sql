DROP TABLE IF EXISTS public.test_table;

CREATE TABLE public.test_table (
	id int PRIMARY KEY,
	name varchar(255)
);

ALTER TABLE public.test_table REPLICA IDENTITY FULL;

-- datatypes 

drop table if exists num_types;

create table num_types(id serial primary key, v1 smallint, v2 integer,v3 bigint,v4 decimal(6,3),v5 numeric, v6 money);

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

-- table for cutover/backlog checks
DROP TABLE IF EXISTS public.cutover_table;
CREATE TABLE public.cutover_table (
	id TEXT PRIMARY KEY
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