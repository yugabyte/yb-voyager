INSERT INTO ORDER_ITEMS_RANGE_PARTITIONED (order_id,order_datetime, customer_id, order_status, store_id)
VALUES (80,CURRENT_TIMESTAMP, 10, 'Pending', 1);

INSERT INTO ORDER_ITEMS_RANGE_PARTITIONED (order_id,order_datetime, customer_id, order_status, store_id)
VALUES (81,CURRENT_TIMESTAMP, 20, 'Shipped', 2);

UPDATE ORDER_ITEMS_RANGE_PARTITIONED
SET order_status = 'Delivered'
WHERE order_id = 5;

DELETE FROM ORDER_ITEMS_RANGE_PARTITIONED
WHERE order_id = 10;

INSERT INTO ACCOUNTS_LIST_PARTITIONED (id, account_number, customer_id, branch_id, region, status)
VALUES (1, 1001, 1, 1, 'OR', 'A');

INSERT INTO ACCOUNTS_LIST_PARTITIONED (id, account_number, customer_id, branch_id, region, status)
VALUES (2, 2001, 2, 2, 'NM', 'A');

UPDATE ACCOUNTS_LIST_PARTITIONED
SET status = 'I'
WHERE account_number = 1001;

DELETE FROM ACCOUNTS_LIST_PARTITIONED
WHERE account_number = 2001;

INSERT INTO ORDERS_INTERVAL_PARTITION (customer_id, status, order_date)
VALUES (1, 'Processing', TO_DATE('2023-10-15', 'YYYY-MM-DD'));

INSERT INTO ORDERS_INTERVAL_PARTITION (customer_id, status, order_date)
VALUES (2, 'Shipped', TO_DATE('2023-10-20', 'YYYY-MM-DD'));

UPDATE ORDERS_INTERVAL_PARTITION
SET status = 'Delivered'
WHERE order_id = 106;

DELETE FROM ORDERS_INTERVAL_PARTITION
WHERE order_id = 87;

INSERT INTO SALES_HASH (s_productid, s_saledate, s_custid, s_totalprice)
VALUES (1, TO_DATE('2023-10-15', 'YYYY-MM-DD'), 101, 100.50);

INSERT INTO SALES_HASH (s_productid, s_saledate, s_custid, s_totalprice)
VALUES (2, TO_DATE('2023-10-20', 'YYYY-MM-DD'), 202, 200.75);

UPDATE SALES_HASH
SET s_totalprice = 150.00
WHERE s_productid = 1;

DELETE FROM SALES_HASH
WHERE s_productid = 2;

INSERT INTO sub_par_test (emp_name, job_id, hire_date)
VALUES ('John Doe', 'HR_REP', TO_DATE('2002-12-31', 'YYYY-MM-DD'));

INSERT INTO sub_par_test (emp_name, job_id, hire_date)
VALUES ('Jane Smith', 'AC_ACCOUNT', TO_DATE('2004-06-15', 'YYYY-MM-DD'));

UPDATE sub_par_test
SET emp_name = 'Updated Name'
WHERE hire_date = TO_DATE('2004-06-15', 'YYYY-MM-DD');

DELETE FROM sub_par_test
WHERE hire_date = TO_DATE('2002-12-31', 'YYYY-MM-DD');

INSERT INTO empty_partition_table (id, region, status) VALUES (1, 'CA', 'A');
INSERT INTO empty_partition_table (id, region, status) VALUES (2, 'OR', 'B');
INSERT INTO empty_partition_table (id, region, status) VALUES (3, 'NY', 'C');
INSERT INTO empty_partition_table (id, region, status) VALUES (4, 'NJ', 'D');

commit;
