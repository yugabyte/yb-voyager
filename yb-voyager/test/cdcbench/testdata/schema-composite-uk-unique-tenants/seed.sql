-- tenant_id == id: no two rows share a tenant, so the workload is
-- conflict-free under BOTH per-column and composite-tuple semantics
TRUNCATE comp_items;
INSERT INTO comp_items SELECT i, i, 'slug_' || i, md5(i::text) FROM generate_series(1, 20000) i;
