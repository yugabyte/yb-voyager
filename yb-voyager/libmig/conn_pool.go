package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v4"
	log "github.com/sirupsen/logrus"
)

type ConnectionParams struct {
	NumConnections int
	ConnUriList    []string
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

func (pool *ConnectionPool) createNewConnection() (conn *pgx.Conn, err error) {
	idx := pool.getNextUriIndex()
	n := len(pool.params.ConnUriList)
	for i := 0; i < n; i++ {
		uri := pool.params.ConnUriList[(idx+i)%n]
		conn, err = pgx.Connect(context.Background(), uri)
		if err != nil {
			log.Warnf("Failed to connect to %q: %s", uri, err)
			continue
		}
		// Connection established.
		log.Infof("Connected to %q", uri)
		err = pool.setSessionVars(conn)
		if err != nil {
			conn.Close(context.Background())
			return nil, err
		}
		break
	}
	return conn, err
}

func (pool *ConnectionPool) getNextUriIndex() int {
	pool.Lock()
	defer pool.Unlock()

	pool.nextUriIndex++
	if pool.nextUriIndex == len(pool.params.ConnUriList) {
		pool.nextUriIndex = 0
	}
	return pool.nextUriIndex
}

var sessionVars = map[string]string{
	"client_encoding":                 "'UTF-8'",
	"yb_disable_transactional_writes": "true",
	"session_replication_role":        "replica",
	"yb_enable_upsert_mode":           "true",
}

func (pool *ConnectionPool) setSessionVars(conn *pgx.Conn) error {
	for k, v := range sessionVars {
		// TODO: Add version check.
		cmd := fmt.Sprintf("SET %s TO %s;", k, v)
		_, err := conn.Exec(context.Background(), cmd)
		if err != nil {
			return err
		}
	}
	return nil
}
