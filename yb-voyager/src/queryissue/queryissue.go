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
	"strings"

	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/issue"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/queryparser"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/ybversion"
)

type ParserIssueDetector struct {
	// TODO: Add fields here
	// e.g. store composite types, etc. for future processing.
}

func NewParserIssueDetector() *ParserIssueDetector {
	return &ParserIssueDetector{}
}

func (p *ParserIssueDetector) getIssuesNotFixedInTargetDbVersion(issues []issue.IssueInstance, targetDbVersion *ybversion.YBVersion) ([]issue.IssueInstance, error) {
	var filteredIssues []issue.IssueInstance
	for _, i := range issues {
		fixed, err := i.IsFixedIn(targetDbVersion)
		if err != nil {
			return nil, fmt.Errorf("checking if issue %v is supported: %w", i, err)
		}
		if !fixed {
			filteredIssues = append(filteredIssues, i)
		}
	}
	return filteredIssues, nil
}

func (p *ParserIssueDetector) GetAllIssues(query string, targetDbVersion *ybversion.YBVersion) ([]issue.IssueInstance, error) {
	issues, err := p.getAllIssues(query)
	if err != nil {
		return issues, err
	}

	return p.getIssuesNotFixedInTargetDbVersion(issues, targetDbVersion)
}

func (p *ParserIssueDetector) getAllIssues(query string) ([]issue.IssueInstance, error) {
	parseTree, err := queryparser.Parse(query)
	if err != nil {
		return nil, fmt.Errorf("error parsing query: %w", err)
	}

	if queryparser.IsPLPGSQLObject(parseTree) {
		objType, objName := queryparser.GetObjectTypeAndObjectName(parseTree)
		plpgsqlQueries, err := queryparser.GetAllPLPGSQLStatements(query)
		if err != nil {
			return nil, fmt.Errorf("error getting all the queries from query: %w", err)
		}
		var issues []issue.IssueInstance
		for _, plpgsqlQuery := range plpgsqlQueries {
			issuesInQuery, err := p.getAllIssues(plpgsqlQuery)
			if err != nil {
				//there can be plpgsql expr queries no parseable via parser e.g. "withdrawal > balance"
				log.Errorf("error getting issues in query-%s: %v", query, err)
				continue
			}
			issues = append(issues, issuesInQuery...)
		}

		percentTypeSyntaxIssues, err := p.GetPercentTypeSyntaxIssues(query)
		if err != nil {
			return nil, fmt.Errorf("error getting reference TYPE syntax issues: %v", err)
		}
		issues = append(issues, percentTypeSyntaxIssues...)

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
		issues, err := p.getAllIssues(selectStmtQuery)
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

	issues, err := p.getDMLIssues(query)
	if err != nil {
		return nil, fmt.Errorf("error getting DML issues: %w", err)
	}
	return issues, nil
}

func (p *ParserIssueDetector) GetPercentTypeSyntaxIssues(query string) ([]issue.IssueInstance, error) {
	parseTree, err := queryparser.Parse(query)
	if err != nil {
		return nil, fmt.Errorf("error parsing the query-%s: %v", query, err)
	}

	objType, objName := queryparser.GetObjectTypeAndObjectName(parseTree)
	typeNames, err := queryparser.GetAllTypeNamesInPlpgSQLStmt(query)
	if err != nil {
		return nil, fmt.Errorf("error getting type names in PLPGSQL: %v", err)
	}

	/*
		Caveats of GetAllTypeNamesInPlpgSQLStmt():
			1. Not returning typename for variables in function parameter from this function (in correct in json as UNKNOWN), for that using the GetTypeNamesFromFuncParameters()
			2. Not returning the return type from this function (not available in json), for that using the GetReturnTypeOfFunc()
	*/
	if queryparser.IsFunctionObject(parseTree) {
		typeNames = append(typeNames, queryparser.GetReturnTypeOfFunc(parseTree))
	}
	typeNames = append(typeNames, queryparser.GetFuncParametersTypeNames(parseTree)...)
	var issues []issue.IssueInstance
	for _, typeName := range typeNames {
		if strings.HasSuffix(typeName, "%TYPE") {
			issues = append(issues, issue.NewPercentTypeSyntaxIssue(objType, objName, typeName)) // TODO: confirm
		}
	}
	return issues, nil
}

func (p *ParserIssueDetector) GetDMLIssues(query string, targetDbVersion *ybversion.YBVersion) ([]issue.IssueInstance, error) {
	issues, err := p.getDMLIssues(query)
	if err != nil {
		return issues, err
	}

	return p.getIssuesNotFixedInTargetDbVersion(issues, targetDbVersion)
}

// TODO: in future when we will DDL issues detection here we need `GetDDLIssues`
func (p *ParserIssueDetector) getDMLIssues(query string) ([]issue.IssueInstance, error) {
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
