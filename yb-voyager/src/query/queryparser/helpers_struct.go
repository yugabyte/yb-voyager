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
	"slices"
	"strconv"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

func IsPLPGSQLObject(parseTree *pg_query.ParseResult) bool {
	// CREATE FUNCTION is same parser NODE for FUNCTION/PROCEDURE
	node, ok := getCreateFuncStmtNode(parseTree)
	return ok && (node.CreateFunctionStmt.SqlBody == nil) //TODO fix proper https://github.com/pganalyze/pg_query_go/issues/129
}

func IsViewObject(parseTree *pg_query.ParseResult) bool {
	_, isViewStmt := parseTree.Stmts[0].Stmt.Node.(*pg_query.Node_ViewStmt)
	return isViewStmt
}

func IsMviewObject(parseTree *pg_query.ParseResult) bool {
	createAsNode, isCreateAsStmt := getCreateTableAsStmtNode(parseTree) //for MVIEW case
	return isCreateAsStmt && createAsNode.CreateTableAsStmt.Objtype == pg_query.ObjectType_OBJECT_MATVIEW
}

func getDefineStmtNode(parseTree *pg_query.ParseResult) (*pg_query.DefineStmt, bool) {
	node, ok := parseTree.Stmts[0].Stmt.Node.(*pg_query.Node_DefineStmt)
	return node.DefineStmt, ok
}

func IsCollationObject(parseTree *pg_query.ParseResult) bool {
	collation, ok := getDefineStmtNode(parseTree)
	/*
		stmts:{stmt:{define_stmt:{kind:OBJECT_COLLATION  defnames:{string:{sval:"ignore_accents"}}  definition:{def_elem:{defname:"provider"
		arg:{type_name:{names:{string:{sval:"icu"}}  typemod:-1  location:48}}  defaction:DEFELEM_UNSPEC  location:37}}  definition:{def_elem:{defname:"locale"
		arg:{string:{sval:"und-u-ks-level1-kc-true"}}  defaction:DEFELEM_UNSPEC  location:55}}  definition:{def_elem:{defname:"deterministic"
		arg:{string:{sval:"false"}}  defaction:DEFELEM_UNSPEC  location:91}}}}  stmt_len:113}
	*/
	return ok && collation.Kind == pg_query.ObjectType_OBJECT_COLLATION
}

func GetObjectTypeAndObjectName(parseTree *pg_query.ParseResult) (string, string) {
	createFuncNode, isCreateFunc := getCreateFuncStmtNode(parseTree)
	viewNode, isViewStmt := getCreateViewNode(parseTree)
	createAsNode, _ := getCreateTableAsStmtNode(parseTree)
	createTableNode, isCreateTable := getCreateTableStmtNode(parseTree)
	createIndexNode, isCreateIndex := getCreateIndexStmtNode(parseTree)
	alterTableNode, isAlterTable := getAlterStmtNode(parseTree)
	switch true {
	case isCreateFunc:
		/*
			version:160001 stmts:{stmt:{create_function_stmt:{replace:true funcname:{string:{sval:"public"}} funcname:{string:{sval:"add_employee"}}
			parameters:{function_parameter:{name:"emp_name" arg_type:{names:{string:{sval:"pg_catalog"}} names:{string:{sval:"varchar"}}
			typemod:-1 location:62} mode:FUNC_PARAM_DEFAULT}} parameters:{funct ...

			version:160001 stmts:{stmt:{create_function_stmt:{is_procedure:true replace:true funcname:{string:{sval:"public"}}
			funcname:{string:{sval:"add_employee"}} parameters:{function_parameter:{name:"emp_name" arg_type:{names:{string:{sval:"pg_catalog"}}
			names:{string:{sval:"varchar"}} typemod:-1 location:63} mode:FUNC_PARAM_DEFAULT}} ...
		*/
		stmt := createFuncNode.CreateFunctionStmt
		objectType := "FUNCTION"
		if stmt.IsProcedure {
			objectType = "PROCEDURE"
		}
		funcNameList := stmt.GetFuncname()
		funcSchemaName, funcName := getSchemaAndObjectName(funcNameList)
		return objectType, utils.BuildObjectName(funcSchemaName, funcName)
	case isViewStmt:
		viewName := viewNode.ViewStmt.View
		return "VIEW", GetObjectNameFromRangeVar(viewName)
	case IsMviewObject(parseTree):
		intoMview := createAsNode.CreateTableAsStmt.Into.Rel
		return "MVIEW", GetObjectNameFromRangeVar(intoMview)
	case isCreateTable:
		return "TABLE", GetObjectNameFromRangeVar(createTableNode.CreateStmt.Relation)
	case isAlterTable:
		return "TABLE", GetObjectNameFromRangeVar(alterTableNode.AlterTableStmt.Relation)
	case isCreateIndex:
		indexName := createIndexNode.IndexStmt.Idxname
		schemaName := createIndexNode.IndexStmt.Relation.GetSchemaname()
		tableName := createIndexNode.IndexStmt.Relation.GetRelname()
		fullyQualifiedName := utils.BuildObjectName(schemaName, tableName)
		displayObjName := fmt.Sprintf("%s ON %s", indexName, fullyQualifiedName)
		return "INDEX", displayObjName
	default:
		panic("unsupported type of parseResult")
	}
}

