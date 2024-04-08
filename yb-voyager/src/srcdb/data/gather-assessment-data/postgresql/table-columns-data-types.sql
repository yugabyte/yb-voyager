-- gathering schema_name, table_name, column_name, data_type
-- Create a temporary table to gather column information for all tables in the specified schemas
CREATE TEMP TABLE temp_column_info AS
SELECT
    table_schema AS schema_name,
    table_name,
    column_name,
    data_type
FROM
    information_schema.columns
WHERE
    table_schema = ANY(ARRAY[string_to_array(:'schema_list', ',')]);

-- Now you can use the temporary table to fetch and export the data
\copy temp_column_info TO 'table-columns-data-types.csv' WITH CSV HEADER;

-- Drop the temporary table after use
DROP TABLE temp_column_info;