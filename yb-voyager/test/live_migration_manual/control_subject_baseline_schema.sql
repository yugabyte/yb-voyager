-- Schema only: run on **PostgreSQL (source)** and **YugabyteDB (target)**.
-- Creates empty public.control_t / public.subject_t (no seed rows).
-- See control_subject_baseline.sql for how this fits the full reset flow.
\connect postgres

DROP DATABASE schema_drift;

CREATE DATABASE schema_drift;

\connect schema_drift

BEGIN;

DROP TABLE IF EXISTS public.subject_t CASCADE;
DROP TABLE IF EXISTS public.control_t CASCADE;

CREATE TABLE public.control_t (
	id   BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL,
	note TEXT
);

CREATE TABLE public.subject_t (
	id   BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL,
	note TEXT
);

ALTER TABLE public.control_t  REPLICA IDENTITY FULL;
ALTER TABLE public.subject_t REPLICA IDENTITY FULL;

COMMIT;
