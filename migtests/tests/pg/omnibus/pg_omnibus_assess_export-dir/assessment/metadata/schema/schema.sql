--
-- PostgreSQL database dump
--

-- Dumped from database version 17.2 (Ubuntu 17.2-1.pgdg22.04+1)
-- Dumped by pg_dump version 17.2 (Ubuntu 17.2-1.pgdg22.04+1)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: agg_ex; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA agg_ex;


--
-- Name: am_examples; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA am_examples;


--
-- Name: base_type_examples; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA base_type_examples;


--
-- Name: collation_ex; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA collation_ex;


--
-- Name: composite_type_examples; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA composite_type_examples;


--
-- Name: conversion_example; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA conversion_example;


--
-- Name: create_cast; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA create_cast;


--
-- Name: domain_examples; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA domain_examples;


--
-- Name: enum_example; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA enum_example;


--
-- Name: extension_example; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA extension_example;


--
-- Name: fn_examples; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA fn_examples;


--
-- Name: foreign_db_example; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA foreign_db_example;


--
-- Name: idx_ex; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA idx_ex;


--
-- Name: ordinary_tables; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA ordinary_tables;


--
-- Name: public; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA public;


--
-- Name: range_type_example; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA range_type_example;


--
-- Name: regress_rls_schema; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA regress_rls_schema;


--
-- Name: trigger_test; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA trigger_test;


--
-- Name: bad_us; Type: COLLATION; Schema: collation_ex; Owner: -
--

CREATE COLLATION collation_ex.bad_us (provider = libc, locale = 'en_US.utf8');


--
-- Name: german_phonebook; Type: COLLATION; Schema: collation_ex; Owner: -
--

CREATE COLLATION collation_ex.german_phonebook (provider = icu, locale = 'de-u-co-phonebk');


--
-- Name: us; Type: COLLATION; Schema: collation_ex; Owner: -
--

CREATE COLLATION collation_ex.us (provider = libc, locale = 'en_US.utf8');


--
-- Name: hstore; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS hstore WITH SCHEMA extension_example;


--
-- Name: postgres_fdw; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS postgres_fdw WITH SCHEMA foreign_db_example;


--
-- Name: avg_state; Type: TYPE; Schema: agg_ex; Owner: -
--

CREATE TYPE agg_ex.avg_state AS (
	total bigint,
	count bigint
);


--
-- Name: base_type; Type: SHELL TYPE; Schema: base_type_examples; Owner: -
--

CREATE TYPE base_type_examples.base_type;


--
-- Name: base_fn_in(cstring); Type: FUNCTION; Schema: base_type_examples; Owner: -
--

CREATE FUNCTION base_type_examples.base_fn_in(cstring) RETURNS base_type_examples.base_type
    LANGUAGE internal IMMUTABLE STRICT
    AS $$boolin$$;


--
-- Name: base_fn_out(base_type_examples.base_type); Type: FUNCTION; Schema: base_type_examples; Owner: -
--

CREATE FUNCTION base_type_examples.base_fn_out(base_type_examples.base_type) RETURNS cstring
    LANGUAGE internal IMMUTABLE STRICT
    AS $$boolout$$;


--
-- Name: base_type; Type: TYPE; Schema: base_type_examples; Owner: -
--

CREATE TYPE base_type_examples.base_type (
    INTERNALLENGTH = variable,
    INPUT = base_type_examples.base_fn_in,
    OUTPUT = base_type_examples.base_fn_out,
    ALIGNMENT = int4,
    STORAGE = plain
);


--
-- Name: int42; Type: SHELL TYPE; Schema: base_type_examples; Owner: -
--

CREATE TYPE base_type_examples.int42;


--
-- Name: int42_in(cstring); Type: FUNCTION; Schema: base_type_examples; Owner: -
--

CREATE FUNCTION base_type_examples.int42_in(cstring) RETURNS base_type_examples.int42
    LANGUAGE internal IMMUTABLE STRICT
    AS $$int4in$$;


--
-- Name: int42_out(base_type_examples.int42); Type: FUNCTION; Schema: base_type_examples; Owner: -
--

CREATE FUNCTION base_type_examples.int42_out(base_type_examples.int42) RETURNS cstring
    LANGUAGE internal IMMUTABLE STRICT
    AS $$int4out$$;


--
-- Name: int42; Type: TYPE; Schema: base_type_examples; Owner: -
--

CREATE TYPE base_type_examples.int42 (
    INTERNALLENGTH = 4,
    INPUT = base_type_examples.int42_in,
    OUTPUT = base_type_examples.int42_out,
    DEFAULT = '42',
    ALIGNMENT = int4,
    STORAGE = plain,
    PASSEDBYVALUE
);


--
-- Name: text_w_default; Type: SHELL TYPE; Schema: base_type_examples; Owner: -
--

CREATE TYPE base_type_examples.text_w_default;


--
-- Name: text_w_default_in(cstring); Type: FUNCTION; Schema: base_type_examples; Owner: -
--

CREATE FUNCTION base_type_examples.text_w_default_in(cstring) RETURNS base_type_examples.text_w_default
    LANGUAGE internal IMMUTABLE STRICT
    AS $$textin$$;


--
-- Name: text_w_default_out(base_type_examples.text_w_default); Type: FUNCTION; Schema: base_type_examples; Owner: -
--

CREATE FUNCTION base_type_examples.text_w_default_out(base_type_examples.text_w_default) RETURNS cstring
    LANGUAGE internal IMMUTABLE STRICT
    AS $$textout$$;


--
-- Name: text_w_default; Type: TYPE; Schema: base_type_examples; Owner: -
--

CREATE TYPE base_type_examples.text_w_default (
    INTERNALLENGTH = variable,
    INPUT = base_type_examples.text_w_default_in,
    OUTPUT = base_type_examples.text_w_default_out,
    DEFAULT = 'zippo',
    ALIGNMENT = int4,
    STORAGE = plain
);


--
-- Name: default_test_row; Type: TYPE; Schema: base_type_examples; Owner: -
--

CREATE TYPE base_type_examples.default_test_row AS (
	f1 base_type_examples.text_w_default,
	f2 base_type_examples.int42
);


--
-- Name: myvarchar; Type: SHELL TYPE; Schema: base_type_examples; Owner: -
--

CREATE TYPE base_type_examples.myvarchar;


--
-- Name: myvarcharin(cstring, oid, integer); Type: FUNCTION; Schema: base_type_examples; Owner: -
--

CREATE FUNCTION base_type_examples.myvarcharin(cstring, oid, integer) RETURNS base_type_examples.myvarchar
    LANGUAGE internal IMMUTABLE STRICT PARALLEL SAFE
    AS $$varcharin$$;


--
-- Name: myvarcharout(base_type_examples.myvarchar); Type: FUNCTION; Schema: base_type_examples; Owner: -
--

CREATE FUNCTION base_type_examples.myvarcharout(base_type_examples.myvarchar) RETURNS cstring
    LANGUAGE internal IMMUTABLE STRICT PARALLEL SAFE
    AS $$varcharout$$;


--
-- Name: myvarcharrecv(internal, oid, integer); Type: FUNCTION; Schema: base_type_examples; Owner: -
--

CREATE FUNCTION base_type_examples.myvarcharrecv(internal, oid, integer) RETURNS base_type_examples.myvarchar
    LANGUAGE internal STABLE STRICT PARALLEL SAFE
    AS $$varcharrecv$$;


--
-- Name: myvarcharsend(base_type_examples.myvarchar); Type: FUNCTION; Schema: base_type_examples; Owner: -
--

CREATE FUNCTION base_type_examples.myvarcharsend(base_type_examples.myvarchar) RETURNS bytea
    LANGUAGE internal STABLE STRICT PARALLEL SAFE
    AS $$varcharsend$$;


--
-- Name: myvarchar; Type: TYPE; Schema: base_type_examples; Owner: -
--

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


--
-- Name: myvarchardom; Type: DOMAIN; Schema: base_type_examples; Owner: -
--

CREATE DOMAIN base_type_examples.myvarchardom AS base_type_examples.myvarchar;


--
-- Name: shell; Type: TYPE; Schema: base_type_examples; Owner: -
--

CREATE TYPE base_type_examples.shell;


--
-- Name: basic_comp_type; Type: TYPE; Schema: composite_type_examples; Owner: -
--

CREATE TYPE composite_type_examples.basic_comp_type AS (
	f1 integer,
	f2 text
);


--
-- Name: enum_abc; Type: TYPE; Schema: composite_type_examples; Owner: -
--

CREATE TYPE composite_type_examples.enum_abc AS ENUM (
    'a',
    'b',
    'c'
);


--
-- Name: nested; Type: TYPE; Schema: composite_type_examples; Owner: -
--

CREATE TYPE composite_type_examples.nested AS (
	foo composite_type_examples.basic_comp_type,
	bar composite_type_examples.enum_abc
);


--
-- Name: abc; Type: TYPE; Schema: create_cast; Owner: -
--

CREATE TYPE create_cast.abc AS ENUM (
    'a',
    'b',
    'c'
);


--
-- Name: is_positive(integer); Type: FUNCTION; Schema: domain_examples; Owner: -
--

CREATE FUNCTION domain_examples.is_positive(i integer) RETURNS boolean
    LANGUAGE sql IMMUTABLE
    AS $$
  SELECT i > 0
$$;


--
-- Name: positive_number; Type: DOMAIN; Schema: domain_examples; Owner: -
--

CREATE DOMAIN domain_examples.positive_number AS integer
	CONSTRAINT should_be_positive CHECK (domain_examples.is_positive(VALUE));


--
-- Name: is_even(domain_examples.positive_number); Type: FUNCTION; Schema: domain_examples; Owner: -
--

CREATE FUNCTION domain_examples.is_even(i domain_examples.positive_number) RETURNS boolean
    LANGUAGE sql IMMUTABLE
    AS $$
  SELECT (i % 2) = 0
$$;


--
-- Name: positive_even_number; Type: DOMAIN; Schema: domain_examples; Owner: -
--

CREATE DOMAIN domain_examples.positive_even_number AS domain_examples.positive_number
	CONSTRAINT should_be_even CHECK (domain_examples.is_even(VALUE));


--
-- Name: us_postal_code; Type: DOMAIN; Schema: domain_examples; Owner: -
--

CREATE DOMAIN domain_examples.us_postal_code AS text
	CONSTRAINT check_postal_code_regex CHECK (((VALUE ~ '^\d{5}$'::text) OR (VALUE ~ '^\d{5}-\d{4}$'::text)));


--
-- Name: bug_severity; Type: TYPE; Schema: enum_example; Owner: -
--

CREATE TYPE enum_example.bug_severity AS ENUM (
    'low',
    'med',
    'high'
);


--
-- Name: bug_status; Type: TYPE; Schema: enum_example; Owner: -
--

CREATE TYPE enum_example.bug_status AS ENUM (
    'new',
    'open',
    'closed'
);


--
-- Name: bug_info; Type: TYPE; Schema: enum_example; Owner: -
--

CREATE TYPE enum_example.bug_info AS (
	status enum_example.bug_status,
	severity enum_example.bug_severity
);


--
-- Name: example_type; Type: TYPE; Schema: foreign_db_example; Owner: -
--

CREATE TYPE foreign_db_example.example_type AS (
	a integer,
	b text
);


--
-- Name: is_positive(integer); Type: FUNCTION; Schema: foreign_db_example; Owner: -
--

CREATE FUNCTION foreign_db_example.is_positive(i integer) RETURNS boolean
    LANGUAGE sql IMMUTABLE
    AS $$
  SELECT i > 0
$$;


--
-- Name: positive_number; Type: DOMAIN; Schema: foreign_db_example; Owner: -
--

CREATE DOMAIN foreign_db_example.positive_number AS integer NOT NULL
	CONSTRAINT positive_number_check CHECK (foreign_db_example.is_positive(VALUE));


--
-- Name: my_float8mi(double precision, double precision); Type: FUNCTION; Schema: range_type_example; Owner: -
--

CREATE FUNCTION range_type_example.my_float8mi(a double precision, b double precision) RETURNS double precision
    LANGUAGE sql IMMUTABLE
    AS $$
    SELECT float8mi(a,b)
  $$;


--
-- Name: float8_range; Type: TYPE; Schema: range_type_example; Owner: -
--

