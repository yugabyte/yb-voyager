drop extension if exists pgcrypto;

CREATE EXTENSION pgcrypto;

drop table if exists ext_test;

create table ext_test(id serial PRIMARY KEY, password text);

select * from ext_test;

drop aggregate if exists inc_sum(int);

CREATE AGGREGATE inc_sum(int) (
    sfunc = int4pl,
    stype = int,
    initcond = 10
);

select aggfnoid,agginitval,aggkind  from pg_aggregate
where aggfnoid = 'inc_sum'::regproc;

drop table if exists tt;

CREATE TABLE tt (i int PRIMARY KEY);

drop table if exists audit;

create table audit(id text PRIMARY KEY);

drop function if exists auditlogfunc();

CREATE OR REPLACE FUNCTION auditlogfunc() RETURNS TRIGGER AS $example_table$
   BEGIN
      INSERT INTO AUDIT(id) VALUES (current_timestamp); -- random comment
      RETURN NEW;
   END;
$example_table$ LANGUAGE plpgsql;

drop trigger if exists audit_trigger on tt;

CREATE TRIGGER audit_trigger AFTER INSERT ON tt
FOR EACH ROW EXECUTE PROCEDURE auditlogfunc();

drop procedure if exists tt_insert_data;

CREATE OR REPLACE PROCEDURE tt_insert_data("i" integer)

LANGUAGE SQL

AS $$

INSERT INTO "tt" VALUES ("i");

$$;

CREATE OR REPLACE FUNCTION total ()
RETURNS integer AS $agg_output$
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
$agg_output$ LANGUAGE plpgsql;

select total();
SELECT * from audit;

-- contains Types, Domains, Comments, Mixed Cases

DROP type IF EXISTS enum_kind cascade; 

CREATE TYPE enum_kind AS ENUM (
    'YES',
    'NO',
    'UNKNOWN'
);


DROP type IF EXISTS Item_details cascade;

CREATE TYPE Item_details AS (  
    item_id INT,  
    item_name VARCHAR,  
    item_price Numeric(5,2)  
);  


DROP DOMAIN IF EXISTS person_name cascade; 

CREATE DOMAIN person_name AS   
VARCHAR NOT NULL CHECK (value!~ '\s'); 

drop table if exists Recipients;

CREATE TABLE Recipients (  
    ID SERIAL PRIMARY KEY,  
    First_name person_name,  
    Last_name person_name,  
    Misc enum_kind  
    );  
    

drop table if exists session_log;

create table session_log 
( 
   userid int not null PRIMARY KEY, 
   phonenumber int
); 

comment on column session_log.userid is 'The user ID';
comment on column session_log.phonenumber is 'The phone number including the area code';

comment on table session_log is 'Our session logs';

\d+ session_log

\dt+ session_log


drop table if exists "Mixed_Case_Table_Name_Test";

Create table "Mixed_Case_Table_Name_Test" (
	id serial primary key,
	first_name VARCHAR(50),
	last_name VARCHAR(50),
	email VARCHAR(50),
	gender VARCHAR(50),
	ip_address VARCHAR(20)
    
);

-- aut int NOT NULL GENERATED ALWAYS AS ((id*9)+1) stored


CREATE TABLE "group" (
    id int PRIMARY KEY,
    name varchar(10)
);

