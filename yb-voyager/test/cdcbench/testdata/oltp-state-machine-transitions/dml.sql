-- Each payment walks a five-step state machine; every step demotes the
-- current transition (most_recent=false) and appends the successor
-- (most_recent=true). REAL conflicts on the partial index: an append must
-- not apply before its predecessor's demotion commits.
-- 2000 payments x 5 steps x 2 events = 20000 events.
DO $$
DECLARE
    states text[] := ARRAY['pending', 'submitted', 'authorized', 'settled', 'completed'];
BEGIN
    FOR i IN 1..2000 LOOP
        FOR s IN 1..5 LOOP
            UPDATE payment_transitions SET most_recent = false
            WHERE payment_id = i AND sort_key = s - 1;
            INSERT INTO payment_transitions
            VALUES (2000 + (i - 1) * 5 + s, i, s, states[s], md5((i * 10 + s)::text), true);
        END LOOP;
    END LOOP;
END $$;
