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


