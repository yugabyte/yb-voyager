package queryparser

import (
	"encoding/json"
	"fmt"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v5"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const (
	PLPGSQL_EXPR            = "PLpgSQL_expr"
	ACTION                  = "action"
	QUERY                   = "query"
	PLPGSQL_FUNCTION        = "PLpgSQL_function"
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

func GetProtoMessageFromParseTree(parseTree *pg_query.ParseResult) protoreflect.Message {
	return parseTree.Stmts[0].Stmt.ProtoReflect()
}

func GetAllPLPGSQLStatements(query string, parsedJson string) ([]string, error) {
	if parsedJson == "" {
		return []string{}, nil
	}
	var parsedJsonMapList []map[string]interface{}
	log.Debugf("parsing the json string-%s of stmt-%s", parsedJson, query)
	err := json.Unmarshal([]byte(parsedJson), &parsedJsonMapList)
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
					q := strings.Trim(query.(string), "'")
					q = strings.TrimSpace(q)
					if !strings.HasSuffix(q, ";") {
						q += ";"
					}

					*plPgSqlStatements = append(*plPgSqlStatements, q)
				}
			}
		default:
			TraversePlPgSQLActions(v, plPgSqlStatements)
		}
	}
}
