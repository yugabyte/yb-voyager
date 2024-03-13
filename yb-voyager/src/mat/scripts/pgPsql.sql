/*
Copyright (c) YugabyteDB, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

DO $$
BEGIN
    IF length(:'schema_list') = 0 THEN
        RAISE NOTICE 'No schema list provided. Prompting user...';
        \prompt 'Enter a comma-separated list of schema names: ' schema_list
    END IF;
END $$;

\echo 'collect stats about size of tables'
-- Create a temporary table to store the query results
CREATE TEMP TABLE temp_table AS
SELECT
    c.oid,
    n.nspname AS schema_name,
    c.relname AS table_name,
    pg_catalog.pg_table_size(c.oid) AS table_size
FROM 
    pg_class c
JOIN 
    pg_namespace n ON n.oid = c.relnamespace
WHERE 
    c.oid > 16384
    AND c.relkind IN ('r', 'i')
    AND n.nspname = ANY(ARRAY[string_to_array(:'schema_list', ',')])
ORDER BY 
    pg_catalog.pg_table_size(c.oid) DESC;

-- TODO: handle storing the info(column names) with schema name(required in case of multi schema migration)
-- Output the contents of the temporary table to a CSV file
\copy temp_table TO 'sharding__table-sizes.csv' WITH CSV HEADER;
-- Drop the temporary table
DROP TABLE temp_table;


\echo 'collect stats about iops of tables'
-- similar gather the IOPS for all the tables in these schemas
CREATE TEMP TABLE temp_table_usage AS
SELECT
    relname,
    seq_tup_read AS seq_reads,
    n_tup_ins + n_tup_upd + n_tup_del AS row_writes
FROM
    pg_stat_user_tables
WHERE
    schemaname = ANY(ARRAY[string_to_array(:'schema_list', ',')])
ORDER BY
    seq_tup_read DESC;

-- Now you can use the temporary table to fetch the data
\copy temp_table_usage TO 'sharding__table-iops.csv' WITH CSV HEADER;
-- Drop the temporary table
DROP TABLE temp_table_usage;


-- TODO: finalize the query, approx count or exact count(any optimization also if possible)
-- similar gather the row count for all the tables in these schemas
\echo 'collect stats about row count of tables'
CREATE OR REPLACE FUNCTION get_table_row_counts(VARIADIC schema_names TEXT[])
RETURNS TABLE (schema_name TEXT, tbl_name TEXT, row_count INTEGER) AS
$$
DECLARE
    table_record RECORD;
BEGIN
    RAISE NOTICE 'schema_names: %', schema_names;
    FOREACH schema_name IN ARRAY schema_names LOOP
        FOR table_record IN 
            SELECT table_name 
            FROM information_schema.tables 
            WHERE table_schema = schema_name AND table_type = 'BASE TABLE' 
        LOOP
            EXECUTE format(
                'SELECT %L::TEXT AS schema_name, %L::TEXT AS tbl_name, count(*) AS row_count FROM %I.%I', 
                schema_name,
                table_record.table_name, 
                schema_name, 
                table_record.table_name
            ) INTO schema_name, tbl_name, row_count;
            
            RETURN NEXT;
        END LOOP;
    END LOOP;
END
$$ LANGUAGE PLPGSQL;

CREATE TEMP TABLE temp_table AS
    SELECT * FROM get_table_row_counts(VARIADIC ARRAY[string_to_array(:'schema_list', ',')]);
\copy temp_table TO 'sharding__table-row-counts.csv' WITH CSV HEADER;

DROP TABLE temp_table;
DROP FUNCTION get_table_row_counts(VARIADIC schema_names TEXT[]);


-- TODO: Test and handle(if required) the queries for case-sensitive and reserved keywords cases