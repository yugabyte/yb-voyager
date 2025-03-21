-- schema and tests for sequences in public and non-public schemas
-- cases including multiple identity and serial columns
/*

Tables - sequence_check1,sequence_check2,sequence_check3,multiple_identity_columns,multiple_serial_columns,Case_Sensitive_Seq,
         schema1.sequence_check1,schema1.sequence_check2,schema1.sequence_check3,schema1.multiple_identity_columns,schema1.multiple_serial_columns 
         foo,bar,schema2.foo,schema2.bar,schema3.foo,schema4.bar,foo_bar,sales_region(London,boston,sydney),users

Sequences - sequence_check1_id_seq -> sequence_check1, sequence_check2_id_seq -> sequence_check2, sequence_check3_id_seq -> sequence_check3,(multiple_identity_columns_id_seq,multiple_identity_columns_id2_seq)  -> multiple_identity_columns,
(multiple_serial_columns_id_seq,multiple_serial_columns_id2_seq) -> multiple_serial_columns, Case_Sensitive_Seq_id_seq -> Case_Sensitive_Seq, schema1.sequence_check1_id_seq -> schema1.sequence_check1,
schema1.sequence_check2_id_seq -> schema1.sequence_check2, schema1.sequence_check3_id_seq -> schema1.sequence_check3, (schema1.multiple_identity_columns_id_seq,schema2.multiple_identity_columns_id2_seq) -> schema1.multiple_identity_columns,
(schema1.multiple_serial_columns_id_seq,schema1.multiple_serial_columns_id2_seq) -> schema1.multiple_serial_columns, schema2.baz -> (schema2.foo, schema2.bar),
 sales_region_id_seq -> sales_region, user_code_seq -> users, baz -> (foo, bar), schema3.baz -> (schema3.foo, schema3.bar), foo_bar_baz -> foo_bar

Sequences not actually owned by tables-
public.seq1, schema2.seq1, schema3.seq1, schema4.seq1


export EXPORT_TABLE_LIST="sequence_check1,sequence_check3,multiple_serial_columns,Case_Sensitive_Seq,schema1.sequence_check1,schema1.sequence_check2,schema1.multiple_identity_columns,foo,bar,schema3.foo,schema4.bar,London,sydney,users"
sequence_check2_id_seq
multiple_identity_columns_id_seq
multiple_identity_columns_id2_seq
schema1.sequence_check3_id_seq
schema1.multiple_serial_columns_id_seq,schema1.multiple_serial_columns_id2_seq
foo_bar_baz

*/
drop table if exists sequence_check1;

create table sequence_check1 (
    id int generated by default as identity,
    name varchar(100) not null,
    balance dec(15, 2) not null,
    primary key(id)
);

drop table if exists sequence_check2;

create table sequence_check2 (
    id serial primary key,
    first_name VARCHAR(50),
    last_name VARCHAR(50),
    email VARCHAR(50),
    gender VARCHAR(50),
    ip_address VARCHAR(20)
);

CREATE SEQUENCE sequence_check3_id_seq;

drop table if exists sequence_check3;

CREATE TABLE sequence_check3 (
    id integer NOT NULL DEFAULT nextval('sequence_check3_id_seq') PRIMARY KEY,
    name varchar(20)
);

ALTER SEQUENCE sequence_check3_id_seq OWNED BY sequence_check3.id;

drop table if exists multiple_identity_columns;

create table multiple_identity_columns (
    id int generated by default as identity,
    name varchar(100) not null,
    balance dec(15, 2) not null,
    id2 int generated by default as identity,
    name2 varchar(100) not null,
    balance2 dec(15, 2) not null,
    primary key(id, id2)
);

drop table if exists multiple_serial_columns;

create table multiple_serial_columns (
    id serial,
    name varchar(100) not null,
    balance dec(15, 2) not null,
    id2 serial,
    name2 varchar(100) not null,
    balance2 dec(15, 2) not null,
    primary key(id, id2)
);


create table "Case_Sensitive_Seq" (
    id serial,
    name varchar(100) not null,
    balance dec(15, 2) not null,
    name2 varchar(100) not null,
    balance2 dec(15, 2) not null,
    primary key(id)
);


-- same as above but with schema
create schema schema1;

drop table if exists schema1.sequence_check1;
create table schema1.sequence_check1 (
    id int generated by default as identity,
    name varchar(100) not null,
    balance dec(15, 2) not null,
    primary key(id)
);

