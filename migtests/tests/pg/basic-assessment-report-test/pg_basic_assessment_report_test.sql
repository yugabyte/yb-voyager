CREATE SCHEMA public;

CREATE SCHEMA schema1;

-- Performance optimization

CREATE TABLE schema1.test_hotspot_timestamp(
    id int primary key,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    employed boolean
);

CREATE INDEX idx_test_hotspot_timestamp ON schema1.test_hotspot_timestamp (created_at);

CREATE INDEX idx_test_low_card_employed ON schema1.test_hotspot_timestamp (employed);

INSERT INTO schema1.test_hotspot_timestamp (id, created_at, employed) 
SELECT
    i, 
    now() - (i % 100) * interval '1 day', 
    (i % 2 = 0)
FROM generate_series(1, 1000) AS i;

ANALYZE; -- this should before the other schemas so that none of them are analyzed and no column stats for them are populated

-- Unsupported feature

CREATE TABLE parent_table (
    id SERIAL PRIMARY KEY,
    common_column1 TEXT,
    common_column2 INTEGER
);

CREATE TABLE child_table (
    specific_column1 DATE
) INHERITS (parent_table);

CREATE TABLE schema1.employees2 (
    id SERIAL PRIMARY KEY,
    first_name VARCHAR(50) NOT NULL,
    last_name VARCHAR(50) NOT NULL,
    full_name VARCHAR(101) GENERATED ALWAYS AS (first_name || ' ' || last_name) STORED,
    Department varchar(50)
);

--creating 4 indexes to make it a sharded table
CREATE INDEX on idx_emoloyees2 on schema1.employees2 (Department);
CREATE INDEX idx_employees2_full_name ON schema1.employees2 (full_name);
CREATE INDEX idx_employees2_first_name ON schema1.employees2 (first_name);
CREATE INDEX idx_employees2_last_name ON schema1.employees2 (last_name);

--Unsupported datatype

CREATE TABLE Mixed_Data_Types_Table1 (
    id int PRIMARY KEY,
    data jsonb,
    snapshot_data TXID_SNAPSHOT
);

-- unsupported pl/pgsql objects


CREATE OR REPLACE FUNCTION public.manage_large_object(loid OID) RETURNS VOID AS $$
BEGIN
    IF loid IS NOT NULL THEN
        -- Unlink the large object to free up storage
        PERFORM lo_unlink(loid);
    END IF;
END;
$$ LANGUAGE plpgsql;

-- migration caveats 

CREATE ROLE test_policy;

CREATE POLICY policy_test_report ON parent_table TO test_policy USING (true);

-- Unsupported query constructs 
drop extension if exists pg_stat_statements;

create extension pg_stat_statements;

SELECT * FROM pg_stat_statements;

SELECT ctid, tableoid, xmin, xmax, cmin, cmax
FROM schema1.employees2;

