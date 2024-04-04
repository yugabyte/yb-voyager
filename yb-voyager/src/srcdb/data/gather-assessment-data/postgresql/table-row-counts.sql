-- Create a temporary table to store the row counts
CREATE TEMP TABLE temp_table (
    schema_name TEXT,
    table_name TEXT,
    row_count INTEGER
);

-- Set the schema_list variable
SET vars.schema_list TO :'schema_list';

DO $$
DECLARE
    schema_list TEXT[] := string_to_array(current_setting('vars.schema_list'), ',');
    schema_name TEXT;
    table_record RECORD;
    sql_statement TEXT;
BEGIN
    -- Iterate over each schema in the schema list
    FOREACH schema_name IN ARRAY schema_list LOOP
        -- Iterate over each table in the current schema
        FOR table_record IN 
            SELECT table_schema, table_name
            FROM information_schema.tables 
            WHERE table_schema = schema_name AND table_type = 'BASE TABLE' 
        LOOP
            -- Execute dynamic SQL to insert row count data into temp_table
            EXECUTE format(
                'INSERT INTO temp_table (schema_name, table_name, row_count) SELECT %L, %L, COUNT(*) FROM %I.%I',
                table_record.table_schema,
                table_record.table_name,
                table_record.table_schema,
                table_record.table_name
            );
        END LOOP;
    END LOOP;
END $$;

-- Export the data from the temporary table to a CSV file
\copy temp_table TO 'table-row-counts.csv' WITH CSV HEADER;

-- Drop the temporary table
DROP TABLE temp_table;
