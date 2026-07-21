-- TPC-C-ish checkout across FOUR co-streamed tables: order insert, 3 line
-- items, 2 hot inventory decrements, payment insert, order status update.
-- 2500 x 8 = 20000 events, zero conflicts.
DO $$
BEGIN
    FOR i IN 1..2500 LOOP
        INSERT INTO co_orders (id, order_no, status, c0) VALUES (i, 'ord_' || i, 'new', md5(i::text));
        INSERT INTO co_items (id, order_id, product_id, qty) VALUES (3*i - 2, i, 1 + (i % 200), 1);
        INSERT INTO co_items (id, order_id, product_id, qty) VALUES (3*i - 1, i, 1 + ((i + 7) % 200), 2);
        INSERT INTO co_items (id, order_id, product_id, qty) VALUES (3*i,     i, 1 + ((i + 13) % 200), 1);
        UPDATE co_inventory SET stock = stock - 1 WHERE id = 1 + (i % 200);
        UPDATE co_inventory SET stock = stock - 1 WHERE id = 1 + ((i + 7) % 200);
        INSERT INTO co_payments (id, txn_id, amount) VALUES (i, 'txn_' || i, 100 + i);
        UPDATE co_orders SET status = 'paid' WHERE id = i;
    END LOOP;
END $$;
