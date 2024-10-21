package queryparser

import (
	"fmt"

	pg_query "github.com/pganalyze/pg_query_go/v5"
	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const (
	PG_QUERY_NODE_NODE      = "pg_query.Node"
	PG_QUERY_STRING_NODE    = "pg_query.String"
	PG_QUERY_ASTAR_NODE     = "pg_query.A_Star"
	PG_QUERY_XMLEXPR_NODE   = "pg_query.XmlExpr"
	PG_QUERY_FUNCCALL_NODE  = "pg_query.FuncCall"
	PG_QUERY_COLUMNREF_NODE = "pg_query.ColumnRef"
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
	visited := make(map[protoreflect.Message]bool)
	unsupportedConstructs := make(map[string]bool)

	log.Debugf("Query: %s\n", qp.QueryString)
	log.Debugf("Tree: %+v\n", qp.ParseTree)
	detectors := []UnsupportedConstructDetector{
		NewFuncCallDetector(),
		NewColumnRefDetector(),
		NewXmlExprDetector(),
	}

	compositeDetector := &CompositeDetector{detectors: detectors}
	processor := func(msg protoreflect.Message) error {
		constructs, err := compositeDetector.Detect(msg)
		if err != nil {
			return err
		}
		for _, c := range constructs {
			unsupportedConstructs[c] = true
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
	for unsupportedConstruct, ok := range unsupportedConstructs {
		if !ok {
			continue
		}
		result = append(result, utils.UnsupportedQueryConstruct{
			ConstructType: unsupportedConstruct,
			Query:         qp.QueryString,
		})
	}

	return result, nil
}
