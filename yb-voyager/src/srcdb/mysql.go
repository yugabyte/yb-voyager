package srcdb

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

type MySQL struct {
	source *Source

	db *sql.DB
}

var mysqlUnsupportedDataTypes = []string{"TINYBLOB", "BLOB", "MEDIUMBLOB", "LONGBLOB"}

func newMySQL(s *Source) *MySQL {
	return &MySQL{source: s}
}

func (ms *MySQL) Connect() error {
	db, err := sql.Open("mysql", ms.getConnectionUri())
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
		utils.ErrExit("Failed to query %q for row count of %q: %s", query, tableName, err)
	}
	log.Infof("Table %q has %v rows.", tableName, rowCount)
	return rowCount
}

func (ms *MySQL) GetTableApproxRowCount(tableName *sqlname.SourceName) int64 {
	var approxRowCount sql.NullInt64 // handles case: value of the row is null, default for int64 is 0
	query := fmt.Sprintf("SELECT table_rows from information_schema.tables "+
		"where table_name = '%s' and table_schema = '%s'",
		tableName.ObjectName.Unquoted, tableName.SchemaName.Unquoted)

	log.Infof("Querying '%s' approx row count of table %q", query, tableName.String())
	err := ms.db.QueryRow(query).Scan(&approxRowCount)
	if err != nil {
		utils.ErrExit("Failed to query %q for approx row count of %q: %s", query, tableName.String(), err)
	}

	log.Infof("Table %q has approx %v rows.", tableName.String(), approxRowCount)
	return approxRowCount.Int64
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

func (ms *MySQL) GetAllTableNames() []*sqlname.SourceName {
	var tableNames []*sqlname.SourceName
	query := fmt.Sprintf("SELECT table_name FROM information_schema.tables "+
		"WHERE table_schema = '%s' && table_type = 'BASE TABLE'", ms.source.DBName)
	log.Infof(`query used to GetAllTableNames(): "%s"`, query)

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
		tableNames = append(tableNames, sqlname.NewSourceName(ms.source.DBName, tableName))
	}
	log.Infof("GetAllTableNames(): %s", tableNames)
	return tableNames
}

func (ms *MySQL) getConnectionUri() string {
	source := ms.source
	if source.Uri != "" {
		return source.Uri
	}

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

	source.Uri = fmt.Sprintf("%s:%s@(%s:%d)/%s?%s", source.User, source.Password,
		source.Host, source.Port, source.DBName, tlsString)
	return source.Uri
}

func (ms *MySQL) ExportSchema(exportDir string) {
	ora2pgExtractSchema(ms.source, exportDir)
}

func (ms *MySQL) ExportData(ctx context.Context, exportDir string, tableList []*sqlname.SourceName, quitChan chan bool, exportDataStart, exportSuccessChan chan bool, tablesColumnList map[*sqlname.SourceName][]string) {
	ora2pgExportDataOffline(ctx, ms.source, exportDir, tableList, tablesColumnList, quitChan, exportDataStart, exportSuccessChan)
}

func (ms *MySQL) ExportDataPostProcessing(exportDir string, tablesProgressMetadata map[string]*utils.TableProgressMetadata) {
	renameDataFilesForReservedWords(tablesProgressMetadata)
	exportedRowCount := getExportedRowCount(tablesProgressMetadata)
	dfd := datafile.Descriptor{
		FileFormat:    datafile.SQL,
		TableRowCount: exportedRowCount,
		Delimiter:     "\t",
		HasHeader:     false,
		ExportDir:     exportDir,
	}
	dfd.Save()
}

func (ms *MySQL) GetCharset() (string, error) {
	var charset string
	query := "SELECT @@character_set_database"
	err := ms.db.QueryRow(query).Scan(&charset)
	if err != nil {
		return "", fmt.Errorf("run query %q on source: %w", query, err)
	}
	return charset, nil
}

func (ms *MySQL) FilterUnsupportedTables(tableList []*sqlname.SourceName, useDebezium bool) ([]*sqlname.SourceName, []*sqlname.SourceName) {
	return tableList, nil
}

func (ms *MySQL) FilterEmptyTables(tableList []*sqlname.SourceName) ([]*sqlname.SourceName, []*sqlname.SourceName) {
	var nonEmptyTableList, emptyTableList []*sqlname.SourceName
	for _, tableName := range tableList {
		query := fmt.Sprintf(`SELECT 1 FROM %s LIMIT 1;`, tableName.Qualified.MinQuoted)
		if !IsTableEmpty(ms.db, query) {
			nonEmptyTableList = append(nonEmptyTableList, tableName)
		} else {
			emptyTableList = append(emptyTableList, tableName)
		}
	}
	return nonEmptyTableList, emptyTableList
}

func (ms *MySQL) GetTableColumns(tableName *sqlname.SourceName) ([]string, []string, []string) {
	var columns, dataTypes []string
	query := fmt.Sprintf("SELECT COLUMN_NAME, DATA_TYPE from INFORMATION_SCHEMA.COLUMNS where table_schema = '%s' and table_name='%s'", tableName.SchemaName.Unquoted, tableName.ObjectName.Unquoted)
	rows, err := ms.db.Query(query)
	if err != nil {
		utils.ErrExit("failed to query %q for finding table columns: %v", query, err)
	}
	for rows.Next() {
		var column, dataType string
		err := rows.Scan(&column, &dataType)
		if err != nil {
			utils.ErrExit("failed to scan column name from output of query %q: %v", query, err)
		}
		columns = append(columns, column)
		dataTypes = append(dataTypes, dataType)
	}
	return columns, dataTypes, nil
}

func (ms *MySQL) GetColumnsWithSupportedTypes(tableList []*sqlname.SourceName, useDebezium bool) (map[*sqlname.SourceName][]string, []string) {
	tableColumnMap := make(map[*sqlname.SourceName][]string)
	var unsupportedColumnNames []string
	for _, tableName := range tableList {
		columns, dataTypes, _ := ms.GetTableColumns(tableName)
		var supportedColumnNames []string
		for i := 0; i < len(columns); i++ {
			if utils.InsensitiveSliceContains(mysqlUnsupportedDataTypes, dataTypes[i]) {
				log.Infof("Skipping unsupproted column %s.%s of type %s", tableName.ObjectName.MinQuoted, columns[i], dataTypes[i])
				unsupportedColumnNames = append(unsupportedColumnNames, fmt.Sprintf("%s.%s of type %s", tableName.ObjectName.MinQuoted, columns[i], dataTypes[i]))
			} else {
				supportedColumnNames = append(supportedColumnNames, columns[i])
			}

		}
		if len(supportedColumnNames) == len(columns) {
			tableColumnMap[tableName] = []string{"*"}
		} else {
			tableColumnMap[tableName] = supportedColumnNames
		}
	}
	return tableColumnMap, unsupportedColumnNames
}

func (ms *MySQL) IsTablePartition(table *sqlname.SourceName) bool {
	panic("not implemented")
}
