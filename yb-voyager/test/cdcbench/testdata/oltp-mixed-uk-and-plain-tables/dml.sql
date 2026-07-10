-- The most common real migration shape: ONE table with a unique key (20% of
-- traffic) co-streamed with a plain audit table (80% of traffic). Measures the
-- collateral damage of UK-table conflict checks on unrelated tables sharing
-- the single ingest thread. Zero conflicts.
-- 4000 UK updates + 16000 plain inserts = 20000 events.
DO $$
BEGIN
    FOR i IN 1..20000 LOOP
        IF i % 5 = 0 THEN
            UPDATE accounts SET email = 'emx_' || (i / 5) WHERE id = i / 5;
        ELSE
            INSERT INTO audit_log (id, actor, action, c0) VALUES (i, i % 4000, 'act', md5(i::text));
        END IF;
    END LOOP;
END $$;
