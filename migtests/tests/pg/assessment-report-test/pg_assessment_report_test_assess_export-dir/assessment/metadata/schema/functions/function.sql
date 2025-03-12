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


CREATE FUNCTION public.asterisks(n integer) RETURNS SETOF text
    LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
    BEGIN ATOMIC
 SELECT repeat('*'::text, g.g) AS repeat
    FROM generate_series(1, asterisks.n) g(g);
END;


CREATE FUNCTION public.asterisks1(n integer) RETURNS text
    LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
    RETURN repeat('*'::text, n);


CREATE FUNCTION public.auditlogfunc() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
   BEGIN
      INSERT INTO AUDIT(id) VALUES (current_timestamp); -- random comment
      RETURN NEW;
   END;
$$;


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


CREATE FUNCTION schema2.asterisks(n integer) RETURNS SETOF text
    LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
    BEGIN ATOMIC
 SELECT repeat('*'::text, g.g) AS repeat
    FROM generate_series(1, asterisks.n) g(g);
END;


CREATE FUNCTION schema2.asterisks1(n integer) RETURNS text
    LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
    RETURN repeat('*'::text, n);


CREATE FUNCTION schema2.auditlogfunc() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
   BEGIN
      INSERT INTO AUDIT(id) VALUES (current_timestamp); -- random comment
      RETURN NEW;
   END;
$$;


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


