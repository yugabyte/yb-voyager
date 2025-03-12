-- setting variables for current session
SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;


CREATE TABLE public."Case_Sensitive_Columns" (
    id integer NOT NULL,
    "user" character varying(50),
    "Last_Name" character varying(50),
    email character varying(50),
    gender character varying(50),
    ip_address character varying(20)
);


CREATE TABLE public."Mixed_Case_Table_Name_Test" (
    id integer NOT NULL,
    first_name character varying(50),
    last_name character varying(50),
    email character varying(50),
    gender character varying(50),
    ip_address character varying(20)
);


CREATE TABLE public."Recipients" (
    id integer NOT NULL,
    first_name public.person_name,
    last_name public.person_name,
    misc public.enum_kind
);


CREATE TABLE public."WITH" (
    id integer NOT NULL,
    "WITH" character varying(100)
)
WITH (fillfactor='75', autovacuum_enabled='true', autovacuum_analyze_scale_factor='0.05');


CREATE TABLE public.audit (
    id text
);


CREATE TABLE public.bigint_multirange_table (
    id integer NOT NULL,
    value_ranges int8multirange
);


CREATE TABLE public.sales_region (
    id integer NOT NULL,
    amount integer,
    branch text,
    region text NOT NULL
)
PARTITION BY LIST (region);


CREATE TABLE public.boston (
    id integer NOT NULL,
    amount integer,
    branch text,
    region text NOT NULL
);


CREATE TABLE public.c (
    id integer,
    c text,
    nc text
);


CREATE TABLE public.parent_table (
    id integer NOT NULL,
    common_column1 text,
    common_column2 integer
);


CREATE TABLE public.child_table (
    specific_column1 date
)
INHERITS (public.parent_table);


CREATE TABLE public.citext_type (
    id integer,
    data public.citext
);


CREATE TABLE public.combined_tbl (
    id integer NOT NULL,
    c cidr,
    maddr macaddr,
    maddr8 macaddr8,
    lsn pg_lsn,
    inds3 interval day to second(3),
    d daterange,
    bitt bit(13),
    bittv bit varying(15),
    address public.address_type,
    raster public.lo,
    arr_enum public.enum_kind[] NOT NULL,
    data public.hstore
);


CREATE TABLE public.date_multirange_table (
    id integer NOT NULL,
    project_dates datemultirange
);


CREATE TABLE public.documents (
    id integer NOT NULL,
    title_tsvector tsvector,
    content_tsvector tsvector,
    list_of_sections text[]
);


CREATE TABLE public.employees (
    employee_id integer NOT NULL,
    first_name character varying(100),
    last_name character varying(100),
    department character varying(50)
);


CREATE TABLE public.employees2 (
    id integer NOT NULL,
    first_name character varying(50) NOT NULL,
    last_name character varying(50) NOT NULL,
    full_name character varying(101) GENERATED ALWAYS AS ((((first_name)::text || ' '::text) || (last_name)::text)) STORED,
    department character varying(50)
);


CREATE TABLE public.employeescopyfromwhere (
    id integer NOT NULL,
    name text NOT NULL,
    age integer NOT NULL
);


CREATE TABLE public.employeescopyonerror (
    id integer NOT NULL,
    name text NOT NULL,
    age integer NOT NULL
);


CREATE TABLE public.employeesforview (
    id integer NOT NULL,
    first_name character varying(50) NOT NULL,
    last_name character varying(50) NOT NULL,
    salary numeric(10,2) NOT NULL
);


CREATE TABLE public.ext_test (
    id integer NOT NULL,
    password text
);


CREATE TABLE public.foo (
    id integer NOT NULL,
    value text
);


CREATE TABLE public.inet_type (
    id integer,
    data inet
);


CREATE TABLE public.int_multirange_table (
    id integer NOT NULL,
    value_ranges int4multirange
);


CREATE TABLE public.library_nested (
    lib_id integer,
    lib_data xml
);


CREATE TABLE public.london (
    id integer NOT NULL,
    amount integer,
    branch text,
    region text NOT NULL
);


