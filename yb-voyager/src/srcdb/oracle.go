package srcdb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	"golang.org/x/exp/slices"
)

type Oracle struct {
	source *Source

	db *sql.DB
}

var oracleUnsuportedDataTypes = []string{"BLOB", "BFILE", "URITYPE", "XMLTYPE",
"SYS.AnyData", "SYS.AnyType", "SYS.AnyDataSet", "ROWID", "UROWID","SDO_GEOMETRY","SDO_POINT_TYPE", "SDO_ELEM_INFO_ARRAY", "SDO_ORDINATE_ARRAY","SDO_GTYPE","SDO_SRID", "SDO_POINT", "SDO_ORDINATES", "SDO_DIM_ARRAY", "SDO_ORGSCL_TYPE","SDO_STRING_ARRAY", "JSON"}

func newOracle(s *Source) *Oracle {
	return &Oracle{source: s}
}

func (ora *Oracle) Connect() error {
	db, err := sql.Open("godror", ora.getConnectionUri())
	ora.db = db
	return err
}

func (ora *Oracle) CheckRequiredToolsAreInstalled() {
	checkTools("ora2pg", "sqlplus")
}

func (ora *Oracle) GetTableRowCount(tableName string) int64 {
	var rowCount int64
	query := fmt.Sprintf("select count(*) from %s", tableName)

	log.Infof("Querying row count of table %q", tableName)
	err := ora.db.QueryRow(query).Scan(&rowCount)
	if err != nil {
		utils.ErrExit("Failed to query %q for row count of %q: %s", query, tableName, err)
	}
	log.Infof("Table %q has %v rows.", tableName, rowCount)
	return rowCount
}

func (ora *Oracle) GetTableApproxRowCount(tableName *sqlname.SourceName) int64 {
	var approxRowCount sql.NullInt64 // handles case: value of the row is null, default for int64 is 0
	query := fmt.Sprintf("SELECT NUM_ROWS FROM ALL_TABLES "+
		"WHERE TABLE_NAME = '%s' and OWNER =  '%s'",
		tableName.ObjectName.Unquoted, tableName.SchemaName.Unquoted)

	log.Infof("Querying '%s' approx row count of table %q", query, tableName.String())
	err := ora.db.QueryRow(query).Scan(&approxRowCount)
	if err != nil {
		utils.ErrExit("Failed to query %q for approx row count of %q: %s", query, tableName.String(), err)
	}

	log.Infof("Table %q has approx %v rows.", tableName.String(), approxRowCount)
	return approxRowCount.Int64
}

func (ora *Oracle) GetVersion() string {
	var version string
	query := "SELECT BANNER FROM V$VERSION"
	// query sample output: Oracle Database 19c Enterprise Edition Release 19.0.0.0.0 - Production
	err := ora.db.QueryRow(query).Scan(&version)
	if err != nil {
		utils.ErrExit("run query %q on source: %s", query, err)
	}
	return version
}

