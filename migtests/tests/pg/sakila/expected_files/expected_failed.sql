/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/table.sql
*/
CREATE TABLE public.payment_p2007_01 (CONSTRAINT payment_p2007_01_payment_date_check CHECK (payment_date >= '2007-01-01 00:00:00'::timestamp AND payment_date < '2007-02-01 00:00:00'::timestamp)) INHERITS (public.payment);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/table.sql
*/
CREATE TABLE public.payment_p2007_02 (CONSTRAINT payment_p2007_02_payment_date_check CHECK (payment_date >= '2007-02-01 00:00:00'::timestamp AND payment_date < '2007-03-01 00:00:00'::timestamp)) INHERITS (public.payment);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/table.sql
*/
CREATE TABLE public.payment_p2007_03 (CONSTRAINT payment_p2007_03_payment_date_check CHECK (payment_date >= '2007-03-01 00:00:00'::timestamp AND payment_date < '2007-04-01 00:00:00'::timestamp)) INHERITS (public.payment);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/table.sql
*/
CREATE TABLE public.payment_p2007_04 (CONSTRAINT payment_p2007_04_payment_date_check CHECK (payment_date >= '2007-04-01 00:00:00'::timestamp AND payment_date < '2007-05-01 00:00:00'::timestamp)) INHERITS (public.payment);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/table.sql
*/
CREATE TABLE public.payment_p2007_05 (CONSTRAINT payment_p2007_05_payment_date_check CHECK (payment_date >= '2007-05-01 00:00:00'::timestamp AND payment_date < '2007-06-01 00:00:00'::timestamp)) INHERITS (public.payment);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/table.sql
*/
CREATE TABLE public.payment_p2007_06 (CONSTRAINT payment_p2007_06_payment_date_check CHECK (payment_date >= '2007-06-01 00:00:00'::timestamp AND payment_date < '2007-07-01 00:00:00'::timestamp)) INHERITS (public.payment);

/*
ERROR: index method "gist" not supported yet (SQLSTATE XX000)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX film_fulltext_idx ON public.film USING gist (fulltext);

/*
ERROR: relation "public.payment_p2007_01" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY public.payment_p2007_01 ALTER COLUMN payment_id SET DEFAULT nextval('public.payment_payment_id_seq'::regclass);

/*
ERROR: relation "public.payment_p2007_02" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY public.payment_p2007_02 ALTER COLUMN payment_id SET DEFAULT nextval('public.payment_payment_id_seq'::regclass);

/*
ERROR: relation "public.payment_p2007_03" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY public.payment_p2007_03 ALTER COLUMN payment_id SET DEFAULT nextval('public.payment_payment_id_seq'::regclass);

/*
ERROR: relation "public.payment_p2007_04" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY public.payment_p2007_04 ALTER COLUMN payment_id SET DEFAULT nextval('public.payment_payment_id_seq'::regclass);

/*
ERROR: relation "public.payment_p2007_05" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY public.payment_p2007_05 ALTER COLUMN payment_id SET DEFAULT nextval('public.payment_payment_id_seq'::regclass);

/*
ERROR: relation "public.payment_p2007_06" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY public.payment_p2007_06 ALTER COLUMN payment_id SET DEFAULT nextval('public.payment_payment_id_seq'::regclass);

/*
ERROR: relation "public.payment_p2007_01" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX idx_fk_payment_p2007_01_customer_id ON public.payment_p2007_01 USING btree (customer_id ASC);

/*
ERROR: relation "public.payment_p2007_01" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX idx_fk_payment_p2007_01_staff_id ON public.payment_p2007_01 USING btree (staff_id ASC);

/*
ERROR: relation "public.payment_p2007_02" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX idx_fk_payment_p2007_02_customer_id ON public.payment_p2007_02 USING btree (customer_id ASC);

/*
ERROR: relation "public.payment_p2007_02" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX idx_fk_payment_p2007_02_staff_id ON public.payment_p2007_02 USING btree (staff_id ASC);

/*
ERROR: relation "public.payment_p2007_03" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX idx_fk_payment_p2007_03_customer_id ON public.payment_p2007_03 USING btree (customer_id ASC);

/*
ERROR: relation "public.payment_p2007_03" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX idx_fk_payment_p2007_03_staff_id ON public.payment_p2007_03 USING btree (staff_id ASC);

/*
ERROR: relation "public.payment_p2007_04" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX idx_fk_payment_p2007_04_customer_id ON public.payment_p2007_04 USING btree (customer_id ASC);

/*
ERROR: relation "public.payment_p2007_04" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX idx_fk_payment_p2007_04_staff_id ON public.payment_p2007_04 USING btree (staff_id ASC);

/*
ERROR: relation "public.payment_p2007_05" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX idx_fk_payment_p2007_05_customer_id ON public.payment_p2007_05 USING btree (customer_id ASC);

/*
ERROR: relation "public.payment_p2007_05" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX idx_fk_payment_p2007_05_staff_id ON public.payment_p2007_05 USING btree (staff_id ASC);

/*
ERROR: relation "public.payment_p2007_06" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX idx_fk_payment_p2007_06_customer_id ON public.payment_p2007_06 USING btree (customer_id ASC);

/*
ERROR: relation "public.payment_p2007_06" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX idx_fk_payment_p2007_06_staff_id ON public.payment_p2007_06 USING btree (staff_id ASC);

/*
ERROR: relation "public.payment_p2007_01" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/rules/rule.sql
*/
CREATE RULE payment_insert_p2007_01 AS
    ON INSERT TO public.payment
   WHERE ((new.payment_date >= '2007-01-01 00:00:00'::timestamp without time zone) AND (new.payment_date < '2007-02-01 00:00:00'::timestamp without time zone)) DO INSTEAD  INSERT INTO public.payment_p2007_01 (payment_id, customer_id, staff_id, rental_id, amount, payment_date)
  VALUES (DEFAULT, new.customer_id, new.staff_id, new.rental_id, new.amount, new.payment_date);

