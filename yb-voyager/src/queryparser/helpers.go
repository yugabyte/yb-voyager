package queryparser

import "google.golang.org/protobuf/reflect/protoreflect"

const (
	DOCS_LINK_PREFIX        = "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/"
	POSTGRESQL_PREFIX       = "postgresql/"
	ADVISORY_LOCKS_DOC_LINK = DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#advisory-locks"
	SYSTEM_COLUMNS_DOC_LINK = DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#system-columns"
	XML_FUNCTIONS_DOC_LINK  = DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#xml-functions"
)

// Sample example: {func_call:{funcname:{string:{sval:"pg_advisory_lock"}}
func GetFuncNameFromFuncCall(funcCallNode protoreflect.Message) string {
	if GetMsgFullName(funcCallNode) != PG_QUERY_FUNCCALL_NODE {
		return ""
	}

	funcnameField := funcCallNode.Get(funcCallNode.Descriptor().Fields().ByName("funcname"))
	funcnameList := funcnameField.List()
	var names []string

	// TODO: simplification to directly access last item of funcnameList
	for i := 0; i < funcnameList.Len(); i++ {
		item := funcnameList.Get(i)
		name := GetStringValueFromNode(item.Message())
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
func GetColNameFromColumnRef(columnRefNode protoreflect.Message) string {
	if GetMsgFullName(columnRefNode) != PG_QUERY_COLUMNREF_NODE {
		return ""
	}

	fields := columnRefNode.Get(columnRefNode.Descriptor().Fields().ByName("fields"))
	fieldsList := fields.List()
	var names []string

	// TODO: simplification to directly access last item of fieldsList
	for i := 0; i < fieldsList.Len(); i++ {
		item := fieldsList.Get(i)
		name := GetStringValueFromNode(item.Message())
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
func GetStringValueFromNode(nodeMsg protoreflect.Message) string {
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

func GetMsgFullName(msg protoreflect.Message) string {
	return string(msg.Descriptor().FullName())
}
