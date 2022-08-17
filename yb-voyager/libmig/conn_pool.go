package libmig

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"

	"github.com/jackc/pgx/v4"
	log "github.com/sirupsen/logrus"
)

// TODO: Unify with tgtdb.ConnectionPool.

var sessionVars = map[string]string{
	"client_encoding":          "'UTF-8'",
	"session_replication_role": "replica",
}

type ConnectionParams struct {
	NumConnections int
	ConnUriList    []string
	SessionVars    map[string]string
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
	if pool.params.SessionVars == nil {
		pool.params.SessionVars = map[string]string{}
	}
	for k, v := range sessionVars {
		pool.params.SessionVars[k] = v
	}
	return pool
}

func (pool *ConnectionPool) WithConn(fn func(*pgx.Conn) error) error {
	var err error

	conn := <-pool.conns
	if conn == nil {
		conn, err = pool.createNewConnection()
		if err != nil {
			return err
		}
	}

	err = fn(conn)

	if err != nil {
		// On err, drop the connection.
		conn.Close(context.Background())
		pool.conns <- nil
	} else {
		pool.conns <- conn
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
	if err != nil {
		log.Warnf("Failed to connect to %q: %s", uri, err)
		return nil, err
	}
	log.Infof("Connected to %q", uri)
	err = pool.setSessionVars(conn)
	if err != nil {
		log.Warnf("Failed to set session vars %q: %s", uri, err)
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

func (pool *ConnectionPool) setSessionVars(conn *pgx.Conn) error {
	err := setSessionVar(conn, "SET client_encoding TO 'UTF-8'")
	if err != nil {
		return err
	}
	err = setSessionVar(conn, "SET session_replication_role TO replica")
	if err != nil {
		return err
	}
	err = setSessionVar(conn, "SET yb_enable_upsert_mode TO true")
	if err != nil {
		if strings.Contains(err.Error(), "unrecognized configuration parameter") {
			log.Warnf("UPSERT mode is not available. Using transactional COPY.")
			return nil
		} else {
			return err
		}
	}
	return setSessionVar(conn, "SET yb_disable_transactional_writes TO true")
}

func setSessionVar(conn *pgx.Conn, sqlStmt string) error {
	_, err := conn.Exec(context.Background(), sqlStmt)
	log.Infof("%s ==> %v", sqlStmt, err)
	if err != nil {
		return fmt.Errorf("%s: %w", sqlStmt, err)
	}
	return nil
}
