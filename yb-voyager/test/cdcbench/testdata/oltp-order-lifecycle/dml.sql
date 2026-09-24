-- The most common OLTP shape: a row is inserted once and then mutated through
-- status transitions that never touch the unique column.
-- 5000 x (1 insert + 3 updates) = 20000 events, zero conflicts.
DO $$
BEGIN
    FOR i IN 1..5000 LOOP
        INSERT INTO orders (id, order_no, status, c0) VALUES (i, 'ord_' || i, 'new', md5(i::text));
        UPDATE orders SET status = 'paid',      c0 = md5(i::text || 'a') WHERE id = i;
        UPDATE orders SET status = 'shipped',   c1 = md5(i::text || 'b') WHERE id = i;
        UPDATE orders SET status = 'delivered', c2 = md5(i::text || 'c') WHERE id = i;
    END LOOP;
END $$;
