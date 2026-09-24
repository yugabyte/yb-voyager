-- Wide rows (50 payload columns): REPLICA IDENTITY FULL makes every update
-- carry a 50-column before-image, measuring decode/convert cost. The unique
-- column is never touched -> zero conflicts.
-- 2 x 10000 updates = 20000 events.
UPDATE wide_table SET c0 = md5(id::text || 'u1'), c1 = md5(id::text || 'u2');
UPDATE wide_table SET c25 = md5(id::text || 'u3'), c26 = md5(id::text || 'u4');
