package srcdb

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v4"
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

func (pg *PostgreSQL) getConnectionString() string {
	source := pg.source

	if source.Uri != "" {
		return source.Uri
	}

	return fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?%s", source.User, source.Password,
		source.Host, source.Port, source.DBName, generateSSLQueryStringIfNotExists(source))
}
