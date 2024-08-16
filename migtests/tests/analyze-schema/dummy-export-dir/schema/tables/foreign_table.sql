--foreign table issues
CREATE FOREIGN TABLE tbl_p(
	id int PRIMARY KEY
) SERVER remote_server
OPTIONS (
    schema_name 'public',
    table_name 'remote_table'
);

--Foreign key constraints on Foreign table is not supported in PG