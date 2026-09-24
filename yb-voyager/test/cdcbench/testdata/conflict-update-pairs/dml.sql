-- 20k updates forming 10k unique-key conflict pairs.
-- Pair i: the first update frees 'seed_email_<i>' (row i gets a new unique
-- email); the second assigns the freed 'seed_email_<i>' to row i+10000.
-- On import, event2.after_email == event1.before_email -> a real
-- before-after unique-key conflict whenever both are in flight.
DO $$
BEGIN
    FOR i IN 1..10000 LOOP
        UPDATE uk_table SET email = 'freed_' || i WHERE id = i;
        UPDATE uk_table SET email = 'seed_email_' || i WHERE id = i + 10000;
    END LOOP;
END $$;
