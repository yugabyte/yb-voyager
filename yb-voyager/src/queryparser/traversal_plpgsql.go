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
	"encoding/json"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
)

const (
	PLPGSQL_EXPR            = "PLpgSQL_expr"
	ACTION                  = "action"
	QUERY                   = "query"
	PLPGSQL_FUNCTION        = "PLpgSQL_function"
)


func GetAllPLPGSQLStatements(query string) ([]string, error) {
	parsedJson, err := ParsePLPGSQLToJson(query)
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
	TraversePlPgSQLActions(actions, &plPgSqlStatements)
	return plPgSqlStatements, nil
}

func TraversePlPgSQLActions(action interface{}, plPgSqlStatements *[]string) {
	actionMap, ok := action.(map[string]interface{})
	if !ok {
		//In case the value of a field is not a <key , val> but a list of <key, val>
		lists, ok := action.([]interface{})
		if ok {
			for _, l := range lists {
				TraversePlPgSQLActions(l, plPgSqlStatements)
			}
		}
		return
	}

	for k, v := range actionMap {
		switch k {
		case PLPGSQL_EXPR:
			expr, ok := v.(map[string]interface{})
			if ok {
				query, ok := expr[QUERY]
				if ok {
					q := formatExprQuery(query.(string))

					*plPgSqlStatements = append(*plPgSqlStatements, q)
				}
			}
		default:
			TraversePlPgSQLActions(v, plPgSqlStatements)
		}
	}
}

// Function to format the PLPGSQL EXPR query from the json string
func formatExprQuery(q string) string {
	q = strings.Trim(q, "'")
	q = strings.TrimSpace(q)
	if !strings.HasSuffix(q, ";") {
		q += ";"
	}
	return q
}
