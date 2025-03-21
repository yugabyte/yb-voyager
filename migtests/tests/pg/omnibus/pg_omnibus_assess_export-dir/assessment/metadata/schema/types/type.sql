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


CREATE TYPE agg_ex.avg_state AS (
	total bigint,
	count bigint
);


CREATE TYPE base_type_examples.base_type (
    INTERNALLENGTH = variable,
    INPUT = base_type_examples.base_fn_in,
    OUTPUT = base_type_examples.base_fn_out,
    ALIGNMENT = int4,
    STORAGE = plain
);


CREATE TYPE base_type_examples.int42 (
    INTERNALLENGTH = 4,
    INPUT = base_type_examples.int42_in,
    OUTPUT = base_type_examples.int42_out,
    DEFAULT = '42',
    ALIGNMENT = int4,
    STORAGE = plain,
    PASSEDBYVALUE
);


CREATE TYPE base_type_examples.text_w_default (
    INTERNALLENGTH = variable,
    INPUT = base_type_examples.text_w_default_in,
    OUTPUT = base_type_examples.text_w_default_out,
    DEFAULT = 'zippo',
    ALIGNMENT = int4,
    STORAGE = plain
);


CREATE TYPE base_type_examples.default_test_row AS (
	f1 base_type_examples.text_w_default,
	f2 base_type_examples.int42
);


CREATE TYPE base_type_examples.myvarchar (
    INTERNALLENGTH = variable,
    INPUT = base_type_examples.myvarcharin,
    OUTPUT = base_type_examples.myvarcharout,
    RECEIVE = base_type_examples.myvarcharrecv,
    SEND = base_type_examples.myvarcharsend,
    TYPMOD_IN = varchartypmodin,
    TYPMOD_OUT = varchartypmodout,
    ANALYZE = ts_typanalyze,
    SUBSCRIPT = raw_array_subscript_handler,
    ALIGNMENT = int4,
    STORAGE = extended
);


CREATE TYPE base_type_examples.shell;


CREATE TYPE composite_type_examples.basic_comp_type AS (
	f1 integer,
	f2 text
);


CREATE TYPE composite_type_examples.enum_abc AS ENUM (
    'a',
    'b',
    'c'
);


CREATE TYPE composite_type_examples.nested AS (
	foo composite_type_examples.basic_comp_type,
	bar composite_type_examples.enum_abc
);


CREATE TYPE create_cast.abc AS ENUM (
    'a',
    'b',
    'c'
);


CREATE TYPE enum_example.bug_severity AS ENUM (
    'low',
    'med',
    'high'
);


CREATE TYPE enum_example.bug_status AS ENUM (
    'new',
    'open',
    'closed'
);


CREATE TYPE enum_example.bug_info AS (
	status enum_example.bug_status,
	severity enum_example.bug_severity
);


CREATE TYPE foreign_db_example.example_type AS (
	a integer,
	b text
);


CREATE TYPE range_type_example.float8_range AS RANGE (
    subtype = double precision,
    multirange_type_name = range_type_example.float8_multirange,
    subtype_diff = range_type_example.my_float8mi
);


