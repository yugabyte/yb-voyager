-- Corner case: archival pattern, updates + deletes only (no inserts). All
-- events enter the cache; all values stay globally distinct. Zero conflicts.
-- 10000 updates + 10000 deletes = 20000 events.
DO $$
BEGIN
    FOR i IN 1..10000 LOOP
        UPDATE uk_table SET email = 'udx_' || i WHERE id = i;
        DELETE FROM uk_table WHERE id = 10000 + i;
    END LOOP;
END $$;
