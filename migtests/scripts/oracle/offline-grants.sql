GRANT CONNECT TO ybvoyager;
GRANT SELECT_CATALOG_ROLE TO ybvoyager;
GRANT SELECT ANY DICTIONARY TO ybvoyager;
GRANT SELECT ON SYS.ARGUMENT$ TO ybvoyager;
GRANT SELECT_CATALOG_ROLE TO ybvoyager;
GRANT SELECT ANY DICTIONARY TO ybvoyager;
GRANT SELECT ON SYS.ARGUMENT$ TO ybvoyager;
BEGIN
    FOR R IN (SELECT owner, object_name FROM all_objects WHERE owner=UPPER('${db_schema}') and object_type = 'TYPE') LOOP
    	EXECUTE IMMEDIATE 'grant execute on '||R.owner||'."'||R.object_name||'" to ybvoyager';
   	END LOOP;
END;
/
BEGIN
    FOR R IN (SELECT owner, object_name FROM all_objects WHERE owner=UPPER('${db_schema}') and object_type in ('VIEW','SEQUENCE','TABLE PARTITION','SYNONYM','MATERIALIZED VIEW')) LOOP
        EXECUTE IMMEDIATE 'grant select on '||R.owner||'."'||R.object_name||'" to ybvoyager';
  	END LOOP;
END;
/
BEGIN
	FOR R IN (SELECT owner, object_name FROM all_objects WHERE owner=UPPER('${db_schema}') and object_type ='TABLE' MINUS SELECT owner, table_name from all_nested_tables where owner = UPPER('${db_schema}')) LOOP
		EXECUTE IMMEDIATE 'grant select on '||R.owner||'."'||R.object_name||'" to  ybvoyager';
	END LOOP;
END;
/
/*
Extra steps required to enable Debezium export
*/
GRANT FLASHBACK ANY TABLE TO ybvoyager;