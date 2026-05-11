-- Deterministic edge-case DML for fallback (target → source).
-- Runs ONCE per iteration as a separate step before the target event generator.
-- Uses high IDs to avoid PK conflicts with source DML rows.

DO $$
DECLARE
    row_id BIGINT;
    i      INT;
BEGIN
    -- ===================== RANGE TABLE: edge cases =====================

    -- Boundary values on target side
    INSERT INTO public.events (id, ref_id, ref_type, recipient_id, destination, channel, event_key, body, created_at, updated_at, handler, recipient_type)
    VALUES
        (900001, 80001, 'OrderDelivery', 80001, 'tgt@test.com', 'email', 'delivered', 'Target boundary start', '2022-01-01 00:00:00', '2022-01-01 00:00:01', 'OrderHandler', 'customer'),
        (900002, 80002, 'UserAlert', 80002, 'tgt@test.com', 'push', 'reminder', 'Target boundary end', '2026-02-28 23:59:59', '2026-02-28 23:59:59', 'PromoHandler', 'agent');

    -- DEFAULT partition insert from target
    INSERT INTO public.events (id, ref_id, ref_type, recipient_id, destination, channel, event_key, body, created_at, updated_at, handler, recipient_type)
    VALUES (900003, 80003, 'OrderPickup', 80003, 'tgt@test.com', 'sms', 'picked_up', 'Target DEFAULT row', '2021-11-01 10:00:00', '2021-11-01 10:00:01', 'UserHandler', 'customer');

    -- Large JSONB (TOAST) from target
    INSERT INTO public.events (id, ref_id, ref_type, recipient_id, destination, channel, event_key, body, created_at, updated_at, handler, metadata, recipient_type)
    VALUES (900004, 80004, 'OrderDelivery', 80004, 'tgt-toast@test.com', 'email', 'delivered', 'Target TOAST test',
        '2024-05-15 12:00:00', '2024-05-15 12:00:01', 'OrderHandler',
        jsonb_build_object('large_field', repeat('t', 10000), 'nested', jsonb_build_object('a', repeat('v', 5000))),
        'customer');

    -- Rapid UPDATEs on target
    FOR i IN 1..10 LOOP
        UPDATE public.events
           SET metadata = jsonb_build_object('target_rapid', i, 'ts', now()),
               updated_at = '2024-05-15 12:00:01'::TIMESTAMP + (i || ' seconds')::INTERVAL
         WHERE id = 900004;
    END LOOP;

    -- Cross-partition UPDATE on target
    INSERT INTO public.events (id, ref_id, ref_type, recipient_id, destination, channel, event_key, body, created_at, updated_at, handler, recipient_type)
    VALUES (900005, 80005, 'OrderPickup', 80005, 'tgt-cross@test.com', 'sms', 'picked_up', 'Will move partitions on target', '2024-04-10 08:00:00', '2024-04-10 08:00:01', 'UserHandler', 'agent');

    UPDATE public.events SET created_at = '2024-05-10 08:00:00', updated_at = '2024-05-10 08:00:01' WHERE id = 900005;

    -- DELETE from target RANGE
    DELETE FROM public.events WHERE id = 900003;

    -- ===================== LIST TABLE: edge cases =====================

    INSERT INTO public.sales_region (id, amount, branch, region, metadata) VALUES
        (90001, 7000, 'Target Branch', 'Sydney', '{"target": true}'::jsonb),
        (90002, 8000, 'Target Branch 2', 'Tokyo', '{"target": true}'::jsonb),
        (90003, 9000, 'Target Default', 'Paris', '{"target": true}'::jsonb);  -- DEFAULT

    UPDATE public.sales_region SET amount = 9999, metadata = metadata || '{"tgt_updated": true}'::jsonb
    WHERE id = 90001 AND region = 'Sydney';

    -- TC17-LIST (target): Cross-partition UPDATE — INSERT into Boston, then UPDATE region to Tokyo
    -- PG internally does DELETE from sales_boston + INSERT into sales_tokyo
    -- CDC must replicate this as a DELETE+INSERT pair on the correct child partitions
    INSERT INTO public.sales_region (id, amount, branch, region, metadata)
    VALUES (88101, 6666, 'Target Will Move', 'Boston', '{"tgt_will_move": true}'::jsonb);

    UPDATE public.sales_region SET region = 'Tokyo', metadata = metadata || '{"tgt_region_change": true}'::jsonb
    WHERE id = 88101 AND region = 'Boston';

    DELETE FROM public.sales_region WHERE id = 90002 AND region = 'Tokyo';

    -- ===================== HASH TABLE: edge cases =====================

    INSERT INTO public.emp (emp_id, emp_name, dep_code, salary, metadata) VALUES
        (90001, 'TargetEmp1', 201, 55000.00, '{"target": true}'::jsonb),
        (90002, 'TargetEmp2', 202, 65000.00, '{"target": true}'::jsonb),
        (90003, 'TargetEmp3', 203, 75000.00, '{"target": true}'::jsonb);

    UPDATE public.emp SET salary = 70000.00, metadata = metadata || '{"tgt_raise": true}'::jsonb
    WHERE emp_id = 90001;

    DELETE FROM public.emp WHERE emp_id = 90003;

    -- ===================== MULTILEVEL TABLE: edge cases =====================

    INSERT INTO public.orders (customer_id, order_date, status, amount, metadata) VALUES
        (9001, '2024-04-10', 'pending',   200.00, '{"target": true}'::jsonb),
        (9002, '2024-08-20', 'shipped',   350.00, '{"target": true}'::jsonb),
        (9003, '2025-03-05', 'delivered', 475.00, '{"target": true}'::jsonb),
        (9004, '2025-06-15', 'returned',  120.00, '{"target": true}'::jsonb);  -- DEFAULT sub-partition

    -- Cross sub-partition UPDATE on target (status change)
    UPDATE public.orders SET status = 'delivered', metadata = metadata || '{"tgt_status_change": true}'::jsonb
    WHERE customer_id = 9001 AND order_date = '2024-04-10' AND status = 'pending';

    -- Cross top-level UPDATE on target (year change)
    UPDATE public.orders SET order_date = '2025-08-20', metadata = metadata || '{"tgt_year_change": true}'::jsonb
    WHERE customer_id = 9002 AND order_date = '2024-08-20' AND status = 'shipped';

    DELETE FROM public.orders WHERE customer_id = 9004 AND order_date = '2025-06-15' AND status = 'returned';

END $$;
