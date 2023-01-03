drop table if exists subpartitioning_test;

CREATE TABLE subpartitioning_test (BILL_NO INT, sale_date DATE, cust_code VARCHAR(15), 
AMOUNT DECIMAL(8,2))
PARTITION BY List(MONTH(sale_date) )
SUBPARTITION BY HASH(TO_DAYS(sale_date))
SUBPARTITIONS 2 (
PARTITION p0 VALUES IN (1, 2),   
PARTITION p1 VALUES IN (3, 4),   
PARTITION p2 VALUES IN (5, 6),   
PARTITION p3 VALUES IN (7,8,9),
PARTITION p4 VALUES IN (10,11,12),
PARTITION p5 VALUES IN (13)
);   


INSERT INTO subpartitioning_test VALUES (1, '2013-01-02', 'C001', 125.56), 
(2, '2013-01-25', 'C003', 456.50), 
(3, '2014-02-15', 'C012', 365.00), 
(4, '2013-03-26', 'C345', 785.00), 
(5, '2013-04-19', 'C234', 656.00), 
(6, '2013-05-31', 'C743', 854.00), 
(11, '2013-05-31', 'C743', 854.00),
(7, '2013-06-11', 'C234', 542.00), 
(8, '2013-07-24', 'C003', 300.00), 
(9, '2013-08-02', 'C456', 475.20),
(10, '2013-11-02', 'C456', 475.20);


SELECT * FROM subpartitioning_test;