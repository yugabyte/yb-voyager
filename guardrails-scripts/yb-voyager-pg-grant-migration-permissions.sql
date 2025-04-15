--- How to use the script:
-- Run the script with psql command line tool, passing the necessary parameters:
--- psql -h <host> -d <database> -U <username> -v voyager_user='<voyager_user>' -v schema_list='<schema_list>' -v is_live_migration=<is_live_migration> -v is_live_migration_fall_back=<is_live_migration_fall_back> -v replication_group='<replication_group>' -f <path_to_script> 
--- Example:
--- psql -h <host> -d <database> -U <username> -v voyager_user='ybvoyager' -v schema_list='schema1,public,schema2' -v is_live_migration=1 -v is_live_migration_fall_back=0 -v replication_group='replication_group' -f /home/ubuntu/yb-voyager-pg-grant-migration-permissions.sql
--- Parameters:
--- <host>: The hostname of the PostgreSQL server.
--- <database>: The name of the database to connect to.
--- <username>: The username to connect with.
--- <voyager_user>: The database user for which permissions are being granted.
--- <schema_list>: A comma-separated list of schemas to grant permissions on. Example 'schema1,public,schema2'.
--- <is_live_migration>: A flag indicating if this is a live migration (1 for true, 0 for false). If set to 0 then the script will check for permissions for an offline migration.
--- <is_live_migration_fall_back>: A flag indicating if this is a live migration with fallback (1 for true, 0 for false). If set to 0 then the script will detect permissions for live migration with fall-forward. Should only be set to 1 when is_live_migration is also set to 1. Does not need to be provided unless is_live_migration is set to 1.
--- <replication_group>: The name of the replication group to be created. Not needed for offline migration.

\echo ''
\echo '--- Checking Variables ---'

-- Check if voyager_user is provided
\if :{?voyager_user}
    \echo 'Voyager user is provided: ':voyager_user
\else
    \echo 'Error: voyager_user flag is not provided!'
    \q
\endif

-- Check if schema_list is provided
\if :{?schema_list}
    \echo 'Schema list is provided: ':schema_list
\else
    \echo 'Error: schema_list flag is not provided!'
    \q
\endif

-- Check if is_live_migration is provided
\if :{?is_live_migration}
    \echo 'Live migration flag is provided: ':is_live_migration
\else
    \echo 'Error: is_live_migration flag is not provided!'
    \q
\endif

-- If live migration is enabled, then is_live_migration_fall_back, replication_group should be provided
\if :is_live_migration

    -- Check if is_live_migration_fall_back is provided
    \if :{?is_live_migration_fall_back}
        \echo 'Live migration fallback flag is provided: ':is_live_migration_fall_back
    \else
        \echo 'Error: is_live_migration_fall_back flag is not provided!'
        \q
    \endif

    -- Check if replication_group is provided
    \if :{?replication_group}
        \echo 'Replication group is provided: ':replication_group
    \else
        \echo 'Error: replication_group flag is not provided!'
        \q
    \endif
\endif

-- If live migration fallback is provided and enabled, then is_live_migration should be enabled
\if :{?is_live_migration_fall_back}
    \if :is_live_migration_fall_back
        \if :is_live_migration
            \echo 'Live migration and fallback flags are both enabled'
        \else
            \echo 'Error: is_live_migration_fall_back is not enabled and live migration fallback flag is enabled!'
            \q
        \endif
    \endif
\endif

\o /dev/null
SET myvars.schema_list = :'schema_list';

\if :is_live_migration
    SET myvars.replication_group = :'replication_group';
\endif

SET myvars.voyager_user = :'voyager_user';
\o

\echo ''
\echo 'Current database: ' :DBNAME
\echo ''
\echo 'Note that on RDS, you may get "Permission Denied" errors for pg_catalog tables (such as pg_statistic). These errors do not affect the migration and can be ignored.'
\echo ''

-- Grant USAGE permission on all schemas to voyager_user
\echo '--- Granting USAGE Permission on Schemas ---'
SELECT 'GRANT USAGE ON SCHEMA ' || schema_name || ' TO ' || :'voyager_user' || ';'
FROM information_schema.schemata
\gexec

-- Grant SELECT permission on all tables in all schemas to voyager_user
\echo ''
\echo '--- Granting SELECT Permission on Tables ---'
SELECT 'GRANT SELECT ON ALL TABLES IN SCHEMA ' || schema_name || ' TO ' || :'voyager_user' || ';'
FROM information_schema.schemata
\gexec

-- Grant SELECT permission on all sequences in all schemas to voyager_user
\echo ''
\echo '--- Granting SELECT Permission on Sequences ---'
SELECT 'GRANT SELECT ON ALL SEQUENCES IN SCHEMA ' || schema_name || ' TO ' || :'voyager_user' || ';'
FROM information_schema.schemata
\gexec

