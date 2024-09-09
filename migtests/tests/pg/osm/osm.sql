--
-- PostgreSQL database dump
--

-- Dumped from database version 12.14
-- Dumped by pg_dump version 14.12

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: public; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA public;


--
-- Name: hstore; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS hstore WITH SCHEMA public;


--
-- Name: postgis; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS postgis WITH SCHEMA public;


--
-- Name: insert_osm_data(integer); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.insert_osm_data(num_rows integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
    i INTEGER;
    max_changeset_id BIGINT;
    max_comment_id BIGINT;
    max_state_sequence BIGINT;
BEGIN
    -- Get the maximum ID currently in the osm_changeset table
    SELECT COALESCE(MAX(id), 0) INTO max_changeset_id FROM osm_changeset;

    -- Get the maximum comment_changeset_id currently in the osm_changeset_comment table
    SELECT COALESCE(MAX(comment_changeset_id), 0) INTO max_comment_id FROM osm_changeset_comment;

    -- Get the maximum sequence currently in the osm_changeset_state table
    SELECT COALESCE(MAX(last_sequence), 0) INTO max_state_sequence FROM osm_changeset_state;

    -- Insert into osm_changeset
    FOR i IN 1..num_rows LOOP
        INSERT INTO osm_changeset (id, user_id, created_at, min_lat, max_lat, min_lon, max_lon, closed_at, open, num_changes, user_name, tags)
        VALUES (
            max_changeset_id + i, 
            max_changeset_id + i + 100, 
            NOW() - (i * INTERVAL '1 day'), 
            10.0 + i, 
            20.0 + i, 
            30.0 + i, 
            40.0 + i, 
            NOW() - (i * INTERVAL '1 hour'), 
            i % 2 = 0, 
            i * 10, 
            'user' || i, 
            hstore('type', 'type' || i) || hstore('status', CASE WHEN i % 2 = 0 THEN 'completed' ELSE 'in_progress' END)
        );
    END LOOP;

    -- Insert into osm_changeset_comment
    FOR i IN 1..num_rows LOOP
        INSERT INTO osm_changeset_comment (comment_changeset_id, comment_user_id, comment_user_name, comment_date, comment_text)
        VALUES (
            max_comment_id + i, 
            max_comment_id + i + 200, 
            'commenter' || i, 
            NOW() - (i * INTERVAL '1 day'), 
            'This is comment ' || i
        );
    END LOOP;

    -- Insert into osm_changeset_state
    FOR i IN 1..num_rows LOOP
        INSERT INTO osm_changeset_state (last_sequence, last_timestamp, update_in_progress)
        VALUES (
            max_state_sequence + i * 1000, 
            NOW() - (i * INTERVAL '1 day'), 
            i % 2
        );
    END LOOP;
END;
$$;


SET default_table_access_method = heap;

--
-- Name: osm_changeset; Type: TABLE; Schema: public; Owner: -
--

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


--
-- Name: osm_changeset_comment; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.osm_changeset_comment (
    comment_changeset_id bigint NOT NULL,
    comment_user_id bigint NOT NULL,
    comment_user_name character varying(255) NOT NULL,
    comment_date timestamp without time zone NOT NULL,
    comment_text text NOT NULL
);


--
-- Name: osm_changeset_state; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.osm_changeset_state (
    last_sequence bigint,
    last_timestamp timestamp without time zone,
    update_in_progress smallint
);


--
-- Name: osm_changeset osm_changeset_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.osm_changeset
    ADD CONSTRAINT osm_changeset_pkey PRIMARY KEY (id);


--
-- Name: changeset_geom_gist; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX changeset_geom_gist ON public.osm_changeset USING gist (geom);


--
-- Name: created_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX created_idx ON public.osm_changeset USING btree (created_at);


--
-- Name: tags_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX tags_idx ON public.osm_changeset USING gin (tags);


--
-- Name: user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_id_idx ON public.osm_changeset USING btree (user_id);


--
-- Name: user_name_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_name_idx ON public.osm_changeset USING btree (user_name);


--
-- PostgreSQL database dump complete
--

