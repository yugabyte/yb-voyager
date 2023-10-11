-- populating all objects in public
/*****************************/
-- contains Aggregates, Procedures, triggers, functions, extensions, inline comments

\i schema.sql

/*******************************************************/
-- Creating and populating the second schema
/*******************************************************/

drop schema if exists schema2 cascade;

Create schema schema2;

set search_path to schema2;

\i schema.sql
