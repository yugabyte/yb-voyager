--
-- PostgreSQL database dump
--

-- Dumped from database version 17.2 (Ubuntu 17.2-1.pgdg22.04+1)
-- Dumped by pg_dump version 17.2 (Ubuntu 17.2-1.pgdg22.04+1)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
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
-- Name: schema2; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA schema2;


--
-- Name: test_views; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_views;


--
-- Name: numeric; Type: COLLATION; Schema: public; Owner: -
--

CREATE COLLATION public."numeric" (provider = icu, locale = 'en-u-kn');


--
-- Name: ignore_accents; Type: COLLATION; Schema: schema2; Owner: -
--

CREATE COLLATION schema2.ignore_accents (provider = icu, deterministic = false, locale = 'und-u-kc-ks-level1');


--
-- Name: citext; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS citext WITH SCHEMA public;


--
-- Name: hstore; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS hstore WITH SCHEMA public;


--
-- Name: lo; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS lo WITH SCHEMA public;


--
-- Name: pg_stat_statements; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pg_stat_statements WITH SCHEMA public;


--
-- Name: pgcrypto; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;


--
-- Name: address_type; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.address_type AS (
	street character varying(100),
	city character varying(50),
	state character varying(50),
	zip_code character varying(10)
);


--
-- Name: enum_kind; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.enum_kind AS ENUM (
    'YES',
    'NO',
    'UNKNOWN'
);


--
-- Name: item_details; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.item_details AS (
	item_id integer,
	item_name character varying,
	item_price numeric(5,2)
);


--
-- Name: person_name; Type: DOMAIN; Schema: public; Owner: -
--

CREATE DOMAIN public.person_name AS character varying NOT NULL
	CONSTRAINT person_name_check CHECK (((VALUE)::text !~ '\s'::text));


--
-- Name: enum_kind; Type: TYPE; Schema: schema2; Owner: -
--

CREATE TYPE schema2.enum_kind AS ENUM (
    'YES',
    'NO',
    'UNKNOWN'
);


--
-- Name: item_details; Type: TYPE; Schema: schema2; Owner: -
--

CREATE TYPE schema2.item_details AS (
	item_id integer,
	item_name character varying,
	item_price numeric(5,2)
);


--
-- Name: person_name; Type: DOMAIN; Schema: schema2; Owner: -
--

CREATE DOMAIN schema2.person_name AS character varying NOT NULL
	CONSTRAINT person_name_check CHECK (((VALUE)::text !~ '\s'::text));


--
-- Name: asterisks(integer); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.asterisks(n integer) RETURNS SETOF text
    LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
    BEGIN ATOMIC
 SELECT repeat('*'::text, g.g) AS repeat
    FROM generate_series(1, asterisks.n) g(g);
END;


--
-- Name: asterisks1(integer); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.asterisks1(n integer) RETURNS text
    LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
    RETURN repeat('*'::text, n);