CREATE TABLE public.mixed_data_types_table1 (
    id integer NOT NULL,
    point_data point,
    snapshot_data txid_snapshot,
    lseg_data lseg,
    box_data box
);


CREATE TABLE public.mixed_data_types_table2 (
    id integer NOT NULL,
    lsn_data pg_lsn,
    lseg_data lseg,
    path_data path
);


CREATE TABLE public.numeric_multirange_table (
    id integer NOT NULL,
    price_ranges nummultirange
);


CREATE TABLE public.orders (
    item public.item_details,
    number_of_items integer,
    created_at timestamp with time zone DEFAULT now()
);


CREATE TABLE public.orders2 (
    id integer NOT NULL,
    order_number character varying(50),
    status character varying(50) NOT NULL,
    shipped_date date
);


CREATE TABLE public.orders_lateral (
    order_id integer,
    customer_id integer,
    order_details xml
);


CREATE TABLE public.ordersentry (
    order_id integer NOT NULL,
    customer_name text NOT NULL,
    product_name text NOT NULL,
    quantity integer NOT NULL,
    price numeric(10,2) NOT NULL,
    processed_at timestamp without time zone,
    r integer DEFAULT regexp_count('This is an example. Another example. Example is a common word.'::text, 'example'::text)
);


CREATE TABLE public.products (
    item public.item_details,
    added_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now()
);


CREATE TABLE public.sales_unique_nulls_not_distinct (
    store_id integer,
    product_id integer,
    sale_date date
);


CREATE TABLE public.sales_unique_nulls_not_distinct_alter (
    store_id integer,
    product_id integer,
    sale_date date
);


CREATE TABLE public.session_log (
    userid integer NOT NULL,
    phonenumber integer
);


CREATE TABLE public.session_log1 (
    userid integer NOT NULL,
    phonenumber integer
);


CREATE TABLE public.session_log2 (
    userid integer NOT NULL,
    phonenumber integer
);


CREATE TABLE public.sydney (
    id integer NOT NULL,
    amount integer,
    branch text,
    region text NOT NULL
);


CREATE UNLOGGED TABLE public.tbl_unlogged (
    id integer,
    val text
);


CREATE TABLE public.test_exclude_basic (
    id integer,
    name text,
    address text
);


CREATE TABLE public.test_jsonb (
    id integer,
    data jsonb,
    data2 text,
    region text
);


CREATE TABLE public.test_xml_type (
    id integer,
    data xml
);


CREATE TABLE public.timestamp_multirange_table (
    id integer NOT NULL,
    event_times tsmultirange
);


CREATE TABLE public.timestamptz_multirange_table (
    id integer NOT NULL,
    global_event_times tstzmultirange
);


CREATE TABLE public.ts_query_table (
    id integer NOT NULL,
    query tsquery
);


CREATE TABLE public.tt (
    i integer
);


CREATE TABLE public.users_unique_nulls_distinct (
    id integer NOT NULL,
    email text
);
ALTER TABLE ONLY public.users_unique_nulls_distinct ALTER COLUMN email SET COMPRESSION pglz;


CREATE TABLE public.users_unique_nulls_not_distinct (
    id integer NOT NULL,
    email text
);


CREATE TABLE public.users_unique_nulls_not_distinct_index (
    id integer NOT NULL,
    email text
);


CREATE TABLE public.with_example1 (
    id integer NOT NULL,
    name character varying(100)
)
WITH (fillfactor='80', autovacuum_enabled='true', autovacuum_vacuum_scale_factor='0.1', autovacuum_freeze_min_age='10000000');


CREATE TABLE public.with_example2 (
    id integer NOT NULL,
    name character varying(100)
)
WITH (fillfactor='80', autovacuum_enabled='true', autovacuum_vacuum_scale_factor='0.1', autovacuum_freeze_min_age='10000000');


CREATE TABLE schema2."Case_Sensitive_Columns" (
    id integer NOT NULL,
    "user" character varying(50),
    "Last_Name" character varying(50),
    email character varying(50),
    gender character varying(50),
    ip_address character varying(20)
);


