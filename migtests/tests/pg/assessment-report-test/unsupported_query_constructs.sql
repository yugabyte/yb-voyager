-- Unsupported Query Constructs 

drop extension if exists pg_stat_statements;

create extension pg_stat_statements;

SELECT pg_stat_statements_reset();

SELECT * FROM pg_stat_statements;

-- System Columns 

SELECT ctid, tableoid, xmin, xmax, cmin, cmax
FROM employees2;

-- XML Functions

SELECT table_to_xml('employees2', true, false, '');

SELECT xmlparse(document '<data><item>A</item></data>') as xmldata;

SELECT xmlforest(first_name AS element1, last_name AS element2) FROM employees2;

SELECT xmlelement(name root, xmlelement(name child, 'value'));

SELECT xml_is_well_formed('<root><child>value</child></root>');

-- Not Reported Currently

-- SELECT *
-- FROM xmltable(
--     '/employees/employee'
--     PASSING '<employees><employee><name>John</name></employee></employees>'
--     COLUMNS 
--         name TEXT PATH 'name'
-- );

-- Advisory Locks

SELECT pg_advisory_lock(1,2); 
SELECT pg_advisory_unlock(1,2); 
SELECT pg_advisory_xact_lock(1,2); 
SELECT pg_advisory_unlock_all();