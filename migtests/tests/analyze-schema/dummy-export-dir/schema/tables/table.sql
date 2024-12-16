-- cases for insufficient columns in PK constraint definition in CREATE TABLE to be reported in analyze-schema
CREATE TABLE sales_data (
        sales_id numeric NOT NULL,
        sales_date timestamp,
        sales_amount numeric,
        PRIMARY KEY (sales_id)
) PARTITION BY RANGE (sales_date) ;

-- cases for partition by expression cannot contain PK/Unique Key to be reported in analyze-schema 
CREATE TABLE salaries2 (
	emp_no bigint NOT NULL,
	salary bigint NOT NULL,
	from_date timestamp NOT NULL,
	to_date timestamp NOT NULL,
	PRIMARY KEY (emp_no,from_date)
) PARTITION BY RANGE (extract(epoch from date(from_date))) ;

CREATE TABLE sales (
	cust_id bigint NOT NULL,
	name varchar(40),
	store_id varchar(20) NOT NULL,
	bill_no bigint NOT NULL,
	bill_date timestamp NOT NULL,
	amount decimal(8,2) NOT NULL,
	PRIMARY KEY (bill_date)
) PARTITION BY RANGE (extract(year from date(bill_date))) ;

-- cases for multi column list partition, to be reported during analyze-schema
CREATE TABLE test_1 (
	id numeric NOT NULL,
	country_code varchar(3),
	record_type varchar(5),
	descriptions varchar(50),
	PRIMARY KEY (id)
) PARTITION BY LIST (country_code, record_type) ;

CREATE TABLE test_2 (
	id numeric NOT NULL PRIMARY KEY,
	country_code varchar(3),
	record_type varchar(5),
	descriptions varchar(50)
) PARTITION BY LIST (country_code, record_type) ;

CREATE TABLE test_non_pk_multi_column_list (
	id numeric NOT NULL PRIMARY KEY,
	country_code varchar(3),
	record_type varchar(5),
	descriptions varchar(50)
) PARTITION BY LIST (country_code, record_type) ;

-- no PK constraint, no need to report during analyze-schema
CREATE TABLE test_3 (
	id numeric,
	country_code varchar,
	record_type varchar(2)
) PARTITION BY RANGE (id) ;

CREATE TABLE test_4 (
	id numeric,
	country_code varchar
) PARTITION BY LIST (id) ;

-- all partition keys are not included in the PK constraint, to be reported during analyze-schema
CREATE TABLE test_5 (
	id numeric NOT NULL,
	country_code varchar(3),
	record_type varchar(5),
	descriptions varchar(50),
	PRIMARY KEY (id)
) PARTITION BY RANGE (country_code, record_type) ;

CREATE TABLE test_6 (
	id numeric NOT NULL,
	country_code varchar(3),
	record_type varchar(5),
	descriptions varchar(50),
	PRIMARY KEY (id,country_code)
) PARTITION BY RANGE (country_code, record_type) ;

CREATE TABLE test_7 (
	id numeric NOT NULL,
	country_code varchar(3),
	record_type varchar(5),
	descriptions varchar(50),
	PRIMARY KEY (id,country_code)
) PARTITION BY RANGE (descriptions, record_type) ;


CREATE TABLE test_8 (
	order_id bigint NOT NULL,
	order_date timestamp,
	order_mode varchar(8),
	customer_id integer,
	order_mode smallint,
	order_total double precision,
	sales_rep_id integer,
	promotion_id integer,
	PRIMARY KEY (order_id,order_mode,customer_id,order_total,sales_rep_id)
) PARTITION BY RANGE (promotion_id, order_date, sales_rep_id) ;

-- all partition key are in PK, no need to report during analyze-schema
CREATE TABLE test_9 (
	order_id bigint NOT NULL,
	order_date timestamp,
	order_mode varchar(8),
	customer_id integer,
	order_mode smallint,
	order_total double precision,
	sales_rep_id integer,
	promotion_id integer,
	PRIMARY KEY (order_id,order_mode,order_date,order_total,sales_rep_id)
) PARTITION BY RANGE (order_total, order_date, sales_rep_id) ;

