package queryparser

import (
	pg_query "github.com/pganalyze/pg_query_go/v5"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type QueryParser struct {
	QueryString string
	ParseTree   *pg_query.ParseResult
}

func New(query string) *QueryParser {
	return &QueryParser{
		QueryString: query,
	}
}

func (qp *QueryParser) Parse() error {
	tree, err := pg_query.Parse(qp.QueryString)
	if err != nil {
		return err
	}
	qp.ParseTree = tree
	return nil
}

func (qp *QueryParser) GetUnsupportedQueryConstructs() ([]utils.UnsupportedQueryConstruct, error) {
	var result []utils.UnsupportedQueryConstruct = nil
	if qp.containsAdvisoryLocks() {
		result = append(result, utils.UnsupportedQueryConstruct{
			ConstructType: ADVISORY_LOCKS,
			Query:         qp.QueryString,
		})
	}
	if qp.containsSystemColumns() {
		result = append(result, utils.UnsupportedQueryConstruct{
			ConstructType: SYSTEM_COLUMNS,
			Query:         qp.QueryString,
		})
	}
	if qp.containsXmlFunctions() {
		result = append(result, utils.UnsupportedQueryConstruct{
			ConstructType: XML_FUNCTIONS,
			Query:         qp.QueryString,
		})
	}

	return result, nil
}