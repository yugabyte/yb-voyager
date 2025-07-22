-- setting variables for current session
SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;



CREATE INDEX created_idx ON public.osm_changeset USING btree (created_at ASC);


CREATE INDEX tags_idx ON public.osm_changeset USING gin (tags);


CREATE INDEX user_id_idx ON public.osm_changeset USING btree (user_id ASC);


CREATE INDEX user_name_idx ON public.osm_changeset USING btree (user_name ASC);