CREATE TYPE range_type_example.float8_range AS RANGE (
    subtype = double precision,
    multirange_type_name = range_type_example.float8_multirange,
    subtype_diff = range_type_example.my_float8mi
);


--
-- Name: transmogrify(create_cast.abc); Type: FUNCTION; Schema: create_cast; Owner: -
--

CREATE FUNCTION create_cast.transmogrify(input create_cast.abc) RETURNS integer
    LANGUAGE sql IMMUTABLE
    AS $$
  SELECT 1;
$$;


--
-- Name: avg_finalfn(agg_ex.avg_state); Type: FUNCTION; Schema: agg_ex; Owner: -
--

CREATE FUNCTION agg_ex.avg_finalfn(state agg_ex.avg_state) RETURNS integer
    LANGUAGE plpgsql
    AS $$
begin
	if state is null then
		return NULL;
	else
		return state.total / state.count;
	end if;
end
$$;


--
-- Name: avg_transfn(agg_ex.avg_state, integer); Type: FUNCTION; Schema: agg_ex; Owner: -
--

CREATE FUNCTION agg_ex.avg_transfn(state agg_ex.avg_state, n integer) RETURNS agg_ex.avg_state
    LANGUAGE plpgsql
    AS $$
declare new_state agg_ex.avg_state;
begin
	raise notice 'agg_ex.avg_transfn called with %', n;
	if state is null then
		if n is not null then
			new_state.total := n;
			new_state.count := 1;
			return new_state;
		end if;
		return null;
	elsif n is not null then
		state.total := state.total + n;
		state.count := state.count + 1;
		return state;
	end if;

	return null;
end
$$;


--
-- Name: sum_finalfn(agg_ex.avg_state); Type: FUNCTION; Schema: agg_ex; Owner: -
--

CREATE FUNCTION agg_ex.sum_finalfn(state agg_ex.avg_state) RETURNS integer
    LANGUAGE plpgsql
    AS $$
begin
	if state is null then
		return NULL;
	else
		return state.total;
	end if;
end
$$;


--
-- Name: fake_op(point, base_type_examples.int42); Type: FUNCTION; Schema: base_type_examples; Owner: -
--

CREATE FUNCTION base_type_examples.fake_op(point, base_type_examples.int42) RETURNS boolean
    LANGUAGE sql IMMUTABLE
    AS $$ select true $$;


--
-- Name: get_default_test(); Type: FUNCTION; Schema: base_type_examples; Owner: -
--

CREATE FUNCTION base_type_examples.get_default_test() RETURNS SETOF base_type_examples.default_test_row
    LANGUAGE sql
    AS $$
  SELECT * FROM base_type_examples.default_test;
$$;


--
-- Name: get_basic(); Type: FUNCTION; Schema: composite_type_examples; Owner: -
--

CREATE FUNCTION composite_type_examples.get_basic() RETURNS SETOF composite_type_examples.basic_comp_type
    LANGUAGE sql
    AS $$
  SELECT f1, f2 FROM composite_type_examples.equivalent_rowtype
$$;


--
-- Name: make_bug_info(enum_example.bug_status, enum_example.bug_severity); Type: FUNCTION; Schema: enum_example; Owner: -
--

CREATE FUNCTION enum_example.make_bug_info(status enum_example.bug_status, severity enum_example.bug_severity) RETURNS enum_example.bug_info
    LANGUAGE sql IMMUTABLE
    AS $$
    SELECT status, severity
  $$;


--
-- Name: should_raise_alarm(enum_example.bug_info); Type: FUNCTION; Schema: enum_example; Owner: -
--

CREATE FUNCTION enum_example.should_raise_alarm(info enum_example.bug_info) RETURNS boolean
    LANGUAGE sql IMMUTABLE
    AS $$
    SELECT info.status = 'new' AND info.severity = 'high'
  $$;


--
-- Name: _hstore(record); Type: FUNCTION; Schema: extension_example; Owner: -
--

CREATE FUNCTION extension_example._hstore(r record) RETURNS extension_example.hstore
    LANGUAGE plpgsql IMMUTABLE STRICT
    AS $$
    BEGIN
      return extension_example.hstore(r);
    END;
  $$;


--
-- Name: depends_on_table_column(); Type: FUNCTION; Schema: fn_examples; Owner: -
--

CREATE FUNCTION fn_examples.depends_on_table_column() RETURNS integer
    LANGUAGE sql
    AS $$
  SELECT id FROM fn_examples.ordinary_table LIMIT 1
$$;


--
-- Name: depends_on_table_column_type(); Type: FUNCTION; Schema: fn_examples; Owner: -
--

CREATE FUNCTION fn_examples.depends_on_table_column_type() RETURNS integer
    LANGUAGE sql
    AS $$
    SELECT id FROM fn_examples.ordinary_table LIMIT 1
  $$;


SET default_table_access_method = heap;

--
-- Name: ordinary_table; Type: TABLE; Schema: fn_examples; Owner: -
--

CREATE TABLE fn_examples.ordinary_table (
    id integer
);


--
-- Name: depends_on_table_rowtype(); Type: FUNCTION; Schema: fn_examples; Owner: -
--

CREATE FUNCTION fn_examples.depends_on_table_rowtype() RETURNS fn_examples.ordinary_table
    LANGUAGE sql
    AS $$
  SELECT * FROM fn_examples.ordinary_table LIMIT 1
$$;


--
-- Name: depends_on_view_column(); Type: FUNCTION; Schema: fn_examples; Owner: -
--

CREATE FUNCTION fn_examples.depends_on_view_column() RETURNS integer
    LANGUAGE sql
    AS $$
  SELECT id FROM fn_examples.basic_view LIMIT 1
$$;


--
-- Name: depends_on_view_column_type(); Type: FUNCTION; Schema: fn_examples; Owner: -
--

CREATE FUNCTION fn_examples.depends_on_view_column_type() RETURNS integer
    LANGUAGE sql
    AS $$
  SELECT id FROM fn_examples.basic_view LIMIT 1
$$;


--
-- Name: insert_to_table(); Type: PROCEDURE; Schema: fn_examples; Owner: -
--

CREATE PROCEDURE fn_examples.insert_to_table()
    LANGUAGE plpgsql
    AS $$
  BEGIN
    IF fn_examples.is_even(2) THEN
      INSERT INTO fn_examples.ordinary_table(id) VALUES (1);
    END IF;
  END;
$$;


--
-- Name: is_even(integer); Type: FUNCTION; Schema: fn_examples; Owner: -
--

CREATE FUNCTION fn_examples.is_even(i integer) RETURNS boolean
    LANGUAGE plpgsql
    AS $$
BEGIN
  IF i % 2 = 0 THEN
    RETURN true;
  ELSE
    RETURN false;
  end if;
END;
$$;


--
-- Name: is_odd(integer); Type: FUNCTION; Schema: fn_examples; Owner: -
--

CREATE FUNCTION fn_examples.is_odd(i integer) RETURNS boolean
    LANGUAGE sql
    AS $$
  SELECT (fn_examples.is_even(i) IS NOT true)
$$;


--
-- Name: polyf(anyelement); Type: FUNCTION; Schema: fn_examples; Owner: -
--

CREATE FUNCTION fn_examples.polyf(x anyelement) RETURNS anyelement
    LANGUAGE sql
    AS $$
  select x + 1
$$;


