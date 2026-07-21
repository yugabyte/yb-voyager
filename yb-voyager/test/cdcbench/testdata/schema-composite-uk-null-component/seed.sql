-- pairs of unnamed drafts sharing a folder: (folder k, name NULL) twice is
-- legal because NULLs are distinct in the unique index
TRUNCATE docs;
INSERT INTO docs
SELECT i, (i + 1) / 2, NULL, md5(i::text), md5(i::text || 'b')
FROM generate_series(1, 10000) i;
