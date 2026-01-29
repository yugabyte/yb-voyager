\i target_delta_DDLs.sql

set search_path to schema2;

\i target_delta_DDLs.sql


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
                        FROM arr_list, generate_series(1011, 1020) AS n, status_list;

UPDATE "Schema".customers SET statuses = 'REACTIVATED' WHERE id = 1009;
UPDATE "Schema".customers SET statuses = 'ACTIVE' WHERE id = 1015;
DELETE FROM "Schema".customers WHERE id = 960;
DELETE FROM "Schema".customers WHERE id = 1012;

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
FROM amount_list, region_list, generate_series(1011,1020) as n;
UPDATE public."Sales_region" SET amount = 1500 WHERE id = 1009;
UPDATE public."Sales_region" SET amount = 2500 WHERE id = 1015;
DELETE FROM public."Sales_region" WHERE id = 960;
DELETE FROM public."Sales_region" WHERE id = 1012;

INSERT INTO "Schema".tbl_seq1 (val) SELECT 'val' || n FROM generate_series(1011,1020) AS n;
UPDATE "Schema".tbl_seq1 SET val = 'Updated Value 1' WHERE id = 1011;
UPDATE "Schema".tbl_seq1 SET val = NULL WHERE id = 1015;
DELETE FROM "Schema".tbl_seq1 WHERE id = 960;
DELETE FROM "Schema".tbl_seq1 WHERE id = 1012;

INSERT INTO "pg-schema"."TestTable"  (val) SELECT 'val' || n FROM generate_series(1001,1010) AS n;
UPDATE "pg-schema"."TestTable"  SET val = 'Updated Value 1' WHERE id = 1011;
UPDATE "pg-schema"."TestTable"  SET val = NULL WHERE id = 955;
DELETE FROM "pg-schema"."TestTable"  WHERE id = 960;
DELETE FROM "pg-schema"."TestTable"  WHERE id = 1012;

INSERT INTO "order".test1 (val) SELECT 'val' || n FROM generate_series(1011,1020) AS n;
UPDATE "order".test1 SET val = 'Updated Value 1' WHERE id = 1011;
UPDATE "order".test1 SET val = NULL WHERE id = 960;
DELETE FROM "order".test1 WHERE id = 960;
DELETE FROM "order".test1 WHERE id = 1012;

INSERT INTO "pg-schema".test2 (val) SELECT 'val' || n FROM generate_series(1011,1020) AS n;
UPDATE "pg-schema".test2 SET val = 'Updated Value 1' WHERE id = 1011;
UPDATE "pg-schema".test2 SET val = NULL WHERE id = 985;
DELETE FROM "pg-schema".test2 WHERE id = 960;
DELETE FROM "pg-schema".test2 WHERE id = 1012;

INSERT INTO "Schema"."Test2" (val) SELECT 'val' || n FROM generate_series(1001,1010) AS n;
UPDATE "Schema"."Test2" SET val = 'Updated Value 1' WHERE id = 1011;
UPDATE "Schema"."Test2" SET val = NULL WHERE id = 677;
DELETE FROM "Schema"."Test2" WHERE id = 956;
DELETE FROM "Schema"."Test2" WHERE id = 1012;