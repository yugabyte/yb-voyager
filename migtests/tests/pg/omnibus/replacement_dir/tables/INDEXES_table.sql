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


-- CREATE INDEX grect2ind2 ON am_examples.fast_emp4000 USING gist2 (home_base);

-- Index on column of UDT is not supported

-- CREATE INDEX idx_1 ON composite_type_examples.ordinary_table USING btree (basic_);

CREATE INDEX gin_idx ON idx_ex.films USING gin (to_tsvector('english'::regconfig, title));


CREATE INDEX title_idx_lower ON idx_ex.films USING btree (lower(title));


CREATE INDEX title_idx_nulls_low ON idx_ex.films USING btree (title NULLS FIRST);


CREATE UNIQUE INDEX title_idx_u2 ON idx_ex.films USING btree (title) INCLUDE (director, rating);


