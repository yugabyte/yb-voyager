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


CREATE AGGREGATE public.inc_sum(integer) (
    SFUNC = int4pl,
    STYPE = integer,
    INITCOND = '10'
);


CREATE AGGREGATE schema2.inc_sum(integer) (
    SFUNC = int4pl,
    STYPE = integer,
    INITCOND = '10'
);


