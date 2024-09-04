CREATE MATERIALIZED VIEW test AS (
                            select x , JSON_ARRAYAGG(trunc(b, 2) order by t desc) as agg
                            FROM test1
                            where t = '1DAY' group by x
                            );