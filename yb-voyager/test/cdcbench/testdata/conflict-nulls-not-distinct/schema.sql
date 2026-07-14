-- UNIQUE NULLS NOT DISTINCT (PG 15+): at most ONE row may hold NULL.
-- NULL behaves like a real value, so moving the NULL between rows requires
-- strict ordering — nil==nil conflict detection is CORRECT here.
DROP TABLE IF EXISTS null_slot CASCADE;
CREATE TABLE null_slot (
    id  int PRIMARY KEY,
    val text,
    c0  text,
    CONSTRAINT null_slot_val_uk UNIQUE NULLS NOT DISTINCT (val)
);
ALTER TABLE null_slot REPLICA IDENTITY FULL;
