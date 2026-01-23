\i snapshot.sql

set search_path to schema2;

\i snapshot.sql

WITH status_list AS (                                                                                                                              
        SELECT '{"ACTIVE", "RECURRING", "REACTIVATED", "EXPIRED"}'::TEXT[] statuses
        ), arr_list AS (
            SELECT '{100, 200, 50, 250}'::INT[] arr
        )                                                  
        INSERT INTO "Schema".customers
        (id, statuses, arr)
            SELECT  n,
                    statuses[1 + mod(n, array_length(statuses, 1))],
                    arr[1 + mod(n, array_length(arr, 1))]
                        FROM arr_list, generate_series(1,1000) AS n, status_list;


WITH region_list AS (                                                                                                                              
    SELECT ARRAY['London', 'Sydney', 'Boston']::TEXT[] region
), amount_list AS (
    SELECT ARRAY[1000, 2000, 5000]::INT[] amount
)                                                          
INSERT INTO public."Sales_region"  
(id, amount, branch, region) 
SELECT 
    n,                                             
    amount[1 + mod(n, array_length(amount, 1))], 
    'Branch ' || n as branch, 
    region[1 + mod(n, array_length(region, 1))] 
FROM amount_list, region_list, generate_series(1,1000) as n;


INSERT INTO "Schema".tbl_seq1 (val) SELECT 'val' || n FROM generate_series(1,1000) AS n;

INSERT INTO "pg-schema"."TestTable" (val) SELECT 'val' || n FROM generate_series(1,1000) AS n;

INSERT INTO "order".test1 (val) SELECT 'val' || n FROM generate_series(1,1000) AS n;

INSERT INTO "pg-schema".test2 (val) SELECT 'val' || n FROM generate_series(1,1000) AS n;

INSERT INTO "Schema"."Test2" (val) SELECT 'val' || n FROM generate_series(1,1000) AS n;