
CREATE TABLE sales_region (id int, amount int, branch text, region text) PARTITION BY LIST (region);
CREATE TABLE London PARTITION OF sales_region FOR VALUES IN ('London');
CREATE TABLE Sydney PARTITION OF sales_region FOR VALUES IN ('Sydney');
CREATE TABLE Boston PARTITION OF sales_region FOR VALUES IN ('Boston');

WITH region_list AS (
     SELECT '{"London", "Boston", "Sydney"}'::TEXT[] region
     ), amount_list AS (
        SELECT '{1000, 2000, 5000}'::INT[] amount
        ) 
        INSERT INTO sales_region  
        (id, amount, branch, region) 
            SELECT 
                n, 
                amount[1 + mod(n, array_length(amount, 1))], 
                'Branch ' || n as branch, 
                region[1 + mod(n, array_length(region, 1))] 
                    FROM amount_list, region_list, generate_series(1,100) as n;


-- Partition by range

CREATE TABLE sales 
    (id int, p_name text, amount int, sale_date timestamp) 
PARTITION BY RANGE (sale_date);
CREATE TABLE sales_2019_Q4 PARTITION OF sales FOR VALUES FROM ('2019-10-01') TO ('2020-01-01');
CREATE TABLE sales_2020_Q1 PARTITION OF sales FOR VALUES FROM ('2020-01-01') TO ('2020-04-01');
CREATE TABLE sales_2020_Q2 PARTITION OF sales FOR VALUES FROM ('2020-04-01') TO ('2020-07-01');

WITH amount_list AS (
        SELECT '{1000, 2000, 5000}'::INT[] amount
        ), date_list AS (
            SELECT '{"2019-11-01", "2020-02-01", "2020-05-01"}'::TIMESTAMP[] sale_date
            ) 
            INSERT INTO sales
            (id, p_name, amount, sale_date)
                SELECT
                    n,
                    'Person ' || n as p_name,
                    amount[1 + mod(n, array_length(amount, 1))],
                    sale_date[1 + mod(n, array_length(amount, 1))]
                        FROM 
                        amount_list,
                        date_list,
                        generate_series(1,100) as n;

-- Partition by hash

CREATE TABLE emp (emp_id int, emp_name text, dep_code int) PARTITION BY HASH (emp_id);

CREATE TABLE emp_0 PARTITION OF emp FOR VALUES WITH (MODULUS 3,REMAINDER 0);
CREATE TABLE emp_1 PARTITION OF emp FOR VALUES WITH (MODULUS 3,REMAINDER 1);
CREATE TABLE emp_2 PARTITION OF emp FOR VALUES WITH (MODULUS 3,REMAINDER 2);

INSERT INTO emp SELECT num, 'user_' || num , (RANDOM()*50)::INTEGER FROM generate_series(1,100) AS num;


-- Multilevel Partition

CREATE TABLE customers (id INTEGER, statuses TEXT, arr NUMERIC) PARTITION BY LIST(statuses);

CREATE TABLE cust_active PARTITION OF customers FOR VALUES IN ('ACTIVE', 'RECURRING','REACTIVATED') PARTITION BY RANGE(arr);
CREATE TABLE cust_other  PARTITION OF customers DEFAULT;

CREATE TABLE cust_arr_small PARTITION OF cust_active FOR VALUES FROM (MINVALUE) TO (101) PARTITION BY HASH(id);
CREATE TABLE cust_part11 PARTITION OF cust_arr_small FOR VALUES WITH (modulus 2, remainder 0);
CREATE TABLE cust_part12 PARTITION OF cust_arr_small FOR VALUES WITH (modulus 2, remainder 1);

CREATE TABLE cust_arr_large PARTITION OF cust_active FOR VALUES FROM (101) TO (MAXVALUE) PARTITION BY HASH(id);
CREATE TABLE cust_part21 PARTITION OF cust_arr_large FOR VALUES WITH (modulus 2, remainder 0);
CREATE TABLE cust_part22 PARTITION OF cust_arr_large FOR VALUES WITH (modulus 2, remainder 1);


WITH status_list AS (
        SELECT '{"ACTIVE", "RECURRING", "REACTIVATED", "EXPIRED"}'::TEXT[] statuses
        )
        INSERT INTO customers 
        (id, statuses, arr)
            SELECT  n,
                    statuses[1 + mod(n, array_length(statuses, 1))],
                    (RANDOM()*200)::INTEGER
                        FROM generate_series(1,100) AS n, status_list;



