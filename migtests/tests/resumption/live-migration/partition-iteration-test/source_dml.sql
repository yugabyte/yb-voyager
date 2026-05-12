-- Deterministic edge-case DML for partition iteration test.
-- Runs ONCE per iteration as a separate step before the event generator.
-- The event generator handles bulk random INSERT/UPDATE/DELETE traffic.
-- This script covers specific edge cases that random traffic may not hit.

DO $$
DECLARE
    row_id BIGINT;
    i      INT;
BEGIN
    -- ===================== RANGE TABLE: edge cases =====================

    -- TC19: Boundary values — rows at exact partition boundaries
    -- RANGE uses inclusive lower, exclusive upper: [start, end)
    INSERT INTO public.events (id, ref_id, ref_type, recipient_id, destination, channel, event_key, body, created_at, updated_at, handler, recipient_type)
    VALUES
        (nextval('events_id_seq'), 99901, 'OrderDelivery', 99901, 'boundary@test.com', 'email', 'delivered', 'Exact start of Jan 2022', '2022-01-01 00:00:00', '2022-01-01 00:00:01', 'OrderHandler', 'customer'),
        (nextval('events_id_seq'), 99902, 'OrderPickup', 99902, 'boundary@test.com', 'sms', 'picked_up', 'Last moment of Jan 2022', '2022-01-31 23:59:59', '2022-01-31 23:59:59', 'UserHandler', 'agent'),
        (nextval('events_id_seq'), 99903, 'UserAlert', 99903, 'boundary@test.com', 'push', 'reminder', 'Exact start of Feb 2026', '2026-02-01 00:00:00', '2026-02-01 00:00:01', 'PromoHandler', 'customer'),
        (nextval('events_id_seq'), 99904, 'PromoOffer', 99904, 'boundary@test.com', 'in_app', 'promo_sent', 'Last moment of Feb 2026', '2026-02-28 23:59:59', '2026-02-28 23:59:59', 'OrderHandler', 'agent');

    -- TC38: DEFAULT partition — rows with timestamps outside all defined ranges
    -- Dec 2021 (before first partition Jan 2022) and Aug 2026 (after last defined range)
    INSERT INTO public.events (id, ref_id, ref_type, recipient_id, destination, channel, event_key, body, created_at, updated_at, handler, recipient_type)
    VALUES
        (nextval('events_id_seq'), 99906, 'OrderDelivery', 99906, 'default@test.com', 'email', 'delivered', 'Lands in DEFAULT (Dec 2021)', '2021-12-15 10:00:00', '2021-12-15 10:00:01', 'OrderHandler', 'customer'),
        (nextval('events_id_seq'), 99907, 'UserAlert', 99907, 'default@test.com', 'push', 'reminder', 'Lands in DEFAULT (Aug 2026)', '2026-08-20 14:30:00', '2026-08-20 14:30:01', 'PromoHandler', 'agent');

    -- TC42: Large JSONB (>8KB triggers TOAST storage in PG)
    -- Validates that CDC replicates TOAST'd columns correctly via REPLICA IDENTITY FULL
    row_id := nextval('events_id_seq');
    INSERT INTO public.events (id, ref_id, ref_type, recipient_id, destination, channel, event_key, body, created_at, updated_at, handler, metadata, recipient_type)
    VALUES (
        row_id, 99908, 'OrderDelivery', 99908, 'toast@test.com', 'email', 'delivered',
        'TOAST test row',
        '2024-06-15 12:00:00', '2024-06-15 12:00:01', 'OrderHandler',
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
               updated_at = '2024-06-15 12:00:01'::TIMESTAMP + (i || ' seconds')::INTERVAL
         WHERE id = row_id;
    END LOOP;

    -- TC17: Cross-partition UPDATE — INSERT into Jun 2024, then UPDATE created_at to Jul 2024
    -- PG internally does DELETE from events_202406 + INSERT into events_202407
    -- CDC must replicate this as a DELETE+INSERT pair on the correct child partitions
    row_id := nextval('events_id_seq');
    INSERT INTO public.events (id, ref_id, ref_type, recipient_id, destination, channel, event_key, body, created_at, updated_at, handler, recipient_type)
    VALUES (row_id, 99909, 'OrderPickup', 99909, 'crosspart@test.com', 'sms', 'picked_up', 'Will move partitions', '2024-06-10 08:00:00', '2024-06-10 08:00:01', 'UserHandler', 'agent');

    UPDATE public.events SET created_at = '2024-07-10 08:00:00', updated_at = '2024-07-10 08:00:01' WHERE id = row_id;

    -- ===================== RANGE TABLE: Unicode / special characters =====================
    INSERT INTO public.events (id, ref_id, ref_type, recipient_id, destination, channel, event_key, body, created_at, updated_at, handler, recipient_type)
    VALUES
        (nextval('events_id_seq'), 77001, 'OrderDelivery', 77001, 'unicode@test.com', 'email', 'delivered',
         E'Emojis: \xF0\x9F\x9A\x80\xF0\x9F\x8E\x89 CJK: \u4F60\u597D\u4E16\u754C RTL: \u0645\u0631\u062D\u0628\u0627 Newline:\nTab:\t',
         '2024-08-15 10:00:00', '2024-08-15 10:00:01', 'OrderHandler', 'customer');

    -- ===================== RANGE TABLE: empty string vs NULL =====================
    INSERT INTO public.events (id, ref_id, ref_type, recipient_id, destination, channel, event_key, body, created_at, updated_at, handler, recipient_type)
    VALUES
        (nextval('events_id_seq'), 77002, 'UserAlert', 77002, '', 'sms', 'reminder',
         NULL, '2024-09-10 10:00:00', '2024-09-10 10:00:01', 'PromoHandler', 'agent'),
        (nextval('events_id_seq'), 77003, 'UserAlert', 77003, NULL, 'push', 'reminder',
         '', '2024-09-10 11:00:00', '2024-09-10 11:00:01', 'PromoHandler', 'customer');

    -- ===================== RANGE TABLE: DELETE + re-INSERT same PK =====================
    row_id := nextval('events_id_seq');
    INSERT INTO public.events (id, ref_id, ref_type, recipient_id, destination, channel, event_key, body, created_at, updated_at, handler, recipient_type)
    VALUES (row_id, 77004, 'OrderPickup', 77004, 'tombstone@test.com', 'sms', 'picked_up', 'Original row', '2024-10-05 08:00:00', '2024-10-05 08:00:01', 'UserHandler', 'agent');

    DELETE FROM public.events WHERE id = row_id;

    INSERT INTO public.events (id, ref_id, ref_type, recipient_id, destination, channel, event_key, body, created_at, updated_at, handler, recipient_type)
    VALUES (row_id, 77005, 'PromoOffer', 77005, 'resurrected@test.com', 'in_app', 'promo_sent', 'Resurrected row', '2024-10-05 09:00:00', '2024-10-05 09:00:01', 'OrderHandler', 'customer');

    -- TC18: First CDC event into empty partition (events_202606)
    -- This partition has zero rows at snapshot time; validates CDC handles first-ever insert
    INSERT INTO public.events (id, ref_id, ref_type, recipient_id, destination, channel, event_key, body, created_at, updated_at, handler, recipient_type)
    VALUES (nextval('events_id_seq'), 99910, 'OrderDelivery', 99910, 'empty-part@test.com', 'email', 'delivered',
            'First row in empty Jun 2026 partition via CDC', '2026-06-15 10:00:00', '2026-06-15 10:00:01', 'OrderHandler', 'customer');

    -- ===================== LIST TABLE: edge cases =====================

    -- Insert into each LIST partition + DEFAULT
    INSERT INTO public.sales_region (amount, branch, region, metadata) VALUES
        (1000, 'Main St', 'London', '{"type": "retail"}'::jsonb),
        (2000, 'Harbor Rd', 'Sydney', '{"type": "wholesale"}'::jsonb),
        (3000, 'Beacon St', 'Boston', '{"type": "retail"}'::jsonb),
        (4000, 'Shibuya', 'Tokyo', '{"type": "flagship"}'::jsonb),
        (5000, 'Unter den Linden', 'Berlin', '{"type": "outlet"}'::jsonb),
        (6000, 'Unknown Blvd', 'Mumbai', '{"type": "new_market"}'::jsonb);  -- lands in DEFAULT

    -- UPDATE on LIST partition
    UPDATE public.sales_region SET amount = amount + 500, metadata = metadata || '{"updated": true}'::jsonb
    WHERE region = 'London';

    -- TC17-LIST: Cross-partition UPDATE — INSERT into London, then UPDATE region to Sydney
    -- PG internally does DELETE from sales_london + INSERT into sales_sydney
    -- CDC must replicate this as a DELETE+INSERT pair on the correct child partitions
    INSERT INTO public.sales_region (amount, branch, region, metadata)
    VALUES (7777, 'Will Move', 'London', '{"will_move": true}'::jsonb)
    RETURNING id INTO row_id;

    UPDATE public.sales_region SET region = 'Sydney', metadata = metadata || '{"region_change": true}'::jsonb
    WHERE id = row_id AND region = 'London';

    -- DELETE from LIST partition
    DELETE FROM public.sales_region WHERE region = 'Berlin';

    -- ===================== HASH TABLE: edge cases =====================

    -- Insert rows that will distribute across HASH partitions
    INSERT INTO public.emp (emp_name, dep_code, salary, metadata) VALUES
        ('Alice', 101, 85000.50, '{"level": "senior"}'::jsonb),
        ('Bob', 102, 72000.00, '{"level": "mid"}'::jsonb),
        ('Charlie', 103, 95000.75, '{"level": "lead"}'::jsonb),
        ('Diana', 101, 68000.00, '{"level": "junior"}'::jsonb),
        ('Eve', 104, 110000.00, '{"level": "principal"}'::jsonb);

    -- UPDATE on HASH partition
    UPDATE public.emp SET salary = salary * 1.10, metadata = metadata || '{"raise": true}'::jsonb
    WHERE emp_name = 'Alice';

    -- DELETE from HASH partition
    DELETE FROM public.emp WHERE emp_name = 'Diana';

    -- ===================== MULTILEVEL TABLE: edge cases =====================

    -- Insert into each sub-partition (year × status combinations)
    INSERT INTO public.orders (customer_id, order_date, status, amount, metadata) VALUES
        (1001, '2024-03-15', 'pending',   150.00, '{"source": "web"}'::jsonb),
        (1002, '2024-06-20', 'shipped',   275.50, '{"source": "app"}'::jsonb),
        (1003, '2024-09-10', 'delivered', 420.00, '{"source": "web"}'::jsonb),
        (1004, '2024-12-25', 'cancelled', 99.99,  '{"source": "phone"}'::jsonb),   -- lands in orders_2024_default
        (1005, '2025-02-14', 'pending',   310.00, '{"source": "app"}'::jsonb),
        (1006, '2025-07-04', 'shipped',   185.75, '{"source": "web"}'::jsonb),
        (1007, '2025-11-28', 'delivered', 550.00, '{"source": "app"}'::jsonb),
        (1008, '2025-08-15', 'refunded',  60.00,  '{"source": "web"}'::jsonb);    -- lands in orders_2025_default

    -- Cross sub-partition UPDATE: change status (moves between LIST sub-partitions within same year)
    UPDATE public.orders SET status = 'shipped', metadata = metadata || '{"status_change": true}'::jsonb
    WHERE customer_id = 1001 AND order_date = '2024-03-15' AND status = 'pending';

    -- Cross top-level partition UPDATE: change order_date year (moves between RANGE partitions)
    UPDATE public.orders SET order_date = '2025-03-15', metadata = metadata || '{"year_change": true}'::jsonb
    WHERE customer_id = 1003 AND order_date = '2024-09-10' AND status = 'delivered';

    -- DELETE from multilevel sub-partition
    DELETE FROM public.orders WHERE customer_id = 1004 AND order_date = '2024-12-25' AND status = 'cancelled';

END $$;
