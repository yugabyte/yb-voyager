-- exactly one row (id=0) holds the single permitted NULL
TRUNCATE null_slot;
INSERT INTO null_slot VALUES (0, NULL, md5('0'));
INSERT INTO null_slot
SELECT i, 'v_' || i, md5(i::text) FROM generate_series(1, 10000) i;
