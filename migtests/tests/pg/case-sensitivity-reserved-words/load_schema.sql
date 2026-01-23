\i schema.sql

drop schema if exists schema2 cascade;

create schema schema2;

set search_path to schema2;

\i schema.sql

CREATE SCHEMA "Schema";
CREATE SCHEMA "pg-schema";
CREATE SCHEMA "order";


-- partition across multiple schemas (one case-insensitive, one case-sensitive)
CREATE TABLE public."Sales_region" (
    id integer NOT NULL,
    amount integer,
    branch text,
    region text NOT NULL,
    PRIMARY KEY(id, region)
)
PARTITION BY LIST (region);

CREATE TABLE "Schema"."Boston" (
    id integer NOT NULL,
    amount integer,
    branch text,
    region text NOT NULL,
    PRIMARY KEY(id, region)
);

CREATE TABLE "Schema"."London" (
    id integer NOT NULL,
    amount integer,
    branch text,
    region text NOT NULL,
    PRIMARY KEY(id, region)
);

CREATE TABLE "Schema"."Sydney" (
    id integer NOT NULL,
    amount integer,
    branch text,
    region text NOT NULL,
    PRIMARY KEY(id, region)
);

-- case sensitive with multi level partitioning  
CREATE TABLE "Schema".customers (
    id integer NOT NULL,
    statuses text NOT NULL,
    arr numeric NOT NULL,
    PRIMARY KEY(id, statuses, arr)
)
PARTITION BY LIST (statuses);


CREATE TABLE "Schema".cust_active (
    id integer NOT NULL,
    statuses text NOT NULL,
    arr numeric NOT NULL,
    PRIMARY KEY(id, statuses, arr)
)
PARTITION BY RANGE (arr);

CREATE TABLE "Schema".cust_arr_large (
    id integer NOT NULL,
    statuses text NOT NULL,
    arr numeric NOT NULL,
    PRIMARY KEY(id, statuses, arr)
)
PARTITION BY HASH (id);

CREATE TABLE "Schema".cust_arr_small (
    id integer NOT NULL,
    statuses text NOT NULL,
    arr numeric NOT NULL,
    PRIMARY KEY(id, statuses, arr)
)
PARTITION BY HASH (id);


CREATE TABLE "Schema".cust_other (
    id integer NOT NULL,
    statuses text NOT NULL,
    arr numeric NOT NULL,
    PRIMARY KEY (id, statuses, arr)
);


CREATE TABLE "Schema".cust_part11 (
    id integer NOT NULL,
    statuses text NOT NULL,
    arr numeric NOT NULL,
    PRIMARY KEY (id, statuses, arr)
);


CREATE TABLE "Schema".cust_part12 (
    id integer NOT NULL,
    statuses text NOT NULL,
    arr numeric NOT NULL,
    PRIMARY KEY (id, statuses, arr)
);

CREATE TABLE "Schema".cust_part21 (
    id integer NOT NULL,
    statuses text NOT NULL,
    arr numeric NOT NULL,
    PRIMARY KEY (id, statuses, arr)
);

CREATE TABLE "Schema".cust_part22 (
    id integer NOT NULL,
    statuses text NOT NULL,
    arr numeric NOT NULL,
    PRIMARY KEY (id, statuses, arr)
);

-- case sensitive schema for sequence

CREATE TABLE "Schema".tbl_seq1 (
    id integer NOT NULL,
    val text,
    PRIMARY KEY (id)
);

CREATE SEQUENCE "Schema".tbl_seq1_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE "Schema".tbl_seq1_id_seq OWNED BY "Schema".tbl_seq1.id;
ALTER TABLE ONLY "Schema".tbl_seq1 ALTER COLUMN id SET DEFAULT nextval('"Schema".tbl_seq1_id_seq'::regclass);


CREATE TABLE "pg-schema"."TestTable" (
    id integer,
    val text,
    PRIMARY KEY (id)
);

CREATE SEQUENCE "pg-schema"."TestTable_id_seq"
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE "pg-schema"."TestTable_id_seq" OWNED BY "pg-schema"."TestTable".id;
ALTER TABLE ONLY "pg-schema"."TestTable" ALTER COLUMN id SET DEFAULT nextval('"pg-schema"."TestTable_id_seq"'::regclass);

ALTER TABLE ONLY public."Sales_region" ATTACH PARTITION "Schema"."Boston" FOR VALUES IN ('Boston');

ALTER TABLE ONLY public."Sales_region" ATTACH PARTITION "Schema"."London" FOR VALUES IN ('London');

ALTER TABLE ONLY public."Sales_region" ATTACH PARTITION "Schema"."Sydney" FOR VALUES IN ('Sydney');


ALTER TABLE ONLY "Schema".customers ATTACH PARTITION "Schema".cust_active FOR VALUES IN ('ACTIVE', 'RECURRING', 'REACTIVATED');

ALTER TABLE ONLY "Schema".cust_active ATTACH PARTITION "Schema".cust_arr_large FOR VALUES FROM ('101') TO (MAXVALUE);

ALTER TABLE ONLY "Schema".cust_active ATTACH PARTITION "Schema".cust_arr_small FOR VALUES FROM (MINVALUE) TO ('101');

ALTER TABLE ONLY "Schema".customers ATTACH PARTITION "Schema".cust_other DEFAULT;

ALTER TABLE ONLY "Schema".cust_arr_small ATTACH PARTITION "Schema".cust_part11 FOR VALUES WITH (modulus 2, remainder 0);

ALTER TABLE ONLY "Schema".cust_arr_small ATTACH PARTITION "Schema".cust_part12 FOR VALUES WITH (modulus 2, remainder 1);

ALTER TABLE ONLY "Schema".cust_arr_large ATTACH PARTITION "Schema".cust_part21 FOR VALUES WITH (modulus 2, remainder 0);

ALTER TABLE ONLY "Schema".cust_arr_large ATTACH PARTITION "Schema".cust_part22 FOR VALUES WITH (modulus 2, remainder 1);


-- case sensitive schema for reserved word
CREATE TABLE "order".test1 (
    id integer NOT NULL,
    val text,
    PRIMARY KEY (id)
);

CREATE SEQUENCE "order".test1_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE "order".test1_id_seq OWNED BY "order".test1.id;
ALTER TABLE ONLY "order".test1 ALTER COLUMN id SET DEFAULT nextval('"order".test1_id_seq'::regclass);

-- case sensitive schema for identity column
CREATE TABLE "pg-schema".test2 (
    id integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    val text
);


CREATE TABLE "Schema"."Test2" (
    id integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    val text
);

