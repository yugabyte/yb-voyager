--gist indexes 
CREATE INDEX film_fulltext_idx ON public.film USING gist (fulltext);

CREATE INDEX idx_actor_last_name ON public.actor USING btree (last_name);
CREATE INDEX idx_name1 ON table_name USING spgist(col1);
CREATE INDEX idx_name2 ON table_name USING rtree(col1);

--gin index
CREATE INDEX idx_name3 ON schema_name.table_name USING gin(col1,col2,col3);

--dropping multiple objects
DROP INDEX idx1,idx2,idx3;

--dropping index concurrentlynot support
DROP INDEX CONCURRENTLY sales_quantity_index;

--index access method not supported
CREATE ACCESS METHOD heptree TYPE INDEX HANDLER heptree_handler;


--alter index case
ALTER INDEX abc set TABLESPACE new_tbl;