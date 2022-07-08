package main

import (
	"context"
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
		if err == nil {
			// Connection established.
			log.Infof("Connected to %q", uri)
			break
		}
	}
	return
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
