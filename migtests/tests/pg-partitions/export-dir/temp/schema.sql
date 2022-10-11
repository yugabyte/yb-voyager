--
-- PostgreSQL database dump
--

-- Dumped from database version 14.5
-- Dumped by pg_dump version 14.5

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: public; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA public;


--
-- Name: SCHEMA public; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON SCHEMA public IS 'standard public schema';


--
-- Name: sales_region; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sales_region (
    id integer,
    amount integer,
    branch text,
    region text
)
PARTITION BY LIST (region);


SET default_table_access_method = heap;

--
-- Name: boston; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.boston (
    id integer,
    amount integer,
    branch text,
    region text
);


--
-- Name: customers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.customers (
    id integer,
    statuses text,
    arr numeric
)
PARTITION BY LIST (statuses);


--
-- Name: cust_active; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cust_active (
    id integer,
    statuses text,
    arr numeric
)
PARTITION BY RANGE (arr);


--
-- Name: cust_arr_large; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cust_arr_large (
    id integer,
    statuses text,
    arr numeric
)
PARTITION BY HASH (id);


--
-- Name: cust_arr_small; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cust_arr_small (
    id integer,
    statuses text,
    arr numeric
)
PARTITION BY HASH (id);


--
-- Name: cust_other; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cust_other (
    id integer,
    statuses text,
    arr numeric
);


--
-- Name: cust_part11; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cust_part11 (
    id integer,
    statuses text,
    arr numeric
);


--
-- Name: cust_part12; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cust_part12 (
    id integer,
    statuses text,
    arr numeric
);


--
-- Name: cust_part21; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cust_part21 (
    id integer,
    statuses text,
    arr numeric
);


--
-- Name: cust_part22; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cust_part22 (
    id integer,
    statuses text,
    arr numeric
);


--
-- Name: emp; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.emp (
    emp_id integer,
    emp_name text,
    dep_code integer
)
PARTITION BY HASH (emp_id);


--
-- Name: emp_0; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.emp_0 (
    emp_id integer,
    emp_name text,
    dep_code integer
);


--
-- Name: emp_1; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.emp_1 (
    emp_id integer,
    emp_name text,
    dep_code integer
);


--
-- Name: emp_2; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.emp_2 (
    emp_id integer,
    emp_name text,
    dep_code integer
);


--
-- Name: london; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.london (
    id integer,
    amount integer,
    branch text,
    region text
);


--
-- Name: sales; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sales (
    id integer,
    p_name text,
    amount integer,
    sale_date timestamp without time zone
)
PARTITION BY RANGE (sale_date);


--
-- Name: sales_2019_q4; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sales_2019_q4 (
    id integer,
    p_name text,
    amount integer,
    sale_date timestamp without time zone
);


--
-- Name: sales_2020_q1; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sales_2020_q1 (
    id integer,
    p_name text,
    amount integer,
    sale_date timestamp without time zone
);


--
-- Name: sales_2020_q2; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sales_2020_q2 (
    id integer,
    p_name text,
    amount integer,
    sale_date timestamp without time zone
);


--
-- Name: sydney; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sydney (
    id integer,
    amount integer,
    branch text,
    region text
);


--
-- Name: boston; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sales_region ATTACH PARTITION public.boston FOR VALUES IN ('Boston');


--
-- Name: cust_active; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customers ATTACH PARTITION public.cust_active FOR VALUES IN ('ACTIVE', 'RECURRING', 'REACTIVATED');


--
-- Name: cust_arr_large; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cust_active ATTACH PARTITION public.cust_arr_large FOR VALUES FROM ('101') TO (MAXVALUE);


--
-- Name: cust_arr_small; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cust_active ATTACH PARTITION public.cust_arr_small FOR VALUES FROM (MINVALUE) TO ('101');


--
-- Name: cust_other; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customers ATTACH PARTITION public.cust_other DEFAULT;


--
-- Name: cust_part11; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cust_arr_small ATTACH PARTITION public.cust_part11 FOR VALUES WITH (modulus 2, remainder 0);


--
-- Name: cust_part12; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cust_arr_small ATTACH PARTITION public.cust_part12 FOR VALUES WITH (modulus 2, remainder 1);


--
-- Name: cust_part21; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cust_arr_large ATTACH PARTITION public.cust_part21 FOR VALUES WITH (modulus 2, remainder 0);


--
-- Name: cust_part22; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cust_arr_large ATTACH PARTITION public.cust_part22 FOR VALUES WITH (modulus 2, remainder 1);


--
-- Name: emp_0; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.emp ATTACH PARTITION public.emp_0 FOR VALUES WITH (modulus 3, remainder 0);


--
-- Name: emp_1; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.emp ATTACH PARTITION public.emp_1 FOR VALUES WITH (modulus 3, remainder 1);


--
-- Name: emp_2; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.emp ATTACH PARTITION public.emp_2 FOR VALUES WITH (modulus 3, remainder 2);


--
-- Name: london; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sales_region ATTACH PARTITION public.london FOR VALUES IN ('London');


--
-- Name: sales_2019_q4; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sales ATTACH PARTITION public.sales_2019_q4 FOR VALUES FROM ('2019-10-01 00:00:00') TO ('2020-01-01 00:00:00');


--
-- Name: sales_2020_q1; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sales ATTACH PARTITION public.sales_2020_q1 FOR VALUES FROM ('2020-01-01 00:00:00') TO ('2020-04-01 00:00:00');


--
-- Name: sales_2020_q2; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sales ATTACH PARTITION public.sales_2020_q2 FOR VALUES FROM ('2020-04-01 00:00:00') TO ('2020-07-01 00:00:00');


--
-- Name: sydney; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sales_region ATTACH PARTITION public.sydney FOR VALUES IN ('Sydney');


--
-- PostgreSQL database dump complete
--

