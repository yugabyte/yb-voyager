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


CREATE SEQUENCE domain_examples.us_snail_addy_address_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE domain_examples.us_snail_addy_address_id_seq OWNED BY domain_examples.us_snail_addy.address_id;


CREATE SEQUENCE enum_example.bugs_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE enum_example.bugs_id_seq OWNED BY enum_example.bugs.id;


CREATE SEQUENCE ordinary_tables.numeric_type_examples_a_bigserial_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE ordinary_tables.numeric_type_examples_a_bigserial_seq OWNED BY ordinary_tables.numeric_type_examples.a_bigserial;


CREATE SEQUENCE ordinary_tables.numeric_type_examples_a_smallserial_seq
    AS smallint
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE ordinary_tables.numeric_type_examples_a_smallserial_seq OWNED BY ordinary_tables.numeric_type_examples.a_smallserial;


CREATE SEQUENCE ordinary_tables.numeric_type_examples_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE ordinary_tables.numeric_type_examples_id_seq OWNED BY ordinary_tables.numeric_type_examples.id;