func (ora *Oracle) GetAllTableNames() []*sqlname.SourceName {
	var tableNames []*sqlname.SourceName
	/* below query will collect all tables under given schema except TEMPORARY tables,
	Index related tables(start with DR$) and materialized view */
	query := fmt.Sprintf(`SELECT table_name
		FROM all_tables
		WHERE owner = '%s' AND TEMPORARY = 'N' AND table_name NOT LIKE 'DR$%%'
		AND table_name NOT LIKE 'AQ$%%' AND
		(owner, table_name) not in (
			SELECT owner, mview_name
			FROM all_mviews
			UNION ALL
			SELECT log_owner, log_table
			FROM all_mview_logs)
		ORDER BY table_name ASC`, ora.source.Schema)
	log.Infof(`query used to GetAllTableNames(): "%s"`, query)

	rows, err := ora.db.Query(query)
	if err != nil {
		utils.ErrExit("error in querying source database for table names: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var tableName string
		err = rows.Scan(&tableName)
		if err != nil {
			utils.ErrExit("error in scanning query rows for table names: %v", err)
		}
		tableName = fmt.Sprintf(`"%s"`, tableName)
		tableNames = append(tableNames, sqlname.NewSourceName(ora.source.Schema, tableName))
	}

	log.Infof("Table Name List: %q", tableNames)

	return tableNames
}

func (ora *Oracle) getConnectionUri() string {
	source := ora.source
	if source.Uri != "" {
		return source.Uri
	}

	switch true {
	case source.DBSid != "":
		source.Uri = fmt.Sprintf(`user="%s" password="%s" connectString="(DESCRIPTION=(ADDRESS=(PROTOCOL=TCP)(HOST=%s)(PORT=%d))(CONNECT_DATA=(SID=%s)))"`,
			source.User, source.Password, source.Host, source.Port, source.DBSid)

	case source.TNSAlias != "":
		source.Uri = fmt.Sprintf(`user="%s" password="%s" connectString="%s"`, source.User, source.Password, source.TNSAlias)

	case source.DBName != "":
		source.Uri = fmt.Sprintf(`user="%s" password="%s" connectString="(DESCRIPTION=(ADDRESS=(PROTOCOL=TCP)(HOST=%s)(PORT=%d))(CONNECT_DATA=(SERVICE_NAME=%s)))"`,
			source.User, source.Password, source.Host, source.Port, source.DBName)
	}

	return source.Uri
}

func (ora *Oracle) ExportSchema(exportDir string) {
	ora2pgExtractSchema(ora.source, exportDir)
}

func (ora *Oracle) ExportData(ctx context.Context, exportDir string, tableList []*sqlname.SourceName, quitChan chan bool, exportDataStart, exportSuccessChan chan bool, tablesColumnList map[string][]string) {
	ora2pgExportDataOffline(ctx, ora.source, exportDir, tableList, tablesColumnList, quitChan, exportDataStart, exportSuccessChan)
}

func (ora *Oracle) ExportDataPostProcessing(exportDir string, tablesProgressMetadata map[string]*utils.TableProgressMetadata) {
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

	identityColumns := getIdentityColumnSequences(exportDir)
	sourceTargetSequenceNames := make(map[string]string)
	for _, identityColumn := range identityColumns {
		targetSequenceName := ora.GetTargetIdentityColumnSequenceName(identityColumn)
		if targetSequenceName == "" {
			continue
		}
		sourceTargetSequenceNames[identityColumn] = targetSequenceName
	}

	replaceAllIdentityColumns(exportDir, sourceTargetSequenceNames)
}

func (ora *Oracle) GetCharset() (string, error) {
	var charset string
	query := "SELECT VALUE FROM NLS_DATABASE_PARAMETERS WHERE PARAMETER = 'NLS_CHARACTERSET'"
	err := ora.db.QueryRow(query).Scan(&charset)
	if err != nil {
		return "", fmt.Errorf("failed to query %q for database encoding: %s", query, err)
	}
	return charset, nil
}

func (ora *Oracle) FilterUnsupportedTables(tableList []*sqlname.SourceName) ([]*sqlname.SourceName, []*sqlname.SourceName) {
	var filteredTableList, unsupportedTableList []*sqlname.SourceName

	// query to find unsupported queue tables
	query := fmt.Sprintf("SELECT queue_table from ALL_QUEUE_TABLES WHERE OWNER = '%s'", ora.source.Schema)
	log.Infof("query for queue tables: %q\n", query)
	rows, err := ora.db.Query(query)
	if err != nil {
		utils.ErrExit("failed to query %q for filtering unsupported queue tables: %v", query, err)
	}
	for rows.Next() {
		var tableName string
		err := rows.Scan(&tableName)
		if err != nil {
			utils.ErrExit("failed to scan tableName from output of query %q: %v", query, err)
		}
		tableName = fmt.Sprintf(`"%s"`, tableName)
		tableSrcName := sqlname.NewSourceName(ora.source.Schema, tableName)
		if slices.Contains(tableList, tableSrcName) {
			unsupportedTableList = append(unsupportedTableList, tableSrcName)
		}
	}

	for _, table := range tableList {
		if !slices.Contains(unsupportedTableList, table) {
			filteredTableList = append(filteredTableList, table)
		}
	}

	return filteredTableList, unsupportedTableList
}

func (ora *Oracle) FilterEmptyTables(tableList []*sqlname.SourceName) ([]*sqlname.SourceName, []*sqlname.SourceName) {
	nonEmptyTableList := make([]*sqlname.SourceName, 0)
	skippedTableList := make([]*sqlname.SourceName, 0)
	for _, tableName := range tableList {
		query := fmt.Sprintf("SELECT 1 FROM %s WHERE ROWNUM=1", tableName.Qualified.MinQuoted)
		if ora.IsNestedTable(tableName) {
			// query to check empty nested oracle tables
			query = fmt.Sprintf(`SELECT 1 from dba_segments 
			where owner = '%s' AND segment_name = '%s' AND segment_type = 'NESTED TABLE'`,
				tableName.SchemaName.Unquoted, tableName.ObjectName.Unquoted)
		}

		if !IsTableEmpty(ora.db, query) {
			nonEmptyTableList = append(nonEmptyTableList, tableName)
		} else {
			skippedTableList = append(skippedTableList, tableName)
		}
	}
	return nonEmptyTableList, skippedTableList
}

func (ora *Oracle) IsNestedTable(tableName *sqlname.SourceName) bool {
	// sql query to find out if it is oracle nested table
	query := fmt.Sprintf("SELECT 1 FROM ALL_NESTED_TABLES WHERE OWNER = '%s' AND TABLE_NAME = '%s'",
		tableName.SchemaName.Unquoted, tableName.ObjectName.Unquoted)
	isNestedTable := 0
	err := ora.db.QueryRow(query).Scan(&isNestedTable)
	if err != nil && err != sql.ErrNoRows {
		utils.ErrExit("error in query to check if table %v is a nested table: %v", tableName, err)
	}
	return isNestedTable == 1
}

func (ora *Oracle) GetTargetIdentityColumnSequenceName(sequenceName string) string {
	var tableName, columnName string
	query := fmt.Sprintf("SELECT table_name, column_name FROM all_tab_identity_cols WHERE owner = '%s' AND sequence_name = '%s'", ora.source.Schema, strings.ToUpper(sequenceName))
	err := ora.db.QueryRow(query).Scan(&tableName, &columnName)

	if err == sql.ErrNoRows {
		return ""
	} else if err != nil {
		utils.ErrExit("failed to query %q for finding identity sequence table and column: %v", query, err)
	}

	return fmt.Sprintf("%s_%s_seq", tableName, columnName)
}

func (ora *Oracle) IsTablePartition(table *sqlname.SourceName) bool {
	panic("not implemented")
}

func (ora *Oracle) GetTableColumns(tableName *sqlname.SourceName) ([]string, []string, []string) {
	var columns, datatypes, datatypes_owner []string
	query := fmt.Sprintf("SELECT COLUMN_NAME, DATA_TYPE, DATA_TYPE_OWNER FROM ALL_TAB_COLUMNS WHERE OWNER = '%s' AND TABLE_NAME = '%s'", tableName.SchemaName.Unquoted, tableName.ObjectName.Unquoted)
	rows, err := ora.db.Query(query)
	if err != nil {
		utils.ErrExit("failed to query %q for finding table columns: %v", query, err)
	}
	for rows.Next() {
		var column, dataType, dataTypeOwner string
		err := rows.Scan(&column, &dataType, &dataTypeOwner)
		if err != nil {
			utils.ErrExit("failed to scan column name from output of query %q: %v", query, err)
		}
		columns = append(columns, column)
		datatypes = append(datatypes, dataType)
		datatypes_owner = append(datatypes_owner, dataTypeOwner)
	}
	return columns, datatypes, datatypes_owner
}

func (ora *Oracle) PartiallySupportedTablesColumnList(tableList []*sqlname.SourceName, useDebezium bool) (map[string][]string, []string) {
	tableColumnMap := make(map[string][]string)
	var unsupportedColumnNames []string
	for _, tableName := range tableList {
		columns, datatypes, datatypes_owner := ora.GetTableColumns(tableName)
		var supportedColumnNames []string
		for i := 0; i < len(columns); i++ {
			unsupported := false
			for _, unsupportedDataType := range oracleUnsuportedDataTypes {
				isUdtWithDebezium := (datatypes_owner[i] == ora.source.Schema) && useDebezium; // datatype owner check is for UDT type detection as VARRAY/Nested tables are created using UDT
				if strings.EqualFold(datatypes[i], unsupportedDataType) || isUdtWithDebezium { 
					unsupported = true 
				}
			}

			if unsupported {
				log.Infof(fmt.Sprintf("Skipping column %s.%s of type %s as it is not supported", tableName.ObjectName.MinQuoted, columns[i], datatypes[i]))
				unsupportedColumnNames = append(unsupportedColumnNames, fmt.Sprintf("%s.%s of type %s", tableName.ObjectName.MinQuoted, columns[i], datatypes[i]))
			} else {
				supportedColumnNames = append(supportedColumnNames, columns[i])
			}

		}
		if len(supportedColumnNames) < len(columns) {
			tableColumnMap[tableName.ObjectName.Unquoted] = supportedColumnNames
		} else if len(supportedColumnNames) == len(columns) {
			var allColumns []string
			allColumns = append(allColumns, "*")
			tableColumnMap[tableName.ObjectName.Unquoted] = allColumns;
		}
	}
		
	return tableColumnMap, unsupportedColumnNames
}

