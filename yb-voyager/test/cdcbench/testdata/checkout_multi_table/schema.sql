DROP TABLE IF EXISTS co_orders, co_items, co_inventory, co_payments CASCADE;
CREATE TABLE co_orders    (id int PRIMARY KEY, order_no text CONSTRAINT co_orders_no_uk UNIQUE, status text, c0 text);
CREATE TABLE co_items     (id int PRIMARY KEY, order_id int, product_id int, qty int);
CREATE TABLE co_inventory (id int PRIMARY KEY, stock int, c0 text);
CREATE TABLE co_payments  (id int PRIMARY KEY, txn_id text CONSTRAINT co_payments_txn_uk UNIQUE, amount int);
ALTER TABLE co_orders    REPLICA IDENTITY FULL;
ALTER TABLE co_items     REPLICA IDENTITY FULL;
ALTER TABLE co_inventory REPLICA IDENTITY FULL;
ALTER TABLE co_payments  REPLICA IDENTITY FULL;
