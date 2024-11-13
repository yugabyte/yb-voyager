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
	"encoding/json"
	"fmt"

	pg_query "github.com/pganalyze/pg_query_go/v5"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/issue"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/queryparser"
)

const (
	ACTION                  = "action"
	PLPGSQL_FUNCTION        = "PLpgSQL_function"
)

type ParserIssueDetector struct {
	// TODO: Add fields here
	// e.g. store composite types, etc. for future processing.
}

func NewParserIssueDetector() *ParserIssueDetector {
	return &ParserIssueDetector{}
}

func (p *ParserIssueDetector) GetIssues(query string) ([]issue.IssueInstance, error) {
	parseTree, err := queryparser.Parse(query)
	if err != nil {
		return nil, fmt.Errorf("error parsing query: %w", err)
	}
	_, isPlPgSQLObject := parseTree.Stmts[0].Stmt.Node.(*pg_query.Node_CreateFunctionStmt) // CREATE FUNCTION is same parser NODE for FUNCTION/PROCEDURE
	if isPlPgSQLObject {
		plpgsqlQueries, err := p.GetAllPLPGSQLStatements(query)
		if err != nil {
			return nil, fmt.Errorf("error getting all the queries from query: %w", err)
		}
		var issues []issue.IssueInstance
		for _, plpgsqlQuery := range plpgsqlQueries {
			issuesInQuery, err := p.GetIssues(plpgsqlQuery)
			if err != nil {
				//there can be plpgsql expr queries no parseable via parser e.g. "withdrawal > balance" 
				log.Infof("error getting issues in query: %w", err)
				continue
			}
			issues = append(issues, issuesInQuery...)
		}
		return issues, nil
	}
	//TODO: add handling for VIEW/MVIEW to parse select 
	return p.getDMLIssues(query, parseTree)
}


func(p *ParserIssueDetector) GetAllPLPGSQLStatements(query string) ([]string, error) {
	parsedJson, err := queryparser.ParsePLPGSQLToJson(query)
	if err != nil {
		log.Infof("error in parsing the stmt-%s to json: %v", query, err)
		return []string{}, err
	}
	if parsedJson == "" {
		return []string{}, nil
	}
	var parsedJsonMapList []map[string]interface{}
	log.Debugf("parsing the json string-%s of stmt-%s", parsedJson, query)
	err = json.Unmarshal([]byte(parsedJson), &parsedJsonMapList)
	if err != nil {
		return []string{}, fmt.Errorf("error parsing the json string of stmt-%s: %v", query, err)
	}

	if len(parsedJsonMapList) == 0 {
		return []string{}, nil
	}

	parsedJsonMap := parsedJsonMapList[0]

	function := parsedJsonMap[PLPGSQL_FUNCTION]
	parsedFunctionMap, ok := function.(map[string]interface{})
	if !ok {
		return []string{}, nil
	}

	actions := parsedFunctionMap[ACTION]
	var plPgSqlStatements []string
	queryparser.TraversePlPgSQLActions(actions, &plPgSqlStatements)
	return plPgSqlStatements, nil
}

func (p *ParserIssueDetector) getDMLIssues(query string, parseTree *pg_query.ParseResult) ([]issue.IssueInstance, error) {
	var result []issue.IssueInstance
	var unsupportedConstructs []string
	visited := make(map[protoreflect.Message]bool)
	detectors := []UnsupportedConstructDetector{
		NewFuncCallDetector(),
		NewColumnRefDetector(),
		NewXmlExprDetector(),
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
	err := queryparser.TraverseParseTree(parseTreeProtoMsg, visited, processor)
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
