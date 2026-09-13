-- Applied after pg/partitions/schema.sql: drop the p1/p2 cross-schema block (this test is
-- public-only) and add DEFAULT partitions so the generator's random values always route.

DROP SCHEMA IF EXISTS p1 CASCADE;
DROP SCHEMA IF EXISTS p2 CASCADE;

CREATE TABLE public.sales_region_default              PARTITION OF public.sales_region              DEFAULT;
CREATE TABLE public.sales_default                     PARTITION OF public.sales                     DEFAULT;
CREATE TABLE public.test_partitions_sequences_default PARTITION OF public.test_partitions_sequences DEFAULT;
