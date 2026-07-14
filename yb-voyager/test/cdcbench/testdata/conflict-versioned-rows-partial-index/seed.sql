TRUNCATE versions;
INSERT INTO versions SELECT i, i, true, md5(i::text) FROM generate_series(1, 5000) i;
