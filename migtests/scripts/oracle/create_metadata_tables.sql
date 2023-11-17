CREATE USER ybvoyager_metadata IDENTIFIED BY "password";
GRANT CONNECT, RESOURCE TO ybvoyager_metadata;
ALTER USER ybvoyager_metadata QUOTA UNLIMITED ON USERS;

-- upgraded to _v3 to avoid conflict after 1.6
CREATE TABLE ybvoyager_metadata.ybvoyager_import_data_batches_metainfo_v3 ( 
            migration_uuid VARCHAR2(36),
            data_file_name VARCHAR2(250),
            batch_number NUMBER(10),
            schema_name VARCHAR2(250),
            table_name VARCHAR2(250),
            rows_imported NUMBER(19),
            PRIMARY KEY (migration_uuid, data_file_name, batch_number, schema_name, table_name)
        );

CREATE TABLE ybvoyager_metadata.ybvoyager_import_data_event_channels_metainfo (
            migration_uuid VARCHAR2(36),
            channel_no INT,
            last_applied_vsn NUMBER(19),
            num_inserts NUMBER(19),
            num_updates NUMBER(19),
            num_deletes NUMBER(19),
            PRIMARY KEY (migration_uuid, channel_no)
        );

CREATE TABLE ybvoyager_metadata.ybvoyager_imported_event_count_by_table (
        migration_uuid VARCHAR2(36),
        table_name VARCHAR2(250),
        channel_no INT,
        total_events NUMBER(19),
        num_inserts NUMBER(19),
        num_updates NUMBER(19),
        num_deletes NUMBER(19),
        PRIMARY KEY (migration_uuid, table_name, channel_no)
    );
