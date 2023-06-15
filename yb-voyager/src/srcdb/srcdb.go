package srcdb

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

type SourceDB interface {
	Connect() error
	Disconnect()
	GetTableRowCount(tableName string) int64
	GetTableApproxRowCount(tableName *sqlname.SourceName) int64
	CheckRequiredToolsAreInstalled()
	GetVersion() string
	GetAllTableNames() []*sqlname.SourceName
	ExportSchema(exportDir string)
	ExportData(ctx context.Context, exportDir string, tableList []*sqlname.SourceName, quitChan chan bool, exportDataStart chan bool, exportSuccessChan chan bool, tablesColumnList map[*sqlname.SourceName][]string)
	ExportDataPostProcessing(exportDir string, tablesProgressMetadata map[string]*utils.TableProgressMetadata)
	GetCharset() (string, error)
	FilterUnsupportedTables(tableList []*sqlname.SourceName, useDebezium bool) ([]*sqlname.SourceName, []*sqlname.SourceName)
	FilterEmptyTables(tableList []*sqlname.SourceName) ([]*sqlname.SourceName, []*sqlname.SourceName)
	GetColumnsWithSupportedTypes(tableList []*sqlname.SourceName, useDebezium bool) (map[*sqlname.SourceName][]string, []string)
	GetTableColumns(tableName *sqlname.SourceName) ([]string, []string, []string)
	IsTablePartition(table *sqlname.SourceName) bool
	GetColumnToSequenceMap(tableList []*sqlname.SourceName) map[string]string
	GetAllSequences() []string
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

func IsTableEmpty(db *sql.DB, query string) bool {
	var rowsExist int
	err := db.QueryRow(query).Scan(&rowsExist)
	if err == sql.ErrNoRows {
		return true
	}
	if err != nil {
		utils.ErrExit("Failed to query %q for row count: %s", query, err)
	}
	return false
}
