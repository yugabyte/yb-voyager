
CREATE USER foreign_db_owner with password 'password';
CREATE DATABASE other_db with owner foreign_db_owner;

CREATE SCHEMA foreign_db_example;
CREATE EXTENSION postgres_fdw WITH SCHEMA foreign_db_example;


-- https://www.postgresql.org/docs/current/sql-createserver.html
CREATE SERVER technically_this_server
  FOREIGN DATA WRAPPER postgres_fdw OPTIONS (
    host 'localhost',
    dbname 'other_db',
    port '5432'
  );

-- https://www.postgresql.org/docs/current/sql-createusermapping.html
CREATE USER MAPPING FOR foreign_db_owner
  SERVER technically_this_server
  OPTIONS (user 'foreign_db_owner', password 'password');

CREATE TYPE foreign_db_example.example_type AS (a INT, b TEXT);

CREATE FUNCTION foreign_db_example.is_positive(i INTEGER) RETURNS BOOLEAN IMMUTABLE AS $$
  SELECT i > 0
$$ LANGUAGE SQL;

-- https://www.postgresql.org/docs/current/sql-createdomain.html
CREATE DOMAIN foreign_db_example.positive_number AS INTEGER NOT NULL CHECK(foreign_db_example.is_positive(VALUE));
-- https://www.postgresql.org/docs/current/sql-createforeigntable.html
CREATE FOREIGN TABLE foreign_db_example.technically_doesnt_exist(
    id INTEGER
    , CONSTRAINT imaginary_table_id_gt_1 CHECK  (id > 1)
    , uses_type foreign_db_example.example_type
    , _uses_type foreign_db_example.example_type GENERATED ALWAYS AS (uses_type) STORED
    , positive_number foreign_db_example.positive_number
    , _positive_number foreign_db_example.positive_number GENERATED ALWAYS AS (positive_number) STORED
  )
  SERVER technically_this_server;

CREATE VIEW foreign_db_example AS
  SELECT * FROM foreign_db_example.technically_doesnt_exist;
-- TODO: test fk constraints
