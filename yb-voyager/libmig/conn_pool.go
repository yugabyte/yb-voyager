package libmig

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"

	"github.com/jackc/pgx/v4"
	log "github.com/sirupsen/logrus"
)

// TODO: Unify with tgtdb.ConnectionPool.

type ConnectionParams struct {
	NumConnections int
	ConnUriList    []string

	EnableUpsertMode           bool
	DisableTransactionalWrites bool
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

	// If UPSERT mode is not available/opted, the COPY must be transactional:
	//     When UPSERT mode is not enabled and transactions are disabled, a COPY command can
	//     fail midway. Upon retrying the same batch, COPY will fail with the "unique key violation" error.
	//     In this case, there is no way to take the batch to completion. Hence transactional mode
	//     is a MUST, when UPSERT mode is not enabled.
	//
	//     When UPSERT mode is NOT enabled AND transactional mode is enabled AND
	//       COPY command fails with the "unique key violation" error,
	//     it indicates that the batch was already fully inserted on the target. Safe to ignore the error.
	//
	// If UPSERT mode is active, transactions can be enabled or disabled.
	//     When UPSERT mode is enabled, retrying a partially inserted batch will not** produce the
	//     "unique key violation" error--irrespective of whether transactions are enabled/disabled.
	//
	// ** CAVEAT:
	//    - Because of https://github.com/yugabyte/yb-voyager/issues/235, retrying the same batch in
	//      UPSERT mode, DOES (incorrectly!) produce the "unique key violation" error.
	//    - Because of https://github.com/yugabyte/yb-voyager/issues/239, when transactions are disabled,
	//      a COPY command can fail with the "Illegal state: Used read time is not set".
	//
	// CONCLUSION: Until the above two issues are fixed, only the transactional mode is known
	//             to work correctly in all cases.
	//
	// Our eventual goal is to use the fastest combination: upsert mode enabled and transactions disabled.
	if pool.params.EnableUpsertMode {
		err = setSessionVar(conn, "SET yb_enable_upsert_mode TO true")
		if err != nil {
			if strings.Contains(err.Error(), "unrecognized configuration parameter") {
				msg := "UPSERT mode is not supported on the target YB version; " +
					"retry after removing the --enable-upsert option"
				return errors.New(msg)
			}
			return err
		}
		if pool.params.DisableTransactionalWrites {
			return setSessionVar(conn, "SET yb_disable_transactional_writes TO true")
		}
	} // else COPY is transactional.
	return nil
}

func setSessionVar(conn *pgx.Conn, sqlStmt string) error {
	_, err := conn.Exec(context.Background(), sqlStmt)
	log.Infof("%s ==> %v", sqlStmt, err)
	if err != nil {
		return fmt.Errorf("%s: %w", sqlStmt, err)
	}
	return nil
}
