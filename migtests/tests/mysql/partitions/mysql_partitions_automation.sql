drop table if exists range_partition_test;

CREATE TABLE range_partition_test ( cust_id INT NOT NULL, name VARCHAR(40),   
store_id VARCHAR(20) NOT NULL, bill_no INT NOT NULL,   
bill_date DATE NOT NULL, amount DECIMAL(8,2) NOT NULL, primary key(cust_id, name))   
PARTITION BY RANGE (cust_id)(   
PARTITION p0 VALUES LESS THAN (3),   
PARTITION p1 VALUES LESS THAN (5),   
PARTITION p2 VALUES LESS THAN (7),   
PARTITION p3 VALUES LESS THAN MAXVALUE);  

INSERT INTO range_partition_test VALUES   
(1, 'Mike', 'S001', 101, '2015-01-02', 125.56),   
(2, 'Robert', 'S003', 103, '2015-01-25', 476.50),   
(3, 'Peter', 'S012', 122, '2016-02-15', 335.00),   
(4, 'Joseph', 'S345', 121, '2016-03-26', 787.00),   
(5, 'Harry', 'S234', 132, '2017-04-19', 678.00),   
(6, 'Stephen', 'S743', 111, '2017-05-31', 864.00),   
(7, 'Jacson', 'S234', 115, '2018-06-11', 762.00),   
(8, 'Smith', 'S012', 125, '2019-07-24', 300.00),   
(9, 'Adam', 'S456', 119, '2019-08-02', 492.20);  

SELECT PARTITION_NAME, TABLE_ROWS FROM INFORMATION_SCHEMA.PARTITIONS WHERE TABLE_NAME='range_partition_test';


drop table if exists list_partition_test;

CREATE TABLE list_partition_test (bill_no INT, bill_date TIMESTAMP, 
cust_code VARCHAR(15) NOT NULL, amount DECIMAL(8,2), primary key(bill_no,bill_date)) 
PARTITION BY LIST(bill_no) (   
PARTITION pEast VALUES IN (1, 3),   
PARTITION pWest VALUES IN (2, 4),   
PARTITION pNorth VALUES IN (5, 6),   
PARTITION pSouth VALUES IN (7,8,9),
PARTITION pEmpty VALUES IN (11));   

INSERT INTO list_partition_test VALUES (1, '2013-01-02', 'C001', 125.56), 
(2, '2013-01-25', 'C003', 456.50), 
(3, '2014-02-15', 'C012', 365.00), 
(4, '2013-03-26', 'C345', 785.00), 
(5, '2013-04-19', 'C234', 656.00), 
(6, '2013-05-31', 'C743', 854.00), 
(7, '2013-06-11', 'C234', 542.00), 
(8, '2013-07-24', 'C003', 300.00), 
(9, '2013-08-02', 'C456', 475.20);

SELECT * FROM list_partition_test;

SELECT PARTITION_NAME, TABLE_ROWS FROM INFORMATION_SCHEMA.PARTITIONS WHERE TABLE_NAME='list_partition_test';


drop table if exists range_columns_partition_test;

CREATE TABLE range_columns_partition_test (
    a INT,
    b INT
)
PARTITION BY RANGE COLUMNS(a, b) (
    PARTITION p0 VALUES LESS THAN (5, 5),
    PARTITION p1 VALUES LESS THAN (MAXVALUE, MAXVALUE)
);

INSERT INTO range_columns_partition_test VALUES (5,5),(3,4), (5,11), (5,12),(4,3),(3,1);

SELECT PARTITION_NAME, TABLE_ROWS FROM INFORMATION_SCHEMA.PARTITIONS WHERE TABLE_NAME='range_columns_partition_test';


drop table if exists key_partition_test;

CREATE TABLE key_partition_test (
    a INT,
    b INT ,
    c INT, primary key(b,c)
)
PARTITION BY key(c)
partitions 2;

INSERT INTO key_partition_test VALUES (5,6,1),(3,4,6), (5,11,1), (5,1,2),(5,5,5);

SELECT PARTITION_NAME, TABLE_ROWS FROM INFORMATION_SCHEMA.PARTITIONS WHERE TABLE_NAME='key_partition_test';

select * from key_partition_test partition(p0);
select * from key_partition_test partition(p1);

/*
drop table if exists hash_partition_test;

CREATE TABLE hash_partition_test (
    a INT,
    b INT primary key,
    c INT
)
PARTITION BY hash(b)
partitions 2;

INSERT INTO hash_partition_test VALUES (5,6,1),(3,4,6), (5,11,1), (5,1,2),(5,5,5);

SELECT PARTITION_NAME, TABLE_ROWS FROM INFORMATION_SCHEMA.PARTITIONS WHERE TABLE_NAME='hash_partition_test';

select * from hash_partition_test partition(p0);
select * from hash_partition_test partition(p1);
*/

drop table if exists subpartitioning_test;

CREATE TABLE subpartitioning_test( cust_id INT NOT NULL, name VARCHAR(40),   
store_id VARCHAR(20) NOT NULL, bill_no INT NOT NULL,   
bill_date DATE NOT NULL, amount DECIMAL(8,2) NOT NULL, primary key(cust_id, amount,bill_no))   
PARTITION BY RANGE (cust_id)
SUBPARTITION BY HASH(bill_no)
SUBPARTITIONS 4
(   
PARTITION p0 VALUES LESS THAN (3),   
PARTITION p1 VALUES LESS THAN (5),   
PARTITION p2 VALUES LESS THAN (7),   
PARTITION p3 VALUES LESS THAN MAXVALUE);  

INSERT INTO subpartitioning_test VALUES   
(1, 'Mike', 'S001', 101, '2015-01-02', 125.56),   
(2, 'Robert', 'S003', 103, '2015-01-25', 476.50),   
(3, 'Peter', 'S012', 122, '2016-02-15', 335.00),   
(4, 'Joseph', 'S345', 121, '2016-03-26', 787.00),   
(5, 'Harry', 'S234', 132, '2017-04-19', 678.00),   
(6, 'Stephen', 'S743', 111, '2017-05-31', 864.00),   
(7, 'Jacson', 'S234', 115, '2018-06-11', 762.00),   
(8, 'Smith', 'S012', 125, '2019-07-24', 300.00),   
(9, 'Adam', 'S456', 119, '2019-08-02', 492.20);  

SELECT PARTITION_NAME, TABLE_ROWS FROM INFORMATION_SCHEMA.PARTITIONS WHERE TABLE_NAME='subpartitioning_test';

