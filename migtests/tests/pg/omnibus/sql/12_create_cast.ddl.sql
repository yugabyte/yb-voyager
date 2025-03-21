-- https://www.postgresql.org/docs/current/sql-createcast.html
CREATE USER castor;
CREATE SCHEMA create_cast;

CREATE TYPE create_cast.abc AS ENUM ('a', 'b', 'c');

CREATE FUNCTION create_cast.transmogrify(input create_cast.abc) RETURNS INTEGER LANGUAGE SQL IMMUTABLE AS $$
  SELECT 1;
$$;

CREATE CAST(create_cast.abc AS INTEGER) WITH FUNCTION create_cast.transmogrify AS ASSIGNMENT;
