-- The single NULL "slot" is handed from row to row: free it (give the holder
-- a real value), then claim it (set the next row's value to NULL). Each pair
-- is a REAL conflict: the claim must not apply before the release commits,
-- or the single-NULL constraint is violated on the target. These conflicts
-- must KEEP being detected even after NULL-distinctness is fixed for
-- ordinary (NULLS DISTINCT) unique indexes.
-- 10000 x 2 updates = 20000 events.
DO $$
BEGIN
    FOR i IN 1..10000 LOOP
        UPDATE null_slot SET val = 'vx_' || (i - 1) WHERE id = i - 1;
        UPDATE null_slot SET val = NULL WHERE id = i;
    END LOOP;
END $$;
