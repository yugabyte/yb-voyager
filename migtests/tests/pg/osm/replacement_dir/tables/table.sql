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
    geom jsonb
);


CREATE TABLE public.osm_changeset_comment (
    comment_changeset_id bigint NOT NULL,
    comment_user_id bigint NOT NULL,
    comment_user_name character varying(255) NOT NULL,
    comment_date timestamp without time zone NOT NULL,
    comment_text text NOT NULL
);


CREATE TABLE public.osm_changeset_state (
    last_sequence bigint,
    last_timestamp timestamp without time zone,
    update_in_progress smallint
);


ALTER TABLE ONLY public.osm_changeset
    ADD CONSTRAINT osm_changeset_pkey PRIMARY KEY (id);


