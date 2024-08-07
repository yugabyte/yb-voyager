-- https://www.postgresql.org/docs/current/sql-createtype.html#id-1.9.3.94.8
CREATE USER enum_owner;
CREATE SCHEMA enum_example;

CREATE TYPE enum_example.bug_status AS ENUM ('new', 'open', 'closed');
COMMENT ON TYPE enum_example.bug_status IS 'ENUM type';

CREATE TYPE enum_example.bug_severity AS ENUM('low', 'med', 'high');

CREATE TYPE enum_example.bug_info AS (
    status enum_example.bug_status
  , severity enum_example.bug_severity
);

CREATE FUNCTION enum_example.make_bug_info(
      status enum_example.bug_status
    , severity enum_example.bug_severity
  ) RETURNS enum_example.bug_info IMMUTABLE AS $$
    SELECT status, severity
  $$ LANGUAGE SQL;

CREATE TABLE IF NOT EXISTS enum_example.bugs (
    id serial
    , description text
    , status enum_example.bug_status
    , _status enum_example.bug_status GENERATED ALWAYS AS (status) STORED
    , severity enum_example.bug_severity
    , _severity enum_example.bug_severity GENERATED ALWAYS AS (severity) STORED
    , info enum_example.bug_info
      GENERATED ALWAYS AS (enum_example.make_bug_info(status, severity)) STORED
);

CREATE VIEW enum_example._bugs AS SELECT id, status FROM enum_example.bugs;

CREATE TABLE enum_example._bug_severity AS SELECT id, severity FROM enum_example.bugs;

CREATE TABLE enum_example.bugs_clone () INHERITS (enum_example.bugs);

CREATE FUNCTION enum_example.should_raise_alarm(info enum_example.bug_info)
  RETURNS BOOLEAN AS $$
    SELECT info.status = 'new' AND info.severity = 'high'
  $$ LANGUAGE SQL IMMUTABLE;

