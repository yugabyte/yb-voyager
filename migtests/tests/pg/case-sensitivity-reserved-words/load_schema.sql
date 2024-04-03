\i schema.sql

drop schema if exists schema2 cascade;

create schema schema2;

set search_path to schema2;

\i schema.sql

