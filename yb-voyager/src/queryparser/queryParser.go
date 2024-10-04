package queryparser

import (
	pg_query "github.com/pganalyze/pg_query_go/v5"
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

func (qp *QueryParser) CheckUnsupportedQueryConstruct() (string, error) {
	if qp.containsAdvisoryLocks() {
		return ADVISORY_LOCKS, nil
	}
	// TODO: Add checks for unsupported constructs - system columns, XML functions
	return "", nil
}
