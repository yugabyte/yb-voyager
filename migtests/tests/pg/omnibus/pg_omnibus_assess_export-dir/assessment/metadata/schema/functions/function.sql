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


CREATE FUNCTION base_type_examples.base_fn_in(cstring) RETURNS base_type_examples.base_type
    LANGUAGE internal IMMUTABLE STRICT
    AS $$boolin$$;


CREATE FUNCTION base_type_examples.base_fn_out(base_type_examples.base_type) RETURNS cstring
    LANGUAGE internal IMMUTABLE STRICT
    AS $$boolout$$;


CREATE FUNCTION base_type_examples.int42_in(cstring) RETURNS base_type_examples.int42
    LANGUAGE internal IMMUTABLE STRICT
    AS $$int4in$$;


CREATE FUNCTION base_type_examples.int42_out(base_type_examples.int42) RETURNS cstring
    LANGUAGE internal IMMUTABLE STRICT
    AS $$int4out$$;


CREATE FUNCTION base_type_examples.text_w_default_in(cstring) RETURNS base_type_examples.text_w_default
    LANGUAGE internal IMMUTABLE STRICT
    AS $$textin$$;


CREATE FUNCTION base_type_examples.text_w_default_out(base_type_examples.text_w_default) RETURNS cstring
    LANGUAGE internal IMMUTABLE STRICT
    AS $$textout$$;


CREATE FUNCTION base_type_examples.myvarcharin(cstring, oid, integer) RETURNS base_type_examples.myvarchar
    LANGUAGE internal IMMUTABLE STRICT PARALLEL SAFE
    AS $$varcharin$$;


CREATE FUNCTION base_type_examples.myvarcharout(base_type_examples.myvarchar) RETURNS cstring
    LANGUAGE internal IMMUTABLE STRICT PARALLEL SAFE
    AS $$varcharout$$;


CREATE FUNCTION base_type_examples.myvarcharrecv(internal, oid, integer) RETURNS base_type_examples.myvarchar
    LANGUAGE internal STABLE STRICT PARALLEL SAFE
    AS $$varcharrecv$$;


CREATE FUNCTION base_type_examples.myvarcharsend(base_type_examples.myvarchar) RETURNS bytea
    LANGUAGE internal STABLE STRICT PARALLEL SAFE
    AS $$varcharsend$$;


CREATE FUNCTION domain_examples.is_positive(i integer) RETURNS boolean
    LANGUAGE sql IMMUTABLE
    AS $$
  SELECT i > 0
$$;


CREATE FUNCTION domain_examples.is_even(i domain_examples.positive_number) RETURNS boolean
    LANGUAGE sql IMMUTABLE
    AS $$
  SELECT (i % 2) = 0
$$;


CREATE FUNCTION foreign_db_example.is_positive(i integer) RETURNS boolean
    LANGUAGE sql IMMUTABLE
    AS $$
  SELECT i > 0
$$;


CREATE FUNCTION range_type_example.my_float8mi(a double precision, b double precision) RETURNS double precision
    LANGUAGE sql IMMUTABLE
    AS $$
    SELECT float8mi(a,b)
  $$;


CREATE FUNCTION create_cast.transmogrify(input create_cast.abc) RETURNS integer
    LANGUAGE sql IMMUTABLE
    AS $$
  SELECT 1;
$$;


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


CREATE FUNCTION base_type_examples.fake_op(point, base_type_examples.int42) RETURNS boolean
    LANGUAGE sql IMMUTABLE
    AS $$ select true $$;


CREATE FUNCTION base_type_examples.get_default_test() RETURNS SETOF base_type_examples.default_test_row
    LANGUAGE sql
    AS $$
  SELECT * FROM base_type_examples.default_test;
$$;


CREATE FUNCTION composite_type_examples.get_basic() RETURNS SETOF composite_type_examples.basic_comp_type
    LANGUAGE sql
    AS $$
  SELECT f1, f2 FROM composite_type_examples.equivalent_rowtype
$$;


