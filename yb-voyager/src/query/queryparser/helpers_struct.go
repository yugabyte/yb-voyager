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
	"github.com/samber/lo"
)

const (
	LIMIT_OPTION_WITH_TIES = pg_query.LimitOption_LIMIT_OPTION_WITH_TIES
)

func IsPLPGSQLObject(parseTree *pg_query.ParseResult) bool {
	// CREATE FUNCTION is same parser NODE for FUNCTION/PROCEDURE
	_, isPlPgSQLObject := getCreateFuncStmtNode(parseTree)
	return isPlPgSQLObject
}

func IsViewObject(parseTree *pg_query.ParseResult) bool {
	_, isViewStmt := parseTree.Stmts[0].Stmt.Node.(*pg_query.Node_ViewStmt)
	return isViewStmt
}

func IsMviewObject(parseTree *pg_query.ParseResult) bool {
	createAsNode, isCreateAsStmt := getCreateTableAsStmtNode(parseTree) //for MVIEW case
	return isCreateAsStmt && createAsNode.CreateTableAsStmt.Objtype == pg_query.ObjectType_OBJECT_MATVIEW
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
		funcSchemaName, funcName := getFunctionObjectName(funcNameList)
		return objectType, lo.Ternary(funcSchemaName != "", fmt.Sprintf("%s.%s", funcSchemaName, funcName), funcName)
	case isViewStmt:
		viewName := viewNode.ViewStmt.View
		return "VIEW", getObjectNameFromRangeVar(viewName)
	case IsMviewObject(parseTree):
		intoMview := createAsNode.CreateTableAsStmt.Into.Rel
		return "MVIEW", getObjectNameFromRangeVar(intoMview)
	case isCreateTable:
		return "TABLE", getObjectNameFromRangeVar(createTableNode.CreateStmt.Relation)
	case isAlterTable:
		return "TABLE", getObjectNameFromRangeVar(alterTableNode.AlterTableStmt.Relation)
	case isCreateIndex:
		indexName := createIndexNode.IndexStmt.Idxname
		schemaName := createIndexNode.IndexStmt.Relation.GetSchemaname()
		tableName := createIndexNode.IndexStmt.Relation.GetRelname()
		fullyQualifiedName := lo.Ternary(schemaName != "", schemaName+"."+tableName, tableName)
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
func getObjectNameFromRangeVar(obj *pg_query.RangeVar) string {
	schema := obj.Schemaname
	name := obj.Relname
	return lo.Ternary(schema != "", fmt.Sprintf("%s.%s", schema, name), name)
}

func getFunctionObjectName(funcNameList []*pg_query.Node) (string, string) {
	funcName := ""
	funcSchemaName := ""
	if len(funcNameList) > 0 {
		funcName = funcNameList[len(funcNameList)-1].GetString_().Sval // func name can be qualified / unqualifed or native / non-native proper func name will always be available at last index
	}
	if len(funcNameList) >= 2 { // Names list will have all the parts of qualified func name
		funcSchemaName = funcNameList[len(funcNameList)-2].GetString_().Sval // // func name can be qualified / unqualifed or native / non-native proper schema name will always be available at last 2nd index
	}
	return funcSchemaName, funcName
}

func getTypeNameAndSchema(typeNames []*pg_query.Node) (string, string) {
	typeName := ""
	typeSchemaName := ""
	if len(typeNames) > 0 {
		typeName = typeNames[len(typeNames)-1].GetString_().Sval // type name can be qualified / unqualifed or native / non-native proper type name will always be available at last index
	}
	if len(typeNames) >= 2 { // Names list will have all the parts of qualified type name
		typeSchemaName = typeNames[len(typeNames)-2].GetString_().Sval // // type name can be qualified / unqualifed or native / non-native proper schema name will always be available at last 2nd index
	}

	return typeName, typeSchemaName
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
