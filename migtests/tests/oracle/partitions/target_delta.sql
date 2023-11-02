INSERT INTO ORDER_ITEMS_RANGE_PARTITIONED (order_datetime, customer_id, order_status, store_id)
VALUES (CURRENT_TIMESTAMP, 15, 'Processing', 3);

INSERT INTO ORDER_ITEMS_RANGE_PARTITIONED (order_datetime, customer_id, order_status, store_id)
VALUES (CURRENT_TIMESTAMP, 25, 'Shipped', 4);

UPDATE ORDER_ITEMS_RANGE_PARTITIONED
SET order_status = 'Completed'
WHERE order_id = 80;

DELETE FROM ORDER_ITEMS_RANGE_PARTITIONED
WHERE order_id = 12;

INSERT INTO ACCOUNTS_LIST_PARTITIONED (id, account_number, customer_id, branch_id, region, status)
VALUES (3, 3001, 3, 3, 'GA', 'A');

INSERT INTO ACCOUNTS_LIST_PARTITIONED (id, account_number, customer_id, branch_id, region, status)
VALUES (4, 4001, 4, 4, 'NY', 'A');

UPDATE ACCOUNTS_LIST_PARTITIONED
SET status = 'I'
WHERE id = 2;

DELETE FROM ACCOUNTS_LIST_PARTITIONED
WHERE account_number = 1001;

INSERT INTO ORDERS_INTERVAL_PARTITION (customer_id, status, order_date)
VALUES (3, 'Processing', TO_DATE('2023-11-15', 'YYYY-MM-DD'));

INSERT INTO ORDERS_INTERVAL_PARTITION (customer_id, status, order_date)
VALUES (4, 'Shipped', TO_DATE('2023-11-20', 'YYYY-MM-DD'));

UPDATE ORDERS_INTERVAL_PARTITION
SET status = 'Delivered'
WHERE order_id = 1;

DELETE FROM ORDERS_INTERVAL_PARTITION
WHERE order_id = 108;

INSERT INTO SALES_HASH (s_productid, s_saledate, s_custid, s_totalprice)
VALUES (3, TO_DATE('2023-11-15', 'YYYY-MM-DD'), 303, 150.25);

INSERT INTO SALES_HASH (s_productid, s_saledate, s_custid, s_totalprice)
VALUES (4, TO_DATE('2023-11-20', 'YYYY-MM-DD'), 404, 220.50);

UPDATE SALES_HASH
SET s_totalprice = 180.00
WHERE s_productid = 3;

DELETE FROM SALES_HASH
WHERE s_productid = 1;

INSERT INTO sub_par_test (emp_name, job_id, hire_date)
VALUES ('Alice Johnson', 'SA_MAN', TO_DATE('2005-03-10', 'YYYY-MM-DD'));

INSERT INTO sub_par_test (emp_name, job_id, hire_date)
VALUES ('Bob Anderson', 'ST_CLERK', TO_DATE('2006-07-25', 'YYYY-MM-DD'));

UPDATE sub_par_test
SET emp_name = 'Updated Employee'
WHERE id=13;

DELETE FROM sub_par_test
WHERE hire_date = TO_DATE('2004-06-15', 'YYYY-MM-DD');

INSERT INTO empty_partition_table2 (id, region, status, description) VALUES (1, 'CA', 'A', 'Description A');
INSERT INTO empty_partition_table2 (id, region, status, description) VALUES (2, 'OR', 'B', 'Description B');
INSERT INTO empty_partition_table2 (id, region, status, description) VALUES (3, 'WA', 'C', 'Description C');
INSERT INTO empty_partition_table2 (id, region, status, description) VALUES (4, 'NJ', 'D', 'Description D');

commit;
