package srcdb

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-db-migration/yb_migrate/src/utils"
)

type MySQL struct {
	source *Source

	db *sql.DB
}

func newMySQL(s *Source) *MySQL {
	return &MySQL{source: s}
}

func (ms *MySQL) Connect() error {
	db, err := sql.Open("mysql", ms.getConnectionString())
	ms.db = db
	return err
}

func (ms *MySQL) CheckRequiredToolsAreInstalled() {
	checkTools("ora2pg")
}

func (ms *MySQL) GetTableRowCount(tableName string) int64 {
	var rowCount int64
	query := fmt.Sprintf("select count(*) from %s", tableName)

	log.Infof("Querying row count of table %s", tableName)
	err := ms.db.QueryRow(query).Scan(&rowCount)
	if err != nil {
		utils.ErrExit("Failed to query row count of %q: %s", tableName, err)
	}
	log.Infof("Table %q has %v rows.", tableName, rowCount)
	return rowCount
}

func (ms *MySQL) GetVersion() string {
	var version string
	query := "SELECT VERSION()"
	err := ms.db.QueryRow(query).Scan(&version)
	if err != nil {
		utils.ErrExit("run query %q on source: %s", query, err)
	}
	return version
}

func (ms *MySQL) GetAllTableNames() []string {
	var tableNames []string
	query := fmt.Sprintf("SELECT table_name FROM information_schema.tables "+
		"WHERE table_schema = '%s' && table_type = 'BASE TABLE'", ms.source.DBName)
	rows, err := ms.db.Query(query)
	if err != nil {
		utils.ErrExit("error in querying source database for table names: %v\n", err)
	}
	defer rows.Close()
	for rows.Next() {
		var tableName string
		err = rows.Scan(&tableName)
		if err != nil {
			utils.ErrExit("error in scanning query rows for table names: %v\n", err)
		}
		tableNames = append(tableNames, tableName)
	}
	return tableNames
}

func (ms *MySQL) GetAllPartitionNames(tableName string) []string {
	panic("Not Implemented")
}

func (ms *MySQL) getConnectionString() string {
	source := ms.source
	parseSSLString(source)
	var tlsString string
	switch source.SSLMode {
	case "disable":
		tlsString = "tls=false"
	case "prefer":
		tlsString = "tls=preferred"
	case "require":
		tlsString = "tls=skip-verify"
	case "verify-ca", "verify-full":
		tlsConf := createTLSConf(source)
		err := mysql.RegisterTLSConfig("custom", &tlsConf)
		if err != nil {
			utils.ErrExit("Failed to register TLS config: %s", err)
		}
		tlsString = "tls=custom"
	default:
		errMsg := "Incorrect SSL Mode Provided. Please enter a valid sslmode."
		panic(errMsg)
	}
	connStr := fmt.Sprintf("%s:%s@(%s:%d)/%s?%s", source.User, source.Password,
		source.Host, source.Port, source.DBName, tlsString)
	return connStr
}

func (ms *MySQL) ExportSchema(exportDir string) {
	ora2pgExtractSchema(ms.source, exportDir)
}

func (ms *MySQL) ExportData(ctx context.Context, exportDir string, tableList []string, quitChan chan bool, exportDataStart chan bool) {
	ora2pgExportDataOffline(ctx, ms.source, exportDir, tableList, quitChan, exportDataStart)
}
