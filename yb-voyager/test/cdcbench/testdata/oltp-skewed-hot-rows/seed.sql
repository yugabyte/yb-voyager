TRUNCATE uk_table;
INSERT INTO uk_table (id, email, c0)
SELECT i, 'hs_' || i, md5(i::text) FROM generate_series(1, 4100) i;
