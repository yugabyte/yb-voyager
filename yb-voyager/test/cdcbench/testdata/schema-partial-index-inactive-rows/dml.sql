-- Payload-only updates to rows OUTSIDE the partial index (deleted=true) that
-- legitimately share email values. Detection drops the index predicate, so
-- in-flight updates of a pair "conflict" on the email column: the documented
-- partial-predicate false positive. Zero semantic conflicts.
-- 2 x 10000 = 20000 events.
DO $$
BEGIN
    FOR i IN 1..10000 LOOP
        UPDATE soft_users SET payload = md5(i::text || 'a') WHERE id = i;
    END LOOP;
    FOR i IN 1..10000 LOOP
        UPDATE soft_users SET payload = md5(i::text || 'b') WHERE id = i;
    END LOOP;
END $$;
