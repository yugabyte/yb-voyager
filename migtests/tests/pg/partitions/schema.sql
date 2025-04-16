
CREATE TABLE sales_region (id int, amount int, branch text, region text, PRIMARY KEY(id, region)) PARTITION BY LIST (region);
CREATE TABLE London PARTITION OF sales_region FOR VALUES IN ('London');
CREATE TABLE Sydney PARTITION OF sales_region FOR VALUES IN ('Sydney');
CREATE TABLE Boston PARTITION OF sales_region FOR VALUES IN ('Boston');

create sequence test_defaul_seq;
CREATE TABLE test_partitions_sequences (id serial, id1 int DEFAULT nextval('test_defaul_seq'), amount int, branch text, region text, PRIMARY KEY(id, region)) PARTITION BY LIST (region);
CREATE TABLE test_partitions_sequences_l PARTITION OF test_partitions_sequences FOR VALUES IN ('London');
CREATE TABLE test_partitions_sequences_s PARTITION OF test_partitions_sequences FOR VALUES IN ('Sydney');
CREATE TABLE test_partitions_sequences_b PARTITION OF test_partitions_sequences FOR VALUES IN ('Boston');

-- Partition by list with parent table in p1 schema and partitions in p2
CREATE SCHEMA p1;
CREATE SCHEMA p2;

CREATE TABLE p1.sales_region (id int, amount int, branch text, region text, PRIMARY KEY(id, region)) PARTITION BY LIST (region);
CREATE TABLE p2.London PARTITION OF p1.sales_region FOR VALUES IN ('London');
CREATE TABLE p2.Sydney PARTITION OF p1.sales_region FOR VALUES IN ('Sydney');
CREATE TABLE p2.Boston PARTITION OF p1.sales_region FOR VALUES IN ('Boston');



-- Partition by range

CREATE TABLE sales 
    (id int, p_name text, amount int, sale_date timestamp, PRIMARY KEY(id, sale_date)) 
PARTITION BY RANGE (sale_date);
CREATE TABLE sales_2019_Q4 PARTITION OF sales FOR VALUES FROM ('2019-10-01') TO ('2020-01-01');
CREATE TABLE sales_2020_Q1 PARTITION OF sales FOR VALUES FROM ('2020-01-01') TO ('2020-04-01');
CREATE TABLE sales_2020_Q2 PARTITION OF sales FOR VALUES FROM ('2020-04-01') TO ('2020-07-01');



-- range columns partiitons

drop table if exists range_columns_partition_test;

CREATE TABLE range_columns_partition_test (a bigint, b bigint, PRIMARY KEY(a,b)) PARTITION BY RANGE (a, b);

CREATE TABLE range_columns_partition_test_p0 PARTITION OF range_columns_partition_test FOR
VALUES
FROM
    (MINVALUE, MINVALUE) TO (5, 5);

CREATE TABLE range_columns_partition_test_p1 PARTITION OF range_columns_partition_test DEFAULT;



-- Partition by hash

CREATE TABLE emp (emp_id int, emp_name text, dep_code int, PRIMARY KEY(emp_id)) PARTITION BY HASH (emp_id);

CREATE TABLE emp_0 PARTITION OF emp FOR VALUES WITH (MODULUS 3,REMAINDER 0);
CREATE TABLE emp_1 PARTITION OF emp FOR VALUES WITH (MODULUS 3,REMAINDER 1);
CREATE TABLE emp_2 PARTITION OF emp FOR VALUES WITH (MODULUS 3,REMAINDER 2);


-- Multilevel Partition

CREATE TABLE customers (id INTEGER, statuses TEXT, arr NUMERIC, PRIMARY KEY(id, statuses, arr)) PARTITION BY LIST(statuses);

CREATE TABLE cust_active PARTITION OF customers FOR VALUES IN ('ACTIVE', 'RECURRING','REACTIVATED') PARTITION BY RANGE(arr);
CREATE TABLE cust_other  PARTITION OF customers DEFAULT;

CREATE TABLE cust_arr_small PARTITION OF cust_active FOR VALUES FROM (MINVALUE) TO (101) PARTITION BY HASH(id);
CREATE TABLE cust_part11 PARTITION OF cust_arr_small FOR VALUES WITH (modulus 2, remainder 0);
CREATE TABLE cust_part12 PARTITION OF cust_arr_small FOR VALUES WITH (modulus 2, remainder 1);

CREATE TABLE cust_arr_large PARTITION OF cust_active FOR VALUES FROM (101) TO (MAXVALUE) PARTITION BY HASH(id);
CREATE TABLE cust_part21 PARTITION OF cust_arr_large FOR VALUES WITH (modulus 2, remainder 0);
CREATE TABLE cust_part22 PARTITION OF cust_arr_large FOR VALUES WITH (modulus 2, remainder 1);
