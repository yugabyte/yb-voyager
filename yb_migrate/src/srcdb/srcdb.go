package srcdb

import "fmt"

type SourceDB interface {
	Connect() error
}

func newSourceDB(source *Source) SourceDB {
	switch source.DBType {
	case "postgres":
		return newPostgreSQL(source)
	case "mysql":
		return newMySQL(source)
	case "oracle":
		return newOracle(source)
	default:
		panic(fmt.Sprintf("unknown source database type %q", source.DBType))
	}
}
