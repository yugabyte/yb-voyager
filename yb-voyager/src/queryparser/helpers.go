package queryparser

import (
	"google.golang.org/protobuf/reflect/protoreflect"
)

const (
	DOCS_LINK_PREFIX        = "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/"
	POSTGRESQL_PREFIX       = "postgresql/"
	ADVISORY_LOCKS_DOC_LINK = DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#advisory-locks-is-not-yet-implemented"
	SYSTEM_COLUMNS_DOC_LINK = DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#system-columns-is-not-yet-supported"
	XML_FUNCTIONS_DOC_LINK  = DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#xml-functions-is-not-yet-supported"
)

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

// Sample example: {func_call:{funcname:{string:{sval:"pg_advisory_lock"}}
func getFuncNameFromFuncCall(funcCallNode protoreflect.Message) string {
	if getMsgFullName(funcCallNode) != PG_QUERY_FUNCCALL_NODE {
		return ""
	}

	funcnameField := funcCallNode.Get(funcCallNode.Descriptor().Fields().ByName("funcname"))
	funcnameList := funcnameField.List()
	var names []string

	// TODO: simplification to directly access last item of funcnameList
	for i := 0; i < funcnameList.Len(); i++ {
		item := funcnameList.Get(i)
		name := getStringValueFromNode(item.Message())
		if name != "" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return ""
	}
	return names[len(names)-1] // ignoring schema_name
}

// Sample example:: {column_ref:{fields:{string:{sval:"xmax"}}
func getColNameFromColumnRef(columnRefNode protoreflect.Message) string {
	if getMsgFullName(columnRefNode) != PG_QUERY_COLUMNREF_NODE {
		return ""
	}

	fields := columnRefNode.Get(columnRefNode.Descriptor().Fields().ByName("fields"))
	fieldsList := fields.List()
	var names []string

	// TODO: simplification to directly access last item of fieldsList
	for i := 0; i < fieldsList.Len(); i++ {
		item := fieldsList.Get(i)
		name := getStringValueFromNode(item.Message())
		if name != "" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return ""
	}
	return names[len(names)-1] // ignoring schema_name
}

// Sample example:: {column_ref:{fields:{string:{sval:"s"}}  fields:{string:{sval:"tableoid"}}  location:7}
func getStringValueFromNode(nodeMsg protoreflect.Message) string {
	if nodeMsg == nil || !nodeMsg.IsValid() {
		return ""
	}

	// 'nodeMsg' is a 'pg_query.Node' getting the set field in the 'node' oneof
	nodeField := nodeMsg.WhichOneof(nodeMsg.Descriptor().Oneofs().ByName("node"))
	if nodeField == nil {
		return ""
	}

	nodeValue := nodeMsg.Get(nodeField)
	node := nodeValue.Message()
	if node == nil || !node.IsValid() {
		return ""
	}

	nodeType := node.Descriptor().FullName()
	switch nodeType {
	case PG_QUERY_STRING_NODE:
		strField := node.Descriptor().Fields().ByName("sval")
		strValue := node.Get(strField)
		return strValue.String()
	// example: SELECT * FROM employees;
	case PG_QUERY_ASTAR_NODE:
		return ""
	default:
		return ""
	}
}

func getMsgFullName(msg protoreflect.Message) string {
	return string(msg.Descriptor().FullName())
}
