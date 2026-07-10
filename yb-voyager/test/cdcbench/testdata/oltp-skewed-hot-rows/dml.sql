-- Skewed access: 80% of updates hammer 100 hot rows (payload only, exercising
-- the same-PK exclusion path), 20% touch cold rows changing the unique value
-- to fresh ones. Zero conflicts.
-- 20000 events.
DO $$
BEGIN
    FOR i IN 1..20000 LOOP
        IF i % 5 = 0 THEN
            UPDATE uk_table SET email = 'coldnew_' || (i / 5) WHERE id = 100 + (i / 5);
        ELSE
            UPDATE uk_table SET c0 = md5(i::text || 'hot') WHERE id = 1 + (i % 100);
        END IF;
    END LOOP;
END $$;
