TRUNCATE uk_table;
INSERT INTO uk_table (id, email, c0)
SELECT i, 'del_' || i, md5(i::text) FROM generate_series(1, 20000) i;
