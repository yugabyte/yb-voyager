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
	pg_query "github.com/pganalyze/pg_query_go/v6"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryparser"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/sqltransformer"
)

func (p *ParserIssueDetector) GenerateRecommendedSql(issue QueryIssue, parseTree *pg_query.ParseResult) (string, error) {
	generator, exists := sqlFixGenerators[issue.Type]
	if !exists {
		return "", nil
	}

	return generator(parseTree, issue)
}

type SqlFixGenerator func(parseTree *pg_query.ParseResult, issue QueryIssue) (string, error)

var sqlFixGenerators = map[string]SqlFixGenerator{
	// Add more issue types as generators are implemented
	NULL_VALUE_INDEXES:          generateNullPartialIndexFix,
	MOST_FREQUENT_VALUE_INDEXES: generateMostFrequentValuePartialIndexFix,
}

func generateNullPartialIndexFix(parseTree *pg_query.ParseResult, issue QueryIssue) (string, error) {
	transformer := sqltransformer.NewTransformer()
	fixedParseTree, err := transformer.AddPartialClauseForFilteringNULL(parseTree)
	if err != nil {
		return "", err
	}

	return queryparser.Deparse(fixedParseTree)
}

func generateMostFrequentValuePartialIndexFix(parseTree *pg_query.ParseResult, issue QueryIssue) (string, error) {
	// Use string constants for all values to ensure correct type conversion in PostgreSQL.
	value, _ := issue.Details[VALUE].(string)
	columnDataType, _ := issue.Details[COLUMN_TYPE].(string)
	transformer := sqltransformer.NewTransformer()
	fixedParseTree, err := transformer.AddPartialClauseForFilteringValue(parseTree, value, columnDataType)
	if err != nil {
		return "", err
	}

	return queryparser.Deparse(fixedParseTree)
}