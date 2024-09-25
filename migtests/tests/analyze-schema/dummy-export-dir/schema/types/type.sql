--alter type case
ALTER TYPE colors ADD VALUE 'orange' AFTER 'red';

ALTER TYPE compfoo ADD ATTRIBUTE f3 int;


CREATE TABLE test_table_in_type_file(
    id int PRIMARY KEY
);

CREATE TYPE address_type AS (     
    street VARCHAR(100),
    city VARCHAR(50),
    state VARCHAR(50),
    zip_code VARCHAR(10)
);

CREATE TYPE non_public.address_type1 AS (     
    street VARCHAR(100),
    city VARCHAR(50),
    state VARCHAR(50),
    zip_code VARCHAR(10)
);

CREATE TYPE non_public.enum_test AS (     
    street VARCHAR(100),
    city VARCHAR(50),
    state VARCHAR(50),
    zip_code VARCHAR(10)
);

CREATE TYPE enum_test As ENUM (
    'test_a1',
    'test_a2'
);