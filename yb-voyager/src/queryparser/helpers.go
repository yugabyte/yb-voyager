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
package queryparser

import (
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

const (
	DOCS_LINK_PREFIX        = "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/"
	POSTGRESQL_PREFIX       = "postgresql/"
	ADVISORY_LOCKS_DOC_LINK = DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#advisory-locks-is-not-yet-implemented"
	SYSTEM_COLUMNS_DOC_LINK = DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#system-columns-is-not-yet-supported"
	XML_FUNCTIONS_DOC_LINK  = DOCS_LINK_PREFIX + POSTGRESQL_PREFIX + "#xml-functions-is-not-yet-supported"
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

	// Retrieve the 'node' oneof descriptor
	nodeOneof := nodeMsg.Descriptor().Oneofs().ByName("node")
	if nodeOneof == nil {
		return ""
	}

	// Determine which field is set in the 'node' oneof
	nodeField := nodeMsg.WhichOneof(nodeOneof)
	if nodeField == nil {
		return ""
	}

	// Get the message corresponding to the set field
	nodeValue := nodeMsg.Get(nodeField)
	node := nodeValue.Message()
	if node == nil || !node.IsValid() {
		return ""
	}

	nodeType := node.Descriptor().FullName()
	switch nodeType {
	case PG_QUERY_STRING_NODE:
		return extractStringField(node, "sval")
	case PG_QUERY_ACONST_NODE:
		return extractAConstString(node)
	case PG_QUERY_TYPECAST_NODE:
		return traverseAndExtractAConst(node, "arg")
	// example: SELECT * FROM employees;
	case PG_QUERY_ASTAR_NODE:
		return ""
	default:
		return ""
	}
}

// extractStringField safely extracts a string field from a node
// Sample example:: {column_ref:{fields:{string:{sval:"s"}}  fields:{string:{sval:"tableoid"}}  location:7}
func extractStringField(node protoreflect.Message, fieldName string) string {
	strField := node.Descriptor().Fields().ByName(protoreflect.Name(fieldName))
	if strField == nil || !node.Has(strField) {
		return ""
	}
	return node.Get(strField).String()
}

// extractAConstString extracts the string from an 'A_Const' node's 'sval' field
// Sample example:: rowexpr:{a_const:{sval:{sval:"//Product"}  location:124}}
func extractAConstString(aConstMsg protoreflect.Message) string {
	// Extract the 'sval' field from 'A_Const'
	svalField := aConstMsg.Descriptor().Fields().ByName("sval")
	if svalField == nil || !aConstMsg.Has(svalField) {
		return ""
	}

	svalMsg := aConstMsg.Get(svalField).Message()
	if svalMsg == nil || !svalMsg.IsValid() {
		return ""
	}

	// Ensure svalMsg is of type 'pg_query.String'
	if svalMsg.Descriptor().FullName() != "pg_query.String" {
		return ""
	}

	// Extract the actual string value from 'pg_query.String'
	return extractStringField(svalMsg, "sval")
}

// traverseAndExtractAConst traverses to a specified field and extracts the 'A_Const' string value
// Sample example:: rowexpr:{type_cast:{arg:{a_const:{sval:{sval:"/order/item"}
func traverseAndExtractAConst(nodeMsg protoreflect.Message, fieldName string) string {
	field := nodeMsg.Descriptor().Fields().ByName(protoreflect.Name(fieldName))
	if field == nil || !nodeMsg.Has(field) {
		return ""
	}

	childMsg := nodeMsg.Get(field).Message()
	if childMsg == nil || !childMsg.IsValid() {
		return ""
	}

	return GetStringValueFromNode(childMsg)
}

/*
isXPathExprForXmlTable checks whether a given string is a valid XPath expression for XMLTABLE()'s rowexpr.
It returns true if the expression starts with '/' or '//', indicating an absolute or anywhere path.
This covers the primary cases used in XMLTABLE() for selecting XML nodes as rows.

XPath Expression Cases Covered for XMLTABLE():
1. Absolute Paths:
- Starts with a single '/' indicating the root node.
- Example: "/library/book"

2. Anywhere Paths:
- Starts with double '//' indicating selection from anywhere in the document.
- Example: "//book/author"

For a comprehensive overview of XPath expressions, refer to:
https://developer.mozilla.org/en-US/docs/Web/XPath
*/
func IsXPathExprForXmlTable(expression string) bool {
	// Trim leading and trailing whitespace
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return false
	}

	// Check if the expression starts with '/' or '//'
	return strings.HasPrefix(expression, "/") || strings.HasPrefix(expression, "//")
}

func GetMsgFullName(msg protoreflect.Message) string {
	return string(msg.Descriptor().FullName())
}

// IsParameterPlaceholder checks if the given node represents a parameter placeholder like $1
func IsParameterPlaceholder(nodeMsg protoreflect.Message) bool {
	if nodeMsg == nil || !nodeMsg.IsValid() {
		return false
	}

	// Retrieve the 'node' oneof descriptor
	nodeOneof := nodeMsg.Descriptor().Oneofs().ByName("node")
	if nodeOneof == nil {
		return false
	}

	// Determine which field is set in the 'node' oneof
	nodeField := nodeMsg.WhichOneof(nodeOneof)
	if nodeField == nil {
		return false
	}

	// Get the message corresponding to the set field
	nodeValue := nodeMsg.Get(nodeField)
	node := nodeValue.Message()
	if node == nil || !node.IsValid() {
		return false
	}

	// Identify the type of the node
	nodeType := node.Descriptor().FullName()
	if nodeType == PG_QUERY_PARAMREF_NODE {
		// This node represents a parameter reference like $1
		return true
	}

	return false
}
