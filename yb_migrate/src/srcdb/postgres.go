package srcdb

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v4"
	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-db-migration/yb_migrate/src/utils"
)

type PostgreSQL struct {
	source *Source

	db *pgx.Conn
}

func newPostgreSQL(s *Source) *PostgreSQL {
	return &PostgreSQL{source: s}
}

func (pg *PostgreSQL) Connect() error {
	db, err := pgx.Connect(context.Background(), pg.getConnectionString())
	pg.db = db
	return err
}

func (pg *PostgreSQL) CheckRequiredToolsAreInstalled() {
	checkTools("pg_dump", "strings", "pg_restore")
}

func (pg *PostgreSQL) GetTableRowCount(tableName string) int64 {
	var rowCount int64
	query := fmt.Sprintf("select count(*) from %s", tableName)

	log.Infof("Querying row count of table %q", tableName)
	err := pg.db.QueryRow(context.Background(), query).Scan(&rowCount)
	if err != nil {
		utils.ErrExit("Failed to query row count of %q: %s", tableName, err)
	}
	log.Infof("Table %q has %v rows.", tableName, rowCount)
	return rowCount
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
	query := `SELECT table_schema, table_name
			  FROM information_schema.tables
			  WHERE table_type = 'BASE TABLE' AND
			        table_schema NOT IN ('pg_catalog', 'information_schema');`

	rows, err := pg.db.Query(context.Background(), query)
	if err != nil {
		utils.ErrExit("error in querying source database for table names: %v\n", err)
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

func (pg *PostgreSQL) getConnectionString() string {
	source := pg.source

	if source.Uri != "" {
		return source.Uri
	}

	return fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?%s", source.User, source.Password,
		source.Host, source.Port, source.DBName, generateSSLQueryStringIfNotExists(source))
}

func (pg *PostgreSQL) ExportSchema(exportDir string) {
	pgdumpExtractSchema(pg.source, exportDir)
}

func (pg *PostgreSQL) ExportData(ctx context.Context, exportDir string, tableList []string, quitChan chan bool, exportDataStart chan bool) {
	pgdumpExportDataOffline(ctx, pg.source, exportDir, tableList, quitChan, exportDataStart)
}

func (pg *PostgreSQL) ExportDataPostProcessing(source *Source, exportDir string, tablesProgressMetadata *map[string]*utils.TableProgressMetadata) {
	renameDataFiles(tablesProgressMetadata)
	saveExportedRowCount(exportDir, tablesProgressMetadata)
}
