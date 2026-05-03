-- Partition iteration test schema
-- Tests --use-partition-root=false with CDC routing to child partitions.
-- Three partition types: RANGE (50 partitions, exceeding Instacart's 44), LIST, HASH.
-- RANGE table mirrors production pattern: no PK on root, PKs on children,
-- shared sequence, 31 columns including JSONB.

-- ===================== RANGE PARTITIONED TABLE =====================
-- 50 monthly partitions (Jan 2022 – Feb 2026) + empty Jun 2026 + DEFAULT = 52 total.
-- Exceeds Instacart partition count (44). Indexes kept minimal to stay under YB tablet limit.

DROP TABLE IF EXISTS public.events CASCADE;
DROP SEQUENCE IF EXISTS public.events_id_seq CASCADE;

CREATE SEQUENCE public.events_id_seq;

CREATE TABLE public.events (
    id              BIGINT       NOT NULL DEFAULT nextval('events_id_seq'::regclass),
    ref_id          BIGINT,
    ref_type        VARCHAR(255),
    recipient_id    BIGINT,
    destination     VARCHAR(255),
    channel         VARCHAR(255),
    event_key       VARCHAR(255),
    body            TEXT,
    created_at      TIMESTAMP    NOT NULL,
    updated_at      TIMESTAMP    NOT NULL,
    handler         VARCHAR(255),
    is_silent       BOOLEAN,
    agent_id        BIGINT,
    candidate_id    BIGINT,
    is_automated    BOOLEAN,
    origin_id       BIGINT,
    metadata        JSONB        DEFAULT '{}'::jsonb,
    external_id     VARCHAR,
    event_type      VARCHAR,
    dispatched_at   TIMESTAMP,
    read_at         TIMESTAMP,
    acted_at        TIMESTAMP,
    errored_at      TIMESTAMP,
    push_token      TEXT,
    platform        VARCHAR,
    received_at     TIMESTAMP,
    attachment_url  TEXT,
    origin_address  TEXT,
    subject         TEXT,
    recipient_type  VARCHAR,
    fallback_domain VARCHAR
) PARTITION BY RANGE (created_at);

ALTER SEQUENCE public.events_id_seq OWNED BY public.events.id;

-- Generate 50 monthly partitions: Jan 2022 through Feb 2026
DO $$
DECLARE
    start_date DATE := '2022-01-01';
    end_date   DATE;
    part_name  TEXT;
BEGIN
    FOR i IN 0..49 LOOP
        end_date := start_date + INTERVAL '1 month';
        part_name := 'events_' || to_char(start_date, 'YYYYMM');
        EXECUTE format(
            'CREATE TABLE public.%I PARTITION OF public.events
                FOR VALUES FROM (%L) TO (%L)',
            part_name, start_date::TIMESTAMP, end_date::TIMESTAMP
        );
        EXECUTE format('ALTER TABLE public.%I ADD PRIMARY KEY (id)', part_name);
        start_date := end_date;
    END LOOP;
END $$;

-- TC18: Empty partition — validates Voyager handles partitions with zero rows
CREATE TABLE public.events_202606 PARTITION OF public.events
    FOR VALUES FROM ('2026-06-01 00:00:00') TO ('2026-07-01 00:00:00');
ALTER TABLE public.events_202606 ADD PRIMARY KEY (id);

-- TC38: DEFAULT partition — catches rows outside all defined ranges
CREATE TABLE public.events_default PARTITION OF public.events DEFAULT;
ALTER TABLE public.events_default ADD PRIMARY KEY (id);

-- REPLICA IDENTITY FULL on root + all children for CDC without PK on root
ALTER TABLE public.events REPLICA IDENTITY FULL;
DO $$
DECLARE
    child TEXT;
BEGIN
    FOR child IN
        SELECT c.relname FROM pg_inherits i
        JOIN pg_class c ON c.oid = i.inhrelid
        JOIN pg_class p ON p.oid = i.inhparent
        JOIN pg_namespace n ON n.oid = p.relnamespace
        WHERE p.relname = 'events' AND n.nspname = 'public'
    LOOP
        EXECUTE format('ALTER TABLE public.%I REPLICA IDENTITY FULL', child);
    END LOOP;
END $$;

-- 4 indexes: 52 RANGE partitions × (1 table + 4 indexes) = 260 + 15 (LIST/HASH/roots) = 275
-- 275 + 220 system = 495 tablets, safely under YB 530 limit
CREATE INDEX idx_events_event_key ON public.events (event_key);
CREATE INDEX idx_events_updated_at ON public.events (updated_at);
CREATE INDEX idx_events_candidate_id ON public.events (candidate_id);
CREATE INDEX idx_events_created_at_ref_id_partial ON public.events (created_at, ref_id)
    WHERE event_key = 'running_late' AND handler = 'OrderHandler' AND ref_type = 'OrderDelivery';


-- ===================== LIST PARTITIONED TABLE =====================
-- Partitioned by region (LIST). PK includes partition key.
-- Tests CDC routing for LIST partitions with --use-partition-root=false.

DROP TABLE IF EXISTS public.sales_region CASCADE;

CREATE TABLE public.sales_region (
    id       SERIAL,
    amount   INT,
    branch   TEXT,
    region   TEXT NOT NULL,
    metadata JSONB DEFAULT '{}'::jsonb,
    PRIMARY KEY (id, region)
) PARTITION BY LIST (region);

CREATE TABLE public.sales_london  PARTITION OF public.sales_region FOR VALUES IN ('London');
CREATE TABLE public.sales_sydney  PARTITION OF public.sales_region FOR VALUES IN ('Sydney');
CREATE TABLE public.sales_boston   PARTITION OF public.sales_region FOR VALUES IN ('Boston');
CREATE TABLE public.sales_tokyo   PARTITION OF public.sales_region FOR VALUES IN ('Tokyo');
CREATE TABLE public.sales_berlin  PARTITION OF public.sales_region FOR VALUES IN ('Berlin');
CREATE TABLE public.sales_default PARTITION OF public.sales_region DEFAULT;


-- ===================== HASH PARTITIONED TABLE =====================
-- Partitioned by HASH on emp_id. PK on root.
-- Tests CDC routing for HASH partitions with --use-partition-root=false.

DROP TABLE IF EXISTS public.emp CASCADE;

CREATE TABLE public.emp (
    emp_id    SERIAL PRIMARY KEY,
    emp_name  TEXT,
    dep_code  INT,
    salary    NUMERIC(10,2),
    metadata  JSONB DEFAULT '{}'::jsonb
) PARTITION BY HASH (emp_id);

CREATE TABLE public.emp_0 PARTITION OF public.emp FOR VALUES WITH (MODULUS 5, REMAINDER 0);
CREATE TABLE public.emp_1 PARTITION OF public.emp FOR VALUES WITH (MODULUS 5, REMAINDER 1);
CREATE TABLE public.emp_2 PARTITION OF public.emp FOR VALUES WITH (MODULUS 5, REMAINDER 2);
CREATE TABLE public.emp_3 PARTITION OF public.emp FOR VALUES WITH (MODULUS 5, REMAINDER 3);
CREATE TABLE public.emp_4 PARTITION OF public.emp FOR VALUES WITH (MODULUS 5, REMAINDER 4);
