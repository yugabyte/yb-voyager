-- TODO: create user as per User creation steps in docs and use that in tests

CREATE TABLE public.foo (
        id INT PRIMARY KEY,
        name VARCHAR
);
INSERT into public.foo values (1, 'abc'), (2, 'xyz');

CREATE TABLE public.bar (
        id INT PRIMARY KEY,
        name VARCHAR
);
INSERT into public.bar values (1, 'abc'), (2, 'xyz');

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

CREATE TABLE public.non_pk1(
        id INT,
        name VARCHAR(255)
);

CREATE TABLE public.non_pk2(
        id INT,
        name VARCHAR(255)
);