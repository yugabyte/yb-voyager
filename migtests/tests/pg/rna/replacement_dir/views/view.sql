-- setting variables for current session
SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;


CREATE VIEW rnacen.rnc_sequence_regions_active_provided AS
 SELECT region.id,
    region.urs_taxid,
    region.region_name,
    region.chromosome,
    region.strand,
    region.region_start,
    region.region_stop,
    region.assembly_id,
    region.was_mapped,
    region.identity,
    region.providing_databases,
    region.exon_count
   FROM rnacen.rnc_sequence_regions region
  WHERE ((region.was_mapped = false) AND (EXISTS ( SELECT 1
           FROM (rnacen.rnc_accession_sequence_region acc_map
             JOIN rnacen.xref ON ((((xref.ac)::text = acc_map.accession) AND (xref.deleted = 'N'::bpchar))))
          WHERE (acc_map.region_id = region.id))));


CREATE VIEW rnacen.rnc_sequence_regions_active_mapped AS
 SELECT regions.id,
    regions.urs_taxid,
    regions.region_name,
    regions.chromosome,
    regions.strand,
    regions.region_start,
    regions.region_stop,
    regions.assembly_id,
    regions.was_mapped,
    regions.identity,
    regions.providing_databases,
    regions.exon_count
   FROM rnacen.rnc_sequence_regions regions
  WHERE ((regions.was_mapped = true) AND (NOT (EXISTS ( SELECT 1
           FROM rnacen.rnc_sequence_regions_active_provided provided
          WHERE ((provided.urs_taxid = regions.urs_taxid) AND ((provided.assembly_id)::text = (regions.assembly_id)::text))))));


-- CREATE VIEW rnacen.unindexed_foreign_keys AS
--  WITH y AS (
--          SELECT format('%I.%I'::text, n1.nspname, c1.relname) AS referencing_tbl,
--             quote_ident((a1.attname)::text) AS referencing_column,
--             t.conname AS existing_fk_on_referencing_tbl,
--             format('%I.%I'::text, n2.nspname, c2.relname) AS referenced_tbl,
--             quote_ident((a2.attname)::text) AS referenced_column,
--             pg_relation_size((format('%I.%I'::text, n1.nspname, c1.relname))::regclass) AS referencing_tbl_bytes,
--             pg_relation_size((format('%I.%I'::text, n2.nspname, c2.relname))::regclass) AS referenced_tbl_bytes,
--             format('CREATE INDEX IF NOT EXISTS %I ON %I.%I(%I);'::text, (((c1.relname)::text || '$'::text) || (a1.attname)::text), n1.nspname, c1.relname, a1.attname) AS suggestion
--            FROM ((((((pg_constraint t
--              JOIN pg_attribute a1 ON (((a1.attrelid = t.conrelid) AND (a1.attnum = t.conkey[1]))))
--              JOIN pg_class c1 ON ((c1.oid = t.conrelid)))
--              JOIN pg_namespace n1 ON ((n1.oid = c1.relnamespace)))
--              JOIN pg_class c2 ON ((c2.oid = t.confrelid)))
--              JOIN pg_namespace n2 ON ((n2.oid = c2.relnamespace)))
--              JOIN pg_attribute a2 ON (((a2.attrelid = t.confrelid) AND (a2.attnum = t.confkey[1]))))
--           WHERE ((t.contype = 'f'::"char") AND (NOT (EXISTS ( SELECT 1
--                    FROM pg_index i
--                   WHERE ((i.indrelid = t.conrelid) AND (i.indkey[0] = t.conkey[1]))))))
--         )
--  SELECT y.referencing_tbl,
--     y.referencing_column,
--     y.existing_fk_on_referencing_tbl,
--     y.referenced_tbl,
--     y.referenced_column,
--     pg_size_pretty(y.referencing_tbl_bytes) AS referencing_tbl_size,
--     pg_size_pretty(y.referenced_tbl_bytes) AS referenced_tbl_size,
--     y.suggestion
--    FROM y
--   ORDER BY y.referencing_tbl_bytes DESC, y.referenced_tbl_bytes DESC, y.referencing_tbl, y.referenced_tbl, y.referencing_column, y.referenced_column;


CREATE VIEW rnacen.xref_count AS
 SELECT count(*) AS xref_count,
    xref.upi
   FROM rnacen.xref
  WHERE (xref.deleted = 'N'::bpchar)
  GROUP BY xref.upi
  ORDER BY (count(*)) DESC;