--Reindexing not supported
REINDEX TABLE my_table;

--Generated stored test cases with different datatype formats
CREATE TABLE order_details (
    detail_id integer NOT NULL,
    quantity integer,
    price_per_unit numeric,
    amount numeric GENERATED ALWAYS AS (((quantity)::numeric * price_per_unit)) STORED
);

CREATE TABLE public.employees4 (
    id integer NOT NULL,
    first_name character varying(50) NOT NULL,
    last_name character varying(50) NOT NULL,
    full_name character varying(101) GENERATED ALWAYS AS ((((first_name)::text || ' '::text) || (last_name)::text)) STORED
);

CREATE TABLE enum_example.bugs (
    id integer NOT NULL,
    description text,
    status enum_example.bug_status,
    _status enum_example.bug_status GENERATED ALWAYS AS (status) STORED,
    severity enum_example.bug_severity,
    _severity enum_example.bug_severity GENERATED ALWAYS AS (severity) STORED,
    info enum_example.bug_info GENERATED ALWAYS AS (enum_example.make_bug_info(status, severity)) STORED
);

--like cases
CREATE TABLE table_xyz
  (LIKE xyz INCLUDING DEFAULTS INCLUDING CONSTRAINTS);

CREATE TABLE table_abc
  (LIKE abc INCLUDING ALL);

--inherits cases
CREATE TABLE table_1 () INHERITS (xyz);

--oids case
Create table table_test (col1 text, col2 int) with (OIDS = TRUE);

--PK on interval column
create table test_interval(
    frequency interval primary key,
	col1 int
);

--alter table cases
ALTER TABLE oldschema.tbl_name SET SCHEMA newschema;

alter table table_name alter column column_name set statistics 100;

alter table test alter column col set STORAGE EXTERNAL;

alter table test_1 alter column col1 set (attribute_option=value);

--disable rule case on alter table
alter table test DISABLE RULE example_rule;

-- for the storage parameters case
ALTER TABLE ONLY public.example ADD CONSTRAINT example_email_key UNIQUE (email) WITH (fillfactor='70');

alter table abc cluster on xyz;

alter table test SET WITHOUT CLUSTER;

alter table test_3 INHERIT test_2;

ALTER TABLE distributors VALIDATE CONSTRAINT distfk;

ALTER TABLE abc
ADD CONSTRAINT cnstr_id
 UNIQUE (id)
DEFERRABLE;

--adding pk to partitioned table
CREATE TABLE public.range_columns_partition_test (
    a bigint NOT NULL,
    b bigint NOT NULL
)
PARTITION BY RANGE (a, b);

ALTER TABLE ONLY public.range_columns_partition_test
    ADD CONSTRAINT range_columns_partition_test_pkey PRIMARY KEY (a, b);

CREATE TABLE public.range_columns_partition_test_copy (
    a bigint NOT NULL,
    b bigint NOT NULL,
	PRIMARY KEY(a, b)
)
PARTITION BY RANGE (a, b);



-- datatype mapping not supported
CREATE TABLE anydata_test (
	id numeric,
	content ANYDATA
) ;

CREATE TABLE anydataset_test (
	id numeric,
	content ANYDATASET
) ;

CREATE TABLE anytype_test (
	id numeric,
	content ANYTYPE
) ;

CREATE TABLE uritype_test (
	id numeric,
	content URITYPE
) ;
-- valid
Alter table only parent_tbl add constraint party_profile_pk primary key (party_profile_id);

--Unsupported PG syntax caught by regex for ALTER TABLE OF..
Alter table only party_profile_part of parent_tbl add constraint party_profile_pk primary key (party_profile_id);

--exclusion constraints
CREATE TABLE "Test"(
	id int, 
	room_id int, 
	time_range trange,
	roomid int,
	timerange tsrange, 
	EXCLUDE USING gist (room_id WITH =, time_range WITH &&),
	CONSTRAINT no_time_overlap_constr EXCLUDE USING gist (roomid WITH =, timerange WITH &&)
);