CREATE TABLE schema2."Mixed_Case_Table_Name_Test" (
    id integer NOT NULL,
    first_name character varying(50),
    last_name character varying(50),
    email character varying(50),
    gender character varying(50),
    ip_address character varying(20)
);


CREATE TABLE schema2."Recipients" (
    id integer NOT NULL,
    first_name schema2.person_name,
    last_name schema2.person_name,
    misc schema2.enum_kind
);


CREATE TABLE schema2."WITH" (
    id integer NOT NULL,
    "WITH" character varying(100)
)
WITH (fillfactor='75', autovacuum_enabled='true', autovacuum_analyze_scale_factor='0.05');


CREATE TABLE schema2.audit (
    id text
);


CREATE TABLE schema2.bigint_multirange_table (
    id integer NOT NULL,
    value_ranges int8multirange
);


CREATE TABLE schema2.sales_region (
    id integer NOT NULL,
    amount integer,
    branch text,
    region text NOT NULL
)
PARTITION BY LIST (region);


CREATE TABLE schema2.boston (
    id integer NOT NULL,
    amount integer,
    branch text,
    region text NOT NULL
);


CREATE TABLE schema2.c (
    id integer,
    c text,
    nc text
);


CREATE TABLE schema2.parent_table (
    id integer NOT NULL,
    common_column1 text,
    common_column2 integer
);


CREATE TABLE schema2.child_table (
    specific_column1 date
)
INHERITS (schema2.parent_table);


CREATE TABLE schema2.date_multirange_table (
    id integer NOT NULL,
    project_dates datemultirange
);


CREATE TABLE schema2.employees2 (
    id integer NOT NULL,
    first_name character varying(50) NOT NULL,
    last_name character varying(50) NOT NULL,
    full_name character varying(101) GENERATED ALWAYS AS ((((first_name)::text || ' '::text) || (last_name)::text)) STORED,
    department character varying(50)
);


CREATE TABLE schema2.employeesforview (
    id integer NOT NULL,
    first_name character varying(50) NOT NULL,
    last_name character varying(50) NOT NULL,
    salary numeric(10,2) NOT NULL
);


CREATE TABLE schema2.ext_test (
    id integer NOT NULL,
    password text
);


CREATE TABLE schema2.foo (
    id integer NOT NULL,
    value text
);


CREATE TABLE schema2.int_multirange_table (
    id integer NOT NULL,
    value_ranges int4multirange
);


CREATE TABLE schema2.london (
    id integer NOT NULL,
    amount integer,
    branch text,
    region text NOT NULL
);


CREATE TABLE schema2.mixed_data_types_table1 (
    id integer NOT NULL,
    point_data point,
    snapshot_data txid_snapshot,
    lseg_data lseg,
    box_data box
);


CREATE TABLE schema2.mixed_data_types_table2 (
    id integer NOT NULL,
    lsn_data pg_lsn,
    lseg_data lseg,
    path_data path
);


CREATE TABLE schema2.numeric_multirange_table (
    id integer NOT NULL,
    price_ranges nummultirange
);


CREATE TABLE schema2.orders (
    item schema2.item_details,
    number_of_items integer,
    created_at timestamp with time zone DEFAULT now()
);


CREATE TABLE schema2.orders2 (
    id integer NOT NULL,
    order_number character varying(50),
    status character varying(50) NOT NULL,
    shipped_date date
);


CREATE TABLE schema2.products (
    item schema2.item_details,
    added_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now()
);


CREATE TABLE schema2.sales_unique_nulls_not_distinct (
    store_id integer,
    product_id integer,
    sale_date date
);


CREATE TABLE schema2.sales_unique_nulls_not_distinct_alter (
    store_id integer,
    product_id integer,
    sale_date date
);


CREATE TABLE schema2.session_log (
    userid integer NOT NULL,
    phonenumber integer
);


CREATE TABLE schema2.session_log1 (
    userid integer NOT NULL,
    phonenumber integer
);


CREATE TABLE schema2.session_log2 (
    userid integer NOT NULL,
    phonenumber integer
);


