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

INSERT INTO public.test_most_freq Values (1, 'active');
INSERT INTO public.test_most_freq Values (2, 'isactive');
INSERT INTO public.test_most_freq Values (3, 'isactive');
INSERT INTO public.test_most_freq Values (4, 'isactive');
INSERT INTO public.test_most_freq Values (5, 'active');
INSERT INTO public.test_most_freq Values (6, 'active');
INSERT INTO public.test_most_freq Values (7, 'active');
INSERT INTO public.test_most_freq Values (8, 'active');
INSERT INTO public.test_most_freq Values (9, 'active');
INSERT INTO public.test_most_freq Values (10, 'active');
INSERT INTO public.test_most_freq Values (11, 'active');
INSERT INTO public.test_most_freq Values (12, 'isactive');
INSERT INTO public.test_most_freq Values (13, 'isactive');
INSERT INTO public.test_most_freq Values (14, 'isactive');
INSERT INTO public.test_most_freq Values (15, 'active');
INSERT INTO public.test_most_freq Values (16, 'active');
INSERT INTO public.test_most_freq Values (17, 'active');
INSERT INTO public.test_most_freq Values (18, 'active');
INSERT INTO public.test_most_freq Values (19, 'active');
INSERT INTO public.test_most_freq Values (20, 'active');
INSERT INTO public.test_most_freq Values (21, 'active');
INSERT INTO public.test_most_freq Values (22, 'isactive');
INSERT INTO public.test_most_freq Values (23, 'isactive');
INSERT INTO public.test_most_freq Values (24, 'isactive');
INSERT INTO public.test_most_freq Values (25, 'active');
INSERT INTO public.test_most_freq Values (26, 'active');
INSERT INTO public.test_most_freq Values (27, 'active');
INSERT INTO public.test_most_freq Values (28, 'active');
INSERT INTO public.test_most_freq Values (29, 'active');
INSERT INTO public.test_most_freq Values (30, 'active');

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
SELECT i, 'DONE' 
FROM generate_series(1, 65) AS i;

INSERT INTO test.test_mostcomm_status
SELECT i,  (ARRAY['FAILED','COMPLETED','PROGRESS','PENDING','RUNNING','CANCELLED','QUEUED','SUCCESS','ERROR','SKIPPED',  'RETRY'])[floor(random() * 11 + 1)::int]
FROM generate_series(66, 100) AS i;

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

CREATE TABLE tbl(id int);

INSERT INTO tbl(id)
SELECT i%2 FROM generate_series(1, 1000) AS i;

CREATE INDEX idx_try on public.tbl(id);

CREATE OR REPLACE FUNCTION test_perf_plpgsql()
RETURNS VOID AS $$
BEGIN
	CREATE INDEX idx_try1 on public.tbl(id);
END;
$$ LANGUAGE plpgsql;

--
-- PK Recommendation Test Cases
--

-- Regular table without PK but with qualifying UNIQUE constraint
CREATE TABLE public.no_pk_table (
    id integer NOT NULL,
    name text NOT NULL,
    email text,
    UNIQUE(id, name)
);

-- Partitioned table without PK but with qualifying UNIQUE constraint
CREATE TABLE public.partitioned_no_pk (
    id integer NOT NULL,
    region text NOT NULL,
    year integer NOT NULL,
    amount numeric,
    UNIQUE(id, region, year)
) PARTITION BY RANGE (year);

CREATE TABLE public.partitioned_no_pk_2023 PARTITION OF public.partitioned_no_pk 
FOR VALUES FROM (2023) TO (2024) PARTITION BY LIST (region);

CREATE TABLE public.partitioned_no_pk_2023_us PARTITION OF public.partitioned_no_pk_2023 
FOR VALUES IN ('US');

-- Multi-level partitioned table without PK
CREATE TABLE public.sales_hierarchy (
    id integer NOT NULL,
    region text NOT NULL,
    year integer NOT NULL,
    quarter text NOT NULL,
    amount numeric,
    UNIQUE(id, region, year, quarter)
) PARTITION BY RANGE (year);

CREATE TABLE public.sales_hierarchy_2023 PARTITION OF public.sales_hierarchy 
FOR VALUES FROM (2023) TO (2024) PARTITION BY LIST (region);

CREATE TABLE public.sales_hierarchy_2023_q1 PARTITION OF public.sales_hierarchy_2023 
FOR VALUES IN ('US') PARTITION BY LIST (quarter);

