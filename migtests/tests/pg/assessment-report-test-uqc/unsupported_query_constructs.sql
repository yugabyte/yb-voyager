-- Unsupported Query Constructs 
DROP EXTENSION IF EXISTS pg_stat_statements;
CREATE EXTENSION pg_stat_statements;
SELECT pg_stat_statements_reset();
SELECT * FROM pg_stat_statements;


-- 1) System columns usage (public schema)
SELECT name, xmin FROM public.employees WHERE id = 1;

-- 2) Advisory locks (hr schema)
SELECT hr.departments.department_name, pg_advisory_lock(hr.departments.department_id)
FROM hr.departments
WHERE department_name = 'Engineering';

-- 3) XML function usage (public schema)
SELECT xmlelement(name "employee_data", name) AS emp_xml
FROM public.employees;

-- 4) Advisory locks (analytics schema)
SELECT metric_name, pg_advisory_lock(metric_id)
FROM analytics.metrics
WHERE metric_value > 0.02;

-- Aggregate functions UQC NOT REPORTING as it need PG16 upgarde in pipeline from PG15
SELECT
        any_value(name) AS any_employee
    FROM employees;

MERGE INTO sales.customer_account ca
USING sales.recent_transactions t      
ON t.customer_id = ca.customer_id
WHEN MATCHED THEN
  UPDATE SET balance = balance + transaction_value
WHEN NOT MATCHED THEN
  INSERT (customer_id, balance)
  VALUES (t.customer_id, t.transaction_value);

select * from sales.customer_account ;
SELECT (sales.get_user_info(2))['name'] AS user_info;

SELECT (jsonb_build_object('name', 'PostgreSQL', 'version', 17, 'open_source', TRUE) || '{"key": "value2"}')['name'] AS json_obj;

SELECT 
    data,
    data['name'] AS name, 
    (data['active']) as active
FROM sales.test_json_chk;

SELECT ('{"a": { "b": {"c": "1"}}}' :: jsonb)['a']['b'] as b;
--PG15
SELECT range_agg(event_range) AS union_of_ranges
FROM sales.events;

SELECT range_intersect_agg(event_range) AS intersection_of_ranges
FROM sales.events;

-- -- PG 16 and above feature
SELECT * 
FROM sales.json_data
WHERE array_column IS JSON ARRAY;

-- PG 15 supports the non-decimal integer literals e.g. hexadecimal, octal, binary
-- but this won't be reported as the PGSS will change the constant integers to parameters - $1, $2...
SELECT 1234, 0x4D2 as hex, 0o2322 as octal, 0b10011010010 as binary;

SELECT 5678901234, 0x1527D27F2, 0o52237223762, 0b101010010011111010010011111110010 \gdesc

WITH w AS NOT MATERIALIZED (                                                               
    SELECT * FROM sales.big_table
)           
SELECT * FROM w AS w1 JOIN w AS w2 ON w1.key = w2.ref
WHERE w2.key = 123;

WITH w AS MATERIALIZED (                                                               
    SELECT * FROM sales.big_table
)           
SELECT * FROM w AS w1 JOIN w AS w2 ON w1.key = w2.ref
WHERE w2.key = 123;

WITH w AS (                                                               
    SELECT * FROM sales.big_table
)           
SELECT * FROM w AS w1 JOIN w AS w2 ON w1.key = w2.ref
WHERE w2.key = 123;

CREATE DATABASE strategy_example
    WITH STRATEGY = 'wal_log';

DROP DATABASE strategy_example;

LISTEN my_table_changes;