CREATE TABLE schema2.sydney (
    id integer NOT NULL,
    amount integer,
    branch text,
    region text NOT NULL
);


CREATE UNLOGGED TABLE schema2.tbl_unlogged (
    id integer,
    val text
);


CREATE TABLE schema2.test_xml_type (
    id integer,
    data xml
);


CREATE TABLE schema2.timestamp_multirange_table (
    id integer NOT NULL,
    event_times tsmultirange
);


CREATE TABLE schema2.timestamptz_multirange_table (
    id integer NOT NULL,
    global_event_times tstzmultirange
);


CREATE TABLE schema2.tt (
    i integer
);


CREATE TABLE schema2.users_unique_nulls_distinct (
    id integer NOT NULL,
    email text
);
ALTER TABLE ONLY schema2.users_unique_nulls_distinct ALTER COLUMN email SET COMPRESSION pglz;


CREATE TABLE schema2.users_unique_nulls_not_distinct (
    id integer NOT NULL,
    email text
);


CREATE TABLE schema2.users_unique_nulls_not_distinct_index (
    id integer NOT NULL,
    email text
);


CREATE TABLE schema2.with_example1 (
    id integer NOT NULL,
    name character varying(100)
)
WITH (fillfactor='80', autovacuum_enabled='true', autovacuum_vacuum_scale_factor='0.1', autovacuum_freeze_min_age='10000000');


CREATE TABLE schema2.with_example2 (
    id integer NOT NULL,
    name character varying(100)
)
WITH (fillfactor='80', autovacuum_enabled='true', autovacuum_vacuum_scale_factor='0.1', autovacuum_freeze_min_age='10000000');


CREATE TABLE test_views.view_table1 (
    id integer NOT NULL,
    first_name character varying(50),
    last_name character varying(50),
    email character varying(50),
    gender character varying(50),
    ip_address character varying(20)
);


CREATE TABLE test_views.view_table2 (
    id integer NOT NULL,
    first_name character varying(50),
    last_name character varying(50),
    email character varying(50),
    gender character varying(50),
    ip_address character varying(20)
);


ALTER TABLE ONLY public."Case_Sensitive_Columns" ALTER COLUMN id SET DEFAULT nextval('public."Case_Sensitive_Columns_id_seq"'::regclass);


ALTER TABLE ONLY public."Mixed_Case_Table_Name_Test" ALTER COLUMN id SET DEFAULT nextval('public."Mixed_Case_Table_Name_Test_id_seq"'::regclass);


ALTER TABLE ONLY public."Recipients" ALTER COLUMN id SET DEFAULT nextval('public."Recipients_id_seq"'::regclass);


ALTER TABLE ONLY public."WITH" ALTER COLUMN id SET DEFAULT nextval('public."WITH_id_seq"'::regclass);


ALTER TABLE ONLY public.bigint_multirange_table ALTER COLUMN id SET DEFAULT nextval('public.bigint_multirange_table_id_seq'::regclass);


ALTER TABLE ONLY public.child_table ALTER COLUMN id SET DEFAULT nextval('public.parent_table_id_seq'::regclass);


ALTER TABLE ONLY public.date_multirange_table ALTER COLUMN id SET DEFAULT nextval('public.date_multirange_table_id_seq'::regclass);


ALTER TABLE ONLY public.employees ALTER COLUMN employee_id SET DEFAULT nextval('public.employees_employee_id_seq'::regclass);


ALTER TABLE ONLY public.employees2 ALTER COLUMN id SET DEFAULT nextval('public.employees2_id_seq'::regclass);


ALTER TABLE ONLY public.employeesforview ALTER COLUMN id SET DEFAULT nextval('public.employeesforview_id_seq'::regclass);


ALTER TABLE ONLY public.ext_test ALTER COLUMN id SET DEFAULT nextval('public.ext_test_id_seq'::regclass);


ALTER TABLE ONLY public.int_multirange_table ALTER COLUMN id SET DEFAULT nextval('public.int_multirange_table_id_seq'::regclass);


