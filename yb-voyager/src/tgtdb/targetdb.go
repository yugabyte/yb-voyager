/*
Copyright (c) YugabyteDB, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package tgtdb

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v4"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type TargetDB struct {
	sync.Mutex
	tconf *TargetConf
	conn  *pgx.Conn
}

func newTargetDB(tconf *TargetConf) *TargetDB {
	return &TargetDB{tconf: tconf}
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
	tdb.Mutex.Lock()
	defer tdb.Mutex.Unlock()
	connStr := tdb.tconf.GetConnectionUri()
	conn, err := pgx.Connect(context.Background(), connStr)
	if err != nil {
		return fmt.Errorf("connect to target db: %s", err)
	}
	tdb.conn = conn
	return nil
}

func (tdb *TargetDB) Disconnect() {
	if tdb.conn == nil {
		log.Infof("No connection to the target database to close")
	}

	err := tdb.conn.Close(context.Background())
	if err != nil {
		log.Infof("Failed to close connection to the target database: %s", err)
	}
}

func (tdb *TargetDB) EnsureConnected() {
	err := tdb.Connect()
	if err != nil {
		utils.ErrExit("Failed to connect to the target DB: %s", err)
	}
}

func (tdb *TargetDB) GetVersion() string {
	if tdb.tconf.dbVersion != "" {
		return tdb.tconf.dbVersion
	}

	tdb.EnsureConnected()
	tdb.Mutex.Lock()
	defer tdb.Mutex.Unlock()
	query := "SELECT setting FROM pg_settings WHERE name = 'server_version'"
	err := tdb.conn.QueryRow(context.Background(), query).Scan(&tdb.tconf.dbVersion)
	if err != nil {
		utils.ErrExit("get target db version: %s", err)
	}
	return tdb.tconf.dbVersion
}