CREATE TABLE public.meeting (
    id integer NOT NULL,
    room_id integer NOT NULL,
    time_range tsrange NOT NULL
);

ALTER TABLE ONLY public.meeting
    ADD CONSTRAINT no_time_overlap EXCLUDE USING gist (room_id WITH =, time_range WITH &&);


--deferrable constraints all should be reported except foreign key constraints
CREATE TABLE public.pr (
	id integer PRIMARY KEY,
	name text
);

CREATE TABLE public.foreign_def_test (
	id integer,
	id1 integer,
	val text
);

ALTER TABLE ONLY public.foreign_def_test
    ADD CONSTRAINT foreign_def_test_id_fkey FOREIGN KEY (id) REFERENCES public.pr(id) DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE public.users (
	id int PRIMARY KEY,
	email text
);

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_email_key UNIQUE (email) DEFERRABLE;

create table foreign_def_test1(id int references pr(id) INITIALLY DEFERRED, c1 int);

create table foreign_def_test2(id int, name text, FOREIGN KEY (id) REFERENCES public.pr(id) DEFERRABLE INITIALLY DEFERRED);

create table unique_def_test(id int UNIQUE DEFERRABLE, c1 int);

create table unique_def_test1(id int, c1 int, UNIQUE(id)  DEFERRABLE INITIALLY DEFERRED);

CREATE TABLE test_xml_type(id int, data xml);

CREATE TABLE test_xid_type(id int, data xid);


CREATE TABLE public.test_jsonb (
    id integer,
    data jsonb,
	data2 text
);

CREATE TABLE public.inet_type (
    id integer,
    data inet
);

CREATE TABLE public.citext_type (
    id integer,
    data public.citext
);

CREATE TABLE public.documents (
    id integer NOT NULL,
    title_tsvector tsvector,
    content_tsvector tsvector,
	list_of_sections text[]
);

CREATE TABLE public.ts_query_table (
    id int generated by default as identity,
    query tsquery
);

create table combined_tbl (
	id int, 
	c cidr, 
	ci circle, 
	b box, 
	j json UNIQUE,
	l line, 
	ls lseg, 
	maddr macaddr, 
	maddr8 macaddr8, 
	p point, 
	lsn pg_lsn, 
	p1 path, 
	p2 polygon, 
	id1 txid_snapshot,
	bitt bit (13),
	bittv bit varying(15),
	CONSTRAINT pk PRIMARY KEY (id, maddr8)
);

ALTER TABLE combined_tbl 
		ADD CONSTRAINT combined_tbl_unique UNIQUE(id, bitt);

CREATE TABLE combined_tbl1(
	id int,
	t tsrange, 
	d daterange, 
	tz tstzrange, 
	n numrange, 
	i4 int4range UNIQUE, 
	i8 int8range,
	inym INTERVAL YEAR TO MONTH,
	inds INTERVAL DAY TO SECOND(9),
	PRIMARY KEY(id, t, n)
);

ALTER TABLE combined_tbl1 
		ADD CONSTRAINT combined_tbl1_unique UNIQUE(id, d);

CREATE UNLOGGED TABLE tbl_unlogged (id int, val text);

CREATE TABLE test_udt (
	employee_id SERIAL PRIMARY KEY,
	employee_name VARCHAR(100),
	home_address address_type,
	some_field enum_test,
	home_address1 non_public.address_type1
);

CREATE TABLE test_arr_enum (
	id int,
	arr text[],
	arr_enum enum_test[]
);

CREATE TABLE public.locations (
    id integer NOT NULL,
    name character varying(100),
    geom geometry(Point,4326)
 );

 CREATE TABLE public.xml_data_example (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255),
    description XML DEFAULT xmlparse(document '<product><name>Default Product</name><price>100.00</price><category>Electronics</category></product>'),
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE image (title text, raster lo);