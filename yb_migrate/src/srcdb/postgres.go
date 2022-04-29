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

func (pg *PostgreSQL) getConnectionString() string {
	source := pg.source

	if source.Uri != "" {
		return source.Uri
	}

	return fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?%s", source.User, source.Password,
		source.Host, source.Port, source.DBName, generateSSLQueryStringIfNotExists(source))
}