func isArrayType(typeName *pg_query.TypeName) bool {
	return len(typeName.GetArrayBounds()) > 0
}

// Range Var is the struct to get the relation information like relation name, schema name, persisted relation or not, etc..
func GetObjectNameFromRangeVar(obj *pg_query.RangeVar) string {
	if obj == nil {
		log.Infof("RangeVar is nil")
		return ""
	}

	schema := obj.Schemaname
	name := obj.Relname
	return utils.BuildObjectName(schema, name)
}

func getSchemaAndObjectName(nameList []*pg_query.Node) (string, string) {
	objName := ""
	schemaName := ""
	if len(nameList) > 0 {
		objName = nameList[len(nameList)-1].GetString_().Sval // obj name can be qualified / unqualifed or native / non-native proper func name will always be available at last index
	}
	if len(nameList) >= 2 { // Names list will have all the parts of qualified func name
		schemaName = nameList[len(nameList)-2].GetString_().Sval // // obj name can be qualified / unqualifed or native / non-native proper schema name will always be available at last 2nd index
	}
	return schemaName, objName
}

/*
extractTypeMods extracts type modifier values (typmods) from a slice of pg_query AST nodes.

Typmods represent additional information about a column's datatype — such as length for VARCHAR
or precision and scale for NUMERIC.

Examples:
  - VARCHAR(10) has typmods: [10]
  - NUMERIC(8, 2) has typmods: [8, 2]

This function iterates over the typmod nodes, looks for A_Const integer constants, and returns
the list of extracted integer values as a slice.

Input AST (for NUMERIC(8,2)):

	[ a_const: { ival: { ival: 8 } }, a_const: { ival: { ival: 2 } } ]

Output:

	[]int32{8, 2}
*/
func extractTypeMods(typmods []*pg_query.Node) []int32 {
	result := make([]int32, 0, len(typmods))
	for _, node := range typmods {
		if aconst := node.GetAConst(); aconst != nil {
			switch val := aconst.Val.(type) {
			case *pg_query.A_Const_Ival:
				if val.Ival != nil {
					result = append(result, val.Ival.Ival)
				}
			}
		}
	}
	return result
}

func getCreateTableAsStmtNode(parseTree *pg_query.ParseResult) (*pg_query.Node_CreateTableAsStmt, bool) {
	node, ok := parseTree.Stmts[0].Stmt.Node.(*pg_query.Node_CreateTableAsStmt)
	return node, ok
}

func getCreateViewNode(parseTree *pg_query.ParseResult) (*pg_query.Node_ViewStmt, bool) {
	node, ok := parseTree.Stmts[0].Stmt.Node.(*pg_query.Node_ViewStmt)
	return node, ok
}

func getCreateFuncStmtNode(parseTree *pg_query.ParseResult) (*pg_query.Node_CreateFunctionStmt, bool) {
	node, ok := parseTree.Stmts[0].Stmt.Node.(*pg_query.Node_CreateFunctionStmt)
	return node, ok
}

func getCreateExtensionStmtNode(parseTree *pg_query.ParseResult) (*pg_query.Node_CreateExtensionStmt, bool) {
	node, ok := parseTree.Stmts[0].Stmt.Node.(*pg_query.Node_CreateExtensionStmt)
	return node, ok
}

func getCreateTableStmtNode(parseTree *pg_query.ParseResult) (*pg_query.Node_CreateStmt, bool) {
	node, ok := parseTree.Stmts[0].Stmt.Node.(*pg_query.Node_CreateStmt)
	return node, ok
}

