-- Auth/session pattern: unique tokens inserted on login, deleted on logout,
-- ~100 sessions alive at any moment, token values never reused.
-- 10000 inserts + 9900 deletes = 19900 events, zero conflicts.
DO $$
BEGIN
    FOR i IN 1..10000 LOOP
        INSERT INTO sessions (id, token, c0) VALUES (i, 'tok_' || i, md5(i::text));
        IF i > 100 THEN
            DELETE FROM sessions WHERE id = i - 100;
        END IF;
    END LOOP;
END $$;
