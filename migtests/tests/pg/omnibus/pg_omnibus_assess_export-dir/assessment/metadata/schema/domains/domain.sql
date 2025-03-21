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


CREATE DOMAIN base_type_examples.myvarchardom AS base_type_examples.myvarchar;


CREATE DOMAIN domain_examples.positive_number AS integer
	CONSTRAINT should_be_positive CHECK (domain_examples.is_positive(VALUE));


CREATE DOMAIN domain_examples.positive_even_number AS domain_examples.positive_number
	CONSTRAINT should_be_even CHECK (domain_examples.is_even(VALUE));


CREATE DOMAIN domain_examples.us_postal_code AS text
	CONSTRAINT check_postal_code_regex CHECK (((VALUE ~ '^\d{5}$'::text) OR (VALUE ~ '^\d{5}-\d{4}$'::text)));


CREATE DOMAIN foreign_db_example.positive_number AS integer NOT NULL
	CONSTRAINT positive_number_check CHECK (foreign_db_example.is_positive(VALUE));


