-- https://www.postgresql.org/docs/current/sql-createconversion.html
-- postgres/src/test/modules/test_ddl_deparse/sql/create_conversion.sql
CREATE USER conversion_owner;
CREATE SCHEMA conversion_example;
CREATE CONVERSION conversion_example.myconv FOR 'LATIN1' TO 'UTF8' FROM iso8859_1_to_utf8;
