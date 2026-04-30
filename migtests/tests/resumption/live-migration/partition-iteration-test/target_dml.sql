-- Deterministic edge-case DML for fallback (target → source).
-- Runs ONCE per iteration as a separate step before the target event generator.
-- Uses id_offset to avoid PK conflicts with source DML rows.

DO $$
DECLARE
    row_id BIGINT;
BEGIN
    -- ===================== RANGE TABLE =====================
    -- Insert across a few different monthly partitions using high IDs
    INSERT INTO public.events (id, ref_id, ref_type, recipient_id, destination, channel, event_key, body, created_at, updated_at, handler, recipient_type)
    VALUES
        (900001, 80001, 'OrderDelivery', 80001, 'tgt@test.com', 'email', 'delivered', 'Target fallback row 1', '2023-03-15 10:00:00', '2023-03-15 10:00:01', 'OrderHandler', 'customer'),
        (900002, 80002, 'UserAlert', 80002, 'tgt@test.com', 'push', 'reminder', 'Target fallback row 2', '2024-08-20 14:00:00', '2024-08-20 14:00:01', 'PromoHandler', 'agent'),
        (900003, 80003, 'OrderPickup', 80003, 'tgt@test.com', 'sms', 'picked_up', 'Target fallback row 3', '2025-11-10 08:00:00', '2025-11-10 08:00:01', 'UserHandler', 'customer');

    -- UPDATE on target RANGE partition
    UPDATE public.events SET metadata = '{"target_updated": true}'::jsonb WHERE id = 900001;

    -- DELETE from target RANGE partition
    DELETE FROM public.events WHERE id = 900003;

    -- ===================== LIST TABLE =====================
    INSERT INTO public.sales_region (id, amount, branch, region, metadata) VALUES
        (90001, 7000, 'Target Branch', 'Sydney', '{"target": true}'::jsonb),
        (90002, 8000, 'Target Branch 2', 'Tokyo', '{"target": true}'::jsonb);

    UPDATE public.sales_region SET amount = 9999 WHERE id = 90001 AND region = 'Sydney';

    -- ===================== HASH TABLE =====================
    INSERT INTO public.emp (emp_id, emp_name, dep_code, salary, metadata) VALUES
        (90001, 'TargetEmp1', 201, 55000.00, '{"target": true}'::jsonb),
        (90002, 'TargetEmp2', 202, 65000.00, '{"target": true}'::jsonb);

    UPDATE public.emp SET salary = 70000.00 WHERE emp_id = 90001;

END $$;
