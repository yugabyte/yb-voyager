-- Corner case: deletes only (bulk purge). Deletes are cached but never wait,
-- so the cache fills and nothing ever scans it. Zero conflicts.
-- 20000 events.
DELETE FROM uk_table;
