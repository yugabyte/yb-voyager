-- TODO: create user as per User creation steps in docs and use that in tests

CREATE TABLE foo (
        id INT PRIMARY KEY,
        name VARCHAR
);

CREATE TABLE bar (
        id INT PRIMARY KEY,
        name VARCHAR
);

CREATE TABLE public.unique_table (
        id SERIAL PRIMARY KEY,
        email VARCHAR(100),
        phone VARCHAR(100),
        address VARCHAR(255),
        UNIQUE (email, phone)  -- Unique constraint on combination of columns
);

CREATE UNIQUE INDEX unique_address_idx ON public.unique_table (address);  -- Unique Index

CREATE TABLE public.table1 (
        id SERIAL PRIMARY KEY,
        name VARCHAR(100)
);

CREATE TABLE public.table2 (
        id SERIAL PRIMARY KEY,
        email VARCHAR(100)
);