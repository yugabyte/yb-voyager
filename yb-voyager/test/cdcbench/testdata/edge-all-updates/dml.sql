-- 20k updates, zero conflicts by construction: every new email
-- 'bench_new_<id>' is globally unique and never equals any seed email, so no
-- before/after unique-key value is ever shared between two events.
UPDATE uk_table SET email = 'bench_new_' || id, c0 = md5(id::text || 'upd');
