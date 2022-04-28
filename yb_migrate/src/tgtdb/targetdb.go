package tgtdb

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v4"
	"github.com/yugabyte/yb-db-migration/yb_migrate/src/utils"
)

type TargetDB struct {
	target *Target

	conn *pgx.Conn
}

func newTargetDB(target *Target) *TargetDB {
	return &TargetDB{target: target}
}

func (tdb *TargetDB) Conn() *pgx.Conn {
	if tdb.conn == nil {
		utils.ErrExit("Called TargetDB.Conn() before TargetDB.Connect()")
	}
	return tdb.conn
}

func (tdb *TargetDB) Connect() error {
	if tdb.conn != nil {
		// Already connected.
		return nil
	}
	connStr := tdb.target.GetConnectionUri()
	conn, err := pgx.Connect(context.Background(), connStr)
	if err != nil {
		return fmt.Errorf("connect to target db: %s", err)
	}
	tdb.conn = conn
	return nil
}

func (tdb *TargetDB) GetVersion() string {
	return ""
}
