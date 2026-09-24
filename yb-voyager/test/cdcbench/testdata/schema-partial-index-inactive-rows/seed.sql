-- pairs of soft-DELETED rows legitimately SHARING an email (the partial
-- index only covers NOT deleted rows)
TRUNCATE soft_users;
INSERT INTO soft_users
SELECT i, 'dup_' || ((i + 1) / 2), true, md5(i::text) FROM generate_series(1, 10000) i;
