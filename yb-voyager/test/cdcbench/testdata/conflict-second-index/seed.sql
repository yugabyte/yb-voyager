TRUNCATE accounts2;
INSERT INTO accounts2 SELECT i, 'em_' || i, 'un_' || i, md5(i::text) FROM generate_series(1, 20000) i;
