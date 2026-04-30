-- Target DML: generates fallback CDC events on YB target.
-- Uses id_offset=900000 to avoid PK conflicts with source DML rows.
-- 500 batches x 5 months = 2500 iterations: INSERT + UPDATE + conditional DELETE.
-- Runs during "import data to source" fallback phase to test reverse CDC.

DO $$
DECLARE
    base_ts     TIMESTAMP;
    row_ts      TIMESTAMP;
    row_id      BIGINT;
    months      INT[] := ARRAY[1, 2, 3, 4, 5];
    m           INT;
    channels    TEXT[] := ARRAY['email', 'sms', 'push', 'in_app'];
    ref_types   TEXT[] := ARRAY['OrderDelivery', 'OrderPickup', 'UserAlert', 'PromoOffer'];
    handlers    TEXT[] := ARRAY['OrderHandler', 'UserHandler', 'PromoHandler'];
    evt_keys    TEXT[] := ARRAY['running_late', 'delivered', 'picked_up', 'reminder', 'promo_sent'];
    id_offset   BIGINT := 900000;
BEGIN
    FOR batch IN 1..500 LOOP
        FOREACH m IN ARRAY months LOOP
            base_ts := ('2026-' || LPAD(m::TEXT, 2, '0') || '-01')::TIMESTAMP;
            row_ts  := base_ts + (batch % 28 || ' days')::INTERVAL
                                + ((batch + 12) % 24 || ' hours')::INTERVAL
                                + ((batch + 30) % 60 || ' minutes')::INTERVAL;

            row_id := id_offset + (batch - 1) * 10 + m;
            INSERT INTO public.events (
                id, ref_id, ref_type, recipient_id, destination, channel,
                event_key, body, created_at, updated_at, handler, is_silent,
                agent_id, candidate_id, is_automated, origin_id, metadata,
                external_id, event_type, dispatched_at, read_at, acted_at,
                errored_at, push_token, platform, received_at, attachment_url,
                origin_address, subject, recipient_type, fallback_domain
            ) VALUES (
                row_id,
                batch * 10 + m + 50000,
                ref_types[1 + (batch + m) % 4],
                batch * 100 + m + 80000,
                'tgt-user' || (batch * 100 + m) || '@example.com',
                channels[1 + (batch + m) % 4],
                evt_keys[1 + (batch + m) % 5],
                'Target event body for batch ' || batch || ' month ' || m,
                row_ts,
                row_ts + '1 second'::INTERVAL,
                handlers[1 + (batch + m) % 3],
                (batch % 5 = 0),
                CASE WHEN batch % 3 = 0 THEN batch * 7 + m + 50000 ELSE NULL END,
                CASE WHEN batch % 4 = 0 THEN batch * 11 + m + 50000 ELSE NULL END,
                (batch % 2 = 0),
                batch * 3 + m + 50000,
                jsonb_build_object('batch', batch, 'month', m, 'source', 'target'),
                'tgt-ext-' || batch || '-' || m,
                CASE WHEN batch % 2 = 0 THEN 'transactional' ELSE 'marketing' END,
                CASE WHEN batch % 3 <> 0 THEN row_ts + '5 seconds'::INTERVAL ELSE NULL END,
                CASE WHEN batch % 7 = 0 THEN row_ts + '1 hour'::INTERVAL ELSE NULL END,
                CASE WHEN batch % 11 = 0 THEN row_ts + '2 hours'::INTERVAL ELSE NULL END,
                CASE WHEN batch % 13 = 0 THEN row_ts + '30 minutes'::INTERVAL ELSE NULL END,
                CASE WHEN batch % 3 = 0 THEN 'tgt-push-' || batch || '-' || m ELSE NULL END,
                CASE WHEN batch % 7 = 0 THEN 'ios' WHEN batch % 3 = 0 THEN 'android' ELSE NULL END,
                CASE WHEN batch % 3 <> 0 THEN row_ts + '10 seconds'::INTERVAL ELSE NULL END,
                CASE WHEN batch % 9 = 0 THEN 'https://cdn.example.com/tgt/' || batch || '.png' ELSE NULL END,
                'tgt-noreply@example.com',
                'Target subject for batch ' || batch,
                CASE WHEN batch % 2 = 0 THEN 'customer' ELSE 'agent' END,
                CASE WHEN batch % 15 = 0 THEN 'tgt-fallback.example.com' ELSE NULL END
            );

            UPDATE public.events
               SET metadata = metadata || '{"target_updated": true}'::jsonb,
                   updated_at = updated_at + '1 minute'::INTERVAL
             WHERE id = row_id;

            IF batch % 3 = 0 THEN
                DELETE FROM public.events WHERE id = row_id;
            END IF;
        END LOOP;

        IF batch % 50 = 0 THEN
            PERFORM pg_sleep(0.1);
        END IF;
    END LOOP;
END $$;
