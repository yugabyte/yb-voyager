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

/*
This package contains all the logic related to parsing a query string, and extracting details out of it.
We mainly use the pg_query_go library to help with this.

The main functions in this package are:
1. Use pg_query_go to parse the query string into a ParseResult (i.e. a parseTree)
2. Traverse and process each protobufMessage node of the ParseTree.
3. For PLPGSQL, convert the PLPGSQL to JSON; get all the statements out of the PLPGSQL block.
we can put all the parser related logic (the parsing, the parsing of plpgsql to json, the traversal through the proto messages, the traversal through the nested plpgsql json, adding clauses to statements, etc
*/
package queryparser

import (
	"fmt"
	"os"

	pg_query "github.com/pganalyze/pg_query_go/v6"
	log "github.com/sirupsen/logrus"
)

func Parse(query string) (*pg_query.ParseResult, error) {
	log.Debugf("parsing the query [%s]", query)
	tree, err := pg_query.Parse(query)
	if err != nil {
		return nil, err
	}
	log.Debugf("query: %s\n", query)
	log.Debugf("parse tree: %v\n", tree)
	return tree, nil
}

func ParsePLPGSQLToJson(query string) (string, error) {
	log.Debugf("parsing the PLPGSQL to json query [%s]", query)
	jsonString, err := pg_query.ParsePlPgSqlToJSON(query)
	if err != nil {
		return "", err
	}
	return jsonString, err
}

func ParseSqlFile(filePath string) (*pg_query.ParseResult, error) {
	log.Debugf("parsing the file %q", filePath)
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file failed: %v", err)
	}

	tree, err := pg_query.Parse(string(bytes))
	if err != nil {
		return nil, err
	}
	log.Debugf("sql file contents: %s\n", string(bytes))
	log.Debugf("parse tree: %v\n", tree)
	return tree, nil
}

func ProcessDDL(parseTree *pg_query.ParseResult) (DDLObject, error) {
	processor, err := GetDDLProcessor(parseTree)
	if err != nil {
		return nil, fmt.Errorf("getting processor failed: %v", err)
	}

	ddlObject, err := processor.Process(parseTree)
	if err != nil {
		return nil, fmt.Errorf("parsing DDL failed: %v", err)
	}

	return ddlObject, nil
}
