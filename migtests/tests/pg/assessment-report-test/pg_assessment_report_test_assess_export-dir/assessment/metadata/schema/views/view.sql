-- setting variables for current session
SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;


CREATE VIEW public.ordersentry_view AS
 SELECT order_id,
    customer_name,
    product_name,
    quantity,
    price,
    XMLELEMENT(NAME "OrderDetails", XMLELEMENT(NAME "Customer", customer_name), XMLELEMENT(NAME "Product", product_name), XMLELEMENT(NAME "Quantity", quantity), XMLELEMENT(NAME "TotalPrice", (price * (quantity)::numeric))) AS order_xml,
    XMLCONCAT(XMLELEMENT(NAME "Customer", customer_name), XMLELEMENT(NAME "Product", product_name)) AS summary_xml,
    pg_try_advisory_lock((hashtext((customer_name || product_name)))::bigint) AS lock_acquired,
    ctid AS row_ctid,
    xmin AS transaction_id
   FROM public.ordersentry;


CREATE VIEW public.sales_employees AS
 SELECT id,
    first_name,
    last_name,
    full_name
   FROM public.employees2
  WHERE ((department)::text = 'sales'::text)
  WITH CASCADED CHECK OPTION;


CREATE VIEW public.top_employees_view AS
 SELECT id,
    first_name,
    last_name,
    salary
   FROM ( SELECT employeesforview.id,
            employeesforview.first_name,
            employeesforview.last_name,
            employeesforview.salary
           FROM public.employeesforview
          ORDER BY employeesforview.salary DESC
         FETCH FIRST 2 ROWS WITH TIES) top_employees;


CREATE VIEW public.view_explicit_security_invoker WITH (security_invoker='true') AS
 SELECT employee_id,
    first_name
   FROM public.employees;


CREATE VIEW schema2.sales_employees AS
 SELECT id,
    first_name,
    last_name,
    full_name
   FROM schema2.employees2
  WHERE ((department)::text = 'sales'::text)
  WITH CASCADED CHECK OPTION;


CREATE VIEW schema2.top_employees_view AS
 SELECT id,
    first_name,
    last_name,
    salary
   FROM ( SELECT employeesforview.id,
            employeesforview.first_name,
            employeesforview.last_name,
            employeesforview.salary
           FROM schema2.employeesforview
          ORDER BY employeesforview.salary DESC
         FETCH FIRST 2 ROWS WITH TIES) top_employees;


CREATE VIEW test_views.v1 AS
 SELECT first_name,
    last_name
   FROM test_views.view_table1
  WHERE ((gender)::text = 'Female'::text);


CREATE VIEW test_views.v2 AS
 SELECT a.first_name,
    b.last_name
   FROM test_views.view_table1 a,
    test_views.view_table2 b
  WHERE (a.id = b.id);


CREATE VIEW test_views.v3 AS
 SELECT a.first_name,
    b.last_name
   FROM (test_views.view_table1 a
     JOIN test_views.view_table2 b USING (id));


CREATE VIEW test_views.v4 AS
 SELECT ((((first_name)::text || ' '::text) || (last_name)::text) || ';'::text) AS full_name
   FROM test_views.view_table1 a;


