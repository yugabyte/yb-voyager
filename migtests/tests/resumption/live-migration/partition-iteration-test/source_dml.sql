-- Source DML: generates CDC events across all 5 monthly partitions.
-- 500 batches x 5 months = 2500 iterations, each doing:
--   2 INSERTs (full 31-col + partial col) + 1 UPDATE + conditional DELETE
-- Total: ~5000 INSERTs, ~2500 UPDATEs, ~833 DELETEs per run.
-- Plus edge-case DML at the end (boundary values, TOAST, cross-partition, DEFAULT).

DO $$
DECLARE
    base_ts     TIMESTAMP;
    row_ts      TIMESTAMP;
    row_id      BIGINT;
    months      INT[] := ARRAY[1, 2, 3, 4, 5];
    m           INT;
    i           INT;
    channels    TEXT[] := ARRAY['email', 'sms', 'push', 'in_app'];
    ref_types   TEXT[] := ARRAY['OrderDelivery', 'OrderPickup', 'UserAlert', 'PromoOffer'];
    handlers    TEXT[] := ARRAY['OrderHandler', 'UserHandler', 'PromoHandler'];
    evt_keys    TEXT[] := ARRAY['running_late', 'delivered', 'picked_up', 'reminder', 'promo_sent'];
BEGIN
    -- Main DML loop: bulk INSERTs/UPDATEs/DELETEs across Jan-May partitions
    FOR batch IN 1..500 LOOP
        FOREACH m IN ARRAY months LOOP
            base_ts := ('2026-' || LPAD(m::TEXT, 2, '0') || '-01')::TIMESTAMP;
            row_ts  := base_ts + (batch % 28 || ' days')::INTERVAL
                                + (batch % 24 || ' hours')::INTERVAL
                                + (batch % 60 || ' minutes')::INTERVAL;

            -- INSERT 1: Full 31-column row (all columns populated/conditional)
            row_id := nextval('events_id_seq');
            INSERT INTO public.events (
                id, ref_id, ref_type, recipient_id, destination, channel,
                event_key, body, created_at, updated_at, handler, is_silent,
                agent_id, candidate_id, is_automated, origin_id, metadata,
                external_id, event_type, dispatched_at, read_at, acted_at,
                errored_at, push_token, platform, received_at, attachment_url,
                origin_address, subject, recipient_type, fallback_domain
            ) VALUES (
                row_id,
                batch * 10 + m,
                ref_types[1 + (batch + m) % 4],
                batch * 100 + m,
                'user' || (batch * 100 + m) || '@example.com',
                channels[1 + (batch + m) % 4],
                evt_keys[1 + (batch + m) % 5],
                'Event body for batch ' || batch || ' month ' || m,
                row_ts,
                row_ts + '1 second'::INTERVAL,
                handlers[1 + (batch + m) % 3],
                (batch % 5 = 0),
                CASE WHEN batch % 3 = 0 THEN batch * 7 + m ELSE NULL END,
                CASE WHEN batch % 4 = 0 THEN batch * 11 + m ELSE NULL END,
                (batch % 2 = 0),
                batch * 3 + m,
                jsonb_build_object('batch', batch, 'month', m, 'priority', CASE WHEN batch % 2 = 0 THEN 'high' ELSE 'low' END),
                'ext-' || batch || '-' || m,
                CASE WHEN batch % 2 = 0 THEN 'transactional' ELSE 'marketing' END,
                CASE WHEN batch % 3 <> 0 THEN row_ts + '5 seconds'::INTERVAL ELSE NULL END,
                CASE WHEN batch % 7 = 0 THEN row_ts + '1 hour'::INTERVAL ELSE NULL END,
                CASE WHEN batch % 11 = 0 THEN row_ts + '2 hours'::INTERVAL ELSE NULL END,
                CASE WHEN batch % 13 = 0 THEN row_ts + '30 minutes'::INTERVAL ELSE NULL END,
                CASE WHEN batch % 3 = 0 THEN 'push-tok-' || batch || '-' || m ELSE NULL END,
                CASE WHEN batch % 7 = 0 THEN 'ios' WHEN batch % 3 = 0 THEN 'android' ELSE NULL END,
                CASE WHEN batch % 3 <> 0 THEN row_ts + '10 seconds'::INTERVAL ELSE NULL END,
                CASE WHEN batch % 9 = 0 THEN 'https://cdn.example.com/img/' || batch || '.png' ELSE NULL END,
                'noreply@example.com',
                'Subject line for batch ' || batch,
                CASE WHEN batch % 2 = 0 THEN 'customer' ELSE 'agent' END,
                CASE WHEN batch % 15 = 0 THEN 'fallback.example.com' ELSE NULL END
            );

            -- INSERT 2: Partial-column row (subset of columns, rest NULL/default)
            row_id := nextval('events_id_seq');
            INSERT INTO public.events (
                id, ref_id, ref_type, recipient_id, destination, channel,
                event_key, body, created_at, updated_at, handler, is_silent,
                agent_id, candidate_id, is_automated, origin_id, metadata,
                external_id, event_type, dispatched_at, push_token, subject,
                recipient_type
            ) VALUES (
                row_id,
                batch * 10 + m + 1000,
                ref_types[1 + (batch + m + 1) % 4],
                batch * 100 + m + 5000,
                'agent' || (batch + m) || '@example.com',
                channels[1 + (batch + m + 1) % 4],
                evt_keys[1 + (batch + m + 1) % 5],
                'Secondary event batch ' || batch,
                row_ts + '30 seconds'::INTERVAL,
                row_ts + '31 seconds'::INTERVAL,
                handlers[1 + (batch + m + 1) % 3],
                false,
                batch * 7 + m,
                NULL,
                true,
                batch * 3 + m + 100,
                '{"source": "auto"}'::jsonb,
                'ext-s-' || batch || '-' || m,
                'transactional',
                row_ts + '35 seconds'::INTERVAL,
                'push-s-' || batch || '-' || m,
                'Auto subject ' || batch,
                'customer'
            );

            -- UPDATE the second insert: JSONB merge + timestamp columns
            UPDATE public.events
               SET metadata = metadata || '{"updated": true}'::jsonb,
                   updated_at = updated_at + '1 minute'::INTERVAL,
                   read_at = created_at + '2 hours'::INTERVAL
             WHERE id = row_id;

            -- DELETE every 3rd batch (~33% of second-inserts deleted)
            IF batch % 3 = 0 THEN
                DELETE FROM public.events WHERE id = row_id;
            END IF;
        END LOOP;

        IF batch % 50 = 0 THEN
            PERFORM pg_sleep(0.1);
        END IF;
    END LOOP;

    -- ===================== EDGE CASE DML =====================

    -- TC19: Boundary values — rows at exact partition boundaries
    -- Validates correct routing when created_at is exactly on the edge
    -- (RANGE uses inclusive lower, exclusive upper: [start, end) )
    INSERT INTO public.events (id, ref_id, ref_type, recipient_id, destination, channel, event_key, body, created_at, updated_at, handler, recipient_type)
    VALUES
        (nextval('events_id_seq'), 99901, 'OrderDelivery', 99901, 'boundary@test.com', 'email', 'delivered', 'Exact start of Jan', '2026-01-01 00:00:00', '2026-01-01 00:00:01', 'OrderHandler', 'customer'),
        (nextval('events_id_seq'), 99902, 'OrderPickup', 99902, 'boundary@test.com', 'sms', 'picked_up', 'Last moment of Jan', '2026-01-31 23:59:59', '2026-01-31 23:59:59', 'UserHandler', 'agent'),
        (nextval('events_id_seq'), 99903, 'UserAlert', 99903, 'boundary@test.com', 'push', 'reminder', 'Exact start of Feb', '2026-02-01 00:00:00', '2026-02-01 00:00:01', 'PromoHandler', 'customer'),
        (nextval('events_id_seq'), 99904, 'PromoOffer', 99904, 'boundary@test.com', 'in_app', 'promo_sent', 'Exact start of May', '2026-05-01 00:00:00', '2026-05-01 00:00:01', 'OrderHandler', 'agent'),
        (nextval('events_id_seq'), 99905, 'OrderDelivery', 99905, 'boundary@test.com', 'email', 'running_late', 'Last moment of May', '2026-05-31 23:59:59', '2026-05-31 23:59:59', 'UserHandler', 'customer');

    -- TC38: DEFAULT partition — rows with timestamps outside all defined ranges
    -- Dec 2025 and Aug 2026 both land in events_default
    INSERT INTO public.events (id, ref_id, ref_type, recipient_id, destination, channel, event_key, body, created_at, updated_at, handler, recipient_type)
    VALUES
        (nextval('events_id_seq'), 99906, 'OrderDelivery', 99906, 'default@test.com', 'email', 'delivered', 'Lands in DEFAULT partition (Dec 2025)', '2025-12-15 10:00:00', '2025-12-15 10:00:01', 'OrderHandler', 'customer'),
        (nextval('events_id_seq'), 99907, 'UserAlert', 99907, 'default@test.com', 'push', 'reminder', 'Lands in DEFAULT partition (Aug 2026)', '2026-08-20 14:30:00', '2026-08-20 14:30:01', 'PromoHandler', 'agent');

    -- TC42: Large JSONB (>8KB triggers TOAST storage in PG)
    -- Validates that CDC replicates TOAST'd columns correctly via REPLICA IDENTITY FULL
    row_id := nextval('events_id_seq');
    INSERT INTO public.events (id, ref_id, ref_type, recipient_id, destination, channel, event_key, body, created_at, updated_at, handler, metadata, recipient_type)
    VALUES (
        row_id, 99908, 'OrderDelivery', 99908, 'toast@test.com', 'email', 'delivered',
        'TOAST test row',
        '2026-03-15 12:00:00', '2026-03-15 12:00:01', 'OrderHandler',
        jsonb_build_object(
            'large_field', repeat('x', 10000),
            'nested', jsonb_build_object('a', repeat('y', 5000), 'b', repeat('z', 5000))
        ),
        'customer'
    );

    -- TC43: Rapid UPDATEs on same row — 10 consecutive updates without pause
    -- Validates CDC handles multiple events for same PK without losing final state
    FOR i IN 1..10 LOOP
        UPDATE public.events
           SET metadata = jsonb_build_object('rapid_update', i, 'ts', now(), 'payload', repeat('u', 500)),
               updated_at = '2026-03-15 12:00:01'::TIMESTAMP + (i || ' seconds')::INTERVAL
         WHERE id = row_id;
    END LOOP;

    -- TC17: Cross-partition UPDATE — INSERT into March, then UPDATE created_at to April
    -- PG internally does DELETE from events_202603 + INSERT into events_202604.
    -- CDC must replicate this as a DELETE+INSERT pair on the correct child partitions.
    row_id := nextval('events_id_seq');
    INSERT INTO public.events (id, ref_id, ref_type, recipient_id, destination, channel, event_key, body, created_at, updated_at, handler, recipient_type)
    VALUES (row_id, 99909, 'OrderPickup', 99909, 'crosspart@test.com', 'sms', 'picked_up', 'Will move partitions', '2026-03-10 08:00:00', '2026-03-10 08:00:01', 'UserHandler', 'agent');

    UPDATE public.events SET created_at = '2026-04-10 08:00:00', updated_at = '2026-04-10 08:00:01' WHERE id = row_id;

END $$;
