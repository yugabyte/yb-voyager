-- setting variables for current session
SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;


CREATE TYPE public.address_type AS (
	street character varying(100),
	city character varying(50),
	state character varying(50),
	zip_code character varying(10)
);


CREATE TYPE public.enum_kind AS ENUM (
    'YES',
    'NO',
    'UNKNOWN'
);


CREATE TYPE public.item_details AS (
	item_id integer,
	item_name character varying,
	item_price numeric(5,2)
);


CREATE TYPE schema2.enum_kind AS ENUM (
    'YES',
    'NO',
    'UNKNOWN'
);


CREATE TYPE schema2.item_details AS (
	item_id integer,
	item_name character varying,
	item_price numeric(5,2)
);


