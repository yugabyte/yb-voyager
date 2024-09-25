-- populating all objects in public
/*****************************/

\i snapshot_schema.sql

/*******************************************************/
-- Creating and populating the second schema
/*******************************************************/

DROP SCHEMA IF EXISTS non_public cascade;
CREATE SCHEMA non_public;
SET search_path to non_public;

\i snapshot_schema.sql

