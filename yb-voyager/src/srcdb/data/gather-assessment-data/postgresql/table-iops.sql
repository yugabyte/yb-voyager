-- similar gather the IOPS for all the tables in these schemas
CREATE TEMP TABLE temp_table AS
SELECT
    schemaname as schema_name,
    relname as table_name,
    seq_tup_read AS seq_reads,
    n_tup_ins + n_tup_upd + n_tup_del AS row_writes
FROM
    pg_stat_user_tables
WHERE
    schemaname = ANY(ARRAY[string_to_array(:'schema_list', ',')])
ORDER BY
    seq_tup_read DESC;

-- Now you can use the temporary table to fetch the data
\copy temp_table TO 'table-iops.csv' WITH CSV HEADER;

-- Drop the temporary table
DROP TABLE temp_table;