--
-- Name: auditlogfunc(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.auditlogfunc() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
   BEGIN
      INSERT INTO AUDIT(id) VALUES (current_timestamp); -- random comment
      RETURN NEW;
   END;
$$;


--
-- Name: check_sales_region(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.check_sales_region() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

    IF NEW.amount < 0 THEN
        RAISE EXCEPTION 'Amount cannot be negative';
    END IF;

    IF NEW.branch IS NULL OR NEW.branch = '' THEN
        RAISE EXCEPTION 'Branch name cannot be null or empty';
    END IF;

    RETURN NEW;
END;
$$;


--
-- Name: insert_non_decimal(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.insert_non_decimal() RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
    -- Create a table for demonstration
    CREATE TEMP TABLE non_decimal_table (
        id SERIAL,
        binary_value INTEGER,
        octal_value INTEGER,
        hex_value INTEGER
    );
    SELECT 5678901234, 0x1527D27F2, 0o52237223762, 0b101010010011111010010011111110010;
    -- Insert values into the table
    --not reported as parser converted these values to decimal ones while giving parseTree
    INSERT INTO non_decimal_table (binary_value, octal_value, hex_value)
    VALUES (0b1010, 0o012, 0xA); -- Binary (10), Octal (10), Hexadecimal (10)

    RAISE NOTICE 'Row inserted with non-decimal integers.';
END;
$$;


--
-- Name: manage_large_object(oid); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.manage_large_object(loid oid) RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF loid IS NOT NULL THEN
        -- Unlink the large object to free up storage
        PERFORM lo_unlink(loid);
    END IF;
END;
$$;


--
-- Name: notify_and_insert(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.notify_and_insert() RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
	LISTEN my_table_changes;
    INSERT INTO my_table (name) VALUES ('Charlie');
	NOTIFY my_table_changes, 'New row added with name: Charlie';
    PERFORM pg_notify('my_table_changes', 'New row added with name: Charlie');
	UNLISTEN my_table_changes;
END;
$$;


--
-- Name: prevent_update_shipped_without_date(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.prevent_update_shipped_without_date() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF TG_OP = 'UPDATE' AND NEW.status = 'shipped' AND NEW.shipped_date IS NULL THEN
        RAISE EXCEPTION 'Cannot update status to shipped without setting shipped_date';
    END IF;
    RETURN NEW;
END;
$$;


--
-- Name: process_combined_tbl(integer, cidr, bit, interval); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.process_combined_tbl(p_id integer, p_c cidr, p_bitt bit, p_inds3 interval) RETURNS macaddr
    LANGUAGE plpgsql
    AS $$
DECLARE
    v_maddr public.combined_tbl.maddr%TYPE;   
BEGIN
    -- Example logic: Assigning local variable using passed-in parameter
    v_maddr := p_c::text;  -- Example conversion (cidr to macaddr), just for illustration

    -- Processing the passed parameters
    RAISE NOTICE 'Processing: ID = %, CIDR = %, BIT = %, Interval = %, MAC = %',
        p_id, p_c, p_bitt, p_inds3, v_maddr;

    -- Returning a value of the macaddr type (this could be more meaningful logic)
    RETURN v_maddr;  -- Returning a macaddr value
END;
$$;


--
-- Name: process_order(integer); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.process_order(orderid integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
    lock_acquired BOOLEAN;
BEGIN
    lock_acquired := pg_try_advisory_lock(orderid); -- not able to report this as it is an assignment statement TODO: fix when support this 

    IF NOT lock_acquired THEN
        RAISE EXCEPTION 'Order % already being processed by another session', orderid;
    END IF;

    UPDATE orders
    SET processed_at = NOW()
    WHERE orders.order_id = orderid;

    RAISE NOTICE 'Order % processed successfully', orderid;

    PERFORM pg_advisory_unlock(orderid);
END;
$$;


--
-- Name: total(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.total() RETURNS integer
    LANGUAGE plpgsql
    AS $$
/******************************************************************************
  PACKAGE NAME :  fnc_req_ansr_delete
  DESCRIPTION:    This function return Request ID, Reason ID, along with JSON structure of Repetition_N

  REVISION HISTORY
  Date          Modified By         Description
  ----------    -----------         -----------------------------------------
  18/05/2018    Saurabh Singh  		Initial implementation.
  
	Input Parms:
    1. pv_in_per_id     -- WID (mandatory)                		--the function parameter type: rs_req_dbo.req.per_id%TYPE
    2. pv_in_pln_id     -- Plan Identifier (mandatory)    		--the function parameter type: rs_req_dbo.req.pln_id%TYPE
    3. pv_in_wi_req_id  -- Request Identifier (mandatory) 		--the function parameter type: rs_req_dbo.req_ansr.wi_req_id%TYPE,
    4. pv_in_rsn_id 		-- Reason ID Passed by API [REQUIRED]	--the function parameter type: rs_req_dbo.req_ansr.rsn_id%TYPE
    5. pv_in_json			-- JSON arry containing Repetition_N
	6. pv_in_req_src_x		jsonb		All logging information
	
	Output Parms:
	1. pv_out_json 		-- 	Return JSON Array of Repetition_N as rsc_id and rec_upd_tmst: 
    							{"rsc_id": null, "rec_upd_tmst": "2018-06-07T20:20:00"}

  
  29/05/2018	Saurabh Singh		Removed Return Code and Message from the function
  17/07/2018    Eoin Kelly          Implemented changes to JSON output and input params.
  08/08/2018    Saurabh Singh       Implemented New logic for Null Reason ID and null jason to delete based on what is provided.
  
  Sample Input:
  Select * from rs_req_fnc_dbo.fnc_req_ansr_delete(
  '{"api_appl_id":"default","api_lg_trk_id":"default","api_usr_role_c":"default","api_usr_id_ty_c":"default","api_usr_id":"default","api_read_only_i":"default"}',
  123456, 25001, 2, 2, '{"repitition_numbers" : [{"repitition_n" : 2}]}');
  
  Sample Output:
  {"rsc_id": null, "rec_upd_tmst": "2018-06-07T20:20:00"}    --- rsc_id is the REPITITION NUMBER
  
  ******************************************************************************/
declare
	total integer;
BEGIN
   SELECT inc_sum(i) into total FROM tt;
   RETURN total;
END;
$$;


--
-- Name: tt_insert_data(integer); Type: PROCEDURE; Schema: public; Owner: -
--

CREATE PROCEDURE public.tt_insert_data(IN i integer)
    LANGUAGE sql
    AS $$

INSERT INTO public."tt" VALUES ("i");

$$;


--
-- Name: update_combined_tbl_data(integer, cidr, bit, daterange); Type: PROCEDURE; Schema: public; Owner: -
--

CREATE PROCEDURE public.update_combined_tbl_data(IN p_id integer, IN p_c cidr, IN p_bitt bit, IN p_d daterange)
    LANGUAGE plpgsql
    AS $$
DECLARE
    v_new_mac public.combined_tbl.maddr%TYPE;   
BEGIN
    -- Example: Using a local variable to store a macaddr value (for illustration)
    v_new_mac := '00:14:22:01:23:45'::macaddr;

    -- Updating the table with provided parameters
    UPDATE public.combined_tbl
    SET 
        c = p_c,              -- Updating cidr type column
        bitt = p_bitt,        -- Updating bit column
        d = p_d,              -- Updating daterange column
        maddr = v_new_mac     -- Using the local macaddr variable in update
    WHERE id = p_id;

    RAISE NOTICE 'Updated record with ID: %, CIDR: %, BIT: %, Date range: %', 
        p_id, p_c, p_bitt, p_d;
END;
$$;


--
-- Name: asterisks(integer); Type: FUNCTION; Schema: schema2; Owner: -
--

CREATE FUNCTION schema2.asterisks(n integer) RETURNS SETOF text
    LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
    BEGIN ATOMIC
 SELECT repeat('*'::text, g.g) AS repeat
    FROM generate_series(1, asterisks.n) g(g);
END;


--
-- Name: asterisks1(integer); Type: FUNCTION; Schema: schema2; Owner: -
--

CREATE FUNCTION schema2.asterisks1(n integer) RETURNS text
    LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
    RETURN repeat('*'::text, n);


--
-- Name: auditlogfunc(); Type: FUNCTION; Schema: schema2; Owner: -
--

CREATE FUNCTION schema2.auditlogfunc() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
   BEGIN
      INSERT INTO AUDIT(id) VALUES (current_timestamp); -- random comment
      RETURN NEW;
   END;
$$;


--
-- Name: insert_non_decimal(); Type: FUNCTION; Schema: schema2; Owner: -
--

CREATE FUNCTION schema2.insert_non_decimal() RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
    -- Create a table for demonstration
    CREATE TEMP TABLE non_decimal_table (
        id SERIAL,
        binary_value INTEGER,
        octal_value INTEGER,
        hex_value INTEGER
    );
    SELECT 5678901234, 0x1527D27F2, 0o52237223762, 0b101010010011111010010011111110010;
    -- Insert values into the table
    --not reported as parser converted these values to decimal ones while giving parseTree
    INSERT INTO non_decimal_table (binary_value, octal_value, hex_value)
    VALUES (0b1010, 0o012, 0xA); -- Binary (10), Octal (10), Hexadecimal (10)

    RAISE NOTICE 'Row inserted with non-decimal integers.';
END;
$$;


--
-- Name: notify_and_insert(); Type: FUNCTION; Schema: schema2; Owner: -
--

CREATE FUNCTION schema2.notify_and_insert() RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
	LISTEN my_table_changes;
    INSERT INTO my_table (name) VALUES ('Charlie');
	NOTIFY my_table_changes, 'New row added with name: Charlie';
    PERFORM pg_notify('my_table_changes', 'New row added with name: Charlie');
	UNLISTEN my_table_changes;
END;
$$;


--
-- Name: prevent_update_shipped_without_date(); Type: FUNCTION; Schema: schema2; Owner: -
--

CREATE FUNCTION schema2.prevent_update_shipped_without_date() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF TG_OP = 'UPDATE' AND NEW.status = 'shipped' AND NEW.shipped_date IS NULL THEN
        RAISE EXCEPTION 'Cannot update status to shipped without setting shipped_date';
    END IF;
    RETURN NEW;
END;
$$;


--
-- Name: process_order(integer); Type: FUNCTION; Schema: schema2; Owner: -
--

CREATE FUNCTION schema2.process_order(orderid integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
    lock_acquired BOOLEAN;
BEGIN
    lock_acquired := pg_try_advisory_lock(orderid); -- not able to report this as it is an assignment statement TODO: fix when support this 

    IF NOT lock_acquired THEN
        RAISE EXCEPTION 'Order % already being processed by another session', orderid;
    END IF;

    UPDATE orders
    SET processed_at = NOW()
    WHERE orders.order_id = orderid;

    RAISE NOTICE 'Order % processed successfully', orderid;

    PERFORM pg_advisory_unlock(orderid);
END;
$$;


--
-- Name: total(); Type: FUNCTION; Schema: schema2; Owner: -
--

CREATE FUNCTION schema2.total() RETURNS integer
    LANGUAGE plpgsql
    AS $$
/******************************************************************************
  PACKAGE NAME :  fnc_req_ansr_delete
  DESCRIPTION:    This function return Request ID, Reason ID, along with JSON structure of Repetition_N

  REVISION HISTORY
  Date          Modified By         Description
  ----------    -----------         -----------------------------------------
  18/05/2018    Saurabh Singh  		Initial implementation.
  
	Input Parms:
    1. pv_in_per_id     -- WID (mandatory)                		--the function parameter type: rs_req_dbo.req.per_id%TYPE
    2. pv_in_pln_id     -- Plan Identifier (mandatory)    		--the function parameter type: rs_req_dbo.req.pln_id%TYPE
    3. pv_in_wi_req_id  -- Request Identifier (mandatory) 		--the function parameter type: rs_req_dbo.req_ansr.wi_req_id%TYPE,
    4. pv_in_rsn_id 		-- Reason ID Passed by API [REQUIRED]	--the function parameter type: rs_req_dbo.req_ansr.rsn_id%TYPE
    5. pv_in_json			-- JSON arry containing Repetition_N
	6. pv_in_req_src_x		jsonb		All logging information
	
	Output Parms:
	1. pv_out_json 		-- 	Return JSON Array of Repetition_N as rsc_id and rec_upd_tmst: 
    							{"rsc_id": null, "rec_upd_tmst": "2018-06-07T20:20:00"}

  
  29/05/2018	Saurabh Singh		Removed Return Code and Message from the function
  17/07/2018    Eoin Kelly          Implemented changes to JSON output and input params.
  08/08/2018    Saurabh Singh       Implemented New logic for Null Reason ID and null jason to delete based on what is provided.
  
  Sample Input:
  Select * from rs_req_fnc_dbo.fnc_req_ansr_delete(
  '{"api_appl_id":"default","api_lg_trk_id":"default","api_usr_role_c":"default","api_usr_id_ty_c":"default","api_usr_id":"default","api_read_only_i":"default"}',
  123456, 25001, 2, 2, '{"repitition_numbers" : [{"repitition_n" : 2}]}');
  
  Sample Output:
  {"rsc_id": null, "rec_upd_tmst": "2018-06-07T20:20:00"}    --- rsc_id is the REPITITION NUMBER
  
  ******************************************************************************/
declare
	total integer;
BEGIN
   SELECT inc_sum(i) into total FROM tt;
   RETURN total;
END;
$$;


--
-- Name: tt_insert_data(integer); Type: PROCEDURE; Schema: schema2; Owner: -
--

CREATE PROCEDURE schema2.tt_insert_data(IN i integer)
    LANGUAGE sql
    AS $$

INSERT INTO public."tt" VALUES ("i");

$$;


--
-- Name: inc_sum(integer); Type: AGGREGATE; Schema: public; Owner: -
--

CREATE AGGREGATE public.inc_sum(integer) (
    SFUNC = int4pl,
    STYPE = integer,
    INITCOND = '10'
);


--
-- Name: inc_sum(integer); Type: AGGREGATE; Schema: schema2; Owner: -
--

CREATE AGGREGATE schema2.inc_sum(integer) (
    SFUNC = int4pl,
    STYPE = integer,
    INITCOND = '10'
);


SET default_table_access_method = heap;

--
-- Name: Case_Sensitive_Columns; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public."Case_Sensitive_Columns" (
    id integer NOT NULL,
    "user" character varying(50),
    "Last_Name" character varying(50),
    email character varying(50),
    gender character varying(50),
    ip_address character varying(20)
);


--
-- Name: Case_Sensitive_Columns_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public."Case_Sensitive_Columns_id_seq"
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: Case_Sensitive_Columns_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public."Case_Sensitive_Columns_id_seq" OWNED BY public."Case_Sensitive_Columns".id;


--
-- Name: Mixed_Case_Table_Name_Test; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public."Mixed_Case_Table_Name_Test" (
    id integer NOT NULL,
    first_name character varying(50),
    last_name character varying(50),
    email character varying(50),
    gender character varying(50),
    ip_address character varying(20)
);


--
-- Name: Mixed_Case_Table_Name_Test_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public."Mixed_Case_Table_Name_Test_id_seq"
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: Mixed_Case_Table_Name_Test_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public."Mixed_Case_Table_Name_Test_id_seq" OWNED BY public."Mixed_Case_Table_Name_Test".id;


--
-- Name: Recipients; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public."Recipients" (
    id integer NOT NULL,
    first_name public.person_name,
    last_name public.person_name,
    misc public.enum_kind
);


--
-- Name: Recipients_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public."Recipients_id_seq"
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: Recipients_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public."Recipients_id_seq" OWNED BY public."Recipients".id;


--
-- Name: WITH; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public."WITH" (
    id integer NOT NULL,
    "WITH" character varying(100)
)
WITH (fillfactor='75', autovacuum_enabled='true', autovacuum_analyze_scale_factor='0.05');


--
-- Name: WITH_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public."WITH_id_seq"
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: WITH_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public."WITH_id_seq" OWNED BY public."WITH".id;


--
-- Name: audit; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.audit (
    id text
);


--
-- Name: bigint_multirange_table; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.bigint_multirange_table (
    id integer NOT NULL,
    value_ranges int8multirange
);


--
-- Name: bigint_multirange_table_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.bigint_multirange_table_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: bigint_multirange_table_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.bigint_multirange_table_id_seq OWNED BY public.bigint_multirange_table.id;


--
-- Name: sales_region; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sales_region (
    id integer NOT NULL,
    amount integer,
    branch text,
    region text NOT NULL
)
PARTITION BY LIST (region);


--
-- Name: boston; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.boston (
    id integer NOT NULL,
    amount integer,
    branch text,
    region text NOT NULL
);


--
-- Name: c; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.c (
    id integer,
    c text,
    nc text
);


--
-- Name: parent_table; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.parent_table (
    id integer NOT NULL,
    common_column1 text,
    common_column2 integer
);


--
-- Name: child_table; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.child_table (
    specific_column1 date
)
INHERITS (public.parent_table);


--
-- Name: citext_type; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.citext_type (
    id integer,
    data public.citext
);


--
-- Name: combined_tbl; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.combined_tbl (
    id integer NOT NULL,
    c cidr,
    maddr macaddr,
    maddr8 macaddr8,
    lsn pg_lsn,
    inds3 interval day to second(3),
    d daterange,
    bitt bit(13),
    bittv bit varying(15),
    address public.address_type,
    raster public.lo,
    arr_enum public.enum_kind[] NOT NULL,
    data public.hstore
);


--
-- Name: date_multirange_table; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.date_multirange_table (
    id integer NOT NULL,
    project_dates datemultirange
);


--
-- Name: date_multirange_table_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.date_multirange_table_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: date_multirange_table_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.date_multirange_table_id_seq OWNED BY public.date_multirange_table.id;


--
-- Name: documents; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.documents (
    id integer NOT NULL,
    title_tsvector tsvector,
    content_tsvector tsvector,
    list_of_sections text[]
);


--
-- Name: employees; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.employees (
    employee_id integer NOT NULL,
    first_name character varying(100),
    last_name character varying(100),
    department character varying(50)
);


--
-- Name: employees2; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.employees2 (
    id integer NOT NULL,
    first_name character varying(50) NOT NULL,
    last_name character varying(50) NOT NULL,
    full_name character varying(101) GENERATED ALWAYS AS ((((first_name)::text || ' '::text) || (last_name)::text)) STORED,
    department character varying(50)
);


--
-- Name: employees2_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.employees2_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: employees2_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.employees2_id_seq OWNED BY public.employees2.id;


--
-- Name: employees_employee_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.employees_employee_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: employees_employee_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.employees_employee_id_seq OWNED BY public.employees.employee_id;


--
-- Name: employeescopyfromwhere; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.employeescopyfromwhere (
    id integer NOT NULL,
    name text NOT NULL,
    age integer NOT NULL
);


--
-- Name: employeescopyonerror; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.employeescopyonerror (
    id integer NOT NULL,
    name text NOT NULL,
    age integer NOT NULL
);


--
-- Name: employeesforview; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.employeesforview (
    id integer NOT NULL,
    first_name character varying(50) NOT NULL,
    last_name character varying(50) NOT NULL,
    salary numeric(10,2) NOT NULL
);


--
-- Name: employeesforview_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.employeesforview_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: employeesforview_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.employeesforview_id_seq OWNED BY public.employeesforview.id;


--
-- Name: ext_test; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ext_test (
    id integer NOT NULL,
    password text
);


--
-- Name: ext_test_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.ext_test_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ext_test_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.ext_test_id_seq OWNED BY public.ext_test.id;


--
-- Name: foo; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.foo (
    id integer NOT NULL,
    value text
);


--
-- Name: inet_type; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.inet_type (
    id integer,
    data inet
);


--
-- Name: int_multirange_table; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.int_multirange_table (
    id integer NOT NULL,
    value_ranges int4multirange
);


--
-- Name: int_multirange_table_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.int_multirange_table_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: int_multirange_table_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.int_multirange_table_id_seq OWNED BY public.int_multirange_table.id;


--
-- Name: library_nested; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.library_nested (
    lib_id integer,
    lib_data xml
);


--
-- Name: london; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.london (
    id integer NOT NULL,
    amount integer,
    branch text,
    region text NOT NULL
);


--
-- Name: mixed_data_types_table1; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.mixed_data_types_table1 (
    id integer NOT NULL,
    point_data point,
    snapshot_data txid_snapshot,
    lseg_data lseg,
    box_data box
);


--
-- Name: mixed_data_types_table1_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.mixed_data_types_table1_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: mixed_data_types_table1_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.mixed_data_types_table1_id_seq OWNED BY public.mixed_data_types_table1.id;


--
-- Name: mixed_data_types_table2; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.mixed_data_types_table2 (
    id integer NOT NULL,
    lsn_data pg_lsn,
    lseg_data lseg,
    path_data path
);


--
-- Name: mixed_data_types_table2_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.mixed_data_types_table2_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: mixed_data_types_table2_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.mixed_data_types_table2_id_seq OWNED BY public.mixed_data_types_table2.id;


--
-- Name: numeric_multirange_table; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.numeric_multirange_table (
    id integer NOT NULL,
    price_ranges nummultirange
);


--
-- Name: numeric_multirange_table_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.numeric_multirange_table_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: numeric_multirange_table_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.numeric_multirange_table_id_seq OWNED BY public.numeric_multirange_table.id;


--
-- Name: orders; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.orders (
    item public.item_details,
    number_of_items integer,
    created_at timestamp with time zone DEFAULT now()
);


--
-- Name: orders2; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.orders2 (
    id integer NOT NULL,
    order_number character varying(50),
    status character varying(50) NOT NULL,
    shipped_date date
);


--
-- Name: orders2_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.orders2_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: orders2_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.orders2_id_seq OWNED BY public.orders2.id;


--
-- Name: orders_lateral; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.orders_lateral (
    order_id integer,
    customer_id integer,
    order_details xml
);


--
-- Name: ordersentry; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ordersentry (
    order_id integer NOT NULL,
    customer_name text NOT NULL,
    product_name text NOT NULL,
    quantity integer NOT NULL,
    price numeric(10,2) NOT NULL,
    processed_at timestamp without time zone,
    r integer DEFAULT regexp_count('This is an example. Another example. Example is a common word.'::text, 'example'::text)
);


--
-- Name: ordersentry_order_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.ordersentry_order_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ordersentry_order_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.ordersentry_order_id_seq OWNED BY public.ordersentry.order_id;


--
-- Name: ordersentry_view; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.ordersentry_view AS
 SELECT order_id,
    customer_name,
    product_name,
    quantity,
    price,
    XMLELEMENT(NAME "OrderDetails", XMLELEMENT(NAME "Customer", customer_name), XMLELEMENT(NAME "Product", product_name), XMLELEMENT(NAME "Quantity", quantity), XMLELEMENT(NAME "TotalPrice", (price * (quantity)::numeric))) AS order_xml,
    XMLCONCAT(XMLELEMENT(NAME "Customer", customer_name), XMLELEMENT(NAME "Product", product_name)) AS summary_xml,
    pg_try_advisory_lock((hashtext((customer_name || product_name)))::bigint) AS lock_acquired,
    ctid AS row_ctid,
    xmin AS transaction_id
   FROM public.ordersentry;


--
-- Name: parent_table_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.parent_table_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: parent_table_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.parent_table_id_seq OWNED BY public.parent_table.id;


--
-- Name: products; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.products (
    item public.item_details,
    added_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now()
);


--
-- Name: sales_employees; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.sales_employees AS
 SELECT id,
    first_name,
    last_name,
    full_name
   FROM public.employees2
  WHERE ((department)::text = 'sales'::text)
  WITH CASCADED CHECK OPTION;


--
-- Name: sales_unique_nulls_not_distinct; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sales_unique_nulls_not_distinct (
    store_id integer,
    product_id integer,
    sale_date date
);


--
-- Name: sales_unique_nulls_not_distinct_alter; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sales_unique_nulls_not_distinct_alter (
    store_id integer,
    product_id integer,
    sale_date date
);


--
-- Name: session_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.session_log (
    userid integer NOT NULL,
    phonenumber integer
);


--
-- Name: session_log1; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.session_log1 (
    userid integer NOT NULL,
    phonenumber integer
);


--
-- Name: session_log2; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.session_log2 (
    userid integer NOT NULL,
    phonenumber integer
);


--
-- Name: sydney; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sydney (
    id integer NOT NULL,
    amount integer,
    branch text,
    region text NOT NULL
);


--
-- Name: tbl_unlogged; Type: TABLE; Schema: public; Owner: -
--

CREATE UNLOGGED TABLE public.tbl_unlogged (
    id integer,
    val text
);


--
-- Name: test_exclude_basic; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.test_exclude_basic (
    id integer,
    name text,
    address text
);


--
-- Name: test_jsonb; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.test_jsonb (
    id integer,
    data jsonb,
    data2 text,
    region text
);


--
-- Name: test_xml_type; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.test_xml_type (
    id integer,
    data xml
);


--
-- Name: timestamp_multirange_table; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.timestamp_multirange_table (
    id integer NOT NULL,
    event_times tsmultirange
);


--
-- Name: timestamp_multirange_table_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.timestamp_multirange_table_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: timestamp_multirange_table_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.timestamp_multirange_table_id_seq OWNED BY public.timestamp_multirange_table.id;


--
-- Name: timestamptz_multirange_table; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.timestamptz_multirange_table (
    id integer NOT NULL,
    global_event_times tstzmultirange
);


--
-- Name: timestamptz_multirange_table_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.timestamptz_multirange_table_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: timestamptz_multirange_table_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.timestamptz_multirange_table_id_seq OWNED BY public.timestamptz_multirange_table.id;


--
-- Name: top_employees_view; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.top_employees_view AS
 SELECT id,
    first_name,
    last_name,
    salary
   FROM ( SELECT employeesforview.id,
            employeesforview.first_name,
            employeesforview.last_name,
            employeesforview.salary
           FROM public.employeesforview
          ORDER BY employeesforview.salary DESC
         FETCH FIRST 2 ROWS WITH TIES) top_employees;


--
-- Name: ts_query_table; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ts_query_table (
    id integer NOT NULL,
    query tsquery
);


--
-- Name: ts_query_table_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.ts_query_table ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.ts_query_table_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: tt; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tt (
    i integer
);


--
-- Name: users_unique_nulls_distinct; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users_unique_nulls_distinct (
    id integer NOT NULL,
    email text
);
ALTER TABLE ONLY public.users_unique_nulls_distinct ALTER COLUMN email SET COMPRESSION pglz;


--
-- Name: users_unique_nulls_not_distinct; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users_unique_nulls_not_distinct (
    id integer NOT NULL,
    email text
);


--
-- Name: users_unique_nulls_not_distinct_index; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users_unique_nulls_not_distinct_index (
    id integer NOT NULL,
    email text
);


--
-- Name: view_explicit_security_invoker; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.view_explicit_security_invoker WITH (security_invoker='true') AS
 SELECT employee_id,
    first_name
   FROM public.employees;


--
-- Name: with_example1; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.with_example1 (
    id integer NOT NULL,
    name character varying(100)
)
WITH (fillfactor='80', autovacuum_enabled='true', autovacuum_vacuum_scale_factor='0.1', autovacuum_freeze_min_age='10000000');


--
-- Name: with_example1_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.with_example1_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: with_example1_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.with_example1_id_seq OWNED BY public.with_example1.id;


--
-- Name: with_example2; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.with_example2 (
    id integer NOT NULL,
    name character varying(100)
)
WITH (fillfactor='80', autovacuum_enabled='true', autovacuum_vacuum_scale_factor='0.1', autovacuum_freeze_min_age='10000000');


--
-- Name: with_example2_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.with_example2_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: with_example2_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.with_example2_id_seq OWNED BY public.with_example2.id;


--
-- Name: Case_Sensitive_Columns; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2."Case_Sensitive_Columns" (
    id integer NOT NULL,
    "user" character varying(50),
    "Last_Name" character varying(50),
    email character varying(50),
    gender character varying(50),
    ip_address character varying(20)
);


--
-- Name: Case_Sensitive_Columns_id_seq; Type: SEQUENCE; Schema: schema2; Owner: -
--

CREATE SEQUENCE schema2."Case_Sensitive_Columns_id_seq"
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: Case_Sensitive_Columns_id_seq; Type: SEQUENCE OWNED BY; Schema: schema2; Owner: -
--

ALTER SEQUENCE schema2."Case_Sensitive_Columns_id_seq" OWNED BY schema2."Case_Sensitive_Columns".id;


--
-- Name: Mixed_Case_Table_Name_Test; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2."Mixed_Case_Table_Name_Test" (
    id integer NOT NULL,
    first_name character varying(50),
    last_name character varying(50),
    email character varying(50),
    gender character varying(50),
    ip_address character varying(20)
);


--
-- Name: Mixed_Case_Table_Name_Test_id_seq; Type: SEQUENCE; Schema: schema2; Owner: -
--

CREATE SEQUENCE schema2."Mixed_Case_Table_Name_Test_id_seq"
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: Mixed_Case_Table_Name_Test_id_seq; Type: SEQUENCE OWNED BY; Schema: schema2; Owner: -
--

ALTER SEQUENCE schema2."Mixed_Case_Table_Name_Test_id_seq" OWNED BY schema2."Mixed_Case_Table_Name_Test".id;


--
-- Name: Recipients; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2."Recipients" (
    id integer NOT NULL,
    first_name schema2.person_name,
    last_name schema2.person_name,
    misc schema2.enum_kind
);


--
-- Name: Recipients_id_seq; Type: SEQUENCE; Schema: schema2; Owner: -
--

CREATE SEQUENCE schema2."Recipients_id_seq"
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: Recipients_id_seq; Type: SEQUENCE OWNED BY; Schema: schema2; Owner: -
--

ALTER SEQUENCE schema2."Recipients_id_seq" OWNED BY schema2."Recipients".id;


--
-- Name: WITH; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2."WITH" (
    id integer NOT NULL,
    "WITH" character varying(100)
)
WITH (fillfactor='75', autovacuum_enabled='true', autovacuum_analyze_scale_factor='0.05');


--
-- Name: WITH_id_seq; Type: SEQUENCE; Schema: schema2; Owner: -
--

CREATE SEQUENCE schema2."WITH_id_seq"
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: WITH_id_seq; Type: SEQUENCE OWNED BY; Schema: schema2; Owner: -
--

ALTER SEQUENCE schema2."WITH_id_seq" OWNED BY schema2."WITH".id;


--
-- Name: audit; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.audit (
    id text
);


--
-- Name: bigint_multirange_table; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.bigint_multirange_table (
    id integer NOT NULL,
    value_ranges int8multirange
);


--
-- Name: bigint_multirange_table_id_seq; Type: SEQUENCE; Schema: schema2; Owner: -
--

CREATE SEQUENCE schema2.bigint_multirange_table_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: bigint_multirange_table_id_seq; Type: SEQUENCE OWNED BY; Schema: schema2; Owner: -
--

ALTER SEQUENCE schema2.bigint_multirange_table_id_seq OWNED BY schema2.bigint_multirange_table.id;


--
-- Name: sales_region; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.sales_region (
    id integer NOT NULL,
    amount integer,
    branch text,
    region text NOT NULL
)
PARTITION BY LIST (region);


--
-- Name: boston; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.boston (
    id integer NOT NULL,
    amount integer,
    branch text,
    region text NOT NULL
);


--
-- Name: c; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.c (
    id integer,
    c text,
    nc text
);


--
-- Name: parent_table; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.parent_table (
    id integer NOT NULL,
    common_column1 text,
    common_column2 integer
);


--
-- Name: child_table; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.child_table (
    specific_column1 date
)
INHERITS (schema2.parent_table);


--
-- Name: date_multirange_table; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.date_multirange_table (
    id integer NOT NULL,
    project_dates datemultirange
);


--
-- Name: date_multirange_table_id_seq; Type: SEQUENCE; Schema: schema2; Owner: -
--

CREATE SEQUENCE schema2.date_multirange_table_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: date_multirange_table_id_seq; Type: SEQUENCE OWNED BY; Schema: schema2; Owner: -
--

ALTER SEQUENCE schema2.date_multirange_table_id_seq OWNED BY schema2.date_multirange_table.id;


--
-- Name: employees2; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.employees2 (
    id integer NOT NULL,
    first_name character varying(50) NOT NULL,
    last_name character varying(50) NOT NULL,
    full_name character varying(101) GENERATED ALWAYS AS ((((first_name)::text || ' '::text) || (last_name)::text)) STORED,
    department character varying(50)
);


--
-- Name: employees2_id_seq; Type: SEQUENCE; Schema: schema2; Owner: -
--

CREATE SEQUENCE schema2.employees2_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: employees2_id_seq; Type: SEQUENCE OWNED BY; Schema: schema2; Owner: -
--

ALTER SEQUENCE schema2.employees2_id_seq OWNED BY schema2.employees2.id;


--
-- Name: employeesforview; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.employeesforview (
    id integer NOT NULL,
    first_name character varying(50) NOT NULL,
    last_name character varying(50) NOT NULL,
    salary numeric(10,2) NOT NULL
);


--
-- Name: employeesforview_id_seq; Type: SEQUENCE; Schema: schema2; Owner: -
--

CREATE SEQUENCE schema2.employeesforview_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: employeesforview_id_seq; Type: SEQUENCE OWNED BY; Schema: schema2; Owner: -
--

ALTER SEQUENCE schema2.employeesforview_id_seq OWNED BY schema2.employeesforview.id;


--
-- Name: ext_test; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.ext_test (
    id integer NOT NULL,
    password text
);


--
-- Name: ext_test_id_seq; Type: SEQUENCE; Schema: schema2; Owner: -
--

CREATE SEQUENCE schema2.ext_test_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ext_test_id_seq; Type: SEQUENCE OWNED BY; Schema: schema2; Owner: -
--

ALTER SEQUENCE schema2.ext_test_id_seq OWNED BY schema2.ext_test.id;


--
-- Name: foo; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.foo (
    id integer NOT NULL,
    value text
);


--
-- Name: int_multirange_table; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.int_multirange_table (
    id integer NOT NULL,
    value_ranges int4multirange
);


--
-- Name: int_multirange_table_id_seq; Type: SEQUENCE; Schema: schema2; Owner: -
--

CREATE SEQUENCE schema2.int_multirange_table_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: int_multirange_table_id_seq; Type: SEQUENCE OWNED BY; Schema: schema2; Owner: -
--

ALTER SEQUENCE schema2.int_multirange_table_id_seq OWNED BY schema2.int_multirange_table.id;


--
-- Name: london; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.london (
    id integer NOT NULL,
    amount integer,
    branch text,
    region text NOT NULL
);


--
-- Name: mixed_data_types_table1; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.mixed_data_types_table1 (
    id integer NOT NULL,
    point_data point,
    snapshot_data txid_snapshot,
    lseg_data lseg,
    box_data box
);


--
-- Name: mixed_data_types_table1_id_seq; Type: SEQUENCE; Schema: schema2; Owner: -
--

CREATE SEQUENCE schema2.mixed_data_types_table1_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: mixed_data_types_table1_id_seq; Type: SEQUENCE OWNED BY; Schema: schema2; Owner: -
--

ALTER SEQUENCE schema2.mixed_data_types_table1_id_seq OWNED BY schema2.mixed_data_types_table1.id;


--
-- Name: mixed_data_types_table2; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.mixed_data_types_table2 (
    id integer NOT NULL,
    lsn_data pg_lsn,
    lseg_data lseg,
    path_data path
);


--
-- Name: mixed_data_types_table2_id_seq; Type: SEQUENCE; Schema: schema2; Owner: -
--

CREATE SEQUENCE schema2.mixed_data_types_table2_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: mixed_data_types_table2_id_seq; Type: SEQUENCE OWNED BY; Schema: schema2; Owner: -
--

ALTER SEQUENCE schema2.mixed_data_types_table2_id_seq OWNED BY schema2.mixed_data_types_table2.id;


--
-- Name: numeric_multirange_table; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.numeric_multirange_table (
    id integer NOT NULL,
    price_ranges nummultirange
);


--
-- Name: numeric_multirange_table_id_seq; Type: SEQUENCE; Schema: schema2; Owner: -
--

CREATE SEQUENCE schema2.numeric_multirange_table_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: numeric_multirange_table_id_seq; Type: SEQUENCE OWNED BY; Schema: schema2; Owner: -
--

ALTER SEQUENCE schema2.numeric_multirange_table_id_seq OWNED BY schema2.numeric_multirange_table.id;


--
-- Name: orders; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.orders (
    item schema2.item_details,
    number_of_items integer,
    created_at timestamp with time zone DEFAULT now()
);


--
-- Name: orders2; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.orders2 (
    id integer NOT NULL,
    order_number character varying(50),
    status character varying(50) NOT NULL,
    shipped_date date
);


--
-- Name: orders2_id_seq; Type: SEQUENCE; Schema: schema2; Owner: -
--

CREATE SEQUENCE schema2.orders2_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: orders2_id_seq; Type: SEQUENCE OWNED BY; Schema: schema2; Owner: -
--

ALTER SEQUENCE schema2.orders2_id_seq OWNED BY schema2.orders2.id;


--
-- Name: parent_table_id_seq; Type: SEQUENCE; Schema: schema2; Owner: -
--

CREATE SEQUENCE schema2.parent_table_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: parent_table_id_seq; Type: SEQUENCE OWNED BY; Schema: schema2; Owner: -
--

ALTER SEQUENCE schema2.parent_table_id_seq OWNED BY schema2.parent_table.id;


--
-- Name: products; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.products (
    item schema2.item_details,
    added_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now()
);


--
-- Name: sales_employees; Type: VIEW; Schema: schema2; Owner: -
--

CREATE VIEW schema2.sales_employees AS
 SELECT id,
    first_name,
    last_name,
    full_name
   FROM schema2.employees2
  WHERE ((department)::text = 'sales'::text)
  WITH CASCADED CHECK OPTION;


--
-- Name: sales_unique_nulls_not_distinct; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.sales_unique_nulls_not_distinct (
    store_id integer,
    product_id integer,
    sale_date date
);


--
-- Name: sales_unique_nulls_not_distinct_alter; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.sales_unique_nulls_not_distinct_alter (
    store_id integer,
    product_id integer,
    sale_date date
);


--
-- Name: session_log; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.session_log (
    userid integer NOT NULL,
    phonenumber integer
);


--
-- Name: session_log1; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.session_log1 (
    userid integer NOT NULL,
    phonenumber integer
);


--
-- Name: session_log2; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.session_log2 (
    userid integer NOT NULL,
    phonenumber integer
);


--
-- Name: sydney; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.sydney (
    id integer NOT NULL,
    amount integer,
    branch text,
    region text NOT NULL
);


--
-- Name: tbl_unlogged; Type: TABLE; Schema: schema2; Owner: -
--

CREATE UNLOGGED TABLE schema2.tbl_unlogged (
    id integer,
    val text
);


--
-- Name: test_xml_type; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.test_xml_type (
    id integer,
    data xml
);


--
-- Name: timestamp_multirange_table; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.timestamp_multirange_table (
    id integer NOT NULL,
    event_times tsmultirange
);


--
-- Name: timestamp_multirange_table_id_seq; Type: SEQUENCE; Schema: schema2; Owner: -
--

CREATE SEQUENCE schema2.timestamp_multirange_table_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: timestamp_multirange_table_id_seq; Type: SEQUENCE OWNED BY; Schema: schema2; Owner: -
--

ALTER SEQUENCE schema2.timestamp_multirange_table_id_seq OWNED BY schema2.timestamp_multirange_table.id;


--
-- Name: timestamptz_multirange_table; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.timestamptz_multirange_table (
    id integer NOT NULL,
    global_event_times tstzmultirange
);


--
-- Name: timestamptz_multirange_table_id_seq; Type: SEQUENCE; Schema: schema2; Owner: -
--

CREATE SEQUENCE schema2.timestamptz_multirange_table_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: timestamptz_multirange_table_id_seq; Type: SEQUENCE OWNED BY; Schema: schema2; Owner: -
--

ALTER SEQUENCE schema2.timestamptz_multirange_table_id_seq OWNED BY schema2.timestamptz_multirange_table.id;


--
-- Name: top_employees_view; Type: VIEW; Schema: schema2; Owner: -
--

CREATE VIEW schema2.top_employees_view AS
 SELECT id,
    first_name,
    last_name,
    salary
   FROM ( SELECT employeesforview.id,
            employeesforview.first_name,
            employeesforview.last_name,
            employeesforview.salary
           FROM schema2.employeesforview
          ORDER BY employeesforview.salary DESC
         FETCH FIRST 2 ROWS WITH TIES) top_employees;


--
-- Name: tt; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.tt (
    i integer
);


--
-- Name: users_unique_nulls_distinct; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.users_unique_nulls_distinct (
    id integer NOT NULL,
    email text
);
ALTER TABLE ONLY schema2.users_unique_nulls_distinct ALTER COLUMN email SET COMPRESSION pglz;


--
-- Name: users_unique_nulls_not_distinct; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.users_unique_nulls_not_distinct (
    id integer NOT NULL,
    email text
);


--
-- Name: users_unique_nulls_not_distinct_index; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.users_unique_nulls_not_distinct_index (
    id integer NOT NULL,
    email text
);


--
-- Name: with_example1; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.with_example1 (
    id integer NOT NULL,
    name character varying(100)
)
WITH (fillfactor='80', autovacuum_enabled='true', autovacuum_vacuum_scale_factor='0.1', autovacuum_freeze_min_age='10000000');


--
-- Name: with_example1_id_seq; Type: SEQUENCE; Schema: schema2; Owner: -
--

CREATE SEQUENCE schema2.with_example1_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: with_example1_id_seq; Type: SEQUENCE OWNED BY; Schema: schema2; Owner: -
--

ALTER SEQUENCE schema2.with_example1_id_seq OWNED BY schema2.with_example1.id;


--
-- Name: with_example2; Type: TABLE; Schema: schema2; Owner: -
--

CREATE TABLE schema2.with_example2 (
    id integer NOT NULL,
    name character varying(100)
)
WITH (fillfactor='80', autovacuum_enabled='true', autovacuum_vacuum_scale_factor='0.1', autovacuum_freeze_min_age='10000000');


--
-- Name: with_example2_id_seq; Type: SEQUENCE; Schema: schema2; Owner: -
--

CREATE SEQUENCE schema2.with_example2_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: with_example2_id_seq; Type: SEQUENCE OWNED BY; Schema: schema2; Owner: -
--

ALTER SEQUENCE schema2.with_example2_id_seq OWNED BY schema2.with_example2.id;


--
-- Name: view_table1; Type: TABLE; Schema: test_views; Owner: -
--

CREATE TABLE test_views.view_table1 (
    id integer NOT NULL,
    first_name character varying(50),
    last_name character varying(50),
    email character varying(50),
    gender character varying(50),
    ip_address character varying(20)
);


--
-- Name: xyz_mview; Type: MATERIALIZED VIEW; Schema: test_views; Owner: -
--

CREATE MATERIALIZED VIEW test_views.xyz_mview AS
 SELECT first_name,
    last_name
   FROM test_views.view_table1
  WHERE ((gender)::text = 'Male'::text)
  WITH NO DATA;


--
-- Name: abc_mview; Type: MATERIALIZED VIEW; Schema: test_views; Owner: -
--

CREATE MATERIALIZED VIEW test_views.abc_mview AS
 SELECT first_name,
    last_name
   FROM test_views.xyz_mview
  WITH NO DATA;


--
-- Name: mv1; Type: MATERIALIZED VIEW; Schema: test_views; Owner: -
--

CREATE MATERIALIZED VIEW test_views.mv1 AS
 SELECT first_name,
    last_name
   FROM test_views.view_table1
  WHERE ((gender)::text = 'Male'::text)
  WITH NO DATA;


--
-- Name: v1; Type: VIEW; Schema: test_views; Owner: -
--

CREATE VIEW test_views.v1 AS
 SELECT first_name,
    last_name
   FROM test_views.view_table1
  WHERE ((gender)::text = 'Female'::text);


--
-- Name: view_table2; Type: TABLE; Schema: test_views; Owner: -
--

CREATE TABLE test_views.view_table2 (
    id integer NOT NULL,
    first_name character varying(50),
    last_name character varying(50),
    email character varying(50),
    gender character varying(50),
    ip_address character varying(20)
);


--
-- Name: v2; Type: VIEW; Schema: test_views; Owner: -
--

CREATE VIEW test_views.v2 AS
 SELECT a.first_name,
    b.last_name
   FROM test_views.view_table1 a,
    test_views.view_table2 b
  WHERE (a.id = b.id);


--
-- Name: v3; Type: VIEW; Schema: test_views; Owner: -
--

CREATE VIEW test_views.v3 AS
 SELECT a.first_name,
    b.last_name
   FROM (test_views.view_table1 a
     JOIN test_views.view_table2 b USING (id));


--
-- Name: v4; Type: VIEW; Schema: test_views; Owner: -
--

CREATE VIEW test_views.v4 AS
 SELECT ((((first_name)::text || ' '::text) || (last_name)::text) || ';'::text) AS full_name
   FROM test_views.view_table1 a;


--
-- Name: view_table1_id_seq; Type: SEQUENCE; Schema: test_views; Owner: -
--

CREATE SEQUENCE test_views.view_table1_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: view_table1_id_seq; Type: SEQUENCE OWNED BY; Schema: test_views; Owner: -
--

ALTER SEQUENCE test_views.view_table1_id_seq OWNED BY test_views.view_table1.id;


--
-- Name: view_table2_id_seq; Type: SEQUENCE; Schema: test_views; Owner: -
--

CREATE SEQUENCE test_views.view_table2_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: view_table2_id_seq; Type: SEQUENCE OWNED BY; Schema: test_views; Owner: -
--

ALTER SEQUENCE test_views.view_table2_id_seq OWNED BY test_views.view_table2.id;


--
-- Name: boston; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sales_region ATTACH PARTITION public.boston FOR VALUES IN ('Boston');


--
-- Name: london; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sales_region ATTACH PARTITION public.london FOR VALUES IN ('London');


--
-- Name: sydney; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sales_region ATTACH PARTITION public.sydney FOR VALUES IN ('Sydney');


--
-- Name: boston; Type: TABLE ATTACH; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.sales_region ATTACH PARTITION schema2.boston FOR VALUES IN ('Boston');


--
-- Name: london; Type: TABLE ATTACH; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.sales_region ATTACH PARTITION schema2.london FOR VALUES IN ('London');


--
-- Name: sydney; Type: TABLE ATTACH; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.sales_region ATTACH PARTITION schema2.sydney FOR VALUES IN ('Sydney');


--
-- Name: Case_Sensitive_Columns id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public."Case_Sensitive_Columns" ALTER COLUMN id SET DEFAULT nextval('public."Case_Sensitive_Columns_id_seq"'::regclass);


--
-- Name: Mixed_Case_Table_Name_Test id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public."Mixed_Case_Table_Name_Test" ALTER COLUMN id SET DEFAULT nextval('public."Mixed_Case_Table_Name_Test_id_seq"'::regclass);


--
-- Name: Recipients id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public."Recipients" ALTER COLUMN id SET DEFAULT nextval('public."Recipients_id_seq"'::regclass);


--
-- Name: WITH id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public."WITH" ALTER COLUMN id SET DEFAULT nextval('public."WITH_id_seq"'::regclass);


--
-- Name: bigint_multirange_table id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bigint_multirange_table ALTER COLUMN id SET DEFAULT nextval('public.bigint_multirange_table_id_seq'::regclass);


--
-- Name: child_table id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.child_table ALTER COLUMN id SET DEFAULT nextval('public.parent_table_id_seq'::regclass);


--
-- Name: date_multirange_table id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.date_multirange_table ALTER COLUMN id SET DEFAULT nextval('public.date_multirange_table_id_seq'::regclass);


--
-- Name: employees employee_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.employees ALTER COLUMN employee_id SET DEFAULT nextval('public.employees_employee_id_seq'::regclass);


--
-- Name: employees2 id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.employees2 ALTER COLUMN id SET DEFAULT nextval('public.employees2_id_seq'::regclass);


--
-- Name: employeesforview id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.employeesforview ALTER COLUMN id SET DEFAULT nextval('public.employeesforview_id_seq'::regclass);


--
-- Name: ext_test id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ext_test ALTER COLUMN id SET DEFAULT nextval('public.ext_test_id_seq'::regclass);


--
-- Name: int_multirange_table id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.int_multirange_table ALTER COLUMN id SET DEFAULT nextval('public.int_multirange_table_id_seq'::regclass);


--
-- Name: mixed_data_types_table1 id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.mixed_data_types_table1 ALTER COLUMN id SET DEFAULT nextval('public.mixed_data_types_table1_id_seq'::regclass);


--
-- Name: mixed_data_types_table2 id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.mixed_data_types_table2 ALTER COLUMN id SET DEFAULT nextval('public.mixed_data_types_table2_id_seq'::regclass);


--
-- Name: numeric_multirange_table id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.numeric_multirange_table ALTER COLUMN id SET DEFAULT nextval('public.numeric_multirange_table_id_seq'::regclass);


--
-- Name: orders2 id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.orders2 ALTER COLUMN id SET DEFAULT nextval('public.orders2_id_seq'::regclass);


--
-- Name: ordersentry order_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ordersentry ALTER COLUMN order_id SET DEFAULT nextval('public.ordersentry_order_id_seq'::regclass);


--
-- Name: parent_table id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.parent_table ALTER COLUMN id SET DEFAULT nextval('public.parent_table_id_seq'::regclass);


--
-- Name: timestamp_multirange_table id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.timestamp_multirange_table ALTER COLUMN id SET DEFAULT nextval('public.timestamp_multirange_table_id_seq'::regclass);


--
-- Name: timestamptz_multirange_table id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.timestamptz_multirange_table ALTER COLUMN id SET DEFAULT nextval('public.timestamptz_multirange_table_id_seq'::regclass);


--
-- Name: with_example1 id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.with_example1 ALTER COLUMN id SET DEFAULT nextval('public.with_example1_id_seq'::regclass);


--
-- Name: with_example2 id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.with_example2 ALTER COLUMN id SET DEFAULT nextval('public.with_example2_id_seq'::regclass);


--
-- Name: Case_Sensitive_Columns id; Type: DEFAULT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2."Case_Sensitive_Columns" ALTER COLUMN id SET DEFAULT nextval('schema2."Case_Sensitive_Columns_id_seq"'::regclass);


--
-- Name: Mixed_Case_Table_Name_Test id; Type: DEFAULT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2."Mixed_Case_Table_Name_Test" ALTER COLUMN id SET DEFAULT nextval('schema2."Mixed_Case_Table_Name_Test_id_seq"'::regclass);


--
-- Name: Recipients id; Type: DEFAULT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2."Recipients" ALTER COLUMN id SET DEFAULT nextval('schema2."Recipients_id_seq"'::regclass);


--
-- Name: WITH id; Type: DEFAULT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2."WITH" ALTER COLUMN id SET DEFAULT nextval('schema2."WITH_id_seq"'::regclass);


--
-- Name: bigint_multirange_table id; Type: DEFAULT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.bigint_multirange_table ALTER COLUMN id SET DEFAULT nextval('schema2.bigint_multirange_table_id_seq'::regclass);


--
-- Name: child_table id; Type: DEFAULT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.child_table ALTER COLUMN id SET DEFAULT nextval('schema2.parent_table_id_seq'::regclass);


--
-- Name: date_multirange_table id; Type: DEFAULT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.date_multirange_table ALTER COLUMN id SET DEFAULT nextval('schema2.date_multirange_table_id_seq'::regclass);


--
-- Name: employees2 id; Type: DEFAULT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.employees2 ALTER COLUMN id SET DEFAULT nextval('schema2.employees2_id_seq'::regclass);


--
-- Name: employeesforview id; Type: DEFAULT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.employeesforview ALTER COLUMN id SET DEFAULT nextval('schema2.employeesforview_id_seq'::regclass);


--
-- Name: ext_test id; Type: DEFAULT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.ext_test ALTER COLUMN id SET DEFAULT nextval('schema2.ext_test_id_seq'::regclass);


--
-- Name: int_multirange_table id; Type: DEFAULT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.int_multirange_table ALTER COLUMN id SET DEFAULT nextval('schema2.int_multirange_table_id_seq'::regclass);


--
-- Name: mixed_data_types_table1 id; Type: DEFAULT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.mixed_data_types_table1 ALTER COLUMN id SET DEFAULT nextval('schema2.mixed_data_types_table1_id_seq'::regclass);


--
-- Name: mixed_data_types_table2 id; Type: DEFAULT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.mixed_data_types_table2 ALTER COLUMN id SET DEFAULT nextval('schema2.mixed_data_types_table2_id_seq'::regclass);


--
-- Name: numeric_multirange_table id; Type: DEFAULT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.numeric_multirange_table ALTER COLUMN id SET DEFAULT nextval('schema2.numeric_multirange_table_id_seq'::regclass);


--
-- Name: orders2 id; Type: DEFAULT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.orders2 ALTER COLUMN id SET DEFAULT nextval('schema2.orders2_id_seq'::regclass);


--
-- Name: parent_table id; Type: DEFAULT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.parent_table ALTER COLUMN id SET DEFAULT nextval('schema2.parent_table_id_seq'::regclass);


--
-- Name: timestamp_multirange_table id; Type: DEFAULT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.timestamp_multirange_table ALTER COLUMN id SET DEFAULT nextval('schema2.timestamp_multirange_table_id_seq'::regclass);


--
-- Name: timestamptz_multirange_table id; Type: DEFAULT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.timestamptz_multirange_table ALTER COLUMN id SET DEFAULT nextval('schema2.timestamptz_multirange_table_id_seq'::regclass);


--
-- Name: with_example1 id; Type: DEFAULT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.with_example1 ALTER COLUMN id SET DEFAULT nextval('schema2.with_example1_id_seq'::regclass);


--
-- Name: with_example2 id; Type: DEFAULT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.with_example2 ALTER COLUMN id SET DEFAULT nextval('schema2.with_example2_id_seq'::regclass);


--
-- Name: view_table1 id; Type: DEFAULT; Schema: test_views; Owner: -
--

ALTER TABLE ONLY test_views.view_table1 ALTER COLUMN id SET DEFAULT nextval('test_views.view_table1_id_seq'::regclass);


--
-- Name: view_table2 id; Type: DEFAULT; Schema: test_views; Owner: -
--

ALTER TABLE ONLY test_views.view_table2 ALTER COLUMN id SET DEFAULT nextval('test_views.view_table2_id_seq'::regclass);


--
-- Name: Case_Sensitive_Columns Case_Sensitive_Columns_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public."Case_Sensitive_Columns"
    ADD CONSTRAINT "Case_Sensitive_Columns_pkey" PRIMARY KEY (id);


--
-- Name: Mixed_Case_Table_Name_Test Mixed_Case_Table_Name_Test_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public."Mixed_Case_Table_Name_Test"
    ADD CONSTRAINT "Mixed_Case_Table_Name_Test_pkey" PRIMARY KEY (id);


--
-- Name: Recipients Recipients_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public."Recipients"
    ADD CONSTRAINT "Recipients_pkey" PRIMARY KEY (id);


--
-- Name: WITH WITH_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public."WITH"
    ADD CONSTRAINT "WITH_pkey" PRIMARY KEY (id);


--
-- Name: bigint_multirange_table bigint_multirange_table_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bigint_multirange_table
    ADD CONSTRAINT bigint_multirange_table_pkey PRIMARY KEY (id);


--
-- Name: sales_region sales_region_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sales_region
    ADD CONSTRAINT sales_region_pkey PRIMARY KEY (id, region);


--
-- Name: boston boston_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.boston
    ADD CONSTRAINT boston_pkey PRIMARY KEY (id, region);


--
-- Name: combined_tbl combined_tbl_bittv_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.combined_tbl
    ADD CONSTRAINT combined_tbl_bittv_key UNIQUE (bittv);


--
-- Name: combined_tbl combined_tbl_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.combined_tbl
    ADD CONSTRAINT combined_tbl_pkey PRIMARY KEY (id, arr_enum);


--
-- Name: date_multirange_table date_multirange_table_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.date_multirange_table
    ADD CONSTRAINT date_multirange_table_pkey PRIMARY KEY (id);


--
-- Name: employees2 employees2_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.employees2
    ADD CONSTRAINT employees2_pkey PRIMARY KEY (id);


--
-- Name: employees employees_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.employees
    ADD CONSTRAINT employees_pkey PRIMARY KEY (employee_id);


--
-- Name: employeescopyfromwhere employeescopyfromwhere_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.employeescopyfromwhere
    ADD CONSTRAINT employeescopyfromwhere_pkey PRIMARY KEY (id);


--
-- Name: employeescopyonerror employeescopyonerror_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.employeescopyonerror
    ADD CONSTRAINT employeescopyonerror_pkey PRIMARY KEY (id);


--
-- Name: employeesforview employeesforview_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.employeesforview
    ADD CONSTRAINT employeesforview_pkey PRIMARY KEY (id);


--
-- Name: foo foo_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.foo
    ADD CONSTRAINT foo_pkey PRIMARY KEY (id);


--
-- Name: int_multirange_table int_multirange_table_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.int_multirange_table
    ADD CONSTRAINT int_multirange_table_pkey PRIMARY KEY (id);


--
-- Name: london london_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.london
    ADD CONSTRAINT london_pkey PRIMARY KEY (id, region);


--
-- Name: mixed_data_types_table1 mixed_data_types_table1_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.mixed_data_types_table1
    ADD CONSTRAINT mixed_data_types_table1_pkey PRIMARY KEY (id);


--
-- Name: mixed_data_types_table2 mixed_data_types_table2_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.mixed_data_types_table2
    ADD CONSTRAINT mixed_data_types_table2_pkey PRIMARY KEY (id);


--
-- Name: test_exclude_basic no_same_name_address; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.test_exclude_basic
    ADD CONSTRAINT no_same_name_address EXCLUDE USING btree (name WITH =, address WITH =);


--
-- Name: numeric_multirange_table numeric_multirange_table_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.numeric_multirange_table
    ADD CONSTRAINT numeric_multirange_table_pkey PRIMARY KEY (id);


--
-- Name: orders2 orders2_order_number_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.orders2
    ADD CONSTRAINT orders2_order_number_key UNIQUE (order_number) DEFERRABLE;


--
-- Name: orders2 orders2_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.orders2
    ADD CONSTRAINT orders2_pkey PRIMARY KEY (id);


--
-- Name: ordersentry ordersentry_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ordersentry
    ADD CONSTRAINT ordersentry_pkey PRIMARY KEY (order_id);


--
-- Name: parent_table parent_table_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.parent_table
    ADD CONSTRAINT parent_table_pkey PRIMARY KEY (id);


--
-- Name: sales_unique_nulls_not_distinct sales_unique_nulls_not_distin_store_id_product_id_sale_date_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sales_unique_nulls_not_distinct
    ADD CONSTRAINT sales_unique_nulls_not_distin_store_id_product_id_sale_date_key UNIQUE NULLS NOT DISTINCT (store_id, product_id, sale_date);


--
-- Name: sales_unique_nulls_not_distinct_alter sales_unique_nulls_not_distinct_alter_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sales_unique_nulls_not_distinct_alter
    ADD CONSTRAINT sales_unique_nulls_not_distinct_alter_unique UNIQUE NULLS NOT DISTINCT (store_id, product_id, sale_date);


--
-- Name: sydney sydney_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sydney
    ADD CONSTRAINT sydney_pkey PRIMARY KEY (id, region);


--
-- Name: timestamp_multirange_table timestamp_multirange_table_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.timestamp_multirange_table
    ADD CONSTRAINT timestamp_multirange_table_pkey PRIMARY KEY (id);


--
-- Name: timestamptz_multirange_table timestamptz_multirange_table_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.timestamptz_multirange_table
    ADD CONSTRAINT timestamptz_multirange_table_pkey PRIMARY KEY (id);


--
-- Name: combined_tbl uk; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.combined_tbl
    ADD CONSTRAINT uk UNIQUE (lsn);


--
-- Name: users_unique_nulls_distinct users_unique_nulls_distinct_email_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users_unique_nulls_distinct
    ADD CONSTRAINT users_unique_nulls_distinct_email_key UNIQUE (email);


--
-- Name: users_unique_nulls_distinct users_unique_nulls_distinct_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users_unique_nulls_distinct
    ADD CONSTRAINT users_unique_nulls_distinct_pkey PRIMARY KEY (id);


--
-- Name: users_unique_nulls_not_distinct users_unique_nulls_not_distinct_email_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users_unique_nulls_not_distinct
    ADD CONSTRAINT users_unique_nulls_not_distinct_email_key UNIQUE NULLS NOT DISTINCT (email);


--
-- Name: users_unique_nulls_not_distinct_index users_unique_nulls_not_distinct_index_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users_unique_nulls_not_distinct_index
    ADD CONSTRAINT users_unique_nulls_not_distinct_index_pkey PRIMARY KEY (id);


--
-- Name: users_unique_nulls_not_distinct users_unique_nulls_not_distinct_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users_unique_nulls_not_distinct
    ADD CONSTRAINT users_unique_nulls_not_distinct_pkey PRIMARY KEY (id);


--
-- Name: with_example1 with_example1_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.with_example1
    ADD CONSTRAINT with_example1_pkey PRIMARY KEY (id);


--
-- Name: with_example2 with_example2_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.with_example2
    ADD CONSTRAINT with_example2_pkey PRIMARY KEY (id);


--
-- Name: Case_Sensitive_Columns Case_Sensitive_Columns_pkey; Type: CONSTRAINT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2."Case_Sensitive_Columns"
    ADD CONSTRAINT "Case_Sensitive_Columns_pkey" PRIMARY KEY (id);


--
-- Name: Mixed_Case_Table_Name_Test Mixed_Case_Table_Name_Test_pkey; Type: CONSTRAINT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2."Mixed_Case_Table_Name_Test"
    ADD CONSTRAINT "Mixed_Case_Table_Name_Test_pkey" PRIMARY KEY (id);


--
-- Name: Recipients Recipients_pkey; Type: CONSTRAINT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2."Recipients"
    ADD CONSTRAINT "Recipients_pkey" PRIMARY KEY (id);


--
-- Name: WITH WITH_pkey; Type: CONSTRAINT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2."WITH"
    ADD CONSTRAINT "WITH_pkey" PRIMARY KEY (id);


--
-- Name: bigint_multirange_table bigint_multirange_table_pkey; Type: CONSTRAINT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.bigint_multirange_table
    ADD CONSTRAINT bigint_multirange_table_pkey PRIMARY KEY (id);


--
-- Name: sales_region sales_region_pkey; Type: CONSTRAINT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.sales_region
    ADD CONSTRAINT sales_region_pkey PRIMARY KEY (id, region);


--
-- Name: boston boston_pkey; Type: CONSTRAINT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.boston
    ADD CONSTRAINT boston_pkey PRIMARY KEY (id, region);


--
-- Name: date_multirange_table date_multirange_table_pkey; Type: CONSTRAINT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.date_multirange_table
    ADD CONSTRAINT date_multirange_table_pkey PRIMARY KEY (id);


--
-- Name: employees2 employees2_pkey; Type: CONSTRAINT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.employees2
    ADD CONSTRAINT employees2_pkey PRIMARY KEY (id);


--
-- Name: employeesforview employeesforview_pkey; Type: CONSTRAINT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.employeesforview
    ADD CONSTRAINT employeesforview_pkey PRIMARY KEY (id);


--
-- Name: foo foo_pkey; Type: CONSTRAINT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.foo
    ADD CONSTRAINT foo_pkey PRIMARY KEY (id);


--
-- Name: int_multirange_table int_multirange_table_pkey; Type: CONSTRAINT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.int_multirange_table
    ADD CONSTRAINT int_multirange_table_pkey PRIMARY KEY (id);


--
-- Name: london london_pkey; Type: CONSTRAINT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.london
    ADD CONSTRAINT london_pkey PRIMARY KEY (id, region);


--
-- Name: mixed_data_types_table1 mixed_data_types_table1_pkey; Type: CONSTRAINT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.mixed_data_types_table1
    ADD CONSTRAINT mixed_data_types_table1_pkey PRIMARY KEY (id);


--
-- Name: mixed_data_types_table2 mixed_data_types_table2_pkey; Type: CONSTRAINT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.mixed_data_types_table2
    ADD CONSTRAINT mixed_data_types_table2_pkey PRIMARY KEY (id);


--
-- Name: numeric_multirange_table numeric_multirange_table_pkey; Type: CONSTRAINT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.numeric_multirange_table
    ADD CONSTRAINT numeric_multirange_table_pkey PRIMARY KEY (id);


--
-- Name: orders2 orders2_order_number_key; Type: CONSTRAINT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.orders2
    ADD CONSTRAINT orders2_order_number_key UNIQUE (order_number) DEFERRABLE;


--
-- Name: orders2 orders2_pkey; Type: CONSTRAINT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.orders2
    ADD CONSTRAINT orders2_pkey PRIMARY KEY (id);


--
-- Name: parent_table parent_table_pkey; Type: CONSTRAINT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.parent_table
    ADD CONSTRAINT parent_table_pkey PRIMARY KEY (id);


--
-- Name: sales_unique_nulls_not_distinct sales_unique_nulls_not_distin_store_id_product_id_sale_date_key; Type: CONSTRAINT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.sales_unique_nulls_not_distinct
    ADD CONSTRAINT sales_unique_nulls_not_distin_store_id_product_id_sale_date_key UNIQUE NULLS NOT DISTINCT (store_id, product_id, sale_date);


--
-- Name: sales_unique_nulls_not_distinct_alter sales_unique_nulls_not_distinct_alter_unique; Type: CONSTRAINT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.sales_unique_nulls_not_distinct_alter
    ADD CONSTRAINT sales_unique_nulls_not_distinct_alter_unique UNIQUE NULLS NOT DISTINCT (store_id, product_id, sale_date);


--
-- Name: sydney sydney_pkey; Type: CONSTRAINT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.sydney
    ADD CONSTRAINT sydney_pkey PRIMARY KEY (id, region);


--
-- Name: timestamp_multirange_table timestamp_multirange_table_pkey; Type: CONSTRAINT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.timestamp_multirange_table
    ADD CONSTRAINT timestamp_multirange_table_pkey PRIMARY KEY (id);


--
-- Name: timestamptz_multirange_table timestamptz_multirange_table_pkey; Type: CONSTRAINT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.timestamptz_multirange_table
    ADD CONSTRAINT timestamptz_multirange_table_pkey PRIMARY KEY (id);


--
-- Name: users_unique_nulls_distinct users_unique_nulls_distinct_email_key; Type: CONSTRAINT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.users_unique_nulls_distinct
    ADD CONSTRAINT users_unique_nulls_distinct_email_key UNIQUE (email);


--
-- Name: users_unique_nulls_distinct users_unique_nulls_distinct_pkey; Type: CONSTRAINT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.users_unique_nulls_distinct
    ADD CONSTRAINT users_unique_nulls_distinct_pkey PRIMARY KEY (id);


--
-- Name: users_unique_nulls_not_distinct users_unique_nulls_not_distinct_email_key; Type: CONSTRAINT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.users_unique_nulls_not_distinct
    ADD CONSTRAINT users_unique_nulls_not_distinct_email_key UNIQUE NULLS NOT DISTINCT (email);


--
-- Name: users_unique_nulls_not_distinct_index users_unique_nulls_not_distinct_index_pkey; Type: CONSTRAINT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.users_unique_nulls_not_distinct_index
    ADD CONSTRAINT users_unique_nulls_not_distinct_index_pkey PRIMARY KEY (id);


--
-- Name: users_unique_nulls_not_distinct users_unique_nulls_not_distinct_pkey; Type: CONSTRAINT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.users_unique_nulls_not_distinct
    ADD CONSTRAINT users_unique_nulls_not_distinct_pkey PRIMARY KEY (id);


--
-- Name: with_example1 with_example1_pkey; Type: CONSTRAINT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.with_example1
    ADD CONSTRAINT with_example1_pkey PRIMARY KEY (id);


--
-- Name: with_example2 with_example2_pkey; Type: CONSTRAINT; Schema: schema2; Owner: -
--

ALTER TABLE ONLY schema2.with_example2
    ADD CONSTRAINT with_example2_pkey PRIMARY KEY (id);


--
-- Name: view_table1 view_table1_pkey; Type: CONSTRAINT; Schema: test_views; Owner: -
--

ALTER TABLE ONLY test_views.view_table1
    ADD CONSTRAINT view_table1_pkey PRIMARY KEY (id);


--
-- Name: view_table2 view_table2_pkey; Type: CONSTRAINT; Schema: test_views; Owner: -
--

ALTER TABLE ONLY test_views.view_table2
    ADD CONSTRAINT view_table2_pkey PRIMARY KEY (id);


--
-- Name: idx1; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx1 ON public.combined_tbl USING btree (c);


--
-- Name: idx2; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx2 ON public.combined_tbl USING btree (maddr);


--
-- Name: idx3; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx3 ON public.combined_tbl USING btree (maddr8);


--
-- Name: idx4; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx4 ON public.combined_tbl USING btree (lsn);


--
-- Name: idx5; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx5 ON public.combined_tbl USING btree (bitt);


--
-- Name: idx6; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx6 ON public.combined_tbl USING btree (bittv);


--
-- Name: idx7; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx7 ON public.combined_tbl USING btree (address);


--
-- Name: idx8; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx8 ON public.combined_tbl USING btree (d);


--
-- Name: idx9; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx9 ON public.combined_tbl USING btree (inds3);


--
-- Name: idx_array; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_array ON public.documents USING btree (list_of_sections);


--
-- Name: idx_box_data; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_box_data ON public.mixed_data_types_table1 USING gist (box_data);


--
-- Name: idx_box_data_brin; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_box_data_brin ON public.mixed_data_types_table1 USING brin (box_data);


--
-- Name: idx_citext; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_citext ON public.citext_type USING btree (data);


--
-- Name: idx_citext1; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_citext1 ON public.citext_type USING btree (lower((data)::text));


--
-- Name: idx_citext2; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_citext2 ON public.citext_type USING btree (((data)::text));


--
-- Name: idx_inet; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_inet ON public.inet_type USING btree (data);


--
-- Name: idx_inet1; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_inet1 ON public.inet_type USING btree (((data)::text));


--
-- Name: idx_json; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_json ON public.test_jsonb USING btree (data);


--
-- Name: idx_json2; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_json2 ON public.test_jsonb USING btree (((data2)::jsonb));


--
-- Name: idx_point_data; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_point_data ON public.mixed_data_types_table1 USING gist (point_data);


--
-- Name: idx_valid; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_valid ON public.test_jsonb USING btree (((data)::text));


--
-- Name: tsquery_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX tsquery_idx ON public.ts_query_table USING btree (query);


--
-- Name: tsvector_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX tsvector_idx ON public.documents USING btree (title_tsvector, id);


--
-- Name: users_unique_nulls_not_distinct_index_email; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX users_unique_nulls_not_distinct_index_email ON public.users_unique_nulls_not_distinct_index USING btree (email) NULLS NOT DISTINCT;


--
-- Name: idx_box_data; Type: INDEX; Schema: schema2; Owner: -
--

CREATE INDEX idx_box_data ON schema2.mixed_data_types_table1 USING gist (box_data);


--
-- Name: idx_box_data_spgist; Type: INDEX; Schema: schema2; Owner: -
--

CREATE INDEX idx_box_data_spgist ON schema2.mixed_data_types_table1 USING spgist (box_data);


--
-- Name: idx_point_data; Type: INDEX; Schema: schema2; Owner: -
--

CREATE INDEX idx_point_data ON schema2.mixed_data_types_table1 USING gist (point_data);


--
-- Name: users_unique_nulls_not_distinct_index_email; Type: INDEX; Schema: schema2; Owner: -
--

CREATE UNIQUE INDEX users_unique_nulls_not_distinct_index_email ON schema2.users_unique_nulls_not_distinct_index USING btree (email) NULLS NOT DISTINCT;


--
-- Name: boston_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.sales_region_pkey ATTACH PARTITION public.boston_pkey;


--
-- Name: london_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.sales_region_pkey ATTACH PARTITION public.london_pkey;


--
-- Name: sydney_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.sales_region_pkey ATTACH PARTITION public.sydney_pkey;


--
-- Name: boston_pkey; Type: INDEX ATTACH; Schema: schema2; Owner: -
--

ALTER INDEX schema2.sales_region_pkey ATTACH PARTITION schema2.boston_pkey;


--
-- Name: london_pkey; Type: INDEX ATTACH; Schema: schema2; Owner: -
--

ALTER INDEX schema2.sales_region_pkey ATTACH PARTITION schema2.london_pkey;


--
-- Name: sydney_pkey; Type: INDEX ATTACH; Schema: schema2; Owner: -
--

ALTER INDEX schema2.sales_region_pkey ATTACH PARTITION schema2.sydney_pkey;


--
-- Name: v1 protect_test_views_v1; Type: RULE; Schema: test_views; Owner: -
--

CREATE RULE protect_test_views_v1 AS
    ON UPDATE TO test_views.v1 DO INSTEAD NOTHING;


--
-- Name: v2 protect_test_views_v2; Type: RULE; Schema: test_views; Owner: -
--

CREATE RULE protect_test_views_v2 AS
    ON UPDATE TO test_views.v2 DO INSTEAD NOTHING;


--
-- Name: v3 protect_test_views_v3; Type: RULE; Schema: test_views; Owner: -
--

CREATE RULE protect_test_views_v3 AS
    ON UPDATE TO test_views.v3 DO INSTEAD NOTHING;


--
-- Name: view_table1 protect_test_views_view_table1; Type: RULE; Schema: test_views; Owner: -
--

CREATE RULE protect_test_views_view_table1 AS
    ON UPDATE TO test_views.view_table1 DO INSTEAD NOTHING;


--
-- Name: tt audit_trigger; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER audit_trigger AFTER INSERT ON public.tt FOR EACH ROW EXECUTE FUNCTION public.auditlogfunc();


--
-- Name: sales_region before_sales_region_insert_update; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER before_sales_region_insert_update BEFORE INSERT OR UPDATE ON public.sales_region FOR EACH ROW EXECUTE FUNCTION public.check_sales_region();


--
-- Name: orders2 enforce_shipped_date_constraint; Type: TRIGGER; Schema: public; Owner: -
--

CREATE CONSTRAINT TRIGGER enforce_shipped_date_constraint AFTER UPDATE ON public.orders2 NOT DEFERRABLE INITIALLY IMMEDIATE FOR EACH ROW WHEN ((((new.status)::text = 'shipped'::text) AND (new.shipped_date IS NULL))) EXECUTE FUNCTION public.prevent_update_shipped_without_date();


--
-- Name: combined_tbl t_raster; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER t_raster BEFORE DELETE OR UPDATE ON public.combined_tbl FOR EACH ROW EXECUTE FUNCTION public.lo_manage('raster');


--
-- Name: tt audit_trigger; Type: TRIGGER; Schema: schema2; Owner: -
--

CREATE TRIGGER audit_trigger AFTER INSERT ON schema2.tt FOR EACH ROW EXECUTE FUNCTION schema2.auditlogfunc();


--
-- Name: orders2 enforce_shipped_date_constraint; Type: TRIGGER; Schema: schema2; Owner: -
--

CREATE CONSTRAINT TRIGGER enforce_shipped_date_constraint AFTER UPDATE ON schema2.orders2 NOT DEFERRABLE INITIALLY IMMEDIATE FOR EACH ROW WHEN ((((new.status)::text = 'shipped'::text) AND (new.shipped_date IS NULL))) EXECUTE FUNCTION schema2.prevent_update_shipped_without_date();


--
-- Name: test_jsonb test_jsonb_id_region_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.test_jsonb
    ADD CONSTRAINT test_jsonb_id_region_fkey FOREIGN KEY (id, region) REFERENCES public.sales_region(id, region);


--
-- Name: test_exclude_basic policy_test_fine; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY policy_test_fine ON public.test_exclude_basic USING (((id % 2) = 1));


--
-- Name: employees2 policy_test_fine_2; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY policy_test_fine_2 ON public.employees2 USING ((id <> ALL (ARRAY[12, 123, 41241])));


--
-- Name: test_xml_type policy_test_report; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY policy_test_report ON public.test_xml_type TO test_policy USING (true);


--
-- Name: test_xml_type policy_test_report; Type: POLICY; Schema: schema2; Owner: -
--

CREATE POLICY policy_test_report ON schema2.test_xml_type TO test_policy USING (true);


--
-- PostgreSQL database dump complete
--

