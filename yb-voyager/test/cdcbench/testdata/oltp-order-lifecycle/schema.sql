DROP TABLE IF EXISTS orders CASCADE;
CREATE TABLE orders (
    id       int PRIMARY KEY,
    order_no text CONSTRAINT orders_order_no_uk UNIQUE,
    status   text,
    c0 text, c1 text, c2 text, c3 text, c4 text, c5 text
);
ALTER TABLE orders REPLICA IDENTITY FULL;
