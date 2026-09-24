-- "Replace" upsert idiom: DELETE a row and re-INSERT a new row with the SAME
-- unique value. The documented DELETE-INSERT conflict type: the insert must
-- not be applied before the delete.
-- 10000 x (delete + insert) = 20000 events, real conflicts.
DO $$
BEGIN
    FOR i IN 1..10000 LOOP
        DELETE FROM uk_table WHERE id = i;
        INSERT INTO uk_table (id, email, c0) VALUES (10000 + i, 'rk_' || i, md5(i::text || 'x'));
    END LOOP;
END $$;
