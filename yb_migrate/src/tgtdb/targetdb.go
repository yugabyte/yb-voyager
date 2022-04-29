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

// TODO We should not export `Conn`. This is temporary--until we refactor all target db access.
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

func (tdb *TargetDB) EnsureConnected() {
	err := tdb.Connect()
	if err != nil {
		utils.ErrExit("Failed to connect to the target DB: %s", err)
	}
}

func (tdb *TargetDB) GetVersion() string {
	tdb.EnsureConnected()
	var version string
	query := "SELECT setting FROM pg_settings WHERE name = 'server_version'"
	err := tdb.conn.QueryRow(context.Background(), query).Scan(&version)
	if err != nil {
		utils.ErrExit("get target db version: %s", err)
	}
	return version
}
