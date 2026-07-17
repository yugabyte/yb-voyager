TRUNCATE accounts;
INSERT INTO accounts (id, email, username, c0)
SELECT i, 'em_' || i, 'un_' || i, md5(i::text) FROM generate_series(1, 20000) i;