drop table if exists schema1.sequence_check2;
create table schema1.sequence_check2 (
	id serial primary key,
	first_name VARCHAR(50),
	last_name VARCHAR(50),
	email VARCHAR(50),
	gender VARCHAR(50),
	ip_address VARCHAR(20)
);

CREATE SEQUENCE schema1.sequence_check3_id_seq;

drop table if exists schema1.sequence_check3;
CREATE TABLE schema1.sequence_check3 (
    id integer NOT NULL DEFAULT nextval('schema1.sequence_check3_id_seq') PRIMARY KEY,
    name varchar(20)
);

ALTER SEQUENCE schema1.sequence_check3_id_seq OWNED BY schema1.sequence_check3.id;

drop table if exists schema1.multiple_identity_columns;
create table schema1.multiple_identity_columns (
    id int generated by default as identity,
    name varchar(100) not null,
    balance dec(15, 2) not null,
    id2 int generated by default as identity,
    name2 varchar(100) not null,
    balance2 dec(15, 2) not null,
    primary key(id, id2)
);


drop table if exists schema1.multiple_serial_columns;
create table schema1.multiple_serial_columns (
    id serial,
    name varchar(100) not null,
    balance dec(15, 2) not null,
    id2 serial,
    name2 varchar(100) not null,
    balance2 dec(15, 2) not null,
    primary key(id, id2)
);

-- Single Sequence attached to two columns of different table
create sequence baz;
create table foo(id bigint default nextval('baz'), value text);
create table bar(id bigint default nextval('baz'), value date);


-- Single Sequence attached to two columns of different table in non-public schema
create schema schema2;
create sequence schema2.baz;
create table schema2.foo(id bigint default nextval('schema2.baz'), value text);
create table schema2.bar(id bigint default nextval('schema2.baz'), value date);
ALTER SEQUENCE schema2.baz OWNED BY schema2.foo;
ALTER SEQUENCE schema2.baz OWNED BY schema2.foo;

-- Single Sequence attached to two columns of different table in different schemas
create schema schema3;
create schema schema4;
create sequence schema3.baz;
create table schema3.foo(id bigint default nextval('schema3.baz'), value text);
create table schema4.bar(id bigint default nextval('schema3.baz'), value date);


-- Single Sequence attached to two columns of a table
create sequence foo_bar_baz;
create table foo_bar(id bigint default nextval('foo_bar_baz'), value text, id2 bigint default nextval('foo_bar_baz'), value2 text);



CREATE TABLE sales_region (id serial, amount int, branch text, region text, PRIMARY KEY (id,region)) PARTITION BY LIST (region);
CREATE TABLE London PARTITION OF sales_region FOR VALUES IN ('London');
CREATE TABLE Sydney PARTITION OF sales_region FOR VALUES IN ('Sydney');
CREATE TABLE Boston PARTITION OF sales_region FOR VALUES IN ('Boston');



CREATE SEQUENCE user_code_seq
START WITH 1
INCREMENT BY 1;

CREATE OR REPLACE FUNCTION generate_user_code() RETURNS TEXT AS $$
DECLARE
    new_code TEXT;
BEGIN
    SELECT 'USR' || LPAD(nextval('user_code_seq')::TEXT, 4, '0') 
    INTO new_code;
    RETURN new_code;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE users (
    user_id SERIAL PRIMARY KEY,
    user_code TEXT UNIQUE,
    name TEXT
);

ALTER TABLE ONLY users ALTER COLUMN user_code SET DEFAULT generate_user_code();

ALTER SEQUENCE public.user_code_seq OWNED BY public.users.user_code;

--SEQUENCES not attached to any table columns
CREATE SEQUENCE public.seq1;
CREATE SEQUENCE schema2.seq1;
CREATE SEQUENCE schema3.seq1;
CREATE SEQUENCE schema4.seq1;

select pg_catalog.nextval('public.seq1');
select pg_catalog.nextval('public.seq1');
select pg_catalog.nextval('schema2.seq1');
select pg_catalog.nextval('schema2.seq1');
select pg_catalog.nextval('schema2.seq1');
select pg_catalog.nextval('schema3.seq1');
select pg_catalog.nextval('schema3.seq1');
select pg_catalog.nextval('schema3.seq1');
select pg_catalog.nextval('schema3.seq1');
select pg_catalog.nextval('schema3.seq1');
select pg_catalog.nextval('schema4.seq1');
select pg_catalog.nextval('schema4.seq1');
