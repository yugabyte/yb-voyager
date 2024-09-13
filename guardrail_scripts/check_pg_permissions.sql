--- How to use the script:
--- Run the script with psql command line tool, passing the necessary parameters:
--- psql -h <host> -d <database> -U <username> -v db_user='<db_user>' -v schema_list='<schema_list>' -v db_instance_type='<db_instance_type>' -v is_live_migration=<is_live_migration> -v is_live_migration_fall_back=<is_live_migration_fall_back> -f <path_to_script> > <path_to_output_file>
--- Example:
--- psql -h <host> -d <database> -U <username> -v db_user='ybvoyager' -v schema_list='schema1,public,schema2' -v db_instance_type='rds' -v is_live_migration=1 -v is_live_migration_fall_back=1 -f /home/ubuntu/script.sql > /home/ubuntu/script_output.txt
--- Parameters:
--- <host>: The hostname of the PostgreSQL server.
--- <database>: The name of the database to connect to.
--- <username>: The username to connect with.
--- <db_user>: The database user for which permissions are being checked.
--- <schema_list>: A comma-separated list of schemas to check permissions on. Example 'schema1,public,schema2'.
--- <db_instance_type>: The type of the database instance. Options are 'rds' or 'standalone'.
--- <is_live_migration>: A flag indicating if this is a live migration (1 for true, 0 for false). If set to 0 then the script will check for permissions for an offline migration.
--- <is_live_migration_fall_back>: A flag indicating if this is a live migration with fallback (1 for true, 0 for false). If set to 0 then the script will detect permissions for live migration with fall-forward. Should only be set to 1 when is_live_migration is also set to 1.
--- <path_to_script>: The path to the SQL script to be executed.
--- <path_to_output_file>: The path where the output of the script will be saved.

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

-- Check if db_instance_type is provided
\if :{?db_instance_type}
    \echo 'Database instance type (db_instance_type) is provided: ':db_instance_type
\else
    \echo 'Error: Database instance type (db_instance_type) is not provided!'
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


--- Check USAGE permission on schemas
\echo '--- USAGE Permission Results ---'
SELECT
    schema_name,
    'USAGE' AS permission,
    CASE WHEN has_schema_privilege(:'db_user', schema_name, 'USAGE') THEN 'Granted' ELSE 'Missing' END AS status
FROM information_schema.schemata;

-- Check SELECT permission on tables
\echo '--- SELECT Table Permission Results ---'
\echo 'Note that on RDS, you may get "Permission Denied" errors for pg_catalog tables (such as pg_statistic). These errors do not affect the migration and can be ignored if they show permission as "Missing".'
SELECT
    table_schema AS schema_name,
    table_name,
    CASE WHEN has_table_privilege(:'db_user', table_schema || '.' || table_name, 'SELECT') THEN 'Granted' ELSE 'Missing' END AS status
FROM information_schema.tables
WHERE table_type = 'BASE TABLE';

-- Check SELECT permission on sequences
\echo '--- SELECT Sequence Permission Results ---'
SELECT
    sequence_schema AS schema_name,
    sequence_name,
    CASE WHEN has_sequence_privilege(:'db_user', sequence_schema || '.' || sequence_name, 'SELECT') THEN 'Granted' ELSE 'Missing' END AS status
FROM information_schema.sequences;

-- Check wal_level
\if :is_live_migration
    \echo '--- Wal Level Check ---'
    SELECT
        CASE WHEN current_setting('wal_level') <> 'logical'
             THEN 'wal_level is set to ' || current_setting('wal_level') || ', it should be logical.'
             ELSE 'wal_level is correct.'
        END AS wal_level_status;

-- Check replica identity for tables in the given schemas
    \echo '--- Replica Identity Results ---'
    SELECT
        n.nspname AS schema_name,
        c.relname AS table_name,
        c.relreplident AS replica_identity,
        CASE WHEN c.relreplident <> 'f' THEN 'Missing FULL' ELSE 'Correct' END AS status
    FROM pg_class c
    JOIN pg_namespace n ON c.relnamespace = n.oid
    WHERE n.nspname = ANY(string_to_array(:'schema_list', ','))
      AND c.relkind = 'r';

-- Check specific user permission based on db_instance_type
    \echo '--- User Permission Check based on db_instance_type ---'
    SELECT
        :'db_user' AS user_name,
        CASE 
            WHEN :'db_instance_type' = 'rds' THEN 'rds_replication'
            WHEN :'db_instance_type' = 'standalone' THEN 'REPLICATION'
            ELSE 'Unknown'
        END AS permission,
        CASE 
            WHEN :'db_instance_type' = 'rds' AND EXISTS (
                SELECT 1 
                FROM pg_roles 
                WHERE rolname = :'db_user' 
                  AND pg_has_role(:'db_user', 'rds_replication', 'USAGE')
            ) THEN 'Granted'
            WHEN :'db_instance_type' = 'standalone' AND EXISTS (
                SELECT 1 
                FROM pg_roles 
                WHERE rolname = :'db_user' 
                  AND rolreplication
            ) THEN 'Granted'
            ELSE 'Missing'
        END AS status;

-- Check if user has create permission on the db
    \echo '--- CREATE Permission on Source DB ---'
    SELECT
        :'db_user' AS user_name,
        'CREATE_on_source_db' AS permission,
        CASE WHEN EXISTS (
            SELECT 1
            FROM pg_database
            WHERE datname = current_database()
              AND has_database_privilege(:'db_user', datname, 'CREATE')
        ) THEN 'Granted' ELSE 'Missing' END AS status;


-- Check table ownership and whether db_user has ownership
\echo '--- Table Ownership Check ---'
WITH table_ownership AS (
    SELECT
        n.nspname AS schema_name,
        c.relname AS table_name,
        pg_get_userbyid(c.relowner) AS owner_name
    FROM pg_class c
    JOIN pg_namespace n ON c.relnamespace = n.oid
    WHERE c.relkind = 'r' -- 'r' indicates a table
      AND n.nspname = ANY(string_to_array(:'schema_list', ','))
)
SELECT
    schema_name,
    table_name,
    owner_name,
    CASE
        WHEN owner_name = :'db_user' THEN 'db_user has ownership over the table.'
        WHEN EXISTS (
            SELECT 1
            FROM pg_roles r
            JOIN pg_auth_members am ON r.oid = am.roleid
            JOIN pg_roles ur ON am.member = ur.oid
            WHERE r.rolname = owner_name
              AND ur.rolname = :'db_user'
        ) THEN 'db_user is a member of the group that owns the table.'
        ELSE 'db_user does not have ownership over the table.'
    END AS ownership_status
FROM table_ownership;

-- Check if db_user has SELECT, INSERT, UPDATE, DELETE permissions on schemas
    \if :is_live_migration_fall_back
        \echo '--- Check GRANT SELECT, INSERT, UPDATE, DELETE Permissions on Schemas ---'
        SELECT
            schemaname AS schema_name,
            tablename AS table_name,
            CASE
            WHEN has_table_privilege(:'db_user', schemaname || '.' || tablename, 'SELECT, INSERT, UPDATE, DELETE') THEN 'Granted'
            ELSE 'Missing'
            END AS grant_status
        FROM pg_tables
        WHERE schemaname = ANY(string_to_array(:'schema_list', ','));
    \endif
\endif

