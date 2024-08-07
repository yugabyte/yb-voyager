-- setting variables for current session
SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;


CREATE TABLE public.c (
    i integer NOT NULL,
    t integer,
    x text
);


CREATE TABLE public.t (
    i integer NOT NULL,
    ts timestamp(0) with time zone DEFAULT now(),
    j json,
    t text,
    e public.myenum,
    c public.mycomposit
);


ALTER TABLE ONLY public.c ALTER COLUMN i SET DEFAULT nextval('public.c_i_seq'::regclass);


ALTER FOREIGN TABLE ONLY public.f_c ALTER COLUMN i SET DEFAULT nextval('public.f_c_i_seq'::regclass);


ALTER FOREIGN TABLE ONLY public.f_t ALTER COLUMN i SET DEFAULT nextval('public.f_t_i_seq'::regclass);


ALTER TABLE ONLY public.t ALTER COLUMN i SET DEFAULT nextval('public.t_i_seq'::regclass);


ALTER TABLE ONLY public.t
    ADD CONSTRAINT t_pkey PRIMARY KEY (i);


ALTER TABLE ONLY public.c
    ADD CONSTRAINT c_t_fkey FOREIGN KEY (t) REFERENCES public.t(i);


create server p10 foreign data wrapper postgres_fdw options (host 'localhost', dbname 'p10');


CREATE FOREIGN TABLE public.f_c (
    i integer NOT NULL,
    t integer,
    x text
)
SERVER p10
OPTIONS (
    table_name 'c'
);


CREATE FOREIGN TABLE public.f_t (
    i integer NOT NULL,
    ts timestamp(0) with time zone DEFAULT now(),
    j json,
    t text,
    e public.myenum,
    c public.mycomposit
)
SERVER p10
OPTIONS (
    table_name 't'
);


