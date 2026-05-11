-- Partition iteration test schema
-- Tests --use-partition-root=false with CDC routing to child partitions.
-- Three partition types: RANGE (50 partitions), LIST, HASH, plus multilevel.
-- RANGE table mirrors production pattern: no PK on root, PKs on children,
-- shared sequence, 31 columns including JSONB.

-- ===================== RANGE PARTITIONED TABLE =====================
-- 50 monthly partitions (Jan 2022 – Feb 2026) + empty Jun 2026 = 51 total.
-- No DEFAULT partition (matches production pattern).
-- Indexes kept minimal to stay under YB tablet limit.

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
-- Partitioned by region (LIST). No PK on root, PK on children only.
-- Tests CDC routing for LIST partitions with --use-partition-root=false.

DROP TABLE IF EXISTS public.sales_region CASCADE;

CREATE TABLE public.sales_region (
    id       SERIAL,
    amount   INT,
    branch   TEXT,
    region   TEXT NOT NULL,
    metadata JSONB DEFAULT '{}'::jsonb
) PARTITION BY LIST (region);

CREATE TABLE public.sales_london  PARTITION OF public.sales_region FOR VALUES IN ('London');
CREATE TABLE public.sales_sydney  PARTITION OF public.sales_region FOR VALUES IN ('Sydney');
CREATE TABLE public.sales_boston   PARTITION OF public.sales_region FOR VALUES IN ('Boston');
CREATE TABLE public.sales_tokyo   PARTITION OF public.sales_region FOR VALUES IN ('Tokyo');
CREATE TABLE public.sales_berlin  PARTITION OF public.sales_region FOR VALUES IN ('Berlin');
CREATE TABLE public.sales_default PARTITION OF public.sales_region DEFAULT;

ALTER TABLE public.sales_london  ADD PRIMARY KEY (id, region);
ALTER TABLE public.sales_sydney  ADD PRIMARY KEY (id, region);
ALTER TABLE public.sales_boston   ADD PRIMARY KEY (id, region);
ALTER TABLE public.sales_tokyo   ADD PRIMARY KEY (id, region);
ALTER TABLE public.sales_berlin  ADD PRIMARY KEY (id, region);
ALTER TABLE public.sales_default ADD PRIMARY KEY (id, region);

ALTER TABLE public.sales_region REPLICA IDENTITY FULL;
ALTER TABLE public.sales_london  REPLICA IDENTITY FULL;
ALTER TABLE public.sales_sydney  REPLICA IDENTITY FULL;
ALTER TABLE public.sales_boston   REPLICA IDENTITY FULL;
ALTER TABLE public.sales_tokyo   REPLICA IDENTITY FULL;
ALTER TABLE public.sales_berlin  REPLICA IDENTITY FULL;
ALTER TABLE public.sales_default REPLICA IDENTITY FULL;


-- ===================== HASH PARTITIONED TABLE =====================
-- Partitioned by HASH on emp_id. No PK on root, PK on children only.
-- Tests CDC routing for HASH partitions with --use-partition-root=false.

DROP TABLE IF EXISTS public.emp CASCADE;

