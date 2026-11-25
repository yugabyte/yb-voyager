//go:build unit

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
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The prepared-statement cache helpers key only on the *pgx.Conn pointer; they
// never dereference the connection. So we can exercise the collision logic with
// bare conn objects and no real database.
func newTestPool(t *testing.T) *ConnectionPool {
	t.Helper()
	pool, err := NewConnectionPool(&ConnectionParams{
		NumConnections:    1,
		NumMaxConnections: 1,
		ConnUriList:       []string{"postgres://localhost/test"},
	})
	require.NoError(t, err)
	return pool
}

// Regression test for the PID-collision bug. Two distinct connections that
// (under the old PID-keyed cache) could share a backend PID across nodes must
// be tracked independently. Caching a statement on one connection must never
// make us believe it is prepared on the other.
func TestPreparedStmtCache_NoCrossConnCollision(t *testing.T) {
	pool := newTestPool(t)

	// Two separate connection objects. With the old uint32-PID key these could
	// collide; with the *pgx.Conn key they are always distinct.
	conn1 := &pgx.Conn{}
	conn2 := &pgx.Conn{}

	const stmt = "stmt_abc"

	// Initially neither connection has the statement prepared.
	assert.False(t, pool.isStmtAlreadyPreparedOnConn(conn1, stmt))
	assert.False(t, pool.isStmtAlreadyPreparedOnConn(conn2, stmt))

	// Prepare on conn1 only.
	pool.cachePreparedStmtForConn(conn1, stmt)

	assert.True(t, pool.isStmtAlreadyPreparedOnConn(conn1, stmt))
	// conn2 must still report "not prepared" — this is the bug the PID key caused.
	assert.False(t, pool.isStmtAlreadyPreparedOnConn(conn2, stmt))
}

// Distinct statement names on the same connection are tracked independently.
func TestPreparedStmtCache_PerStatement(t *testing.T) {
	pool := newTestPool(t)
	conn := &pgx.Conn{}

	pool.cachePreparedStmtForConn(conn, "stmt_a")

	assert.True(t, pool.isStmtAlreadyPreparedOnConn(conn, "stmt_a"))
	assert.False(t, pool.isStmtAlreadyPreparedOnConn(conn, "stmt_b"))
}

// Closing a connection must evict its cache entry so the entry (which holds a
// live reference to the conn) does not leak.
func TestPreparedStmtCache_RemoveOnClose(t *testing.T) {
	pool := newTestPool(t)
	conn := &pgx.Conn{}

	pool.cachePreparedStmtForConn(conn, "stmt_a")
	assert.True(t, pool.isStmtAlreadyPreparedOnConn(conn, "stmt_a"))

	pool.removePreparedStmtCacheForConn(conn)

	assert.False(t, pool.isStmtAlreadyPreparedOnConn(conn, "stmt_a"))
	_, present := pool.connToPreparedStmtCache[conn]
	assert.False(t, present, "cache entry should be removed on close")
}

func unreachableTargetURI(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	require.NoError(t, listener.Close())
	return fmt.Sprintf("postgres://yugabyte@%s/yugabyte?sslmode=disable&connect_timeout=1", addr)
}

func withConnResult(pool *ConnectionPool) <-chan error {
	result := make(chan error, 1)
	go func() {
		result <- pool.WithConn(func(conn *pgx.Conn) (bool, error) {
			return false, nil
		})
	}()
	return result
}

func TestWithConn_FailedConnectDoesNotLeakPoolSlot(t *testing.T) {
	const withConnTimeout = 5 * time.Second

	testCases := []struct {
		name                string
		numConnections      int
		numMaxConnections   int
		pendingConnsToClose int
	}{
		{name: "parallel-jobs 1", numConnections: 1, numMaxConnections: 1},
		{name: "parallel-jobs 4", numConnections: 4, numMaxConnections: 4},
		{name: "adaptive parallelism with headroom", numConnections: 2, numMaxConnections: 8},
		{name: "adaptive parallelism with a pending shrink", numConnections: 4, numMaxConnections: 8, pendingConnsToClose: 2},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pool, err := NewConnectionPool(&ConnectionParams{
				NumConnections:    tc.numConnections,
				NumMaxConnections: tc.numMaxConnections,
				ConnUriList:       []string{unreachableTargetURI(t)},
			})
			require.NoError(t, err)
			if tc.pendingConnsToClose > 0 {
				require.NoError(t, pool.UpdateNumConnections(-tc.pendingConnsToClose))
			}

			// One more attempt than there are slots: with the leak, the last one finds the pool empty.
			for attempt := 1; attempt <= tc.numConnections+1; attempt++ {
				select {
				case err := <-withConnResult(pool):
					var connectErr *pgconn.ConnectError
					require.ErrorAs(t, err, &connectErr, "attempt %d", attempt)
				case <-time.After(withConnTimeout):
					require.FailNowf(t, "WithConn hung waiting for a pool slot",
						"attempt %d: slots in conns=%d idleConns=%d, pool size=%d",
						attempt, len(pool.conns), len(pool.idleConns), pool.GetNumConnections())
				}
			}

			assert.Equal(t, tc.numMaxConnections, len(pool.conns)+len(pool.idleConns),
				"every slot must be back in conns or idleConns once no WithConn call is in flight")
			assert.Equal(t, 0, pool.pendingConnsToClose, "failed connects must consume a pending shrink")
			assert.Equal(t, pool.GetNumConnections(), len(pool.conns), "conns must hold exactly pool.size slots when idle")
		})
	}
}
