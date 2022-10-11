-- setting variables for current session
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


CREATE TABLE public.sales_region (
    id integer,
    amount integer,
    branch text,
    region text
)
PARTITION BY LIST (region);




CREATE TABLE public.boston (
    id integer,
    amount integer,
    branch text,
    region text
);



CREATE TABLE public.customers (
    id integer,
    statuses text,
    arr numeric
)
PARTITION BY LIST (statuses);



CREATE TABLE public.cust_active (
    id integer,
    statuses text,
    arr numeric
)
PARTITION BY RANGE (arr);



CREATE TABLE public.cust_arr_large (
    id integer,
    statuses text,
    arr numeric
)
PARTITION BY HASH (id);



CREATE TABLE public.cust_arr_small (
    id integer,
    statuses text,
    arr numeric
)
PARTITION BY HASH (id);



CREATE TABLE public.cust_other (
    id integer,
    statuses text,
    arr numeric
);



CREATE TABLE public.cust_part11 (
    id integer,
    statuses text,
    arr numeric
);



CREATE TABLE public.cust_part12 (
    id integer,
    statuses text,
    arr numeric
);



CREATE TABLE public.cust_part21 (
    id integer,
    statuses text,
    arr numeric
);



CREATE TABLE public.cust_part22 (
    id integer,
    statuses text,
    arr numeric
);



CREATE TABLE public.emp (
    emp_id integer,
    emp_name text,
    dep_code integer
)
PARTITION BY HASH (emp_id);



CREATE TABLE public.emp_0 (
    emp_id integer,
    emp_name text,
    dep_code integer
);



CREATE TABLE public.emp_1 (
    emp_id integer,
    emp_name text,
    dep_code integer
);



CREATE TABLE public.emp_2 (
    emp_id integer,
    emp_name text,
    dep_code integer
);



CREATE TABLE public.london (
    id integer,
    amount integer,
    branch text,
    region text
);



CREATE TABLE public.sales (
    id integer,
    p_name text,
    amount integer,
    sale_date timestamp without time zone
)
PARTITION BY RANGE (sale_date);



CREATE TABLE public.sales_2019_q4 (
    id integer,
    p_name text,
    amount integer,
    sale_date timestamp without time zone
);



CREATE TABLE public.sales_2020_q1 (
    id integer,
    p_name text,
    amount integer,
    sale_date timestamp without time zone
);



CREATE TABLE public.sales_2020_q2 (
    id integer,
    p_name text,
    amount integer,
    sale_date timestamp without time zone
);



CREATE TABLE public.sydney (
    id integer,
    amount integer,
    branch text,
    region text
);



ALTER TABLE ONLY public.sales_region ATTACH PARTITION public.boston FOR VALUES IN ('Boston');



ALTER TABLE ONLY public.customers ATTACH PARTITION public.cust_active FOR VALUES IN ('ACTIVE', 'RECURRING', 'REACTIVATED');



ALTER TABLE ONLY public.cust_active ATTACH PARTITION public.cust_arr_large FOR VALUES FROM ('101') TO (MAXVALUE);



ALTER TABLE ONLY public.cust_active ATTACH PARTITION public.cust_arr_small FOR VALUES FROM (MINVALUE) TO ('101');



ALTER TABLE ONLY public.customers ATTACH PARTITION public.cust_other DEFAULT;



ALTER TABLE ONLY public.cust_arr_small ATTACH PARTITION public.cust_part11 FOR VALUES WITH (modulus 2, remainder 0);



ALTER TABLE ONLY public.cust_arr_small ATTACH PARTITION public.cust_part12 FOR VALUES WITH (modulus 2, remainder 1);



ALTER TABLE ONLY public.cust_arr_large ATTACH PARTITION public.cust_part21 FOR VALUES WITH (modulus 2, remainder 0);



ALTER TABLE ONLY public.cust_arr_large ATTACH PARTITION public.cust_part22 FOR VALUES WITH (modulus 2, remainder 1);



ALTER TABLE ONLY public.emp ATTACH PARTITION public.emp_0 FOR VALUES WITH (modulus 3, remainder 0);



ALTER TABLE ONLY public.emp ATTACH PARTITION public.emp_1 FOR VALUES WITH (modulus 3, remainder 1);



ALTER TABLE ONLY public.emp ATTACH PARTITION public.emp_2 FOR VALUES WITH (modulus 3, remainder 2);



ALTER TABLE ONLY public.sales_region ATTACH PARTITION public.london FOR VALUES IN ('London');



ALTER TABLE ONLY public.sales ATTACH PARTITION public.sales_2019_q4 FOR VALUES FROM ('2019-10-01 00:00:00') TO ('2020-01-01 00:00:00');



ALTER TABLE ONLY public.sales ATTACH PARTITION public.sales_2020_q1 FOR VALUES FROM ('2020-01-01 00:00:00') TO ('2020-04-01 00:00:00');



ALTER TABLE ONLY public.sales ATTACH PARTITION public.sales_2020_q2 FOR VALUES FROM ('2020-04-01 00:00:00') TO ('2020-07-01 00:00:00');



ALTER TABLE ONLY public.sales_region ATTACH PARTITION public.sydney FOR VALUES IN ('Sydney');


