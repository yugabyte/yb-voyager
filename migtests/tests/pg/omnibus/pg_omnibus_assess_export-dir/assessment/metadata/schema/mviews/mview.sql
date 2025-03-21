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


CREATE MATERIALIZED VIEW am_examples.heapmv AS
 SELECT a,
    repeat
   FROM am_examples.heaptable
  WITH NO DATA;


CREATE MATERIALIZED VIEW am_examples.tableam_tblmv_heapx AS
 SELECT f1
   FROM am_examples.tableam_tbl_heapx
  WITH NO DATA;


