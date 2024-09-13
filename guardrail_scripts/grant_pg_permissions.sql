--- How to use the script:
-- Run the script with psql command line tool, passing the necessary parameters:
--- psql -h <host> -d <database> -U <username> -v db_user='<db_user>' -v schema_list='<schema_list>' -v replication_group='<replication_group>' -v original_owner_of_tables='<original_owner_of_tables>' -v db_name='<db_name>' -v is_live_migration=<is_live_migration> -v is_live_migration_fall_back=<is_live_migration_fall_back> -f <path_to_script> 
--- Example:
--- psql -h <host> -d <database> -U <username> -v db_user='ybvoyager' -v schema_list='schema1,public,schema2' -v replication_group='replication_group' -v original_owner_of_tables='postgres' -v db_name='test_db' -v is_live_migration=1 -v is_live_migration_fall_back=0 -f /home/ubuntu/grant_pg_permissions.sql
--- Parameters:
--- <host>: The hostname of the PostgreSQL server.
--- <database>: The name of the database to connect to.
--- <username>: The username to connect with.
--- <db_user>: The database user for which permissions are being granted.
--- <schema_list>: A comma-separated list of schemas to grant permissions on. Example 'schema1,public,schema2'.
--- <replication_group>: The name of the replication group to be created.
--- <original_owner_of_tables>: The original owner of the tables to be added to the replication group.
--- <db_name>: The name of the database to grant CREATE permission on.
--- <is_live_migration>: A flag indicating if this is a live migration (1 for true, 0 for false). If set to 0 then the script will check for permissions for an offline migration.
--- <is_live_migration_fall_back>: A flag indicating if this is a live migration with fallback (1 for true, 0 for false). If set to 0 then the script will detect permissions for live migration with fall-forward. Should only be set to 1 when is_live_migration is also set to 1.

\echo '--- Checking Variables ---'

-- Check if db_user is provided
\if :{?db_user}
    \echo 'Database user (db_user) is provided: ':db_user
\else
    \echo 'Error: Database user (db_user) is not provided!'
    \q
\endif

-- Check if schema_list is provided
\if :{?schema_list}
    \echo 'Schema list (schema_list) is provided: ':schema_list
\else
    \echo 'Error: Schema list (schema_list) is not provided!'
    \q
\endif

-- Check if replication_group is provided
\if :{?replication_group}
    \echo 'Replication group (replication_group) is provided: ':replication_group
\else
    \echo 'Error: Replication group (replication_group) is not provided!'
    \q
\endif

-- Check if original_owner_of_tables is provided
\if :{?original_owner_of_tables}
    \echo 'Original owner of tables (original_owner_of_tables) is provided: ':original_owner_of_tables
\else
    \echo 'Error: Original owner of tables (original_owner_of_tables) is not provided!'
    \q
\endif

-- Check if db_name is provided
\if :{?db_name}
    \echo 'Database name (db_name) is provided: ':db_name
\else
    \echo 'Error: Database name (db_name) is not provided!'
    \q
\endif

-- Check if is_live_migration is provided
\if :{?is_live_migration}
    \echo 'Live migration flag (is_live_migration) is provided: ':is_live_migration
\else
    \echo 'Error: Live migration flag (is_live_migration) is not provided!'
    \q
\endif

-- Check if is_live_migration_fall_back is provided
\if :{?is_live_migration_fall_back}
    \echo 'Live migration fallback flag (is_live_migration_fall_back) is provided: ':is_live_migration_fall_back
\else
    \echo 'Error: Live migration fallback flag (is_live_migration_fall_back) is not provided!'
    \q
\endif

\o /dev/null
SET myvars.schema_list = :'schema_list';
SET myvars.replication_group = :'replication_group';
SET myvars.db_name = :'db_name';
SET myvars.db_user = :'db_user';
\o

-- Grant USAGE permission on all schemas to db_user
\echo '--- Granting USAGE Permission on Schemas ---'
SELECT 'GRANT USAGE ON SCHEMA ' || schema_name || ' TO ' || :'db_user' || ';'
FROM information_schema.schemata
\gexec

-- Grant SELECT permission on all tables in all schemas to db_user
\echo '--- Granting SELECT Permission on Tables ---'
\echo 'Note that on RDS, you may get "Permission Denied" errors for pg_catalog tables (such as pg_statistic). These errors do not affect the migration and can be ignored.'
SELECT 'GRANT SELECT ON ALL TABLES IN SCHEMA ' || schema_name || ' TO ' || :'db_user' || ';'
FROM information_schema.schemata
\gexec

-- Grant SELECT permission on all sequences in all schemas to db_user
\echo '--- Granting SELECT Permission on Sequences ---'
SELECT 'GRANT SELECT ON ALL SEQUENCES IN SCHEMA ' || schema_name || ' TO ' || :'db_user' || ';'
FROM information_schema.schemata
\gexec

-- Change the replica identity of all tables to FULL
\if :is_live_migration
    \echo '--- Changing Replica Identity to FULL ---'
    DO $$
    DECLARE
    r RECORD;
    BEGIN
    -- Change the replica identity of all tables to FULL
    FOR r IN (SELECT table_schema, '"' || table_name || '"' AS t_name  
                FROM information_schema.tables 
                WHERE table_schema = ANY(string_to_array(current_setting('myvars.schema_list'), ','))
                AND table_type = 'BASE TABLE')
    LOOP
        EXECUTE 'ALTER TABLE ' || r.table_schema || '.' || r.t_name || ' REPLICA IDENTITY FULL';
    END LOOP;
    END $$;

    -- Create a replication group
    \echo '--- Creating Replication Group ---'
    CREATE ROLE :replication_group;

    -- Add the original owner of the tables to the group
    \echo '--- Adding Original Owner to Replication Group ---'
    GRANT :replication_group TO :original_owner_of_tables;

    -- Add the user ybvoyager to the replication group
    \echo '--- Adding User to Replication Group ---'
    GRANT :replication_group TO :db_user;

    -- Transfer ownership of the tables to the replication group
    \echo '--- Transferring Ownership of Tables to Replication Group ---'
    DO $$
    DECLARE
        r RECORD;
    BEGIN
        FOR r IN
            SELECT table_schema, '"' || table_name || '"' AS t_name
            FROM information_schema.tables
            WHERE table_schema = ANY(string_to_array(current_setting('myvars.schema_list'), ',')::text[])
        LOOP
            EXECUTE 'ALTER TABLE ' || r.table_schema || '.' || r.t_name || ' OWNER TO ' || current_setting('myvars.replication_group');
        END LOOP;
    END $$;

    -- Grant CREATE permission on the specified database to the specified user
    \echo '--- Granting CREATE Permission on Database ---'
    DO $$
    BEGIN
    EXECUTE format('GRANT CREATE ON DATABASE %I TO %I;', current_setting('myvars.db_name'), current_setting('myvars.db_user'));
    END $$;

    -- Grant SELECT, INSERT, UPDATE, DELETE on all tables in specified schemas
    \if :is_live_migration_fall_back
        \echo '--- Granting SELECT, INSERT, UPDATE, DELETE on All Tables in Specified Schemas ---'
        SELECT 
            'GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA ' || schema_name || ' TO ' || :'db_user' || ';'
        FROM 
            information_schema.schemata
        WHERE 
            schema_name = ANY(string_to_array(:'schema_list', ','))
        \gexec
    \endif
\endif



