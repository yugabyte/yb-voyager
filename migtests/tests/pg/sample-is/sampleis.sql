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
-- Name: btree_gist; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS btree_gist WITH SCHEMA public;


--
-- Name: dblink; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS dblink WITH SCHEMA public;


--
-- Name: hstore; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS hstore WITH SCHEMA public;


--
-- Name: uuid-ossp; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;


--
-- Name: currency; Type: DOMAIN; Schema: public; Owner: -
--

CREATE DOMAIN public.currency AS numeric(10,2);


SET default_table_access_method = heap;

--
-- Name: agent_statuses; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.agent_statuses (
    agent_uuid uuid,
    state text,
    "time" timestamp with time zone
);


--
-- Name: agents; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.agents (
    uuid uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    name text,
    birth date,
    affiliation text,
    tags text[]
);


--
-- Name: countries; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.countries (
    name text NOT NULL
);


--
-- Name: expenses; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.expenses (
    agent_uuid uuid,
    incurred date,
    price public.currency,
    name text
);


--
-- Name: expensive_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.expensive_items (
    item text
);


--
-- Name: gear_names; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.gear_names (
    name text
);


--
-- Name: points; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.points (
    x double precision,
    y double precision
);


--
-- Name: reports; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.reports (
    agent_uuid uuid,
    "time" timestamp with time zone,
    attrs public.hstore DEFAULT ''::public.hstore,
    report text,
    report_tsv tsvector
);


--
-- Name: secret_missions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.secret_missions (
    operation_name text NOT NULL,
    agent_uuid uuid,
    location text,
    mission_timeline tstzrange
);


--
-- Name: agents agents_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agents
    ADD CONSTRAINT agents_pkey PRIMARY KEY (uuid);


--
-- Name: secret_missions cnt_solo_agent; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.secret_missions
    ADD CONSTRAINT cnt_solo_agent EXCLUDE USING gist (location WITH =, mission_timeline WITH &&);


--
-- Name: countries countries_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.countries
    ADD CONSTRAINT countries_pkey PRIMARY KEY (name);


--
-- Name: secret_missions secret_missions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.secret_missions
    ADD CONSTRAINT secret_missions_pkey PRIMARY KEY (operation_name);


--
-- Name: reports_attrs_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX reports_attrs_idx ON public.reports USING gin (attrs);


--
-- Name: reports_report_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX reports_report_idx ON public.reports USING gin (report_tsv);


--
-- Name: reports report_tsv_update; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER report_tsv_update BEFORE INSERT OR UPDATE ON public.reports FOR EACH ROW EXECUTE FUNCTION tsvector_update_trigger('report_tsv', 'pg_catalog.english', 'report');


--
-- Name: secret_missions fk_secret_mission_agent; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.secret_missions
    ADD CONSTRAINT fk_secret_mission_agent FOREIGN KEY (agent_uuid) REFERENCES public.agents(uuid);


--
-- Name: secret_missions fk_secret_mission_location; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.secret_missions
    ADD CONSTRAINT fk_secret_mission_location FOREIGN KEY (location) REFERENCES public.countries(name);


--
-- PostgreSQL database dump complete
--

