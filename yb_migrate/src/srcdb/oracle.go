package srcdb

import (
	"database/sql"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-db-migration/yb_migrate/src/utils"
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
