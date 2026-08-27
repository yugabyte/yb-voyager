//go:build integration

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
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SQLSTATE raised when the server terminates a session that sat idle inside a
// transaction for longer than idle_in_transaction_session_timeout.
const sqlStateIdleInTxnTimeout = "25P03"

// Covers what only a real target can show: that a connection handed out by the
// import pool actually carries the timeout.
func TestPooledConnectionHasIdleInTransactionTimeoutSet(t *testing.T) {
	yb, ok := testYugabyteDBTarget.TargetDB.(*TargetYugabyteDB)
	require.True(t, ok, "expected a *TargetYugabyteDB")

	// Init() runs during suite setup, which is what populates SessionVars.
	require.NotEmpty(t, yb.Tconf.SessionVars)

	pool, err := NewConnectionPool(&ConnectionParams{
		NumConnections:    1,
		NumMaxConnections: 1,
		ConnUriList:       []string{testYugabyteDBTarget.GetConnectionString()},
		SessionInitScript: yb.Tconf.SessionVars,
	})
	require.NoError(t, err)

	var timeout string
	err = pool.WithConn(func(conn *pgx.Conn) (bool, error) {
		return false, conn.QueryRow(context.Background(),
			"SHOW idle_in_transaction_session_timeout").Scan(&timeout)
	})
	require.NoError(t, err)
	assert.Equal(t, "5min", timeout)
}

// The whole point of the timeout is that the server kills an orphaned
// in-transaction session. When that happens to a live pooled connection, the
// caller must see 25P03 and the pool must recover: WithConn() discards the dead
// connection and the next caller gets a fresh, usable one.
func TestPoolRecoversFromIdleInTransactionTermination(t *testing.T) {
	pool, err := NewConnectionPool(&ConnectionParams{
		NumConnections:    1,
		NumMaxConnections: 1,
		ConnUriList:       []string{testYugabyteDBTarget.GetConnectionString()},
		// Deliberately tiny so the test does not wait out the real 5min default.
		SessionInitScript: []string{"SET idle_in_transaction_session_timeout = '1s'"},
	})
	require.NoError(t, err)

	// WHEN: a transaction sits idle past the timeout.
	err = pool.WithConn(func(conn *pgx.Conn) (bool, error) {
		ctx := context.Background()
		if _, err := conn.Exec(ctx, "BEGIN"); err != nil {
			return false, err
		}
		time.Sleep(3 * time.Second)
		_, err := conn.Exec(ctx, "SELECT 1")
		return false, err
	})

	// THEN: the server terminated it, and the error says why.
	require.Error(t, err)
	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr), "expected a *pgconn.PgError, got %v", err)
	assert.Equal(t, sqlStateIdleInTxnTimeout, pgErr.Code)

	// AND: the pool replaced the dead connection, so it is usable again.
	var one int
	err = pool.WithConn(func(conn *pgx.Conn) (bool, error) {
		return false, conn.QueryRow(context.Background(), "SELECT 1").Scan(&one)
	})
	require.NoError(t, err)
	assert.Equal(t, 1, one)
}
