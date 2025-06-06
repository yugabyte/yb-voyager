-- TODO: create user as per User creation steps in docs and use that in tests

-- Used ORACLE_DATABASE=DMS i.e. pluggable database to create APP_USER
ALTER SESSION SET CONTAINER = "DMS";


-- creating tables under YBVOYAGER schema, same as APP_USER
CREATE TABLE YBVOYAGER.foo (
    id NUMBER PRIMARY KEY,
    name VARCHAR2(255)
);

CREATE TABLE YBVOYAGER.bar (
    id NUMBER PRIMARY KEY,
    name VARCHAR2(255)
);

CREATE TABLE YBVOYAGER.unique_table (
    id NUMBER PRIMARY KEY,
    email VARCHAR2(100),
    phone VARCHAR2(100),
    address VARCHAR2(255),
    CONSTRAINT email_phone_unq UNIQUE (email, phone)
);

CREATE TABLE YBVOYAGER.table1 (
    id NUMBER PRIMARY KEY,
    name VARCHAR2(100)
);

CREATE TABLE YBVOYAGER.table2 (
    id NUMBER PRIMARY KEY,
    email VARCHAR2(100)
);


CREATE TABLE YBVOYAGER.non_pk1 (
    id NUMBER,
    name VARCHAR2(10)
);

CREATE TABLE YBVOYAGER.non_pk2 (
    id NUMBER,
    name VARCHAR2(10)
);


-- Creating different index types on YBVOYAGER.unique_table for testing
-- Normal B-tree index on address
CREATE UNIQUE INDEX YBVOYAGER.unique_address_idx ON YBVOYAGER.unique_table (address);

-- Reverse key index on phone
CREATE INDEX YBVOYAGER.idx_phone_reverse ON YBVOYAGER.unique_table(phone) REVERSE;

-- Domain index on address column (requires Oracle Text to be installed)
CREATE INDEX YBVOYAGER.idx_address_text ON YBVOYAGER.unique_table(address)
  INDEXTYPE IS CTXSYS.CONTEXT;