/*
ERROR: relation "public.payment_p2007_02" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/rules/rule.sql
*/
CREATE RULE payment_insert_p2007_02 AS
    ON INSERT TO public.payment
   WHERE ((new.payment_date >= '2007-02-01 00:00:00'::timestamp without time zone) AND (new.payment_date < '2007-03-01 00:00:00'::timestamp without time zone)) DO INSTEAD  INSERT INTO public.payment_p2007_02 (payment_id, customer_id, staff_id, rental_id, amount, payment_date)
  VALUES (DEFAULT, new.customer_id, new.staff_id, new.rental_id, new.amount, new.payment_date);

/*
ERROR: relation "public.payment_p2007_03" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/rules/rule.sql
*/
CREATE RULE payment_insert_p2007_03 AS
    ON INSERT TO public.payment
   WHERE ((new.payment_date >= '2007-03-01 00:00:00'::timestamp without time zone) AND (new.payment_date < '2007-04-01 00:00:00'::timestamp without time zone)) DO INSTEAD  INSERT INTO public.payment_p2007_03 (payment_id, customer_id, staff_id, rental_id, amount, payment_date)
  VALUES (DEFAULT, new.customer_id, new.staff_id, new.rental_id, new.amount, new.payment_date);

/*
ERROR: relation "public.payment_p2007_04" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/rules/rule.sql
*/
CREATE RULE payment_insert_p2007_04 AS
    ON INSERT TO public.payment
   WHERE ((new.payment_date >= '2007-04-01 00:00:00'::timestamp without time zone) AND (new.payment_date < '2007-05-01 00:00:00'::timestamp without time zone)) DO INSTEAD  INSERT INTO public.payment_p2007_04 (payment_id, customer_id, staff_id, rental_id, amount, payment_date)
  VALUES (DEFAULT, new.customer_id, new.staff_id, new.rental_id, new.amount, new.payment_date);

/*
ERROR: relation "public.payment_p2007_05" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/rules/rule.sql
*/
CREATE RULE payment_insert_p2007_05 AS
    ON INSERT TO public.payment
   WHERE ((new.payment_date >= '2007-05-01 00:00:00'::timestamp without time zone) AND (new.payment_date < '2007-06-01 00:00:00'::timestamp without time zone)) DO INSTEAD  INSERT INTO public.payment_p2007_05 (payment_id, customer_id, staff_id, rental_id, amount, payment_date)
  VALUES (DEFAULT, new.customer_id, new.staff_id, new.rental_id, new.amount, new.payment_date);

/*
ERROR: relation "public.payment_p2007_06" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/rules/rule.sql
*/
CREATE RULE payment_insert_p2007_06 AS
    ON INSERT TO public.payment
   WHERE ((new.payment_date >= '2007-06-01 00:00:00'::timestamp without time zone) AND (new.payment_date < '2007-07-01 00:00:00'::timestamp without time zone)) DO INSTEAD  INSERT INTO public.payment_p2007_06 (payment_id, customer_id, staff_id, rental_id, amount, payment_date)
  VALUES (DEFAULT, new.customer_id, new.staff_id, new.rental_id, new.amount, new.payment_date);

