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

type TargetYugabyteDB struct {
	sync.Mutex
	tconf *TargetConf
	conn  *pgx.Conn
}

func newTargetYugabyteDB(tconf *TargetConf) *TargetYugabyteDB {
	return &TargetYugabyteDB{tconf: tconf}
}

// TODO We should not export `Conn`. This is temporary--until we refactor all target db access.
func (yb *TargetYugabyteDB) Conn() *pgx.Conn {
	if yb.conn == nil {
		utils.ErrExit("Called TargetDB.Conn() before TargetDB.Connect()")
	}
	return yb.conn
}

func (yb *TargetYugabyteDB) Connect() error {
	if yb.conn != nil {
		// Already connected.
		return nil
	}
	yb.Mutex.Lock()
	defer yb.Mutex.Unlock()
	connStr := yb.tconf.GetConnectionUri()
	conn, err := pgx.Connect(context.Background(), connStr)
	if err != nil {
		return fmt.Errorf("connect to target db: %s", err)
	}
	yb.conn = conn
	return nil
}

func (yb *TargetYugabyteDB) Disconnect() {
	if yb.conn == nil {
		log.Infof("No connection to the target database to close")
	}

	err := yb.conn.Close(context.Background())
	if err != nil {
		log.Infof("Failed to close connection to the target database: %s", err)
	}
}

func (yb *TargetYugabyteDB) EnsureConnected() {
	err := yb.Connect()
	if err != nil {
		utils.ErrExit("Failed to connect to the target DB: %s", err)
	}
}

func (yb *TargetYugabyteDB) GetVersion() string {
	if yb.tconf.dbVersion != "" {
		return yb.tconf.dbVersion
	}

	yb.EnsureConnected()
	yb.Mutex.Lock()
	defer yb.Mutex.Unlock()
	query := "SELECT setting FROM pg_settings WHERE name = 'server_version'"
	err := yb.conn.QueryRow(context.Background(), query).Scan(&yb.tconf.dbVersion)
	if err != nil {
		utils.ErrExit("get target db version: %s", err)
	}
	return yb.tconf.dbVersion
}