CREATE TABLE public.sales_hierarchy_2023_q1_us PARTITION OF public.sales_hierarchy_2023_q1 
FOR VALUES IN ('Q1');

-- Table with multiple UNIQUE constraints - should get multiple PK options
CREATE TABLE public.multiple_unique_options (
    id integer NOT NULL,
    name text NOT NULL,
    email text NOT NULL,
    phone text NOT NULL,
    UNIQUE(id, name),
    UNIQUE(email),
    UNIQUE(phone, name)
);

-- Partitioned table with multiple UNIQUE constraints containing all partition columns
CREATE TABLE public.partitioned_multiple_unique (
    id integer NOT NULL,
    region text NOT NULL,
    year integer NOT NULL,
    quarter text NOT NULL,
    amount numeric,
    UNIQUE(id, region, year, quarter),
    UNIQUE(id, year, region, quarter)  -- Same columns, different order
) PARTITION BY RANGE (year);

CREATE TABLE public.partitioned_multiple_unique_2023 PARTITION OF public.partitioned_multiple_unique 
FOR VALUES FROM (2023) TO (2024) PARTITION BY LIST (region);

CREATE TABLE public.partitioned_multiple_unique_2023_us PARTITION OF public.partitioned_multiple_unique_2023 
FOR VALUES IN ('US');

-- Simple UNIQUE INDEX on single column
CREATE TABLE public.unique_index_simple (
    id integer NOT NULL,
    name text NOT NULL,
    email text
);

CREATE UNIQUE INDEX idx_unique_simple ON public.unique_index_simple(id);

-- Composite UNIQUE INDEX on multiple columns
CREATE TABLE public.unique_index_composite (
    id integer NOT NULL,
    name text NOT NULL,
    email text NOT NULL,
    phone text
);

CREATE UNIQUE INDEX idx_unique_composite ON public.unique_index_composite(id, name);

-- Multiple UNIQUE INDEXES - should get multiple PK options
CREATE TABLE public.unique_index_multiple (
    id integer NOT NULL,
    name text NOT NULL,
    email text NOT NULL,
    phone text NOT NULL
);

CREATE UNIQUE INDEX idx_unique_multiple_1 ON public.unique_index_multiple(id);
CREATE UNIQUE INDEX idx_unique_multiple_2 ON public.unique_index_multiple(email);
CREATE UNIQUE INDEX idx_unique_multiple_3 ON public.unique_index_multiple(phone, name);

-- Partitioned table with UNIQUE INDEX including all partition columns
CREATE TABLE public.partitioned_unique_index (
    id integer NOT NULL,
    region text NOT NULL,
    year integer NOT NULL,
    amount numeric
) PARTITION BY RANGE (year);

CREATE UNIQUE INDEX idx_partitioned_unique ON public.partitioned_unique_index(id, region, year);

CREATE TABLE public.partitioned_unique_index_2023 PARTITION OF public.partitioned_unique_index 
FOR VALUES FROM (2023) TO (2024);

CREATE TABLE public.partitioned_unique_index_2023_us PARTITION OF public.partitioned_unique_index_2023 
FOR VALUES IN ('US');

-- Mix of UNIQUE constraint and UNIQUE INDEX on different columns
CREATE TABLE public.mix_constraint_index (
    id integer NOT NULL,
    name text NOT NULL,
    email text NOT NULL,
    phone text NOT NULL,
    UNIQUE(id, name)
);

CREATE UNIQUE INDEX idx_mix_email ON public.mix_constraint_index(email);
CREATE UNIQUE INDEX idx_mix_phone ON public.mix_constraint_index(phone);

-- Mix of UNIQUE constraint and UNIQUE INDEX on same columns (should deduplicate)
CREATE TABLE public.mix_constraint_index_same (
    id integer NOT NULL,
    name text NOT NULL,
    email text NOT NULL,
    UNIQUE(id, name)
);

CREATE UNIQUE INDEX idx_mix_same ON public.mix_constraint_index_same(id, name);

-- Insert test data for PK recommendation tables
INSERT INTO public.no_pk_table (id, name, email) 
SELECT i, 'user_' || i, 'user' || i || '@example.com' 
FROM generate_series(1, 100) AS i;

INSERT INTO public.partitioned_no_pk (id, region, year, amount)
SELECT i, 'US', 2023, i * 100
FROM generate_series(1, 50) AS i;

INSERT INTO public.sales_hierarchy (id, region, year, quarter, amount)
SELECT i, 'US', 2023, 'Q1', i * 1000
FROM generate_series(1, 25) AS i;

