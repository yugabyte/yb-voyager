-- setting variables for current session
SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

CREATE SERVER technically_this_server
  FOREIGN DATA WRAPPER postgres_fdw OPTIONS (
    host 'localhost',
    dbname 'other_db',
    port '5432'
  );

CREATE FOREIGN TABLE foreign_db_example.technically_doesnt_exist (
    id integer,
    uses_type foreign_db_example.example_type,
    _uses_type foreign_db_example.example_type,
    positive_number foreign_db_example.positive_number,
    _positive_number foreign_db_example.positive_number,
    CONSTRAINT imaginary_table_id_gt_1 CHECK ((id > 1))
)
SERVER technically_this_server;

CREATE OR REPLACE FUNCTION foreign_db_example.populate_defaults()
RETURNS TRIGGER AS $$
BEGIN
    NEW._uses_type := COALESCE(NEW._uses_type, NEW.uses_type);
    NEW._positive_number := COALESCE(NEW._positive_number, NEW.positive_number);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER populate_defaults_trigger
BEFORE INSERT OR UPDATE ON foreign_db_example.technically_doesnt_exist
FOR EACH ROW
EXECUTE FUNCTION foreign_db_example.populate_defaults();

