



CREATE OR REPLACE FUNCTION add_two (num1 integer,num2 integer) RETURNS integer AS $body$
BEGIN
    return num1+num2;
END;
$body$
LANGUAGE PLPGSQL
SECURITY DEFINER
 IMMUTABLE;
-- REVOKE ALL ON FUNCTION add_two (num1 integer,num2 integer) FROM PUBLIC;





CREATE OR REPLACE FUNCTION f_name (first_n varchar(50),last_n varchar(50)) RETURNS varchar AS $body$
BEGIN
    return concat(first_n,' ',last_n);
END;
$body$
LANGUAGE PLPGSQL
SECURITY DEFINER
 IMMUTABLE;
-- REVOKE ALL ON FUNCTION f_name (first_n varchar(50),last_n varchar(50)) FROM PUBLIC;

