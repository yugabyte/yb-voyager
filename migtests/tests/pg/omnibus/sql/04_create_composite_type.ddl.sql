-- https://www.postgresql.org/docs/current/rowtypes.html
-- https://www.postgresql.org/docs/current/sql-createtype.html
CREATE USER composite_type_owner;
CREATE SCHEMA composite_type_examples;
CREATE TYPE composite_type_examples.basic_comp_type AS (f1 int, f2 text);
CREATE TYPE composite_type_examples.enum_abc AS ENUM('a', 'b', 'c');
CREATE TYPE composite_type_examples.nested AS (
    foo composite_type_examples.basic_comp_type
  , bar composite_type_examples.enum_abc
);
CREATE TABLE composite_type_examples.equivalent_rowtype (f1 int, f2 text);
-- TODO: figure out the limit for nesting
CREATE FUNCTION composite_type_examples.get_basic() RETURNS SETOF composite_type_examples.basic_comp_type AS $$
  SELECT f1, f2 FROM composite_type_examples.equivalent_rowtype
$$ LANGUAGE SQL;

CREATE TABLE composite_type_examples.ordinary_table(
    basic_ composite_type_examples.basic_comp_type
  , CONSTRAINT check_f1_gt_1 CHECK ((basic_).f1 > 1)
  , _basic composite_type_examples.basic_comp_type GENERATED ALWAYS AS (basic_) STORED
  , CONSTRAINT check_f1_gt_1_again CHECK ((_basic).f1 > 1)
  , nested composite_type_examples.nested
  , CONSTRAINT check_nested_f1_gt_1 CHECK (((nested).foo).f1 > 1)
  , _nested composite_type_examples.nested GENERATED ALWAYS AS (nested) STORED
  , CONSTRAINT check_nested_f1_gt_1_again CHECK (((_nested).foo).f1 > 1)
);
CREATE INDEX idx_1 ON composite_type_examples.ordinary_table(basic_);

CREATE TABLE composite_type_examples.inherited_table ()
  INHERITS (composite_type_examples.ordinary_table);

CREATE VIEW composite_type_examples.basic_view AS
  SELECT * FROM composite_type_examples.ordinary_table;

DO $$
  BEGIN
    CREATE TABLE composite_type_examples.i_0(i INT);
    -- loop, creating nested composite types. If there's a limit, it's at least 256 deep.
    FOR i IN 1..256 LOOP
      EXECUTE 'CREATE TABLE composite_type_examples.i_' || CAST(i AS TEXT) || '(i composite_type_examples.i_' || CAST((i-1) AS TEXT) || ');';
    END LOOP;
  END;
$$ LANGUAGE plpgsql;
