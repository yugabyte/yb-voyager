TRUNCATE accounts, audit_log;
INSERT INTO accounts SELECT i, 'em_' || i, md5(i::text) FROM generate_series(1, 4000) i;
