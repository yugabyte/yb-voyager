package queryparser

import (
	"google.golang.org/protobuf/reflect/protoreflect"
)

var unsupportedAdvLockFuncs = []string{
	"pg_advisory_lock", "pg_try_advisory_lock", "pg_advisory_xact_lock",
	"pg_advisory_unlock", "pg_advisory_unlock_all", "pg_try_advisory_xact_lock",
}

var unsupportedSysCols = []string{
	"xmin", "xmax", "cmin", "cmax", "ctid",
}

// TODO: verify the list (any missing, or extra/invalid)
var xmlFunctions = []string{
	"cursor_to_xml", "cursor_to_xmlschema", // Cursor to XML
	"database_to_xml", "database_to_xml_and_xmlschema", "database_to_xmlschema", // Database to XML
	"query_to_xml", "query_to_xml_and_xmlschema", "query_to_xmlschema", // Query to XML
	"schema_to_xml", "schema_to_xml_and_xmlschema", "schema_to_xmlschema", // Schema to XML
	"table_to_xml", "table_to_xml_and_xmlschema", "table_to_xmlschema", // Table to XML
	"xmlagg", "xmlcomment", "xmlconcat2", // XML Aggregation and Construction
	"xmlexists", "xmlvalidate", // XML Existence and Validation
	"xpath", "xpath_exists", // XPath Functions
	"xml_in", "xml_out", "xml_recv", "xml_send", // System XML I/O
	"xml", // Data Type Conversion
}

// Sample example: {func_call:{funcname:{string:{sval:"pg_advisory_lock"}}
func getFuncNameFromFuncCall(funcCallNode protoreflect.Message) string {
	funcnameField := funcCallNode.Get(funcCallNode.Descriptor().Fields().ByName("funcname"))
	funcnameList := funcnameField.List()
	var names []string
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
	fields := columnRefNode.Get(columnRefNode.Descriptor().Fields().ByName("fields"))
	fieldsList := fields.List()
	var names []string
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
