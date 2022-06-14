package srcdb

import (
	"context"
	"database/sql"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type Oracle struct {
	source *Source

	db *sql.DB
}

func newOracle(s *Source) *Oracle {
	return &Oracle{source: s}
}

func (ora *Oracle) Connect() error {
	db, err := sql.Open("godror", ora.getConnectionString())
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
		utils.ErrExit("Failed to query row count of %q: %s", tableName, err)
	}
	log.Infof("Table %q has %v rows.", tableName, rowCount)
	return rowCount
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

func (ora *Oracle) GetAllTableNames() []string {
	var tableNames []string
	/* below query will collect all tables under given schema except TEMPORARY tables,
	Index related tables(start with DR$) and materialized view */
	query := fmt.Sprintf(`SELECT table_name
		FROM all_tables
		WHERE owner = '%s' AND TEMPORARY = 'N' AND table_name NOT LIKE 'DR$%%' AND
		(owner, table_name) not in (
			SELECT owner, mview_name
			FROM all_mviews
			UNION ALL
			SELECT log_owner, log_table
			FROM all_mview_logs)
		ORDER BY table_name ASC`, ora.source.Schema)
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

		tableNames = append(tableNames, tableName)
	}

	log.Infof("Table Name List: %q", tableNames)

	return tableNames
}

func (ora *Oracle) GetAllPartitionNames(tableName string) []string {
	query := fmt.Sprintf("SELECT partition_name FROM all_tab_partitions "+
		"WHERE table_name = '%s' AND table_owner = '%s' ORDER BY partition_name ASC",
		tableName, ora.source.Schema)
	rows, err := ora.db.Query(query)
	if err != nil {
		utils.ErrExit("failed to list partitions of table %q: %v", tableName, err)
	}
	defer rows.Close()

	var partitionNames []string
	for rows.Next() {
		var partitionName string
		err = rows.Scan(&partitionName)
		if err != nil {
			utils.ErrExit("error in scanning query rows: %v", err)
		}
		partitionNames = append(partitionNames, partitionName)
		// TODO: Support subpartition(find subparititions for each partition)
	}
	log.Infof("Partition Names for parent table %q: %q", tableName, partitionNames)
	return partitionNames
}

func (ora *Oracle) getConnectionString() string {
	source := ora.source
	var connStr string

	switch true {
	case source.DBSid != "":
		connStr = fmt.Sprintf("%s/%s@(DESCRIPTION=(ADDRESS=(PROTOCOL=TCP)(HOST=%s)(PORT=%d))(CONNECT_DATA=(SID=%s)))",
			source.User, source.Password, source.Host, source.Port, source.DBSid)

	case source.TNSAlias != "":
		connStr = fmt.Sprintf("%s/%s@%s", source.User, source.Password, source.TNSAlias)

	case source.DBName != "":
		connStr = fmt.Sprintf("%s/%s@(DESCRIPTION=(ADDRESS=(PROTOCOL=TCP)(HOST=%s)(PORT=%d))(CONNECT_DATA=(SERVICE_NAME=%s)))",
			source.User, source.Password, source.Host, source.Port, source.DBName)
	}

	return connStr
}

func (ora *Oracle) ExportSchema(exportDir string) {
	ora2pgExtractSchema(ora.source, exportDir)
}

func (ora *Oracle) ExportData(ctx context.Context, exportDir string, tableList []string, quitChan chan bool, exportDataStart chan bool) {
	ora2pgExportDataOffline(ctx, ora.source, exportDir, tableList, quitChan, exportDataStart)
}

func (ora *Oracle) ExportDataPostProcessing(exportDir string, tablesProgressMetadata map[string]*utils.TableProgressMetadata) {
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
