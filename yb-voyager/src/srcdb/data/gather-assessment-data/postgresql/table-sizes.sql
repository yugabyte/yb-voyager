-- Create a temporary table to store the query results
CREATE TEMP TABLE temp_table AS
SELECT
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
\copy temp_table TO 'table-sizes.csv' WITH CSV HEADER;
-- Drop the temporary table
DROP TABLE temp_table;
