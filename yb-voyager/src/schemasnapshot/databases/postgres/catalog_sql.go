// Copyright (c) YugabyteDB, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package postgres — pg_catalog SELECT statements used by the loaders.
// Each loader function in loader.go pulls its query constant from here.

package postgres

// sqlServerVersion probes the PostgreSQL server version string.
const sqlServerVersion = `SHOW server_version`

// sqlLoadTablesFmt is the format template for loading tables. It expects %s to be
// replaced with a comma-separated list of $N placeholders (one per schema).
// Columns returned: oid, schema, name, relkind
const sqlLoadTablesFmt = `
SELECT c.oid::text       AS oid,
       n.nspname         AS schema,
       c.relname         AS name,
       c.relkind         AS relkind
FROM   pg_class     c
JOIN   pg_namespace n ON n.oid = c.relnamespace
WHERE  c.relkind IN ('r','p','f')
  AND  n.nspname IN (%s)
ORDER  BY n.nspname, c.relname
`

// sqlLoadTableLinksFmt is the format template for loading BOTH declarative-partition
// and legacy-inheritance parent/child relationships from pg_inherits. It expects %s to
// be replaced with a comma-separated list of $N placeholders (one per schema). Rows are
// distinguished by the is_partition flag (c.relispartition): true for declarative
// partitions, false for legacy INHERITS hierarchies.
// Columns returned: child_oid, child_schema, child_name, parent_oid, parent_schema, parent_name, is_partition
const sqlLoadTableLinksFmt = `
SELECT i.inhrelid::text AS child_oid, cn.nspname AS child_schema, c.relname AS child_name,
       i.inhparent::text AS parent_oid, pn.nspname AS parent_schema, p.relname AS parent_name,
       c.relispartition AS is_partition
FROM   pg_inherits i
JOIN   pg_class     c  ON c.oid = i.inhrelid
JOIN   pg_namespace cn ON cn.oid = c.relnamespace
JOIN   pg_class     p  ON p.oid = i.inhparent
JOIN   pg_namespace pn ON pn.oid = p.relnamespace
WHERE  cn.nspname IN (%s)
ORDER  BY i.inhparent, i.inhrelid
`

// sqlLoadColumnsFmt is the format template for loading columns. It expects %s to be
// replaced with a comma-separated list of $N placeholders (one per schema).
// Columns returned: table_oid, attnum, schema, table_name, col_name, data_type, not_null, col_default
const sqlLoadColumnsFmt = `
SELECT a.attrelid::text                                   AS table_oid,
       a.attnum::text                                     AS attnum,
       n.nspname                                          AS schema,
       c.relname                                          AS table_name,
       a.attname                                          AS col_name,
       format_type(a.atttypid, a.atttypmod)               AS data_type,
       a.attnotnull                                       AS not_null,
       COALESCE(pg_get_expr(d.adbin, d.adrelid), '')      AS col_default
FROM   pg_attribute  a
JOIN   pg_class      c ON c.oid = a.attrelid
JOIN   pg_namespace  n ON n.oid = c.relnamespace
LEFT   JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
WHERE  a.attnum > 0
  AND  NOT a.attisdropped
  AND  c.relkind IN ('r','p','f')
  AND  n.nspname IN (%s)
ORDER  BY a.attrelid, a.attnum
`
