TRUNCATE journal, balances;
INSERT INTO balances SELECT i, 0, md5(i::text) FROM generate_series(1, 100) i;