ALTER TABLE ONLY public.mixed_data_types_table1 ALTER COLUMN id SET DEFAULT nextval('public.mixed_data_types_table1_id_seq'::regclass);


ALTER TABLE ONLY public.mixed_data_types_table2 ALTER COLUMN id SET DEFAULT nextval('public.mixed_data_types_table2_id_seq'::regclass);


ALTER TABLE ONLY public.numeric_multirange_table ALTER COLUMN id SET DEFAULT nextval('public.numeric_multirange_table_id_seq'::regclass);


ALTER TABLE ONLY public.orders2 ALTER COLUMN id SET DEFAULT nextval('public.orders2_id_seq'::regclass);


ALTER TABLE ONLY public.ordersentry ALTER COLUMN order_id SET DEFAULT nextval('public.ordersentry_order_id_seq'::regclass);


ALTER TABLE ONLY public.parent_table ALTER COLUMN id SET DEFAULT nextval('public.parent_table_id_seq'::regclass);


ALTER TABLE ONLY public.timestamp_multirange_table ALTER COLUMN id SET DEFAULT nextval('public.timestamp_multirange_table_id_seq'::regclass);


ALTER TABLE ONLY public.timestamptz_multirange_table ALTER COLUMN id SET DEFAULT nextval('public.timestamptz_multirange_table_id_seq'::regclass);


ALTER TABLE ONLY public.with_example1 ALTER COLUMN id SET DEFAULT nextval('public.with_example1_id_seq'::regclass);


ALTER TABLE ONLY public.with_example2 ALTER COLUMN id SET DEFAULT nextval('public.with_example2_id_seq'::regclass);


ALTER TABLE ONLY schema2."Case_Sensitive_Columns" ALTER COLUMN id SET DEFAULT nextval('schema2."Case_Sensitive_Columns_id_seq"'::regclass);


ALTER TABLE ONLY schema2."Mixed_Case_Table_Name_Test" ALTER COLUMN id SET DEFAULT nextval('schema2."Mixed_Case_Table_Name_Test_id_seq"'::regclass);


ALTER TABLE ONLY schema2."Recipients" ALTER COLUMN id SET DEFAULT nextval('schema2."Recipients_id_seq"'::regclass);


ALTER TABLE ONLY schema2."WITH" ALTER COLUMN id SET DEFAULT nextval('schema2."WITH_id_seq"'::regclass);


ALTER TABLE ONLY schema2.bigint_multirange_table ALTER COLUMN id SET DEFAULT nextval('schema2.bigint_multirange_table_id_seq'::regclass);


ALTER TABLE ONLY schema2.child_table ALTER COLUMN id SET DEFAULT nextval('schema2.parent_table_id_seq'::regclass);


ALTER TABLE ONLY schema2.date_multirange_table ALTER COLUMN id SET DEFAULT nextval('schema2.date_multirange_table_id_seq'::regclass);


ALTER TABLE ONLY schema2.employees2 ALTER COLUMN id SET DEFAULT nextval('schema2.employees2_id_seq'::regclass);


ALTER TABLE ONLY schema2.employeesforview ALTER COLUMN id SET DEFAULT nextval('schema2.employeesforview_id_seq'::regclass);


ALTER TABLE ONLY schema2.ext_test ALTER COLUMN id SET DEFAULT nextval('schema2.ext_test_id_seq'::regclass);


ALTER TABLE ONLY schema2.int_multirange_table ALTER COLUMN id SET DEFAULT nextval('schema2.int_multirange_table_id_seq'::regclass);


ALTER TABLE ONLY schema2.mixed_data_types_table1 ALTER COLUMN id SET DEFAULT nextval('schema2.mixed_data_types_table1_id_seq'::regclass);


ALTER TABLE ONLY schema2.mixed_data_types_table2 ALTER COLUMN id SET DEFAULT nextval('schema2.mixed_data_types_table2_id_seq'::regclass);


ALTER TABLE ONLY schema2.numeric_multirange_table ALTER COLUMN id SET DEFAULT nextval('schema2.numeric_multirange_table_id_seq'::regclass);


