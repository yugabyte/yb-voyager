-- Payload-only updates to draft pairs. Semantically ZERO conflicts: every
-- index tuple has a NULL component, and SQL treats NULLs as distinct. But
-- detection compares tuples with nil==nil, so in-flight updates of a pair
-- (same folder_id, both names NULL) false-positive against each other.
-- Flip ExpectConflicts to false when NULL-distinctness is fixed.
-- 2 x 10000 = 20000 events.
DO $$
BEGIN
    FOR i IN 1..10000 LOOP
        UPDATE docs SET c0 = md5(i::text || 'p1') WHERE id = i;
    END LOOP;
    FOR i IN 1..10000 LOOP
        UPDATE docs SET c1 = md5(i::text || 'p2') WHERE id = i;
    END LOOP;
END $$;
