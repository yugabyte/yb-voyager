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


CREATE FOREIGN TABLE foreign_db_example.technically_doesnt_exist (
    id integer,
    uses_type foreign_db_example.example_type,
    _uses_type foreign_db_example.example_type GENERATED ALWAYS AS (uses_type) STORED,
    positive_number foreign_db_example.positive_number,
    _positive_number foreign_db_example.positive_number GENERATED ALWAYS AS (positive_number) STORED,
    CONSTRAINT imaginary_table_id_gt_1 CHECK ((id > 1))
)
SERVER technically_this_server;


