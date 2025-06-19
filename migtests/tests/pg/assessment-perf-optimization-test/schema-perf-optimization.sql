--
-- Name: public; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA public;


--
-- Name: test; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test;



--
-- Name: Test_caseSensitive; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public."Test_caseSensitive" (
    id integer,
    "ValCaseSenstive" text
);


--
-- Name: t; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.t (
    id integer,
    id2 integer
);


--
-- Name: test_low_card; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.test_low_card (
    id integer,
    isactive boolean
);


--
-- Name: test_most_freq; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.test_most_freq (
    id integer,
    status text
);


--
-- Name: test_multi_col_idx; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.test_multi_col_idx (
    id integer,
    id1 integer,
    val text,
    id2 integer
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id integer NOT NULL,
    name character varying,
    employed boolean
);


--
-- Name: t; Type: TABLE; Schema: test; Owner: -
--

CREATE TABLE test.t (
    id integer,
    val text,
    val1 integer
);


--
-- Name: t_low_most; Type: TABLE; Schema: test; Owner: -
--

CREATE TABLE test.t_low_most (
    id integer,
    val1 integer
);

INSERT INTO public."Test_caseSensitive"  SELECT i%2, 'val_' || i from generate_series(1, 100) as i;

INSERT INTO public.t(id) SELECT i from generate_series(1,500) as i;

INSERT INTO test_low_card SELECT i, i%2 = 0 from generate_series(1,100) as i;

INSERT INTO public.test_most_freq
SELECT i, 
    CASE 
        WHEN random() < 0.65 THEN 'active'
        ELSE 'isactive'
    END
FROM generate_series(1, 100) AS i;


INSERT INTO public.test_multi_col_idx
SELECT i, 
    i%100,
    'val_' || i%50,
    i%10
FROM generate_series(1, 10000) AS i;

INSERT INTO public.users (id, name, employed)
SELECT
    gs.id,
    '',
    (random() < 0.5) AS employed  
FROM generate_series(1, 655) AS gs(id);

INSERT INTO public.users (id, name, employed)
SELECT
    gs.id,
    (ARRAY['Alice', 'Bob', 'Charlie', 'Diana', 'Eve', 'Frank', 'Grace', 'acb', 'asfa', 'John', 'Steve', 'Rob'])[floor(random() * 12 + 1)::int] AS name,
    (random() < 0.5) AS employed  
FROM generate_series(656, 1000) AS gs(id);


INSERT INTO test.t(id, val, val1) 
SELECT i,
    'val_' || (i%2),
    CASE
        WHEN i%2 = 0 THEN 121
        ELSE NULL
    END AS val1
from generate_series(1,500) as i;

INSERT INTO test.t_low_most 
SELECT i%2, i from generate_series(1,500) as i;



--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx ON public.test_low_card USING btree (isactive) WHERE (isactive = false);


--
-- Name: idx_muli_col_key_first_col; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_muli_col_key_first_col ON public.test_multi_col_idx USING btree ((((id1 || '_'::text) || id2)), val);


--
-- Name: idx_muli_col_key_first_col1; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_muli_col_key_first_col1 ON public.test_multi_col_idx USING btree ((((id1 || '_'::text) || id2)));


--
-- Name: idx_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_name ON public.users USING btree (name);


CREATE INDEX idx_name1 ON public.users USING btree (name) where name != '';-- this is most common value partial index 


--
-- Name: idx_test_case_sensitive; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_test_case_sensitive ON public."Test_caseSensitive" USING btree (id);


--
-- Name: idx_test_multi_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_test_multi_id ON public.test_multi_col_idx USING btree (id);


--
-- Name: idx_test_multi_id1; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_test_multi_id1 ON public.test_multi_col_idx USING btree (id1);


--
-- Name: idx_test_multi_id1_id2; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_test_multi_id1_id2 ON public.test_multi_col_idx USING btree (id1, id2);


--
-- Name: idx_test_multi_id2; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_test_multi_id2 ON public.test_multi_col_idx USING btree (id2);


--
-- Name: idx_test_multi_val; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_test_multi_val ON public.test_multi_col_idx USING btree (val);


--
-- Name: idxtt; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idxtt ON public.t USING btree (id2);

CREATE INDEX idxtt1 ON public.t USING btree (id2) where id2 IS NOT NULL; -- this is null partial index 


--
-- Name: indx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX indx ON public.test_most_freq USING btree (status) WHERE (status <> 'active'::text);


--
-- Name: indx1; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX indx1 ON public.test_most_freq USING btree (status, id);


--
-- Name: indx34; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX indx34 ON public.test_most_freq USING btree (id) WHERE (status = 'active'::text);

CREATE TABLE test.test_mostcomm_status (
    id integer,
    status text
);
INSERT INTO test.test_mostcomm_status
SELECT i,
    CASE
        WHEN random() <= 0.65 THEN 'DONE'
        ELSE (ARRAY['FAILED','COMPLETED','PROGRESS','PENDING','RUNNING','CANCELLED','QUEUED','SUCCESS','ERROR','SKIPPED',  'RETRY'])[floor(random() * 11 + 1)::int]
    END
FROM generate_series(1, 100) AS i;

CREATE INDEX idx_partial_high ON test.test_mostcomm_status USING btree (status) WHERE status NOT IN ('DONE');

CREATE INDEX idx_normal ON test.test_mostcomm_status USING btree (status);

--
-- Name: idx; Type: INDEX; Schema: test; Owner: -
--

CREATE INDEX idx ON test.t USING btree (val);


--
-- Name: idx2; Type: INDEX; Schema: test; Owner: -
--

CREATE INDEX idx2 ON test.t USING btree (val1);


--
-- Name: idx_try; Type: INDEX; Schema: test; Owner: -
--

CREATE INDEX idx_try ON test.t_low_most USING btree (id);


--
-- PostgreSQL database dump complete
--

ANALYZE;