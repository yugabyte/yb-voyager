/*
Copyright (c) YugabyteDB, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package queryissue

import (
	mapset "github.com/deckarep/golang-set/v2"
)

// Refer: https://www.postgresql.org/docs/current/functions-admin.html#FUNCTIONS-ADVISORY-LOCKS
var unsupportedAdvLockFuncs = mapset.NewThreadUnsafeSet([]string{
	"pg_advisory_lock", "pg_advisory_lock_shared",
	"pg_advisory_unlock", "pg_advisory_unlock_all", "pg_advisory_unlock_shared",
	"pg_advisory_xact_lock", "pg_advisory_xact_lock_shared",
	"pg_try_advisory_lock", "pg_try_advisory_lock_shared",
	"pg_try_advisory_xact_lock", "pg_try_advisory_xact_lock_shared",
}...)

var unsupportedSysCols = mapset.NewThreadUnsafeSet([]string{
	"xmin", "xmax", "cmin", "cmax", "ctid",
}...)

// Refer: https://www.postgresql.org/docs/17/functions-xml.html#FUNCTIONS-XML-PROCESSING
var unsupportedXmlFunctions = mapset.NewThreadUnsafeSet([]string{
	// 1. Producing XML content
	"xmltext", "xmlcomment", "xmlconcat", "xmlelement", "xmlforest",
	"xmlpi", "xmlroot", "xmlagg",
	// 2. XML predicates
	"xml", "xmlexists", "xml_is_well_formed", "xml_is_well_formed_document",
	"xml_is_well_formed_content",
	// 3. Processing XML
	"xpath", "xpath_exists", "xmltable",
	// 4. Mapping Table to XML
	"table_to_xml", "table_to_xmlschema", "table_to_xml_and_xmlschema",
	"cursor_to_xmlschema", "cursor_to_xml",
	"query_to_xmlschema", "query_to_xml", "query_to_xml_and_xmlschema",
	"schema_to_xml", "schema_to_xmlschema", "schema_to_xml_and_xmlschema",
	"database_to_xml", "database_to_xmlschema", "database_to_xml_and_xmlschema",

	/*
		5. extras - not in ref doc but exists
		SELECT proname FROM pg_proc
		WHERE prorettype = 'xml'::regtype;
	*/
	"xmlconcat2", "xmlvalidate", "xml_in", "xml_out", "xml_recv", "xml_send", // System XML I/O
}...)

var unsupportedRegexFunctions = mapset.NewThreadUnsafeSet([]string{
	"regexp_count", "regexp_instr", "regexp_like",
}...)

var UnsupportedIndexMethods = []string{
	"gist",
	"brin",
	"spgist",
}

// Reference for some of the types https://docs.yugabyte.com/stable/api/ysql/datatypes/ (datatypes with type 1)
var UnsupportedIndexDatatypes = []string{
	"citext",
	"tsvector",
	"tsquery",
	"jsonb",
	"inet",
	"json",
	"macaddr",
	"macaddr8",
	"cidr",
	"bit",    // for BIT (n)
	"varbit", // for BIT varying (n)
	"daterange",
	"tsrange",
	"tstzrange",
	"numrange",
	"int4range",
	"int8range",
	"interval", // same for INTERVAL YEAR TO MONTH and INTERVAL DAY TO SECOND
	//Below ones are not supported on PG as well with atleast btree access method. Better to have in our list though
	//Need to understand if there is other method or way available in PG to have these index key [TODO]
	"circle",
	"box",
	"line",
	"lseg",
	"point",
	"pg_lsn",
	"path",
	"polygon",
	"txid_snapshot",
	// array as well but no need to add it in the list as fetching this type is a different way TODO: handle better with specific types
}

var unsupportedRangeAggFunctions = mapset.NewThreadUnsafeSet([]string{
	//range agg function added in PG15 - https://www.postgresql.org/docs/15/functions-aggregate.html#:~:text=Yes-,range_agg,-(%20value%20anyrange
	"range_agg", "range_intersect_agg",
}...)

const (
	ANY_VALUE = "any_value" //any_value function is added in  PG16 - https://www.postgresql.org/docs/16/functions-aggregate.html#id-1.5.8.27.5.2.4.1.1.1.1

	// // json functions, refer - https://www.postgresql.org/about/featurematrix/detail/395/
	JSON_OBJECTAGG = "JSON_OBJECTAGG"
	JSON_ARRAY     = "JSON_ARRAY"
	JSON_ARRAYAGG  = "JSON_ARRAYAGG"
	JSON_OBJECT    = "JSON_OBJECT"
	//json query functions supported in PG 17, refer - https://www.postgresql.org/docs/17/functions-json.html#FUNCTIONS-SQLJSON-QUERYING
	JSON_EXISTS = "JSON_EXISTS"
	JSON_QUERY  = "JSON_QUERY"
	JSON_VALUE  = "JSON_VALUE"
	JSON_TABLE  = "JSON_TABLE"

	//https://www.postgresql.org/docs/current/sql-notify.html#:~:text=two%2Dphase%20commit.-,pg_notify,-To%20send%20a
	PG_NOTIFY_FUNC = "pg_notify"
)

