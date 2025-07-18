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
	"fmt"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func GetProtoMessageFromParseTree(parseTree *pg_query.ParseResult) protoreflect.Message {
	return parseTree.Stmts[0].Stmt.ProtoReflect()
}

func GetMsgFullName(msg protoreflect.Message) string {
	return string(msg.Descriptor().FullName())
}

// Sample example: {func_call:{funcname:{string:{sval:"pg_advisory_lock"}}
func GetFuncNameFromFuncCall(funcCallNode protoreflect.Message) (string, string) {
	if GetMsgFullName(funcCallNode) != PG_QUERY_FUNCCALL_NODE {
		return "", ""
	}

	funcnameList := GetListField(funcCallNode, "funcname")
	var names []string
	for i := 0; i < funcnameList.Len(); i++ {
		item := funcnameList.Get(i)
		name := GetStringValueFromNode(item.Message())
		if name != "" {
			names = append(names, name)
		}
	}

	if len(names) == 0 {
		return "", ""
	} else if len(names) == 1 {
		return "", names[0]
	}
	return names[0], names[1]
}

// Sample example:: {column_ref:{fields:{string:{sval:"xmax"}}
func GetColNameFromColumnRef(columnRefNode protoreflect.Message) (string, string) {
	if GetMsgFullName(columnRefNode) != PG_QUERY_COLUMNREF_NODE {
		return "", ""
	}

	fieldsList := GetListField(columnRefNode, "fields")
	var names []string
	for i := 0; i < fieldsList.Len(); i++ {
		item := fieldsList.Get(i)
		name := GetStringValueFromNode(item.Message())
		if name != "" {
			names = append(names, name)
		}
	}

	if len(names) == 0 {
		return "", ""
	} else if len(names) == 1 {
		return "", names[0]
	}
	return names[0], names[1]
}

// Sample example:: {column_ref:{fields:{string:{sval:"s"}}  fields:{string:{sval:"tableoid"}}  location:7}
func GetStringValueFromNode(nodeMsg protoreflect.Message) string {
	if nodeMsg == nil || !nodeMsg.IsValid() {
		return ""
	}

	nodeField := getOneofActiveField(nodeMsg, "node")
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
	// Represents a simple string literal in a query, such as names or values directly provided in the SQL text.
	case PG_QUERY_STRING_NODE:
		return GetStringField(node, "sval")
	// Represents a constant value in SQL expressions, often used for literal values like strings, numbers, or keywords.
	case PG_QUERY_ACONST_NODE:
		return getStringFromAConstMsg(node)
	// Represents a type casting operation in SQL, where a value is explicitly converted from one type to another using a cast expression.
	case PG_QUERY_TYPECAST_NODE:
		return getStringFromTypeCastMsg(node, "arg")
	// Represents the asterisk '*' used in SQL to denote the selection of all columns in a table. Example: SELECT * FROM employees;
	case PG_QUERY_ASTAR_NODE:
		return ""
	default:
		return ""
	}
}

// getStringFromAConstMsg extracts the string from an 'A_Const' node's 'sval' field
// Sample example:: rowexpr:{a_const:{sval:{sval:"//Product"}  location:124}}
func getStringFromAConstMsg(aConstMsg protoreflect.Message) string {
	svalMsg := GetMessageField(aConstMsg, "sval")
	if svalMsg == nil {
		return ""
	}

	return GetStringField(svalMsg, "sval")
}

// getStringFromTypeCastMsg traverses to a specified field and extracts the 'A_Const' string value
// Sample example:: rowexpr:{type_cast:{arg:{a_const:{sval:{sval:"/order/item"}
func getStringFromTypeCastMsg(nodeMsg protoreflect.Message, fieldName string) string {
	childMsg := GetMessageField(nodeMsg, fieldName)
	if childMsg == nil {
		return ""
	}

	return GetStringValueFromNode(childMsg)
}