ALTER TABLE ONLY schema2.orders2 ALTER COLUMN id SET DEFAULT nextval('schema2.orders2_id_seq'::regclass);


ALTER TABLE ONLY schema2.parent_table ALTER COLUMN id SET DEFAULT nextval('schema2.parent_table_id_seq'::regclass);


ALTER TABLE ONLY schema2.timestamp_multirange_table ALTER COLUMN id SET DEFAULT nextval('schema2.timestamp_multirange_table_id_seq'::regclass);


ALTER TABLE ONLY schema2.timestamptz_multirange_table ALTER COLUMN id SET DEFAULT nextval('schema2.timestamptz_multirange_table_id_seq'::regclass);


ALTER TABLE ONLY schema2.with_example1 ALTER COLUMN id SET DEFAULT nextval('schema2.with_example1_id_seq'::regclass);


ALTER TABLE ONLY schema2.with_example2 ALTER COLUMN id SET DEFAULT nextval('schema2.with_example2_id_seq'::regclass);


ALTER TABLE ONLY test_views.view_table1 ALTER COLUMN id SET DEFAULT nextval('test_views.view_table1_id_seq'::regclass);


ALTER TABLE ONLY test_views.view_table2 ALTER COLUMN id SET DEFAULT nextval('test_views.view_table2_id_seq'::regclass);


ALTER TABLE ONLY public."Case_Sensitive_Columns"
    ADD CONSTRAINT "Case_Sensitive_Columns_pkey" PRIMARY KEY (id);


ALTER TABLE ONLY public."Mixed_Case_Table_Name_Test"
    ADD CONSTRAINT "Mixed_Case_Table_Name_Test_pkey" PRIMARY KEY (id);


ALTER TABLE ONLY public."Recipients"
    ADD CONSTRAINT "Recipients_pkey" PRIMARY KEY (id);


ALTER TABLE ONLY public."WITH"
    ADD CONSTRAINT "WITH_pkey" PRIMARY KEY (id);


ALTER TABLE ONLY public.bigint_multirange_table
    ADD CONSTRAINT bigint_multirange_table_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.sales_region
    ADD CONSTRAINT sales_region_pkey PRIMARY KEY (id, region);


ALTER TABLE ONLY public.boston
    ADD CONSTRAINT boston_pkey PRIMARY KEY (id, region);


ALTER TABLE ONLY public.combined_tbl
    ADD CONSTRAINT combined_tbl_bittv_key UNIQUE (bittv);


ALTER TABLE ONLY public.combined_tbl
    ADD CONSTRAINT combined_tbl_pkey PRIMARY KEY (id, arr_enum);


ALTER TABLE ONLY public.date_multirange_table
    ADD CONSTRAINT date_multirange_table_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.employees2
    ADD CONSTRAINT employees2_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.employees
    ADD CONSTRAINT employees_pkey PRIMARY KEY (employee_id);


ALTER TABLE ONLY public.employeescopyfromwhere
    ADD CONSTRAINT employeescopyfromwhere_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.employeescopyonerror
    ADD CONSTRAINT employeescopyonerror_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.employeesforview
    ADD CONSTRAINT employeesforview_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.foo
    ADD CONSTRAINT foo_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.int_multirange_table
    ADD CONSTRAINT int_multirange_table_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.london
    ADD CONSTRAINT london_pkey PRIMARY KEY (id, region);


ALTER TABLE ONLY public.mixed_data_types_table1
    ADD CONSTRAINT mixed_data_types_table1_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.mixed_data_types_table2
    ADD CONSTRAINT mixed_data_types_table2_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.test_exclude_basic
    ADD CONSTRAINT no_same_name_address EXCLUDE USING btree (name WITH =, address WITH =);


ALTER TABLE ONLY public.numeric_multirange_table
    ADD CONSTRAINT numeric_multirange_table_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.orders2
    ADD CONSTRAINT orders2_order_number_key UNIQUE (order_number) DEFERRABLE;


ALTER TABLE ONLY public.orders2
    ADD CONSTRAINT orders2_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.ordersentry
    ADD CONSTRAINT ordersentry_pkey PRIMARY KEY (order_id);