INSERT INTO public.multiple_unique_options (id, name, email, phone) 
SELECT i, 'user_' || i, 'user' || i || '@example.com', '555-' || LPAD(i::text, 4, '0')
FROM generate_series(1, 50) AS i;

INSERT INTO public.partitioned_multiple_unique (id, region, year, quarter, amount)
SELECT i, 'US', 2023, 'Q1', i * 1000
FROM generate_series(1, 20) AS i;

INSERT INTO public.unique_index_simple (id, name, email) 
SELECT i, 'user_' || i, 'user' || i || '@example.com' 
FROM generate_series(1, 50) AS i;

INSERT INTO public.unique_index_composite (id, name, email, phone) 
SELECT i, 'user_' || i, 'user' || i || '@example.com', '555-' || i
FROM generate_series(1, 50) AS i;

INSERT INTO public.unique_index_multiple (id, name, email, phone) 
SELECT i, 'user_' || i, 'user' || i || '@example.com', '555-' || i
FROM generate_series(1, 50) AS i;

INSERT INTO public.partitioned_unique_index (id, region, year, amount)
SELECT i, 'US', 2023, i * 100
FROM generate_series(1, 25) AS i;

INSERT INTO public.mix_constraint_index (id, name, email, phone) 
SELECT i, 'user_' || i, 'user' || i || '@example.com', '555-' || i
FROM generate_series(1, 50) AS i;

INSERT INTO public.mix_constraint_index_same (id, name, email) 
SELECT i, 'user_' || i, 'user' || i || '@example.com'
FROM generate_series(1, 50) AS i;

-- Negative test cases - should NOT get PK recommendations

-- Root partitioned table where mid-level already has PK - should NOT get PK recommendation
CREATE TABLE public.sales_with_mid_pk (
    id integer NOT NULL,
    sale_date date NOT NULL,
    region text NOT NULL,
    amount numeric,
    UNIQUE(id, sale_date, region)  -- This would normally qualify for PK recommendation
) PARTITION BY RANGE (sale_date);

CREATE TABLE public.sales_with_mid_pk_2024 PARTITION OF public.sales_with_mid_pk 
FOR VALUES FROM ('2024-01-01') TO ('2025-01-01') PARTITION BY LIST (region);

CREATE TABLE public.sales_with_mid_pk_2024_asia PARTITION OF public.sales_with_mid_pk_2024 
FOR VALUES IN ('Asia');

-- Add PK to mid-level table (this prevents root from getting PK recommendation)
ALTER TABLE public.sales_with_mid_pk_2024 
ADD CONSTRAINT sales_with_mid_pk_2024_pk PRIMARY KEY (id, region);

-- Table with UNIQUE constraint but nullable columns
CREATE TABLE public.no_pk_nullable (
    id integer,
    name text,
    email text,
    UNIQUE(id, name)
);

-- Partitioned table without UNIQUE constraint
CREATE TABLE public.partitioned_no_unique (
    id integer NOT NULL,
    region text NOT NULL,
    year integer NOT NULL,
    amount numeric
) PARTITION BY RANGE (year);

CREATE TABLE public.partitioned_no_unique_2023 PARTITION OF public.partitioned_no_unique 
FOR VALUES FROM (2023) TO (2024) PARTITION BY LIST (region);

CREATE TABLE public.partitioned_no_unique_2023_us PARTITION OF public.partitioned_no_unique_2023 
FOR VALUES IN ('US');

-- Multi-level partitioned table without UNIQUE constraint
CREATE TABLE public.sales_hierarchy_no_unique (
    id integer NOT NULL,
    region text NOT NULL,
    year integer NOT NULL,
    quarter text NOT NULL,
    amount numeric
) PARTITION BY RANGE (year);

CREATE TABLE public.sales_hierarchy_no_unique_2023 PARTITION OF public.sales_hierarchy_no_unique 
FOR VALUES FROM (2023) TO (2024) PARTITION BY LIST (region);

CREATE TABLE public.sales_hierarchy_no_unique_2023_q1 PARTITION OF public.sales_hierarchy_no_unique_2023 
FOR VALUES IN ('US') PARTITION BY LIST (quarter);

CREATE TABLE public.sales_hierarchy_no_unique_2023_q1_us PARTITION OF public.sales_hierarchy_no_unique_2023_q1 
FOR VALUES IN ('Q1');

-- Table that already has a PRIMARY KEY
CREATE TABLE public.has_pk_already (
    id integer NOT NULL,
    name text NOT NULL,
    email text,
    PRIMARY KEY(id),
    UNIQUE(name)
);

