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
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/stretchr/testify/assert"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
)

func TestBasic(t *testing.T) {
	// GIVEN: a conn pool of size 10.
	size := 10
	postgres := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
		Username("postgres").
		Password("postgres").
		Database("test").
		Port(9876).
		StartTimeout(45 * time.Second))
	err := postgres.Start()
	assert.NoError(t, err)
	defer postgres.Stop()

	connParams := &ConnectionParams{
		NumConnections:    size,
		NumMaxConnections: size,
		ConnUriList:       []string{fmt.Sprintf("postgresql://postgres:postgres@localhost:%d/test", 9876)},
		SessionInitScript: []string{},
	}
	pool := NewConnectionPool(connParams)
	assert.Equal(t, size, len(pool.conns))

	// WHEN: multiple goroutines acquire connection, perform some operation
	// and release connection back to pool
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go dummyProcess(pool, 1, &wg)
	}
	wg.Wait()

	// THEN: all connections are released back to pool
	assert.Equal(t, size, len(pool.conns))
}

func dummyProcess(pool *ConnectionPool, seconds int, wg *sync.WaitGroup) {
	defer wg.Done()
	_ = pool.WithConn(func(conn *pgx.Conn) (retry bool, err error) {
		time.Sleep(time.Duration(seconds) * time.Second)
		return false, nil
	})
}
