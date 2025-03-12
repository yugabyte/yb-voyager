-- setting variables for current session
SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;


CREATE EXTENSION IF NOT EXISTS citext WITH SCHEMA public;


CREATE EXTENSION IF NOT EXISTS hstore WITH SCHEMA public;


CREATE EXTENSION IF NOT EXISTS lo WITH SCHEMA public;


CREATE EXTENSION IF NOT EXISTS pg_stat_statements WITH SCHEMA public;


CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;


