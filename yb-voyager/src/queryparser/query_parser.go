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

func ParseDDL(query string) (DDLObject, error) {
	parseTree, err := Parse(query)
	if err != nil {
		return nil, fmt.Errorf("parsing query failed: %v", err)
	}

	parser, err := GetDDLParser(parseTree)
	if err != nil {
		return nil, fmt.Errorf("getting parser failed: %v", err)
	}

	ddlObject, err := parser.Parse(parseTree)
	if err != nil {
		return nil, fmt.Errorf("parsing DDL failed: %v", err)
	}

	return ddlObject, nil
}

func DeparseSelectStmt(selectStmt *pg_query.SelectStmt) (string, error) {
	if selectStmt != nil {
		parseResult := &pg_query.ParseResult{
			Stmts: []*pg_query.RawStmt{
				{
					Stmt: &pg_query.Node{
						Node: &pg_query.Node_SelectStmt{SelectStmt: selectStmt},
					},
				},
			},
		}

		// Deparse the SelectStmt to get the string representation
		selectSQL, err := pg_query.Deparse(parseResult)
		return selectSQL, err
	}
	return "", nil
}

func GetProtoMessageFromParseTree(parseTree *pg_query.ParseResult) protoreflect.Message {
	return parseTree.Stmts[0].Stmt.ProtoReflect()
}

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

