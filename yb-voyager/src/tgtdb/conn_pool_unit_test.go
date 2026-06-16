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
	"testing"

	"github.com/jackc/pgx/v4"
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
