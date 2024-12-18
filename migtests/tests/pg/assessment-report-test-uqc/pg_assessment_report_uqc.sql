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