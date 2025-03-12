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


CREATE PROCEDURE public.tt_insert_data(IN i integer)
    LANGUAGE sql
    AS $$
INSERT INTO public."tt" VALUES ("i");
$$;


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


CREATE PROCEDURE schema2.tt_insert_data(IN i integer)
    LANGUAGE sql
    AS $$
INSERT INTO public."tt" VALUES ("i");
$$;


