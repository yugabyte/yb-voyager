-- running with role postgres with admin privileges
-- https://www.postgresql.org/docs/current/sql-createrole.html
-- https://www.postgresql.org/docs/current/sql-createuser.html
-- users and roles are global in the database server, and possibly the db cluster
-- user and group are aliases for role.
CREATE ROLE super_user SUPERUSER;
CREATE ROLE normal_user NOSUPERUSER;

CREATE ROLE db_creator CREATEDB;
CREATE ROLE db_consumer NOCREATEDB;

CREATE ROLE role_progenitor CREATEROLE;
CREATE ROLE no_create_role NOCREATEROLE;

CREATE ROLE replicant REPLICATION;
CREATE ROLE blade_runner NOREPLICATION;

CREATE ROLE no_rls BYPASSRLS;
CREATE ROLE rls_enforced NOBYPASSRLS;

CREATE ROLE inheritor INHERIT IN ROLE replicant, normal_user, db_creator, role_progenitor;
CREATE ROLE nope NOINHERIT IN ROLE replicant, normal_user, db_creator, role_progenitor;

CREATE GROUP example_users;

CREATE USER alice IN GROUP example_users;
CREATE ROLE bob LOGIN IN GROUP example_users;
CREATE USER carl IN GROUP example_users;
