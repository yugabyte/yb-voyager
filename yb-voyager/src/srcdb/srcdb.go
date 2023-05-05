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
	GetTableRowCount(tableName string) int64
	GetTableApproxRowCount(tableProgressMetadata *utils.TableProgressMetadata) int64
	CheckRequiredToolsAreInstalled()
	GetVersion() string
	GetAllTableNames() []*sqlname.SourceName
	GetAllPartitionNames(tableName string) []string
	ExportSchema(exportDir string)
	ExportData(ctx context.Context, exportDir string, tableList []*sqlname.SourceName, quitChan chan bool, exportDataStart chan bool, exportSuccessChan chan bool)
	ExportDataPostProcessing(exportDir string, tablesProgressMetadata map[string]*utils.TableProgressMetadata)
	GetCharset() (string, error)
	FilterUnsupportedTables(tableList []*sqlname.SourceName) []*sqlname.SourceName
	FilterEmptyTables(tableList []*sqlname.SourceName) []*sqlname.SourceName
	PartiallySupportedTablesColumnList(tableList []*sqlname.SourceName) (map[string][]string, []string)
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