ALTER TABLE ONLY public.parent_table
    ADD CONSTRAINT parent_table_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.sales_unique_nulls_not_distinct
    ADD CONSTRAINT sales_unique_nulls_not_distin_store_id_product_id_sale_date_key UNIQUE NULLS NOT DISTINCT (store_id, product_id, sale_date);


ALTER TABLE ONLY public.sales_unique_nulls_not_distinct_alter
    ADD CONSTRAINT sales_unique_nulls_not_distinct_alter_unique UNIQUE NULLS NOT DISTINCT (store_id, product_id, sale_date);


ALTER TABLE ONLY public.sydney
    ADD CONSTRAINT sydney_pkey PRIMARY KEY (id, region);


ALTER TABLE ONLY public.timestamp_multirange_table
    ADD CONSTRAINT timestamp_multirange_table_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.timestamptz_multirange_table
    ADD CONSTRAINT timestamptz_multirange_table_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.combined_tbl
    ADD CONSTRAINT uk UNIQUE (lsn);


ALTER TABLE ONLY public.users_unique_nulls_distinct
    ADD CONSTRAINT users_unique_nulls_distinct_email_key UNIQUE (email);


ALTER TABLE ONLY public.users_unique_nulls_distinct
    ADD CONSTRAINT users_unique_nulls_distinct_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.users_unique_nulls_not_distinct
    ADD CONSTRAINT users_unique_nulls_not_distinct_email_key UNIQUE NULLS NOT DISTINCT (email);


ALTER TABLE ONLY public.users_unique_nulls_not_distinct_index
    ADD CONSTRAINT users_unique_nulls_not_distinct_index_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.users_unique_nulls_not_distinct
    ADD CONSTRAINT users_unique_nulls_not_distinct_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.with_example1
    ADD CONSTRAINT with_example1_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.with_example2
    ADD CONSTRAINT with_example2_pkey PRIMARY KEY (id);


ALTER TABLE ONLY schema2."Case_Sensitive_Columns"
    ADD CONSTRAINT "Case_Sensitive_Columns_pkey" PRIMARY KEY (id);


ALTER TABLE ONLY schema2."Mixed_Case_Table_Name_Test"
    ADD CONSTRAINT "Mixed_Case_Table_Name_Test_pkey" PRIMARY KEY (id);


ALTER TABLE ONLY schema2."Recipients"
    ADD CONSTRAINT "Recipients_pkey" PRIMARY KEY (id);


ALTER TABLE ONLY schema2."WITH"
    ADD CONSTRAINT "WITH_pkey" PRIMARY KEY (id);


ALTER TABLE ONLY schema2.bigint_multirange_table
    ADD CONSTRAINT bigint_multirange_table_pkey PRIMARY KEY (id);


ALTER TABLE ONLY schema2.sales_region
    ADD CONSTRAINT sales_region_pkey PRIMARY KEY (id, region);


ALTER TABLE ONLY schema2.boston
    ADD CONSTRAINT boston_pkey PRIMARY KEY (id, region);


ALTER TABLE ONLY schema2.date_multirange_table
    ADD CONSTRAINT date_multirange_table_pkey PRIMARY KEY (id);


ALTER TABLE ONLY schema2.employees2
    ADD CONSTRAINT employees2_pkey PRIMARY KEY (id);


ALTER TABLE ONLY schema2.employeesforview
    ADD CONSTRAINT employeesforview_pkey PRIMARY KEY (id);


ALTER TABLE ONLY schema2.foo
    ADD CONSTRAINT foo_pkey PRIMARY KEY (id);


ALTER TABLE ONLY schema2.int_multirange_table
    ADD CONSTRAINT int_multirange_table_pkey PRIMARY KEY (id);


ALTER TABLE ONLY schema2.london
    ADD CONSTRAINT london_pkey PRIMARY KEY (id, region);


ALTER TABLE ONLY schema2.mixed_data_types_table1
    ADD CONSTRAINT mixed_data_types_table1_pkey PRIMARY KEY (id);


