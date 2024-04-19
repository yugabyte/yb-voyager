
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
                    FROM amount_list, region_list, generate_series(1,1000) as n;


WITH region_list AS (
     SELECT '{"London", "Boston", "Sydney"}'::TEXT[] region
     ), amount_list AS (
        SELECT '{1000, 2000, 5000}'::INT[] amount
        ) 
        INSERT INTO test_partitions_sequences  
        (amount, branch, region) 
            SELECT 
                amount[1 + mod(n, array_length(amount, 1))], 
                'Branch ' || n as branch, 
                region[1 + mod(n, array_length(region, 1))] 
                    FROM amount_list, region_list, generate_series(1,1000) as n;

WITH region_list AS (
     SELECT '{"London", "Boston", "Sydney"}'::TEXT[] region
     ), amount_list AS (
        SELECT '{1000, 2000, 5000}'::INT[] amount
        ) 
        INSERT INTO p1.sales_region  
        (id, amount, branch, region) 
            SELECT 
                n, 
                amount[1 + mod(n, array_length(amount, 1))], 
                'Branch ' || n as branch, 
                region[1 + mod(n, array_length(region, 1))] 
                    FROM amount_list, region_list, generate_series(1,1000) as n;

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
                        generate_series(1,1000) as n;

INSERT INTO
    range_columns_partition_test
VALUES
    (5, 5),
    (3, 4),
    (5, 11),
    (5, 12),
    (4, 3),
    (3, 1);

\ d + range_columns_partition_test
SELECT
    tableoid :: regclass,
    *
FROM
    range_columns_partition_test;

INSERT INTO emp SELECT num, 'user_' || num , (RANDOM()*50)::INTEGER FROM generate_series(1,1000) AS num;


WITH status_list AS (
        SELECT '{"ACTIVE", "RECURRING", "REACTIVATED", "EXPIRED"}'::TEXT[] statuses
        ), arr_list AS (
            SELECT '{100, 200, 50, 250}'::INT[] arr
        )
        INSERT INTO "Customers" 
        (id, statuses, arr)
            SELECT  n,
                    statuses[1 + mod(n, array_length(statuses, 1))],
                    arr[1 + mod(n, array_length(arr, 1))]
                        FROM arr_list, generate_series(1,1000) AS n, status_list;

