create role ordinary_table_owner;
create schema ordinary_tables;
-- https://www.postgresql.org/docs/current/sql-createtable.html

-- the tables below demonstrate ordinary tables with all the built-in datatypes
-- plus pk and not null constraints. However, there are no inter-table or more
-- complicated dependencies in the ordinary_tables schema.

CREATE TABLE ordinary_tables.numeric_type_examples(
    id SERIAL primary key,
    -- https://www.postgresql.org/docs/current/datatype-numeric.html
    an_integer INTEGER NOT NULL,
    an_int INT, -- should be the same as integer
    an_int4 INT4,
    an_int8 INT8,
    a_bigint BIGINT,
    a_smallint SMALLINT,
    a_decimal DECIMAL,
    a_numeric numeric,
    a_real REAL,
    a_double DOUBLE PRECISION,
    a_smallserial SMALLSERIAL,
    a_bigserial BIGSERIAL
);

COMMENT ON TABLE ordinary_tables.numeric_type_examples IS
  'examples of numeric types';
COMMENT ON COLUMN ordinary_tables.numeric_type_examples.id IS
  'serial id';

ALTER TABLE ordinary_tables.numeric_type_examples
ADD COLUMN another_numeric NUMERIC(3);

ALTER TABLE ordinary_tables.numeric_type_examples
ADD COLUMN yet_another_numeric NUMERIC(6, 4);

CREATE TABLE ordinary_tables.money_example(
    money MONEY -- https://www.postgresql.org/docs/current/datatype-money.html
);

CREATE TABLE ordinary_tables.character_examples(
  id TEXT PRIMARY KEY,
  a_varchar VARCHAR,
  a_limited_varchar CHARACTER VARYING(10),
  a_single_char CHARACTER,
  n_char CHAR(11)
);

CREATE TABLE ordinary_tables.binary_examples(
  bytes BYTEA PRIMARY KEY
);

CREATE TABLE ordinary_tables.time(
  --
    ts_with_tz TIMESTAMP WITH TIME ZONE
  , ts_with_tz_precision TIMESTAMP(2) WITH TIME ZONE
  , ts_with_ntz TIMESTAMP WITHOUT TIME ZONE
  , ts_with_ntz_precision TIMESTAMP(3) WITHOUT TIME ZONE
  , t_with_tz TIME WITH TIME ZONE
  , t_with_tz_precision TIME(4) WITH TIME ZONE
  , t_with_ntz TIME WITHOUT TIME ZONE
  , t_with_ntz_precision TIME(5) WITHOUT TIME ZONE
  , date DATE
  , interval_year INTERVAL YEAR
  , interval_month INTERVAL MONTH
  , interval_day INTERVAL DAY
  , interval_hour INTERVAL HOUR
  , interval_minute INTERVAL MINUTE
  , interval_second INTERVAL SECOND
  , interval_year_to_month INTERVAL YEAR TO MONTH
  , interval_day_to_hour INTERVAL DAY TO HOUR
  , interval_day_to_minute INTERVAL DAY TO MINUTE
  , interval_day_to_second INTERVAL DAY TO SECOND
  , interval_hour_to_minute INTERVAL HOUR TO MINUTE
  , interval_hour_to_second INTERVAL HOUR TO SECOND
  , interval_minute_to_second INTERVAL MINUTE TO SECOND
);

CREATE TABLE ordinary_tables.boolean_examples(
  b BOOLEAN
);

CREATE TABLE ordinary_tables.geometric_examples(
    point_example point
  , line_example line
  , lseg_example lseg
  , box_example box
  , path_example path
  , polygon_example polygon
  , circle_example circle
);

CREATE TABLE ordinary_tables.network_addr_examples(
    cidr_example cidr
  , inet_example inet
  , macaddr_example macaddr
  , macaddr8_example macaddr8
);

CREATE TABLE ordinary_tables.bit_string_examples(
    bit_example bit(10)
  , bit_varyint_example bit varying(20)
);
