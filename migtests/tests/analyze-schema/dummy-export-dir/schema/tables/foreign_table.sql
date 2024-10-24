--foreign table issues
CREATE FOREIGN TABLE tbl_p(
	id int PRIMARY KEY
) SERVER remote_server
OPTIONS (
    schema_name 'public',
    table_name 'remote_table'
);


CREATE FOREIGN TABLE public.locations (
    id integer NOT NULL,
    name character varying(100),
    geom geometry(Point,4326)
 ) SERVER remote_server
OPTIONS (
    schema_name 'public',
    table_name 'remote_locations'
);

--Foreign key constraints on Foreign table is not supported in PG