/*
Note: XMLTABLE() is not a simple function(stored in FuncCall node), its a table function
which generates set of rows using the info(about rows, columns, content) provided to it
Hence its requires a more complex node structure(RangeTableFunc node) to represent.

XMLTABLE transforms XML data into relational table format, making it easier to query XML structures.
Detection in RangeTableFunc Node:
- docexpr: Refers to the XML data source, usually a column storing XML.
- rowexpr: XPath expression (starting with '/' or '//') defining the rows in the XML.
- columns: Specifies the data extraction from XML into relational columns.

Example: Converting XML data about books into a table:
SQL Query:

	SELECT x.*
	FROM XMLTABLE(
		'/bookstore/book'
		PASSING xml_column
		COLUMNS
			title TEXT PATH 'title',
			author TEXT PATH 'author'
	) AS x;

Parsetree: stmt:{select_stmt:{target_list:{res_target:{val:{column_ref:{fields:{string:{sval:"x"}}  fields:{a_star:{}}  location:7}}  location:7}}
from_clause:{range_table_func:
	{docexpr:{column_ref:{fields:{string:{sval:"xml_column"}}  location:57}}
	rowexpr:{a_const:{sval:{sval:"/bookstore/book"}  location:29}}
	columns:{range_table_func_col:{colname:"title"  type_name:{names:{string:{sval:"text"}}  typemod:-1  location:87}  colexpr:{a_const:{sval:{sval:"title"}  location:97}}  location:81}}
	columns:{range_table_func_col:{colname:"author"  type_name:{names:{string:{sval:"text"}}  typemod:-1  location:116}  colexpr:{a_const:{sval:{sval:"author"}  location:126}}  location:109}}
alias:{aliasname:"x"}  location:17}}  limit_option:LIMIT_OPTION_DEFAULT  op:SETOP_NONE}}  stmt_len:142

Here, 'docexpr' points to 'xml_column' containing XML data, 'rowexpr' selects each 'book' node, and 'columns' extract 'title' and 'author' from each book.
Hence Presence of XPath in 'rowexpr' and structured 'columns' typically indicates XMLTABLE usage.
*/
// Function to detect if a RangeTableFunc node represents XMLTABLE()
func IsXMLTable(rangeTableFunc protoreflect.Message) bool {
	if GetMsgFullName(rangeTableFunc) != PG_QUERY_RANGETABLEFUNC_NODE {
		return false
	}

	log.Debug("checking if range table func node is for XMLTABLE()")
	// Check for 'docexpr' field
	docexprField := rangeTableFunc.Descriptor().Fields().ByName("docexpr")
	if docexprField == nil {
		return false
	}
	docexprNode := rangeTableFunc.Get(docexprField).Message()
	if docexprNode == nil {
		return false
	}

	// Check for 'rowexpr' field
	rowexprField := rangeTableFunc.Descriptor().Fields().ByName("rowexpr")
	if rowexprField == nil {
		return false
	}
	rowexprNode := rangeTableFunc.Get(rowexprField).Message()
	if rowexprNode == nil {
		return false
	}

	xpath := GetStringValueFromNode(rowexprNode)
	log.Debugf("xpath expression in the node: %s\n", xpath)
	// Keep both cases check(param placeholder and absolute check)
	if xpath == "" {
		// Attempt to check if 'rowexpr' is a parameter placeholder like '$1'
		isPlaceholder := IsParameterPlaceholder(rowexprNode)
		if !isPlaceholder {
			return false
		}
	} else if !IsXPathExprForXmlTable(xpath) {
		return false
	}

	// Check for 'columns' field
	columnsField := rangeTableFunc.Descriptor().Fields().ByName("columns")
	if columnsField == nil {
		return false
	}

	columnsList := rangeTableFunc.Get(columnsField).List()
	if columnsList.Len() == 0 {
		return false
	}

	// this means all the required fields of RangeTableFunc node for being a XMLTABLE() are present
	return true
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

// IsParameterPlaceholder checks if the given node represents a parameter placeholder like $1
func IsParameterPlaceholder(nodeMsg protoreflect.Message) bool {
	if nodeMsg == nil || !nodeMsg.IsValid() {
		return false
	}

	nodeField := getOneofActiveField(nodeMsg, "node")
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

// getOneofActiveField retrieves the active field from a specified oneof in a Protobuf message.
// It returns the FieldDescriptor of the active field if a field is set, or nil if no field is set or the oneof is not found.
func getOneofActiveField(msg protoreflect.Message, oneofName string) protoreflect.FieldDescriptor {
	if msg == nil {
		return nil
	}

	// Get the descriptor of the message and find the oneof by name
	descriptor := msg.Descriptor()
	if descriptor == nil {
		return nil
	}

	oneofDescriptor := descriptor.Oneofs().ByName(protoreflect.Name(oneofName))
	if oneofDescriptor == nil {
		return nil
	}

	// Determine which field within the oneof is set
	return msg.WhichOneof(oneofDescriptor)
}

func GetStatementType(msg protoreflect.Message) string {
	nodeMsg := getOneofActiveField(msg, "node")
	if nodeMsg == nil {
		return ""
	}

	// Get the message corresponding to the set field
	nodeValue := msg.Get(nodeMsg)
	node := nodeValue.Message()
	if node == nil || !node.IsValid() {
		return ""
	}
	return GetMsgFullName(node)
}

func getOneofActiveNode(msg protoreflect.Message) protoreflect.Message {
	nodeField := getOneofActiveField(msg, "node")
	if nodeField == nil {
		return nil
	}

	value := msg.Get(nodeField)
	node := value.Message()
	if node == nil || !node.IsValid() {
		return nil
	}

	return node
}

// == Generic helper functions ==

// GetStringField retrieves a string field from a message.
// Sample example:: {column_ref:{fields:{string:{sval:"s"}}  fields:{string:{sval:"tableoid"}}  location:7}
func GetStringField(msg protoreflect.Message, fieldName string) string {
	field := msg.Descriptor().Fields().ByName(protoreflect.Name(fieldName))
	if field != nil && msg.Has(field) {
		return msg.Get(field).String()
	}
	return ""
}

// GetMessageField retrieves a message field from a message.
func GetMessageField(msg protoreflect.Message, fieldName string) protoreflect.Message {
	field := msg.Descriptor().Fields().ByName(protoreflect.Name(fieldName))
	if field != nil && msg.Has(field) {
		return msg.Get(field).Message()
	}
	return nil
}

func GetBoolField(msg protoreflect.Message, fieldName string) bool {
	field := msg.Descriptor().Fields().ByName(protoreflect.Name(fieldName))
	if field != nil && msg.Has(field) {
		return msg.Get(field).Bool()
	}
	return false
}

// GetListField retrieves a list field from a message.
func GetListField(msg protoreflect.Message, fieldName string) protoreflect.List {
	field := msg.Descriptor().Fields().ByName(protoreflect.Name(fieldName))
	if field != nil && msg.Has(field) {
		return msg.Get(field).List()
	}
	return nil
}

// GetEnumNumField retrieves a enum field from a message
// FieldDescriptor{Syntax: proto3, FullName: pg_query.JsonFuncExpr.op, Number: 1, Cardinality: optional, Kind: enum, HasJSONName: true, JSONName: "op", Enum: pg_query.JsonExprOp}
// val:{json_func_expr:{op:JSON_QUERY_OP  context_item:{raw_expr:{column_ref:{fields:{string:{sval:"details"}}  location:2626}}  format:{format_type:JS_FORMAT_DEFAULT  encoding:JS_ENC_DEFAULT
func GetEnumNumField(msg protoreflect.Message, fieldName string) protoreflect.EnumNumber {
	field := msg.Descriptor().Fields().ByName(protoreflect.Name(fieldName))
	if field != nil && msg.Has(field) {
		return msg.Get(field).Enum()
	}
	return 0
}

// GetSchemaAndObjectName extracts the schema and object name from a list.
func GetSchemaAndObjectName(nameList protoreflect.List) (string, string) {
	var schemaName, objectName string

	if nameList.Len() == 1 {
		objectName = GetStringField(nameList.Get(0).Message(), "string")
	} else if nameList.Len() == 2 {
		schemaName = GetStringField(nameList.Get(0).Message(), "string")
		objectName = GetStringField(nameList.Get(1).Message(), "string")
	}
	return schemaName, objectName
}

func ProtoAsSelectStmt(msg protoreflect.Message) (*pg_query.SelectStmt, error) {
	selectStmtNode, ok := msg.Interface().(*pg_query.SelectStmt)
	if !ok {
		return nil, fmt.Errorf("failed to cast msg to %s", PG_QUERY_SELECTSTMT_NODE)
	}
	return selectStmtNode, nil
}

func ProtoAsAConstNode(msg protoreflect.Message) (*pg_query.A_Const, bool) {
	aConstNode, ok := msg.Interface().(*pg_query.A_Const)
	if !ok {
		return nil, false
	}
	return aConstNode, true
}

func ProtoAsCreateExtensionStmt(msg protoreflect.Message) (*pg_query.CreateExtensionStmt, bool) {
	createExtStmtNode, ok := msg.Interface().(*pg_query.CreateExtensionStmt)
	if !ok {
		return nil, false
	}
	return createExtStmtNode, true
}

func ProtoAsCreateForeignTableStmt(msg protoreflect.Message) (*pg_query.CreateForeignTableStmt, bool) {
	createForeignTableStmtNode, ok := msg.Interface().(*pg_query.CreateForeignTableStmt)
	if !ok {
		return nil, false
	}
	return createForeignTableStmtNode, true
}

func ProtoAsCTENode(msg protoreflect.Message) (*pg_query.CommonTableExpr, error) {
	cteNode, ok := msg.Interface().(*pg_query.CommonTableExpr)
	if !ok {
		return nil, fmt.Errorf("failed to cast msg to %s", PG_QUERY_CTE_NODE)
	}
	return cteNode, nil
}

func ProtoAsDefElemNode(msg protoreflect.Message) (*pg_query.DefElem, bool) {
	defElemNode, ok := msg.Interface().(*pg_query.DefElem)
	if !ok {
		return nil, false
	}
	return defElemNode, true
}

func ProtoAsIndexStmtNode(msg protoreflect.Message) (*pg_query.IndexStmt, bool) {
	indexStmtNode, ok := msg.Interface().(*pg_query.IndexStmt)
	if !ok {
		return nil, false
	}

	return indexStmtNode, true
}

func ProtoAsIndexElemNode(msg protoreflect.Message) (*pg_query.IndexElem, bool) {
	indexElemNode, ok := msg.Interface().(*pg_query.IndexElem)
	if !ok {
		return nil, false
	}

	return indexElemNode, true
}

func ProtoAsTableConstraintNode(msg protoreflect.Message) (*pg_query.Constraint, bool) {
	consNode, ok := msg.Interface().(*pg_query.Constraint)
	if !ok {
		return nil, false
	}

	return consNode, true
}

func ProtoAsAlterTableStmtNode(msg protoreflect.Message) (*pg_query.AlterTableStmt, bool) {
	alterTableStmtNode, ok := msg.Interface().(*pg_query.AlterTableStmt)
	if !ok {
		return nil, false
	}

	return alterTableStmtNode, true
}

func ProtoAsAlterTableCmdNode(msg protoreflect.Message) (*pg_query.AlterTableCmd, bool) {
	alterTableCmdNode, ok := msg.Interface().(*pg_query.AlterTableCmd)
	if !ok {
		return nil, false
	}

	return alterTableCmdNode, true
}

func ProtoAsReplicaIdentityStmtNode(msg protoreflect.Message) (*pg_query.ReplicaIdentityStmt, bool) {
	replicaIdentityStmtNode, ok := msg.Interface().(*pg_query.ReplicaIdentityStmt)
	if !ok {
		return nil, false
	}

	return replicaIdentityStmtNode, true
}

func ProtoAsTransactionStmt(msg protoreflect.Message) (*pg_query.TransactionStmt, error) {
	node, ok := msg.Interface().(*pg_query.TransactionStmt)
	if !ok {
		return nil, fmt.Errorf("failed to cast msg to %s", PG_QUERY_TRANSACTION_STMT_NODE)
	}

	return node, nil
}

func ProtoAsCreateSchemaStmtNode(msg protoreflect.Message) (*pg_query.CreateSchemaStmt, bool) {
	createSchemaStmtNode, ok := msg.Interface().(*pg_query.CreateSchemaStmt)
	if !ok {
		return nil, false
	}

	return createSchemaStmtNode, true
}

func ProtoAsRenameStmtNode(msg protoreflect.Message) (*pg_query.RenameStmt, bool) {
	alterSchemaStmtNode, ok := msg.Interface().(*pg_query.RenameStmt)
	if !ok {
		return nil, false
	}

	return alterSchemaStmtNode, true
}

func ProtoAsAlterOwnerStmtNode(msg protoreflect.Message) (*pg_query.AlterOwnerStmt, bool) {
	alterOwnerStmtNode, ok := msg.Interface().(*pg_query.AlterOwnerStmt)
	if !ok {
		return nil, false
	}

	return alterOwnerStmtNode, true
}

func ProtoAsDropStmtNode(msg protoreflect.Message) (*pg_query.DropStmt, bool) {
	dropStmtNode, ok := msg.Interface().(*pg_query.DropStmt)
	if !ok {
		return nil, false
	}

	return dropStmtNode, true
}

func ProtoAsGrantStmtNode(msg protoreflect.Message) (*pg_query.GrantStmt, bool) {
	grantStmtNode, ok := msg.Interface().(*pg_query.GrantStmt)
	if !ok {
		return nil, false
	}

	return grantStmtNode, true
}

func ProtoAsAlterObjectSchemaStmtNode(msg protoreflect.Message) (*pg_query.AlterObjectSchemaStmt, bool) {
	alterObjectSchemaStmtNode, ok := msg.Interface().(*pg_query.AlterObjectSchemaStmt)
	if !ok {
		return nil, false
	}

	return alterObjectSchemaStmtNode, true
}

func ProtoAsCreateSeqStmtNode(msg protoreflect.Message) (*pg_query.CreateSeqStmt, bool) {
	createSeqStmtNode, ok := msg.Interface().(*pg_query.CreateSeqStmt)
	if !ok {
		return nil, false
	}

	return createSeqStmtNode, true
}

func ProtoAsAlterSeqStmtNode(msg protoreflect.Message) (*pg_query.AlterSeqStmt, bool) {
	alterSeqStmtNode, ok := msg.Interface().(*pg_query.AlterSeqStmt)
	if !ok {
		return nil, false
	}

	return alterSeqStmtNode, true
}

func ProtoAsDefineStmtNode(msg protoreflect.Message) (*pg_query.DefineStmt, bool) {
	defineStmtNode, ok := msg.Interface().(*pg_query.DefineStmt)
	if !ok {
		return nil, false
	}

	return defineStmtNode, true
}

func ProtoAsCreateEnumStmtNode(msg protoreflect.Message) (*pg_query.CreateEnumStmt, bool) {
	createEnumStmtNode, ok := msg.Interface().(*pg_query.CreateEnumStmt)
	if !ok {
		return nil, false
	}

	return createEnumStmtNode, true
}

func ProtoAsAlterEnumStmtNode(msg protoreflect.Message) (*pg_query.AlterEnumStmt, bool) {
	alterEnumStmtNode, ok := msg.Interface().(*pg_query.AlterEnumStmt)
	if !ok {
		return nil, false
	}

	return alterEnumStmtNode, true
}

func ProtoAsCompositeTypeStmtNode(msg protoreflect.Message) (*pg_query.CompositeTypeStmt, bool) {
	compositeTypeStmtNode, ok := msg.Interface().(*pg_query.CompositeTypeStmt)
	if !ok {
		return nil, false
	}
	return compositeTypeStmtNode, true
}

func ProtoAsCreateDomainStmtNode(msg protoreflect.Message) (*pg_query.CreateDomainStmt, bool) {
	createDomainStmtNode, ok := msg.Interface().(*pg_query.CreateDomainStmt)
	if !ok {
		return nil, false
	}
	return createDomainStmtNode, true
}

func ProtoAsRangeVarNode(msg protoreflect.Message) (*pg_query.RangeVar, bool) {
	rangeVarNode, ok := msg.Interface().(*pg_query.RangeVar)
	if !ok {
		return nil, false
	}
	return rangeVarNode, true
}

func ProtoAsColumnDef(msg protoreflect.Message) (*pg_query.ColumnDef, bool) {
	columnDefNode, ok := msg.Interface().(*pg_query.ColumnDef)
	if !ok {
		return nil, false
	}
	return columnDefNode, true
}

func ProtoAsColumnRefNode(msg protoreflect.Message) (*pg_query.ColumnRef, bool) {
	columnRefNode, ok := msg.Interface().(*pg_query.ColumnRef)
	if !ok {
		return nil, false
	}
	return columnRefNode, true
}

func ProtoAsColumnRef(msg protoreflect.Message) (*pg_query.ColumnRef, bool) {
	columnRefNode, ok := msg.Interface().(*pg_query.ColumnRef)
	if !ok {
		return nil, false
	}
	return columnRefNode, true
}

func ProtoAsResTargetNode(msg protoreflect.Message) (*pg_query.ResTarget, bool) {
	resTargetNode, ok := msg.Interface().(*pg_query.ResTarget)
	if !ok {
		return nil, false
	}
	return resTargetNode, true
}

func ProtoAsAliasNode(msg protoreflect.Message) (*pg_query.Alias, bool) {
	aliasNode, ok := msg.Interface().(*pg_query.Alias)
	if !ok {
		return nil, false
	}
	return aliasNode, true
}

func ProtoAsTypeNameNode(msg protoreflect.Message) (*pg_query.TypeName, bool) {
	typeNameNode, ok := msg.Interface().(*pg_query.TypeName)
	if !ok {
		return nil, false
	}
	return typeNameNode, true
}

func ProtoAsRoleSpecNode(msg protoreflect.Message) (*pg_query.RoleSpec, bool) {
	roleSpecNode, ok := msg.Interface().(*pg_query.RoleSpec)
	if !ok {
		return nil, false
	}
	return roleSpecNode, true
}

func ProtoAsCreateStmtNode(msg protoreflect.Message) (*pg_query.CreateStmt, bool) {
	createStmtNode, ok := msg.Interface().(*pg_query.CreateStmt)
	if !ok {
		return nil, false
	}
	return createStmtNode, true
}

/*
Example:
options:{def_elem:{defname:"security_invoker" arg:{string:{sval:"true"}} defaction:DEFELEM_UNSPEC location:32}}
options:{def_elem:{defname:"security_barrier" arg:{string:{sval:"false"}} defaction:DEFELEM_UNSPEC location:57}}
Extract all defnames from the def_eleme node
*/
func TraverseAndExtractDefNamesFromDefElem(msg protoreflect.Message) (map[string]string, error) {
	defNamesWithValues := make(map[string]string)
	collectorFunc := func(msg protoreflect.Message) error {
		if GetMsgFullName(msg) != PG_QUERY_DEFELEM_NODE {
			return nil
		}

		defElemNode, ok := ProtoAsDefElemNode(msg)
		if !ok {
			return fmt.Errorf("failed to cast msg to %s", PG_QUERY_DEFELEM_NODE)
		}

		defName := defElemNode.Defname
		arg := defElemNode.GetArg()
		if arg != nil && arg.GetString_() != nil {
			defElemVal := arg.GetString_().Sval
			defNamesWithValues[defName] = defElemVal
		} else {
			log.Warnf("defElem Node doesn't have arg or the arg is not the string type [%s]", defElemNode)
			//TODO: see how to handle this later where GetString_() is not directly available or arg is of different type
			//e.g. defname:"provider"  arg:{type_name:{names:{string:{sval:"icu"}}  typemod:-1  location:37}}  defaction:DEFELEM_UNSPEC  location:26defname:"locale"
			defNamesWithValues[defName] = ""
		}
		return nil
	}
	visited := make(map[protoreflect.Message]bool)
	err := TraverseParseTree(msg, visited, collectorFunc)
	if err != nil {
		return nil, fmt.Errorf("failed to traverse parse tree for fetching defnames: %w", err)
	}

	return defNamesWithValues, nil
}

func GetAIndirectionNode(msg protoreflect.Message) (*pg_query.A_Indirection, bool) {
	protoMsg := msg.Interface().(protoreflect.ProtoMessage)
	aIndirection, ok := protoMsg.(*pg_query.A_Indirection)
	return aIndirection, ok
}
