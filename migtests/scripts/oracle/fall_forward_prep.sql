CREATE USER FF_SCHEMA IDENTIFIED BY "password";
GRANT all privileges to FF_SCHEMA;

-- Grant all privileges on ybvoyager_metadata schema table to user ybvoyager
DECLARE
   v_sql VARCHAR2(4000);
BEGIN
   FOR table_rec IN (SELECT table_name FROM all_tables WHERE owner = 'YBVOYAGER_METADATA') LOOP
      v_sql := 'GRANT ALL PRIVILEGES ON YBVOYAGER_METADATA.' || table_rec.table_name || ' TO C##YBVOYAGER';
      EXECUTE IMMEDIATE v_sql;
   END LOOP;
END;
/


