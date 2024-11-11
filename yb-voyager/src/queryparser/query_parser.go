package queryparser

import (
	"encoding/json"
	"fmt"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v5"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

const (
	PG_QUERY_NODE_NODE      = "pg_query.Node"
	PG_QUERY_STRING_NODE    = "pg_query.String"
	PG_QUERY_ASTAR_NODE     = "pg_query.A_Star"
	PG_QUERY_XMLEXPR_NODE   = "pg_query.XmlExpr"
	PG_QUERY_FUNCCALL_NODE  = "pg_query.FuncCall"
	PG_QUERY_COLUMNREF_NODE = "pg_query.ColumnRef"
	PLPGSQL_EXPR            = "PLpgSQL_expr"
	ACTION                  = "action"
	QUERY                   = "query"
	PLPGSQL_FUNCTION        = "PLpgSQL_function"
)

type QueryParser struct {
	QueryString    string
	ParseTree      *pg_query.ParseResult
	ParseJson      string
	PlPgSQLQueries []string
}

func New(query string) *QueryParser {
	return &QueryParser{
		QueryString: query,
	}
}

func (qp *QueryParser) Parse() error {
	log.Debugf("parsing the query-%s", qp.QueryString)
	tree, err := pg_query.Parse(qp.QueryString)
	if err != nil {
		return err
	}
	qp.ParseTree = tree
	return nil
}

func (qp *QueryParser) ParsePLPGSQLToJson() error {
	log.Debugf("parsing the PLPGSQL to json query-%s", qp.QueryString)
	jsonString, err := pg_query.ParsePlPgSqlToJSON(qp.QueryString)
	if err != nil {
		return err
	}
	qp.ParseJson = jsonString
	return nil
}

func (qp *QueryParser) GetUnsupportedQueryConstructs() ([]utils.UnsupportedQueryConstruct, error) {
	if qp.ParseTree == nil {
		return nil, fmt.Errorf("query's parse tree is null")
	}

	var result []utils.UnsupportedQueryConstruct = nil
	visited := make(map[protoreflect.Message]bool)
	var unsupportedConstructs []string

	log.Debugf("Query: %s\n", qp.QueryString)
	log.Debugf("ParseTree: %+v\n", qp.ParseTree)
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

	parseTreeMsg := qp.ParseTree.Stmts[0].Stmt.ProtoReflect()
	err := TraverseParseTree(parseTreeMsg, visited, processor)
	if err != nil {
		return result, fmt.Errorf("error traversing parse tree message: %w", err)
	}

	/*
		TraverseParseTree() will detect unsupported construct for each node
		It is possible in the same query, the constructs is used multiple times and hence reported duplicates
	*/
	log.Debugf("detected unsupported constructs: %+v", unsupportedConstructs)
	for _, unsupportedConstruct := range unsupportedConstructs {
		result = append(result, utils.UnsupportedQueryConstruct{
			ConstructType: unsupportedConstruct,
			Query:         qp.QueryString,
			DocsLink:      getDocsLink(unsupportedConstruct),
		})
	}

	return result, nil
}

func getDocsLink(constructType string) string {
	switch constructType {
	case ADVISORY_LOCKS:
		return ADVISORY_LOCKS_DOC_LINK
	case SYSTEM_COLUMNS:
		return SYSTEM_COLUMNS_DOC_LINK
	case XML_FUNCTIONS:
		return XML_FUNCTIONS_DOC_LINK
	default:
		panic(fmt.Sprintf("construct type %q not supported", constructType))
	}
}

func (qe *QueryParser) GetAllPLPGSQLStatements() ([]string, error) {
	if qe.ParseJson == "" {
		return []string{}, nil
	}
	var parsedJsonMapList []map[string]interface{}
	log.Debugf("parsing the json string-%s of stmt-%s", qe.ParseJson, qe.QueryString)
	err := json.Unmarshal([]byte(qe.ParseJson), &parsedJsonMapList)
	if err != nil {
		return []string{}, fmt.Errorf("error parsing the json string of stmt-%s: %v", qe.QueryString, err)
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
