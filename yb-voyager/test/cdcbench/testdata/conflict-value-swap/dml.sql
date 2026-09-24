-- Classic unique-value swap between row pairs via a temporary value
-- (A->tmp, B->A's old, A->B's old). Real UPDATE-UPDATE conflict chains.
-- 6666 x 3 updates = 19998 events.
DO $$
BEGIN
    FOR i IN 1..6666 LOOP
        UPDATE uk_table SET email = 'tmp_' || i          WHERE id = 2*i - 1;
        UPDATE uk_table SET email = 'sw_' || (2*i - 1)   WHERE id = 2*i;
        UPDATE uk_table SET email = 'sw_' || (2*i)       WHERE id = 2*i - 1;
    END LOOP;
END $$;
