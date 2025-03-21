-- adapted from https://raw.githubusercontent.com/postgres/postgres/master/src/test/regress/sql/create_type.sql
-- https://www.postgresql.org/docs/current/sql-createtype.html

CREATE USER base_type_owner;
CREATE SCHEMA base_type_examples;


-- Test creation and destruction of shell types
CREATE TYPE base_type_examples.shell;

--
-- Test type-related default values (broken in releases before PG 7.2)
--
-- This part of the test also exercises the "new style" approach of making
-- a shell type and then filling it in.
--
CREATE TYPE base_type_examples.int42;
CREATE TYPE base_type_examples.text_w_default;

-- Make dummy I/O routines using the existing internal support for int4, text
CREATE FUNCTION base_type_examples.int42_in(cstring)
   RETURNS base_type_examples.int42
   AS 'int4in'
   LANGUAGE internal STRICT IMMUTABLE;
CREATE FUNCTION base_type_examples.int42_out(base_type_examples.int42)
   RETURNS cstring
   AS 'int4out'
   LANGUAGE internal STRICT IMMUTABLE;
CREATE FUNCTION base_type_examples.text_w_default_in(cstring)
   RETURNS base_type_examples.text_w_default
   AS 'textin'
   LANGUAGE internal STRICT IMMUTABLE;
CREATE FUNCTION base_type_examples.text_w_default_out(base_type_examples.text_w_default)
   RETURNS cstring
   AS 'textout'
   LANGUAGE internal STRICT IMMUTABLE;

CREATE TYPE base_type_examples.int42 (
   internallength = 4,
   input = base_type_examples.int42_in,
   output = base_type_examples.int42_out,
   alignment = int4,
   default = 42,
   passedbyvalue
);

CREATE TYPE base_type_examples.text_w_default (
   internallength = variable,
   input = base_type_examples.text_w_default_in,
   output = base_type_examples.text_w_default_out,
   alignment = int4,
   default = 'zippo'
);

CREATE TABLE base_type_examples.default_test (f1 base_type_examples.text_w_default, f2 base_type_examples.int42);

-- Test stand-alone composite type

CREATE TYPE base_type_examples.default_test_row AS (f1 base_type_examples.text_w_default, f2 base_type_examples.int42);

CREATE FUNCTION base_type_examples.get_default_test() RETURNS SETOF base_type_examples.default_test_row AS '
  SELECT * FROM base_type_examples.default_test;
' LANGUAGE SQL;

-- Test comments
COMMENT ON TYPE base_type_examples.default_test_row IS 'good comment';
COMMENT ON COLUMN base_type_examples.default_test_row.f1 IS 'good comment';
COMMENT ON COLUMN base_type_examples.default_test_row.f2 IS NULL;


-- Check dependencies are established when creating a new type
CREATE TYPE base_type_examples.base_type;
CREATE FUNCTION base_type_examples.base_fn_in(cstring) RETURNS base_type_examples.base_type AS 'boolin'
    LANGUAGE internal IMMUTABLE STRICT;
CREATE FUNCTION base_type_examples.base_fn_out(base_type_examples.base_type) RETURNS cstring AS 'boolout'
    LANGUAGE internal IMMUTABLE STRICT;
CREATE TYPE base_type_examples.base_type(INPUT = base_type_examples.base_fn_in, OUTPUT = base_type_examples.base_fn_out);


-- Test creation of an operator over a user-defined type
CREATE FUNCTION base_type_examples.fake_op(point, base_type_examples.int42)
   RETURNS bool
   AS $$ select true $$
   LANGUAGE SQL IMMUTABLE;

CREATE OPERATOR <% (
   leftarg = point,
   rightarg = base_type_examples.int42,
   procedure = base_type_examples.fake_op,
   commutator = >% ,
   negator = >=%
);

--
-- Test CREATE/ALTER TYPE using a type that's compatible with varchar,
-- so we can re-use those support functions
--
CREATE TYPE base_type_examples.myvarchar;

CREATE FUNCTION base_type_examples.myvarcharin(cstring, oid, integer) RETURNS base_type_examples.myvarchar
LANGUAGE internal IMMUTABLE PARALLEL SAFE STRICT AS 'varcharin';

CREATE FUNCTION base_type_examples.myvarcharout(base_type_examples.myvarchar) RETURNS cstring
LANGUAGE internal IMMUTABLE PARALLEL SAFE STRICT AS 'varcharout';

CREATE FUNCTION base_type_examples.myvarcharsend(base_type_examples.myvarchar) RETURNS bytea
LANGUAGE internal STABLE PARALLEL SAFE STRICT AS 'varcharsend';

CREATE FUNCTION base_type_examples.myvarcharrecv(internal, oid, integer) RETURNS base_type_examples.myvarchar
LANGUAGE internal STABLE PARALLEL SAFE STRICT AS 'varcharrecv';

CREATE TYPE base_type_examples.myvarchar (
    input = base_type_examples.myvarcharin,
    output = base_type_examples.myvarcharout,
    alignment = integer,
    storage = main
);

-- want to check updating of a domain over the target type, too
CREATE DOMAIN base_type_examples.myvarchardom AS base_type_examples.myvarchar;

ALTER TYPE base_type_examples.myvarchar SET (storage = extended);

ALTER TYPE base_type_examples.myvarchar SET (
    send = base_type_examples.myvarcharsend,
    receive = base_type_examples.myvarcharrecv,
    typmod_in = varchartypmodin,
    typmod_out = varchartypmodout,
    -- these are bogus, but it's safe as long as we don't use the type:
    analyze = ts_typanalyze,
    subscript = raw_array_subscript_handler
);
