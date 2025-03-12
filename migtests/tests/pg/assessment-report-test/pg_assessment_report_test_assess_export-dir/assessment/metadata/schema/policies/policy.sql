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


CREATE POLICY policy_test_fine ON public.test_exclude_basic USING (((id % 2) = 1));


CREATE POLICY policy_test_fine_2 ON public.employees2 USING ((id <> ALL (ARRAY[12, 123, 41241])));


CREATE POLICY policy_test_report ON public.test_xml_type TO test_policy USING (true);


CREATE POLICY policy_test_report ON schema2.test_xml_type TO test_policy USING (true);


