-- Customer pattern: records INSERTed as drafts with NULL unique columns; the
-- unique value is only filled by a later UPDATE, followed by ordinary profile
-- updates. Semantically there are ZERO unique-key conflicts here (SQL treats
-- NULLs as distinct), but the conflict detection cache currently treats
-- nil==nil as a conflict, producing false positives.
-- 5000 x (1 insert + 3 updates) = 20000 events.
DO $$
BEGIN
    FOR i IN 1..5000 LOOP
        INSERT INTO uk_table (id, email, c0) VALUES (i, NULL, md5(i::text || 'draft'));
        UPDATE uk_table SET email = 'filled_' || i WHERE id = i;
        UPDATE uk_table SET c0 = md5(i::text || 'p1') WHERE id = i;
        UPDATE uk_table SET c1 = md5(i::text || 'p2') WHERE id = i;
    END LOOP;
END $$;
