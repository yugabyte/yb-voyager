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


CREATE TRIGGER audit_trigger AFTER INSERT ON public.tt FOR EACH ROW EXECUTE FUNCTION public.auditlogfunc();


CREATE TRIGGER before_sales_region_insert_update BEFORE INSERT OR UPDATE ON public.sales_region FOR EACH ROW EXECUTE FUNCTION public.check_sales_region();


CREATE CONSTRAINT TRIGGER enforce_shipped_date_constraint AFTER UPDATE ON public.orders2 NOT DEFERRABLE INITIALLY IMMEDIATE FOR EACH ROW WHEN ((((new.status)::text = 'shipped'::text) AND (new.shipped_date IS NULL))) EXECUTE FUNCTION public.prevent_update_shipped_without_date();


CREATE TRIGGER t_raster BEFORE DELETE OR UPDATE ON public.combined_tbl FOR EACH ROW EXECUTE FUNCTION public.lo_manage('raster');


CREATE TRIGGER audit_trigger AFTER INSERT ON schema2.tt FOR EACH ROW EXECUTE FUNCTION schema2.auditlogfunc();


CREATE CONSTRAINT TRIGGER enforce_shipped_date_constraint AFTER UPDATE ON schema2.orders2 NOT DEFERRABLE INITIALLY IMMEDIATE FOR EACH ROW WHEN ((((new.status)::text = 'shipped'::text) AND (new.shipped_date IS NULL))) EXECUTE FUNCTION schema2.prevent_update_shipped_without_date();


