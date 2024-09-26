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


CREATE TRIGGER report_tsv_update BEFORE INSERT OR UPDATE ON public.reports FOR EACH ROW EXECUTE FUNCTION tsvector_update_trigger('report_tsv', 'pg_catalog.english', 'report');

CREATE OR REPLACE FUNCTION public.check_exclusion_constraints()
RETURNS TRIGGER AS $$
BEGIN
    -- Check for overlapping mission_timeline and duplicate location
    PERFORM 1
    FROM public.secret_missions
    WHERE location = NEW.location
      AND NEW.mission_timeline && mission_timeline
      AND operation_name <> NEW.operation_name; -- Exclude the current row in case of update

    IF FOUND THEN
        RAISE EXCEPTION 'Location % already has a mission within the given timeline %', NEW.location, NEW.mission_timeline;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_check_exclusion_constraints
BEFORE INSERT OR UPDATE ON public.secret_missions
FOR EACH ROW EXECUTE FUNCTION check_exclusion_constraints();




