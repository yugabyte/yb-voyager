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

import (
	"fmt"
	"slices"

	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/issue"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/queryparser"
)

type ParserIssueDetector struct {
	/*
		this will contain the information in this format:
		public.table1 -> {
			column1: citext | jsonb | inet | tsquery | tsvector | array
			...
		}
		schema2.table2 -> {
			column3: citext | jsonb | inet | tsquery | tsvector | array
			...
		}
		Here only those columns on tables are stored which have unsupported type for Index in YB
	*/
	columnsWithUnsupportedIndexDatatypes map[string]map[string]string
	/*
		list of composite types with fully qualified typename in the exported schema
	*/
	CompositeTypes []string
	/*
		list of enum types with fully qualified typename in the exported schema
	*/
	EnumTypes []string

	partitionTablesMap map[string]bool

	// key is partitioned table, value is sqlInfo (sqlstmt, fpath) where the ADD PRIMARY KEY statement resides
	primaryConsInAlter map[string]*queryparser.AlterTable
}

func NewParserIssueDetector() *ParserIssueDetector {
	return &ParserIssueDetector{
		columnsWithUnsupportedIndexDatatypes: make(map[string]map[string]string),
		CompositeTypes:                       make([]string, 0),
		EnumTypes:                            make([]string, 0),
		partitionTablesMap:                   make(map[string]bool),
		primaryConsInAlter:                   make(map[string]*queryparser.AlterTable),
	}
}

func (p *ParserIssueDetector) GetAllIssues(query string) ([]issue.IssueInstance, error) {
	plpgsqlIssues, err := p.GetAllPLPGSQLIssues(query)
	if err != nil {
		return nil, fmt.Errorf("error getting plpgsql issues: %v", err)
	}

	dmlIssues, err := p.GetDMLIssues(query)
	if err != nil {
		return nil, fmt.Errorf("error getting dml issues: %v", err)
	}
	ddlIssues, err := p.GetDDLIssues(query)
	if err != nil {
		return nil, fmt.Errorf("error getting ddl issues: %v", err)
	}
	return lo.Flatten([][]issue.IssueInstance{plpgsqlIssues, dmlIssues, ddlIssues}), nil
}

func (p *ParserIssueDetector) GetAllPLPGSQLIssues(query string) ([]issue.IssueInstance, error) {
	parseTree, err := queryparser.Parse(query)
	if err != nil {
		return nil, fmt.Errorf("error parsing query: %w", err)
	}
	//TODO handle this in DDLPARSER, DDLIssueDetector
	if queryparser.IsPLPGSQLObject(parseTree) {
		objType, objName := queryparser.GetObjectTypeAndObjectName(parseTree)
		plpgsqlQueries, err := queryparser.GetAllPLPGSQLStatements(query)
		if err != nil {
			return nil, fmt.Errorf("error getting all the queries from query: %w", err)
		}
		var issues []issue.IssueInstance
		for _, plpgsqlQuery := range plpgsqlQueries {
			issuesInQuery, err := p.GetAllIssues(plpgsqlQuery)
			if err != nil {
				//there can be plpgsql expr queries no parseable via parser e.g. "withdrawal > balance"
				log.Errorf("error getting issues in query-%s: %v", query, err)
				continue
			}
			issues = append(issues, issuesInQuery...)
		}
		return lo.Map(issues, func(i issue.IssueInstance, _ int) issue.IssueInstance {
			//Replacing the objectType and objectName to the original ObjectType and ObjectName of the PLPGSQL object
			//e.g. replacing the DML_QUERY and "" to FUNCTION and <func_name>
			i.ObjectType = objType
			i.ObjectName = objName
			return i
		}), nil
	}
	//Handle the Mview/View DDL's Select stmt issues
	if queryparser.IsViewObject(parseTree) || queryparser.IsMviewObject(parseTree) {
		objType, objName := queryparser.GetObjectTypeAndObjectName(parseTree)
		selectStmtQuery, err := queryparser.GetSelectStmtQueryFromViewOrMView(parseTree)
		if err != nil {
			return nil, fmt.Errorf("error deparsing a select stmt: %v", err)
		}
		issues, err := p.GetDMLIssues(selectStmtQuery)
		if err != nil {
			return nil, err
		}
		return lo.Map(issues, func(i issue.IssueInstance, _ int) issue.IssueInstance {
			//Replacing the objectType and objectName to the original ObjectType and ObjectName of the PLPGSQL object
			//e.g. replacing the DML_QUERY and "" to FUNCTION and <func_name>
			i.ObjectType = objType
			i.ObjectName = objName
			return i
		}), nil
	}

	return nil, nil
}

