-- 20k inserts into the table without unique indexes (control workload)
INSERT INTO no_uk_table
SELECT i,
       md5(i::text || 'n0'), md5(i::text || 'n1'), md5(i::text || 'n2'),
       md5(i::text || 'n3'), md5(i::text || 'n4'), md5(i::text || 'n5'),
       md5(i::text || 'n6'), md5(i::text || 'n7'), md5(i::text || 'n8'),
       md5(i::text || 'n9'), md5(i::text || 'n10'), md5(i::text || 'n11')
FROM generate_series(1, 20000) i;
