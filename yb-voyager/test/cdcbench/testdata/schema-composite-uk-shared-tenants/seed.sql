TRUNCATE tenant_items;
INSERT INTO tenant_items (id, tenant_id, slug, c0)
SELECT i, 1 + (i % 100), 'slug_' || i, md5(i::text) FROM generate_series(1, 20000) i;
