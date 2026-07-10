-- Real conflicts confined to the SECOND unique index (username); email is
-- never touched. Exercises per-index detection asymmetrically.
-- 10000 pairs x 2 updates = 20000 events.
DO $$
BEGIN
    FOR i IN 1..10000 LOOP
        UPDATE accounts2 SET username = 'unx_' || i          WHERE id = 2*i - 1;
        UPDATE accounts2 SET username = 'un_'  || (2*i - 1)  WHERE id = 2*i;
    END LOOP;
END $$;
