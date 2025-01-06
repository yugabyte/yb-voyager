-- Create multiple schemas
CREATE SCHEMA public;
CREATE SCHEMA sales;
CREATE SCHEMA hr;
CREATE SCHEMA analytics;

-- Create tables in each schema
CREATE TABLE public.employees (
    id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT,
    department_id INT
);

CREATE TABLE sales.orders (
    order_id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    customer_id INT,
    amount NUMERIC
);

CREATE TABLE hr.departments (
    department_id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    department_name TEXT,
    location TEXT
);

CREATE TABLE analytics.metrics (
    metric_id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    metric_name TEXT,
    metric_value NUMERIC
);

-- Create a view in public schema
CREATE VIEW public.employee_view AS
SELECT e.id, e.name, d.department_name
FROM public.employees e
JOIN hr.departments d ON e.department_id = d.department_id;


-- Insert some sample data
INSERT INTO hr.departments (department_name, location) VALUES ('Engineering', 'Building A');
INSERT INTO hr.departments (department_name, location) VALUES ('Sales', 'Building B');
INSERT INTO public.employees (name, department_id) VALUES ('Alice', 1), ('Bob', 1), ('Charlie', 2);
INSERT INTO sales.orders (customer_id, amount) VALUES (101, 500.00), (102, 1200.00);
INSERT INTO analytics.metrics (metric_name, metric_value) VALUES ('ConversionRate', 0.023), ('ChurnRate', 0.05);

create view sales.employ_depart_view AS  SELECT
        any_value(name) AS any_employee
    FROM employees;

CREATE TABLE sales.test_json_chk (
    id int,
    name text,
    email text,
    active text,
    data jsonb,
    CHECK (data['key']<>'{}')
);

INSERT INTO sales.test_json_chk (id, name, email, active, data)
VALUES (1, 'John Doe', 'john@example.com', 'Y', jsonb_build_object('key', 'value',  'name', 'John Doe', 'active', 'Y'));

INSERT INTO sales.test_json_chk (id, name, email, active, data)
VALUES (2, 'Jane Smith', 'jane@example.com', 'N', jsonb_build_object('key', 'value',  'name', 'Jane Smith', 'active', 'N'));

CREATE OR REPLACE FUNCTION sales.get_user_info(user_id INT)
RETURNS JSONB AS $$
BEGIN
    PERFORM 
        data,
        data['name'] AS name, 
        (data['active']) as active
    FROM sales.test_json_chk;

    RETURN (
        SELECT jsonb_build_object(
            'id', id,
            'name', name,
            'email', email,
            'active', active
        )
        FROM sales.test_json_chk
        WHERE id = user_id
    );
END;
$$ LANGUAGE plpgsql;
CREATE TABLE sales.events (
    id int PRIMARY KEY,
    event_range daterange
);

-- Insert some ranges
INSERT INTO sales.events (id, event_range) VALUES
    (1,'[2024-01-01, 2024-01-10]'::daterange),
    (2,'[2024-01-05, 2024-01-15]'::daterange),
    (3,'[2024-01-20, 2024-01-25]'::daterange);

CREATE VIEW sales.event_analysis_view AS
SELECT
    range_agg(event_range) AS all_event_ranges
FROM
    sales.events;

CREATE VIEW sales.event_analysis_view2 AS
SELECT
    range_intersect_agg(event_range) AS overlapping_range
FROM
    sales.events;

-- PG 16 and above feature 
CREATE TABLE sales.json_data (
    id int PRIMARY KEY,
    array_column TEXT CHECK (array_column IS JSON ARRAY),
    unique_keys_column TEXT CHECK (unique_keys_column IS JSON WITH UNIQUE KEYS)
);

INSERT INTO public.json_data (
    id, data_column, object_column, array_column, scalar_column, unique_keys_column
) VALUES (
    1, '{"key": "value"}',
    2, '{"name": "John", "age": 30}',
    3, '[1, 2, 3, 4]',
    4, '"hello"',
    5, '{"uniqueKey1": "value1", "uniqueKey2": "value2"}'
);
