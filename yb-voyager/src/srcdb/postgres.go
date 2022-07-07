package srcdb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v4"
	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type PostgreSQL struct {
	source *Source

	db *pgx.Conn
}

func newPostgreSQL(s *Source) *PostgreSQL {
	return &PostgreSQL{source: s}
}

func (pg *PostgreSQL) Connect() error {
	db, err := pgx.Connect(context.Background(), pg.getConnectionUri())
	pg.db = db
	return err
}

func (pg *PostgreSQL) CheckRequiredToolsAreInstalled() {
	checkTools("pg_dump", "strings", "pg_restore")
}

func (pg *PostgreSQL) GetTableRowCount(tableName string) int64 {
	// new conn to avoid conn busy err as multiple parallel(and time-taking) queries possible
	conn, err := pgx.Connect(context.Background(), pg.getConnectionUri())
	if err != nil {
		utils.ErrExit("Failed to connect to the source database for table row count: %s", err)
	}
	defer conn.Close(context.Background())

	var rowCount int64
	query := fmt.Sprintf("select count(*) from %s", tableName)
	log.Infof("Querying row count of table %q", tableName)
	err = conn.QueryRow(context.Background(), query).Scan(&rowCount)
	if err != nil {
		utils.ErrExit("Failed to query %q for row count of %q: %s", query, tableName, err)
	}
	log.Infof("Table %q has %v rows.", tableName, rowCount)
	return rowCount
}

func (pg *PostgreSQL) GetTableApproxRowCount(tableProgressMetadata *utils.TableProgressMetadata) int64 {
	var approxRowCount sql.NullInt64 // handles case: value of the row is null, default for int64 is 0
	query := fmt.Sprintf("SELECT reltuples::bigint FROM pg_class "+
		"where oid = '%s'::regclass", tableProgressMetadata.FullTableName)
	// table partition names are under schema as a namespace

	log.Infof("Querying '%s' approx row count of table %q", query, tableProgressMetadata.TableName)
	if err != nil {
	log.Infof("Table %q has approx %v rows.", tableProgressMetadata.FullTableName, approxRowCount)
	return approxRowCount.Int64
}

func (pg *PostgreSQL) GetVersion() string {
	var version string
	query := "SELECT setting from pg_settings where name = 'server_version'"
	err := pg.db.QueryRow(context.Background(), query).Scan(&version)
	if err != nil {
		utils.ErrExit("run query %q on source: %s", query, err)
	}
	return version
}

func (pg *PostgreSQL) GetAllTableNames() []string {
	list := strings.Split(pg.source.Schema, "|")
	querySchemaList := "'" + strings.Join(list, "','") + "'"
	query := fmt.Sprintf(`SELECT table_schema, table_name
			  FROM information_schema.tables
			  WHERE table_type = 'BASE TABLE' AND
			        table_schema IN (%s);`, querySchemaList)

	rows, err := pg.db.Query(context.Background(), query)
	if err != nil {
		utils.ErrExit("error in querying(%q) source database for table names: %v\n", query, err)
	}
	defer rows.Close()

	var tableNames []string
	var tableName, tableSchema string

	for rows.Next() {
		err = rows.Scan(&tableSchema, &tableName)
		if err != nil {
			utils.ErrExit("error in scanning query rows for table names: %v\n", err)
		}
		if nameContainsCapitalLetter(tableName) {
			// Surround the table name with double quotes.
			tableName = fmt.Sprintf("\"%s\"", tableName)
		}
		tableNames = append(tableNames, fmt.Sprintf("%s.%s", tableSchema, tableName))
	}
	log.Infof("Query found %d tables in the source db: %v", len(tableNames), tableNames)
	return tableNames
}

func (pg *PostgreSQL) GetAllPartitionNames(tableName string) []string {
	panic("Not Implemented")
}

func (pg *PostgreSQL) getConnectionUri() string {
	source := pg.source
	if source.Uri != "" {
		return source.Uri
	}

	source.Uri = fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?%s", source.User, source.Password,
		source.Host, source.Port, source.DBName, generateSSLQueryStringIfNotExists(source))
	return source.Uri
}

func (pg *PostgreSQL) ExportSchema(exportDir string) {
	pgdumpExtractSchema(pg.source.Schema, pg.getConnectionUri(), exportDir)
}

func (pg *PostgreSQL) ExportData(ctx context.Context, exportDir string, tableList []string, quitChan chan bool, exportDataStart chan bool) {
	pgdumpExportDataOffline(ctx, pg.source, pg.getConnectionUri(), exportDir, tableList, quitChan, exportDataStart)
}

func (pg *PostgreSQL) ExportDataPostProcessing(exportDir string, tablesProgressMetadata map[string]*utils.TableProgressMetadata) {
	renameDataFiles(tablesProgressMetadata)
	exportedRowCount := getExportedRowCount(tablesProgressMetadata)
	dfd := datafile.Descriptor{
		FileFormat:    datafile.TEXT,
		TableRowCount: exportedRowCount,
		Delimiter:     "\t",
		HasHeader:     false,
		ExportDir:     exportDir,
	}
	dfd.Save()
}
