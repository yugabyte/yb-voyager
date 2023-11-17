CREATE ROLE FF_SCHEMA_writer_role;

BEGIN
    FOR R IN (SELECT owner, object_name FROM all_objects WHERE owner=UPPER('FF_SCHEMA') and object_type ='TABLE' MINUS SELECT owner, table_name from all_nested_tables where owner = UPPER('FF_SCHEMA'))
    LOOP
       EXECUTE IMMEDIATE 'GRANT SELECT, INSERT, UPDATE, DELETE, ALTER on '||R.owner||'."'||R.object_name||'" to  FF_SCHEMA_writer_role';
    END LOOP;
END;
/

DECLARE
   v_sql VARCHAR2(4000);
BEGIN
    FOR table_rec IN (SELECT table_name FROM all_tables WHERE owner = 'YBVOYAGER_METADATA') LOOP
     v_sql := 'GRANT ALL PRIVILEGES ON YBVOYAGER_METADATA.' || table_rec.table_name || ' TO FF_SCHEMA_writer_role';
      EXECUTE IMMEDIATE v_sql;
    END LOOP;
END;
/

GRANT CREATE ANY SEQUENCE, SELECT ANY SEQUENCE, ALTER ANY SEQUENCE TO FF_SCHEMA_writer_role;

CREATE USER YBVOYAGER_FF IDENTIFIED BY password;
GRANT CONNECT TO YBVOYAGER_FF;
GRANT FF_SCHEMA_writer_role TO YBVOYAGER_FF;



