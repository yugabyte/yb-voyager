create schema non_public;
set search_path to non_public;
create table x(id int primary key,id2 int);

CREATE TABLE user_table (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE,
    status VARCHAR(50) DEFAULT 'active'
);

CREATE TYPE decline_reason AS ENUM (
    'duplicate_payment_method',
    'server_failure'
);

CREATE TABLE test_enum (
    id int PRIMARY KEY,
    reason decline_reason
);
