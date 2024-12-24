--view ddl having WITH CHECK OPTION
CREATE VIEW v1 AS SELECT * FROM t1 WHERE a < 2
WITH CHECK OPTION;

--view ddl having WITH LOCAL CHECK OPTION
CREATE VIEW v2 AS SELECT * FROM t1 WHERE a < 2
WITH LOCAL CHECK OPTION;

--dropping multiple objects
DROP VIEW IF EXISTS view1,view2,view3;


--JSON_ARRAYAGG() not available
CREATE OR REPLACE view test AS (
                            select x , JSON_ARRAYAGG(trunc(b, 2) order by t desc) as agg
                            FROM test1
                            where t = '1DAY' group by x
                            );
CREATE VIEW view_name AS SELECT * from test_arr_enum;
--Unsupported PG Syntax
--For this case we will have two issues reported one by regex and other by Unsupported PG syntax with error msg
ALTER VIEW view_name TO select * from test;

CREATE VIEW public.orders_view AS
 SELECT orders.order_id,
    orders.customer_name,
    orders.product_name,
    orders.quantity,
    orders.price,
    XMLELEMENT(NAME "OrderDetails", XMLELEMENT(NAME "Customer", orders.customer_name), XMLELEMENT(NAME "Product", orders.product_name), XMLELEMENT(NAME "Quantity", orders.quantity), XMLELEMENT(NAME "TotalPrice", (orders.price * (orders.quantity)::numeric))) AS order_xml,
    XMLCONCAT(XMLELEMENT(NAME "Customer", orders.customer_name), XMLELEMENT(NAME "Product", orders.product_name)) AS summary_xml,
    pg_try_advisory_lock((hashtext((orders.customer_name || orders.product_name)))::bigint) AS lock_acquired,
    orders.ctid AS row_ctid,
    orders.xmin AS transaction_id
   FROM public.orders;

CREATE VIEW public.my_films_view AS 
SELECT jt.* FROM
 my_films,
 JSON_TABLE ( js, '$.favorites[*]'
   COLUMNS (
    id FOR ORDINALITY,
    kind text PATH '$.kind',
    NESTED PATH '$.films[*]' COLUMNS (
      title text FORMAT JSON PATH '$.title' OMIT QUOTES,
      director text PATH '$.director' KEEP QUOTES))) AS jt;