/*
ERROR: could not open extension control file "/home/yugabyte/yb-software/yugabyte-2024.1.1.0-b93-centos-x86_64/postgres/share/extension/postgis.control": No such file or directory (SQLSTATE 58P01)
File :/home/centos/yb-voyager/migtests/tests/pg/osm/export-dir/schema/extensions/extension.sql
*/
CREATE EXTENSION IF NOT EXISTS postgis WITH SCHEMA public;

/*
ERROR: type "public.geometry" does not exist (SQLSTATE 42704)
File :/home/centos/yb-voyager/migtests/tests/pg/osm/export-dir/schema/tables/table.sql
*/
CREATE TABLE public.osm_changeset (id bigint NOT NULL, user_id bigint, created_at timestamp, min_lat numeric(10, 7), max_lat numeric(10, 7), min_lon numeric(10, 7), max_lon numeric(10, 7), closed_at timestamp, open boolean, num_changes int, user_name varchar(255), tags public.hstore, geom public.geometry(polygon, 4326), CONSTRAINT osm_changeset_pkey PRIMARY KEY (id)) WITH (colocation=false);

/*
ERROR: relation "public.osm_changeset" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/osm/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX changeset_geom_gist ON public.osm_changeset USING gist (geom);

/*
ERROR: relation "public.osm_changeset" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/osm/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX created_idx ON public.osm_changeset USING btree (created_at ASC);

/*
ERROR: relation "public.osm_changeset" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/osm/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX tags_idx ON public.osm_changeset USING gin (tags);

/*
ERROR: relation "public.osm_changeset" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/osm/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX user_id_idx ON public.osm_changeset USING btree (user_id ASC);

/*
ERROR: relation "public.osm_changeset" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/osm/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX user_name_idx ON public.osm_changeset USING btree (user_name ASC);