func getCreateIndexStmtNode(parseTree *pg_query.ParseResult) (*pg_query.Node_IndexStmt, bool) {
	node, ok := parseTree.Stmts[0].Stmt.Node.(*pg_query.Node_IndexStmt)
	return node, ok
}

func getAlterStmtNode(parseTree *pg_query.ParseResult) (*pg_query.Node_AlterTableStmt, bool) {
	node, ok := parseTree.Stmts[0].Stmt.Node.(*pg_query.Node_AlterTableStmt)
	return node, ok
}

func getCreateTriggerStmtNode(parseTree *pg_query.ParseResult) (*pg_query.Node_CreateTrigStmt, bool) {
	node, ok := parseTree.Stmts[0].Stmt.Node.(*pg_query.Node_CreateTrigStmt)
	return node, ok
}

func getPolicyStmtNode(parseTree *pg_query.ParseResult) (*pg_query.Node_CreatePolicyStmt, bool) {
	node, ok := parseTree.Stmts[0].Stmt.Node.(*pg_query.Node_CreatePolicyStmt)
	return node, ok
}

func getCompositeTypeStmtNode(parseTree *pg_query.ParseResult) (*pg_query.Node_CompositeTypeStmt, bool) {
	node, ok := parseTree.Stmts[0].Stmt.Node.(*pg_query.Node_CompositeTypeStmt)
	return node, ok
}

func getEnumTypeStmtNode(parseTree *pg_query.ParseResult) (*pg_query.Node_CreateEnumStmt, bool) {
	node, ok := parseTree.Stmts[0].Stmt.Node.(*pg_query.Node_CreateEnumStmt)
	return node, ok
}
func getForeignTableStmtNode(parseTree *pg_query.ParseResult) (*pg_query.Node_CreateForeignTableStmt, bool) {
	node, ok := parseTree.Stmts[0].Stmt.Node.(*pg_query.Node_CreateForeignTableStmt)
	return node, ok
}

func IsFunctionObject(parseTree *pg_query.ParseResult) bool {
	funcNode, ok := getCreateFuncStmtNode(parseTree)
	if !ok {
		return false
	}
	return !funcNode.CreateFunctionStmt.IsProcedure
}

/*
return type ex-
CREATE OR REPLACE FUNCTION public.process_combined_tbl(

	...

)
RETURNS public.combined_tbl.maddr%TYPE AS
return_type:{names:{string:{sval:"public"}}  names:{string:{sval:"combined_tbl"}}  names:{string:{sval:"maddr"}}
pct_type:true  typemod:-1  location:226}
*/
func GetReturnTypeOfFunc(parseTree *pg_query.ParseResult) string {
	funcNode, _ := getCreateFuncStmtNode(parseTree)
	returnType := funcNode.CreateFunctionStmt.GetReturnType()
	return convertParserTypeNameToString(returnType)
}

func getQualifiedTypeName(typeNames []*pg_query.Node) string {
	var typeNameStrings []string
	for _, n := range typeNames {
		typeNameStrings = append(typeNameStrings, n.GetString_().Sval)
	}
	return strings.Join(typeNameStrings, ".")
}

func convertParserTypeNameToString(typeVar *pg_query.TypeName) string {
	if typeVar == nil {
		return ""
	}
	typeNames := typeVar.GetNames()
	finalTypeName := getQualifiedTypeName(typeNames) // type name can qualified table_name.column in case of %TYPE
	if typeVar.PctType {                             // %TYPE declaration, so adding %TYPE for using it further
		return finalTypeName + "%TYPE"
	}
	return finalTypeName
}

/*
function ex -
CREATE OR REPLACE FUNCTION public.process_combined_tbl(

	    p_id int,
	    p_c public.combined_tbl.c%TYPE,
	    p_bitt public.combined_tbl.bitt%TYPE,
		..

)
parseTree-
parameters:{function_parameter:{name:"p_id"  arg_type:{names:{string:{sval:"pg_catalog"}}  names:{string:{sval:"int4"}}  typemod:-1  location:66}
mode:FUNC_PARAM_DEFAULT}}  parameters:{function_parameter:{name:"p_c"  arg_type:{names:{string:{sval:"public"}}  names:{string:{sval:"combined_tbl"}}
names:{string:{sval:"c"}}  pct_type:true  typemod:-1  location:87}  mode:FUNC_PARAM_DEFAULT}}  parameters:{function_parameter:{name:"p_bitt"
arg_type:{names:{string:{sval:"public"}}  names:{string:{sval:"combined_tbl"}}  names:{string:{sval:"bitt"}}  pct_type:true  typemod:-1
location:136}  mode:FUNC_PARAM_DEFAULT}}
*/
func GetFuncParametersTypeNames(parseTree *pg_query.ParseResult) []string {
	funcNode, _ := getCreateFuncStmtNode(parseTree)
	parameters := funcNode.CreateFunctionStmt.GetParameters()
	var paramTypeNames []string
	for _, param := range parameters {
		funcParam, ok := param.Node.(*pg_query.Node_FunctionParameter)
		if ok {
			paramType := funcParam.FunctionParameter.ArgType
			paramTypeNames = append(paramTypeNames, convertParserTypeNameToString(paramType))
		}
	}
	return paramTypeNames
}