/*
ERROR: relation "public.payment_p2007_01" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY public.payment_p2007_01 ADD CONSTRAINT payment_p2007_01_customer_id_fkey FOREIGN KEY (customer_id) REFERENCES public.customer (customer_id);

/*
ERROR: relation "public.payment_p2007_01" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY public.payment_p2007_01 ADD CONSTRAINT payment_p2007_01_rental_id_fkey FOREIGN KEY (rental_id) REFERENCES public.rental (rental_id);

/*
ERROR: relation "public.payment_p2007_01" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY public.payment_p2007_01 ADD CONSTRAINT payment_p2007_01_staff_id_fkey FOREIGN KEY (staff_id) REFERENCES public.staff (staff_id);

/*
ERROR: relation "public.payment_p2007_02" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY public.payment_p2007_02 ADD CONSTRAINT payment_p2007_02_customer_id_fkey FOREIGN KEY (customer_id) REFERENCES public.customer (customer_id);

/*
ERROR: relation "public.payment_p2007_02" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY public.payment_p2007_02 ADD CONSTRAINT payment_p2007_02_rental_id_fkey FOREIGN KEY (rental_id) REFERENCES public.rental (rental_id);

/*
ERROR: relation "public.payment_p2007_02" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY public.payment_p2007_02 ADD CONSTRAINT payment_p2007_02_staff_id_fkey FOREIGN KEY (staff_id) REFERENCES public.staff (staff_id);

/*
ERROR: relation "public.payment_p2007_03" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY public.payment_p2007_03 ADD CONSTRAINT payment_p2007_03_customer_id_fkey FOREIGN KEY (customer_id) REFERENCES public.customer (customer_id);

/*
ERROR: relation "public.payment_p2007_03" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY public.payment_p2007_03 ADD CONSTRAINT payment_p2007_03_rental_id_fkey FOREIGN KEY (rental_id) REFERENCES public.rental (rental_id);

/*
ERROR: relation "public.payment_p2007_03" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY public.payment_p2007_03 ADD CONSTRAINT payment_p2007_03_staff_id_fkey FOREIGN KEY (staff_id) REFERENCES public.staff (staff_id);

/*
ERROR: relation "public.payment_p2007_04" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY public.payment_p2007_04 ADD CONSTRAINT payment_p2007_04_customer_id_fkey FOREIGN KEY (customer_id) REFERENCES public.customer (customer_id);

/*
ERROR: relation "public.payment_p2007_04" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY public.payment_p2007_04 ADD CONSTRAINT payment_p2007_04_rental_id_fkey FOREIGN KEY (rental_id) REFERENCES public.rental (rental_id);

/*
ERROR: relation "public.payment_p2007_04" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY public.payment_p2007_04 ADD CONSTRAINT payment_p2007_04_staff_id_fkey FOREIGN KEY (staff_id) REFERENCES public.staff (staff_id);

/*
ERROR: relation "public.payment_p2007_05" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY public.payment_p2007_05 ADD CONSTRAINT payment_p2007_05_customer_id_fkey FOREIGN KEY (customer_id) REFERENCES public.customer (customer_id);

/*
ERROR: relation "public.payment_p2007_05" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY public.payment_p2007_05 ADD CONSTRAINT payment_p2007_05_rental_id_fkey FOREIGN KEY (rental_id) REFERENCES public.rental (rental_id);

/*
ERROR: relation "public.payment_p2007_05" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY public.payment_p2007_05 ADD CONSTRAINT payment_p2007_05_staff_id_fkey FOREIGN KEY (staff_id) REFERENCES public.staff (staff_id);

/*
ERROR: relation "public.payment_p2007_06" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY public.payment_p2007_06 ADD CONSTRAINT payment_p2007_06_customer_id_fkey FOREIGN KEY (customer_id) REFERENCES public.customer (customer_id);

/*
ERROR: relation "public.payment_p2007_06" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY public.payment_p2007_06 ADD CONSTRAINT payment_p2007_06_rental_id_fkey FOREIGN KEY (rental_id) REFERENCES public.rental (rental_id);

/*
ERROR: relation "public.payment_p2007_06" does not exist (SQLSTATE 42P01)
File :/home/centos/yb-voyager/migtests/tests/pg/sakila/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY public.payment_p2007_06 ADD CONSTRAINT payment_p2007_06_staff_id_fkey FOREIGN KEY (staff_id) REFERENCES public.staff (staff_id);

