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


CREATE COLLATION collation_ex.bad_us (provider = libc, locale = 'en_US.utf8');


CREATE COLLATION collation_ex.german_phonebook (provider = icu, locale = 'de-u-co-phonebk');


CREATE COLLATION collation_ex.us (provider = libc, locale = 'en_US.utf8');


