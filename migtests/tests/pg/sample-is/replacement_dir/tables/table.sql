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


CREATE TABLE public.agent_statuses (
    agent_uuid uuid,
    state text,
    "time" timestamp with time zone
);


CREATE TABLE public.agents (
    uuid uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    name text,
    birth date,
    affiliation text,
    tags text[]
);


CREATE TABLE public.countries (
    name text NOT NULL
);


CREATE TABLE public.expenses (
    agent_uuid uuid,
    incurred date,
    price public.currency,
    name text
);


CREATE TABLE public.expensive_items (
    item text
);


CREATE TABLE public.gear_names (
    name text
);


CREATE TABLE public.points (
    x double precision,
    y double precision
);


CREATE TABLE public.reports (
    agent_uuid uuid,
    "time" timestamp with time zone,
    attrs public.hstore DEFAULT ''::public.hstore,
    report text,
    report_tsv tsvector
);


CREATE TABLE public.secret_missions (
    operation_name text NOT NULL,
    agent_uuid uuid,
    location text,
    mission_timeline tstzrange
);


ALTER TABLE ONLY public.agents
    ADD CONSTRAINT agents_pkey PRIMARY KEY (uuid);


ALTER TABLE ONLY public.countries
    ADD CONSTRAINT countries_pkey PRIMARY KEY (name);


ALTER TABLE ONLY public.secret_missions
    ADD CONSTRAINT secret_missions_pkey PRIMARY KEY (operation_name);


ALTER TABLE ONLY public.secret_missions
    ADD CONSTRAINT fk_secret_mission_agent FOREIGN KEY (agent_uuid) REFERENCES public.agents(uuid);


ALTER TABLE ONLY public.secret_missions
    ADD CONSTRAINT fk_secret_mission_location FOREIGN KEY (location) REFERENCES public.countries(name);