-- Child partition table (should be skipped)
CREATE TABLE public.parent_partitioned (
    id integer NOT NULL,
    region text NOT NULL,
    data text
) PARTITION BY LIST (region);

CREATE TABLE public.child_partition_skipped PARTITION OF public.parent_partitioned (
    UNIQUE(id, region)
) FOR VALUES IN ('US');

-- UNIQUE INDEX with nullable columns
CREATE TABLE public.unique_index_nullable (
    id integer,
    name text,
    email text NOT NULL
);

CREATE UNIQUE INDEX idx_unique_nullable ON public.unique_index_nullable(id, name);

-- Table that already has PRIMARY KEY with UNIQUE INDEX
CREATE TABLE public.has_pk_with_unique_index (
    id integer NOT NULL,
    name text NOT NULL,
    email text NOT NULL,
    PRIMARY KEY(id)
);

CREATE UNIQUE INDEX idx_has_pk_unique ON public.has_pk_with_unique_index(email);

-- Expression-based UNIQUE INDEX (should be excluded)
CREATE TABLE public.unique_index_expression (
    id integer NOT NULL,
    name text NOT NULL,
    email text NOT NULL
);

CREATE UNIQUE INDEX idx_unique_expression ON public.unique_index_expression((UPPER(name)));

-- Mixed column and expression UNIQUE INDEX (should be excluded)
CREATE TABLE public.unique_index_mixed (
    id integer NOT NULL,
    name text NOT NULL,
    email text NOT NULL
);

CREATE UNIQUE INDEX idx_unique_mixed ON public.unique_index_mixed(id, (UPPER(name)));

-- Root partitioned table with UNIQUE INDEX missing partition columns
CREATE TABLE public.partitioned_unique_index_missing (
    id integer NOT NULL,
    region text NOT NULL,
    year integer NOT NULL,
    amount numeric
) PARTITION BY RANGE (year);

CREATE UNIQUE INDEX idx_partitioned_missing ON public.partitioned_unique_index_missing(id, region);

CREATE TABLE public.partitioned_unique_index_missing_2023 PARTITION OF public.partitioned_unique_index_missing 
FOR VALUES FROM (2023) TO (2024);

CREATE TABLE public.partitioned_unique_index_missing_2023_us PARTITION OF public.partitioned_unique_index_missing_2023 
FOR VALUES IN ('US');

-- Insert test data for negative test cases
INSERT INTO public.no_pk_nullable (id, name, email) 
SELECT i, 'user_' || i, 'user' || i || '@example.com' 
FROM generate_series(1, 50) AS i;

INSERT INTO public.partitioned_no_unique (id, region, year, amount)
SELECT i, 'US', 2023, i * 100
FROM generate_series(1, 25) AS i;

INSERT INTO public.sales_hierarchy_no_unique (id, region, year, quarter, amount)
SELECT i, 'US', 2023, 'Q1', i * 1000
FROM generate_series(1, 15) AS i;

INSERT INTO public.has_pk_already (id, name, email) 
SELECT i, 'user_' || i, 'user' || i || '@example.com' 
FROM generate_series(1, 75) AS i;

INSERT INTO public.parent_partitioned (id, region, data)
SELECT i, 'US', 'data_' || i
FROM generate_series(1, 30) AS i;

INSERT INTO public.sales_with_mid_pk (id, sale_date, region, amount)
SELECT i, '2024-06-15', 'Asia', i * 100
FROM generate_series(1, 30) AS i;

INSERT INTO public.unique_index_nullable (id, name, email) 
SELECT i, 'user_' || i, 'user' || i || '@example.com'
FROM generate_series(1, 50) AS i;

INSERT INTO public.has_pk_with_unique_index (id, name, email) 
SELECT i, 'user_' || i, 'user' || i || '@example.com'
FROM generate_series(1, 50) AS i;

INSERT INTO public.unique_index_expression (id, name, email) 
SELECT i, 'user_' || i, 'user' || i || '@example.com'
FROM generate_series(1, 50) AS i;

INSERT INTO public.unique_index_mixed (id, name, email) 
SELECT i, 'user_' || i, 'user' || i || '@example.com'
FROM generate_series(1, 50) AS i;

INSERT INTO public.partitioned_unique_index_missing (id, region, year, amount)
SELECT i, 'US', 2023, i * 100
FROM generate_series(1, 25) AS i;

--
-- PostgreSQL database dump complete
--

ANALYZE;
