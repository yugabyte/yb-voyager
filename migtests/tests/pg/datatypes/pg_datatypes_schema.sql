-- datatypes 

drop table if exists num_types;

create table num_types(id serial primary key, v1 smallint, v2 integer,v3 bigint,v4 decimal(6,3),v5 numeric, v6 money);

\d num_types

drop table if exists decimal_types;

create table decimal_types(id int PRIMARY KEY, n1 numeric(108,9), n2 numeric(19,2));

drop type if exists week cascade;

CREATE TYPE week AS ENUM ('Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun');

drop table if exists datatypes1;

create table datatypes1(id serial primary key, bool_type boolean,char_type1 CHAR (1),varchar_type VARCHAR(100),byte_type bytea, enum_type week);

\d datatypes1

drop table if exists datetime_type;

create table datetime_type(id serial primary key, v1 date, v2 time, v3 timestamp,v4 TIMESTAMP without TIME ZONE default CURRENT_TIMESTAMP(0));

\d datetime_type

drop table if exists datetime_type2;

create table datetime_type2(id serial primary key, v1 timestamp);

\d datetime_type2

drop table if exists datatypes2;

create table datatypes2(id serial primary key, v1 json, v2 BIT(10), v3 int ARRAY[4], v4 text[][], v5 BIT VARYING);

\d datatypes2

drop table if exists null_and_default;
create table null_and_default(id int PRIMARY KEY, b boolean default false, i int default 10, val varchar default 'testdefault');

create EXTENSION hstore;

CREATE TABLE hstore_example (
    id SERIAL PRIMARY KEY,
    data hstore
);

drop table if exists tsvector_table cascade;

CREATE TABLE tsvector_table (
    id SERIAL PRIMARY KEY,
    title TEXT,
    content TEXT,
    title_tsv TSVECTOR,
    content_tsv TSVECTOR
);

drop table if exists enum_array_table cascade;

CREATE TABLE enum_array_table (
    id SERIAL PRIMARY KEY,
    day_name week,
    week_days week[],
    description TEXT
);

drop type if exists full_address cascade;

CREATE TYPE full_address AS ( city TEXT, street TEXT, zip_code INT ); 

drop table if exists composite_types cascade;

CREATE TABLE composite_types ( id SERIAL primary key, address full_address ); 

drop table if exists composite_array_types cascade;

CREATE TABLE composite_array_types ( id SERIAL primary key, addresses full_address[] );

-- Domain Datatypes
CREATE DOMAIN social_security_number AS TEXT
    CHECK (VALUE ~ '^\d{3}-\d{2}-\d{4}$');

CREATE DOMAIN positive_int AS INT
    CHECK (VALUE > 0);

CREATE DOMAIN email_address AS TEXT
    CHECK (VALUE ~* '^[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}$');

CREATE DOMAIN rating_1_to_5 AS INT
    CHECK (VALUE BETWEEN 1 AND 5);

CREATE DOMAIN phone_number AS TEXT
    CHECK (VALUE ~ '^\+?[0-9]{10,15}$');

CREATE DOMAIN full_name AS TEXT
    CHECK (VALUE ~ '^[A-Za-z]+\s[A-Za-z]+$');

CREATE DOMAIN app_settings AS JSONB
    CHECK (VALUE ? 'version');

CREATE TABLE domain_types (
    id SERIAL primary key,
    ssn social_security_number,
    email email_address,
    rating rating_1_to_5,
    prefs app_settings
);

CREATE TABLE domain_array_types (
    id SERIAL primary key,
    ssn_list social_security_number[],
    phone_list phone_number[],
    name_list full_name[]
);

-- Range Datatypes
CREATE TYPE price_range AS RANGE (
    subtype = numeric
);

CREATE TYPE discount_range AS RANGE (
    subtype = float8
);

CREATE TYPE period_range AS RANGE (
    subtype = date
);

CREATE TYPE active_ts_range AS RANGE (
    subtype = timestamptz
);

CREATE TABLE range_types (
    id SERIAL PRIMARY KEY,
    price_range_col price_range,
    discount_range_col discount_range,
    period_range_col period_range,
    active_ts_range_col active_ts_range
);