CREATE TABLE public.emp (
    emp_id    SERIAL,
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

ALTER TABLE public.emp_0 ADD PRIMARY KEY (emp_id);
ALTER TABLE public.emp_1 ADD PRIMARY KEY (emp_id);
ALTER TABLE public.emp_2 ADD PRIMARY KEY (emp_id);
ALTER TABLE public.emp_3 ADD PRIMARY KEY (emp_id);
ALTER TABLE public.emp_4 ADD PRIMARY KEY (emp_id);

ALTER TABLE public.emp REPLICA IDENTITY FULL;
ALTER TABLE public.emp_0 REPLICA IDENTITY FULL;
ALTER TABLE public.emp_1 REPLICA IDENTITY FULL;
ALTER TABLE public.emp_2 REPLICA IDENTITY FULL;
ALTER TABLE public.emp_3 REPLICA IDENTITY FULL;
ALTER TABLE public.emp_4 REPLICA IDENTITY FULL;


-- ===================== MULTILEVEL (SUB)PARTITIONED TABLE =====================
-- RANGE (by order_date) → LIST (by status) sub-partitioning.
-- No PK on root/intermediate, PK on leaf children only.
-- Tests CDC routing through two levels of partition hierarchy.

DROP TABLE IF EXISTS public.orders CASCADE;

CREATE TABLE public.orders (
    order_id    SERIAL,
    customer_id BIGINT,
    order_date  DATE NOT NULL,
    status      TEXT NOT NULL,
    amount      NUMERIC(10,2),
    metadata    JSONB DEFAULT '{}'::jsonb
) PARTITION BY RANGE (order_date);

-- Level 1: RANGE by year (widened to 2023-2026 for better random data coverage)
CREATE TABLE public.orders_2023 PARTITION OF public.orders
    FOR VALUES FROM ('2023-01-01') TO ('2024-01-01')
    PARTITION BY LIST (status);

CREATE TABLE public.orders_2024 PARTITION OF public.orders
    FOR VALUES FROM ('2024-01-01') TO ('2025-01-01')
    PARTITION BY LIST (status);

CREATE TABLE public.orders_2025 PARTITION OF public.orders
    FOR VALUES FROM ('2025-01-01') TO ('2026-01-01')
    PARTITION BY LIST (status);

CREATE TABLE public.orders_2026 PARTITION OF public.orders
    FOR VALUES FROM ('2026-01-01') TO ('2027-01-01')
    PARTITION BY LIST (status);

-- Level 2: LIST by status under each year
CREATE TABLE public.orders_2023_pending   PARTITION OF public.orders_2023 FOR VALUES IN ('pending');
CREATE TABLE public.orders_2023_shipped   PARTITION OF public.orders_2023 FOR VALUES IN ('shipped');
CREATE TABLE public.orders_2023_delivered PARTITION OF public.orders_2023 FOR VALUES IN ('delivered');
CREATE TABLE public.orders_2023_default   PARTITION OF public.orders_2023 DEFAULT;

CREATE TABLE public.orders_2024_pending   PARTITION OF public.orders_2024 FOR VALUES IN ('pending');
CREATE TABLE public.orders_2024_shipped   PARTITION OF public.orders_2024 FOR VALUES IN ('shipped');
CREATE TABLE public.orders_2024_delivered PARTITION OF public.orders_2024 FOR VALUES IN ('delivered');
CREATE TABLE public.orders_2024_default   PARTITION OF public.orders_2024 DEFAULT;

CREATE TABLE public.orders_2025_pending   PARTITION OF public.orders_2025 FOR VALUES IN ('pending');
CREATE TABLE public.orders_2025_shipped   PARTITION OF public.orders_2025 FOR VALUES IN ('shipped');
CREATE TABLE public.orders_2025_delivered PARTITION OF public.orders_2025 FOR VALUES IN ('delivered');
CREATE TABLE public.orders_2025_default   PARTITION OF public.orders_2025 DEFAULT;

CREATE TABLE public.orders_2026_pending   PARTITION OF public.orders_2026 FOR VALUES IN ('pending');
CREATE TABLE public.orders_2026_shipped   PARTITION OF public.orders_2026 FOR VALUES IN ('shipped');
CREATE TABLE public.orders_2026_delivered PARTITION OF public.orders_2026 FOR VALUES IN ('delivered');
CREATE TABLE public.orders_2026_default   PARTITION OF public.orders_2026 DEFAULT;

-- PK on leaf partitions only
ALTER TABLE public.orders_2023_pending   ADD PRIMARY KEY (order_id, order_date, status);
ALTER TABLE public.orders_2023_shipped   ADD PRIMARY KEY (order_id, order_date, status);
ALTER TABLE public.orders_2023_delivered ADD PRIMARY KEY (order_id, order_date, status);
ALTER TABLE public.orders_2023_default   ADD PRIMARY KEY (order_id, order_date, status);
ALTER TABLE public.orders_2024_pending   ADD PRIMARY KEY (order_id, order_date, status);
ALTER TABLE public.orders_2024_shipped   ADD PRIMARY KEY (order_id, order_date, status);
ALTER TABLE public.orders_2024_delivered ADD PRIMARY KEY (order_id, order_date, status);
ALTER TABLE public.orders_2024_default   ADD PRIMARY KEY (order_id, order_date, status);
ALTER TABLE public.orders_2025_pending   ADD PRIMARY KEY (order_id, order_date, status);
ALTER TABLE public.orders_2025_shipped   ADD PRIMARY KEY (order_id, order_date, status);
ALTER TABLE public.orders_2025_delivered ADD PRIMARY KEY (order_id, order_date, status);
ALTER TABLE public.orders_2025_default   ADD PRIMARY KEY (order_id, order_date, status);
ALTER TABLE public.orders_2026_pending   ADD PRIMARY KEY (order_id, order_date, status);
ALTER TABLE public.orders_2026_shipped   ADD PRIMARY KEY (order_id, order_date, status);
ALTER TABLE public.orders_2026_delivered ADD PRIMARY KEY (order_id, order_date, status);
ALTER TABLE public.orders_2026_default   ADD PRIMARY KEY (order_id, order_date, status);

-- REPLICA IDENTITY FULL on root, intermediate, and leaf partitions
ALTER TABLE public.orders REPLICA IDENTITY FULL;
ALTER TABLE public.orders_2023 REPLICA IDENTITY FULL;
ALTER TABLE public.orders_2024 REPLICA IDENTITY FULL;
ALTER TABLE public.orders_2025 REPLICA IDENTITY FULL;
ALTER TABLE public.orders_2026 REPLICA IDENTITY FULL;
ALTER TABLE public.orders_2023_pending   REPLICA IDENTITY FULL;
ALTER TABLE public.orders_2023_shipped   REPLICA IDENTITY FULL;
ALTER TABLE public.orders_2023_delivered REPLICA IDENTITY FULL;
ALTER TABLE public.orders_2023_default   REPLICA IDENTITY FULL;
ALTER TABLE public.orders_2024_pending   REPLICA IDENTITY FULL;
ALTER TABLE public.orders_2024_shipped   REPLICA IDENTITY FULL;
ALTER TABLE public.orders_2024_delivered REPLICA IDENTITY FULL;
ALTER TABLE public.orders_2024_default   REPLICA IDENTITY FULL;
ALTER TABLE public.orders_2025_pending   REPLICA IDENTITY FULL;
ALTER TABLE public.orders_2025_shipped   REPLICA IDENTITY FULL;
ALTER TABLE public.orders_2025_delivered REPLICA IDENTITY FULL;
ALTER TABLE public.orders_2025_default   REPLICA IDENTITY FULL;
ALTER TABLE public.orders_2026_pending   REPLICA IDENTITY FULL;
ALTER TABLE public.orders_2026_shipped   REPLICA IDENTITY FULL;
ALTER TABLE public.orders_2026_delivered REPLICA IDENTITY FULL;
ALTER TABLE public.orders_2026_default   REPLICA IDENTITY FULL;
