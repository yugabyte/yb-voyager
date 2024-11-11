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