-- Grant READ on the pg_stat_statments for Assessing the source 
\echo ''
\echo '--- Granting READ Permission on the pg_stat_statments for Assessing the source database'
GRANT pg_read_all_stats to :voyager_user;


-- Change the replica identity of all tables to FULL
\if :is_live_migration
    \echo ''
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

    -- Grant replication permissions to the user
    \echo ''
    \echo '--- Granting Replication Permissions ---'
    WITH is_rds AS (
        SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'rds_superuser') AS db_is_rds
    )
    SELECT 
        CASE 
            WHEN db_is_rds THEN 
                'GRANT rds_replication TO ' || :'voyager_user' || ';'
            ELSE 
                'ALTER USER ' || :'voyager_user' || ' REPLICATION;'
        END AS permission_command
    FROM is_rds
    \gexec

    -- Create a replication group
    \echo ''
    \echo '--- Creating Replication Group ---'
    CREATE ROLE :replication_group;

    -- Prompt the user to let them know that ownership of the tables will be transferred to the replication group and proceed only if they confirm
    \echo ''
    \echo 'This script will transfer ownership of all tables in the specified schemas to the specified replication group. The migration user and the original owner of the tables will be added to the replication group.'
    -- Only 'yes' or 'no' are valid inputs. 'y' and 'n' are not valid inputs.
    \prompt 'Proceed with the transfer of ownership? (yes/no): ' proceed

    \if :proceed
        \echo 'Proceeding with the transfer of ownership...'
    \else
        \echo 'Aborting the script.'
        \q
    \endif

    -- Add the original owner of the tables to the group
    \echo ''
    \echo '--- Adding Original Owner to Replication Group ---'
    DO $$
    DECLARE
        tableowner TEXT;
        schema_list TEXT[] := string_to_array(current_setting('myvars.schema_list'), ',');  -- Convert the schema list to an array
        replication_group TEXT := current_setting('myvars.replication_group');  -- Get the replication group from settings
    BEGIN
        -- Generate the GRANT statements and execute them dynamically
        FOR tableowner IN
            SELECT DISTINCT t.tableowner
            FROM pg_catalog.pg_tables t
            WHERE t.schemaname = ANY (schema_list)  -- Use the schema_list variable
            AND NOT EXISTS (
                SELECT 1
                FROM pg_roles r
                WHERE r.rolname = t.tableowner
                    AND pg_has_role(t.tableowner, replication_group, 'USAGE')  -- Use the replication_group variable
            )
        LOOP
            -- Display the GRANT statement
            RAISE NOTICE 'Granting role: GRANT % TO %;', replication_group, tableowner;

            -- Execute the GRANT statement
            EXECUTE format('GRANT %I TO %I;', replication_group, tableowner);
        END LOOP;
    END $$;

    -- Add the user ybvoyager to the replication group
    \echo ''
    \echo '--- Adding User to Replication Group ---'
    GRANT :replication_group TO :voyager_user;

    -- Transfer ownership of the tables to the replication group
    \echo ''
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
    \echo ''
    \echo '--- Granting CREATE Permission on Database ---'
    DO $$
    DECLARE
        db_name text;
    BEGIN
        -- Get the current database name
        db_name := current_database();

        -- Grant CREATE permission on the current database to the specified user
        EXECUTE format('GRANT CREATE ON DATABASE %I TO %I;', db_name, current_setting('myvars.voyager_user'));
    END $$;

    -- Grant SELECT, INSERT, UPDATE, DELETE on all tables and SELECT, UPDATE, USAGE on all sequences in specified schemas
    \if :is_live_migration_fall_back
        \echo ''
        \echo '--- Granting SELECT, INSERT, UPDATE, DELETE on All Tables in Specified Schemas ---'
        SELECT 
            'GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA ' || schema_name || ' TO ' || :'voyager_user' || ';'
        FROM 
            information_schema.schemata
        WHERE 
            schema_name = ANY(string_to_array(:'schema_list', ','))
        \gexec

        \echo ''
        \echo '--- Granting SELECT, UPDATE, USAGE on All Sequences in Specified Schemas ---'
        SELECT 
            'GRANT SELECT, UPDATE, USAGE ON ALL SEQUENCES IN SCHEMA ' || schema_name || ' TO ' || :'voyager_user' || ';'
        FROM 
            information_schema.schemata
        WHERE 
            schema_name = ANY(string_to_array(:'schema_list', ','))
        \gexec

        DO $$ 
        BEGIN 
        IF (substring((SELECT setting FROM pg_catalog.pg_settings WHERE name = 'server_version'), '^[0-9]+')::int >= 15) THEN
            RAISE NOTICE 'Granting set on PARAMETER session_replication_role TO %;', current_setting('myvars.voyager_user');
            EXECUTE format('GRANT SET ON PARAMETER session_replication_role TO %I;', current_setting('myvars.voyager_user'));
        END IF;
        END $$;

    \endif
\endif