func IsDDL(parseTree *pg_query.ParseResult) (bool, error) {
	ddlParser, err := GetDDLProcessor(parseTree)
	if err != nil {
		return false, fmt.Errorf("error getting a ddl parser: %w", err)
	}
	_, ok := ddlParser.(*NoOpProcessor)
	//Considering all the DDLs we have a Processor for as of now.
	//Not Full-proof as we don't have all DDL types but atleast we will skip all the types we know currently
	return !ok, nil
}

/*
this function checks whether the current node handles the jsonb data or not by evaluating all different type of nodes -
column ref - column of jsonb type
type cast - constant data with type casting to jsonb type
func call - function call returning the jsonb data
Expression - if any of left and right operands are of node type handling jsonb data
*/
func DoesNodeHandleJsonbData(node *pg_query.Node, jsonbColumns []string, jsonbFunctions []string) bool {
	switch {
	case node.GetColumnRef() != nil:
		/*
			SELECT numbers[1] AS first_number
			FROM array_data;
			{a_indirection:{arg:{column_ref:{fields:{string:{sval:"numbers"}}  location:69}}
			indirection:{a_indices:{uidx:{a_const:{ival:{ival:1}  location:77}}}}}}  location:69}}
		*/
		_, col := GetColNameFromColumnRef(node.GetColumnRef().ProtoReflect())
		if slices.Contains(jsonbColumns, col) {
			return true
		}

	case node.GetTypeCast() != nil:
		/*
			SELECT ('{"a": {"b": {"c": 1}}}'::jsonb)['a']['b']['c'];
			{a_indirection:{arg:{type_cast:{arg:{a_const:{sval:{sval:"{\"a\": {\"b\": {\"c\": 1}}}"}  location:280}}
			type_name:{names:{string:{sval:"jsonb"}}  typemod:-1  location:306}  location:304}}
		*/
		typeCast := node.GetTypeCast()
		_, typeName := getSchemaAndObjectName(typeCast.GetTypeName().GetNames())
		if typeName == "jsonb" {
			return true
		}
	case node.GetFuncCall() != nil:
		/*
			SELECT (jsonb_build_object('name', 'PostgreSQL', 'version', 14, 'open_source', TRUE))['name'] AS json_obj;
			val:{a_indirection:{arg:{func_call:{funcname:{string:{sval:"jsonb_build_object"}}  args:{a_const:{sval:{sval:"name"}
			location:194}}  args:{a_const:{sval:{sval:"PostgreSQL"}  location:202}}  args:{a_const:{sval:{sval:"version"}  location:216}}
			args:{a_const:{ival:{ival:14}  location:227}}
		*/
		funcCall := node.GetFuncCall()
		_, funcName := getSchemaAndObjectName(funcCall.Funcname)
		if slices.Contains(jsonbFunctions, funcName) {
			return true
		}
	case node.GetAExpr() != nil:
		/*
			SELECT ('{"key": "value1"}'::jsonb || '{"key1": "value2"}'::jsonb)['key'] AS object_in_array;
			val:{a_indirection:{arg:{a_expr:{kind:AEXPR_OP  name:{string:{sval:"||"}}  lexpr:{type_cast:{arg:{a_const:{sval:{sval:"{\"key\": \"value1\"}"}
			location:81}}  type_name:{names:{string:{sval:"jsonb"}}  typemod:-1  location:102}  location:100}}  rexpr:{type_cast:{arg:{a_const:{sval:{sval:"{\"key1\": \"value2\"}"}
			location:111}}  type_name:{names:{string:{sval:"jsonb"}}  typemod:-1  location:132}  location:130}}  location:108}}  indirection:{a_indices:{uidx:{a_const:{sval:{sval:"key"}
			location:139}}}}}}

			SELECT (data || '{"new_key": "new_value"}' )['name'] FROM test_jsonb;
			{val:{a_indirection:{arg:{a_expr:{kind:AEXPR_OP  name:{string:{sval:"||"}}  lexpr:{column_ref:{fields:{string:{sval:"data"}}  location:10}}  rexpr:{a_const:{sval:{sval:"{\"new_key\": \"new_value\"}"}
			location:18}}  location:15}}  indirection:{a_indices:{uidx:{a_const:{sval:{sval:"name"}

			SELECT (jsonb_build_object('name', 'PostgreSQL', 'version', 14, 'open_source', TRUE) || '{"key": "value2"}')['name'] AS json_obj;
			{val:{a_indirection:{arg:{a_expr:{kind:AEXPR_OP  name:{string:{sval:"||"}}  lexpr:{column_ref:{fields:{string:{sval:"data"}}  location:10}}  rexpr:{a_const:{sval:{sval:"{\"new_key\": \"new_value\"}"}
			location:18}}  location:15}}  indirection:{a_indices:{uidx:{a_const:{sval:{sval:"name"}  location:47}}}}}}  location:9}}
		*/
		expr := node.GetAExpr()
		lExpr := expr.GetLexpr()
		rExpr := expr.GetRexpr()
		if lExpr != nil && DoesNodeHandleJsonbData(lExpr, jsonbColumns, jsonbFunctions) {
			return true
		}
		if rExpr != nil && DoesNodeHandleJsonbData(rExpr, jsonbColumns, jsonbFunctions) {
			return true
		}
	}
	return false
}

