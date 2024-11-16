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
	pg_query "github.com/pganalyze/pg_query_go/v5"
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