--
-- Name: log_table_alteration(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.log_table_alteration() RETURNS event_trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
  RAISE NOTICE 'command % issued', tg_tag;
END;
$$;


--
-- Name: arg_depends_on_range_type(range_type_example.float8_range); Type: FUNCTION; Schema: range_type_example; Owner: -
--

CREATE FUNCTION range_type_example.arg_depends_on_range_type(r range_type_example.float8_range) RETURNS boolean
    LANGUAGE sql IMMUTABLE
    AS $$ SELECT true $$;


--
-- Name: return_depends_on_range_type(); Type: FUNCTION; Schema: range_type_example; Owner: -
--

CREATE FUNCTION range_type_example.return_depends_on_range_type() RETURNS range_type_example.float8_range
    LANGUAGE sql IMMUTABLE
    AS $$
    SELECT '[1.2, 3.4]'::range_type_example.float8_range
  $$;


--
-- Name: f_leak(text); Type: FUNCTION; Schema: regress_rls_schema; Owner: -
--

CREATE FUNCTION regress_rls_schema.f_leak(text) RETURNS boolean
    LANGUAGE plpgsql COST 1e-07
    AS $_$BEGIN RAISE NOTICE 'f_leak => %', $1; RETURN true; END$_$;


--
-- Name: op_leak(integer, integer); Type: FUNCTION; Schema: regress_rls_schema; Owner: -
--

CREATE FUNCTION regress_rls_schema.op_leak(integer, integer) RETURNS boolean
    LANGUAGE plpgsql
    AS $_$BEGIN RAISE NOTICE 'op_leak => %, %', $1, $2; RETURN $1 < $2; END$_$;


--
-- Name: check_account_update(); Type: FUNCTION; Schema: trigger_test; Owner: -
--

CREATE FUNCTION trigger_test.check_account_update() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
    BEGIN
      RAISE NOTICE 'trigger_func(%) called: action = %, when = %, level = %, old = %, new = %',
                        TG_ARGV[0], TG_OP, TG_WHEN, TG_LEVEL, OLD, NEW;
      RETURN NULL;
    END;
  $$;


--
-- Name: log_account_update(); Type: FUNCTION; Schema: trigger_test; Owner: -
--

CREATE FUNCTION trigger_test.log_account_update() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
    BEGIN
      INSERT INTO trigger_test.update_log(account_id) VALUES (1);
      RETURN NULL;
    END;
  $$;


--
-- Name: view_insert_row(); Type: FUNCTION; Schema: trigger_test; Owner: -
--

CREATE FUNCTION trigger_test.view_insert_row() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
    BEGIN
      INSERT INTO trigger_test.accounts(id, balance) VALUES (NEW.id, NEW.balance);
      RETURN NEW;
    END;
  $$;


--
-- Name: my_avg(integer); Type: AGGREGATE; Schema: agg_ex; Owner: -
--

CREATE AGGREGATE agg_ex.my_avg(integer) (
    SFUNC = agg_ex.avg_transfn,
    STYPE = agg_ex.avg_state,
    FINALFUNC = agg_ex.avg_finalfn
);


--
-- Name: my_sum(integer); Type: AGGREGATE; Schema: agg_ex; Owner: -
--

CREATE AGGREGATE agg_ex.my_sum(integer) (
    SFUNC = agg_ex.avg_transfn,
    STYPE = agg_ex.avg_state,
    FINALFUNC = agg_ex.sum_finalfn
);


--
-- Name: <%; Type: OPERATOR; Schema: public; Owner: -
--

CREATE OPERATOR public.<% (
    FUNCTION = base_type_examples.fake_op,
    LEFTARG = point,
    RIGHTARG = base_type_examples.int42,
    COMMUTATOR = OPERATOR(public.>%),
    NEGATOR = OPERATOR(public.>=%)
);


--
-- Name: <<<; Type: OPERATOR; Schema: regress_rls_schema; Owner: -
--

CREATE OPERATOR regress_rls_schema.<<< (
    FUNCTION = regress_rls_schema.op_leak,
    LEFTARG = integer,
    RIGHTARG = integer,
    RESTRICT = scalarltsel
);


--
-- Name: box_ops; Type: OPERATOR FAMILY; Schema: am_examples; Owner: -
--

CREATE OPERATOR FAMILY am_examples.box_ops USING gist2;
ALTER OPERATOR FAMILY am_examples.box_ops USING gist2 ADD
    OPERATOR 1 <<(box,box) ,
    OPERATOR 2 &<(box,box) ,
    OPERATOR 3 &&(box,box) ,
    OPERATOR 4 &>(box,box) ,
    OPERATOR 5 >>(box,box) ,
    OPERATOR 6 ~=(box,box) ,
    OPERATOR 7 @>(box,box) ,
    OPERATOR 8 <@(box,box) ,
    OPERATOR 9 &<|(box,box) ,
    OPERATOR 10 <<|(box,box) ,
    OPERATOR 11 |>>(box,box) ,
    OPERATOR 12 |&>(box,box);


--
-- Name: box_ops; Type: OPERATOR CLASS; Schema: am_examples; Owner: -
--

CREATE OPERATOR CLASS am_examples.box_ops
    DEFAULT FOR TYPE box USING gist2 FAMILY am_examples.box_ops AS
    FUNCTION 1 (box, box) gist_box_consistent(internal,box,smallint,oid,internal) ,
    FUNCTION 2 (box, box) gist_box_union(internal,internal) ,
    FUNCTION 5 (box, box) gist_box_penalty(internal,internal,internal) ,
    FUNCTION 6 (box, box) gist_box_picksplit(internal,internal) ,
    FUNCTION 7 (box, box) gist_box_same(box,box,internal);


--
-- Name: gist2_fam; Type: OPERATOR FAMILY; Schema: am_examples; Owner: -
--

CREATE OPERATOR FAMILY am_examples.gist2_fam USING gist2;


--
-- Name: myconv; Type: CONVERSION; Schema: conversion_example; Owner: -
--

CREATE CONVERSION conversion_example.myconv FOR 'LATIN1' TO 'UTF8' FROM iso8859_1_to_utf8;


--
-- Name: my_table; Type: TABLE; Schema: agg_ex; Owner: -
--

CREATE TABLE agg_ex.my_table (
    i integer
);


--
-- Name: my_view; Type: VIEW; Schema: agg_ex; Owner: -
--

CREATE VIEW agg_ex.my_view AS
 SELECT agg_ex.my_sum(i) AS my_sum,
    agg_ex.my_avg(i) AS my_avg
   FROM agg_ex.my_table;


--
-- Name: am_partitioned; Type: TABLE; Schema: am_examples; Owner: -
--

CREATE TABLE am_examples.am_partitioned (
    x integer,
    y integer
)
PARTITION BY HASH (x);


--
-- Name: fast_emp4000; Type: TABLE; Schema: am_examples; Owner: -
--

CREATE TABLE am_examples.fast_emp4000 (
    home_base box
);


SET default_table_access_method = heap2;

--
-- Name: heaptable; Type: TABLE; Schema: am_examples; Owner: -
--

CREATE TABLE am_examples.heaptable (
    a integer,
    repeat text
);


--
-- Name: heapmv; Type: MATERIALIZED VIEW; Schema: am_examples; Owner: -
--

CREATE MATERIALIZED VIEW am_examples.heapmv AS
 SELECT a,
    repeat
   FROM am_examples.heaptable
  WITH NO DATA;


--
-- Name: tableam_parted_heapx; Type: TABLE; Schema: am_examples; Owner: -
--

CREATE TABLE am_examples.tableam_parted_heapx (
    a text,
    b integer
)
PARTITION BY LIST (a);


SET default_table_access_method = heap;

--
-- Name: tableam_parted_1_heapx; Type: TABLE; Schema: am_examples; Owner: -
--

CREATE TABLE am_examples.tableam_parted_1_heapx (
    a text,
    b integer
);


--
-- Name: tableam_parted_2_heapx; Type: TABLE; Schema: am_examples; Owner: -
--

CREATE TABLE am_examples.tableam_parted_2_heapx (
    a text,
    b integer
);


--
-- Name: tableam_parted_heap2; Type: TABLE; Schema: am_examples; Owner: -
--

CREATE TABLE am_examples.tableam_parted_heap2 (
    a text,
    b integer
)
PARTITION BY LIST (a);


--
-- Name: tableam_parted_c_heap2; Type: TABLE; Schema: am_examples; Owner: -
--

CREATE TABLE am_examples.tableam_parted_c_heap2 (
    a text,
    b integer
);


SET default_table_access_method = heap2;

--
-- Name: tableam_parted_d_heap2; Type: TABLE; Schema: am_examples; Owner: -
--

CREATE TABLE am_examples.tableam_parted_d_heap2 (
    a text,
    b integer
);


--
-- Name: tableam_tbl_heap2; Type: TABLE; Schema: am_examples; Owner: -
--

CREATE TABLE am_examples.tableam_tbl_heap2 (
    f1 integer
);


SET default_table_access_method = heap;

--
-- Name: tableam_tbl_heapx; Type: TABLE; Schema: am_examples; Owner: -
--

CREATE TABLE am_examples.tableam_tbl_heapx (
    f1 integer
);


SET default_table_access_method = heap2;

--
-- Name: tableam_tblas_heap2; Type: TABLE; Schema: am_examples; Owner: -
--

CREATE TABLE am_examples.tableam_tblas_heap2 (
    f1 integer
);


SET default_table_access_method = heap;

--
-- Name: tableam_tblas_heapx; Type: TABLE; Schema: am_examples; Owner: -
--

CREATE TABLE am_examples.tableam_tblas_heapx (
    f1 integer
);


SET default_table_access_method = heap2;

--
-- Name: tableam_tblmv_heapx; Type: MATERIALIZED VIEW; Schema: am_examples; Owner: -
--

CREATE MATERIALIZED VIEW am_examples.tableam_tblmv_heapx AS
 SELECT f1
   FROM am_examples.tableam_tbl_heapx
  WITH NO DATA;


SET default_table_access_method = heap;

--
-- Name: default_test; Type: TABLE; Schema: base_type_examples; Owner: -
--

CREATE TABLE base_type_examples.default_test (
    f1 base_type_examples.text_w_default,
    f2 base_type_examples.int42
);


--
-- Name: ordinary_table; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.ordinary_table (
    basic_ composite_type_examples.basic_comp_type,
    _basic composite_type_examples.basic_comp_type GENERATED ALWAYS AS (basic_) STORED,
    nested composite_type_examples.nested,
    _nested composite_type_examples.nested GENERATED ALWAYS AS (nested) STORED,
    CONSTRAINT check_f1_gt_1 CHECK (((basic_).f1 > 1)),
    CONSTRAINT check_f1_gt_1_again CHECK (((_basic).f1 > 1)),
    CONSTRAINT check_nested_f1_gt_1 CHECK (((nested).foo.f1 > 1)),
    CONSTRAINT check_nested_f1_gt_1_again CHECK (((_nested).foo.f1 > 1))
);


--
-- Name: basic_view; Type: VIEW; Schema: composite_type_examples; Owner: -
--

CREATE VIEW composite_type_examples.basic_view AS
 SELECT basic_,
    _basic,
    nested,
    _nested
   FROM composite_type_examples.ordinary_table;


--
-- Name: equivalent_rowtype; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.equivalent_rowtype (
    f1 integer,
    f2 text
);


--
-- Name: i_0; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_0 (
    i integer
);


--
-- Name: i_1; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_1 (
    i composite_type_examples.i_0
);


--
-- Name: i_2; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_2 (
    i composite_type_examples.i_1
);


--
-- Name: i_3; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_3 (
    i composite_type_examples.i_2
);


--
-- Name: i_4; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_4 (
    i composite_type_examples.i_3
);


--
-- Name: i_5; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_5 (
    i composite_type_examples.i_4
);


--
-- Name: i_6; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_6 (
    i composite_type_examples.i_5
);


--
-- Name: i_7; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_7 (
    i composite_type_examples.i_6
);


--
-- Name: i_8; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_8 (
    i composite_type_examples.i_7
);


--
-- Name: i_9; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_9 (
    i composite_type_examples.i_8
);


--
-- Name: i_10; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_10 (
    i composite_type_examples.i_9
);


--
-- Name: i_11; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_11 (
    i composite_type_examples.i_10
);


--
-- Name: i_12; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_12 (
    i composite_type_examples.i_11
);


--
-- Name: i_13; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_13 (
    i composite_type_examples.i_12
);


--
-- Name: i_14; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_14 (
    i composite_type_examples.i_13
);


--
-- Name: i_15; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_15 (
    i composite_type_examples.i_14
);


--
-- Name: i_16; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_16 (
    i composite_type_examples.i_15
);


--
-- Name: i_17; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_17 (
    i composite_type_examples.i_16
);


--
-- Name: i_18; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_18 (
    i composite_type_examples.i_17
);


--
-- Name: i_19; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_19 (
    i composite_type_examples.i_18
);


--
-- Name: i_20; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_20 (
    i composite_type_examples.i_19
);


--
-- Name: i_21; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_21 (
    i composite_type_examples.i_20
);


--
-- Name: i_22; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_22 (
    i composite_type_examples.i_21
);


--
-- Name: i_23; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_23 (
    i composite_type_examples.i_22
);


--
-- Name: i_24; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_24 (
    i composite_type_examples.i_23
);


--
-- Name: i_25; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_25 (
    i composite_type_examples.i_24
);


--
-- Name: i_26; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_26 (
    i composite_type_examples.i_25
);


--
-- Name: i_27; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_27 (
    i composite_type_examples.i_26
);


--
-- Name: i_28; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_28 (
    i composite_type_examples.i_27
);


--
-- Name: i_29; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_29 (
    i composite_type_examples.i_28
);


--
-- Name: i_30; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_30 (
    i composite_type_examples.i_29
);


--
-- Name: i_31; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_31 (
    i composite_type_examples.i_30
);


--
-- Name: i_32; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_32 (
    i composite_type_examples.i_31
);


--
-- Name: i_33; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_33 (
    i composite_type_examples.i_32
);


--
-- Name: i_34; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_34 (
    i composite_type_examples.i_33
);


--
-- Name: i_35; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_35 (
    i composite_type_examples.i_34
);


--
-- Name: i_36; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_36 (
    i composite_type_examples.i_35
);


--
-- Name: i_37; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_37 (
    i composite_type_examples.i_36
);


--
-- Name: i_38; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_38 (
    i composite_type_examples.i_37
);


--
-- Name: i_39; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_39 (
    i composite_type_examples.i_38
);


--
-- Name: i_40; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_40 (
    i composite_type_examples.i_39
);


--
-- Name: i_41; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_41 (
    i composite_type_examples.i_40
);


--
-- Name: i_42; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_42 (
    i composite_type_examples.i_41
);


--
-- Name: i_43; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_43 (
    i composite_type_examples.i_42
);


--
-- Name: i_44; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_44 (
    i composite_type_examples.i_43
);


--
-- Name: i_45; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_45 (
    i composite_type_examples.i_44
);


--
-- Name: i_46; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_46 (
    i composite_type_examples.i_45
);


--
-- Name: i_47; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_47 (
    i composite_type_examples.i_46
);


--
-- Name: i_48; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_48 (
    i composite_type_examples.i_47
);


--
-- Name: i_49; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_49 (
    i composite_type_examples.i_48
);


--
-- Name: i_50; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_50 (
    i composite_type_examples.i_49
);


--
-- Name: i_51; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_51 (
    i composite_type_examples.i_50
);


--
-- Name: i_52; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_52 (
    i composite_type_examples.i_51
);


--
-- Name: i_53; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_53 (
    i composite_type_examples.i_52
);


--
-- Name: i_54; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_54 (
    i composite_type_examples.i_53
);


--
-- Name: i_55; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_55 (
    i composite_type_examples.i_54
);


--
-- Name: i_56; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_56 (
    i composite_type_examples.i_55
);


--
-- Name: i_57; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_57 (
    i composite_type_examples.i_56
);


--
-- Name: i_58; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_58 (
    i composite_type_examples.i_57
);


--
-- Name: i_59; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_59 (
    i composite_type_examples.i_58
);


--
-- Name: i_60; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_60 (
    i composite_type_examples.i_59
);


--
-- Name: i_61; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_61 (
    i composite_type_examples.i_60
);


--
-- Name: i_62; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_62 (
    i composite_type_examples.i_61
);


--
-- Name: i_63; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_63 (
    i composite_type_examples.i_62
);


--
-- Name: i_64; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_64 (
    i composite_type_examples.i_63
);


--
-- Name: i_65; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_65 (
    i composite_type_examples.i_64
);


--
-- Name: i_66; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_66 (
    i composite_type_examples.i_65
);


--
-- Name: i_67; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_67 (
    i composite_type_examples.i_66
);


--
-- Name: i_68; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_68 (
    i composite_type_examples.i_67
);


--
-- Name: i_69; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_69 (
    i composite_type_examples.i_68
);


--
-- Name: i_70; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_70 (
    i composite_type_examples.i_69
);


--
-- Name: i_71; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_71 (
    i composite_type_examples.i_70
);


--
-- Name: i_72; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_72 (
    i composite_type_examples.i_71
);


--
-- Name: i_73; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_73 (
    i composite_type_examples.i_72
);


--
-- Name: i_74; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_74 (
    i composite_type_examples.i_73
);


--
-- Name: i_75; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_75 (
    i composite_type_examples.i_74
);


--
-- Name: i_76; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_76 (
    i composite_type_examples.i_75
);


--
-- Name: i_77; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_77 (
    i composite_type_examples.i_76
);


--
-- Name: i_78; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_78 (
    i composite_type_examples.i_77
);


--
-- Name: i_79; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_79 (
    i composite_type_examples.i_78
);


--
-- Name: i_80; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_80 (
    i composite_type_examples.i_79
);


--
-- Name: i_81; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_81 (
    i composite_type_examples.i_80
);


--
-- Name: i_82; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_82 (
    i composite_type_examples.i_81
);


--
-- Name: i_83; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_83 (
    i composite_type_examples.i_82
);


--
-- Name: i_84; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_84 (
    i composite_type_examples.i_83
);


--
-- Name: i_85; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_85 (
    i composite_type_examples.i_84
);


--
-- Name: i_86; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_86 (
    i composite_type_examples.i_85
);


--
-- Name: i_87; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_87 (
    i composite_type_examples.i_86
);


--
-- Name: i_88; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_88 (
    i composite_type_examples.i_87
);


--
-- Name: i_89; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_89 (
    i composite_type_examples.i_88
);


--
-- Name: i_90; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_90 (
    i composite_type_examples.i_89
);


--
-- Name: i_91; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_91 (
    i composite_type_examples.i_90
);


--
-- Name: i_92; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_92 (
    i composite_type_examples.i_91
);


--
-- Name: i_93; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_93 (
    i composite_type_examples.i_92
);


--
-- Name: i_94; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_94 (
    i composite_type_examples.i_93
);


--
-- Name: i_95; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_95 (
    i composite_type_examples.i_94
);


--
-- Name: i_96; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_96 (
    i composite_type_examples.i_95
);


--
-- Name: i_97; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_97 (
    i composite_type_examples.i_96
);


--
-- Name: i_98; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_98 (
    i composite_type_examples.i_97
);


--
-- Name: i_99; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_99 (
    i composite_type_examples.i_98
);


--
-- Name: i_100; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_100 (
    i composite_type_examples.i_99
);


--
-- Name: i_101; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_101 (
    i composite_type_examples.i_100
);


--
-- Name: i_102; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_102 (
    i composite_type_examples.i_101
);


--
-- Name: i_103; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_103 (
    i composite_type_examples.i_102
);


--
-- Name: i_104; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_104 (
    i composite_type_examples.i_103
);


--
-- Name: i_105; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_105 (
    i composite_type_examples.i_104
);


--
-- Name: i_106; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_106 (
    i composite_type_examples.i_105
);


--
-- Name: i_107; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_107 (
    i composite_type_examples.i_106
);


--
-- Name: i_108; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_108 (
    i composite_type_examples.i_107
);


--
-- Name: i_109; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_109 (
    i composite_type_examples.i_108
);


--
-- Name: i_110; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_110 (
    i composite_type_examples.i_109
);


--
-- Name: i_111; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_111 (
    i composite_type_examples.i_110
);


--
-- Name: i_112; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_112 (
    i composite_type_examples.i_111
);


--
-- Name: i_113; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_113 (
    i composite_type_examples.i_112
);


--
-- Name: i_114; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_114 (
    i composite_type_examples.i_113
);


--
-- Name: i_115; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_115 (
    i composite_type_examples.i_114
);


--
-- Name: i_116; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_116 (
    i composite_type_examples.i_115
);


--
-- Name: i_117; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_117 (
    i composite_type_examples.i_116
);


--
-- Name: i_118; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_118 (
    i composite_type_examples.i_117
);


--
-- Name: i_119; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_119 (
    i composite_type_examples.i_118
);


--
-- Name: i_120; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_120 (
    i composite_type_examples.i_119
);


--
-- Name: i_121; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_121 (
    i composite_type_examples.i_120
);


--
-- Name: i_122; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_122 (
    i composite_type_examples.i_121
);


--
-- Name: i_123; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_123 (
    i composite_type_examples.i_122
);


--
-- Name: i_124; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_124 (
    i composite_type_examples.i_123
);


--
-- Name: i_125; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_125 (
    i composite_type_examples.i_124
);


--
-- Name: i_126; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_126 (
    i composite_type_examples.i_125
);


--
-- Name: i_127; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_127 (
    i composite_type_examples.i_126
);


--
-- Name: i_128; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_128 (
    i composite_type_examples.i_127
);


--
-- Name: i_129; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_129 (
    i composite_type_examples.i_128
);


--
-- Name: i_130; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_130 (
    i composite_type_examples.i_129
);


--
-- Name: i_131; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_131 (
    i composite_type_examples.i_130
);


--
-- Name: i_132; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_132 (
    i composite_type_examples.i_131
);


--
-- Name: i_133; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_133 (
    i composite_type_examples.i_132
);


--
-- Name: i_134; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_134 (
    i composite_type_examples.i_133
);


--
-- Name: i_135; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_135 (
    i composite_type_examples.i_134
);


--
-- Name: i_136; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_136 (
    i composite_type_examples.i_135
);


--
-- Name: i_137; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_137 (
    i composite_type_examples.i_136
);


--
-- Name: i_138; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_138 (
    i composite_type_examples.i_137
);


--
-- Name: i_139; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_139 (
    i composite_type_examples.i_138
);


--
-- Name: i_140; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_140 (
    i composite_type_examples.i_139
);


--
-- Name: i_141; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_141 (
    i composite_type_examples.i_140
);


--
-- Name: i_142; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_142 (
    i composite_type_examples.i_141
);


--
-- Name: i_143; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_143 (
    i composite_type_examples.i_142
);


--
-- Name: i_144; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_144 (
    i composite_type_examples.i_143
);


--
-- Name: i_145; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_145 (
    i composite_type_examples.i_144
);


--
-- Name: i_146; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_146 (
    i composite_type_examples.i_145
);


--
-- Name: i_147; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_147 (
    i composite_type_examples.i_146
);


--
-- Name: i_148; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_148 (
    i composite_type_examples.i_147
);


--
-- Name: i_149; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_149 (
    i composite_type_examples.i_148
);


--
-- Name: i_150; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_150 (
    i composite_type_examples.i_149
);


--
-- Name: i_151; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_151 (
    i composite_type_examples.i_150
);


--
-- Name: i_152; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_152 (
    i composite_type_examples.i_151
);


--
-- Name: i_153; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_153 (
    i composite_type_examples.i_152
);


--
-- Name: i_154; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_154 (
    i composite_type_examples.i_153
);


--
-- Name: i_155; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_155 (
    i composite_type_examples.i_154
);


--
-- Name: i_156; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_156 (
    i composite_type_examples.i_155
);


--
-- Name: i_157; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_157 (
    i composite_type_examples.i_156
);


--
-- Name: i_158; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_158 (
    i composite_type_examples.i_157
);


--
-- Name: i_159; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_159 (
    i composite_type_examples.i_158
);


--
-- Name: i_160; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_160 (
    i composite_type_examples.i_159
);


--
-- Name: i_161; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_161 (
    i composite_type_examples.i_160
);


--
-- Name: i_162; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_162 (
    i composite_type_examples.i_161
);


--
-- Name: i_163; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_163 (
    i composite_type_examples.i_162
);


--
-- Name: i_164; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_164 (
    i composite_type_examples.i_163
);


--
-- Name: i_165; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_165 (
    i composite_type_examples.i_164
);


--
-- Name: i_166; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_166 (
    i composite_type_examples.i_165
);


--
-- Name: i_167; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_167 (
    i composite_type_examples.i_166
);


--
-- Name: i_168; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_168 (
    i composite_type_examples.i_167
);


--
-- Name: i_169; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_169 (
    i composite_type_examples.i_168
);


--
-- Name: i_170; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_170 (
    i composite_type_examples.i_169
);


--
-- Name: i_171; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_171 (
    i composite_type_examples.i_170
);


--
-- Name: i_172; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_172 (
    i composite_type_examples.i_171
);


--
-- Name: i_173; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_173 (
    i composite_type_examples.i_172
);


--
-- Name: i_174; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_174 (
    i composite_type_examples.i_173
);


--
-- Name: i_175; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_175 (
    i composite_type_examples.i_174
);


--
-- Name: i_176; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_176 (
    i composite_type_examples.i_175
);


--
-- Name: i_177; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_177 (
    i composite_type_examples.i_176
);


--
-- Name: i_178; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_178 (
    i composite_type_examples.i_177
);


--
-- Name: i_179; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_179 (
    i composite_type_examples.i_178
);


--
-- Name: i_180; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_180 (
    i composite_type_examples.i_179
);


--
-- Name: i_181; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_181 (
    i composite_type_examples.i_180
);


--
-- Name: i_182; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_182 (
    i composite_type_examples.i_181
);


--
-- Name: i_183; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_183 (
    i composite_type_examples.i_182
);


--
-- Name: i_184; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_184 (
    i composite_type_examples.i_183
);


--
-- Name: i_185; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_185 (
    i composite_type_examples.i_184
);


--
-- Name: i_186; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_186 (
    i composite_type_examples.i_185
);


--
-- Name: i_187; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_187 (
    i composite_type_examples.i_186
);


--
-- Name: i_188; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_188 (
    i composite_type_examples.i_187
);


--
-- Name: i_189; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_189 (
    i composite_type_examples.i_188
);


--
-- Name: i_190; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_190 (
    i composite_type_examples.i_189
);


--
-- Name: i_191; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_191 (
    i composite_type_examples.i_190
);


--
-- Name: i_192; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_192 (
    i composite_type_examples.i_191
);


--
-- Name: i_193; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_193 (
    i composite_type_examples.i_192
);


--
-- Name: i_194; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_194 (
    i composite_type_examples.i_193
);


--
-- Name: i_195; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_195 (
    i composite_type_examples.i_194
);


--
-- Name: i_196; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_196 (
    i composite_type_examples.i_195
);


--
-- Name: i_197; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_197 (
    i composite_type_examples.i_196
);


--
-- Name: i_198; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_198 (
    i composite_type_examples.i_197
);


--
-- Name: i_199; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_199 (
    i composite_type_examples.i_198
);


--
-- Name: i_200; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_200 (
    i composite_type_examples.i_199
);


--
-- Name: i_201; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_201 (
    i composite_type_examples.i_200
);


--
-- Name: i_202; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_202 (
    i composite_type_examples.i_201
);


--
-- Name: i_203; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_203 (
    i composite_type_examples.i_202
);


--
-- Name: i_204; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_204 (
    i composite_type_examples.i_203
);


--
-- Name: i_205; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_205 (
    i composite_type_examples.i_204
);


--
-- Name: i_206; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_206 (
    i composite_type_examples.i_205
);


--
-- Name: i_207; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_207 (
    i composite_type_examples.i_206
);


--
-- Name: i_208; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_208 (
    i composite_type_examples.i_207
);


--
-- Name: i_209; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_209 (
    i composite_type_examples.i_208
);


--
-- Name: i_210; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_210 (
    i composite_type_examples.i_209
);


--
-- Name: i_211; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_211 (
    i composite_type_examples.i_210
);


--
-- Name: i_212; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_212 (
    i composite_type_examples.i_211
);


--
-- Name: i_213; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_213 (
    i composite_type_examples.i_212
);


--
-- Name: i_214; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_214 (
    i composite_type_examples.i_213
);


--
-- Name: i_215; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_215 (
    i composite_type_examples.i_214
);


--
-- Name: i_216; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_216 (
    i composite_type_examples.i_215
);


--
-- Name: i_217; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_217 (
    i composite_type_examples.i_216
);


--
-- Name: i_218; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_218 (
    i composite_type_examples.i_217
);


--
-- Name: i_219; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_219 (
    i composite_type_examples.i_218
);


--
-- Name: i_220; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_220 (
    i composite_type_examples.i_219
);


--
-- Name: i_221; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_221 (
    i composite_type_examples.i_220
);


--
-- Name: i_222; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_222 (
    i composite_type_examples.i_221
);


--
-- Name: i_223; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_223 (
    i composite_type_examples.i_222
);


--
-- Name: i_224; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_224 (
    i composite_type_examples.i_223
);


--
-- Name: i_225; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_225 (
    i composite_type_examples.i_224
);


--
-- Name: i_226; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_226 (
    i composite_type_examples.i_225
);


--
-- Name: i_227; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_227 (
    i composite_type_examples.i_226
);


--
-- Name: i_228; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_228 (
    i composite_type_examples.i_227
);


--
-- Name: i_229; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_229 (
    i composite_type_examples.i_228
);


--
-- Name: i_230; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_230 (
    i composite_type_examples.i_229
);


--
-- Name: i_231; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_231 (
    i composite_type_examples.i_230
);


--
-- Name: i_232; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_232 (
    i composite_type_examples.i_231
);


--
-- Name: i_233; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_233 (
    i composite_type_examples.i_232
);


--
-- Name: i_234; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_234 (
    i composite_type_examples.i_233
);


--
-- Name: i_235; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_235 (
    i composite_type_examples.i_234
);


--
-- Name: i_236; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_236 (
    i composite_type_examples.i_235
);


--
-- Name: i_237; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_237 (
    i composite_type_examples.i_236
);


--
-- Name: i_238; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_238 (
    i composite_type_examples.i_237
);


--
-- Name: i_239; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_239 (
    i composite_type_examples.i_238
);


--
-- Name: i_240; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_240 (
    i composite_type_examples.i_239
);


--
-- Name: i_241; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_241 (
    i composite_type_examples.i_240
);


--
-- Name: i_242; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_242 (
    i composite_type_examples.i_241
);


--
-- Name: i_243; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_243 (
    i composite_type_examples.i_242
);


--
-- Name: i_244; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_244 (
    i composite_type_examples.i_243
);


--
-- Name: i_245; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_245 (
    i composite_type_examples.i_244
);


--
-- Name: i_246; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_246 (
    i composite_type_examples.i_245
);


--
-- Name: i_247; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_247 (
    i composite_type_examples.i_246
);


--
-- Name: i_248; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_248 (
    i composite_type_examples.i_247
);


--
-- Name: i_249; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_249 (
    i composite_type_examples.i_248
);


--
-- Name: i_250; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_250 (
    i composite_type_examples.i_249
);


--
-- Name: i_251; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_251 (
    i composite_type_examples.i_250
);


--
-- Name: i_252; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_252 (
    i composite_type_examples.i_251
);


--
-- Name: i_253; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_253 (
    i composite_type_examples.i_252
);


--
-- Name: i_254; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_254 (
    i composite_type_examples.i_253
);


--
-- Name: i_255; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_255 (
    i composite_type_examples.i_254
);


--
-- Name: i_256; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.i_256 (
    i composite_type_examples.i_255
);


--
-- Name: inherited_table; Type: TABLE; Schema: composite_type_examples; Owner: -
--

CREATE TABLE composite_type_examples.inherited_table (
)
INHERITS (composite_type_examples.ordinary_table);


--
-- Name: even_numbers; Type: TABLE; Schema: domain_examples; Owner: -
--

CREATE TABLE domain_examples.even_numbers (
    e domain_examples.positive_even_number
);


--
-- Name: us_snail_addy; Type: TABLE; Schema: domain_examples; Owner: -
--

CREATE TABLE domain_examples.us_snail_addy (
    address_id integer NOT NULL,
    street1 text NOT NULL,
    street2 text,
    street3 text,
    city text NOT NULL,
    postal domain_examples.us_postal_code NOT NULL
);


--
-- Name: us_snail_addy_address_id_seq; Type: SEQUENCE; Schema: domain_examples; Owner: -
--

CREATE SEQUENCE domain_examples.us_snail_addy_address_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: us_snail_addy_address_id_seq; Type: SEQUENCE OWNED BY; Schema: domain_examples; Owner: -
--

ALTER SEQUENCE domain_examples.us_snail_addy_address_id_seq OWNED BY domain_examples.us_snail_addy.address_id;


--
-- Name: _bug_severity; Type: TABLE; Schema: enum_example; Owner: -
--

CREATE TABLE enum_example._bug_severity (
    id integer,
    severity enum_example.bug_severity
);


--
-- Name: bugs; Type: TABLE; Schema: enum_example; Owner: -
--

CREATE TABLE enum_example.bugs (
    id integer NOT NULL,
    description text,
    status enum_example.bug_status,
    _status enum_example.bug_status GENERATED ALWAYS AS (status) STORED,
    severity enum_example.bug_severity,
    _severity enum_example.bug_severity GENERATED ALWAYS AS (severity) STORED,
    info enum_example.bug_info GENERATED ALWAYS AS (enum_example.make_bug_info(status, severity)) STORED
);


--
-- Name: _bugs; Type: VIEW; Schema: enum_example; Owner: -
--

CREATE VIEW enum_example._bugs AS
 SELECT id,
    status
   FROM enum_example.bugs;


--
-- Name: bugs_clone; Type: TABLE; Schema: enum_example; Owner: -
--

CREATE TABLE enum_example.bugs_clone (
)
INHERITS (enum_example.bugs);


--
-- Name: bugs_id_seq; Type: SEQUENCE; Schema: enum_example; Owner: -
--

CREATE SEQUENCE enum_example.bugs_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: bugs_id_seq; Type: SEQUENCE OWNED BY; Schema: enum_example; Owner: -
--

ALTER SEQUENCE enum_example.bugs_id_seq OWNED BY enum_example.bugs.id;


--
-- Name: dependent_view; Type: VIEW; Schema: extension_example; Owner: -
--

CREATE VIEW extension_example.dependent_view AS
 SELECT key,
    value
   FROM extension_example.each('""=>"1", "b"=>NULL, "aaa"=>"bq"'::extension_example.hstore) each(key, value);


--
-- Name: testhstore; Type: TABLE; Schema: extension_example; Owner: -
--

CREATE TABLE extension_example.testhstore (
    h extension_example.hstore
);


--
-- Name: basic_view; Type: VIEW; Schema: fn_examples; Owner: -
--

CREATE VIEW fn_examples.basic_view AS
 SELECT id
   FROM fn_examples.ordinary_table;


--
-- Name: technically_doesnt_exist; Type: FOREIGN TABLE; Schema: foreign_db_example; Owner: -
--

CREATE FOREIGN TABLE foreign_db_example.technically_doesnt_exist (
    id integer,
    uses_type foreign_db_example.example_type,
    _uses_type foreign_db_example.example_type GENERATED ALWAYS AS (uses_type) STORED,
    positive_number foreign_db_example.positive_number,
    _positive_number foreign_db_example.positive_number GENERATED ALWAYS AS (positive_number) STORED,
    CONSTRAINT imaginary_table_id_gt_1 CHECK ((id > 1))
)
SERVER technically_this_server;


--
-- Name: films; Type: TABLE; Schema: idx_ex; Owner: -
--

CREATE TABLE idx_ex.films (
    id integer NOT NULL,
    title text,
    director text,
    rating integer,
    code text
);


--
-- Name: binary_examples; Type: TABLE; Schema: ordinary_tables; Owner: -
--

CREATE TABLE ordinary_tables.binary_examples (
    bytes bytea NOT NULL
);


--
-- Name: bit_string_examples; Type: TABLE; Schema: ordinary_tables; Owner: -
--

CREATE TABLE ordinary_tables.bit_string_examples (
    bit_example bit(10),
    bit_varyint_example bit varying(20)
);


--
-- Name: boolean_examples; Type: TABLE; Schema: ordinary_tables; Owner: -
--

CREATE TABLE ordinary_tables.boolean_examples (
    b boolean
);


--
-- Name: character_examples; Type: TABLE; Schema: ordinary_tables; Owner: -
--

CREATE TABLE ordinary_tables.character_examples (
    id text NOT NULL,
    a_varchar character varying,
    a_limited_varchar character varying(10),
    a_single_char character(1),
    n_char character(11)
);


--
-- Name: geometric_examples; Type: TABLE; Schema: ordinary_tables; Owner: -
--

CREATE TABLE ordinary_tables.geometric_examples (
    point_example point,
    line_example line,
    lseg_example lseg,
    box_example box,
    path_example path,
    polygon_example polygon,
    circle_example circle
);


--
-- Name: money_example; Type: TABLE; Schema: ordinary_tables; Owner: -
--

CREATE TABLE ordinary_tables.money_example (
    money money
);


--
-- Name: network_addr_examples; Type: TABLE; Schema: ordinary_tables; Owner: -
--

CREATE TABLE ordinary_tables.network_addr_examples (
    cidr_example cidr,
    inet_example inet,
    macaddr_example macaddr,
    macaddr8_example macaddr8
);


--
-- Name: numeric_type_examples; Type: TABLE; Schema: ordinary_tables; Owner: -
--

CREATE TABLE ordinary_tables.numeric_type_examples (
    id integer NOT NULL,
    an_integer integer NOT NULL,
    an_int integer,
    an_int4 integer,
    an_int8 bigint,
    a_bigint bigint,
    a_smallint smallint,
    a_decimal numeric,
    a_numeric numeric,
    a_real real,
    a_double double precision,
    a_smallserial smallint NOT NULL,
    a_bigserial bigint NOT NULL,
    another_numeric numeric(3,0),
    yet_another_numeric numeric(6,4)
);


--
-- Name: numeric_type_examples_a_bigserial_seq; Type: SEQUENCE; Schema: ordinary_tables; Owner: -
--

CREATE SEQUENCE ordinary_tables.numeric_type_examples_a_bigserial_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: numeric_type_examples_a_bigserial_seq; Type: SEQUENCE OWNED BY; Schema: ordinary_tables; Owner: -
--

ALTER SEQUENCE ordinary_tables.numeric_type_examples_a_bigserial_seq OWNED BY ordinary_tables.numeric_type_examples.a_bigserial;


--
-- Name: numeric_type_examples_a_smallserial_seq; Type: SEQUENCE; Schema: ordinary_tables; Owner: -
--

CREATE SEQUENCE ordinary_tables.numeric_type_examples_a_smallserial_seq
    AS smallint
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: numeric_type_examples_a_smallserial_seq; Type: SEQUENCE OWNED BY; Schema: ordinary_tables; Owner: -
--

ALTER SEQUENCE ordinary_tables.numeric_type_examples_a_smallserial_seq OWNED BY ordinary_tables.numeric_type_examples.a_smallserial;


--
-- Name: numeric_type_examples_id_seq; Type: SEQUENCE; Schema: ordinary_tables; Owner: -
--

CREATE SEQUENCE ordinary_tables.numeric_type_examples_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: numeric_type_examples_id_seq; Type: SEQUENCE OWNED BY; Schema: ordinary_tables; Owner: -
--

ALTER SEQUENCE ordinary_tables.numeric_type_examples_id_seq OWNED BY ordinary_tables.numeric_type_examples.id;


--
-- Name: time; Type: TABLE; Schema: ordinary_tables; Owner: -
--

CREATE TABLE ordinary_tables."time" (
    ts_with_tz timestamp with time zone,
    ts_with_tz_precision timestamp(2) with time zone,
    ts_with_ntz timestamp without time zone,
    ts_with_ntz_precision timestamp(3) without time zone,
    t_with_tz time with time zone,
    t_with_tz_precision time(4) with time zone,
    t_with_ntz time without time zone,
    t_with_ntz_precision time(5) without time zone,
    date date,
    interval_year interval year,
    interval_month interval month,
    interval_day interval day,
    interval_hour interval hour,
    interval_minute interval minute,
    interval_second interval second,
    interval_year_to_month interval year to month,
    interval_day_to_hour interval day to hour,
    interval_day_to_minute interval day to minute,
    interval_day_to_second interval day to second,
    interval_hour_to_minute interval hour to minute,
    interval_hour_to_second interval hour to second,
    interval_minute_to_second interval minute to second
);


--
-- Name: foreign_db_example; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.foreign_db_example AS
 SELECT id,
    uses_type,
    _uses_type,
    positive_number,
    _positive_number
   FROM foreign_db_example.technically_doesnt_exist;


--
-- Name: example_tbl; Type: TABLE; Schema: range_type_example; Owner: -
--

CREATE TABLE range_type_example.example_tbl (
    col range_type_example.float8_range
);


--
-- Name: depends_on_col_using_type; Type: VIEW; Schema: range_type_example; Owner: -
--

CREATE VIEW range_type_example.depends_on_col_using_type AS
 SELECT col
   FROM range_type_example.example_tbl;


--
-- Name: b1; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.b1 (
    a integer,
    b text
);


--
-- Name: bv1; Type: VIEW; Schema: regress_rls_schema; Owner: -
--

CREATE VIEW regress_rls_schema.bv1 WITH (security_barrier='true') AS
 SELECT a,
    b
   FROM regress_rls_schema.b1
  WHERE (a > 0)
  WITH CASCADED CHECK OPTION;


--
-- Name: category; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.category (
    cid integer NOT NULL,
    cname text
);


--
-- Name: dependee; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.dependee (
    x integer,
    y integer
);


--
-- Name: dependent; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.dependent (
    x integer,
    y integer
);


--
-- Name: dob_t1; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.dob_t1 (
    c1 integer
);


--
-- Name: dob_t2; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.dob_t2 (
    c1 integer
)
PARTITION BY RANGE (c1);


--
-- Name: document; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.document (
    did integer NOT NULL,
    cid integer,
    dlevel integer NOT NULL,
    dauthor name,
    dtitle text,
    dnotes text DEFAULT ''::text
);


--
-- Name: part_document; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.part_document (
    did integer,
    cid integer,
    dlevel integer NOT NULL,
    dauthor name,
    dtitle text
)
PARTITION BY RANGE (cid);


--
-- Name: part_document_fiction; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.part_document_fiction (
    did integer,
    cid integer,
    dlevel integer NOT NULL,
    dauthor name,
    dtitle text
);


--
-- Name: part_document_nonfiction; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.part_document_nonfiction (
    did integer,
    cid integer,
    dlevel integer NOT NULL,
    dauthor name,
    dtitle text
);


--
-- Name: part_document_satire; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.part_document_satire (
    did integer,
    cid integer,
    dlevel integer NOT NULL,
    dauthor name,
    dtitle text
);


--
-- Name: r1; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.r1 (
    a integer NOT NULL
);


--
-- Name: r1_2; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.r1_2 (
    a integer
);

ALTER TABLE ONLY regress_rls_schema.r1_2 FORCE ROW LEVEL SECURITY;


--
-- Name: r1_3; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.r1_3 (
    a integer NOT NULL
);

ALTER TABLE ONLY regress_rls_schema.r1_3 FORCE ROW LEVEL SECURITY;


--
-- Name: r1_4; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.r1_4 (
    a integer NOT NULL
);


--
-- Name: r1_5; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.r1_5 (
    a integer NOT NULL
);


--
-- Name: r2; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.r2 (
    a integer
);


--
-- Name: r2_3; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.r2_3 (
    a integer
);

ALTER TABLE ONLY regress_rls_schema.r2_3 FORCE ROW LEVEL SECURITY;


--
-- Name: r2_4; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.r2_4 (
    a integer
);

ALTER TABLE ONLY regress_rls_schema.r2_4 FORCE ROW LEVEL SECURITY;


--
-- Name: r2_5; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.r2_5 (
    a integer
);

ALTER TABLE ONLY regress_rls_schema.r2_5 FORCE ROW LEVEL SECURITY;


--
-- Name: rec1; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.rec1 (
    x integer,
    y integer
);


--
-- Name: rec1v; Type: VIEW; Schema: regress_rls_schema; Owner: -
--

CREATE VIEW regress_rls_schema.rec1v AS
 SELECT x,
    y
   FROM regress_rls_schema.rec1;


--
-- Name: rec1v_2; Type: VIEW; Schema: regress_rls_schema; Owner: -
--

CREATE VIEW regress_rls_schema.rec1v_2 WITH (security_barrier='true') AS
 SELECT x,
    y
   FROM regress_rls_schema.rec1;


--
-- Name: rec2; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.rec2 (
    a integer,
    b integer
);


--
-- Name: rec2v; Type: VIEW; Schema: regress_rls_schema; Owner: -
--

CREATE VIEW regress_rls_schema.rec2v AS
 SELECT a,
    b
   FROM regress_rls_schema.rec2;


--
-- Name: rec2v_2; Type: VIEW; Schema: regress_rls_schema; Owner: -
--

CREATE VIEW regress_rls_schema.rec2v_2 WITH (security_barrier='true') AS
 SELECT a,
    b
   FROM regress_rls_schema.rec2;


--
-- Name: ref_tbl; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.ref_tbl (
    a integer
);


--
-- Name: y1; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.y1 (
    a integer,
    b text
);


--
-- Name: rls_sbv; Type: VIEW; Schema: regress_rls_schema; Owner: -
--

CREATE VIEW regress_rls_schema.rls_sbv WITH (security_barrier='true') AS
 SELECT a,
    b
   FROM regress_rls_schema.y1
  WHERE regress_rls_schema.f_leak(b);


--
-- Name: rls_sbv_2; Type: VIEW; Schema: regress_rls_schema; Owner: -
--

CREATE VIEW regress_rls_schema.rls_sbv_2 WITH (security_barrier='true') AS
 SELECT a,
    b
   FROM regress_rls_schema.y1
  WHERE regress_rls_schema.f_leak(b);


--
-- Name: rls_tbl; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.rls_tbl (
    a integer
);


--
-- Name: rls_tbl_2; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.rls_tbl_2 (
    a integer
);


--
-- Name: rls_tbl_3; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.rls_tbl_3 (
    a integer,
    b integer,
    c integer
);

ALTER TABLE ONLY regress_rls_schema.rls_tbl_3 FORCE ROW LEVEL SECURITY;


--
-- Name: rls_view; Type: VIEW; Schema: regress_rls_schema; Owner: -
--

CREATE VIEW regress_rls_schema.rls_view AS
 SELECT a
   FROM regress_rls_schema.rls_tbl;


--
-- Name: z1; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.z1 (
    a integer,
    b text
);


--
-- Name: rls_view_2; Type: VIEW; Schema: regress_rls_schema; Owner: -
--

CREATE VIEW regress_rls_schema.rls_view_2 AS
 SELECT a,
    b
   FROM regress_rls_schema.z1
  WHERE regress_rls_schema.f_leak(b);


--
-- Name: rls_view_3; Type: VIEW; Schema: regress_rls_schema; Owner: -
--

CREATE VIEW regress_rls_schema.rls_view_3 AS
 SELECT a,
    b
   FROM regress_rls_schema.z1
  WHERE regress_rls_schema.f_leak(b);


--
-- Name: rls_view_4; Type: VIEW; Schema: regress_rls_schema; Owner: -
--

CREATE VIEW regress_rls_schema.rls_view_4 AS
 SELECT a,
    b
   FROM regress_rls_schema.z1
  WHERE regress_rls_schema.f_leak(b);


--
-- Name: s1; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.s1 (
    a integer,
    b text
);


--
-- Name: s2; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.s2 (
    x integer,
    y text
);


--
-- Name: t; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.t (
    c integer
);


--
-- Name: t1; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.t1 (
    id integer NOT NULL,
    a integer,
    b text
);


--
-- Name: t1_2; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.t1_2 (
    a integer
);


--
-- Name: t1_3; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.t1_3 (
    a integer,
    b text
);


--
-- Name: t2; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.t2 (
    c double precision
)
INHERITS (regress_rls_schema.t1);


--
-- Name: t2_3; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.t2_3 (
    a integer,
    b text
);


--
-- Name: t3_3; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.t3_3 (
    id integer NOT NULL,
    c text,
    b text,
    a integer
)
INHERITS (regress_rls_schema.t1_3);


--
-- Name: tbl1; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.tbl1 (
    c text
);


--
-- Name: test_qual_pushdown; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.test_qual_pushdown (
    abc text
);


--
-- Name: uaccount; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.uaccount (
    pguser name NOT NULL,
    seclv integer
);


--
-- Name: v2; Type: VIEW; Schema: regress_rls_schema; Owner: -
--

CREATE VIEW regress_rls_schema.v2 AS
 SELECT x,
    y
   FROM regress_rls_schema.s2
  WHERE (y ~~ '%af%'::text);


--
-- Name: x1; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.x1 (
    a integer,
    b text,
    c text
);


--
-- Name: y2; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.y2 (
    a integer,
    b text
);


--
-- Name: z1_blacklist; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.z1_blacklist (
    a integer
);


--
-- Name: z2; Type: TABLE; Schema: regress_rls_schema; Owner: -
--

CREATE TABLE regress_rls_schema.z2 (
    a integer,
    b text
);


--
-- Name: accounts; Type: TABLE; Schema: trigger_test; Owner: -
--

CREATE TABLE trigger_test.accounts (
    id integer NOT NULL,
    balance real
);


--
-- Name: accounts_view; Type: VIEW; Schema: trigger_test; Owner: -
--

CREATE VIEW trigger_test.accounts_view AS
 SELECT id,
    balance
   FROM trigger_test.accounts;


--
-- Name: update_log; Type: TABLE; Schema: trigger_test; Owner: -
--

CREATE TABLE trigger_test.update_log (
    "timestamp" timestamp without time zone DEFAULT now() NOT NULL,
    account_id integer NOT NULL
);


--
-- Name: tableam_parted_1_heapx; Type: TABLE ATTACH; Schema: am_examples; Owner: -
--

ALTER TABLE ONLY am_examples.tableam_parted_heapx ATTACH PARTITION am_examples.tableam_parted_1_heapx FOR VALUES IN ('a', 'b');


--
-- Name: tableam_parted_2_heapx; Type: TABLE ATTACH; Schema: am_examples; Owner: -
--

ALTER TABLE ONLY am_examples.tableam_parted_heapx ATTACH PARTITION am_examples.tableam_parted_2_heapx FOR VALUES IN ('c', 'd');


--
-- Name: tableam_parted_c_heap2; Type: TABLE ATTACH; Schema: am_examples; Owner: -
--

ALTER TABLE ONLY am_examples.tableam_parted_heap2 ATTACH PARTITION am_examples.tableam_parted_c_heap2 FOR VALUES IN ('c');


--
-- Name: tableam_parted_d_heap2; Type: TABLE ATTACH; Schema: am_examples; Owner: -
--

ALTER TABLE ONLY am_examples.tableam_parted_heap2 ATTACH PARTITION am_examples.tableam_parted_d_heap2 FOR VALUES IN ('d');


--
-- Name: part_document_fiction; Type: TABLE ATTACH; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE ONLY regress_rls_schema.part_document ATTACH PARTITION regress_rls_schema.part_document_fiction FOR VALUES FROM (11) TO (12);


--
-- Name: part_document_nonfiction; Type: TABLE ATTACH; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE ONLY regress_rls_schema.part_document ATTACH PARTITION regress_rls_schema.part_document_nonfiction FOR VALUES FROM (99) TO (100);


--
-- Name: part_document_satire; Type: TABLE ATTACH; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE ONLY regress_rls_schema.part_document ATTACH PARTITION regress_rls_schema.part_document_satire FOR VALUES FROM (55) TO (56);


--
-- Name: us_snail_addy address_id; Type: DEFAULT; Schema: domain_examples; Owner: -
--

ALTER TABLE ONLY domain_examples.us_snail_addy ALTER COLUMN address_id SET DEFAULT nextval('domain_examples.us_snail_addy_address_id_seq'::regclass);


--
-- Name: bugs id; Type: DEFAULT; Schema: enum_example; Owner: -
--

ALTER TABLE ONLY enum_example.bugs ALTER COLUMN id SET DEFAULT nextval('enum_example.bugs_id_seq'::regclass);


--
-- Name: bugs_clone id; Type: DEFAULT; Schema: enum_example; Owner: -
--

ALTER TABLE ONLY enum_example.bugs_clone ALTER COLUMN id SET DEFAULT nextval('enum_example.bugs_id_seq'::regclass);


--
-- Name: numeric_type_examples id; Type: DEFAULT; Schema: ordinary_tables; Owner: -
--

ALTER TABLE ONLY ordinary_tables.numeric_type_examples ALTER COLUMN id SET DEFAULT nextval('ordinary_tables.numeric_type_examples_id_seq'::regclass);


--
-- Name: numeric_type_examples a_smallserial; Type: DEFAULT; Schema: ordinary_tables; Owner: -
--

ALTER TABLE ONLY ordinary_tables.numeric_type_examples ALTER COLUMN a_smallserial SET DEFAULT nextval('ordinary_tables.numeric_type_examples_a_smallserial_seq'::regclass);


--
-- Name: numeric_type_examples a_bigserial; Type: DEFAULT; Schema: ordinary_tables; Owner: -
--

ALTER TABLE ONLY ordinary_tables.numeric_type_examples ALTER COLUMN a_bigserial SET DEFAULT nextval('ordinary_tables.numeric_type_examples_a_bigserial_seq'::regclass);


--
-- Name: us_snail_addy us_snail_addy_pkey; Type: CONSTRAINT; Schema: domain_examples; Owner: -
--

ALTER TABLE ONLY domain_examples.us_snail_addy
    ADD CONSTRAINT us_snail_addy_pkey PRIMARY KEY (address_id);


--
-- Name: films films_pkey; Type: CONSTRAINT; Schema: idx_ex; Owner: -
--

ALTER TABLE ONLY idx_ex.films
    ADD CONSTRAINT films_pkey PRIMARY KEY (id);


--
-- Name: binary_examples binary_examples_pkey; Type: CONSTRAINT; Schema: ordinary_tables; Owner: -
--

ALTER TABLE ONLY ordinary_tables.binary_examples
    ADD CONSTRAINT binary_examples_pkey PRIMARY KEY (bytes);


--
-- Name: character_examples character_examples_pkey; Type: CONSTRAINT; Schema: ordinary_tables; Owner: -
--

ALTER TABLE ONLY ordinary_tables.character_examples
    ADD CONSTRAINT character_examples_pkey PRIMARY KEY (id);


--
-- Name: numeric_type_examples numeric_type_examples_pkey; Type: CONSTRAINT; Schema: ordinary_tables; Owner: -
--

ALTER TABLE ONLY ordinary_tables.numeric_type_examples
    ADD CONSTRAINT numeric_type_examples_pkey PRIMARY KEY (id);


--
-- Name: category category_pkey; Type: CONSTRAINT; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE ONLY regress_rls_schema.category
    ADD CONSTRAINT category_pkey PRIMARY KEY (cid);


--
-- Name: document document_pkey; Type: CONSTRAINT; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE ONLY regress_rls_schema.document
    ADD CONSTRAINT document_pkey PRIMARY KEY (did);


--
-- Name: r1_3 r1_3_pkey; Type: CONSTRAINT; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE ONLY regress_rls_schema.r1_3
    ADD CONSTRAINT r1_3_pkey PRIMARY KEY (a);


--
-- Name: r1_4 r1_4_pkey; Type: CONSTRAINT; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE ONLY regress_rls_schema.r1_4
    ADD CONSTRAINT r1_4_pkey PRIMARY KEY (a);


--
-- Name: r1_5 r1_5_pkey; Type: CONSTRAINT; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE ONLY regress_rls_schema.r1_5
    ADD CONSTRAINT r1_5_pkey PRIMARY KEY (a);


--
-- Name: r1 r1_pkey; Type: CONSTRAINT; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE ONLY regress_rls_schema.r1
    ADD CONSTRAINT r1_pkey PRIMARY KEY (a);


--
-- Name: t1 t1_pkey; Type: CONSTRAINT; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE ONLY regress_rls_schema.t1
    ADD CONSTRAINT t1_pkey PRIMARY KEY (id);


--
-- Name: t3_3 t3_3_pkey; Type: CONSTRAINT; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE ONLY regress_rls_schema.t3_3
    ADD CONSTRAINT t3_3_pkey PRIMARY KEY (id);


--
-- Name: uaccount uaccount_pkey; Type: CONSTRAINT; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE ONLY regress_rls_schema.uaccount
    ADD CONSTRAINT uaccount_pkey PRIMARY KEY (pguser);


--
-- Name: accounts accounts_pkey; Type: CONSTRAINT; Schema: trigger_test; Owner: -
--

ALTER TABLE ONLY trigger_test.accounts
    ADD CONSTRAINT accounts_pkey PRIMARY KEY (id);


--
-- Name: update_log update_log_pk; Type: CONSTRAINT; Schema: trigger_test; Owner: -
--

ALTER TABLE ONLY trigger_test.update_log
    ADD CONSTRAINT update_log_pk PRIMARY KEY ("timestamp", account_id);


--
-- Name: grect2ind2; Type: INDEX; Schema: am_examples; Owner: -
--

CREATE INDEX grect2ind2 ON am_examples.fast_emp4000 USING gist2 (home_base);


--
-- Name: grect2ind3; Type: INDEX; Schema: am_examples; Owner: -
--

CREATE INDEX grect2ind3 ON am_examples.fast_emp4000 USING gist2 (home_base);


--
-- Name: idx_1; Type: INDEX; Schema: composite_type_examples; Owner: -
--

CREATE INDEX idx_1 ON composite_type_examples.ordinary_table USING btree (basic_);


--
-- Name: hidx; Type: INDEX; Schema: extension_example; Owner: -
--

CREATE INDEX hidx ON extension_example.testhstore USING gist (h extension_example.gist_hstore_ops (siglen='32'));


--
-- Name: gin_idx; Type: INDEX; Schema: idx_ex; Owner: -
--

CREATE INDEX gin_idx ON idx_ex.films USING gin (to_tsvector('english'::regconfig, title)) WITH (fastupdate=off);


--
-- Name: title_idx; Type: INDEX; Schema: idx_ex; Owner: -
--

CREATE UNIQUE INDEX title_idx ON idx_ex.films USING btree (title) WITH (fillfactor='70');


--
-- Name: title_idx_lower; Type: INDEX; Schema: idx_ex; Owner: -
--

CREATE INDEX title_idx_lower ON idx_ex.films USING btree (lower(title));


--
-- Name: title_idx_nulls_low; Type: INDEX; Schema: idx_ex; Owner: -
--

CREATE INDEX title_idx_nulls_low ON idx_ex.films USING btree (title NULLS FIRST);


--
-- Name: title_idx_u1; Type: INDEX; Schema: idx_ex; Owner: -
--

CREATE UNIQUE INDEX title_idx_u1 ON idx_ex.films USING btree (title);


--
-- Name: title_idx_u2; Type: INDEX; Schema: idx_ex; Owner: -
--

CREATE UNIQUE INDEX title_idx_u2 ON idx_ex.films USING btree (title) INCLUDE (director, rating);


--
-- Name: title_idx_with_duplicates; Type: INDEX; Schema: idx_ex; Owner: -
--

CREATE INDEX title_idx_with_duplicates ON idx_ex.films USING btree (title) WITH (deduplicate_items=off);


--
-- Name: accounts check_balance_update; Type: TRIGGER; Schema: trigger_test; Owner: -
--

CREATE TRIGGER check_balance_update BEFORE UPDATE OF balance ON trigger_test.accounts FOR EACH ROW EXECUTE FUNCTION trigger_test.check_account_update();


--
-- Name: accounts check_update; Type: TRIGGER; Schema: trigger_test; Owner: -
--

CREATE TRIGGER check_update BEFORE UPDATE ON trigger_test.accounts FOR EACH ROW EXECUTE FUNCTION trigger_test.check_account_update();


--
-- Name: accounts check_update_when_difft_balance; Type: TRIGGER; Schema: trigger_test; Owner: -
--

CREATE TRIGGER check_update_when_difft_balance BEFORE UPDATE ON trigger_test.accounts FOR EACH ROW WHEN ((old.balance IS DISTINCT FROM new.balance)) EXECUTE FUNCTION trigger_test.check_account_update();


--
-- Name: accounts log_update; Type: TRIGGER; Schema: trigger_test; Owner: -
--

CREATE TRIGGER log_update AFTER UPDATE ON trigger_test.accounts FOR EACH ROW WHEN ((old.* IS DISTINCT FROM new.*)) EXECUTE FUNCTION trigger_test.log_account_update();


--
-- Name: accounts_view view_insert; Type: TRIGGER; Schema: trigger_test; Owner: -
--

CREATE TRIGGER view_insert INSTEAD OF INSERT ON trigger_test.accounts_view FOR EACH ROW EXECUTE FUNCTION trigger_test.view_insert_row();


--
-- Name: document document_cid_fkey; Type: FK CONSTRAINT; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE ONLY regress_rls_schema.document
    ADD CONSTRAINT document_cid_fkey FOREIGN KEY (cid) REFERENCES regress_rls_schema.category(cid);


--
-- Name: r2_3 r2_3_a_fkey; Type: FK CONSTRAINT; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE ONLY regress_rls_schema.r2_3
    ADD CONSTRAINT r2_3_a_fkey FOREIGN KEY (a) REFERENCES regress_rls_schema.r1(a);


--
-- Name: r2_4 r2_4_a_fkey; Type: FK CONSTRAINT; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE ONLY regress_rls_schema.r2_4
    ADD CONSTRAINT r2_4_a_fkey FOREIGN KEY (a) REFERENCES regress_rls_schema.r1(a) ON DELETE CASCADE;


--
-- Name: r2_5 r2_5_a_fkey; Type: FK CONSTRAINT; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE ONLY regress_rls_schema.r2_5
    ADD CONSTRAINT r2_5_a_fkey FOREIGN KEY (a) REFERENCES regress_rls_schema.r1(a) ON UPDATE CASCADE;


--
-- Name: update_log update_log_account_id_fkey; Type: FK CONSTRAINT; Schema: trigger_test; Owner: -
--

ALTER TABLE ONLY trigger_test.update_log
    ADD CONSTRAINT update_log_account_id_fkey FOREIGN KEY (account_id) REFERENCES trigger_test.accounts(id);


--
-- Name: b1; Type: ROW SECURITY; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE regress_rls_schema.b1 ENABLE ROW LEVEL SECURITY;

--
-- Name: dependent d1; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY d1 ON regress_rls_schema.dependent USING ((x = ( SELECT d.x
   FROM regress_rls_schema.dependee d
  WHERE (d.y = d.y))));


--
-- Name: document; Type: ROW SECURITY; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE regress_rls_schema.document ENABLE ROW LEVEL SECURITY;

--
-- Name: t p; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p ON regress_rls_schema.t USING (((c % 2) = 1));


--
-- Name: tbl1 p; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p ON regress_rls_schema.tbl1 TO regress_rls_eve, regress_rls_frank USING (true);


--
-- Name: x1 p0; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p0 ON regress_rls_schema.x1 USING ((c = CURRENT_USER));


--
-- Name: b1 p1; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p1 ON regress_rls_schema.b1 USING (((a % 2) = 0));


--
-- Name: dob_t1 p1; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p1 ON regress_rls_schema.dob_t1 TO regress_rls_dob_role1 USING (true);


--
-- Name: dob_t2 p1; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p1 ON regress_rls_schema.dob_t2 TO regress_rls_dob_role1, regress_rls_dob_role2 USING (true);


--
-- Name: document p1; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p1 ON regress_rls_schema.document USING ((dlevel <= ( SELECT uaccount.seclv
   FROM regress_rls_schema.uaccount
  WHERE (uaccount.pguser = CURRENT_USER))));


--
-- Name: r1 p1; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p1 ON regress_rls_schema.r1 USING (true);


--
-- Name: r1_2 p1; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p1 ON regress_rls_schema.r1_2 USING (false);


--
-- Name: r1_3 p1; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p1 ON regress_rls_schema.r1_3 USING (false);


--
-- Name: r2 p1; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p1 ON regress_rls_schema.r2 FOR SELECT USING (true);


--
-- Name: r2_3 p1; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p1 ON regress_rls_schema.r2_3 USING (false);


--
-- Name: r2_4 p1; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p1 ON regress_rls_schema.r2_4 USING (false);


--
-- Name: r2_5 p1; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p1 ON regress_rls_schema.r2_5 USING (false);


--
-- Name: rls_tbl p1; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p1 ON regress_rls_schema.rls_tbl USING ((EXISTS ( SELECT 1
   FROM regress_rls_schema.ref_tbl)));


--
-- Name: rls_tbl_3 p1; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p1 ON regress_rls_schema.rls_tbl_3 USING ((rls_tbl_3.* >= ROW(1, 1, 1)));


--
-- Name: s1 p1; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p1 ON regress_rls_schema.s1 USING ((a IN ( SELECT s2.x
   FROM regress_rls_schema.s2
  WHERE (s2.y ~~ '%2f%'::text))));


--
-- Name: t1 p1; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p1 ON regress_rls_schema.t1 USING (((a % 2) = 0));


--
-- Name: t1_2 p1; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p1 ON regress_rls_schema.t1_2 TO regress_rls_bob USING (((a % 2) = 0));


--
-- Name: t1_3 p1; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p1 ON regress_rls_schema.t1_3 USING (((a % 2) = 0));


--
-- Name: x1 p1; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p1 ON regress_rls_schema.x1 FOR SELECT USING (((a % 2) = 0));


--
-- Name: y1 p1; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p1 ON regress_rls_schema.y1 USING (((a % 2) = 0));


--
-- Name: y2 p1; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p1 ON regress_rls_schema.y2 USING (((a % 2) = 0));


--
-- Name: z1 p1; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p1 ON regress_rls_schema.z1 TO regress_rls_group1 USING (((a % 2) = 0));


--
-- Name: dob_t1 p1_2; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p1_2 ON regress_rls_schema.dob_t1 TO regress_rls_dob_role1, regress_rls_dob_role2 USING (true);


--
-- Name: s1 p1_2; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p1_2 ON regress_rls_schema.s1 USING ((a IN ( SELECT v2.x
   FROM regress_rls_schema.v2)));


--
-- Name: t1_3 p1_2; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p1_2 ON regress_rls_schema.t1_3 USING (((a % 2) = 0));


--
-- Name: y1 p1_2; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p1_2 ON regress_rls_schema.y1 FOR SELECT USING (((a % 2) = 1));


--
-- Name: dob_t1 p1_3; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p1_3 ON regress_rls_schema.dob_t1 TO regress_rls_dob_role1 USING (true);


--
-- Name: document p1_3; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p1_3 ON regress_rls_schema.document FOR SELECT USING (true);


--
-- Name: dob_t1 p1_4; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p1_4 ON regress_rls_schema.dob_t1 TO regress_rls_dob_role1, regress_rls_dob_role2 USING (true);


--
-- Name: document p1_4; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p1_4 ON regress_rls_schema.document FOR SELECT USING (true);


--
-- Name: document p1r; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p1r ON regress_rls_schema.document AS RESTRICTIVE TO regress_rls_dave USING ((cid <> 44));


--
-- Name: r2 p2; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p2 ON regress_rls_schema.r2 FOR INSERT WITH CHECK (false);


--
-- Name: s2 p2; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p2 ON regress_rls_schema.s2 USING ((x IN ( SELECT s1.a
   FROM regress_rls_schema.s1
  WHERE (s1.b ~~ '%22%'::text))));


--
-- Name: t1_2 p2; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p2 ON regress_rls_schema.t1_2 TO regress_rls_carol USING (((a % 4) = 0));


--
-- Name: t2 p2; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p2 ON regress_rls_schema.t2 USING (((a % 2) = 1));


--
-- Name: t2_3 p2; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p2 ON regress_rls_schema.t2_3 USING (((a % 2) = 1));


--
-- Name: x1 p2; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p2 ON regress_rls_schema.x1 FOR INSERT WITH CHECK (((a % 2) = 1));


--
-- Name: y1 p2; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p2 ON regress_rls_schema.y1 FOR SELECT USING ((a > 2));


--
-- Name: y2 p2; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p2 ON regress_rls_schema.y2 USING (((a % 3) = 0));


--
-- Name: z1 p2; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p2 ON regress_rls_schema.z1 TO regress_rls_group2 USING (((a % 2) = 1));


--
-- Name: s2 p2_2; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p2_2 ON regress_rls_schema.s2 USING ((x IN ( SELECT s1.a
   FROM regress_rls_schema.s1
  WHERE (s1.b ~~ '%d2%'::text))));


--
-- Name: document p2_3; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p2_3 ON regress_rls_schema.document FOR INSERT WITH CHECK ((dauthor = CURRENT_USER));


--
-- Name: document p2_4; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p2_4 ON regress_rls_schema.document FOR INSERT WITH CHECK ((dauthor = CURRENT_USER));


--
-- Name: document p2r; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p2r ON regress_rls_schema.document AS RESTRICTIVE TO regress_rls_dave USING (((cid <> 44) AND (cid < 50)));


--
-- Name: r2 p3; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p3 ON regress_rls_schema.r2 FOR UPDATE USING (false);


--
-- Name: s1 p3; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p3 ON regress_rls_schema.s1 FOR INSERT WITH CHECK ((a = ( SELECT s1_1.a
   FROM regress_rls_schema.s1 s1_1)));


--
-- Name: x1 p3; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p3 ON regress_rls_schema.x1 FOR UPDATE USING (((a % 2) = 0));


--
-- Name: y2 p3; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p3 ON regress_rls_schema.y2 USING (((a % 4) = 0));


--
-- Name: z1 p3; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p3 ON regress_rls_schema.z1 AS RESTRICTIVE USING ((NOT (a IN ( SELECT z1_blacklist.a
   FROM regress_rls_schema.z1_blacklist))));


--
-- Name: z1 p3_2; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p3_2 ON regress_rls_schema.z1 AS RESTRICTIVE USING ((NOT (a IN ( SELECT z1_blacklist.a
   FROM regress_rls_schema.z1_blacklist))));


--
-- Name: document p3_3; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p3_3 ON regress_rls_schema.document FOR UPDATE USING ((cid = ( SELECT category.cid
   FROM regress_rls_schema.category
  WHERE (category.cname = 'novel'::text)))) WITH CHECK ((dauthor = CURRENT_USER));


--
-- Name: document p3_4; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p3_4 ON regress_rls_schema.document FOR UPDATE USING ((cid = ( SELECT category.cid
   FROM regress_rls_schema.category
  WHERE (category.cname = 'novel'::text)))) WITH CHECK ((dauthor = CURRENT_USER));


--
-- Name: document p3_with_all; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p3_with_all ON regress_rls_schema.document USING ((cid = ( SELECT category.cid
   FROM regress_rls_schema.category
  WHERE (category.cname = 'novel'::text)))) WITH CHECK ((dauthor = CURRENT_USER));


--
-- Name: document p3_with_default; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p3_with_default ON regress_rls_schema.document FOR UPDATE USING ((cid = ( SELECT category.cid
   FROM regress_rls_schema.category
  WHERE (category.cname = 'novel'::text))));


--
-- Name: r2 p4; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p4 ON regress_rls_schema.r2 FOR DELETE USING (false);


--
-- Name: x1 p4; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p4 ON regress_rls_schema.x1 FOR DELETE USING ((a < 8));


--
-- Name: document p4_4; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY p4_4 ON regress_rls_schema.document FOR DELETE USING ((cid = ( SELECT category.cid
   FROM regress_rls_schema.category
  WHERE (category.cname = 'manga'::text))));


--
-- Name: part_document; Type: ROW SECURITY; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE regress_rls_schema.part_document ENABLE ROW LEVEL SECURITY;

--
-- Name: part_document pp1; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY pp1 ON regress_rls_schema.part_document USING ((dlevel <= ( SELECT uaccount.seclv
   FROM regress_rls_schema.uaccount
  WHERE (uaccount.pguser = CURRENT_USER))));


--
-- Name: part_document pp1r; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY pp1r ON regress_rls_schema.part_document AS RESTRICTIVE TO regress_rls_dave USING ((cid < 55));


--
-- Name: r1; Type: ROW SECURITY; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE regress_rls_schema.r1 ENABLE ROW LEVEL SECURITY;

--
-- Name: rec1 r1; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY r1 ON regress_rls_schema.rec1 USING ((x = ( SELECT r.x
   FROM regress_rls_schema.rec1 r
  WHERE (r.y = r.y))));


--
-- Name: r1_2; Type: ROW SECURITY; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE regress_rls_schema.r1_2 ENABLE ROW LEVEL SECURITY;

--
-- Name: rec1 r1_2; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY r1_2 ON regress_rls_schema.rec1 USING ((x = ( SELECT rec2.a
   FROM regress_rls_schema.rec2
  WHERE (rec2.b = rec1.y))));


--
-- Name: r1_3; Type: ROW SECURITY; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE regress_rls_schema.r1_3 ENABLE ROW LEVEL SECURITY;

--
-- Name: rec1 r1_3; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY r1_3 ON regress_rls_schema.rec1 USING ((x = ( SELECT rec2v.a
   FROM regress_rls_schema.rec2v
  WHERE (rec2v.b = rec1.y))));


--
-- Name: rec1 r1_4; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY r1_4 ON regress_rls_schema.rec1 USING ((x = ( SELECT rec2v_2.a
   FROM regress_rls_schema.rec2v_2
  WHERE (rec2v_2.b = rec1.y))));


--
-- Name: r2; Type: ROW SECURITY; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE regress_rls_schema.r2 ENABLE ROW LEVEL SECURITY;

--
-- Name: rec2 r2; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY r2 ON regress_rls_schema.rec2 USING ((a = ( SELECT rec1.x
   FROM regress_rls_schema.rec1
  WHERE (rec1.y = rec2.b))));


--
-- Name: rec2 r2_2; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY r2_2 ON regress_rls_schema.rec2 USING ((a = ( SELECT rec1v_2.x
   FROM regress_rls_schema.rec1v_2
  WHERE (rec1v_2.y = rec2.b))));


--
-- Name: r2_3; Type: ROW SECURITY; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE regress_rls_schema.r2_3 ENABLE ROW LEVEL SECURITY;

--
-- Name: rec2 r2_3; Type: POLICY; Schema: regress_rls_schema; Owner: -
--

CREATE POLICY r2_3 ON regress_rls_schema.rec2 USING ((a = ( SELECT rec1v.x
   FROM regress_rls_schema.rec1v
  WHERE (rec1v.y = rec2.b))));


--
-- Name: r2_4; Type: ROW SECURITY; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE regress_rls_schema.r2_4 ENABLE ROW LEVEL SECURITY;

--
-- Name: r2_5; Type: ROW SECURITY; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE regress_rls_schema.r2_5 ENABLE ROW LEVEL SECURITY;

--
-- Name: rec1; Type: ROW SECURITY; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE regress_rls_schema.rec1 ENABLE ROW LEVEL SECURITY;

--
-- Name: rec2; Type: ROW SECURITY; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE regress_rls_schema.rec2 ENABLE ROW LEVEL SECURITY;

--
-- Name: rls_tbl; Type: ROW SECURITY; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE regress_rls_schema.rls_tbl ENABLE ROW LEVEL SECURITY;

--
-- Name: rls_tbl_2; Type: ROW SECURITY; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE regress_rls_schema.rls_tbl_2 ENABLE ROW LEVEL SECURITY;

--
-- Name: rls_tbl_3; Type: ROW SECURITY; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE regress_rls_schema.rls_tbl_3 ENABLE ROW LEVEL SECURITY;

--
-- Name: s1; Type: ROW SECURITY; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE regress_rls_schema.s1 ENABLE ROW LEVEL SECURITY;

--
-- Name: s2; Type: ROW SECURITY; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE regress_rls_schema.s2 ENABLE ROW LEVEL SECURITY;

--
-- Name: t; Type: ROW SECURITY; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE regress_rls_schema.t ENABLE ROW LEVEL SECURITY;

--
-- Name: t1; Type: ROW SECURITY; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE regress_rls_schema.t1 ENABLE ROW LEVEL SECURITY;

--
-- Name: t1_2; Type: ROW SECURITY; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE regress_rls_schema.t1_2 ENABLE ROW LEVEL SECURITY;

--
-- Name: t1_3; Type: ROW SECURITY; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE regress_rls_schema.t1_3 ENABLE ROW LEVEL SECURITY;

--
-- Name: t2; Type: ROW SECURITY; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE regress_rls_schema.t2 ENABLE ROW LEVEL SECURITY;

--
-- Name: t2_3; Type: ROW SECURITY; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE regress_rls_schema.t2_3 ENABLE ROW LEVEL SECURITY;

--
-- Name: x1; Type: ROW SECURITY; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE regress_rls_schema.x1 ENABLE ROW LEVEL SECURITY;

--
-- Name: y1; Type: ROW SECURITY; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE regress_rls_schema.y1 ENABLE ROW LEVEL SECURITY;

--
-- Name: y2; Type: ROW SECURITY; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE regress_rls_schema.y2 ENABLE ROW LEVEL SECURITY;

--
-- Name: z1; Type: ROW SECURITY; Schema: regress_rls_schema; Owner: -
--

ALTER TABLE regress_rls_schema.z1 ENABLE ROW LEVEL SECURITY;

--
-- PostgreSQL database dump complete
--

