DO $$
BEGIN
    IF length(:'schema_list') = 0 THEN
        RAISE NOTICE 'No schema list provided. Prompting user...';
        \prompt 'Enter a comma-separated list of schema names: ' schema_list
    END IF;
END $$;

-- Define the filenames dynamically
\set table_sizes_file 'sharding__table-sizes.csv'
\set table_iops_file 'sharding__table-iops.csv'
\set table_row_counts_file 'sharding__table-row-counts.csv'

-- Define and execute the command to collect stats about size of tables and save to CSV file
\set sizes_command '\\copy (
    SELECT
        n.nspname AS schema_name,
        c.relname AS table_name,
        pg_table_size(c.oid) AS table_size
    FROM 
        pg_class c
    JOIN 
        pg_namespace n ON n.oid = c.relnamespace
    WHERE 
        c.oid > 16384
        AND c.relkind IN ('r', 'i')
        AND n.nspname = ANY(ARRAY[:'schema_list']::TEXT[])
    ORDER BY 
        pg_table_size(c.oid) DESC
) to :'table_sizes_file' WITH CSV HEADER'
:sizes_command

-- Execute the command to collect stats about size of tables and save to CSV file
:sizes_command

-- Define and execute the command to collect stats about iops of tables and save to CSV file
\set iops_command '\\copy (
    SELECT
        schemaname AS schema_name,
        relname AS table_name,
        seq_tup_read AS seq_reads,
        n_tup_ins + n_tup_upd + n_tup_del AS row_writes
    FROM
        pg_stat_user_tables
    WHERE
        schemaname = ANY(ARRAY[:'schema_list']::TEXT[])
    ORDER BY
        seq_tup_read DESC
) to :'table_iops_file' WITH CSV HEADER'
:iops_command

-- Execute the command to collect stats about iops of tables and save to CSV file
:iops_command

-- Define and execute the command to collect stats about row count of tables and save to CSV file
\set row_counts_command '\\copy (
    SELECT
        table_schema AS schema_name,
        table_name,
        table_rows AS row_count
    FROM 
        information_schema.tables 
    WHERE 
        table_schema = ANY(ARRAY[:'schema_list']::TEXT[]) 
        AND table_type = 'BASE TABLE'
) to :'table_row_counts_file' WITH CSV HEADER'
:row_counts_command

-- Execute the command to collect stats about row count of tables and save to CSV file
:row_counts_command
