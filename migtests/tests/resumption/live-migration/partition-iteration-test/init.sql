-- Partition iteration test schema
-- Mirrors a real production pattern: RANGE-partitioned by timestamp,
-- NO primary key on root table, PKs on all child partitions,
-- shared sequence for globally unique IDs, 31 columns including JSONB.
-- Tests --use-partition-root=false with CDC routing to child partitions.

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

-- Monthly partitions (Jan-May have DML data, June is intentionally empty)
CREATE TABLE public.events_202601 PARTITION OF public.events
    FOR VALUES FROM ('2026-01-01 00:00:00') TO ('2026-02-01 00:00:00');
ALTER TABLE public.events_202601 ADD PRIMARY KEY (id);

CREATE TABLE public.events_202602 PARTITION OF public.events
    FOR VALUES FROM ('2026-02-01 00:00:00') TO ('2026-03-01 00:00:00');
ALTER TABLE public.events_202602 ADD PRIMARY KEY (id);

CREATE TABLE public.events_202603 PARTITION OF public.events
    FOR VALUES FROM ('2026-03-01 00:00:00') TO ('2026-04-01 00:00:00');
ALTER TABLE public.events_202603 ADD PRIMARY KEY (id);

CREATE TABLE public.events_202604 PARTITION OF public.events
    FOR VALUES FROM ('2026-04-01 00:00:00') TO ('2026-05-01 00:00:00');
ALTER TABLE public.events_202604 ADD PRIMARY KEY (id);

CREATE TABLE public.events_202605 PARTITION OF public.events
    FOR VALUES FROM ('2026-05-01 00:00:00') TO ('2026-06-01 00:00:00');
ALTER TABLE public.events_202605 ADD PRIMARY KEY (id);

-- TC18: Empty partition — validates that Voyager handles partitions with zero rows
-- (snapshot is no-op, CDC never targets it, hash validation confirms 0=0)
CREATE TABLE public.events_202606 PARTITION OF public.events
    FOR VALUES FROM ('2026-06-01 00:00:00') TO ('2026-07-01 00:00:00');
ALTER TABLE public.events_202606 ADD PRIMARY KEY (id);

-- TC38: DEFAULT partition — catches rows with timestamps outside all defined ranges
-- (validates CDC routing for rows that land in the catch-all partition)
CREATE TABLE public.events_default PARTITION OF public.events DEFAULT;
ALTER TABLE public.events_default ADD PRIMARY KEY (id);

-- REPLICA IDENTITY FULL required for UPDATEs/DELETEs on tables without PK on root.
-- All column values included in CDC events for row identification.
ALTER TABLE public.events REPLICA IDENTITY FULL;
ALTER TABLE public.events_202601 REPLICA IDENTITY FULL;
ALTER TABLE public.events_202602 REPLICA IDENTITY FULL;
ALTER TABLE public.events_202603 REPLICA IDENTITY FULL;
ALTER TABLE public.events_202604 REPLICA IDENTITY FULL;
ALTER TABLE public.events_202605 REPLICA IDENTITY FULL;
ALTER TABLE public.events_202606 REPLICA IDENTITY FULL;
ALTER TABLE public.events_default REPLICA IDENTITY FULL;

CREATE INDEX idx_events_push_token ON public.events (push_token);
CREATE INDEX idx_events_candidate_id ON public.events (candidate_id);
CREATE INDEX idx_events_event_key ON public.events (event_key);
CREATE INDEX idx_events_external_id ON public.events (external_id);
CREATE INDEX idx_events_ref_id_ref_type ON public.events (ref_id, ref_type);
CREATE INDEX idx_events_ref_id_agent_id ON public.events (ref_id, agent_id);
CREATE INDEX idx_events_ref_type_created_at ON public.events (ref_type, created_at);
CREATE INDEX idx_events_agent_id ON public.events (agent_id);
CREATE INDEX idx_events_updated_at ON public.events (updated_at);
CREATE INDEX idx_events_recipient_id ON public.events (recipient_id);

CREATE INDEX idx_events_created_at_ref_id_partial ON public.events (created_at, ref_id)
    WHERE event_key = 'running_late' AND handler = 'OrderHandler' AND ref_type = 'OrderDelivery';
