-- 20k seed rows (exported in the snapshot, not as change events)
TRUNCATE uk_table;
INSERT INTO uk_table
SELECT i,
       'seed_email_' || i,
       md5(i::text || 'c0'), md5(i::text || 'c1'), md5(i::text || 'c2'),
       md5(i::text || 'c3'), md5(i::text || 'c4'), md5(i::text || 'c5'),
       md5(i::text || 'c6'), md5(i::text || 'c7'), md5(i::text || 'c8'),
       md5(i::text || 'c9'), md5(i::text || 'c10'), md5(i::text || 'c11')
FROM generate_series(1, 20000) i;
