/*
ERROR: index method "gist" not supported yet (SQLSTATE XX000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX film_fulltext_idx ON public.film USING gist (fulltext);

