TRUNCATE co_orders, co_items, co_inventory, co_payments;
INSERT INTO co_inventory SELECT i, 1000, md5(i::text) FROM generate_series(1, 200) i;
