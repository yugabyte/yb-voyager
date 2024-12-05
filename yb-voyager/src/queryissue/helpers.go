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

// Refer: https://www.postgresql.org/docs/current/functions-admin.html#FUNCTIONS-ADVISORY-LOCKS
var unsupportedAdvLockFuncs = []string{
	"pg_advisory_lock", "pg_advisory_lock_shared",
	"pg_advisory_unlock", "pg_advisory_unlock_all", "pg_advisory_unlock_shared",
	"pg_advisory_xact_lock", "pg_advisory_xact_lock_shared",
	"pg_try_advisory_lock", "pg_try_advisory_lock_shared",
	"pg_try_advisory_xact_lock", "pg_try_advisory_xact_lock_shared",
}

var unsupportedSysCols = []string{
	"xmin", "xmax", "cmin", "cmax", "ctid",
}

// Refer: https://www.postgresql.org/docs/17/functions-xml.html#FUNCTIONS-XML-PROCESSING
var unsupportedXmlFunctions = []string{
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
}

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
