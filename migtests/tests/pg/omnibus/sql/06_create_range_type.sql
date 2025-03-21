-- https://www.postgresql.org/docs/current/sql-createtype.html
-- https://www.postgresql.org/docs/current/rangetypes.html
CREATE USER range_type_owner;
CREATE SCHEMA range_type_example;

-- TODO: SUBTYPE_OPCLASS = subtype_operator_class
-- TODO: COLLATION = collation
-- TODO: CANONICAL = canonical_function

CREATE FUNCTION range_type_example.my_float8mi(a double precision, b double precision)
  RETURNS DOUBLE PRECISION
  LANGUAGE SQL IMMUTABLE
  AS $$
    SELECT float8mi(a,b)
  $$;

CREATE TYPE range_type_example.float8_range AS RANGE (
    subtype = float8
  , subtype_diff = range_type_example.my_float8mi
);
COMMENT ON TYPE range_type_example.float8_range IS 'RANGE test';



CREATE TABLE range_type_example.example_tbl(
  col range_type_example.float8_range
);
CREATE FUNCTION range_type_example.arg_depends_on_range_type(r range_type_example.float8_range) RETURNS BOOLEAN LANGUAGE SQL IMMUTABLE AS $$ SELECT true $$;
CREATE FUNCTION range_type_example.return_depends_on_range_type()
  RETURNS range_type_example.float8_range
  LANGUAGE SQL IMMUTABLE AS $$
    SELECT '[1.2, 3.4]'::range_type_example.float8_range
  $$;
CREATE VIEW range_type_example.depends_on_col_using_type AS SELECT col FROM range_type_example.example_tbl;
