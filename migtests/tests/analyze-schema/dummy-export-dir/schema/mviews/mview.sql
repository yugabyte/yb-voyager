CREATE MATERIALIZED VIEW test AS (
                            select x , JSON_ARRAYAGG(trunc(b, 2) order by t desc) as agg
                            FROM test1
                            where t = '1DAY' group by x
                            );

CREATE MATERIALIZED VIEW public.sample_data_view AS
 SELECT sample_data.id,
    sample_data.name,
    sample_data.description,
    XMLFOREST(sample_data.name AS name, sample_data.description AS description) AS xml_data,
    pg_try_advisory_lock((sample_data.id)::bigint) AS lock_acquired,
    sample_data.ctid AS row_ctid,
    sample_data.xmin AS xmin_value
   FROM public.sample_data
  WITH NO DATA;