ALTER TABLE ONLY schema2.mixed_data_types_table2
    ADD CONSTRAINT mixed_data_types_table2_pkey PRIMARY KEY (id);


ALTER TABLE ONLY schema2.numeric_multirange_table
    ADD CONSTRAINT numeric_multirange_table_pkey PRIMARY KEY (id);


ALTER TABLE ONLY schema2.orders2
    ADD CONSTRAINT orders2_order_number_key UNIQUE (order_number) DEFERRABLE;


ALTER TABLE ONLY schema2.orders2
    ADD CONSTRAINT orders2_pkey PRIMARY KEY (id);


ALTER TABLE ONLY schema2.parent_table
    ADD CONSTRAINT parent_table_pkey PRIMARY KEY (id);


ALTER TABLE ONLY schema2.sales_unique_nulls_not_distinct
    ADD CONSTRAINT sales_unique_nulls_not_distin_store_id_product_id_sale_date_key UNIQUE NULLS NOT DISTINCT (store_id, product_id, sale_date);


ALTER TABLE ONLY schema2.sales_unique_nulls_not_distinct_alter
    ADD CONSTRAINT sales_unique_nulls_not_distinct_alter_unique UNIQUE NULLS NOT DISTINCT (store_id, product_id, sale_date);


ALTER TABLE ONLY schema2.sydney
    ADD CONSTRAINT sydney_pkey PRIMARY KEY (id, region);


ALTER TABLE ONLY schema2.timestamp_multirange_table
    ADD CONSTRAINT timestamp_multirange_table_pkey PRIMARY KEY (id);


ALTER TABLE ONLY schema2.timestamptz_multirange_table
    ADD CONSTRAINT timestamptz_multirange_table_pkey PRIMARY KEY (id);


ALTER TABLE ONLY schema2.users_unique_nulls_distinct
    ADD CONSTRAINT users_unique_nulls_distinct_email_key UNIQUE (email);


ALTER TABLE ONLY schema2.users_unique_nulls_distinct
    ADD CONSTRAINT users_unique_nulls_distinct_pkey PRIMARY KEY (id);


ALTER TABLE ONLY schema2.users_unique_nulls_not_distinct
    ADD CONSTRAINT users_unique_nulls_not_distinct_email_key UNIQUE NULLS NOT DISTINCT (email);


ALTER TABLE ONLY schema2.users_unique_nulls_not_distinct_index
    ADD CONSTRAINT users_unique_nulls_not_distinct_index_pkey PRIMARY KEY (id);


ALTER TABLE ONLY schema2.users_unique_nulls_not_distinct
    ADD CONSTRAINT users_unique_nulls_not_distinct_pkey PRIMARY KEY (id);


ALTER TABLE ONLY schema2.with_example1
    ADD CONSTRAINT with_example1_pkey PRIMARY KEY (id);


ALTER TABLE ONLY schema2.with_example2
    ADD CONSTRAINT with_example2_pkey PRIMARY KEY (id);


ALTER TABLE ONLY test_views.view_table1
    ADD CONSTRAINT view_table1_pkey PRIMARY KEY (id);


ALTER TABLE ONLY test_views.view_table2
    ADD CONSTRAINT view_table2_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.test_jsonb
    ADD CONSTRAINT test_jsonb_id_region_fkey FOREIGN KEY (id, region) REFERENCES public.sales_region(id, region);


ALTER TABLE ONLY public.sales_region ATTACH PARTITION public.boston FOR VALUES IN ('Boston');


ALTER TABLE ONLY public.sales_region ATTACH PARTITION public.london FOR VALUES IN ('London');


ALTER TABLE ONLY public.sales_region ATTACH PARTITION public.sydney FOR VALUES IN ('Sydney');


ALTER TABLE ONLY schema2.sales_region ATTACH PARTITION schema2.boston FOR VALUES IN ('Boston');


ALTER TABLE ONLY schema2.sales_region ATTACH PARTITION schema2.london FOR VALUES IN ('London');


ALTER TABLE ONLY schema2.sales_region ATTACH PARTITION schema2.sydney FOR VALUES IN ('Sydney');


