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
	"math/rand"
	"sync"
	"time"

	"github.com/jackc/pgx/v4"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var defaultSessionVars = []string{
	"SET client_encoding to 'UTF-8'",
	"SET session_replication_role to replica",
}

type ConnectionParams struct {
	NumConnections    int
	ConnUriList       []string
	SessionInitScript []string
}

type ConnectionPool struct {
	sync.Mutex
	params       *ConnectionParams
	conns        chan *pgx.Conn
	nextUriIndex int
}

func NewConnectionPool(params *ConnectionParams) *ConnectionPool {
	pool := &ConnectionPool{
		params: params,
		conns:  make(chan *pgx.Conn, params.NumConnections),
	}
	for i := 0; i < params.NumConnections; i++ {
		pool.conns <- nil
	}
	if pool.params.SessionInitScript == nil {
		pool.params.SessionInitScript = defaultSessionVars
	}
	return pool
}

func (pool *ConnectionPool) WithConn(fn func(*pgx.Conn) (bool, error)) error {
	var err error
	retry := true

	for retry {
		conn, gotIt := <-pool.conns
		if !gotIt {
			// The following sleep is intentional. It is added so that voyager does not
			// overwhelm the database. See the description in PR https://github.com/yugabyte/yb-voyager/pull/920 .
			time.Sleep(2 * time.Second)
			continue
		}
		if conn == nil {
			conn, err = pool.createNewConnection()
			if err != nil {
				return err
			}
		}

		retry, err = fn(conn)
		if err != nil {
			// On err, drop the connection.
			conn.Close(context.Background())
			pool.conns <- nil
		} else {
			pool.conns <- conn
		}
	}

	return err
}

func (pool *ConnectionPool) createNewConnection() (*pgx.Conn, error) {
	idx := pool.getNextUriIndex()
	uri := pool.params.ConnUriList[idx]
	conn, err := pool.connect(uri)
	if err != nil {
		for _, uri := range pool.shuffledConnUriList() {
			conn, err = pool.connect(uri)
			if err == nil {
				break
			}
		}
	}
	return conn, err
}

func (pool *ConnectionPool) connect(uri string) (*pgx.Conn, error) {
	conn, err := pgx.Connect(context.Background(), uri)
	redactedUri := utils.GetRedactedURLs([]string{uri})[0]
	if err != nil {
		log.Warnf("Failed to connect to %q: %s", redactedUri, err)
		return nil, err
	}
	log.Infof("Connected to %q", redactedUri)
	err = pool.initSession(conn)
	if err != nil {
		log.Warnf("Failed to set session vars %q: %s", redactedUri, err)
		conn.Close(context.Background())
		conn = nil
	}
	return conn, err
}

func (pool *ConnectionPool) shuffledConnUriList() []string {
	connUriList := make([]string, len(pool.params.ConnUriList))
	copy(connUriList, pool.params.ConnUriList)

	rand.Shuffle(len(connUriList), func(i, j int) {
		connUriList[i], connUriList[j] = connUriList[j], connUriList[i]
	})
	return connUriList
}

func (pool *ConnectionPool) getNextUriIndex() int {
	pool.Lock()
	defer pool.Unlock()

	pool.nextUriIndex = (pool.nextUriIndex + 1) % len(pool.params.ConnUriList)

	return pool.nextUriIndex
}

func (pool *ConnectionPool) initSession(conn *pgx.Conn) error {
	for _, v := range pool.params.SessionInitScript {
		_, err := conn.Exec(context.Background(), v)
		if err != nil {
			return err
		}
	}
	return nil
}