func GetSelectStmtQueryFromViewOrMView(parseTree *pg_query.ParseResult) (string, error) {
	viewNode, isViewStmt := getCreateViewNode(parseTree)
	createAsNode, _ := getCreateTableAsStmtNode(parseTree) //For MVIEW case
	var selectStmt *pg_query.SelectStmt
	if isViewStmt {
		selectStmt = viewNode.ViewStmt.GetQuery().GetSelectStmt()
	} else {
		selectStmt = createAsNode.CreateTableAsStmt.GetQuery().GetSelectStmt()
	}
	selectStmtQuery, err := DeparseSelectStmt(selectStmt)
	if err != nil {
		return "", fmt.Errorf("deparsing the select stmt: %v", err)
	}
	return selectStmtQuery, nil
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
		return objectType, getFunctionObjectName(funcNameList)
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

func GetIndexAccessMethod(parseTree *pg_query.ParseResult) string {
	indexNode, _ := getCreateIndexStmtNode(parseTree)
	return indexNode.IndexStmt.AccessMethod
}

// Range Var is the struct to get the relation information like relation name, schema name, persisted relation or not, etc..
func getObjectNameFromRangeVar(obj *pg_query.RangeVar) string {
	schema := obj.Schemaname
	name := obj.Relname
	return lo.Ternary(schema != "", fmt.Sprintf("%s.%s", schema, name), name)
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

func getPolicyStmtNode(parseTree *pg_query.ParseResult) (*pg_query.Node_CreatePolicyStmt, bool) {
	node, ok := parseTree.Stmts[0].Stmt.Node.(*pg_query.Node_CreatePolicyStmt)
	return node, ok
}

func IsAlterTable(parseTree *pg_query.ParseResult) bool {
	_, isAlter := getAlterStmtNode(parseTree)
	return isAlter
}

func IsCreateIndex(parseTree *pg_query.ParseResult) bool {
	_, isCreateIndex := getCreateIndexStmtNode(parseTree)
	return isCreateIndex
}

func IsCreateTable(parseTree *pg_query.ParseResult) bool {
	_, IsCreateTable := getCreateTableStmtNode(parseTree)
	return IsCreateTable
}

func IsCreatePolicy(parseTree *pg_query.ParseResult) bool {
	_, isCreatePolicy := getPolicyStmtNode(parseTree)
	return isCreatePolicy
}

// func GetGeneratedColumns(parseTree *pg_query.ParseResult) []string {
// 	createTableNode, _ := getCreateTableStmtNode(parseTree)
// 	columns := createTableNode.CreateStmt.TableElts
// 	var generatedColumns []string
// 	for _, column := range columns {
// 		//In case CREATE DDL has PRIMARY KEY(column_name) - it will be included in columns but won't have columnDef as its a constraint
// 		if column.GetColumnDef() != nil {
// 			constraints := column.GetColumnDef().Constraints
// 			for _, constraint := range constraints {
// 				if constraint.GetConstraint().Contype == pg_query.ConstrType_CONSTR_GENERATED {
// 					generatedColumns = append(generatedColumns, column.GetColumnDef().Colname)
// 				}
// 			}
// 		}
// 	}
// 	return generatedColumns
// }

// func IsUnloggedTable(parseTree *pg_query.ParseResult) bool {
// 	createTableNode, _ := getCreateTableStmtNode(parseTree)
// 	/*
// 		e.g CREATE UNLOGGED TABLE tbl_unlogged (id int, val text);
// 		stmt:{create_stmt:{relation:{schemaname:"public" relname:"tbl_unlogged" inh:true relpersistence:"u" location:19}
// 		table_elts:{column_def:{colname:"id" type_name:{names:{string:{sval:"pg_catalog"}} names:{string:{sval:"int4"}}
// 		typemod:-1 location:54} is_local:true location:51}} table_elts:{column_def:{colname:"val" type_name:{names:{string:{sval:"text"}}
// 		typemod:-1 location:93} is_local:true location:89}} oncommit:ONCOMMIT_NOOP}} stmt_len:99
// 		here, relpersistence is the information about the persistence of this table where u-> unlogged, p->persistent, t->temporary tables
// 	*/
// 	return createTableNode.CreateStmt.Relation.GetRelpersistence() == "u"
// }

// func HaveStorageOptions(parseTree *pg_query.ParseResult) bool {
// 	createIndexNode, isCreateIndex := getCreateIndexStmtNode(parseTree)
// 	alterTableNode, isAlterTable := getAlterStmtNode(parseTree)
// 	var options []*pg_query.Node
// 	switch true {
// 	case isCreateIndex:
// 		/*
// 			e.g. CREATE INDEX idx on table_name(id) with (fillfactor='70');
// 			index_stmt:{idxname:"idx" relation:{relname:"table_name" inh:true relpersistence:"p" location:21} access_method:"btree"
// 			index_params:{index_elem:{name:"id" ordering:SORTBY_DEFAULT nulls_ordering:SORTBY_NULLS_DEFAULT}}
// 			options:{def_elem:{defname:"fillfactor" arg:{string:{sval:"70"}} ...
// 			here again similar to ALTER table Storage parameters options is the high level field in for WITH options.
// 		*/
// 		options = createIndexNode.IndexStmt.GetOptions()
// 	case isAlterTable:
// 		/*
// 			e.g. alter table test add constraint uk unique(id) with (fillfactor='70');
// 			alter_table_cmd:{subtype:AT_AddConstraint def:{constraint:{contype:CONSTR_UNIQUE conname:"asd" location:292
// 			keys:{string:{sval:"id"}} options:{def_elem:{defname:"fillfactor" arg:{string:{sval:"70"}}...
// 			Similarly here we are trying to get the constraint if any and then get the options field which is WITH options
// 			in this case only so checking that for this case.
// 		*/
// 		options = alterTableNode.AlterTableStmt.Cmds[0].GetAlterTableCmd().GetDef().GetConstraint().GetOptions()
// 	}

// 	return len(options) > 0
// }

// func HaveSetAttributes(parseTree *pg_query.ParseResult) bool {
// 	alterTableNode, isAlterTable := getAlterStmtNode(parseTree)
// 	var list []*pg_query.Node
// 	switch true {

// 	case isAlterTable:
// 		// this will the list of items in the SET (attribute=value, ..)
// 		/*
// 			e.g. alter table test_1 alter column col1 set (attribute_option=value);
// 			cmds:{alter_table_cmd:{subtype:AT_SetOptions name:"col1" def:{list:{items:{def_elem:{defname:"attribute_option"
// 			arg:{type_name:{names:{string:{sval:"value"}} typemod:-1 location:263}} defaction:DEFELEM_UNSPEC location:246}}}}...
// 			for set attribute issue we will the type of alter setting the options and in the 'def' definition field which has the
// 			information of the type, we will check if there is any list which will only present in case there is syntax like <SubTYPE> (...)
// 		*/
// 		list = alterTableNode.AlterTableStmt.Cmds[0].GetAlterTableCmd().GetDef().GetList().GetItems()
// 	}

// 	return len(list) > 0
// }

// func GetAlterTableType(parseTree *pg_query.ParseResult) pg_query.AlterTableType {
// 	alterTableNode, _ := getAlterStmtNode(parseTree)
// 	return alterTableNode.AlterTableStmt.Cmds[0].GetAlterTableCmd().GetSubtype()
// }

// func GetRuleNameInAlterTable(parseTree *pg_query.ParseResult) string {
// 	alterTableNode, _ := getAlterStmtNode(parseTree)
// 	/*
// 		e.g. ALTER TABLE example DISABLE example_rule;
// 		cmds:{alter_table_cmd:{subtype:AT_DisableRule name:"example_rule" behavior:DROP_RESTRICT}} objtype:OBJECT_TABLE}}
// 		checking the subType is sufficient in this case
// 	*/
// 	return alterTableNode.AlterTableStmt.Cmds[0].GetAlterTableCmd().GetName()

// }
