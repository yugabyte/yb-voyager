/*
ERROR: could not open extension control file "/home/yugabyte/yb-software/yugabyte-2024.1.1.0-b93-centos-x86_64/postgres/share/extension/postgis.control": No such file or directory (SQLSTATE 58P01)
File :/home/centos/yb-voyager/migtests/tests/pg/osm/export-dir/schema/extensions/extension.sql
*/
CREATE EXTENSION IF NOT EXISTS postgis WITH SCHEMA public;

/*
ERROR: type "public.geometry" does not exist (SQLSTATE 42704)
File :/home/centos/yb-voyager/migtests/tests/pg/osm/export-dir/schema/tables/table.sql
*/
CREATE TABLE public.osm_changeset (
    id bigint NOT NULL,
    user_id bigint,
    created_at timestamp without time zone,
    min_lat numeric(10,7),
    max_lat numeric(10,7),
    min_lon numeric(10,7),
    max_lon numeric(10,7),
    closed_at timestamp without time zone,
    open boolean,
    num_changes integer,
    user_name character varying(255),
    tags public.hstore,
    geom public.geometry(Polygon,4326)
);

/*
ERROR: relation "public.osm_changeset" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/osm/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY public.osm_changeset
    ADD CONSTRAINT osm_changeset_pkey PRIMARY KEY (id);

/*
ERROR: relation "public.osm_changeset" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/osm/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX changeset_geom_gist ON public.osm_changeset USING gist (geom);

/*
ERROR: relation "public.osm_changeset" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/osm/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX created_idx ON public.osm_changeset USING btree (created_at);

/*
ERROR: relation "public.osm_changeset" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/osm/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX tags_idx ON public.osm_changeset USING gin (tags);

/*
ERROR: relation "public.osm_changeset" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/osm/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX user_id_idx ON public.osm_changeset USING btree (user_id);

/*
ERROR: relation "public.osm_changeset" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/osm/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX user_name_idx ON public.osm_changeset USING btree (user_name);

