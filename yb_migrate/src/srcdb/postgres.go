package srcdb

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v4"
	log "github.com/sirupsen/logrus"
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

func (pg *PostgreSQL) GetTableRowCount(tableName string) int64 {
	var rowCount int64
	query := fmt.Sprintf("select count(*) from %s", tableName)

	log.Infof("Querying row count of table %q", tableName)
	err := pg.db.QueryRow(context.Background(), query).Scan(&rowCount)
	if err != nil {
		ErrExit("Failed to query row count of %q: %s", tableName, err)
	}
	log.Infof("Table %q has %v rows.", tableName, rowCount)
	return rowCount
}

func (pg *PostgreSQL) getConnectionString() string {
	source := pg.source

	if source.Uri != "" {
		return source.Uri
	}

	return fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?%s", source.User, source.Password,
		source.Host, source.Port, source.DBName, generateSSLQueryStringIfNotExists(source))
}
