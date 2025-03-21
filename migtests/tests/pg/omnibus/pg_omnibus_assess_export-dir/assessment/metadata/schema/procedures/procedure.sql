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


CREATE PROCEDURE fn_examples.insert_to_table()
    LANGUAGE plpgsql
    AS $$
  BEGIN
    IF fn_examples.is_even(2) THEN
      INSERT INTO fn_examples.ordinary_table(id) VALUES (1);
    END IF;
  END;
$$;


