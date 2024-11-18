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

	pg_query "github.com/pganalyze/pg_query_go/v5"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func Parse(query string) (*pg_query.ParseResult, error) {
	log.Debugf("parsing the query [%s]", query)
	tree, err := pg_query.Parse(query)
	if err != nil {
		return nil, err
	}
	return tree, nil
}

func ParsePLPGSQLToJson(query string) (string, error) {
	log.Debugf("parsing the PLPGSQL to json query-%s", query)
	jsonString, err := pg_query.ParsePlPgSqlToJSON(query)
	if err != nil {
		return "", err
	}
	return jsonString, err
}

func DeparseSelectStmt(selectStmt *pg_query.SelectStmt) (string, error) {
	if selectStmt != nil {
		deparseResult := &pg_query.ParseResult{
			Stmts: []*pg_query.RawStmt{
				{
					Stmt: &pg_query.Node{
						Node: &pg_query.Node_SelectStmt{SelectStmt: selectStmt},
					},
				},
			},
		}

		// Deparse the SelectStmt to get the string representation
		selectSQL, err := pg_query.Deparse(deparseResult)
		return selectSQL, err
	}
	return "", nil
}

func GetProtoMessageFromParseTree(parseTree *pg_query.ParseResult) protoreflect.Message {
	return parseTree.Stmts[0].Stmt.ProtoReflect()
}

func IsPLPGSQLObject(parseTree *pg_query.ParseResult) bool {
	// CREATE FUNCTION is same parser NODE for FUNCTION/PROCEDURE
	_, isPlPgSQLObject := parseTree.Stmts[0].Stmt.Node.(*pg_query.Node_CreateFunctionStmt)
	return isPlPgSQLObject
}

func GetObjectTypeAndObjectName(parseTree *pg_query.ParseResult) (string, string) {
	createFuncNode, isCreateFunc := parseTree.Stmts[0].Stmt.Node.(*pg_query.Node_CreateFunctionStmt)
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
		return objectType, getFunctionObjectName(funcNameList)
	}
	return "", ""
}

func getFunctionObjectName(funcNameList []*pg_query.Node) string {
	funcName := ""
	funcSchemaName := ""
	if len(funcNameList) > 0 {
		funcName = funcNameList[len(funcNameList)-1].GetString_().Sval // func name can be qualified / unqualifed or native / non-native proper func name will always be available at last index
	}
	if len(funcNameList) >= 2 { // Names list will have all the parts of qualified func name
		funcSchemaName = funcNameList[len(funcNameList)-2].GetString_().Sval // // func name can be qualified / unqualifed or native / non-native proper schema name will always be available at last 2nd index
	}
	return lo.Ternary(funcSchemaName != "", fmt.Sprintf("%s.%s", funcSchemaName, funcName), funcName)
}
