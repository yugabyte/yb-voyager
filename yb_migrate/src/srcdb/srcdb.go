package srcdb

import "fmt"

type SourceDB interface {
	Connect() error
	GetTableRowCount(tableName string) int64
}

func newSourceDB(source *Source) SourceDB {
	switch source.DBType {
	case "postgresql":
		return newPostgreSQL(source)
	case "mysql":
		return newMySQL(source)
	case "oracle":
		return newOracle(source)
	default:
		panic(fmt.Sprintf("unknown source database type %q", source.DBType))
	}
}