CREATE FUNCTION enum_example.make_bug_info(status enum_example.bug_status, severity enum_example.bug_severity) RETURNS enum_example.bug_info
    LANGUAGE sql IMMUTABLE
    AS $$
    SELECT status, severity
  $$;


CREATE FUNCTION enum_example.should_raise_alarm(info enum_example.bug_info) RETURNS boolean
    LANGUAGE sql IMMUTABLE
    AS $$
    SELECT info.status = 'new' AND info.severity = 'high'
  $$;


CREATE FUNCTION extension_example._hstore(r record) RETURNS extension_example.hstore
    LANGUAGE plpgsql IMMUTABLE STRICT
    AS $$
    BEGIN
      return extension_example.hstore(r);
    END;
  $$;


CREATE FUNCTION fn_examples.depends_on_table_column() RETURNS integer
    LANGUAGE sql
    AS $$
  SELECT id FROM fn_examples.ordinary_table LIMIT 1
$$;


CREATE FUNCTION fn_examples.depends_on_table_column_type() RETURNS integer
    LANGUAGE sql
    AS $$
    SELECT id FROM fn_examples.ordinary_table LIMIT 1
  $$;


CREATE FUNCTION fn_examples.depends_on_table_rowtype() RETURNS fn_examples.ordinary_table
    LANGUAGE sql
    AS $$
  SELECT * FROM fn_examples.ordinary_table LIMIT 1
$$;


CREATE FUNCTION fn_examples.depends_on_view_column() RETURNS integer
    LANGUAGE sql
    AS $$
  SELECT id FROM fn_examples.basic_view LIMIT 1
$$;


CREATE FUNCTION fn_examples.depends_on_view_column_type() RETURNS integer
    LANGUAGE sql
    AS $$
  SELECT id FROM fn_examples.basic_view LIMIT 1
$$;


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


CREATE FUNCTION fn_examples.is_odd(i integer) RETURNS boolean
    LANGUAGE sql
    AS $$
  SELECT (fn_examples.is_even(i) IS NOT true)
$$;


CREATE FUNCTION fn_examples.polyf(x anyelement) RETURNS anyelement
    LANGUAGE sql
    AS $$
  select x + 1
$$;


CREATE FUNCTION public.log_table_alteration() RETURNS event_trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
  RAISE NOTICE 'command % issued', tg_tag;
END;
$$;


CREATE FUNCTION range_type_example.arg_depends_on_range_type(r range_type_example.float8_range) RETURNS boolean
    LANGUAGE sql IMMUTABLE
    AS $$ SELECT true $$;


CREATE FUNCTION range_type_example.return_depends_on_range_type() RETURNS range_type_example.float8_range
    LANGUAGE sql IMMUTABLE
    AS $$
    SELECT '[1.2, 3.4]'::range_type_example.float8_range
  $$;


CREATE FUNCTION regress_rls_schema.f_leak(text) RETURNS boolean
    LANGUAGE plpgsql COST 1e-07
    AS $_$BEGIN RAISE NOTICE 'f_leak => %', $1; RETURN true; END$_$;


CREATE FUNCTION regress_rls_schema.op_leak(integer, integer) RETURNS boolean
    LANGUAGE plpgsql
    AS $_$BEGIN RAISE NOTICE 'op_leak => %, %', $1, $2; RETURN $1 < $2; END$_$;


CREATE FUNCTION trigger_test.check_account_update() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
    BEGIN
      RAISE NOTICE 'trigger_func(%) called: action = %, when = %, level = %, old = %, new = %',
                        TG_ARGV[0], TG_OP, TG_WHEN, TG_LEVEL, OLD, NEW;
      RETURN NULL;
    END;
  $$;


CREATE FUNCTION trigger_test.log_account_update() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
    BEGIN
      INSERT INTO trigger_test.update_log(account_id) VALUES (1);
      RETURN NULL;
    END;
  $$;


CREATE FUNCTION trigger_test.view_insert_row() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
    BEGIN
      INSERT INTO trigger_test.accounts(id, balance) VALUES (NEW.id, NEW.balance);
      RETURN NEW;
    END;
  $$;


