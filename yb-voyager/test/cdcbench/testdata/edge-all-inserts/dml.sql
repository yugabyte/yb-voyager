-- Corner case: inserts only (bulk append with unique values). Inserts check
-- the conflict cache but are never cached themselves, so the cache stays
-- empty and every check is against nothing. Zero conflicts.
-- 20000 events.
INSERT INTO uk_table (id, email, c0)
SELECT i, 'ins_' || i, md5(i::text) FROM generate_series(1, 20000) i;
