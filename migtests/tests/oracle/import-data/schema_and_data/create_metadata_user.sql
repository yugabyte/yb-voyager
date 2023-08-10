CREATE USER ybvoyager_metadata IDENTIFIED BY password;
GRANT CONNECT, RESOURCE TO ybvoyager_metadata;
ALTER USER ybvoyager_metadata QUOTA UNLIMITED ON USERS;

CREATE TABLE ybvoyager_metadata.ybvoyager_import_data_batches_metainfo_v2 (
			data_file_name VARCHAR2(250),
			batch_number NUMBER(10),
			schema_name VARCHAR2(250),
			table_name VARCHAR2(250),
			rows_imported NUMBER(19),
			PRIMARY KEY (data_file_name, batch_number, schema_name, table_name)
		);
