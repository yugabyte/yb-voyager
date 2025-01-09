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