func DeparseRawStmts(rawStmts []*pg_query.RawStmt) ([]string, error) {
	var deparsedStmts []string
	for _, rawStmt := range rawStmts {
		deparsedStmt, err := pg_query.Deparse(&pg_query.ParseResult{Stmts: []*pg_query.RawStmt{rawStmt}})
		if err != nil {
			return nil, fmt.Errorf("error deparsing statement: %w", err)
		}
		deparsedStmts = append(deparsedStmts, deparsedStmt+";") // adding semicolon to make it a valid SQL statement
	}

	return deparsedStmts, nil
}

func DeparseParseTree(parseTree *pg_query.ParseResult) (string, error) {
	if parseTree == nil || len(parseTree.Stmts) == 0 {
		return "", fmt.Errorf("parse tree is empty or invalid")
	}

	deparsedStmt, err := pg_query.Deparse(parseTree)
	if err != nil {
		return "", fmt.Errorf("error deparsing parse tree: %w", err)
	}

	return deparsedStmt, nil
}

func getAConstValue(node *pg_query.Node) string {

	if node == nil {
		return ""
	}
	if node.GetTypeCast() != nil {
		return getAConstValue(node.GetTypeCast().GetArg())
	}
	if node.GetAConst() == nil {
		return ""
	}

	aConst := node.GetAConst()
	if aConst.GetVal() == nil {
		//if it doesn't have val
		return ""
	}

	switch {
	case aConst.GetSval() != nil:
		return aConst.GetSval().Sval
	case aConst.GetIval() != nil:
		return strconv.Itoa(int(aConst.GetIval().Ival))
	case aConst.GetFval() != nil:
		return aConst.GetFval().Fval
	case aConst.GetBsval() != nil:
		return aConst.GetBsval().Bsval
	case aConst.GetBoolval() != nil:
		return aConst.GetBoolval().String()
	}
	return ""
}

func TraverseAndFindColumnName(node *pg_query.Node) string {
	if node.GetColumnRef() != nil {
		_, colName := GetColNameFromColumnRef(node.GetColumnRef().ProtoReflect())
		return colName
	}
	switch {
	case node.GetTypeCast() != nil:
		/*
			WHERE ((status)::text <> 'active'::text)
			- where_clause:{a_expr:{kind:AEXPR_OP name:{string:{sval:"<>"}} lexpr:{type_cast:{arg:{column_ref:{fields:{string:{sval:"status"}} location:167}}
			  type_name:{names:{string:{sval:"text"}} typemod:-1 location:176} location:174}} rexpr:{type_cast:{arg:{a_const:{sval:{sval:"active"}
		*/
		return TraverseAndFindColumnName(node.GetTypeCast().Arg)
		//add more cases if possible for columnRef TODO:
	}

	return ""
}