// parseTree, err := queryparser.Parse(query)
// if err != nil {
// 	return nil, fmt.Errorf("error parsing query: %w", err)
// }
// var issues []issue.IssueInstance
// if queryparser.IsCreateTable(parseTree) {
// 	objType, objName := queryparser.GetObjectTypeAndObjectName(parseTree)

// 	//GENERATED COLUMNS
// 	generatedColumns := queryparser.GetGeneratedColumns(parseTree)
// 	if len(generatedColumns) > 0 {
// 		issues = append(issues, issue.NewGeneratedColumnsIssue(objType, objName, query, generatedColumns))
// 	}

// 	if queryparser.IsUnloggedTable(parseTree) {
// 		issues = append(issues, issue.NewUnloggedTableIssue(objType, objName, query))
// 	}
// }

// if queryparser.IsAlterTable(parseTree) {
// 	objType, objName := queryparser.GetObjectTypeAndObjectName(parseTree)

// 	alterTableSubType := queryparser.GetAlterTableType(parseTree)
// 	switch alterTableSubType {
// 	case queryparser.SET_OPTIONS:
// 		if queryparser.HaveSetAttributes(parseTree) {
// 			issues = append(issues, issue.NewSetAttributeIssue(objType, objName, query))
// 		}
// 	case queryparser.ADD_CONSTRAINT:
// 		if queryparser.HaveStorageOptions(parseTree) {
// 			issues = append(issues, issue.NewStorageParameterIssue(objType, objName, query))
// 		}
// 	case queryparser.DISABLE_RULE:
// 		ruleName := queryparser.GetRuleNameInAlterTable(parseTree)
// 		issues = append(issues, issue.NewDisableRuleIssue(objType, objName, query, ruleName))
// 	case queryparser.CLUSTER_ON:
// 		/*
// 			e.g. ALTER TABLE example CLUSTER ON idx;
// 			stmt:{alter_table_stmt:{relation:{relname:"example" inh:true relpersistence:"p" location:13}
// 			cmds:{alter_table_cmd:{subtype:AT_ClusterOn name:"idx" behavior:DROP_RESTRICT}} objtype:OBJECT_TABLE}} stmt_len:32

// 		*/
// 		issues = append(issues, issue.NewClusterONIssue(objType, objName, query))

// 	}
// }

// if queryparser.IsCreateIndex(parseTree) {
// 	objType, objName := queryparser.GetObjectTypeAndObjectName(parseTree)

// 	indexAccessMethod := queryparser.GetIndexAccessMethod(parseTree)
// 	if slices.Contains(UnsupportedIndexMethods, indexAccessMethod) {
// 		issues = append(issues, issue.NewUnsupportedIndexMethodIssue(objType, objName, query, indexAccessMethod))
// 	}

// 	if queryparser.HaveStorageOptions(parseTree) {
// 		issues = append(issues, issue.NewStorageParameterIssue(objType, objName, query))
// 	}

// }

