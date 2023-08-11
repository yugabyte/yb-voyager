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

CREATE TABLE ybvoyager_metadata.ybvoyager_import_data_event_channels_metainfo (
				migration_uuid VARCHAR2(36),
				channel_no INT,
				last_applied_vsn NUMBER(19),
				PRIMARY KEY (migration_uuid, channel_no)
			);

-- Grant all privileges on all objects in the schema
GRANT SELECT, INSERT, UPDATE, DELETE ON ybvoyager_metadata.ybvoyager_import_data_event_channels_metainfo TO ybvoyager;
GRANT SELECT, INSERT, UPDATE, DELETE ON ybvoyager_metadata.ybvoyager_import_data_batches_metainfo_v2 TO ybvoyager;
-- Grant SELECT, INSERT, and DELETE privileges on all tables in the schema
BEGIN
    FOR t IN (SELECT table_name FROM all_tables WHERE owner = 'TEST_SCHEMA') LOOP
        EXECUTE IMMEDIATE 'GRANT SELECT, INSERT, DELETE ON TEST_SCHEMA.' || t.table_name || ' TO ybvoyager';
    END LOOP;
END;
/
