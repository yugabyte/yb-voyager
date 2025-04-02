-- Insert into the sales_region table
INSERT INTO sales_region (id, amount, branch, region) VALUES (1011, 1000, 'Branch 1011', 'Sydney');

-- Insert into a specific partition
INSERT INTO Boston (id, amount, branch, region) VALUES (1012, 2000, 'Branch 1012', 'Boston');

-- Update the sales_region table
UPDATE sales_region SET amount = 1500 WHERE id = 13;

-- Update a specific partition
UPDATE Sydney SET amount = 2500 WHERE id = 1011 AND region = 'Sydney';

-- Delete from the sales_region table
DELETE FROM sales_region WHERE id = 5;

-- Delete from a specific partition
DELETE FROM sales_region WHERE id = 99;

-- Insert into the test_partitions_sequences table
INSERT INTO test_partitions_sequences (id, id1, amount, branch, region) VALUES (1011, 1230, 1000, 'Branch 1011', 'Sydney');

-- Insert into a specific partition
INSERT INTO test_partitions_sequences_b (id, amount, branch, region) VALUES (1012, 2000, 'Branch 1012', 'Boston');

-- Update the test_partitions_sequences table
UPDATE test_partitions_sequences SET amount = 1500 WHERE id = 13;

-- Update a specific partition
UPDATE test_partitions_sequences_s SET amount = 2500 WHERE id = 1011 AND region = 'Sydney';

-- Delete from the test_partitions_sequences table
DELETE FROM test_partitions_sequences WHERE id = 5;

-- Delete from a specific partition
DELETE FROM test_partitions_sequences WHERE id = 99;

-- Insert into the p1.sales_region table
INSERT INTO p1.sales_region (id, amount, branch, region) VALUES (1011, 1000, 'Branch 1011', 'Sydney');

-- Insert into a specific partition of target
INSERT INTO p2.boston_region (id, amount, branch, region) VALUES (1012, 2000, 'Branch 1012', 'Boston');

-- Update the p1.sales_region table
UPDATE p1.sales_region SET amount = 1500 WHERE id = 16;

-- Update a specific partition
UPDATE p2.sydney_region SET amount = 2500 WHERE id = 1011 AND region = 'Sydney';

-- Delete from the p1.sales_region table
DELETE FROM p1.sales_region WHERE id = 17;

-- Delete from a specific partition
DELETE FROM p2.sydney_region WHERE id = 200;


-- Insert into the sales table
INSERT INTO sales (id, p_name, amount, sale_date) VALUES (1011, 'Person 1011', 1000, '2019-11-01');

-- Insert into a specific partition
INSERT INTO sales_2020_Q1 (id, p_name, amount, sale_date) VALUES (1012, 'Person 1012', 2000, '2020-02-01');

-- Update the sales table
UPDATE sales SET amount = 1500 WHERE id = 101;

-- Update a specific partition
UPDATE sales_2019_Q4 SET amount = 2500 WHERE id = 1011 AND sale_date = '2019-11-01';

-- Delete from the sales table
DELETE FROM sales WHERE id = 166;

-- Insert into the range_columns_partition_test table
INSERT INTO range_columns_partition_test (a, b) VALUES (107, 107);

-- Insert into a specific partition
INSERT INTO range_columns_partition_test_p1 (a, b) VALUES (145, 115);

-- Update the range_columns_partition_test table
UPDATE range_columns_partition_test SET b = 1500 WHERE a = 107;

-- Update a specific partition
UPDATE range_columns_partition_test_p1 SET b = 25006 WHERE a = 14;

-- Insert into the emp table
INSERT INTO emp (emp_id, emp_name, dep_code) VALUES (1011, 'Person 1011', 1008);

-- Update the emp table
UPDATE emp SET dep_code = 1500 WHERE emp_id = 102;

-- Update a specific partition
UPDATE emp_1 SET dep_code = 2500 WHERE emp_id = 1011;

-- Delete from the emp table
DELETE FROM emp WHERE emp_id = 143;

-- Insert into the customers table
INSERT INTO customers (id, statuses, arr) VALUES (1011, 'REACTIVATED', 1000);

-- Insert into a specific partition
INSERT INTO cust_active (id, statuses, arr) VALUES (1012, 'RECURRING', 2000);

-- Insert into a specific partition
INSERT INTO cust_other (id, statuses, arr) VALUES (1013, 'OTHER', 3000);

-- Update the customers table
UPDATE customers SET arr = 1500 WHERE id = 145;

-- Update a specific partition
UPDATE cust_active SET arr = 2500 WHERE id = 1011;

-- Delete from the customers table
DELETE FROM customers WHERE id = 14;


