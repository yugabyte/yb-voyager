-- Unsupported Query Constructs 
DROP EXTENSION IF EXISTS pg_stat_statements;
CREATE EXTENSION pg_stat_statements;
SELECT pg_stat_statements_reset();
SELECT * FROM pg_stat_statements;


-- 1) System columns usage (public schema)
-- Using xmin is considered a system column, which is unsupported.
SELECT name, xmin FROM public.employees WHERE id = 1;

-- 2) Advisory locks (hr schema)
-- pg_advisory_lock is an unsupported advisory lock function.
SELECT hr.departments.department_name, pg_advisory_lock(hr.departments.department_id)
FROM hr.departments
WHERE department_name = 'Engineering';

-- 3) XML function usage (public schema)
-- xmlelement is an XML-related function considered unsupported.
SELECT xmlelement(name "employee_data", name) AS emp_xml
FROM public.employees;

-- 4) Advisory locks (analytics schema)
-- Also uses pg_advisory_lock for another unsupported construct scenario.
SELECT metric_name, pg_advisory_lock(metric_id)
FROM analytics.metrics
WHERE metric_value > 0.02;