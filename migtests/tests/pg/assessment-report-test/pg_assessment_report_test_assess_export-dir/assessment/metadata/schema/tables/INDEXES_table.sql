-- setting variables for current session
SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;


CREATE INDEX idx1 ON public.combined_tbl USING btree (c);


CREATE INDEX idx2 ON public.combined_tbl USING btree (maddr);


CREATE INDEX idx3 ON public.combined_tbl USING btree (maddr8);


CREATE INDEX idx4 ON public.combined_tbl USING btree (lsn);


CREATE INDEX idx5 ON public.combined_tbl USING btree (bitt);


CREATE INDEX idx6 ON public.combined_tbl USING btree (bittv);


CREATE INDEX idx7 ON public.combined_tbl USING btree (address);


CREATE INDEX idx8 ON public.combined_tbl USING btree (d);


CREATE INDEX idx9 ON public.combined_tbl USING btree (inds3);


CREATE INDEX idx_array ON public.documents USING btree (list_of_sections);


CREATE INDEX idx_box_data ON public.mixed_data_types_table1 USING gist (box_data);


CREATE INDEX idx_box_data_brin ON public.mixed_data_types_table1 USING brin (box_data);


CREATE INDEX idx_citext ON public.citext_type USING btree (data);


CREATE INDEX idx_citext1 ON public.citext_type USING btree (lower((data)::text));


CREATE INDEX idx_citext2 ON public.citext_type USING btree (((data)::text));


CREATE INDEX idx_inet ON public.inet_type USING btree (data);


CREATE INDEX idx_inet1 ON public.inet_type USING btree (((data)::text));


CREATE INDEX idx_json ON public.test_jsonb USING btree (data);


CREATE INDEX idx_json2 ON public.test_jsonb USING btree (((data2)::jsonb));


CREATE INDEX idx_point_data ON public.mixed_data_types_table1 USING gist (point_data);


CREATE INDEX idx_valid ON public.test_jsonb USING btree (((data)::text));


CREATE INDEX tsquery_idx ON public.ts_query_table USING btree (query);


CREATE INDEX tsvector_idx ON public.documents USING btree (title_tsvector, id);


CREATE UNIQUE INDEX users_unique_nulls_not_distinct_index_email ON public.users_unique_nulls_not_distinct_index USING btree (email) NULLS NOT DISTINCT;


CREATE INDEX idx_box_data ON schema2.mixed_data_types_table1 USING gist (box_data);


CREATE INDEX idx_box_data_spgist ON schema2.mixed_data_types_table1 USING spgist (box_data);


CREATE INDEX idx_point_data ON schema2.mixed_data_types_table1 USING gist (point_data);


CREATE UNIQUE INDEX users_unique_nulls_not_distinct_index_email ON schema2.users_unique_nulls_not_distinct_index USING btree (email) NULLS NOT DISTINCT;


ALTER INDEX public.sales_region_pkey ATTACH PARTITION public.boston_pkey;


ALTER INDEX public.sales_region_pkey ATTACH PARTITION public.london_pkey;


ALTER INDEX public.sales_region_pkey ATTACH PARTITION public.sydney_pkey;


ALTER INDEX schema2.sales_region_pkey ATTACH PARTITION schema2.boston_pkey;


ALTER INDEX schema2.sales_region_pkey ATTACH PARTITION schema2.london_pkey;


ALTER INDEX schema2.sales_region_pkey ATTACH PARTITION schema2.sydney_pkey;


