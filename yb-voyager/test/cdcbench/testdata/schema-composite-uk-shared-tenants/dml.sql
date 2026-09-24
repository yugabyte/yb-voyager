-- Multi-tenant SaaS pattern: UNIQUE(tenant_id, slug). Slugs are renamed to
-- globally fresh values, so under true composite-tuple semantics there are
-- ZERO conflicts. But artifacts exported with the flattened unique-key
-- metadata degrade to per-column checks, and in-flight updates of rows in the
-- SAME tenant share the tenant_id column value -> false-positive conflicts.
-- 20000 events.
UPDATE tenant_items SET slug = 'slugx_' || id;
