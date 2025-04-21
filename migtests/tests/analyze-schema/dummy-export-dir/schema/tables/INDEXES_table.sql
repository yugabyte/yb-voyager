--gist indexes 
CREATE INDEX film_fulltext_idx ON public.film USING gist (fulltext);

CREATE INDEX idx_actor_last_name ON public.actor USING btree (last_name);
CREATE INDEX idx_name1 ON table_name USING spgist (col1);
CREATE INDEX idx_name2 ON table_name USING rtree (col1);

--gin index
CREATE INDEX idx_name3 ON schema_name.table_name USING gin (col1,col2,col3);

--valid case
CREATE INDEX idx_fileinfo_name_splitted ON public.fileinfo USING gin (to_tsvector('english'::regconfig, translate((name)::text, '.,-'::text, ' '::text)));


--dropping multiple objects
DROP INDEX idx1,idx2,idx3;

--dropping index concurrentlynot support
DROP INDEX CONCURRENTLY sales_quantity_index;

--index access method not supported
CREATE ACCESS METHOD heptree TYPE INDEX HANDLER heptree_handler;

--alter index case
ALTER INDEX abc set TABLESPACE new_tbl;

--for the case of storage parameters
CREATE INDEX abc ON public.example USING btree (new_id) WITH (fillfactor='70');

-- for the duplicate index name
CREATE INDEX abc ON schema2.example USING btree (new_id) WITH (fillfactor='70'); 


--normal indexes on column with types not supported
CREATE INDEX tsvector_idx ON public.documents  (title_tsvector, id);

CREATE INDEX tsquery_idx ON public.ts_query_table (query);

CREATE INDEX idx_citext ON public.citext_type USING btree (data);

CREATE INDEX idx_inet ON public.inet_type USING btree (data);

CREATE INDEX idx_json ON public.test_jsonb (data);

-- expression index casting to types not supported
CREATE INDEX idx_json2 ON public.test_jsonb ((data2::jsonb)); 

-- valid case
create index idx_valid on public.test_jsonb ((data::text)); 

create index idx_array on public.documents (list_of_sections);

--indexes on misc datatypes
CREATE index idx1 on combined_tbl (c);

CREATE index idx2 on combined_tbl (ci);

CREATE index idx3 on combined_tbl (b);

CREATE index idx4 on combined_tbl (j);

CREATE index idx5 on combined_tbl (l);

CREATE index idx6 on combined_tbl (ls);

CREATE index idx7 on combined_tbl (maddr);

CREATE index idx8 on combined_tbl (maddr8);

CREATE index idx9 on combined_tbl (p);

CREATE index idx10 on combined_tbl (lsn);

CREATE index idx11 on combined_tbl (p1);

CREATE index idx12 on combined_tbl (p2);

CREATE index idx13 on combined_tbl (id1);

CREATE INDEX idx14 on combined_tbl (bitt);

CREATE INDEX idx15 on combined_tbl (bittv);

CREATE INDEX idx1 on combined_tbl1 (d);

CREATE INDEX idx2 on combined_tbl1 (t);

CREATE INDEX idx3 on combined_tbl1 (tz);

CREATE INDEX idx4 on combined_tbl1 (n);

CREATE INDEX idx5 on combined_tbl1 (i4);

CREATE INDEX idx6 on combined_tbl1 (i8);

CREATE INDEX idx7  on combined_tbl1 (inym);

CREATE INDEX idx8  on combined_tbl1 (inds);

CREATE INDEX idx_udt on test_udt(home_address);

CREATE INDEX idx_udt1 on test_udt(home_address1);

CREATE INDEX idx_enum on test_udt(some_field);

CREATE INDEX "idx&_enum2" on test_udt((some_field::non_public.enum_test));

-- Create a unique index on a column with NULLs with the NULLS NOT DISTINCT option
CREATE UNIQUE INDEX users_unique_nulls_not_distinct_index_email
    ON users_unique_nulls_not_distinct_index (email)
    NULLS NOT DISTINCT;

-- Indexes on timestamp/date
CREATE INDEX idx_sales on sales(bill_date);

