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
