DROP TABLE IF EXISTS tenant_items CASCADE;
CREATE TABLE tenant_items (
    id        int PRIMARY KEY,
    tenant_id int,
    slug      text,
    c0 text, c1 text,
    CONSTRAINT tenant_items_tenant_slug_uk UNIQUE (tenant_id, slug)
);
ALTER TABLE tenant_items REPLICA IDENTITY FULL;
