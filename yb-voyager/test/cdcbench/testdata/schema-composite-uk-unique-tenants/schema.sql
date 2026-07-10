DROP TABLE IF EXISTS comp_items CASCADE;
CREATE TABLE comp_items (
    id int PRIMARY KEY, tenant_id int, slug text, c0 text,
    CONSTRAINT comp_items_tenant_slug_uk UNIQUE (tenant_id, slug)
);
ALTER TABLE comp_items REPLICA IDENTITY FULL;
