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
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v4"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var MAX_CONNS = 100
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
	params                    *ConnectionParams
	conns                     chan *pgx.Conn
	connIdToPreparedStmtCache map[uint32]map[string]bool // cache list of prepared statements per connection
	nextUriIndex              int
	disableThrottling         bool
	size                      int
	sizeChangeRequests        chan int
	connLock                  sync.Mutex
}

func NewConnectionPool(params *ConnectionParams) *ConnectionPool {
	pool := &ConnectionPool{
		params:                    params,
		conns:                     make(chan *pgx.Conn, MAX_CONNS),
		connIdToPreparedStmtCache: make(map[uint32]map[string]bool),
		disableThrottling:         false,
		size:                      params.NumConnections,
		sizeChangeRequests:        make(chan int, 1),
	}
	for i := 0; i < params.NumConnections; i++ {
		pool.conns <- nil
	}
	if pool.params.SessionInitScript == nil {
		pool.params.SessionInitScript = defaultSessionVars
	}
	return pool
}

func (pool *ConnectionPool) GetNumConnections() int {
	return pool.size
}

func (pool *ConnectionPool) UpdateNumConnections(newSize int) bool {
	if newSize < 1 || newSize > MAX_CONNS {
		return false
	}
	select {
	case pool.sizeChangeRequests <- newSize:
		return true
	default:
		return false
	}
}

func (pool *ConnectionPool) DisableThrottling() {
	pool.disableThrottling = true
}

func (pool *ConnectionPool) WithConn(fn func(*pgx.Conn) (bool, error)) error {
	var err error
	retry := true

	for retry {
		var conn *pgx.Conn
		var gotIt bool
		if pool.disableThrottling {
			pool.connLock.Lock()
			// deal with changes in size change requests
			select {
			case newSize := <-pool.sizeChangeRequests:
				if newSize > pool.size {
					for i := 0; i < newSize-pool.size; i++ {
						pool.conns <- nil
					}
				} else if newSize < pool.size {
					for i := 0; i < pool.size-newSize; i++ {
						conn = <-pool.conns
						if conn != nil {
							conn.Close(context.Background())
						}
					}
				}
				oldSize := pool.size
				pool.size = newSize
				utils.PrintAndLog("PARALLELISM: Updated pool size from %d to %d", oldSize, newSize)
			default:

			}

			conn = <-pool.conns
			pool.connLock.Unlock()
		} else {
			pool.connLock.Lock()
			conn, gotIt = <-pool.conns
			if !gotIt {
				pool.connLock.Unlock()
				// The following sleep is intentional. It is added so that voyager does not
				// overwhelm the database. See the description in PR https://github.com/yugabyte/yb-voyager/pull/920 .
				time.Sleep(2 * time.Second)
				continue
			}
			pool.connLock.Unlock()
		}
		if conn == nil {
			conn, err = pool.createNewConnection()
			if err != nil {
				return err
			}
		}

		retry, err = fn(conn)
		if err != nil {
			// On err, drop the connection and clear the prepared statement cache.
			conn.Close(context.Background())
			pool.Lock()
			// assuming PID will still be available
			delete(pool.connIdToPreparedStmtCache, conn.PgConn().PID())
			pool.Unlock()
			pool.conns <- nil
		} else {
			pool.conns <- conn
		}
	}

	return err
}

func (pool *ConnectionPool) PrepareStatement(conn *pgx.Conn, stmtName string, stmt string) error {
	if pool.isStmtAlreadyPreparedOnConn(conn.PgConn().PID(), stmtName) {
		return nil
	}

	_, err := conn.Prepare(context.Background(), stmtName, stmt)
	if err != nil {
		log.Errorf("failed to prepare statement %q: %s", stmtName, err)
		return fmt.Errorf("failed to prepare statement %q: %w", stmtName, err)
	}
	pool.cachePreparedStmtForConn(conn.PgConn().PID(), stmtName)
	return err
}

func (pool *ConnectionPool) cachePreparedStmtForConn(connId uint32, ps string) {
	pool.Lock()
	defer pool.Unlock()
	if pool.connIdToPreparedStmtCache[connId] == nil {
		pool.connIdToPreparedStmtCache[connId] = make(map[string]bool)
	}
	pool.connIdToPreparedStmtCache[connId][ps] = true
}

func (pool *ConnectionPool) isStmtAlreadyPreparedOnConn(connId uint32, ps string) bool {
	pool.Lock()
	defer pool.Unlock()
	if pool.connIdToPreparedStmtCache[connId] == nil {
		return false
	}
	return pool.connIdToPreparedStmtCache[connId][ps]
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
			if strings.Contains(err.Error(), ERROR_MSG_PERMISSION_DENIED) {
				return nil
			}
			return err
		}
	}
	return nil
}
