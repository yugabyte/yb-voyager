CREATE TABLE foo(id int PRIMARY KEY, name varchar);

CREATE TABLE bar(id int PRIMARY KEY, name varchar);

CREATE TABLE public.unique_table (
        id serial PRIMARY KEY,
        email VARCHAR(100),
        phone VARCHAR(100),
        address VARCHAR(255),
        UNIQUE (email, phone)  -- Unique constraint on combination of columns
);

CREATE UNIQUE INDEX unique_address_idx ON public.unique_table (address); -- Unique Index
CREATE TABLE public.table1 (id serial PRIMARY KEY, name VARCHAR(100));
CREATE TABLE public.table2 (id serial PRIMARY KEY, email VARCHAR(100));