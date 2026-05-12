-- Deterministic edge-case DML for fallback (target → source).
-- Runs ONCE per iteration on the YB target.
-- All IDs are sequence-generated to avoid conflicts across iterations.

DO $$
DECLARE
    row_id BIGINT;
    i      INT;
BEGIN
    -- ===================== RANGE TABLE: edge cases =====================

    -- Boundary values on target side
    row_id := nextval('events_id_seq');
    INSERT INTO public.events (id, ref_id, ref_type, recipient_id, destination, channel, event_key, body, created_at, updated_at, handler, recipient_type)
    VALUES
        (row_id, 80001, 'OrderDelivery', 80001, 'tgt@test.com', 'email', 'delivered', 'Target boundary start', '2022-01-01 00:00:00', '2022-01-01 00:00:01', 'OrderHandler', 'customer');

    row_id := nextval('events_id_seq');
    INSERT INTO public.events (id, ref_id, ref_type, recipient_id, destination, channel, event_key, body, created_at, updated_at, handler, recipient_type)
    VALUES
        (row_id, 80002, 'UserAlert', 80002, 'tgt@test.com', 'push', 'reminder', 'Target boundary end', '2026-02-28 23:59:59', '2026-02-28 23:59:59', 'PromoHandler', 'agent');

    -- DEFAULT partition insert from target
    row_id := nextval('events_id_seq');
    INSERT INTO public.events (id, ref_id, ref_type, recipient_id, destination, channel, event_key, body, created_at, updated_at, handler, recipient_type)
    VALUES (row_id, 80003, 'OrderPickup', 80003, 'tgt@test.com', 'sms', 'picked_up', 'Target DEFAULT row', '2021-11-01 10:00:00', '2021-11-01 10:00:01', 'UserHandler', 'customer');

    DELETE FROM public.events WHERE id = row_id;

    -- Large JSONB (TOAST) from target
    row_id := nextval('events_id_seq');
    INSERT INTO public.events (id, ref_id, ref_type, recipient_id, destination, channel, event_key, body, created_at, updated_at, handler, metadata, recipient_type)
    VALUES (row_id, 80004, 'OrderDelivery', 80004, 'tgt-toast@test.com', 'email', 'delivered', 'Target TOAST test',
        '2024-05-15 12:00:00', '2024-05-15 12:00:01', 'OrderHandler',
        jsonb_build_object('large_field', repeat('t', 10000), 'nested', jsonb_build_object('a', repeat('v', 5000))),
        'customer');

    -- Rapid UPDATEs on target
    FOR i IN 1..10 LOOP
        UPDATE public.events
           SET metadata = jsonb_build_object('target_rapid', i, 'ts', now()),
               updated_at = '2024-05-15 12:00:01'::TIMESTAMP + (i || ' seconds')::INTERVAL
         WHERE id = row_id;
    END LOOP;

    -- Unicode / special characters from target
    INSERT INTO public.events (id, ref_id, ref_type, recipient_id, destination, channel, event_key, body, created_at, updated_at, handler, recipient_type)
    VALUES
        (nextval('events_id_seq'), 80010, 'OrderDelivery', 80010, 'tgt-unicode@test.com', 'email', 'delivered',
         E'Emojis: \xF0\x9F\x9A\x80\xF0\x9F\x8E\x89 CJK: \u4F60\u597D\u4E16\u754C RTL: \u0645\u0631\u062D\u0628\u0627 Newline:\nTab:\t',
         '2024-07-15 10:00:00', '2024-07-15 10:00:01', 'OrderHandler', 'customer');

    -- Empty string vs NULL from target
    INSERT INTO public.events (id, ref_id, ref_type, recipient_id, destination, channel, event_key, body, created_at, updated_at, handler, recipient_type)
    VALUES
        (nextval('events_id_seq'), 80011, 'UserAlert', 80011, '', 'sms', 'reminder',
         NULL, '2024-08-10 10:00:00', '2024-08-10 10:00:01', 'PromoHandler', 'agent');

    INSERT INTO public.events (id, ref_id, ref_type, recipient_id, destination, channel, event_key, body, created_at, updated_at, handler, recipient_type)
    VALUES
        (nextval('events_id_seq'), 80012, 'UserAlert', 80012, NULL, 'push', 'reminder',
         '', '2024-08-10 11:00:00', '2024-08-10 11:00:01', 'PromoHandler', 'customer');

    -- DELETE + re-INSERT same PK from target
    row_id := nextval('events_id_seq');
    INSERT INTO public.events (id, ref_id, ref_type, recipient_id, destination, channel, event_key, body, created_at, updated_at, handler, recipient_type)
    VALUES (row_id, 80013, 'OrderPickup', 80013, 'tgt-tombstone@test.com', 'sms', 'picked_up', 'Target original', '2024-09-05 08:00:00', '2024-09-05 08:00:01', 'UserHandler', 'agent');

    DELETE FROM public.events WHERE id = row_id;

    INSERT INTO public.events (id, ref_id, ref_type, recipient_id, destination, channel, event_key, body, created_at, updated_at, handler, recipient_type)
    VALUES (row_id, 80014, 'PromoOffer', 80014, 'tgt-resurrected@test.com', 'in_app', 'promo_sent', 'Target resurrected', '2024-09-05 09:00:00', '2024-09-05 09:00:01', 'OrderHandler', 'customer');

    -- Cross-partition UPDATE on target (RANGE: move from Apr 2024 to May 2024)
    row_id := nextval('events_id_seq');
    INSERT INTO public.events (id, ref_id, ref_type, recipient_id, destination, channel, event_key, body, created_at, updated_at, handler, recipient_type)
    VALUES (row_id, 80005, 'OrderPickup', 80005, 'tgt-cross@test.com', 'sms', 'picked_up', 'Will move partitions on target', '2024-04-10 08:00:00', '2024-04-10 08:00:01', 'UserHandler', 'agent');

    UPDATE public.events SET created_at = '2024-05-10 08:00:00', updated_at = '2024-05-10 08:00:01' WHERE id = row_id;

    -- ===================== LIST TABLE: edge cases =====================

    INSERT INTO public.sales_region (amount, branch, region, metadata)
    VALUES (7000, 'Target Branch', 'Sydney', '{"target": true}'::jsonb)
    RETURNING id INTO row_id;

    UPDATE public.sales_region SET amount = 9999, metadata = metadata || '{"tgt_updated": true}'::jsonb
    WHERE id = row_id AND region = 'Sydney';

    INSERT INTO public.sales_region (amount, branch, region, metadata)
    VALUES (8000, 'Target Branch 2', 'Tokyo', '{"target": true}'::jsonb);

    INSERT INTO public.sales_region (amount, branch, region, metadata)
    VALUES (9000, 'Target Default', 'Paris', '{"target": true}'::jsonb);

    -- TC17-LIST (target): Cross-partition UPDATE (region is PK column on YB)
    INSERT INTO public.sales_region (amount, branch, region, metadata)
    VALUES (6666, 'Target Will Move', 'Boston', '{"tgt_will_move": true}'::jsonb)
    RETURNING id INTO row_id;

    UPDATE public.sales_region SET region = 'Tokyo', metadata = metadata || '{"tgt_region_change": true}'::jsonb
    WHERE id = row_id AND region = 'Boston';

    -- ===================== HASH TABLE: edge cases =====================

    INSERT INTO public.emp (emp_name, dep_code, salary, metadata)
    VALUES ('TargetEmp1', 201, 55000.00, '{"target": true}'::jsonb)
    RETURNING emp_id INTO row_id;

    INSERT INTO public.emp (emp_name, dep_code, salary, metadata)
    VALUES ('TargetEmp2', 202, 65000.00, '{"target": true}'::jsonb);

    INSERT INTO public.emp (emp_name, dep_code, salary, metadata)
    VALUES ('TargetEmp3', 203, 75000.00, '{"target": true}'::jsonb);

    UPDATE public.emp SET salary = 70000.00, metadata = metadata || '{"tgt_raise": true}'::jsonb
    WHERE emp_id = row_id;

    DELETE FROM public.emp WHERE emp_id = row_id;

    -- ===================== MULTILEVEL TABLE: edge cases =====================

    INSERT INTO public.orders (customer_id, order_date, status, amount, metadata)
    VALUES (990001, '2024-04-10', 'pending', 200.00, '{"target": true}'::jsonb);

    INSERT INTO public.orders (customer_id, order_date, status, amount, metadata)
    VALUES (990002, '2024-08-20', 'shipped', 350.00, '{"target": true}'::jsonb);

    INSERT INTO public.orders (customer_id, order_date, status, amount, metadata)
    VALUES (990003, '2025-03-05', 'delivered', 475.00, '{"target": true}'::jsonb);

    INSERT INTO public.orders (customer_id, order_date, status, amount, metadata)
    VALUES (990004, '2025-06-15', 'returned', 120.00, '{"target": true}'::jsonb);

    -- Cross sub-partition UPDATE on target (status change: pending → delivered)
    UPDATE public.orders SET status = 'delivered', metadata = metadata || '{"tgt_status_change": true}'::jsonb
    WHERE customer_id = 990001 AND order_date = '2024-04-10' AND status = 'pending';

    -- Cross top-level UPDATE on target (year change: 2024 → 2025)
    UPDATE public.orders SET order_date = '2025-08-20', metadata = metadata || '{"tgt_year_change": true}'::jsonb
    WHERE customer_id = 990002 AND order_date = '2024-08-20' AND status = 'shipped';

    DELETE FROM public.orders WHERE customer_id = 990004 AND order_date = '2025-06-15' AND status = 'returned';

END $$;
