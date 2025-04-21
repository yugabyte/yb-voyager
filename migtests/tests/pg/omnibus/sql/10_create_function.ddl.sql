-- https://github.com/postgres/postgres/blob/master/src/test/regress/sql/polymorphism.sql
CREATE USER fn_owner;

CREATE SCHEMA fn_examples;

CREATE FUNCTION fn_examples.polyf(x anyelement) RETURNS anyelement AS $$
  select x + 1
$$ language sql;

CREATE FUNCTION fn_examples.is_even(i integer) RETURNS BOOLEAN LANGUAGE plpgsql AS $$
BEGIN
  IF i % 2 = 0 THEN
    RETURN true;
  ELSE
    RETURN false;
  end if;
END;
$$;

CREATE FUNCTION fn_examples.is_odd(i integer) RETURNS BOOLEAN LANGUAGE SQL AS $$
  SELECT (fn_examples.is_even(i) IS NOT true)
$$;

CREATE TABLE fn_examples.ordinary_table(id INTEGER);
CREATE FUNCTION fn_examples.depends_on_table_column() RETURNS INTEGER LANGUAGE SQL AS $$
  SELECT id FROM fn_examples.ordinary_table LIMIT 1
$$;

CREATE FUNCTION fn_examples.depends_on_table_column_type()
  RETURNS fn_examples.ordinary_table.id%TYPE
  LANGUAGE SQL AS $$
    SELECT id FROM fn_examples.ordinary_table LIMIT 1
  $$;
-- the %rowtype syntax isn't necessary
CREATE FUNCTION fn_examples.depends_on_table_rowtype() RETURNS fn_examples.ordinary_table LANGUAGE SQL AS $$
  SELECT * FROM fn_examples.ordinary_table LIMIT 1
$$;

CREATE VIEW fn_examples.basic_view AS SELECT * FROM fn_examples.ordinary_table;

CREATE FUNCTION fn_examples.depends_on_view_column() RETURNS INTEGER LANGUAGE SQL AS $$
  SELECT id FROM fn_examples.basic_view LIMIT 1
$$;

CREATE FUNCTION fn_examples.depends_on_view_column_type() RETURNS fn_examples.basic_view.id%TYPE LANGUAGE SQL AS $$
  SELECT id FROM fn_examples.basic_view LIMIT 1
$$;

CREATE PROCEDURE fn_examples.insert_to_table() LANGUAGE plpgsql AS $$
  BEGIN
    IF fn_examples.is_even(2) THEN
      INSERT INTO fn_examples.ordinary_table(id) VALUES (1);
    END IF;
  END;
$$;
