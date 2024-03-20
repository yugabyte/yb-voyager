-- Insert into the sales_region table
INSERT INTO sales_region (id, amount, branch, region) VALUES (1001, 1000, 'Branch 1001', 'London');

-- Insert into a specific partition
INSERT INTO Boston (id, amount, branch, region) VALUES (1002, 2000, 'Branch 1002', 'Boston');

-- Update the sales_region table
UPDATE sales_region SET amount = 1500 WHERE id = 10;

-- Update a specific partition
UPDATE London SET amount = 2500 WHERE id = 1001 AND region = 'London';

-- Delete from the sales_region table
DELETE FROM sales_region WHERE id = 1;

-- Delete from a specific partition
DELETE FROM sales_region WHERE id = 2;

-- Insert into the p1.sales_region table
INSERT INTO p1.sales_region (id, amount, branch, region) VALUES (1001, 1000, 'Branch 1001', 'London');

-- Insert into a specific partition
INSERT INTO p2.Boston (id, amount, branch, region) VALUES (1002, 2000, 'Branch 1002', 'Boston');

-- Update the p1.sales_region table
UPDATE p1.sales_region SET amount = 1500 WHERE id = 10;

-- Update a specific partition
UPDATE p2.London SET amount = 2500 WHERE id = 1001 AND region = 'London';

-- Delete from the p1.sales_region table
DELETE FROM p1.sales_region WHERE id = 1;

-- Delete from a specific partition
DELETE FROM p2.Sydney WHERE id = 2;

-- Insert into the sales table
INSERT INTO sales (id, p_name, amount, sale_date) VALUES (1001, 'Person 1001', 1000, '2019-11-01');

-- Insert into a specific partition
INSERT INTO sales_2020_Q1 (id, p_name, amount, sale_date) VALUES (1002, 'Person 1002', 2000, '2020-02-01');

-- Update the sales table
UPDATE sales SET amount = 1500 WHERE id = 10;

-- Update a specific partition
UPDATE sales_2019_Q4 SET amount = 2500 WHERE id = 1001 AND sale_date = '2019-11-01';

-- Delete from the sales table
DELETE FROM sales WHERE id = 1;

-- Insert into the range_columns_partition_test table
INSERT INTO range_columns_partition_test (a, b) VALUES (10, 10);

-- Insert into a specific partition
INSERT INTO range_columns_partition_test_p1 (a, b) VALUES (14, 11);

-- Update the range_columns_partition_test table
UPDATE range_columns_partition_test SET b = 1500 WHERE a = 10;

-- Update a specific partition
UPDATE range_columns_partition_test_p0 SET b = 2500 WHERE a = 14;

-- Insert into the emp table
INSERT INTO emp (emp_id, emp_name, dep_code) VALUES (1001, 'Person 1001', 1000);

-- Update the emp table
UPDATE emp SET dep_code = 1500 WHERE emp_id = 10;

-- Update a specific partition
UPDATE emp_1 SET dep_code = 2500 WHERE emp_id = 1001;

-- Delete from the emp table
DELETE FROM emp WHERE emp_id = 1;

-- Insert into the customers table
INSERT INTO customers (id, statuses, arr) VALUES (1001, 'ACTIVE', 1000);

-- Insert into a specific partition
INSERT INTO cust_active (id, statuses, arr) VALUES (1002, 'RECURRING', 2000);

-- Insert into a specific partition
INSERT INTO cust_other (id, statuses, arr) VALUES (1003, 'OTHER', 3000);

-- Update the customers table
UPDATE customers SET arr = 1500 WHERE id = 10;

-- Update a specific partition
UPDATE cust_active SET arr = 2500 WHERE id = 1001;

-- Delete from the customers table
DELETE FROM customers WHERE id = 1;