var unsupportedLargeObjectFunctions = mapset.NewThreadUnsafeSet([]string{

	//refer - https://www.postgresql.org/docs/current/lo-interfaces.html#LO-CREATE
	"lo_create", "lo_creat", "lo_import", "lo_import_with_oid",
	"lo_export", "lo_open", "lo_write", "lo_read", "lo_lseek", "lo_lseek64",
	"lo_tell", "lo_tell64", "lo_truncate", "lo_truncate64", "lo_close",
	"lo_unlink",

	//server side functions - https://www.postgresql.org/docs/current/lo-funcs.html
	"lo_from_bytea", "lo_put", "lo_get",

	//functions provided by lo extension, refer - https://www.postgresql.org/docs/current/lo.html#LO-RATIONALE
	"lo_manage", "lo_oid",
}...)

// catalog functions return type jsonb
var catalogFunctionsReturningJsonb = mapset.NewThreadUnsafeSet([]string{
	/*
			SELECT
		    DISTINCT p.proname AS Function_Name
		FROM
		    pg_catalog.pg_proc p
		    LEFT JOIN pg_catalog.pg_language l ON p.prolang = l.oid
		    LEFT JOIN pg_catalog.pg_namespace n ON p.pronamespace = n.oid
		WHERE
		    pg_catalog.pg_function_is_visible(p.oid) AND pg_catalog.pg_get_function_result(p.oid) = 'jsonb'

		ORDER BY Function_Name;
	*/
	"jsonb_agg", "jsonb_agg_finalfn", "jsonb_agg_strict", "jsonb_array_element",
	"jsonb_build_array", "jsonb_build_object", "jsonb_concat", "jsonb_delete",
	"jsonb_delete_path", "jsonb_extract_path", "jsonb_in", "jsonb_insert",
	"jsonb_object", "jsonb_object_agg", "jsonb_object_agg_finalfn", "jsonb_object_agg_strict",
	"jsonb_object_agg_unique", "jsonb_object_agg_unique_strict", "jsonb_object_field", "jsonb_path_query_array",
	"jsonb_path_query_array_tz", "jsonb_path_query_first", "jsonb_path_query_first_tz", "jsonb_recv",
	"jsonb_set", "jsonb_set_lax", "jsonb_strip_nulls", "to_jsonb", "ts_headline",
}...)

var nonDecimalIntegerLiterals = []string{
	"0x",
	"0X",
	"0o",
	"0O",
	"0b",
	"0B",
	//https://github.com/pganalyze/pg_query_go/blob/38c866daa3fdb0a7af78741476d6b89029c19afe/parser/src_backend_utils_adt_numutils.c#L59C30-L61C76
	// the prefix "0x" could be "0X" as well so should check both
}

var unsupportedDatabaseOptionsFromPG15 = mapset.NewThreadUnsafeSet([]string{
	"icu_locale", "locale_provider", "locale", "strategy", "collation_version", "oid",
}...)

var unsupportedDatabaseOptionsFromPG17 = mapset.NewThreadUnsafeSet([]string{
	"builtin_locale", "icu_rules",
}...)


var supportedExtensionsOnYB = []string{
	"adminpack", "amcheck", "autoinc", "bloom", "btree_gin", "btree_gist", "citext", "cube",
	"dblink", "dict_int", "dict_xsyn", "earthdistance", "file_fdw", "fuzzystrmatch", "hll", "hstore",
	"hypopg", "insert_username", "intagg", "intarray", "isn", "lo", "ltree", "moddatetime",
	"orafce", "pageinspect", "pg_buffercache", "pg_cron", "pg_freespacemap", "pg_hint_plan", "pg_prewarm", "pg_stat_monitor",
	"pg_stat_statements", "pg_trgm", "pg_visibility", "pgaudit", "pgcrypto", "pgrowlocks", "pgstattuple", "plpgsql",
	"postgres_fdw", "refint", "seg", "sslinfo", "tablefunc", "tcn", "timetravel", "tsm_system_rows",
	"tsm_system_time", "unaccent", `"uuid-ossp"`, "yb_pg_metrics", "yb_test_extension",
}