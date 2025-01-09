
--dropping multiple object
DROP COLLATION IF EXISTS coll1,coll2,coll3;

CREATE COLLATION special1 (provider = icu, locale = 'en@colCaseFirst=upper;colReorder=grek-latn', deterministic = true);

CREATE COLLATION ignore_accents (provider = icu, locale = 'und-u-ks-level1-kc-true', deterministic = false);

 CREATE COLLATION schema2.upperfirst (provider = icu, locale = 'en-u-kf-upper');