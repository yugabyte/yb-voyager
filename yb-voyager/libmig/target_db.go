package libmig

import (
	"context"
	"io"
	"strings"

	"github.com/jackc/pgx/v4"
	log "github.com/sirupsen/logrus"
)

type TargetDB struct {
	connPool *ConnectionPool
}

func NewTargetDB(connPool *ConnectionPool) *TargetDB {
	return &TargetDB{connPool: connPool}
}

func (tdb *TargetDB) Copy(ctx context.Context, copyCommand string, r io.Reader) (int64, error) {
	var rowsAffected int64
	err := tdb.connPool.WithConn(func(c *pgx.Conn) error {
		res, err := c.PgConn().CopyFrom(context.Background(), r, copyCommand)
		rowsAffected = res.RowsAffected()
		log.Infof("%q => rowsAffected: %v", copyCommand, rowsAffected)
		if err != nil && strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			err = nil
		}
		return err
	})

	return rowsAffected, err
}
