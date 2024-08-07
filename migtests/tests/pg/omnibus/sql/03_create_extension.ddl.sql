-- https://www.postgresql.org/docs/current/sql-createextension.html
CREATE USER extension_user;
CREATE SCHEMA extension_example;

CREATE EXTENSION hstore WITH SCHEMA extension_example CASCADE;
COMMENT ON EXTENSION hstore IS 'extensions are effectively global, despite the WITH SCHEMA clause';

CREATE TABLE extension_example.testhstore (h extension_example.hstore);

CREATE INDEX hidx ON extension_example.testhstore
  USING GIST (h extension_example.gist_hstore_ops(siglen=32));

CREATE VIEW extension_example.dependent_view AS
  SELECT * FROM extension_example.each('aaa=>bq, b=>NULL, ""=>1');

CREATE FUNCTION extension_example._hstore(r RECORD)
  RETURNS extension_example.hstore
  LANGUAGE plpgsql IMMUTABLE STRICT
  AS $$
    BEGIN
      return extension_example.hstore(r);
    END;
  $$;