func (p *ParserIssueDetector) ParseRequiredDDLs(query string) error {
	ddlObj, err := queryparser.ParseDDL(query)
	if err != nil {
		return fmt.Errorf("error parsing DDL: %w", err)
	}

	switch ddlObj.(type) {
	case *queryparser.AlterTable:
		alter, _ := ddlObj.(*queryparser.AlterTable)
		if alter.ConstraintType == queryparser.PRIMARY_CONSTR_TYPE {
			//For the case ALTER and CREATE are not not is expected order where ALTER is before CREATE
			alter.Query = query
			p.primaryConsInAlter[alter.GetObjectName()] = alter
		}
	case *queryparser.Table:
		table, _ := ddlObj.(*queryparser.Table)
		if table.IsPartitioned {
			p.partitionTablesMap[table.GetObjectName()] = true
		}

		for _, col := range table.Columns {
			isUnsupportedType := slices.Contains(UnsupportedIndexDatatypes, col.TypeName)
			isUDTType := slices.Contains(p.CompositeTypes, col.GetFullTypeName())
			switch true {
			case col.IsArrayType:
				//For Array types and storing the type as "array" as of now we can enhance the to have specific type e.g. INT4ARRAY
				_, ok := p.columnsWithUnsupportedIndexDatatypes[table.GetObjectName()]
				if !ok {
					p.columnsWithUnsupportedIndexDatatypes[table.GetObjectName()] = make(map[string]string)
				}
				p.columnsWithUnsupportedIndexDatatypes[table.GetObjectName()][col.ColumnName] = "array"
			case isUnsupportedType || isUDTType:
				_, ok := p.columnsWithUnsupportedIndexDatatypes[table.GetObjectName()]
				if !ok {
					p.columnsWithUnsupportedIndexDatatypes[table.GetObjectName()] = make(map[string]string)
				}
				p.columnsWithUnsupportedIndexDatatypes[table.GetObjectName()][col.ColumnName] = col.TypeName
				if isUDTType { //For UDTs
					p.columnsWithUnsupportedIndexDatatypes[table.GetObjectName()][col.ColumnName] = "user_defined_type"
				}
			}
		}

	case *queryparser.CreateType:
		typeObj, _ := ddlObj.(*queryparser.CreateType)
		if typeObj.IsEnum {
			p.EnumTypes = append(p.EnumTypes, typeObj.GetObjectName())
		} else {
			p.CompositeTypes = append(p.CompositeTypes, typeObj.GetObjectName())
		}
	}
	return nil
}

func (p *ParserIssueDetector) GetDDLIssues(query string) ([]issue.IssueInstance, error) {
	// Parse the query into a DDL object
	ddlObj, err := queryparser.ParseDDL(query)
	if err != nil {
		return nil, fmt.Errorf("error parsing DDL: %w", err)
	}
	// Get the appropriate issue detector
	detector, err := p.GetDDLDetector(ddlObj)
	if err != nil {
		return nil, fmt.Errorf("error getting issue detector: %w", err)
	}

	// Detect issues
	issues, err := detector.DetectIssues(ddlObj)
	if err != nil {
		return nil, fmt.Errorf("error detecting issues: %w", err)
	}

	// Add the original query to each issue
	for i := range issues {
		if issues[i].SqlStatement == "" {
			issues[i].SqlStatement = query
		}
	}

	return issues, nil

}

func (p *ParserIssueDetector) GetDMLIssues(query string) ([]issue.IssueInstance, error) {
	parseTree, err := queryparser.Parse(query)
	if err != nil {
		return nil, fmt.Errorf("error parsing query: %w", err)
	}
	var result []issue.IssueInstance
	var unsupportedConstructs []string
	visited := make(map[protoreflect.Message]bool)
	detectors := []UnsupportedConstructDetector{
		NewFuncCallDetector(),
		NewColumnRefDetector(),
		NewXmlExprDetector(),
		NewRangeTableFuncDetector(),
	}

	processor := func(msg protoreflect.Message) error {
		for _, detector := range detectors {
			log.Debugf("running detector %T", detector)
			constructs, err := detector.Detect(msg)
			if err != nil {
				log.Debugf("error in detector %T: %v", detector, err)
				return fmt.Errorf("error in detectors %T: %w", detector, err)
			}
			unsupportedConstructs = lo.Union(unsupportedConstructs, constructs)
		}
		return nil
	}

	parseTreeProtoMsg := queryparser.GetProtoMessageFromParseTree(parseTree)
	err = queryparser.TraverseParseTree(parseTreeProtoMsg, visited, processor)
	if err != nil {
		return result, fmt.Errorf("error traversing parse tree message: %w", err)
	}

	for _, unsupportedConstruct := range unsupportedConstructs {
		switch unsupportedConstruct {
		case ADVISORY_LOCKS:
			result = append(result, issue.NewAdvisoryLocksIssue(issue.DML_QUERY_OBJECT_TYPE, "", query))
		case SYSTEM_COLUMNS:
			result = append(result, issue.NewSystemColumnsIssue(issue.DML_QUERY_OBJECT_TYPE, "", query))
		case XML_FUNCTIONS:
			result = append(result, issue.NewXmlFunctionsIssue(issue.DML_QUERY_OBJECT_TYPE, "", query))
		}
	}
	return result, nil
}
