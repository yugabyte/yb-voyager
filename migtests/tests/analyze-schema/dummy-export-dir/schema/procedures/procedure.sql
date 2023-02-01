--drop temporary table clause
CREATE OR REPLACE PROCEDURE foo (p_id integer) AS $body$
BEGIN
    drop temporary table if exists temp;

    create temporary table temp(id int, name text);

    insert into temp(id,name) select id,p_name from bar where p_id=id;

    select name from temp;

end;
$body$
LANGUAGE PLPGSQL
SECURITY DEFINER
;