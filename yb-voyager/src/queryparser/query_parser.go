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

func GetProtoMessageFromParseTree(parseTree *pg_query.ParseResult) protoreflect.Message {
	return parseTree.Stmts[0].Stmt.ProtoReflect()
}

// type QueryParser struct {
// 	QueryString string
// 	ParseTree   *pg_query.ParseResult
// }

// func New(query string) *QueryParser {
// 	return &QueryParser{
// 		QueryString: query,
// 	}
// }

// func (qp *QueryParser) Parse() error {
// 	log.Debugf("parsing the query-%s", qp.QueryString)
// 	tree, err := pg_query.Parse(qp.QueryString)
// 	if err != nil {
// 		return err
// 	}
// 	qp.ParseTree = tree
// 	return nil
// }

// func (qp *QueryParser) GetUnsupportedQueryConstructs() ([]utils.UnsupportedQueryConstruct, error) {
// 	if qp.ParseTree == nil {
// 		return nil, fmt.Errorf("query's parse tree is null")
// 	}

// 	var result []utils.UnsupportedQueryConstruct = nil

// 	var unsupportedConstructs []string

// 	log.Debugf("Query: %s\n", qp.QueryString)
// 	log.Debugf("ParseTree: %+v\n", qp.ParseTree)
// 	detectors := []UnsupportedConstructDetector{
// 		NewFuncCallDetector(),
// 		NewColumnRefDetector(),
// 		NewXmlExprDetector(),
// 	}

// 	processor := func(msg protoreflect.Message) error {
// 		for _, detector := range detectors {
// 			log.Debugf("running detector %T", detector)
// 			constructs, err := detector.Detect(msg)
// 			if err != nil {
// 				log.Debugf("error in detector %T: %v", detector, err)
// 				return fmt.Errorf("error in detectors %T: %w", detector, err)
// 			}
// 			unsupportedConstructs = lo.Union(unsupportedConstructs, constructs)
// 		}
// 		return nil
// 	}

// 	parseTreeMsg := qp.ParseTree.Stmts[0].Stmt.ProtoReflect()
// 	err := TraverseParseTree(parseTreeMsg, visited, processor)
// 	if err != nil {
// 		return result, fmt.Errorf("error traversing parse tree message: %w", err)
// 	}

// 	/*
// 		TraverseParseTree() will detect unsupported construct for each node
// 		It is possible in the same query, the constructs is used multiple times and hence reported duplicates
// 	*/
// 	log.Debugf("detected unsupported constructs: %+v", unsupportedConstructs)
// 	for _, unsupportedConstruct := range unsupportedConstructs {
// 		result = append(result, utils.UnsupportedQueryConstruct{
// 			ConstructType: unsupportedConstruct,
// 			Query:         qp.QueryString,
// 			DocsLink:      getDocsLink(unsupportedConstruct),
// 		})
// 	}

// 	return result, nil
// }

// func getDocsLink(constructType string) string {
// 	switch constructType {
// 	case ADVISORY_LOCKS:
// 		return ADVISORY_LOCKS_DOC_LINK
// 	case SYSTEM_COLUMNS:
// 		return SYSTEM_COLUMNS_DOC_LINK
// 	case XML_FUNCTIONS:
// 		return XML_FUNCTIONS_DOC_LINK
// 	default:
// 		panic(fmt.Sprintf("construct type %q not supported", constructType))
// 	}
// }