CREATE TABLE range_array_types (
    id SERIAL PRIMARY KEY,
    price_ranges price_range[],
    discount_ranges discount_range[],
    period_ranges period_range[],
    ts_ranges active_ts_range[]
);

-- Extension Datatypes
CREATE EXTENSION IF NOT EXISTS HSTORE;
CREATE EXTENSION IF NOT EXISTS CITEXT;
CREATE EXTENSION IF NOT EXISTS LTREE;

CREATE TABLE extension_types (
    id SERIAL primary key,
    col_hstore  HSTORE,
    col_citext  CITEXT,
    col_ltree   LTREE
);

-- Extension Arrays Datatypes
CREATE TABLE extension_arrays (
  id SERIAL primary key,
  col_hstore HSTORE[],
  col_citext CITEXT[],
  col_ltree  LTREE[]
);

-- Nested Datatypes
CREATE TYPE employment_status AS ENUM ('active', 'inactive', 'terminated');
CREATE TYPE region AS ENUM ('north', 'south', 'east', 'west');
CREATE DOMAIN email_address AS TEXT CHECK (POSITION('@' IN VALUE) > 1);

CREATE TYPE employee_info AS (
  emp_name TEXT,
  emp_status employment_status,
  emp_email email_address
);


CREATE TYPE client_info AS (
  client_name TEXT,
  region region,
  contact_email email_address
);

CREATE TABLE audit_log (
  id SERIAL PRIMARY KEY,
  involved_employees employee_info[],
  affected_clients client_info[],
  transaction_refs INT[],
  created_at TIMESTAMP DEFAULT NOW()
);

-- Numeric Types
CREATE TABLE numeric_types (
    id            serial PRIMARY KEY,
    real_col     real,
    double_col  double precision,
    small_serial_col  smallserial,
    big_serial_col   bigserial
);

\d numeric_types

-- Numeric Arrays
CREATE TABLE numeric_arrays (
    id            serial PRIMARY KEY,
    real_col     real[],
    double_col  double precision[]
);

\d numeric_arrays

-- Datetime Types
CREATE TABLE datetime_types (
    id          serial PRIMARY KEY,
    timestamptz_col    timestamptz,
    timetz_col timetz,
    interval_col    interval
);

\d datetime_types

-- Datetime Arrays
CREATE TABLE datetime_arrays (
    id          serial PRIMARY KEY,
    timestamptz_col    timestamptz[],
    timetz_col timetz[],
    interval_col    interval[]
);

\d datetime_arrays

-- Geometry Types
CREATE TABLE geometry_types (
    id       serial PRIMARY KEY,
    point_col        point,
    line_col       line,
    lseg_col      lseg,
    box_col       box,
    path_col path,
    polygon_col     polygon,
    circle_col     circle
);

\d geometry_types

-- Geometry Arrays
CREATE TABLE geometry_arrays (
    id          serial PRIMARY KEY,
    point_col        point[],
    line_col       line[],
    lseg_col      lseg[],
    box_col       box[],
    path_col path[],
    polygon_col     polygon[],
    circle_col     circle[]
);

\d geometry_arrays

-- Network Types
CREATE TABLE network_types (
    id       serial PRIMARY KEY,
    cidr_col cidr,
    inet_col  inet,
    macaddr_col    macaddr,
    macaddr8_col    macaddr8
);

\d network_types

-- Network Arrays
CREATE TABLE network_arrays (
    id          serial PRIMARY KEY,
    cidr_col cidr[],
    inet_col inet[],
    macaddr_col macaddr[],
    macaddr8_col macaddr8[]
);

\d network_arrays

-- Misc Types
CREATE TABLE misc_types (
    id            serial PRIMARY KEY,
    pg_lsn_col   pg_lsn,
    txid_snapshot_col txid_snapshot
);

\d misc_types

-- Misc Arrays
CREATE TABLE misc_arrays (
    id            serial PRIMARY KEY,
    pg_lsn_col pg_lsn[],
    txid_snapshot_col txid_snapshot[]
);

\d misc_arrays