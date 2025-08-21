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


CREATE INDEX film_fulltext_idx ON public.film USING gin (fulltext);


CREATE INDEX idx_actor_last_name ON public.actor USING btree (last_name ASC);


CREATE INDEX idx_fk_address_id ON public.customer USING btree (address_id ASC);


CREATE INDEX idx_fk_city_id ON public.address USING btree (city_id ASC);


CREATE INDEX idx_fk_country_id ON public.city USING btree (country_id ASC);


CREATE INDEX idx_fk_customer_id ON public.payment USING btree (customer_id ASC);


CREATE INDEX idx_fk_film_id ON public.film_actor USING btree (film_id ASC);


CREATE INDEX idx_fk_inventory_id ON public.rental USING btree (inventory_id ASC);


CREATE INDEX idx_fk_language_id ON public.film USING btree (language_id ASC);


CREATE INDEX idx_fk_original_language_id ON public.film USING btree (original_language_id ASC);


CREATE INDEX idx_fk_payment_p2007_01_customer_id ON public.payment_p2007_01 USING btree (customer_id ASC);


CREATE INDEX idx_fk_payment_p2007_01_staff_id ON public.payment_p2007_01 USING btree (staff_id ASC);


CREATE INDEX idx_fk_payment_p2007_02_customer_id ON public.payment_p2007_02 USING btree (customer_id ASC);


CREATE INDEX idx_fk_payment_p2007_02_staff_id ON public.payment_p2007_02 USING btree (staff_id ASC);


CREATE INDEX idx_fk_payment_p2007_03_customer_id ON public.payment_p2007_03 USING btree (customer_id ASC);


CREATE INDEX idx_fk_payment_p2007_03_staff_id ON public.payment_p2007_03 USING btree (staff_id ASC);


CREATE INDEX idx_fk_payment_p2007_04_customer_id ON public.payment_p2007_04 USING btree (customer_id ASC);


CREATE INDEX idx_fk_payment_p2007_04_staff_id ON public.payment_p2007_04 USING btree (staff_id ASC);


CREATE INDEX idx_fk_payment_p2007_05_customer_id ON public.payment_p2007_05 USING btree (customer_id ASC);


CREATE INDEX idx_fk_payment_p2007_05_staff_id ON public.payment_p2007_05 USING btree (staff_id ASC);


CREATE INDEX idx_fk_payment_p2007_06_customer_id ON public.payment_p2007_06 USING btree (customer_id ASC);


CREATE INDEX idx_fk_payment_p2007_06_staff_id ON public.payment_p2007_06 USING btree (staff_id ASC);


CREATE INDEX idx_fk_staff_id ON public.payment USING btree (staff_id ASC);


CREATE INDEX idx_fk_store_id ON public.customer USING btree (store_id ASC);


CREATE INDEX idx_last_name ON public.customer USING btree (last_name ASC);


CREATE INDEX idx_store_id_film_id ON public.inventory USING btree (store_id ASC, film_id ASC);


CREATE INDEX idx_title ON public.film USING btree (title ASC);


CREATE UNIQUE INDEX idx_unq_manager_staff_id ON public.store USING btree (manager_staff_id ASC);


CREATE UNIQUE INDEX idx_unq_rental_rental_date_inventory_id_customer_id ON public.rental USING btree (rental_date ASC, inventory_id ASC, customer_id ASC);


