-- Versioned-rows pattern behind partial unique indexes
-- (UNIQUE(entity_id) WHERE most_recent) — the exact case the conflict cache's
-- before-before logic documents: demote the current version, insert the new
-- one with the same entity_id. Real conflicts: the insert must not apply
-- before the demotion. 5000 x 4 = 20000 events.
DO $$
BEGIN
    FOR i IN 1..5000 LOOP
        UPDATE versions SET most_recent = false WHERE id = i;
        INSERT INTO versions (id, entity_id, most_recent, payload) VALUES (5000 + i, i, true, md5(i::text || 'v2'));
        UPDATE versions SET payload = md5(i::text || 'p1') WHERE id = i;
        UPDATE versions SET payload = md5(i::text || 'p2') WHERE id = 5000 + i;
    END LOOP;
END $$;
